package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Allow overriding in tests
var (
	packagesLoad        = packages.Load
	packagesPrintErrors = packages.PrintErrors
	goModDownload       = downloadModule
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

	// Find usage of the dependency in the project
	usage := a.findUsage(upgrade.Module)
	pkgPaths := make([]string, 0, len(usage.Imports))
	for pkgPath := range usage.Imports {
		pkgPaths = append(pkgPaths, pkgPath)
	}
	sort.Strings(pkgPaths)

	// Load API surface of the imported packages for old and new versions
	oldAPI, err := a.loadModuleAPI(upgrade.Module, upgrade.OldVersion, pkgPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to load old API: %w", err)
	}

	newAPI, err := a.loadModuleAPI(upgrade.Module, upgrade.NewVersion, pkgPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to load new API: %w", err)
	}
	// Resolve queries such as "latest" to the concrete version analyzed
	upgrade.NewVersion = newAPI.Version

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
	sort.Strings(unused)

	return unused, nil
}

// loadProject loads the Go packages for the project
func (a *Analyzer) loadProject() error {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports |
			packages.NeedDeps | packages.NeedTypes | packages.NeedSyntax |
			packages.NeedTypesInfo | packages.NeedModule,
		Dir: a.projectPath,
		// Test files use dependencies too - often exclusively, as with
		// testify - and break just as hard when those dependencies change
		Tests: true,
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

// moduleInfo is the subset of `go mod download -json` output we rely on
type moduleInfo struct {
	Path    string
	Version string
	Dir     string
	Error   string
}

// downloadModule fetches module@version into the module cache and reports
// where it lives. workDir must not be inside another module.
func downloadModule(module, version, workDir string) (*moduleInfo, error) {
	spec := fmt.Sprintf("%s@%s", module, version)
	cmd := exec.Command("go", "mod", "download", "-json", spec)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	var info moduleInfo
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		if runErr != nil {
			return nil, fmt.Errorf("go mod download %s: %v: %s", spec, runErr, strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("go mod download %s: invalid output: %w", spec, err)
	}
	if info.Error != "" {
		return nil, fmt.Errorf("go mod download %s: %s", spec, info.Error)
	}
	if runErr != nil {
		return nil, fmt.Errorf("go mod download %s: %v: %s", spec, runErr, strings.TrimSpace(stderr.String()))
	}
	if info.Dir == "" {
		return nil, fmt.Errorf("go mod download %s: no module directory reported", spec)
	}
	return &info, nil
}

// writeModFile prepares a writable go.mod/go.sum copy in workDir so the
// module can be loaded from the read-only module cache via -modfile.
func writeModFile(module, moduleDir, workDir string) (string, error) {
	modFile := filepath.Join(workDir, "go.mod")
	content, err := os.ReadFile(filepath.Join(moduleDir, "go.mod"))
	if os.IsNotExist(err) {
		// Pre-modules code: synthesize a minimal go.mod
		content = []byte(fmt.Sprintf("module %s\n", module))
	} else if err != nil {
		return "", err
	}
	if err := os.WriteFile(modFile, content, 0o644); err != nil {
		return "", err
	}

	sum, err := os.ReadFile(filepath.Join(moduleDir, "go.sum"))
	if err == nil {
		err = os.WriteFile(filepath.Join(workDir, "go.sum"), sum, 0o644)
	} else if os.IsNotExist(err) {
		err = nil
	}
	return modFile, err
}

// loadModuleAPI loads the exported API surface of the given packages of
// module@version. Packages that do not exist in that version are skipped, so
// their symbols show up as removed when diffed against a version that has them.
func (a *Analyzer) loadModuleAPI(module, version string, pkgPaths []string) (*API, error) {
	api := &API{
		Version:    version,
		Funcs:      make(map[string]*Function),
		Types:      make(map[string]*Type),
		Interfaces: make(map[string]*Interface),
	}

	workDir, err := os.MkdirTemp("", "go-semver-audit-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	info, err := goModDownload(module, version, workDir)
	if err != nil {
		return nil, err
	}
	api.Version = info.Version
	spec := fmt.Sprintf("%s@%s", module, info.Version)

	var patterns []string
	for _, pkgPath := range pkgPaths {
		rel := strings.TrimPrefix(strings.TrimPrefix(pkgPath, module), "/")
		if fi, err := os.Stat(filepath.Join(info.Dir, filepath.FromSlash(rel))); err != nil || !fi.IsDir() {
			continue
		}
		pattern := "."
		if rel != "" {
			pattern = "./" + rel
		}
		patterns = append(patterns, pattern)
	}
	if len(patterns) == 0 {
		return api, nil
	}

	modFile, err := writeModFile(module, info.Dir, workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare go.mod for %s: %w", spec, err)
	}

	cfg := &packages.Config{
		Mode:       packages.NeedName | packages.NeedTypes,
		Dir:        info.Dir,
		BuildFlags: []string{"-mod=mod", "-modfile=" + modFile},
		Env:        append(os.Environ(), "GOWORK=off", "GOFLAGS="),
	}

	pkgs, err := packagesLoad(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load module %s: %w", spec, err)
	}

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			// Never diff a partially loaded API: that silently hides breakages
			return nil, fmt.Errorf("failed to load package %s from %s: %v", pkg.PkgPath, spec, pkg.Errors[0])
		}
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
				api.Funcs[symbolKey(pkg.PkgPath, obj.Name())] = &Function{
					Name:      obj.Name(),
					Signature: sig.String(),
					PkgPath:   pkg.PkgPath,
					PkgName:   pkg.Name,
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
						methods[i] = interfaceMethodString(iface.Method(i))
					}
					api.Interfaces[symbolKey(pkg.PkgPath, obj.Name())] = &Interface{
						Name:    obj.Name(),
						Methods: methods,
						PkgPath: pkg.PkgPath,
						PkgName: pkg.Name,
					}
				} else {
					// Regular type
					api.Types[symbolKey(pkg.PkgPath, obj.Name())] = &Type{
						Name:    obj.Name(),
						Kind:    named.Underlying().String(),
						PkgPath: pkg.PkgPath,
						PkgName: pkg.Name,
					}

					// Add methods for this type
					for i := 0; i < named.NumMethods(); i++ {
						method := named.Method(i)
						if method.Exported() {
							sig := method.Type().(*types.Signature)
							api.Funcs[methodSymbolKey(pkg.PkgPath, obj.Name(), method.Name())] = &Function{
								Name:      method.Name(),
								Recv:      obj.Name(),
								Signature: sig.String(),
								PkgPath:   pkg.PkgPath,
								PkgName:   pkg.Name,
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

	// Collect every package of the target module that the project imports
	// before scanning, so the scan does not depend on package ordering
	for _, pkg := range a.pkgs {
		for _, imp := range pkg.Imports {
			if imp.Module != nil && imp.Module.Path == module {
				usage.Imports[imp.PkgPath] = true
			}
		}
	}

	for _, pkg := range a.pkgs {
		// Scan for symbol usage in the package
		if pkg.TypesInfo == nil {
			continue
		}

		for ident, obj := range pkg.TypesInfo.Uses {
			if obj == nil || !obj.Exported() {
				continue
			}

			keys := usageKeys(obj, usage.Imports)
			if len(keys) == 0 {
				continue
			}

			pos := pkg.Fset.Position(ident.Pos())
			loc := Location{File: a.relativePath(pos.Filename), Line: pos.Line}
			for _, key := range keys {
				usage.Symbols[key] = append(usage.Symbols[key], loc)
			}
		}
	}

	for key, locations := range usage.Symbols {
		// TypesInfo.Uses is a map, so locations arrive in random order
		sort.Slice(locations, func(i, j int) bool {
			if locations[i].File != locations[j].File {
				return locations[i].File < locations[j].File
			}
			return locations[i].Line < locations[j].Line
		})
		// With tests loaded, a package's regular files are type-checked twice
		// (once on their own, once in the test variant), so drop the repeats
		usage.Symbols[key] = slices.Compact(locations)
	}

	return usage
}

// relativePath reports a file relative to the project root with forward
// slashes, so reports read the same on every OS and in PR comments. Both the
// path as given and its symlink-resolved form are tried, because the go
// command may report files under either (e.g. /var vs /private/var on macOS).
func (a *Analyzer) relativePath(file string) string {
	if a.projectPath == "" || !filepath.IsAbs(file) {
		return filepath.ToSlash(file)
	}

	roots := []string{a.projectPath}
	if resolved, err := filepath.EvalSymlinks(a.projectPath); err == nil && resolved != a.projectPath {
		roots = append(roots, resolved)
	}
	for _, root := range roots {
		rel, err := filepath.Rel(root, file)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(file)
}

// usageKeys returns the symbol keys a used object should be recorded under,
// restricted to packages of the module being analyzed.
//
// Using a method also records usage of its receiver type: code that only ever
// does cfg := lib.New(); cfg.Validate() never names Config itself, but still
// breaks if Config is removed.
func usageKeys(obj types.Object, moduleImports map[string]bool) []string {
	switch o := obj.(type) {
	case *types.Func:
		sig, _ := o.Type().(*types.Signature)
		if sig == nil || sig.Recv() == nil {
			return packageLevelKeys(o, moduleImports)
		}

		recv := receiverNamed(sig.Recv().Type())
		if recv == nil || recv.Obj().Pkg() == nil {
			return nil
		}
		pkgPath := recv.Obj().Pkg().Path()
		if !moduleImports[pkgPath] {
			return nil
		}
		recvName := recv.Obj().Name()
		return []string{
			methodSymbolKey(pkgPath, recvName, o.Name()),
			symbolKey(pkgPath, recvName),
		}

	case *types.TypeName:
		return packageLevelKeys(o, moduleImports)

	case *types.Var:
		// Struct fields are not part of the API surface we diff yet, and
		// recording them under their bare name produces false matches
		// against package-level symbols
		if o.IsField() {
			return nil
		}
		return packageLevelKeys(o, moduleImports)
	}

	return nil
}

// packageLevelKeys returns the single key for a package-level symbol, or nil if
// the symbol does not belong to the module being analyzed.
func packageLevelKeys(obj types.Object, moduleImports map[string]bool) []string {
	if obj.Pkg() == nil || !moduleImports[obj.Pkg().Path()] {
		return nil
	}
	return []string{symbolKey(obj.Pkg().Path(), obj.Name())}
}

// receiverNamed unwraps a method receiver type to its named type, looking
// through pointers, aliases and generic instantiations.
func receiverNamed(t types.Type) *types.Named {
	t = types.Unalias(t)
	if ptr, ok := t.(*types.Pointer); ok {
		t = types.Unalias(ptr.Elem())
	}
	named, _ := t.(*types.Named)
	return named
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
