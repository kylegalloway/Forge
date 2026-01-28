# Forge Documentation

Welcome to the Forge documentation. This guide will help you navigate through the available documentation based on your needs.

## Getting Started

New to Forge? Start here to get up and running quickly.

- [User Guide](getting-started/USER_GUIDE.md) - Complete guide to using Forge for Zarf packages and UDS bundles
- [Deployment Guide](getting-started/DEPLOYMENT.md) - Deployment scenarios and configurations

## CLI Tools

- [kubectl-forge Plugin](../cmd/kubectl-forge/README.md) - kubectl plugin for listing, downloading artifacts, and debugging Forge jobs

## Development

Contributing to Forge or extending its functionality? These guides cover development workflows and architecture.

- [Architecture](development/ARCHITECTURE.md) - System design and component overview
- [Testing Guide](development/TESTING.md) - Running tests, coverage, and CI workflows
- [KIND Setup](development/KIND_SETUP.md) - Local development environment setup
- [KIND Testing with Public Images](development/KIND_TESTING_PUBLIC_IMAGES.md) - Testing workflows
- [Gitea Testing](development/GITEA_TESTING.md) - Testing with private Git repositories
- [Releasing](development/RELEASING.md) - Release process and versioning
- [ServiceAccount Reference](development/SERVICEACCOUNT_REFERENCE.md) - RBAC and policy configuration reference
- [Attestation Verification](development/ATTESTATION_VERIFICATION.md) - SLSA provenance and package signing
- [Logging](development/LOGGING.md) - Logging conventions and practices
- [Structured Logging](development/STRUCTURED-LOGGING.md) - Structured logging implementation details
- [Tool Versions](development/TOOL_VERSIONS.md) - Dependency version tracking

## Operations

Running Forge in production? These guides cover deployment, hosting, and operational concerns.

- [Hosting Setup](operations/HOSTING_SETUP.md) - Container registry and Helm chart hosting (GHCR, GitHub Releases)
- [Namespace-Scoped Deployment](operations/NAMESPACE_SCOPED_DEPLOYMENT.md) - Cluster-wide vs namespace-scoped RBAC
- [Production Checklist](operations/PRODUCTION_CHECKLIST.md) - Pre-deployment verification checklist
- [Runbook](operations/RUNBOOK.md) - Operational procedures and common tasks
- [Troubleshooting](operations/TROUBLESHOOTING.md) - Debugging Zarf package jobs and common issues
- [UDS Troubleshooting](operations/UDS_TROUBLESHOOTING.md) - Debugging UDS bundle jobs
- [AWS Credentials](operations/AWS_CREDENTIALS.md) - AWS credential management

## Security

Security documentation and compliance information.

- [Pod Security Standards](security/pod-security-standards.md) - Kubernetes PSS compliance
- [RBAC Permissions](security/rbac-permissions.md) - Detailed RBAC requirements

## Planning

Future enhancements and roadmap items.

- [TODO](plans/TODO.md) - Current issues and future enhancements

## Quick Links

- [Main README](../README.md) - Project overview and quick start
- [Contributing Guide](../CONTRIBUTING.md) - Development workflow and guidelines
- [Helm Chart README](../chart/README.md) - Helm chart documentation

## Documentation Organization

This documentation is organized into five main categories:

1. **getting-started/** - User-focused guides for learning Forge
2. **development/** - Developer-focused guides for contributing
3. **operations/** - Operator-focused guides for production deployments
4. **security/** - Security documentation and compliance information
5. **plans/** - Future enhancements and roadmap items

Each category serves a distinct audience with different needs and levels of experience with Forge.
