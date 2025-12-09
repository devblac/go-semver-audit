package report

import (
	"strings"
	"testing"

	"github.com/devblac/go-semver-audit/internal/analyzer"
)

func TestFormatHTML(t *testing.T) {
	result := &analyzer.Result{
		Module:     "github.com/example/lib",
		OldVersion: "v1.0.0",
		NewVersion: "v2.0.0",
		Changes: &analyzer.Diff{
			Removed: []analyzer.RemovedSymbol{
				{
					Name: "OldFunc",
					Type: "function",
					UsedIn: []analyzer.Location{
						{File: "main.go", Line: 45},
					},
				},
			},
			Changed: []analyzer.ChangedSignature{
				{
					Name:         "ParseConfig",
					OldSignature: "func(string) error",
					NewSignature: "func(string, ...Option) error",
					UsedIn:       []analyzer.Location{{File: "config.go", Line: 23}},
				},
			},
			InterfaceChanges: []analyzer.InterfaceChange{
				{
					Name:           "Handler",
					RemovedMethods: []string{"Handle(ctx context.Context) error"},
					AddedMethods:   []string{"HandleWithContext(ctx context.Context, meta Metadata) error"},
					UsedIn:         []analyzer.Location{{File: "handler.go", Line: 67}},
				},
			},
			Added: []analyzer.AddedSymbol{{Name: "NewFunc", Type: "function"}},
		},
		UnusedDeps: []string{"github.com/unused/dep"},
	}

	out, err := FormatHTML(result)
	if err != nil {
		t.Fatalf("FormatHTML() error = %v", err)
	}

	expect := []string{
		"<!DOCTYPE html>",
		"github.com/example/lib",
		"v1.0.0",
		"v2.0.0",
		"Breaking changes",
		"Removed symbols",
		"Changed signatures",
		"Modified interfaces",
		"Unused dependencies",
		"main.go:45",
		"config.go:23",
		"handler.go:67",
	}

	for _, want := range expect {
		if !strings.Contains(out, want) {
			t.Fatalf("expected HTML output to contain %q", want)
		}
	}
}
