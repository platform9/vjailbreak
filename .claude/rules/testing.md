---
paths:
  - "**/*_test.go"
  - "ui/e2e/**/*.spec.ts"
  - "**/*.test.ts"
  - "**/*.test.tsx"
---

# Testing Standards

Rules and requirements for testing across all vJailbreak components.

## Testing Philosophy

- Write tests before or alongside implementation
- Test behavior, not implementation details
- Aim for high coverage of critical paths
- Keep tests fast and reliable
- Make tests readable and maintainable

## Go Testing

### Unit Tests

**File naming**: `*_test.go` in the same package as the code being tested

**Test function naming**: `TestFunctionName` or `TestStructName_MethodName`

**Example**:
```go
func TestMigrationController_Reconcile(t *testing.T) {
    // Arrange
    // Act
    // Assert
}
```

### Table-Driven Tests
Use table-driven tests for multiple scenarios:
```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    error
        wantErr bool
    }{
        // test cases
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

### Component-Specific Requirements

#### Controller Tests (`k8s/migration/`)
```bash
cd k8s/migration
make test
```

**Requirements**:
- Use `envtest` for integration testing with real Kubernetes API
- Mock external dependencies (vCenter, OpenStack)
- Test reconciliation loops thoroughly
- Test error handling and retry logic
- Verify status updates

#### V2V Helper Tests (`v2v-helper/`)
```bash
make test-v2v-helper
# Or directly:
cd v2v-helper
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go test ./... -v
```

**CRITICAL REQUIREMENTS**:
- Tests REQUIRE `CGO_ENABLED=1 GOOS=linux GOARCH=amd64`
- Tests will NOT compile on macOS without Linux cross-compilation toolchain
- Use Docker or Linux VM for development on macOS
- Mock libguestfs operations where possible
- Test disk conversion workflows
- Test error recovery scenarios

#### API Server Tests (`pkg/vpwned/`)
```bash
cd pkg/vpwned
go test ./... -v
```

**Requirements**:
- Test API handlers with mock dependencies
- Test OpenStack client integration
- Test request validation
- Verify OpenAPI spec compliance
- Mock external API calls

### Mocking

**Use interfaces for dependencies**:
```go
type VMwareClient interface {
    GetVM(ctx context.Context, id string) (*VM, error)
}

// Mock in tests
type mockVMwareClient struct {
    getVMFunc func(ctx context.Context, id string) (*VM, error)
}
```

**Use testify/mock for complex mocks**:
```go
import "github.com/stretchr/testify/mock"

type MockClient struct {
    mock.Mock
}

func (m *MockClient) GetVM(ctx context.Context, id string) (*VM, error) {
    args := m.Called(ctx, id)
    return args.Get(0).(*VM), args.Error(1)
}
```

### Test Coverage

**Run with coverage**:
```bash
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**Coverage expectations**:
- Critical paths: >80% coverage
- Error handling: Test all error paths
- Edge cases: Test boundary conditions

## TypeScript/JavaScript Testing

### Unit Tests (React Testing Library)

**File naming**: `*.test.ts` or `*.test.tsx` alongside component files

**Example**:
```typescript
import { render, screen } from '@testing-library/react';
import { MigrationForm } from './MigrationForm';

describe('MigrationForm', () => {
  it('renders form fields', () => {
    render(<MigrationForm />);
    expect(screen.getByLabelText('VM Name')).toBeInTheDocument();
  });
});
```

### E2E Tests (Playwright)

**Location**: `ui/e2e/` — recursive, discovered automatically via `testDir: './e2e'`

**Cypress is retired.** `ui/cypress/` no longer exists and `cypress` is not a dependency. Do not
reintroduce it or add a third E2E runner (Constitution Principle XI).

**Running tests**:
```bash
cd ui
yarn pw:run                       # whole suite, starts the dev server itself
yarn pw:run e2e/migration/        # one directory
yarn pw:open                      # interactive UI mode
yarn pw:run --shard=1/8           # what CI runs, per shard
```

**Test critical user flows**:
- Migration creation workflow (standard and rolling)
- Migration status monitoring, pod logs, admin cutover
- Retry flow, error handling and validation
- Network and storage mapping
- Global Settings and credentials

**Stub every backend call** — specs must never reach a real vCenter, OpenStack or Kubernetes API.
Reuse the shared helpers rather than hand-rolling routes:

