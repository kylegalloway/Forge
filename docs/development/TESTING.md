# Testing Guide

Forge includes comprehensive testing at multiple levels to ensure reliability and correctness.

## Test Categories

### 1. Unit Tests

Unit tests cover individual packages and functions with high coverage.

```bash
# Run all unit tests with coverage
make test

# View HTML coverage report
make test-coverage

# Run unit tests only (fast, no integration)
make test-unit
```

**Coverage by Package:**

| Package | Coverage | Key Tests |
|---------|----------|-----------|
| pkg/actions | 82.4% | Build, Publish, Deploy handlers (Zarf) |
| pkg/actions/uds | 82.7% | Create, Publish, Deploy handlers (UDS) |
| pkg/destinations | 92.5% | S3, OCI, Local destinations |
| pkg/sources | 100% | Git, S3, OCI, Local sources |
| pkg/policy | 84.0% | Policy engine, webhook validation |
| pkg/credentials | 100% | Secret extraction, mounting |
| pkg/telemetry | 69.2% | Metrics, tracing, OTel |
| pkg/controller | 62.1% | Zarf reconciliation, event handling |
| pkg/controller (UDS) | 65.3% | UDS reconciliation, job monitoring |
| pkg/webhook | 82.1% | Admission webhook validation |
| pkg/attestation | 71.5% | Storage backends, SLSA provenance |
| pkg/constants | 100% | Constant definitions (trivial) |

**Overall Coverage**: ~52% (improved from 48.3%)

### 2. YAML Validation Tests

Validates all Kubernetes manifests for correctness.

```bash
# Run YAML validation tests
make test-validation
```

Tests include:

- CRD schema validation
- RBAC manifest correctness
- Sample resource validation
- Network policy validation
- Deployment manifest validation

### 3. End-to-End (E2E) Tests

Tests policy enforcement and workflow validation on a running cluster.

```bash
# Run E2E tests (requires running cluster)
make e2e-test

# Or run directly with custom namespace
./scripts/test-e2e.sh [namespace]
```

**Test Scenarios:**

- ServiceAccount policy creation
- Build-only ZarfPackageJobs
- Policy violation detection (unauthorized actions)
- Policy violation detection (unauthorized repositories)
- Multi-action operations (BuildPublish)
- Metrics endpoint validation
- Status field population

**Prerequisites:**

- Running Kubernetes cluster
- Forge deployed via Helm (or legacy manifests)
- kubectl configured
- helm installed (for new deployments)

### 4. Integration Tests (Kind)

Full end-to-end integration tests using Kind clusters.

```bash
# Run full integration test (creates cluster, deploys, tests, cleans up)
make integration-test

# Keep cluster for debugging
make integration-test-keep

# Run with custom configuration
KIND_CLUSTER_NAME=my-test \
NAMESPACE=my-namespace \
CLEANUP_ON_SUCCESS=false \
./scripts/test-integration-kind.sh
```

**What It Tests:**

1. **Prerequisites Check** - Validates required tools (kind, kubectl, docker, helm)
2. **Cluster Creation** - Creates fresh Kind cluster
3. **Image Build & Load** - Builds controller image and loads into Kind
4. **Forge Deployment** - Deploys Forge via Helm chart
5. **ServiceAccount Policies** - Creates dev and platform ServiceAccounts
6. **Build Action** - Tests authorized Build operation
7. **Policy Enforcement** - Tests unauthorized action blocking
8. **Repository Policies** - Tests unauthorized repository blocking
9. **Multi-Action Workflow** - Tests BuildPublish workflow
10. **Status Fields** - Validates status field population
11. **Health Endpoints** - Tests /healthz endpoint
12. **Metrics Endpoint** - Tests /metrics endpoint with Forge metrics
13. **Controller Logs** - Validates log accessibility
14. **Resource Listing** - Lists all created resources

**Configuration Options:**

