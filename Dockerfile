# Multi-stage build for efficient Go application
FROM golang:1.27.3-alpine AS builder

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
FROM alpine:3.24

# Install ca-certificates for HTTPS and tzdata for timezone support
# Also install sudo for elevated privileges
# 20261020-r0 drops the roots behind the compromised eMudhra intermediates
# (CVE-2026-111943); the floor also busts a stale build cache.
RUN apk --no-cache add 'ca-certificates>=20261020-r0' tzdata sudo

# Create non-root user with sudo privileges for chown operations
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    echo "appuser ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/appuser && \
    chmod 0440 /etc/sudoers.d/appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/traefik-acme-converter /app/traefik-acme-converter

# Copy entrypoint script
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Create default directories with appropriate permissions
RUN mkdir -p /traefik/certs /app/output && \
    chown -R appuser:appuser /app/output

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

# Set default non-root user for security (satisfies Docker Scout)
# Override with --user root or user: "0:0" in docker-compose when USER_UID=0 is needed
USER appuser

# Use entrypoint script to handle user switching
ENTRYPOINT ["/app/entrypoint.sh"]
