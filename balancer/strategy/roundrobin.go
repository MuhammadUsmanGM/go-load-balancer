package strategy

import (
	"net/http"

	"github.com/go-load-balancer/balancer"
)

// RoundRobin implements round-robin load balancing.
type RoundRobin struct {
	current uint64
}

// NewRoundRobin creates a new round-robin strategy.
func NewRoundRobin() *RoundRobin {
	return &RoundRobin{}
}

// Select returns the next healthy backend using round-robin.
func (rr *RoundRobin) Select(backends []*balancer.Backend, r *http.Request) (*balancer.Backend, error) {
	n := uint64(len(backends))
	for i := uint64(0); i < n; i++ {
		idx := rr.current % n
		rr.current++
		be := backends[idx]
		if be.IsHealthy() {
			return be, nil
		}
	}
	return nil, balancer.ErrNoHealthyBackends
}

// Name returns the strategy name.
func (rr *RoundRobin) Name() string {
	return "round-robin"
}
