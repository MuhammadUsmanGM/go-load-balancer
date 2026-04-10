package strategy

import (
	"net/http/httptest"
	"testing"

	"github.com/go-load-balancer/balancer"
)

func createTestBackends(weights []int) []*balancer.Backend {
	urls := []string{
		"http://localhost:8081",
		"http://localhost:8082",
		"http://localhost:8083",
	}
	// Only create as many backends as the weights slice specifies
	count := len(weights)
	backends := make([]*balancer.Backend, count)
	for i := 0; i < count; i++ {
		backends[i] = createBackend(urls[i], weights[i])
	}
	return backends
}

func createBackend(rawURL string, weight int) *balancer.Backend {
	// Create a minimal backend via the balancer
	b, _ := balancer.NewBalancer([]balancer.BackendWithWeight{
		{URL: rawURL, Weight: weight},
	}, nil)
	return b.GetBackends()[0]
}

func TestRoundRobin_Selection(t *testing.T) {
	backends := createTestBackends([]int{1, 1, 1})
	strat := NewRoundRobin()

	counts := make(map[string]int)
	req := httptest.NewRequest("GET", "/", nil)

	for i := 0; i < 9; i++ {
		be, err := strat.Select(backends, req)
		if err != nil {
			t.Fatalf("Select returned error: %v", err)
		}
		counts[be.URL.String()]++
	}

	// Each backend should get exactly 3 requests
	for _, be := range backends {
		if counts[be.URL.String()] != 3 {
			t.Errorf("backend %s got %d requests, want 3", be.URL.String(), counts[be.URL.String()])
		}
	}
}

func TestRoundRobin_SkipUnhealthy(t *testing.T) {
	backends := createTestBackends([]int{1, 1, 1})
	backends[1].SetHealthy(false)
	strat := NewRoundRobin()

	req := httptest.NewRequest("GET", "/", nil)
	for i := 0; i < 6; i++ {
		be, err := strat.Select(backends, req)
		if err != nil {
			t.Fatalf("Select returned error: %v", err)
		}
		if be.URL.String() == "http://localhost:8082" {
			t.Error("unhealthy backend should be skipped")
		}
	}
}

func TestRoundRobin_AllUnhealthy(t *testing.T) {
	backends := createTestBackends([]int{1, 1})
	backends[0].SetHealthy(false)
	backends[1].SetHealthy(false)
	
	// Debug: verify backends are actually unhealthy
	if backends[0].IsHealthy() {
		t.Log("WARNING: backend 0 is still healthy")
	}
	if backends[1].IsHealthy() {
		t.Log("WARNING: backend 1 is still healthy")
	}
	
	strat := NewRoundRobin()

	req := httptest.NewRequest("GET", "/", nil)
	be, err := strat.Select(backends, req)
	if err == nil {
		t.Fatalf("expected error when all backends are unhealthy, got backend: %s", be.URL.String())
	}
}

func TestLeastConnections_Selection(t *testing.T) {
	backends := createTestBackends([]int{1, 1, 1})
	strat := NewLeastConnections()

	req := httptest.NewRequest("GET", "/", nil)

	// Add some connections to first backend
	backends[0].IncrementConnections()
	backends[0].IncrementConnections()

	// Should select backend with least connections
	be, err := strat.Select(backends, req)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	// Should pick either backend 1 or 2 (both have 0 connections)
	if be.URL.String() == "http://localhost:8081" {
		t.Error("expected backend with least connections, got backend with 2 connections")
	}
}

func TestLeastConnections_SkipUnhealthy(t *testing.T) {
	backends := createTestBackends([]int{1, 1, 1})
	backends[0].SetHealthy(false)
	backends[1].SetHealthy(false)
	backends[2].IncrementConnections()
	strat := NewLeastConnections()

	req := httptest.NewRequest("GET", "/", nil)
	be, err := strat.Select(backends, req)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	if be.URL.String() != "http://localhost:8083" {
		t.Errorf("expected http://localhost:8083, got %s", be.URL.String())
	}
}

func TestWeightedRoundRobin_Selection(t *testing.T) {
	// Backend 0: weight 1, Backend 1: weight 2, Backend 2: weight 1
	backends := createTestBackends([]int{1, 2, 1})
	strat := NewWeightedRoundRobin()

	counts := make(map[string]int)
	req := httptest.NewRequest("GET", "/", nil)

	// 4 requests should distribute as 1:2:1
	for i := 0; i < 4; i++ {
		be, err := strat.Select(backends, req)
		if err != nil {
			t.Fatalf("Select returned error: %v", err)
		}
		counts[be.URL.String()]++
	}

	// Backend 1 (weight 2) should get 2 requests, others 1 each
	if counts["http://localhost:8081"] != 1 {
		t.Errorf("backend 8081 got %d requests, want 1", counts["http://localhost:8081"])
	}
	if counts["http://localhost:8082"] != 2 {
		t.Errorf("backend 8082 got %d requests, want 2", counts["http://localhost:8082"])
	}
	if counts["http://localhost:8083"] != 1 {
		t.Errorf("backend 8083 got %d requests, want 1", counts["http://localhost:8083"])
	}
}

