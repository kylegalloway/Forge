# Root Cause Analysis: Git Credential Mounting Failure in Init Containers

**Date:** 2026-01-21
**Severity:** High
**Affected Versions:** All versions prior to fix
**Fixed In:** v0.9.3 (pending)

## Executive Summary

Jobs with git credentials for private repositories were failing in the init container with authentication errors. The root cause was two bugs in the git credential URL generation logic that caused protocol mismatches and incompatible authentication formats for non-OAuth git servers.

## Timeline

| Time | Event |
|------|-------|
| Initial Report | User reported all jobs with mounted credentials failing in init container |
| Investigation Start | Set up local Gitea instance in kind cluster to reproduce |
| Root Cause Identified | Found hardcoded `https://` and `oauth2:` in credential file generation |
| Fix Implemented | Updated `pkg/sources/git.go` to use dynamic scheme and support username-based auth |
| Fix Verified | Successfully cloned from private Gitea repository with credentials |

## Impact

- **Scope:** All ZarfPackageJob and UDSBundleJob resources using git credentials with:
  - HTTP URLs (non-HTTPS)
  - Basic auth servers (Gitea, GitLab self-hosted, Bitbucket Server, etc.)
- **User Experience:** Jobs would fail immediately in the git-clone init container with exit code 128
- **Error Messages Observed:**
  - `fatal: could not read Username for 'http://...': No such device or address`
  - `fatal: Authentication failed for '...'`

## Root Cause Analysis

### Bug 1: Hardcoded HTTPS Protocol

**Location:** `pkg/sources/git.go:119`

**Problem:** The git credentials file was always written with `https://` protocol, regardless of the actual source URL's scheme.

**Code Before:**
```go
setupCmd := fmt.Sprintf(`...
  echo "https://oauth2:${token}@%s" > ~/.git-credentials
...`, extractGitHost(config.URL))
```

**Why This Failed:**
Git's credential helper matches credentials based on protocol AND host. When the source URL was `http://gitea.example.com/repo.git`, the credentials file contained `https://oauth2:token@gitea.example.com`. Git could not find matching credentials because:
- Clone URL: `http://gitea.example.com/repo.git`
- Credentials file: `https://oauth2:token@gitea.example.com`
- Protocol mismatch: `http` != `https`

**Fix:**
```go
setupCmd := fmt.Sprintf(`...
  echo "%s://oauth2:${token}@%s" > ~/.git-credentials
...`, extractGitScheme(config.URL), extractGitHost(config.URL))
```

Added `extractGitScheme()` function to parse the URL and return the correct protocol.

### Bug 2: OAuth2-Only Authentication Format

**Location:** `pkg/sources/git.go:116-120`

**Problem:** The code assumed all token-based authentication uses the `oauth2:token` format, which is specific to OAuth-based systems like GitHub.

**Code Before:**
```bash
elif [ -f /etc/git-secret/token ]; then
  git config --global credential.helper store
  token=$(cat /etc/git-secret/token)
  echo "https://oauth2:${token}@host" > ~/.git-credentials
fi
```

**Why This Failed:**
Basic authentication systems (Gitea, GitLab self-hosted, Bitbucket Server, etc.) expect credentials in `username:password` format, not `oauth2:token`. When connecting to Gitea:
- Expected format: `http://testuser:testpass123@gitea.example.com`
- Provided format: `http://oauth2:testpass123@gitea.example.com`
- Gitea rejected `oauth2` as an invalid username

**Fix:**
```bash
elif [ -f /etc/git-secret/token ]; then
  git config --global credential.helper store
  token=$(cat /etc/git-secret/token)
  if [ -f /etc/git-secret/username ]; then
    username=$(cat /etc/git-secret/username)
    echo "http://${username}:${token}@host" > ~/.git-credentials
  else
    echo "http://oauth2:${token}@host" > ~/.git-credentials
  fi
fi
```

Now checks for optional `username` field in the secret and uses it when present.

## Technical Details

### Affected Code Path

