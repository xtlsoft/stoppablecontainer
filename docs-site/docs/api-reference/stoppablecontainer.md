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
    metadata: <ObjectMeta>
    spec: <PodSpec>
  provider: <ProviderSpec>
  hostPathPrefix: <string>
status:
  phase: <string>
  nodeName: <string>
  conditions: <[]Condition>
```

## Spec Fields

### `spec.running`

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Required | Yes |
| Default | `false` |

Controls whether the consumer container should be running.

- `true`: Consumer pod is created and running
- `false`: Consumer pod is deleted, provider continues running (filesystem preserved)

**Example:**

```yaml
spec:
  running: true
```

### `spec.template`

| Property | Value |
|----------|-------|
| Type | `PodTemplateSpec` |
| Required | Yes |

Template for creating the consumer pod. This uses the standard Kubernetes PodSpec structure for full compatibility.

### `spec.template.metadata`

| Property | Value |
|----------|-------|
| Type | `ObjectMeta` |
| Required | No |

Pod metadata including labels and annotations. These are passed through to the created consumer pod, enabling integration with:

- **Admission controllers**: Istio, Linkerd, Vault Agent, etc.
- **Quota systems**: Kueue, resource quotas
- **Service meshes**: Any mesh that uses sidecar injection
- **Monitoring systems**: Prometheus, Datadog, etc.

cat > /home/xtlsoft/repos/github.com/xtlsoft/stoppablecontainer/config/samples/stoppablecontainer_v1alpha1_stoppablecontainerinstance.yaml << 'EOF'
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainerInstance
metadata:
  name: example
  labels:
    app.kubernetes.io/name: stoppablecontainer
    app.kubernetes.io/instance: example
spec:
  # Reference to parent StoppableContainer
  stoppableContainerName: example
  
  # Whether the consumer should be running
  running: true
  
  template:
    # Pod metadata
    metadata:
      labels:
        app: example
    
    # Standard Kubernetes PodSpec
    spec:
      containers:
        - name: main
          image: ubuntu:22.04
          command:
            - /bin/bash
            - -c
            - sleep infinity
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
  
  provider:
    resources:
      requests:
        memory: "32Mi"
        cpu: "10m"
  
  hostPathPrefix: "/var/lib/stoppablecontainer"
EOF! note "System Labels"
    The following labels are managed by the controller and will override user-specified values:
    
    - `stoppablecontainer.xtlsoft.top/managed-by`
    - `stoppablecontainer.xtlsoft.top/instance`
    - `stoppablecontainer.xtlsoft.top/role`

**Example:**

```yaml
template:
  metadata:
    labels:
      app: my-app
      version: v1
      # Kueue integration
      kueue.x-k8s.io/queue-name: user-queue
    annotations:
      prometheus.io/scrape: "true"
      prometheus.io/port: "8080"
```

### `spec.template.spec`

| Property | Value |
|----------|-------|
| Type | `PodSpec` |
| Required | Yes |

Standard Kubernetes PodSpec. The first container in the `containers` list is used as the main workload container.

cat > /home/xtlsoft/repos/github.com/xtlsoft/stoppablecontainer/config/samples/stoppablecontainer_v1alpha1_stoppablecontainerinstance.yaml << 'EOF'
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainerInstance
metadata:
  name: example
  labels:
    app.kubernetes.io/name: stoppablecontainer
    app.kubernetes.io/instance: example
spec:
  # Reference to parent StoppableContainer
  stoppableContainerName: example
  
  # Whether the consumer should be running
  running: true
  
  template:
    # Pod metadata
    metadata:
      labels:
        app: example
    
    # Standard Kubernetes PodSpec
    spec:
      containers:
        - name: main
          image: ubuntu:22.04
          command:
            - /bin/bash
            - -c
            - sleep infinity
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
  
  provider:
    resources:
      requests:
        memory: "32Mi"
        cpu: "10m"
  
  hostPathPrefix: "/var/lib/stoppablecontainer"
EOF! important "Managed Fields"
    The following fields are managed by the controller and will be overridden:
    
    - `nodeName`: Set to match the provider pod's node
    - `restartPolicy`: Set to `Always`
    - Container `image`: Replaced with exec-wrapper image
    - Container `command`: Replaced with exec-wrapper entrypoint

#### Common PodSpec Fields

| Field | Description |
|-------|-------------|
| `containers` | List of containers (first one is the main workload) |
| `initContainers` | Init containers to run before the main container |
| `volumes` | Volumes to mount in the pod |
| `serviceAccountName` | Service account for the pod |
| `nodeSelector` | Node selection constraints |
| `affinity` | Affinity and anti-affinity rules |
| `tolerations` | Tolerations for taints |
| `schedulerName` | Custom scheduler (e.g., for Kueue) |
| `priorityClassName` | Priority class for scheduling |
| `securityContext` | Pod-level security context |
| `imagePullSecrets` | Secrets for pulling images |

**Example:**

```yaml
template:
  spec:
    containers:
      - name: main
        image: python:3.11
        command: ["python", "-m", "http.server", "8080"]
        ports:
          - containerPort: 8080
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "1"
            memory: "512Mi"
        env:
          - name: DEBUG
            value: "true"
    volumes:
      - name: data
        persistentVolumeClaim:
          claimName: my-pvc
    serviceAccountName: my-service-account
    nodeSelector:
      kubernetes.io/os: linux
    tolerations:
      - key: "dedicated"
        operator: "Equal"
        value: "workloads"
        effect: "NoSchedule"
