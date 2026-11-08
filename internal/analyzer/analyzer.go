package analyzer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/build"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/tools/go/packages"
)

// Allow overriding in tests
var (
	packagesLoad        = packages.Load
	packagesPrintErrors = packages.PrintErrors
	goModDownload       = downloadModule
)

// ErrModuleNotImported is returned by Analyze when no package of the project,
// tests included, imports the module being upgraded, so there is no usage to
// check it against.
var ErrModuleNotImported = errors.New("not found in project dependencies")

// Analyzer performs static analysis on Go projects
type Analyzer struct {
	projectPath string
	pkgs        []*packages.Package
	// loaded is set once the project has been type-checked. Upgrades leave
	// the project untouched, so one load serves every module analyzed.
	loaded bool
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
	if err := a.LoadProject(); err != nil {
		return nil, err
	}

	// Get current version from project dependencies. A major version upgrade
	// changes the module path, so the project requires an earlier major
	// version under another path, which is what the new one is compared to.
	upgrade.OldModule = upgrade.Module
	currentVersion, err := a.getCurrentVersion(upgrade.Module)
	if err != nil {
		oldModule, findErr := previousMajor(upgrade.Module, a.dependencyModules())
		if findErr != nil {
			return nil, fmt.Errorf("failed to determine current version: %w", findErr)
		}
		if oldModule == "" {
			return nil, fmt.Errorf("failed to determine current version: %w", err)
		}
		upgrade.OldModule = oldModule
		if currentVersion, err = a.getCurrentVersion(oldModule); err != nil {
			return nil, fmt.Errorf("failed to determine current version: %w", err)
		}
	}
	upgrade.OldVersion = currentVersion

	// Find usage of the dependency in the project. Usage, and every symbol key
	// diffed against it, is in terms of the old module's package paths.
	usage := a.findUsage(upgrade.OldModule)
	pkgPaths := make([]string, 0, len(usage.Imports))
	for pkgPath := range usage.Imports {
		pkgPaths = append(pkgPaths, pkgPath)
	}
	sort.Strings(pkgPaths)

	// Load API surface of the imported packages for old and new versions
	oldAPI, err := a.loadModuleAPI(upgrade.OldModule, upgrade.OldVersion, pkgPaths, upgrade.OldModule)
	if err != nil {
		return nil, fmt.Errorf("failed to load old API: %w", err)
	}

	newAPI, err := a.loadModuleAPI(upgrade.Module, upgrade.NewVersion, pkgPaths, upgrade.OldModule)
	if err != nil {
		return nil, fmt.Errorf("failed to load new API: %w", err)
	}
	// Resolve queries such as "latest" to the concrete version analyzed
	upgrade.NewVersion = newAPI.Version

	// Diff the APIs
	diff := diffAPIs(oldAPI, newAPI, usage)

	result := &Result{
		Module:     upgrade.Module,
		OldVersion: upgrade.OldVersion,
		NewVersion: upgrade.NewVersion,
		Changes:    diff,
	}
	if upgrade.OldModule != upgrade.Module {
		result.OldModule = upgrade.OldModule
	}
	return result, nil
}

// LoadProject type-checks the project unless that was already done. Calling it
// before analyzing several upgrades separates a project that does not build,
// which fails them all, from a failure specific to one module.
func (a *Analyzer) LoadProject() error {
	if a.loaded {
		return nil
	}
	if err := a.loadProject(); err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}
	a.loaded = true
	return nil
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

	return "", fmt.Errorf("module %s %w", module, ErrModuleNotImported)
}

// dependencyModules returns the path of every module the project's packages
// import from
func (a *Analyzer) dependencyModules() []string {
	seen := make(map[string]bool)
	var paths []string
	for _, pkg := range a.pkgs {
		for _, dep := range a.getDependencyModules(pkg) {
			if !seen[dep.Path] {
				seen[dep.Path] = true
				paths = append(paths, dep.Path)
			}
		}
	}
	sort.Strings(paths)
	return paths
}

