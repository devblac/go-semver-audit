package analyzer

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

// Allow overriding in tests
var (
	packagesLoad        = packages.Load
	packagesPrintErrors = packages.PrintErrors
)

// Analyzer performs static analysis on Go projects
type Analyzer struct {
	projectPath string
	pkgs        []*packages.Package
}

// New creates a new Analyzer for the given project path
func New(projectPath string) (*Analyzer, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("project path does not exist: %s", absPath)
	}

	return &Analyzer{
		projectPath: absPath,
	}, nil
}

// Analyze performs the dependency upgrade analysis
func (a *Analyzer) Analyze(upgrade *Upgrade) (*Result, error) {
	// Load the project packages
	if err := a.loadProject(); err != nil {
		return nil, fmt.Errorf("failed to load project: %w", err)
	}

	// Get current version from project dependencies
	currentVersion, err := a.getCurrentVersion(upgrade.Module)
	if err != nil {
		return nil, fmt.Errorf("failed to determine current version: %w", err)
	}
	upgrade.OldVersion = currentVersion

	// Load API surface for old and new versions
	oldAPI, err := a.loadModuleAPI(upgrade.Module, upgrade.OldVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to load old API: %w", err)
	}

	newAPI, err := a.loadModuleAPI(upgrade.Module, upgrade.NewVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to load new API: %w", err)
	}

	// Find usage of the dependency in the project
	usage := a.findUsage(upgrade.Module)

	// Diff the APIs
	diff := diffAPIs(oldAPI, newAPI, usage)

	return &Result{
		Module:     upgrade.Module,
		OldVersion: upgrade.OldVersion,
		NewVersion: upgrade.NewVersion,
		Changes:    diff,
		UnusedDeps: nil, // Filled by separate call if requested
	}, nil
}

// FindUnusedDependencies identifies dependencies that are no longer used
func (a *Analyzer) FindUnusedDependencies() ([]string, error) {
	if len(a.pkgs) == 0 {
		if err := a.loadProject(); err != nil {
			return nil, err
		}
	}

	// Get all direct dependencies from go.mod
	dependencies, err := a.getDirectDependencies()
	if err != nil {
		return nil, err
	}

	// Find which dependencies are actually imported
	imported := make(map[string]bool)
	for _, pkg := range a.pkgs {
		for _, imp := range pkg.Imports {
			// Extract module path from import path
			modPath := extractModulePath(imp.PkgPath)
			if modPath != "" {
				imported[modPath] = true
			}
		}
	}

	// Identify unused dependencies
	var unused []string
	for _, dep := range dependencies {
		if !imported[dep] {
			unused = append(unused, dep)
		}
	}

	return unused, nil
}

// loadProject loads the Go packages for the project
func (a *Analyzer) loadProject() error {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports |
			packages.NeedDeps | packages.NeedTypes | packages.NeedSyntax |
			packages.NeedTypesInfo | packages.NeedModule,
		Dir: a.projectPath,
	}

	pkgs, err := packagesLoad(cfg, "./...")
	if err != nil {
		return fmt.Errorf("failed to load packages: %w", err)
	}

	if packagesPrintErrors(pkgs) > 0 {
		return fmt.Errorf("packages contain errors")
	}

	a.pkgs = pkgs
	return nil
}

// getCurrentVersion retrieves the current version of a module from go.mod
func (a *Analyzer) getCurrentVersion(module string) (string, error) {
	// Look through loaded packages to find the module version
	for _, pkg := range a.pkgs {
		if pkg.Module != nil && pkg.Module.Path == module {
			return pkg.Module.Version, nil
		}
		// Check dependencies
		for _, dep := range a.getDependencyModules(pkg) {
			if dep.Path == module {
				return dep.Version, nil
			}
		}
	}

	return "", fmt.Errorf("module %s not found in project dependencies", module)
}

// getDependencyModules extracts dependency modules from a package
func (a *Analyzer) getDependencyModules(pkg *packages.Package) []*packages.Module {
	var modules []*packages.Module
	seen := make(map[string]bool)

	for _, imp := range pkg.Imports {
		if imp.Module != nil && !seen[imp.Module.Path] {
			modules = append(modules, imp.Module)
			seen[imp.Module.Path] = true
		}
	}

	return modules
}

