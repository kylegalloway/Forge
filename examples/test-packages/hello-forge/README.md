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

This package can be used to test Forge in resource-constrained environments. You can reference it from a ZarfPackageJob using a local source or by pointing to this directory in the repository.

Example workflow:

1. Clone this repository
2. Point a ZarfPackageJob source to this directory
3. Build the package
4. Verify successful completion

Expected build time: ~15-30 seconds in Kind

See [examples/samples/](../../samples/) for complete ZarfPackageJob workflow examples.
