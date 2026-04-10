package proxy

import (
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/go-load-balancer/balancer"
	"github.com/go-load-balancer/logging"
	"github.com/go-load-balancer/metrics"
)

// statusWriter wraps ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	code    int
	written bool
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
	return sw.ResponseWriter.Write(b)
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
			l.Error("no healthy backends", map[string]interface{}{
				"method": r.Method,
				"path":   r.URL.Path,
			})
			m.Record("none", http.StatusServiceUnavailable)
			return
		}

		backendURL := backend.URL.String()
		backend.IncrementConnections()
		defer backend.DecrementConnections()

		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = backend.URL.Scheme
				req.URL.Host = backend.URL.Host
				req.Host = backend.URL.Host
			},
			Transport: transport,
			ErrorHandler: func(rw http.ResponseWriter, req *http.Request, e error) {
				l.Error("proxy error", map[string]interface{}{
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
		m.Record(backendURL, sw.code)
		l.Info("request completed", map[string]interface{}{
			"method":   r.Method,
			"path":     r.URL.Path,
			"backend":  backendURL,
			"status":   sw.code,
			"duration": duration.String(),
		})
	}
}
