# Forge End-to-End Tests

Simple, automated tests for Forge functionality that work on both local Kind clusters and production clusters.

## Test Suite

| Test | Description | Duration | Prerequisites |
|------|-------------|----------|---------------|
| **01-simple-build** | Build Zarf package from Git | ~2 min | None (public repo) |
| **02-simple-deploy** | Deploy Zarf package to cluster | ~3 min | Sufficient cluster resources |
| **03-health-check** | Verify controller health/metrics | ~10 sec | Controller deployed |

## Quick Start

### Run All Tests (Automated)

```bash
# In Kind cluster (creates cluster, deploys, tests, cleans up)
make e2e-test

# Against existing cluster
make e2e-test-existing
```

### Run Individual Tests

Each test directory contains:
- `README.md` - Detailed test description and manual steps
- YAML manifests - Kubernetes resources for the test
- `test.sh` (if applicable) - Automated test script

Example:
```bash
cd 01-simple-build
kubectl apply -f serviceaccount.yaml
kubectl apply -f zarfpackagejob.yaml
kubectl get zarfpackagejobs -w
```

## Test Design Principles

1. **Portable**: Tests work on both Kind and production clusters
2. **Minimal**: Each test focuses on one piece of functionality
3. **Self-contained**: All required resources included
4. **Documented**: Clear success criteria and troubleshooting steps
5. **Automated**: Can be run via `make e2e-test`

## Prerequisites

### For Kind Clusters

- `kind` installed
- `kubectl` installed
- Docker or Podman running

### For Production Clusters

- Valid kubeconfig with cluster access
- Sufficient RBAC permissions (create ServiceAccounts, Jobs, etc.)
- Forge controller already deployed

## Running Tests

### Against Kind Cluster (Recommended)

```bash
# Full workflow: create cluster, deploy Forge, run tests, cleanup
make e2e-test

# Keep cluster for debugging
make e2e-test-keep

# Manual steps
make kind-create
make kind-load
make install
./run-all-tests.sh
```

### Against Production Cluster

```bash
# Ensure controller is deployed first
helm install forge ./chart/forge -n forge-system --create-namespace

# Run tests
./run-all-tests.sh

# Or run individual tests
cd 01-simple-build && kubectl apply -f .
cd 02-simple-deploy && kubectl apply -f .
cd 03-health-check && ./test.sh
```

## Troubleshooting

### Test 01 (Simple Build) Fails

**Symptom**: Job fails or times out

**Check**:
```bash
kubectl get zarfpackagejob test-simple-build -o yaml
kubectl logs -l forge.dev/package=test-simple-build
```

**Common Issues**:
- Git repository inaccessible (network/firewall)
- Insufficient CPU/memory resources
- ServiceAccount policy too restrictive

### Test 02 (Simple Deploy) Fails

**Symptom**: Package doesn't deploy

**Check**:
```bash
kubectl get zarfpackagejob test-simple-deploy -o yaml
kubectl logs -l forge.dev/package=test-simple-deploy
kubectl get pods -n zarf
```

**Common Issues**:
- Cluster doesn't have required resources
- RBAC permissions insufficient for deployment
- Package incompatible with cluster version

### Test 03 (Health Check) Fails

**Symptom**: Endpoints return errors

**Check**:
```bash
kubectl get pods -n forge-system
kubectl logs -n forge-system -l app=forge-controller
```

**Common Issues**:
- Controller not fully started (wait 30s)
- Port forwarding failed (check firewall)
- Service not created correctly

## Cleanup

```bash
# Delete test resources
kubectl delete zarfpackagejobs test-simple-build test-simple-deploy
kubectl delete serviceaccount test-builder test-deployer
kubectl delete namespace zarf --ignore-not-found=true

# Delete Kind cluster
make kind-delete

# Uninstall Forge
helm uninstall forge -n forge-system
```

## Adding New Tests

1. Create new directory: `tests/e2e/NN-test-name/`
2. Add `README.md` with test description
3. Add required YAML manifests
4. Add `test.sh` if test needs automation
5. Update `run-all-tests.sh` to include new test
6. Update this README's test table

### Test Template

```markdown
# Test NN: Test Name

## Description
Brief description of what this test validates

## Prerequisites
- Requirement 1
- Requirement 2

## Expected Behavior
1. Step 1 happens
2. Step 2 happens
3. Final outcome

## Running the Test
\`\`\`bash
kubectl apply -f .
\`\`\`

## Success Criteria
- Criterion 1
- Criterion 2
```
