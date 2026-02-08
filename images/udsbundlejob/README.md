# UDS Bundle Job Container Image

This directory contains a Dockerfile for building the Forge `udsbundlejob` container image (wraps the UDS CLI for in-cluster execution).

## Why This Exists

While Defense Unicorns publishes an official UDS CLI image at `ghcr.io/defenseunicorns/uds-cli`,
Forge provides this custom version to ensure consistency with the Zarf CLI image and to include
all required dependencies for running UDS bundle operations in Kubernetes Jobs.

## Building the Image

```bash
# Build for your local architecture
docker build -t localhost/udsbundlejob:v0.11.17 images/udsbundlejob/

# Or build for a specific UDS version
docker build -t localhost/udsbundlejob:v0.11.17 \
  --build-arg UDS_VERSION=v0.28.0 \
  images/udsbundlejob/

# Build multi-arch (requires docker buildx)
docker buildx build --platform linux/amd64,linux/arm64 \
  -t localhost/udsbundlejob:v0.11.17 \
  images/udsbundlejob/ --push
```

## For Kind Testing

```bash
# Build the image
docker build -t localhost/udsbundlejob:v0.11.17 images/udsbundlejob/

# Load into Kind cluster
kind load docker-image localhost/udsbundlejob:v0.11.17 --name forge-demo
```

## For Production

For production deployments, you can either:

1. Use the official Defense Unicorns image: `ghcr.io/defenseunicorns/uds-cli:v0.27.13`
2. Build this image and push to your internal container registry
3. Override via Helm values or environment variables:
   - Helm: `udsCLI.image.repository` and `udsCLI.image.tag`
   - Env: `FORGE_UDS_CLI_IMAGE`

## Image Contents

- **Base**: Alpine Linux 3.20
- **UDS CLI**: Downloaded from official GitHub releases
- **Dependencies**: git, curl, bash, ca-certificates, docker-cli
- **User**: Runs as non-root user (UID 65532)
- **Workdir**: `/workspace`

## Updating the Version

Run the tool version update script to fetch the latest upstream releases (this updates Dockerfile ARGs only):

```bash
./scripts/update-tool-versions.sh
```

## Notes

- The image tag should match the version in `pkg/constants/config.go`
- The official UDS releases are at: https://github.com/defenseunicorns/uds-cli/releases
- UID 65532 is used to avoid conflicts with Zarf CLI (UID 1000)
