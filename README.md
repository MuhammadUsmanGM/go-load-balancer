# Go HTTP Load Balancer

A production-grade HTTP load balancer written in Go with **advanced observability features** including Prometheus metrics, distributed tracing with OpenTelemetry, request ID tracking, and structured logging. It distributes incoming requests across multiple backends using **multiple configurable strategies**, with automatic health checking and real-time metrics.

## Features

### 🎯 Load Balancing

- **Multiple Load Balancing Strategies**:
  - **Round-Robin** -- distributes requests evenly across backends in rotation
  - **Least Connections** -- routes to the backend with fewest active requests
  - **Weighted Round-Robin** -- assigns proportional traffic based on backend capacity
  - **IP Hash** -- ensures session persistence by routing same client IP to same backend
  - **Random with 2 Choices** -- picks 2 random backends, selects the lesser-loaded one
- **Active health checks** -- periodic HTTP probes to detect backend failures
- **Connection tracking** -- monitors active connections for intelligent load balancing
- **JSON configuration** -- simple file-based setup, no external dependencies

### 📊 Advanced Observability

- **Prometheus Metrics**:
  - Request rate and duration (histograms with p50, p95, p99)
  - Active connections per backend (gauges)
  - Backend health status (gauges)
  - Health check duration and failures (histograms & counters)
  - Response size distribution (histograms)
  - Go runtime metrics (goroutines)
  
- **Distributed Tracing (OpenTelemetry)**:
  - Full request lifecycle tracing
  - Trace context propagation via W3C TraceContext
  - Compatible with Jaeger, Zipkin, and other OpenTelemetry backends
  - Automatic span creation with HTTP attributes
  
- **Request ID Tracking**:
  - Auto-generated unique request IDs (`X-Request-ID` header)
  - Propagated through all logs and traces
  - Supports incoming request ID preservation
  
- **Structured JSON Logging**:
  - Machine-readable log output
  - Request IDs embedded in all log entries
  - Timestamp, level, message, and custom fields
  - Service name and metadata

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
  "strategy": "round-robin",
  "backends": [
    {"url": "http://localhost:8081", "weight": 1},
    {"url": "http://localhost:8082", "weight": 2},
    {"url": "http://localhost:8083", "weight": 1}
  ],
  "health_check": {
    "interval": "10s",
    "path": "/health",
    "timeout": "2s"
  },
  "metrics": {
    "enabled": true,
    "path": "/metrics"
  },
  "tracing": {
    "enabled": false,
    "endpoint": "localhost:4318",
    "insecure": true,
    "service": "go-load-balancer"
  }
}
```

| Field | Description |
|-------|-------------|
| `listen_addr` | Address and port the load balancer listens on |
| `strategy` | Load balancing algorithm: `round-robin`, `least-connections`, `weighted-round-robin`, `ip-hash`, or `random-two-choices` (default: `round-robin`) |
| `backends[].url` | URL of each backend server |
| `backends[].weight` | Backend capacity weight for `weighted-round-robin` strategy (higher = more traffic) |
| `health_check.interval` | Time between health check rounds (Go duration, e.g. `10s`) |
| `health_check.path` | HTTP path to probe on each backend |
| `health_check.timeout` | Timeout for each health check request |
| `metrics.enabled` | Enable Prometheus metrics endpoint (default: `true`) |
| `metrics.path` | Path for metrics endpoint (default: `/metrics`) |
| `tracing.enabled` | Enable OpenTelemetry distributed tracing (default: `false`) |
| `tracing.endpoint` | OTLP endpoint for trace export (default: `localhost:4318`) |
| `tracing.insecure` | Use insecure connection for OTLP endpoint (default: `true`) |
| `tracing.service` | Service name for traces (default: `go-load-balancer`) |

### Load Balancing Strategies

| Strategy | Description | Use Case |
|----------|-------------|----------|
| `round-robin` | Distributes requests in circular order | General purpose, equal backends |
| `least-connections` | Routes to backend with fewest active requests | Varying request durations |
| `weighted-round-robin` | Proportional distribution by weight | Different backend capacities |
| `ip-hash` | Same client IP always goes to same backend | Session persistence required |
| `random-two-choices` | Picks 2 random backends, chooses lesser-loaded | Simple load distribution |

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `/*` | All requests are proxied to the next healthy backend |
| `/metrics` | Prometheus metrics endpoint (scrape with Prometheus or view in browser) |

### Response Headers

All responses include:
- `X-Request-ID`: Unique request identifier for tracing and debugging

## Prometheus Metrics

The load balancer exposes comprehensive Prometheus metrics at `/metrics`:

### Request Metrics
- `lb_requests_total` - Total HTTP requests (labels: backend, status_code, method)
- `lb_request_duration_seconds` - Request latency histogram (labels: backend, status_code, method)
- `lb_requests_per_second` - Request rate per backend
- `lb_response_size_bytes` - Response size histogram

### Connection Metrics
- `lb_active_connections` - Active connections per backend (gauge)
- `lb_backend_healthy` - Backend health status (1 = healthy, 0 = unhealthy)
- `lb_backend_weight` - Backend weight configuration

### Health Check Metrics
- `lb_healthcheck_duration_seconds` - Health check latency histogram
- `lb_healthcheck_failures_total` - Total health check failures

### Runtime Metrics
- `lb_goroutines` - Current number of goroutines

### Example Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'load-balancer'
    scrape_interval: 5s
    static_configs:
      - targets: ['localhost:8080']
```

## Distributed Tracing

The load balancer supports distributed tracing via OpenTelemetry. To enable tracing:

1. **Configure tracing in `config.json`**:
```json
{
  "tracing": {
    "enabled": true,
    "endpoint": "localhost:4318",
    "insecure": true,
    "service": "go-load-balancer"
  }
}
```

2. **Start an OpenTelemetry collector** (e.g., Jaeger):
```bash
docker run -d --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 16686:16686 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest
```

3. **View traces** at http://localhost:16686

### Trace Context Propagation

The load balancer automatically:
- Extracts trace context from incoming requests (W3C TraceContext)
- Creates spans for each request with HTTP attributes
- Propagates trace context to backend services
- Includes request ID in span attributes

## Grafana Dashboard

A pre-built Grafana dashboard is included at `grafana-dashboard.json`. It provides:

- Request rate and duration graphs (p50, p95, p99)
- Active connections per backend
- Backend health status indicators
- Health check performance
- Error rate tracking
- Response size distribution
- Runtime metrics

### Import Dashboard

1. Open Grafana and go to **Dashboards > Import**
2. Upload `grafana-dashboard.json`
3. Select your Prometheus data source
4. Dashboard will appear as "Go Load Balancer - Observability Dashboard"

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
