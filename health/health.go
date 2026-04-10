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
	balancer *balancer.Balancer
	interval time.Duration
	path     string
	client   *http.Client
	logger   *logging.Logger
	metrics  *metrics.Metrics
}

// NewChecker creates a health checker.
func NewChecker(b *balancer.Balancer, interval time.Duration, path string, timeout time.Duration, logger *logging.Logger, met *metrics.Metrics) *Checker {
	return &Checker{
		balancer: b,
		interval: interval,
		path:     path,
		client:   &http.Client{Timeout: timeout},
		logger:   logger,
		metrics:  met,
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

func (c *Checker) checkAll() {
	for _, status := range c.balancer.Snapshot() {
		start := time.Now()
		healthy := c.probe(status.URL)
		duration := time.Since(start).Seconds()
		
		// Record health check metrics
		if c.metrics != nil {
			c.metrics.RecordHealthCheckDuration(status.URL, duration)
			if !healthy {
				c.metrics.RecordHealthCheckFailure(status.URL)
				c.metrics.SetBackendHealthy(status.URL, false)
			} else {
				c.metrics.SetBackendHealthy(status.URL, true)
			}
		}

		wasHealthy := status.Healthy
		c.balancer.SetAlive(status.URL, healthy)

		// Log only on state transitions to avoid noise.
		if healthy && !wasHealthy {
			c.logger.Info("backend recovered", map[string]interface{}{
				"backend": status.URL,
			})
		} else if !healthy && wasHealthy {
			c.logger.Error("backend down", map[string]interface{}{
				"backend": status.URL,
			})
		}
	}
}

func (c *Checker) probe(rawURL string) bool {
	resp, err := c.client.Get(fmt.Sprintf("%s%s", rawURL, c.path))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
