package metrics

import (
	"encoding/json"
	"net/http"
	"sync"
)

// Metrics tracks request counts, status codes, and per-backend stats.
type Metrics struct {
	mu            sync.Mutex
	totalRequests int64
	statusCodes   map[int]int64
	backendCounts map[string]int64
}

// NewMetrics creates a new metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{
		statusCodes:   make(map[int]int64),
		backendCounts: make(map[string]int64),
	}
}

// Record increments counters for a proxied request.
func (m *Metrics) Record(backendURL string, statusCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalRequests++
	m.statusCodes[statusCode]++
	m.backendCounts[backendURL]++
}

type snapshot struct {
	TotalRequests int64            `json:"total_requests"`
	StatusCodes   map[int]int64    `json:"status_codes"`
	BackendCounts map[string]int64 `json:"backend_counts"`
}

// Handler returns an HTTP handler that serves metrics as JSON.
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		// Copy maps so we don't hold lock during JSON encoding.
		sc := make(map[int]int64, len(m.statusCodes))
		for k, v := range m.statusCodes {
			sc[k] = v
		}
		bc := make(map[string]int64, len(m.backendCounts))
		for k, v := range m.backendCounts {
			bc[k] = v
		}
		s := snapshot{
			TotalRequests: m.totalRequests,
			StatusCodes:   sc,
			BackendCounts: bc,
		}
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s)
	}
}
