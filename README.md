# Forge Helm Repository

This repository hosts Helm charts for Forge - a Kubernetes controller for Zarf package operations.

## Usage

Add the Helm repository:

```bash
helm repo add forge https://kylegalloway.github.io/forge
helm repo update
```

Search for available charts:

```bash
helm search repo forge
```

Install Forge:

```bash
helm install forge forge/forge
```

Install a specific version:

```bash
helm install forge forge/forge --version 1.0.0
```

## Available Charts

- **forge** - Kubernetes controller for declarative Zarf package operations

## Documentation

For full documentation, visit the [main repository](https://github.com/kylegalloway/forge).

## Chart Versions

Chart versions are published automatically via GitHub Actions when version tags are pushed to the repository.
