# Contributing

Thank you for your interest in contributing to StoppableContainer! This guide will help you get started.

## Code of Conduct

Please be respectful and constructive in all interactions.

## Getting Started

### Prerequisites

- Go 1.21 or later
- Docker
- kubectl
- Access to a Kubernetes cluster (kind, minikube, or remote)
- make

### Setting Up Development Environment

1. **Fork and clone the repository**:

```bash
git clone https://github.com/YOUR_USERNAME/stoppablecontainer.git
cd stoppablecontainer
```

2. **Install dependencies**:

```bash
make install-tools
```

3. **Set up a local cluster** (if you don't have one):

```bash
# Using kind
kind create cluster --name sc-dev

# Or using minikube
minikube start
```

4. **Install cert-manager**:

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.0/cert-manager.yaml
kubectl wait --for=condition=available --timeout=300s deployment/cert-manager -n cert-manager
```

5. **Install CRDs**:

```bash
make install
```

### Running Locally

Run the controller locally against your cluster:

```bash
make run
```

This watches your kubeconfig cluster and runs the controller on your machine.

### Building and Deploying

Build and deploy to your cluster:

```bash
# Build the image
make docker-build IMG=my-registry/stoppablecontainer:dev

# Push (if using remote registry)
make docker-push IMG=my-registry/stoppablecontainer:dev

# Deploy
make deploy IMG=my-registry/stoppablecontainer:dev
```

For kind:

```bash
# Build and load directly
make docker-build IMG=stoppablecontainer:dev
kind load docker-image stoppablecontainer:dev --name sc-dev
make deploy IMG=stoppablecontainer:dev
```

## Project Structure

```
stoppablecontainer/
├── api/v1alpha1/           # CRD types and deepcopy
├── cmd/
│   ├── main.go             # Controller entrypoint
│   ├── exec-wrapper/       # Exec wrapper binary
│   └── pause/              # Pause container binary
├── config/
│   ├── crd/                # CRD manifests
│   ├── default/            # Default kustomize overlay
│   ├── manager/            # Controller deployment
│   └── rbac/               # RBAC rules
├── internal/
│   ├── controller/         # Reconciliation logic
│   └── provider/           # Pod builders
├── test/
│   ├── e2e/                # End-to-end tests
│   └── utils/              # Test utilities
└── docs-site/              # Documentation
```

## Making Changes

### Modifying CRDs

1. Edit types in `api/v1alpha1/`
2. Regenerate code:

```bash
make generate
make manifests
```

3. Update CRDs in cluster:

```bash
make install
```

### Modifying Controller Logic

1. Edit files in `internal/controller/` or `internal/provider/`
2. Run tests:

```bash
make test
```

3. Test locally:

```bash
make run
```

### Adding Tests

- Unit tests: Place in the same package with `_test.go` suffix
- E2E tests: Add to `test/e2e/`

Run all tests:

```bash
make test
make test-e2e
```

## Submitting Changes

### Commit Messages

Follow conventional commits:

```
type(scope): description

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Examples:

```
feat(controller): add support for pod priority

fix(provider): handle nil security context

docs: update installation guide
```

### Pull Request Process

1. Create a feature branch:

```bash
git checkout -b feature/my-feature
```

2. Make your changes and commit
3. Ensure tests pass:

```bash
make test
make lint
```

4. Push and create PR:

```bash
git push origin feature/my-feature
```

5. Fill out the PR template
6. Address review feedback
7. Squash and merge

## Code Style

### Go

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` and `golangci-lint`
- Run lint:

```bash
make lint
```

### Kubernetes Resources

- Use consistent naming: `stoppablecontainer-*`
- Include appropriate labels
- Document all fields

## Documentation

### Updating Docs

Documentation is in `docs-site/docs/`. It uses MkDocs with Material theme.

Preview locally:

```bash
pip install mkdocs-material mkdocstrings
mkdocs serve
```

### Adding New Pages

1. Create the markdown file in appropriate folder
2. Add to `mkdocs.yml` navigation
3. Link from related pages

## Getting Help

- Open an issue for bugs or feature requests
- Use discussions for questions
- Check existing issues before creating new ones

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.
