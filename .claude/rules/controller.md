---
paths:
  - "k8s/migration/**/*.go"
---

# Controller Development Rules

Rules for developing the vJailbreak Kubernetes controller manager.

## External Documentation

**ALWAYS consult these resources when working on controller code:**
- **controller-runtime**: https://pkg.go.dev/sigs.k8s.io/controller-runtime
- **k3s documentation**: https://docs.k3s.io/ for appliance-specific Kubernetes behavior
- **Kubernetes API conventions**: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md

## CRD Development

### After Editing CRD Types
- ALWAYS run `make generate` inside `k8s/migration/` after editing types in `api/v1alpha1/`
- This regenerates:
  - `zz_generated.deepcopy.go` files
  - CRD YAML manifests
  - Client code
- Test changes with `cd k8s/migration && make test`

### CRD Best Practices
- Add proper validation tags to struct fields
- Include comprehensive status conditions
- Document fields with `// +kubebuilder:` markers
- Use standard Kubernetes API conventions for naming

## Controller Reconciliation

### Reconciler Patterns
- Implement idempotent reconciliation logic
- Use `ctrl.Result{Requeue: true}` for transient errors
- Return errors for permanent failures
- Update status conditions to reflect reconciliation state

### Error Handling
- Distinguish between transient and permanent errors
- Log errors with appropriate context
- Update CR status to reflect error conditions
- Use exponential backoff for retries

## Testing

### Test Requirements
- Write unit tests for reconciliation logic
- Use `envtest` for integration testing with real Kubernetes API
- Mock external dependencies (vCenter, OpenStack)
- Test error paths and edge cases

### Running Tests
```bash
cd k8s/migration
make test
```

## Build Targets

### Controller-Specific Make Targets
- `make docker-build` - Build controller image
- `make generate` - Generate deepcopy and CRD manifests
- `make manifests` - Generate CRD YAML only
- `make test` - Run controller tests
- `make lint` - Run golangci-lint

### Image Building
- Use `make vjail-controller` from repo root to build controller + v2v-helper
- Use `make vjail-controller-only` to build only controller (skip v2v-helper rebuild)

## Module Management

- This is an independent Go module at `k8s/migration/`
- Run `go mod tidy` from `k8s/migration/` directory
- Cross-module imports use full module path: `github.com/platform9/vjailbreak/pkg/common/...`

## Common Patterns

### Client Usage
- Use `client.Client` from controller-runtime for Kubernetes operations
- Prefer `Get`, `List`, `Create`, `Update`, `Patch` over raw API calls
- Use `Status().Update()` for status subresource updates

### Logging
- Use structured logging with `logr.Logger`
- Include relevant context (CR name, namespace, reconciliation attempt)
- Log at appropriate levels (Info, Error, Debug)

## Debugging

### Local Development
```bash
# Run controller locally against configured cluster
make run-local
```

### Checking Logs
```bash
kubectl -n vjailbreak logs -l control-plane=controller-manager -f
```

### Inspecting CRs
```bash
kubectl -n migration-system get migration <name> -o yaml
```
