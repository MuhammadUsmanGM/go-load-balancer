package balancer

import (
	"errors"
	"net/url"
	"sync"
	"sync/atomic"
)

// Backend represents a single backend server.
type Backend struct {
	URL     *url.URL
	healthy atomic.Bool
}

// IsHealthy returns the backend's current health status.
func (b *Backend) IsHealthy() bool {
	return b.healthy.Load()
}

// SetHealthy updates the backend's health status.
func (b *Backend) SetHealthy(v bool) {
	b.healthy.Store(v)
}

// Balancer distributes requests across healthy backends using round-robin.
type Balancer struct {
	backends []*Backend
	current  atomic.Uint64
	mu       sync.RWMutex
}

// NewBalancer creates a balancer from a list of backend URLs.
func NewBalancer(urls []string) (*Balancer, error) {
	if len(urls) == 0 {
		return nil, errors.New("no backends provided")
	}
	backends := make([]*Backend, len(urls))
	for i, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, err
		}
		be := &Backend{URL: u}
		be.healthy.Store(true)
		backends[i] = be
	}
	return &Balancer{backends: backends}, nil
}

// Next returns the next healthy backend using round-robin selection.
func (b *Balancer) Next() (*Backend, error) {
	n := uint64(len(b.backends))
	for i := uint64(0); i < n; i++ {
		idx := b.current.Add(1) - 1
		be := b.backends[idx%n]
		if be.IsHealthy() {
			return be, nil
		}
	}
	return nil, errors.New("no healthy backends available")
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
