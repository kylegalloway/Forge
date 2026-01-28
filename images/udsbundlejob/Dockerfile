# Dockerfile for UDS CLI container image
# Used by Forge to run create/deploy/publish jobs in Kubernetes
#
# This image is published to ghcr.io/kylegalloway/forge/udsbundlejob
# While Defense Unicorns publishes an official UDS CLI image,
# Forge provides this version for consistency and to ensure
# all required dependencies are included.
#
# Includes:
# - UDS CLI for bundle operations
# - AWS CLI for S3 publishing destinations
# - kubectl for in-cluster kubeconfig generation
# - Docker CLI for container operations
# - Git, curl, and other utilities

FROM alpine:3.20

# UDS CLI version to install
ARG UDS_VERSION=v0.27.21

# Install dependencies including AWS CLI and kubectl
# Note: aws-cli package provides full AWS CLI v2 functionality
# kubectl is needed for in-cluster kubeconfig generation and debugging
RUN apk add --no-cache \
    ca-certificates \
    git \
    curl \
    bash \
    docker-cli \
    aws-cli \
    kubectl

# Set shell to use pipefail
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

# Download and install UDS CLI
RUN ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') && \
    curl -sL "https://github.com/defenseunicorns/uds-cli/releases/download/${UDS_VERSION}/uds-cli_${UDS_VERSION}_Linux_${ARCH}" \
    -o /usr/local/bin/uds && \
    chmod +x /usr/local/bin/uds

# Create non-root user for running UDS
# Using UID 65532 to match the DefaultUDSUID constant
RUN adduser -D -u 65532 uds && \
    mkdir -p /workspace /output && \
    chown -R uds:uds /workspace /output

USER uds
WORKDIR /workspace

# Verify installations
RUN uds version && aws --version && kubectl version --client

ENTRYPOINT ["/bin/sh"]
