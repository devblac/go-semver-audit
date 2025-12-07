# Contributing to go-semver-audit

Thank you for your interest in contributing to go-semver-audit! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful, constructive, and professional in all interactions.

## How to Contribute

### Reporting Bugs

Before creating a bug report:
1. Check existing issues to avoid duplicates
2. Gather relevant information (Go version, OS, command used, expected vs actual behavior)

When creating a bug report, include:
- Clear, descriptive title
- Steps to reproduce
- Expected behavior
- Actual behavior
- Environment details (Go version, OS)
- Any relevant logs or output

### Suggesting Enhancements

Enhancement suggestions are welcome! Include:
- Clear description of the feature
- Use cases and motivation
- Possible implementation approach (optional)
- Examples of similar features in other tools (if applicable)

### Pull Requests

1. **Fork the repository** and create your branch from `main`
2. **Make your changes** following the code style guidelines
3. **Add tests** for any new functionality
4. **Update documentation** if you change behavior
5. **Run the test suite** and ensure everything passes
6. **Submit your pull request**

## Development Setup

### Prerequisites

- Go 1.21 or later
- Git
- Make (optional, but recommended)

### Getting Started

```bash
# Clone your fork
git clone https://github.com/devblac/go-semver-audit.git
cd go-semver-audit

# Install dependencies
go mod download

# Build the project
make build

# Run tests
make test
```

## Code Style Guidelines

### Go Style

Follow [Effective Go](https://go.dev/doc/effective_go) and the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments):

- Use `gofmt` to format code (or `make fmt`)
- Keep functions small and focused
- Use meaningful variable and function names
- Document all exported functions, types, and packages
- Handle errors explicitly, never ignore them
- Prefer table-driven tests

### Examples

#### Good
```go
// ParseConfig parses a configuration file and returns the config or an error.
func ParseConfig(path string) (*Config, error) {
    if path == "" {
        return nil, errors.New("path cannot be empty")
    }
    // ... implementation
}
```

#### Bad
```go
func pc(p string) (*Config, error) {
    // ... implementation with no documentation
}
```

### Testing

- Write table-driven tests when possible
- Test edge cases and error conditions
- Use descriptive test names
- Keep tests focused and independent

Example:
```go
func TestParseUpgrade(t *testing.T) {
    tests := []struct {
        name    string
        spec    string
        want    *Upgrade
        wantErr bool
    }{
        {
            name: "valid upgrade",
            spec: "github.com/pkg/errors@v0.9.1",
            want: &Upgrade{...},
            wantErr: false,
        },
        // ... more test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ... test implementation
        })
    }
}
```

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run tests for a specific package
go test ./internal/analyzer/...

# Run a specific test
go test -run TestParseUpgrade ./internal/analyzer
```

### Writing Tests

- Place tests in `*_test.go` files in the same package
- Use `testdata/` directory for test fixtures
- Mock external dependencies when necessary
- Ensure tests are deterministic and fast

## Commit Messages

Write clear, concise commit messages:

- Use present tense ("Add feature" not "Added feature")
- Use imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit first line to 72 characters
- Reference issues and pull requests when applicable

Examples:
```
Add support for interface method detection

Fix panic when analyzing empty modules

Update README with installation instructions

Closes #123
```

## Pull Request Process

1. Update README.md with details of changes if applicable
2. Update tests to cover your changes
3. Ensure all tests pass (`make check`)
4. Update CHANGELOG.md if significant (we'll handle this during review)
5. The PR will be merged once approved by maintainers

## Project Structure

```
go-semver-audit/
├── cmd/
│   └── go-semver-audit/     # CLI entrypoint
├── internal/
│   ├── analyzer/            # Core analysis logic
│   └── report/              # Output formatting
├── testdata/                # Test fixtures
├── Makefile                 # Build automation
└── README.md
```

## Areas for Contribution

Good first contributions:
- Improve error messages
- Add more test cases
- Enhance documentation
- Fix typos and formatting issues

More involved contributions:
- Support for build tags
- Better type alias handling
- Performance optimizations
- Support for additional output formats

## Questions?

If you have questions about contributing, feel free to:
- Open an issue with the "question" label
- Reach out to maintainers

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

