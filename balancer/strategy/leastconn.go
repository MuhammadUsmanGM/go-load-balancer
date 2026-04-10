package strategy

import (
	"net/http"

	"github.com/go-load-balancer/balancer"
)

// LeastConnections routes requests to the backend with fewest active connections.
type LeastConnections struct{}

// NewLeastConnections creates a new least-connections strategy.
func NewLeastConnections() *LeastConnections {
	return &LeastConnections{}
}

// Select returns the healthy backend with the fewest active connections.
func (lc *LeastConnections) Select(backends []*balancer.Backend, r *http.Request) (*balancer.Backend, error) {
	var selected *balancer.Backend
	minConns := uint64(^uint64(0)) // max uint64

	for _, be := range backends {
		if !be.IsHealthy() {
			continue
		}
		conns := be.ActiveConnections()
		if conns < minConns {
			minConns = conns
			selected = be
		}
	}

	if selected == nil {
		return nil, balancer.ErrNoHealthyBackends
	}
	return selected, nil
}

// Name returns the strategy name.
func (lc *LeastConnections) Name() string {
	return "least-connections"
}
