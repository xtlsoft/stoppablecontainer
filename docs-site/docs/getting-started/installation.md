# Installation

This guide walks you through installing StoppableContainer on your Kubernetes cluster.

## Prerequisites

Before installing StoppableContainer, ensure you have:

- **Kubernetes cluster** (v1.25 or later recommended)
- **kubectl** configured to access your cluster
- **cert-manager** installed (for webhook certificates)
- **Container runtime**: containerd (recommended) or Docker

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

### Option 1: Using Helm (Recommended)

```bash
# Add the Helm repository
helm repo add stoppablecontainer https://xtlsoft.github.io/stoppablecontainer/charts
helm repo update

# Install StoppableContainer
helm install stoppablecontainer stoppablecontainer/stoppablecontainer \
  -n stoppablecontainer-system --create-namespace
```

This installs:

- The StoppableContainer controller
- The mount-helper DaemonSet (runs on all nodes)
- Required CRDs and RBAC

### Option 2: Using YAML Manifest

For a simple installation without Helm, you can use the pre-built manifest file:

```bash
# Install all components with a single command
kubectl apply -f https://github.com/xtlsoft/stoppablecontainer/releases/latest/download/install.yaml
```

This manifest includes:

- Custom Resource Definitions (CRDs)
- Controller Deployment
- Mount-helper DaemonSet
- RBAC configuration (ServiceAccount, Role, RoleBinding)
- Webhook configuration

To install a specific version:

```bash
kubectl apply -f https://github.com/xtlsoft/stoppablecontainer/releases/download/v0.1.2/install.yaml
```

### Option 3: Building from Source

Clone the repository and deploy:

```bash
git clone https://github.com/xtlsoft/stoppablecontainer.git
cd stoppablecontainer

# Build and push images (replace with your registry)
make docker-build docker-push IMG=your-registry/stoppablecontainer:latest
make docker-build-exec-wrapper && docker push your-registry/stoppablecontainer-exec:latest
make docker-build-mount-helper && docker push your-registry/stoppablecontainer-mount-helper:latest

# Install CRDs
make install

# Deploy the mount-helper DaemonSet (required!)
make deploy-daemonset MOUNT_HELPER_IMG=your-registry/stoppablecontainer-mount-helper:latest

# Deploy the controller
make deploy IMG=your-registry/stoppablecontainer:latest
```

## Installing kubectl-sc Plugin (Optional)

The `kubectl-sc` plugin provides a convenient CLI for managing StoppableContainers:

```bash
# Linux amd64
curl -LO https://github.com/xtlsoft/stoppablecontainer/releases/latest/download/kubectl-sc-linux-amd64
chmod +x kubectl-sc-linux-amd64
sudo mv kubectl-sc-linux-amd64 /usr/local/bin/kubectl-sc

# Or install with Go
go install github.com/xtlsoft/stoppablecontainer/cmd/kubectl-sc@latest

# Verify installation
kubectl sc version
```

See [kubectl Plugin Guide](../user-guide/kubectl-plugin.md) for more details.

## Verifying Installation

Check that the controller and mount-helper are running:

```bash
kubectl get pods -n stoppablecontainer-system
```

You should see output similar to:

```
NAME                                                    READY   STATUS    RESTARTS   AGE
stoppablecontainer-controller-manager-xxxxx-xxxxx       1/1     Running   0          1m
stoppablecontainer-mount-helper-xxxxx                   1/1     Running   0          1m  # one per node
```

Verify CRDs are installed:

```bash
kubectl get crd | grep stoppablecontainer
```

Expected output:

```
stoppablecontainerinstances.stoppablecontainer.xtlsoft.top   2026-01-01T00:00:00Z
stoppablecontainers.stoppablecontainer.xtlsoft.top           2026-01-01T00:00:00Z
```

## Uninstalling

To remove StoppableContainer from your cluster:

### If installed with Helm:

```bash
# Remove all StoppableContainer resources first
kubectl delete stoppablecontainers --all -A

# Uninstall Helm release
helm uninstall stoppablecontainer -n stoppablecontainer-system
```

### If installed with YAML manifest:

```bash
# Remove all StoppableContainer resources first
kubectl delete stoppablecontainers --all -A

# Wait for cleanup
kubectl wait --for=delete stoppablecontainerinstances --all -A --timeout=120s

# Delete the manifest
kubectl delete -f https://github.com/xtlsoft/stoppablecontainer/releases/latest/download/install.yaml
```

### If installed from source:

```bash
# Remove all StoppableContainer resources first
kubectl delete stoppablecontainers --all -A

# Wait for cleanup
kubectl wait --for=delete stoppablecontainerinstances --all -A --timeout=120s

# Undeploy the controller and DaemonSet
make undeploy
make undeploy-daemonset

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
