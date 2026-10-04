# go-semver-audit

[![CI](https://github.com/devblac/go-semver-audit/actions/workflows/ci.yml/badge.svg)](https://github.com/devblac/go-semver-audit/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/devblac/go-semver-audit/branch/main/graph/badge.svg)](https://codecov.io/gh/devblac/go-semver-audit)
[![Go Report Card](https://goreportcard.com/badge/github.com/devblac/go-semver-audit)](https://goreportcard.com/report/github.com/devblac/go-semver-audit)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/devblac/go-semver-audit)](https://github.com/devblac/go-semver-audit)

Find out which breaking changes in a Go dependency upgrade hit **your** code — which symbols, and where you use them — before you upgrade.

## Why This Exists

Semantic versioning promises that minor and patch releases are compatible, but Go modules break that promise more often than you'd like: `v0.x` modules may change anything, and maintainers sometimes break compatibility by accident. When a Dependabot or Renovate PR turns red, the build tells you *that* something broke, not what changed upstream or how much of your code it touches.

`go-semver-audit` compares the exported API of the version you use with the version you're upgrading to, then checks which of the changed symbols your code — including your tests — actually uses. You get a short report of the breaking changes that affect you, with old and new signatures and the file and line of every use.

## 30-second start
- Install: `go install github.com/devblac/go-semver-audit/cmd/go-semver-audit@latest`
- Run in your module: `go-semver-audit -upgrade github.com/pkg/errors@v0.9.1`
- Read the text report (default). Use `-json` or `-markdown` for automation, or the [GitHub Action](#github-action-audit-dependabot-and-renovate-prs) to audit dependency PRs automatically.

## When to use it
- Before bumping a dependency, especially `v0.x` modules and multi-version jumps
- On every Dependabot or Renovate PR, via the GitHub Action
- When a team asks “what will this break?” and you need a quick, actionable answer

## Features

- **API Diff Analysis**: Compares exported functions, methods, types, and interfaces between dependency versions
- **Usage-Aware**: Only reports changes to APIs your code uses, test files included, with the file and line of each use
- **Pull Request Bot**: A GitHub Action that audits every dependency a PR upgrades and comments with the result
- **CI-Friendly**: JSON and Markdown output, and exit codes that tell "breaking changes" apart from "analysis failed"
- **Unused Dependency Detection** (experimental): Optionally list dependencies the project no longer imports
- **Nothing Is Executed**: Dependency code is downloaded and type-checked, never run

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

A *minor* release, `v1.4.0` → `v1.5.0`, that removed a function, added a `context.Context` parameter to a method, and added a method to an interface the project implements in a test (`go-semver-audit -upgrade example.com/kvstore@v1.5.0 -v`):

```
Analyzing upgrade: example.com/kvstore v1.4.0 -> v1.5.0

⚠️  BREAKING CHANGES DETECTED

Summary: 3 breaking change(s) affecting 4 location(s).

What to fix next:
  - Remove/replace kvstore.OpenWithOptions (function) at main.go:11
  - Update call to kvstore.Store.Get at cache/cache.go:10, and 1 more
  - Update implementations of kvstore.Iterator at cache/cache_test.go:15

Removed Symbols:
  - kvstore.OpenWithOptions (function) (used in: main.go:11)

Changed Signatures:
  - kvstore.Store.Get
    Old: func(key string) ([]byte, error)
    New: func(ctx context.Context, key string) ([]byte, error)
    Used in: cache/cache.go:10, main.go:17

Modified Interfaces:
  - kvstore.Iterator
    Added methods:
      - Err() error
    Used in: cache/cache_test.go:15

Summary: 3 breaking change(s) affecting 4 location(s) in your code.
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

1. **Load the Project**: Type-check your module, test files included, and read the dependency's current version
2. **Fetch Versions**: Download both versions with `go mod download` (cached in the module cache)
3. **Extract APIs**: Type-check the dependency packages you import, in both versions, and collect their exported functions, methods, types, and interfaces
4. **Analyze Usage**: Find every exported symbol of the dependency your code uses, matched by package and receiver type
5. **Diff & Compare**: Identify removed symbols, changed signatures, and modified interfaces
6. **Generate Report**: Output only the changes that affect symbols you use

## Limitations

This tool performs **static analysis only** and has inherent limitations:

- **Cannot detect behavioral changes**: If a function signature stays the same but behavior changes, we won't catch it
- **Major versions are not supported**: A `/v2` (or later) release is a different module path, so `-upgrade example.com/lib/v2@v2.0.0` cannot be compared against `example.com/lib`
- **Not compared yet**: Exported variables and constants, struct fields, and changes to a type's underlying definition
- **Possible false positives**: Renaming a parameter is reported as a signature change, although callers are unaffected; a changed interface is reported wherever it is used, although adding a method only breaks code that *implements* it
- **Unused dependencies** (`-unused`) is experimental and can report dependencies that are in use
- **Reflection blind spots**: Dynamic calls via reflection may not be detected as usage
- **Vendored dependencies**: Analysis assumes standard module layout; vendored code may not be handled correctly
- **CGO dependencies**: Modules with cgo may not be fully analyzable
- **Semantic compatibility**: We detect API surface changes, not semantic versioning violations
- **Build tags**: May not analyze all conditional compilation branches
- **Type aliases**: Complex type aliasing chains may be oversimplified

**This tool is a safety aid, not a guarantee.** Always test your upgrades thoroughly.

## Troubleshooting

**`module ... not found in project dependencies`**: No package of your module imports the dependency, tests included. It may be an indirect dependency, or imported only under build tags that are not active on your platform.

**`packages contain errors`**: The project, including its tests, must compile at its *current* dependency versions. Check with `go vet ./...`.

**The first run is slow**: Both versions of the dependency, and their own dependencies, are downloaded and type-checked. Later runs reuse the module and build caches.

## Development

```bash
go test ./...          # all tests, including the offline end-to-end test
go test -short ./...   # unit tests only
make check             # format, lint and test
```

CI runs the tests on Linux, Windows and macOS with Go 1.24 and 1.27, plus `go vet`, `gofmt` and `staticcheck`. See [TESTING.md](TESTING.md) for details and [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

Releases: pushing a `v*` tag builds cross-platform binaries with GoReleaser and publishes a GitHub release.

## Related Tools

- [gorelease](https://pkg.go.dev/golang.org/x/exp/cmd/gorelease): Checks a module you *publish* for incompatible changes before tagging a release; `go-semver-audit` is the consumer-side counterpart
- [golang.org/x/exp/apidiff](https://pkg.go.dev/golang.org/x/exp/apidiff): Library for computing API differences between two package versions
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck): Reports known vulnerabilities that affect your code

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

Built with `golang.org/x/tools/go/packages` for robust Go code analysis.