# ScriptRunner JSON Schemas

This directory contains JSON schemas for client-side validation and IDE integration.

## Files

- `scriptrunner-v1alpha1.json` - JSON Schema for ScriptRunner v1alpha1

## Usage

### VS Code Integration

Add to `.vscode/settings.json`:

```json
{
  "yaml.schemas": {
    "./schemas/scriptrunner-v1alpha1.json": "*scriptrunner*.yaml"
  }
}
```

See [.vscode/settings.json.example](../.vscode/settings.json.example) for a complete example.

### IntelliJ / PyCharm

1. Go to Settings → Languages & Frameworks → Schemas and DTDs → JSON Schema Mappings
2. Add new mapping:
   - Name: ScriptRunner
   - Schema file: Point to `schemas/scriptrunner-v1alpha1.json`
   - Schema version: JSON Schema version 7
   - File path pattern: `*.scriptrunner.yaml`

### Inline YAML Reference

Add to the top of your YAML file:

```yaml
# yaml-language-server: $schema=../schemas/scriptrunner-v1alpha1.json
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
# ...
```

## Regenerating Schema

The schema can be regenerated from the CRD:

```bash
# If CRD is installed in cluster
./scripts/generate-json-schema.sh

# Specify output location
./scripts/generate-json-schema.sh path/to/output.json
```

## Hosting for Users

For production use, host the schema at a stable URL:

**Option 1: GitHub Pages**

```bash
# Commit schema to docs/ folder
git add schemas/scriptrunner-v1alpha1.json
git commit -m "Update schema"
git push

# Enable GitHub Pages for the repo
# Schema available at: https://your-org.github.io/scriptrunner/schemas/scriptrunner-v1alpha1.json
```

**Option 2: Internal Server**

```bash
# Copy to web server
cp schemas/scriptrunner-v1alpha1.json /var/www/html/schemas/

# Available at: https://your-company.com/schemas/scriptrunner-v1alpha1.json
```

Then users reference the URL:

```json
{
  "yaml.schemas": {
    "https://your-company.com/schemas/scriptrunner-v1alpha1.json": "*.scriptrunner.yaml"
  }
}
```

## Benefits

With IDE integration, users get:

- ✅ **Autocomplete** - Field name suggestions
- ✅ **Documentation** - Inline help on hover
- ✅ **Validation** - Real-time error highlighting
- ✅ **Type Checking** - String/number/array validation
- ✅ **Examples** - Example values shown

## See Also

- [CLIENT_VALIDATION.md](../docs/CLIENT_VALIDATION.md) - Complete client validation guide
- [USER_GUIDE.md](../docs/USER_GUIDE.md) - User documentation
