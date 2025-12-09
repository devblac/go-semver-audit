package report

import (
	"strings"
	"testing"

	"github.com/devblac/go-semver-audit/internal/analyzer"
)

func TestFormatText(t *testing.T) {
	tests := []struct {
		name    string
		result  *analyzer.Result
		verbose bool
		want    []string // strings that should be present in output
		wantNot []string // strings that should not be present
	}{
		{
			name: "no breaking changes",
			result: &analyzer.Result{
				Module:     "github.com/example/lib",
				OldVersion: "v1.0.0",
				NewVersion: "v1.1.0",
				Changes:    &analyzer.Diff{},
			},
			verbose: false,
			want: []string{
				"github.com/example/lib",
				"v1.0.0 -> v1.1.0",
				"No breaking changes",
			},
			wantNot: []string{
				"BREAKING CHANGES",
			},
		},
		{
			name: "removed function",
			result: &analyzer.Result{
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
				},
			},
			verbose: false,
			want: []string{
				"BREAKING CHANGES",
				"Summary: 1 breaking change(s) affecting 1 location(s).",
				"What to fix next:",
				"Remove/replace OldFunc (function) at main.go:45",
				"Removed Symbols",
				"OldFunc",
				"function",
				"main.go:45",
			},
		},
		{
			name: "changed signature",
			result: &analyzer.Result{
				Module:     "github.com/example/lib",
				OldVersion: "v1.0.0",
				NewVersion: "v2.0.0",
				Changes: &analyzer.Diff{
					Changed: []analyzer.ChangedSignature{
						{
							Name:         "ParseConfig",
							OldSignature: "func(string) error",
							NewSignature: "func(string, ...Option) error",
							UsedIn: []analyzer.Location{
								{File: "config.go", Line: 23},
							},
						},
					},
				},
			},
			verbose: true,
			want: []string{
				"BREAKING CHANGES",
				"Changed Signatures",
				"ParseConfig",
				"func(string) error",
				"func(string, ...Option) error",
				"config.go:23",
			},
		},
		{
			name: "interface changes",
			result: &analyzer.Result{
				Module:     "github.com/example/lib",
				OldVersion: "v1.0.0",
				NewVersion: "v2.0.0",
				Changes: &analyzer.Diff{
					InterfaceChanges: []analyzer.InterfaceChange{
						{
							Name:           "Handler",
							RemovedMethods: []string{"Handle(ctx context.Context) error"},
							AddedMethods:   []string{"HandleWithContext(ctx context.Context, meta Metadata) error"},
							UsedIn: []analyzer.Location{
								{File: "handler.go", Line: 67},
							},
						},
					},
				},
			},
			verbose: false,
			want: []string{
				"BREAKING CHANGES",
				"Modified Interfaces",
				"Handler",
				"Removed methods",
				"Added methods",
				"handler.go:67",
			},
		},
		{
			name: "added symbols verbose",
			result: &analyzer.Result{
				Module:     "github.com/example/lib",
				OldVersion: "v1.0.0",
				NewVersion: "v1.1.0",
				Changes: &analyzer.Diff{
					Added: []analyzer.AddedSymbol{
						{Name: "NewFunc", Type: "function"},
					},
				},
			},
			verbose: true,
			want: []string{
				"Added Symbols",
				"NewFunc",
			},
		},
		{
			name: "added symbols non-verbose",
			result: &analyzer.Result{
				Module:     "github.com/example/lib",
				OldVersion: "v1.0.0",
				NewVersion: "v1.1.0",
				Changes: &analyzer.Diff{
					Added: []analyzer.AddedSymbol{
						{Name: "NewFunc", Type: "function"},
					},
				},
			},
			verbose: false,
			wantNot: []string{
				"Added Symbols",
			},
		},
		{
			name: "unused dependencies",
			result: &analyzer.Result{
				Module:     "github.com/example/lib",
				OldVersion: "v1.0.0",
				NewVersion: "v1.1.0",
				Changes:    &analyzer.Diff{},
				UnusedDeps: []string{"github.com/unused/dep"},
			},
			verbose: false,
			want: []string{
				"Unused Dependencies",
				"github.com/unused/dep",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatText(tt.result, tt.verbose)
			if err != nil {
				t.Errorf("FormatText() error = %v", err)
				return
			}

			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("FormatText() output missing expected string %q", want)
				}
			}

			for _, wantNot := range tt.wantNot {
				if strings.Contains(got, wantNot) {
					t.Errorf("FormatText() output contains unexpected string %q", wantNot)
				}
			}
		})
	}
}

func TestFormatLocations(t *testing.T) {
	tests := []struct {
		name      string
		locations []analyzer.Location
		max       int
		want      string
	}{
		{
			name:      "empty",
			locations: []analyzer.Location{},
			max:       3,
			want:      "",
		},
		{
			name: "single location",
			locations: []analyzer.Location{
				{File: "main.go", Line: 10},
			},
			max:  3,
			want: "main.go:10",
		},
		{
			name: "multiple locations under max",
			locations: []analyzer.Location{
				{File: "main.go", Line: 10},
				{File: "handler.go", Line: 25},
			},
			max:  3,
			want: "main.go:10, handler.go:25",
		},
		{
			name: "multiple locations over max",
			locations: []analyzer.Location{
				{File: "main.go", Line: 10},
				{File: "handler.go", Line: 25},
				{File: "config.go", Line: 5},
				{File: "util.go", Line: 100},
			},
			max:  2,
			want: "main.go:10, handler.go:25, and 2 more",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatLocations(tt.locations, tt.max)
			if got != tt.want {
				t.Errorf("formatLocations() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCountAffectedLocations(t *testing.T) {
	changes := &analyzer.Diff{
		Removed: []analyzer.RemovedSymbol{
			{UsedIn: []analyzer.Location{{File: "a.go", Line: 1}, {File: "b.go", Line: 2}}},
		},
		Changed: []analyzer.ChangedSignature{
			{UsedIn: []analyzer.Location{{File: "c.go", Line: 3}}},
		},
		InterfaceChanges: []analyzer.InterfaceChange{
			{UsedIn: []analyzer.Location{{File: "d.go", Line: 4}, {File: "e.go", Line: 5}}},
		},
	}

	got := countAffectedLocations(changes)
	want := 5

	if got != want {
		t.Errorf("countAffectedLocations() = %d, want %d", got, want)
	}
}
