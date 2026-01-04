# Security

This document covers the security model, considerations, and best practices for StoppableContainer.

## Security Model Overview

StoppableContainer uses a DaemonSet-based architecture to centralize privileged operations:

| Component | Security Level | Reason |
|-----------|---------------|--------|
| mount-helper DaemonSet | Privileged | Handles all mount/chroot operations |
| Provider Pod | Standard | No elevated privileges required |
| Consumer Pod | Standard | No elevated privileges required |
| Controller | Standard | No elevated privileges |

!!! success "Improved Security Model"
    With the DaemonSet architecture, application pods (providers and consumers) run with **no special privileges**. All privileged operations are centralized in the mount-helper DaemonSet.

## mount-helper DaemonSet Security

### Why Privileged?

The mount-helper DaemonSet requires elevated privileges to:

1. **Mount overlayfs**: Creating persistent overlayfs layers for container rootfs
2. **Bind mount**: Sharing the rootfs with consumer pods via HostPath
3. **Execute chroot operations**: Running exec-wrapper within the rootfs

!!! info "Privilege Centralization"
    Instead of every provider/consumer pod needing capabilities, only the mount-helper DaemonSet runs privileged. This is a more secure design.

### Mitigations

The mount-helper DaemonSet reduces risk through:

1. **Minimal attack surface**: Only performs mount operations
2. **Controlled operations**: Only responds to controller requests
3. **No application code**: Doesn't run user workloads
4. **Single instance per node**: Easy to audit and monitor

Example NetworkPolicy for mount-helper:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-mount-helper-egress
  namespace: stoppablecontainer-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: mount-helper
  policyTypes:
    - Egress
  egress: []  # Deny all egress
```

## Provider Pod Security

### No Special Privileges Required

Provider pods run with standard container security:

```yaml
# No special securityContext needed
securityContext: {}
```

The provider pod simply runs the container image's entrypoint. All rootfs sharing is handled by the mount-helper DaemonSet.

### Mitigations

1. **Standard isolation**: Normal container namespace isolation
2. **No host access**: Cannot access host filesystem directly
3. **Network policies**: Standard Kubernetes network policies apply

## Consumer Pod Security

### No Special Privileges Required

Consumer pods also run with standard container security:

```yaml
# No capabilities needed
securityContext: {}
```

The consumer pod uses the exec-wrapper (`/.sc-bin/sc-exec`) which communicates with the mount-helper DaemonSet to perform chroot operations.

### How exec-wrapper Works Securely

1. Consumer pod starts with a mounted HostPath volume
2. exec-wrapper sends RPC request to mount-helper DaemonSet
3. mount-helper (privileged) performs the chroot and executes the command
4. Output is relayed back to the consumer

This design means the consumer pod itself never needs SYS_ADMIN or SYS_CHROOT capabilities.

## Isolation Considerations

### Filesystem Isolation

Each StoppableContainerInstance has isolated filesystem via overlayfs:

- ✅ Each instance has its own writable layer
- ✅ Base image (lowerdir) is read-only and shared
- ✅ Changes are isolated per instance
- ✅ Cannot access host filesystem

### Network Isolation

All pods use standard Kubernetes networking:

- ✅ Normal pod network policies apply
- ✅ Can use service mesh
- ✅ Separate network namespace from host

### Process Isolation

Processes run in standard container namespaces:

- ✅ Separate PID namespace
- ✅ Separate user namespace (if configured)
- ✅ Standard container isolation
- ✅ No elevated capabilities in application pods

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

### 5. Secure the mount-helper DaemonSet

The mount-helper is the only privileged component. Protect it:

```yaml
# Restrict who can modify the DaemonSet
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: mount-helper-admin
  namespace: stoppablecontainer-system
rules:
  - apiGroups: ["apps"]
    resources: ["daemonsets"]
    resourceNames: ["mount-helper"]
    verbs: ["get", "list", "watch"]  # Read-only for most users
```

### 6. Run as Non-Root (Application Pods)

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
| mount-helper compromise | Minimal attack surface, no application code, network isolation |
| Provider pod compromise | No elevated privileges, standard container isolation |
| Consumer pod compromise | No elevated privileges, uses exec-wrapper via RPC |
| Rootfs tampering | Overlayfs isolation per instance, base image read-only |
| Cross-instance attacks | Each instance has separate overlayfs layers |
| Privilege escalation | Only DaemonSet is privileged, application pods are standard |

### Attack Vectors

#### mount-helper DaemonSet

The mount-helper is the most sensitive component:

- Runs privileged on every node
- Mitigated by: minimal functionality, network isolation, audit logging
- Recommendation: Monitor mount-helper logs and restrict access to its namespace

#### exec-wrapper RPC

The exec-wrapper communicates with mount-helper:

- Uses Unix socket or gRPC
- Mitigated by: socket permissions, request validation
- Recommendation: Ensure socket is not accessible outside intended pods

## Compliance Considerations

### Container Security Benchmarks

StoppableContainer may require exceptions for:

- CIS Benchmark: 5.2.1 (privileged containers) - mount-helper DaemonSet only
- Application pods (provider/consumer): No exceptions needed

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
