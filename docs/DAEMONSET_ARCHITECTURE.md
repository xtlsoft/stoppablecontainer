# StoppableContainer DaemonSet Architecture

## Overview

This architecture uses a privileged DaemonSet (`mount-helper`) to handle all privileged mount operations, allowing both Provider and Consumer pods to run without elevated privileges.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Host Node                                       │
│                                                                              │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │         StoppableContainer mount-helper DaemonSet (privileged)         │ │
│  │                                                                        │ │
│  │  - Runs on every node                                                  │ │
│  │  - Polls for request.json files in hostPath directories               │ │
│  │  - Finds rootfs container by scanning /proc for ROOTFS_MARKER env     │ │
│  │  - Reads overlayfs options from /proc/PID/mounts                       │ │
│  │  - Recreates overlayfs mount on host with adjusted paths              │ │
│  │  - Pre-mounts proc/dev/sys for consumer                               │ │
│  │  - Writes ready.json to signal completion                             │ │
│  │                                                                        │ │
│  │  Capabilities: hostPID, privileged, Bidirectional mount propagation   │ │
│  │  Mounts: /host (host root), /var/lib/stoppablecontainer               │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                              │                                               │
│                              │ overlayfs mount on host                       │
│                              ▼                                               │
│  ┌──────────────────────────────────────────────────────────────────┐       │
│  │                 Provider Pod (NOT privileged!)                    │       │
│  │  ┌─────────────────────────┐  ┌─────────────────────────────────┐│       │
│  │  │  rootfs container       │  │  provider container             ││       │
│  │  │  (user's image)         │  │  (alpine, non-privileged)       ││       │
│  │  │                         │  │                                 ││       │
│  │  │  - Runs sc-pause binary │  │  - Writes request.json with     ││       │
│  │  │  - Provides filesystem  │  │    pod_uid to hostPath          ││       │
│  │  │  - NO mount operations  │  │  - Waits for ready.json from    ││       │
│  │  │  - env: ROOTFS_MARKER   │  │    DaemonSet                    ││       │
│  │  │                         │  │  - Verifies rootfs mount        ││       │
│  │  └─────────────────────────┘  └─────────────────────────────────┘│       │
│  └──────────────────────────────────────────────────────────────────┘       │
│                              │                                               │
│                              │ hostPath (HostToContainer propagation)        │
│                              ▼                                               │
│  ┌──────────────────────────────────────────────────────────────────┐       │
│  │              Consumer Pod (only CAP_SYS_CHROOT)                   │       │
│  │                                                                   │       │
│  │  - Waits for rootfs and proc mount to be ready                    │       │
│  │  - chroot into pre-mounted rootfs                                 │       │
│  │  - Executes user command                                          │       │
│  │  - NO mount operations needed (handled by DaemonSet)!             │       │
│  │                                                                   │       │
│  │  securityContext:                                                 │       │
│  │    capabilities:                                                  │       │
│  │      add: ["SYS_CHROOT"]                                          │       │
│  │      drop: ["ALL"] (optional)                                     │       │
│  └──────────────────────────────────────────────────────────────────┘       │
│                                                                              │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                    hostPath Volume Structure                           │ │
│  │  /var/lib/stoppablecontainer/<namespace>/<name>/                       │ │
│  │  ├── request.json   # Provider writes pod_uid for DaemonSet            │ │
│  │  ├── ready.json     # DaemonSet signals mount complete                 │ │
│  │  └── rootfs/        # Mounted overlayfs with proc/dev/sys              │ │
│  │      ├── proc/      # Mounted by DaemonSet                             │ │
│  │      ├── dev/       # Bind-mounted from host by DaemonSet              │ │
│  │      ├── sys/       # Bind-mounted from host by DaemonSet              │ │
│  │      └── ...        # Container filesystem                             │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Communication Protocol

### 1. Provider → DaemonSet (Mount Request)

Provider writes a request file to the hostPath:
```
/var/lib/stoppablecontainer/<ns>/<name>/request.json
```

Content:
```json
{
  "pod_uid": "abc123-def456-...",
  "namespace": "default",
  "name": "my-instance"
}
```

### 2. DaemonSet Processing

The mount-helper DaemonSet polls the hostPath directories and:

1. **Finds the request.json file** with pod UID
2. **Locates the rootfs container** by:
   - Scanning `/proc` for processes with matching pod UID in cgroup
   - Verifying `ROOTFS_MARKER=true` environment variable
3. **Reads overlayfs configuration** from `/proc/<pid>/mounts`
4. **Adjusts paths** to add `/host` prefix (since DaemonSet mounts host root at `/host`)
5. **Creates overlayfs mount** on host at `<hostPath>/rootfs` using `syscall.Mount`
6. **Mounts proc/dev/sys** inside the rootfs:
   - `proc` - new proc filesystem
   - `dev`, `sys` - bind-mount from host
   - `/dev/pts`, `/dev/shm` - bind-mount if available
7. **Removes request.json** and **writes ready.json**

### 3. DaemonSet → Provider/Consumer (Ready Signal)

DaemonSet creates:
```
/var/lib/stoppablecontainer/<ns>/<name>/ready.json
```

Content:
```json
{
  "status": "ready"
}
```

## Key Implementation Details

### Container Identification

The DaemonSet identifies the rootfs container using:
1. **Pod UID from cgroup**: `/proc/<pid>/cgroup` contains pod UID (with `-` replaced by `_`)
2. **ROOTFS_MARKER env var**: `/proc/<pid>/environ` contains `ROOTFS_MARKER=true`

### Overlayfs Recreation

Instead of using nsenter (which can fail due to rootfs changes), the DaemonSet:
1. Reads overlayfs options from the container's `/proc/<pid>/mounts`:
   ```
   overlay / overlay rw,lowerdir=/var/lib/containerd/.../lower,upperdir=...,workdir=... 0 0
   ```
2. Adjusts paths to use `/host` prefix:
   ```
   lowerdir=/host/var/lib/containerd/.../lower,upperdir=/host/...,workdir=/host/...
   ```
3. Creates a new overlay mount on the host filesystem

### Mount Propagation

- **DaemonSet**: Uses `Bidirectional` propagation to create mounts visible on host
- **Provider/Consumer**: Use `HostToContainer` propagation to see mounts created by DaemonSet

## Security Analysis

### Attack Surface Comparison

| Component | Old Architecture | DaemonSet Architecture |
|-----------|-----------------|------------------------|
| DaemonSet | N/A | privileged (1 per node) |
| Provider | **privileged** | non-privileged |
| Consumer | CAP_SYS_ADMIN + CAP_SYS_CHROOT | **CAP_SYS_CHROOT only** |

### Benefits

1. **Reduced blast radius**: Only DaemonSet is privileged, one per node (not per instance)
2. **No user code in privileged context**: User image runs without any privileges
3. **Centralized audit**: All privileged operations go through the DaemonSet
4. **Consumer safety**: Even if user code escapes chroot, it cannot do mount operations
5. **No CAP_SYS_ADMIN in user pods**: This powerful capability is no longer needed

### DaemonSet Security Measures

1. **Pod UID validation**: Only processes mount requests for pods with matching UID
2. **ROOTFS_MARKER verification**: Only mounts for containers explicitly marked
3. **Path validation**: Verifies overlay paths are within expected directories
4. **No network access needed**: DaemonSet only needs hostPID and host filesystem access

## Deployment

### Build and Deploy

```bash
# Build mount-helper image
make docker-build-mount-helper MOUNT_HELPER_IMG=myregistry/mount-helper:latest

# Push to registry
docker push myregistry/mount-helper:latest

# Deploy DaemonSet
make deploy-daemonset MOUNT_HELPER_IMG=myregistry/mount-helper:latest

# Deploy controller (as usual)
make deploy IMG=myregistry/controller:latest
```

### Prerequisites

- The mount-helper DaemonSet must be deployed before creating StoppableContainerInstance resources
- The DaemonSet runs in the `stoppablecontainer-system` namespace
