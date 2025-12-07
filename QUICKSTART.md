# Quick Start Guide

Get started with `go-semver-audit` in 5 minutes.

## Installation

### Option 1: Install from Source (Recommended)

```bash
# Clone the repository
git clone https://github.com/devblac/go-semver-audit.git
cd go-semver-audit

# Build and install
make install

# Verify installation
go-semver-audit -version
```

### Option 2: Build Locally

```bash
# Build the binary
make build

# The binary will be in bin/go-semver-audit
./bin/go-semver-audit -version
```

## Basic Usage

### 1. Check a Single Dependency Upgrade

Navigate to your Go project and run:

```bash
go-semver-audit -upgrade github.com/pkg/errors@v0.9.1
```

This will:
- Analyze your current version vs v0.9.1
- Identify breaking changes
- Show which changes affect your code

### 2. View Detailed Output

Use verbose mode for more details:

```bash
go-semver-audit -upgrade github.com/gin-gonic/gin@v1.9.0 -v
```

### 3. Get JSON Output for CI

Perfect for automation and CI pipelines:

```bash
go-semver-audit -upgrade golang.org/x/sync@v0.5.0 -json > report.json
```

### 4. Strict Mode for CI

Exit with non-zero code even on warnings:

```bash
go-semver-audit -upgrade github.com/stretchr/testify@v1.8.0 -strict
```

### 5. Detect Unused Dependencies

After analyzing an upgrade, check for unused dependencies:

```bash
go-semver-audit -upgrade github.com/gorilla/mux@v1.8.0 -unused
```

## Understanding the Output

### Text Output Example

```
Analyzing upgrade: github.com/example/lib v1.2.0 -> v2.0.0

⚠️  BREAKING CHANGES DETECTED

Removed Functions:
  - OldHelper (used in: main.go:45, utils/helper.go:12)
  
Changed Signatures:
  - ParseConfig
    Old: func ParseConfig(path string) (*Config, error)
    New: func ParseConfig(path string, opts ...Option) (*Config, error)
    Used in: config/loader.go:23

Summary: 2 breaking changes affecting 3 locations in your code.
```

### What This Means

- **Removed Functions**: Functions that no longer exist and are used in your code
- **Changed Signatures**: Functions with different parameters or return types
- **Modified Interfaces**: Interfaces with added/removed methods
- **Used in**: File paths and line numbers where the symbol is used

### JSON Output Structure

```json
{
  "module": "github.com/example/lib",
  "old_version": "v1.2.0",
  "new_version": "v2.0.0",
  "breaking": true,
  "removed": [
    {
      "name": "OldHelper",
      "type": "function",
      "used_in": [
        {"file": "main.go", "line": 45}
      ]
    }
  ],
  "changed": [...],
  "interface_changes": [...]
}
```

## Common Workflows

### Before Upgrading a Dependency

```bash
# 1. Check what the upgrade would break
go-semver-audit -upgrade github.com/yourpackage@v2.0.0

# 2. Review the output and plan your changes

# 3. Actually upgrade
go get github.com/yourpackage@v2.0.0

# 4. Fix the breaking changes identified
```

### In CI/CD Pipeline

```yaml
# .github/workflows/dependency-check.yml
- name: Check dependency upgrade
  run: |
    go-semver-audit -upgrade ${{ env.PACKAGE }}@${{ env.VERSION }} -json -strict
  continue-on-error: true
```

### Batch Analysis

```bash
# Check multiple upgrades
for pkg in "github.com/pkg/errors@v0.9.1" "github.com/gin-gonic/gin@v1.9.0"; do
  echo "Checking $pkg"
  go-semver-audit -upgrade "$pkg"
  echo "---"
done
```

## Exit Codes

- `0` - No breaking changes (or only warnings in non-strict mode)
- `1` - Breaking changes detected, or warnings in strict mode

Use this in scripts:

```bash
if go-semver-audit -upgrade github.com/pkg/errors@v0.9.1; then
  echo "Safe to upgrade!"
else
  echo "Breaking changes detected!"
fi
```

## Tips and Tricks

### 1. Target Specific Project Directory

```bash
go-semver-audit -path /path/to/project -upgrade module@version
```

### 2. Combine with Go Commands

```bash
# See current version
go list -m github.com/pkg/errors

# Check upgrade
go-semver-audit -upgrade github.com/pkg/errors@v0.9.1

# If safe, upgrade
go get github.com/pkg/errors@v0.9.1
```

### 3. Save Reports

```bash
# Save text report
go-semver-audit -upgrade module@version > report.txt

# Save JSON report
go-semver-audit -upgrade module@version -json > report.json
```

## Troubleshooting

### "Module not found in dependencies"

Make sure the module is in your `go.mod`:

```bash
go list -m all | grep module-name
```

### "Failed to load packages"

Ensure your project compiles:

```bash
go build ./...
```

### Slow Analysis

For large projects, the first run may be slow as Go downloads module versions. Subsequent runs will be faster.

## Next Steps

- Read the full [README.md](README.md) for detailed documentation
- Check [CONTRIBUTING.md](CONTRIBUTING.md) to contribute
- Review [testdata/](testdata/) for usage examples
- Report issues on GitHub

## Help

```bash
go-semver-audit -help
```

For more information, visit the [GitHub repository](https://github.com/devblac/go-semver-audit).

