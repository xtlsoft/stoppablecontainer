# Managing Lifecycle

This guide covers how to manage the lifecycle of StoppableContainers.

## Lifecycle States

A StoppableContainer can be in the following phases:

| Phase | Description |
|-------|-------------|
| `Pending` | Waiting for provider pod to be ready |
| `Running` | Both provider and consumer are running |
| `Stopped` | Provider running, consumer not created |
| `Error` | An error occurred |

## Starting a Container

### Via kubectl patch

```bash
kubectl patch stoppablecontainer my-app --type=merge -p '{"spec":{"running":true}}'
```

### Via kubectl edit

```bash
kubectl edit stoppablecontainer my-app
# Change spec.running to true
```

### Via kubectl apply

```yaml
# my-app.yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: my-app
spec:
  running: true  # Changed from false
  template:
    container:
      image: nginx:latest
```

```bash
kubectl apply -f my-app.yaml
```

## Stopping a Container

### Via kubectl patch

```bash
kubectl patch stoppablecontainer my-app --type=merge -p '{"spec":{"running":false}}'
```

### What Happens When Stopping

1. Controller updates the StoppableContainerInstance
2. Consumer pod is deleted (graceful termination)
3. Provider pod continues running
4. Rootfs remains available for quick restart

### Graceful Termination

The consumer pod follows standard Kubernetes termination:

1. SIGTERM sent to main process
2. Wait for `terminationGracePeriodSeconds` (default 30s)
3. SIGKILL if still running
4. Pod deleted

To customize termination behavior, ensure your application handles SIGTERM.

## Checking Status

### Quick Status

```bash
kubectl get stoppablecontainer my-app
```

Output:

```
NAME     RUNNING   PHASE     NODE                 AGE
my-app   true      Running   kind-control-plane   5m
```

### Detailed Status

```bash
kubectl describe stoppablecontainer my-app
```

### Watching Status Changes

```bash
kubectl get stoppablecontainer my-app -w
```

## Viewing Logs

### Consumer Logs

```bash
# Current logs
kubectl logs my-app-consumer

# Follow logs
kubectl logs my-app-consumer -f

# Previous container logs (after restart)
kubectl logs my-app-consumer --previous
```

### Provider Logs

```bash
kubectl logs my-app-provider -c provider
```

## Executing Commands

### Using kubectl-sc (Recommended)

The `kubectl-sc` plugin automatically handles exec-wrapper:

```bash
# Single command
kubectl sc exec my-app -- ls -la /

# Interactive shell
kubectl sc exec my-app -- /bin/bash
```

### Using kubectl exec Directly

When using kubectl directly, you must use the exec-wrapper:

```bash
# Single command (note: use /.sc-bin/sc-exec)
kubectl exec my-app-consumer -- /.sc-bin/sc-exec ls -la /

# Interactive shell
kubectl exec -it my-app-consumer -- /.sc-bin/sc-exec /bin/bash
```

### In Provider (Debugging)

```bash
kubectl exec -it my-app-provider -c provider -- /bin/sh
```

## Restarting a Container

### Full Restart (Stop then Start)

```bash
# Stop
kubectl patch stoppablecontainer my-app --type=merge -p '{"spec":{"running":false}}'

# Wait for consumer to terminate
kubectl wait --for=delete pod/my-app-consumer --timeout=60s

# Start
kubectl patch stoppablecontainer my-app --type=merge -p '{"spec":{"running":true}}'
```

### Quick Toggle Script

```bash
#!/bin/bash
# restart-sc.sh
NAME=$1

kubectl patch stoppablecontainer $NAME --type=merge -p '{"spec":{"running":false}}'
sleep 2
kubectl patch stoppablecontainer $NAME --type=merge -p '{"spec":{"running":true}}'
echo "Restarted $NAME"
```

Usage:

```bash
./restart-sc.sh my-app
```

## Batch Operations

### Stop Multiple Containers

```bash
# Stop all in namespace
kubectl get stoppablecontainer -o name | xargs -I {} kubectl patch {} --type=merge -p '{"spec":{"running":false}}'

# Stop by label
kubectl get stoppablecontainer -l app=batch-worker -o name | xargs -I {} kubectl patch {} --type=merge -p '{"spec":{"running":false}}'
```

### Start Multiple Containers

```bash
# Start all in namespace
kubectl get stoppablecontainer -o name | xargs -I {} kubectl patch {} --type=merge -p '{"spec":{"running":true}}'

# Start by label
kubectl get stoppablecontainer -l tier=frontend -o name | xargs -I {} kubectl patch {} --type=merge -p '{"spec":{"running":true}}'
```

## Monitoring

### Using kubectl top

```bash
# Consumer resource usage
kubectl top pod my-app-consumer

# Provider resource usage
kubectl top pod my-app-provider
```

### Prometheus Metrics

The controller exposes metrics at `/metrics`:

```promql
# Reconciliation count
controller_runtime_reconcile_total{controller="stoppablecontainer"}

# Reconciliation errors
controller_runtime_reconcile_errors_total{controller="stoppablecontainer"}

# Active workers
controller_runtime_active_workers{controller="stoppablecontainer"}
```

## Troubleshooting Lifecycle Issues

### Container Won't Start

1. Check controller logs:

```bash
kubectl logs -n stoppablecontainer-system -l control-plane=controller-manager
```

2. Check provider pod status:

```bash
kubectl describe pod my-app-provider
```

3. Check events:

```bash
kubectl get events --field-selector involvedObject.name=my-app
```

### Container Won't Stop

1. Check if consumer pod is stuck:

```bash
kubectl get pod my-app-consumer -o yaml | grep -A 5 "status:"
```

2. Force delete if necessary:

```bash
kubectl delete pod my-app-consumer --force --grace-period=0
```

### Provider Pod Unhealthy

1. Check provider logs:

```bash
kubectl logs my-app-provider -c provider
kubectl logs my-app-provider -c pause
```

2. Check node status:

```bash
kubectl describe node $(kubectl get pod my-app-provider -o jsonpath='{.spec.nodeName}')
```

## Automating Lifecycle

### Using Cron-like Scheduling

You can use Kubernetes CronJobs to schedule stop/start:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: stop-dev-containers
spec:
  schedule: "0 18 * * 1-5"  # 6 PM weekdays
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: stoppablecontainer-manager
          containers:
            - name: kubectl
              image: bitnami/kubectl:latest
              command:
                - /bin/sh
                - -c
                - |
                  kubectl get stoppablecontainer -l env=dev -o name | \
                  xargs -I {} kubectl patch {} --type=merge -p '{"spec":{"running":false}}'
          restartPolicy: OnFailure
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: start-dev-containers
spec:
  schedule: "0 9 * * 1-5"  # 9 AM weekdays
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: stoppablecontainer-manager
          containers:
            - name: kubectl
              image: bitnami/kubectl:latest
              command:
                - /bin/sh
                - -c
                - |
                  kubectl get stoppablecontainer -l env=dev -o name | \
                  xargs -I {} kubectl patch {} --type=merge -p '{"spec":{"running":true}}'
          restartPolicy: OnFailure
```

## Next Steps

- [Advanced Usage](advanced.md) - Advanced patterns and techniques
- [API Reference](../api-reference/stoppablecontainer.md) - Full API documentation
