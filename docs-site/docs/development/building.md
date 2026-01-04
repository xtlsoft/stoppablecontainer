# Building

This guide covers how to build StoppableContainer from source.

## Prerequisites

- Go 1.21 or later
- Docker (for building images)
- make
- kubectl (for deployment)

## Quick Build

```bash
# Clone the repository
git clone https://github.com/xtlsoft/stoppablecontainer.git
cd stoppablecontainer

# Build the controller binary
make build

# Build the Docker image
make docker-build IMG=stoppablecontainer:latest
```

## Build Targets

### Controller Binary

Build the controller binary:

```bash
make build
```

Output: `bin/manager`

### Docker Image

Build the multi-architecture Docker image:

```bash
make docker-build IMG=your-registry/stoppablecontainer:tag
```

Push to registry:

```bash
make docker-push IMG=your-registry/stoppablecontainer:tag
```

Build and push in one command:

```bash
make docker-buildx IMG=your-registry/stoppablecontainer:tag
```

### Exec Wrapper

Build the exec wrapper binary:

```bash
go build -o bin/exec-wrapper ./cmd/exec-wrapper
```

### Pause Binary

Build the pause container binary:

```bash
go build -o bin/pause ./cmd/pause
```

## Generated Code

### Regenerate All

```bash
make generate
make manifests
```

### What Gets Generated

| Target | Description |
|--------|-------------|
| `make generate` | DeepCopy methods, client code |
| `make manifests` | CRD YAML, RBAC rules |

## Build Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `IMG` | `controller:latest` | Docker image name and tag |
| `PLATFORMS` | `linux/amd64,linux/arm64` | Target platforms for multi-arch build |
| `GOFLAGS` | (empty) | Additional Go build flags |

### Customizing the Build

Modify `Makefile` or pass variables:

```bash
# Custom image
make docker-build IMG=my-registry/sc:v1.0.0

# Debug build
make build GOFLAGS="-gcflags='all=-N -l'"

# Single platform
make docker-build PLATFORMS=linux/amd64
```

## Multi-Architecture Builds

Build for multiple architectures:

```bash
# Requires docker buildx
make docker-buildx IMG=my-registry/stoppablecontainer:latest
```

Supported platforms:

- `linux/amd64`
- `linux/arm64`

## Verification

### Run Unit Tests

```bash
make test
```

### Run Linter

```bash
make lint
```

### Run E2E Tests

```bash
make test-e2e
```

## Dockerfile Details

The Dockerfile uses a multi-stage build:

```dockerfile
# Stage 1: Build
FROM golang:1.21 AS builder
WORKDIR /workspace
COPY . .
RUN CGO_ENABLED=0 go build -o manager cmd/main.go

# Stage 2: Runtime
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /workspace/manager /manager
ENTRYPOINT ["/manager"]
```

### Optimizations

- Multi-stage build for smaller images
- Distroless base image
- Non-root user
- CGO disabled for static binary

## Troubleshooting Builds

### Go Module Issues

```bash
# Clean module cache
go clean -modcache

# Verify modules
go mod verify

# Update dependencies
go mod tidy
```

### Docker Build Issues

```bash
# Clean Docker cache
docker builder prune

# Build with no cache
make docker-build IMG=sc:latest DOCKER_BUILD_ARGS="--no-cache"
```

### Code Generation Issues

```bash
# Clean generated files
rm -rf api/v1alpha1/zz_generated.*
rm -rf config/crd/bases/*

# Regenerate
make generate manifests
```

## Next Steps

- [Testing](testing.md) - Run tests
- [Contributing](contributing.md) - Contribution guidelines
