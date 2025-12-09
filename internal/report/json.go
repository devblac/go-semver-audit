package report

import (
	"encoding/json"

	"github.com/devblac/go-semver-audit/internal/analyzer"
)

// JSONReport represents the JSON output structure
type JSONReport struct {
	Module            string                `json:"module"`
	OldVersion        string                `json:"old_version"`
	NewVersion        string                `json:"new_version"`
	Breaking          bool                  `json:"breaking"`
	BreakingCount     int                   `json:"breaking_count"`
	AffectedLocations int                   `json:"affected_locations"`
	Removed           []RemovedItem         `json:"removed,omitempty"`
	Changed           []ChangedItem         `json:"changed,omitempty"`
	InterfaceChanges  []InterfaceChangeItem `json:"interface_changes,omitempty"`
	Added             []AddedItem           `json:"added,omitempty"`
	UnusedDeps        []string              `json:"unused_dependencies,omitempty"`
}

// RemovedItem represents a removed symbol in JSON
type RemovedItem struct {
	Name   string     `json:"name"`
	Type   string     `json:"type"`
	UsedIn []Location `json:"used_in,omitempty"`
}

// ChangedItem represents a changed signature in JSON
type ChangedItem struct {
	Name         string     `json:"name"`
	OldSignature string     `json:"old_signature"`
	NewSignature string     `json:"new_signature"`
	UsedIn       []Location `json:"used_in,omitempty"`
}

// InterfaceChangeItem represents interface changes in JSON
type InterfaceChangeItem struct {
	Name           string     `json:"name"`
	AddedMethods   []string   `json:"added_methods,omitempty"`
	RemovedMethods []string   `json:"removed_methods,omitempty"`
	UsedIn         []Location `json:"used_in,omitempty"`
}

// AddedItem represents an added symbol in JSON
type AddedItem struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Location represents a source code location in JSON
type Location struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// FormatJSON generates a JSON report
func FormatJSON(result *analyzer.Result) (string, error) {
	report := JSONReport{
		Module:            result.Module,
		OldVersion:        result.OldVersion,
		NewVersion:        result.NewVersion,
		Breaking:          result.HasBreakingChanges(),
		BreakingCount:     len(result.Changes.Removed) + len(result.Changes.Changed) + len(result.Changes.InterfaceChanges),
		AffectedLocations: countAffectedLocations(result.Changes),
	}

	// Convert removed symbols
	for _, removed := range result.Changes.Removed {
		item := RemovedItem{
			Name: removed.Name,
			Type: removed.Type,
		}
		for _, loc := range removed.UsedIn {
			item.UsedIn = append(item.UsedIn, Location{
				File: loc.File,
				Line: loc.Line,
			})
		}
		report.Removed = append(report.Removed, item)
	}

	// Convert changed signatures
	for _, changed := range result.Changes.Changed {
		item := ChangedItem{
			Name:         changed.Name,
			OldSignature: changed.OldSignature,
			NewSignature: changed.NewSignature,
		}
		for _, loc := range changed.UsedIn {
			item.UsedIn = append(item.UsedIn, Location{
				File: loc.File,
				Line: loc.Line,
			})
		}
		report.Changed = append(report.Changed, item)
	}

	// Convert interface changes
	for _, iface := range result.Changes.InterfaceChanges {
		item := InterfaceChangeItem{
			Name:           iface.Name,
			AddedMethods:   iface.AddedMethods,
			RemovedMethods: iface.RemovedMethods,
		}
		for _, loc := range iface.UsedIn {
			item.UsedIn = append(item.UsedIn, Location{
				File: loc.File,
				Line: loc.Line,
			})
		}
		report.InterfaceChanges = append(report.InterfaceChanges, item)
	}

	// Convert added symbols
	for _, added := range result.Changes.Added {
		report.Added = append(report.Added, AddedItem{
			Name: added.Name,
			Type: added.Type,
		})
	}

	// Add unused dependencies
	report.UnusedDeps = result.UnusedDeps

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data) + "\n", nil
}
