package report

import (
	"encoding/json"
	"testing"

	"github.com/devblac/go-semver-audit/internal/analyzer"
)

func TestFormatJSON(t *testing.T) {
	tests := []struct {
		name    string
		result  *analyzer.Result
		wantErr bool
	}{
		{
			name: "no breaking changes",
			result: &analyzer.Result{
				Module:     "github.com/example/lib",
				OldVersion: "v1.0.0",
				NewVersion: "v1.1.0",
				Changes:    &analyzer.Diff{},
			},
			wantErr: false,
		},
		{
			name: "with breaking changes",
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
					Added: []analyzer.AddedSymbol{
						{Name: "NewFunc", Type: "function"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "with unused dependencies",
			result: &analyzer.Result{
				Module:     "github.com/example/lib",
				OldVersion: "v1.0.0",
				NewVersion: "v1.1.0",
				Changes:    &analyzer.Diff{},
				UnusedDeps: []string{"github.com/unused/dep"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatJSON(tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("FormatJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				// Validate that the output is valid JSON
				var report JSONReport
				if err := json.Unmarshal([]byte(got), &report); err != nil {
					t.Errorf("FormatJSON() produced invalid JSON: %v", err)
				}

				// Validate key fields
				if report.Module != tt.result.Module {
					t.Errorf("JSONReport.Module = %q, want %q", report.Module, tt.result.Module)
				}
				if report.OldVersion != tt.result.OldVersion {
					t.Errorf("JSONReport.OldVersion = %q, want %q", report.OldVersion, tt.result.OldVersion)
				}
				if report.NewVersion != tt.result.NewVersion {
					t.Errorf("JSONReport.NewVersion = %q, want %q", report.NewVersion, tt.result.NewVersion)
				}
				if report.Breaking != tt.result.HasBreakingChanges() {
					t.Errorf("JSONReport.Breaking = %v, want %v", report.Breaking, tt.result.HasBreakingChanges())
				}

				// Validate removed symbols
				if len(report.Removed) != len(tt.result.Changes.Removed) {
					t.Errorf("JSONReport.Removed count = %d, want %d", len(report.Removed), len(tt.result.Changes.Removed))
				}

				// Validate changed signatures
				if len(report.Changed) != len(tt.result.Changes.Changed) {
					t.Errorf("JSONReport.Changed count = %d, want %d", len(report.Changed), len(tt.result.Changes.Changed))
				}

				// Validate interface changes
				if len(report.InterfaceChanges) != len(tt.result.Changes.InterfaceChanges) {
					t.Errorf("JSONReport.InterfaceChanges count = %d, want %d", len(report.InterfaceChanges), len(tt.result.Changes.InterfaceChanges))
				}

				// Validate unused dependencies
				if len(report.UnusedDeps) != len(tt.result.UnusedDeps) {
					t.Errorf("JSONReport.UnusedDeps count = %d, want %d", len(report.UnusedDeps), len(tt.result.UnusedDeps))
				}
			}
		})
	}
}

func TestJSONReportStructure(t *testing.T) {
	// Test that the JSON structure is correct
	result := &analyzer.Result{
		Module:     "github.com/test/module",
		OldVersion: "v1.0.0",
		NewVersion: "v2.0.0",
		Changes: &analyzer.Diff{
			Removed: []analyzer.RemovedSymbol{
				{
					Name: "TestFunc",
					Type: "function",
					UsedIn: []analyzer.Location{
						{File: "test.go", Line: 10},
					},
				},
			},
		},
	}

	output, err := FormatJSON(result)
	if err != nil {
		t.Fatalf("FormatJSON() error = %v", err)
	}

	var report JSONReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Check structure
	if len(report.Removed) != 1 {
		t.Fatalf("Expected 1 removed symbol, got %d", len(report.Removed))
	}

	removed := report.Removed[0]
	if removed.Name != "TestFunc" {
		t.Errorf("Removed.Name = %q, want %q", removed.Name, "TestFunc")
	}
	if removed.Type != "function" {
		t.Errorf("Removed.Type = %q, want %q", removed.Type, "function")
	}
	if len(removed.UsedIn) != 1 {
		t.Errorf("Removed.UsedIn length = %d, want 1", len(removed.UsedIn))
	}
	if removed.UsedIn[0].File != "test.go" {
		t.Errorf("Removed.UsedIn[0].File = %q, want %q", removed.UsedIn[0].File, "test.go")
	}
	if removed.UsedIn[0].Line != 10 {
		t.Errorf("Removed.UsedIn[0].Line = %d, want 10", removed.UsedIn[0].Line)
	}
}
