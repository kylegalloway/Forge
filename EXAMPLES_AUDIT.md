# Examples and Tests Directory Audit

## Summary

Analyzed all example and test YAML files to identify overlap, duplication, and fitness for purpose.

---

## Directory Structure

```text
examples/
├── samples/              # Complex reference examples (KEEP)
│   ├── zarf/            # 3 examples: Git→OCI, Local→S3, BuildDeploy
│   └── uds/             # 3 examples: Git→OCI, Local→S3, BuildDeploy
├── policies/            # Policy reference examples (KEEP)
│   └── uds/             # 3 SAs: permissive, restricted, ci-cd
├── service-accounts/     # General policy examples (KEEP)
│   ├── simple-test-sa.yaml
│   └── service-account-example.yaml
└── test-packages/        # Minimal test packages (REMOVE)
    └── hello-forge/     # Not used anywhere

tests/
└── e2e/                  # NEW automated tests (KEEP)
    ├── 01-simple-build/
    ├── 02-simple-deploy/
    └── 03-health-check/
```

---

## Analysis by Directory

### ✅ KEEP: `examples/samples/`

**Purpose**: Complex reference examples showing real-world workflows

**Content**:
- Zarf: Git→OCI, Local→S3, BuildDeploy (Headlamp deployment)
- UDS: Git→OCI, Local→S3, BuildDeploy

**Fitness**: GOOD
- Comprehensive READMEs with prerequisites, usage, monitoring
- Include complete resources (ServiceAccount + Job + supporting files)
- Show advanced features (OCI publish, S3, multi-step workflows)
- Educational value for users

**Issues**: None

**Recommendation**: Keep as-is, update main README to clarify relationship with tests/e2e/

---

### ✅ KEEP: `examples/policies/uds/`

**Purpose**: UDS-specific ServiceAccount policy examples

**Content**:
- `permissive-serviceaccount.yaml` - Development use
- `restricted-serviceaccount.yaml` - Production use (create/deploy only)
- `ci-cd-serviceaccount.yaml` - CI/CD pipelines (create/publish only)

**Fitness**: EXCELLENT
- Demonstrates different security postures
- Includes example Secrets for credentials
- Well-documented with use cases

**Issues**: None

**Recommendation**: Keep as-is

---

### ✅ KEEP: `examples/service-accounts/`

**Purpose**: General Zarf policy examples

**Content**:
- `simple-test-sa.yaml` - Quick testing SA (default namespace)
- `service-account-example.yaml` - Multi-team SAs (3 SAs, 3 namespaces)

**Fitness**: GOOD
- Comprehensive README (195 lines)
- Policy annotation reference table
- Best practices section
- Glob pattern examples

**Overlap with policies/uds/**: Minimal - these are Zarf-focused, policies/uds/ is UDS-focused

**Issues**: None

**Recommendation**: Keep as-is

---

### ❌ REMOVE: `examples/test-packages/hello-forge/`

**Purpose**: Minimal Zarf package for testing in resource-constrained environments

**Content**:
- Single component: copies text file to `/tmp/hello-forge.txt`
- Build time: ~15-30 seconds
- Designed for Kind clusters

**Fitness**: POOR - No longer needed

**Issues**:
1. **Not used anywhere** - Our new e2e tests use public repos (zarf-hello-world)
2. **Requires local source** - Tests use Git sources instead
3. **Maintenance burden** - Another package to keep updated
4. **README says**: "You can reference it from a ZarfPackageJob using a local source" but:
   - Local sources disabled by default (security)
   - E2E tests don't use local sources
   - No example actually uses this

**Recommendation**: **REMOVE** - Replaced by public repos in e2e tests

---

### ✅ KEEP: `tests/e2e/`

**Purpose**: Automated functional tests that work on Kind and prod clusters

**Content**:
- 01-simple-build: Git → Zarf package (devMode)
- 02-simple-deploy: Package → cluster deployment
- 03-health-check: Controller health/metrics verification

**Fitness**: EXCELLENT
- Simple, focused tests
- Portable (Kind + prod)
- Automated runner script
- Clear success criteria
- No external dependencies (uses public repos)

**Overlap with examples/samples/**: Minimal
- tests/e2e: Simple, automation-focused, uses zarf-hello-world
- examples/samples: Complex, reference-focused, uses custom packages

**Issues**: None

**Recommendation**: Keep as-is

---

## Duplication Analysis

### ServiceAccounts

**examples/service-accounts/simple-test-sa.yaml** vs **tests/e2e/*/serviceaccount.yaml**:

| Aspect | examples/service-accounts/ | tests/e2e/ |
|--------|---------------------------|------------|
| Purpose | Reference for users | Functional testing |
| Permissions | Specific repos/registries | Wildcard (permissive) |
| Namespace | default | default |
| Documentation | Extensive README | Test-specific README |
| Audience | End users | CI/CD and developers |

