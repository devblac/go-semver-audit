package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/devblac/go-semver-audit/internal/analyzer"
	"github.com/devblac/go-semver-audit/internal/report"
)

const version = "0.1.0"

// Exit codes. Breaking changes and tool failures are reported separately so
// CI can tell "the upgrade is risky" apart from "the audit did not run".
const (
	exitOK       = 0 // analysis completed, nothing to report
	exitBreaking = 1 // breaking changes found (or warnings, with -strict)
	exitError    = 2 // the analysis could not be completed
)

type config struct {
	projectPath string
	upgrade     string
	jsonOutput  bool
	htmlOutput  bool
	strict      bool
	unused      bool
	verbose     bool
	showVersion bool
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
	formatJSONFn           = report.FormatJSON
	formatHTMLFn           = report.FormatHTML
	formatTextFn           = report.FormatText
	exitFunc               = os.Exit
	stdoutWriter io.Writer = os.Stdout
	stderrWriter io.Writer = os.Stderr
)

func main() {
	cfg := parseFlags()

	if cfg.showVersion {
		fmt.Fprintf(stdoutWriter, "go-semver-audit version %s\n", version)
		exitFunc(exitOK)
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
		fmt.Fprintf(stderrWriter, "\nExit codes:\n")
		fmt.Fprintf(stderrWriter, "  %d  no breaking changes\n", exitOK)
		fmt.Fprintf(stderrWriter, "  %d  breaking changes detected (or warnings, with -strict)\n", exitBreaking)
		fmt.Fprintf(stderrWriter, "  %d  the analysis could not be completed\n", exitError)
	}

	flag.Parse()

	return cfg
}

func run(cfg config) error {
	// Parse the upgrade specification
	moduleUpgrade, err := parseUpgradeFn(cfg.upgrade)
	if err != nil {
		return fmt.Errorf("invalid upgrade specification: %w", err)
	}

	if cfg.verbose {
		fmt.Fprintf(stderrWriter, "Analyzing project at: %s\n", cfg.projectPath)
		fmt.Fprintf(stderrWriter, "Upgrade: %s %s -> %s\n",
			moduleUpgrade.Module, moduleUpgrade.OldVersion, moduleUpgrade.NewVersion)
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
	if cfg.jsonOutput && cfg.htmlOutput {
		return fmt.Errorf("cannot use -json and -html together")
	}

	switch {
	case cfg.jsonOutput:
		output, err = formatJSONFn(result)
	case cfg.htmlOutput:
		output, err = formatHTMLFn(result)
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
