# kubectl-sc: kubectl Plugin for StoppableContainer

`kubectl-sc` is a kubectl plugin that provides a convenient command-line interface for managing StoppableContainers. It simplifies common operations like creating, starting, stopping, and executing commands in containers with **persistent root filesystems**.

## Overview

StoppableContainer enables containers whose root filesystem persists even when the workload is stopped. This is useful for:

- **Development environments** - Stop your container, preserve all installed packages and configurations
- **Stateful applications** - Maintain filesystem state across restarts without external volumes
- **Cost optimization** - Stop containers when not in use while preserving their state

## Installation

### From Binary Release

Download the latest release for your platform:

```bash
# Linux amd64
curl -LO https://github.com/xtlsoft/stoppablecontainer/releases/latest/download/kubectl-sc-linux-amd64
chmod +x kubectl-sc-linux-amd64
sudo mv kubectl-sc-linux-amd64 /usr/local/bin/kubectl-sc

# Linux arm64
curl -LO https://github.com/xtlsoft/stoppablecontainer/releases/latest/download/kubectl-sc-linux-arm64
chmod +x kubectl-sc-linux-arm64
sudo mv kubectl-sc-linux-arm64 /usr/local/bin/kubectl-sc

# macOS amd64
curl -LO https://github.com/xtlsoft/stoppablecontainer/releases/latest/download/kubectl-sc-darwin-amd64
chmod +x kubectl-sc-darwin-amd64
sudo mv kubectl-sc-darwin-amd64 /usr/local/bin/kubectl-sc

# macOS arm64 (Apple Silicon)
curl -LO https://github.com/xtlsoft/stoppablecontainer/releases/latest/download/kubectl-sc-darwin-arm64
chmod +x kubectl-sc-darwin-arm64
sudo mv kubectl-sc-darwin-arm64 /usr/local/bin/kubectl-sc
```

### From Source

```bash
go install github.com/xtlsoft/stoppablecontainer/cmd/kubectl-sc@latest
```

### Verify Installation

```bash
kubectl sc version
```

## Usage

### List StoppableContainers

```bash
# List in current namespace
kubectl sc list

# List in all namespaces
kubectl sc list -A

# Aliases: ls, get
kubectl sc ls
kubectl sc get
```

### Create a StoppableContainer

```bash
# Create with a simple command
kubectl sc create my-app --image=ubuntu:22.04 -- /bin/bash

# Create with environment variables
kubectl sc create my-app --image=nginx:latest -e PORT=8080 -e DEBUG=true

# Create with port mappings
kubectl sc create my-app --image=nginx:latest -p 80:http -p 443:https

# Create with working directory
kubectl sc create my-app --image=python:3.11 -w /app -- python app.py

# Create but don't start immediately
kubectl sc create my-app --image=ubuntu:22.04 --running=false -- /bin/bash
```

### Show Status

```bash
# Show detailed status
kubectl sc status my-app

# Output as JSON
kubectl sc status my-app -o json

# Output as YAML
kubectl sc status my-app -o yaml
```

### Start/Stop

```bash
# Start a container
kubectl sc start my-app

# Start and wait until running
kubectl sc start my-app --wait

# Start with custom timeout
kubectl sc start my-app --wait --timeout=5m

# Stop a container
kubectl sc stop my-app

# Stop and wait
kubectl sc stop my-app --wait
```

### Execute Commands

```bash
# Run a command
kubectl sc exec my-app -- ls -la /

# Interactive shell
kubectl sc exec my-app -it -- /bin/bash

# Run with stdin
echo "hello" | kubectl sc exec my-app -i -- cat
```

### View Logs

```bash
# View logs
kubectl sc logs my-app

# Follow logs
kubectl sc logs my-app -f

# Tail last N lines
kubectl sc logs my-app --tail=100

# Show timestamps
kubectl sc logs my-app --timestamps

# Previous container logs
kubectl sc logs my-app -p
```

### Delete

```bash
# Delete a container
kubectl sc delete my-app

# Force delete
kubectl sc delete my-app --force

# Aliases: rm, remove
kubectl sc rm my-app
```

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--namespace` | `-n` | Kubernetes namespace |
| `--kubeconfig` | | Path to kubeconfig file |
| `--all-namespaces` | `-A` | List across all namespaces |

## Examples

### Development Workflow

```bash
# Create a development container
kubectl sc create dev-env --image=ubuntu:22.04 -- /bin/bash

# Attach to it and install packages
kubectl sc exec dev-env -it -- /bin/bash
# Inside: apt update && apt install -y python3 nodejs npm ...

# Stop when not needed (preserves all installed packages!)
kubectl sc stop dev-env --wait

# Resume later (all your packages are still there)
kubectl sc start dev-env --wait

# Continue working
kubectl sc exec dev-env -it -- /bin/bash

# Delete when done (filesystem is destroyed)
kubectl sc delete dev-env
```

### Web Application

```bash
# Create a web server
kubectl sc create web-app --image=nginx:latest -p 80:http

# Check status
kubectl sc status web-app

# View logs
kubectl sc logs web-app -f

# Stop for maintenance
kubectl sc stop web-app

# Resume
kubectl sc start web-app
```

## Integration with Scripts

The plugin is designed to work well in scripts:

```bash
#!/bin/bash

# Create and wait for container to be ready
kubectl sc create my-app --image=python:3.11 -- python -m http.server 8080
kubectl sc start my-app --wait --timeout=2m

# Run tests
kubectl sc exec my-app -- python -m pytest /app/tests

# Cleanup
kubectl sc delete my-app
```

## Comparison with kubectl

| Operation | kubectl | kubectl sc |
|-----------|---------|------------|
| List | `kubectl get stoppablecontainers` | `kubectl sc list` |
| Create | Apply YAML manifest | `kubectl sc create NAME --image=IMAGE` |
| Start | Patch spec.running=true | `kubectl sc start NAME` |
| Stop | Patch spec.running=false | `kubectl sc stop NAME` |
| Exec | `kubectl exec NAME -- CMD` | `kubectl sc exec NAME -- CMD` |
| Logs | `kubectl logs NAME` | `kubectl sc logs NAME` |
| Delete | `kubectl delete stoppablecontainer NAME` | `kubectl sc delete NAME` |

!!! note "Direct kubectl exec now works"
    You can now use regular `kubectl exec NAME -- CMD` to execute commands inside the container. The command automatically runs inside the chroot environment with the user's rootfs. The consumer pod uses the same name as the StoppableContainerInstance (no `-consumer` suffix).
