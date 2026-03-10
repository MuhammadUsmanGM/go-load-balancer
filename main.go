package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-load-balancer/balancer"
	"github.com/go-load-balancer/config"
	"github.com/go-load-balancer/health"
	"github.com/go-load-balancer/logging"
	"github.com/go-load-balancer/metrics"
	"github.com/go-load-balancer/proxy"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go-load-balancer <config.json>")
		os.Exit(1)
	}

	cfg, err := config.Load(os.Args[1])
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	urls := make([]string, len(cfg.Backends))
	for i, b := range cfg.Backends {
		urls[i] = b.URL
	}

	bal, err := balancer.NewBalancer(urls)
	if err != nil {
		log.Fatalf("failed to create balancer: %v", err)
	}

	logger := logging.NewLogger()
	met := metrics.NewMetrics()

	// Parse health check durations
	interval, err := time.ParseDuration(cfg.HealthCheck.Interval)
	if err != nil {
		log.Fatalf("invalid health check interval: %v", err)
	}
	timeout, err := time.ParseDuration(cfg.HealthCheck.Timeout)
	if err != nil {
		log.Fatalf("invalid health check timeout: %v", err)
	}

	// Start health checker
	checker := health.NewChecker(bal, interval, cfg.HealthCheck.Path, timeout, logger)
	checker.Start(context.Background())

	http.Handle("/metrics", met.Handler())
	http.Handle("/", proxy.NewHandler(bal, met, logger))

	logger.Info("load balancer starting", map[string]interface{}{
		"addr": cfg.ListenAddr,
	})
	if err := http.ListenAndServe(cfg.ListenAddr, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
