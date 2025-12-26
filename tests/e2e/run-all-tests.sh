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
    kubectl delete zarfpackagejobs test-simple-build test-simple-deploy --ignore-not-found=true
    kubectl delete serviceaccount test-builder test-deployer --ignore-not-found=true
    kubectl delete namespace zarf --ignore-not-found=true
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

# Test 2: Simple Build
echo "=================================="
echo "Test 02: Simple Build"
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
        kubectl logs -l forge.dev/package=test-simple-build --tail=50
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

# Test 3: Simple Deploy
echo "=================================="
echo "Test 03: Simple Deploy"
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
        kubectl logs -l forge.dev/package=test-simple-deploy --tail=50
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

# Print summary
echo "=================================="
echo "Test Suite Summary"
echo "=================================="
echo "Total tests:  $TESTS_TOTAL"
echo -e "${GREEN}Passed:       $TESTS_PASSED${NC}"
if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Failed:       $TESTS_FAILED${NC}"
else
    echo -e "Failed:       $TESTS_FAILED"
fi
echo ""

if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
else
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
fi
