package proxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/go-load-balancer/balancer"
	"github.com/go-load-balancer/circuitbreaker"
	"github.com/go-load-balancer/logging"
	"github.com/go-load-balancer/metrics"
)

// statusWriter wraps ResponseWriter to capture the status code and size.
type statusWriter struct {
	http.ResponseWriter
	code    int
	written bool
	size    int
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.written {
		sw.code = code
		sw.written = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.written {
		sw.code = http.StatusOK
		sw.written = true
	}
	n, err := sw.ResponseWriter.Write(b)
	sw.size += n
	return n, err
}

// ProxyConfig holds the proxy configuration.
type ProxyConfig struct {
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
	MaxConnsPerHost     int
	MaxRetries          int
	RetryEnabled        bool
}

// NewHandler returns an HTTP handler that proxies requests through the balancer.
func NewHandler(b *balancer.Balancer, m *metrics.Metrics, l *logging.Logger, cfg ProxyConfig, circuitBreakers map[string]*circuitbreaker.CircuitBreaker) http.HandlerFunc {
	// Build transport with configurable connection pooling
	transport := &http.Transport{
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,
		MaxConnsPerHost:     cfg.MaxConnsPerHost,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var backend *balancer.Backend
		var backendURL string
		var sw *statusWriter
		var err error

		// Try to proxy request with retries
		maxAttempts := 1
		if cfg.RetryEnabled {
			maxAttempts = cfg.MaxRetries + 1
		}

		for attempt := 0; attempt < maxAttempts; attempt++ {
			// Get next healthy backend
			backend, err = b.Next(r)
			if err != nil {
				http.Error(w, "service unavailable", http.StatusServiceUnavailable)
				l.WithRequestID(r.Context(), "error", "no healthy backends", map[string]interface{}{
					"method": r.Method,
					"path":   r.URL.Path,
					"attempt": attempt + 1,
				})
				m.Record("none", "503", r.Method, 0, 0)
				return
			}

			backendURL = backend.URL.String()

			// Check circuit breaker
			if cb, exists := circuitBreakers[backendURL]; exists {
				if !cb.AllowRequest() {
					l.WithRequestID(r.Context(), "warn", "circuit breaker open", map[string]interface{}{
						"backend": backendURL,
						"attempt": attempt + 1,
					})
					if attempt < maxAttempts-1 {
						continue // Try another backend
					}
					http.Error(w, "service unavailable", http.StatusServiceUnavailable)
					m.Record(backendURL, "503", r.Method, 0, 0)
					return
				}
			}

			backend.IncrementConnections()
			
			// Create proxy for this attempt
			proxy := &httputil.ReverseProxy{
				Director: func(req *http.Request) {
					req.URL.Scheme = backend.URL.Scheme
					req.URL.Host = backend.URL.Host
					req.Host = backend.URL.Host
				},
				Transport: transport,
				ErrorHandler: func(rw http.ResponseWriter, req *http.Request, e error) {
					// Record failure in circuit breaker
					if cb, exists := circuitBreakers[backendURL]; exists {
						cb.RecordFailure()
					}
					
					l.WithRequestID(r.Context(), "error", "proxy error", map[string]interface{}{
						"backend": backendURL,
						"error":   e.Error(),
						"attempt": attempt + 1,
					})
					
					// Mark backend as potentially unhealthy
					b.SetAlive(backendURL, false)
					
					// Return error to trigger retry if enabled
					http.Error(rw, "bad gateway", http.StatusBadGateway)
				},
			}

			sw = &statusWriter{ResponseWriter: w, code: http.StatusOK}
			proxy.ServeHTTP(sw, r)
			
			// Decrement connections
			backend.DecrementConnections()
			m.SetActiveConnections(backendURL, backend.ActiveConnections())

			// Check if request was successful (5xx = failure for circuit breaker)
			if cb, exists := circuitBreakers[backendURL]; exists {
				if sw.code >= 500 {
					cb.RecordFailure()
				} else {
					cb.RecordSuccess()
				}
			}

			// If successful or last attempt, break retry loop
			if sw.code < 500 || attempt == maxAttempts-1 {
				break
			}

			// Log retry
			l.WithRequestID(r.Context(), "info", "retrying request", map[string]interface{}{
				"backend":   backendURL,
				"status":    sw.code,
				"attempt":   attempt + 1,
				"max_retry": cfg.MaxRetries,
			})
		}

		// Record metrics
		duration := time.Since(start)
		m.Record(backendURL, http.StatusText(sw.code), r.Method, duration.Seconds(), int64(sw.size))
		l.WithRequestID(r.Context(), "info", "request completed", map[string]interface{}{
			"method":   r.Method,
			"path":     r.URL.Path,
			"backend":  backendURL,
			"status":   sw.code,
			"duration": duration.String(),
			"bytes":    sw.size,
		})
	}
}

// InitCircuitBreakers initializes circuit breakers for each backend.
func InitCircuitBreakers(backends []string, threshold int64, timeout time.Duration) map[string]*circuitbreaker.CircuitBreaker {
	breakers := make(map[string]*circuitbreaker.CircuitBreaker)
	for _, url := range backends {
		breakers[url] = circuitbreaker.New(threshold, timeout)
	}
	return breakers
}

// UpdateContext adds retry and circuit breaker info to context.
func UpdateContext(ctx context.Context, key string, value interface{}) context.Context {
	return context.WithValue(ctx, key, value)
}
