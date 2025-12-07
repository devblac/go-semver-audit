package analyzer

import "strings"

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
	UnusedDeps []string
}

// HasBreakingChanges returns true if the result contains breaking changes
func (r *Result) HasBreakingChanges() bool {
	if r.Changes == nil {
		return false
	}
	return len(r.Changes.Removed) > 0 ||
		len(r.Changes.Changed) > 0 ||
		len(r.Changes.InterfaceChanges) > 0
}

// HasWarnings returns true if the result contains warnings
func (r *Result) HasWarnings() bool {
	if r.Changes == nil {
		return false
	}
	return len(r.Changes.Added) > 0 || len(r.UnusedDeps) > 0
}

// API represents the exported API surface of a module
type API struct {
	Funcs      map[string]*Function
	Types      map[string]*Type
	Interfaces map[string]*Interface
}

// Function represents an exported function or method
type Function struct {
	Name      string
	Signature string
	PkgPath   string
	IsMethod  bool
}

// Type represents an exported type
type Type struct {
	Name    string
	Kind    string
	PkgPath string
}

// Interface represents an exported interface
type Interface struct {
	Name    string
	Methods []string
	PkgPath string
}

// Usage tracks which symbols are used in the project
type Usage struct {
	Symbols map[string][]Location
	Imports map[string]bool
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
	Name      string
	Type      string // "function", "type", "interface"
	UsedIn    []Location
}

// AddedSymbol represents a symbol that was added
type AddedSymbol struct {
	Name string
	Type string
}

// ChangedSignature represents a function/method with changed signature
type ChangedSignature struct {
	Name         string
	OldSignature string
	NewSignature string
	UsedIn       []Location
}

// InterfaceChange represents changes to an interface
type InterfaceChange struct {
	Name            string
	AddedMethods    []string
	RemovedMethods  []string
	ChangedMethods  []string
	UsedIn          []Location
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

