# StreamSpace Container Images

This directory contains standardized container images for StreamSpace sessions.

## Image Design Philosophy

StreamSpace images are designed with the following principles:

1. **Protocol Standardization**: All images expose streaming on a consistent port
2. **Security First**: Run as non-root, minimal attack surface
3. **Performance Optimized**: Hardware acceleration support, optimized codecs
4. **Kubernetes Ready**: Health checks, resource limits, graceful shutdown

## Available Images

### chrome-selkies

Chrome browser with Selkies-GStreamer WebRTC streaming.

**Features:**
- Google Chrome stable
- Selkies WebRTC streaming (low latency)
- Hardware acceleration (NVENC, VA-API)
- Audio support
- Clipboard sharing

**Build:**
```bash
cd chrome-selkies
docker build -t ghcr.io/streamspace-dev/chrome-selkies:latest .
```

**Test locally:**
```bash
docker run -p 8080:8080 ghcr.io/streamspace-dev/chrome-selkies:latest
# Open http://localhost:8080 in browser
```

## Image Standards

### Ports

| Protocol | Port | Description |
|----------|------|-------------|
| Selkies WebRTC | 8080 | Primary streaming port |
| VNC (fallback) | 5900 | VNC protocol |
| noVNC (fallback) | 6080 | Web VNC |

### Environment Variables

All images should support these standard variables:

| Variable | Default | Description |
|----------|---------|-------------|
| DISPLAY_WIDTH | 1920 | Display width |
| DISPLAY_HEIGHT | 1080 | Display height |
| DISPLAY_DPI | 96 | Display DPI |
| PUID | 1000 | User ID |
| PGID | 1000 | Group ID |
| TZ | UTC | Timezone |

### Labels

All images should include these OCI labels:

```dockerfile
LABEL org.opencontainers.image.title="StreamSpace <App Name>"
LABEL org.opencontainers.image.description="<Description>"
LABEL org.opencontainers.image.version="<Version>"
LABEL org.opencontainers.image.vendor="StreamSpace"
LABEL org.opencontainers.image.source="https://github.com/streamspace-dev/streamspace"
```

### Health Checks

All images must include a health check:

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8080/ || exit 1
```

## Building Images

### Local Build

```bash
cd images/<image-name>
docker build -t ghcr.io/streamspace-dev/<image-name>:latest .
```

### CI/CD Build

Images are automatically built and pushed to GHCR on:
- Push to main branch
- Release tags

## Testing Images

### Quick Test

```bash
# Run the image
docker run -d -p 8080:8080 --name test-session ghcr.io/streamspace-dev/<image>:latest

# Check health
docker inspect --format='{{.State.Health.Status}}' test-session

# View logs
docker logs test-session

# Cleanup
docker rm -f test-session
```

### Integration Test

```bash
# Run with StreamSpace agent
./scripts/test-image.sh ghcr.io/streamspace-dev/<image>:latest
```

## LinuxServer Compatibility

For maximum compatibility with LinuxServer images, StreamSpace images can also be built to expose port 3000 with KasmVNC:

```dockerfile
# Alternative: LinuxServer-compatible base
FROM lscr.io/linuxserver/baseimage-kasmvnc:ubuntujammy
```

This provides compatibility with existing LinuxServer catalog images.

## Creating New Images

1. Create a new directory under `images/`
2. Copy the template from an existing image
3. Modify the Dockerfile for your application
4. Update the entrypoint script
5. Add to the template catalog in the API
6. Test locally before pushing

## Future Images

Planned images for StreamSpace:

- [ ] `firefox-selkies` - Firefox with Selkies WebRTC
- [ ] `vscode-selkies` - VS Code with Selkies WebRTC
- [ ] `ubuntu-desktop` - Full Ubuntu desktop
- [ ] `blender-selkies` - Blender 3D with GPU acceleration
- [ ] `gimp-selkies` - GIMP image editor
