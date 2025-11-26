# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Download dependencies
RUN go mod download

# Copy the source code
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build the controller
# Use TARGETARCH to build for the target platform (set by Docker/Podman automatically)
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -a -o controller cmd/controller/main.go

# Runtime stage
FROM alpine:3.20

WORKDIR /

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy the controller binary from builder
COPY --from=builder /workspace/controller .

# Run as non-root user
USER 65532:65532

ENTRYPOINT ["/controller"]
