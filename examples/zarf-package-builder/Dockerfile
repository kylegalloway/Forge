# Zarf Package Builder Image
# This image contains the Zarf CLI and scripts for building Zarf packages
FROM alpine:3.19

# Install dependencies
RUN apk add --no-cache \
    bash \
    curl \
    git \
    jq \
    tar \
    gzip \
    ca-certificates \
    python3 \
    py3-pip \
    file

# Install Zarf CLI
ARG ZARF_VERSION=v0.66.0
ARG TARGETARCH
RUN ZARF_ARCH=$([ "$TARGETARCH" = "amd64" ] && echo "amd64" || echo "arm64") && \
    curl -sL "https://github.com/defenseunicorns/zarf/releases/download/${ZARF_VERSION}/zarf_${ZARF_VERSION}_Linux_${ZARF_ARCH}" \
    -o /usr/local/bin/zarf && \
    chmod +x /usr/local/bin/zarf

# Install Helm CLI
ARG HELM_VERSION=v3.14.0
ARG TARGETARCH
RUN HELM_ARCH=$([ "$TARGETARCH" = "amd64" ] && echo "amd64" || echo "arm64") && \
    curl -sL "https://get.helm.sh/helm-${HELM_VERSION}-linux-${HELM_ARCH}.tar.gz" | tar -xzv -C /tmp && \
    mv /tmp/linux-${HELM_ARCH}/helm /usr/local/bin/helm && \
    chmod +x /usr/local/bin/helm && \
    rm -rf /tmp/linux-${HELM_ARCH}

# Install AWS CLI
RUN pip3 install --no-cache-dir awscli --break-system-packages

# Create scripts directory
RUN mkdir -p /scripts

# Copy Zarf build scripts
COPY scripts/*.sh /scripts/
RUN chmod +x /scripts/*.sh

# Copy test package for local testing
COPY test-package/ /workspace/test-package/

# Set working directory
WORKDIR /workspace

# Default command shows help
CMD ["zarf", "version"]
