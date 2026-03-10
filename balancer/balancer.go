package balancer

import (
	"errors"
	"net/url"
	"sync"
)

type Backend struct {
	URL     *url.URL
	Healthy bool
}

type Balancer struct {
	mu       sync.Mutex
	backends []*Backend
	current  uint64
}

func NewBalancer(urls []string) (*Balancer, error) {
	backends := make([]*Backend, 0, len(urls))
	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, err
		}
		backends = append(backends, &Backend{URL: u, Healthy: true})
	}
	return &Balancer{backends: backends}, nil
}

func (b *Balancer) Next() (*Backend, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	n := len(b.backends)
	for i := 0; i < n; i++ {
		idx := b.current % uint64(n)
		b.current++
		if b.backends[idx].Healthy {
			return b.backends[idx], nil
		}
	}
	return nil, errors.New("no healthy backends available")
}

func (b *Balancer) SetHealthy(rawURL string, healthy bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, be := range b.backends {
		if be.URL.String() == rawURL {
			be.Healthy = healthy
			return
		}
	}
}

func (b *Balancer) GetBackends() []*Backend {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]*Backend, len(b.backends))
	copy(out, b.backends)
	return out
}
