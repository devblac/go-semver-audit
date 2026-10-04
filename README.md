# go-semver-audit

[![CI](https://github.com/devblac/go-semver-audit/actions/workflows/ci.yml/badge.svg)](https://github.com/devblac/go-semver-audit/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/devblac/go-semver-audit/branch/main/graph/badge.svg)](https://codecov.io/gh/devblac/go-semver-audit)
[![Go Report Card](https://goreportcard.com/badge/github.com/devblac/go-semver-audit)](https://goreportcard.com/report/github.com/devblac/go-semver-audit)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/devblac/go-semver-audit)](https://github.com/devblac/go-semver-audit)

A production-ready CLI tool for analyzing breaking changes in Go dependency upgrades.

## Why This Exists

Upgrading Go dependencies is risky. Your code might compile successfully after bumping a version, but runtime breaks or unexpected behavior can slip through. While Go's semantic versioning and module system help, they don't catch everything—especially when maintainers accidentally break compatibility or you're jumping multiple versions.

`go-semver-audit` performs static analysis to compare the public API surface of your current dependency version against a proposed upgrade, then checks which exported symbols your code actually uses. It produces a risk report highlighting potential breaking changes that affect your project.

Think of it as a safety net before you commit to that upgrade.

## 30-second start
- Install: `go install github.com/devblac/go-semver-audit/cmd/go-semver-audit@latest`
- Run once: `go-semver-audit -upgrade github.com/pkg/errors@v0.9.1`
- Read the text report (default). Use `-json` for CI or `-strict` to fail on warnings.

## When to use it
- Before bumping a dependency (especially majors or multi-version jumps)
- When you need proof that an upgrade is safe for your code
- When a team asks “what will this break?” and you need a quick, actionable report

## Features

- **API Diff Analysis**: Compares exported types, functions, methods, and interfaces between dependency versions
- **Usage-Aware**: Only reports breaking changes for APIs you actually use in your code
- **Risk Reports**: Clear, actionable output showing removed functions, changed signatures, modified interfaces
- **Batch Mode**: Analyze multiple dependency upgrades in one run
- **Dead Dependency Detection**: Optionally identify unused dependencies after upgrades
- **CI-Friendly**: JSON output mode and non-zero exit codes for automation
- **Static Analysis Only**: No code execution, no compilation of untrusted code

## Installation

### From Source

```bash
go install github.com/devblac/go-semver-audit/cmd/go-semver-audit@latest
```

### Build Locally

```bash
git clone https://github.com/devblac/go-semver-audit.git
cd go-semver-audit
go build -o bin/go-semver-audit ./cmd/go-semver-audit
```

## Usage

### Basic Usage

Analyze a single dependency upgrade in the current directory:

```bash
go-semver-audit -upgrade github.com/gin-gonic/gin@v1.9.0
```

### Specify Project Path

```bash
go-semver-audit -path ./myproject -upgrade github.com/pkg/errors@v0.9.1
```

### JSON Output for CI

```bash
go-semver-audit -upgrade github.com/stretchr/testify@v1.8.0 -json
```

### HTML Report for Sharing

```bash
go-semver-audit -upgrade github.com/gorilla/mux@v1.8.0 -html > audit.html
```

### Strict Mode (Exit Non-Zero on Warnings)

```bash
go-semver-audit -upgrade golang.org/x/sync@v0.5.0 -strict
```

### Detect Unused Dependencies

```bash
go-semver-audit -upgrade github.com/gorilla/mux@v1.8.0 -unused
```

### Markdown Report for Pull Requests

```bash
go-semver-audit -upgrade github.com/gorilla/mux@v1.8.0 -markdown
```

## GitHub Action: Audit Dependabot and Renovate PRs

The action comments on every pull request that bumps a Go dependency, listing the breaking changes that hit **your** code — which symbols changed, the old and new signatures, and the file and line where you use them. On later pushes it updates the same comment instead of adding new ones.

```yaml
# .github/workflows/semver-audit.yml
name: Dependency audit

on:
  pull_request:
    paths: ["**/go.mod"]

permissions:
  contents: read
  pull-requests: write # to post the comment

jobs:
  go-semver-audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - uses: devblac/go-semver-audit@main # pin a release tag or commit SHA
```

It works out which direct dependencies the PR upgrades by comparing the base and head `go.mod`, then analyzes each one against the PR's **base** commit — the code the upgrade is being applied to. Indirect dependencies are skipped.

| Input | Description | Default |
|-------|-------------|---------|
| `working-directory` | Directory containing the `go.mod` to audit, relative to the repository root | `.` |
| `comment` | Post the report as a PR comment | `true` |
| `fail-on-breaking` | Fail the step when an upgrade breaks code in this project | `true` |
| `fail-on-error` | Fail the step when an upgrade could not be analyzed | `false` |
| `github-token` | Token used to post the comment | `${{ github.token }}` |

Outputs: `breaking` (`"true"`/`"false"`), `upgrades` (number analyzed) and `report` (path to the Markdown report). The report is also written to the job summary, so it is visible even when the token cannot comment — for example on pull requests from forks.

For a monorepo, run the action once per module with a different `working-directory`.

## Example Output

```
Analyzing upgrade: github.com/example/lib v1.2.0 -> v2.0.0

⚠️  BREAKING CHANGES DETECTED

Removed Functions:
  - lib.OldHelper (used in: main.go:45, utils/helper.go:12)
  
Changed Signatures:
  - lib.ParseConfig
    Old: func ParseConfig(path string) (*Config, error)
    New: func ParseConfig(path string, opts ...Option) (*Config, error)
    Used in: config/loader.go:23
  
Modified Interfaces:
  - lib.Handler
    Removed method: Handle(ctx context.Context) error
    Added method: HandleWithContext(ctx context.Context, meta Metadata) error
    Implementations found in: handlers/http.go:67

Summary: 3 breaking changes affecting 4 locations in your code.
```

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-path` | Path to Go project to analyze | `.` (current directory) |
| `-upgrade` | Dependency upgrade in format `module@version` | (required) |
| `-json` | Output results as JSON | `false` |
| `-html` | Output results as HTML | `false` |
| `-markdown` | Output results as GitHub-flavored Markdown | `false` |
| `-strict` | Also exit non-zero on warnings (unused dependencies) | `false` |
| `-unused` | Report unused dependencies after upgrade | `false` |
| `-list-upgrades` | Print `module@version` for each direct dependency whose version differs between the project's `go.mod` and the given `go.mod`, then exit | - |
| `-v` | Verbose output | `false` |
| `-help` | Show help message | - |

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | No breaking changes |
| `1` | Breaking changes detected (or warnings, with `-strict`) |
| `2` | The analysis could not be completed |

## How It Works

1. **Parse Current State**: Load your Go project and identify current dependency versions
2. **Fetch Versions**: Download/cache both old and new versions of the target dependency
3. **Extract APIs**: Parse exported symbols (types, functions, methods, interfaces) from both versions
4. **Analyze Usage**: Scan your codebase to find which exported symbols you actually import and use
5. **Diff & Compare**: Identify removed symbols, changed signatures, modified interfaces
6. **Generate Report**: Output only breaking changes that affect symbols you use

## Limitations

This tool performs **static analysis only** and has inherent limitations:

- **Cannot detect behavioral changes**: If a function signature stays the same but behavior changes, we won't catch it
- **Reflection blind spots**: Dynamic calls via reflection may not be detected as usage
- **Vendored dependencies**: Analysis assumes standard module layout; vendored code may not be handled correctly
- **CGO dependencies**: Modules with cgo may not be fully analyzable
- **Semantic compatibility**: We detect API surface changes, not semantic versioning violations
- **Build tags**: May not analyze all conditional compilation branches
- **Type aliases**: Complex type aliasing chains may be oversimplified

**This tool is a safety aid, not a guarantee.** Always test your upgrades thoroughly.

## Testing

The project includes comprehensive test coverage and continuous integration.

### Running Tests Locally

```bash
# Run all tests
go test ./...

# Run tests with coverage
make test-coverage

# Run with race detector
go test -race ./...
```

See [TESTING.md](TESTING.md) for detailed testing documentation.

### Continuous Integration

CI runs automatically on all pull requests and pushes to main/develop branches:
- Tests across multiple OS (Linux, Windows, macOS) and Go versions (1.21, 1.22)
- Linting with `go vet`, `gofmt`, and `staticcheck`
- Coverage reporting via Codecov
- Build verification

[![CI Status](https://github.com/yourusername/go-semver-audit/actions/workflows/ci.yml/badge.svg)](https://github.com/yourusername/go-semver-audit/actions/workflows/ci.yml)

### Releases
- Tag a version: `git tag v0.x.y && git push origin v0.x.y`
- CI (GitHub Actions) builds cross-platform binaries via GoReleaser and publishes the GitHub Release
- Local dry run: `goreleaser release --skip=publish --clean`

## Contributing

Contributions welcome! This project aims to stay minimal and focused.

### Quick Start

```bash
# Clone the repository
git clone https://github.com/devblac/go-semver-audit.git
cd go-semver-audit

# Run tests
make test

# Run all checks (format, lint, test)
make check
```

### Adding Features

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for your changes
4. Ensure all tests pass and code is formatted (`go fmt ./...`)
5. Commit with clear messages (`git commit -m 'Add detection for interface embedding'`)
6. Push and open a Pull Request

### Code Style

- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Keep functions small and focused
- Use table-driven tests
- Document exported functions and types
- Handle errors explicitly, no silent failures

### Reporting Issues

Use GitHub Issues to report bugs or suggest features. Include:
- Go version (`go version`)
- Operating system
- Full command you ran
- Expected vs actual behavior
- Minimal reproduction example if possible

## Related Tools

- [golang.org/x/exp/apidiff](https://pkg.go.dev/golang.org/x/exp/apidiff): Experimental API diff tool (library-focused)
- [go mod graph](https://go.dev/ref/mod#go-mod-graph): Visualize dependency graphs
- [nancy](https://github.com/sonatype-nexus-community/nancy): Vulnerability scanner for Go dependencies

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

Built with `golang.org/x/tools/go/packages` for robust Go code analysis.