package gomod

import (
	"reflect"
	"strings"
	"testing"
)

func TestDirectUpgrades(t *testing.T) {
	oldGoMod := `module example.com/app

go 1.24.0

require github.com/stretchr/testify v1.8.0

require (
	golang.org/x/exp v0.0.0-20230713183714-613f0c0eb8a1
	example.com/unchanged v1.0.0
	example.com/removed v1.0.0
	example.com/major v1.4.0
	example.com/downgraded v1.5.0
	golang.org/x/sys v0.10.0 // indirect
)
`
	newGoMod := `module example.com/app

go 1.24.0

require github.com/stretchr/testify v1.9.0

require (
	golang.org/x/exp v0.0.0-20230801115018-d63ba01acd4b
	example.com/unchanged v1.0.0
	example.com/added v1.0.0
	example.com/major/v2 v2.0.0
	example.com/downgraded v1.4.0
	golang.org/x/sys v0.12.0 // indirect
)
`

	got, err := DirectUpgrades([]byte(oldGoMod), []byte(newGoMod))
	if err != nil {
		t.Fatalf("DirectUpgrades() error = %v", err)
	}

	want := []Upgrade{
		// A downgrade is still a version change the project has to survive
		{Module: "example.com/downgraded", OldVersion: "v1.5.0", NewVersion: "v1.4.0"},
		{Module: "github.com/stretchr/testify", OldVersion: "v1.8.0", NewVersion: "v1.9.0"},
		{Module: "golang.org/x/exp", OldVersion: "v0.0.0-20230713183714-613f0c0eb8a1", NewVersion: "v0.0.0-20230801115018-d63ba01acd4b"},
	}
	// Not listed: unchanged, added, removed, the /v2 path change, and the
	// indirect golang.org/x/sys bump
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DirectUpgrades() =\n%+v\nwant\n%+v", got, want)
	}
}

func TestDirectUpgradesNoChanges(t *testing.T) {
	goMod := []byte("module example.com/app\n\ngo 1.24.0\n\nrequire example.com/lib v1.0.0\n")

	got, err := DirectUpgrades(goMod, goMod)
	if err != nil {
		t.Fatalf("DirectUpgrades() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("DirectUpgrades() = %+v, want none", got)
	}
}

func TestDirectUpgradesToleratesUnknownDirectives(t *testing.T) {
	// A directive from a Go release newer than our x/mod must not break parsing
	newGoMod := []byte("module example.com/app\n\ngo 1.30.0\n\nfuturedirective something\n\nrequire example.com/lib v1.1.0\n")
	oldGoMod := []byte("module example.com/app\n\ngo 1.24.0\n\nrequire example.com/lib v1.0.0\n")

	got, err := DirectUpgrades(oldGoMod, newGoMod)
	if err != nil {
		t.Fatalf("DirectUpgrades() error = %v", err)
	}
	if len(got) != 1 || got[0].NewVersion != "v1.1.0" {
		t.Fatalf("DirectUpgrades() = %+v, want example.com/lib v1.1.0", got)
	}
}

func TestDirectUpgradesInvalidFile(t *testing.T) {
	valid := []byte("module example.com/app\n")
	_, err := DirectUpgrades(valid, []byte("require (\n"))
	if err == nil || !strings.Contains(err.Error(), "new go.mod") {
		t.Fatalf("DirectUpgrades() expected parse error naming the new go.mod, got %v", err)
	}
}
