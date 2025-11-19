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
    ca-certificates

# Install Zarf CLI
ARG ZARF_VERSION=v0.66.0
RUN curl -sL "https://github.com/defenseunicorns/zarf/releases/download/${ZARF_VERSION}/zarf_${ZARF_VERSION}_Linux_amd64" \
    -o /usr/local/bin/zarf && \
    chmod +x /usr/local/bin/zarf

# Create scripts directory
RUN mkdir -p /scripts

# Copy Zarf build scripts
COPY scripts/*.sh /scripts/
RUN chmod +x /scripts/*.sh

# Set working directory
WORKDIR /workspace

# Default command shows help
CMD ["zarf", "version"]
