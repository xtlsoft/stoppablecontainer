# Security

This document covers the security model, considerations, and best practices for StoppableContainer.

## Security Model Overview

StoppableContainer operates with a split security model:

| Component | Security Level | Reason |
|-----------|---------------|--------|
| Provider Pod | Privileged | Required for Bidirectional mount propagation |
| Consumer Pod | Capabilities | Only SYS_ADMIN and SYS_CHROOT needed |
| Controller | Standard | No elevated privileges |

## Provider Pod Security

### Why Privileged?

The provider pod requires `privileged: true` due to a Kubernetes limitation:

!!! warning "Kubernetes Requirement"
    Bidirectional mount propagation is only allowed for privileged containers.
    
    See: [Kubernetes Mount Propagation Documentation](https://kubernetes.io/docs/concepts/storage/volumes/#mount-propagation)

The provider pod needs Bidirectional mount propagation to share the rootfs with the host and subsequently with consumer pods.

### Mitigations

To reduce risk from the privileged provider pod:

1. **Minimal attack surface**: The provider only runs a simple shell script
2. **No network access needed**: Consider adding NetworkPolicy
3. **No secrets mounted**: Only the rootfs volume is mounted
4. **Runs a controlled image**: Same image as user container, but predictable entrypoint

Example NetworkPolicy:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-provider-egress
spec:
  podSelector:
    matchLabels:
      stoppablecontainer.xtlsoft.top/role: provider
  policyTypes:
    - Egress
  egress: []  # Deny all egress
```

## Consumer Pod Security

### Capabilities Instead of Privileged

Consumer pods use Linux capabilities instead of privileged mode:

```yaml
securityContext:
  capabilities:
    add:
      - SYS_ADMIN
      - SYS_CHROOT
```

### Why These Capabilities?

| Capability | Required For |
|------------|--------------|
| `SYS_ADMIN` | Mounting proc, sys, and dev filesystems |
| `SYS_CHROOT` | Calling chroot() to enter the rootfs |

### What This Allows

With these capabilities, the container can:

- ✅ Mount filesystems (proc, sys, dev)
- ✅ Chroot into directories
- ❌ Access other namespaces
- ❌ Load kernel modules
- ❌ Modify system settings
- ❌ Access raw devices (beyond /dev bind mount)

### Capability Merging

If you specify additional capabilities, they're merged with the required ones:

```yaml
spec:
  template:
    container:
      securityContext:
        capabilities:
          add:
            - NET_ADMIN  # Your additional capability
            # SYS_ADMIN and SYS_CHROOT added automatically
```

## Isolation Considerations

### Filesystem Isolation

The consumer container chroots into the extracted rootfs:

- ✅ Cannot access host filesystem (except /dev via bind mount)
- ✅ Has its own proc/sys mounts
- ⚠️ Shares the rootfs with other consumers of the same instance

### Network Isolation

Consumer pods use standard Kubernetes networking:

- ✅ Normal pod network policies apply
- ✅ Can use service mesh
- ✅ Separate network namespace from host

### Process Isolation

Processes run in the consumer container's namespaces:

- ✅ Separate PID namespace
- ✅ Separate user namespace (if configured)
- ✅ Standard container isolation

## Best Practices

### 1. Use Pod Security Standards

Apply appropriate Pod Security Standards to namespaces:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: stoppable-workloads
  labels:
    pod-security.kubernetes.io/enforce: baseline
    pod-security.kubernetes.io/warn: restricted
```

Note: StoppableContainer pods require `baseline` level, not `restricted`.

### 2. Limit Node Access

Use node selectors and taints to isolate StoppableContainer workloads:

```yaml
spec:
  template:
    nodeSelector:
      workload-type: stoppable
    tolerations:
      - key: "stoppable-only"
        operator: "Exists"
        effect: "NoSchedule"
```

### 3. Network Policies

Implement network policies to restrict traffic:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: stoppable-policy
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/managed-by: stoppablecontainer
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              role: frontend
  egress:
    - to:
        - podSelector:
            matchLabels:
              role: database
```

### 4. Resource Limits

Always set resource limits to prevent DoS:

```yaml
spec:
  template:
    container:
      resources:
        limits:
          cpu: "1"
          memory: "512Mi"
        requests:
          cpu: "100m"
          memory: "128Mi"
```

### 5. Use Read-Only Root Filesystem

For extra security, consider read-only containers:

```yaml
spec:
  template:
    container:
      securityContext:
        readOnlyRootFilesystem: true
```

Note: This may require configuring writable volumes for logs/temp files.

### 6. Run as Non-Root

When possible, run as non-root:

```yaml
spec:
  template:
    container:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        runAsGroup: 1000
```

## Threat Model

### Potential Threats

| Threat | Mitigation |
|--------|------------|
| Provider pod compromise | Limited attack surface, no secrets, network isolation |
| Consumer pod escape | Capabilities limited to SYS_ADMIN/SYS_CHROOT |
| Rootfs tampering | Consider read-only rootfs, use volumes for state |
| Cross-container attacks | Each instance has separate rootfs |
| Privilege escalation | Minimal capabilities, non-root when possible |

### Attack Vectors

#### Container Escape via SYS_ADMIN

The SYS_ADMIN capability is powerful but mitigated by:

- Chroot environment limits accessible filesystems
- No access to host processes
- Standard namespace isolation still applies

#### Rootfs Modification

Malicious modification of rootfs could affect restarts:

- Consider immutable infrastructure patterns
- Use volumes for any state that shouldn't persist
- Monitor rootfs for unauthorized changes

## Compliance Considerations

### Container Security Benchmarks

StoppableContainer may require exceptions for:

- CIS Benchmark: 5.2.1 (privileged containers) - Provider only
- CIS Benchmark: 5.2.7 (NET_RAW capability) - Not required

Document these exceptions in your security policies.

### Auditing

Enable Kubernetes audit logging for StoppableContainer resources:

```yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: RequestResponse
    resources:
      - group: stoppablecontainer.xtlsoft.top
        resources: ["*"]
```

## Next Steps

- [Architecture](architecture.md) - Understand the system design
- [Configuration](../getting-started/configuration.md) - Security-related configuration
