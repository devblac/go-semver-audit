package analyzer

import (
	"testing"
)

func TestParseUpgrade(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    *Upgrade
		wantErr bool
	}{
		{
			name: "valid upgrade",
			spec: "github.com/pkg/errors@v0.9.1",
			want: &Upgrade{
				Module:     "github.com/pkg/errors",
				NewVersion: "v0.9.1",
			},
			wantErr: false,
		},
		{
			name: "valid upgrade with subdomain",
			spec: "golang.org/x/tools@v0.16.0",
			want: &Upgrade{
				Module:     "golang.org/x/tools",
				NewVersion: "v0.16.0",
			},
			wantErr: false,
		},
		{
			name:    "missing version",
			spec:    "github.com/pkg/errors",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing module",
			spec:    "@v0.9.1",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty string",
			spec:    "",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "multiple @ symbols",
			spec:    "github.com/pkg/errors@v0.9.1@extra",
			want:    nil,
			wantErr: true,
		},
		{
			name: "whitespace handling",
			spec: "  github.com/pkg/errors@v0.9.1  ",
			want: &Upgrade{
				Module:     "github.com/pkg/errors",
				NewVersion: "v0.9.1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUpgrade(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseUpgrade() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Module != tt.want.Module || got.NewVersion != tt.want.NewVersion {
					t.Errorf("ParseUpgrade() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestResultHasBreakingChanges(t *testing.T) {
	tests := []struct {
		name   string
		result *Result
		want   bool
	}{
		{
			name: "no changes",
			result: &Result{
				Changes: &Diff{},
			},
			want: false,
		},
		{
			name: "only additions",
			result: &Result{
				Changes: &Diff{
					Added: []AddedSymbol{{Name: "NewFunc", Type: "function"}},
				},
			},
			want: false,
		},
		{
			name: "removed function",
			result: &Result{
				Changes: &Diff{
					Removed: []RemovedSymbol{{Name: "OldFunc", Type: "function"}},
				},
			},
			want: true,
		},
		{
			name: "changed signature",
			result: &Result{
				Changes: &Diff{
					Changed: []ChangedSignature{{Name: "Func", OldSignature: "old", NewSignature: "new"}},
				},
			},
			want: true,
		},
		{
			name: "interface change",
			result: &Result{
				Changes: &Diff{
					InterfaceChanges: []InterfaceChange{{Name: "IFace", RemovedMethods: []string{"Method"}}},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasBreakingChanges(); got != tt.want {
				t.Errorf("Result.HasBreakingChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultHasWarnings(t *testing.T) {
	tests := []struct {
		name   string
		result *Result
		want   bool
	}{
		{
			name: "no warnings",
			result: &Result{
				Changes: &Diff{},
			},
			want: false,
		},
		{
			name: "additions",
			result: &Result{
				Changes: &Diff{
					Added: []AddedSymbol{{Name: "NewFunc", Type: "function"}},
				},
			},
			want: true,
		},
		{
			name: "unused dependencies",
			result: &Result{
				Changes:    &Diff{},
				UnusedDeps: []string{"github.com/unused/dep"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasWarnings(); got != tt.want {
				t.Errorf("Result.HasWarnings() = %v, want %v", got, tt.want)
			}
		})
	}
}
