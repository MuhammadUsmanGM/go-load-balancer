package health

import (
	"context"
	"net/http"
	"time"

	"github.com/go-load-balancer/balancer"
	"github.com/go-load-balancer/logging"
)

type Checker struct {
	balancer *balancer.Balancer
	interval time.Duration
	path     string
	timeout  time.Duration
	logger   *logging.Logger
}

func NewChecker(b *balancer.Balancer, interval time.Duration, path string, timeout time.Duration, logger *logging.Logger) *Checker {
	return &Checker{
		balancer: b,
		interval: interval,
		path:     path,
		timeout:  timeout,
		logger:   logger,
	}
}

func (c *Checker) Start(ctx context.Context) {
	client := &http.Client{Timeout: c.timeout}
	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, be := range c.balancer.GetBackends() {
					target := be.URL.String() + c.path
					resp, err := client.Get(target)
					healthy := err == nil && resp.StatusCode == http.StatusOK
					if resp != nil {
						resp.Body.Close()
					}
					c.balancer.SetHealthy(be.URL.String(), healthy)
					if !healthy {
						c.logger.Error("health check failed", map[string]interface{}{
							"backend": be.URL.String(),
						})
					}
				}
			}
		}
	}()
}
