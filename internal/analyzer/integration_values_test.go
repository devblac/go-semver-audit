package analyzer

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestAnalyzeValuesFieldsAndTypes covers the parts of the API surface besides
// functions and interfaces: package-level vars and consts, exported struct
// fields, and the underlying types of named types.
func TestAnalyzeValuesFieldsAndTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	const libPath = "example.com/vals"
	root := t.TempDir()
	proxyDir := filepath.Join(root, "proxy")

	publishModule(t, proxyDir, libPath, "v1.0.0", map[string]string{
		"vals.go": `package vals

const MaxSize = 10

const Timeout int = 5

const Gone = "gone"

const Same = "same"

var Default = "x"

var Mode = 1

type ID int

type Name string

type Config struct {
	Host    string
	Port    int
	Retries int
	Debug   bool
}

type Pair[T any] struct{ Left, Right T }

type Dropped struct{ X int }

func New() *Config { return &Config{} }
`,
	})
	publishModule(t, proxyDir, libPath, "v1.1.0", map[string]string{
		"vals.go": `package vals

// The value changes, not the type: nothing breaks
const MaxSize = 20

const Timeout int64 = 5

const Same = "same"

var Default = 1

// A var that became a const can no longer be assigned to
const Mode = 1

type ID string

type Name string

// Port changes type, Retries is removed, Debug is untouched and Extra is
// new; adding a field breaks no keyed use
type Config struct {
	Host    string
	Port    string
	Debug   bool
	Extra   int
}

type Pair[T any] struct{ Left T }

func New() *Config { return &Config{} }
`,
	})

	projectDir := filepath.Join(root, "app")
	writeFiles(t, projectDir, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.21\n\nrequire " + libPath + " v1.0.0\n",
		"main.go": `package main

import "example.com/vals"

func main() {
	_ = vals.MaxSize + vals.Timeout
	_ = vals.Gone + vals.Same + vals.Default
	vals.Mode = 2
	var id vals.ID = 1
	_ = id
	var n vals.Name = "n"
	_ = n

	// The struct type is only reached through New, and fields are set both
	// by selector and by composite literal key
	cfg := vals.New()
	cfg.Port = 8080
	_ = vals.Config{Host: "h", Retries: 3, Debug: true}

	// Fields of an instantiated generic struct
	p := vals.Pair[int]{Left: 1, Right: 2}
	_ = p.Right

	_ = vals.Dropped{X: 1}
}
`,
	})

	useOfflineProxy(t, proxyDir)

	a, err := New(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.Analyze(&Upgrade{Module: libPath, NewVersion: "v1.1.0"})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	changed := map[string]ChangedSignature{}
	for _, c := range result.Changes.Changed {
		changed[c.Name] = c
	}
	removed := map[string]RemovedSymbol{}
	for _, r := range result.Changes.Removed {
		removed[r.Name] = r
	}

	wantChanged := map[string]string{
		"vals.Timeout":     "const",
		"vals.Default":     "var",
		"vals.Mode":        "var",
		"vals.ID":          "type",
		"vals.Config.Port": "field",
	}
	for name, kind := range wantChanged {
		c, ok := changed[name]
		if !ok {
			t.Errorf("expected %s to be reported as changed, got %+v", name, result.Changes.Changed)
			continue
		}
		if c.Type != kind {
			t.Errorf("%s reported as %q, want %q", name, c.Type, kind)
		}
	}
	if c := changed["vals.Mode"]; c.OldSignature != "var int" || c.NewSignature != "const untyped int" {
		t.Errorf("vals.Mode signatures = %q -> %q, want \"var int\" -> \"const untyped int\"", c.OldSignature, c.NewSignature)
	}
	if c := changed["vals.Config.Port"]; c.OldSignature != "int" || c.NewSignature != "string" {
		t.Errorf("vals.Config.Port types = %q -> %q, want int -> string", c.OldSignature, c.NewSignature)
	}

	wantRemoved := map[string]string{
		"vals.Gone":           "const",
		"vals.Config.Retries": "field",
		"vals.Pair.Right":     "field",
		"vals.Dropped":        "type",
	}
	for name, kind := range wantRemoved {
		r, ok := removed[name]
		if !ok {
			t.Errorf("expected %s to be reported as removed, got %+v", name, result.Changes.Removed)
			continue
		}
		if r.Type != kind {
			t.Errorf("%s reported as %q, want %q", name, r.Type, kind)
		}
	}

	// Unchanged, value-only and additive changes break nothing
	for _, name := range []string{"vals.MaxSize", "vals.Same", "vals.Name", "vals.Config", "vals.Config.Host", "vals.Config.Debug", "vals.Pair", "vals.Pair.Left"} {
		if _, ok := changed[name]; ok {
			t.Errorf("%s was reported as changed: %+v", name, changed[name])
		}
		if _, ok := removed[name]; ok {
			t.Errorf("%s was reported as removed: %+v", name, removed[name])
		}
	}
	// A removed struct is one break, not one per field
	if _, ok := removed["vals.Dropped.X"]; ok {
		t.Errorf("vals.Dropped.X reported separately from its removed type")
	}
	if len(result.Changes.Changed) != len(wantChanged) || len(result.Changes.Removed) != len(wantRemoved) {
		t.Errorf("unexpected extra changes: changed %+v, removed %+v", result.Changes.Changed, result.Changes.Removed)
	}
}
