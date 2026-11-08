package analyzer

import (
	"fmt"
	"strings"
)

// Upgrade represents a dependency upgrade specification
type Upgrade struct {
	Module     string
	OldVersion string
	NewVersion string
}

// Result contains the analysis results
type Result struct {
	Module     string
	OldVersion string
	NewVersion string
	Changes    *Diff
	// BuildErrors are the errors from type-checking the project against
	// NewVersion. Only VerifyBuild (-verify) fills them in.
	BuildErrors []BuildError
}

// BuildError is a compile error in the project, or in one of its
// dependencies, after the upgrade
type BuildError struct {
	File    string // relative to the project root when inside it; empty if unknown
	Line    int    // 0 if unknown
	Col     int    // 0 if unknown
	Message string
}

// String renders the error as the go command does, e.g. "main.go:8:16: msg"
func (e BuildError) String() string {
	switch {
	case e.File == "":
		return e.Message
	case e.Line == 0:
		return fmt.Sprintf("%s: %s", e.File, e.Message)
	case e.Col == 0:
		return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Message)
	}
	return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Col, e.Message)
}

// HasBreakingChanges returns true if the API diff found breaking changes, or
// the project does not compile against the new version
func (r *Result) HasBreakingChanges() bool {
	return r.HasAPIBreakingChanges() || len(r.BuildErrors) > 0
}

// HasAPIBreakingChanges returns true if the API diff found breaking changes,
// regardless of the compile check
func (r *Result) HasAPIBreakingChanges() bool {
	if r.Changes == nil {
		return false
	}
	return len(r.Changes.Removed) > 0 ||
		len(r.Changes.Changed) > 0 ||
		len(r.Changes.InterfaceChanges) > 0
}

// API represents the exported API surface of a module.
// All maps are keyed by symbol key (see symbols.go).
type API struct {
	Version    string
	Funcs      map[string]*Function
	Types      map[string]*Type
	Interfaces map[string]*Interface
	Values     map[string]*Value // package-level vars and consts
	Fields     map[string]*Field // exported fields of exported struct types
}

// Function represents an exported function or method
type Function struct {
	Name      string // bare function or method name
	Recv      string // receiver type name for methods, empty for functions
	Signature string // for display, with parameter names
	// SignatureKey is what two versions are compared by: the signature without
	// parameter names (see signatureKey). Empty means compare Signature.
	SignatureKey string
	PkgPath      string
	PkgName      string // name in reports, see packageQualifier
	IsMethod     bool
}

// Display returns the report-friendly name of the function or method
func (f *Function) Display() string {
	return displayName(f.PkgName, f.Recv, f.Name)
}

// comparisonSignature returns what two versions of the function are compared by
func (f *Function) comparisonSignature() string {
	if f.SignatureKey != "" {
		return f.SignatureKey
	}
	return f.Signature
}

// Kind returns "method" or "function", for reporting
func (f *Function) Kind() string {
	if f.IsMethod {
		return "method"
	}
	return "function"
}

// Type represents an exported type
type Type struct {
	Name string
	Kind string // underlying type, for display
	// UnderlyingKey is what two versions' underlying types are compared by
	// (see underlyingKey). Empty means the underlying type is not compared.
	UnderlyingKey string
	PkgPath       string
	PkgName       string // name in reports, see packageQualifier
}

// Display returns the report-friendly name of the type
func (t *Type) Display() string {
	return displayName(t.PkgName, "", t.Name)
}

// Value represents an exported package-level variable or constant
type Value struct {
	Name    string
	Kind    string // "var" or "const"
	Type    string // for display
	TypeKey string // what two versions are compared by, see typeKey
	PkgPath string
	PkgName string // name in reports, see packageQualifier
}

// Display returns the report-friendly name of the value
func (v *Value) Display() string {
	return displayName(v.PkgName, "", v.Name)
}

// Signature renders the value for reports, e.g. "const untyped int"
func (v *Value) Signature() string {
	return v.Kind + " " + v.Type
}

