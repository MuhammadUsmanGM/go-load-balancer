package balancer

import (
	"errors"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// ErrNoHealthyBackends is returned when no healthy backends are available.
var ErrNoHealthyBackends = errors.New("no healthy backends available")

// Backend represents a single backend server.
type Backend struct {
	URL     *url.URL
	healthy atomic.Bool
	weight  int

	// Active connections tracking for least-connections strategy.
	activeConns atomic.Uint64

	// Current weight for weighted round-robin (internal state).
	currentWeight atomic.Int64

	// Advanced health check tracking
	consecutiveFailures atomic.Int64
	consecutiveSuccesses atomic.Int64
	recoveryTime        atomic.Int64 // timestamp when backend recovered (for slow start)
	totalRequests       atomic.Int64
	totalFailures       atomic.Int64
}

// IsHealthy returns the backend's current health status.
func (b *Backend) IsHealthy() bool {
	return b.healthy.Load()
}

// SetHealthy updates the backend's health status.
func (b *Backend) SetHealthy(v bool) {
	b.healthy.Store(v)
}

// Weight returns the backend's configured weight.
func (b *Backend) Weight() int {
	return b.weight
}

// SetWeight updates the backend's weight.
func (b *Backend) SetWeight(w int) {
	b.weight = w
}

// ActiveConnections returns the number of active connections.
func (b *Backend) ActiveConnections() uint64 {
	return b.activeConns.Load()
}

// IncrementConnections atomically increases the active connection count.
func (b *Backend) IncrementConnections() {
	b.activeConns.Add(1)
}

// DecrementConnections atomically decreases the active connection count.
func (b *Backend) DecrementConnections() {
	b.activeConns.Add(^uint64(0)) // atomic subtract 1 using wrap-around
}

// CurrentWeight returns the backend's current weight for WRR algorithm.
func (b *Backend) CurrentWeight() int64 {
	return b.currentWeight.Load()
}

// IncreaseCurrentWeight increases the backend's current weight.
func (b *Backend) IncreaseCurrentWeight(w int) {
	b.currentWeight.Add(int64(w))
}

// DecreaseCurrentWeight decreases the backend's current weight.
func (b *Backend) DecreaseCurrentWeight(totalWeight int) {
	b.currentWeight.Add(-int64(totalWeight))
}

// RecordFailure records a proxy error for passive health checking.
func (b *Backend) RecordFailure() {
	b.consecutiveFailures.Add(1)
	b.consecutiveSuccesses.Store(0)
	b.totalFailures.Add(1)
}

// RecordSuccess records a successful proxy response.
func (b *Backend) RecordSuccess() {
	b.consecutiveSuccesses.Add(1)
	b.consecutiveFailures.Store(0)
	b.totalRequests.Add(1)
}

// GetConsecutiveFailures returns the consecutive failure count.
func (b *Backend) GetConsecutiveFailures() int64 {
	return b.consecutiveFailures.Load()
}

// GetConsecutiveSuccesses returns the consecutive success count.
func (b *Backend) GetConsecutiveSuccesses() int64 {
	return b.consecutiveSuccesses.Load()
}

// MarkRecovered sets the backend as recovered and records the timestamp.
func (b *Backend) MarkRecovered() {
	b.recoveryTime.Store(time.Now().UnixMilli())
}

// GetRecoveryTime returns when the backend recovered.
func (b *Backend) GetRecoveryTime() int64 {
	return b.recoveryTime.Load()
}

// GetSlowStartWeight returns a weight multiplier based on slow start progress.
// Returns 0.0 to 1.0 based on how far into the slow start period we are.
func (b *Backend) GetSlowStartWeight(slowStartDuration time.Duration) float64 {
	recoveryTime := b.GetRecoveryTime()
	if recoveryTime == 0 {
		return 1.0 // No slow start active
	}

	elapsed := time.Since(time.UnixMilli(recoveryTime))
	if elapsed >= slowStartDuration {
		return 1.0 // Slow start complete
	}

	// Linear ramp from 0.1 to 1.0
	progress := float64(elapsed) / float64(slowStartDuration)
	return 0.1 + (0.9 * progress)
}

// IncrementRequests tracks total requests for this backend.
func (b *Backend) IncrementRequests() {
	b.totalRequests.Add(1)
}

// GetStats returns backend health statistics.
func (b *Backend) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"healthy":                b.IsHealthy(),
		"consecutive_failures":   b.consecutiveFailures.Load(),
		"consecutive_successes":  b.consecutiveSuccesses.Load(),
		"total_requests":         b.totalRequests.Load(),
		"total_failures":         b.totalFailures.Load(),
		"active_connections":     b.activeConns.Load(),
		"recovery_time_ms":       b.recoveryTime.Load(),
	}
}

// Balancer distributes requests across healthy backends using a configurable strategy.
type Balancer struct {
	backends []*Backend
	strategy Strategy
	mu       sync.RWMutex
}

// BackendWithWeight represents a backend URL with its weight.
type BackendWithWeight struct {
	URL    string
	Weight int
}

// NewBalancer creates a balancer from backend URLs with weights and a strategy.
func NewBalancer(backends []BackendWithWeight, strat Strategy) (*Balancer, error) {
	if len(backends) == 0 {
		return nil, errors.New("no backends provided")
	}

	backendList := make([]*Backend, len(backends))
	for i, bw := range backends {
		u, err := url.Parse(bw.URL)
		if err != nil {
			return nil, err
		}
		be := &Backend{
			URL:    u,
			weight: bw.Weight,
		}
		if bw.Weight == 0 {
			be.weight = 1 // Default weight
		}
		be.healthy.Store(true)
		backendList[i] = be
	}

	return &Balancer{
		backends: backendList,
		strategy: strat,
	}, nil
}

// NewBalancerSimple creates a balancer from backend URLs with equal weights.
func NewBalancerSimple(urls []string) (*Balancer, error) {
	backends := make([]BackendWithWeight, len(urls))
	for i, u := range urls {
		backends[i] = BackendWithWeight{URL: u, Weight: 1}
	}
	return NewBalancer(backends, nil)
}

// Next returns the next healthy backend using the configured strategy.
func (b *Balancer) Next(r *http.Request) (*Backend, error) {
	if b.strategy == nil {
		return nil, errors.New("no strategy configured")
	}
	return b.strategy.Select(b.backends, r)
}

// SetStrategy updates the load balancing strategy.
func (b *Balancer) SetStrategy(strat Strategy) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.strategy = strat
}

// GetStrategy returns the current strategy name.
func (b *Balancer) GetStrategy() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.strategy == nil {
		return "none"
	}
	return b.strategy.Name()
}

// GetBackends returns all backends for health checker iteration.
func (b *Balancer) GetBackends() []*Backend {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.backends
}

// SetAlive updates the health status of the backend matching the given URL.
func (b *Balancer) SetAlive(rawURL string, alive bool) {
	for _, be := range b.backends {
		if be.URL.String() == rawURL {
			be.SetHealthy(alive)
			return
		}
	}
}

// Snapshot returns a copy of backend states for safe reading.
type BackendStatus struct {
	URL     string `json:"url"`
	Healthy bool   `json:"healthy"`
}

func (b *Balancer) Snapshot() []BackendStatus {
	out := make([]BackendStatus, len(b.backends))
	for i, be := range b.backends {
		out[i] = BackendStatus{
			URL:     be.URL.String(),
			Healthy: be.IsHealthy(),
		}
	}
	return out
}
