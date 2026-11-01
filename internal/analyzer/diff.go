package analyzer

import (
	"slices"
	"sort"
	"strings"
)

// diffAPIs compares two API surfaces and returns the differences
func diffAPIs(oldAPI, newAPI *API, usage *Usage) *Diff {
	diff := &Diff{
		Removed:          []RemovedSymbol{},
		Added:            []AddedSymbol{},
		Changed:          []ChangedSignature{},
		InterfaceChanges: []InterfaceChange{},
	}

	// Check for removed functions and methods
	for key, oldFunc := range oldAPI.Funcs {
		newFunc, exists := newAPI.Funcs[key]
		if !exists {
			// A method that disappeared along with its receiver type is
			// already covered by the type's own entry; reporting both would
			// count one break several times over
			if oldFunc.IsMethod && !typeExists(newAPI, oldFunc.PkgPath, oldFunc.Recv) {
				continue
			}

			// Only report if it's actually used
			locations := usage.Symbols[key]
			if len(locations) > 0 {
				diff.Removed = append(diff.Removed, RemovedSymbol{
					Name:    oldFunc.Display(),
					Package: oldFunc.PkgPath,
					Type:    oldFunc.Kind(),
					UsedIn:  locations,
				})
			}
			continue
		}

		// Function exists, check if signature changed
		if oldFunc.comparisonSignature() != newFunc.comparisonSignature() {
			locations := usage.Symbols[key]
			if len(locations) > 0 {
				diff.Changed = append(diff.Changed, ChangedSignature{
					Name:         oldFunc.Display(),
					Package:      oldFunc.PkgPath,
					Type:         oldFunc.Kind(),
					OldSignature: oldFunc.Signature,
					NewSignature: newFunc.Signature,
					UsedIn:       locations,
				})
			}
		}
	}

	// Check for added functions (informational)
	for key, newFunc := range newAPI.Funcs {
		if _, exists := oldAPI.Funcs[key]; !exists {
			diff.Added = append(diff.Added, AddedSymbol{
				Name:    newFunc.Display(),
				Package: newFunc.PkgPath,
				Type:    newFunc.Kind(),
			})
		}
	}

	// Check for removed types, and types whose underlying type changed
	for key, oldType := range oldAPI.Types {
		newType, exists := newAPI.Types[key]
		if !exists {
			locations := usage.Symbols[key]
			if len(locations) > 0 {
				diff.Removed = append(diff.Removed, RemovedSymbol{
					Name:    oldType.Display(),
					Package: oldType.PkgPath,
					Type:    "type",
					UsedIn:  locations,
				})
			}
			continue
		}

		if oldType.UnderlyingKey != "" && newType.UnderlyingKey != "" && oldType.UnderlyingKey != newType.UnderlyingKey {
			locations := usage.Symbols[key]
			if len(locations) > 0 {
				diff.Changed = append(diff.Changed, ChangedSignature{
					Name:         oldType.Display(),
					Package:      oldType.PkgPath,
					Type:         "type",
					OldSignature: oldType.Kind,
					NewSignature: newType.Kind,
					UsedIn:       locations,
				})
			}
		}
	}

	// Check for added types (informational)
	for key, newType := range newAPI.Types {
		if _, exists := oldAPI.Types[key]; !exists {
			diff.Added = append(diff.Added, AddedSymbol{
				Name:    newType.Display(),
				Package: newType.PkgPath,
				Type:    "type",
			})
		}
	}

	// Check for interface changes
	for key, oldIface := range oldAPI.Interfaces {
		if newIface, exists := newAPI.Interfaces[key]; exists {
			change := diffInterfaces(key, oldIface, newIface, usage)
			if change != nil {
				diff.InterfaceChanges = append(diff.InterfaceChanges, *change)
			}
		} else {
			// Interface was removed
			locations := usage.Symbols[key]
			if len(locations) > 0 {
				diff.Removed = append(diff.Removed, RemovedSymbol{
					Name:    oldIface.Display(),
					Package: oldIface.PkgPath,
					Type:    "interface",
					UsedIn:  locations,
				})
			}
		}
	}

	// Check for added interfaces (informational)
	for key, newIface := range newAPI.Interfaces {
		if _, exists := oldAPI.Interfaces[key]; !exists {
			diff.Added = append(diff.Added, AddedSymbol{
				Name:    newIface.Display(),
				Package: newIface.PkgPath,
				Type:    "interface",
			})
		}
	}

	diffValues(diff, oldAPI, newAPI, usage)
	diffFields(diff, oldAPI, newAPI, usage)

	// The API surfaces are maps, so everything above is collected in random
	// order. Sort before returning to keep reports stable across runs.
	sortDiff(diff)

	return diff
}

// sortDiff orders every collection in a Diff so that repeated runs over the
// same versions produce byte-identical reports.
func sortDiff(diff *Diff) {
	// Display names are unique within a module, but the full package path
	// breaks any remaining tie so the order never depends on map iteration
	sort.Slice(diff.Removed, func(i, j int) bool {
		a, b := diff.Removed[i], diff.Removed[j]
		return lessByKeys(a.Name, b.Name, a.Type, b.Type, a.Package, b.Package)
	})

	sort.Slice(diff.Added, func(i, j int) bool {
		a, b := diff.Added[i], diff.Added[j]
		return lessByKeys(a.Name, b.Name, a.Type, b.Type, a.Package, b.Package)
	})

	sort.Slice(diff.Changed, func(i, j int) bool {
		a, b := diff.Changed[i], diff.Changed[j]
		return lessByKeys(a.Name, b.Name, a.Package, b.Package)
	})

	sort.Slice(diff.InterfaceChanges, func(i, j int) bool {
		a, b := diff.InterfaceChanges[i], diff.InterfaceChanges[j]
		return lessByKeys(a.Name, b.Name, a.Package, b.Package)
	})
}

