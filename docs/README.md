# Forge Documentation

Welcome to the Forge documentation. This guide will help you navigate through the available documentation based on your needs.

## Getting Started

New to Forge? Start here to get up and running quickly.

- [User Guide](getting-started/USER_GUIDE.md) - Complete guide to using Forge for Zarf package operations
- [KIND Setup](getting-started/KIND_SETUP.md) - Local development environment setup with KIND
- [KIND Testing with Public Images](getting-started/KIND_TESTING_PUBLIC_IMAGES.md) - Testing workflows with publicly available images

## Development

Contributing to Forge or extending its functionality? These guides cover development workflows and architecture.

- [Testing Guide](development/TESTING.md) - Running tests, coverage, and CI workflows
- [Releasing](development/RELEASING.md) - Release process and versioning
- [ServiceAccount Reference](development/SERVICEACCOUNT_REFERENCE.md) - RBAC and policy configuration reference
- [Attestation Verification](development/ATTESTATION_VERIFICATION.md) - SLSA provenance and package signing

## Operations

Running Forge in production? These guides cover deployment, hosting, and operational concerns.

- [Hosting](operations/HOSTING.md) - Production hosting strategy (container registries, Helm charts)
- [Hosting Setup](operations/HOSTING_SETUP.md) - Step-by-step hosting configuration
- [Namespace-Scoped Deployment](operations/NAMESPACE_SCOPED_DEPLOYMENT.md) - Cluster-wide vs namespace-scoped RBAC
- [Production Checklist](operations/PRODUCTION_CHECKLIST.md) - Pre-deployment verification checklist
- [Runbook](operations/RUNBOOK.md) - Operational procedures and common tasks
- [Troubleshooting](operations/TROUBLESHOOTING.md) - Debugging failed jobs and common issues

## Quick Links

- [Main README](../README.md) - Project overview and quick start
- [Contributing Guide](../CONTRIBUTING.md) - Development workflow and guidelines
- [Helm Chart README](../chart/README.md) - Helm chart documentation
- [Deployment Guide](../DEPLOYMENT.md) - Deployment scenarios and configurations

## Documentation Organization

This documentation is organized into three main categories:

1. **getting-started/** - User-focused guides for learning Forge
2. **development/** - Developer-focused guides for contributing
3. **operations/** - Operator-focused guides for production deployments

Each category serves a distinct audience with different needs and levels of experience with Forge.
