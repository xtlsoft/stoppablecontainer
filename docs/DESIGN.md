# StoppableContainer Operator Design

## Overview

StoppableContainer is a Kubernetes operator that enables "stoppable" containers - containers whose ephemeral filesystem persists even when the workload is not running. This is achieved by separating the filesystem storage (Provider) from the compute (Consumer).

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    StoppableContainer CRD                        │
│  (User-facing resource, similar to VirtualMachine in KubeVirt)  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ creates/manages
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│               StoppableContainerInstance CRD                     │
│  (Running instance, similar to VirtualMachineInstance)          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ creates/manages
                              ▼
┌─────────────────────┐                    ┌─────────────────────┐
│    Provider Pod     │◄──── hostPath ────►│    Consumer Pod     │
│  (Filesystem only)  │    (propagated)    │  (Actual compute)   │
└─────────────────────┘                    └─────────────────────┘
```

## Components

### 1. StoppableContainer (SC)

The main user-facing CRD. Users create this to define their stoppable container.

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: my-container
spec:
  running: true  # Set to false to stop, true to start
  template:
    spec:
      image: ubuntu:22.04
      command: ["/bin/bash"]
      resources:
        requests:
          memory: "1Gi"
          cpu: "1"
        limits:
          memory: "2Gi"
          cpu: "2"
      volumeMounts:
        - name: data
          mountPath: /data
    volumes:
      - name: data
        persistentVolumeClaim:
          claimName: my-data
```

### 2. StoppableContainerInstance (SCI)

Created when `spec.running: true`. Represents a running instance.

- Manages the Provider Pod (always running while SCI exists)
- Manages the Consumer Pod (the actual workload)

### 3. Provider Pod

A minimal pod that:
- Uses the actual container image as a sidecar/init container
- Exposes the rootfs via bind mount to a hostPath
- Uses bidirectional mount propagation
- Requests minimal resources (BestEffort/Burstable QoS)
- Stays running as long as the SCI exists

Structure:
```
Provider Pod
├── Init Container (restartPolicy: Always)
│   └── Runs the actual image with `pause` or `sleep infinity`
│       (This container's rootfs is what we want to preserve)
└── Main Container
    └── Lightweight container that:
        1. Waits for init container to start
        2. Bind-mounts /proc/<pid>/root to /propagated/<name>
        3. Sleeps forever
```

### 4. Consumer Pod

The actual workload pod that:
- Receives the rootfs from Provider via hostPath
- Mounts rootfs to /rootfs (not /)
- Uses a chroot wrapper for all commands
- Has proper /dev, /proc, /sys bind mounts inside /rootfs
- Maps Kubernetes-mounted volumes correctly

### 5. Chroot Wrapper (stoppablecontainer-exec)

A Go binary that:
- Is injected into the Consumer Pod
- Handles `kubectl exec` by wrapping commands in chroot
- Creates symlinks in /bin, /usr/bin for common executables
- Handles SUID securely (drops privileges appropriately)

## Detailed Flow

### Starting a StoppableContainer

1. User creates/updates SC with `spec.running: true`
2. Controller creates SCI if not exists
3. SCI controller creates Provider Pod
4. Provider Pod starts:
   - Init container (actual image) starts and runs `pause`
   - Main container waits for init container, then bind-mounts its rootfs
5. Once Provider is ready, SCI controller creates Consumer Pod
6. Consumer Pod mounts the rootfs and starts the actual workload

### Stopping a StoppableContainer

1. User updates SC with `spec.running: false`
2. Controller deletes Consumer Pod (workload stops)
3. Provider Pod remains running (rootfs preserved)
4. SCI is deleted or marked as stopped

### Deleting a StoppableContainer

1. User deletes SC
2. Controller deletes SCI
3. SCI controller deletes both Provider and Consumer Pods
4. Rootfs is lost (ephemeral)

## Security Considerations

1. **Privileged Provider Pod**: Required for mount propagation
   - Mitigated by running minimal code
   - Uses restrictive RBAC

2. **Chroot Wrapper SUID**: 
   - Only needed if running non-root in Consumer
   - Carefully validates paths
   - Drops privileges after chroot

3. **HostPath Usage**:
   - Each SC gets unique hostPath directory
   - Cleanup on deletion
   - Could use CSI driver in future

## Implementation Phases

### Phase 1: Basic Functionality
- StoppableContainer CRD
- StoppableContainerInstance CRD
- Provider Pod with rootfs exposure
- Consumer Pod with chroot wrapper
- Basic start/stop lifecycle

### Phase 2: Enhanced Features
- Multiple container support
- Network policy integration
- Service account mapping
- Init containers support

### Phase 3: Production Ready
- CSI driver for rootfs (replace hostPath)
- Metrics and monitoring
- Webhooks for validation
- High availability

## File Structure

```
stoppablecontainer/
├── api/
│   └── v1alpha1/
│       ├── stoppablecontainer_types.go
│       ├── stoppablecontainerinstance_types.go
│       └── groupversion_info.go
├── cmd/
│   ├── manager/
│   │   └── main.go
│   └── exec-wrapper/
│       └── main.go           # The chroot wrapper binary
├── internal/
│   ├── controller/
│   │   ├── stoppablecontainer_controller.go
│   │   └── stoppablecontainerinstance_controller.go
│   └── provider/
│       └── templates.go      # Pod templates
├── config/
│   ├── crd/
│   ├── rbac/
│   ├── manager/
│   └── samples/
├── hack/
│   └── provider-init.sh      # Scripts for provider container
└── Dockerfile
```
