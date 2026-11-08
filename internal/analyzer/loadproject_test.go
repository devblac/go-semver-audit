package analyzer

import (
	"errors"
	"testing"

	"golang.org/x/tools/go/packages"
)

// The project is type-checked once per Analyzer, however many upgrades it
// analyzes; a failed load is not remembered, so a later call can retry.
func TestLoadProjectLoadsOnce(t *testing.T) {
	loads := 0
	fail := true
	restoreLoad := mockPackagesLoad(func(cfg *packages.Config, patterns ...string) ([]*packages.Package, error) {
		loads++
		if fail {
			return nil, errors.New("load failure")
		}
		return []*packages.Package{{PkgPath: "example.com/app"}}, nil
	})
	defer restoreLoad()
	restorePrint := mockPackagesPrintErrors(func(pkgs []*packages.Package) int { return 0 })
	defer restorePrint()

	a := &Analyzer{projectPath: "."}
	if err := a.LoadProject(); err == nil {
		t.Fatal("LoadProject() expected an error")
	}

	fail = false
	for range 3 {
		if err := a.LoadProject(); err != nil {
			t.Fatalf("LoadProject() error = %v", err)
		}
	}
	// Analyze goes through LoadProject too; the module is not imported, which
	// is reported with ErrModuleNotImported
	if _, err := a.Analyze(&Upgrade{Module: "example.com/lib", NewVersion: "v1.1.0"}); !errors.Is(err, ErrModuleNotImported) {
		t.Fatalf("Analyze() error = %v, want ErrModuleNotImported", err)
	}

	if loads != 2 {
		t.Fatalf("project loaded %d times, want 2 (one failure, then one success)", loads)
	}
}
