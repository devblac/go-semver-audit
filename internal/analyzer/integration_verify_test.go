package analyzer

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestVerifyBuildCatchesUnkeyedLiteral covers a break the API diff cannot see:
// adding a field to a struct breaks unkeyed composite literals of it. The
// compile check must report it, at the right place, without writing to the
// project.
func TestVerifyBuildCatchesUnkeyedLiteral(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	const libPath = "example.com/geo"
	root := t.TempDir()
	proxyDir := filepath.Join(root, "proxy")

	publishModule(t, proxyDir, libPath, "v1.0.0", map[string]string{
		"geo.go": "package geo\n\ntype Point struct{ X, Y int }\n",
	})
	publishModule(t, proxyDir, libPath, "v1.1.0", map[string]string{
		"geo.go": "package geo\n\ntype Point struct{ X, Y, Z int }\n",
	})

	projectDir := filepath.Join(root, "app")
	writeFiles(t, projectDir, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.21\n\nrequire " + libPath + " v1.0.0\n",
		"main.go": `package main

import "example.com/geo"

func main() {
	p := geo.Point{1, 2}
	_ = p.X
}
`,
		// The same literal in a test file, to check it is reported too
		"main_test.go": `package main

import (
	"testing"

	"example.com/geo"
)

func TestPoint(t *testing.T) {
	_ = geo.Point{3, 4}
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
	if result.HasBreakingChanges() {
		t.Fatalf("the API diff is not expected to see this break, got %+v", result.Changes)
	}

	// The analysis itself runs with GOFLAGS=-mod=mod here and may have written
	// go.sum, so the snapshot is taken after it
	before := snapshotDir(t, projectDir)

	buildErrs, err := a.VerifyBuild(result.Module, result.NewVersion)
	if err != nil {
		t.Fatalf("VerifyBuild() error = %v", err)
	}

	if after := snapshotDir(t, projectDir); !maps.Equal(before, after) {
		t.Errorf("VerifyBuild modified the project directory:\nbefore %v\nafter  %v", slices.Sorted(maps.Keys(before)), slices.Sorted(maps.Keys(after)))
	}

	var got []string
	for _, e := range buildErrs {
		got = append(got, e.File+":"+strconv.Itoa(e.Line))
	}
	// main.go is type-checked both on its own and in the test variant, but
	// each error is reported once
	want := []string{"main.go:6", "main_test.go:10"}
	if !slices.Equal(got, want) {
		t.Fatalf("build errors at %v, want %v (%+v)", got, want, buildErrs)
	}
	for _, e := range buildErrs {
		if e.Col == 0 || !strings.Contains(e.Message, "too few values") {
			t.Errorf("unexpected build error %+v", e)
		}
	}

	result.BuildErrors = buildErrs
	if !result.HasBreakingChanges() || result.HasAPIBreakingChanges() {
		t.Errorf("a result with only build errors must count as breaking, but not as an API break")
	}
}

// TestVerifyBuildNoErrors checks that a compatible upgrade compiles cleanly
func TestVerifyBuildNoErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	const libPath = "example.com/geo"
	root := t.TempDir()
	proxyDir := filepath.Join(root, "proxy")

	publishModule(t, proxyDir, libPath, "v1.0.0", map[string]string{
		"geo.go": "package geo\n\ntype Point struct{ X, Y int }\n",
	})
	publishModule(t, proxyDir, libPath, "v1.1.0", map[string]string{
		"geo.go": "package geo\n\ntype Point struct{ X, Y, Z int }\n",
	})

	projectDir := filepath.Join(root, "app")
	writeFiles(t, projectDir, map[string]string{
		"go.mod":  "module example.com/app\n\ngo 1.21\n\nrequire " + libPath + " v1.0.0\n",
		"main.go": "package main\n\nimport \"example.com/geo\"\n\nfunc main() { _ = geo.Point{X: 1, Y: 2} }\n",
	})

	useOfflineProxy(t, proxyDir)

	a, err := New(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Analyze(&Upgrade{Module: libPath, NewVersion: "v1.1.0"}); err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	buildErrs, err := a.VerifyBuild(libPath, "v1.1.0")
	if err != nil {
		t.Fatalf("VerifyBuild() error = %v", err)
	}
	if len(buildErrs) != 0 {
		t.Fatalf("expected no build errors with keyed literals, got %+v", buildErrs)
	}

	// A version that does not exist is a failure of the check itself, not a
	// compile error
	if _, err := a.VerifyBuild(libPath, "v9.9.9"); err == nil || !strings.Contains(err.Error(), "go get") {
		t.Fatalf("VerifyBuild() with an unknown version: error = %v, want a go get error", err)
	}
}

// snapshotDir returns the contents of every file under dir by relative path
func snapshotDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		files[filepath.ToSlash(rel)] = string(content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
