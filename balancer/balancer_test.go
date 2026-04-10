package balancer

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// rrStrategy is a test helper that wraps RoundRobin for testing
type rrStrategy struct {
	current uint64
}

func (rr *rrStrategy) Select(backends []*Backend, r *http.Request) (*Backend, error) {
	n := uint64(len(backends))
	for i := uint64(0); i < n; i++ {
		idx := rr.current % n
		rr.current++
		be := backends[idx]
		if be.IsHealthy() {
			return be, nil
		}
	}
	return nil, ErrNoHealthyBackends
}

func (rr *rrStrategy) Name() string {
	return "round-robin"
}

func TestNewBalancer(t *testing.T) {
	backends := []BackendWithWeight{
		{URL: "http://localhost:8081", Weight: 1},
		{URL: "http://localhost:8082", Weight: 1},
		{URL: "http://localhost:8083", Weight: 1},
	}
	b, err := NewBalancer(backends, &rrStrategy{})
	if err != nil {
		t.Fatalf("NewBalancer returned error: %v", err)
	}
	snap := b.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3 backends, got %d", len(snap))
	}
	for i, s := range snap {
		if s.URL != backends[i].URL {
			t.Errorf("backend %d URL = %s, want %s", i, s.URL, backends[i].URL)
		}
		if !s.Healthy {
			t.Errorf("backend %d should be healthy by default", i)
		}
	}
}

func TestNewBalancerEmpty(t *testing.T) {
	_, err := NewBalancer(nil, &rrStrategy{})
	if err == nil {
		t.Fatal("expected error for empty backends")
	}
}

func TestRoundRobin(t *testing.T) {
	backends := []BackendWithWeight{
		{URL: "http://localhost:8081", Weight: 1},
		{URL: "http://localhost:8082", Weight: 1},
		{URL: "http://localhost:8083", Weight: 1},
	}
	b, err := NewBalancer(backends, &rrStrategy{})
	if err != nil {
		t.Fatalf("NewBalancer returned error: %v", err)
	}

	counts := make(map[string]int)
	req := httptest.NewRequest("GET", "/", nil)
	for i := 0; i < 9; i++ {
		be, err := b.Next(req)
		if err != nil {
			t.Fatalf("Next returned error: %v", err)
		}
		counts[be.URL.String()]++
	}

	for _, bw := range backends {
		if counts[bw.URL] != 3 {
			t.Errorf("backend %s got %d requests, want 3", bw.URL, counts[bw.URL])
		}
	}
}

func TestSkipUnhealthy(t *testing.T) {
	backends := []BackendWithWeight{
		{URL: "http://localhost:8081", Weight: 1},
		{URL: "http://localhost:8082", Weight: 1},
		{URL: "http://localhost:8083", Weight: 1},
	}
	b, err := NewBalancer(backends, &rrStrategy{})
	if err != nil {
		t.Fatalf("NewBalancer returned error: %v", err)
	}

	b.SetAlive("http://localhost:8082", false)

	req := httptest.NewRequest("GET", "/", nil)
	for i := 0; i < 6; i++ {
		be, err := b.Next(req)
		if err != nil {
			t.Fatalf("Next returned error: %v", err)
		}
		if be.URL.String() == "http://localhost:8082" {
			t.Error("unhealthy backend should be skipped")
		}
	}
}

func TestAllUnhealthy(t *testing.T) {
	backends := []BackendWithWeight{
		{URL: "http://localhost:8081", Weight: 1},
		{URL: "http://localhost:8082", Weight: 1},
	}
	b, err := NewBalancer(backends, &rrStrategy{})
	if err != nil {
		t.Fatalf("NewBalancer returned error: %v", err)
	}

	b.SetAlive("http://localhost:8081", false)
	b.SetAlive("http://localhost:8082", false)

	req := httptest.NewRequest("GET", "/", nil)
	_, err = b.Next(req)
	if err == nil {
		t.Fatal("expected error when all backends are unhealthy")
	}
}

func TestSetAliveRecovery(t *testing.T) {
	backends := []BackendWithWeight{
		{URL: "http://localhost:8081", Weight: 1},
	}
	b, err := NewBalancer(backends, &rrStrategy{})
	if err != nil {
		t.Fatalf("NewBalancer returned error: %v", err)
	}

	b.SetAlive("http://localhost:8081", false)
	req := httptest.NewRequest("GET", "/", nil)
	_, err = b.Next(req)
	if err == nil {
		t.Error("expected error after marking backend unhealthy")
	}

	b.SetAlive("http://localhost:8081", true)
	be, err := b.Next(req)
	if err != nil {
		t.Fatalf("Next returned error after recovery: %v", err)
	}
	if be.URL.String() != "http://localhost:8081" {
		t.Errorf("expected http://localhost:8081, got %s", be.URL.String())
	}
}
