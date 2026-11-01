package report

import (
	"fmt"
	"strings"

	"github.com/devblac/go-semver-audit/internal/analyzer"
)

// markdownMaxLocations caps how many usage locations a table row lists
const markdownMaxLocations = 3

// FormatMarkdown generates a GitHub-flavored Markdown report for a pull request
// comment or a job summary. Each result renders as one self-contained section,
// so reports for several upgrades can be concatenated.
func FormatMarkdown(result *analyzer.Result) (string, error) {
	var b strings.Builder

	fmt.Fprintf(&b, "### `%s` %s → %s\n\n", result.Module, result.OldVersion, result.NewVersion)

	changes := result.Changes
	if changes == nil {
		changes = &analyzer.Diff{}
	}

	if !result.HasBreakingChanges() {
		b.WriteString("✅ No breaking changes in the APIs this project uses.\n\n")
	} else {
		breakingCount := len(changes.Removed) + len(changes.Changed) + len(changes.InterfaceChanges)
		fmt.Fprintf(&b, "⚠️ **%s** affecting **%s** in this project.\n\n",
			plural(breakingCount, "breaking change", "breaking changes"),
			plural(countAffectedLocations(changes), "location", "locations"))

		b.WriteString("| Change | Symbol | Used in |\n")
		b.WriteString("|---|---|---|\n")
		for _, removed := range changes.Removed {
			writeMarkdownRow(&b, "Removed "+removed.Type, removed.Name, removed.UsedIn)
		}
		for _, changed := range changes.Changed {
			writeMarkdownRow(&b, "Signature changed", changed.Name, changed.UsedIn)
		}
		for _, iface := range changes.InterfaceChanges {
			writeMarkdownRow(&b, "Interface changed", iface.Name, iface.UsedIn)
		}
		b.WriteString("\n")

		writeMarkdownDetails(&b, changes)
	}

	if len(result.UnusedDeps) > 0 {
		deps := make([]string, len(result.UnusedDeps))
		for i, dep := range result.UnusedDeps {
			deps[i] = "`" + dep + "`"
		}
		fmt.Fprintf(&b, "**Unused dependencies:** %s\n\n", strings.Join(deps, ", "))
	}

	return b.String(), nil
}

// writeMarkdownRow writes one table row for a breaking change
func writeMarkdownRow(b *strings.Builder, change, symbol string, locations []analyzer.Location) {
	fmt.Fprintf(b, "| %s | `%s` | %s |\n",
		escapeTableCell(change), escapeTableCell(symbol), markdownLocations(locations))
}

// writeMarkdownDetails writes the old and new signatures, and interface method
// changes, in a collapsed block so the table stays scannable
func writeMarkdownDetails(b *strings.Builder, changes *analyzer.Diff) {
	if len(changes.Changed) == 0 && len(changes.InterfaceChanges) == 0 {
		return
	}

	b.WriteString("<details>\n<summary>Details</summary>\n\n")
	for _, changed := range changes.Changed {
		fmt.Fprintf(b, "`%s`\n\n```diff\n- %s\n+ %s\n```\n\n", changed.Name, changed.OldSignature, changed.NewSignature)
	}
	for _, iface := range changes.InterfaceChanges {
		fmt.Fprintf(b, "`%s`\n\n```diff\n", iface.Name)
		for _, method := range iface.RemovedMethods {
			fmt.Fprintf(b, "- %s\n", method)
		}
		for _, method := range iface.AddedMethods {
			fmt.Fprintf(b, "+ %s\n", method)
		}
		b.WriteString("```\n\n")
	}
	b.WriteString("</details>\n\n")
}

// markdownLocations renders up to markdownMaxLocations locations as code spans
func markdownLocations(locations []analyzer.Location) string {
	if len(locations) == 0 {
		return ""
	}

	var parts []string
	for i, loc := range locations {
		if i == markdownMaxLocations {
			parts = append(parts, fmt.Sprintf("+%d more", len(locations)-markdownMaxLocations))
			break
		}
		parts = append(parts, fmt.Sprintf("`%s:%d`", escapeTableCell(loc.File), loc.Line))
	}
	return strings.Join(parts, ", ")
}

// escapeTableCell keeps a pipe in a value from splitting the table row
func escapeTableCell(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

// plural formats a count with the singular or plural noun
func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, pluralForm)
}
