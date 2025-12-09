package analyzer

import (
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestNewAnalyzer(t *testing.T) {
	tmp := t.TempDir()

	a, err := New(tmp)
	if err != nil {
		t.Fatalf("New() returned error for existing path: %v", err)
	}

	want, err := filepath.Abs(tmp)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	if a.projectPath != want {
		t.Fatalf("New() projectPath = %s, want %s", a.projectPath, want)
	}

	missing := filepath.Join(tmp, "missing", "project")
	if _, err := New(missing); err == nil {
		t.Fatalf("New() expected error for missing path")
	}
}

func TestGetCurrentVersion(t *testing.T) {
	const module = "example.com/lib"

	a := &Analyzer{
		pkgs: []*packages.Package{
			{
				Module: &packages.Module{Path: module, Version: "v1.2.3"},
			},
		},
	}

	version, err := a.getCurrentVersion(module)
	if err != nil {
		t.Fatalf("getCurrentVersion() error = %v", err)
	}
	if version != "v1.2.3" {
		t.Fatalf("getCurrentVersion() = %s, want %s", version, "v1.2.3")
	}

	// Should fall back to dependency modules
	a.pkgs = []*packages.Package{
		{
			Module: &packages.Module{Path: "example.com/user", Version: "v0.0.0"},
			Imports: map[string]*packages.Package{
				"example.com/lib/pkg": {
					Module: &packages.Module{Path: module, Version: "v0.9.0"},
				},
			},
		},
	}

	version, err = a.getCurrentVersion(module)
	if err != nil {
		t.Fatalf("getCurrentVersion() dependency error = %v", err)
	}
	if version != "v0.9.0" {
		t.Fatalf("getCurrentVersion() dependency = %s, want %s", version, "v0.9.0")
	}
}

func TestGetDirectDependencies(t *testing.T) {
	a := &Analyzer{
		pkgs: []*packages.Package{
			{
				Imports: map[string]*packages.Package{
					"example.com/a/pkg": {Module: &packages.Module{Path: "example.com/a"}},
					"example.com/b/pkg": {Module: &packages.Module{Path: "example.com/b"}},
				},
			},
			{
				Imports: map[string]*packages.Package{
					"example.com/a/other": {Module: &packages.Module{Path: "example.com/a"}},
				},
			},
		},
	}

	deps, err := a.getDirectDependencies()
	if err != nil {
		t.Fatalf("getDirectDependencies() error = %v", err)
	}

	want := []string{"example.com/a", "example.com/b"}
	if len(deps) != len(want) {
		t.Fatalf("getDirectDependencies() count = %d, want %d", len(deps), len(want))
	}

	if !containsAll(deps, want) {
		t.Fatalf("getDirectDependencies() = %v, want %v", deps, want)
	}
}

func TestFindUnusedDependencies(t *testing.T) {
	a := &Analyzer{
		pkgs: []*packages.Package{
			{
				Imports: map[string]*packages.Package{
					"example.com/a": {PkgPath: "example.com/a", Module: &packages.Module{Path: "example.com/a"}},
					"example.com/b": {PkgPath: "example.com/b", Module: &packages.Module{Path: "example.com/b"}},
					// PkgPath intentionally empty so it is never marked as imported
					"example.com/c": {PkgPath: "", Module: &packages.Module{Path: "example.com/c"}},
				},
			},
		},
	}

	unused, err := a.FindUnusedDependencies()
	if err != nil {
		t.Fatalf("FindUnusedDependencies() error = %v", err)
	}

	if !reflect.DeepEqual(unused, []string{"example.com/c"}) {
		t.Fatalf("FindUnusedDependencies() = %v, want [example.com/c]", unused)
	}
}

func TestFindUsage(t *testing.T) {
	const module = "example.com/lib"

	fset := token.NewFileSet()
	userFile := fset.AddFile("main.go", -1, 50)
	ident := ast.NewIdent("Foo")
	ident.NamePos = userFile.Pos(5)

	libPkg := types.NewPackage(module, "lib")
	sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false)
	obj := types.NewFunc(token.NoPos, libPkg, "Foo", sig)

	pkg := &packages.Package{
		PkgPath: "example.com/user",
		Fset:    fset,
		Imports: map[string]*packages.Package{
			module: {PkgPath: module, Module: &packages.Module{Path: module}},
		},
		TypesInfo: &types.Info{
			Uses: map[*ast.Ident]types.Object{
				ident:               obj,
				ast.NewIdent("bar"): types.NewFunc(token.NoPos, libPkg, "bar", sig), // not exported
			},
		},
	}

	a := &Analyzer{pkgs: []*packages.Package{pkg}}
	usage := a.findUsage(module)

	if !usage.Imports[module] {
		t.Fatalf("findUsage() missing import entry for %s", module)
	}

	locations := usage.Symbols["Foo"]
	if len(locations) != 1 {
		t.Fatalf("findUsage() expected 1 location for Foo, got %d", len(locations))
	}
	if locations[0].File != "main.go" || locations[0].Line == 0 {
		t.Fatalf("findUsage() returned unexpected location %+v", locations[0])
	}

	if _, ok := usage.Symbols["bar"]; ok {
		t.Fatalf("findUsage() should ignore non-exported symbols")
	}
}

