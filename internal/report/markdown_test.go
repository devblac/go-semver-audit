package report

import (
	"strings"
	"testing"

	"github.com/devblac/go-semver-audit/internal/analyzer"
)

func TestFormatMarkdownBreakingChanges(t *testing.T) {
	result := &analyzer.Result{
		Module:     "example.com/lib",
		OldVersion: "v1.0.0",
		NewVersion: "v2.0.0",
		Changes: &analyzer.Diff{
			Removed: []analyzer.RemovedSymbol{
				{Name: "lib.OldHelper", Type: "function", UsedIn: []analyzer.Location{
					{File: "main.go", Line: 12},
					{File: "a.go", Line: 1}, {File: "b.go", Line: 2}, {File: "c.go", Line: 3}, {File: "d.go", Line: 4},
				}},
			},
			Changed: []analyzer.ChangedSignature{
				{
					Name:         "lib.Config.Validate",
					OldSignature: "func(strict bool) error",
					NewSignature: "func(ctx context.Context) error",
					UsedIn:       []analyzer.Location{{File: "config/load.go", Line: 23}},
				},
			},
			InterfaceChanges: []analyzer.InterfaceChange{
				{
					Name:           "lib.Handler",
					RemovedMethods: []string{"Handle(ctx context.Context) error"},
					AddedMethods:   []string{"HandleWithContext(ctx context.Context, meta Metadata) error"},
					UsedIn:         []analyzer.Location{{File: "handlers/http.go", Line: 67}},
				},
			},
			// Informational only, must not appear in the PR comment
			Added: []analyzer.AddedSymbol{{Name: "lib.NewHelper", Type: "function"}},
		},
	}

	got, err := FormatMarkdown(result)
	if err != nil {
		t.Fatalf("FormatMarkdown() error = %v", err)
	}

	for _, want := range []string{
		"### `example.com/lib` v1.0.0 → v2.0.0",
		"⚠️ **3 breaking changes** affecting **7 locations** in this project.",
		"| Change | Symbol | Used in |",
		"| Removed function | `lib.OldHelper` | `main.go:12`, `a.go:1`, `b.go:2`, +2 more |",
		"| Signature changed | `lib.Config.Validate` | `config/load.go:23` |",
		"| Interface changed | `lib.Handler` | `handlers/http.go:67` |",
		"<details>",
		"- func(strict bool) error\n+ func(ctx context.Context) error",
		"- Handle(ctx context.Context) error\n+ HandleWithContext(ctx context.Context, meta Metadata) error",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("FormatMarkdown() missing %q\n--- got ---\n%s", want, got)
		}
	}

	if strings.Contains(got, "NewHelper") {
		t.Errorf("FormatMarkdown() should not list added symbols\n%s", got)
	}
}

func TestFormatMarkdownNoBreakingChanges(t *testing.T) {
	result := &analyzer.Result{
		Module:     "example.com/lib",
		OldVersion: "v1.0.0",
		NewVersion: "v1.0.1",
		Changes:    &analyzer.Diff{},
		UnusedDeps: []string{"example.com/unused"},
	}

	got, err := FormatMarkdown(result)
	if err != nil {
		t.Fatalf("FormatMarkdown() error = %v", err)
	}

	if !strings.Contains(got, "✅ No breaking changes") {
		t.Errorf("FormatMarkdown() missing no-breaking-changes line\n%s", got)
	}
	if strings.Contains(got, "| Change |") || strings.Contains(got, "<details>") {
		t.Errorf("FormatMarkdown() should not render a table or details without breaking changes\n%s", got)
	}
	if !strings.Contains(got, "**Unused dependencies:** `example.com/unused`") {
		t.Errorf("FormatMarkdown() missing unused dependencies\n%s", got)
	}
}

func TestFormatMarkdownSingularAndEscaping(t *testing.T) {
	result := &analyzer.Result{
		Module: "example.com/lib",
		Changes: &analyzer.Diff{
			Removed: []analyzer.RemovedSymbol{
				{Name: "lib.Gone", Type: "function", UsedIn: []analyzer.Location{{File: "odd|name.go", Line: 1}}},
			},
		},
	}

	got, err := FormatMarkdown(result)
	if err != nil {
		t.Fatalf("FormatMarkdown() error = %v", err)
	}

	if !strings.Contains(got, "**1 breaking change** affecting **1 location**") {
		t.Errorf("FormatMarkdown() should use singular nouns\n%s", got)
	}
	// An unescaped pipe would split the row into an extra column
	if !strings.Contains(got, "`odd\\|name.go:1`") {
		t.Errorf("FormatMarkdown() should escape pipes in table cells\n%s", got)
	}
	// Removals alone carry no signatures to show
	if strings.Contains(got, "<details>") {
		t.Errorf("FormatMarkdown() should not render details for removals only\n%s", got)
	}
}
