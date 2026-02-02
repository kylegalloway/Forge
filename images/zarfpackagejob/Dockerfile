# Dockerfile for Zarf CLI container image
# Used by Forge to run build/deploy/publish jobs in Kubernetes
#
# This image is published to ghcr.io/kylegalloway/forge/zarfpackagejob
# The official Zarf project does not publish a CLI container image,
# so Forge provides this containerized version.
#
# Includes:
# - Zarf CLI for package operations
# - AWS CLI for S3 publishing destinations
# - kubectl for in-cluster kubeconfig generation
# - Git, curl, and other utilities

FROM alpine:3.20

# Zarf version to install
ARG ZARF_VERSION=v0.69.0

# Install dependencies including AWS CLI and kubectl
# Note: aws-cli package provides full AWS CLI v2 functionality
# kubectl is needed for in-cluster kubeconfig generation and debugging
RUN apk add --no-cache \
    ca-certificates \
    git \
    curl \
    bash \
    aws-cli \
    kubectl \
    yq-go

# Set shell to use pipefail
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

# Download and install Zarf CLI
RUN ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') && \
    curl -sL "https://github.com/zarf-dev/zarf/releases/download/${ZARF_VERSION}/zarf_${ZARF_VERSION}_Linux_${ARCH}" \
    -o /usr/local/bin/zarf && \
    chmod +x /usr/local/bin/zarf

# Create non-root user for running Zarf
RUN adduser -D -u 1000 zarf && \
    mkdir -p /workspace /output && \
    chown -R zarf:zarf /workspace /output

USER zarf
WORKDIR /workspace

# Verify installations
RUN zarf version && aws --version && kubectl version --client

ENTRYPOINT ["/bin/sh"]
