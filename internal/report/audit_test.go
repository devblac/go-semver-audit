package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/devblac/go-semver-audit/internal/analyzer"
)

func sampleAudit() *Audit {
	return &Audit{
		Results: []*analyzer.Result{
			{
				Module: "example.com/broken", OldVersion: "v1.0.0", NewVersion: "v1.1.0",
				Changes: &analyzer.Diff{
					Removed: []analyzer.RemovedSymbol{{
						Name: "lib.Gone", Type: "function",
						UsedIn: []analyzer.Location{{File: "main.go", Line: 7}},
					}},
				},
			},
			{Module: "example.com/clean", OldVersion: "v0.2.0", NewVersion: "v0.3.0", Changes: &analyzer.Diff{}},
		},
		Failed: []Failure{
			{Module: "example.com/fails", OldVersion: "v1.0.0", NewVersion: "v1.0.1", Error: "go mod download: exit status 1\nnot found <here>"},
			{Module: "example.com/offline", OldVersion: "v2.0.0", Error: "lookup failed"},
		},
		NotImported: []analyzer.Upgrade{{Module: "example.com/tagged", OldVersion: "v1.0.0", NewVersion: "v1.2.0"}},
	}
}

func TestAuditSummary(t *testing.T) {
	tests := []struct {
		name  string
		audit *Audit
		want  string
	}{
		{"empty", &Audit{}, "No direct dependency has a newer version available"},
		{
			"one clean",
			&Audit{Results: []*analyzer.Result{{Module: "a", Changes: &analyzer.Diff{}}}},
			"1 dependency update audited: 0 with breaking changes, 1 without",
		},
		{
			"everything",
			sampleAudit(),
			"5 dependency updates audited: 1 with breaking changes, 1 without, 2 could not be analyzed, 1 not imported by the project",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.audit.Summary(); got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Each multi-module report must embed the single-module report of every
// result unchanged, so the two modes read the same
func TestAuditReportsEmbedSingleReports(t *testing.T) {
	audit := sampleAudit()

	text, err := FormatTextAudit(audit, true)
	if err != nil {
		t.Fatal(err)
	}
	markdown, err := FormatMarkdownAudit(audit)
	if err != nil {
		t.Fatal(err)
	}

	for _, result := range audit.Results {
		single, _ := FormatText(result, true)
		if !strings.Contains(text, strings.TrimRight(single, "\n")) {
			t.Errorf("text audit does not contain the report of %s", result.Module)
		}
		single, _ = FormatMarkdown(result)
		if !strings.Contains(markdown, single) {
			t.Errorf("markdown audit does not contain the report of %s", result.Module)
		}
	}
}

func TestFormatTextAudit(t *testing.T) {
	got, err := FormatTextAudit(sampleAudit(), false)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(got, "5 dependency updates audited: 1 with breaking changes, 1 without, 2 could not be analyzed, 1 not imported by the project.\n\n") {
		t.Errorf("missing summary header:\n%s", got)
	}
	for _, want := range []string{
		textSeparator + "Analyzing upgrade: example.com/broken v1.0.0 -> v1.1.0\n",
		textSeparator + "Analyzing upgrade: example.com/clean v0.2.0 -> v0.3.0\n",
		// Multi-line errors stay indented under their module
		"Could not be analyzed:\n  - example.com/fails v1.0.0 -> v1.0.1\n    go mod download: exit status 1\n    not found <here>\n",
		// The update could not be looked up, so there is no new version to show
		"  - example.com/offline v2.0.0\n    lookup failed\n",
		"Not imported by any package of the project (nothing to check):\n  - example.com/tagged v1.0.0 -> v1.2.0\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("text audit is missing %q:\n%s", want, got)
		}
	}
}

func TestFormatTextAuditEmpty(t *testing.T) {
	got, err := FormatTextAudit(&Audit{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "No direct dependency has a newer version available.\n\n" {
		t.Errorf("FormatTextAudit(empty) = %q", got)
	}
}

func TestFormatMarkdownAudit(t *testing.T) {
	got, err := FormatMarkdownAudit(sampleAudit())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"### `example.com/broken` v1.0.0 → v1.1.0",
		"### `example.com/clean` v0.2.0 → v0.3.0",
		"### Could not be analyzed\n\n`example.com/fails` v1.0.0 → v1.0.1\n\n```\ngo mod download: exit status 1\nnot found <here>\n```\n\n",
		"`example.com/offline` v2.0.0\n\n```\nlookup failed\n```",
		"### Not imported by this project\n\n- `example.com/tagged` v1.0.0 → v1.2.0\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("markdown audit is missing %q:\n%s", want, got)
		}
	}
}

func TestFormatJSONAudit(t *testing.T) {
	out, err := FormatJSONAudit(sampleAudit())
	if err != nil {
		t.Fatal(err)
	}

	var got JSONAuditReport
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if !got.Breaking {
		t.Errorf("breaking = false, want true")
	}
	if len(got.Results) != 2 || got.Results[0].Module != "example.com/broken" || got.Results[0].BreakingCount != 1 ||
		got.Results[1].Module != "example.com/clean" || got.Results[1].Breaking {
		t.Errorf("unexpected results: %+v", got.Results)
	}
	if len(got.Errors) != 2 || got.Errors[0].NewVersion != "v1.0.1" || got.Errors[1].Error != "lookup failed" {
		t.Errorf("unexpected errors: %+v", got.Errors)
	}
	if len(got.NotImported) != 1 || got.NotImported[0].Module != "example.com/tagged" {
		t.Errorf("unexpected not_imported: %+v", got.NotImported)
	}

	// Each result is the single-upgrade JSON object, field for field
	single, _ := FormatJSON(sampleAudit().Results[0])
	var want JSONReport
	if err := json.Unmarshal([]byte(single), &want); err != nil {
		t.Fatal(err)
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got.Results[0])
	if string(wantJSON) != string(gotJSON) {
		t.Errorf("result differs from FormatJSON:\n got %s\nwant %s", gotJSON, wantJSON)
	}
}

// Empty lists are [] rather than null, so consumers can iterate them blindly
func TestFormatJSONAuditEmpty(t *testing.T) {
	out, err := FormatJSONAudit(&Audit{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"breaking": false`, `"results": []`, `"errors": []`, `"not_imported": []`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in %s", want, out)
		}
	}
}

func TestFormatHTMLAudit(t *testing.T) {
	got, err := FormatHTMLAudit(sampleAudit())
	if err != nil {
		t.Fatal(err)
	}

	if strings.Count(got, "<!DOCTYPE html>") != 1 || strings.Count(got, "</html>") != 1 {
		t.Errorf("expected a single HTML document:\n%s", got)
	}
	for _, want := range []string{
		"<title>go-semver-audit: 2 upgrade(s) analyzed</title>",
		"5 dependency updates audited: 1 with breaking changes",
		"example.com/broken v1.0.0 → v1.1.0",
		"example.com/clean v0.2.0 → v0.3.0",
		"<strong>lib.Gone</strong>",
		"<h2>Could not be analyzed</h2>",
		// Error text is escaped
		"not found &lt;here&gt;",
		"<strong>example.com/offline v2.0.0</strong>",
		"<h2>Not imported by the project</h2>",
		"example.com/tagged v1.0.0 → v1.2.0",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("HTML audit is missing %q", want)
		}
	}
}

func TestFormatHTMLAuditStatus(t *testing.T) {
	failedOnly := &Audit{Failed: []Failure{{Module: "example.com/a", OldVersion: "v1.0.0", Error: "boom"}}}
	got, err := FormatHTMLAudit(failedOnly)
	if err != nil {
		t.Fatal(err)
	}
	// A failure must not be summarized as "no breaking changes"
	if !strings.Contains(got, "Some upgrades could not be analyzed") || strings.Contains(got, "No breaking changes") {
		t.Errorf("unexpected status for an audit with only failures:\n%s", got)
	}
}
