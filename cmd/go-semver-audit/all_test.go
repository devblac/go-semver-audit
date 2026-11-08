package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/devblac/go-semver-audit/internal/analyzer"
	"github.com/devblac/go-semver-audit/internal/gomod"
	"github.com/devblac/go-semver-audit/internal/report"
)

// stubAllMode wires runAll to a fixed set of available updates and per-module
// analysis outcomes, and captures stdout, stderr and the exit code.
func stubAllMode(t *testing.T, upgrades []gomod.Upgrade, lookupErrors []gomod.UpdateError, outcomes map[string]stubOutcome) (*stubAnalyzer, *bytes.Buffer, *int) {
	t.Helper()
	restore := stubGlobals()
	t.Cleanup(restore)

	exitCode := exitOK
	exitFunc = func(code int) { exitCode = code }
	stdout := &bytes.Buffer{}
	stdoutWriter = stdout
	stderrWriter = &bytes.Buffer{}

	availableUpdatesFn = func(string) ([]gomod.Upgrade, []gomod.UpdateError, error) {
		return upgrades, lookupErrors, nil
	}
	stub := &stubAnalyzer{byModule: outcomes}
	newAnalyzerFn = func(path string) (analyzerClient, error) {
		stub.projectPath = path
		return stub, nil
	}
	return stub, stdout, &exitCode
}

func cleanResult(module, oldVersion, newVersion string) *analyzer.Result {
	return &analyzer.Result{Module: module, OldVersion: oldVersion, NewVersion: newVersion, Changes: &analyzer.Diff{}}
}

func breakingResult(module, oldVersion, newVersion string) *analyzer.Result {
	return &analyzer.Result{Module: module, OldVersion: oldVersion, NewVersion: newVersion, Changes: &analyzer.Diff{
		Removed: []analyzer.RemovedSymbol{{Name: "lib.Gone", Type: "function", UsedIn: []analyzer.Location{{File: "main.go", Line: 3}}}},
	}}
}

func TestRunAll_AnalyzesEveryUpdateAndKeepsGoingAfterFailures(t *testing.T) {
	upgrades := []gomod.Upgrade{
		{Module: "example.com/broken", OldVersion: "v1.0.0", NewVersion: "v1.1.0"},
		{Module: "example.com/clean", OldVersion: "v0.2.0", NewVersion: "v0.3.0"},
		{Module: "example.com/fails", OldVersion: "v1.0.0", NewVersion: "v1.0.1"},
		{Module: "example.com/tagged", OldVersion: "v1.0.0", NewVersion: "v1.2.0"},
	}
	lookupErrors := []gomod.UpdateError{{Module: "example.com/offline", Version: "v1.0.0", Err: "proxy unreachable"}}
	stub, stdout, exitCode := stubAllMode(t, upgrades, lookupErrors, map[string]stubOutcome{
		"example.com/broken": {result: breakingResult("example.com/broken", "v1.0.0", "v1.1.0")},
		"example.com/clean":  {result: cleanResult("example.com/clean", "v0.2.0", "v0.3.0")},
		"example.com/fails":  {err: errors.New("failed to load new API: boom")},
		"example.com/tagged": {err: fmt.Errorf("failed to determine current version: module example.com/tagged %w", analyzer.ErrModuleNotImported)},
	})

	if err := runAll(config{projectPath: "proj", all: true}); err != nil {
		t.Fatalf("runAll() error = %v", err)
	}

	// The project is loaded once, up front, and every module is analyzed even
	// though one of them fails
	if stub.loadCalls != 1 {
		t.Errorf("LoadProject called %d times, want 1", stub.loadCalls)
	}
	if len(stub.analyzeCalls) != len(upgrades) {
		t.Fatalf("Analyze called %d times, want %d", len(stub.analyzeCalls), len(upgrades))
	}
	if call := stub.analyzeCalls[0]; call.Module != "example.com/broken" || call.NewVersion != "v1.1.0" {
		t.Errorf("first Analyze call = %+v", call)
	}
	if stub.projectPath != "proj" {
		t.Errorf("analyzer project path = %q, want proj", stub.projectPath)
	}

	// Breaking changes win over failures
	if *exitCode != exitBreaking {
		t.Errorf("exit code = %d, want %d", *exitCode, exitBreaking)
	}

	out := stdout.String()
	for _, want := range []string{
		"5 dependency updates audited: 1 with breaking changes, 1 without, 2 could not be analyzed, 1 not imported by the project.",
		"Analyzing upgrade: example.com/broken v1.0.0 -> v1.1.0",
		"Analyzing upgrade: example.com/clean v0.2.0 -> v0.3.0",
		"example.com/fails v1.0.0 -> v1.0.1\n    failed to load new API: boom",
		"example.com/offline v1.0.0\n    failed to look up available updates: proxy unreachable",
		"Not imported by any package of the project (nothing to check):\n  - example.com/tagged v1.0.0 -> v1.2.0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text report is missing %q:\n%s", want, out)
		}
	}
	// Failures are listed by module, wherever they came from
	if strings.Index(out, "example.com/fails") > strings.Index(out, "example.com/offline") {
		t.Errorf("failures are not sorted by module:\n%s", out)
	}
}

