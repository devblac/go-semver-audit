package report

import (
	"fmt"
	"strings"

	"github.com/devblac/go-semver-audit/internal/analyzer"
)

// FormatText generates a human-readable text report
func FormatText(result *analyzer.Result, verbose bool) (string, error) {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("Analyzing upgrade: %s %s -> %s\n\n",
		result.Module, result.OldVersion, result.NewVersion))

	// Check if there are any breaking changes
	hasBreaking := result.HasBreakingChanges()
	breakingCount := len(result.Changes.Removed) + len(result.Changes.Changed) + len(result.Changes.InterfaceChanges)
	usageCount := countAffectedLocations(result.Changes)

	if !hasBreaking {
		b.WriteString("✓ No breaking changes detected.\n\n")
	} else {
		b.WriteString("⚠️  BREAKING CHANGES DETECTED\n\n")
	}

	if hasBreaking {
		b.WriteString(fmt.Sprintf("Summary: %d breaking change(s) affecting %d location(s).\n\n", breakingCount, usageCount))

		if fixes := summarizeFixes(result.Changes, 3); len(fixes) > 0 {
			b.WriteString("What to fix next:\n")
			for _, fix := range fixes {
				b.WriteString(fmt.Sprintf("  - %s\n", fix))
			}
			b.WriteString("\n")
		}
	}

	changes := result.Changes

	// Report removed symbols
	if len(changes.Removed) > 0 {
		b.WriteString("Removed Symbols:\n")
		for _, removed := range changes.Removed {
			b.WriteString(fmt.Sprintf("  - %s (%s)", removed.Name, removed.Type))
			if len(removed.UsedIn) > 0 {
				b.WriteString(" (used in: ")
				locations := formatLocations(removed.UsedIn, 3)
				b.WriteString(locations)
				b.WriteString(")")
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Report changed signatures
	if len(changes.Changed) > 0 {
		b.WriteString("Changed Signatures:\n")
		for _, changed := range changes.Changed {
			b.WriteString(fmt.Sprintf("  - %s\n", changed.Name))
			if verbose {
				b.WriteString(fmt.Sprintf("    Old: %s\n", changed.OldSignature))
				b.WriteString(fmt.Sprintf("    New: %s\n", changed.NewSignature))
			}
			if len(changed.UsedIn) > 0 {
				locations := formatLocations(changed.UsedIn, 3)
				b.WriteString(fmt.Sprintf("    Used in: %s\n", locations))
			}
		}
		b.WriteString("\n")
	}

	// Report interface changes
	if len(changes.InterfaceChanges) > 0 {
		b.WriteString("Modified Interfaces:\n")
		for _, iface := range changes.InterfaceChanges {
			b.WriteString(fmt.Sprintf("  - %s\n", iface.Name))
			if len(iface.RemovedMethods) > 0 {
				b.WriteString("    Removed methods:\n")
				for _, method := range iface.RemovedMethods {
					b.WriteString(fmt.Sprintf("      - %s\n", method))
				}
			}
			if len(iface.AddedMethods) > 0 {
				b.WriteString("    Added methods:\n")
				for _, method := range iface.AddedMethods {
					b.WriteString(fmt.Sprintf("      - %s\n", method))
				}
			}
			if len(iface.UsedIn) > 0 {
				locations := formatLocations(iface.UsedIn, 3)
				b.WriteString(fmt.Sprintf("    Used in: %s\n", locations))
			}
		}
		b.WriteString("\n")
	}

	// Report added symbols (informational, only in verbose mode)
	if verbose && len(changes.Added) > 0 {
		b.WriteString("Added Symbols (informational):\n")
		for _, added := range changes.Added {
			b.WriteString(fmt.Sprintf("  + %s (%s)\n", added.Name, added.Type))
		}
		b.WriteString("\n")
	}

	// Report unused dependencies
	if len(result.UnusedDeps) > 0 {
		b.WriteString("Unused Dependencies:\n")
		for _, dep := range result.UnusedDeps {
			b.WriteString(fmt.Sprintf("  - %s\n", dep))
		}
		b.WriteString("\n")
	}

	// Summary
	if hasBreaking {
		b.WriteString(fmt.Sprintf("Summary: %d breaking change(s) affecting %d location(s) in your code.\n",
			breakingCount, usageCount))
	}

	return b.String(), nil
}

// summarizeFixes returns a short list of items to address first.
func summarizeFixes(changes *analyzer.Diff, max int) []string {
	var fixes []string

	for _, removed := range changes.Removed {
		if len(removed.UsedIn) == 0 {
			continue
		}
		fixes = append(fixes, fmt.Sprintf("Remove/replace %s (%s) at %s", removed.Name, removed.Type, formatLocations(removed.UsedIn, 1)))
	}

	for _, changed := range changes.Changed {
		if len(changed.UsedIn) == 0 {
			continue
		}
		fixes = append(fixes, fmt.Sprintf("Update call to %s at %s", changed.Name, formatLocations(changed.UsedIn, 1)))
	}

	for _, iface := range changes.InterfaceChanges {
		if len(iface.UsedIn) == 0 {
			continue
		}
		action := "Update implementations"
		fixes = append(fixes, fmt.Sprintf("%s of %s at %s", action, iface.Name, formatLocations(iface.UsedIn, 1)))
	}

	if len(fixes) > max {
		return fixes[:max]
	}
	return fixes
}

// formatLocations formats a list of locations for display
func formatLocations(locations []analyzer.Location, max int) string {
	if len(locations) == 0 {
		return ""
	}

	var parts []string
	for i, loc := range locations {
		if i >= max {
			parts = append(parts, fmt.Sprintf("and %d more", len(locations)-max))
			break
		}
		parts = append(parts, fmt.Sprintf("%s:%d", loc.File, loc.Line))
	}

	return strings.Join(parts, ", ")
}

// countAffectedLocations counts total number of affected code locations
func countAffectedLocations(changes *analyzer.Diff) int {
	count := 0

	for _, removed := range changes.Removed {
		count += len(removed.UsedIn)
	}

	for _, changed := range changes.Changed {
		count += len(changed.UsedIn)
	}

	for _, iface := range changes.InterfaceChanges {
		count += len(iface.UsedIn)
	}

	return count
}
