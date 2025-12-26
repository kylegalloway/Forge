# Test 03: Health Check

## Description

Tests controller health and readiness endpoints:
- Controller pods are running
- Health endpoint responds (`:8081/healthz`)
- Readiness endpoint responds (`:8081/readyz`)
- Metrics endpoint is accessible (`:8080/metrics`)

## Prerequisites

- Forge controller deployed in `forge-system` namespace

## Expected Behavior

1. Controller pods are running and ready
2. Health endpoint returns 200 OK
3. Readiness endpoint returns 200 OK
4. Metrics endpoint exposes Prometheus metrics

## Running the Test

```bash
# Run the health check script
./test.sh

# Or run commands manually:

# Check controller pods
kubectl get pods -n forge-system -l app=forge-controller

# Port forward to health endpoint
kubectl port-forward -n forge-system svc/forge-controller 8081:8081 &

# Check health
curl http://localhost:8081/healthz
# Expected: OK (200)

# Check readiness
curl http://localhost:8081/readyz
# Expected: Ready (200)

# Port forward to metrics endpoint
kubectl port-forward -n forge-system svc/forge-controller 8080:8080 &

# Check metrics
curl http://localhost:8080/metrics | grep forge_
# Expected: Various forge_* metrics

# Cleanup port forwards
pkill -f "port-forward.*forge-controller"
```

## Success Criteria

- All controller pods are Running and Ready (1/1)
- `/healthz` returns HTTP 200 with body "OK"
- `/readyz` returns HTTP 200 with body "Ready"
- `/metrics` returns Prometheus metrics containing `forge_` prefix
