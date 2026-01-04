# Installation

This guide walks you through installing StoppableContainer on your Kubernetes cluster.

## Prerequisites

Before installing StoppableContainer, ensure you have:

- **Kubernetes cluster** (v1.25 or later recommended)
- **kubectl** configured to access your cluster
- **cert-manager** installed (for webhook certificates)
- **Nodes with HostPath support** (most clusters have this)

## Installing cert-manager

StoppableContainer uses cert-manager for managing TLS certificates. If you don't have it installed:

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.0/cert-manager.yaml
```

Wait for cert-manager to be ready:

```bash
kubectl wait --for=condition=available --timeout=300s deployment/cert-manager -n cert-manager
kubectl wait --for=condition=available --timeout=300s deployment/cert-manager-webhook -n cert-manager
kubectl wait --for=condition=available --timeout=300s deployment/cert-manager-cainjector -n cert-manager
```

## Installing StoppableContainer

### Option 1: Using kubectl (Recommended)

Apply the release manifests directly:

```bash
# Install CRDs
kubectl apply -f https://raw.githubusercontent.com/xtlsoft/stoppablecontainer/main/config/crd/bases/stoppablecontainer.xtlsoft.top_stoppablecontainers.yaml
kubectl apply -f https://raw.githubusercontent.com/xtlsoft/stoppablecontainer/main/config/crd/bases/stoppablecontainer.xtlsoft.top_stoppablecontainerinstances.yaml

# Deploy the controller
kubectl apply -k https://github.com/xtlsoft/stoppablecontainer/config/default
```

### Option 2: Building from Source

Clone the repository and deploy:

```bash
git clone https://github.com/xtlsoft/stoppablecontainer.git
cd stoppablecontainer

# Build and push the image (replace with your registry)
make docker-build docker-push IMG=your-registry/stoppablecontainer:latest

# Deploy to cluster
make deploy IMG=your-registry/stoppablecontainer:latest
```

## Verifying Installation

Check that the controller is running:

```bash
kubectl get pods -n stoppablecontainer-system
```

You should see output similar to:

```
NAME                                                    READY   STATUS    RESTARTS   AGE
stoppablecontainer-controller-manager-xxxxx-xxxxx       1/1     Running   0          1m
```

Verify CRDs are installed:

```bash
kubectl get crd | grep stoppablecontainer
```

Expected output:

```
stoppablecontainerinstances.stoppablecontainer.xtlsoft.top   2024-01-01T00:00:00Z
stoppablecontainers.stoppablecontainer.xtlsoft.top           2024-01-01T00:00:00Z
```

## Uninstalling

To remove StoppableContainer from your cluster:

```bash
# Remove all StoppableContainer resources first
kubectl delete stoppablecontainers --all -A

# Wait for cleanup
kubectl wait --for=delete stoppablecontainerinstances --all -A --timeout=120s

# Undeploy the controller
make undeploy

# Or if you installed via kubectl:
kubectl delete -k https://github.com/xtlsoft/stoppablecontainer/config/default

# Remove CRDs
kubectl delete crd stoppablecontainers.stoppablecontainer.xtlsoft.top
kubectl delete crd stoppablecontainerinstances.stoppablecontainer.xtlsoft.top
```

## Troubleshooting

### Controller not starting

Check the controller logs:

```bash
kubectl logs -n stoppablecontainer-system -l control-plane=controller-manager
```

### cert-manager issues

Ensure cert-manager webhooks are ready:

```bash
kubectl get pods -n cert-manager
kubectl get certificate -n stoppablecontainer-system
```

### Image pull issues

If pods fail with `ImagePullBackOff`, ensure your cluster can access the container registry. For private registries, create an image pull secret:

```bash
kubectl create secret docker-registry regcred \
  --docker-server=your-registry \
  --docker-username=your-username \
  --docker-password=your-password \
  -n stoppablecontainer-system
```

## Next Steps

- [Quick Start Guide](quickstart.md) - Create your first StoppableContainer
- [Configuration](configuration.md) - Configure StoppableContainer options
