#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Test results
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

echo "=================================="
echo "Forge E2E Test Suite"
echo "=================================="
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up test resources..."

    # Zarf resources
    for job in test-simple-build test-simple-deploy; do
        kubectl delete zarfpackagejob "$job" -n default --ignore-not-found=true 2>/dev/null || true
        echo "zarfpackagejob.forge.dev \"$job\" deleted from default namespace" 2>/dev/null || true
    done

    # UDS resources
    for job in test-uds-create test-uds-deploy; do
        kubectl delete udsbundlejob "$job" -n default --ignore-not-found=true 2>/dev/null || true
        echo "udsbundlejob.forge.dev \"$job\" deleted from default namespace" 2>/dev/null || true
    done

    # Service accounts
    for sa in test-builder test-deployer test-uds-creator test-uds-deployer; do
        kubectl delete serviceaccount "$sa" -n default --ignore-not-found=true 2>/dev/null || true
        echo "serviceaccount \"$sa\" deleted from default namespace" 2>/dev/null || true
    done

    # Deployed namespaces from tests
    kubectl delete namespace headlamp --ignore-not-found=true 2>/dev/null || true
    kubectl delete namespace zarf --ignore-not-found=true 2>/dev/null || true

    echo "Cleanup complete"
}

# Register cleanup on exit
trap cleanup EXIT

# Test 1: Health Check
echo "=================================="
echo "Test 01: Health Check"
echo "=================================="
cd 03-health-check
if ./test.sh; then
    echo -e "${GREEN}✓ Test 01 PASSED${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}✗ Test 01 FAILED${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
TESTS_TOTAL=$((TESTS_TOTAL + 1))
cd ..
echo ""

# Test 2: Simple Zarf Build
echo "=================================="
echo "Test 02: Simple Zarf Build"
echo "=================================="
cd 01-simple-build

# Apply resources
kubectl apply -f serviceaccount.yaml
kubectl apply -f zarfpackagejob.yaml

# Wait for completion (timeout 5 minutes)
echo "Waiting for build to complete (timeout: 5 minutes)..."
TIMEOUT=300
ELAPSED=0
while [ $ELAPSED -lt $TIMEOUT ]; do
    PHASE=$(kubectl get zarfpackagejob test-simple-build -o jsonpath='{.status.phase}' 2>/dev/null || echo "")

    if [ "$PHASE" = "Completed" ]; then
        echo -e "${GREEN}✓ Test 02 PASSED - Build completed successfully${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        break
    elif [ "$PHASE" = "Failed" ]; then
        echo -e "${RED}✗ Test 02 FAILED - Build failed${NC}"
        kubectl logs -l forge.dev/package=test-simple-build --tail=50 2>/dev/null || true
        TESTS_FAILED=$((TESTS_FAILED + 1))
        break
    fi

    sleep 5
    ELAPSED=$((ELAPSED + 5))
    echo -n "."
done

if [ $ELAPSED -ge $TIMEOUT ]; then
    echo -e "${RED}✗ Test 02 FAILED - Timeout${NC}"
    kubectl get zarfpackagejob test-simple-build -o yaml
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

TESTS_TOTAL=$((TESTS_TOTAL + 1))
cd ..
echo ""

# Test 3: Simple Zarf Deploy
echo "=================================="
echo "Test 03: Simple Zarf Deploy"
echo "=================================="
cd 02-simple-deploy

# Apply resources
kubectl apply -f serviceaccount.yaml
kubectl apply -f zarfpackagejob.yaml

# Wait for completion (timeout 5 minutes)
echo "Waiting for deployment to complete (timeout: 5 minutes)..."
TIMEOUT=300
ELAPSED=0
while [ $ELAPSED -lt $TIMEOUT ]; do
    PHASE=$(kubectl get zarfpackagejob test-simple-deploy -o jsonpath='{.status.phase}' 2>/dev/null || echo "")

    if [ "$PHASE" = "Completed" ]; then
        echo -e "${GREEN}✓ Test 03 PASSED - Deployment completed successfully${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        break
    elif [ "$PHASE" = "Failed" ]; then
        echo -e "${RED}✗ Test 03 FAILED - Deployment failed${NC}"
        kubectl logs -l forge.dev/package=test-simple-deploy --tail=50 2>/dev/null || true
        TESTS_FAILED=$((TESTS_FAILED + 1))
        break
    fi

    sleep 5
    ELAPSED=$((ELAPSED + 5))
    echo -n "."
