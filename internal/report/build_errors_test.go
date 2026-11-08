package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/devblac/go-semver-audit/internal/analyzer"
)

// buildOnlyResult is a result where only the compile check (-verify) found a
// problem, as with a field added to a struct used in an unkeyed literal
func buildOnlyResult() *analyzer.Result {
	return &analyzer.Result{
		Module:     "example.com/geo",
		OldVersion: "v1.0.0",
		NewVersion: "v1.1.0",
		Changes:    &analyzer.Diff{},
		BuildErrors: []analyzer.BuildError{
			{File: "main.go", Line: 6, Col: 17, Message: "too few values in struct literal of type geo.Point"},
			{Message: "no required module provides package example.com/geo/old"},
		},
	}
}

// bothResult has an API break and a compile error
func bothResult() *analyzer.Result {
	result := buildOnlyResult()
	result.BuildErrors = result.BuildErrors[:1]
	result.Changes.Removed = []analyzer.RemovedSymbol{
		{Name: "geo.Origin", Type: "function", UsedIn: []analyzer.Location{{File: "main.go", Line: 9}}},
	}
	return result
}

func TestFormatTextBuildErrors(t *testing.T) {
	got, err := FormatText(buildOnlyResult(), false)
	if err != nil {
		t.Fatal(err)
	}
	want := `Analyzing upgrade: example.com/geo v1.0.0 -> v1.1.0

⚠️  BREAKING CHANGES DETECTED

Compile errors with the new version (2):
  - main.go:6:17: too few values in struct literal of type geo.Point
  - no required module provides package example.com/geo/old
  The API diff found no breaking change that explains these errors.

`
	if got != want {
		t.Errorf("FormatText() =\n%s\nwant\n%s", got, want)
	}

	got, err = FormatText(bothResult(), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Summary: 1 breaking change(s) affecting 1 location(s).",
		"Compile errors with the new version (1):\n  - main.go:6:17: too few values",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("FormatText() missing %q\n%s", want, got)
		}
	}
	// With an API break reported, the errors may well be explained by it
	if strings.Contains(got, "explains these errors") {
		t.Errorf("FormatText() should not call the errors unexplained when the API diff found a break\n%s", got)
	}
}

func TestFormatJSONBuildErrors(t *testing.T) {
	got, err := FormatJSON(buildOnlyResult())
	if err != nil {
		t.Fatal(err)
	}

	var report JSONReport
	if err := json.Unmarshal([]byte(got), &report); err != nil {
		t.Fatal(err)
	}
	if !report.Breaking || report.BreakingCount != 0 {
		t.Errorf("breaking = %v, breaking_count = %d; want true, 0 (compile errors break, but are not API changes)",
			report.Breaking, report.BreakingCount)
	}
	want := BuildErrorItem{File: "main.go", Line: 6, Column: 17, Message: "too few values in struct literal of type geo.Point"}
	if len(report.BuildErrors) != 2 || report.BuildErrors[0] != want {
		t.Errorf("build_errors = %+v, want %+v first", report.BuildErrors, want)
	}
	// Unknown positions are left out rather than reported as 0
	if !strings.Contains(got, `{
      "message": "no required module provides package example.com/geo/old"
    }`) {
		t.Errorf("FormatJSON() should omit an unknown position\n%s", got)
	}

	clean, err := FormatJSON(&analyzer.Result{Module: "example.com/geo", Changes: &analyzer.Diff{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(clean, "build_errors") {
		t.Errorf("FormatJSON() should omit build_errors without -verify errors\n%s", clean)
	}
}

func TestFormatMarkdownBuildErrors(t *testing.T) {
	got, err := FormatMarkdown(buildOnlyResult())
	if err != nil {
		t.Fatal(err)
	}
	want := "### `example.com/geo` v1.0.0 → v1.1.0\n\n" +
		"⚠️ **This project does not compile with v1.1.0**, although the API diff found no breaking change that explains it.\n\n" +
		"**Compile errors with the new version** (2):\n\n```text\n" +
		"main.go:6:17: too few values in struct literal of type geo.Point\n" +
		"no required module provides package example.com/geo/old\n" +
		"```\n\n"
	if got != want {
		t.Errorf("FormatMarkdown() =\n%s\nwant\n%s", got, want)
	}

	got, err = FormatMarkdown(bothResult())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "| Removed function | `geo.Origin` |") || !strings.Contains(got, "**Compile errors with the new version** (1)") {
		t.Errorf("FormatMarkdown() should render the table and the compile errors\n%s", got)
	}
	if strings.Contains(got, "does not compile with") {
		t.Errorf("FormatMarkdown() should not call the errors unexplained when the API diff found a break\n%s", got)
	}
}

func TestFormatHTMLBuildErrors(t *testing.T) {
	got, err := FormatHTML(buildOnlyResult())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Breaking changes detected",
		`<div class="label">Compile errors</div>`,
		"<h2>Compile errors with the new version</h2>",
		"no breaking change that explains these errors",
		"<code>main.go:6:17: too few values in struct literal of type geo.Point</code>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("FormatHTML() missing %q", want)
		}
	}

	clean, err := FormatHTML(&analyzer.Result{Module: "example.com/geo", Changes: &analyzer.Diff{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(clean, "Compile errors") {
		t.Errorf("FormatHTML() should not render compile errors without -verify errors")
	}
}
