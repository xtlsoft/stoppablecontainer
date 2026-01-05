# Frequently Asked Questions

## General

### What is StoppableContainer?

StoppableContainer is a Kubernetes operator that manages containers with persistent root filesystems. It enables filesystem changes to survive container restarts - something not possible with regular Kubernetes pods.

### Why would I use this instead of regular Pods?

StoppableContainer is beneficial when:

- You need filesystem changes to persist across restarts
- You want to install packages or modify files and keep them after restart
- You're building development environments or long-running interactive workloads
- You need an environment that "remembers" its state

### Is this production-ready?

StoppableContainer is currently in alpha (`v1alpha1`). It works well for many use cases but may have breaking API changes before v1.

## Architecture

### Why are there two pods (provider and consumer)?

The two-pod architecture separates concerns:

- **Provider**: Maintains the persistent rootfs, runs continuously
- **Consumer**: Executes user workloads, created/deleted based on `running` state

This allows the rootfs to persist while the user workload can be quickly started/stopped.

### Why does the provider pod need privileged mode?

It doesn't! With the DaemonSet architecture, the **mount-helper DaemonSet** handles all privileged operations. Provider and consumer pods run with standard container privileges.

The mount-helper DaemonSet is privileged because it needs to:
- Create overlayfs mounts
- Perform bind mounts
- Execute chroot operations

This centralizes privilege to a single, auditable component.

### Can I run multiple consumers from one provider?

Currently, each StoppableContainer has one provider and one consumer. Multiple consumers from a single provider is a planned feature.

## Security

### Is it safe to run the mount-helper DaemonSet?

The mount-helper DaemonSet runs privileged but:

- Only performs mount/chroot operations (no application code)
- Single instance per node (easy to audit)
- Can be isolated with NetworkPolicy
- Doesn't process user data

Application pods (provider and consumer) run with **no special privileges**.

### Can users escape the container?

Consumer containers use:
- Standard container isolation (namespaces, cgroups)
- Overlayfs for filesystem isolation
- exec-wrapper for chroot operations (handled by mount-helper)

The consumer pod itself has no special capabilities.

### How do I secure StoppableContainer workloads?

- Secure the mount-helper DaemonSet namespace
- Use NetworkPolicy to restrict traffic
- Apply resource limits
- Run as non-root when possible
- Implement RBAC to control who can create StoppableContainers

## Functionality

### What happens to data when the container stops?

Changes to the rootfs persist across stop/start cycles. The rootfs is not reset.

For truly ephemeral containers, use volumes for any mutable state.

### Can I use persistent volumes?

Currently, volume mounts are not supported in StoppableContainer. This is a planned feature.

Workaround: Store data externally (database, object storage, etc.)

### Can I update the image without deleting the container?

No, currently you must delete and recreate the StoppableContainer to update the image. In-place updates are planned.

### Why is my container slow to start the first time?

The first start includes:

1. Scheduling provider pod and mount-helper setup
2. Pulling the image
3. Creating overlayfs mount
4. Starting consumer

Subsequent starts are faster because the overlayfs layers are already prepared.

### Can I use custom entrypoints?

Yes, use `command` and `args` in the container spec:

```yaml
container:
  image: ubuntu:22.04
  command: ["/bin/bash", "-c"]
  args: ["echo 'Custom entrypoint'; exec /my-app"]
```

## Troubleshooting

### Container stuck in Pending

1. Check if provider pod is running:
   ```bash
   kubectl get pod <name>-provider
   ```

2. Check if mount-helper DaemonSet is running:
   ```bash
   kubectl get daemonset -n stoppablecontainer-system mount-helper
   ```

3. Check events:
   ```bash
   kubectl describe stoppablecontainer <name>
   ```

4. Check controller logs:
   ```bash
   kubectl logs -n stoppablecontainer-system -l control-plane=controller-manager
   ```

### Consumer won't start

1. Check mount-helper logs:
   ```bash
   kubectl logs -n stoppablecontainer-system -l app.kubernetes.io/name=mount-helper
   ```

2. Check consumer pod events:
   ```bash
   kubectl describe pod <name>
   ```

### Container exits immediately

1. Check consumer logs:
   ```bash
   kubectl logs <name>
   ```

2. Ensure command doesn't exit:
   ```yaml
   command: ["/bin/sh", "-c"]
   args: ["your-command; sleep infinity"]  # Keep alive for debugging
   ```

### Can't delete StoppableContainer

If deletion is stuck:

1. Check for finalizers:
   ```bash
   kubectl get stoppablecontainer <name> -o yaml | grep finalizers
   ```

2. Force delete pods:
   ```bash
   kubectl delete pod <name>-provider <name> --force --grace-period=0
   ```

3. Remove finalizers if needed:
   ```bash
   kubectl patch stoppablecontainer <name> -p '{"metadata":{"finalizers":null}}' --type=merge
   ```

## Compatibility

### Which Kubernetes versions are supported?

Kubernetes 1.25 and later are officially supported.

### Does it work with managed Kubernetes (GKE, EKS, AKS)?

Yes, StoppableContainer works on managed Kubernetes clusters. Ensure:

- cert-manager is installed (for controller)
- mount-helper DaemonSet can run privileged (usually enabled by default)
- HostPath volumes are supported

### Does it work with containerd / CRI-O?

Yes, StoppableContainer is container runtime agnostic. It works with containerd, CRI-O, and Docker.

### Can I use it with Istio / service mesh?

Yes, but consider:

- Consumer pods will get sidecar injection
- Provider pods may or may not need sidecars
- Network policies should account for mesh traffic

## Performance

### How fast is container start?

The primary goal of StoppableContainer is **persistent rootfs**, not startup speed. However, you may see some improvement:

| Scenario | Time |
|----------|------|
| First start (cold) | 5-30 seconds (image dependent) |
| Subsequent starts (warm) | A few seconds |

### What's the memory overhead?

- Provider pod: ~10-20 MB plus image size
- Consumer pod: Your application's requirements
- Rootfs: Stored on node filesystem (HostPath)

### How many StoppableContainers can I run per node?

Depends on:

- Node disk space (for rootfs storage)
- Memory and CPU for provider/consumer pods
- Kubernetes limits

Practical limit: Hundreds per node for small containers.

## Future

### What features are planned?

- Volume mount support
- In-place image updates
- Multiple consumers per provider
- Resource sharing optimizations
- Metrics and observability improvements

### How can I contribute?

See the [Contributing Guide](development/contributing.md).

### Where can I get help?

- GitHub Issues for bugs and features
- GitHub Discussions for questions
- Documentation site for guides
