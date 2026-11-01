package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/devblac/go-semver-audit/internal/analyzer"
	"github.com/devblac/go-semver-audit/internal/gomod"
	"github.com/devblac/go-semver-audit/internal/report"
)

// version is stamped into release binaries by GoReleaser
// (-ldflags "-X main.version=..."). It stays empty in other builds.
var version = ""

// currentVersion reports the version of this binary. Without a stamped
// version it falls back to the build info: `go install module@vX.Y.Z` records
// vX.Y.Z, and since Go 1.24 a build inside a git checkout records a version
// derived from the tags (e.g. v0.1.2-0.20261004...+dirty). Only builds with
// no version information at all report "dev".
func currentVersion() string {
	if version != "" {
		return version
	}
	if info, ok := readBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// Exit codes. Breaking changes and tool failures are reported separately so
// CI can tell "the upgrade is risky" apart from "the audit did not run".
const (
	exitOK       = 0 // analysis completed, nothing to report
	exitBreaking = 1 // breaking changes found (or warnings, with -strict)
	exitError    = 2 // the analysis could not be completed
)

type config struct {
	projectPath    string
	upgrade        string
	listUpgrades   string
	jsonOutput     bool
	htmlOutput     bool
	markdownOutput bool
	strict         bool
	unused         bool
	verbose        bool
	showVersion    bool
}

// Allow dependency injection for testing.
type analyzerClient interface {
	Analyze(*analyzer.Upgrade) (*analyzer.Result, error)
	FindUnusedDependencies() ([]string, error)
}

var (
	parseUpgradeFn = analyzer.ParseUpgrade
	newAnalyzerFn  = func(projectPath string) (analyzerClient, error) {
		return analyzer.New(projectPath)
	}
	formatJSONFn               = report.FormatJSON
	formatHTMLFn               = report.FormatHTML
	formatMarkdownFn           = report.FormatMarkdown
	formatTextFn               = report.FormatText
	exitFunc                   = os.Exit
	readBuildInfo              = debug.ReadBuildInfo
	stdoutWriter     io.Writer = os.Stdout
	stderrWriter     io.Writer = os.Stderr
)

func main() {
	cfg := parseFlags()

	if cfg.showVersion {
		fmt.Fprintf(stdoutWriter, "go-semver-audit version %s\n", currentVersion())
		exitFunc(exitOK)
		return
	}

	if cfg.listUpgrades != "" {
		if err := listUpgrades(cfg); err != nil {
			fmt.Fprintf(stderrWriter, "Error: %v\n", err)
			exitFunc(exitError)
		}
		return
	}

	if cfg.upgrade == "" {
		fmt.Fprintln(stderrWriter, "Error: -upgrade flag is required")
		fmt.Fprintln(stderrWriter, "Usage: go-semver-audit -upgrade module@version [options]")
		flag.Usage()
		exitFunc(exitError)
		return
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(stderrWriter, "Error: %v\n", err)
		exitFunc(exitError)
		return
	}
}

func parseFlags() config {
	cfg := config{}

	flag.StringVar(&cfg.projectPath, "path", ".", "Path to Go project to analyze")
	flag.StringVar(&cfg.upgrade, "upgrade", "", "Dependency upgrade in format module@version (required)")
	flag.BoolVar(&cfg.jsonOutput, "json", false, "Output results as JSON")
	flag.BoolVar(&cfg.htmlOutput, "html", false, "Output results as HTML")
	flag.BoolVar(&cfg.markdownOutput, "markdown", false, "Output results as GitHub-flavored Markdown (for PR comments)")
	flag.StringVar(&cfg.listUpgrades, "list-upgrades", "",
		"Print module@version for each direct dependency whose version differs between\nthe project's go.mod and the given go.mod, then exit")
	flag.BoolVar(&cfg.strict, "strict", false, "Exit non-zero on warnings (not just errors)")
	flag.BoolVar(&cfg.unused, "unused", false, "Report unused dependencies after upgrade")
	flag.BoolVar(&cfg.verbose, "v", false, "Verbose output")
	flag.BoolVar(&cfg.showVersion, "version", false, "Show version information")

	flag.Usage = func() {
		fmt.Fprintf(stderrWriter, "Usage: go-semver-audit [options]\n\n")
		fmt.Fprintf(stderrWriter, "Analyze breaking changes in Go dependency upgrades.\n\n")
		fmt.Fprintf(stderrWriter, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(stderrWriter, "\nExample:\n")
		fmt.Fprintf(stderrWriter, "  go-semver-audit -upgrade github.com/pkg/errors@v0.9.1\n")
		fmt.Fprintf(stderrWriter, "  go-semver-audit -path ./myproject -upgrade github.com/gin-gonic/gin@v1.9.0 -json\n")
		fmt.Fprintf(stderrWriter, "  go-semver-audit -list-upgrades /tmp/pr-head/go.mod\n")
		fmt.Fprintf(stderrWriter, "\nExit codes:\n")
		fmt.Fprintf(stderrWriter, "  %d  no breaking changes\n", exitOK)
		fmt.Fprintf(stderrWriter, "  %d  breaking changes detected (or warnings, with -strict)\n", exitBreaking)
		fmt.Fprintf(stderrWriter, "  %d  the analysis could not be completed\n", exitError)
	}

	flag.Parse()

	return cfg
}

// listUpgrades prints module@version for every direct dependency whose version
// differs between the project's go.mod and the go.mod given to -list-upgrades
func listUpgrades(cfg config) error {
	oldGoMod, err := os.ReadFile(filepath.Join(cfg.projectPath, "go.mod"))
	if err != nil {
		return fmt.Errorf("failed to read project go.mod: %w", err)
	}
	newGoMod, err := os.ReadFile(cfg.listUpgrades)
	if err != nil {
		return fmt.Errorf("failed to read go.mod to compare against: %w", err)
	}

	upgrades, err := gomod.DirectUpgrades(oldGoMod, newGoMod)
	if err != nil {
		return err
	}
	for _, u := range upgrades {
		fmt.Fprintf(stdoutWriter, "%s@%s\n", u.Module, u.NewVersion)
	}
	return nil
}

func run(cfg config) error {
	// Validate output flags before spending time on the analysis
	formats := 0
	for _, enabled := range []bool{cfg.jsonOutput, cfg.htmlOutput, cfg.markdownOutput} {
		if enabled {
			formats++
		}
	}
	if formats > 1 {
		return fmt.Errorf("use only one of -json, -html or -markdown")
	}

	// Parse the upgrade specification
	moduleUpgrade, err := parseUpgradeFn(cfg.upgrade)
	if err != nil {
		return fmt.Errorf("invalid upgrade specification: %w", err)
	}

	if cfg.verbose {
		fmt.Fprintf(stderrWriter, "Analyzing project at: %s\n", cfg.projectPath)
		// The current version is only known once the project is loaded
		fmt.Fprintf(stderrWriter, "Upgrade: %s -> %s\n", moduleUpgrade.Module, moduleUpgrade.NewVersion)
	}

	// Create analyzer
	a, err := newAnalyzerFn(cfg.projectPath)
	if err != nil {
		return fmt.Errorf("failed to initialize analyzer: %w", err)
	}

	// Perform analysis
	result, err := a.Analyze(moduleUpgrade)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Check for unused dependencies if requested
	if cfg.unused {
		unused, err := a.FindUnusedDependencies()
		if err != nil && cfg.verbose {
			fmt.Fprintf(stderrWriter, "Warning: failed to detect unused dependencies: %v\n", err)
		} else {
			result.UnusedDeps = unused
		}
	}

	// Generate report
	var output string
	switch {
	case cfg.jsonOutput:
		output, err = formatJSONFn(result)
	case cfg.htmlOutput:
		output, err = formatHTMLFn(result)
	case cfg.markdownOutput:
		output, err = formatMarkdownFn(result)
	default:
		output, err = formatTextFn(result, cfg.verbose)
	}
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	fmt.Fprint(stdoutWriter, output)

	// Determine exit code
	exitCode := determineExitCode(result, cfg.strict)
	if exitCode != 0 {
		exitFunc(exitCode)
		return nil
	}

	return nil
}

func determineExitCode(result *analyzer.Result, strict bool) int {
	// Exit non-zero if there are breaking changes
	if result.HasBreakingChanges() {
		return exitBreaking
	}

	// In strict mode, exit non-zero if there are any warnings
	if strict && result.HasWarnings() {
		return exitBreaking
	}

	return exitOK
}
