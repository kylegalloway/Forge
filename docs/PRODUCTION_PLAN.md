# Production Plan

## Overview

This document outlines the steps required to take Forge from a development prototype to a production-ready system.

## 1. CI/CD Implementation

We will use GitHub Actions for Continuous Integration and Continuous Delivery.

### Workflows

1.  **PR Validation (`.github/workflows/pr-validation.yaml`)**:
    *   Triggers on Pull Requests.
    *   Runs `go fmt`, `go vet`, `golangci-lint`.
    *   Runs unit tests (`go test ./...`).
    *   Builds the controller binary to ensure compilation.

2.  **Release (`.github/workflows/release.yaml`)**:
    *   Triggers on tag creation (`v*`).
    *   Builds and pushes Docker images to `ghcr.io/kylegalloway/forge`.
    *   Generates and publishes release artifacts (CRD YAMLs, deployment YAMLs).
    *   Creates a GitHub Release.

3.  **E2E Testing (`.github/workflows/e2e.yaml`)**:
    *   Triggers on PRs (optional) and main branch commits.
    *   Spins up a Kind cluster.
    *   Installs Forge.
    *   Runs a suite of E2E tests (Build, Publish, Deploy) using real Zarf packages.

## 2. Testing Strategy

### Unit Tests
*   **Coverage Goal**: >80% for core logic (`pkg/policy`, `pkg/actions`, `pkg/sources`).
*   **Mocking**: Use `fake.Clientset` for Kubernetes client mocking.

### Integration Tests
*   **Focus**: Controller reconciliation logic.
*   **Tools**: `envtest` (from controller-runtime) to run a local API server.

### End-to-End (E2E) Tests
*   **Focus**: Full user workflows.
*   **Scenarios**:
    1.  **Simple Build**: Build a package from a public Git repo.
    2.  **Policy Denial**: Verify that a restricted ServiceAccount cannot perform disallowed actions.
    3.  **Full Pipeline**: Build -> Publish -> Deploy.
    4.  **UDS Bundle**: Deploy a UDS bundle.

## 3. Observability & Monitoring

*   **Metrics**: Ensure all critical paths emit Prometheus metrics.
    *   `forge_reconcile_duration_seconds`
    *   `forge_action_total{action="build", status="success"}`
    *   `forge_policy_decisions_total{decision="allow"}`
*   **Dashboards**: Create a standard Grafana dashboard JSON.
*   **Alerts**: Define Prometheus alerting rules for high failure rates or controller downtime.

## 4. Security Hardening

*   **Image Scanning**: Integrate Trivy/Grype into CI pipeline.
*   **RBAC Review**: Audit the controller's RBAC permissions (it currently needs broad permissions to manage Jobs and Secrets).
*   **Network Policies**: Define default NetworkPolicies for the controller namespace.

## 5. Documentation

*   **API Reference**: Auto-generate API docs from CRD types.
*   **Troubleshooting Guide**: Expand with common error scenarios.
*   **Deployment Guide**: Helm chart creation.
