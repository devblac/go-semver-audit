package gomod

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// goListOutput mimics `go list -m -u -json all`: concatenated objects, with
// the main module first and fields this package does not use
const goListOutput = `{
	"Path": "example.com/app",
	"Main": true,
	"Dir": "/src/app",
	"GoMod": "/src/app/go.mod",
	"GoVersion": "1.26"
}
{
	"Path": "github.com/stretchr/testify",
	"Version": "v1.8.0",
	"Update": {
		"Path": "github.com/stretchr/testify",
		"Version": "v1.9.0",
		"Time": "2024-03-03T12:30:00Z"
	},
	"Time": "2022-06-27T13:52:11Z",
	"GoMod": "/mod/cache/download/github.com/stretchr/testify/@v/v1.8.0.mod",
	"GoVersion": "1.13"
}
{
	"Path": "example.com/current",
	"Version": "v1.2.0"
}
{
	"Path": "example.com/alpha",
	"Version": "v0.3.0",
	"Update": {
		"Path": "example.com/alpha",
		"Version": "v0.4.1"
	}
}
{
	"Path": "golang.org/x/sys",
	"Version": "v0.10.0",
	"Update": {
		"Path": "golang.org/x/sys",
		"Version": "v0.12.0"
	},
	"Indirect": true
}
{
	"Path": "example.com/forked",
	"Version": "v1.0.0",
	"Update": {
		"Path": "example.com/forked",
		"Version": "v1.1.0"
	},
	"Replace": {
		"Path": "example.com/fork",
		"Version": "v1.0.1"
	}
}
{
	"Path": "example.com/unreachable",
	"Version": "v1.0.0",
	"Error": {
		"Err": "module lookup disabled by GOPROXY=off"
	}
}
`

func TestParseAvailableUpdates(t *testing.T) {
	upgrades, lookupErrors, err := ParseAvailableUpdates([]byte(goListOutput))
	if err != nil {
		t.Fatalf("ParseAvailableUpdates() error = %v", err)
	}

	// Only direct, unreplaced requirements with an update, sorted by path.
	// The main module, an up-to-date module, an indirect one and a replaced
	// one are all left out.
	want := []Upgrade{
		{Module: "example.com/alpha", OldVersion: "v0.3.0", NewVersion: "v0.4.1"},
		{Module: "github.com/stretchr/testify", OldVersion: "v1.8.0", NewVersion: "v1.9.0"},
	}
	if !reflect.DeepEqual(upgrades, want) {
		t.Errorf("upgrades = %+v, want %+v", upgrades, want)
	}

	wantErrors := []UpdateError{
		{Module: "example.com/unreachable", Version: "v1.0.0", Err: "module lookup disabled by GOPROXY=off"},
	}
	if !reflect.DeepEqual(lookupErrors, wantErrors) {
		t.Errorf("lookup errors = %+v, want %+v", lookupErrors, wantErrors)
	}
}

func TestParseAvailableUpdatesEmpty(t *testing.T) {
	upgrades, lookupErrors, err := ParseAvailableUpdates(nil)
	if err != nil || len(upgrades) != 0 || len(lookupErrors) != 0 {
		t.Fatalf("ParseAvailableUpdates(nil) = %v, %v, %v; want nothing", upgrades, lookupErrors, err)
	}
}

func TestParseAvailableUpdatesInvalid(t *testing.T) {
	_, _, err := ParseAvailableUpdates([]byte(`{"Path": "example.com/a"} {"Path": `))
	if err == nil || !strings.Contains(err.Error(), "failed to parse go list output") {
		t.Fatalf("expected a parse error, got %v", err)
	}
}

func TestAvailableUpdatesRunsGoListInProject(t *testing.T) {
	orig := goListModules
	defer func() { goListModules = orig }()

	var gotDir string
	goListModules = func(projectDir string) ([]byte, error) {
		gotDir = projectDir
		return []byte(goListOutput), nil
	}

	upgrades, _, err := AvailableUpdates("/src/app")
	if err != nil {
		t.Fatalf("AvailableUpdates() error = %v", err)
	}
	if gotDir != "/src/app" {
		t.Errorf("go list ran in %q, want /src/app", gotDir)
	}
	if len(upgrades) != 2 {
		t.Errorf("expected 2 upgrades, got %+v", upgrades)
	}

	goListModules = func(string) ([]byte, error) { return nil, errors.New("go list failed") }
	if _, _, err := AvailableUpdates("/src/app"); err == nil {
		t.Fatal("expected the go list error to be returned")
	}
}
