package strategy

import (
	"net/http"

	"github.com/go-load-balancer/balancer"
)

// WeightedRoundRobin implements weighted round-robin load balancing.
// Backends with higher weights receive proportionally more requests.
type WeightedRoundRobin struct {
	currentWeight int
	currentIndex  int
}

// NewWeightedRoundRobin creates a new weighted round-robin strategy.
func NewWeightedRoundRobin() *WeightedRoundRobin {
	return &WeightedRoundRobin{}
}

// Select returns the next healthy backend using weighted round-robin.
// Uses the smooth weighted round-robin algorithm.
func (wrr *WeightedRoundRobin) Select(backends []*balancer.Backend, r *http.Request) (*balancer.Backend, error) {
	if len(backends) == 0 {
		return nil, balancer.ErrNoHealthyBackends
	}

	totalWeight := 0
	for _, be := range backends {
		totalWeight += be.Weight()
	}

	if totalWeight == 0 {
		// Fallback to simple round-robin if no weights
		return wrr.selectUnweighted(backends)
	}

	// Smooth weighted round-robin
	var selected *balancer.Backend
	maxWeight := int64(-1)

	for _, be := range backends {
		if !be.IsHealthy() {
			continue
		}
		be.IncreaseCurrentWeight(be.Weight())
		if be.CurrentWeight() > maxWeight {
			maxWeight = be.CurrentWeight()
			selected = be
		}
	}

	if selected == nil {
		return nil, balancer.ErrNoHealthyBackends
	}

	selected.DecreaseCurrentWeight(totalWeight)
	return selected, nil
}

// selectUnweighted falls back to simple round-robin.
func (wrr *WeightedRoundRobin) selectUnweighted(backends []*balancer.Backend) (*balancer.Backend, error) {
	n := uint64(len(backends))
	for i := uint64(0); i < n; i++ {
		idx := wrr.currentIndex % int(n)
		wrr.currentIndex++
		be := backends[idx]
		if be.IsHealthy() {
			return be, nil
		}
	}
	return nil, balancer.ErrNoHealthyBackends
}

// Name returns the strategy name.
func (wrr *WeightedRoundRobin) Name() string {
	return "weighted-round-robin"
}
