# Build stage
FROM golang:1.21 AS builder

# Build arguments for versioning
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE
# Docker Buildx automatically provides TARGETARCH
ARG TARGETARCH

WORKDIR /workspace

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build the controller with version info for the target architecture
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -a \
    -ldflags "-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o manager cmd/main.go

# Final stage - minimal runtime image
FROM gcr.io/distroless/static:nonroot

# Labels for metadata
LABEL org.opencontainers.image.title="StreamSpace Controller"
LABEL org.opencontainers.image.description="Kubernetes controller for StreamSpace platform"
LABEL org.opencontainers.image.vendor="StreamSpace"
LABEL org.opencontainers.image.source="https://github.com/yourusername/streamspace"

WORKDIR /

# Copy controller binary
COPY --from=builder /workspace/manager .

# Use nonroot user (distroless default)
USER 65532:65532

ENTRYPOINT ["/manager"]
