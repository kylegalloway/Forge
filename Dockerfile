# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Download dependencies
RUN go mod download

# Copy the source code
COPY cmd/ cmd/
COPY pkg/ pkg/

# Use TARGETARCH to build for the target platform (set by Docker/Podman automatically)
ARG TARGETARCH

# Build binaries for both controller and webhook
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -a -o controller cmd/controller/main.go && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -a -o webhook cmd/webhook/main.go

# Controller runtime stage
FROM alpine:3.20 AS controller

WORKDIR /

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy the controller binary from builder
COPY --from=builder /workspace/controller .

# Run as non-root user
USER 65532:65532

ENTRYPOINT ["/controller"]

# Webhook runtime stage
FROM alpine:3.20 AS webhook

WORKDIR /

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy the webhook binary from builder
COPY --from=builder /workspace/webhook .

# Run as non-root user
USER 65532:65532

ENTRYPOINT ["/webhook"]
