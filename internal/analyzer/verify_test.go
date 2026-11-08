package analyzer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestSplitErrorPos(t *testing.T) {
	tests := []struct {
		pos       string
		file      string
		line, col int
	}{
		{"", "", 0, 0},
		{"-", "", 0, 0},
		{"/app/main.go:8:16", "/app/main.go", 8, 16},
		{"/app/main.go:8", "/app/main.go", 8, 0},
		{"/app/main.go", "/app/main.go", 0, 0},
		{`C:\app\main.go:8:16`, `C:\app\main.go`, 8, 16},
		{`C:\app\main.go`, `C:\app\main.go`, 0, 0},
	}
	for _, tt := range tests {
		file, line, col := splitErrorPos(tt.pos)
		if file != tt.file || line != tt.line || col != tt.col {
			t.Errorf("splitErrorPos(%q) = %q, %d, %d; want %q, %d, %d", tt.pos, file, line, col, tt.file, tt.line, tt.col)
		}
	}
}

func TestCollectBuildErrors(t *testing.T) {
	root := t.TempDir()
	a := &Analyzer{projectPath: root}
	mainGo := filepath.Join(root, "main.go")

	msg := "too few values in struct literal of type geo.Point"
	// The test variant repeats the package's own errors
	pkgs := []*packages.Package{
		{ID: "app", Errors: []packages.Error{{Pos: mainGo + ":6:17", Msg: msg}}},
		{ID: "app [app.test]", Errors: []packages.Error{
			{Pos: mainGo + ":6:17", Msg: msg},
			{Pos: filepath.Join(root, "main_test.go") + ":10:16", Msg: msg},
		}},
		{ID: "dep", Errors: []packages.Error{{Pos: "", Msg: "no required module provides package"}}},
	}

	got := a.collectBuildErrors(pkgs)
	want := []BuildError{
		{Message: "no required module provides package"},
		{File: "main.go", Line: 6, Col: 17, Message: msg},
		{File: "main_test.go", Line: 10, Col: 16, Message: msg},
	}
	if len(got) != len(want) {
		t.Fatalf("collectBuildErrors() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("collectBuildErrors()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBuildErrorString(t *testing.T) {
	tests := []struct {
		err  BuildError
		want string
	}{
		{BuildError{File: "main.go", Line: 6, Col: 17, Message: "m"}, "main.go:6:17: m"},
		{BuildError{File: "main.go", Line: 6, Message: "m"}, "main.go:6: m"},
		{BuildError{File: "main.go", Message: "m"}, "main.go: m"},
		{BuildError{Message: "m"}, "m"},
	}
	for _, tt := range tests {
		if got := tt.err.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

// TestVerifyBuildGoGetFails checks that a failed upgrade is reported as an
// error of the check, and that go get only ever sees the copies of go.mod
func TestVerifyBuildGoGetFails(t *testing.T) {
	projectDir := t.TempDir()
	goMod := "module example.com/app\n\ngo 1.21\n"
	goSum := "example.com/lib v1.0.0 h1:abc=\n"
	writeFiles(t, projectDir, map[string]string{"go.mod": goMod, "go.sum": goSum})

	orig := goGet
	defer func() { goGet = orig }()
	var gotModFile string
	goGet = func(dir, modFile, spec string) error {
		gotModFile = modFile
		if dir != projectDir || spec != "example.com/lib@v1.1.0" {
			t.Errorf("goGet(%q, _, %q), want project dir and example.com/lib@v1.1.0", dir, spec)
		}
		if filepath.Dir(modFile) == projectDir {
			t.Errorf("goGet got the project's own go.mod")
		}
		if sum, err := os.ReadFile(filepath.Join(filepath.Dir(modFile), "go.sum")); err != nil || string(sum) != goSum {
			t.Errorf("go.sum was not copied next to the temp go.mod: %q, %v", sum, err)
		}
		return errors.New("go get example.com/lib@v1.1.0: unknown revision")
	}

	a := &Analyzer{projectPath: projectDir}
	if _, err := a.VerifyBuild("example.com/lib", "v1.1.0"); err == nil {
		t.Fatal("VerifyBuild() succeeded although go get failed")
	}
	if _, err := os.Stat(gotModFile); !os.IsNotExist(err) {
		t.Errorf("temp go.mod %s was not removed: %v", gotModFile, err)
	}
}
