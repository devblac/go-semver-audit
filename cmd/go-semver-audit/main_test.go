package main

import (
	"flag"
	"testing"

	"github.com/yourusername/go-semver-audit/internal/analyzer"
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
}

func TestConfigStruct(t *testing.T) {
	// Test that config struct can be created and fields accessed
	cfg := config{
		projectPath: "/test/path",
		upgrade:     "github.com/example/module@v1.0.0",
		jsonOutput:  true,
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
