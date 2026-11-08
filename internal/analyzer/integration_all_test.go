package analyzer

import (
	"errors"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/devblac/go-semver-audit/internal/gomod"
	"golang.org/x/tools/go/packages"
)

// TestAuditAllAvailableUpdates runs what -all does against a local proxy:
// find the direct dependencies with a newer version with the real go
// command, then analyze each with one Analyzer that loads the project once.
func TestAuditAllAvailableUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	root := t.TempDir()
	proxyDir := filepath.Join(root, "proxy")

	// alpha removes a function the project calls
	publishModule(t, proxyDir, "example.com/alpha", "v1.0.0", map[string]string{
		"alpha.go": "package alpha\n\nfunc Old() int { return 1 }\n\nfunc Kept() int { return 2 }\n",
	})
	publishModule(t, proxyDir, "example.com/alpha", "v1.1.0", map[string]string{
		"alpha.go": "package alpha\n\nfunc Kept() int { return 2 }\n",
	})
	// beta only adds a function
	publishModule(t, proxyDir, "example.com/beta", "v1.0.0", map[string]string{
		"beta.go": "package beta\n\nfunc Hello() string { return \"hi\" }\n",
	})
	publishModule(t, proxyDir, "example.com/beta", "v1.0.1", map[string]string{
		"beta.go": "package beta\n\nfunc Hello() string { return \"hi\" }\n\nfunc Bye() string { return \"bye\" }\n",
	})
	// gamma has no newer version
	publishModule(t, proxyDir, "example.com/gamma", "v1.0.0", map[string]string{
		"gamma.go": "package gamma\n\nfunc G() {}\n",
	})
	// unused is required but never imported
	publishModule(t, proxyDir, "example.com/unused", "v1.0.0", map[string]string{
		"unused.go": "package unused\n\nfunc U() {}\n",
	})
	publishModule(t, proxyDir, "example.com/unused", "v1.1.0", map[string]string{
		"unused.go": "package unused\n",
	})

	projectDir := filepath.Join(root, "app")
	writeFiles(t, projectDir, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.21\n\nrequire (\n" +
			"\texample.com/alpha v1.0.0\n\texample.com/beta v1.0.0\n\texample.com/gamma v1.0.0\n\texample.com/unused v1.0.0\n)\n",
		"main.go": `package main

import (
	"example.com/alpha"
	"example.com/beta"
	"example.com/gamma"
)

func main() {
	_ = alpha.Old() + alpha.Kept()
	_ = beta.Hello()
	gamma.G()
}
`,
	})

	useOfflineProxy(t, proxyDir)

	upgrades, lookupErrors, err := gomod.AvailableUpdates(projectDir)
	if err != nil {
		t.Fatalf("AvailableUpdates() error = %v", err)
	}
	if len(lookupErrors) != 0 {
		t.Fatalf("unexpected lookup errors: %+v", lookupErrors)
	}
	wantUpgrades := []gomod.Upgrade{
		{Module: "example.com/alpha", OldVersion: "v1.0.0", NewVersion: "v1.1.0"},
		{Module: "example.com/beta", OldVersion: "v1.0.0", NewVersion: "v1.0.1"},
		{Module: "example.com/unused", OldVersion: "v1.0.0", NewVersion: "v1.1.0"},
	}
	if !reflect.DeepEqual(upgrades, wantUpgrades) {
		t.Fatalf("AvailableUpdates() = %+v, want %+v", upgrades, wantUpgrades)
	}

	projectLoads := 0
	realLoad := packagesLoad
	restore := mockPackagesLoad(func(cfg *packages.Config, patterns ...string) ([]*packages.Package, error) {
		if len(patterns) == 1 && patterns[0] == "./..." {
			projectLoads++
		}
		return realLoad(cfg, patterns...)
	})
	defer restore()

	a, err := New(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	results := map[string]*Result{}
	for _, upgrade := range upgrades {
		result, err := a.Analyze(&Upgrade{Module: upgrade.Module, NewVersion: upgrade.NewVersion})
		if upgrade.Module == "example.com/unused" {
			if !errors.Is(err, ErrModuleNotImported) {
				t.Errorf("Analyze(%s) error = %v, want ErrModuleNotImported", upgrade.Module, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("Analyze(%s) error = %v", upgrade.Module, err)
		}
		results[upgrade.Module] = result
	}

	if projectLoads != 1 {
		t.Errorf("project loaded %d times for %d upgrades, want 1", projectLoads, len(upgrades))
	}

	alpha := results["example.com/alpha"]
	if alpha == nil || !alpha.HasBreakingChanges() || len(alpha.Changes.Removed) != 1 || alpha.Changes.Removed[0].Name != "alpha.Old" {
		t.Errorf("expected alpha.Old to be reported as removed, got %+v", alpha)
	}
	if alpha != nil && (alpha.OldVersion != "v1.0.0" || alpha.NewVersion != "v1.1.0") {
		t.Errorf("alpha versions = %s -> %s", alpha.OldVersion, alpha.NewVersion)
	}
	if beta := results["example.com/beta"]; beta == nil || beta.HasBreakingChanges() {
		t.Errorf("expected beta to have no breaking changes, got %+v", beta)
	}
}