func TestLoadModuleAPI(t *testing.T) {
	restore := mockPackagesLoad(func(cfg *packages.Config, patterns ...string) ([]*packages.Package, error) {
		return []*packages.Package{buildAPIPackage("example.com/lib")}, nil
	})
	defer restore()

	a := &Analyzer{projectPath: "."}
	api, err := a.loadModuleAPI("example.com/lib", "v1.0.0")
	if err != nil {
		t.Fatalf("loadModuleAPI() error = %v", err)
	}

	if api.Funcs["Func"] == nil {
		t.Fatalf("loadModuleAPI() missing exported function")
	}
	if api.Types["Thing"] == nil {
		t.Fatalf("loadModuleAPI() missing exported type")
	}
	if api.Interfaces["Handler"] == nil {
		t.Fatalf("loadModuleAPI() missing exported interface")
	}
	if api.Funcs["Thing.Do"] == nil || !api.Funcs["Thing.Do"].IsMethod {
		t.Fatalf("loadModuleAPI() missing method binding")
	}
}

func TestAnalyzeWithMockLoader(t *testing.T) {
	const module = "example.com/lib"

	// Packages representing the user's project
	projectPkg := buildUsagePackage(module)

	// Old API includes OldFunc and a simple Handler interface
	oldAPIPkg := buildAPIPackageWithChanges(module, apiDefinition{
		funcs: map[string]*types.Signature{
			"OldFunc": newSignature(nil, nil),
			"Parse":   newSignature([]*types.Var{types.NewVar(token.NoPos, types.NewPackage(module, "lib"), "p", types.Typ[types.String])}, []*types.Var{types.NewVar(token.NoPos, types.NewPackage(module, "lib"), "", types.Typ[types.Bool])}),
		},
		interfaces: map[string][]*types.Func{
			"Handler": {types.NewFunc(token.NoPos, types.NewPackage(module, "lib"), "Handle", newSignature(nil, nil))},
		},
	})

	// New API removes OldFunc and changes Parse signature / Handler
	newAPIPkg := buildAPIPackageWithChanges(module, apiDefinition{
		funcs: map[string]*types.Signature{
			"Parse": newSignature([]*types.Var{
				types.NewVar(token.NoPos, types.NewPackage(module, "lib"), "p", types.Typ[types.String]),
				types.NewVar(token.NoPos, types.NewPackage(module, "lib"), "n", types.Typ[types.Int]),
			}, []*types.Var{types.NewVar(token.NoPos, types.NewPackage(module, "lib"), "", types.Typ[types.Bool])}),
		},
		interfaces: map[string][]*types.Func{
			"Handler": {
				types.NewFunc(token.NoPos, types.NewPackage(module, "lib"), "HandleWithContext", newSignature(nil, nil)),
			},
		},
	})

	restore := mockPackagesLoad(func(cfg *packages.Config, patterns ...string) ([]*packages.Package, error) {
		switch patterns[0] {
		case "./...":
			return []*packages.Package{projectPkg}, nil
		case module + "@v1.0.0":
			return []*packages.Package{oldAPIPkg}, nil
		case module + "@v2.0.0":
			return []*packages.Package{newAPIPkg}, nil
		default:
			return nil, nil
		}
	})
	defer restore()

	a := &Analyzer{projectPath: "."}
	result, err := a.Analyze(&Upgrade{Module: module, NewVersion: "v2.0.0"})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if result.OldVersion != "v1.0.0" {
		t.Fatalf("Analyze() OldVersion = %s, want v1.0.0", result.OldVersion)
	}
	if len(result.Changes.Removed) == 0 {
		t.Fatalf("Analyze() expected removed symbols")
	}
	if len(result.Changes.Changed) == 0 {
		t.Fatalf("Analyze() expected changed signatures")
	}
	if len(result.Changes.InterfaceChanges) == 0 {
		t.Fatalf("Analyze() expected interface changes")
	}
}

