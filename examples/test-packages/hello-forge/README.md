# Hello Forge Test Package

A minimal Zarf package designed to successfully build in resource-constrained environments like Kind clusters.

## What This Does

This package contains a single component that:

- Copies a text file (`message.txt`) to `/tmp/hello-forge.txt`
- Prints a success message during creation
- Completes quickly (under 30 seconds)

## Why This Exists

The full Zarf repository is too large and complex to build in a test Kind cluster. This minimal package provides a way to verify that Forge is working correctly without requiring significant resources.

## Testing Locally

You can test this package directly with Zarf:

```bash
cd examples/test-packages/hello-forge
zarf package create . --confirm
```

## Using with Forge

This package is used by the test ZarfPackageJob in `examples/zarfpackagejobs/hello-forge-test.yaml`.

The job will:

1. Clone this repository
2. Navigate to this directory
3. Build the package
4. Complete successfully

Expected build time: ~15-30 seconds in Kind
