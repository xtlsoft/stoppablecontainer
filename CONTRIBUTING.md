# Contributing to StoppableContainer

Thank you for your interest in contributing to StoppableContainer!

## Development Setup

### Prerequisites

- Go 1.22+
- Docker or Podman
- kubectl
- Kind (for local Kubernetes cluster)
- Make

### Local Development

1. Clone the repository:
   ```bash
   git clone https://github.com/xtlsoft/stoppablecontainer.git
   cd stoppablecontainer
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Install CRDs to your cluster:
   ```bash
   make install
   ```

4. Run the controller locally:
   ```bash
   make run
   ```

### Running Tests

```bash
# Unit tests
make test

# Lint
make lint

# E2E tests (requires Kind)
make test-e2e
```

### Building Images

```bash
# Build all images
make docker-build IMG=myregistry/stoppablecontainer:dev
make docker-build-exec-wrapper EXEC_WRAPPER_IMG=myregistry/stoppablecontainer-exec:dev
make docker-build-mount-helper MOUNT_HELPER_IMG=myregistry/stoppablecontainer-mount-helper:dev
```

## Code Style

- Follow standard Go conventions
- Run `make lint` before submitting PRs
- Add tests for new functionality

## Pull Request Process

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes
4. Add/update tests as needed
5. Run `make test && make lint`
6. Submit a pull request

## Architecture Overview

See [docs/DAEMONSET_ARCHITECTURE.md](docs/DAEMONSET_ARCHITECTURE.md) for the overall architecture.

### Key Components

- **Controller** (`cmd/main.go`): Kubernetes operator that manages StoppableContainer resources
- **mount-helper** (`cmd/mount-helper/`): DaemonSet that handles privileged mount operations
- **exec-wrapper** (`cmd/exec-wrapper/`): Helper for `kubectl exec` inside chroot
- **pause** (`cmd/pause/`): Static pause binary for rootfs container

## License

Apache License 2.0
