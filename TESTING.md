# Testing Guide

This document describes the testing infrastructure and how to run tests for go-semver-audit.

## Running Tests

### All Tests

Run all tests with verbose output:

```bash
go test -v ./...
```

### Specific Package

Run tests for a specific package:

```bash
go test -v ./internal/analyzer/
go test -v ./internal/report/
go test -v ./cmd/go-semver-audit/
```

### With Race Detection

Run tests with race detector enabled:

```bash
go test -race ./...
```

## Test Coverage

### Generate Coverage Report

Generate a coverage profile and view the results:

```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./...

# View coverage summary
go tool cover -func=coverage.out

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html
```

Or use the Makefile:

```bash
make test-coverage
```

This will:
1. Run all tests with coverage enabled
2. Generate `coverage.out` (machine-readable)
3. Generate `coverage.html` (human-readable, open in browser)

### Coverage Thresholds

Current coverage (as of last update):
- `cmd/go-semver-audit`: 25.4%
- `internal/analyzer`: 26.4%
- `internal/report`: 99.0%

We don't enforce strict coverage thresholds, but aim to:
- Test all public APIs
- Cover critical business logic
- Test error conditions and edge cases
- Maintain or improve coverage with new changes

## Continuous Integration

### GitHub Actions

The project uses GitHub Actions for CI. The workflow (`.github/workflows/ci.yml`) runs on:
- Every push to `main` or `develop` branches
- Every pull request targeting `main` or `develop`

The CI pipeline includes:

#### Test Job
- **Matrix**: Tests across multiple OS (Ubuntu, Windows, macOS) and Go versions (1.21, 1.22)
- **Steps**:
  1. Checkout code
  2. Set up Go
  3. Download dependencies
  4. Verify dependencies
  5. Run tests with race detector and coverage
  6. Generate coverage reports
  7. Upload coverage to Codecov (Ubuntu/Go 1.22 only)

#### Lint Job
- Runs `go vet` for static analysis
- Checks code formatting with `gofmt`
- Runs `staticcheck` for additional linting

#### Build Job
- Verifies the binary can be built
- Tests the built binary with `-version` flag

### Local CI Simulation

To simulate the CI environment locally:

```bash
# Format check
gofmt -s -l .

# Vet check
go vet ./...

# Staticcheck (install first if needed)
go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./...

# Build
go build -v ./cmd/go-semver-audit

# Test with race detector and coverage
go test -race -coverprofile=coverage.out ./...
```

Or run all checks at once:

```bash
make check
```

## Test Structure

### Table-Driven Tests

We use table-driven tests throughout the codebase:

```go
func TestParseUpgrade(t *testing.T) {
    tests := []struct {
        name    string
        spec    string
        want    *Upgrade
        wantErr bool
    }{
        {
            name:    "valid upgrade",
            spec:    "github.com/pkg/errors@v0.9.1",
            want:    &Upgrade{Module: "github.com/pkg/errors", NewVersion: "v0.9.1"},
            wantErr: false,
        },
        // ... more test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseUpgrade(tt.spec)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseUpgrade() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            // ... assertions
        })
    }
}
```

### Test Files

Test files follow Go conventions:
- Named `*_test.go`
- Located in the same package as the code they test
- Use the `testing` package

### Test Data

The `testdata/` directory contains fixtures for integration tests:
- `oldlib/` - Example library at version 1.0
- `newlib/` - Example library at version 2.0
- `userproject/` - Example project using the library

## Writing Tests

### Guidelines

1. **Test Public APIs**: All exported functions, types, and methods should have tests
2. **Test Error Cases**: Don't just test the happy path
3. **Use Descriptive Names**: Test names should clearly describe what they test
4. **Keep Tests Focused**: Each test should verify one specific behavior
5. **Use Table-Driven Tests**: When testing multiple similar cases
6. **Avoid External Dependencies**: Mock or stub external services
7. **Make Tests Deterministic**: Tests should always produce the same result

### Example Test

```go
func TestFormatJSON(t *testing.T) {
    result := &analyzer.Result{
        Module:     "github.com/example/lib",
        OldVersion: "v1.0.0",
        NewVersion: "v2.0.0",
        Changes:    &analyzer.Diff{},
    }
    
    output, err := FormatJSON(result)
    if err != nil {
        t.Fatalf("FormatJSON() error = %v", err)
    }
    
    // Verify output is valid JSON
    var report JSONReport
    if err := json.Unmarshal([]byte(output), &report); err != nil {
        t.Errorf("FormatJSON() produced invalid JSON: %v", err)
    }
    
    // Verify key fields
    if report.Module != result.Module {
        t.Errorf("JSONReport.Module = %q, want %q", report.Module, result.Module)
    }
}
```

## Debugging Tests

### Run a Single Test

```bash
go test -run TestParseUpgrade ./internal/analyzer/
```

### Run with Verbose Output

```bash
go test -v -run TestParseUpgrade ./internal/analyzer/
```

### Show Test Coverage for Specific Test

```bash
go test -coverprofile=coverage.out -run TestParseUpgrade ./internal/analyzer/
go tool cover -func=coverage.out
```

### Use Debugger

Most Go IDEs support setting breakpoints in tests. Alternatively, use `dlv`:

```bash
go install github.com/go-delve/delve/cmd/dlv@latest
dlv test ./internal/analyzer/ -- -test.run TestParseUpgrade
```

## Benchmarks

Currently, the project doesn't have benchmark tests, but they can be added following Go's benchmark conventions:

```go
func BenchmarkParseUpgrade(b *testing.B) {
    spec := "github.com/pkg/errors@v0.9.1"
    for i := 0; i < b.N; i++ {
        _, _ = ParseUpgrade(spec)
    }
}
```

Run benchmarks:

```bash
go test -bench=. ./...
```

## Integration Testing

For integration testing with real Go modules:
1. Use the `testdata/` directory for fixture projects
2. Consider using temporary directories for test isolation
3. Mock external calls (e.g., downloading modules) when possible

## Code Quality Tools

### go vet

Static analysis tool that examines Go source code:

```bash
go vet ./...
```

### staticcheck

Advanced static analysis:

```bash
staticcheck ./...
```

### gofmt

Code formatting:

```bash
# Check formatting
gofmt -s -l .

# Apply formatting
gofmt -s -w .
```

Or use the Makefile:

```bash
make fmt
make lint
```

## Troubleshooting

### Tests Fail on Windows

Some tests may need adjustments for Windows file paths. Use `filepath.Join()` and `filepath.ToSlash()` for cross-platform compatibility.

### Coverage Files Not Generated

Ensure you're using the correct flags:

```bash
go test -coverprofile=coverage.out ./...
```

### Race Detector Warnings

Race conditions should be fixed, not ignored. The race detector helps find concurrent access issues:

```bash
go test -race ./...
```

## Contributing Tests

When contributing:
1. Add tests for new features
2. Update tests for changed behavior
3. Ensure all tests pass locally before pushing
4. Verify coverage hasn't decreased significantly
5. Follow the existing test patterns and style

See [CONTRIBUTING.md](CONTRIBUTING.md) for more details.


