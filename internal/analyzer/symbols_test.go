package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

func TestInterfaceMethodString(t *testing.T) {
	pkg := types.NewPackage("example.com/kvstore", "kvstore")
	errType := types.Universe.Lookup("error").Type()

	errMethod := types.NewFunc(token.NoPos, pkg, "Err", newSignature(nil, []*types.Var{
		types.NewVar(token.NoPos, pkg, "", errType),
	}))
	keyMethod := types.NewFunc(token.NoPos, pkg, "Key", newSignature(
		[]*types.Var{types.NewVar(token.NoPos, pkg, "prefix", types.Typ[types.String])},
		[]*types.Var{types.NewVar(token.NoPos, pkg, "", types.Typ[types.String])},
	))

	// Declared directly on the interface
	direct := types.NewInterfaceType([]*types.Func{errMethod, keyMethod}, nil)
	direct.Complete()

	// The same methods, promoted from an embedded interface
	inner := types.NewInterfaceType([]*types.Func{errMethod, keyMethod}, nil)
	innerNamed := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "Inner", nil), inner, nil)
	embedding := types.NewInterfaceType(nil, []types.Type{innerNamed})
	embedding.Complete()

	want := map[string]bool{"Err() error": true, "Key(prefix string) string": true}
	for name, iface := range map[string]*types.Interface{"direct": direct, "embedded": embedding} {
		if iface.NumMethods() != len(want) {
			t.Fatalf("%s: NumMethods() = %d, want %d", name, iface.NumMethods(), len(want))
		}
		for i := 0; i < iface.NumMethods(); i++ {
			got := interfaceMethodString(iface.Method(i))
			if !want[got] {
				t.Errorf("%s: interfaceMethodString() = %q, want one of %v", name, got, want)
			}
		}
	}
}

// signatureOf type-checks src and returns the signature of its function F
func signatureOf(t *testing.T, src string) *types.Signature {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", "package x\n"+src, 0)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	pkg, err := (&types.Config{}).Check("example.com/x", fset, []*ast.File{file}, nil)
	if err != nil {
		t.Fatalf("type-check %q: %v", src, err)
	}
	return pkg.Scope().Lookup("F").Type().(*types.Signature)
}

func TestSignatureKey(t *testing.T) {
	tests := []struct {
		name     string
		old, new string
		same     bool
	}{
		{"parameter renamed", "func F(key string) error { return nil }", "func F(k string) error { return nil }", true},
		{"result named", "func F() (int, error) { return 0, nil }", "func F() (n int, err error) { return }", true},
		{"generic parameter renamed", "func F[E any](s []E) {}", "func F[E any](items []E) {}", true},
		{"parameter type changed", "func F(key string) {}", "func F(key int) {}", false},
		{"parameter added", "func F(key string) {}", "func F(key string, n int) {}", false},
		{"result added", "func F() {}", "func F() error { return nil }", false},
		{"slice became variadic", "func F(xs []int) {}", "func F(xs ...int) {}", false},
		{"constraint tightened", "func F[E any](s []E) {}", "func F[E comparable](s []E) {}", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldKey := signatureKey(signatureOf(t, tt.old))
			newKey := signatureKey(signatureOf(t, tt.new))
			if (oldKey == newKey) != tt.same {
				t.Errorf("signatureKey() old %q, new %q: same = %v, want %v", oldKey, newKey, oldKey == newKey, tt.same)
			}
		})
	}
}
