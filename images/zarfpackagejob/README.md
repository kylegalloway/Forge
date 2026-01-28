# Zarf Package Job Container Image

This directory contains a Dockerfile for building the Forge `zarfpackagejob` container image (wraps the Zarf CLI for in-cluster execution).

## Why This Exists

The Zarf project distributes their CLI as binaries only - they do not publish container images. However, Forge needs to run Zarf commands inside Kubernetes Job pods, which requires a container image.

This Dockerfile packages the official Zarf CLI binary into an Alpine-based container image.

## Building the Image

```bash
# Build for your local architecture
docker build -t localhost/zarfpackagejob:v0.11.1 images/zarfpackagejob/

# Or build for a specific Zarf version
docker build -t localhost/zarfpackagejob:v0.11.1 \
  --build-arg ZARF_VERSION=v0.42.0 \
  images/zarfpackagejob/

# Build multi-arch (requires docker buildx)
docker buildx build --platform linux/amd64,linux/arm64 \
  -t localhost/zarfpackagejob:v0.11.1 \
  images/zarfpackagejob/ --push
```

## For Kind Testing

```bash
# Build the image
docker build -t localhost/zarfpackagejob:v0.11.1 images/zarfpackagejob/

# Load into Kind cluster
kind load docker-image localhost/zarfpackagejob:v0.11.1 --name forge-demo
```

## For Production

For production deployments, you should:

1. Build this image and push to your internal container registry
2. Update `pkg/constants/config.go` or Helm values to reference your registry image:
   ```go
   DefaultZarfCLIImage = "your-registry.io/zarfpackagejob:v0.11.1"
   ```
3. Or use Helm values to override the image (if/when that feature is added)

## Image Contents

- **Base**: Alpine Linux 3.20
- **Zarf CLI**: Downloaded from official GitHub releases
- **Dependencies**: git, curl, bash, ca-certificates
- **User**: Runs as non-root user (UID 1000)
- **Workdir**: `/workspace`

## Notes

- The image tag should match the version in `pkg/constants/config.go`
- The official Zarf releases are at: https://github.com/zarf-dev/zarf/releases
- This is a stopgap until Zarf publishes official container images
