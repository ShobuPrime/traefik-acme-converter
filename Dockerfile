# Multi-stage build for efficient Go application
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application with optimizations for AMD64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o traefik-acme-converter \
    .

# Final stage - minimal runtime image
FROM alpine:latest

# Install ca-certificates for HTTPS and tzdata for timezone support
RUN apk --no-cache add ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/traefik-acme-converter /app/traefik-acme-converter

# Create default directories
RUN mkdir -p /traefik/certs /app/output

# Run as root by default to allow chown operations
# Users can override with --user flag if they want to run as non-root

# Add OCI labels for supply chain attestation
LABEL org.opencontainers.image.source="https://github.com/ShobuPrime/traefik-acme-converter" \
      org.opencontainers.image.description="Traefik ACME certificate converter with fsnotify" \
      org.opencontainers.image.licenses="GPL-3.0" \
      org.opencontainers.image.title="traefik-acme-converter" \
      org.opencontainers.image.vendor="ShobuPrime"

# Set default environment variables
ENV WATCH_DIR=/traefik/certs \
    PROVIDER=ACME \
    OUTPUT_DIR=/app/output \
    USER_UID=1000 \
    USER_GID=1000 \
    USE_WATCHER=true

# Run the application
CMD ["/app/traefik-acme-converter"]
