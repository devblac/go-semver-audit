package gomod

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Allow overriding in tests, which must not reach a module proxy
var goListModules = listModules

// UpdateError is a direct dependency whose available updates could not be
// looked up, for example because the module proxy refused the request.
type UpdateError struct {
	Module  string
	Version string
	Err     string
}

// listedModule is the subset of `go list -m -json` output we rely on
type listedModule struct {
	Path     string
	Version  string
	Main     bool
	Indirect bool
	Update   *struct{ Version string }
	Replace  *struct{ Path, Version string }
	Error    *struct{ Err string }
}

// AvailableUpdates reports every direct dependency of the module in
// projectDir that has a newer version available, as `go list -m -u` sees it.
// Only updates within the same major version are found: a /v2 release is a
// different module path, which go list does not consider an update.
func AvailableUpdates(projectDir string) ([]Upgrade, []UpdateError, error) {
	data, err := goListModules(projectDir)
	if err != nil {
		return nil, nil, err
	}
	return ParseAvailableUpdates(data)
}

// listModules runs `go list -m -u -json all` in projectDir and returns its
// output. -e keeps one module whose update lookup fails from hiding the
// others: the failure is reported in that module's Error field instead.
func listModules(projectDir string) ([]byte, error) {
	args := []string{"list", "-m", "-u", "-e", "-json"}
	// With a vendor directory the go command defaults to -mod=vendor, where it
	// refuses to look up upgrades. -mod=readonly reads the same go.mod without
	// ever writing to it.
	if fi, err := os.Stat(filepath.Join(projectDir, "vendor")); err == nil && fi.IsDir() {
		args = append(args, "-mod=readonly")
	}
	cmd := exec.Command("go", append(args, "all")...)
	cmd.Dir = projectDir
	// Unlike module downloads, this runs in the project itself, so GOFLAGS
	// (e.g. -mod=mod) apply as they do when the project is loaded. A go.work
	// would add the other workspace modules' requirements, so it is ignored.
	cmd.Env = append(os.Environ(), "GOWORK=off")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list -m -u all: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// ParseAvailableUpdates selects the direct dependencies with a newer version
// from `go list -m -u -json` output, sorted by module path. It also returns
// the direct dependencies whose update lookup failed.
//
// Indirect requirements are skipped for the same reason as in DirectUpgrades.
// Replaced modules are skipped too: the project builds against the
// replacement, so comparing two upstream versions would not describe it.
func ParseAvailableUpdates(data []byte) ([]Upgrade, []UpdateError, error) {
	var upgrades []Upgrade
	var lookupErrors []UpdateError

	// The output is a stream of JSON objects, one per module, not an array
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var m listedModule
		if err := dec.Decode(&m); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, nil, fmt.Errorf("failed to parse go list output: %w", err)
		}

		if m.Main || m.Indirect || m.Replace != nil {
			continue
		}
		if m.Error != nil {
			lookupErrors = append(lookupErrors, UpdateError{Module: m.Path, Version: m.Version, Err: m.Error.Err})
			continue
		}
		if m.Update == nil || m.Update.Version == "" {
			continue
		}
		upgrades = append(upgrades, Upgrade{Module: m.Path, OldVersion: m.Version, NewVersion: m.Update.Version})
	}

	sort.Slice(upgrades, func(i, j int) bool {
		return upgrades[i].Module < upgrades[j].Module
	})
	sort.Slice(lookupErrors, func(i, j int) bool {
		return lookupErrors[i].Module < lookupErrors[j].Module
	})
	return upgrades, lookupErrors, nil
}
