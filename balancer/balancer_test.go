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
	snap := b.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3 backends, got %d", len(snap))
	}
	for i, s := range snap {
		if s.URL != urls[i] {
			t.Errorf("backend %d URL = %s, want %s", i, s.URL, urls[i])
		}
		if !s.Healthy {
			t.Errorf("backend %d should be healthy by default", i)
		}
	}
}

func TestNewBalancerEmpty(t *testing.T) {
	_, err := NewBalancer(nil)
	if err == nil {
		t.Fatal("expected error for empty backends")
	}
}

func TestRoundRobin(t *testing.T) {
	urls := []string{"http://localhost:8081", "http://localhost:8082", "http://localhost:8083"}
	b, err := NewBalancer(urls)
	if err != nil {
		t.Fatalf("NewBalancer returned error: %v", err)
	}

	counts := make(map[string]int)
	for i := 0; i < 9; i++ {
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

	b.SetAlive("http://localhost:8082", false)

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

	b.SetAlive("http://localhost:8081", false)
	b.SetAlive("http://localhost:8082", false)

	_, err = b.Next()
	if err == nil {
		t.Fatal("expected error when all backends are unhealthy")
	}
}

func TestSetAliveRecovery(t *testing.T) {
	urls := []string{"http://localhost:8081"}
	b, err := NewBalancer(urls)
	if err != nil {
		t.Fatalf("NewBalancer returned error: %v", err)
	}

	b.SetAlive("http://localhost:8081", false)
	_, err = b.Next()
	if err == nil {
		t.Error("expected error after marking backend unhealthy")
	}

	b.SetAlive("http://localhost:8081", true)
	be, err := b.Next()
	if err != nil {
		t.Fatalf("Next returned error after recovery: %v", err)
	}
	if be.URL.String() != "http://localhost:8081" {
		t.Errorf("expected http://localhost:8081, got %s", be.URL.String())
	}
}
