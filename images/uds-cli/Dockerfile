# Dockerfile for UDS CLI container image
# Used by Forge to run UDS bundle jobs in Kubernetes
#
# This image is published to ghcr.io/kylegalloway/forge/uds-cli
# While Defense Unicorns publishes an official UDS CLI image,
# Forge provides this version for consistency and to ensure
# all required dependencies are included.

FROM alpine:3.20

# UDS CLI version to install
ARG UDS_VERSION=v0.27.21

# Install dependencies
RUN apk add --no-cache \
    ca-certificates \
    git \
    curl \
    bash \
    docker-cli

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

# Verify installation
RUN uds version

ENTRYPOINT ["/usr/local/bin/uds"]
CMD ["--help"]
