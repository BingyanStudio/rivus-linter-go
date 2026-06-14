package analyzer

import (
	"go/token"
	"testing"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/BingyanStudio/rivus-linter-go/internal/model"
)

// loadTestSSA builds SSA from testdata directory.
func loadTestSSA(t *testing.T) (*ssa.Program, []*ssa.Package) {
	t.Helper()
	cfg := &packages.Config{
		Dir:  ".",
		Mode: packages.LoadAllSyntax,
	}
	initial, err := packages.Load(cfg, "../../testdata")
	if err != nil {
		t.Fatalf("failed to load packages: %v", err)
	}
	prog, pkgs := ssautil.AllPackages(initial, 0)
	for _, pkg := range pkgs {
		if pkg != nil {
			pkg.Build()
		}
	}
	return prog, pkgs
}

func findFunc(t *testing.T, pkgs []*ssa.Package, name string) *ssa.Function {
	t.Helper()
	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		for _, mem := range pkg.Members {
			if fn, ok := mem.(*ssa.Function); ok && fn.Name() == name {
				return fn
			}
		}
	}
	t.Fatalf("function %q not found", name)
	return nil
}

func hasFlag(flags []model.Flag, ft model.FlagType) bool {
	for _, f := range flags {
		if f.Type == ft {
			return true
		}
	}
	return false
}

func TestScanPanic(t *testing.T) {
	_, pkgs := loadTestSSA(t)
	fn := findFunc(t, pkgs, "PanicDirect")
	flags := ScanFunc(fn, token.NewFileSet())
	if !hasFlag(flags, model.FlagPanic) {
		t.Error("expected FlagPanic for PanicDirect")
	}
}

func TestScanGoroutine(t *testing.T) {
	_, pkgs := loadTestSSA(t)
	fn := findFunc(t, pkgs, "LaunchGoroutine")
	flags := ScanFunc(fn, token.NewFileSet())
	if !hasFlag(flags, model.FlagGoroutine) {
		t.Error("expected FlagGoroutine for LaunchGoroutine")
	}
}

func TestScanIO(t *testing.T) {
	_, pkgs := loadTestSSA(t)
	tests := []struct {
		name string
		want model.FlagType
	}{
		{"ReadFile", model.FlagIO},
		{"DialNetwork", model.FlagIO},
	}
	for _, tt := range tests {
		fn := findFunc(t, pkgs, tt.name)
		flags := ScanFunc(fn, token.NewFileSet())
		if !hasFlag(flags, tt.want) {
			t.Errorf("expected %v for %s", tt.want, tt.name)
		}
	}
}

func TestScanSideEffect(t *testing.T) {
	_, pkgs := loadTestSSA(t)
	tests := []struct {
		name string
		want model.FlagType
	}{
		{"ReadEnv", model.FlagSideEffect},
		{"UseRandom", model.FlagSideEffect},
	}
	for _, tt := range tests {
		fn := findFunc(t, pkgs, tt.name)
		flags := ScanFunc(fn, token.NewFileSet())
		if !hasFlag(flags, tt.want) {
			t.Errorf("expected %v for %s", tt.want, tt.name)
		}
	}
}

func TestScanPure(t *testing.T) {
	_, pkgs := loadTestSSA(t)
	tests := []string{"Add", "Greet"}
	for _, name := range tests {
		fn := findFunc(t, pkgs, name)
		flags := ScanFunc(fn, token.NewFileSet())
		if len(flags) > 0 {
			t.Errorf("expected no flags for %s, got %v", name, flags)
		}
	}
}
