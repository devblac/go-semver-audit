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

// interfaceMethodString renders an interface method as "Name(params) results",
// which is what interface methods are compared and reported by.
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
