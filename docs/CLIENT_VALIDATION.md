# Client-Side Validation for ScriptRunner

This guide explains options for validating ScriptRunner YAML before applying to the cluster.

## Why Client-Side Validation?

**Benefits:**
- Catch errors before applying to cluster
- Faster feedback loop
- No API server round-trip
- Works offline
- Better IDE integration

**The CRD provides server-side validation**, but client-side validation improves the developer experience.

## Option 1: kubectl with --dry-run (Simplest)

The easiest approach - use kubectl's built-in validation:

```bash
# Validate against CRD without creating the resource
kubectl apply --dry-run=server -f my-scriptrunner.yaml

# Or just validate the YAML structure
kubectl apply --dry-run=client -f my-scriptrunner.yaml
```

**Pros:**
- No additional tools needed
- Uses actual CRD validation
- Checks RBAC permissions too (with `--dry-run=server`)

**Cons:**
- Requires cluster access
- Slower than local validation

### User Workflow

```bash
# 1. Write YAML
cat > my-task.yaml <<EOF
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: my-task
spec:
  scriptRef: /scripts/process-data.sh
  inputs:
    environment: "production"
EOF

# 2. Validate
kubectl apply --dry-run=server -f my-task.yaml

# 3. If valid, apply for real
kubectl apply -f my-task.yaml
```

## Option 2: kubeconform (Fast Local Validation)

