# Installation Guide

Complete installation instructions for `go-semver-audit`.

## Prerequisites

- **Go 1.21 or later** - [Download Go](https://go.dev/dl/)
- **Git** - For cloning the repository
- **Make** (optional) - For convenient build commands

Verify your Go installation:

```bash
go version
```

## Installation Methods

### Method 1: Install from Source (Recommended)

This method compiles and installs the binary to your `$GOPATH/bin` (or `$GOBIN`).

```bash
# Clone the repository
git clone https://github.com/yourusername/go-semver-audit.git
cd go-semver-audit

# Install the binary
go install ./cmd/go-semver-audit

# Or using make
make install
```

The binary will be installed to your Go bin directory. Ensure this is in your PATH:

**Linux/macOS:**
```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

**Windows (PowerShell):**
```powershell
$env:PATH += ";$(go env GOPATH)\bin"
```

Verify installation:
```bash
go-semver-audit -version
```

### Method 2: Build Locally

Build the binary without installing it system-wide:

```bash
# Clone the repository
git clone https://github.com/yourusername/go-semver-audit.git
cd go-semver-audit

# Build using make
make build

# Or build directly with go
go build -o bin/go-semver-audit ./cmd/go-semver-audit
```

The binary will be in the `bin/` directory:

**Linux/macOS:**
```bash
./bin/go-semver-audit -version
```

**Windows:**
```powershell
.\bin\go-semver-audit.exe -version
```

### Method 3: Go Install from GitHub

Once published, you can install directly from GitHub:

```bash
go install github.com/yourusername/go-semver-audit/cmd/go-semver-audit@latest
```

## Platform-Specific Instructions

### Linux

```bash
# Install Go if not already installed
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Add Go bin to PATH permanently
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc

# Install go-semver-audit
go install github.com/yourusername/go-semver-audit/cmd/go-semver-audit@latest
```

### macOS

```bash
# Install Go using Homebrew
brew install go

# Add Go bin to PATH (if not already)
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.zshrc
source ~/.zshrc

# Install go-semver-audit
go install github.com/yourusername/go-semver-audit/cmd/go-semver-audit@latest
```

### Windows

1. Download and install Go from [go.dev/dl](https://go.dev/dl/)
2. Open PowerShell as Administrator
3. Install go-semver-audit:

```powershell
go install github.com/yourusername/go-semver-audit/cmd/go-semver-audit@latest
```

4. Add Go bin to PATH if not automatic:

```powershell
$goPath = go env GOPATH
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";$goPath\bin", "User")
```

## Verification

After installation, verify the tool works:

```bash
# Check version
go-semver-audit -version

# View help
go-semver-audit -help

# Test on a project (navigate to any Go project)
cd /path/to/your/go/project
go-semver-audit -upgrade github.com/pkg/errors@v0.9.1
```

## Building from Source (Development)

For development or contributing:

```bash
# Clone and enter directory
git clone https://github.com/yourusername/go-semver-audit.git
cd go-semver-audit

# Download dependencies
go mod download

# Build
make build

# Run tests
make test

# Run with coverage
make test-coverage

# Format code
make fmt

# Run linter
make lint

# All checks (format + lint + test)
make check
```

## Docker (Optional)

If you prefer to use Docker:

```dockerfile
# Create a Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o go-semver-audit ./cmd/go-semver-audit

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/go-semver-audit /usr/local/bin/
ENTRYPOINT ["go-semver-audit"]
```

Build and run:

```bash
docker build -t go-semver-audit .
docker run --rm -v $(pwd):/work -w /work go-semver-audit -upgrade module@version
```

## Troubleshooting

### "Command not found"

- Ensure `$GOPATH/bin` is in your PATH
- Verify installation directory: `which go-semver-audit` (Linux/macOS) or `where go-semver-audit` (Windows)
- Check Go environment: `go env GOPATH`

### "Permission denied"

**Linux/macOS:**
```bash
chmod +x $(which go-semver-audit)
```

### Build Errors

- Ensure Go version is 1.21 or later: `go version`
- Clean and rebuild: `go clean -cache && make build`
- Check for network issues downloading dependencies

### Module Errors

```bash
# Clear module cache
go clean -modcache

# Re-download dependencies
go mod download
```

## Uninstallation

### Remove Installed Binary

```bash
# Find the binary
which go-semver-audit  # Linux/macOS
where go-semver-audit  # Windows

# Remove it
rm $(which go-semver-audit)  # Linux/macOS
del $(where go-semver-audit)  # Windows
```

### Remove Source

```bash
cd ..
rm -rf go-semver-audit
```

## Updating

### If Installed via go install

```bash
go install github.com/yourusername/go-semver-audit/cmd/go-semver-audit@latest
```

### If Built from Source

```bash
cd go-semver-audit
git pull origin main
make install
```

## Next Steps

- Read the [Quick Start Guide](QUICKSTART.md)
- Check out the [README](README.md) for usage examples
- Review [CONTRIBUTING.md](CONTRIBUTING.md) to contribute

## Support

If you encounter issues:

1. Check the [Troubleshooting](#troubleshooting) section above
2. Search existing [GitHub Issues](https://github.com/yourusername/go-semver-audit/issues)
3. Open a new issue with:
   - Go version (`go version`)
   - OS and version
   - Installation method used
   - Error messages
   - Steps to reproduce

