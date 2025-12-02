#!/bin/bash
set -e

NAMESPACE="${NAMESPACE:-default}"

echo "🧪 Testing Forge with example ZarfPackageJobs..."

# Apply ServiceAccount from examples
echo "👤 Creating ServiceAccount for testing..."
kubectl apply -f examples/service-accounts/simple-test-sa.yaml

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📦 Test: Hello Forge (minimal test package)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Apply hello-forge test from examples
kubectl apply -f examples/zarfpackagejobs/hello-forge-test.yaml

echo "⏳ Waiting for job to be processed..."
sleep 5

echo ""
echo "📊 ZarfPackageJob Status:"
kubectl get zarfpackagejobs -n ${NAMESPACE}

echo ""
echo "🔍 Kubernetes Jobs:"
kubectl get jobs -n ${NAMESPACE}

echo ""
echo "📋 Pod Status:"
kubectl get pods -n ${NAMESPACE}

# Check for failures and provide diagnostics
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔍 Diagnostic Information"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check ZarfPackageJob phase
PHASE=$(kubectl get zarfpackagejob hello-forge-test -n ${NAMESPACE} -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
echo "Job Phase: ${PHASE}"

if [ "$PHASE" = "Failed" ] || [ "$PHASE" = "Unknown" ]; then
  echo ""
  echo "⚠️  Job is in ${PHASE} state. Showing detailed information..."
  echo ""

  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "📝 ZarfPackageJob Description:"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  kubectl describe zarfpackagejob hello-forge-test -n ${NAMESPACE}

  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "🎯 Controller Logs (last 50 lines):"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  kubectl logs -n forge-system -l app.kubernetes.io/name=forge-controller --tail=50 2>/dev/null || echo "No controller logs available"
fi

# Check pod status for detailed errors
POD_STATUS=$(kubectl get pods -n ${NAMESPACE} -l forge.forge.dev/package=hello-forge-test -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "Unknown")

if [ "$POD_STATUS" = "Pending" ] || [ "$POD_STATUS" = "Failed" ] || [ "$POD_STATUS" = "Unknown" ]; then
  echo ""
  echo "⚠️  Pod is in ${POD_STATUS} state. Checking for issues..."
  echo ""

  # Get pod name
  POD_NAME=$(kubectl get pods -n ${NAMESPACE} -l forge.forge.dev/package=hello-forge-test -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

  if [ -n "$POD_NAME" ]; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📝 Pod Description (${POD_NAME}):"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    kubectl describe pod ${POD_NAME} -n ${NAMESPACE}

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🔍 Pod Events:"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    kubectl get events -n ${NAMESPACE} --field-selector involvedObject.name=${POD_NAME} --sort-by='.lastTimestamp'

    # Check for ImagePullBackOff or ErrImagePull
    if kubectl describe pod ${POD_NAME} -n ${NAMESPACE} | grep -q "ImagePullBackOff\|ErrImagePull"; then
      echo ""
      echo "❌ IMAGE PULL ERROR DETECTED"
      echo ""
      echo "The Zarf CLI image is missing from the cluster."
      echo "This usually means the image wasn't loaded properly."
      echo ""
      echo "To fix:"
      echo "  1. Check if zarf image exists: podman images | grep zarf"
      echo "  2. Reload the image:"
      echo "     podman save localhost/zarf:v0.66.0 -o /tmp/zarf-cli.tar"
      echo "     kind load image-archive /tmp/zarf-cli.tar --name forge-demo"
      echo "     rm /tmp/zarf-cli.tar"
      echo "  3. Delete the failed job and pod:"
      echo "     kubectl delete zarfpackagejob hello-forge-test -n ${NAMESPACE}"
      echo "  4. Re-run this test script"
    fi
  fi
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Test job applied and diagnostics complete!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📝 Useful commands:"
echo ""
echo "Watch job progress:"
echo "  kubectl get zarfpackagejobs -n ${NAMESPACE} -w"
echo ""
echo "View ZarfPackageJob details:"
echo "  kubectl describe zarfpackagejob hello-forge-test -n ${NAMESPACE}"
echo ""
echo "View controller logs:"
echo "  kubectl logs -n forge-system -l app.kubernetes.io/name=forge-controller -f"
echo ""
echo "View job pod logs (once pod is running):"
echo "  kubectl logs -n ${NAMESPACE} -l forge.forge.dev/package=hello-forge-test -f"
echo ""
echo "Delete test resources:"
echo "  kubectl delete zarfpackagejob hello-forge-test -n ${NAMESPACE}"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "💡 Other example jobs available in examples/zarfpackagejobs/:"
echo "   - build-only-git.yaml"
echo "   - build-publish-deploy-git.yaml"
echo "   - deploy-from-oci.yaml"
echo "   - publish-s3-to-oci.yaml"
echo ""
