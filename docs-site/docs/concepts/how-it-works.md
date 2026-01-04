# How It Works

This document provides a detailed technical explanation of how StoppableContainer achieves persistent root filesystems.

## The Problem

Traditional Kubernetes containers have ephemeral filesystems:

1. **No persistence** - Any files created or packages installed are lost when the pod is deleted
2. **No state across restarts** - Even if you restart a pod, the filesystem starts fresh
3. **Volume limitations** - You can only persist data in explicitly mounted volumes, not the entire rootfs

This is problematic for:

- Development environments where you want to install packages and tools
- Workloads that modify configuration files at runtime
- Educational platforms where students need persistent environments

## The Solution

StoppableContainer keeps the container's rootfs intact across stop/start cycles:

| Traditional | StoppableContainer |
|-------------|-------------------|
| ❌ Filesystem lost on pod delete | ✅ Filesystem preserved |
| ❌ Packages must be in image | ✅ Install packages at runtime |
| ❌ No modification persistence | ✅ All changes survive restarts |

## Implementation Details

### 1. DaemonSet-Based Mount Architecture

The mount-helper DaemonSet runs on every node and handles all privileged operations:

```bash
# mount-helper scans for provider containers
for each provider_pod:
    # Find the rootfs container by ROOTFS_MARKER env var
    pid = find_rootfs_container_pid()
    
    # Access the container's filesystem via /proc
    rootfs_path = /proc/{pid}/root
    
    # Create overlayfs mount
    mount -t overlay overlay \
        -o lowerdir={rootfs_path},upperdir={upper},workdir={work} \
        {target_path}
    
    # Signal readiness
    write ready.json
```

This approach allows the rootfs to persist because:

1. The **provider pod** stays running even when the workload is stopped
2. The **overlayfs upper layer** captures all modifications
3. The **mount-helper** maintains the mount as long as the provider is alive

### 2. HostPath Sharing

The rootfs is shared between provider and consumer via HostPath:

```yaml
volumes:
  - name: rootfs-host
    hostPath:
      path: /var/lib/stoppablecontainer/{instance-name}
      type: DirectoryOrCreate
```

This allows the consumer pod (on the same node) to access the mounted filesystem with all modifications.

### 3. Consumer Pod Startup

The consumer container uses the exec-wrapper binary that:

1. Waits for the rootfs to be ready (checks for ready.json)
2. Chroots into the mounted rootfs
3. Executes the user's command

```bash
# Simplified exec-wrapper logic
wait_for_ready "/rootfs/ready.json"
chroot /rootfs /bin/sh -c "$USER_COMMAND"
```

### 4. Exec Wrapper

The exec-wrapper (`/.sc-bin/sc-exec`) is a Go binary that:

1. Sets up the execution environment inside the chroot
2. Handles signal forwarding
3. Finds and executes the requested command

When you run `kubectl sc exec my-app -- /bin/bash`, it automatically uses the exec-wrapper to enter the chroot environment.

## Why Overlayfs?

StoppableContainer uses overlayfs for the rootfs mount:

```
┌─────────────────────────────────────┐
│           Merged View               │ ← What the consumer sees
├─────────────────────────────────────┤
│  Upper Layer (read-write)           │ ← Modifications stored here
├─────────────────────────────────────┤
│  Lower Layer (read-only)            │ ← Original container image
└─────────────────────────────────────┘
```

Benefits:

1. **Efficient storage**: Only modifications are stored in the upper layer
2. **Image integrity**: Original image is never modified
3. **Quick restarts**: No need to copy the entire filesystem

## Comparison with Alternatives

### vs. Container Checkpointing (CRIU)

| CRIU | StoppableContainer |
|------|-------------------|
| Saves/restores process state | Only manages filesystem |
| Requires kernel support | Works on any Kubernetes cluster |
| Complex failure modes | Simple, predictable behavior |
| Not widely supported | Works everywhere |
| Complex failure modes | Simple, predictable behavior |
| Not widely supported | Works everywhere |

StoppableContainer's approach is simpler and more compatible.

## Limitations

### Same-Node Requirement

Provider and consumer pods must run on the same node because they share the filesystem via HostPath. The controller ensures this by:

1. Recording the node where the provider runs
2. Using `nodeSelector` to schedule consumers on the same node

### Filesystem State

The rootfs is persistent and isolated via overlayfs:

- ✅ Each instance has its own writable layer
- ✅ Changes persist across container restarts
- ✅ Base image remains unchanged (copy-on-write)
- ⚠️ Each StoppableContainerInstance has isolated changes

This is the primary benefit of StoppableContainer - your filesystem modifications survive pod restarts.

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
