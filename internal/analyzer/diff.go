package analyzer

import "sort"

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
					Name:   oldFunc.Display(),
					Type:   oldFunc.Kind(),
					UsedIn: locations,
				})
			}
			continue
		}

		// Function exists, check if signature changed
		if oldFunc.Signature != newFunc.Signature {
			locations := usage.Symbols[key]
			if len(locations) > 0 {
				diff.Changed = append(diff.Changed, ChangedSignature{
					Name:         oldFunc.Display(),
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
				Name: newFunc.Display(),
				Type: newFunc.Kind(),
			})
		}
	}

	// Check for removed types
	for key, oldType := range oldAPI.Types {
		if _, exists := newAPI.Types[key]; !exists {
			locations := usage.Symbols[key]
			if len(locations) > 0 {
				diff.Removed = append(diff.Removed, RemovedSymbol{
					Name:   oldType.Display(),
					Type:   "type",
					UsedIn: locations,
				})
			}
		}
	}

	// Check for added types (informational)
	for key, newType := range newAPI.Types {
		if _, exists := oldAPI.Types[key]; !exists {
			diff.Added = append(diff.Added, AddedSymbol{
				Name: newType.Display(),
				Type: "type",
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
					Name:   oldIface.Display(),
					Type:   "interface",
					UsedIn: locations,
				})
			}
		}
	}

	// Check for added interfaces (informational)
	for key, newIface := range newAPI.Interfaces {
		if _, exists := oldAPI.Interfaces[key]; !exists {
			diff.Added = append(diff.Added, AddedSymbol{
				Name: newIface.Display(),
				Type: "interface",
			})
		}
	}

	// The API surfaces are maps, so everything above is collected in random
	// order. Sort before returning to keep reports stable across runs.
	sortDiff(diff)

	return diff
}

// sortDiff orders every collection in a Diff so that repeated runs over the
// same versions produce byte-identical reports.
func sortDiff(diff *Diff) {
	sort.Slice(diff.Removed, func(i, j int) bool {
		if diff.Removed[i].Name != diff.Removed[j].Name {
			return diff.Removed[i].Name < diff.Removed[j].Name
		}
		return diff.Removed[i].Type < diff.Removed[j].Type
	})

	sort.Slice(diff.Added, func(i, j int) bool {
		if diff.Added[i].Name != diff.Added[j].Name {
			return diff.Added[i].Name < diff.Added[j].Name
		}
		return diff.Added[i].Type < diff.Added[j].Type
	})

	sort.Slice(diff.Changed, func(i, j int) bool {
		return diff.Changed[i].Name < diff.Changed[j].Name
	})

	sort.Slice(diff.InterfaceChanges, func(i, j int) bool {
		return diff.InterfaceChanges[i].Name < diff.InterfaceChanges[j].Name
	})
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
	oldMethods := make(map[string]bool)
	for _, method := range oldIface.Methods {
		oldMethods[method] = true
	}

	newMethods := make(map[string]bool)
	for _, method := range newIface.Methods {
		newMethods[method] = true
	}

	var added, removed []string

	// Find removed methods
	for method := range oldMethods {
		if !newMethods[method] {
			removed = append(removed, method)
		}
	}

	// Find added methods
	for method := range newMethods {
		if !oldMethods[method] {
			added = append(added, method)
		}
	}

	sort.Strings(added)
	sort.Strings(removed)

	// If there are changes and the interface is used, report it
	if (len(added) > 0 || len(removed) > 0) && len(usage.Symbols[key]) > 0 {
		return &InterfaceChange{
			Name:           oldIface.Display(),
			AddedMethods:   added,
			RemovedMethods: removed,
			UsedIn:         usage.Symbols[key],
		}
	}

	return nil
}
