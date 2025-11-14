# Build stage
FROM golang:1.21-alpine AS builder

# Build arguments for versioning
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE

# Install build dependencies
RUN apk add --no-cache git make ca-certificates

WORKDIR /workspace

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/

# Build the API server with version info
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a \
    -ldflags "-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o api-server cmd/main.go

# Final stage - minimal runtime image
FROM alpine:3.19

# Labels for metadata
LABEL org.opencontainers.image.title="StreamSpace API"
LABEL org.opencontainers.image.description="StreamSpace API Backend"
LABEL org.opencontainers.image.vendor="StreamSpace"
LABEL org.opencontainers.image.source="https://github.com/yourusername/streamspace"

# Install runtime dependencies
RUN apk add --no-cache ca-certificates git tzdata wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /workspace/api-server .

# Create directory for repository clones
RUN mkdir -p /tmp/streamspace-repos && chmod 777 /tmp/streamspace-repos

# Run as non-root user
RUN addgroup -g 1000 streamspace && \
    adduser -D -u 1000 -G streamspace streamspace && \
    chown -R streamspace:streamspace /app /tmp/streamspace-repos

USER streamspace

# Expose API port
EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8000/health || exit 1

# Environment variables with defaults
ENV API_PORT=8000 \
    DB_HOST=localhost \
    DB_PORT=5432 \
    DB_USER=streamspace \
    DB_NAME=streamspace \
    GIN_MODE=release \
    SYNC_INTERVAL=1h

ENTRYPOINT ["/app/api-server"]
