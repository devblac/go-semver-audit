package analyzer

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// TestAnalyzeInterfaceImplementedVsCalled checks that interface changes are
// reported only where they break: added methods at the project's own
// implementations, removed methods at their calls.
func TestAnalyzeInterfaceImplementedVsCalled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	const libPath = "example.com/ifaces"
	root := t.TempDir()
	proxyDir := filepath.Join(root, "proxy")

	publishModule(t, proxyDir, libPath, "v1.0.0", map[string]string{
		"ifaces.go": `package ifaces

type Handler interface{ Handle() error }

func Serve(h Handler) {}

type Store interface{ Get(key string) string }

func NewStore() Store { return nil }

type Logger interface{ Log(msg string) }

type Base struct{}

func (Base) Handle() error { return nil }

type Reader interface {
	Read() string
	Old() string
}

func NewReader() Reader { return nil }
`,
	})
	// Every interface gains or loses a method
	publishModule(t, proxyDir, libPath, "v1.1.0", map[string]string{
		"ifaces.go": `package ifaces

type Handler interface {
	Handle() error
	Close() error
}

func Serve(h Handler) {}

type Store interface {
	Get(key string) string
	Put(key, value string)
}

func NewStore() Store { return nil }

type Logger interface {
	Log(msg string)
	Flush()
}

type Base struct{}

func (Base) Handle() error { return nil }

func (Base) Close() error { return nil }

type Reader interface{ Read() string }

func NewReader() Reader { return nil }
`,
	})

	projectDir := filepath.Join(root, "app")
	writeFiles(t, projectDir, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.21\n\nrequire " + libPath + " v1.0.0\n",
		"main.go": `package main

import "example.com/ifaces"

// Implements Handler with a method of its own: breaks when Close is added
type myHandler struct{}

func (myHandler) Handle() error { return nil }

// Gets Handler's methods from the library's Base, which grows Close too
type based struct{ ifaces.Base }

// Gets Logger's methods by embedding it, so Flush comes along
type wrapped struct{ ifaces.Logger }

func main() {
	ifaces.Serve(myHandler{})
	ifaces.Serve(based{})
	_ = wrapped{}

	// Only called: Store values come from the library
	s := ifaces.NewStore()
	_ = s.Get("k")

	// Old is removed, and is called here
	r := ifaces.NewReader()
	_ = r.Read() + r.Old()
}
`,
		"main_test.go": `package main

import (
	"testing"

	"example.com/ifaces"
)

type fakeHandler struct{}

func (*fakeHandler) Handle() error { return nil }

func TestServe(t *testing.T) {
	ifaces.Serve(&fakeHandler{})
}
`,
	})

	useOfflineProxy(t, proxyDir)

	a, err := New(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.Analyze(&Upgrade{Module: libPath, NewVersion: "v1.1.0"})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	got := map[string]InterfaceChange{}
	for _, c := range result.Changes.InterfaceChanges {
		got[c.Name] = c
	}

	handler, ok := got["ifaces.Handler"]
	if !ok {
		t.Fatalf("expected ifaces.Handler to be reported for its implementations, got %+v", result.Changes.InterfaceChanges)
	}
	// myHandler and the pointer-receiver fake from the test file, but not
	// based, which gets Close from ifaces.Base
	wantImpl := []Location{{File: "main.go", Line: 6}, {File: "main_test.go", Line: 9}}
	if !reflect.DeepEqual(handler.ImplementedIn, wantImpl) || !reflect.DeepEqual(handler.UsedIn, wantImpl) {
		t.Errorf("ifaces.Handler ImplementedIn = %v, UsedIn = %v, want both %v", handler.ImplementedIn, handler.UsedIn, wantImpl)
	}

	reader, ok := got["ifaces.Reader"]
	if !ok {
		t.Fatalf("expected ifaces.Reader to be reported for the call to Old, got %+v", result.Changes.InterfaceChanges)
	}
	if len(reader.UsedIn) != 1 || reader.UsedIn[0].Line != 27 || len(reader.ImplementedIn) != 0 {
		t.Errorf("ifaces.Reader UsedIn = %v, ImplementedIn = %v, want only the call at main.go:27", reader.UsedIn, reader.ImplementedIn)
	}

	for _, name := range []string{"ifaces.Store", "ifaces.Logger"} {
		if c, ok := got[name]; ok {
			t.Errorf("%s gained a method but nothing in the project implements it, yet it was reported: %+v", name, c)
		}
	}
}
