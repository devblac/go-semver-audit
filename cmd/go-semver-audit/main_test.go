package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
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
			name: "warnings non-strict",
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
			name: "warnings strict",
			result: &analyzer.Result{
				Changes: &analyzer.Diff{
					Added: []analyzer.AddedSymbol{
						{Name: "NewFunc", Type: "function"},
					},
				},
			},
			strict: true,
			want:   1,
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

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}

	if !strings.Contains(stderr.String(), "-upgrade flag is required") {
		t.Fatalf("expected upgrade required message, got %q", stderr.String())
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
			Module:  "github.com/example/mod",
			Changes: &analyzer.Diff{Added: []analyzer.AddedSymbol{{Name: "New", Type: "func"}}},
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

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
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

	if err := run(cfg); err == nil || !strings.Contains(err.Error(), "cannot use -json and -html together") {
		t.Fatalf("expected conflict error, got %v", err)
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
		formatTextFn = oldFormatText
		exitFunc = oldExit
		stdoutWriter = oldStdout
		stderrWriter = oldStderr
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}
}
