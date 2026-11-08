package analyzer

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestAnalyzeMajorVersionUpgrade covers an upgrade to a new major version,
// whose module path changes: the project requires example.com/lib and asks
// about example.com/lib/v2. Signatures that mention the module's own types
// (*Config) read example.com/lib.Config in one version and
// example.com/lib/v2.Config in the other, and must not count as changed.
func TestAnalyzeMajorVersionUpgrade(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	const (
		oldPath = "example.com/lib"
		newPath = "example.com/lib/v2"
	)
	root := t.TempDir()
	proxyDir := filepath.Join(root, "proxy")

	publishModule(t, proxyDir, oldPath, "v1.0.0", map[string]string{
		"lib.go": `package lib

type Config struct {
	Name string
	Old  int
}

type ID int

type Option func(*Config)

type Handler interface{ Handle(c *Config) error }

var Default = &Config{}

func New() *Config { return &Config{} }

func Load(path string) (*Config, error) { return nil, nil }

func With(opts ...Option) *Config { return nil }

func (c *Config) Clone() *Config { return c }

func Removed() {}

func Changed(n int) string { return "" }
`,
		"sub/sub.go": `package sub

import "example.com/lib"

func Use(c *lib.Config) lib.ID { return 0 }

func Gone() {}
`,
	})
	publishModule(t, proxyDir, newPath, "v2.0.0", map[string]string{
		"lib.go": `package lib

type Config struct {
	Name string
}

type ID int

type Option func(*Config)

type Handler interface{ Handle(c *Config) error }

var Default = &Config{}

func New() *Config { return &Config{} }

// Only the parameter name changes, which breaks no caller
func Load(file string) (*Config, error) { return nil, nil }

func With(opts ...Option) *Config { return nil }

func (c *Config) Clone() *Config { return c }

func Changed(n int64) string { return "" }
`,
		"sub/sub.go": `package sub

import "example.com/lib/v2"

func Use(c *lib.Config) lib.ID { return 0 }
`,
	})

	projectDir := filepath.Join(root, "app")
	writeFiles(t, projectDir, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.21\n\nrequire " + oldPath + " v1.0.0\n",
		"main.go": `package main

import (
	"example.com/lib"
	"example.com/lib/sub"
)

type handler struct{}

func (handler) Handle(c *lib.Config) error { return nil }

var _ lib.Handler = handler{}

func main() {
	cfg := lib.New()
	cfg, _ = lib.Load("x")
	cfg = lib.With(func(c *lib.Config) {}).Clone()
	cfg = lib.Default
	_ = cfg.Name + lib.Changed(1)
	_ = cfg.Old
	lib.Removed()
	_ = sub.Use(cfg)
	sub.Gone()
}
`,
	})

	useOfflineProxy(t, proxyDir)

	a, err := New(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.Analyze(&Upgrade{Module: newPath, NewVersion: "v2.0.0"})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if result.Module != newPath || result.OldModule != oldPath ||
		result.OldVersion != "v1.0.0" || result.NewVersion != "v2.0.0" {
		t.Fatalf("Analyze() upgrade = %s %s -> %s %s, want %s v1.0.0 -> %s v2.0.0",
			result.OldModule, result.OldVersion, result.Module, result.NewVersion, oldPath, newPath)
	}

	// Package is the path the project imports today
	removed := map[string]string{}
	for _, r := range result.Changes.Removed {
		removed[r.Name] = r.Package
	}
	wantRemoved := map[string]string{
		"lib.Removed":    "example.com/lib",
		"lib.Config.Old": "example.com/lib",
		"sub.Gone":       "example.com/lib/sub",
	}
	for name, pkg := range wantRemoved {
		if got, ok := removed[name]; !ok || got != pkg {
			t.Errorf("expected %s (%s) to be reported as removed, got %+v", name, pkg, result.Changes.Removed)
		}
	}
	if len(removed) != len(wantRemoved) {
		t.Errorf("removed = %+v, want only %v", result.Changes.Removed, wantRemoved)
	}

	if len(result.Changes.Changed) != 1 || result.Changes.Changed[0].Name != "lib.Changed" {
		t.Fatalf("changed = %+v, want only lib.Changed", result.Changes.Changed)
	}
	if c := result.Changes.Changed[0]; c.OldSignature != "func(n int) string" || c.NewSignature != "func(n int64) string" {
		t.Errorf("lib.Changed signatures = %q -> %q", c.OldSignature, c.NewSignature)
	}

	// New, Load, With, Clone, Default, Handler and sub.Use all mention *Config
	// or ID, whose package path is the only thing that changed
	if len(result.Changes.InterfaceChanges) != 0 {
		t.Errorf("interface changes = %+v, want none", result.Changes.InterfaceChanges)
	}
}