func TestRunAll_ExitCodes(t *testing.T) {
	upgrade := []gomod.Upgrade{{Module: "example.com/a", OldVersion: "v1.0.0", NewVersion: "v1.1.0"}}
	tests := []struct {
		name         string
		upgrades     []gomod.Upgrade
		lookupErrors []gomod.UpdateError
		outcome      stubOutcome
		want         int
	}{
		{"clean", upgrade, nil, stubOutcome{result: cleanResult("example.com/a", "v1.0.0", "v1.1.0")}, exitOK},
		{"breaking", upgrade, nil, stubOutcome{result: breakingResult("example.com/a", "v1.0.0", "v1.1.0")}, exitBreaking},
		{"analysis failed", upgrade, nil, stubOutcome{err: errors.New("boom")}, exitError},
		{"lookup failed", nil, []gomod.UpdateError{{Module: "example.com/b", Version: "v1.0.0", Err: "boom"}}, stubOutcome{}, exitError},
		{"not imported", upgrade, nil, stubOutcome{err: analyzer.ErrModuleNotImported}, exitOK},
		{"nothing to update", nil, nil, stubOutcome{}, exitOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, exitCode := stubAllMode(t, tt.upgrades, tt.lookupErrors, map[string]stubOutcome{"example.com/a": tt.outcome})
			if err := runAll(config{projectPath: ".", all: true}); err != nil {
				t.Fatalf("runAll() error = %v", err)
			}
			if *exitCode != tt.want {
				t.Errorf("exit code = %d, want %d", *exitCode, tt.want)
			}
		})
	}
}

func TestRunAll_NoUpdatesSkipsTheAnalysis(t *testing.T) {
	stub, stdout, _ := stubAllMode(t, nil, nil, nil)
	if err := runAll(config{projectPath: ".", all: true}); err != nil {
		t.Fatalf("runAll() error = %v", err)
	}
	if stub.loadCalls != 0 {
		t.Errorf("the project was loaded although there was nothing to audit")
	}
	if got := stdout.String(); got != "No direct dependency has a newer version available.\n\n" {
		t.Errorf("output = %q", got)
	}
}

func TestRunAll_ProjectLoadFailureFailsTheRun(t *testing.T) {
	stub, stdout, _ := stubAllMode(t, []gomod.Upgrade{{Module: "example.com/a", OldVersion: "v1.0.0", NewVersion: "v1.1.0"}}, nil, nil)
	stub.loadErr = errors.New("failed to load project: packages contain errors")

	err := runAll(config{projectPath: ".", all: true})
	if err == nil || !strings.Contains(err.Error(), "packages contain errors") {
		t.Fatalf("runAll() error = %v, want the project load error", err)
	}
	if len(stub.analyzeCalls) != 0 || stdout.Len() != 0 {
		t.Errorf("expected no analysis and no report after the project failed to load")
	}
}

