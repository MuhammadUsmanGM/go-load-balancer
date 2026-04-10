package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type BackendConfig struct {
	URL          string `json:"url"`
	Weight       int    `json:"weight,omitempty"`
	HealthPath   string `json:"health_path,omitempty"` // Custom health endpoint per backend
}

type HealthConfig struct {
	Interval                    string `json:"interval"`
	Path                        string `json:"path"`
	Timeout                     string `json:"timeout"`
	UnhealthyThreshold        int    `json:"unhealthy_threshold"`         // consecutive failures before marking unhealthy
	HealthyThreshold          int    `json:"healthy_threshold"`           // consecutive successes before marking healthy
	SlowStartDuration         string `json:"slow_start_duration"`         // gradual traffic increase after recovery
	PassiveHealthCheckEnabled bool   `json:"passive_health_check_enabled"` // mark unhealthy based on proxy errors
}

type MetricsConfig struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}

type TracingConfig struct {
	Enabled  bool   `json:"enabled"`
	Endpoint string `json:"endpoint"`
	Insecure bool   `json:"insecure"`
	Service  string `json:"service"`
}

type RateLimitConfig struct {
	Enabled bool    `json:"enabled"`
	Rate    float64 `json:"rate"`  // requests per second
	Burst   int64   `json:"burst"` // max burst size
}

type CircuitBreakerConfig struct {
	Enabled          bool   `json:"enabled"`
	FailureThreshold int64  `json:"failure_threshold"` // failures before opening
	RecoveryTimeout  string `json:"recovery_timeout"`  // Go duration
}

type ConnectionPoolConfig struct {
	MaxIdleConns        int    `json:"max_idle_conns"`
	MaxIdleConnsPerHost int    `json:"max_idle_conns_per_host"`
	IdleConnTimeout     string `json:"idle_conn_timeout"`
	MaxConnsPerHost     int    `json:"max_conns_per_host"`
}

type RetryConfig struct {
	Enabled bool `json:"enabled"`
	MaxRetries int `json:"max_retries"`
}

type Config struct {
	ListenAddr     string             `json:"listen_addr"`
	Backends       []BackendConfig    `json:"backends"`
	HealthCheck    HealthConfig       `json:"health_check"`
	Strategy       string             `json:"strategy,omitempty"`
	Metrics        MetricsConfig      `json:"metrics"`
	Tracing        TracingConfig      `json:"tracing"`
	RateLimit      RateLimitConfig    `json:"rate_limit"`
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker"`
	ConnectionPool ConnectionPoolConfig `json:"connection_pool"`
	Retry          RetryConfig        `json:"retry"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Set default strategy if not specified
	if cfg.Strategy == "" {
		cfg.Strategy = "round-robin"
	}

	// Set default metrics config
	if !cfg.Metrics.Enabled {
		cfg.Metrics.Enabled = true
	}
	if cfg.Metrics.Path == "" {
		cfg.Metrics.Path = "/metrics"
	}

	// Set default tracing config
	if cfg.Tracing.Service == "" {
		cfg.Tracing.Service = "go-load-balancer"
	}

	// Set default health check config
	if cfg.HealthCheck.UnhealthyThreshold == 0 {
		cfg.HealthCheck.UnhealthyThreshold = 3 // 3 consecutive failures
	}
	if cfg.HealthCheck.HealthyThreshold == 0 {
		cfg.HealthCheck.HealthyThreshold = 2 // 2 consecutive successes
	}
	if cfg.HealthCheck.Path == "" {
		cfg.HealthCheck.Path = "/health"
	}

	// Set default rate limit config
	if cfg.RateLimit.Rate == 0 {
		cfg.RateLimit.Rate = 100 // 100 req/s
	}
	if cfg.RateLimit.Burst == 0 {
		cfg.RateLimit.Burst = 20 // burst of 20
	}

	// Set default circuit breaker config
	if cfg.CircuitBreaker.FailureThreshold == 0 {
		cfg.CircuitBreaker.FailureThreshold = 5
	}
	if cfg.CircuitBreaker.RecoveryTimeout == "" {
		cfg.CircuitBreaker.RecoveryTimeout = "30s"
	}

	// Set default connection pool config
	if cfg.ConnectionPool.MaxIdleConnsPerHost == 0 {
		cfg.ConnectionPool.MaxIdleConnsPerHost = 100
	}
	if cfg.ConnectionPool.IdleConnTimeout == "" {
		cfg.ConnectionPool.IdleConnTimeout = "90s"
	}

	// Set default retry config
	if cfg.Retry.MaxRetries == 0 {
		cfg.Retry.MaxRetries = 2
	}

	return &cfg, nil
}
