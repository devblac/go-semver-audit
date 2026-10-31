// Package gomod compares go.mod files to find which dependencies an update
// changes, such as the ones in a Dependabot or Renovate pull request.
package gomod

import (
	"fmt"
	"sort"

	"golang.org/x/mod/modfile"
)

// Upgrade is a direct dependency whose required version differs between two
// go.mod files.
type Upgrade struct {
	Module     string
	OldVersion string
	NewVersion string
}

// DirectUpgrades returns every direct requirement present in both go.mod files
// whose version changed, sorted by module path.
//
// Indirect requirements are skipped: the project does not import them, so
// their API cannot break it directly. A requirement whose module path changes
// (a major version bump to /v2 or beyond) shows up as one removal plus one
// addition and is not paired up.
func DirectUpgrades(oldGoMod, newGoMod []byte) ([]Upgrade, error) {
	oldReqs, err := directRequirements("old go.mod", oldGoMod)
	if err != nil {
		return nil, err
	}
	newReqs, err := directRequirements("new go.mod", newGoMod)
	if err != nil {
		return nil, err
	}

	var upgrades []Upgrade
	for path, newVersion := range newReqs {
		oldVersion, ok := oldReqs[path]
		if !ok || oldVersion == newVersion {
			continue
		}
		upgrades = append(upgrades, Upgrade{Module: path, OldVersion: oldVersion, NewVersion: newVersion})
	}

	sort.Slice(upgrades, func(i, j int) bool {
		return upgrades[i].Module < upgrades[j].Module
	})
	return upgrades, nil
}

// directRequirements maps each direct requirement's module path to its version
func directRequirements(name string, data []byte) (map[string]string, error) {
	// ParseLax ignores directives it does not know, so go.mod files written
	// by newer Go releases still parse; only require lines matter here
	file, err := modfile.ParseLax(name, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", name, err)
	}

	reqs := make(map[string]string)
	for _, req := range file.Require {
		if req.Indirect {
			continue
		}
		reqs[req.Mod.Path] = req.Mod.Version
	}
	return reqs, nil
}
