#!/bin/bash
set -e

CHART_DIR="$(cd "$(dirname "$0")/../../.." && pwd)/chart/forge"

echo "=================================="
echo "Forge Controller HA Validation"
echo "=================================="
echo ""

FAILURES=0

# -------------------------------------------------------
# Part 1: Live cluster checks (if controller is deployed)
# -------------------------------------------------------
if kubectl get namespace forge-system &>/dev/null; then
    echo "--- Live Cluster Checks ---"
    echo ""

    # Check leader election lease exists
    echo "Checking leader election lease..."
    if kubectl get lease forge-controller-lock -n forge-system &>/dev/null; then
        HOLDER=$(kubectl get lease forge-controller-lock -n forge-system -o jsonpath='{.spec.holderIdentity}' 2>/dev/null || echo "")
        if [ -n "$HOLDER" ]; then
            echo "  Leader election lease exists, holder: $HOLDER"
        else
            echo "  Leader election lease exists but has no holder (standby mode or starting up)"
        fi
    else
        echo "  Leader election lease not found (controller may not have leader election enabled)"
    fi
    echo ""
else
    echo "Skipping live cluster checks (forge-system namespace not found)"
    echo ""
fi

# -------------------------------------------------------
# Part 2: Helm template rendering tests
# -------------------------------------------------------
echo "--- Helm Template Rendering Tests ---"
echo ""

if ! command -v helm &>/dev/null; then
    echo "SKIP: helm CLI not available, skipping template tests"
    exit 0
fi

# Test 1: Default values - LE enabled, no PDB (1 replica), no workers flag
echo "Test 1: Default values..."
DEFAULT_OUTPUT=$(helm template test-forge "$CHART_DIR" 2>&1)

if echo "$DEFAULT_OUTPUT" | grep -q -- "--enable-leader-election"; then
    echo "  --enable-leader-election present (default)"
else
    echo "  FAIL: --enable-leader-election missing in default template"
    FAILURES=$((FAILURES + 1))
fi

if echo "$DEFAULT_OUTPUT" | grep -q -- "--leader-election-lease-duration=15s"; then
    echo "  --leader-election-lease-duration=15s present"
else
    echo "  FAIL: --leader-election-lease-duration missing"
    FAILURES=$((FAILURES + 1))
fi

if echo "$DEFAULT_OUTPUT" | grep -q -- "--leader-election-namespace=forge-system"; then
    echo "  --leader-election-namespace=forge-system present"
else
    echo "  FAIL: --leader-election-namespace missing"
    FAILURES=$((FAILURES + 1))
fi

# Default has 1 replica, so no controller PDB
if echo "$DEFAULT_OUTPUT" | grep -A2 "kind: PodDisruptionBudget" | grep -q "test-forge-controller"; then
    echo "  FAIL: Controller PDB should not exist with 1 replica"
    FAILURES=$((FAILURES + 1))
else
    echo "  No controller PDB with 1 replica"
fi

if echo "$DEFAULT_OUTPUT" | grep -q -- '"--workers='; then
    echo "  FAIL: --workers flag should not appear with default value (2)"
    FAILURES=$((FAILURES + 1))
else
    echo "  No --workers flag with default value"
fi
echo ""

# Test 2: replicaCount=3 - PDB created, anti-affinity present
echo "Test 2: replicaCount=3..."
HA_OUTPUT=$(helm template test-forge "$CHART_DIR" --set controller.replicaCount=3 2>&1)

if echo "$HA_OUTPUT" | grep -A2 "kind: PodDisruptionBudget" | grep -q "test-forge-controller"; then
    echo "  Controller PDB created with 3 replicas"
else
    echo "  FAIL: Controller PDB missing with 3 replicas"
    FAILURES=$((FAILURES + 1))
fi

if echo "$HA_OUTPUT" | grep -q "podAntiAffinity"; then
    echo "  Pod anti-affinity present"
else
    echo "  FAIL: Pod anti-affinity missing with 3 replicas"
    FAILURES=$((FAILURES + 1))
