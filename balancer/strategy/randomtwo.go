package strategy

import (
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/go-load-balancer/balancer"
)

// RandomTwoChoices picks two random backends and selects the one with fewer connections.
type RandomTwoChoices struct {
	mu  sync.Mutex
	rng *rand.Rand
}

// NewRandomTwoChoices creates a new random with 2 choices strategy.
func NewRandomTwoChoices() *RandomTwoChoices {
	return &RandomTwoChoices{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Select returns the healthier backend with fewer connections from two random choices.
func (rtc *RandomTwoChoices) Select(backends []*balancer.Backend, r *http.Request) (*balancer.Backend, error) {
	n := len(backends)
	if n == 0 {
		return nil, balancer.ErrNoHealthyBackends
	}

	// If only one backend, just check if it's healthy
	if n == 1 {
		if backends[0].IsHealthy() {
			return backends[0], nil
		}
		return nil, balancer.ErrNoHealthyBackends
	}

	rtc.mu.Lock()
	defer rtc.mu.Unlock()

	// Pick two random backends
	var best *balancer.Backend
	minConns := uint64(^uint64(0))

	// Try up to 2 times to find healthy backends
	for attempts := 0; attempts < 2; attempts++ {
		idx := rtc.rng.Intn(n)
		be := backends[idx]

		if !be.IsHealthy() {
			continue
		}

		conns := be.ActiveConnections()
		if conns < minConns {
			minConns = conns
			best = be
		}
	}

	if best == nil {
		// Fallback: try all backends to find any healthy one
		for _, be := range backends {
			if be.IsHealthy() {
				return be, nil
			}
		}
		return nil, balancer.ErrNoHealthyBackends
	}

	return best, nil
}

// Name returns the strategy name.
func (rtc *RandomTwoChoices) Name() string {
	return "random-two-choices"
}
