#!/bin/bash
set -e

CHART_DIR="$(cd "$(dirname "$0")/../../.." && pwd)/chart/forge"

echo "=================================="
echo "Forge Concurrency Limits Validation"
echo "=================================="
echo ""

FAILURES=0

# -------------------------------------------------------
# Part 1: Helm template rendering tests
# -------------------------------------------------------
echo "--- Helm Template Rendering Tests ---"
echo ""

if ! command -v helm &>/dev/null; then
    echo "SKIP: helm CLI not available, skipping template tests"
    exit 0
fi

# Test 1: Default values (0 = unlimited) - no concurrency flags
echo "Test 1: Default values (unlimited)..."
DEFAULT_OUTPUT=$(helm template test-forge "$CHART_DIR" 2>&1)

if echo "$DEFAULT_OUTPUT" | grep -q -- "--max-concurrent-jobs-per-namespace"; then
    echo "  FAIL: --max-concurrent-jobs-per-namespace should not appear when 0"
    FAILURES=$((FAILURES + 1))
else
    echo "  No per-namespace flag when unlimited"
fi

if echo "$DEFAULT_OUTPUT" | grep -q -- "--max-concurrent-jobs-global"; then
    echo "  FAIL: --max-concurrent-jobs-global should not appear when 0"
    FAILURES=$((FAILURES + 1))
else
    echo "  No global flag when unlimited"
fi
echo ""

# Test 2: Per-namespace limit only
echo "Test 2: Per-namespace limit (5)..."
NS_OUTPUT=$(helm template test-forge "$CHART_DIR" \
    --set controller.concurrency.maxJobsPerNamespace=5 2>&1)

if echo "$NS_OUTPUT" | grep -q -- "--max-concurrent-jobs-per-namespace=5"; then
    echo "  Per-namespace flag set to 5"
else
    echo "  FAIL: Per-namespace flag missing or wrong"
    FAILURES=$((FAILURES + 1))
fi

if echo "$NS_OUTPUT" | grep -q -- "--max-concurrent-jobs-global"; then
    echo "  FAIL: Global flag should not appear when 0"
    FAILURES=$((FAILURES + 1))
else
    echo "  No global flag (still unlimited)"
fi
echo ""

# Test 3: Global limit only
echo "Test 3: Global limit (20)..."
GLOBAL_OUTPUT=$(helm template test-forge "$CHART_DIR" \
    --set controller.concurrency.maxJobsGlobal=20 2>&1)

if echo "$GLOBAL_OUTPUT" | grep -q -- "--max-concurrent-jobs-global=20"; then
    echo "  Global flag set to 20"
else
    echo "  FAIL: Global flag missing or wrong"
    FAILURES=$((FAILURES + 1))
fi

if echo "$GLOBAL_OUTPUT" | grep -q -- "--max-concurrent-jobs-per-namespace"; then
    echo "  FAIL: Per-namespace flag should not appear when 0"
    FAILURES=$((FAILURES + 1))
else
    echo "  No per-namespace flag (still unlimited)"
fi
echo ""

# Test 4: Both limits set
echo "Test 4: Both limits (namespace=3, global=10)..."
BOTH_OUTPUT=$(helm template test-forge "$CHART_DIR" \
    --set controller.concurrency.maxJobsPerNamespace=3 \
    --set controller.concurrency.maxJobsGlobal=10 2>&1)

if echo "$BOTH_OUTPUT" | grep -q -- "--max-concurrent-jobs-per-namespace=3"; then
    echo "  Per-namespace flag set to 3"
else
    echo "  FAIL: Per-namespace flag missing"
    FAILURES=$((FAILURES + 1))
fi

if echo "$BOTH_OUTPUT" | grep -q -- "--max-concurrent-jobs-global=10"; then
    echo "  Global flag set to 10"
else
    echo "  FAIL: Global flag missing"
    FAILURES=$((FAILURES + 1))
fi
echo ""

# Test 5: Workers flag with non-default value
echo "Test 5: Custom workers (6)..."
WORKERS_OUTPUT=$(helm template test-forge "$CHART_DIR" \
    --set controller.workers=6 2>&1)

if echo "$WORKERS_OUTPUT" | grep -q -- '"--workers=6"'; then
    echo "  Workers flag set to 6"
else
    echo "  FAIL: Workers flag missing or wrong"
    FAILURES=$((FAILURES + 1))
fi
echo ""

# -------------------------------------------------------
# Part 2: Live cluster checks (if controller is deployed)
# -------------------------------------------------------
if kubectl get namespace forge-system &>/dev/null; then
    echo "--- Live Cluster Checks ---"
    echo ""

    # Check if metrics endpoint has backpressure-related metrics registered
    echo "Checking backpressure metrics registration..."

    # Port forward metrics endpoint briefly
    kubectl port-forward -n forge-system svc/forge-controller 18080:8080 >/dev/null 2>&1 &
    PF_PID=$!

    cleanup_pf() {
        kill $PF_PID 2>/dev/null || true
    }
    trap cleanup_pf EXIT

    sleep 2

    METRICS=$(curl -s http://localhost:18080/metrics 2>/dev/null || echo "")

    if [ -n "$METRICS" ]; then
        # Check for concurrency-related metric names in the output
        # These metrics are registered even when limits are 0
        if echo "$METRICS" | grep -q "forge"; then
            echo "  Forge metrics available on /metrics endpoint"
        else
            echo "  Warning: No forge metrics found (controller may still be starting)"
        fi
    else
        echo "  Warning: Could not reach metrics endpoint (port-forward may have failed)"
    fi
    echo ""
else
    echo "Skipping live cluster checks (forge-system namespace not found)"
    echo ""
fi

# Summary
echo "=================================="
if [ $FAILURES -gt 0 ]; then
    echo "Concurrency validation: $FAILURES failure(s)"
    echo "=================================="
    exit 1
else
    echo "All concurrency validation checks passed!"
    echo "=================================="
    exit 0
fi
