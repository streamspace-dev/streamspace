# Build arguments for versioning
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE

# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY package.json package-lock.json* ./

# Install dependencies
RUN npm ci

# Copy source code
COPY . .

# Pass build args to Vite build
ARG VERSION
ARG COMMIT
ARG BUILD_DATE

ENV VITE_VERSION=${VERSION}
ENV VITE_COMMIT=${COMMIT}
ENV VITE_BUILD_DATE=${BUILD_DATE}

# Build the app
RUN npm run build

# Production stage - serve with nginx
FROM nginx:1.25-alpine

# Build arguments for labels
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE

# OCI-compliant labels
LABEL org.opencontainers.image.title="StreamSpace UI"
LABEL org.opencontainers.image.description="Web-based user interface for StreamSpace platform"
LABEL org.opencontainers.image.vendor="StreamSpace"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.created="${BUILD_DATE}"
LABEL org.opencontainers.image.revision="${COMMIT}"
LABEL org.opencontainers.image.source="https://github.com/JoshuaAFerguson/streamspace"
LABEL org.opencontainers.image.licenses="MIT"

# Copy built assets from builder
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Expose port 80
EXPOSE 80

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost/ || exit 1

# Run nginx
CMD ["nginx", "-g", "daemon off;"]
