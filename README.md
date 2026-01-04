# StoppableContainer Operator

[![Tests](https://github.com/xtlsoft/stoppablecontainer/actions/workflows/test.yml/badge.svg)](https://github.com/xtlsoft/stoppablecontainer/actions/workflows/test.yml)
[![E2E Tests](https://github.com/xtlsoft/stoppablecontainer/actions/workflows/test-e2e.yml/badge.svg)](https://github.com/xtlsoft/stoppablecontainer/actions/workflows/test-e2e.yml)
[![Lint](https://github.com/xtlsoft/stoppablecontainer/actions/workflows/lint.yml/badge.svg)](https://github.com/xtlsoft/stoppablecontainer/actions/workflows/lint.yml)

StoppableContainer is a Kubernetes operator that enables **stoppable containers** - containers whose ephemeral filesystem persists even when the workload is not running. This is similar to how KubeVirt handles VMs with VirtualMachine and VirtualMachineInstance.

## How It Works

```
┌─────────────────────────────────────────────────────────────────┐
│                    StoppableContainer CRD                        │
│  (User-facing resource, like VirtualMachine in KubeVirt)        │
│  spec.running: true/false                                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ creates/manages
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│               StoppableContainerInstance CRD                     │
│  (Running instance, like VirtualMachineInstance)                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ creates/manages
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   mount-helper DaemonSet                         │
│  (Privileged, runs on every node)                               │
│  - Scans for mount requests from provider pods                  │
│  - Creates overlayfs mounts on host                             │
│  - Mounts proc/dev/sys for consumer pods                        │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
┌─────────────────────────────┐ ┌─────────────────────────────────┐
│       Provider Pod          │ │       Consumer Pod               │
│  (NOT privileged!)          │ │  (Only CAP_SYS_CHROOT)          │
│  ┌───────────────────────┐  │ │  ┌─────────────────────────────┐│
│  │ rootfs container      │  │ │  │ consumer container          ││
│  │ (user image + pause)  │  │ │  │ chroot into mounted rootfs  ││
│  │ ROOTFS_MARKER=true    │  │ │  │ runs user command           ││
│  └───────────────────────┘  │ │  └─────────────────────────────┘│
│  ┌───────────────────────┐  │ └─────────────────────────────────┘
│  │ provider container    │  │
│  │ writes request.json   │  │
│  │ waits for ready.json  │  │
│  └───────────────────────┘  │
└─────────────────────────────┘
```

### Key Components

1. **mount-helper DaemonSet**: A privileged DaemonSet that runs on every node
   - Handles all privileged mount operations centrally
   - Finds rootfs containers by scanning `/proc` for `ROOTFS_MARKER=true`
   - Creates overlayfs mounts from container filesystem
   - One privileged pod per node instead of per workload

2. **Provider Pod**: A minimal pod that holds the container's filesystem
   - Uses the user's image as a sidecar container with an injected pause binary
   - Works with ANY image including scratch/distroless (no shell required)
   - **NOT privileged** - just writes request.json for DaemonSet
   - Uses HostToContainer mount propagation

3. **Consumer Pod**: The actual workload
   - Receives rootfs from DaemonSet via hostPath
   - Only requires `CAP_SYS_CHROOT` capability
   - **NO CAP_SYS_ADMIN needed** - DaemonSet handles all mounts

4. **Exec Wrapper**: A Go binary for proper `kubectl exec` support
   - Located at `/.sc-bin/sc-exec` in the consumer container
   - Use it to enter the chroot environment: `kubectl exec <pod> -- /.sc-bin/sc-exec <command>`

## Features

- ✅ Stop/Start containers while preserving ephemeral filesystem
- ✅ Similar UX to KubeVirt's VM/VMI pattern
- ✅ Works with ANY image (including scratch/distroless)
- ✅ **Secure by design** - No privileged user workloads
- ✅ Resource management for both provider and consumer
- ✅ Volume mounts support
- ✅ Environment variables
- ✅ Service account mapping

## Installation

### Prerequisites

- Kubernetes cluster v1.25+
- kubectl configured
- Container runtime: containerd (recommended) or Docker

### Quick Start with Helm

```bash
helm repo add stoppablecontainer https://xtlsoft.github.io/stoppablecontainer
helm install stoppablecontainer stoppablecontainer/stoppablecontainer -n stoppablecontainer-system --create-namespace
```

### Manual Installation

```bash
# Build and push images
make docker-build docker-push IMG=ghcr.io/<your-org>/stoppablecontainer:latest
make docker-build-exec-wrapper && docker push ghcr.io/<your-org>/stoppablecontainer-exec:latest
make docker-build-mount-helper && docker push ghcr.io/<your-org>/stoppablecontainer-mount-helper:latest

# Install CRDs
make install

# Deploy the mount-helper DaemonSet (required!)
make deploy-daemonset MOUNT_HELPER_IMG=ghcr.io/<your-org>/stoppablecontainer-mount-helper:latest

# Deploy the operator
make deploy IMG=ghcr.io/<your-org>/stoppablecontainer:latest
```

## Usage

### Create a StoppableContainer

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: my-container
spec:
  running: true  # Set to false to stop
  template:
    container:
      image: ubuntu:22.04
      command:
        - /bin/bash
        - -c
        - |
          echo "Hello from StoppableContainer!"
          sleep infinity
      resources:
        requests:
          memory: "256Mi"
          cpu: "250m"
  provider:
    resources:
      requests:
        memory: "32Mi"
        cpu: "10m"
```

Apply it:

```bash
kubectl apply -f my-container.yaml
```

### Check Status

```bash
kubectl get stoppablecontainer
kubectl get stoppablecontainerinstance
kubectl get pods
```

### Stop the Container (Preserve Filesystem)

```bash
kubectl patch stoppablecontainer my-container -p '{"spec":{"running":false}}' --type=merge
```

The consumer pod will be deleted, but the provider pod remains, preserving the filesystem.

### Start Again

```bash
kubectl patch stoppablecontainer my-container -p '{"spec":{"running":true}}' --type=merge
```

A new consumer pod is created with the preserved filesystem!

### Execute Commands

To execute commands inside the container's chroot environment, use the exec-wrapper:

```bash
# Use exec-wrapper to run commands in the chroot
kubectl exec -it <consumer-pod-name> -- /.sc-bin/sc-exec /bin/bash

# Example: List files in chroot
kubectl exec <consumer-pod-name> -- /.sc-bin/sc-exec ls /

# Note: Direct kubectl exec without exec-wrapper will run in the container's
# original filesystem, not the chroot. Use exec-wrapper for proper access.
```

### Delete

```bash
kubectl delete stoppablecontainer my-container
```

This deletes both pods and the preserved filesystem.

## Configuration

### StoppableContainer Spec

| Field | Description | Default |
|-------|-------------|---------|
| `spec.running` | Whether the container should be running | `false` |
| `spec.template.container.image` | Container image | Required |
| `spec.template.container.command` | Command to run | Image default |
| `spec.template.container.args` | Command arguments | - |
| `spec.template.container.resources` | Resource requirements | - |
| `spec.template.container.env` | Environment variables | - |
| `spec.template.container.volumeMounts` | Volume mounts | - |
| `spec.template.volumes` | Pod volumes | - |
| `spec.provider.resources` | Provider pod resources | Minimal |
| `spec.hostPathPrefix` | Host path for mount propagation | `/var/lib/stoppablecontainer` |

## Security Considerations

StoppableContainer uses a DaemonSet-based architecture for improved security:

| Component | Privilege Level | Notes |
|-----------|----------------|-------|
| mount-helper DaemonSet | Privileged | 1 per node, centrally managed |
| Provider Pod | **Non-privileged** | No special capabilities |
| Consumer Pod | CAP_SYS_CHROOT only | Minimal privilege for chroot |

**Key Security Benefits:**
1. **No privileged user workloads** - User code never runs with elevated privileges
2. **Centralized audit** - All privileged operations go through the DaemonSet
3. **Reduced blast radius** - Compromise of user pod doesn't grant mount capabilities
4. **Pod Security Standards compatible** - Consumer pods can run with restricted PSS

**Requirements:**
- The mount-helper DaemonSet requires `hostPID` and privileged mode
- Uses hostPath volumes for mount propagation
- Consumer uses chroot which requires `CAP_SYS_CHROOT`

## Development

```bash
# Run locally (requires CRDs and DaemonSet deployed)
make run

# Run unit tests
make test

# Run E2E tests (uses Kind)
make test-e2e

# Generate manifests
make manifests

# Build all images
make docker-build docker-build-exec-wrapper docker-build-mount-helper
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test && make lint`
5. Submit a pull request

## License

Apache License 2.0

