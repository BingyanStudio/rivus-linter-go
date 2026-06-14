package analyzer

import (
	"go/token"
	"strings"

	"golang.org/x/tools/go/ssa"

	"github.com/BingyanStudio/rivus-linter-go/internal/model"
)

// ScanFunc analyzes a single SSA function and returns its own flags
// (not including flags from called functions).
func ScanFunc(fn *ssa.Function, fset *token.FileSet) []model.Flag {
	if fn == nil || fn.Blocks == nil {
		return nil
	}

	var flags []model.Flag

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			flags = append(flags, scanInstruction(instr, fset)...)
		}
	}

	return flags
}

// scanInstruction checks a single SSA instruction for side effects.
func scanInstruction(instr ssa.Instruction, fset *token.FileSet) []model.Flag {
	var flags []model.Flag
	pos := model.PositionFromToken(fset, instr.Pos())

	switch i := instr.(type) {
	case *ssa.Call:
		flags = append(flags, scanCall(i, pos)...)

	case *ssa.Panic:
		flags = append(flags, model.Flag{Type: model.FlagPanic, Position: pos})

	case *ssa.Go:
		flags = append(flags, model.Flag{Type: model.FlagGoroutine, Position: pos})

	case *ssa.MakeInterface:
		// Check if the value being boxed is from an unsafe package.
		// This is handled by the call scanner when the source is an unsafe function.

	case *ssa.FieldAddr:
		// FieldAddr on an unsafe.Pointer-typed value would be flagged
		// via the call chain. No direct check needed here.

	case *ssa.MapUpdate:
		// Writing to a global map would be caught by Global detection.

	case *ssa.Store:
		// Check if storing to a global variable.
		if addr, ok := i.Addr.(*ssa.Global); ok {
			_ = addr // Global store detected; flagged via Global scan.
		}
	}

	return flags
}

// scanCall checks a call instruction for side effects.
func scanCall(call *ssa.Call, pos model.Position) []model.Flag {
	var flags []model.Flag

	// Get the called function name.
	name := callName(call)

	// Check stdlib mapping first.
	if stdFlags, ok := LookupStdFlags(name); ok {
		for _, ft := range model.AllFlags() {
			if stdFlags.Has(ft) {
				flags = append(flags, model.Flag{Type: ft, Position: pos})
			}
		}
		return flags
	}

	// Check for panic calls (runtime.gopanic or builtin panic).
	if isPanicCall(call) {
		flags = append(flags, model.Flag{Type: model.FlagPanic, Position: pos})
	}

	// Check for unsafe operations.
	if isUnsafeCall(name) {
		flags = append(flags, model.Flag{Type: model.FlagUnsafe, Position: pos})
	}

	return flags
}

// callName returns the fully qualified name of the called function.
func callName(call *ssa.Call) string {
	if call.Call.StaticCallee() != nil {
		fn := call.Call.StaticCallee()
		if fn.Pkg != nil {
			return fn.Pkg.Pkg.Path() + "." + fn.Name()
		}
		return fn.Name()
	}
	return ""
}

// isPanicCall checks if a call is a panic.
func isPanicCall(call *ssa.Call) bool {
	// Check for builtin panic.
	if call.Call.IsInvoke() {
		return false
	}
	if fn := call.Call.StaticCallee(); fn != nil {
		if fn.Name() == "gopanic" && fn.Pkg != nil && fn.Pkg.Pkg.Path() == "runtime" {
			return true
		}
	}
	return false
}

// isUnsafeCall checks if a function name belongs to the unsafe package.
func isUnsafeCall(name string) bool {
	return strings.HasPrefix(name, "unsafe.")
}

// ScanGlobals checks a function for reads/writes to global variables.
func ScanGlobals(fn *ssa.Function, fset *token.FileSet) []model.Flag {
	if fn == nil || fn.Blocks == nil {
		return nil
	}

	var flags []model.Flag
	seen := make(map[*ssa.Global]bool)

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			switch i := instr.(type) {
			case *ssa.Store:
				if g, ok := i.Addr.(*ssa.Global); ok && !seen[g] {
					seen[g] = true
					pos := model.PositionFromToken(fset, instr.Pos())
					flags = append(flags, model.Flag{Type: model.FlagSideEffect, Position: pos})
				}
			case *ssa.UnOp:
				if g, ok := i.X.(*ssa.Global); ok && !seen[g] {
					seen[g] = true
					pos := model.PositionFromToken(fset, instr.Pos())
					flags = append(flags, model.Flag{Type: model.FlagSideEffect, Position: pos})
				}
			case *ssa.FieldAddr:
				if g, ok := i.X.(*ssa.Global); ok && !seen[g] {
					seen[g] = true
					pos := model.PositionFromToken(fset, instr.Pos())
					flags = append(flags, model.Flag{Type: model.FlagSideEffect, Position: pos})
				}
			case *ssa.IndexAddr:
				if g, ok := i.X.(*ssa.Global); ok && !seen[g] {
					seen[g] = true
					pos := model.PositionFromToken(fset, instr.Pos())
					flags = append(flags, model.Flag{Type: model.FlagSideEffect, Position: pos})
				}
			}
		}
	}

	return flags
}

// ScanCGO checks if a function uses CGO.
func ScanCGO(fn *ssa.Function) bool {
	if fn == nil || fn.Pkg == nil {
		return false
	}
	// CGO functions are in the "C" pseudo-package.
	return fn.Pkg.Pkg.Path() == "C"
}
