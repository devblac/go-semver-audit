# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- GitHub Action (`uses: devblac/go-semver-audit@...`) that audits every direct dependency a pull request upgrades - such as a Dependabot or Renovate PR - and posts the result as a PR comment, updated in place on later pushes. Also writes a job summary, exposes `breaking`/`upgrades`/`report` outputs, and can fail the check on breaking changes. Analysis runs against the PR's base commit, where the project is known to build
- `-markdown` output: a GitHub-flavored Markdown report for pull request comments and job summaries, with a table of breaking changes and collapsible signature details
- `-list-upgrades <go.mod>`: prints `module@version` for each direct dependency whose version differs between the project's go.mod and the given one, e.g. the head of a Dependabot or Renovate PR. Indirect requirements and `/vN` module path changes are skipped

### Fixed
- Module versions are now fetched with `go mod download` and loaded from the module cache. Previously `packages.Load("module@version")` always failed silently, so every upgrade was reported as having no breaking changes
- Analysis fails loudly when a dependency package cannot be type-checked instead of diffing an empty API
- Packages removed in the new version are reported as removed symbols
- Version queries such as `@latest` are resolved to the concrete version analyzed
- Symbols are matched by package path and receiver type instead of bare name. Methods were recorded as `Type.Method` in the API but as `Method` in usage, so **no method signature change or removal was ever detected**; same-named symbols in different packages of a module also overwrote each other (`lib.New` vs `lib/keep.New`)
- Struct field usage is no longer recorded under the field's bare name, where it produced false matches against package-level symbols
- Calling a method now counts as using its receiver type, so `cfg := lib.New(); cfg.Validate()` is still caught when `Config` itself is removed
- Methods of a type that was removed entirely are no longer listed separately from the type, which counted one break several times over
- `_test.go` files are now part of the usage analysis. Dependencies used only from tests (testify is the most common Go dependency bump) were reported as "module not found in project dependencies", and breaking changes in them went undetected
- Interface methods are compared and reported as `Name(params) results` instead of `func (example.com/pkg.Iface).Name(...)`. Besides being noisy, the receiver in the old form is the *embedded* interface for promoted methods, so a version that inlined an embedded interface reported its methods as removed and added at once
- Reports are now deterministic. Symbols, interface methods, usage locations and unused dependencies were collected from maps and emitted in random order, so repeated runs over the same versions produced different output
- `-strict` no longer treats added symbols as warnings. Almost every upgrade adds something, so the flag failed on every upgrade and was unusable as a CI signal; it now fails on unused dependencies only
- Unused dependencies are reported as warnings even when the API diff is empty

### Changed
- Minimum Go version raised to 1.24 (was 1.21) to pick up `golang.org/x/tools` v0.39.0, required for the module-loading fix above and needed to type-check dependencies built with newer Go toolchains
- CI now tests against Go 1.24 and 1.27 instead of 1.21 and 1.22
- Reports now name symbols as `package.Symbol` (`slices.SortFunc`) and methods as `package.Type.Method` (`lib.Config.Validate`) instead of bare names, which were ambiguous once a module had more than one package. This changes the `name` field in JSON output, and the `type` field is now `method` for methods
- README is stored as UTF-8 instead of UTF-16, and documents the GitHub Action, `-markdown`, `-list-upgrades` and exit codes
- `-version` reports the real version instead of a hardcoded `0.1.0`: release binaries are stamped by GoReleaser, and `go install ...@vX.Y.Z` builds read it from their build info
- Removed QUICKSTART.md, INSTALL.md, PROJECT_STRUCTURE.md and CI_SETUP_SUMMARY.md, which duplicated the README or were out of date; CODE_OF_CONDUCT.md, previously empty, adopts the Contributor Covenant 2.1
- README claims corrected: no longer "production-ready", no batch mode, no claim that dependency code is never compiled; the example output is real, and Limitations lists known gaps and false positives
- CI installs staticcheck v0.8.1; v0.4.7 does not compile on Go 1.27
- Conflicting output flags (`-json`, `-html`, `-markdown`) are rejected before the analysis runs instead of after it
- Usage locations are reported relative to the project root with forward slashes (`internal/app/handler.go:42`) instead of as absolute paths, in every output format
- Exit codes now distinguish failure kinds: `0` no breaking changes, `1` breaking changes (or warnings with `-strict`), `2` the analysis could not be completed. Previously a tool failure and a detected breaking change both exited `1`, so CI could not tell them apart. Documented in `-help`

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

