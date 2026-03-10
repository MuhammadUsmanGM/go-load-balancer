# Testing Guide

## Unit Tests

Run all unit tests:

```bash
go test ./...
```

Run with verbose output:

```bash
go test -v ./...
```

Run tests for a specific package:

```bash
go test -v ./balancer/
go test -v ./config/
```

### Expected Results

All tests should pass with `ok` status:

- `balancer.TestNewBalancer` - verifies backends are created with correct URLs and healthy status
- `balancer.TestRoundRobin` - verifies even distribution across 3 backends (3 requests each out of 9)
- `balancer.TestSkipUnhealthy` - verifies unhealthy backends are never selected
- `balancer.TestAllUnhealthy` - verifies proper error when no backends are available
- `balancer.TestSetHealthy` - verifies toggling backend health on and off
- `config.TestLoadValid` - verifies correct parsing of config.json
- `config.TestLoadInvalid` - verifies error on malformed JSON
- `config.TestLoadMissing` - verifies error on missing file

## Manual Integration Test

### 1. Start Test Backends

Open three separate terminals and start backend servers:

```bash
go run cmd/testbackend/main.go 8081
go run cmd/testbackend/main.go 8082
go run cmd/testbackend/main.go 8083
```

### 2. Start the Load Balancer

```bash
go run main.go config.json
```

### 3. Send Test Requests

```bash
curl http://localhost:8080/
```

Expected: response from one of the backends, e.g. `Hello from backend :8081`

Send multiple requests to observe round-robin:

```bash
for i in $(seq 1 6); do curl -s http://localhost:8080/; done
```

Expected output (order cycles through backends):

```
Hello from backend :8081
Hello from backend :8082
Hello from backend :8083
Hello from backend :8081
Hello from backend :8082
Hello from backend :8083
```

### 4. Test Health Check Behavior

Stop one backend (e.g. Ctrl+C on port 8082). After the health check interval (10s), requests should only go to the remaining healthy backends.

### 5. Check Metrics

```bash
curl http://localhost:8080/metrics
```

Expected: JSON with total_requests, status_codes, and backend_counts.

## Simple Load Test

Run 100 requests and count distribution:

```bash
for i in $(seq 1 100); do curl -s http://localhost:8080/; done | sort | uniq -c
```

Expected: roughly equal distribution across all healthy backends. With 3 backends and 100 requests, expect approximately 33-34 requests per backend.

## Fault Tolerance Test

1. Start all 3 backends and the load balancer
2. Send requests to confirm all backends receive traffic
3. Kill one backend (Ctrl+C)
4. Wait 10+ seconds for the health checker to detect the failure
5. Send more requests -- only the 2 remaining backends should receive traffic
6. Restart the stopped backend
7. Wait 10+ seconds for the health checker to detect recovery
8. Send requests again -- all 3 backends should receive traffic