| Variable | Default | Description |
|----------|---------|-------------|
| `KIND_CLUSTER_NAME` | `forge-integration-test` | Name of Kind cluster |
| `NAMESPACE` | `forge-system` | Kubernetes namespace |
| `CONTROLLER_IMAGE` | `forge-controller:test` | Controller image tag |
| `CLEANUP_ON_SUCCESS` | `true` | Delete cluster on success |
| `FAIL_FAST` | `false` | Stop on first failure |

**Output:**

- Color-coded test results
- Pass/fail tracking for each test
- Summary with total/passed/failed counts
- Detailed logs for debugging

**Cleanup:**

- Automatically deletes test resources
- Deletes Kind cluster on success (configurable)
- Keeps cluster on failure for debugging

### 5. Registry Integration Tests (Kind + Gitea)

Full integration tests with Gitea OCI registry to validate publish workflows.

```bash
# Run registry integration test (creates cluster with Gitea, tests publish workflows)
make integration-test-registry

# Keep cluster for debugging
make integration-test-registry-keep

# Run with custom configuration
KIND_CLUSTER_NAME=my-test \
GITEA_NAMESPACE=gitea \
CLEANUP_ON_SUCCESS=false \
./scripts/test-integration-registry.sh
```

**What It Tests:**

1. **Prerequisites Check** - Validates required tools (kind, kubectl, helm, docker, jq)
2. **Cluster Creation** - Creates fresh Kind cluster with port mappings
3. **Gitea Deployment** - Deploys Gitea with container registry support via Helm
4. **Gitea Configuration** - Configures users and container registry
5. **Registry Credentials** - Creates Kubernetes secrets for registry auth
6. **Image Build & Load** - Builds controller image and loads into Kind
7. **Forge Deployment** - Deploys Forge via Helm chart
8. **Publish ServiceAccount** - Creates SA with OCI publish permissions
9. **BuildPublish to OCI** - Tests Build→Publish workflow to Gitea registry
10. **Build Then Publish** - Tests separate Build and Publish operations
11. **Full Workflow with Attestation** - Tests BuildPublish with attestation annotations
12. **Policy Enforcement** - Tests unauthorized registry blocking
13. **Package Verification** - Verifies package appears in Gitea registry
14. **Status Tracking** - Validates publish operation status fields
15. **Controller Logs** - Checks logs for publish operations

**Configuration Options:**

| Variable | Default | Description |
|----------|---------|-------------|
| `KIND_CLUSTER_NAME` | `forge-registry-test` | Name of Kind cluster |
| `NAMESPACE` | `forge-system` | Kubernetes namespace |
| `GITEA_NAMESPACE` | `gitea` | Gitea namespace |
| `CONTROLLER_IMAGE` | `forge-controller:test` | Controller image tag |
| `CLEANUP_ON_SUCCESS` | `true` | Delete cluster on success |

**Features:**

- Deploys full Gitea instance with OCI registry support
- Tests real OCI publish workflows (not mocked)
- Validates registry authentication
- Tests attestation tracking annotations
- Verifies packages in registry via API

**Access Gitea UI:**
```bash
# Port-forward to Gitea
kubectl port-forward -n gitea svc/gitea-http 3000:3000

# Open browser to http://localhost:3000
# Login: giteadmin / giteapassword
# Registry user: forgeuser / forgepassword
```

## Running Tests in CI/CD

### GitHub Actions

All tests run automatically in CI:

```yaml
# .github/workflows/ci.yaml

- name: Run Unit Tests

  run: make test

- name: Upload Coverage

  uses: codecov/codecov-action@v3
  with:
    files: ./cover.out
```

### GitLab CI

```yaml
# .gitlab-ci.yml
test:
  stage: test
  script:
    - make test
    - go tool cover -func=cover.out

  coverage: '/^total:.*?(\d+\.\d+)%$/'
```

## Local Development Workflow

### Quick Development Cycle