// previousMajor finds the dependency that target is another major version of:
// the one whose path differs from target only in its major version suffix
// (example.com/lib and example.com/lib/v2, or gopkg.in/yaml.v2 and
// gopkg.in/yaml.v3). It returns "" if there is none.
func previousMajor(target string, deps []string) (string, error) {
	prefix, _, ok := module.SplitPathVersion(target)
	if !ok {
		return "", nil
	}
	if err := module.CheckPath(target); err != nil {
		return "", fmt.Errorf("invalid module path: %w", err)
	}

	var matches []string
	for _, dep := range deps {
		if dep == target {
			continue
		}
		if depPrefix, _, ok := module.SplitPathVersion(dep); ok && depPrefix == prefix {
			matches = append(matches, dep)
		}
	}
	if len(matches) > 1 {
		// Each major version is a separate module the project may use side
		// by side; guessing would leave the usage of the others unchecked
		return "", fmt.Errorf("module %s not found in project dependencies, and it could replace any of %s",
			target, strings.Join(matches, ", "))
	}
	if len(matches) == 0 {
		return "", nil
	}
	return matches[0], nil
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

// hasGoPackage reports whether dir holds an importable package in this version
// of a module. The directory existing is not enough: a module's root package
// can be removed while its subpackages keep the directory alive, and a package
// directory can be left with only tests or files excluded by build constraints.
// Other problems are left for packages.Load to report.
func hasGoPackage(dir string) bool {
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return false
	}

	pkg, err := build.ImportDir(dir, 0)
	var noGo *build.NoGoError
	if errors.As(err, &noGo) {
		return false
	}
	if err == nil && len(pkg.GoFiles)+len(pkg.CgoFiles) == 0 {
		return false
	}
	return true
}

