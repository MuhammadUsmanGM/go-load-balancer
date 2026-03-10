# Go HTTP Load Balancer - Architecture

## Overview

A simple HTTP load balancer in Go that distributes requests across backends using round-robin selection, with health checking, metrics, and structured logging.

## Module Structure

Single Go module (`go-load-balancer`), packages under project root:

| Package | File | Responsibility |
|---------|------|----------------|
| `main` | `main.go` | Entry point. Loads config from CLI arg, wires components, starts HTTP server. |
| `config` | `config/config.go` | JSON config struct and file loader. |
| `balancer` | `balancer/balancer.go` | Round-robin backend selection with `sync.Mutex`. Tracks healthy/unhealthy backends. |
| `proxy` | `proxy/proxy.go` | Creates `httputil.ReverseProxy` handler per request using balancer. |
| `health` | `health/health.go` | Periodic health checker goroutine. HTTP GET to each backend's health path. |
| `metrics` | `metrics/metrics.go` | In-memory atomic counters. Exposes `/metrics` JSON endpoint. |
| `logging` | `logging/logging.go` | Structured JSON logger wrapping the `log` package. |

## Config Structure (config.json)

```json
{
  "listen_addr": ":8080",
  "backends": [
    {"url": "http://localhost:8081"},
    {"url": "http://localhost:8082"},
    {"url": "http://localhost:8083"}
  ],
  "health_check": {
    "interval": "10s",
    "path": "/health",
    "timeout": "2s"
  }
}
```

## Request Lifecycle

1. Client sends HTTP request to load balancer on `:8080`
2. Proxy handler calls `Balancer.Next()` to get next healthy backend (round-robin)
3. `httputil.ReverseProxy` forwards the request to the selected backend
4. Backend response is returned to the client
5. Metrics counters are incremented (total requests, per-backend counts)

## Key Types

### Balancer (`balancer/balancer.go`)

```go
type Backend struct {
    URL     *url.URL
    Healthy bool
}

type Balancer struct {
    mu       sync.Mutex
    backends []*Backend
    current  uint64
}

func (b *Balancer) Next() (*Backend, error)           // Round-robin, skips unhealthy
func (b *Balancer) SetHealthy(url string, healthy bool) // Called by health checker
func (b *Balancer) GetBackends() []*Backend             // For health checker iteration
```

### Health Checker (`health/health.go`)

```go
func Start(ctx context.Context, b *balancer.Balancer, interval time.Duration, path string, timeout time.Duration)
```

Runs a goroutine that periodically HTTP GETs `backend.URL + path`. Updates `Balancer.SetHealthy()` based on response status.

### Proxy (`proxy/proxy.go`)

```go
func NewHandler(b *balancer.Balancer, m *metrics.Metrics) http.Handler
```

Returns an `http.Handler` that selects a backend via `b.Next()` and proxies the request using `httputil.ReverseProxy`.

### Metrics (`metrics/metrics.go`)

```go
type Metrics struct {
    TotalRequests  atomic.Int64
    BackendCounts  map[string]*atomic.Int64
}

func (m *Metrics) Handler() http.Handler  // Serves /metrics as JSON
func (m *Metrics) Record(backendURL string)
```

### Config (`config/config.go`)

```go
type Config struct {
    ListenAddr  string          `json:"listen_addr"`
    Backends    []BackendConfig `json:"backends"`
    HealthCheck HealthConfig    `json:"health_check"`
}

func Load(path string) (*Config, error)
```

## Wiring (main.go)

```
1. Parse CLI arg for config path
2. config.Load(path)
3. Create Balancer with backend URLs
4. Create Metrics
5. Start health checker goroutine
6. Register proxy handler on "/" and metrics handler on "/metrics"
7. ListenAndServe on config.ListenAddr
```

## Dependencies

Standard library only: `net/http`, `net/http/httputil`, `net/url`, `encoding/json`, `sync`, `sync/atomic`, `context`, `time`, `log`, `os`, `flag`.
