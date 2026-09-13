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
`,
		"sub/sub.go": `package sub

func Gone() {}
`,
	})
	// v1.1.0 changes SortFunc (same break as golang.org/x/exp/slices in 2023),
	// removes OldHelper and deletes the sub package entirely.
	publishModule(t, proxyDir, libPath, "v1.1.0", map[string]string{
		"lib.go": `package lib

func SortFunc(s []int, cmp func(a, b int) int) {}

func Stable() string { return "stable" }
`,
	})

	projectDir := filepath.Join(root, "app")
	writeFiles(t, projectDir, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.21\n\nrequire " + libPath + " v1.0.0\n",
		"main.go": `package main

import (
	"example.com/lib"
	"example.com/lib/sub"
)

func main() {
	lib.SortFunc([]int{2, 1}, func(a, b int) bool { return a < b })
	_ = lib.OldHelper() + lib.Stable()
	sub.Gone()
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
	if !changed["SortFunc"] {
		t.Errorf("expected SortFunc signature change, got %+v", result.Changes.Changed)
	}
	if changed["Stable"] {
		t.Errorf("Stable did not change but was reported")
	}

	removed := map[string]bool{}
	for _, r := range result.Changes.Removed {
		removed[r.Name] = true
	}
	for _, name := range []string{"OldHelper", "Gone"} {
		if !removed[name] {
			t.Errorf("expected %s to be reported as removed, got %+v", name, result.Changes.Removed)
		}
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
