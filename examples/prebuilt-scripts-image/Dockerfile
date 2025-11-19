FROM alpine:3.18

# Install common utilities
RUN apk add --no-cache \
    bash \
    curl \
    jq \
    python3 \
    py3-pip

# Create scripts directory
RUN mkdir -p /scripts

# Copy pre-built scripts
COPY scripts/ /scripts/

# Make all scripts executable
RUN chmod +x /scripts/*.sh /scripts/*.py

# Set scripts directory in PATH
ENV PATH="/scripts:${PATH}"

# Set working directory
WORKDIR /workspace

# Default command (can be overridden)
CMD ["/bin/sh"]
