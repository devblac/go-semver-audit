// Package gomod compares go.mod files to find which dependencies an update
// changes, such as the ones in a Dependabot or Renovate pull request.
package gomod

import (
	"fmt"
	"sort"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// Upgrade is a direct dependency whose required version differs between two
// go.mod files.
type Upgrade struct {
	Module string
	// OldModule is the module path required before, when a major version
	// upgrade changed it (example.com/lib for example.com/lib/v2); empty
	// otherwise
	OldModule  string
	OldVersion string
	NewVersion string
}

// DirectUpgrades returns every direct requirement present in both go.mod files
// whose version changed, sorted by module path.
//
// Indirect requirements are skipped: the project does not import them, so
// their API cannot break it directly. A requirement whose module path changes
// with a major version bump (example.com/lib to example.com/lib/v2, or
// gopkg.in/yaml.v2 to gopkg.in/yaml.v3) is paired up when the old path is
// dropped in the same change and nothing else could have been replaced. A
// project that keeps requiring the old major version next to the new one has
// added a dependency, not upgraded one.
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
		if !ok {
			if oldPath := replacedMajor(path, oldReqs, newReqs); oldPath != "" {
				upgrades = append(upgrades, Upgrade{
					Module:     path,
					OldModule:  oldPath,
					OldVersion: oldReqs[oldPath],
					NewVersion: newVersion,
				})
			}
			continue
		}
		if oldVersion == newVersion {
			continue
		}
		upgrades = append(upgrades, Upgrade{Module: path, OldVersion: oldVersion, NewVersion: newVersion})
	}

	sort.Slice(upgrades, func(i, j int) bool {
		return upgrades[i].Module < upgrades[j].Module
	})
	return upgrades, nil
}

// replacedMajor returns the old requirement that the new requirement path is
// another major version of, if exactly one was dropped from the new go.mod
func replacedMajor(path string, oldReqs, newReqs map[string]string) string {
	prefix, _, ok := module.SplitPathVersion(path)
	if !ok {
		return ""
	}
	var match string
	for oldPath := range oldReqs {
		if _, kept := newReqs[oldPath]; kept {
			continue
		}
		if oldPrefix, _, ok := module.SplitPathVersion(oldPath); ok && oldPrefix == prefix {
			if match != "" {
				return ""
			}
			match = oldPath
		}
	}
	return match
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
