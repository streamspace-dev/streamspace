# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o k8s-agent .

# Runtime stage
FROM alpine:latest

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 agent && \
    adduser -D -u 1000 -G agent agent

# Set working directory
WORKDIR /home/agent

# Copy binary from builder
COPY --from=builder /app/k8s-agent /usr/local/bin/k8s-agent

# Change ownership
RUN chown -R agent:agent /home/agent

# Switch to non-root user
USER agent

# Entrypoint
ENTRYPOINT ["k8s-agent"]
