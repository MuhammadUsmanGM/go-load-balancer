package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type BackendConfig struct {
	URL string `json:"url"`
}

type HealthConfig struct {
	Interval string `json:"interval"`
	Path     string `json:"path"`
	Timeout  string `json:"timeout"`
}

type Config struct {
	ListenAddr  string          `json:"listen_addr"`
	Backends    []BackendConfig `json:"backends"`
	HealthCheck HealthConfig    `json:"health_check"`
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
	return &cfg, nil
}
