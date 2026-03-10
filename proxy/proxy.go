package proxy

import (
	"net/http"
	"net/http/httputil"

	"github.com/go-load-balancer/balancer"
	"github.com/go-load-balancer/logging"
	"github.com/go-load-balancer/metrics"
)

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.code = code
	sw.ResponseWriter.WriteHeader(code)
}

func NewHandler(b *balancer.Balancer, m *metrics.Metrics, l *logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		backend, err := b.Next()
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
		l.Info("proxying request", map[string]interface{}{
			"method":  r.Method,
			"path":    r.URL.Path,
			"backend": backendURL,
		})

		proxy := httputil.NewSingleHostReverseProxy(backend.URL)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
			l.Error("proxy error", map[string]interface{}{
				"backend": backendURL,
				"error":   e.Error(),
			})
			b.SetHealthy(backendURL, false)
			http.Error(w, "bad gateway", http.StatusBadGateway)
		}

		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		proxy.ServeHTTP(sw, r)
		m.Record(backendURL, sw.code)
	}
}
