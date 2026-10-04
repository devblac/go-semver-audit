package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/devblac/go-semver-audit/internal/analyzer"
)

func TestDetermineExitCode(t *testing.T) {
	tests := []struct {
		name   string
		result *analyzer.Result
		strict bool
		want   int
	}{
		{
			name: "no changes",
			result: &analyzer.Result{
				Changes: &analyzer.Diff{},
			},
			strict: false,
			want:   0,
		},
		{
			name: "breaking changes",
			result: &analyzer.Result{
				Changes: &analyzer.Diff{
					Removed: []analyzer.RemovedSymbol{
						{Name: "OldFunc", Type: "function"},
					},
				},
			},
			strict: false,
			want:   1,
		},
		{
			name: "additions non-strict",
			result: &analyzer.Result{
				Changes: &analyzer.Diff{
					Added: []analyzer.AddedSymbol{
						{Name: "NewFunc", Type: "function"},
					},
				},
			},
			strict: false,
			want:   0,
		},
		{
			// Added symbols are informational: they must not trip -strict,
			// otherwise the flag fails on every upgrade
			name: "additions strict",
			result: &analyzer.Result{
				Changes: &analyzer.Diff{
					Added: []analyzer.AddedSymbol{
						{Name: "NewFunc", Type: "function"},
					},
				},
			},
			strict: true,
			want:   0,
		},
		{
			name: "unused dependencies non-strict",
			result: &analyzer.Result{
				Changes:    &analyzer.Diff{},
				UnusedDeps: []string{"github.com/unused/dep"},
			},
			strict: false,
			want:   0,
		},
		{
			name: "unused dependencies strict",
			result: &analyzer.Result{
				Changes:    &analyzer.Diff{},
				UnusedDeps: []string{"github.com/unused/dep"},
			},
			strict: true,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineExitCode(tt.result, tt.strict)
			if got != tt.want {
				t.Errorf("determineExitCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMain_ShowsVersionAndExits(t *testing.T) {
	restore := stubGlobals()
	defer restore()

	var exitCode int
	exitFunc = func(code int) { exitCode = code }

	stdout := &bytes.Buffer{}
	stdoutWriter = stdout
	stderrWriter = &bytes.Buffer{}

	os.Args = []string{"go-semver-audit", "-version"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	main()

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	if !strings.Contains(stdout.String(), "go-semver-audit version") {
		t.Fatalf("expected version output, got %q", stdout.String())
	}
}

func TestMain_MissingUpgradeExitsWithUsage(t *testing.T) {
	restore := stubGlobals()
	defer restore()

	var exitCode int
	exitFunc = func(code int) { exitCode = code }

	stdoutWriter = &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	stderrWriter = stderr

	os.Args = []string{"go-semver-audit"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	main()

	if exitCode != exitError {
		t.Fatalf("expected exit code %d, got %d", exitError, exitCode)
	}

	if !strings.Contains(stderr.String(), "-upgrade flag is required") {
		t.Fatalf("expected upgrade required message, got %q", stderr.String())
	}
}

func TestMain_AnalysisFailureExitsWithErrorCode(t *testing.T) {
	restore := stubGlobals()
	defer restore()

	var exitCode int
	exitFunc = func(code int) { exitCode = code }

	stdoutWriter = &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	stderrWriter = stderr

	parseUpgradeFn = func(spec string) (*analyzer.Upgrade, error) {
		return &analyzer.Upgrade{Module: "example.com/mod", NewVersion: "v1.0.0"}, nil
	}
	newAnalyzerFn = func(path string) (analyzerClient, error) {
		return &stubAnalyzer{analyzeErr: errors.New("module not found")}, nil
	}

	os.Args = []string{"go-semver-audit", "-upgrade", "example.com/mod@v1.0.0"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	main()

	// A failed analysis must be distinguishable from "breaking changes found"
	if exitCode != exitError {
		t.Fatalf("expected exit code %d, got %d", exitError, exitCode)
	}
	if !strings.Contains(stderr.String(), "module not found") {
		t.Fatalf("expected analysis error on stderr, got %q", stderr.String())
	}
}

func TestRun_GeneratesTextReportWithUnusedDeps(t *testing.T) {
	restore := stubGlobals()
	defer restore()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	stdoutWriter = stdout
	stderrWriter = stderr

	parseUpgradeFn = func(spec string) (*analyzer.Upgrade, error) {
		return &analyzer.Upgrade{
			Module:     "github.com/example/mod",
			OldVersion: "v1.0.0",
			NewVersion: "v1.1.0",
		}, nil
	}

	fakeAnalyzer := &stubAnalyzer{
		analyzeResult: &analyzer.Result{
			Module:     "github.com/example/mod",
			OldVersion: "v1.0.0",
			NewVersion: "v1.1.0",
			Changes:    &analyzer.Diff{},
		},
		unused: []string{"github.com/unused/dep"},
	}
	newAnalyzerFn = func(path string) (analyzerClient, error) {
		fakeAnalyzer.projectPath = path
		return fakeAnalyzer, nil
	}

	formatTextFn = func(res *analyzer.Result, verbose bool) (string, error) {
		return "text report\n", nil
	}

	cfg := config{
		projectPath: "testdata/userproject",
		upgrade:     "github.com/example/mod@v1.1.0",
		jsonOutput:  false,
		strict:      false,
		unused:      true,
		verbose:     true,
	}

	if err := run(cfg); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "text report") {
		t.Fatalf("expected text report, got %q", stdout.String())
	}
	if fakeAnalyzer.projectPath == "" {
		t.Fatalf("expected analyzer to receive project path")
	}
	if len(fakeAnalyzer.analyzeCalls) != 1 {
		t.Fatalf("expected one analyze call, got %d", len(fakeAnalyzer.analyzeCalls))
	}
}

func TestRun_JSONStrictExitsOnWarnings(t *testing.T) {
	restore := stubGlobals()
	defer restore()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	stdoutWriter = stdout
	stderrWriter = stderr

	parseUpgradeFn = func(spec string) (*analyzer.Upgrade, error) {
		return &analyzer.Upgrade{
			Module:     "github.com/example/mod",
			OldVersion: "v1.0.0",
			NewVersion: "v2.0.0",
		}, nil
	}

	fakeAnalyzer := &stubAnalyzer{
		analyzeResult: &analyzer.Result{
			Module:     "github.com/example/mod",
			Changes:    &analyzer.Diff{Added: []analyzer.AddedSymbol{{Name: "New", Type: "func"}}},
			UnusedDeps: []string{"github.com/unused/dep"},
		},
	}
	newAnalyzerFn = func(path string) (analyzerClient, error) {
		return fakeAnalyzer, nil
	}

	formatJSONFn = func(res *analyzer.Result) (string, error) {
		return `{"report":true}`, nil
	}

	var exitCode int
	exitFunc = func(code int) { exitCode = code }

	cfg := config{
		projectPath: "testdata/userproject",
		upgrade:     "github.com/example/mod@v2.0.0",
		jsonOutput:  true,
		strict:      true,
		unused:      false,
		verbose:     false,
	}

	if err := run(cfg); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if exitCode != exitBreaking {
		t.Fatalf("expected exit code %d, got %d", exitBreaking, exitCode)
	}
	if !strings.Contains(stdout.String(), `"report":true`) {
		t.Fatalf("expected JSON output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_HTMLReport(t *testing.T) {
	restore := stubGlobals()
	defer restore()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	stdoutWriter = stdout
	stderrWriter = stderr

	parseUpgradeFn = func(spec string) (*analyzer.Upgrade, error) {
		return &analyzer.Upgrade{
			Module:     "github.com/example/mod",
			OldVersion: "v1.0.0",
			NewVersion: "v1.1.0",
		}, nil
	}

	fakeAnalyzer := &stubAnalyzer{
		analyzeResult: &analyzer.Result{
			Module:  "github.com/example/mod",
			Changes: &analyzer.Diff{},
		},
	}
	newAnalyzerFn = func(path string) (analyzerClient, error) { return fakeAnalyzer, nil }
	formatHTMLFn = func(res *analyzer.Result) (string, error) { return "<html>ok</html>", nil }

	cfg := config{
		projectPath: "testdata/userproject",
		upgrade:     "github.com/example/mod@v1.1.0",
		htmlOutput:  true,
	}

	if err := run(cfg); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "<html>ok</html>") {
		t.Fatalf("expected HTML output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRun_JSONAndHTMLConflict(t *testing.T) {
	restore := stubGlobals()
	defer restore()

	parseUpgradeFn = func(spec string) (*analyzer.Upgrade, error) {
		return &analyzer.Upgrade{Module: "example.com/mod"}, nil
	}
	newAnalyzerFn = func(path string) (analyzerClient, error) {
		return &stubAnalyzer{analyzeResult: &analyzer.Result{Module: "example.com/mod", Changes: &analyzer.Diff{}}}, nil
	}

	cfg := config{
		projectPath: ".",
		upgrade:     "example.com/mod@v1.0.0",
		jsonOutput:  true,
		htmlOutput:  true,
	}

	if err := run(cfg); err == nil || !strings.Contains(err.Error(), "use only one of -json, -html or -markdown") {
		t.Fatalf("expected conflict error, got %v", err)
	}

	cfg.htmlOutput = false
	cfg.markdownOutput = true
	if err := run(cfg); err == nil || !strings.Contains(err.Error(), "use only one of") {
		t.Fatalf("expected conflict error for -json with -markdown, got %v", err)
	}
}

func TestRun_FormatConflictFailsBeforeAnalysis(t *testing.T) {
	restore := stubGlobals()
	defer restore()

	analyzed := false
	parseUpgradeFn = func(spec string) (*analyzer.Upgrade, error) {
		return &analyzer.Upgrade{Module: "example.com/mod"}, nil
	}
	newAnalyzerFn = func(path string) (analyzerClient, error) {
		analyzed = true
		return &stubAnalyzer{analyzeResult: &analyzer.Result{Changes: &analyzer.Diff{}}}, nil
	}

	err := run(config{upgrade: "example.com/mod@v1.0.0", htmlOutput: true, markdownOutput: true})
	if err == nil {
		t.Fatalf("expected conflict error")
	}
	if analyzed {
		t.Fatalf("flag validation should happen before the (slow) analysis")
	}
}

func TestRun_MarkdownReport(t *testing.T) {
	restore := stubGlobals()
	defer restore()

	stdout := &bytes.Buffer{}
	stdoutWriter = stdout
	stderrWriter = &bytes.Buffer{}

	parseUpgradeFn = func(spec string) (*analyzer.Upgrade, error) {
		return &analyzer.Upgrade{Module: "example.com/mod", NewVersion: "v1.1.0"}, nil
	}
	newAnalyzerFn = func(path string) (analyzerClient, error) {
		return &stubAnalyzer{analyzeResult: &analyzer.Result{Module: "example.com/mod", Changes: &analyzer.Diff{}}}, nil
	}
	formatMarkdownFn = func(res *analyzer.Result) (string, error) { return "### markdown\n", nil }

	if err := run(config{upgrade: "example.com/mod@v1.1.0", markdownOutput: true}); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "### markdown") {
		t.Fatalf("expected Markdown output, got %q", stdout.String())
	}
}

func TestMain_ListUpgrades(t *testing.T) {
	restore := stubGlobals()
	defer restore()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"),
		"module example.com/app\n\ngo 1.24.0\n\nrequire (\n\texample.com/a v1.0.0\n\texample.com/b v1.0.0\n)\n")
	headGoMod := filepath.Join(dir, "head.go.mod")
	writeFile(t, headGoMod,
		"module example.com/app\n\ngo 1.24.0\n\nrequire (\n\texample.com/a v1.2.0\n\texample.com/b v1.0.0\n)\n")

	exitCode := -1
	exitFunc = func(code int) { exitCode = code }
	stdout := &bytes.Buffer{}
	stdoutWriter = stdout
	stderrWriter = &bytes.Buffer{}

	os.Args = []string{"go-semver-audit", "-path", dir, "-list-upgrades", headGoMod}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	main()

	if exitCode != -1 {
		t.Fatalf("expected a normal return, got exit code %d", exitCode)
	}
	if got := stdout.String(); got != "example.com/a@v1.2.0\n" {
		t.Fatalf("-list-upgrades output = %q, want %q", got, "example.com/a@v1.2.0\n")
	}
}

func TestMain_ListUpgradesMissingGoMod(t *testing.T) {
	restore := stubGlobals()
	defer restore()

	exitCode := -1
	exitFunc = func(code int) { exitCode = code }
	stdoutWriter = &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	stderrWriter = stderr

	dir := t.TempDir()
	os.Args = []string{"go-semver-audit", "-path", dir, "-list-upgrades", filepath.Join(dir, "missing.mod")}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	main()

	if exitCode != exitError {
		t.Fatalf("expected exit code %d, got %d", exitError, exitCode)
	}
	if !strings.Contains(stderr.String(), "go.mod") {
		t.Fatalf("expected an error about go.mod, got %q", stderr.String())
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRun_ParseUpgradeError(t *testing.T) {
	restore := stubGlobals()
	defer restore()

	parseUpgradeFn = func(spec string) (*analyzer.Upgrade, error) {
		return nil, errors.New("bad spec")
	}

	err := run(config{upgrade: "not-valid"})
	if err == nil || !strings.Contains(err.Error(), "invalid upgrade specification") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestRun_LogsWarningOnUnusedDepsErrorVerbose(t *testing.T) {
	restore := stubGlobals()
	defer restore()

	stderr := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	stderrWriter = stderr
	stdoutWriter = stdout

	parseUpgradeFn = func(spec string) (*analyzer.Upgrade, error) {
		return &analyzer.Upgrade{Module: "example.com/mod", NewVersion: "v1.2.0"}, nil
	}

	fakeAnalyzer := &stubAnalyzer{
		analyzeResult: &analyzer.Result{
			Module:  "example.com/mod",
			Changes: &analyzer.Diff{},
		},
		unusedErr: errors.New("boom"),
	}
	newAnalyzerFn = func(path string) (analyzerClient, error) { return fakeAnalyzer, nil }
	formatTextFn = func(res *analyzer.Result, verbose bool) (string, error) { return "ok\n", nil }

	cfg := config{
		projectPath: ".",
		upgrade:     "example.com/mod@v1.2.0",
		unused:      true,
		verbose:     true,
	}

	if err := run(cfg); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !strings.Contains(stderr.String(), "failed to detect unused dependencies") {
		t.Fatalf("expected warning, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Fatalf("expected report output, got %q", stdout.String())
	}
}

func TestParseFlags(t *testing.T) {
	// Save original command line args
	oldArgs := flag.CommandLine
	defer func() { flag.CommandLine = oldArgs }()

	// Reset flag.CommandLine for testing
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

	// Test default values
	cfg := parseFlags()

	if cfg.projectPath != "." {
		t.Errorf("Expected default projectPath '.', got %q", cfg.projectPath)
	}
	if cfg.jsonOutput {
		t.Errorf("Expected jsonOutput false, got true")
	}
	if cfg.strict {
		t.Errorf("Expected strict false, got true")
	}
	if cfg.unused {
		t.Errorf("Expected unused false, got true")
	}
	if cfg.verbose {
		t.Errorf("Expected verbose false, got true")
	}
	if cfg.htmlOutput {
		t.Errorf("Expected htmlOutput false, got true")
	}
}

func TestConfigStruct(t *testing.T) {
	// Test that config struct can be created and fields accessed
	cfg := config{
		projectPath: "/test/path",
		upgrade:     "github.com/example/module@v1.0.0",
		jsonOutput:  true,
		htmlOutput:  true,
		strict:      true,
		unused:      true,
		verbose:     true,
		showVersion: false,
	}

	if cfg.projectPath != "/test/path" {
		t.Errorf("Expected projectPath '/test/path', got %q", cfg.projectPath)
	}
	if cfg.upgrade != "github.com/example/module@v1.0.0" {
		t.Errorf("Expected upgrade 'github.com/example/module@v1.0.0', got %q", cfg.upgrade)
	}
	if !cfg.jsonOutput {
		t.Errorf("Expected jsonOutput true, got false")
	}
	if !cfg.htmlOutput {
		t.Errorf("Expected htmlOutput true, got false")
	}
	if !cfg.strict {
		t.Errorf("Expected strict true, got false")
	}
	if !cfg.unused {
		t.Errorf("Expected unused true, got false")
	}
	if !cfg.verbose {
		t.Errorf("Expected verbose true, got false")
	}
	if cfg.showVersion {
		t.Errorf("Expected showVersion false, got true")
	}
}

type stubAnalyzer struct {
	analyzeResult *analyzer.Result
	analyzeErr    error
	analyzeCalls  []*analyzer.Upgrade
	unused        []string
	unusedErr     error
	projectPath   string
}

func (s *stubAnalyzer) Analyze(upgrade *analyzer.Upgrade) (*analyzer.Result, error) {
	s.analyzeCalls = append(s.analyzeCalls, upgrade)
	return s.analyzeResult, s.analyzeErr
}

func (s *stubAnalyzer) FindUnusedDependencies() ([]string, error) {
	return s.unused, s.unusedErr
}

func stubGlobals() func() {
	oldParseUpgrade := parseUpgradeFn
	oldNewAnalyzer := newAnalyzerFn
	oldFormatJSON := formatJSONFn
	oldFormatHTML := formatHTMLFn
	oldFormatMarkdown := formatMarkdownFn
	oldFormatText := formatTextFn
	oldExit := exitFunc
	oldStdout := stdoutWriter
	oldStderr := stderrWriter
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine

	return func() {
		parseUpgradeFn = oldParseUpgrade
		newAnalyzerFn = oldNewAnalyzer
		formatJSONFn = oldFormatJSON
		formatHTMLFn = oldFormatHTML
		formatMarkdownFn = oldFormatMarkdown
		formatTextFn = oldFormatText
		exitFunc = oldExit
		stdoutWriter = oldStdout
		stderrWriter = oldStderr
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}
}

func TestCurrentVersion(t *testing.T) {
	origVersion, origReadBuildInfo := version, readBuildInfo
	defer func() { version, readBuildInfo = origVersion, origReadBuildInfo }()

	buildInfo := func(mainVersion string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: mainVersion}}, true
		}
	}

	tests := []struct {
		name      string
		stamped   string
		buildInfo func() (*debug.BuildInfo, bool)
		want      string
	}{
		{"release binary stamped by GoReleaser", "v0.2.0", buildInfo("(devel)"), "v0.2.0"},
		{"go install module@version", "", buildInfo("v0.2.0"), "v0.2.0"},
		{"build without version info", "", buildInfo("(devel)"), "dev"},
		{"no build info", "", func() (*debug.BuildInfo, bool) { return nil, false }, "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, readBuildInfo = tt.stamped, tt.buildInfo
			if got := currentVersion(); got != tt.want {
				t.Errorf("currentVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
