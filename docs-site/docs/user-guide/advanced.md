# Advanced Usage

This guide covers advanced patterns and techniques for StoppableContainer.

## Pre-warming Pattern

Pre-warm containers ahead of time for instant availability:

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: prewarmed-python
  labels:
    ready-pool: python
spec:
  running: false  # Don't run, just prepare rootfs
  template:
    container:
      image: python:3.11
      command: ["python", "-c", "print('Ready')"]
```

Create a pool of pre-warmed containers:

```bash
for i in {1..10}; do
  cat <<EOF | kubectl apply -f -
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: python-pool-$i
  labels:
    pool: python
    status: available
spec:
  running: false
  template:
    container:
      image: python:3.11
      command: ["python"]
      args: ["-c", "import time; time.sleep(3600)"]
EOF
done
```

Claim a container from the pool:

```bash
#!/bin/bash
# claim-container.sh

# Find available container
CONTAINER=$(kubectl get stoppablecontainer -l pool=python,status=available -o name | head -1)

if [ -z "$CONTAINER" ]; then
  echo "No available containers"
  exit 1
fi

# Mark as claimed
kubectl label $CONTAINER status=claimed --overwrite

# Start it
kubectl patch $CONTAINER --type=merge -p '{"spec":{"running":true}}'

echo "Claimed and started: $CONTAINER"
```

## Multi-tenant Isolation

Create isolated environments per user/tenant:

```yaml
# Template for tenant environments
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: tenant-${TENANT_ID}
  namespace: tenant-${TENANT_ID}
  labels:
    tenant: ${TENANT_ID}
spec:
  running: false
  template:
    container:
      image: ${TENANT_IMAGE}
      env:
        - name: TENANT_ID
          value: ${TENANT_ID}
      resources:
        limits:
          cpu: "1"
          memory: "512Mi"
    nodeSelector:
      tenant-class: ${TENANT_CLASS}
```

## Cost Optimization

### Scheduled Shutdown

Stop containers during off-hours:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: nightly-shutdown
spec:
  schedule: "0 22 * * *"  # 10 PM daily
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: sc-manager
          containers:
            - name: shutdown
              image: bitnami/kubectl:latest
              command: ["/bin/sh", "-c"]
              args:
                - |
                  kubectl get stoppablecontainer -l tier=dev -o name | \
                  xargs -I {} kubectl patch {} --type=merge -p '{"spec":{"running":false}}'
          restartPolicy: OnFailure
```

### Idle Detection

Use a sidecar or external service to detect idle containers:

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: auto-stop-app
  annotations:
    idle-timeout: "30m"
spec:
  running: true
  template:
    container:
      image: nginx:alpine
```

Idle detection controller (pseudocode):

```go
// Watch for containers with idle-timeout annotation
// Monitor activity via metrics
// Stop containers after idle timeout
```

## Integration with External Services

### Webhook Notifications

Configure webhook notifications on lifecycle events:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lifecycle-hooks
data:
  on-start.sh: |
    #!/bin/sh
    curl -X POST https://webhook.example.com/started \
      -d '{"container": "'$SC_NAME'", "namespace": "'$SC_NAMESPACE'"}'
  
  on-stop.sh: |
    #!/bin/sh
    curl -X POST https://webhook.example.com/stopped \
      -d '{"container": "'$SC_NAME'", "namespace": "'$SC_NAMESPACE'"}'
```

### Service Discovery

Register containers with service discovery:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: stoppable-apps
spec:
  selector:
    app.kubernetes.io/managed-by: stoppablecontainer
  ports:
    - port: 80
      targetPort: 8080
```

## Custom Entrypoint Patterns

### Initialization Script

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: app-with-init
spec:
  running: true
  template:
    container:
      image: python:3.11
      command: ["/bin/sh", "-c"]
      args:
        - |
          # Initialization
          echo "Initializing..."
          pip install -q requests
          
          # Main application
          python -c "
          import time
          print('Application started')
          while True:
              time.sleep(60)
          "
```

### Signal Handling

Ensure proper signal handling for graceful shutdown:

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: signal-handler
spec:
  running: true
  template:
    container:
      image: python:3.11
      command: ["python", "-c"]
      args:
        - |
          import signal
          import sys
          import time
          
          def handler(signum, frame):
              print(f"Received signal {signum}, shutting down...")
              # Cleanup code here
              sys.exit(0)
          
          signal.signal(signal.SIGTERM, handler)
          signal.signal(signal.SIGINT, handler)
          
          print("Running...")
          while True:
              time.sleep(1)
```

## Debugging Techniques

### Attach to Provider for Rootfs Inspection

```bash
# List rootfs contents
kubectl exec my-app-provider -c provider -- ls -la /rootfs/rootfs

# Check ready file
kubectl exec my-app-provider -c provider -- cat /rootfs/ready

# Inspect mounts
kubectl exec my-app-provider -c provider -- mount | grep rootfs
```

### Consumer Debug Mode

Create a debug consumer that doesn't exit:

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: debug-app
spec:
  running: true
  template:
    container:
      image: ubuntu:22.04
      command: ["/bin/bash", "-c"]
      args:
        - |
          echo "Debug mode - container will stay running"
          exec /bin/bash
```

### View Container Internals

```bash
# Check mounts inside consumer
kubectl exec debug-app-consumer -- mount

# Check processes
kubectl exec debug-app-consumer -- ps aux

# Check capabilities
kubectl exec debug-app-consumer -- cat /proc/self/status | grep Cap
```

## Performance Tuning

### Optimize for Startup Time

1. **Use small base images**:

```dockerfile
FROM alpine:3.18 AS builder
# Build steps

FROM scratch
COPY --from=builder /app /app
CMD ["/app"]
```

2. **Pre-install dependencies**:

Build custom image with all dependencies:

```dockerfile
FROM python:3.11-slim
RUN pip install --no-cache-dir \
    numpy pandas matplotlib scikit-learn
```

3. **Use local registry**:

Deploy a registry close to your cluster to reduce image pull times.

### Optimize for Memory

Share rootfs across multiple instances on the same image:

```yaml
# All these share the same extracted layers on the node
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: worker-1
spec:
  template:
    container:
      image: python:3.11-slim  # Same image
---
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: worker-2
spec:
  template:
    container:
      image: python:3.11-slim  # Same image
```

## Migration Patterns

### From Deployment to StoppableContainer

Before (Deployment):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: app
          image: nginx:latest
          command: ["nginx", "-g", "daemon off;"]
```

After (StoppableContainer):

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: my-app
spec:
  running: true
  template:
    container:
      image: nginx:latest
      command: ["nginx", "-g", "daemon off;"]
```

### Gradual Migration

Use labels to track migration status:

```yaml
metadata:
  labels:
    migration-status: converted
    original-deployment: my-old-deployment
```

## Next Steps

- [API Reference](../api-reference/stoppablecontainer.md) - Complete API documentation
- [FAQ](../faq.md) - Frequently asked questions
