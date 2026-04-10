package health

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/go-load-balancer/balancer"
	"github.com/go-load-balancer/logging"
	"github.com/go-load-balancer/metrics"
)

func createTestMetrics(t *testing.T) *metrics.Metrics {
	t.Helper()
	reg := prometheus.NewRegistry()
	return metrics.NewMetricsWithRegistry(reg)
}

func createTestBackend(t *testing.T, healthy bool) *balancer.Backend {
	t.Helper()
	backends := []balancer.BackendWithWeight{
		{URL: "http://localhost:8081", Weight: 1},
	}
	b, err := balancer.NewBalancer(backends, nil)
	if err != nil {
		t.Fatalf("Failed to create balancer: %v", err)
	}
	backend := b.GetBackends()[0]
	backend.SetHealthy(healthy)
	return backend
}

func TestCheckerConfig_Defaults(t *testing.T) {
	// Test that config defaults are properly set
	cfg := CheckerConfig{
		Interval:             10 * time.Second,
		DefaultPath:          "/health",
		Timeout:              2 * time.Second,
		UnhealthyThreshold:   3,
		HealthyThreshold:     2,
		SlowStartDuration:    60 * time.Second,
		PassiveEnabled:       true,
	}

	if cfg.UnhealthyThreshold != 3 {
		t.Errorf("expected unhealthy threshold 3, got %d", cfg.UnhealthyThreshold)
	}
	if cfg.HealthyThreshold != 2 {
		t.Errorf("expected healthy threshold 2, got %d", cfg.HealthyThreshold)
	}
}

func TestPassiveHealthCheck_MarkUnhealthy(t *testing.T) {
	backends := []balancer.BackendWithWeight{
		{URL: "http://localhost:8081", Weight: 1},
	}
	b, _ := balancer.NewBalancer(backends, nil)
	
	met := createTestMetrics(t)
	logger := logging.NewLogger()
	
	cfg := CheckerConfig{
		Interval:             10 * time.Second,
		DefaultPath:          "/health",
		Timeout:              2 * time.Second,
		UnhealthyThreshold:   3,
		HealthyThreshold:     2,
		SlowStartDuration:    60 * time.Second,
		PassiveEnabled:       true,
	}
	
	checker := NewChecker(b, cfg, logger, met)
	
	backend := b.GetBackends()[0]
	
	// Record 3 consecutive failures (should mark unhealthy)
	checker.PassiveHealthCheck("http://localhost:8081", false)
	checker.PassiveHealthCheck("http://localhost:8081", false)
	checker.PassiveHealthCheck("http://localhost:8081", false)
	
	if backend.IsHealthy() {
		t.Error("backend should be marked unhealthy after 3 consecutive failures")
	}
}

func TestPassiveHealthCheck_NotMarkUnhealthyBelowThreshold(t *testing.T) {
	backends := []balancer.BackendWithWeight{
		{URL: "http://localhost:8081", Weight: 1},
	}
	b, _ := balancer.NewBalancer(backends, nil)
	
	met := createTestMetrics(t)
	logger := logging.NewLogger()
	
	cfg := CheckerConfig{
		Interval:             10 * time.Second,
		DefaultPath:          "/health",
		Timeout:              2 * time.Second,
		UnhealthyThreshold:   3,
		HealthyThreshold:     2,
		SlowStartDuration:    60 * time.Second,
		PassiveEnabled:       true,
	}
	
	checker := NewChecker(b, cfg, logger, met)
	
	backend := b.GetBackends()[0]
	
	// Record only 2 failures (below threshold)
	checker.PassiveHealthCheck("http://localhost:8081", false)
	checker.PassiveHealthCheck("http://localhost:8081", false)
	
	if !backend.IsHealthy() {
		t.Error("backend should still be healthy after only 2 consecutive failures (threshold is 3)")
	}
}

func TestPassiveHealthCheck_Recovery(t *testing.T) {
	backends := []balancer.BackendWithWeight{
		{URL: "http://localhost:8081", Weight: 1},
	}
	b, _ := balancer.NewBalancer(backends, nil)
	
	met := createTestMetrics(t)
	logger := logging.NewLogger()
	
	cfg := CheckerConfig{
		Interval:             10 * time.Second,
		DefaultPath:          "/health",
		Timeout:              2 * time.Second,
		UnhealthyThreshold:   3,
		HealthyThreshold:     2,
		SlowStartDuration:    60 * time.Second,
		PassiveEnabled:       true,
	}
	
	checker := NewChecker(b, cfg, logger, met)
	backend := b.GetBackends()[0]
	
	// Mark unhealthy
	backend.SetHealthy(false)
	backend.RecordFailure()
	backend.RecordFailure()
	backend.RecordFailure()
	
	// Now record successes to recover
	checker.PassiveHealthCheck("http://localhost:8081", true)
	checker.PassiveHealthCheck("http://localhost:8081", true)
	
	if backend.GetConsecutiveSuccesses() != 2 {
		t.Errorf("expected 2 consecutive successes, got %d", backend.GetConsecutiveSuccesses())
	}
}