func TestAnalyzeFailsWhenProjectCannotLoad(t *testing.T) {
	restoreLoad := mockPackagesLoad(func(cfg *packages.Config, patterns ...string) ([]*packages.Package, error) {
		return nil, errors.New("load failure")
	})
	defer restoreLoad()
	restorePrint := mockPackagesPrintErrors(func(pkgs []*packages.Package) int { return 0 })
	defer restorePrint()

	a := &Analyzer{projectPath: "."}
	_, err := a.Analyze(&Upgrade{Module: "example.com/lib", NewVersion: "v1.0.0"})
	if err == nil || !strings.Contains(err.Error(), "failed to load project") {
		t.Fatalf("Analyze() expected load project error, got %v", err)
	}
}

func TestAnalyzeFailsWhenModuleVersionMissing(t *testing.T) {
	restoreLoad := mockPackagesLoad(func(cfg *packages.Config, patterns ...string) ([]*packages.Package, error) {
		return []*packages.Package{
			{
				PkgPath: "example.com/app",
				Module:  &packages.Module{Path: "example.com/other", Version: "v0.1.0"},
				Imports: map[string]*packages.Package{},
			},
		}, nil
	})
	defer restoreLoad()
	restorePrint := mockPackagesPrintErrors(func(pkgs []*packages.Package) int { return 0 })
	defer restorePrint()

	a := &Analyzer{projectPath: "."}
	_, err := a.Analyze(&Upgrade{Module: "example.com/missing", NewVersion: "v2.0.0"})
	if err == nil || !strings.Contains(err.Error(), "module example.com/missing not found") {
		t.Fatalf("Analyze() expected missing module error, got %v", err)
	}
}

func TestLoadProjectFailsOnPackageErrors(t *testing.T) {
	restoreLoad := mockPackagesLoad(func(cfg *packages.Config, patterns ...string) ([]*packages.Package, error) {
		return []*packages.Package{{PkgPath: "example.com/app"}}, nil
	})
	defer restoreLoad()
	restorePrint := mockPackagesPrintErrors(func(pkgs []*packages.Package) int { return 1 })
	defer restorePrint()

	a := &Analyzer{projectPath: "."}
	if err := a.loadProject(); err == nil {
		t.Fatalf("loadProject() expected error due to package errors")
	}
}

func TestFindUnusedDependenciesLoadProjectError(t *testing.T) {
	restoreLoad := mockPackagesLoad(func(cfg *packages.Config, patterns ...string) ([]*packages.Package, error) {
		return nil, errors.New("load failure")
	})
	defer restoreLoad()
	restorePrint := mockPackagesPrintErrors(func(pkgs []*packages.Package) int { return 0 })
	defer restorePrint()

	a := &Analyzer{projectPath: ".", pkgs: nil}
	if _, err := a.FindUnusedDependencies(); err == nil {
		t.Fatalf("FindUnusedDependencies() expected error when loadProject fails")
	}
}

// --- Helpers ---