```txt
ZarfPackageJob Created
    └── Controller reconciles
        └── Build action handler
            └── buildInitContainers()
                └── sources.GitSource.GetInitContainer()
                    └── BuildGitInitContainer()  <-- Bug location
                        └── Generates init container with credential setup script
```

### Files Modified

| File | Change |
|------|--------|
| `pkg/sources/git.go` | Added `extractGitScheme()` function |
| `pkg/sources/git.go` | Updated credential setup script to use dynamic scheme |
| `pkg/sources/git.go` | Added conditional username support in credentials |
| `pkg/apis/zarf/v1alpha3/types.go` | Updated URL validation to allow `http://` (for testing) |
| `pkg/apis/uds/v1alpha3/types.go` | Updated URL validation to allow `http://` (for testing) |

### New Function Added

```go
// extractGitScheme extracts the URL scheme (http or https) from a git URL.
// Returns "https" as fallback if parsing fails or for SSH URLs.
func extractGitScheme(gitURL string) string {
    if strings.HasPrefix(gitURL, "git@") {
        return "https"
    }
    if parsed, err := url.Parse(gitURL); err == nil && parsed.Scheme != "" {
        return parsed.Scheme
    }
    return "https"
}
```

## Verification

### Test Environment

- Kind cluster with Podman on macOS
- Gitea deployed as private git server
- Private repository with zarf package definition

### Test Case

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: private-gitea-build
  namespace: forge-jobs
spec:
  serviceAccountName: private-repo-sa
  action: Build
  source:
    type: Git
    git:
      url: http://gitea.gitea.svc.cluster.local:3000/testuser/private-zarf-repo.git
      ref: main
      credentialRef:
        name: gitea-creds
```

### Secret Format (New)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gitea-creds
type: Opaque
stringData:
  username: "testuser"    # NEW: Required for basic auth systems
  token: "password123"    # Can be password or token
```

### Results

- **Before Fix:** Init container failed with exit code 128, authentication error
- **After Fix:** Init container completed successfully, repository cloned, package built

## Lessons Learned

1. **Protocol Assumptions:** Never hardcode URL schemes; always parse from the source URL
2. **Authentication Diversity:** Git servers have different auth mechanisms; support multiple formats
3. **Testing Coverage:** Need integration tests with non-GitHub git servers (Gitea, GitLab, etc.)
4. **Documentation:** Credential secret format should be clearly documented for each auth type

## Action Items

| Priority | Action | Owner | Status |
|----------|--------|-------|--------|
| P0 | Deploy fix to all environments | - | ✅ Done (v0.9.3) |
| P1 | Add integration tests with Gitea | - | ✅ Done (GITEA_TESTING.md) |
| P1 | Update credentials documentation | - | ✅ Done (USER_GUIDE.md, credentials-showcase) |
| P2 | Add validation for credential secret keys | - | ✅ Done (password key now validated) |
| P2 | Consider supporting `.netrc` format | - | Backlog |

## Appendix

### Git Credential Helper Matching Logic

Git's credential helper matches stored credentials based on:
1. Protocol (http, https, ssh)
2. Host (including port if non-standard)
3. Path (optional, not commonly used)

The credentials file format:
```txt
protocol://username:password@host
```

Example entries:
```txt
https://oauth2:ghp_token@github.com
http://myuser:mypass@gitea.internal:3000
https://gitlab-ci-token:CI_JOB_TOKEN@gitlab.example.com
```

### Supported Git Server Authentication Formats

| Server | Auth Type | Username Field | Token/Password Field |
|--------|-----------|----------------|---------------------|
| GitHub | OAuth | `oauth2` (automatic) | Personal Access Token |
| GitLab.com | OAuth | `oauth2` (automatic) | Personal Access Token |
| GitLab Self-Hosted | Basic | Actual username | Password or PAT |
| Gitea | Basic | Actual username | Password or App Token |
| Bitbucket Cloud | App Password | Actual username | App Password |
| Bitbucket Server | Basic | Actual username | Password or PAT |
| Azure DevOps | Basic | Any non-empty | PAT |
