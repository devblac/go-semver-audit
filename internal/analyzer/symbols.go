package analyzer

import (
	"fmt"
	"go/types"
	"strconv"
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
			b.WriteString(typeKey(tp.Constraint()))
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
		b.WriteString(typeKey(t))
	}
}

// typeKey renders a type for signature comparison. It follows
// types.TypeString, except that function types nested anywhere inside - a
// callback parameter, a method of an anonymous interface, a field of an
// anonymous struct - are rendered without parameter names as well, because
// those names are not part of type identity. Struct field names and tags are
// part of it, and are kept. Aliases render as the type they stand for.
func typeKey(t types.Type) string {
	switch t := types.Unalias(t).(type) {
	case *types.Signature:
		return "func" + signatureKey(t)
	case *types.Pointer:
		return "*" + typeKey(t.Elem())
	case *types.Slice:
		return "[]" + typeKey(t.Elem())
	case *types.Array:
		return fmt.Sprintf("[%d]%s", t.Len(), typeKey(t.Elem()))
	case *types.Map:
		return "map[" + typeKey(t.Key()) + "]" + typeKey(t.Elem())
	case *types.Chan:
		dir := "chan "
		switch t.Dir() {
		case types.SendOnly:
			dir = "chan<- "
		case types.RecvOnly:
			dir = "<-chan "
		}
		return dir + "(" + typeKey(t.Elem()) + ")"
	case *types.Struct:
		var b strings.Builder
		b.WriteString("struct{")
		for i := 0; i < t.NumFields(); i++ {
			if i > 0 {
				b.WriteString("; ")
			}
			field := t.Field(i)
			if !field.Embedded() {
				b.WriteString(field.Name())
				b.WriteString(" ")
			}
			b.WriteString(typeKey(field.Type()))
			if tag := t.Tag(i); tag != "" {
				b.WriteString(" ")
				b.WriteString(strconv.Quote(tag))
			}
		}
		b.WriteString("}")
		return b.String()
	case *types.Interface:
		if !t.IsMethodSet() {
			// A constraint with type terms (~int | ~string) has no
			// parameter names to strip
			return types.TypeString(t, nil)
		}
		var b strings.Builder
		b.WriteString("interface{")
		// Methods come sorted, with embedded interfaces flattened in
		for i := 0; i < t.NumMethods(); i++ {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(interfaceMethodKey(t.Method(i)))
		}
		b.WriteString("}")
		return b.String()
	case *types.Named:
		// Named types render by name, which also stops recursive types
		var b strings.Builder
		if pkg := t.Obj().Pkg(); pkg != nil {
			b.WriteString(pkg.Path())
			b.WriteString(".")
		}
		b.WriteString(t.Obj().Name())
		if args := t.TypeArgs(); args.Len() > 0 {
			b.WriteString("[")
			for i := 0; i < args.Len(); i++ {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(typeKey(args.At(i)))
			}
			b.WriteString("]")
		}
		return b.String()
	default:
		return types.TypeString(t, nil)
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

// packageQualifier is how a package is named in reports: its path relative to
// the module root, which is unique within the module - api/foo/v1 and
// api/bar/v1 are both "package v1", so package names alone would render both
// as v1.Config. The module's root package has no relative path and uses its
// package name, so the common cases read as before (slices.SortFunc).
func packageQualifier(module, pkgPath, pkgName string) string {
	if rel := strings.TrimPrefix(pkgPath, module+"/"); rel != pkgPath && rel != "" {
		return rel
	}
	return pkgName
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
