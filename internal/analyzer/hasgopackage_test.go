package analyzer

import (
	"path/filepath"
	"testing"
)

func TestHasGoPackage(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		dir   string // relative to the module root
		want  bool
	}{
		{
			name:  "package with Go files",
			files: map[string]string{"lib.go": "package lib\n"},
			want:  true,
		},
		{
			name:  "missing directory",
			files: map[string]string{"lib.go": "package lib\n"},
			dir:   "gone",
			want:  false,
		},
		{
			// The case that used to abort the analysis: the root directory
			// survives because of subpackages and non-Go files
			name:  "root package removed, subpackage and README remain",
			files: map[string]string{"README.md": "# lib\n", "sub/sub.go": "package sub\n"},
			want:  false,
		},
		{
			name:  "only test files left",
			files: map[string]string{"lib_test.go": "package lib\n"},
			want:  false,
		},
		{
			name:  "all files excluded by build constraints",
			files: map[string]string{"lib.go": "//go:build ignore\n\npackage lib\n"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			moduleDir := t.TempDir()
			writeFiles(t, moduleDir, tt.files)

			if got := hasGoPackage(filepath.Join(moduleDir, tt.dir)); got != tt.want {
				t.Errorf("hasGoPackage() = %v, want %v", got, tt.want)
			}
		})
	}
}
