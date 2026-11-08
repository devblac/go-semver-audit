package report

import (
	"fmt"
	"strings"

	"github.com/devblac/go-semver-audit/internal/analyzer"
)

// Audit is the outcome of auditing every direct dependency that has a newer
// version available (-all). Each formatter renders it as a short summary
// followed by the single-upgrade report of every analyzed module.
type Audit struct {
	Results []*analyzer.Result
	// Failed lists the upgrades that could not be analyzed
	Failed []Failure
	// NotImported lists upgrades of dependencies that no package of the
	// project imports, tests included: there is no usage to check, so they
	// cannot break the build as far as this tool can tell
	NotImported []analyzer.Upgrade
}

// Failure is an upgrade that could not be analyzed
type Failure struct {
	Module     string
	OldVersion string
	NewVersion string // empty when the available update could not be looked up
	Error      string
}

// BreakingModules counts the analyzed upgrades with breaking changes
func (a *Audit) BreakingModules() int {
	count := 0
	for _, result := range a.Results {
		if result.HasBreakingChanges() {
			count++
		}
	}
	return count
}

// Summary describes the audit in one sentence, without a trailing period
func (a *Audit) Summary() string {
	total := len(a.Results) + len(a.Failed) + len(a.NotImported)
	if total == 0 {
		return "No direct dependency has a newer version available"
	}

	breaking := a.BreakingModules()
	parts := []string{
		fmt.Sprintf("%d with breaking changes", breaking),
		fmt.Sprintf("%d without", len(a.Results)-breaking),
	}
	if len(a.Failed) > 0 {
		parts = append(parts, fmt.Sprintf("%d could not be analyzed", len(a.Failed)))
	}
	if len(a.NotImported) > 0 {
		parts = append(parts, fmt.Sprintf("%d not imported by the project", len(a.NotImported)))
	}
	return fmt.Sprintf("%s audited: %s",
		plural(total, "dependency update", "dependency updates"), strings.Join(parts, ", "))
}

// upgradeLabel names an upgrade as "module old -> new", leaving out a new
// version that is not known
func upgradeLabel(module, oldVersion, newVersion, arrow string) string {
	if newVersion == "" {
		return module + " " + oldVersion
	}
	return fmt.Sprintf("%s %s %s %s", module, oldVersion, arrow, newVersion)
}
