package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-load-balancer/balancer"
	"github.com/go-load-balancer/logging"
	"github.com/go-load-balancer/metrics"
)

// Checker periodically probes backend health endpoints.
type Checker struct {
	balancer             *balancer.Balancer
	interval             time.Duration
	defaultPath          string
	timeout              time.Duration
	client               *http.Client
	logger               *logging.Logger
	metrics              *metrics.Metrics
	unhealthyThreshold   int
	healthyThreshold     int
	slowStartDuration    time.Duration
	passiveEnabled       bool
}

// CheckerConfig holds health checker configuration.
type CheckerConfig struct {
	Interval             time.Duration
	DefaultPath          string
	Timeout              time.Duration
	UnhealthyThreshold   int // consecutive failures before marking unhealthy
	HealthyThreshold     int // consecutive successes before marking healthy
	SlowStartDuration    time.Duration
	PassiveEnabled       bool
}

// NewChecker creates a health checker.
func NewChecker(b *balancer.Balancer, cfg CheckerConfig, logger *logging.Logger, met *metrics.Metrics) *Checker {
	return &Checker{
		balancer:             b,
		interval:             cfg.Interval,
		defaultPath:          cfg.DefaultPath,
		timeout:              cfg.Timeout,
		client:               &http.Client{Timeout: cfg.Timeout},
		logger:               logger,
		metrics:              met,
		unhealthyThreshold:   cfg.UnhealthyThreshold,
		healthyThreshold:     cfg.HealthyThreshold,
		slowStartDuration:    cfg.SlowStartDuration,
		passiveEnabled:       cfg.PassiveEnabled,
	}
}

// Start begins periodic health checking in a background goroutine.
// It runs an immediate check, then checks every interval until ctx is cancelled.
func (c *Checker) Start(ctx context.Context) {
	go func() {
		c.checkAll() // immediate first check
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.checkAll()
			}
		}
	}()
}

// checkAll performs health checks on all backends.
func (c *Checker) checkAll() {
	for _, backend := range c.balancer.GetBackends() {
		// Use per-backend health path if configured, otherwise use default
		healthPath := c.defaultPath
		if backend.HealthPath != "" {
			healthPath = backend.HealthPath
		}
		
		c.checkBackend(backend, healthPath)
	}
}

// checkBackend performs a health check on a single backend.
func (c *Checker) checkBackend(backend *balancer.Backend, healthPath string) {
	start := time.Now()
	wasHealthy := backend.IsHealthy()
	
	healthy := c.probe(backend.URL.String(), healthPath)
	duration := time.Since(start).Seconds()
	
	// Record health check metrics
	if c.metrics != nil {
		c.metrics.RecordHealthCheckDuration(backend.URL.String(), duration)
	}
	
	if healthy {
		c.handleSuccess(backend, wasHealthy)
	} else {
		c.handleFailure(backend, wasHealthy)
	}
}

// handleSuccess processes a successful health check.
func (c *Checker) handleSuccess(backend *balancer.Backend, wasHealthy bool) {
	backend.RecordSuccess()
	
	// Mark healthy after reaching threshold
	consecutiveSuccesses := backend.GetConsecutiveSuccesses()
	if consecutiveSuccesses >= int64(c.healthyThreshold) {
		if !wasHealthy {
			// Backend recovered
			backend.SetHealthy(true)
			backend.MarkRecovered()
			c.logger.Info("backend recovered", map[string]interface{}{
				"backend":              backend.URL.String(),
				"consecutive_successes": consecutiveSuccesses,
			})
			if c.metrics != nil {
				c.metrics.SetBackendHealthy(backend.URL.String(), true)
			}
		}
	}
	
	if c.metrics != nil && wasHealthy {
		c.metrics.SetBackendHealthy(backend.URL.String(), true)
	}
}

// handleFailure processes a failed health check.
func (c *Checker) handleFailure(backend *balancer.Backend, wasHealthy bool) {
	backend.RecordFailure()
	
	if c.metrics != nil {
		c.metrics.RecordHealthCheckFailure(backend.URL.String())
	}
	
	// Mark unhealthy after reaching threshold
	consecutiveFailures := backend.GetConsecutiveFailures()
	if consecutiveFailures >= int64(c.unhealthyThreshold) {
		if wasHealthy {
			// Backend just went down
			backend.SetHealthy(false)
			c.logger.Error("backend down", map[string]interface{}{
				"backend":             backend.URL.String(),
				"consecutive_failures": consecutiveFailures,
			})
			if c.metrics != nil {
				c.metrics.SetBackendHealthy(backend.URL.String(), false)
			}
		}
	}
}

// probe performs a single health check request.
func (c *Checker) probe(rawURL string, healthPath string) bool {
	resp, err := c.client.Get(fmt.Sprintf("%s%s", rawURL, healthPath))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// PassiveHealthCheck marks a backend as potentially unhealthy based on proxy errors.
// This is called from the proxy when it encounters errors.
func (c *Checker) PassiveHealthCheck(backendURL string, success bool) {
	if !c.passiveEnabled {
		return
	}
	
	// Find the backend
	for _, backend := range c.balancer.GetBackends() {
		if backend.URL.String() == backendURL {
			if success {
				backend.RecordSuccess()
			} else {
				backend.RecordFailure()
				
				// Check if should mark unhealthy
				consecutiveFailures := backend.GetConsecutiveFailures()
				if consecutiveFailures >= int64(c.unhealthyThreshold) && backend.IsHealthy() {
					backend.SetHealthy(false)
					c.logger.Error("backend marked unhealthy (passive)", map[string]interface{}{
						"backend":              backendURL,
						"consecutive_failures": consecutiveFailures,
					})
					if c.metrics != nil {
						c.metrics.SetBackendHealthy(backendURL, false)
					}
				}
			}
			return
		}
	}
}

// GetBackendStats returns health statistics for all backends.
func (c *Checker) GetBackendStats() []map[string]interface{} {
	backends := c.balancer.GetBackends()
	stats := make([]map[string]interface{}, len(backends))
	for i, backend := range backends {
		stat := backend.GetStats()
		stat["url"] = backend.URL.String()
		stats[i] = stat
	}
	return stats
}
