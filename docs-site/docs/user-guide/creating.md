# Creating StoppableContainers

This guide covers different patterns and examples for creating StoppableContainers.

## Basic Examples

### Simple Web Server

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: web-server
spec:
  running: true
  template:
    container:
      image: nginx:alpine
      command: ["nginx", "-g", "daemon off;"]
```

### Python Application

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: python-app
spec:
  running: true
  template:
    container:
      image: python:3.11-slim
      command: ["python", "-m", "http.server", "8080"]
      env:
        - name: PYTHONUNBUFFERED
          value: "1"
```

### Node.js Application

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: node-app
spec:
  running: true
  template:
    container:
      image: node:20-slim
      command: ["node", "-e"]
      args:
        - |
          const http = require('http');
          const server = http.createServer((req, res) => {
            res.writeHead(200);
            res.end('Hello from StoppableContainer!');
          });
          server.listen(3000, () => console.log('Server running'));
```

## With Environment Variables

### From Literals

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: app-with-env
spec:
  running: true
  template:
    container:
      image: busybox:stable
      command: ["/bin/sh", "-c"]
      args: ["echo $MESSAGE; sleep 3600"]
      env:
        - name: MESSAGE
          value: "Hello, World!"
        - name: DEBUG
          value: "true"
```

### From ConfigMap

```yaml
# First create a ConfigMap
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  DATABASE_HOST: "db.example.com"
  LOG_LEVEL: "info"
---
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: app-with-configmap
spec:
  running: true
  template:
    container:
      image: busybox:stable
      command: ["/bin/sh", "-c"]
      args: ["env | grep -E 'DATABASE|LOG'; sleep 3600"]
      env:
        - name: DATABASE_HOST
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: DATABASE_HOST
        - name: LOG_LEVEL
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: LOG_LEVEL
```

### From Secret

```yaml
# First create a Secret
apiVersion: v1
kind: Secret
metadata:
  name: app-secrets
type: Opaque
stringData:
  DB_PASSWORD: "supersecret"
  API_KEY: "abc123"
---
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: app-with-secrets
spec:
  running: true
  template:
    container:
      image: busybox:stable
      command: ["/bin/sh", "-c"]
      args: ["echo 'Secrets loaded'; sleep 3600"]
      env:
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: app-secrets
              key: DB_PASSWORD
        - name: API_KEY
          valueFrom:
            secretKeyRef:
              name: app-secrets
              key: API_KEY
```

## With Resource Limits

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: limited-app
spec:
  running: true
  template:
    container:
      image: python:3.11-slim
      command: ["python", "-c"]
      args: ["import time; [print(f'tick {i}') or time.sleep(1) for i in range(3600)]"]
      resources:
        limits:
          cpu: "500m"
          memory: "256Mi"
        requests:
          cpu: "100m"
          memory: "128Mi"
```

## With Node Selection

### Using Node Selector

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: app-on-specific-node
spec:
  running: true
  template:
    container:
      image: busybox:stable
      command: ["/bin/sh", "-c", "hostname; sleep 3600"]
    nodeSelector:
      kubernetes.io/os: linux
      disktype: ssd
```

### Using Tolerations

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: app-with-tolerations
spec:
  running: true
  template:
    container:
      image: busybox:stable
      command: ["/bin/sh", "-c", "sleep 3600"]
    tolerations:
      - key: "dedicated"
        operator: "Equal"
        value: "stoppable"
        effect: "NoSchedule"
```

## With Security Context

### Run as Non-Root

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: non-root-app
spec:
  running: true
  template:
    container:
      image: python:3.11-slim
      command: ["python", "-c", "import os; print(f'Running as UID {os.getuid()}')"]
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        runAsGroup: 1000
```

### With Additional Capabilities

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: app-with-caps
spec:
  running: true
  template:
    container:
      image: busybox:stable
      command: ["/bin/sh", "-c", "sleep 3600"]
      securityContext:
        capabilities:
          add:
            - NET_ADMIN  # If your application needs network admin capabilities
```

## Development Environments

### Interactive Shell (Pre-warmed)

Create a pre-warmed shell environment:

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: dev-shell
spec:
  running: false  # Pre-warm only
  template:
    container:
      image: ubuntu:22.04
      command: ["/bin/bash", "-c"]
      args: ["echo 'Dev environment ready'; exec /bin/bash"]
```

Start when needed:

```bash
kubectl patch stoppablecontainer dev-shell --type=merge -p '{"spec":{"running":true}}'
kubectl sc exec dev-shell -- /bin/bash
```

### Python Development Environment

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: python-dev
spec:
  running: true
  template:
    container:
      image: python:3.11
      command: ["/bin/bash", "-c"]
      args:
        - |
          pip install ipython numpy pandas matplotlib
          echo "Development environment ready"
          exec /bin/bash
```

## Batch Processing Pattern

Create multiple stoppable containers for batch jobs:

```yaml
# batch-template.yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: batch-worker-${WORKER_ID}
spec:
  running: false  # Start on demand
  template:
    container:
      image: python:3.11-slim
      command: ["python", "-c"]
      args:
        - |
          import os
          worker_id = os.environ.get('WORKER_ID', 'unknown')
          print(f'Processing batch for worker {worker_id}')
          # Process batch...
      env:
        - name: WORKER_ID
          value: "${WORKER_ID}"
```

Deploy multiple workers:

```bash
for i in {1..5}; do
  WORKER_ID=$i envsubst < batch-template.yaml | kubectl apply -f -
done
```

Start specific workers:

```bash
kubectl patch stoppablecontainer batch-worker-1 --type=merge -p '{"spec":{"running":true}}'
kubectl patch stoppablecontainer batch-worker-3 --type=merge -p '{"spec":{"running":true}}'
```

## Next Steps

- [Managing Lifecycle](lifecycle.md) - Start, stop, and manage containers
- [Advanced Usage](advanced.md) - Advanced patterns and techniques
