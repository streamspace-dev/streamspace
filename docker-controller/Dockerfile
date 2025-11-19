# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy source code
COPY . .

# Download dependencies and generate go.sum if missing
RUN go mod tidy && go mod download

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o docker-controller ./cmd/main.go

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /app/docker-controller /app/docker-controller

# Run as non-root user
RUN adduser -D -u 1000 controller
USER controller

ENTRYPOINT ["/app/docker-controller"]
