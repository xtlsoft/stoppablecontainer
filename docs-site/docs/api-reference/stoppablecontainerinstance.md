# StoppableContainerInstance API Reference

This document provides the API reference for the StoppableContainerInstance custom resource.

!!! note
    StoppableContainerInstance is an internal resource managed by the controller. You typically don't need to create these directly.

## Resource Definition

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainerInstance
metadata:
  name: <string>
  namespace: <string>
spec:
  running: <boolean>
  node: <string>
  template:
    container: <ContainerSpec>
    nodeSelector: <map[string]string>
    tolerations: <[]Toleration>
status:
  phase: <string>
  node: <string>
  conditions: <[]Condition>
```

## Purpose

StoppableContainerInstance serves as an intermediary between StoppableContainer and the actual pods:

```
StoppableContainer
       │
       ▼
StoppableContainerInstance  ← You are here
       │
       ├─► Provider Pod
       └─► Consumer Pod
```

## Spec Fields

### `spec.running`

| Property | Value |
|----------|-------|
| Type | `boolean` |
| Required | Yes |

Whether the consumer should be running. Copied from parent StoppableContainer.

### `spec.node`

| Property | Value |
|----------|-------|
| Type | `string` |
| Required | No |

The specific node where pods should run. Set by the controller when the provider pod is scheduled.

### `spec.template`

| Property | Value |
|----------|-------|
| Type | `ContainerTemplate` |
| Required | Yes |

Container template copied from parent StoppableContainer.

## Status Fields

### `status.phase`

| Property | Value |
|----------|-------|
| Type | `string` |
| Values | `Pending`, `Running`, `Stopped`, `Error` |

Current phase of the instance.

### `status.node`

| Property | Value |
|----------|-------|
| Type | `string` |

Node where the pods are running.

### `status.conditions`

| Property | Value |
|----------|-------|
| Type | `[]Condition` |

Detailed conditions.

## Relationship with StoppableContainer

The controller maintains the following invariants:

1. Each StoppableContainer has exactly one StoppableContainerInstance
2. The Instance's spec mirrors the parent's spec
3. The parent's status mirrors the Instance's status

## Example

```yaml
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainerInstance
metadata:
  name: my-app
  namespace: default
  ownerReferences:
    - apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
      kind: StoppableContainer
      name: my-app
      controller: true
spec:
  running: true
  node: worker-1
  template:
    container:
      image: nginx:latest
      command: ["nginx", "-g", "daemon off;"]
status:
  phase: Running
  node: worker-1
  conditions:
    - type: Ready
      status: "True"
```

## Debugging

### View Instance for a StoppableContainer

```bash
kubectl get stoppablecontainerinstance my-app -o yaml
```

### Check Instance Pods

```bash
# Provider
kubectl get pod my-app-provider

# Consumer (same name as the StoppableContainerInstance)
kubectl get pod my-app
```

### Manual Cleanup (Emergency)

If an Instance is stuck, you can delete it (the parent will recreate it):

```bash
kubectl delete stoppablecontainerinstance my-app
```

!!! warning
    This is for emergency situations only. Let the controller manage instances normally.

## See Also

- [StoppableContainer API Reference](stoppablecontainer.md)
- [Architecture](../concepts/architecture.md)
