package analyzer

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/mod/module"
	"golang.org/x/mod/zip"
)

// TestAnalyzeRealModules runs the full pipeline against real module versions
// served from a local file-based GOPROXY, so no network access is needed.
func TestAnalyzeRealModules(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	const libPath = "example.com/lib"
	root := t.TempDir()
	proxyDir := filepath.Join(root, "proxy")

	publishModule(t, proxyDir, libPath, "v1.0.0", map[string]string{
		"lib.go": `package lib

func SortFunc(s []int, less func(a, b int) bool) {}

func OldHelper() string { return "old" }

func Stable() string { return "stable" }

func New() *Config { return &Config{} }

type Config struct{ Name string }

func (c *Config) Validate(strict bool) error { return nil }
`,
		"keep/keep.go": `package keep

func New() string { return "keep" }
`,
		"sub/sub.go": `package sub

func Gone() {}
`,
	})
	// v1.1.0 changes SortFunc (same break as golang.org/x/exp/slices in 2023),
	// changes the Config.Validate method signature, removes OldHelper and
	// lib.New, and deletes the sub package entirely. keep.New is untouched,
	// so it must not be confused with the removed lib.New.
	publishModule(t, proxyDir, libPath, "v1.1.0", map[string]string{
		"lib.go": `package lib

import "context"

func SortFunc(s []int, cmp func(a, b int) int) {}

func Stable() string { return "stable" }

type Config struct{ Name string }

func (c *Config) Validate(ctx context.Context) error { return nil }
`,
		"keep/keep.go": `package keep

func New() string { return "keep" }
`,
	})

	projectDir := filepath.Join(root, "app")
	writeFiles(t, projectDir, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.21\n\nrequire " + libPath + " v1.0.0\n",
		"main.go": `package main

import (
	"example.com/lib"
	"example.com/lib/keep"
	"example.com/lib/sub"
)

func main() {
	lib.SortFunc([]int{2, 1}, func(a, b int) bool { return a < b })
	_ = lib.OldHelper() + lib.Stable() + keep.New()
	sub.Gone()

	// cfg's type is never named here, so the receiver type is only reachable
	// through the method call below
	cfg := lib.New()
	_ = cfg.Validate(true)
}
`,
	})

	modCache, err := os.MkdirTemp("", "go-semver-audit-modcache-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// The module cache is read-only; let the go command remove it
		clean := exec.Command("go", "clean", "-modcache")
		clean.Env = append(os.Environ(), "GOMODCACHE="+modCache)
		_ = clean.Run()
		os.RemoveAll(modCache)
	})

	t.Setenv("GOMODCACHE", modCache)
	t.Setenv("GOPROXY", fileURL(proxyDir))
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOTOOLCHAIN", "local")

	a, err := New(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.Analyze(&Upgrade{Module: libPath, NewVersion: "latest"})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if result.OldVersion != "v1.0.0" || result.NewVersion != "v1.1.0" {
		t.Fatalf("Analyze() versions = %s -> %s, want v1.0.0 -> v1.1.0", result.OldVersion, result.NewVersion)
	}

	changed := map[string]bool{}
	for _, c := range result.Changes.Changed {
		changed[c.Name] = true
	}
	if !changed["lib.SortFunc"] {
		t.Errorf("expected lib.SortFunc signature change, got %+v", result.Changes.Changed)
	}
	// Methods are keyed by receiver; before that they never matched usage and
	// changed method signatures went undetected entirely
	if !changed["lib.Config.Validate"] {
		t.Errorf("expected lib.Config.Validate signature change, got %+v", result.Changes.Changed)
	}
	if changed["lib.Stable"] {
		t.Errorf("lib.Stable did not change but was reported")
	}

	removed := map[string]bool{}
	for _, r := range result.Changes.Removed {
		removed[r.Name] = true
	}
	for _, name := range []string{"lib.OldHelper", "lib.New", "sub.Gone"} {
		if !removed[name] {
			t.Errorf("expected %s to be reported as removed, got %+v", name, result.Changes.Removed)
		}
	}
	// keep.New survives the upgrade: same bare name as the removed lib.New,
	// different package, and it must not be dragged in by the collision
	if removed["keep.New"] {
		t.Errorf("keep.New still exists but was reported as removed: %+v", result.Changes.Removed)
	}
}

// publishModule writes module@version into a GOPROXY directory layout.
func publishModule(t *testing.T, proxyDir, modPath, version string, files map[string]string) {
	t.Helper()

	srcDir := filepath.Join(t.TempDir(), "src")
	goMod := "module " + modPath + "\n\ngo 1.21\n"
	files["go.mod"] = goMod
	writeFiles(t, srcDir, files)

	versionDir := filepath.Join(proxyDir, filepath.FromSlash(modPath), "@v")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	zipFile, err := os.Create(filepath.Join(versionDir, version+".zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer zipFile.Close()
	if err := zip.CreateFromDir(zipFile, module.Version{Path: modPath, Version: version}, srcDir); err != nil {
		t.Fatal(err)
	}

	writeFiles(t, versionDir, map[string]string{
		version + ".mod":  goMod,
		version + ".info": `{"Version":"` + version + `"}`,
	})

	list, err := os.OpenFile(filepath.Join(versionDir, "list"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer list.Close()
	if _, err := list.WriteString(version + "\n"); err != nil {
		t.Fatal(err)
	}
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func fileURL(path string) string {
	path = filepath.ToSlash(path)
	if runtime.GOOS == "windows" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "file://" + path
}
