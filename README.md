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

### 🛡️ Production Features

- **Rate Limiting**:
  - Token bucket algorithm per client IP
  - Configurable rate and burst size
  - Automatic client tracking
  - HTTP 429 response when limit exceeded
  
- **Circuit Breaker Pattern**:
  - Three states: Closed, Half-Open, Open
  - Configurable failure threshold
  - Automatic recovery with half-open testing
  - Prevents cascading failures
  - Per-backend circuit breakers
  
- **Connection Pooling**:
  - Configurable max idle connections
  - Per-host connection limits
  - Idle connection timeout
  - Connection reuse optimization
  
- **Request Retry**:
  - Automatic retry on backend failures (5xx errors)
  - Configurable max retry attempts
  - Intelligent backend selection on retry
  - Retry attempt tracking in logs
  
- **Hot-Reload Configuration**:
  - Watch config file for changes using fsnotify
  - Zero-downtime config updates
  - Automatic reload without restart
  - Graceful transition to new settings

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

## Production Features

### Rate Limiting

The load balancer includes a token bucket rate limiter that tracks requests per client IP:

```json
{
  "rate_limit": {
    "enabled": true,
    "rate": 100,      // requests per second
    "burst": 20       // max burst size
  }
}
```

**How it works:**
- Each client IP gets its own token bucket
- Tokens refill at the configured rate (100/sec = 1 token per 10ms)
- Burst allows temporary spikes up to the burst limit
- Returns HTTP 429 (Too Many Requests) when limit exceeded

**Example:**
```bash
# With rate=100 and burst=20, you can send 20 requests instantly
# Then 100 more per second
for i in {1..25}; do curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/; done
# First 20: 200 OK, Next 5: 429 Too Many Requests
```

### Circuit Breaker

Prevents cascading failures by stopping traffic to failing backends:

```json
{
  "circuit_breaker": {
    "enabled": true,
    "failure_threshold": 5,        // failures before opening
    "recovery_timeout": "30s"      // time before testing recovery
  }
}
```

**Circuit Breaker States:**
1. **Closed** - Normal operation, tracking failures count
2. **Open** - Rejecting all requests after reaching threshold
3. **Half-Open** - Testing if backend recovered (after recovery timeout)

**Lifecycle:**
1. Backend fails 5 times → Circuit opens (rejects requests)
2. After 30 seconds → Circuit goes half-open (allows 1 test request)
3. Test succeeds → Circuit closes (normal operation resumes)
4. Test fails → Circuit reopens (wait another 30s)

### Connection Pooling

Optimize connection reuse with configurable pooling:

```json
{
  "connection_pool": {
    "max_idle_conns": 100,              // global max idle connections
    "max_idle_conns_per_host": 100,     // per-backend idle connections
    "idle_conn_timeout": "90s",         // how long to keep idle conns
    "max_conns_per_host": 100           // total connections per backend
  }
}
```

**Best Practices:**
- Set `max_idle_conns_per_host` based on expected concurrent requests
- Higher values = better performance but more memory
- `idle_conn_timeout` prevents stale connections

### Request Retry

Automatically retry failed requests on different backends:

```json
{
  "retry": {
    "enabled": true,
    "max_retries": 2    // retry up to 2 times
  }
}
```

**How it works:**
1. Request to Backend A fails (5xx error)
2. Retry on Backend B (different backend via load balancer)
3. If B also fails, retry on Backend C
4. Logs include attempt count for debugging

**Example log output:**
```json
{
  "level": "info",
  "message": "retrying request",
  "backend": "http://localhost:8081",
  "status": 502,
  "attempt": 1,
  "max_retry": 2
}
```

### Hot-Reload Configuration

Update configuration without restarting the load balancer:

**How it works:**
1. Load balancer watches `config.json` for changes
2. On file save, automatically reloads configuration
3. Applies new settings gracefully

**What can be changed:**
- Backend weights (traffic distribution)
- Rate limits
- Circuit breaker thresholds
- Connection pool settings
- Retry configuration

**Example:**
```bash
# In one terminal, run the load balancer
./go-load-balancer config.json

# In another terminal, edit config.json
# Change a backend weight from 1 to 3
# Save the file - changes apply automatically!
```

**Note:** Some changes (like adding/removing backends) may require careful handling in production. The current implementation logs changes but doesn't dynamically add/remove backends without restart.

## Advanced Health Checks

The load balancer implements a sophisticated health checking system with multiple layers of backend monitoring:

### Active Health Checking

Periodic HTTP probes to detect backend failures:

