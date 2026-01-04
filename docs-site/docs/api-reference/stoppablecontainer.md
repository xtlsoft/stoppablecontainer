# StoppableContainer API Reference

This document provides the complete API reference for the StoppableContainer custom resource.

## Resource Definition

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: <string>
  namespace: <string>
spec:
  running: <boolean>
  template:
    container: <ContainerSpec>
    nodeSelector: <map[string]string>
    tolerations: <[]Toleration>
status:
  phase: <string>
  node: <string>
  conditions: <[]Condition>
```

## Spec Fields

### `spec.running`

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Required | Yes |
| Default | N/A |

Controls whether the consumer container should be running.

- `true`: Consumer pod is created and running
- `false`: Consumer pod is deleted, provider continues running

**Example:**

```yaml
spec:
  running: true
```

### `spec.template`

| Property | Value |
|----------|-------|
| Type | `ContainerTemplate` |
| Required | Yes |

Template for creating the container.

### `spec.template.container`

| Property | Value |
|----------|-------|
| Type | `ContainerSpec` |
| Required | Yes |

Container specification following Kubernetes Container spec.

#### `spec.template.container.image`

| Property | Value |
|----------|-------|
| Type | `string` |
| Required | Yes |

Container image to use.

**Example:**

```yaml
container:
  image: nginx:1.25
```

#### `spec.template.container.imagePullPolicy`

| Property | Value |
|----------|-------|
| Type | `string` |
| Required | No |
| Default | `IfNotPresent` |
| Values | `Always`, `IfNotPresent`, `Never` |

Image pull policy.

#### `spec.template.container.command`

| Property | Value |
|----------|-------|
| Type | `[]string` |
| Required | No |

Command to run (overrides image ENTRYPOINT).

**Example:**

```yaml
container:
  command: ["python", "-m", "http.server"]
```

#### `spec.template.container.args`

| Property | Value |
|----------|-------|
| Type | `[]string` |
| Required | No |

Arguments to the command (overrides image CMD).

**Example:**

```yaml
container:
  command: ["python"]
  args: ["-c", "print('hello')"]
```

#### `spec.template.container.env`

| Property | Value |
|----------|-------|
| Type | `[]EnvVar` |
| Required | No |

Environment variables.

**Example:**

```yaml
container:
  env:
    - name: DEBUG
      value: "true"
    - name: SECRET
      valueFrom:
        secretKeyRef:
          name: my-secret
          key: password
```

#### `spec.template.container.resources`

| Property | Value |
|----------|-------|
| Type | `ResourceRequirements` |
| Required | No |

Resource limits and requests.

**Example:**

```yaml
container:
  resources:
    limits:
      cpu: "1"
      memory: "512Mi"
    requests:
      cpu: "100m"
      memory: "128Mi"
```

#### `spec.template.container.securityContext`

| Property | Value |
|----------|-------|
| Type | `SecurityContext` |
| Required | No |

Security context for the container.

!!! note
    `SYS_ADMIN` and `SYS_CHROOT` capabilities are automatically added.

**Example:**

```yaml
container:
  securityContext:
    runAsUser: 1000
    runAsGroup: 1000
    runAsNonRoot: true
    capabilities:
      add:
        - NET_ADMIN
```

### `spec.template.nodeSelector`

| Property | Value |
|----------|-------|
| Type | `map[string]string` |
| Required | No |

Node selection constraints.

**Example:**

```yaml
template:
  nodeSelector:
    kubernetes.io/os: linux
    disktype: ssd
```

### `spec.template.tolerations`

| Property | Value |
|----------|-------|
| Type | `[]Toleration` |
| Required | No |

Pod tolerations for scheduling.

**Example:**

```yaml
template:
  tolerations:
    - key: "dedicated"
      operator: "Equal"
      value: "workloads"
      effect: "NoSchedule"
```

## Status Fields

### `status.phase`

| Property | Value |
|----------|-------|
| Type | `string` |
| Values | `Pending`, `Running`, `Stopped`, `Error` |

Current phase of the StoppableContainer.

| Phase | Description |
|-------|-------------|
| `Pending` | Waiting for provider pod to be ready |
| `Running` | Both provider and consumer are running |
| `Stopped` | Provider running, no consumer |
| `Error` | An error occurred |

### `status.node`

| Property | Value |
|----------|-------|
| Type | `string` |

Name of the node where pods are scheduled.

### `status.conditions`

| Property | Value |
|----------|-------|
| Type | `[]Condition` |

Detailed conditions for the resource.

**Condition Types:**

| Type | Description |
|------|-------------|
| `Ready` | Overall readiness |
| `ProviderReady` | Provider pod is ready |
| `ConsumerReady` | Consumer pod is ready (when running) |

## Full Example

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: complete-example
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
        - name: APP_ENV
          value: "production"
      resources:
        limits:
          cpu: "1"
          memory: "512Mi"
        requests:
          cpu: "100m"
          memory: "128Mi"
      securityContext:
        runAsUser: 1000
        runAsGroup: 1000
    nodeSelector:
      kubernetes.io/os: linux
    tolerations:
      - key: "workload"
        operator: "Equal"
        value: "stoppable"
        effect: "NoSchedule"
status:
  phase: Running
  node: worker-1
  conditions:
    - type: Ready
      status: "True"
      reason: ContainerRunning
      message: "All components are running"
    - type: ProviderReady
      status: "True"
      reason: PodRunning
      message: "Provider pod is running"
    - type: ConsumerReady
      status: "True"
      reason: PodRunning
      message: "Consumer pod is running"
```

## kubectl Commands

### Create

```bash
kubectl apply -f stoppablecontainer.yaml
```

### Get

```bash
# List all
kubectl get stoppablecontainer

# Get specific
kubectl get stoppablecontainer my-app

# Get with details
kubectl get stoppablecontainer my-app -o yaml
```

### Update

```bash
# Edit interactively
kubectl edit stoppablecontainer my-app

# Patch
kubectl patch stoppablecontainer my-app --type=merge -p '{"spec":{"running":false}}'
```

### Delete

```bash
kubectl delete stoppablecontainer my-app
```

## See Also

- [StoppableContainerInstance API Reference](stoppablecontainerinstance.md)
- [Configuration Guide](../getting-started/configuration.md)