done

if [ $ELAPSED -ge $TIMEOUT ]; then
    echo -e "${RED}✗ Test 03 FAILED - Timeout${NC}"
    kubectl get zarfpackagejob test-simple-deploy -o yaml
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

TESTS_TOTAL=$((TESTS_TOTAL + 1))
cd ..
echo ""

# Test 4: UDS Bundle Create
echo "=================================="
echo "Test 04: UDS Bundle Create"
echo "=================================="
cd 04-uds-create

# Apply resources
kubectl apply -f serviceaccount.yaml
kubectl apply -f udsbundlejob.yaml

# Wait for completion (timeout 5 minutes)
echo "Waiting for UDS bundle create to complete (timeout: 5 minutes)..."
TIMEOUT=300
ELAPSED=0
while [ $ELAPSED -lt $TIMEOUT ]; do
    PHASE=$(kubectl get udsbundlejob test-uds-create -o jsonpath='{.status.phase}' 2>/dev/null || echo "")

    if [ "$PHASE" = "Completed" ]; then
        echo -e "${GREEN}✓ Test 04 PASSED - UDS bundle created successfully${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        break
    elif [ "$PHASE" = "Failed" ]; then
        echo -e "${RED}✗ Test 04 FAILED - UDS bundle creation failed${NC}"
        kubectl logs -l forge.dev/bundle=test-uds-create --tail=50 2>/dev/null || true
        TESTS_FAILED=$((TESTS_FAILED + 1))
        break
    fi

    sleep 5
    ELAPSED=$((ELAPSED + 5))
    echo -n "."
done

if [ $ELAPSED -ge $TIMEOUT ]; then
    echo -e "${RED}✗ Test 04 FAILED - Timeout${NC}"
    kubectl get udsbundlejob test-uds-create -o yaml
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

TESTS_TOTAL=$((TESTS_TOTAL + 1))
cd ..
echo ""

# Test 5: UDS Bundle Deploy
echo "=================================="
echo "Test 05: UDS Bundle Deploy"
echo "=================================="
cd 05-uds-deploy

# Apply resources
kubectl apply -f serviceaccount.yaml
kubectl apply -f udsbundlejob.yaml

# Wait for completion (timeout 8 minutes - deploy takes longer)
echo "Waiting for UDS bundle deploy to complete (timeout: 8 minutes)..."
TIMEOUT=480
ELAPSED=0
while [ $ELAPSED -lt $TIMEOUT ]; do
    PHASE=$(kubectl get udsbundlejob test-uds-deploy -o jsonpath='{.status.phase}' 2>/dev/null || echo "")

    if [ "$PHASE" = "Completed" ]; then
        echo -e "${GREEN}✓ Test 05 PASSED - UDS bundle deployed successfully${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        break
    elif [ "$PHASE" = "Failed" ]; then
        echo -e "${RED}✗ Test 05 FAILED - UDS bundle deployment failed${NC}"
        kubectl logs -l forge.dev/bundle=test-uds-deploy --tail=50 2>/dev/null || true
        TESTS_FAILED=$((TESTS_FAILED + 1))
        break
    fi

    sleep 5
    ELAPSED=$((ELAPSED + 5))
    echo -n "."
done

if [ $ELAPSED -ge $TIMEOUT ]; then
    echo -e "${RED}✗ Test 05 FAILED - Timeout${NC}"
    kubectl get udsbundlejob test-uds-deploy -o yaml
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

TESTS_TOTAL=$((TESTS_TOTAL + 1))
cd ..
echo ""

# Print summary
echo "=================================="
echo "Test Suite Summary"
echo "=================================="
echo "Total tests:  $TESTS_TOTAL"
echo -e "${GREEN}Passed:       $TESTS_PASSED${NC}"
if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Failed:       $TESTS_FAILED${NC}"
else
    echo "Failed:       $TESTS_FAILED"
fi
echo ""

if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
else
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
fi
