package analyzer

import (
	"testing"
)

func TestDiffAPIs(t *testing.T) {
	tests := []struct {
		name   string
		oldAPI *API
		newAPI *API
		usage  *Usage
		want   struct {
			removedCount   int
			addedCount     int
			changedCount   int
			interfaceCount int
		}
	}{
		{
			name: "no changes",
			oldAPI: &API{
				Funcs: map[string]*Function{
					"Foo": {Name: "Foo", Signature: "func() error"},
				},
			},
			newAPI: &API{
				Funcs: map[string]*Function{
					"Foo": {Name: "Foo", Signature: "func() error"},
				},
			},
			usage: &Usage{
				Symbols: map[string][]Location{
					"Foo": {{File: "main.go", Line: 10}},
				},
			},
			want: struct {
				removedCount   int
				addedCount     int
				changedCount   int
				interfaceCount int
			}{0, 0, 0, 0},
		},
		{
			name: "function removed and used",
			oldAPI: &API{
				Funcs: map[string]*Function{
					"OldFunc": {Name: "OldFunc", Signature: "func() error"},
				},
			},
			newAPI: &API{
				Funcs: map[string]*Function{},
			},
			usage: &Usage{
				Symbols: map[string][]Location{
					"OldFunc": {{File: "main.go", Line: 10}},
				},
			},
			want: struct {
				removedCount   int
				addedCount     int
				changedCount   int
				interfaceCount int
			}{1, 0, 0, 0},
		},
		{
			name: "function removed but not used",
			oldAPI: &API{
				Funcs: map[string]*Function{
					"UnusedFunc": {Name: "UnusedFunc", Signature: "func() error"},
				},
			},
			newAPI: &API{
				Funcs: map[string]*Function{},
			},
			usage: &Usage{
				Symbols: map[string][]Location{},
			},
			want: struct {
				removedCount   int
				addedCount     int
				changedCount   int
				interfaceCount int
			}{0, 0, 0, 0},
		},
		{
			name: "function added",
			oldAPI: &API{
				Funcs: map[string]*Function{},
			},
			newAPI: &API{
				Funcs: map[string]*Function{
					"NewFunc": {Name: "NewFunc", Signature: "func() error"},
				},
			},
			usage: &Usage{
				Symbols: map[string][]Location{},
			},
			want: struct {
				removedCount   int
				addedCount     int
				changedCount   int
				interfaceCount int
			}{0, 1, 0, 0},
		},
		{
			name: "signature changed",
			oldAPI: &API{
				Funcs: map[string]*Function{
					"Func": {Name: "Func", Signature: "func() error"},
				},
			},
			newAPI: &API{
				Funcs: map[string]*Function{
					"Func": {Name: "Func", Signature: "func(context.Context) error"},
				},
			},
			usage: &Usage{
				Symbols: map[string][]Location{
					"Func": {{File: "main.go", Line: 10}},
				},
			},
			want: struct {
				removedCount   int
				addedCount     int
				changedCount   int
				interfaceCount int
			}{0, 0, 1, 0},
		},
		{
			name: "interface method removed",
			oldAPI: &API{
				Interfaces: map[string]*Interface{
					"Handler": {
						Name:    "Handler",
						Methods: []string{"Handle() error", "Close() error"},
					},
				},
			},
			newAPI: &API{
				Interfaces: map[string]*Interface{
					"Handler": {
						Name:    "Handler",
						Methods: []string{"Handle() error"},
					},
				},
			},
			usage: &Usage{
				Symbols: map[string][]Location{
					"Handler": {{File: "main.go", Line: 10}},
				},
			},
			want: struct {
				removedCount   int
				addedCount     int
				changedCount   int
				interfaceCount int
			}{0, 0, 0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diffAPIs(tt.oldAPI, tt.newAPI, tt.usage)

			if len(got.Removed) != tt.want.removedCount {
				t.Errorf("diffAPIs() removed count = %v, want %v", len(got.Removed), tt.want.removedCount)
			}
			if len(got.Added) != tt.want.addedCount {
				t.Errorf("diffAPIs() added count = %v, want %v", len(got.Added), tt.want.addedCount)
			}
			if len(got.Changed) != tt.want.changedCount {
				t.Errorf("diffAPIs() changed count = %v, want %v", len(got.Changed), tt.want.changedCount)
			}
			if len(got.InterfaceChanges) != tt.want.interfaceCount {
				t.Errorf("diffAPIs() interface changes count = %v, want %v", len(got.InterfaceChanges), tt.want.interfaceCount)
			}
		})
	}
}

func TestDiffInterfaces(t *testing.T) {
	tests := []struct {
		name     string
		oldIface *Interface
		newIface *Interface
		usage    *Usage
		wantNil  bool
	}{
		{
			name: "no changes",
			oldIface: &Interface{
				Name:    "Handler",
				Methods: []string{"Handle() error"},
			},
			newIface: &Interface{
				Name:    "Handler",
				Methods: []string{"Handle() error"},
			},
			usage: &Usage{
				Symbols: map[string][]Location{
					"Handler": {{File: "main.go", Line: 10}},
				},
			},
			wantNil: true,
		},
		{
			name: "method added and used",
			oldIface: &Interface{
				Name:    "Handler",
				Methods: []string{"Handle() error"},
			},
			newIface: &Interface{
				Name:    "Handler",
				Methods: []string{"Handle() error", "Close() error"},
			},
			usage: &Usage{
				Symbols: map[string][]Location{
					"Handler": {{File: "main.go", Line: 10}},
				},
			},
			wantNil: false,
		},
		{
			name: "method removed and used",
			oldIface: &Interface{
				Name:    "Handler",
				Methods: []string{"Handle() error", "Close() error"},
			},
			newIface: &Interface{
				Name:    "Handler",
				Methods: []string{"Handle() error"},
			},
			usage: &Usage{
				Symbols: map[string][]Location{
					"Handler": {{File: "main.go", Line: 10}},
				},
			},
			wantNil: false,
		},
		{
			name: "changes but not used",
			oldIface: &Interface{
				Name:    "Handler",
				Methods: []string{"Handle() error"},
			},
			newIface: &Interface{
				Name:    "Handler",
				Methods: []string{"Handle() error", "Close() error"},
			},
			usage: &Usage{
				Symbols: map[string][]Location{},
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diffInterfaces(tt.oldIface.Name, tt.oldIface, tt.newIface, tt.usage)
			if (got == nil) != tt.wantNil {
				t.Errorf("diffInterfaces() returned nil = %v, wantNil %v", got == nil, tt.wantNil)
			}
		})
	}
}
