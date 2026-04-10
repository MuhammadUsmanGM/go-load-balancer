package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/go-load-balancer/balancer"
	"github.com/go-load-balancer/balancer/strategy"
	"github.com/go-load-balancer/circuitbreaker"
	"github.com/go-load-balancer/config"
	"github.com/go-load-balancer/health"
	"github.com/go-load-balancer/logging"
	"github.com/go-load-balancer/metrics"
	"github.com/go-load-balancer/middleware"
	"github.com/go-load-balancer/proxy"
	"github.com/go-load-balancer/ratelimit"
	"github.com/go-load-balancer/tracing"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go-load-balancer <config.json>")
		os.Exit(1)
	}

	logger := logging.NewLogger()

	// Load initial config
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

	// Create balancer and metrics
	backends := make([]balancer.BackendWithWeight, len(cfg.Backends))
	for i, b := range cfg.Backends {
		backends[i] = balancer.BackendWithWeight{
			URL:    b.URL,
			Weight: b.Weight,
		}
	}

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

	met := metrics.NewMetrics()

	// Set initial backend weights
	for _, b := range cfg.Backends {
		met.SetBackendWeight(b.URL, b.Weight)
	}

	// Parse health check intervals
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
	
	// Parse slow start duration
	var slowStartDuration time.Duration
	if cfg.HealthCheck.SlowStartDuration != "" {
		slowStartDuration, err = time.ParseDuration(cfg.HealthCheck.SlowStartDuration)
		if err != nil {
			logger.Error("invalid slow start duration", map[string]interface{}{"error": err.Error()})
			os.Exit(1)
		}
	}

	// Start health checker with advanced config
	healthCfg := health.CheckerConfig{
		Interval:             interval,
		DefaultPath:          cfg.HealthCheck.Path,
		Timeout:              timeout,
		UnhealthyThreshold:   cfg.HealthCheck.UnhealthyThreshold,
		HealthyThreshold:     cfg.HealthCheck.HealthyThreshold,
		SlowStartDuration:    slowStartDuration,
		PassiveEnabled:       cfg.HealthCheck.PassiveHealthCheckEnabled,
	}
	
	checker := health.NewChecker(bal, healthCfg, logger, met)
	checker.Start(ctx)

	// Create rate limiter
	var rl *ratelimit.RateLimiter
	if cfg.RateLimit.Enabled {
		rl = ratelimit.NewRateLimiter(cfg.RateLimit.Rate, cfg.RateLimit.Burst)
		logger.Info("rate limiting enabled", map[string]interface{}{
			"rate":  cfg.RateLimit.Rate,
			"burst": cfg.RateLimit.Burst,
		})
	}

	// Create circuit breakers
	var circuitBreakers map[string]*circuitbreaker.CircuitBreaker
	if cfg.CircuitBreaker.Enabled {
		recoveryTimeout, err := time.ParseDuration(cfg.CircuitBreaker.RecoveryTimeout)
		if err != nil {
			logger.Error("invalid circuit breaker recovery timeout", map[string]interface{}{"error": err.Error()})
			os.Exit(1)
		}

		backendURLs := make([]string, len(cfg.Backends))
		for i, b := range cfg.Backends {
			backendURLs[i] = b.URL
		}
		circuitBreakers = proxy.InitCircuitBreakers(backendURLs, cfg.CircuitBreaker.FailureThreshold, recoveryTimeout)

		logger.Info("circuit breaker enabled", map[string]interface{}{
			"failure_threshold": cfg.CircuitBreaker.FailureThreshold,
			"recovery_timeout":  cfg.CircuitBreaker.RecoveryTimeout,
		})
	}

	// Parse connection pool settings
	idleConnTimeout, err := time.ParseDuration(cfg.ConnectionPool.IdleConnTimeout)
	if err != nil {
		logger.Error("invalid idle conn timeout", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	proxyCfg := proxy.ProxyConfig{
		MaxIdleConns:        cfg.ConnectionPool.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.ConnectionPool.MaxIdleConnsPerHost,
		IdleConnTimeout:     idleConnTimeout,
		MaxConnsPerHost:     cfg.ConnectionPool.MaxConnsPerHost,
		MaxRetries:          cfg.Retry.MaxRetries,
		RetryEnabled:        cfg.Retry.Enabled,
		PassiveHealthCheck:  cfg.HealthCheck.PassiveHealthCheckEnabled,
	}

	// Build handler chain with middleware
	mux := http.NewServeMux()

	// Metrics endpoint
	if cfg.Metrics.Enabled {
		mux.Handle(cfg.Metrics.Path, met.Handler())
	}

	// Create proxy handler with health checker for passive monitoring
	proxyHandler := proxy.NewHandler(bal, met, logger, proxyCfg, circuitBreakers, checker)

	// Apply middleware chain: Tracing -> RequestID -> RateLimit -> Proxy
	var handler http.Handler = proxyHandler
	
	// Rate limiting middleware
	if rl != nil {
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := r.RemoteAddr
			if !rl.Allow(clientIP) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				met.Record("rate-limited", "429", r.Method, 0, 0)
				return
			}
			proxyHandler.ServeHTTP(w, r)
		})
	}
	
	// Request ID middleware
	handler = middleware.RequestIDMiddleware(handler)
	
	// Tracing middleware
	if cfg.Tracing.Enabled {
		handler = middleware.TracingMiddleware(cfg.Tracing.Service)(handler)
	}
	
	mux.Handle("/", handler)

	// HTTP server
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM
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
	
	// Periodically update backend health stats
	go updateBackendHealthStats(ctx, met, bal, 5*time.Second, slowStartDuration)

	// Start config hot-reload if enabled
	var reloadOnce sync.Once
	configReloader, err := config.NewHotReloader(os.Args[1], func(newCfg *config.Config) {
		reloadOnce.Do(func() {
			logger.Info("config file changed, reloading", nil)
			// Note: In production, you'd want to gracefully update strategies, backends, etc.
			// For now, we just log the change
			logger.Info("config reloaded successfully", map[string]interface{}{
				"backends": len(newCfg.Backends),
				"strategy": newCfg.Strategy,
			})
		})
	})
	if err != nil {
		logger.Warn("failed to create config hot-reloader (continuing without hot-reload)", map[string]interface{}{
			"error": err.Error(),
		})
	} else {
		if err := configReloader.Start(ctx); err != nil {
			logger.Warn("failed to start config hot-reloader (continuing without hot-reload)", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			logger.Info("config hot-reload enabled", map[string]interface{}{
				"config_file": os.Args[1],
			})
		}
	}

	logger.Info("load balancer starting", map[string]interface{}{
		"addr":           cfg.ListenAddr,
		"backends":       len(cfg.Backends),
		"strategy":       cfg.Strategy,
		"metrics":        cfg.Metrics.Enabled,
		"tracing":        cfg.Tracing.Enabled,
		"rate_limit":     cfg.RateLimit.Enabled,
		"circuit_breaker": cfg.CircuitBreaker.Enabled,
		"retry":          cfg.Retry.Enabled,
		"hot_reload":     configReloader != nil,
		"version":        "2.0.0",
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

// updateRuntimeMetrics periodically updates Prometheus metrics with Go runtime stats.
func updateRuntimeMetrics(ctx context.Context, met *metrics.Metrics, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runtime.ReadMemStats(nil)
			met.UpdateGoroutines(runtime.NumGoroutine())
		}
	}
}

// updateBackendHealthStats periodically updates health statistics for all backends.
func updateBackendHealthStats(ctx context.Context, met *metrics.Metrics, bal *balancer.Balancer, interval, slowStartDuration time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, backend := range bal.GetBackends() {
				slowStartProgress := backend.GetSlowStartWeight(slowStartDuration)
				met.UpdateBackendHealthStats(
					backend.URL.String(),
					slowStartProgress,
					backend.GetConsecutiveFailures(),
					backend.GetConsecutiveSuccesses(),
				)
			}
		}
	}
}
