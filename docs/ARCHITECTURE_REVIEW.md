# Architecture and Design Review

## Overview

Forge is a Kubernetes controller for managing the lifecycle of Zarf packages and UDS bundles. It automates the building, publishing, and deploying of these artifacts using Kubernetes Jobs.

## Components

### 1. Controller
The core component that watches `ZarfPackage` and `UDSBundle` resources. It reconciles the state of these resources by dispatching actions to specific handlers.
- **Responsibilities**: Resource watching, status updates, policy validation, action dispatching, job monitoring.
- **Key Packages**: `pkg/controller`, `cmd/controller`.

### 2. Action Handlers
Dedicated handlers for each action type (`build`, `publish`, `deploy`). They encapsulate the logic for creating Kubernetes Jobs that execute the respective Zarf CLI commands.
- **Responsibilities**: Job creation, init container configuration (source retrieval), credential mounting.
- **Key Packages**: `pkg/actions`.

### 3. Sources
Abstraction for retrieving package artifacts or source code. Supports Git, S3, OCI, and Local sources.
- **Responsibilities**: Generating init containers to fetch data into a shared workspace.
- **Key Packages**: `pkg/sources`.

### 4. Destinations
Abstraction for where packages are published or deployed. Supports S3, OCI, and Local destinations.
- **Responsibilities**: Generating publish commands and job configuration (credentials).
- **Key Packages**: `pkg/destinations`.

### 5. Policy Engine
Enforces RBAC-like policies based on `ServiceAccount` annotations.
- **Responsibilities**: Validating if a `ServiceAccount` is allowed to perform specific actions, use specific sources/destinations, or deploy to specific targets.
- **Key Packages**: `pkg/policy`.

### 6. Webhook
Validating admission webhook that ensures `ZarfPackage` resources are valid before they are persisted.
- **Responsibilities**: Schema validation, initial policy checks (if applicable).
- **Key Packages**: `pkg/webhook`, `cmd/webhook`.

## Data Flow

1.  **User/CI** creates a `ZarfPackage` or `UDSBundle` CR.
2.  **Webhook** intercepts the request and validates it.
3.  **Controller** detects the new resource.
4.  **Policy Engine** validates the request against the `ServiceAccount`'s permissions.
5.  **Controller** dispatches the request to the appropriate **Action Handler**.
6.  **Action Handler** creates a Kubernetes **Job**:
    *   **Init Container**: Uses `pkg/sources` to fetch artifacts/source code to `/workspace`.
    *   **Main Container**: Executes Zarf CLI command (Build/Publish/Deploy) using `pkg/destinations` configuration.
7.  **Controller** (via `JobMonitor`) tracks the Job's status and updates the CR's status.
8.  **Action Chaining**: If the action is a composite (e.g., `BuildPublish`), the Controller triggers the next action upon successful completion of the previous one.

## Security

*   **ServiceAccount-based Identity**: All actions run as the `ServiceAccount` specified in the CR.
*   **Policy Enforcement**: Fine-grained control over allowed actions and resources via annotations on the `ServiceAccount`.
*   **Credential Management**: Credentials for Git, S3, and OCI are managed via Kubernetes Secrets and mounted only when needed.
*   **Least Privilege**: Jobs run as non-root users with dropped capabilities (where possible).

## Scalability & Reliability

*   **Asynchronous Execution**: Actions are executed as background Jobs, allowing the controller to handle many concurrent requests.
*   **Job Monitoring**: The controller actively monitors Jobs and reflects their status, ensuring the user is aware of progress/failure.
*   **Retry Mechanism**: Kubernetes Jobs handle retries for transient failures (though currently `BackoffLimit` is set to 0 for some actions, which might need review).

## Identified Risks & Issues

1.  **Dependency on Zarf CLI**: The system relies heavily on the Zarf CLI image. Version compatibility between Forge and Zarf CLI is critical.
2.  **Job Cleanup**: Completed Jobs are kept for 1 hour (`TTLSecondsAfterFinished`). High volume of actions could clutter the cluster.
3.  **Log Retrieval**: `getJobLogs` was removed as unused, but retrieving logs for debugging failed Jobs is important. Status updates currently contain only a brief message.
4.  **Race Conditions**: Potential race conditions in status updates if multiple actions update the same resource (mitigated by optimistic locking in K8s client).
5.  **Secret Management**: While secrets are used, the management of these secrets (creation, rotation) is external to Forge.

## Recommendations

1.  **Implement Log Streaming/Archiving**: Store build/deploy logs in a more persistent storage (e.g., S3) or stream them to a logging system.
2.  **Enhanced Status Reporting**: Provide more detailed status information, perhaps including a summary of the Zarf CLI output.
3.  **Garbage Collection**: Implement more aggressive GC for successful Jobs or allow user configuration.
4.  **E2E Testing**: Comprehensive E2E tests covering all source/destination combinations are needed.
5.  **Version Matrix**: Document and test compatibility with different Zarf versions.
