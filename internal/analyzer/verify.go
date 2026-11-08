package analyzer

import (
	"bytes"
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Allow overriding in tests
var goGet = runGoGet

// VerifyBuild type-checks the project, test files included, against
// module@version and returns the compile errors it produces. This catches
// breaks the API diff cannot see, such as a field added to a struct the
// project builds with an unkeyed literal.
//
// The upgrade is applied to copies of go.mod and go.sum in a temp dir, handed
// to the go command with -modfile (which also redirects go.sum), so the
// project's files are never written.
func (a *Analyzer) VerifyBuild(module, version string) ([]BuildError, error) {
	workDir, err := os.MkdirTemp("", "go-semver-audit-verify-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	modFile, err := copyModFiles(a.goModPath(), workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to copy go.mod: %w", err)
	}

	spec := fmt.Sprintf("%s@%s", module, version)
	// go get rather than editing the require line: it also raises the
	// requirements of the new version, as the real upgrade would, and those
	// can break the project too
	if err := goGet(a.projectPath, modFile, spec); err != nil {
		return nil, err
	}

	cfg := &packages.Config{
		// The same mode as loadProject, so dependencies are type-checked from
		// source rather than compiled for export data, and nothing more than
		// the analysis itself needs (such as a C toolchain) is required
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports |
			packages.NeedDeps | packages.NeedTypes | packages.NeedSyntax |
			packages.NeedTypesInfo | packages.NeedModule,
		Dir:        a.projectPath,
		Tests:      true,
		BuildFlags: []string{"-mod=mod", "-modfile=" + modFile},
		Env:        append(os.Environ(), "GOWORK=off"),
	}
	pkgs, err := packagesLoad(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("failed to load project with %s: %w", spec, err)
	}

	return a.collectBuildErrors(pkgs), nil
}

// goModPath returns the project's go.mod, which may be in a parent directory
// of the path being analyzed
func (a *Analyzer) goModPath() string {
	for _, pkg := range a.pkgs {
		if pkg.Module != nil && pkg.Module.Main && pkg.Module.GoMod != "" {
			return pkg.Module.GoMod
		}
	}
	return filepath.Join(a.projectPath, "go.mod")
}

// copyModFiles copies goMod, and the go.sum beside it if there is one, into
// workDir and returns the path of the copy
func copyModFiles(goMod, workDir string) (string, error) {
	content, err := os.ReadFile(goMod)
	if err != nil {
		return "", err
	}
	modFile := filepath.Join(workDir, "go.mod")
	if err := os.WriteFile(modFile, content, 0o644); err != nil {
		return "", err
	}

	sum, err := os.ReadFile(filepath.Join(filepath.Dir(goMod), "go.sum"))
	if os.IsNotExist(err) {
		return modFile, nil
	} else if err != nil {
		return "", err
	}
	return modFile, os.WriteFile(filepath.Join(workDir, "go.sum"), sum, 0o644)
}

// runGoGet upgrades spec in modFile (and the go.sum beside it), resolving the
// project's module graph from dir
func runGoGet(dir, modFile, spec string) error {
	cmd := exec.Command("go", "get", "-modfile="+modFile, spec)
	cmd.Dir = dir
	// -modfile is not allowed in workspace mode
	cmd.Env = append(os.Environ(), "GOWORK=off")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go get %s: %v: %s", spec, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// collectBuildErrors gathers the errors of the project's packages and their
// dependencies. With tests loaded, a package's regular files are type-checked
// twice (once on their own, once in the test variant), so repeats are dropped.
func (a *Analyzer) collectBuildErrors(pkgs []*packages.Package) []BuildError {
	var errs []BuildError
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		for _, e := range pkg.Errors {
			file, line, col := splitErrorPos(e.Pos)
			if file != "" {
				file = a.relativePath(file)
			}
			errs = append(errs, BuildError{File: file, Line: line, Col: col, Message: e.Msg})
		}
	})

	slices.SortFunc(errs, func(x, y BuildError) int {
		return cmp.Or(
			cmp.Compare(x.File, y.File),
			cmp.Compare(x.Line, y.Line),
			cmp.Compare(x.Col, y.Col),
			cmp.Compare(x.Message, y.Message),
		)
	})
	return slices.Compact(errs)
}

// splitErrorPos splits a packages.Error position, "file:line:col",
// "file:line", "file" or "" (or "-"), into its parts. It works from the right
// so a Windows drive letter is not mistaken for a separator.
func splitErrorPos(pos string) (file string, line, col int) {
	if pos == "" || pos == "-" {
		return "", 0, 0
	}

	var nums []int
	rest := pos
	for range 2 {
		i := strings.LastIndexByte(rest, ':')
		if i < 0 {
			break
		}
		n, err := strconv.Atoi(rest[i+1:])
		if err != nil {
			break
		}
		nums = append(nums, n)
		rest = rest[:i]
	}

	switch len(nums) {
	case 2:
		return rest, nums[1], nums[0]
	case 1:
		return rest, nums[0], 0
	}
	return pos, 0, 0
}
