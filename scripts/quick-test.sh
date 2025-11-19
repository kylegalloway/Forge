#!/bin/bash
set -e

# Quick Test Script - Verify controller is working with a simple test
# Usage: ./scripts/quick-test.sh [namespace]

NAMESPACE="${1:-default}"
TEST_NAME="quicktest-$(date +%s)"

echo "Running quick test of ScriptRunner controller..."
echo ""

# Create test ScriptRunner
cat <<EOF | kubectl apply -f -
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: ${TEST_NAME}
  namespace: ${NAMESPACE}
spec:
  inputs:
    test: "quick-test"
    timestamp: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
EOF

echo "✓ Created ScriptRunner: ${TEST_NAME}"
echo ""

# Wait a few seconds
echo "Waiting for Job creation..."
sleep 5

# Check if Job was created
JOB_NAME=$(kubectl get scriptrunner ${TEST_NAME} -n "${NAMESPACE}" -o jsonpath='{.status.jobName}' 2>/dev/null || echo "")

if [ -z "$JOB_NAME" ]; then
    echo "✗ FAILED: No Job was created"
    echo ""
    echo "ScriptRunner status:"
    kubectl get scriptrunner ${TEST_NAME} -n "${NAMESPACE}" -o yaml
    echo ""
    echo "Controller logs:"
    kubectl logs -n scriptrunner-system -l app=scriptrunner-controller --tail=20
    kubectl delete scriptrunner ${TEST_NAME} -n "${NAMESPACE}" --ignore-not-found=true
    exit 1
fi

echo "✓ Job created: ${JOB_NAME}"
echo ""

# Wait for Job to complete
echo "Waiting for Job to complete (max 60s)..."
if kubectl wait --for=condition=complete --timeout=60s job/"${JOB_NAME}" -n "${NAMESPACE}" 2>/dev/null; then
    echo "✓ Job completed successfully"
else
    echo "✗ Job did not complete in time or failed"
    echo ""
    echo "Job status:"
    kubectl get job "${JOB_NAME}" -n "${NAMESPACE}" -o yaml
    echo ""
    echo "Pod logs:"
    kubectl logs job/"${JOB_NAME}" -n "${NAMESPACE}" 2>/dev/null || echo "No logs available yet"
    kubectl delete scriptrunner ${TEST_NAME} -n "${NAMESPACE}" --ignore-not-found=true
    exit 1
fi

echo ""
echo "Job output:"
echo "----------"
kubectl logs job/"${JOB_NAME}" -n "${NAMESPACE}"
echo "----------"
echo ""

# Check ScriptRunner status
echo "ScriptRunner status:"
kubectl get scriptrunner ${TEST_NAME} -n "${NAMESPACE}" -o jsonpath='{.status}' | jq '.' 2>/dev/null || kubectl get scriptrunner ${TEST_NAME} -n "${NAMESPACE}" -o jsonpath='{.status}'
echo ""
echo ""

# Cleanup
echo "Cleaning up..."
kubectl delete scriptrunner ${TEST_NAME} -n "${NAMESPACE}"

echo ""
echo "=================================="
echo "✓ Quick test PASSED"
echo "=================================="
echo ""
echo "Controller is working correctly!"
echo ""