func containsAll(have, want []string) bool {
	set := make(map[string]bool)
	for _, v := range have {
		set[v] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

func mockPackagesLoad(fn func(cfg *packages.Config, patterns ...string) ([]*packages.Package, error)) func() {
	origLoad := packagesLoad
	packagesLoad = fn
	return func() {
		packagesLoad = origLoad
	}
}

func mockPackagesPrintErrors(fn func(pkgs []*packages.Package) int) func() {
	origPrint := packagesPrintErrors
	packagesPrintErrors = fn
	return func() {
		packagesPrintErrors = origPrint
	}
}

func buildAPIPackage(pkgPath string) *packages.Package {
	typesPkg := types.NewPackage(pkgPath, "lib")
	scope := typesPkg.Scope()

	// Exported function
	fn := types.NewFunc(token.NoPos, typesPkg, "Func", newSignature(nil, nil))
	scope.Insert(fn)

	// Interface with one method
	ifaceMethod := types.NewFunc(token.NoPos, typesPkg, "Handle", newSignature(nil, nil))
	iface := types.NewInterfaceType([]*types.Func{ifaceMethod}, nil)
	iface.Complete()
	ifaceName := types.NewTypeName(token.NoPos, typesPkg, "Handler", nil)
	ifaceNamed := types.NewNamed(ifaceName, iface, nil)
	scope.Insert(ifaceNamed.Obj())

	// Named type with one method
	typeName := types.NewTypeName(token.NoPos, typesPkg, "Thing", nil)
	named := types.NewNamed(typeName, types.Typ[types.Int], nil)
	recv := types.NewVar(token.NoPos, typesPkg, "t", named)
	method := types.NewFunc(token.NoPos, typesPkg, "Do", newSignatureWithRecv(recv, nil, nil))
	named.AddMethod(method)
	scope.Insert(typeName)

	return &packages.Package{
		PkgPath: pkgPath,
		Types:   typesPkg,
	}
}

type apiDefinition struct {
	funcs      map[string]*types.Signature
	interfaces map[string][]*types.Func
}

func buildAPIPackageWithChanges(pkgPath string, def apiDefinition) *packages.Package {
	typesPkg := types.NewPackage(pkgPath, "lib")
	scope := typesPkg.Scope()

	for name, sig := range def.funcs {
		scope.Insert(types.NewFunc(token.NoPos, typesPkg, name, sig))
	}

	for name, methods := range def.interfaces {
		iface := types.NewInterfaceType(methods, nil)
		iface.Complete()
		tname := types.NewTypeName(token.NoPos, typesPkg, name, nil)
		named := types.NewNamed(tname, iface, nil)
		scope.Insert(named.Obj())
	}

	return &packages.Package{
		PkgPath: pkgPath,
		Types:   typesPkg,
	}
}

func buildUsagePackage(module string) *packages.Package {
	fset := token.NewFileSet()
	file := fset.AddFile("main.go", -1, 20)
	ident := ast.NewIdent("OldFunc")
	ident.NamePos = file.Pos(2)
	parseIdent := ast.NewIdent("Parse")
	parseIdent.NamePos = file.Pos(5)
	handlerIdent := ast.NewIdent("Handler")
	handlerIdent.NamePos = file.Pos(8)

	libPkg := types.NewPackage(module, "lib")
	sig := newSignature(nil, nil)
	obj := types.NewFunc(token.NoPos, libPkg, "OldFunc", sig)
	parseSig := newSignature([]*types.Var{types.NewVar(token.NoPos, libPkg, "p", types.Typ[types.String])}, nil)
	parseObj := types.NewFunc(token.NoPos, libPkg, "Parse", parseSig)
	handlerIface := types.NewInterfaceType(nil, nil)
	handlerIface.Complete()
	handlerObj := types.NewTypeName(token.NoPos, libPkg, "Handler", handlerIface)
	types.NewNamed(handlerObj, handlerIface, nil)

	return &packages.Package{
		PkgPath: "example.com/user",
		Fset:    fset,
		Imports: map[string]*packages.Package{
			module: {
				PkgPath: module,
				Module:  &packages.Module{Path: module, Version: "v1.0.0"},
			},
		},
		TypesInfo: &types.Info{
			Uses: map[*ast.Ident]types.Object{
				ident:        obj,
				parseIdent:   parseObj,
				handlerIdent: handlerObj,
			},
		},
	}
}

func newSignature(params []*types.Var, results []*types.Var) *types.Signature {
	var ptuple, rtuple *types.Tuple
	if len(params) > 0 {
		ptuple = types.NewTuple(params...)
	}
	if len(results) > 0 {
		rtuple = types.NewTuple(results...)
	}
	return types.NewSignatureType(nil, nil, nil, ptuple, rtuple, false)
}

func newSignatureWithRecv(recv *types.Var, params []*types.Var, results []*types.Var) *types.Signature {
	var ptuple, rtuple *types.Tuple
	if len(params) > 0 {
		ptuple = types.NewTuple(params...)
	}
	if len(results) > 0 {
		rtuple = types.NewTuple(results...)
	}
	return types.NewSignatureType(recv, nil, nil, ptuple, rtuple, false)
}
