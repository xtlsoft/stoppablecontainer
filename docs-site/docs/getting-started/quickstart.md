# Quick Start

This guide helps you create your first StoppableContainer and understand its basic operations.

## Create a Simple StoppableContainer

Let's create a simple container running a shell loop:

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
      image: busybox:stable
      command: ["/bin/sh", "-c"]
      args: ["echo 'Container started!'; while true; do date; sleep 5; done"]
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

Notice the provider pod is still running, keeping the rootfs ready.

## Resume the Container

Start the container again:

```bash
kubectl patch stoppablecontainer demo --type=merge -p '{"spec":{"running":true}}'
```

The consumer pod starts almost instantly:

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
kubectl delete stoppablecontainer demo
```

Both pods will be cleaned up automatically.

## A More Practical Example

Here's a more realistic example with an Nginx web server:

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: web-server
  namespace: default
spec:
  running: true
  template:
    container:
      image: nginx:alpine
      command: ["nginx", "-g", "daemon off;"]
    nodeSelector:
      kubernetes.io/os: linux
    tolerations:
      - key: "node-role.kubernetes.io/master"
        operator: "Exists"
        effect: "NoSchedule"
```

Apply and verify:

```bash
kubectl apply -f web-server.yaml

# Check status
kubectl get stoppablecontainer web-server

# Access nginx (from within cluster)
kubectl exec demo-consumer -- curl localhost
```

## What's Next?

- [Configuration Guide](configuration.md) - Customize container behavior
- [Architecture](../concepts/architecture.md) - Understand how it works
- [Managing Lifecycle](../user-guide/lifecycle.md) - Advanced lifecycle operations
