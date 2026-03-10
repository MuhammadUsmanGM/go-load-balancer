package balancer

import (
	"testing"
)

func TestNewBalancer(t *testing.T) {
	urls := []string{"http://localhost:8081", "http://localhost:8082", "http://localhost:8083"}
	b, err := NewBalancer(urls)
	if err != nil {
		t.Fatalf("NewBalancer returned error: %v", err)
	}
	backends := b.GetBackends()
	if len(backends) != 3 {
		t.Fatalf("expected 3 backends, got %d", len(backends))
	}
	for i, be := range backends {
		if be.URL.String() != urls[i] {
			t.Errorf("backend %d URL = %s, want %s", i, be.URL.String(), urls[i])
		}
		if !be.Healthy {
			t.Errorf("backend %d should be healthy by default", i)
		}
	}
}

func TestRoundRobin(t *testing.T) {
	urls := []string{"http://localhost:8081", "http://localhost:8082", "http://localhost:8083"}
	b, err := NewBalancer(urls)
	if err != nil {
		t.Fatalf("NewBalancer returned error: %v", err)
	}

	counts := make(map[string]int)
	total := 9
	for i := 0; i < total; i++ {
		be, err := b.Next()
		if err != nil {
			t.Fatalf("Next returned error: %v", err)
		}
		counts[be.URL.String()]++
	}

	for _, u := range urls {
		if counts[u] != 3 {
			t.Errorf("backend %s got %d requests, want 3", u, counts[u])
		}
	}
}

func TestSkipUnhealthy(t *testing.T) {
	urls := []string{"http://localhost:8081", "http://localhost:8082", "http://localhost:8083"}
	b, err := NewBalancer(urls)
	if err != nil {
		t.Fatalf("NewBalancer returned error: %v", err)
	}

	b.SetHealthy("http://localhost:8082", false)

	for i := 0; i < 6; i++ {
		be, err := b.Next()
		if err != nil {
			t.Fatalf("Next returned error: %v", err)
		}
		if be.URL.String() == "http://localhost:8082" {
			t.Error("unhealthy backend should be skipped")
		}
	}
}

func TestAllUnhealthy(t *testing.T) {
	urls := []string{"http://localhost:8081", "http://localhost:8082"}
	b, err := NewBalancer(urls)
	if err != nil {
		t.Fatalf("NewBalancer returned error: %v", err)
	}

	b.SetHealthy("http://localhost:8081", false)
	b.SetHealthy("http://localhost:8082", false)

	_, err = b.Next()
	if err == nil {
		t.Fatal("expected error when all backends are unhealthy")
	}
}

func TestSetHealthy(t *testing.T) {
	urls := []string{"http://localhost:8081"}
	b, err := NewBalancer(urls)
	if err != nil {
		t.Fatalf("NewBalancer returned error: %v", err)
	}

	// Initially healthy
	be, err := b.Next()
	if err != nil {
		t.Fatalf("Next returned error: %v", err)
	}
	if !be.Healthy {
		t.Error("backend should be healthy initially")
	}

	// Mark unhealthy
	b.SetHealthy("http://localhost:8081", false)
	_, err = b.Next()
	if err == nil {
		t.Error("expected error after marking backend unhealthy")
	}

	// Mark healthy again
	b.SetHealthy("http://localhost:8081", true)
	be, err = b.Next()
	if err != nil {
		t.Fatalf("Next returned error after re-enabling: %v", err)
	}
	if be.URL.String() != "http://localhost:8081" {
		t.Errorf("expected http://localhost:8081, got %s", be.URL.String())
	}
}
