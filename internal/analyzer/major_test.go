package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestRebasePath(t *testing.T) {
	tests := []struct {
		path, from, to, want string
	}{
		{"example.com/lib", "example.com/lib", "example.com/lib/v2", "example.com/lib/v2"},
		{"example.com/lib/sub", "example.com/lib", "example.com/lib/v2", "example.com/lib/v2/sub"},
		{"example.com/lib/v2/sub/deep", "example.com/lib/v2", "example.com/lib", "example.com/lib/sub/deep"},
		{"gopkg.in/yaml.v3", "gopkg.in/yaml.v3", "gopkg.in/yaml.v2", "gopkg.in/yaml.v2"},
		// Only whole path elements belong to a module
		{"example.com/library", "example.com/lib", "example.com/lib/v2", "example.com/library"},
		{"context", "example.com/lib", "example.com/lib/v2", "context"},
	}
	for _, tt := range tests {
		if got := rebasePath(tt.path, tt.from, tt.to); got != tt.want {
			t.Errorf("rebasePath(%q, %q, %q) = %q, want %q", tt.path, tt.from, tt.to, got, tt.want)
		}
	}
}

// checkPackage type-checks src as the package at path
func checkPackage(t *testing.T, path, src string) *types.Package {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	pkg, err := (&types.Config{}).Check(path, fset, []*ast.File{file}, nil)
	if err != nil {
		t.Fatalf("type-check %s: %v", path, err)
	}
	return pkg
}

// TestKeyRendererRename checks that the same declarations in two major
// versions of a module produce the same comparison keys once the new module's
// paths are renamed to the old one's - including inside type arguments and
// constraints with type terms, which render through types.TypeString - and
// that a real change still shows.
func TestKeyRendererRename(t *testing.T) {
	const src = `package lib

type Config struct{ Name string }

type ID int

type Getter interface{ Get(c *Config) ID }

type Box[T any] struct{ V T }

func F[T interface{ ~string | ID }](c *Config, m map[ID][]Config, b Box[*Config], cb func(Getter) error) (T, *Config) {
	var zero T
	return zero, c
}

func G(c Config) {}
`
	oldPkg := checkPackage(t, "example.com/lib", src)
	newPkg := checkPackage(t, "example.com/lib/v2", src)
	changedPkg := checkPackage(t, "example.com/lib/v2", strings.Replace(src, "func G(c Config)", "func G(c *Config)", 1))

	var plain keyRenderer
	renamed := keyRenderer{rename: func(path string) string {
		return rebasePath(path, "example.com/lib/v2", "example.com/lib")
	}}
	sig := func(pkg *types.Package, name string) *types.Signature {
		return pkg.Scope().Lookup(name).Type().(*types.Signature)
	}

	oldKey := plain.signatureKey(sig(oldPkg, "F"))
	if !strings.Contains(oldKey, "example.com/lib.Config") {
		t.Fatalf("signatureKey() = %q, expected it to name example.com/lib.Config", oldKey)
	}
	if newKey := plain.signatureKey(sig(newPkg, "F")); newKey == oldKey {
		t.Fatalf("without renaming, keys of different modules should differ: %q", newKey)
	}
	if newKey := renamed.signatureKey(sig(newPkg, "F")); newKey != oldKey {
		t.Errorf("renamed signatureKey() =\n%q\nwant\n%q", newKey, oldKey)
	}

	getter := func(pkg *types.Package) *types.Func {
		return pkg.Scope().Lookup("Getter").Type().Underlying().(*types.Interface).Method(0)
	}
	if o, n := plain.interfaceMethodKey(getter(oldPkg)), renamed.interfaceMethodKey(getter(newPkg)); o != n {
		t.Errorf("renamed interfaceMethodKey() = %q, want %q", n, o)
	}

	box := func(pkg *types.Package) types.Type { return pkg.Scope().Lookup("Box").Type() }
	if o, n := plain.underlyingKey(box(oldPkg)), renamed.underlyingKey(box(newPkg)); o != n {
		t.Errorf("renamed underlyingKey() = %q, want %q", n, o)
	}

	if o, n := plain.signatureKey(sig(oldPkg, "G")), renamed.signatureKey(sig(changedPkg, "G")); o == n {
		t.Errorf("G changed from Config to *Config, but keys are equal: %q", o)
	}
}

func TestPreviousMajor(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		deps    []string
		want    string
		wantErr string
	}{
		{"v1 to v2", "example.com/lib/v2", []string{"example.com/other", "example.com/lib"}, "example.com/lib", ""},
		{"v2 to v3", "example.com/lib/v3", []string{"example.com/lib/v2"}, "example.com/lib/v2", ""},
		{"back to v1", "example.com/lib", []string{"example.com/lib/v2"}, "example.com/lib/v2", ""},
		{"gopkg.in", "gopkg.in/yaml.v3", []string{"gopkg.in/yaml.v2"}, "gopkg.in/yaml.v2", ""},
		{"different module", "example.com/lib/v2", []string{"example.com/library", "example.com/lib/sub"}, "", ""},
		{"not a dependency at all", "example.com/lib/v2", nil, "", ""},
		{"ambiguous", "example.com/lib/v3", []string{"example.com/lib", "example.com/lib/v2"}, "",
			"could replace any of example.com/lib, example.com/lib/v2"},
		{"invalid path", "example.com/Lib\x00/v2", []string{"example.com/Lib"}, "", "invalid module path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := previousMajor(tt.target, tt.deps)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("previousMajor() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("previousMajor() = %q, %v, want %q", got, err, tt.want)
			}
		})
	}
}
