package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

	logger := logging.NewLogger()

	cfg, err := config.Load(os.Args[1])
	if err != nil {
		logger.Error("failed to load config", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	urls := make([]string, len(cfg.Backends))
	for i, b := range cfg.Backends {
		urls[i] = b.URL
	}

	bal, err := balancer.NewBalancer(urls)
	if err != nil {
		logger.Error("failed to create balancer", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	met := metrics.NewMetrics()

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checker := health.NewChecker(bal, interval, cfg.HealthCheck.Path, timeout, logger)
	checker.Start(ctx)

	mux := http.NewServeMux()
	mux.Handle("/metrics", met.Handler())
	mux.Handle("/", proxy.NewHandler(bal, met, logger))

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

	logger.Info("load balancer starting", map[string]interface{}{
		"addr":     cfg.ListenAddr,
		"backends": len(cfg.Backends),
	})

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
}