func TestWeightedRoundRobin_SkipUnhealthy(t *testing.T) {
	backends := createTestBackends([]int{1, 2, 1})
	backends[1].SetHealthy(false) // Disable the highest weight
	strat := NewWeightedRoundRobin()

	req := httptest.NewRequest("GET", "/", nil)
	// With weights 1, 2, 1 and backend 1 (weight 2) unhealthy, we have backends 0 and 2 with weight 1 each
	// They should alternate or distribute evenly
	for i := 0; i < 2; i++ { // Only 2 requests since we have 2 healthy backends
		be, err := strat.Select(backends, req)
		if err != nil {
			t.Fatalf("Select returned error: %v", err)
		}
		if be.URL.String() == "http://localhost:8082" {
			t.Error("unhealthy backend should be skipped")
		}
	}
}

func TestIPHash_SameIP(t *testing.T) {
	backends := createTestBackends([]int{1, 1, 1})
	strat := NewIPHash()

	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "192.168.1.1:12345"

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "192.168.1.1:12345"

	be1, err := strat.Select(backends, req1)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	be2, err := strat.Select(backends, req2)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	if be1.URL.String() != be2.URL.String() {
		t.Errorf("same IP should get same backend: %s vs %s", be1.URL.String(), be2.URL.String())
	}
}

func TestIPHash_DifferentIP(t *testing.T) {
	backends := createTestBackends([]int{1, 1, 1})
	strat := NewIPHash()

	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "192.168.1.1:12345"

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "192.168.1.2:12345"

	be1, err := strat.Select(backends, req1)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	be2, err := strat.Select(backends, req2)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	// Different IPs *usually* get different backends (not guaranteed with only 3 backends)
	// Just verify both requests succeeded
	if be1 == nil || be2 == nil {
		t.Error("expected valid backends")
	}
}

func TestIPHash_WithForwardedFor(t *testing.T) {
	backends := createTestBackends([]int{1, 1, 1})
	strat := NewIPHash()

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	req.RemoteAddr = "192.168.1.1:12345"

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("X-Forwarded-For", "10.0.0.1")
	req2.RemoteAddr = "192.168.1.2:12345"

	be1, err := strat.Select(backends, req)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	be2, err := strat.Select(backends, req2)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	if be1.URL.String() != be2.URL.String() {
		t.Errorf("same X-Forwarded-For IP should get same backend: %s vs %s", be1.URL.String(), be2.URL.String())
	}
}

func TestIPHash_SkipUnhealthy(t *testing.T) {
	backends := createTestBackends([]int{1, 1, 1})
	strat := NewIPHash()

	// Mark first backend as unhealthy
	backends[0].SetHealthy(false)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	be, err := strat.Select(backends, req)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	if !be.IsHealthy() {
		t.Error("should select healthy backend")
	}
}

func TestRandomTwoChoices_Selection(t *testing.T) {
	backends := createTestBackends([]int{1, 1, 1})
	strat := NewRandomTwoChoices()

	req := httptest.NewRequest("GET", "/", nil)

	// Run multiple times to ensure it works
	for i := 0; i < 10; i++ {
		be, err := strat.Select(backends, req)
		if err != nil {
			t.Fatalf("Select returned error: %v", err)
		}
		if !be.IsHealthy() {
			t.Error("selected backend should be healthy")
		}
	}
}

func TestRandomTwoChoices_WithConnections(t *testing.T) {
	backends := createTestBackends([]int{1, 1, 1})
	strat := NewRandomTwoChoices()

	// Add connections to first two backends
	backends[0].IncrementConnections()
	backends[0].IncrementConnections()
	backends[0].IncrementConnections()
	backends[1].IncrementConnections()
	backends[1].IncrementConnections()

	// Backend 2 has 0 connections, should be preferred when selected
	req := httptest.NewRequest("GET", "/", nil)
	be, err := strat.Select(backends, req)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	// Just verify a healthy backend was selected
	if !be.IsHealthy() {
		t.Error("selected backend should be healthy")
	}
}

func TestRandomTwoChoices_SingleBackend(t *testing.T) {
	backends := createTestBackends([]int{1})[:1]
	strat := NewRandomTwoChoices()

	req := httptest.NewRequest("GET", "/", nil)
	be, err := strat.Select(backends, req)
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}

	if be.URL.String() != "http://localhost:8081" {
		t.Errorf("expected http://localhost:8081, got %s", be.URL.String())
	}
}

func TestRandomTwoChoices_AllUnhealthy(t *testing.T) {
	backends := createTestBackends([]int{1, 1})
	backends[0].SetHealthy(false)
	backends[1].SetHealthy(false)
	strat := NewRandomTwoChoices()

	req := httptest.NewRequest("GET", "/", nil)
	_, err := strat.Select(backends, req)
	if err == nil {
		t.Fatal("expected error when all backends are unhealthy")
	}
}