// loadModuleAPI loads the exported API surface of the given packages of
// modulePath@version. Packages that do not exist in that version are skipped,
// so their symbols show up as removed when diffed against a version that has
// them.
//
// pkgPaths, and the symbol keys and package paths of the returned API, are in
// terms of keyModule, the module the project imports today. For a major
// version upgrade modulePath is the new module and keyModule the old one:
// example.com/lib/sub is loaded from example.com/lib/v2/sub and its symbols
// keyed as example.com/lib/sub, so both versions' symbols line up with each
// other and with the project's usage. Display strings keep the real paths.
func (a *Analyzer) loadModuleAPI(modulePath, version string, pkgPaths []string, keyModule string) (*API, error) {
	api := &API{
		Version:    version,
		Funcs:      make(map[string]*Function),
		Types:      make(map[string]*Type),
		Interfaces: make(map[string]*Interface),
		Values:     make(map[string]*Value),
		Fields:     make(map[string]*Field),
	}

	workDir, err := os.MkdirTemp("", "go-semver-audit-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	info, err := goModDownload(modulePath, version, workDir)
	if err != nil {
		return nil, err
	}
	api.Version = info.Version
	spec := fmt.Sprintf("%s@%s", modulePath, info.Version)

	var keys keyRenderer
	if modulePath != keyModule {
		keys.rename = func(path string) string { return rebasePath(path, modulePath, keyModule) }
	}

	var patterns []string
	for _, pkgPath := range pkgPaths {
		rel := strings.TrimPrefix(strings.TrimPrefix(pkgPath, keyModule), "/")
		if !hasGoPackage(filepath.Join(info.Dir, filepath.FromSlash(rel))) {
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

	modFile, err := writeModFile(modulePath, info.Dir, workDir)
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
		qualifier := packageQualifier(modulePath, pkg.PkgPath, pkg.Name)
		pkgPath := keys.pkgPath(pkg.PkgPath)
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if !obj.Exported() {
				continue
			}

			switch obj := obj.(type) {
			case *types.Func:
				sig := obj.Type().(*types.Signature)
				api.Funcs[symbolKey(pkgPath, obj.Name())] = &Function{
					Name:         obj.Name(),
					Signature:    sig.String(),
					SignatureKey: keys.signatureKey(sig),
					PkgPath:      pkgPath,
					PkgName:      qualifier,
				}

			case *types.Var, *types.Const:
				kind := "var"
				if _, ok := obj.(*types.Const); ok {
					kind = "const"
				}
				api.Values[symbolKey(pkgPath, obj.Name())] = &Value{
					Name:    obj.Name(),
					Kind:    kind,
					Type:    obj.Type().String(),
					TypeKey: keys.typeKey(obj.Type()),
					PkgPath: pkgPath,
					PkgName: qualifier,
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
					methodKeys := make([]string, iface.NumMethods())
					for i := 0; i < iface.NumMethods(); i++ {
						methods[i] = interfaceMethodString(iface.Method(i))
						methodKeys[i] = keys.interfaceMethodKey(iface.Method(i))
					}
					api.Interfaces[symbolKey(pkgPath, obj.Name())] = &Interface{
						Name:       obj.Name(),
						Methods:    methods,
						MethodKeys: methodKeys,
						PkgPath:    pkgPath,
						PkgName:    qualifier,
					}
				} else {
					// Regular type
					api.Types[symbolKey(pkgPath, obj.Name())] = &Type{
						Name:          obj.Name(),
						Kind:          named.Underlying().String(),
						UnderlyingKey: keys.underlyingKey(named),
						PkgPath:       pkgPath,
						PkgName:       qualifier,
					}

					if st, ok := named.Underlying().(*types.Struct); ok {
						for i := 0; i < st.NumFields(); i++ {
							field := st.Field(i)
							if !field.Exported() {
								continue
							}
							api.Fields[methodSymbolKey(pkgPath, obj.Name(), field.Name())] = &Field{
								Name:    field.Name(),
								Recv:    obj.Name(),
								Type:    field.Type().String(),
								TypeKey: keys.typeKey(field.Type()),
								PkgPath: pkgPath,
								PkgName: qualifier,
							}
						}
					}

					// Add methods for this type
					for i := 0; i < named.NumMethods(); i++ {
						method := named.Method(i)
						if method.Exported() {
							sig := method.Type().(*types.Signature)
							api.Funcs[methodSymbolKey(pkgPath, obj.Name(), method.Name())] = &Function{
								Name:         method.Name(),
								Recv:         obj.Name(),
								Signature:    sig.String(),
								SignatureKey: keys.signatureKey(sig),
								PkgPath:      pkgPath,
								PkgName:      qualifier,
								IsMethod:     true,
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
		Symbols:         make(map[string][]Location),
		Imports:         make(map[string]bool),
		Implementations: make(map[string][]Location),
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

	owners := fieldOwners(a.pkgs, usage.Imports)

	for _, pkg := range a.pkgs {
		// Scan for symbol usage in the package
		if pkg.TypesInfo == nil {
			continue
		}

		for ident, obj := range pkg.TypesInfo.Uses {
			if obj == nil || !obj.Exported() {
				continue
			}

			keys := usageKeys(obj, usage.Imports, owners)
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

	a.findImplementations(module, usage)

	for key, locations := range usage.Symbols {
		usage.Symbols[key] = sortLocations(locations)
	}
	for key, locations := range usage.Implementations {
		usage.Implementations[key] = sortLocations(locations)
	}

	return usage
}

// sortLocations sorts locations and drops repeats. TypesInfo maps are
// iterated in random order, and with tests loaded a package's regular files
// are type-checked twice (once on their own, once in the test variant).
func sortLocations(locations []Location) []Location {
	sort.Slice(locations, func(i, j int) bool {
		if locations[i].File != locations[j].File {
			return locations[i].File < locations[j].File
		}
		return locations[i].Line < locations[j].Line
	})
	return slices.Compact(locations)
}

// findImplementations records, for each exported interface of the module's
// imported packages, the project types that implement it. Adding a method to
// an interface only breaks those types; code that merely calls through the
// interface keeps compiling.
//
// A type counts only if it declares at least one of the interface's methods
// itself. One that gets them all by embedding the interface, or a type of the
// module, picks up added methods automatically.
func (a *Analyzer) findImplementations(module string, usage *Usage) {
	inModule := func(pkg *types.Package) bool {
		return pkg != nil && (pkg.Path() == module || strings.HasPrefix(pkg.Path(), module+"/"))
	}

	type moduleIface struct {
		key   string
		iface *types.Interface
	}
	var ifaces []moduleIface
	seen := make(map[*types.Package]bool)
	for _, pkg := range a.pkgs {
		for _, imp := range pkg.Imports {
			if imp.Types == nil || !usage.Imports[imp.PkgPath] || seen[imp.Types] {
				continue
			}
			seen[imp.Types] = true

			scope := imp.Types.Scope()
			for _, name := range scope.Names() {
				tn, ok := scope.Lookup(name).(*types.TypeName)
				if !ok || !tn.Exported() || tn.IsAlias() {
					continue
				}
				iface, ok := tn.Type().Underlying().(*types.Interface)
				if !ok || iface.NumMethods() == 0 || !iface.IsMethodSet() {
					continue
				}
				ifaces = append(ifaces, moduleIface{symbolKey(imp.PkgPath, tn.Name()), iface})
			}
		}
	}
	if len(ifaces) == 0 {
		return
	}

	for _, pkg := range a.pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		// Defs rather than the package scope, so types declared inside
		// functions are found too; they cannot declare methods, but can get
		// them by embedding a project type
		for ident, obj := range pkg.TypesInfo.Defs {
			tn, ok := obj.(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			named, ok := tn.Type().(*types.Named)
			if !ok || named.TypeParams().Len() > 0 || types.IsInterface(named) {
				continue
			}
			ptr := types.NewPointer(named)

			for _, mi := range ifaces {
				if !types.Implements(named, mi.iface) && !types.Implements(ptr, mi.iface) {
					continue
				}
				if !declaresAnyMethod(ptr, mi.iface, inModule) {
					continue
				}
				pos := pkg.Fset.Position(ident.Pos())
				usage.Implementations[mi.key] = append(usage.Implementations[mi.key],
					Location{File: a.relativePath(pos.Filename), Line: pos.Line})
			}
		}
	}
}

// declaresAnyMethod reports whether t has at least one of iface's methods
// declared on a concrete type outside the module, rather than promoted from an
// embedded interface or an embedded type of the module.
func declaresAnyMethod(t types.Type, iface *types.Interface, inModule func(*types.Package) bool) bool {
	for i := 0; i < iface.NumMethods(); i++ {
		m := iface.Method(i)
		obj, _, _ := types.LookupFieldOrMethod(t, true, m.Pkg(), m.Name())
		fn, ok := obj.(*types.Func)
		if !ok {
			continue
		}
		recv := fn.Type().(*types.Signature).Recv()
		if recv == nil || types.IsInterface(recv.Type()) {
			continue
		}
		if named := receiverNamed(recv.Type()); named != nil && !inModule(named.Obj().Pkg()) {
			return true
		}
	}
	return false
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

// fieldOwner identifies the exported struct type a field belongs to
type fieldOwner struct {
	pkgPath, typeName string
}

// fieldOwners maps each exported field of the exported struct types in the
// module's imported packages to the type that declares it. A *types.Var for a
// field does not know its struct, so usage of cfg.Name could not otherwise be
// told apart from usage of another type's Name.
func fieldOwners(pkgs []*packages.Package, moduleImports map[string]bool) map[*types.Var]fieldOwner {
	owners := make(map[*types.Var]fieldOwner)
	seen := make(map[*types.Package]bool)
	for _, pkg := range pkgs {
		for _, imp := range pkg.Imports {
			if imp.Types == nil || !moduleImports[imp.PkgPath] || seen[imp.Types] {
				continue
			}
			seen[imp.Types] = true

			scope := imp.Types.Scope()
			for _, name := range scope.Names() {
				tn, ok := scope.Lookup(name).(*types.TypeName)
				if !ok || !tn.Exported() || tn.IsAlias() {
					continue
				}
				st, ok := tn.Type().Underlying().(*types.Struct)
				if !ok {
					continue
				}
				for i := 0; i < st.NumFields(); i++ {
					if field := st.Field(i); field.Exported() {
						owners[field] = fieldOwner{imp.PkgPath, tn.Name()}
					}
				}
			}
		}
	}
	return owners
}

// usageKeys returns the symbol keys a used object should be recorded under,
// restricted to packages of the module being analyzed.
//
// Using a method or a field also records usage of its type: code that only
// ever does cfg := lib.New(); cfg.Validate() never names Config itself, but
// still breaks if Config is removed.
func usageKeys(obj types.Object, moduleImports map[string]bool, owners map[*types.Var]fieldOwner) []string {
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
		if o.IsField() {
			// Fields of an instantiated generic struct are copies; the
			// declared field is their origin
			owner, ok := owners[o.Origin()]
			if !ok {
				return nil
			}
			return []string{
				methodSymbolKey(owner.pkgPath, owner.typeName, o.Name()),
				symbolKey(owner.pkgPath, owner.typeName),
			}
		}
		return packageLevelKeys(o, moduleImports)

	case *types.Const:
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
