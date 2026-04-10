package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/go-load-balancer/balancer"
	"github.com/go-load-balancer/balancer/strategy"
	"github.com/go-load-balancer/config"
	"github.com/go-load-balancer/health"
	"github.com/go-load-balancer/logging"
	"github.com/go-load-balancer/metrics"
	"github.com/go-load-balancer/middleware"
	"github.com/go-load-balancer/proxy"
	"github.com/go-load-balancer/tracing"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go-load-balancer <config.json>")
		os.Exit(1)
	}

	logger := logging.NewLogger()

	cfg, err := config.Load(os.Args[1])
	if err != nil {
		logger.Error("failed to load config", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	// Initialize OpenTelemetry tracer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tp, err := tracing.InitTracer(ctx, tracing.Config{
		Enabled:  cfg.Tracing.Enabled,
		Endpoint: cfg.Tracing.Endpoint,
		Insecure: cfg.Tracing.Insecure,
		Service:  cfg.Tracing.Service,
		Version:  "1.0.0",
	})
	if err != nil {
		logger.Error("failed to initialize tracer", map[string]interface{}{"error": err.Error()})
		// Continue without tracing
	} else {
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			tracing.Shutdown(shutdownCtx, tp)
		}()
		logger.Info("tracing initialized", map[string]interface{}{
			"service": cfg.Tracing.Service,
			"enabled": cfg.Tracing.Enabled,
		})
	}

	// Build backends with weights
	backends := make([]balancer.BackendWithWeight, len(cfg.Backends))
	for i, b := range cfg.Backends {
		backends[i] = balancer.BackendWithWeight{
			URL:    b.URL,
			Weight: b.Weight,
		}
	}

	// Create strategy based on config
	strat, err := createStrategy(cfg.Strategy)
	if err != nil {
		logger.Error("failed to create strategy", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	bal, err := balancer.NewBalancer(backends, strat)
	if err != nil {
		logger.Error("failed to create balancer", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	// Initialize Prometheus metrics
	met := metrics.NewMetrics()

	// Set initial backend weights in metrics
	for _, b := range cfg.Backends {
		met.SetBackendWeight(b.URL, b.Weight)
	}

	interval, err := time.ParseDuration(cfg.HealthCheck.Interval)
	if err != nil {
		logger.Error("invalid health check interval", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	timeout, err := time.ParseDuration(cfg.HealthCheck.Timeout)
	if err != nil {
		logger.Error("invalid health check timeout", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	checker := health.NewChecker(bal, interval, cfg.HealthCheck.Path, timeout, logger, met)
	checker.Start(ctx)

	// Build handler chain with middleware
	mux := http.NewServeMux()

	// Metrics endpoint
	if cfg.Metrics.Enabled {
		mux.Handle(cfg.Metrics.Path, met.Handler())
	}

	// Proxy handler with middleware
	proxyHandler := proxy.NewHandler(bal, met, logger)
	
	// Apply middleware chain: Tracing -> RequestID -> Proxy
	handler := middleware.RequestIDMiddleware(proxyHandler)
	if cfg.Tracing.Enabled {
		handler = middleware.TracingMiddleware(cfg.Tracing.Service)(handler)
	}
	
	mux.Handle("/", handler)

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		logger.Info("shutting down", map[string]interface{}{"signal": sig.String()})
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
	}()

	// Periodically update runtime metrics
	go updateRuntimeMetrics(ctx, met, 10*time.Second)

	logger.Info("load balancer starting", map[string]interface{}{
		"addr":      cfg.ListenAddr,
		"backends":  len(cfg.Backends),
		"strategy":  cfg.Strategy,
		"metrics":   cfg.Metrics.Enabled,
		"tracing":   cfg.Tracing.Enabled,
		"version":   "1.0.0",
	})

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
}

// createStrategy returns the appropriate strategy based on config.
func createStrategy(name string) (balancer.Strategy, error) {
	switch name {
	case "round-robin", "":
		return strategy.NewRoundRobin(), nil
	case "least-connections":
		return strategy.NewLeastConnections(), nil
	case "weighted-round-robin":
		return strategy.NewWeightedRoundRobin(), nil
	case "ip-hash":
		return strategy.NewIPHash(), nil
	case "random-two-choices":
		return strategy.NewRandomTwoChoices(), nil
	default:
		return nil, fmt.Errorf("unknown strategy: %s", name)
	}
}

// updateRuntimeMetrics periodicallyically updates Prometheus metrics with Go runtime stats.
func updateRuntimeMetrics(ctx context.Context, met *metrics.Metrics, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			met.UpdateGoroutines(runtime.NumGoroutine())
		}
	}
}
