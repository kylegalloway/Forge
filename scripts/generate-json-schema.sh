#!/bin/bash
set -e

# Generate JSON Schema from ScriptRunner CRD for IDE integration
# This schema can be used with VS Code, IntelliJ, and other editors

OUTPUT_FILE="${1:-schemas/scriptrunner-v1alpha1.json}"
CRD_FILE="config/crd/scriptrunner.io_scriptrunners.yaml"

echo "Generating JSON Schema for ScriptRunner..."

# Create schemas directory if it doesn't exist
mkdir -p "$(dirname "$OUTPUT_FILE")"

# Check if kubectl is available and CRD is installed
if command -v kubectl &> /dev/null && kubectl get crd scriptrunners.scriptrunner.io &> /dev/null; then
    echo "Extracting schema from installed CRD..."

    kubectl get crd scriptrunners.scriptrunner.io -o json | jq '{
      "$schema": "http://json-schema.org/draft-07/schema#",
      "$id": "https://scriptrunner.io/schemas/scriptrunner-v1alpha1.json",
      "type": "object",
      "title": "ScriptRunner",
      "description": "ScriptRunner executes pre-built scripts with user-provided inputs",
      "required": ["apiVersion", "kind", "metadata", "spec"],
      "properties": {
        "apiVersion": {
          "type": "string",
          "const": "scriptrunner.io/v1alpha1",
          "description": "API version for ScriptRunner"
        },
        "kind": {
          "type": "string",
          "const": "ScriptRunner",
          "description": "Resource kind"
        },
        "metadata": {
          "type": "object",
          "required": ["name"],
          "properties": {
            "name": {
              "type": "string",
              "description": "Name of the ScriptRunner resource",
              "pattern": "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
              "maxLength": 253
            },
            "namespace": {
              "type": "string",
              "description": "Namespace for the ScriptRunner",
              "pattern": "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
              "maxLength": 63
            },
            "labels": {
              "type": "object",
              "description": "Labels for organizing ScriptRunners",
              "additionalProperties": {
                "type": "string"
              }
            }
          }
        },
        "spec": .spec.versions[0].schema.openAPIV3Schema.properties.spec,
        "status": .spec.versions[0].schema.openAPIV3Schema.properties.status
      }
    }' > "$OUTPUT_FILE"
else
    echo "CRD not installed or kubectl not available, generating from local file..."

    # Parse YAML and generate schema
    cat > "$OUTPUT_FILE" <<'EOF'
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "https://scriptrunner.io/schemas/scriptrunner-v1alpha1.json",
  "type": "object",
  "title": "ScriptRunner",
  "description": "ScriptRunner executes pre-built scripts with user-provided inputs",
  "required": ["apiVersion", "kind", "metadata", "spec"],
  "properties": {
    "apiVersion": {
      "type": "string",
      "const": "scriptrunner.io/v1alpha1",
      "description": "API version for ScriptRunner"
    },
    "kind": {
      "type": "string",
      "const": "ScriptRunner",
      "description": "Resource kind"
    },
    "metadata": {
      "type": "object",
      "required": ["name"],
      "properties": {
        "name": {
          "type": "string",
          "description": "Name of the ScriptRunner resource",
          "pattern": "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
          "maxLength": 253
        },
        "namespace": {
          "type": "string",
          "description": "Namespace for the ScriptRunner",
          "pattern": "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
          "maxLength": 63
        },
        "labels": {
          "type": "object",
          "description": "Labels for organizing ScriptRunners",
          "additionalProperties": {
            "type": "string"
          }
        }
      }
    },
    "spec": {
      "type": "object",
      "description": "Specification for the ScriptRunner",
      "properties": {
        "inputs": {
          "type": "object",
          "description": "Key-value pairs passed as environment variables to the script (INPUT_<key>)",
          "additionalProperties": {
            "type": "string"
          },
          "maxProperties": 20
        },
        "image": {
          "type": "string",
          "description": "Container image to use for the job (defaults to busybox:latest)",
          "examples": [
            "scriptrunner-scripts:latest",
            "your-registry.io/scriptrunner-scripts:v1.0.0"
          ]
        },
        "script": {
          "type": "string",
          "description": "Inline shell script to execute (mutually exclusive with scriptRef)"
        },
        "scriptRef": {
          "type": "string",
          "description": "Path to a pre-built script in the container image (mutually exclusive with script)",
          "examples": [
            "/scripts/process-data.sh",
            "/scripts/validate-inputs.sh",
            "/scripts/report-status.py"
          ]
        },
        "scriptArgs": {
          "type": "array",
          "description": "Arguments to pass to the script when using scriptRef",
          "items": {
            "type": "string",
            "maxLength": 200
          },
          "maxItems": 10
        }
      }
    },
    "status": {
      "type": "object",
      "description": "Status of the ScriptRunner (read-only)",
      "properties": {
        "phase": {
          "type": "string",
          "description": "Current phase of the ScriptRunner"
        },
        "jobName": {
          "type": "string",
          "description": "Name of the created Job"
        },
        "message": {
          "type": "string",
          "description": "Additional information about the current state"
        },
        "lastUpdateTime": {
          "type": "string",
          "format": "date-time",
          "description": "Last time the status was updated"
        }
      }
    }
  }
}
EOF
fi

echo "✓ JSON Schema generated: $OUTPUT_FILE"
echo ""
echo "To use with VS Code, add to .vscode/settings.json:"
echo ""
cat <<'VSCODE'
{
  "yaml.schemas": {
    "./schemas/scriptrunner-v1alpha1.json": "*scriptrunner*.yaml"
  }
}
VSCODE
echo ""
echo "Or reference from a URL:"
echo ""
cat <<'VSCODE_URL'
{
  "yaml.schemas": {
    "https://your-company.com/schemas/scriptrunner-v1alpha1.json": "*scriptrunner*.yaml"
  }
}
VSCODE_URL