fi

if echo "$HA_OUTPUT" | grep -q "kubernetes.io/hostname"; then
    echo "  Anti-affinity uses hostname topology key"
else
    echo "  FAIL: Anti-affinity hostname topology key missing"
    FAILURES=$((FAILURES + 1))
fi
echo ""

# Test 3: leaderElection.enabled=false - no LE flags
echo "Test 3: Leader election disabled..."
NO_LE_OUTPUT=$(helm template test-forge "$CHART_DIR" --set leaderElection.enabled=false 2>&1)

if echo "$NO_LE_OUTPUT" | grep -q -- "--enable-leader-election"; then
    echo "  FAIL: --enable-leader-election should not be present when disabled"
    FAILURES=$((FAILURES + 1))
else
    echo "  No --enable-leader-election flag when disabled"
fi

if echo "$NO_LE_OUTPUT" | grep -q -- "--leader-election-lease-duration"; then
    echo "  FAIL: LE parameter flags should not be present when disabled"
    FAILURES=$((FAILURES + 1))
else
    echo "  No LE parameter flags when disabled"
fi
echo ""

# Test 4: Custom workers value
echo "Test 4: Custom workers=4..."
WORKERS_OUTPUT=$(helm template test-forge "$CHART_DIR" --set controller.workers=4 2>&1)

if echo "$WORKERS_OUTPUT" | grep -q -- '"--workers=4"'; then
    echo "  --workers=4 flag present"
else
    echo "  FAIL: --workers=4 flag missing"
    FAILURES=$((FAILURES + 1))
fi
echo ""

# Test 5: Custom LE timing
echo "Test 5: Custom LE timing..."
LE_TIMING_OUTPUT=$(helm template test-forge "$CHART_DIR" \
    --set leaderElection.leaseDuration=20s \
    --set leaderElection.renewDeadline=15s \
    --set leaderElection.retryPeriod=3s 2>&1)

if echo "$LE_TIMING_OUTPUT" | grep -q -- "--leader-election-lease-duration=20s"; then
    echo "  Custom lease duration passed"
else
    echo "  FAIL: Custom lease duration not passed"
    FAILURES=$((FAILURES + 1))
fi

if echo "$LE_TIMING_OUTPUT" | grep -q -- "--leader-election-renew-deadline=15s"; then
    echo "  Custom renew deadline passed"
else
    echo "  FAIL: Custom renew deadline not passed"
    FAILURES=$((FAILURES + 1))
fi

if echo "$LE_TIMING_OUTPUT" | grep -q -- "--leader-election-retry-period=3s"; then
    echo "  Custom retry period passed"
else
    echo "  FAIL: Custom retry period not passed"
    FAILURES=$((FAILURES + 1))
fi
echo ""

# Test 6: Concurrency flags
echo "Test 6: Concurrency limit flags..."
CONCURRENCY_OUTPUT=$(helm template test-forge "$CHART_DIR" \
    --set controller.concurrency.maxJobsPerNamespace=5 \
    --set controller.concurrency.maxJobsGlobal=20 2>&1)

if echo "$CONCURRENCY_OUTPUT" | grep -q -- "--max-concurrent-jobs-per-namespace=5"; then
    echo "  Per-namespace concurrency flag present"
else
    echo "  FAIL: Per-namespace concurrency flag missing"
    FAILURES=$((FAILURES + 1))
fi

if echo "$CONCURRENCY_OUTPUT" | grep -q -- "--max-concurrent-jobs-global=20"; then
    echo "  Global concurrency flag present"
else
    echo "  FAIL: Global concurrency flag missing"
    FAILURES=$((FAILURES + 1))
fi
echo ""

# Summary
echo "=================================="
if [ $FAILURES -gt 0 ]; then
    echo "HA validation: $FAILURES failure(s)"
    echo "=================================="
    exit 1
else
    echo "All HA validation checks passed!"
    echo "=================================="
    exit 0
fi
