package analyzer

// diffAPIs compares two API surfaces and returns the differences
func diffAPIs(oldAPI, newAPI *API, usage *Usage) *Diff {
	diff := &Diff{
		Removed:          []RemovedSymbol{},
		Added:            []AddedSymbol{},
		Changed:          []ChangedSignature{},
		InterfaceChanges: []InterfaceChange{},
	}

	// Check for removed functions
	for name, oldFunc := range oldAPI.Funcs {
		if _, exists := newAPI.Funcs[name]; !exists {
			// Function was removed
			locations := usage.Symbols[name]
			if len(locations) > 0 {
				// Only report if it's actually used
				diff.Removed = append(diff.Removed, RemovedSymbol{
					Name:   name,
					Type:   "function",
					UsedIn: locations,
				})
			}
		} else {
			// Function exists, check if signature changed
			newFunc := newAPI.Funcs[name]
			if oldFunc.Signature != newFunc.Signature {
				locations := usage.Symbols[name]
				if len(locations) > 0 {
					diff.Changed = append(diff.Changed, ChangedSignature{
						Name:         name,
						OldSignature: oldFunc.Signature,
						NewSignature: newFunc.Signature,
						UsedIn:       locations,
					})
				}
			}
		}
	}

	// Check for added functions (informational)
	for name := range newAPI.Funcs {
		if _, exists := oldAPI.Funcs[name]; !exists {
			diff.Added = append(diff.Added, AddedSymbol{
				Name: name,
				Type: "function",
			})
		}
	}

	// Check for removed types
	for name := range oldAPI.Types {
		if _, exists := newAPI.Types[name]; !exists {
			locations := usage.Symbols[name]
			if len(locations) > 0 {
				diff.Removed = append(diff.Removed, RemovedSymbol{
					Name:   name,
					Type:   "type",
					UsedIn: locations,
				})
			}
		}
	}

	// Check for added types (informational)
	for name := range newAPI.Types {
		if _, exists := oldAPI.Types[name]; !exists {
			diff.Added = append(diff.Added, AddedSymbol{
				Name: name,
				Type: "type",
			})
		}
	}

	// Check for interface changes
	for name, oldIface := range oldAPI.Interfaces {
		if newIface, exists := newAPI.Interfaces[name]; exists {
			change := diffInterfaces(name, oldIface, newIface, usage)
			if change != nil {
				diff.InterfaceChanges = append(diff.InterfaceChanges, *change)
			}
		} else {
			// Interface was removed
			locations := usage.Symbols[name]
			if len(locations) > 0 {
				diff.Removed = append(diff.Removed, RemovedSymbol{
					Name:   name,
					Type:   "interface",
					UsedIn: locations,
				})
			}
		}
	}

	// Check for added interfaces (informational)
	for name := range newAPI.Interfaces {
		if _, exists := oldAPI.Interfaces[name]; !exists {
			diff.Added = append(diff.Added, AddedSymbol{
				Name: name,
				Type: "interface",
			})
		}
	}

	return diff
}

// diffInterfaces compares two interface definitions
func diffInterfaces(name string, oldIface, newIface *Interface, usage *Usage) *InterfaceChange {
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

	// If there are changes and the interface is used, report it
	if (len(added) > 0 || len(removed) > 0) && len(usage.Symbols[name]) > 0 {
		return &InterfaceChange{
			Name:           name,
			AddedMethods:   added,
			RemovedMethods: removed,
			UsedIn:         usage.Symbols[name],
		}
	}

	return nil
}

