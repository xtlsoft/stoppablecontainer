# Frequently Asked Questions

## General

### What is StoppableContainer?

StoppableContainer is a Kubernetes operator that manages containers with persistent root filesystems. It enables near-instant container starts by keeping the filesystem pre-extracted and ready.

### Why would I use this instead of regular Pods?

StoppableContainer is beneficial when:

- You need fast container starts (sub-100ms)
- Containers are frequently started and stopped
- You want to reduce image extraction overhead
- You're building on-demand or serverless-like workloads

### Is this production-ready?

StoppableContainer is currently in alpha (`v1alpha1`). It works well for many use cases but may have breaking API changes before v1.

## Architecture

### Why are there two pods (provider and consumer)?

The two-pod architecture separates concerns:

- **Provider**: Maintains the persistent rootfs, runs continuously
- **Consumer**: Executes user workloads, created/deleted based on `running` state

This allows the rootfs to persist while the user workload can be quickly started/stopped.

### Why does the provider pod need privileged mode?

Kubernetes requires `privileged: true` for Bidirectional mount propagation. The provider needs this to share the rootfs with the host and consumer pods.

See: [Kubernetes Mount Propagation](https://kubernetes.io/docs/concepts/storage/volumes/#mount-propagation)

### Can I run multiple consumers from one provider?

Currently, each StoppableContainer has one provider and one consumer. Multiple consumers from a single provider is a planned feature.

## Security

### Is it safe to run privileged containers?

The provider pod runs privileged but:

- Runs a minimal script (no complex application code)
- Has no network access needs (can be blocked with NetworkPolicy)
- Doesn't mount secrets or sensitive data

The consumer pod uses only `SYS_ADMIN` and `SYS_CHROOT` capabilities, significantly reducing attack surface compared to privileged mode.

### Can users escape the container?

Consumer containers use standard container isolation (namespaces, cgroups) plus chroot. The capabilities granted are the minimum required for operation.

### How do I secure StoppableContainer workloads?

- Use NetworkPolicy to restrict traffic
- Apply resource limits
- Run as non-root when possible
- Use read-only root filesystem for immutable containers
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

1. Scheduling provider pod
2. Pulling the image
3. Extracting rootfs
4. Starting consumer

Subsequent starts are much faster because the rootfs is already prepared.

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

2. Check events:
   ```bash
   kubectl describe stoppablecontainer <name>
   ```

3. Check controller logs:
   ```bash
   kubectl logs -n stoppablecontainer-system -l control-plane=controller-manager
   ```

### Consumer won't start

1. Verify rootfs is ready:
   ```bash
   kubectl exec <name>-provider -c provider -- cat /rootfs/ready
   ```

2. Check consumer pod events:
   ```bash
   kubectl describe pod <name>-consumer
   ```

### Container exits immediately

1. Check consumer logs:
   ```bash
   kubectl logs <name>-consumer
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
   kubectl delete pod <name>-provider <name>-consumer --force --grace-period=0
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

- cert-manager is installed
- Nodes allow privileged containers (usually enabled by default)
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

Typical startup times:

| Scenario | Time |
|----------|------|
| First start (cold) | 5-30 seconds (image dependent) |
| Subsequent starts (warm) | 50-100 milliseconds |

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
