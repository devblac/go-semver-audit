package analyzer

import "strings"

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
