# Test 13: Concurrency Limits Validation

## Description

Validates that job concurrency limits and backpressure mechanics work correctly:
- Helm template renders concurrency flags when configured
- Helm template omits flags when limits are 0 (unlimited)
- Backpressure metrics are exposed when concurrency limits are active
- Controller CLI accepts `--max-concurrent-jobs-per-namespace` and `--max-concurrent-jobs-global` flags

## Prerequisites

- `helm` CLI available (for template rendering tests)
- Forge controller deployed in `forge-system` namespace (for live checks)

## Expected Behavior

1. With limits set to 0, no concurrency flags appear in the deployment
2. With limits > 0, the correct `--max-concurrent-jobs-*` flags are passed
3. Controller metrics endpoint exposes backpressure-related metrics
4. Controller logs show concurrency configuration on startup

## Running the Test

```bash
./test.sh
```

## Success Criteria

- Concurrency flags rendered correctly in Helm templates
- No flags present when limits are 0 (unlimited)
- Backpressure metrics registered on the metrics endpoint