// loadModuleAPI loads the exported API surface for a specific module version
func (a *Analyzer) loadModuleAPI(module, version string) (*API, error) {
	// Load the module at the specified version
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedSyntax |
			packages.NeedTypesInfo,
		Env: append(os.Environ(), "GOFLAGS=-mod=readonly"),
	}

	modulePattern := fmt.Sprintf("%s@%s", module, version)
	pkgs, err := packagesLoad(cfg, modulePattern)
	if err != nil {
		return nil, fmt.Errorf("failed to load module %s: %w", modulePattern, err)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found for module %s", modulePattern)
	}

	// Extract exported symbols
	api := &API{
		Funcs:      make(map[string]*Function),
		Types:      make(map[string]*Type),
		Interfaces: make(map[string]*Interface),
	}

	for _, pkg := range pkgs {
		if pkg.Types == nil {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if !obj.Exported() {
				continue
			}

			switch obj := obj.(type) {
			case *types.Func:
				sig := obj.Type().(*types.Signature)
				api.Funcs[obj.Name()] = &Function{
					Name:      obj.Name(),
					Signature: sig.String(),
					PkgPath:   pkg.PkgPath,
				}

			case *types.TypeName:
				named, ok := obj.Type().(*types.Named)
				if !ok {
					continue
				}

				// Check if it's an interface
				iface, isInterface := named.Underlying().(*types.Interface)
				if isInterface {
					methods := make([]string, iface.NumMethods())
					for i := 0; i < iface.NumMethods(); i++ {
						methods[i] = iface.Method(i).String()
					}
					api.Interfaces[obj.Name()] = &Interface{
						Name:    obj.Name(),
						Methods: methods,
						PkgPath: pkg.PkgPath,
					}
				} else {
					// Regular type
					api.Types[obj.Name()] = &Type{
						Name:    obj.Name(),
						Kind:    named.Underlying().String(),
						PkgPath: pkg.PkgPath,
					}

					// Add methods for this type
					for i := 0; i < named.NumMethods(); i++ {
						method := named.Method(i)
						if method.Exported() {
							key := fmt.Sprintf("%s.%s", obj.Name(), method.Name())
							sig := method.Type().(*types.Signature)
							api.Funcs[key] = &Function{
								Name:      key,
								Signature: sig.String(),
								PkgPath:   pkg.PkgPath,
								IsMethod:  true,
							}
						}
					}
				}
			}
		}
	}

	return api, nil
}

// findUsage identifies which exported symbols from the module are used in the project
func (a *Analyzer) findUsage(module string) *Usage {
	usage := &Usage{
		Symbols: make(map[string][]Location),
		Imports: make(map[string]bool),
	}

	for _, pkg := range a.pkgs {
		// Check if this package imports the target module
		for _, imp := range pkg.Imports {
			if imp.Module != nil && imp.Module.Path == module {
				usage.Imports[imp.PkgPath] = true
			}
		}

		// Scan for symbol usage in the package
		if pkg.TypesInfo == nil {
			continue
		}

		for ident, obj := range pkg.TypesInfo.Uses {
			if obj == nil || !obj.Exported() {
				continue
			}

			// Check if this symbol belongs to the target module
			pkgPath := ""
			switch o := obj.(type) {
			case *types.Func:
				if o.Pkg() != nil {
					pkgPath = o.Pkg().Path()
				}
			case *types.TypeName:
				if o.Pkg() != nil {
					pkgPath = o.Pkg().Path()
				}
			case *types.Var:
				if o.Pkg() != nil {
					pkgPath = o.Pkg().Path()
				}
			}

			if usage.Imports[pkgPath] {
				symbolName := obj.Name()
				pos := pkg.Fset.Position(ident.Pos())
				usage.Symbols[symbolName] = append(usage.Symbols[symbolName], Location{
					File: pos.Filename,
					Line: pos.Line,
				})
			}
		}
	}

	return usage
}

// getDirectDependencies retrieves direct dependencies from go.mod
func (a *Analyzer) getDirectDependencies() ([]string, error) {
	// This is a simplified implementation
	// In production, you'd parse go.mod properly
	var deps []string
	for _, pkg := range a.pkgs {
		for _, imp := range pkg.Imports {
			if imp.Module != nil && imp.Module.Path != "" {
				deps = append(deps, imp.Module.Path)
			}
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, dep := range deps {
		if !seen[dep] {
			unique = append(unique, dep)
			seen[dep] = true
		}
	}

	return unique, nil
}

// extractModulePath extracts the module path from an import path
func extractModulePath(importPath string) string {
	// Simplified: in production, you'd need proper module resolution
	// This works for most cases
	return importPath
}
