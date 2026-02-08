# Test 12: Controller HA Validation

## Description

Validates that controller High Availability infrastructure is properly configured:
- Leader election lease exists and has an active holder
- Helm template renders correctly with HA values (PDB, anti-affinity, LE flags)
- Helm template omits HA resources for single-replica deployments
- Worker count and leader election flags are passed correctly

## Prerequisites

- Forge controller deployed in `forge-system` namespace
- `helm` CLI available (for template rendering tests)

## Expected Behavior

1. Leader election lease exists in the controller namespace
2. Helm template with `replicaCount=3` produces PDB and anti-affinity
3. Helm template with default values includes `--enable-leader-election`
4. Helm template with `leaderElection.enabled=false` omits LE flags
5. Helm template with `workers=4` includes `--workers=4` flag

## Running the Test

```bash
./test.sh
```

## Success Criteria

- Leader election lease `forge-controller-lock` exists with a holder
- PDB rendered when `replicaCount > 1`, absent when `replicaCount = 1`
- Anti-affinity rendered when `replicaCount > 1`
- `--enable-leader-election` present by default, absent when disabled
- `--workers` flag only present when non-default value set
