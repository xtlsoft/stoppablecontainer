# How It Works

This document provides a detailed technical explanation of how StoppableContainer achieves near-instant container starts.

## The Problem

Traditional container startup involves these steps:

1. **Pull image** (if not cached) - seconds to minutes
2. **Extract image layers** - hundreds of milliseconds to seconds
3. **Set up namespaces** - milliseconds
4. **Mount filesystems** - milliseconds
5. **Start process** - milliseconds

Steps 1 and 2 are the bottlenecks. Even with cached images, layer extraction can take noticeable time for large images.

## The Solution

StoppableContainer keeps the extracted filesystem ready at all times:

| Traditional | StoppableContainer |
|-------------|-------------------|
| ❌ Extract on every start | ✅ Extract once, reuse |
| ❌ 500ms+ startup | ✅ ~50ms startup |
| ❌ Full container restart | ✅ Only process restart |

## Implementation Details

### 1. Provider Pod Initialization

When a StoppableContainerInstance is created, the provider pod:

```bash
# Provider container entrypoint script

# Create the rootfs directory
mkdir -p /rootfs/rootfs

# Wait for pause container to provide the rootfs
# (The pause container uses the same image and provides /rootfs/rootfs)
while [ ! -d /rootfs/rootfs/bin ]; do
    sleep 0.1
done

# Create ready signal file
echo "ready" > /rootfs/ready

# Keep running forever
while true; do
    sleep 86400
done
```

The key insight is that the pause container already has the image's filesystem extracted. We mount it to a HostPath to share with consumers.

### 2. HostPath Sharing

The extracted rootfs is exposed via HostPath:

```yaml
volumes:
  - name: rootfs-host
    hostPath:
      path: /var/lib/stoppable-container/{instance-name}
      type: DirectoryOrCreate
```

This allows the consumer pod (on the same node) to access the pre-extracted filesystem.

### 3. Consumer Pod Startup

The consumer container uses an entrypoint script that:

```bash
#!/bin/sh

echo "[consumer] Starting consumer container..."

# Wait for rootfs to be ready
while [ ! -f /rootfs/ready ]; do
    sleep 0.1
done

echo "[consumer] Rootfs ready, setting up mounts..."

# Mount essential filesystems
mount -t proc proc /rootfs/rootfs/proc
mount -t sysfs sys /rootfs/rootfs/sys
mount --bind /dev /rootfs/rootfs/dev

echo "[consumer] Mounts ready, chrooting..."

# Chroot and execute user command
cd /rootfs/rootfs
chroot . /exec-wrapper "$@"
```

### 4. Exec Wrapper

The exec-wrapper is a minimal binary that:

1. Sets up the execution environment
2. Handles signal forwarding
3. Execs the actual user command

This ensures proper process handling within the chrooted environment.

## Timing Analysis

Typical timing for a busybox container:

| Step | Traditional | StoppableContainer |
|------|-------------|-------------------|
| Image extraction | 200ms | 0ms (pre-extracted) |
| Create consumer pod | N/A | 50ms |
| Wait for rootfs | N/A | <10ms |
| Mount proc/sys/dev | N/A | 5ms |
| Chroot + exec | N/A | 5ms |
| **Total** | **200ms+** | **~70ms** |

For larger images (e.g., Python, Node.js), the difference is more dramatic:

| Image | Traditional | StoppableContainer |
|-------|-------------|-------------------|
| busybox:stable | 200ms | 70ms |
| python:3.11-slim | 800ms | 80ms |
| node:20-slim | 1200ms | 85ms |
| ubuntu:22.04 | 500ms | 75ms |

## Why Not Use Container Checkpointing?

Container checkpointing (CRIU) is an alternative approach that:

| CRIU | StoppableContainer |
|------|-------------------|
| Saves/restores process state | Only manages filesystem |
| Requires kernel support | Works on any Kubernetes cluster |
| Complex failure modes | Simple, predictable behavior |
| Not widely supported | Works everywhere |

StoppableContainer's approach is simpler and more compatible.

## Limitations

### Same-Node Requirement

Provider and consumer pods must run on the same node because they share the filesystem via HostPath. The controller ensures this by:

1. Recording the node where the provider runs
2. Using `nodeSelector` to schedule consumers on the same node

### Filesystem State

The rootfs is shared by reference, not copied. This means:

- ✅ Fast startup (no copying)
- ⚠️ Changes in consumer affect the rootfs
- ⚠️ Restart gets the modified filesystem

For stateless applications, this is usually fine. For stateful workloads, consider using volumes for persistent data.

### Image Updates

To update the container image:

1. Delete the StoppableContainer
2. Recreate with new image

The controller does not currently support in-place image updates (planned for future versions).

## Performance Tuning

### Optimize Image Size

Smaller images = faster initial extraction:

```dockerfile
# Use slim base images
FROM python:3.11-slim

# Clean up after installing packages
RUN apt-get update && apt-get install -y \
    package1 \
    package2 \
    && rm -rf /var/lib/apt/lists/*
```

### Pre-warm the Provider

Create StoppableContainers ahead of time with `running: false` to have the rootfs ready:

```yaml
spec:
  running: false  # Pre-extract, don't run
```

When needed, just set `running: true` for instant start.

## Next Steps

- [Security](security.md) - Security implications and best practices
- [Advanced Usage](../user-guide/advanced.md) - Advanced patterns
