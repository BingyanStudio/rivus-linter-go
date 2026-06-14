package analyzer

import (
	"fmt"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// SSAProgram holds the SSA program and all packages built from it.
type SSAProgram struct {
	Program  *ssa.Program
	Packages []*ssa.Package
	// AllPackages maps package import path to *ssa.Package.
	AllPackages map[string]*ssa.Package
}

// BuildSSA constructs the SSA program from the given loader packages.
// mode controls how much SSA is built (ssa.BuilderMode).
func BuildSSA(pkgs []*packages.Package, mode ssa.BuilderMode) (*SSAProgram, error) {
	prog, ssaPkgs := ssautil.AllPackages(pkgs, mode)

	// Build all packages.
	for _, pkg := range ssaPkgs {
		if pkg != nil {
			pkg.Build()
		}
	}

	allPkgs := make(map[string]*ssa.Package)
	for _, pkg := range ssaPkgs {
		if pkg != nil {
			allPkgs[pkg.Pkg.Path()] = pkg
		}
	}

	return &SSAProgram{
		Program:     prog,
		Packages:    ssaPkgs,
		AllPackages: allPkgs,
	}, nil
}

// LoadAndBuildSSA loads Go packages and builds SSA in one step.
// patterns are package patterns like "./..." or "github.com/foo/bar".
// dir is the working directory.
func LoadAndBuildSSA(dir string, patterns []string) ([]*packages.Package, *SSAProgram, error) {
	cfg := &packages.Config{
		Dir:  dir,
		Mode: packages.LoadAllSyntax | packages.LoadFiles,
	}

	initial, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, nil, fmt.Errorf("loading packages: %w", err)
	}

	if packages.PrintErrors(initial) > 0 {
		return nil, nil, fmt.Errorf("packages had errors")
	}

	prog, err := BuildSSA(initial, ssa.SanityCheckFunctions)
	if err != nil {
		return nil, nil, fmt.Errorf("building SSA: %w", err)
	}

	return initial, prog, nil
}