func TestRunAll_ListFailure(t *testing.T) {
	stubAllMode(t, nil, nil, nil)
	availableUpdatesFn = func(string) ([]gomod.Upgrade, []gomod.UpdateError, error) {
		return nil, nil, errors.New("go list -m -u all: exit status 1")
	}
	err := runAll(config{projectPath: ".", all: true})
	if err == nil || !strings.Contains(err.Error(), "failed to list available updates") {
		t.Fatalf("runAll() error = %v", err)
	}
}

func TestRunAll_JSON(t *testing.T) {
	_, stdout, _ := stubAllMode(t,
		[]gomod.Upgrade{
			{Module: "example.com/a", OldVersion: "v1.0.0", NewVersion: "v1.1.0"},
			{Module: "example.com/b", OldVersion: "v1.0.0", NewVersion: "v1.0.1"},
		}, nil,
		map[string]stubOutcome{
			"example.com/a": {result: breakingResult("example.com/a", "v1.0.0", "v1.1.0")},
			"example.com/b": {err: errors.New("boom")},
		})

	if err := runAll(config{projectPath: ".", all: true, jsonOutput: true}); err != nil {
		t.Fatalf("runAll() error = %v", err)
	}

	var got report.JSONAuditReport
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if !got.Breaking || len(got.Results) != 1 || got.Results[0].Module != "example.com/a" || !got.Results[0].Breaking {
		t.Errorf("unexpected results: %+v", got)
	}
	if len(got.Errors) != 1 || got.Errors[0].Module != "example.com/b" || got.Errors[0].Error != "boom" {
		t.Errorf("unexpected errors: %+v", got.Errors)
	}
}

func TestRunAll_FormatConflict(t *testing.T) {
	stubAllMode(t, nil, nil, nil)
	listed := false
	availableUpdatesFn = func(string) ([]gomod.Upgrade, []gomod.UpdateError, error) {
		listed = true
		return nil, nil, nil
	}
	err := runAll(config{all: true, jsonOutput: true, markdownOutput: true})
	if err == nil || !strings.Contains(err.Error(), "use only one of") {
		t.Fatalf("expected a format conflict error, got %v", err)
	}
	if listed {
		t.Errorf("updates were looked up before the flags were validated")
	}
}

func TestMain_AllIsExclusive(t *testing.T) {
	for _, args := range [][]string{
		{"-all", "-upgrade", "example.com/a@v1.1.0"},
		{"-all", "-list-upgrades", "head.go.mod"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, stdout, exitCode := stubAllMode(t, nil, nil, nil)
			stderr := &bytes.Buffer{}
			stderrWriter = stderr
			listed := false
			availableUpdatesFn = func(string) ([]gomod.Upgrade, []gomod.UpdateError, error) {
				listed = true
				return nil, nil, nil
			}

			os.Args = append([]string{"go-semver-audit"}, args...)
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)

			main()

			if *exitCode != exitError {
				t.Errorf("exit code = %d, want %d", *exitCode, exitError)
			}
			if !strings.Contains(stderr.String(), "-all cannot be combined") {
				t.Errorf("stderr = %q", stderr.String())
			}
			if listed || stdout.Len() != 0 {
				t.Errorf("the audit ran despite conflicting flags")
			}
		})
	}
}

func TestMain_AllRunsTheAudit(t *testing.T) {
	_, stdout, exitCode := stubAllMode(t,
		[]gomod.Upgrade{{Module: "example.com/a", OldVersion: "v1.0.0", NewVersion: "v1.1.0"}}, nil,
		map[string]stubOutcome{"example.com/a": {result: breakingResult("example.com/a", "v1.0.0", "v1.1.0")}})

	os.Args = []string{"go-semver-audit", "-all", "-markdown"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	main()

	if *exitCode != exitBreaking {
		t.Errorf("exit code = %d, want %d", *exitCode, exitBreaking)
	}
	if !strings.Contains(stdout.String(), "### `example.com/a` v1.0.0 → v1.1.0") {
		t.Errorf("expected a Markdown section for the module, got %q", stdout.String())
	}
}