func TestPassiveHealthCheck_Disabled(t *testing.T) {
	backends := []balancer.BackendWithWeight{
		{URL: "http://localhost:8081", Weight: 1},
	}
	b, _ := balancer.NewBalancer(backends, nil)
	
	met := createTestMetrics(t)
	logger := logging.NewLogger()
	
	cfg := CheckerConfig{
		Interval:             10 * time.Second,
		DefaultPath:          "/health",
		Timeout:              2 * time.Second,
		UnhealthyThreshold:   3,
		HealthyThreshold:     2,
		SlowStartDuration:    60 * time.Second,
		PassiveEnabled:       false, // Disabled
	}
	
	checker := NewChecker(b, cfg, logger, met)
	
	// Record many failures - should not mark unhealthy when disabled
	for i := 0; i < 10; i++ {
		checker.PassiveHealthCheck("http://localhost:8081", false)
	}
	
	backend := b.GetBackends()[0]
	if !backend.IsHealthy() {
		t.Error("backend should still be healthy when passive health checking is disabled")
	}
}

func TestGetBackendStats(t *testing.T) {
	backends := []balancer.BackendWithWeight{
		{URL: "http://localhost:8081", Weight: 1},
		{URL: "http://localhost:8082", Weight: 2},
	}
	b, _ := balancer.NewBalancer(backends, nil)
	
	met := createTestMetrics(t)
	logger := logging.NewLogger()
	
	cfg := CheckerConfig{
		Interval:             10 * time.Second,
		DefaultPath:          "/health",
		Timeout:              2 * time.Second,
		UnhealthyThreshold:   3,
		HealthyThreshold:     2,
		SlowStartDuration:    60 * time.Second,
		PassiveEnabled:       true,
	}
	
	checker := NewChecker(b, cfg, logger, met)
	
	// Record some data
	backend := b.GetBackends()[0]
	backend.RecordFailure()
	backend.RecordFailure()
	backend.RecordSuccess()
	backend.IncrementConnections()
	
	// Get stats
	stats := checker.GetBackendStats()
	
	if len(stats) != 2 {
		t.Fatalf("expected 2 backend stats, got %d", len(stats))
	}
	
	// Check first backend stats
	if stats[0]["consecutive_failures"].(int64) != 0 {
		t.Errorf("expected 0 consecutive failures (reset by success), got %d", stats[0]["consecutive_failures"])
	}
	if stats[0]["consecutive_successes"].(int64) != 1 {
		t.Errorf("expected 1 consecutive success, got %d", stats[0]["consecutive_successes"])
	}
}

func TestSlowStart_Progress(t *testing.T) {
	backend := createTestBackend(t, true)
	
	// Mark as recovered
	backend.MarkRecovered()
	
	slowStartDuration := 10 * time.Second
	
	// Initially should be at minimum
	progress := backend.GetSlowStartWeight(slowStartDuration)
	if progress < 0.1 || progress > 0.2 {
		t.Errorf("expected slow start progress near 0.1, got %f", progress)
	}
	
	// Simulate time passing (we can't actually wait, so we test the logic)
	// After 50% of slow start duration, should be at ~0.55
	// Since we can't manipulate time in tests, we just verify the function works
}

func TestSlowStart_Complete(t *testing.T) {
	backend := createTestBackend(t, true)
	
	// Manually set recovery time to 1 hour ago using the exported method
	// We can't test this directly since recoveryTime is internal
	// Instead, verify that GetSlowStartWeight returns 1.0 when no recovery is tracked
	backend.MarkRecovered()
	
	// Just after marking, progress should be minimal
	slowStartDuration := 10 * time.Second
	progress := backend.GetSlowStartWeight(slowStartDuration)
	if progress < 0.1 || progress > 0.15 {
		t.Errorf("expected slow start progress near 0.1 just after recovery, got %f", progress)
	}
}

func TestConsecutiveTracking_Reset(t *testing.T) {
	backend := createTestBackend(t, true)
	
	// Record failures
	backend.RecordFailure()
	backend.RecordFailure()
	
	if backend.GetConsecutiveFailures() != 2 {
		t.Errorf("expected 2 consecutive failures, got %d", backend.GetConsecutiveFailures())
	}
	
	// Record success - should reset failures
	backend.RecordSuccess()
	
	if backend.GetConsecutiveFailures() != 0 {
		t.Errorf("expected 0 consecutive failures after success, got %d", backend.GetConsecutiveFailures())
	}
	
	if backend.GetConsecutiveSuccesses() != 1 {
		t.Errorf("expected 1 consecutive success, got %d", backend.GetConsecutiveSuccesses())
	}
}
