#!/bin/bash
set -e

echo "=================================="
echo "Forge Health Check Test"
echo "=================================="
echo ""

# Check if forge-system namespace exists
if ! kubectl get namespace forge-system &>/dev/null; then
    echo "✗ FAILED: forge-system namespace not found"
    echo "  Please deploy Forge first with 'make install'"
    exit 1
fi
echo "✓ forge-system namespace exists"

# Check controller pods
echo "Checking controller pods..."
PODS=$(kubectl get pods -n forge-system -l app=forge-controller -o jsonpath='{.items[*].metadata.name}')
if [ -z "$PODS" ]; then
    echo "✗ FAILED: No controller pods found"
    exit 1
fi
echo "✓ Controller pods found: $PODS"

# Check pod status
for POD in $PODS; do
    STATUS=$(kubectl get pod "$POD" -n forge-system -o jsonpath='{.status.phase}')
    READY=$(kubectl get pod "$POD" -n forge-system -o jsonpath='{.status.containerStatuses[0].ready}')

    if [ "$STATUS" != "Running" ]; then
        echo "✗ FAILED: Pod $POD is not running (status: $STATUS)"
        exit 1
    fi

    if [ "$READY" != "true" ]; then
        echo "✗ FAILED: Pod $POD is not ready"
        exit 1
    fi

    echo "✓ Pod $POD is running and ready"
done

# Port forward for testing
echo ""
echo "Setting up port forwards..."
kubectl port-forward -n forge-system svc/forge-controller 8081:8081 >/dev/null 2>&1 &
PF_HEALTH=$!
kubectl port-forward -n forge-system svc/forge-controller 8080:8080 >/dev/null 2>&1 &
PF_METRICS=$!

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up port forwards..."
    kill $PF_HEALTH $PF_METRICS 2>/dev/null || true
}
trap cleanup EXIT

# Wait for port forwards to be ready
sleep 2

# Test health endpoint
echo ""
echo "Testing health endpoint..."
HEALTH_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8081/healthz)
if [ "$HEALTH_RESPONSE" != "200" ]; then
    echo "✗ FAILED: Health endpoint returned $HEALTH_RESPONSE (expected 200)"
    exit 1
fi
echo "✓ Health endpoint returned 200 OK"

# Test readiness endpoint
echo ""
echo "Testing readiness endpoint..."
READINESS_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8081/readyz)
if [ "$READINESS_RESPONSE" != "200" ]; then
    echo "✗ FAILED: Readiness endpoint returned $READINESS_RESPONSE (expected 200)"
    exit 1
fi
echo "✓ Readiness endpoint returned 200 Ready"

# Test metrics endpoint
echo ""
echo "Testing metrics endpoint..."
METRICS=$(curl -s http://localhost:8080/metrics)
if ! echo "$METRICS" | grep -q "forge_"; then
    echo "✗ FAILED: Metrics endpoint did not return forge_ metrics"
    exit 1
fi
echo "✓ Metrics endpoint returned Prometheus metrics"

# Count number of forge metrics
METRIC_COUNT=$(echo "$METRICS" | grep -c "^forge_" || true)
echo "  Found $METRIC_COUNT forge_* metrics"

echo ""
echo "=================================="
echo "All health checks passed!"
echo "=================================="
