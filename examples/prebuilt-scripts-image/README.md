# Pre-built Scripts Container Image

This directory contains an example container image with pre-built scripts that can be referenced by ScriptRunner resources.

## Building the Image

```bash
cd examples/prebuilt-scripts-image

# Build with podman
podman build -t scriptrunner-scripts:latest .

# Or with docker
docker build -t scriptrunner-scripts:latest .

# Load into kind for testing
kind load docker-image scriptrunner-scripts:latest --name scriptrunner-dev
```

## Included Scripts

### 1. process-data.sh
Data processing script that accepts arguments and uses INPUT_ environment variables.

**Usage:**
- Can be called with arguments: `process-data.sh operation count`
- Falls back to INPUT_ environment variables
- Demonstrates processing with progress updates

### 2. validate-inputs.sh
Input validation script that checks for required variables and validates their format.

**Required Inputs:**
- `environment`: Must be one of: dev, staging, production
- `version`: Must be in semantic versioning format (X.Y.Z)

**Usage:**
- Validates inputs and exits with error if validation fails
- Useful for ensuring ScriptRunner inputs are correct before processing

### 3. report-status.py
Python script for generating status reports in different formats.

**Arguments:**
- First argument (optional): Report format (text or json)
- Can also be specified via INPUT_format environment variable

**Usage:**
- Generates reports about the ScriptRunner execution
- Demonstrates using Python in ScriptRunner

## Using with ScriptRunner

### Example 1: Using process-data.sh with Arguments

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: process-data-example
spec:
  image: scriptrunner-scripts:latest
  scriptRef: /scripts/process-data.sh
  scriptArgs:
    - "batch-process"
    - "20"
  inputs:
    environment: "production"
    source: "database"
```

### Example 2: Using validate-inputs.sh

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: validate-example
spec:
  image: scriptrunner-scripts:latest
  scriptRef: /scripts/validate-inputs.sh
  inputs:
    environment: "staging"
    version: "1.2.3"
```

### Example 3: Using report-status.py with JSON Output

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: report-example
spec:
  image: scriptrunner-scripts:latest
  scriptRef: /scripts/report-status.py
  scriptArgs:
    - "json"
  inputs:
    service: "api-gateway"
    region: "us-west-2"
```

## Adding Your Own Scripts

1. Add your script to the `scripts/` directory
2. Make sure it starts with a shebang (`#!/bin/bash` or `#!/usr/bin/env python3`)
3. Rebuild the image
4. Reference it in your ScriptRunner with `scriptRef: /scripts/your-script.sh`

## Benefits of Pre-built Scripts

- **Reusability**: Share common scripts across multiple ScriptRunners
- **Version Control**: Scripts are versioned with the container image
- **Testing**: Test scripts independently before deploying
- **Performance**: No need to inline large scripts in YAML
- **Security**: Scripts can be reviewed and scanned as part of image build
- **Complex Logic**: Support for multiple languages and dependencies
