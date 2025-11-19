# Test Package

This is a minimal Zarf package used for testing the ScriptRunner Zarf builder.

It contains:
- This README file
- A simple zarf.yaml with one component
- No container images (for fast builds)

## Usage

This package is automatically included in the Zarf builder image for testing purposes.

To test locally:
```bash
zarf package create . --confirm
```