```json
{
  "health_check": {
    "interval": "10s",
    "path": "/health",
    "timeout": "2s",
    "unhealthy_threshold": 3,
    "healthy_threshold": 2,
    "slow_start_duration": "60s",
    "passive_health_check_enabled": true
  }
}
```

**Configuration Options:**

| Setting | Description | Default |
|---------|-------------|---------|
| `interval` | Time between health check rounds | `10s` |
| `path` | HTTP path to probe | `/health` |
| `timeout` | Timeout for each health check request | `2s` |
| `unhealthy_threshold` | Consecutive failures before marking unhealthy | `3` |
| `healthy_threshold` | Consecutive successes before marking healthy | `2` |
| `slow_start_duration` | Gradual traffic increase after recovery | `60s` |
| `passive_health_check_enabled` | Mark unhealthy based on proxy errors | `true` |

### Consecutive Failures Threshold

Prevents false positives from transient errors:

**How it works:**
- Backend must fail `unhealthy_threshold` (3) consecutive times before being marked unhealthy
- A single success resets the failure counter
- Prevents marking backend unhealthy due to momentary blips

**Example:**
```
Health Check 1: FAIL (consecutive failures: 1)
Health Check 2: FAIL (consecutive failures: 2)
Health Check 3: FAIL (consecutive failures: 3) → MARK UNHEALTHY

vs.

Health Check 1: FAIL (consecutive failures: 1)
Health Check 2: FAIL (consecutive failures: 2)
Health Check 3: SUCCESS (consecutive failures: 0, consecutive successes: 1)
Health Check 4: FAIL (consecutive failures: 1) ← Reset, still healthy
```

### Passive Health Checking

Monitor backend health based on actual proxy errors, not just health checks:

**How it works:**
- Every proxied request is tracked for success/failure
- 5xx responses count as failures
- Consecutive failures from proxy traffic also trigger unhealthy state
- Provides faster detection than periodic health checks alone

**Benefits:**
- Detects issues between health check intervals
- Catches application-level errors (5xx) that `/health` might miss
- Faster response to backend degradation

**Example:**
```json
{
  "level": "error",
  "message": "backend marked unhealthy (passive)",
  "backend": "http://localhost:8081",
  "consecutive_failures": 3
}
```

### Slow Start After Recovery

Gradually increase traffic to recovered backends to prevent overload:

**How it works:**
1. Backend recovers (passes health checks)
2. Marked as healthy but enters "slow start" mode
3. Receives reduced traffic (starts at 10% of normal)
4. Traffic gradually increases over `slow_start_duration`
5. After duration completes, receives full traffic

**Traffic Ramp:**
```
Time 0s:   10% of normal traffic
Time 15s:  32.5% of normal traffic (25% through 60s)
Time 30s:  55% of normal traffic (50% through 60s)
Time 45s:  77.5% of normal traffic (75% through 60s)
Time 60s:  100% of normal traffic (slow start complete)
```

**Why it matters:**
- Backend may need time to warm up caches, connection pools
- Prevents overwhelming a backend that just recovered
- Allows gradual load testing while monitoring stability

**Prometheus Metrics:**
- `lb_backend_slow_start_progress` - Current slow start progress (0.0 to 1.0)
- `lb_backend_consecutive_failures` - Current consecutive failure count
- `lb_backend_consecutive_successes` - Current consecutive success count

### Health Check Flow

Complete health check lifecycle:

```
1. Active Health Check (every 10s)
   ├─ Probe /health endpoint
   ├─ Track success/failure
   └─ Update consecutive counters

2. Passive Health Check (every request)
   ├─ Track 5xx errors
   ├─ Update consecutive counters
   └─ Mark unhealthy if threshold reached

3. State Transitions
   ├─ Healthy → Unhealthy: After N consecutive failures
   └─ Unhealthy → Healthy: After M consecutive successes

4. Recovery
   ├─ Mark recovered timestamp
   ├─ Enter slow start mode
   └─ Gradually increase traffic
```

### Health Check Best Practices

1. **Design a meaningful `/health` endpoint:**
   - Check database connectivity
   - Verify external service connections
   - Return 503 if not ready to serve traffic

2. **Tune thresholds for your environment:**
   - Low threshold (2-3): Fast detection, more false positives
   - High threshold (5-10): Fewer false positives, slower detection

3. **Enable passive health checking:**
   - Catches issues that active checks miss
   - Provides faster detection of degradation

4. **Use slow start for stateful backends:**
   - Databases, caches benefit from warmup period
   - Prevents thundering herd on recovery

5. **Monitor health metrics in Grafana:**
   - Set alerts on consecutive failures
   - Track slow start progress
   - Monitor health check duration

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
