# Go HTTP Load Balancer

A lightweight HTTP load balancer written in Go using only the standard library. It distributes incoming requests across multiple backends using round-robin selection, with automatic health checking, request metrics, and structured JSON logging.

## Features

- **Round-robin load balancing** with automatic skipping of unhealthy backends
- **Active health checks** -- periodic HTTP probes to detect backend failures
- **Request metrics** -- total requests, per-backend counts, and status code breakdown via `/metrics`
- **Structured JSON logging** -- machine-readable log output to stdout
- **JSON configuration** -- simple file-based setup, no external dependencies

## Architecture

The project is organized into focused packages:

| Package | Purpose |
|---------|---------|
| `main` | Entry point, wires all components |
| `config` | Loads and parses JSON configuration |
| `balancer` | Round-robin backend selection with health tracking |
| `proxy` | Reverse proxy handler using `httputil.ReverseProxy` |
| `health` | Periodic background health checker |
| `metrics` | In-memory request counters with JSON endpoint |
| `logging` | Structured JSON logger |

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed design documentation.

## Quick Start

### Prerequisites

- Go 1.21 or later

### Build

```bash
go build -o go-load-balancer .
```

### Run test backends

Start a few simple HTTP servers to act as backends:

```bash
for port in 8081 8082 8083; do
  go run -C testbackend . -addr ":$port" &
done
```

Or use any HTTP servers that respond on the configured ports. A minimal backend only needs to return `200 OK` on `/health`.

### Run the load balancer

```bash
./go-load-balancer config.json
```

The load balancer starts on the address specified in `config.json` (default `:8080`).

## Configuration

All settings are defined in `config.json`:

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

| Field | Description |
|-------|-------------|
| `listen_addr` | Address and port the load balancer listens on |
| `backends[].url` | URL of each backend server |
| `health_check.interval` | Time between health check rounds (Go duration, e.g. `10s`) |
| `health_check.path` | HTTP path to probe on each backend |
| `health_check.timeout` | Timeout for each health check request |

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `/*` | All requests are proxied to the next healthy backend |
| `/metrics` | Returns JSON with request counts and status code breakdown |

### Metrics response example

```json
{
  "total_requests": 150,
  "status_codes": {"200": 145, "502": 5},
  "backend_counts": {
    "http://localhost:8081": 50,
    "http://localhost:8082": 50,
    "http://localhost:8083": 50
  }
}
```

## Testing

```bash
go test ./...
```

## Example Usage

Send requests through the load balancer:

```bash
# Single request
curl http://localhost:8080/

# Multiple requests to see round-robin distribution
for i in $(seq 1 10); do
  curl -s http://localhost:8080/
done

# Check metrics
curl -s http://localhost:8080/metrics | jq .
```
