# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial implementation of go-semver-audit CLI tool
- API surface analysis for Go module dependencies
- Support for detecting removed functions, changed signatures, and interface modifications
- Usage-aware analysis (only reports breaking changes for APIs you use)
- Text and JSON output formats
- Verbose mode for detailed output
- Strict mode for CI/CD integration
- Optional unused dependency detection
- Comprehensive test suite with table-driven tests
- Example test data demonstrating common upgrade scenarios

### Documentation
- Comprehensive README with usage examples
- Quick start guide
- Contributing guidelines
- MIT license
- CI/CD workflow configuration

## [0.1.0] - 2025-12-06

### Added
- Initial release
- Core functionality for analyzing Go dependency upgrades
- Static analysis of exported API surfaces
- Breaking change detection
- Usage tracking in user code
- Multiple output formats (text, JSON)
- CLI with intuitive flags

---

[Unreleased]: https://github.com/devblac/go-semver-audit/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/devblac/go-semver-audit/releases/tag/v0.1.0