```bash
# 1. Make code changes
vim pkg/controller/controller.go

# 2. Run unit tests
make test-unit

# 3. Deploy to kind for manual testing
make kind-redeploy

# 4. Run E2E tests
make e2e-test

# 5. View logs
make dev-logs
```

### Full Validation

```bash
# Run complete test suite
make test && make test-validation

# Run integration test with Kind
make integration-test
```

## Debugging Failed Tests

### Unit Test Failures

```bash
# Run specific package tests with verbose output
go test ./pkg/controller -v

# Run specific test
go test ./pkg/controller -v -run TestReconcile

# Generate coverage profile
go test ./pkg/controller -coverprofile=cover.out
go tool cover -html=cover.out
```

### Integration Test Failures

```bash
# Keep cluster for debugging
make integration-test-keep

# Check controller logs
kubectl logs -n forge-system -l app=forge-controller

# Check ZarfPackageJob status
kubectl get zarfpackagejob -A

# Describe failed resource
kubectl describe zarfpackagejob <name> -n forge-system

# Manual cleanup
kind delete cluster --name forge-integration-test
```

### E2E Test Failures

```bash
# Run with verbose output
bash -x ./scripts/test-e2e.sh

# Check controller status
kubectl get pods -n forge-system

# Check created resources
kubectl get zarfpackagejob -A
kubectl get serviceaccount -A

# Check events
kubectl get events -n forge-system --sort-by='.lastTimestamp'
```

## Adding New Tests

### Adding Unit Tests

1. Create test file: `pkg/mypackage/myfile_test.go`
2. Follow existing patterns:

   ```go
   func TestMyFunction(t *testing.T) {
       // Arrange
       input := "test"

       // Act
       result := MyFunction(input)

       // Assert
       if result != expected {
           t.Errorf("expected %v, got %v", expected, result)
       }
   }
   ```
3. Run tests: `make test`

### Adding E2E Test Cases

Edit `scripts/test-e2e.sh`:

```bash
# Test N: Description
echo "Test N: My new test"
echo "-------------------"
# ... test logic ...
echo "✓ Test passed"
```

### Adding Integration Test Cases

Edit `scripts/test-integration-kind.sh`:

```bash
test_my_feature() {
    test_start "My Feature Description"

    # Test logic here

    if [ condition ]; then
        test_pass "Feature works correctly"
    else
        test_fail "Feature failed"
    fi
}

# Add to main() function
main() {
    # ... existing tests ...
    test_my_feature
    # ...
}
```

## Test Best Practices

### Unit Tests

- ✅ Test one thing per test
- ✅ Use table-driven tests for multiple cases
- ✅ Mock external dependencies
- ✅ Aim for >70% coverage on critical paths
- ❌ Don't test implementation details
- ❌ Don't make tests depend on each other

### Integration Tests

- ✅ Test real workflows end-to-end
- ✅ Clean up resources in defer/trap
- ✅ Use unique names with timestamps
- ✅ Check both success and failure cases
- ❌ Don't depend on external services
- ❌ Don't leave resources behind

### E2E Tests

- ✅ Test from user perspective
- ✅ Validate policy enforcement
- ✅ Check status fields and events
- ✅ Test multi-action workflows
- ❌ Don't hardcode resource names
- ❌ Don't skip cleanup

## Coverage Goals

| Category | Target | Current |
|----------|--------|---------|
| Overall | 60% | ~52% ✅ |
| Critical packages | 80% | 82.4-82.7% ✅ |
| Controller | 70% | 62.1-65.3% (approaching) |
| Policy engine | 80% | 84.0% ✅ |
| Sources/Destinations | 90% | 92.5-100% ✅ |
| Attestation | 70% | 71.5% ✅ |
| Constants | N/A | 100% (trivial) |

## Continuous Improvement

### Tracking Progress

- Coverage reports in CI/CD
- Codecov integration for PRs
- Regular coverage reviews

### Quality Metrics

- All PRs require tests
- No decrease in coverage
- Integration tests pass before merge
- E2E tests run on main branch

---

*Last Updated: 2025-12-17*
