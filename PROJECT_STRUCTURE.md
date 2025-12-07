# Project Structure

This document provides an overview of the `go-semver-audit` project structure.

## Directory Layout

```
go-semver-audit/
├── .github/
│   └── workflows/
│       └── ci.yml              # GitHub Actions CI/CD pipeline
├── cmd/
│   └── go-semver-audit/
│       └── main.go             # CLI entrypoint and flag parsing
├── internal/
│   ├── analyzer/               # Core analysis engine
│   │   ├── analyzer.go         # Main analyzer logic
│   │   ├── diff.go             # API diffing engine
│   │   ├── types.go            # Type definitions
│   │   ├── analyzer_test.go    # (future) analyzer tests
│   │   ├── diff_test.go        # Diff engine tests
│   │   └── types_test.go       # Type and parsing tests
│   └── report/                 # Output formatting
│       ├── text.go             # Human-readable text formatter
│       ├── json.go             # JSON formatter
│       ├── text_test.go        # Text formatter tests
│       └── json_test.go        # JSON formatter tests
├── testdata/                   # Test fixtures
│   ├── oldlib/                 # Sample library v1.0.0
│   │   └── lib.go
│   ├── newlib/                 # Sample library v2.0.0
│   │   └── lib.go
│   ├── userproject/            # Sample user project
│   │   ├── main.go
│   │   └── handler.go
│   └── README.md               # Test data documentation
├── bin/                        # Build output (gitignored)
├── .editorconfig               # Editor configuration
├── .gitignore                  # Git ignore rules
├── CHANGELOG.md                # Version history
├── CONTRIBUTING.md             # Contribution guidelines
├── go.mod                      # Go module definition
├── go.sum                      # Go module checksums
├── LICENSE                     # MIT license
├── Makefile                    # Build automation
├── PROJECT_STRUCTURE.md        # This file
├── QUICKSTART.md               # Quick start guide
└── README.md                   # Main documentation
```

## Key Components

### CLI (`cmd/go-semver-audit/main.go`)

- Parses command-line flags
- Orchestrates the analysis workflow
- Handles exit codes based on results

### Analyzer (`internal/analyzer/`)

**analyzer.go**
- Loads Go projects using `golang.org/x/tools/go/packages`
- Resolves current dependency versions
- Extracts exported API surfaces (functions, types, interfaces)
- Tracks symbol usage in user code
- Identifies unused dependencies

**diff.go**
- Compares old vs new API surfaces
- Detects removed symbols
- Identifies changed signatures
- Tracks interface modifications
- Filters results based on actual usage

**types.go**
- Defines core data structures (Upgrade, Result, API, Diff, etc.)
- Implements upgrade specification parsing
- Provides utility methods (HasBreakingChanges, HasWarnings)

### Report (`internal/report/`)

**text.go**
- Generates human-readable text output
- Supports verbose mode
- Formats locations with context
- Produces summary statistics

**json.go**
- Generates structured JSON output
- Suitable for CI/CD integration
- Provides complete analysis data

### Test Data (`testdata/`)

- Sample libraries demonstrating common breaking changes
- User project examples showing API usage
- Used for integration testing scenarios

## Code Organization

### Package Philosophy

- `cmd/` - Entry points, no business logic
- `internal/` - Implementation details, not importable externally
- `internal/analyzer/` - Analysis engine, pure logic
- `internal/report/` - Presentation layer, formatting only

### Dependencies

- **Standard Library**: Primary dependency
- **golang.org/x/tools/go/packages**: Go code analysis
- **golang.org/x/mod**: Module handling
- **golang.org/x/sync**: Concurrency primitives

### Testing Strategy

- Table-driven tests for all core logic
- Unit tests for parsing, diffing, formatting
- Test coverage for edge cases
- Integration test fixtures in `testdata/`

## Build Artifacts

- `bin/go-semver-audit` - Compiled binary (Linux/Mac)
- `bin/go-semver-audit.exe` - Compiled binary (Windows)
- `coverage.txt` - Test coverage report
- `coverage.html` - HTML coverage visualization

## Configuration Files

- `.gitignore` - Version control exclusions
- `.editorconfig` - Editor settings
- `go.mod` / `go.sum` - Go module management
- `Makefile` - Build automation scripts

## Documentation Files

- `README.md` - Main project documentation
- `QUICKSTART.md` - Getting started guide
- `CONTRIBUTING.md` - Development guidelines
- `CHANGELOG.md` - Version history
- `LICENSE` - MIT license text
- `PROJECT_STRUCTURE.md` - This file

## Future Considerations

Possible additions:
- `examples/` - More complete usage examples
- `docs/` - Extended documentation
- `scripts/` - Utility scripts
- `pkg/` - If we make parts public in the future

