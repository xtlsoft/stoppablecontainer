# Quick Start

This guide helps you create your first StoppableContainer and understand how the persistent rootfs works.

## Prerequisites

Make sure StoppableContainer is installed. See the [Installation Guide](installation.md) for details.

Optionally, install the kubectl-sc plugin for easier management:

```bash
go install github.com/xtlsoft/stoppablecontainer/cmd/kubectl-sc@latest
```

## Create a Simple StoppableContainer

### Using kubectl-sc (Recommended)

```bash
# Create and start a container
kubectl sc create demo --image=ubuntu:22.04 -- /bin/bash -c "while true; do date; sleep 5; done"
```

### Using YAML

```yaml
# my-stoppable.yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: demo
  namespace: default
spec:
  running: true
  template:
    container:
      image: ubuntu:22.04
      command: ["/bin/bash", "-c"]
      args: ["while true; do date; sleep 5; done"]
```

Apply the manifest:

```bash
kubectl apply -f my-stoppable.yaml
```

## Observe the Pods

Watch the pods being created:

```bash
kubectl get pods -w
```

You'll see two pods:

```
NAME            READY   STATUS    RESTARTS   AGE
demo-provider   2/2     Running   0          10s
demo-consumer   1/1     Running   0          5s
```

- **demo-provider**: The long-running pod that maintains the rootfs
- **demo-consumer**: The pod running your actual command

## Check the Status

View the StoppableContainer status:

```bash
kubectl get stoppablecontainer demo
```

Output:

```
NAME   RUNNING   PHASE     NODE                 AGE
demo   true      Running   your-node-name       1m
```

Check the logs:

```bash
kubectl logs demo-consumer
```

## Stop the Container

To stop the container while preserving the rootfs:

```bash
# Using kubectl-sc
kubectl sc stop demo

# Or using kubectl
kubectl patch stoppablecontainer demo --type=merge -p '{"spec":{"running":false}}'
```

Watch the consumer pod get deleted:

```bash
kubectl get pods -w
```

```
NAME            READY   STATUS        RESTARTS   AGE
demo-provider   2/2     Running       0          2m
demo-consumer   1/1     Terminating   0          1m
demo-consumer   0/1     Terminating   0          1m
```

After termination:

```
NAME            READY   STATUS    RESTARTS   AGE
demo-provider   2/2     Running   0          3m
```

Notice the provider pod is still running, keeping the rootfs intact with any modifications you made.

## Resume the Container

Start the container again:

```bash
# Using kubectl-sc
kubectl sc start demo

# Or using kubectl
kubectl patch stoppablecontainer demo --type=merge -p '{"spec":{"running":true}}'
```

The consumer pod is recreated with the preserved rootfs:

```bash
kubectl get pods
```

```
NAME            READY   STATUS    RESTARTS   AGE
demo-provider   2/2     Running   0          5m
demo-consumer   1/1     Running   0          2s
```

## Clean Up

Delete the StoppableContainer:

```bash
# Using kubectl-sc
kubectl sc delete demo

# Or using kubectl
kubectl delete stoppablecontainer demo
```

Both pods will be cleaned up automatically.

## Demonstrating Persistent Rootfs

Let's demonstrate the key feature - filesystem persistence:

```bash
# Create a container
kubectl sc create mydev --image=ubuntu:22.04 -- sleep infinity

# Install a package (this would be lost in normal containers)
kubectl sc exec mydev -- apt update
kubectl sc exec mydev -- apt install -y curl

# Create a file
kubectl sc exec mydev -- bash -c "echo 'Hello World' > /my-file.txt"

# Stop the container
kubectl sc stop mydev --wait

# Start it again
kubectl sc start mydev --wait

# Verify the file and package are still there!
kubectl sc exec mydev -- cat /my-file.txt
kubectl sc exec mydev -- curl --version

# Clean up
kubectl sc delete mydev
```

The installed package (`curl`) and created file (`/my-file.txt`) persist across stop/start cycles.

## What's Next?

- [Configuration Guide](configuration.md) - Customize container behavior
- [kubectl Plugin Guide](../user-guide/kubectl-plugin.md) - Full CLI reference
- [Architecture](../concepts/architecture.md) - Understand how it works
- [Managing Lifecycle](../user-guide/lifecycle.md) - Advanced lifecycle operations
