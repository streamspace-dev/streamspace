# Build Docker Images

Build Docker images for StreamSpace components.

Component: $ARGUMENTS (api, k8s-agent, docker-agent, or ui)

## Build Image
!docker build -t streamspace/$ARGUMENTS:latest -f $ARGUMENTS/Dockerfile .

## Verify Build
!docker images streamspace/$ARGUMENTS

## Optional: Test Image
!docker run --rm streamspace/$ARGUMENTS:latest --version

## Build All Components

If $ARGUMENTS is empty or "all":
1. Build API image
2. Build K8s Agent image
3. Build Docker Agent image
4. Build UI image

Show:
- Build status for each component
- Image sizes
- Any build errors or warnings
- Tag information

## Optimization Tips

After build, suggest:
- Multi-stage build improvements
- Layer caching optimization
- Unnecessary file exclusions (.dockerignore)
- Base image updates
