# Configuration

This guide covers all configuration options available for StoppableContainer.

## StoppableContainer Spec

### Basic Structure

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: my-container
  namespace: default
spec:
  running: true          # Whether the container should be running
  template:              # Container template specification
    container: {}        # Container definition
    nodeSelector: {}     # Optional: Node selection constraints
    tolerations: []      # Optional: Pod tolerations
```

### Container Configuration

The `template.container` field follows the Kubernetes Container spec with some adaptations:

```yaml
spec:
  template:
    container:
      # Required: Container image
      image: nginx:latest
      
      # Optional: Image pull policy (Always, IfNotPresent, Never)
      imagePullPolicy: IfNotPresent
      
      # Optional: Command to run (overrides ENTRYPOINT)
      command: ["nginx"]
      
      # Optional: Arguments to command (overrides CMD)
      args: ["-g", "daemon off;"]
      
      # Optional: Environment variables
      env:
        - name: MY_VAR
          value: "my-value"
        - name: SECRET_VAR
          valueFrom:
            secretKeyRef:
              name: my-secret
              key: password
      
      # Optional: Resource limits and requests
      resources:
        limits:
          cpu: "500m"
          memory: "256Mi"
        requests:
          cpu: "100m"
          memory: "128Mi"
      
      # Optional: Security context
      # Note: SYS_ADMIN and SYS_CHROOT capabilities are automatically added
      securityContext:
        runAsUser: 1000
        runAsGroup: 1000
        capabilities:
          add: ["NET_ADMIN"]  # Additional capabilities
```

### Node Selection

Control which nodes the pods run on:

```yaml
spec:
  template:
    nodeSelector:
      kubernetes.io/os: linux
      disktype: ssd
```

### Tolerations

Allow scheduling on tainted nodes:

```yaml
spec:
  template:
    tolerations:
      - key: "dedicated"
        operator: "Equal"
        value: "workloads"
        effect: "NoSchedule"
      - key: "node-role.kubernetes.io/master"
        operator: "Exists"
        effect: "NoSchedule"
```

## Complete Example

Here's a comprehensive example with all common options:

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: full-example
  namespace: default
  labels:
    app: my-app
    tier: backend
spec:
  running: true
  template:
    container:
      image: python:3.11-slim
      imagePullPolicy: IfNotPresent
      command: ["python"]
      args: ["-m", "http.server", "8080"]
      env:
        - name: PYTHONUNBUFFERED
          value: "1"
        - name: APP_PORT
          value: "8080"
      resources:
        limits:
          cpu: "1"
          memory: "512Mi"
        requests:
          cpu: "250m"
          memory: "256Mi"
      securityContext:
        runAsUser: 1000
        runAsGroup: 1000
    nodeSelector:
      kubernetes.io/os: linux
    tolerations:
      - key: "workload-type"
        operator: "Equal"
        value: "dev"
        effect: "NoSchedule"
```

## Provider Pod Configuration

The provider pod is automatically configured and cannot be directly customized. It:

- Uses the same image as your container
- Runs with `privileged: true` for mount propagation
- Has two containers: `pause` and `provider`
- Exports the rootfs to a HostPath volume

## Consumer Pod Configuration

The consumer pod inherits most settings from your template:

- Environment variables
- Resource limits/requests
- Security context (with required capabilities added)
- Node selector and tolerations

Additional behaviors:

- Automatically adds `SYS_ADMIN` and `SYS_CHROOT` capabilities
- Merges user-specified capabilities with required ones
- Uses an init container to set up the exec wrapper

## Environment Variables

The following environment variables are automatically set in the consumer container:

| Variable | Description |
|----------|-------------|
| `STOPPABLE_CONTAINER_NAME` | Name of the StoppableContainer |
| `STOPPABLE_CONTAINER_NAMESPACE` | Namespace of the StoppableContainer |

## Labels and Annotations

The following labels are automatically added to managed pods:

| Label | Description |
|-------|-------------|
| `app.kubernetes.io/managed-by` | Set to `stoppablecontainer` |
| `stoppablecontainer.xtlsoft.top/instance` | Name of the StoppableContainerInstance |

## Status Fields

The StoppableContainer status includes:

```yaml
status:
  phase: Running      # Current phase (Pending, Running, Stopped)
  node: node-name     # Node where pods are scheduled
  conditions:         # Detailed conditions
    - type: Ready
      status: "True"
      reason: ContainerRunning
      message: "Container is running"
```

## Next Steps

- [Architecture](../concepts/architecture.md) - Understand the system design
- [Security](../concepts/security.md) - Security considerations
- [Creating StoppableContainers](../user-guide/creating.md) - More examples