**Verdict**: NOT duplicates - different purposes

### Test Packages

**examples/test-packages/hello-forge/** vs **tests/e2e/01-simple-build/**:

| Aspect | hello-forge | e2e tests |
|--------|-------------|-----------|
| Package source | Local in repo | Public Git (zarf-hello-world) |
| Source type | Local (requires devMode) | Git (no special mode) |
| Maintenance | We maintain | External project |
| Used by | Nothing | Automated tests |

**Verdict**: hello-forge is OBSOLETE - remove it

---

## Issues Found

### 1. examples/README.md is outdated

**Current state**:
```markdown
| Workflow | Zarf | UDS | Notes |
|----------|------|-----|-------|
| Build only | ⏸️ | ⏸️ | Planned |
| Deploy only | ⏸️ | ⏸️ | Planned |
```

**Reality**: Build-only and Deploy-only now exist in `tests/e2e/`

**Fix**: Update table, add section pointing to tests/e2e/

### 2. No clear separation between reference and testing

**Current**: Users might think `examples/samples/` are for testing

**Fix**: Clarify in READMEs:
- `examples/` = Reference material for learning and customization
- `tests/e2e/` = Automated tests for verification

### 3. hello-forge not used

**Current**: Directory exists but nothing references it

**Fix**: Remove `examples/test-packages/`

---

## Recommended Changes

### 1. Remove Obsolete Test Package

```bash
rm -rf examples/test-packages/
```

**Rationale**: Not used, replaced by public repos in e2e tests

### 2. Update examples/README.md

Add section:

```markdown
## Testing vs Examples

- **`examples/`** - Reference material for users to learn and customize
  - Complex workflows with real-world packages
  - Comprehensive documentation
  - Includes prerequisites, credentials, monitoring

- **`tests/e2e/`** - Automated functional tests
  - Simple, focused tests
  - Work on both Kind and production clusters
  - Used by CI/CD and `make e2e-test`
  - See [tests/e2e/README.md](../tests/e2e/README.md)
```

Update workflow table:

```markdown
| Workflow | Examples | Tests | Notes |
|----------|----------|-------|-------|
| Git → OCI | ✅ samples/zarf/01 | ⏸️ | Reference example |
| Local → S3 | ✅ samples/zarf/02 | ⏸️ | Reference example |
| Build only | ⏸️ | ✅ tests/e2e/01 | Automated test |
| Deploy only | ⏸️ | ✅ tests/e2e/02 | Automated test |
| BuildDeploy | ✅ samples/zarf/03 | ⏸️ | Reference example |
| Health check | ⏸️ | ✅ tests/e2e/03 | Automated test |
```

### 3. Add cross-references

In `examples/samples/zarf/README.md`:

```markdown
## Related Documentation

- [Zarf Documentation](https://zarf.dev/)
- [Forge Controller Documentation](../../../docs/)
- [UDS Bundle Examples](../uds/)
- [Automated Tests](../../../tests/e2e/) - Simple tests for CI/CD
```

In `tests/e2e/README.md`:

```markdown
## Related Examples

For more complex reference examples, see:
- [Zarf Package Examples](../../examples/samples/zarf/)
- [UDS Bundle Examples](../../examples/samples/uds/)
- [ServiceAccount Policies](../../examples/service-accounts/)
```

---

## Final Structure

```text
examples/
├── samples/              # KEEP - Reference examples
│   ├── zarf/            # Complex Zarf workflows
│   └── uds/             # Complex UDS workflows
├── policies/            # KEEP - Policy reference
│   └── uds/             # UDS policy examples
└── service-accounts/     # KEEP - General policy examples

tests/
└── e2e/                  # KEEP - Automated tests
    ├── 01-simple-build/
    ├── 02-simple-deploy/
    └── 03-health-check/

scripts/
├── release.sh           # KEEP
├── test-e2e.sh          # REMOVED (replaced by tests/e2e/)
└── verify-forge-image.sh # KEEP
```

---

## Final Summary

**KEEP (6 directories)**:
- ✅ examples/samples/zarf/ - Reference examples
- ✅ examples/samples/uds/ - Reference examples
- ✅ examples/policies/uds/ - Policy examples
- ✅ examples/service-accounts/ - Policy examples
- ✅ tests/e2e/ - Automated tests
- ✅ scripts/ (except test-e2e.sh)

**REMOVE (2 items)**:
- ❌ examples/test-packages/ - Obsolete, not used
- ❌ scripts/test-e2e.sh - Replaced by make e2e-test

**UPDATE (1 file)**:
- 📝 examples/README.md - Add testing section, update workflow table
