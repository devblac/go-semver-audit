package analyzer

import (
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
