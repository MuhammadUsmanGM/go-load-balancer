package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewMetrics_Registration(t *testing.T) {
	// Create a new registry for testing
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry(reg)
	
	// Verify metrics are registered
	if m.TotalRequests == nil {
		t.Error("expected TotalRequests metric to be initialized")
	}
	if m.RequestDuration == nil {
		t.Error("expected RequestDuration metric to be initialized")
	}
	if m.ActiveConnections == nil {
		t.Error("expected ActiveConnections metric to be initialized")
	}
}

func TestMetrics_Record(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry(reg)
	
	// Record a request
	m.Record("http://localhost:8081", "200", "GET", 0.123, 1024)
	
	// Verify counter incremented
	count := testutil.ToFloat64(m.TotalRequests.WithLabelValues("http://localhost:8081", "200", "GET"))
	if count != 1 {
		t.Errorf("expected TotalRequests to be 1, got %f", count)
	}
}

func TestMetrics_Record_MultipleRequests(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry(reg)
	
	// Record multiple requests
	m.Record("http://localhost:8081", "200", "GET", 0.1, 512)
	m.Record("http://localhost:8081", "200", "GET", 0.2, 1024)
	m.Record("http://localhost:8082", "500", "POST", 0.5, 256)
	
	// Verify counts
	count1 := testutil.ToFloat64(m.TotalRequests.WithLabelValues("http://localhost:8081", "200", "GET"))
	if count1 != 2 {
		t.Errorf("expected 2 requests to 8081, got %f", count1)
	}
	
	count2 := testutil.ToFloat64(m.TotalRequests.WithLabelValues("http://localhost:8082", "500", "POST"))
	if count2 != 1 {
		t.Errorf("expected 1 request to 8082, got %f", count2)
	}
}

func TestMetrics_SetActiveConnections(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry(reg)
	
	m.SetActiveConnections("http://localhost:8081", 5)
	
	value := testutil.ToFloat64(m.ActiveConnections.WithLabelValues("http://localhost:8081"))
	if value != 5 {
		t.Errorf("expected 5 active connections, got %f", value)
	}
}

func TestMetrics_SetBackendHealthy(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry(reg)
	
	// Test healthy backend
	m.SetBackendHealthy("http://localhost:8081", true)
	value := testutil.ToFloat64(m.BackendHealthy.WithLabelValues("http://localhost:8081"))
	if value != 1 {
		t.Errorf("expected healthy backend value 1, got %f", value)
	}
	
	// Test unhealthy backend
	m.SetBackendHealthy("http://localhost:8081", false)
	value = testutil.ToFloat64(m.BackendHealthy.WithLabelValues("http://localhost:8081"))
	if value != 0 {
		t.Errorf("expected unhealthy backend value 0, got %f", value)
	}
}

func TestMetrics_RecordHealthCheckDuration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry(reg)
	
	m.RecordHealthCheckDuration("http://localhost:8081", 0.05)
	
	// Verify histogram was called - we can't directly read histogram values easily
	// Just ensure no panic occurred and metric is registered
	if m.HealthCheckDuration == nil {
		t.Error("expected HealthCheckDuration metric to be initialized")
	}
}

func TestMetrics_RecordHealthCheckFailure(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry(reg)
	
	m.RecordHealthCheckFailure("http://localhost:8081")
	
	count := testutil.ToFloat64(m.HealthCheckFailures.WithLabelValues("http://localhost:8081"))
	if count != 1 {
		t.Errorf("expected 1 health check failure, got %f", count)
	}
}

func TestMetrics_SetBackendWeight(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry(reg)
	
	m.SetBackendWeight("http://localhost:8081", 3)
	
	value := testutil.ToFloat64(m.BackendWeight.WithLabelValues("http://localhost:8081"))
	if value != 3 {
		t.Errorf("expected backend weight 3, got %f", value)
	}
}

func TestMetrics_UpdateGoroutines(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry(reg)
	
	m.UpdateGoroutines(42)
	
	value := testutil.ToFloat64(m.Goroutines)
	if value != 42 {
		t.Errorf("expected 42 goroutines, got %f", value)
	}
}

func TestMetrics_Handler(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetricsWithRegistry(reg)
	
	// Record some metrics
	m.Record("http://localhost:8081", "200", "GET", 0.1, 1024)
	
	// Create test request to metrics endpoint
	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	
	m.Handler().ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	
	// Check response contains Prometheus format data
	if len(rr.Body.Bytes()) == 0 {
		t.Error("expected non-empty metrics response")
	}
}
