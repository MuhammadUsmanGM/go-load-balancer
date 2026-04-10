package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds Prometheus metrics for the load balancer.
type Metrics struct {
	reg prometheus.Registerer

	// Total HTTP requests received.
	TotalRequests *prometheus.CounterVec

	// Request duration by backend and status code.
	RequestDuration *prometheus.HistogramVec

	// Active connections per backend.
	ActiveConnections *prometheus.GaugeVec

	// Backend health status (1 = healthy, 0 = unhealthy).
	BackendHealthy *prometheus.GaugeVec

	// Health check duration.
	HealthCheckDuration *prometheus.HistogramVec

	// Health check failures per backend.
	HealthCheckFailures *prometheus.CounterVec

	// Requests per second by backend.
	RequestsPerSecond *prometheus.CounterVec

	// Response size in bytes.
	ResponseSize *prometheus.HistogramVec

	// Current running goroutines.
	Goroutines prometheus.Gauge

	// Backend weight.
	BackendWeight *prometheus.GaugeVec

	// Backend slow start progress (0.0 to 1.0).
	BackendSlowStart *prometheus.GaugeVec

	// Consecutive health check failures.
	BackendConsecutiveFailures *prometheus.GaugeVec

	// Consecutive health check successes.
	BackendConsecutiveSuccesses *prometheus.GaugeVec
}

// NewMetrics creates and registers all Prometheus metrics.
func NewMetrics() *Metrics {
	return NewMetricsWithRegistry(prometheus.DefaultRegisterer)
}

// NewMetricsWithRegistry creates metrics with a custom Prometheus registry.
func NewMetricsWithRegistry(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		reg: reg,
		TotalRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "lb_requests_total",
				Help: "Total number of HTTP requests received by the load balancer.",
			},
			[]string{"backend", "status_code", "method"},
		),

		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "lb_request_duration_seconds",
				Help:    "HTTP request latency in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"backend", "status_code", "method"},
		),

		ActiveConnections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "lb_active_connections",
				Help: "Number of active connections per backend.",
			},
			[]string{"backend"},
		),

		BackendHealthy: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "lb_backend_healthy",
				Help: "Backend health status (1 = healthy, 0 = unhealthy).",
			},
			[]string{"backend"},
		),

		HealthCheckDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "lb_healthcheck_duration_seconds",
				Help:    "Health check request latency in seconds.",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
			},
			[]string{"backend"},
		),

		HealthCheckFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "lb_healthcheck_failures_total",
				Help: "Total number of health check failures per backend.",
			},
			[]string{"backend"},
		),

		RequestsPerSecond: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "lb_requests_per_second",
				Help: "Rate of requests per backend.",
			},
			[]string{"backend"},
		),

		ResponseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "lb_response_size_bytes",
				Help:    "Response size in bytes.",
				Buckets: []float64{100, 1000, 10000, 100000, 1e6, 1e7},
			},
			[]string{"backend", "status_code"},
		),

		Goroutines: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "lb_goroutines",
				Help: "Current number of goroutines.",
			},
		),

		BackendWeight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "lb_backend_weight",
				Help: "Backend weight for weighted round-robin.",
			},
			[]string{"backend"},
		),

		BackendSlowStart: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "lb_backend_slow_start_progress",
				Help: "Backend slow start progress (0.0 to 1.0).",
			},
			[]string{"backend"},
		),

		BackendConsecutiveFailures: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "lb_backend_consecutive_failures",
				Help: "Consecutive health check failures per backend.",
			},
			[]string{"backend"},
		),

		BackendConsecutiveSuccesses: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "lb_backend_consecutive_successes",
				Help: "Consecutive health check successes per backend.",
			},
			[]string{"backend"},
		),
	}

	// Register all metrics with the registry
	m.reg.MustRegister(m.TotalRequests)
	m.reg.MustRegister(m.RequestDuration)
	m.reg.MustRegister(m.ActiveConnections)
	m.reg.MustRegister(m.BackendHealthy)
	m.reg.MustRegister(m.HealthCheckDuration)
	m.reg.MustRegister(m.HealthCheckFailures)
	m.reg.MustRegister(m.RequestsPerSecond)
	m.reg.MustRegister(m.ResponseSize)
	m.reg.MustRegister(m.Goroutines)
	m.reg.MustRegister(m.BackendWeight)
	m.reg.MustRegister(m.BackendSlowStart)
	m.reg.MustRegister(m.BackendConsecutiveFailures)
	m.reg.MustRegister(m.BackendConsecutiveSuccesses)

	return m
}

// Record records a completed request with duration and status.
func (m *Metrics) Record(backend, statusCode, method string, durationSeconds float64, responseSize int64) {
	m.TotalRequests.WithLabelValues(backend, statusCode, method).Inc()
	m.RequestDuration.WithLabelValues(backend, statusCode, method).Observe(durationSeconds)
	m.RequestsPerSecond.WithLabelValues(backend).Inc()
	m.ResponseSize.WithLabelValues(backend, statusCode).Observe(float64(responseSize))
}

// SetActiveConnections updates the active connection count for a backend.
func (m *Metrics) SetActiveConnections(backend string, count uint64) {
	m.ActiveConnections.WithLabelValues(backend).Set(float64(count))
}

// SetBackendHealthy updates the health status of a backend.
func (m *Metrics) SetBackendHealthy(backend string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	m.BackendHealthy.WithLabelValues(backend).Set(value)
}

// RecordHealthCheckDuration records a health check duration.
func (m *Metrics) RecordHealthCheckDuration(backend string, durationSeconds float64) {
	m.HealthCheckDuration.WithLabelValues(backend).Observe(durationSeconds)
}

// RecordHealthCheckFailure records a health check failure.
func (m *Metrics) RecordHealthCheckFailure(backend string) {
	m.HealthCheckFailures.WithLabelValues(backend).Inc()
}

// SetBackendWeight updates the weight of a backend.
func (m *Metrics) SetBackendWeight(backend string, weight int) {
	m.BackendWeight.WithLabelValues(backend).Set(float64(weight))
}

// UpdateBackendHealthStats updates health check statistics for a backend.
func (m *Metrics) UpdateBackendHealthStats(backend string, slowStartProgress float64, consecutiveFailures, consecutiveSuccesses int64) {
	m.BackendSlowStart.WithLabelValues(backend).Set(slowStartProgress)
	m.BackendConsecutiveFailures.WithLabelValues(backend).Set(float64(consecutiveFailures))
	m.BackendConsecutiveSuccesses.WithLabelValues(backend).Set(float64(consecutiveSuccesses))
}

// UpdateGoroutines updates the goroutine count metric.
func (m *Metrics) UpdateGoroutines(count int) {
	m.Goroutines.Set(float64(count))
}

// Handler returns an HTTP handler that serves Prometheus metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}