```

### `spec.provider`

| Property | Value |
|----------|-------|
| Type | `ProviderSpec` |
| Required | No |

Configuration for the provider pod that holds the filesystem.

#### `spec.provider.resources`

| Property | Value |
|----------|-------|
| Type | `ResourceRequirements` |
| Required | No |

Resource limits and requests for the provider pod. Provider pods are lightweight and typically need minimal resources.

**Default:**

```yaml
resources:
  requests:
    cpu: "10m"
    memory: "16Mi"
  limits:
    cpu: "100m"
    memory: "64Mi"
```

#### `spec.provider.nodeSelector`

| Property | Value |
|----------|-------|
| Type | `map[string]string` |
| Required | No |

Node selection constraints for the provider pod.

#### `spec.provider.tolerations`

| Property | Value |
|----------|-------|
| Type | `[]Toleration` |
| Required | No |

Tolerations for the provider pod.

### `spec.hostPathPrefix`

| Property | Value |
|----------|-------|
| Type | `string` |
| Required | No |
| Default | `/var/lib/stoppablecontainer` |

Host path prefix for mount propagation between provider and consumer pods.

## Status Fields

### `status.phase`

| Property | Value |
|----------|-------|
| Type | `string` |
| Values | `Pending`, `ProviderReady`, `Running`, `Stopped`, `Failed` |

Current phase of the StoppableContainer.

| Phase | Description |
|-------|-------------|
| `Pending` | Waiting for provider pod to be ready |
| `ProviderReady` | Provider is ready, consumer starting |
| `Running` | Both provider and consumer are running |
| `Stopped` | Provider running, consumer stopped (filesystem preserved) |
| `Failed` | An error occurred |

### `status.nodeName`

| Property | Value |
|----------|-------|
| Type | `string` |

Node where the provider pod is running.

### `status.instanceName`

| Property | Value |
|----------|-------|
| Type | `string` |

Name of the associated StoppableContainerInstance.

### `status.conditions`

| Property | Value |
|----------|-------|
| Type | `[]Condition` |

Standard Kubernetes conditions for the resource.

## Integration Examples

### Kueue Integration

To use StoppableContainer with Kueue for job scheduling:

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: ml-training
spec:
  running: true
  template:
    metadata:
      labels:
        kueue.x-k8s.io/queue-name: gpu-queue
    spec:
      schedulerName: "default-scheduler"  # Kueue uses admission webhooks
      priorityClassName: "high-priority"
      containers:
        - name: trainer
          image: pytorch/pytorch:2.0.0-cuda11.7-cudnn8-runtime
          command: ["python", "train.py"]
          resources:
            requests:
              nvidia.com/gpu: "1"
            limits:
              nvidia.com/gpu: "1"
```

### Istio Service Mesh

Istio sidecar injection works automatically when labels are set:

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: web-app
spec:
  running: true
  template:
    metadata:
      labels:
        app: web-app
        version: v1
      annotations:
        sidecar.istio.io/inject: "true"
    spec:
      containers:
        - name: web
          image: nginx:1.25
          ports:
            - containerPort: 80
```

### Vault Secret Injection

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: secure-app
spec:
  running: true
  template:
    metadata:
      annotations:
        vault.hashicorp.com/agent-inject: "true"
        vault.hashicorp.com/role: "my-role"
        vault.hashicorp.com/agent-inject-secret-config: "secret/data/my-app/config"
    spec:
      serviceAccountName: my-app-sa
      containers:
        - name: app
          image: my-app:latest
```

## Complete Example

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: development-env
  labels:
    app.kubernetes.io/name: stoppablecontainer
    app.kubernetes.io/instance: development-env
spec:
  running: true
  
  template:
    metadata:
      labels:
        app: dev-env
        environment: development
      annotations:
        description: "Development environment with persistent filesystem"
    
    spec:
      containers:
        - name: main
          image: ubuntu:22.04
          command:
            - /bin/bash
            - -c
            - |
              echo "Development environment starting..."
              # Install development tools
              apt-get update && apt-get install -y curl vim git
              # Keep running
              sleep infinity
          resources:
            requests:
              memory: "512Mi"
              cpu: "250m"
            limits:
              memory: "2Gi"
              cpu: "2"
          env:
            - name: ENVIRONMENT
              value: "development"
          volumeMounts:
            - name: workspace
              mountPath: /workspace
      
      volumes:
        - name: workspace
          persistentVolumeClaim:
            claimName: dev-workspace-pvc
      
      serviceAccountName: developer-sa
      
      nodeSelector:
        node-type: development
      
      tolerations:
        - key: "dedicated"
          operator: "Equal"
          value: "development"
          effect: "NoSchedule"
  
  provider:
    resources:
      requests:
        memory: "32Mi"
        cpu: "10m"
      limits:
        memory: "64Mi"
        cpu: "100m"
  
  hostPathPrefix: "/var/lib/stoppablecontainer"
```