```typescript
import { goToMigrations, mockRoute, API } from './helpers/migration.helpers'
import { MOCK_MIGRATIONS_LIST } from './helpers/migration.fixtures'

test('migrations table renders', async ({ page }) => {
  await mockRoute(page, API.migrations, 'GET', MOCK_MIGRATIONS_LIST)
  await goToMigrations(page)
  await expect(page.getByTestId('migrations-table')).toBeVisible()
})
```

**Gotchas learned the hard way**:
- Playwright matches the **most recently registered** route first. A trailing `**` glob on a list
  endpoint will swallow by-name requests — anchor list routes with a regex (see the `API` map)
- `mockRoute` falls back (not `continue`) on method mismatch, so GET + PATCH + DELETE can be
  stacked on one URL
- CI has no API host, so `vite.config.ts` points the dev proxy at a dead local port. Requests a
  spec forgets to stub are refused instantly instead of stalling on DNS

## Testing Best Practices

### Arrange-Act-Assert Pattern
```go
func TestExample(t *testing.T) {
    // Arrange - set up test data and dependencies
    input := "test"
    expected := "result"
    
    // Act - execute the code under test
    result := Function(input)
    
    // Assert - verify the results
    if result != expected {
        t.Errorf("got %v, want %v", result, expected)
    }
}
```

### Test Isolation
- Each test should be independent
- Don't rely on test execution order
- Clean up resources after tests
- Use fresh test data for each test

### Test Naming
- Use descriptive test names
- Include the scenario being tested
- Make failures easy to understand
- Use `t.Run()` for subtests

### Error Messages
```go
// BAD
if got != want {
    t.Error("failed")
}

// GOOD
if got != want {
    t.Errorf("Function(%q) = %v, want %v", input, got, want)
}
```

## Integration Testing

### Testing with Real Dependencies
- Use test environments when possible
- Clean up test data after runs
- Don't test against production
- Use feature flags for testing

### Testing Kubernetes Controllers
- Use `envtest` for real API server
- Test CRD creation and updates
- Verify reconciliation behavior
- Test status updates and conditions

### Testing OpenStack Integration
- Mock OpenStack APIs in unit tests
- Use test OpenStack deployment for integration tests
- Verify API call sequences
- Test error handling and retries

## Pre-Commit Testing

### Required Before Committing
- Run relevant test suites
- Fix any failing tests
- Ensure no test warnings
- Verify test coverage hasn't decreased

### Git Hooks
```bash
# Set up pre-commit hooks
make setup-hooks
```

Pre-commit hooks will:
- Validate code formatting
- Run basic checks
- Prevent commits with obvious issues

## Continuous Integration

### PR Requirements
- All tests must pass
- No decrease in test coverage
- New features must include tests
- Bug fixes must include regression tests

### Test Execution in CI
- Controller tests run automatically
- v2v-helper tests run in Linux environment
- UI unit tests (vitest) run in the `ui-unit` job of `packer.yml`
- UI E2E tests (Playwright) run in the `ui-e2e` job of `packer.yml`, split across 8 shards; both
  jobs only run when the change touches `ui/`, and both gate `build-ui`
- Integration tests run against test deployments

## Debugging Tests

### Running Single Tests
```bash
# Go
go test -run TestSpecificTest ./path/to/package

# TypeScript
yarn test MigrationForm.test.tsx
```

### Verbose Output
```bash
# Go
go test -v ./...

# TypeScript
yarn test --verbose
```

### Test Debugging
```go
// Add debug output
t.Logf("Debug info: %+v", data)

// Skip long-running tests during development
if testing.Short() {
    t.Skip("skipping in short mode")
}
```

## Performance Testing

### Benchmarks (Go)
```go
func BenchmarkFunction(b *testing.B) {
    for i := 0; i < b.N; i++ {
        Function()
    }
}
```

**Running benchmarks**:
```bash
go test -bench=. ./...
```

### Load Testing
- Test migration at scale
- Verify resource usage
- Test concurrent operations
- Monitor memory and CPU

## Test Data Management

### Test Fixtures
- Store test data in `testdata/` directories
- Use realistic but anonymized data
- Version control test fixtures
- Document test data requirements

### Cleanup
```go
func TestExample(t *testing.T) {
    // Setup
    resource := createTestResource()
    defer resource.Cleanup()
    
    // Test logic
}
```

## Common Testing Pitfalls

### Avoid
- Flaky tests (non-deterministic failures)
- Tests that depend on external services without mocks
- Tests that modify global state
- Tests that take too long to run
- Tests that don't clean up resources

### Fix
- Use mocks for external dependencies
- Reset global state between tests
- Implement proper cleanup
- Optimize slow tests
- Make tests deterministic
