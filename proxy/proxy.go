package proxy

import (
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/go-load-balancer/balancer"
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

// NewHandler returns an HTTP handler that proxies requests through the balancer.
func NewHandler(b *balancer.Balancer, m *metrics.Metrics, l *logging.Logger) http.HandlerFunc {
	// Pre-build a transport shared across requests for connection pooling.
	transport := &http.Transport{
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		backend, err := b.Next(r)
		if err != nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			l.WithRequestID(r.Context(), "error", "no healthy backends", map[string]interface{}{
				"method": r.Method,
				"path":   r.URL.Path,
			})
			m.Record("none", "503", r.Method, 0, 0)
			return
		}

		backendURL := backend.URL.String()
		backend.IncrementConnections()
		defer func() {
			backend.DecrementConnections()
			// Update active connections metric
			m.SetActiveConnections(backendURL, backend.ActiveConnections())
		}()

		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = backend.URL.Scheme
				req.URL.Host = backend.URL.Host
				req.Host = backend.URL.Host
			},
			Transport: transport,
			ErrorHandler: func(rw http.ResponseWriter, req *http.Request, e error) {
				l.WithRequestID(r.Context(), "error", "proxy error", map[string]interface{}{
					"backend": backendURL,
					"error":   e.Error(),
				})
				b.SetAlive(backendURL, false)
				http.Error(rw, "bad gateway", http.StatusBadGateway)
			},
		}

		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		proxy.ServeHTTP(sw, r)

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