[kubeconform](https://github.com/yannh/kubeconform) validates Kubernetes YAML against schemas locally.

### Setup

```bash
# Install kubeconform
# macOS
brew install kubeconform

# Linux
wget https://github.com/yannh/kubeconform/releases/download/v0.6.4/kubeconform-linux-amd64.tar.gz
tar xf kubeconform-linux-amd64.tar.gz
sudo mv kubeconform /usr/local/bin/
```

### Generate Schema from CRD

```bash
# Extract OpenAPI schema from CRD
kubectl get crd scriptrunners.scriptrunner.io -o json | \
  jq '.spec.versions[0].schema.openAPIV3Schema' > scriptrunner-schema.json

# Or we can provide a pre-generated schema
```

### Validate

```bash
# Validate against local schema
kubeconform -schema-location default \
  -schema-location 'schemas/{{ .ResourceKind }}.json' \
  my-scriptrunner.yaml
```

**Pros:**
- Fast (no cluster access needed)
- Works offline
- Can validate multiple files at once

**Cons:**
- Requires tool installation
- Schema needs to be kept in sync with CRD

## Option 3: Pre-commit Hooks (Automated)

Automatically validate before git commits using [pre-commit](https://pre-commit.com/).

### Setup for Users

**1. Install pre-commit:**

```bash
# macOS
brew install pre-commit

# Linux/macOS via pip
pip install pre-commit
```

**2. Create `.pre-commit-config.yaml` in user's repo:**

```yaml
repos:
  # Validate Kubernetes YAML
  - repo: https://github.com/yannh/kubeconform
    rev: v0.6.4
    hooks:
      - id: kubeconform
        args:
          - -summary
          - -output=text
          - -schema-location=default
          - -schema-location=https://your-company.com/schemas/{{ .ResourceKind }}.json

  # Alternative: Use kubectl if cluster access available
  - repo: local
    hooks:
      - id: kubectl-validate
        name: Validate with kubectl
        entry: kubectl apply --dry-run=client -f
        language: system
        files: '.*scriptrunner.*\.ya?ml$'
```

**3. Install hooks:**

```bash
cd user-repo
pre-commit install
```

**4. Now validation runs automatically:**

```bash
git add my-scriptrunner.yaml
git commit -m "Add new task"
# Validation runs automatically
```

**Pros:**
- Automatic validation
- Catches errors before push
- Consistent across team

**Cons:**
- Requires setup per repository

## Option 4: JSON Schema + IDE Integration (Best Developer Experience)

Provide a JSON Schema that works with VS Code, IntelliJ, and other IDEs.

### Generate JSON Schema

Create a script to extract and enhance the schema:

```bash
#!/bin/bash
# scripts/generate-json-schema.sh

# Get CRD schema
kubectl get crd scriptrunners.scriptrunner.io -o json | \
  jq '{
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "title": "ScriptRunner",
    "description": "A ScriptRunner resource for executing pre-built scripts",
    "properties": {
      "apiVersion": {
        "type": "string",
        "enum": ["scriptrunner.io/v1alpha1"],
        "description": "API version"
      },
      "kind": {
        "type": "string",
        "enum": ["ScriptRunner"],
        "description": "Resource kind"
      },
      "metadata": {
        "type": "object",
        "properties": {
          "name": { "type": "string" },
          "namespace": { "type": "string" }
        },
        "required": ["name"]
      },
      "spec": .spec.versions[0].schema.openAPIV3Schema.properties.spec
    },
    "required": ["apiVersion", "kind", "metadata", "spec"]
  }' > schemas/scriptrunner-v1alpha1.json
```

### VS Code Integration

**Provide this in your user documentation:**

**Method 1: Workspace settings (per-project)**

Create `.vscode/settings.json` in user's project:

```json
{
  "yaml.schemas": {
    "https://your-company.com/schemas/scriptrunner-v1alpha1.json": [
      "*scriptrunner*.yaml",
      "scriptrunners/*.yaml"
    ]
  }
}
```

**Method 2: Inline schema reference**

Users add to the top of their YAML files:

```yaml
# yaml-language-server: $schema=https://your-company.com/schemas/scriptrunner-v1alpha1.json
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: my-task
spec:
  scriptRef: /scripts/process-data.sh
  inputs:
    environment: "production"
```

**Method 3: File naming convention**

If users name files with a specific pattern, configure globally:

```json
{
  "yaml.schemas": {
    "https://your-company.com/schemas/scriptrunner-v1alpha1.json": [
      "**/*.scriptrunner.yaml"
    ]
  }
}
```

### What Users Get

With IDE integration:
- ✅ **Autocomplete** for field names
- ✅ **Inline documentation** on hover
- ✅ **Real-time validation** errors
- ✅ **Type checking** (string vs number)
- ✅ **Enum validation** (approved scripts)

**Screenshot of experience:**
```yaml
spec:
  scriptRef: /scripts/process-data.sh  # ✓ Valid
  scriptRef: /scripts/unknown.sh       # ✗ Error: not in enum
  inputs:
    environment: "production"           # ✓ Valid
    count: 100                          # ⚠ Warning: should be string
```

## Option 5: Custom CLI Tool (Advanced)

Provide a simple CLI wrapper for validation and creation:

```bash
#!/bin/bash
# sr (ScriptRunner CLI)

case "$1" in
  validate)
    kubectl apply --dry-run=server -f "$2"
    ;;
  create)
    kubectl apply --dry-run=server -f "$2" && kubectl apply -f "$2"
    ;;
  list)
    kubectl get scriptrunner -n "${NAMESPACE:-default}"
    ;;
  logs)
    kubectl logs -n "${NAMESPACE:-default}" -l "scriptrunner.io/name=$2"
    ;;
  *)
    echo "Usage: sr {validate|create|list|logs} [file|name]"
    ;;
esac
```

Users run:
```bash
sr validate my-task.yaml
sr create my-task.yaml
sr logs my-task
```

## Recommended Approach

### For Most Users: CRD + kubectl dry-run + IDE

**Provide users with:**

1. **Enhanced CRD with validation** (you already have this)
2. **JSON Schema for IDE integration**
3. **Simple workflow documentation**

**User setup (one-time):**

```bash
# 1. Configure VS Code (create .vscode/settings.json)
cat > .vscode/settings.json <<EOF
{
  "yaml.schemas": {
    "https://your-company.com/schemas/scriptrunner-v1alpha1.json": [
      "*scriptrunner*.yaml"
    ]
  }
}
EOF

# 2. Create alias for validation
alias sr-validate='kubectl apply --dry-run=server -f'
```

**Daily workflow:**

```yaml
# 1. Write YAML (with IDE autocomplete and validation)
# 2. Validate before applying
sr-validate my-task.yaml

# 3. Apply
kubectl apply -f my-task.yaml
```

## Providing Schemas to Users

### Host Schemas Publicly

```bash
# Directory structure
schemas/
├── scriptrunner-v1alpha1.json
├── index.html  # Schema catalog
└── README.md

# Host on GitHub Pages or internal server
# URL: https://your-company.com/schemas/scriptrunner-v1alpha1.json
```

### Include in Documentation

In your USER_GUIDE.md:

```markdown
## IDE Setup

For the best experience, configure your IDE for ScriptRunner validation:

### VS Code

1. Install the YAML extension (if not installed)
2. Add to `.vscode/settings.json`:

\`\`\`json
{
  "yaml.schemas": {
    "https://your-company.com/schemas/scriptrunner-v1alpha1.json": [
      "*scriptrunner*.yaml"
    ]
  }
}
\`\`\`

3. Restart VS Code

Now you'll get autocomplete and validation while editing ScriptRunner YAML!
```

### Package as Helm Chart / Kustomize

Include schema in your deployment package:

```yaml
# kustomize/base/configmap-schemas.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: scriptrunner-schemas
  namespace: scriptrunner-system
data:
  scriptrunner-v1alpha1.json: |
    {
      "$schema": "http://json-schema.org/draft-07/schema#",
      ...
    }
```

Users can download:
```bash
kubectl get configmap scriptrunner-schemas -n scriptrunner-system \
  -o jsonpath='{.data.scriptrunner-v1alpha1\.json}' > schema.json
```

## Enhanced CRD Validation

Your CRD can also be enhanced for better validation:

```yaml
# config/crd/scriptrunner.io_scriptrunners.yaml
spec:
  versions:
  - name: v1alpha1
    schema:
      openAPIV3Schema:
        type: object
        required: ["spec"]
        properties:
          spec:
            type: object
            # Exactly one of script or scriptRef must be specified
            oneOf:
              - required: ["script"]
                properties:
                  scriptRef:
                    maxLength: 0
              - required: ["scriptRef"]
                properties:
                  script:
                    maxLength: 0
            properties:
              scriptRef:
                type: string
                description: Path to pre-built script
                # Enumerate allowed scripts
                enum:
                  - /scripts/process-data.sh
                  - /scripts/validate-inputs.sh
                  - /scripts/report-status.py
                # Or use pattern
                pattern: '^/scripts/[a-z0-9-]+\.(sh|py)$'
              image:
                type: string
                description: Container image
                pattern: '^your-registry\.io/scriptrunner-scripts:v[0-9]+\.[0-9]+\.[0-9]+$'
              inputs:
                type: object
                description: Input key-value pairs
                maxProperties: 20
                additionalProperties:
                  type: string
                  maxLength: 1000
              scriptArgs:
                type: array
                description: Script arguments
                maxItems: 10
                items:
                  type: string
                  maxLength: 200
```

This validation happens server-side but the schema is also used by client tools.

## Comparison Matrix

| Method | Speed | Offline | Setup | IDE Integration | Server Validation |
|--------|-------|---------|-------|-----------------|-------------------|
| kubectl dry-run | Medium | No | None | No | Yes |
| kubeconform | Fast | Yes | Easy | No | No |
| pre-commit | Fast | Yes | Medium | No | Optional |
| JSON Schema | Instant | Yes | Easy | Yes | No |
| Custom CLI | Medium | No | Medium | No | Yes |

## Recommendation Summary

**Minimum (all users):**
- Enhanced CRD validation (you have this)
- Document `kubectl apply --dry-run=server` workflow

**Recommended (better UX):**
- Provide JSON Schema for IDE integration
- Host schema at stable URL
- Include VS Code setup in documentation

**Optional (advanced users):**
- pre-commit hooks for automated validation
- Custom CLI tool for convenience

**The combination of:**
1. Enhanced CRD validation (server-side)
2. JSON Schema (IDE integration)
3. kubectl dry-run (final check)

Provides the best balance of ease-of-use and validation coverage without requiring complex tooling.

## Quick Start for Users

Provide this simple setup guide:

```bash
# 1. One-time VS Code setup
mkdir -p .vscode
cat > .vscode/settings.json <<'EOF'
{
  "yaml.schemas": {
    "https://your-company.com/schemas/scriptrunner-v1alpha1.json": "*.scriptrunner.yaml"
  }
}
EOF

# 2. Create ScriptRunner with .scriptrunner.yaml extension
cat > my-task.scriptrunner.yaml <<'EOF'
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: my-task
spec:
  scriptRef: /scripts/process-data.sh
  inputs:
    environment: "production"
EOF

# 3. Validate
kubectl apply --dry-run=server -f my-task.scriptrunner.yaml

# 4. Apply
kubectl apply -f my-task.scriptrunner.yaml
```

That's it! IDE gives real-time validation, kubectl dry-run gives final server-side check.