// lessByKeys compares (a1, b1, a2, b2, ...) pairs in order, the first unequal
// pair deciding
func lessByKeys(pairs ...string) bool {
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i] != pairs[i+1] {
			return pairs[i] < pairs[i+1]
		}
	}
	return false
}

// typeExists reports whether a named type still exists in the given API,
// either as a plain type or as an interface.
func typeExists(api *API, pkgPath, name string) bool {
	key := symbolKey(pkgPath, name)
	if _, ok := api.Types[key]; ok {
		return true
	}
	_, ok := api.Interfaces[key]
	return ok
}

// diffInterfaces compares two interface definitions. key is the symbol key used
// to look up usage; names in the returned change are report-friendly.
func diffInterfaces(key string, oldIface, newIface *Interface, usage *Usage) *InterfaceChange {
	// Methods are matched by comparison key (parameter names ignored) and
	// reported by their display string
	oldMethods := oldIface.methodsByKey()
	newMethods := newIface.methodsByKey()

	var added, removed []string

	// Find removed methods
	for methodKey, display := range oldMethods {
		if _, ok := newMethods[methodKey]; !ok {
			removed = append(removed, display)
		}
	}

	// Find added methods
	for methodKey, display := range newMethods {
		if _, ok := oldMethods[methodKey]; !ok {
			added = append(added, display)
		}
	}

	sort.Strings(added)
	sort.Strings(removed)

	// Using the interface is not enough for a change to break the project:
	// an added method breaks only the project's own implementations, and a
	// removed one only the calls to it. A changed method is both.
	var implemented, called []Location
	if len(added) > 0 {
		implemented = usage.Implementations[key]
	}
	for _, display := range removed {
		called = append(called, usage.Symbols[key+"."+methodName(display)]...)
	}
	if len(implemented) == 0 && len(called) == 0 {
		return nil
	}

	return &InterfaceChange{
		Name:           oldIface.Display(),
		Package:        oldIface.PkgPath,
		AddedMethods:   added,
		RemovedMethods: removed,
		UsedIn:         sortLocations(append(slices.Clone(implemented), called...)),
		ImplementedIn:  implemented,
	}
}

// methodName returns the name of an interface method from its display string,
// e.g. "Key" from "Key(prefix string) string"
func methodName(display string) string {
	if i := strings.IndexByte(display, '('); i >= 0 {
		return display[:i]
	}
	return display
}

// diffValues reports package-level vars and consts that were removed, or whose
// type changed or that switched between var and const
func diffValues(diff *Diff, oldAPI, newAPI *API, usage *Usage) {
	for key, oldValue := range oldAPI.Values {
		newValue, exists := newAPI.Values[key]
		locations := usage.Symbols[key]
		switch {
		case !exists:
			if len(locations) > 0 {
				diff.Removed = append(diff.Removed, RemovedSymbol{
					Name:    oldValue.Display(),
					Package: oldValue.PkgPath,
					Type:    oldValue.Kind,
					UsedIn:  locations,
				})
			}
		case oldValue.comparisonSignature() != newValue.comparisonSignature():
			if len(locations) > 0 {
				diff.Changed = append(diff.Changed, ChangedSignature{
					Name:         oldValue.Display(),
					Package:      oldValue.PkgPath,
					Type:         oldValue.Kind,
					OldSignature: oldValue.Signature(),
					NewSignature: newValue.Signature(),
					UsedIn:       locations,
				})
			}
		}
	}

	for key, newValue := range newAPI.Values {
		if _, exists := oldAPI.Values[key]; !exists {
			diff.Added = append(diff.Added, AddedSymbol{
				Name:    newValue.Display(),
				Package: newValue.PkgPath,
				Type:    newValue.Kind,
			})
		}
	}
}

// diffFields reports exported struct fields that were removed or whose type
// changed. Fields of a struct type that was removed entirely are covered by
// the type's own entry.
func diffFields(diff *Diff, oldAPI, newAPI *API, usage *Usage) {
	for key, oldField := range oldAPI.Fields {
		newField, exists := newAPI.Fields[key]
		locations := usage.Symbols[key]
		switch {
		case !exists:
			if len(locations) > 0 && typeExists(newAPI, oldField.PkgPath, oldField.Recv) {
				diff.Removed = append(diff.Removed, RemovedSymbol{
					Name:    oldField.Display(),
					Package: oldField.PkgPath,
					Type:    "field",
					UsedIn:  locations,
				})
			}
		case oldField.TypeKey != newField.TypeKey:
			if len(locations) > 0 {
				diff.Changed = append(diff.Changed, ChangedSignature{
					Name:         oldField.Display(),
					Package:      oldField.PkgPath,
					Type:         "field",
					OldSignature: oldField.Type,
					NewSignature: newField.Type,
					UsedIn:       locations,
				})
			}
		}
	}

	for key, newField := range newAPI.Fields {
		if _, exists := oldAPI.Fields[key]; !exists {
			diff.Added = append(diff.Added, AddedSymbol{
				Name:    newField.Display(),
				Package: newField.PkgPath,
				Type:    "field",
			})
		}
	}
}
