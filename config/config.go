package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type BackendConfig struct {
	URL    string `json:"url"`
	Weight int    `json:"weight,omitempty"`
}

type HealthConfig struct {
	Interval string `json:"interval"`
	Path     string `json:"path"`
	Timeout  string `json:"timeout"`
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

type Config struct {
	ListenAddr  string          `json:"listen_addr"`
	Backends    []BackendConfig `json:"backends"`
	HealthCheck HealthConfig    `json:"health_check"`
	Strategy    string          `json:"strategy,omitempty"`
	Metrics     MetricsConfig   `json:"metrics"`
	Tracing     TracingConfig   `json:"tracing"`
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

	return &cfg, nil
}
