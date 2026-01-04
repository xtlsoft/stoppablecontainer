# Testing

This guide covers the testing strategy and how to run tests for StoppableContainer.

## Test Types

| Type | Location | Purpose |
|------|----------|---------|
| Unit Tests | `*_test.go` files | Test individual functions |
| Integration Tests | `internal/controller/suite_test.go` | Test controller with envtest |
| E2E Tests | `test/e2e/` | Test full system on real cluster |

## Running Tests

### All Unit Tests

```bash
make test
```

### Specific Package

```bash
go test ./internal/provider/...
go test ./internal/controller/...
```

### With Coverage

```bash
make test-coverage
# or
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Verbose Output

```bash
go test ./... -v
```

## Unit Tests

### Provider Package Tests

Located in `internal/provider/`:

- `helpers_test.go` - Tests for helper functions
- `provider_pod_test.go` - Tests for provider pod builder
- `consumer_pod_test.go` - Tests for consumer pod builder

Run:

```bash
go test ./internal/provider/... -v
```

Example test:

```go
func TestGetHostPath(t *testing.T) {
    sci := &v1alpha1.StoppableContainerInstance{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test",
            Namespace: "default",
        },
    }
    
    expected := "/var/lib/stoppable-container/default-test"
    result := GetHostPath(sci)
    
    if result != expected {
        t.Errorf("expected %s, got %s", expected, result)
    }
}
```

### Controller Tests

Located in `internal/controller/`:

- `suite_test.go` - Test suite setup with envtest
- `stoppablecontainer_controller_test.go` - SC controller tests
- `stoppablecontainerinstance_controller_test.go` - SCI controller tests

Run:

```bash
go test ./internal/controller/... -v
```

These use [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest) to run a local API server.

## Integration Tests

Integration tests use envtest to run controllers against a local Kubernetes API:

```go
var _ = BeforeSuite(func() {
    testEnv = &envtest.Environment{
        CRDDirectoryPaths: []string{
            filepath.Join("..", "..", "config", "crd", "bases"),
        },
    }
    
    cfg, err := testEnv.Start()
    Expect(err).NotTo(HaveOccurred())
    
    // Setup manager and controllers
})
```

## E2E Tests

End-to-end tests run against a real Kubernetes cluster.

### Prerequisites

- Running Kubernetes cluster (kind recommended)
- kubectl configured
- cert-manager installed

### Running E2E Tests

```bash
# Full suite (builds image, deploys, tests)
make test-e2e

# Skip image build (use existing image)
DOCKER_BUILD_SKIP=true make test-e2e

# Skip cert-manager install
CERT_MANAGER_INSTALL_SKIP=true make test-e2e
```

### E2E Test Structure

```go
var _ = Describe("Manager", Ordered, func() {
    BeforeAll(func() {
        // Install CRDs, deploy controller
    })
    
    AfterAll(func() {
        // Cleanup
    })
    
    It("should run successfully", func() {
        // Verify controller is running
    })
    
    It("should create and manage a StoppableContainer", func() {
        // Create SC, verify pods, test lifecycle
    })
})
```

### E2E Test Environment Variables

| Variable | Description |
|----------|-------------|
| `DOCKER_BUILD_SKIP` | Skip Docker image build |
| `CERT_MANAGER_INSTALL_SKIP` | Skip cert-manager installation |
| `PROJECT_IMAGE` | Override controller image |

## Writing Tests

### Unit Test Guidelines

1. Test one thing per test
2. Use table-driven tests for variations
3. Mock external dependencies
4. Keep tests fast

Example table-driven test:

```go
func TestBuildUserCommand(t *testing.T) {
    tests := []struct {
        name     string
        command  []string
        args     []string
        expected []string
    }{
        {
            name:     "command only",
            command:  []string{"/bin/sh"},
            args:     nil,
            expected: []string{"/bin/sh"},
        },
        {
            name:     "command with args",
            command:  []string{"/bin/sh"},
            args:     []string{"-c", "echo hello"},
            expected: []string{"/bin/sh", "-c", "echo hello"},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### E2E Test Guidelines

1. Clean up resources after tests
2. Use Eventually for async checks
3. Include meaningful failure messages
4. Test both happy path and error cases

Example:

```go
It("should create provider pod", func() {
    // Create resource
    sc := &v1alpha1.StoppableContainer{...}
    Expect(k8sClient.Create(ctx, sc)).To(Succeed())
    
    // Wait for pod
    Eventually(func(g Gomega) {
        pod := &corev1.Pod{}
        err := k8sClient.Get(ctx, types.NamespacedName{
            Name:      sc.Name + "-provider",
            Namespace: sc.Namespace,
        }, pod)
        g.Expect(err).NotTo(HaveOccurred())
        g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
    }, 2*time.Minute, time.Second).Should(Succeed())
    
    // Cleanup
    Expect(k8sClient.Delete(ctx, sc)).To(Succeed())
})
```

## Test Coverage

Generate and view coverage:

```bash
# Generate coverage
go test ./... -coverprofile=coverage.out

# View in terminal
go tool cover -func=coverage.out

# View in browser
go tool cover -html=coverage.out
```

Target coverage areas:

| Package | Target |
|---------|--------|
| `internal/provider` | 80%+ |
| `internal/controller` | 70%+ |
| `api/v1alpha1` | Generated code, lower priority |

## Debugging Tests

### Verbose Output

```bash
go test -v ./internal/provider/...
```

### Run Single Test

```bash
go test -v -run TestGetHostPath ./internal/provider/...
```

### With Debugger

```bash
dlv test ./internal/provider/ -- -test.run TestGetHostPath
```

### E2E Debug Mode

Keep cluster resources after test failure:

```go
AfterEach(func() {
    if CurrentSpecReport().Failed() {
        // Don't cleanup, inspect manually
        Skip("Leaving resources for debugging")
    }
})
```

## CI/CD

Tests run automatically on:

- Pull requests
- Pushes to main
- Release tags

GitHub Actions workflow:

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      - run: make test
      
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: helm/kind-action@v1
      - run: make test-e2e
```

## Next Steps

- [Building](building.md) - Build instructions
- [Contributing](contributing.md) - Contribution guidelines
