package report

import (
	"strings"
	"testing"

	"github.com/devblac/go-semver-audit/internal/analyzer"
)

// TestMajorVersionHeader checks that every format names both module paths
// when a major version upgrade changes the path, and only then
func TestMajorVersionHeader(t *testing.T) {
	major := &analyzer.Result{
		Module:     "example.com/lib/v2",
		OldModule:  "example.com/lib",
		OldVersion: "v1.4.0",
		NewVersion: "v2.0.0",
		Changes:    &analyzer.Diff{},
	}
	same := &analyzer.Result{
		Module:     "example.com/lib",
		OldVersion: "v1.4.0",
		NewVersion: "v1.5.0",
		Changes:    &analyzer.Diff{},
	}

	formats := map[string]struct {
		format     func(*analyzer.Result) (string, error)
		wantMajor  string
		wantSingle string
	}{
		"text": {
			func(r *analyzer.Result) (string, error) { return FormatText(r, false) },
			"Analyzing upgrade: example.com/lib v1.4.0 -> example.com/lib/v2 v2.0.0\n",
			"Analyzing upgrade: example.com/lib v1.4.0 -> v1.5.0\n",
		},
		"markdown": {
			FormatMarkdown,
			"### `example.com/lib` v1.4.0 → `example.com/lib/v2` v2.0.0\n",
			"### `example.com/lib` v1.4.0 → v1.5.0\n",
		},
		"html": {
			FormatHTML,
			"<title>go-semver-audit: example.com/lib v1.4.0 → example.com/lib/v2 v2.0.0</title>",
			"<title>go-semver-audit: example.com/lib v1.4.0 → v1.5.0</title>",
		},
		"json": {
			FormatJSON,
			`"old_module": "example.com/lib",`,
			`"module": "example.com/lib",`,
		},
	}
	for name, f := range formats {
		out, err := f.format(major)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(out, f.wantMajor) {
			t.Errorf("%s major version output missing %q:\n%s", name, f.wantMajor, out)
		}

		out, err = f.format(same)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(out, f.wantSingle) {
			t.Errorf("%s output missing %q:\n%s", name, f.wantSingle, out)
		}
		if strings.Contains(out, "old_module") {
			t.Errorf("%s output names old_module although the path did not change:\n%s", name, out)
		}
	}
}
