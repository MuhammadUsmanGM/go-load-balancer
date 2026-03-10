package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValid(t *testing.T) {
	cfg, err := Load(filepath.Join("testdata", "config.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %s, want :8080", cfg.ListenAddr)
	}
	if len(cfg.Backends) != 3 {
		t.Fatalf("expected 3 backends, got %d", len(cfg.Backends))
	}
	if cfg.Backends[0].URL != "http://localhost:8081" {
		t.Errorf("backend 0 URL = %s, want http://localhost:8081", cfg.Backends[0].URL)
	}
	if cfg.HealthCheck.Interval != "10s" {
		t.Errorf("HealthCheck.Interval = %s, want 10s", cfg.HealthCheck.Interval)
	}
	if cfg.HealthCheck.Path != "/health" {
		t.Errorf("HealthCheck.Path = %s, want /health", cfg.HealthCheck.Path)
	}
	if cfg.HealthCheck.Timeout != "3s" {
		t.Errorf("HealthCheck.Timeout = %s, want 3s", cfg.HealthCheck.Timeout)
	}
}

func TestLoadInvalid(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(tmp, []byte("{invalid json!!!"), 0644)

	_, err := Load(tmp)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load("nonexistent_file.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
