# StoppableContainer Helm Chart

A Helm chart for deploying the StoppableContainer operator on Kubernetes.

## Prerequisites

- Kubernetes 1.25+
- Helm 3.0+

## Installation

### Add the Helm repository

```bash
helm repo add stoppablecontainer https://xtlsoft.github.io/stoppablecontainer
helm repo update
```

### Install the chart

```bash
helm install stoppablecontainer stoppablecontainer/stoppablecontainer \
  -n stoppablecontainer-system \
  --create-namespace
```

### Install with custom values

```bash
helm install stoppablecontainer stoppablecontainer/stoppablecontainer \
  -n stoppablecontainer-system \
  --create-namespace \
  -f my-values.yaml
```

## CRDs

This chart includes CRDs in the `crds/` directory. Helm will automatically install them on first install.

**Important notes about CRDs:**
- CRDs are installed automatically during `helm install`
- CRDs are **NOT** upgraded during `helm upgrade` (Helm's default behavior for safety)
- CRDs are **NOT** deleted during `helm uninstall` (to prevent data loss)

To manually upgrade CRDs:
```bash
kubectl apply -f https://raw.githubusercontent.com/xtlsoft/stoppablecontainer/main/config/crd/bases/stoppablecontainer.xtlsoft.top_stoppablecontainers.yaml
kubectl apply -f https://raw.githubusercontent.com/xtlsoft/stoppablecontainer/main/config/crd/bases/stoppablecontainer.xtlsoft.top_stoppablecontainerinstances.yaml
```

To delete CRDs (⚠️ this will delete all StoppableContainer resources):
```bash
kubectl delete crd stoppablecontainers.stoppablecontainer.xtlsoft.top
kubectl delete crd stoppablecontainerinstances.stoppablecontainer.xtlsoft.top
```

## Configuration

See [values.yaml](values.yaml) for the full list of configurable parameters.

### Key Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.image.repository` | Controller image repository | `ghcr.io/xtlsoft/stoppablecontainer` |
| `controller.image.tag` | Controller image tag | `appVersion` |
| `controller.replicas` | Number of controller replicas | `1` |
| `mountHelper.enabled` | Enable mount-helper DaemonSet | `true` |
| `mountHelper.image.repository` | Mount-helper image repository | `ghcr.io/xtlsoft/stoppablecontainer-mount-helper` |
| `global.hostPathPrefix` | Host path for mount propagation | `/var/lib/stoppablecontainer` |

## Uninstallation

```bash
helm uninstall stoppablecontainer -n stoppablecontainer-system
kubectl delete namespace stoppablecontainer-system
```

**Note**: CRDs are not deleted automatically. See the CRDs section above for manual deletion.