// comparisonSignature returns what two versions of the value are compared by.
// A var that became a const (or the reverse) breaks callers too: a const
// cannot be assigned to or have its address taken.
func (v *Value) comparisonSignature() string {
	return v.Kind + " " + v.TypeKey
}

// Field represents an exported field of an exported struct type
type Field struct {
	Name    string
	Recv    string // the struct type the field belongs to
	Type    string // for display
	TypeKey string // what two versions are compared by, see typeKey
	PkgPath string
	PkgName string // name in reports, see packageQualifier
}

// Display returns the report-friendly name of the field
func (f *Field) Display() string {
	return displayName(f.PkgName, f.Recv, f.Name)
}

// Interface represents an exported interface
type Interface struct {
	Name    string
	Methods []string // for display, e.g. "Key(prefix string) string"
	// MethodKeys parallels Methods with what methods are compared by (see
	// interfaceMethodKey). Nil means compare Methods.
	MethodKeys []string
	PkgPath    string
	PkgName    string // name in reports, see packageQualifier
}

// Display returns the report-friendly name of the interface
func (i *Interface) Display() string {
	return displayName(i.PkgName, "", i.Name)
}

// methodsByKey maps each method's comparison key to its display string
func (i *Interface) methodsByKey() map[string]string {
	methods := make(map[string]string, len(i.Methods))
	for idx, display := range i.Methods {
		key := display
		if idx < len(i.MethodKeys) {
			key = i.MethodKeys[idx]
		}
		methods[key] = display
	}
	return methods
}

// Usage tracks which symbols are used in the project
type Usage struct {
	Symbols map[string][]Location
	Imports map[string]bool
	// Implementations maps an interface's symbol key to the project types
	// that implement it with methods of their own (see implementations)
	Implementations map[string][]Location
}

// Location represents a source code location
type Location struct {
	File string
	Line int
}

// Diff represents the differences between two API surfaces
type Diff struct {
	Removed          []RemovedSymbol
	Added            []AddedSymbol
	Changed          []ChangedSignature
	InterfaceChanges []InterfaceChange
}

// RemovedSymbol represents a symbol that was removed
type RemovedSymbol struct {
	Name    string
	Package string // full import path; disambiguates and orders entries
	Type    string // "function", "type", "interface"
	UsedIn  []Location
}

// AddedSymbol represents a symbol that was added
type AddedSymbol struct {
	Name    string
	Package string // full import path; disambiguates and orders entries
	Type    string
}

// ChangedSignature represents a symbol whose signature or type changed
type ChangedSignature struct {
	Name         string
	Package      string // full import path; disambiguates and orders entries
	Type         string // "function", "method", "var", "const", "field", "type"
	OldSignature string
	NewSignature string
	UsedIn       []Location
}

// IsTypeChange reports whether the change is to the type of a var, const,
// field or named type rather than to a function's signature
func (c ChangedSignature) IsTypeChange() bool {
	return c.Type != "" && c.Type != "function" && c.Type != "method"
}

// InterfaceChange represents changes to an interface
type InterfaceChange struct {
	Name           string
	Package        string // full import path; disambiguates and orders entries
	AddedMethods   []string
	RemovedMethods []string
	ChangedMethods []string
	UsedIn         []Location // implementations and calls that break
	ImplementedIn  []Location // the implementations among UsedIn
}

// ParseUpgrade parses an upgrade specification like "module@version"
func ParseUpgrade(spec string) (*Upgrade, error) {
	parts := strings.Split(spec, "@")
	if len(parts) != 2 {
		return nil, &ParseError{spec}
	}

	module := strings.TrimSpace(parts[0])
	version := strings.TrimSpace(parts[1])

	if module == "" || version == "" {
		return nil, &ParseError{spec}
	}

	return &Upgrade{
		Module:     module,
		NewVersion: version,
	}, nil
}

// ParseError represents an error parsing upgrade specification
type ParseError struct {
	Spec string
}

func (e *ParseError) Error() string {
	return "invalid upgrade specification: " + e.Spec + " (expected format: module@version)"
}
