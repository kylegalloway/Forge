# Dockerfile for Zarf CLI container image
# Used by Forge to run build jobs in Kubernetes
#
# The official Zarf project does not publish a CLI container image,
# so Forge users must build this image themselves.

FROM alpine:3.20

# Zarf version to install
ARG ZARF_VERSION=v0.68.1

# Install dependencies
RUN apk add --no-cache \
    ca-certificates \
    git \
    curl \
    bash

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

# Verify installation
RUN zarf version

ENTRYPOINT ["/usr/local/bin/zarf"]
CMD ["--help"]
