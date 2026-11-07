package analyzer

import (
	"reflect"
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
		{
			name: "type removed and used",
			oldAPI: &API{
				Types: map[string]*Type{
					"Thing": {Name: "Thing", Kind: "struct"},
				},
			},
			newAPI: &API{
				Types: map[string]*Type{},
			},
			usage: &Usage{
				Symbols: map[string][]Location{
					"Thing": {{File: "main.go", Line: 20}},
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
			name: "type added",
			oldAPI: &API{
				Types: map[string]*Type{},
			},
			newAPI: &API{
				Types: map[string]*Type{
					"NewType": {Name: "NewType", Kind: "struct"},
				},
			},
			usage: &Usage{Symbols: map[string][]Location{}},
			want: struct {
				removedCount   int
				addedCount     int
				changedCount   int
				interfaceCount int
			}{0, 1, 0, 0},
		},
		{
			name: "interface removed when used",
			oldAPI: &API{
				Interfaces: map[string]*Interface{
					"Reader": {Name: "Reader", Methods: []string{"Read() error"}},
				},
			},
			newAPI: &API{
				Interfaces: map[string]*Interface{},
			},
			usage: &Usage{
				Symbols: map[string][]Location{
					"Reader": {{File: "main.go", Line: 30}},
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
			name: "interface added",
			oldAPI: &API{
				Interfaces: map[string]*Interface{},
			},
			newAPI: &API{
				Interfaces: map[string]*Interface{
					"Writer": {Name: "Writer", Methods: []string{"Write() error"}},
				},
			},
			usage: &Usage{Symbols: map[string][]Location{}},
			want: struct {
				removedCount   int
				addedCount     int
				changedCount   int
				interfaceCount int
			}{0, 1, 0, 0},
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

func TestDiffAPIsIsDeterministic(t *testing.T) {
	// Enough symbols that random map iteration would almost certainly produce
	// a different ordering on at least one of the runs below.
	oldAPI := &API{
		Funcs: map[string]*Function{
			"Alpha":   {Name: "Alpha", Signature: "func() error"},
			"Bravo":   {Name: "Bravo", Signature: "func(int) error"},
			"Charlie": {Name: "Charlie", Signature: "func() error"},
			"Delta":   {Name: "Delta", Signature: "func() error"},
			"Echo":    {Name: "Echo", Signature: "func() error"},
		},
		Types: map[string]*Type{
			"Config": {Name: "Config", Kind: "struct{}"},
			"Option": {Name: "Option", Kind: "func()"},
		},
		Interfaces: map[string]*Interface{
			"Handler": {Name: "Handler", Methods: []string{"Handle() error", "Close() error", "Flush() error"}},
			"Writer":  {Name: "Writer", Methods: []string{"Write() error"}},
		},
	}
	newAPI := &API{
		Funcs: map[string]*Function{
			// Alpha and Bravo change signature, Charlie is removed
			"Alpha":    {Name: "Alpha", Signature: "func(string) error"},
			"Bravo":    {Name: "Bravo", Signature: "func(int, bool) error"},
			"Delta":    {Name: "Delta", Signature: "func() error"},
			"Echo":     {Name: "Echo", Signature: "func() error"},
			"Foxtrot":  {Name: "Foxtrot", Signature: "func() error"},
			"Golf":     {Name: "Golf", Signature: "func() error"},
			"Hotel":    {Name: "Hotel", Signature: "func() error"},
			"November": {Name: "November", Signature: "func() error"},
		},
		Types: map[string]*Type{
			"Option":   {Name: "Option", Kind: "func()"},
			"Metadata": {Name: "Metadata", Kind: "struct{}"},
		},
		Interfaces: map[string]*Interface{
			"Handler": {Name: "Handler", Methods: []string{"HandleWithContext() error", "Drain() error"}},
			"Writer":  {Name: "Writer", Methods: []string{"Write() error"}},
			"Reader":  {Name: "Reader", Methods: []string{"Read() error"}},
		},
	}
	usage := &Usage{
		Symbols: map[string][]Location{
			"Alpha":   {{File: "main.go", Line: 10}},
			"Bravo":   {{File: "main.go", Line: 20}},
			"Charlie": {{File: "util.go", Line: 5}},
			"Config":  {{File: "config.go", Line: 7}},
			"Handler": {{File: "handler.go", Line: 3}},
		},
	}

	first := diffAPIs(oldAPI, newAPI, usage)
	for i := 0; i < 20; i++ {
		got := diffAPIs(oldAPI, newAPI, usage)
		if !reflect.DeepEqual(first, got) {
			t.Fatalf("diffAPIs() output differs between runs\nrun 1: %+v\nrun %d: %+v", first, i+2, got)
		}
	}

	// Sanity check that the fixture actually exercises every collection
	if len(first.Removed) == 0 || len(first.Added) == 0 || len(first.Changed) == 0 || len(first.InterfaceChanges) == 0 {
		t.Fatalf("fixture did not produce all change kinds: %+v", first)
	}
}

func TestDiffAPIsMethodRemoval(t *testing.T) {
	const pkgPath = "example.com/lib"

	method := &Function{
		Name: "Validate", Recv: "Config", Signature: "func() error",
		PkgPath: pkgPath, PkgName: "lib", IsMethod: true,
	}
	configType := &Type{Name: "Config", Kind: "struct{}", PkgPath: pkgPath, PkgName: "lib"}

	usage := &Usage{
		Symbols: map[string][]Location{
			"example.com/lib.Config.Validate": {{File: "main.go", Line: 5}},
			"example.com/lib.Config":          {{File: "main.go", Line: 5}},
		},
	}

	t.Run("method removed while its type survives", func(t *testing.T) {
		oldAPI := &API{
			Funcs: map[string]*Function{"example.com/lib.Config.Validate": method},
			Types: map[string]*Type{"example.com/lib.Config": configType},
		}
		newAPI := &API{
			Funcs: map[string]*Function{},
			Types: map[string]*Type{"example.com/lib.Config": configType},
		}

		diff := diffAPIs(oldAPI, newAPI, usage)

		if len(diff.Removed) != 1 {
			t.Fatalf("expected 1 removed symbol, got %d: %+v", len(diff.Removed), diff.Removed)
		}
		if diff.Removed[0].Name != "lib.Config.Validate" || diff.Removed[0].Type != "method" {
			t.Fatalf("expected the method to be reported, got %+v", diff.Removed[0])
		}
	})

	t.Run("method removed along with its type", func(t *testing.T) {
		oldAPI := &API{
			Funcs: map[string]*Function{"example.com/lib.Config.Validate": method},
			Types: map[string]*Type{"example.com/lib.Config": configType},
		}
		newAPI := &API{Funcs: map[string]*Function{}, Types: map[string]*Type{}}

		diff := diffAPIs(oldAPI, newAPI, usage)

		// The type's own entry covers the break; listing every method of a
		// deleted type would count one break many times over
		if len(diff.Removed) != 1 {
			t.Fatalf("expected 1 removed symbol, got %d: %+v", len(diff.Removed), diff.Removed)
		}
		if diff.Removed[0].Name != "lib.Config" || diff.Removed[0].Type != "type" {
			t.Fatalf("expected the type to be reported, got %+v", diff.Removed[0])
		}
	})
}

func TestDiffAPIsIgnoresParameterNames(t *testing.T) {
	usage := &Usage{
		Symbols: map[string][]Location{
			"example.com/lib.Get":     {{File: "main.go", Line: 3}},
			"example.com/lib.Handler": {{File: "main.go", Line: 4}},
		},
	}
	oldAPI := &API{
		Funcs: map[string]*Function{
			"example.com/lib.Get": {Name: "Get", Signature: "func(key string) error", SignatureKey: "(string) (error)"},
		},
		Interfaces: map[string]*Interface{
			"example.com/lib.Handler": {
				Name: "Handler", Methods: []string{"Handle(req string) error"}, MethodKeys: []string{"Handle(string) (error)"},
			},
		},
	}
	newAPI := &API{
		Funcs: map[string]*Function{
			"example.com/lib.Get": {Name: "Get", Signature: "func(k string) error", SignatureKey: "(string) (error)"},
		},
		Interfaces: map[string]*Interface{
			"example.com/lib.Handler": {
				Name: "Handler", Methods: []string{"Handle(r string) error"}, MethodKeys: []string{"Handle(string) (error)"},
			},
		},
	}

	diff := diffAPIs(oldAPI, newAPI, usage)

	// Renaming parameters breaks no caller and no implementer
	if len(diff.Changed) != 0 || len(diff.InterfaceChanges) != 0 || len(diff.Removed) != 0 {
		t.Fatalf("parameter renames should not be reported, got %+v", diff)
	}
}

func TestPackageQualifier(t *testing.T) {
	tests := []struct {
		module, pkgPath, pkgName, want string
	}{
		{"example.com/lib", "example.com/lib", "lib", "lib"},
		{"example.com/api", "example.com/api/foo/v1", "v1", "foo/v1"},
		{"example.com/api", "example.com/api/bar/v1", "v1", "bar/v1"},
		{"golang.org/x/exp", "golang.org/x/exp/slices", "slices", "slices"},
		{"example.com/lib/v2", "example.com/lib/v2/sub", "sub", "sub"},
		// A path that merely shares a prefix is not inside the module
		{"example.com/lib", "example.com/library", "library", "library"},
	}
	for _, tt := range tests {
		if got := packageQualifier(tt.module, tt.pkgPath, tt.pkgName); got != tt.want {
			t.Errorf("packageQualifier(%q, %q, %q) = %q, want %q", tt.module, tt.pkgPath, tt.pkgName, got, tt.want)
		}
	}
}

func TestDiffAPIsOrdersSameDisplayNameByPackage(t *testing.T) {
	// Two symbols that render identically still get a stable order
	oldAPI := &API{
		Funcs: map[string]*Function{
			"example.com/a/v1.Config": {Name: "Config", PkgName: "v1", PkgPath: "example.com/a/v1", Signature: "func()"},
			"example.com/b/v1.Config": {Name: "Config", PkgName: "v1", PkgPath: "example.com/b/v1", Signature: "func()"},
		},
	}
	newAPI := &API{Funcs: map[string]*Function{}}
	usage := &Usage{Symbols: map[string][]Location{
		"example.com/a/v1.Config": {{File: "main.go", Line: 1}},
		"example.com/b/v1.Config": {{File: "main.go", Line: 2}},
	}}

	for i := 0; i < 20; i++ {
		diff := diffAPIs(oldAPI, newAPI, usage)
		if len(diff.Removed) != 2 || diff.Removed[0].Package != "example.com/a/v1" || diff.Removed[1].Package != "example.com/b/v1" {
			t.Fatalf("run %d: removed not ordered by package: %+v", i, diff.Removed)
		}
	}
}
