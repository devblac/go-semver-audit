package analyzer

import (
	"go/types"
	"strings"
)

// Symbols are matched between the two API versions, and against the project's
// usage, by a key that includes the package path and - for methods - the
// receiver type.
//
// Bare names are not enough: two packages of the same module can both export
// New, and a method named Close is not the same symbol as a package-level
// function named Close. Keying by bare name made method changes invisible
// (the API recorded "Config.Validate" while usage recorded "Validate") and let
// same-named symbols from different packages overwrite each other.

// symbolKey returns the key for a package-level symbol.
func symbolKey(pkgPath, name string) string {
	return pkgPath + "." + name
}

// methodSymbolKey returns the key for a method on a named type.
func methodSymbolKey(pkgPath, recv, name string) string {
	return pkgPath + "." + recv + "." + name
}

// signatureKey renders the parts of a signature that callers depend on: type
// parameters, parameter and result types, and variadicity. Parameter and result
// names are left out, so renaming a parameter - which breaks no caller - is not
// reported as a signature change. Display strings keep the names.
//
// Type parameter names are kept, because they appear inside the parameter types
// (e.g. []E); renaming a type parameter is still reported.
func signatureKey(sig *types.Signature) string {
	var b strings.Builder

	if tparams := sig.TypeParams(); tparams.Len() > 0 {
		b.WriteString("[")
		for i := 0; i < tparams.Len(); i++ {
			if i > 0 {
				b.WriteString(", ")
			}
			tp := tparams.At(i)
			b.WriteString(tp.Obj().Name())
			b.WriteString(" ")
			b.WriteString(types.TypeString(tp.Constraint(), nil))
		}
		b.WriteString("]")
	}

	b.WriteString("(")
	writeTupleTypes(&b, sig.Params(), sig.Variadic())
	b.WriteString(")")

	if results := sig.Results(); results.Len() > 0 {
		b.WriteString(" (")
		writeTupleTypes(&b, results, false)
		b.WriteString(")")
	}
	return b.String()
}

// writeTupleTypes writes the types of a parameter or result list, without names
func writeTupleTypes(b *strings.Builder, tuple *types.Tuple, variadic bool) {
	for i := 0; i < tuple.Len(); i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		t := tuple.At(i).Type()
		if variadic && i == tuple.Len()-1 {
			if slice, ok := t.(*types.Slice); ok {
				b.WriteString("...")
				t = slice.Elem()
			}
		}
		b.WriteString(types.TypeString(t, nil))
	}
}

// interfaceMethodKey identifies an interface method for comparison: its name
// and signatureKey, so renaming a method's parameters is not a change.
func interfaceMethodKey(m *types.Func) string {
	return m.Name() + signatureKey(m.Type().(*types.Signature))
}

// interfaceMethodString renders an interface method as "Name(params) results",
// which is how interface methods are reported.
//
// types.Func.String() would prefix the receiver instead, e.g.
// "func (example.com/kvstore.Iterator).Err() error". Besides being noisy, that
// receiver is the embedded interface for promoted methods, so the same method
// would read differently depending on whether a version declares it directly
// or embeds it, and show up as removed and added at once.
func interfaceMethodString(m *types.Func) string {
	return m.Name() + strings.TrimPrefix(types.TypeString(m.Type(), nil), "func")
}

// displayName renders a symbol for reports, e.g. "slices.SortFunc" or
// "lib.Config.Validate". Full package paths are too long to read in a report,
// and bare names are ambiguous once more than one package is involved.
func displayName(pkgName, recv, name string) string {
	var b strings.Builder
	if pkgName != "" {
		b.WriteString(pkgName)
		b.WriteString(".")
	}
	if recv != "" {
		b.WriteString(recv)
		b.WriteString(".")
	}
	b.WriteString(name)
	return b.String()
}
