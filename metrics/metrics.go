package metrics

import (
	"encoding/json"
	"net/http"
	"sync"
)

type Metrics struct {
	mu            sync.Mutex
	totalRequests int64
	statusCodes   map[int]int64
	backendCounts map[string]int64
}

func NewMetrics() *Metrics {
	return &Metrics{
		statusCodes:   make(map[int]int64),
		backendCounts: make(map[string]int64),
	}
}

func (m *Metrics) Record(backendURL string, statusCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalRequests++
	m.statusCodes[statusCode]++
	m.backendCounts[backendURL]++
}

func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		data := map[string]interface{}{
			"total_requests": m.totalRequests,
			"status_codes":   m.statusCodes,
			"backend_counts": m.backendCounts,
		}
		m.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}
}
