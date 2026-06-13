# Go Side-Effect Checker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go tool that analyzes functions using SSA to detect side effects (panic, goroutine, I/O, etc.), with caching, JSON/table output, and a public API for future VSCode integration.

**Architecture:** Two-phase analysis — build SSA + Call Graph, then bottom-up traversal to mark flags per function. Flags are bitmasks (efficient). Scanner functions walk SSA instructions independently. Cache uses file-content hashing persisted to disk.

**Tech Stack:** Go 1.26+, `golang.org/x/tools/go/ssa`, `golang.org/x/tools/go/packages`, `golang.org/x/tools/go/callgraph`

---

## File Map

| File | Responsibility |
|------|----------------|
| `internal/model/flag.go` | FlagType constants, Flag struct, bitmask operations |
| `internal/model/position.go` | Position struct, conversion from token.Pos |
| `internal/model/result.go` | FuncResult, PackageResult, AnalysisResult structs |
| `internal/analyzer/ssa.go` | SSA construction from packages |
| `internal/analyzer/callgraph.go` | Call graph construction, topological sort, SCC detection |
| `internal/analyzer/scanner.go` | Side-effect scanners (one per flag type) |
| `internal/analyzer/analyzer.go` | Orchestrator: load packages → build SSA → scan → aggregate |
| `internal/analyzer/stdmap.go` | Predefined flag mapping for standard library functions |
| `internal/analyzer/cache.go` | File-hash-based persistent cache |
| `internal/output/json.go` | JSON formatter |
| `internal/output/table.go` | Table formatter |
| `internal/config/config.go` | Configuration struct and loading |
| `pkg/rivus/api.go` | Public Analyzer interface for VSCode extension |
| `cmd/rivus/main.go` | CLI entry point |
| `internal/model/flag_test.go` | Flag unit tests |
| `internal/analyzer/scanner_test.go` | Scanner tests with testdata |
| `internal/analyzer/cache_test.go` | Cache roundtrip tests |
| `internal/output/table_test.go` | Table output golden tests |
| `internal/output/json_test.go` | JSON output tests |
| `testdata/panic.go` | Test fixture: panic calls |
| `testdata/goroutine.go` | Test fixture: goroutine launches |
| `testdata/io.go` | Test fixture: I/O operations |
| `testdata/sideeffect.go` | Test fixture: global vars, env, random |
| `testdata/unsafe.go` | Test fixture: unsafe usage |
| `testdata/time.go` | Test fixture: time operations |
| `testdata/exit.go` | Test fixture: os.Exit, log.Fatal |
| `testdata/reflect.go` | Test fixture: reflect usage |
| `testdata/context.go` | Test fixture: dangling context |
| `testdata/pure.go` | Test fixture: pure functions (no flags) |
| `testdata/inherit.go` | Test fixture: flag inheritance |

---

### Task 1: Project Setup and Dependencies

**Files:**
- Modify: `go.mod`
- Create: `internal/model/flag.go`
- Create: `internal/model/flag_test.go`

- [ ] **Step 1: Add dependencies to go.mod**

```bash
cd /home/paul/rivus-linter-go
go get golang.org/x/tools/go/ssa
go get golang.org/x/tools/go/packages
go get golang.org/x/tools/go/callgraph
go get golang.org/x/tools/go/ssa/ssautil
```

- [ ] **Step 2: Create the FlagType constants and Flag struct**

Create `internal/model/flag.go`:

```go
package model

// FlagType represents a side-effect category.
type FlagType uint8

const (
    FlagPanic      FlagType = 1 << iota // P
    FlagGoroutine                       // G
    FlagContext                         // D
    FlagIO                             // I
    FlagSideEffect                     // S
    FlagUnsafe                         // U
    FlagTime                           // T
    FlagExit                           // X
    FlagReflect                        // R
    FlagCGO                            // C
)

// flagNames maps each FlagType to its single-letter name.
var flagNames = map[FlagType]string{
    FlagPanic:      "P",
    FlagGoroutine:  "G",
    FlagContext:    "D",
    FlagIO:        "I",
    FlagSideEffect: "S",
    FlagUnsafe:    "U",
    FlagTime:      "T",
    FlagExit:      "X",
    FlagReflect:   "R",
    FlagCGO:       "C",
}

// String returns the single-letter name for a FlagType.
func (f FlagType) String() string {
    if name, ok := flagNames[f]; ok {
        return name
    }
    return "?"
}

// FlagSet is a bitmask of FlagType values.
type FlagSet uint16

// Has reports whether the set contains the given flag.
func (s FlagSet) Has(f FlagType) bool {
    return s&FlagSet(f) != 0
}

// Add adds a flag to the set and returns the new set.
func (s FlagSet) Add(f FlagType) FlagSet {
    return s | FlagSet(f)
}

// Union returns the union of two flag sets.
func (s FlagSet) Union(other FlagSet) FlagSet {
    return s | other
}

// IsEmpty reports whether the set has no flags.
func (s FlagSet) IsEmpty() bool {
    return s == 0
}

// String returns a comma-separated list of flag names.
func (s FlagSet) String() string {
    if s == 0 {
        return ""
    }
    var names []string
    for f, name := range flagNames {
        if s.Has(f) {
            names = append(names, name)
        }
    }
    // Simple sort by name for deterministic output.
    for i := 0; i < len(names); i++ {
        for j := i + 1; j < len(names); j++ {
            if names[j] < names[i] {
                names[i], names[j] = names[j], names[i]
            }
        }
    }
    result := ""
    for i, n := range names {
        if i > 0 {
            result += ", "
        }
        result += n
    }
    return result
}

// AllFlags returns a slice of all defined FlagType values.
func AllFlags() []FlagType {
    return []FlagType{
        FlagPanic, FlagGoroutine, FlagContext, FlagIO, FlagSideEffect,
        FlagUnsafe, FlagTime, FlagExit, FlagReflect, FlagCGO,
    }
}
```

- [ ] **Step 3: Write flag unit tests**

Create `internal/model/flag_test.go`:

```go
package model

import "testing"

func TestFlagSetAddHas(t *testing.T) {
    var s FlagSet
    s = s.Add(FlagPanic)
    s = s.Add(FlagIO)

    if !s.Has(FlagPanic) {
        t.Error("expected FlagPanic")
    }
    if !s.Has(FlagIO) {
        t.Error("expected FlagIO")
    }
    if s.Has(FlagGoroutine) {
        t.Error("unexpected FlagGoroutine")
    }
}

func TestFlagSetUnion(t *testing.T) {
    a := FlagSet(0).Add(FlagPanic).Add(FlagIO)
    b := FlagSet(0).Add(FlagGoroutine).Add(FlagIO)
    c := a.Union(b)

    if !c.Has(FlagPanic) {
        t.Error("expected FlagPanic in union")
    }
    if !c.Has(FlagGoroutine) {
        t.Error("expected FlagGoroutine in union")
    }
    if !c.Has(FlagIO) {
        t.Error("expected FlagIO in union")
    }
}

func TestFlagSetString(t *testing.T) {
    s := FlagSet(0).Add(FlagPanic).Add(FlagIO)
    got := s.String()
    if got != "I, P" {
        t.Errorf("expected 'I, P', got %q", got)
    }
}

func TestFlagTypeString(t *testing.T) {
    tests := []struct {
        f    FlagType
        want string
    }{
        {FlagPanic, "P"},
        {FlagGoroutine, "G"},
        {FlagContext, "D"},
        {FlagIO, "I"},
        {FlagSideEffect, "S"},
        {FlagUnsafe, "U"},
        {FlagTime, "T"},
        {FlagExit, "X"},
        {FlagReflect, "R"},
        {FlagCGO, "C"},
    }
    for _, tt := range tests {
        if got := tt.f.String(); got != tt.want {
            t.Errorf("FlagType(%d).String() = %q, want %q", tt.f, got, tt.want)
        }
    }
}
```

- [ ] **Step 4: Run tests to verify**

```bash
cd /home/paul/rivus-linter-go
go test ./internal/model/... -v
```

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/model/flag.go internal/model/flag_test.go
git commit -m "feat(model): add FlagType bitmask and FlagSet with tests"
```

---

### Task 2: Position and Result Models

**Files:**
- Create: `internal/model/position.go`
- Create: `internal/model/result.go`

- [ ] **Step 1: Create the Position struct**

Create `internal/model/position.go`:

```go
package model

import (
    "go/token"
    "path/filepath"
)

// Position represents a source code location.
type Position struct {
    File string `json:"file"`
    Line int    `json:"line"`
    Col  int    `json:"col"`
}

// PositionFromToken converts a token.Pos to a Position using the provided FileSet.
// If pos is invalid, returns a zero Position.
func PositionFromToken(fset *token.FileSet, pos token.Pos) Position {
    if !pos.IsValid() {
        return Position{}
    }
    p := fset.Position(pos)
    return Position{
        File: filepath.Base(p.Filename),
        Line: p.Line,
        Col:  p.Column,
    }
}
```

- [ ] **Step 2: Create the Result structs**

Create `internal/model/result.go`:

```go
package model

import "time"

// Flag represents a single side-effect occurrence in a function.
type Flag struct {
    Type     FlagType  `json:"type"`
    Position Position  `json:"position"`
    Via      *string   `json:"via,omitempty"` // non-nil if inherited from another function
}

// FuncResult is the analysis result for a single function.
type FuncResult struct {
    FuncName string   `json:"name"`
    Position Position `json:"position"`
    Flags    []Flag   `json:"flags"`
}

// PackageResult is the analysis result for a Go package.
type PackageResult struct {
    Path      string       `json:"path"`
    Functions []FuncResult `json:"functions"`
}

// AnalysisResult is the top-level result of an analysis run.
type AnalysisResult struct {
    Version   string          `json:"version"`
    Timestamp time.Time       `json:"timestamp"`
    Packages  []PackageResult `json:"packages"`
    Errors    []string        `json:"errors,omitempty"`
}

// HasFlags reports whether any function in the result has any flags.
func (r *AnalysisResult) HasFlags() bool {
    for _, pkg := range r.Packages {
        for _, fn := range pkg.Functions {
            if len(fn.Flags) > 0 {
                return true
            }
        }
    }
    return false
}
```

- [ ] **Step 3: Verify compilation**

```bash
cd /home/paul/rivus-linter-go
go build ./internal/model/...
```

Expected: clean build.

- [ ] **Step 4: Commit**

```bash
git add internal/model/position.go internal/model/result.go
git commit -m "feat(model): add Position, Flag, and Result structs"
```

---

### Task 3: Standard Library Flag Mapping

**Files:**
- Create: `internal/analyzer/stdmap.go`
- Create: `internal/analyzer/stdmap_test.go`

- [ ] **Step 1: Create the stdlib flag mapping**

Create `internal/analyzer/stdmap.go`:

```go
package analyzer

import "github.com/BingyanStudio/rivus-linter-go/internal/model"

// stdFuncFlags maps "package.Function" to its FlagSet.
// This is used for standard library functions so we don't need to analyze their SSA.
var stdFuncFlags = map[string]model.FlagSet{
    // Panic
    "runtime.gopanic": model.FlagSet(model.FlagPanic),

    // Exit
    "os.Exit":          model.FlagSet(model.FlagExit),
    "log.Fatal":        model.FlagSet(model.FlagExit),
    "log.Fatalf":       model.FlagSet(model.FlagExit),
    "log.Fatalln":      model.FlagSet(model.FlagExit),
    "log.Logger.Fatal":  model.FlagSet(model.FlagExit),
    "log.Logger.Fatalf": model.FlagSet(model.FlagExit),
    "log.Logger.Fatalln": model.FlagSet(model.FlagExit),

    // I/O - os
    "os.Open":      model.FlagSet(model.FlagIO),
    "os.Create":    model.FlagSet(model.FlagIO),
    "os.ReadFile":  model.FlagSet(model.FlagIO),
    "os.WriteFile": model.FlagSet(model.FlagIO),
    "os.Mkdir":     model.FlagSet(model.FlagIO),
    "os.MkdirAll":  model.FlagSet(model.FlagIO),
    "os.Remove":    model.FlagSet(model.FlagIO),
    "os.RemoveAll": model.FlagSet(model.FlagIO),
    "os.Rename":    model.FlagSet(model.FlagIO),
    "os.Stat":      model.FlagSet(model.FlagIO),

    // I/O - net
    "net.Dial":        model.FlagSet(model.FlagIO),
    "net.DialTimeout": model.FlagSet(model.FlagIO),
    "net.Listen":      model.FlagSet(model.FlagIO),

    // I/O - database/sql
    "database/sql.Open": model.FlagSet(model.FlagIO),

    // I/O - io
    "io.ReadFull":    model.FlagSet(model.FlagIO),
    "io.ReadAtLeast": model.FlagSet(model.FlagIO),
    "io.Copy":        model.FlagSet(model.FlagIO),
    "io.CopyN":       model.FlagSet(model.FlagIO),

    // Side Effect - env
    "os.Getenv":   model.FlagSet(model.FlagSideEffect),
    "os.LookupEnv": model.FlagSet(model.FlagSideEffect),
    "os.Setenv":   model.FlagSet(model.FlagSideEffect),
    "os.Unsetenv": model.FlagSet(model.FlagSideEffect),
    "os.Environ":  model.FlagSet(model.FlagSideEffect),

    // Side Effect - random
    "rand.Int":     model.FlagSet(model.FlagSideEffect),
    "rand.Intn":    model.FlagSet(model.FlagSideEffect),
    "rand.Float64": model.FlagSet(model.FlagSideEffect),
    "rand.Read":    model.FlagSet(model.FlagSideEffect),

    // Unsafe
    "unsafe.Pointer":    model.FlagSet(model.FlagUnsafe),
    "unsafe.Sizeof":     model.FlagSet(model.FlagUnsafe),
    "unsafe.Offsetof":   model.FlagSet(model.FlagUnsafe),
    "unsafe.Alignof":    model.FlagSet(model.FlagUnsafe),
    "unsafe.Add":        model.FlagSet(model.FlagUnsafe),
    "unsafe.Slice":      model.FlagSet(model.FlagUnsafe),
    "unsafe.SliceData":  model.FlagSet(model.FlagUnsafe),
    "unsafe.String":     model.FlagSet(model.FlagUnsafe),
    "unsafe.StringData": model.FlagSet(model.FlagUnsafe),

    // Time
    "time.Now":       model.FlagSet(model.FlagTime),
    "time.After":     model.FlagSet(model.FlagTime),
    "time.Sleep":     model.FlagSet(model.FlagTime),
    "time.Tick":      model.FlagSet(model.FlagTime),
    "time.NewTicker": model.FlagSet(model.FlagTime),
    "time.Since":     model.FlagSet(model.FlagTime),
    "time.Until":     model.FlagSet(model.FlagTime),

    // Reflection
    "reflect.TypeOf":    model.FlagSet(model.FlagReflect),
    "reflect.ValueOf":   model.FlagSet(model.FlagReflect),
    "reflect.DeepEqual": model.FlagSet(model.FlagReflect),

    // Context (dangling)
    "context.Background": model.FlagSet(model.FlagContext),
    "context.TODO":       model.FlagSet(model.FlagContext),
}

// LookupStdFlags returns the flags for a standard library function.
// The key should be in "package.Function" or "package.Type.Method" format.
// Returns (flags, true) if found, (0, false) if not a known stdlib function.
func LookupStdFlags(name string) (model.FlagSet, bool) {
    flags, ok := stdFuncFlags[name]
    return flags, ok
}
```

- [ ] **Step 2: Write tests for stdmap**

Create `internal/analyzer/stdmap_test.go`:

```go
package analyzer

import (
    "testing"

    "github.com/BingyanStudio/rivus-linter-go/internal/model"
)

func TestLookupStdFlags(t *testing.T) {
    tests := []struct {
        name     string
        wantOK   bool
        wantFlag model.FlagType
    }{
        {"os.Exit", true, model.FlagExit},
        {"os.Open", true, model.FlagIO},
        {"time.Now", true, model.FlagTime},
        {"context.Background", true, model.FlagContext},
        {"fmt.Println", false, 0},
        {"math.Abs", false, 0},
    }
    for _, tt := range tests {
        flags, ok := LookupStdFlags(tt.name)
        if ok != tt.wantOK {
            t.Errorf("LookupStdFlags(%q): ok = %v, want %v", tt.name, ok, tt.wantOK)
        }
        if ok && !flags.Has(tt.wantFlag) {
            t.Errorf("LookupStdFlags(%q): missing flag %v", tt.name, tt.wantFlag)
        }
    }
}
```

- [ ] **Step 3: Run tests**

```bash
cd /home/paul/rivus-linter-go
go test ./internal/analyzer/... -v -run TestLookupStdFlags
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/analyzer/stdmap.go internal/analyzer/stdmap_test.go
git commit -m "feat(analyzer): add standard library flag mapping"
```

---

### Task 4: SSA Construction

**Files:**
- Create: `internal/analyzer/ssa.go`

- [ ] **Step 1: Create SSA builder**

Create `internal/analyzer/ssa.go`:

```go
package analyzer

import (
    "fmt"

    "golang.org/x/tools/go/packages"
    "golang.org/x/tools/go/ssa"
    "golang.org/x/tools/go/ssa/ssautil"
)

// SSAProgram holds the SSA program and all packages built from it.
type SSAProgram struct {
    Program *ssa.Program
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
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/paul/rivus-linter-go
go build ./internal/analyzer/...
```

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/analyzer/ssa.go
git commit -m "feat(analyzer): add SSA construction helper"
```

---

### Task 5: Side-Effect Scanners

**Files:**
- Create: `internal/analyzer/scanner.go`
- Create: `internal/analyzer/scanner_test.go`
- Create: `testdata/` directory with test fixtures

- [ ] **Step 1: Create the scanner**

Create `internal/analyzer/scanner.go`:

```go
package analyzer

import (
    "go/token"
    "go/types"
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
        if fn.Parent() != nil {
            return fn.Parent().Pkg().Path() + "." + fn.Name()
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
        if fn.Name() == "gopanic" && fn.Pkg() != nil && fn.Pkg().Path() == "runtime" {
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

// ScanTypeAssertions checks for type assertions that use reflect.
// This is a heuristic — type assertions themselves aren't reflect,
// but checking reflect.TypeOf/ValueOf calls is handled in scanCall.
func ScanTypeAssertions(fn *ssa.Function, fset *token.FileSet) []model.Flag {
    // Type assertions are handled via the call scanner.
    return nil
}

// ScanCGO checks if a function uses CGO.
func ScanCGO(fn *ssa.Function) bool {
    if fn == nil || fn.Pkg() == nil {
        return false
    }
    // CGO functions are in the "C" pseudo-package.
    return fn.Pkg().Path() == "C"
}

// IsReflectCall checks if a call is a reflect package function.
func IsReflectCall(name string) bool {
    return strings.HasPrefix(name, "reflect.")
}

// IsTimeCall checks if a call is a time package function.
func IsTimeCall(name string) bool {
    return strings.HasPrefix(name, "time.")
}
```

- [ ] **Step 2: Create test fixtures**

Create `testdata/panic.go`:

```go
package testdata

func PanicDirect() {
    panic("oh no")
}

func PanicViaHelper() {
    helper := func() {
        panic("from helper")
    }
    helper()
}
```

Create `testdata/goroutine.go`:

```go
package testdata

func LaunchGoroutine() {
    go func() {
        // async work
    }()
}

func LaunchNamed() {
    go helper()
}

func helper() {}
```

Create `testdata/io.go`:

```go
package testdata

import (
    "os"
    "net"
)

func ReadFile() {
    os.ReadFile("test.txt")
}

func DialNetwork() {
    net.Dial("tcp", "localhost:8080")
}
```

Create `testdata/sideeffect.go`:

```go
package testdata

import (
    "os"
    "rand"
)

var globalVar int

func ReadGlobal() int {
    return globalVar
}

func WriteGlobal() {
    globalVar = 42
}

func ReadEnv() {
    os.Getenv("HOME")
}

func UseRandom() {
    rand.Intn(100)
}
```

Create `testdata/pure.go`:

```go
package testdata

func Add(a, b int) int {
    return a + b
}

func Greet(name string) string {
    return "hello " + name
}
```

- [ ] **Step 3: Write scanner tests**

Create `internal/analyzer/scanner_test.go`:

```go
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
```

- [ ] **Step 4: Run scanner tests**

```bash
cd /home/paul/rivus-linter-go
go test ./internal/analyzer/... -v -run TestScan
```

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/analyzer/scanner.go internal/analyzer/scanner_test.go testdata/
git commit -m "feat(analyzer): add side-effect scanners with tests"
```

---

### Task 6: Call Graph and Bottom-Up Traversal

**Files:**
- Create: `internal/analyzer/callgraph.go`

- [ ] **Step 1: Create call graph traversal**

Create `internal/analyzer/callgraph.go`:

```go
package analyzer

import (
    "go/token"

    "golang.org/x/tools/go/callgraph"
    "golang.org/x/tools/go/callgraph/cha"
    "golang.org/x/tools/go/ssa"

    "github.com/BingyanStudio/rivus-linter-go/internal/model"
)

// FuncFlags stores the computed flags for each function.
type FuncFlags struct {
    // Flags is the complete set of flags (own + inherited).
    Flags model.FlagSet
    // Details contains the individual flag entries with positions.
    Details []model.Flag
}

// BuildCallGraph builds a call graph using the CHA (Class Hierarchy Analysis) algorithm.
func BuildCallGraph(prog *ssa.Program) *callgraph.Graph {
    return cha.CallGraph(prog)
}

// AnalyzeCallGraph performs bottom-up analysis on the call graph.
// It returns a map from function key ("pkg.Func") to FuncFlags.
func AnalyzeCallGraph(cg *callgraph.Graph, fset *token.FileSet) map[string]*FuncFlags {
    results := make(map[string]*FuncFlags)

    // Collect all functions from the call graph.
    allFuncs := collectFunctions(cg)

    // Build adjacency: callee -> callers.
    callersOf := buildReverseAdj(cg)

    // Compute topological order (bottom-up).
    order := topoSort(allFuncs, cg)

    // Process in bottom-up order.
    for _, fn := range order {
        key := funcKey(fn)
        if _, exists := results[key]; exists {
            continue
        }

        // Scan this function's own instructions.
        ownFlags := ScanFunc(fn, fset)
        ownFlags = append(ownFlags, ScanGlobals(fn, fset)...)

        // Build the FlagSet from own flags.
        var flagSet model.FlagSet
        var details []model.Flag
        for _, f := range ownFlags {
            flagSet = flagSet.Add(f.Type)
            details = append(details, f)
        }

        // Inherit flags from callees.
        for _, edge := range cg.Nodes[fn].Out {
            callee := edge.Callee.Func
            calleeKey := funcKey(callee)
            if calleeFlags, ok := results[calleeKey]; ok {
                // Inherit flags that we don't already have.
                for _, ft := range model.AllFlags() {
                    if calleeFlags.Flags.Has(ft) && !flagSet.Has(ft) {
                        flagSet = flagSet.Add(ft)
                        viaName := callee.Name()
                        details = append(details, model.Flag{
                            Type:     ft,
                            Position: model.PositionFromToken(fset, edge.Pos()),
                            Via:      &viaName,
                        })
                    }
                }
            }
        }

        results[key] = &FuncFlags{
            Flags:   flagSet,
            Details: details,
        }
    }

    return results
}

// funcKey returns a unique key for a function.
func funcKey(fn *ssa.Function) string {
    if fn.Pkg() != nil {
        return fn.Pkg().Path() + "." + fn.Name()
    }
    return fn.Name()
}

// collectFunctions returns all functions in the call graph.
func collectFunctions(cg *callgraph.Graph) []*ssa.Function {
    var funcs []*ssa.Function
    for fn, node := range cg.Nodes {
        if node != nil {
            funcs = append(funcs, fn)
        }
    }
    return funcs
}

// buildReverseAdj builds a reverse adjacency list: callee -> list of (caller, edge).
func buildReverseAdj(cg *callgraph.Graph) map[*ssa.Function][]*callgraph.Edge {
    result := make(map[*ssa.Function][]*callgraph.Edge)
    for _, node := range cg.Nodes {
        if node == nil {
            continue
        }
        for _, out := range node.Out {
            result[out.Callee.Func] = append(result[out.Callee.Func], out)
        }
    }
    return result
}

// topoSort returns functions in topological order (leaves first).
// Uses Kahn's algorithm.
func topoSort(funcs []*ssa.Function, cg *callgraph.Graph) []*ssa.Function {
    // Compute in-degree for each function.
    inDegree := make(map[*ssa.Function]int)
    for _, fn := range funcs {
        if _, ok := inDegree[fn]; !ok {
            inDegree[fn] = 0
        }
        if node := cg.Nodes[fn]; node != nil {
            for _, out := range node.Out {
                inDegree[out.Callee.Func]++
            }
        }
    }

    // Start with functions that have no callees (leaves).
    var queue []*ssa.Function
    for _, fn := range funcs {
        if inDegree[fn] == 0 {
            queue = append(queue, fn)
        }
    }

    var order []*ssa.Function
    for len(queue) > 0 {
        fn := queue[0]
        queue = queue[1:]
        order = append(order, fn)

        if node := cg.Nodes[fn]; node != nil {
            for _, out := range node.Out {
                inDegree[out.Callee.Func]--
                if inDegree[out.Callee.Func] == 0 {
                    queue = append(queue, out.Callee.Func)
                }
            }
        }
    }

    // If there are remaining functions (cycles), add them at the end.
    seen := make(map[*ssa.Function]bool)
    for _, fn := range order {
        seen[fn] = true
    }
    for _, fn := range funcs {
        if !seen[fn] {
            order = append(order, fn)
        }
    }

    return order
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/paul/rivus-linter-go
go build ./internal/analyzer/...
```

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/analyzer/callgraph.go
git commit -m "feat(analyzer): add call graph construction and bottom-up traversal"
```

---

### Task 7: Analyzer Orchestrator

**Files:**
- Create: `internal/analyzer/analyzer.go`
- Create: `internal/analyzer/analyzer_test.go`

- [ ] **Step 1: Create the analyzer orchestrator**

Create `internal/analyzer/analyzer.go`:

```go
package analyzer

import (
    "context"
    "fmt"
    "time"

    "golang.org/x/tools/go/ssa"

    "github.com/BingyanStudio/rivus-linter-go/internal/model"
)

// Analyzer is the main analysis engine.
type Analyzer struct {
    cache *Cache
}

// New creates a new Analyzer.
func New(cacheDir string) *Analyzer {
    return &Analyzer{
        cache: NewCache(cacheDir),
    }
}

// AnalyzeOptions configures an analysis run.
type AnalyzeOptions struct {
    // Patterns are the package patterns to analyze (e.g., "./...").
    Patterns []string
    // Dir is the working directory.
    Dir string
    // NoCache disables the cache.
    NoCache bool
}

// Analyze performs the full analysis pipeline.
func (a *Analyzer) Analyze(ctx context.Context, opts AnalyzeOptions) (*model.AnalysisResult, error) {
    start := time.Now()

    result := &model.AnalysisResult{
        Version:   "1.0",
        Timestamp: start,
    }

    // Load packages and build SSA.
    initial, prog, err := LoadAndBuildSSA(opts.Dir, opts.Patterns)
    if err != nil {
        result.Errors = append(result.Errors, fmt.Sprintf("SSA build: %v", err))
        return result, err
    }

    // Build call graph.
    cg := BuildCallGraph(prog.Program)

    // Analyze the call graph (bottom-up traversal).
    funcFlags := AnalyzeCallGraph(cg, prog.Program.Fset)

    // Group results by package.
    pkgResults := make(map[string]*model.PackageResult)

    for _, pkg := range initial {
        pkgPath := pkg.PkgPath()
        if _, ok := pkgResults[pkgPath]; ok {
            continue
        }
        pkgResults[pkgPath] = &model.PackageResult{
            Path: pkgPath,
        }
    }

    // Populate function results.
    for key, ff := range funcFlags {
        // Parse package path from key.
        pkgPath, funcName := splitFuncKey(key)
        if pkgPath == "" {
            continue
        }

        pr, ok := pkgResults[pkgPath]
        if !ok {
            pr = &model.PackageResult{Path: pkgPath}
            pkgResults[pkgPath] = pr
        }

        fr := model.FuncResult{
            FuncName: funcName,
            Flags:    ff.Details,
        }

        // Set position from SSA if available.
        if ssaPkg := prog.AllPackages[pkgPath]; ssaPkg != nil {
            for _, mem := range ssaPkg.Members {
                if fn, ok := mem.(*ssa.Function); ok && fn.Name() == funcName {
                    fr.Position = model.PositionFromToken(prog.Program.Fset, fn.Pos())
                    break
                }
            }
        }

        pr.Functions = append(pr.Functions, fr)
    }

    // Collect results.
    for _, pr := range pkgResults {
        result.Packages = append(result.Packages, *pr)
    }

    return result, nil
}

// splitFuncKey splits "pkg.Func" into ("pkg", "Func").
func splitFuncKey(key string) (string, string) {
    for i := len(key) - 1; i >= 0; i-- {
        if key[i] == '.' {
            return key[:i], key[i+1:]
        }
    }
    return "", key
}
```

- [ ] **Step 2: Write integration test**

Create `internal/analyzer/analyzer_test.go`:

```go
package analyzer

import (
    "context"
    "testing"
)

func TestAnalyzeIntegration(t *testing.T) {
    a := New("")
    result, err := a.Analyze(context.Background(), AnalyzeOptions{
        Patterns: []string{"../../testdata"},
        Dir:      ".",
    })
    if err != nil {
        t.Fatalf("Analyze failed: %v", err)
    }

    if len(result.Packages) == 0 {
        t.Fatal("expected at least one package in results")
    }

    // Check that we found some flags.
    found := false
    for _, pkg := range result.Packages {
        for _, fn := range pkg.Functions {
            if len(fn.Flags) > 0 {
                found = true
                t.Logf("Function %s has flags: %v", fn.FuncName, fn.Flags)
            }
        }
    }
    if !found {
        t.Error("expected at least one function with flags")
    }
}
```

- [ ] **Step 3: Run tests**

```bash
cd /home/paul/rivus-linter-go
go test ./internal/analyzer/... -v -run TestAnalyzeIntegration -timeout 60s
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/analyzer/analyzer.go internal/analyzer/analyzer_test.go
git commit -m "feat(analyzer): add analyzer orchestrator with integration test"
```

---

### Task 8: Cache

**Files:**
- Create: `internal/analyzer/cache.go`
- Create: `internal/analyzer/cache_test.go`

- [ ] **Step 1: Create the cache**

Create `internal/analyzer/cache.go`:

```go
package analyzer

import (
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"
    "time"
)

// Cache provides file-hash-based persistent caching of analysis results.
type Cache struct {
    dir string
}

// NewCache creates a new Cache. If dir is empty, caching is disabled.
func NewCache(dir string) *Cache {
    return &Cache{dir: dir}
}

// cacheIndex is the structure of the cache index file.
type cacheIndex struct {
    Version  string                  `json:"version"`
    Packages map[string]packageCache `json:"packages"`
}

// packageCache stores the hash and cached function results for a package.
type packageCache struct {
    FileHash  string          `json:"file_hash"`
    Functions json.RawMessage `json:"functions"`
    Timestamp time.Time       `json:"timestamp"`
}

// PackageHash computes a hash of all Go files in a directory.
func PackageHash(dir string) (string, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return "", err
    }

    h := sha256.New()
    var files []string
    for _, e := range entries {
        if !e.IsDir() && filepath.Ext(e.Name()) == ".go" {
            files = append(files, e.Name())
        }
    }
    sort.Strings(files)

    for _, name := range files {
        path := filepath.Join(dir, name)
        f, err := os.Open(path)
        if err != nil {
            continue
        }
        io.Copy(h, f)
        f.Close()
    }

    return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Get retrieves cached results for a package if the hash matches.
func (c *Cache) Get(pkgPath string, currentHash string) (json.RawMessage, bool) {
    if c.dir == "" {
        return nil, false
    }

    idx, err := c.loadIndex()
    if err != nil {
        return nil, false
    }

    pkg, ok := idx.Packages[pkgPath]
    if !ok {
        return nil, false
    }

    if pkg.FileHash != currentHash {
        return nil, false
    }

    return pkg.Functions, true
}

// Store saves results for a package in the cache.
func (c *Cache) Store(pkgPath string, fileHash string, functions json.RawMessage) error {
    if c.dir == "" {
        return nil
    }

    if err := os.MkdirAll(c.dir, 0o755); err != nil {
        return err
    }

    idx, err := c.loadIndex()
    if err != nil {
        idx = &cacheIndex{
            Version:  "1.0",
            Packages: make(map[string]packageCache),
        }
    }

    idx.Packages[pkgPath] = packageCache{
        FileHash:  fileHash,
        Functions: functions,
        Timestamp: time.Now(),
    }

    return c.saveIndex(idx)
}

// Clear removes the cache directory.
func (c *Cache) Clear() error {
    if c.dir == "" {
        return nil
    }
    return os.RemoveAll(c.dir)
}

func (c *Cache) indexPath() string {
    return filepath.Join(c.dir, "index.json")
}

func (c *Cache) loadIndex() (*cacheIndex, error) {
    data, err := os.ReadFile(c.indexPath())
    if err != nil {
        return nil, err
    }
    var idx cacheIndex
    if err := json.Unmarshal(data, &idx); err != nil {
        return nil, err
    }
    return &idx, nil
}

func (c *Cache) saveIndex(idx *cacheIndex) error {
    data, err := json.MarshalIndent(idx, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(c.indexPath(), data, 0o644)
}
```

- [ ] **Step 2: Write cache tests**

Create `internal/analyzer/cache_test.go`:

```go
package analyzer

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
)

func TestCacheRoundtrip(t *testing.T) {
    dir := t.TempDir()
    c := NewCache(dir)

    // Store some data.
    data := json.RawMessage(`[{"name":"Foo","flags":[]}]`)
    if err := c.Store("example.com/pkg", "abc123", data); err != nil {
        t.Fatalf("Store failed: %v", err)
    }

    // Retrieve with matching hash.
    got, ok := c.Get("example.com/pkg", "abc123")
    if !ok {
        t.Fatal("Get returned false for matching hash")
    }
    if string(got) != string(data) {
        t.Errorf("Get returned %s, want %s", got, data)
    }

    // Retrieve with different hash.
    _, ok = c.Get("example.com/pkg", "different")
    if ok {
        t.Error("Get returned true for non-matching hash")
    }
}

func TestCacheClear(t *testing.T) {
    dir := t.TempDir()
    c := NewCache(dir)

    data := json.RawMessage(`[]`)
    c.Store("pkg", "hash", data)

    if err := c.Clear(); err != nil {
        t.Fatalf("Clear failed: %v", err)
    }

    _, ok := c.Get("pkg", "hash")
    if ok {
        t.Error("expected cache miss after clear")
    }
}

func TestPackageHash(t *testing.T) {
    dir := t.TempDir()

    // Write a Go file.
    goFile := filepath.Join(dir, "test.go")
    os.WriteFile(goFile, []byte("package test\n"), 0o644)

    hash1, err := PackageHash(dir)
    if err != nil {
        t.Fatalf("PackageHash failed: %v", err)
    }

    // Same content should produce same hash.
    hash2, err := PackageHash(dir)
    if err != nil {
        t.Fatalf("PackageHash failed: %v", err)
    }
    if hash1 != hash2 {
        t.Errorf("expected same hash, got %s and %s", hash1, hash2)
    }

    // Different content should produce different hash.
    os.WriteFile(goFile, []byte("package test\n// changed\n"), 0o644)
    hash3, err := PackageHash(dir)
    if err != nil {
        t.Fatalf("PackageHash failed: %v", err)
    }
    if hash1 == hash3 {
        t.Error("expected different hash after file change")
    }
}
```

- [ ] **Step 3: Run tests**

```bash
cd /home/paul/rivus-linter-go
go test ./internal/analyzer/... -v -run TestCache -timeout 30s
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/analyzer/cache.go internal/analyzer/cache_test.go
git commit -m "feat(analyzer): add file-hash-based persistent cache"
```

---

### Task 9: Output Formatters

**Files:**
- Create: `internal/output/json.go`
- Create: `internal/output/table.go`
- Create: `internal/output/table_test.go`
- Create: `internal/output/json_test.go`

- [ ] **Step 1: Create JSON formatter**

Create `internal/output/json.go`:

```go
package output

import (
    "encoding/json"
    "io"

    "github.com/BingyanStudio/rivus-linter-go/internal/model"
)

// JSON formats the analysis result as JSON.
func JSON(w io.Writer, result *model.AnalysisResult) error {
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    return enc.Encode(result)
}
```

- [ ] **Step 2: Create table formatter**

Create `internal/output/table.go`:

```go
package output

import (
    "fmt"
    "io"
    "sort"
    "strings"

    "github.com/BingyanStudio/rivus-linter-go/internal/model"
)

// Table formats the analysis result as a human-readable table.
func Table(w io.Writer, result *model.AnalysisResult) error {
    if len(result.Packages) == 0 {
        fmt.Fprintln(w, "No packages analyzed.")
        return nil
    }

    // Collect all functions with flags.
    type row struct {
        FuncName string
        Flags    string
        Details  string
    }

    var rows []row
    for _, pkg := range result.Packages {
        for _, fn := range pkg.Functions {
            if len(fn.Flags) == 0 {
                continue
            }

            // Collect unique flag types.
            flagTypes := make(map[model.FlagType]bool)
            for _, f := range fn.Flags {
                flagTypes[f.Type] = true
            }
            var flagNames []string
            for ft := range flagTypes {
                flagNames = append(flagNames, ft.String())
            }
            sort.Strings(flagNames)

            // Build details.
            var details []string
            for _, f := range fn.Flags {
                if f.Via != nil {
                    details = append(details, fmt.Sprintf("%s via %s (%s:%d)",
                        f.Type, *f.Via, f.Position.File, f.Position.Line))
                } else {
                    details = append(details, fmt.Sprintf("%s at %s:%d",
                        f.Type, f.Position.File, f.Position.Line))
                }
            }

            fullName := fn.FuncName
            if pkg.Path != "" {
                fullName = pkg.Path + "." + fn.FuncName
            }

            rows = append(row{
                FuncName: fullName,
                Flags:    strings.Join(flagNames, ", "),
                Details:  strings.Join(details, "; "),
            })
        }
    }

    if len(rows) == 0 {
        fmt.Fprintln(w, "No side effects detected.")
        return nil
    }

    // Sort by function name.
    sort.Slice(rows, func(i, j int) bool {
        return rows[i].FuncName < rows[j].FuncName
    })

    // Print header.
    fmt.Fprintf(w, "%-40s %-10s %s\n", "Function", "Flags", "Details")
    fmt.Fprintln(w, strings.Repeat("─", 80))

    // Print rows.
    for _, r := range rows {
        fmt.Fprintf(w, "%-40s %-10s %s\n", r.FuncName, r.Flags, r.Details)
    }

    return nil
}
```

- [ ] **Step 3: Write output tests**

Create `internal/output/json_test.go`:

```go
package output

import (
    "bytes"
    "encoding/json"
    "testing"
    "time"

    "github.com/BingyanStudio/rivus-linter-go/internal/model"
)

func TestJSONOutput(t *testing.T) {
    via := "Helper"
    result := &model.AnalysisResult{
        Version:   "1.0",
        Timestamp: time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC),
        Packages: []model.PackageResult{
            {
                Path: "example.com/pkg",
                Functions: []model.FuncResult{
                    {
                        FuncName: "ProcessData",
                        Position: model.Position{File: "handler.go", Line: 42, Col: 1},
                        Flags: []model.Flag{
                            {Type: model.FlagIO, Position: model.Position{File: "handler.go", Line: 45, Col: 5}},
                            {Type: model.FlagPanic, Position: model.Position{File: "utils.go", Line: 12, Col: 3}, Via: &via},
                        },
                    },
                },
            },
        },
    }

    var buf bytes.Buffer
    if err := JSON(&buf, result); err != nil {
        t.Fatalf("JSON failed: %v", err)
    }

    // Verify it's valid JSON.
    var parsed model.AnalysisResult
    if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
        t.Fatalf("output is not valid JSON: %v", err)
    }

    if len(parsed.Packages) != 1 {
        t.Errorf("expected 1 package, got %d", len(parsed.Packages))
    }
}
```

Create `internal/output/table_test.go`:

```go
package output

import (
    "bytes"
    "strings"
    "testing"
    "time"

    "github.com/BingyanStudio/rivus-linter-go/internal/model"
)

func TestTableOutput(t *testing.T) {
    result := &model.AnalysisResult{
        Version:   "1.0",
        Timestamp: time.Now(),
        Packages: []model.PackageResult{
            {
                Path: "example.com/pkg",
                Functions: []model.FuncResult{
                    {
                        FuncName: "ProcessData",
                        Flags: []model.Flag{
                            {Type: model.FlagIO, Position: model.Position{File: "handler.go", Line: 45, Col: 5}},
                        },
                    },
                },
            },
        },
    }

    var buf bytes.Buffer
    if err := Table(&buf, result); err != nil {
        t.Fatalf("Table failed: %v", err)
    }

    output := buf.String()
    if !strings.Contains(output, "Function") {
        t.Error("expected header 'Function'")
    }
    if !strings.Contains(output, "ProcessData") {
        t.Error("expected 'ProcessData' in output")
    }
    if !strings.Contains(output, "I") {
        t.Error("expected flag 'I' in output")
    }
}

func TestTableNoFlags(t *testing.T) {
    result := &model.AnalysisResult{
        Version:   "1.0",
        Timestamp: time.Now(),
        Packages: []model.PackageResult{
            {
                Path: "example.com/pkg",
                Functions: []model.FuncResult{
                    {FuncName: "PureFunc"},
                },
            },
        },
    }

    var buf bytes.Buffer
    Table(&buf, result)

    if strings.Contains(buf.String(), "PureFunc") {
        t.Error("pure functions should not appear in table output")
    }
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/paul/rivus-linter-go
go test ./internal/output/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/output/json.go internal/output/table.go internal/output/json_test.go internal/output/table_test.go
git commit -m "feat(output): add JSON and table formatters with tests"
```

---

### Task 10: Configuration

**Files:**
- Create: `internal/config/config.go`

- [ ] **Step 1: Create configuration**

Create `internal/config/config.go`:

```go
package config

import (
    "encoding/json"
    "os"
    "path/filepath"
)

// Config holds the analysis configuration.
type Config struct {
    // Patterns are the package patterns to analyze.
    Patterns []string `json:"patterns"`
    // Dir is the working directory (default: current directory).
    Dir string `json:"dir,omitempty"`
    // CacheDir is the cache directory (default: ".rivus-cache").
    CacheDir string `json:"cache_dir,omitempty"`
    // Format is the output format: "json" or "table".
    Format string `json:"format,omitempty"`
    // Output is the output file path (empty = stdout).
    Output string `json:"output,omitempty"`
    // NoCache disables the cache.
    NoCache bool `json:"no_cache,omitempty"`
}

// Default returns a Config with default values.
func Default() *Config {
    return &Config{
        Patterns: []string{"./..."},
        CacheDir: ".rivus-cache",
        Format:   "table",
    }
}

// LoadFile loads a config from a JSON file.
func LoadFile(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}

// SaveFile saves a config to a JSON file.
func SaveFile(path string, cfg *Config) error {
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return err
    }
    data, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0o644)
}

// Merge applies non-zero values from other to cfg.
func (cfg *Config) Merge(other *Config) {
    if len(other.Patterns) > 0 {
        cfg.Patterns = other.Patterns
    }
    if other.Dir != "" {
        cfg.Dir = other.Dir
    }
    if other.CacheDir != "" {
        cfg.CacheDir = other.CacheDir
    }
    if other.Format != "" {
        cfg.Format = other.Format
    }
    if other.Output != "" {
        cfg.Output = other.Output
    }
    if other.NoCache {
        cfg.NoCache = true
    }
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/paul/rivus-linter-go
go build ./internal/config/...
```

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add configuration struct"
```

---

### Task 11: Public API

**Files:**
- Create: `pkg/rivus/api.go`

- [ ] **Step 1: Create the public API**

Create `pkg/rivus/api.go`:

```go
// Package rivus provides the public API for the Go side-effect checker.
// This package is designed to be consumed by VSCode extensions and other tools.
package rivus

import (
    "context"
    "encoding/json"
    "time"

    "github.com/BingyanStudio/rivus-linter-go/internal/analyzer"
    "github.com/BingyanStudio/rivus-linter-go/internal/model"
)

// Analyzer is the main analysis interface.
type Analyzer interface {
    // Analyze analyzes functions in specified packages.
    Analyze(ctx context.Context, opts AnalyzeOptions) (*model.AnalysisResult, error)
    // AnalyzeFunc analyzes a single function by name.
    AnalyzeFunc(ctx context.Context, funcName string) (*model.FuncResult, error)
    // ClearCache clears the analysis cache.
    ClearCache() error
}

// AnalyzeOptions configures an analysis run.
type AnalyzeOptions struct {
    // Patterns are the package patterns to analyze.
    // If empty, defaults to "./...".
    Patterns []string
    // Dir is the working directory.
    // If empty, defaults to the current directory.
    Dir string
    // NoCache disables the cache.
    NoCache bool
}

// analyzerImpl implements Analyzer.
type analyzerImpl struct {
    inner *analyzer.Analyzer
}

// New creates a new Analyzer.
// cacheDir is the directory for persistent cache (e.g., ".rivus-cache").
func New(cacheDir string) Analyzer {
    return &analyzerImpl{
        inner: analyzer.New(cacheDir),
    }
}

// Analyze performs the full analysis pipeline.
func (a *analyzerImpl) Analyze(ctx context.Context, opts AnalyzeOptions) (*model.AnalysisResult, error) {
    return a.inner.Analyze(ctx, analyzer.AnalyzeOptions{
        Patterns: opts.Patterns,
        Dir:      opts.Dir,
        NoCache:  opts.NoCache,
    })
}

// AnalyzeFunc analyzes a single function by name.
// This loads all packages and searches for the function.
func (a *analyzerImpl) AnalyzeFunc(ctx context.Context, funcName string) (*model.FuncResult, error) {
    // For now, analyze all packages and find the function.
    // A future optimization could narrow the search.
    result, err := a.inner.Analyze(ctx, analyzer.AnalyzeOptions{
        Patterns: []string{"./..."},
    })
    if err != nil {
        return nil, err
    }

    for _, pkg := range result.Packages {
        for _, fn := range pkg.Functions {
            if fn.FuncName == funcName {
                return &fn, nil
            }
        }
    }

    return nil, nil // not found
}

// ClearCache clears the analysis cache.
func (a *analyzerImpl) ClearCache() error {
    return a.inner.ClearCache()
}

// ToJSON converts an AnalysisResult to JSON bytes.
func ToJSON(result *model.AnalysisResult) ([]byte, error) {
    return json.MarshalIndent(result, "", "  ")
}

// ResultFromJSON parses JSON bytes into an AnalysisResult.
func FromJSON(data []byte) (*model.AnalysisResult, error) {
    var result model.AnalysisResult
    if err := json.Unmarshal(data, &result); err != nil {
        return nil, err
    }
    return &result, nil
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/paul/rivus-linter-go
go build ./pkg/rivus/...
```

Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add pkg/rivus/api.go
git commit -m "feat(api): add public Analyzer interface for VSCode integration"
```

---

### Task 12: CLI Entry Point

**Files:**
- Create: `cmd/rivus/main.go`

- [ ] **Step 1: Create the CLI**

Create `cmd/rivus/main.go`:

```go
package main

import (
    "context"
    "fmt"
    "io"
    "os"

    "github.com/BingyanStudio/rivus-linter-go/internal/analyzer"
    "github.com/BingyanStudio/rivus-linter-go/internal/config"
    "github.com/BingyanStudio/rivus-linter-go/internal/output"
)

func main() {
    if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}

func run(args []string, stdout, stderr io.Writer) error {
    cfg := config.Default()

    // Simple arg parsing.
    for i := 0; i < len(args); i++ {
        switch args[i] {
        case "--format", "-f":
            if i+1 < len(args) {
                i++
                cfg.Format = args[i]
            }
        case "--output", "-o":
            if i+1 < len(args) {
                i++
                cfg.Output = args[i]
            }
        case "--no-cache":
            cfg.NoCache = true
        case "--dir", "-d":
            if i+1 < len(args) {
                i++
                cfg.Dir = args[i]
            }
        case "--help", "-h":
            fmt.Fprintln(stdout, usage)
            return nil
        case "cache":
            if i+1 < len(args) && args[i+1] == "clear" {
                c := analyzer.New(cfg.CacheDir)
                if err := c.ClearCache(); err != nil {
                    return fmt.Errorf("clearing cache: %w", err)
                }
                fmt.Fprintln(stdout, "Cache cleared.")
                return nil
            }
            return fmt.Errorf("unknown cache command")
        default:
            cfg.Patterns = append(cfg.Patterns, args[i])
        }
    }

    // Run analysis.
    a := analyzer.New(cfg.CacheDir)
    result, err := a.Analyze(context.Background(), analyzer.AnalyzeOptions{
        Patterns: cfg.Patterns,
        Dir:      cfg.Dir,
        NoCache:  cfg.NoCache,
    })
    if err != nil {
        return fmt.Errorf("analysis failed: %w", err)
    }

    // Determine output writer.
    w := stdout
    if cfg.Output != "" {
        f, err := os.Create(cfg.Output)
        if err != nil {
            return fmt.Errorf("creating output file: %w", err)
        }
        defer f.Close()
        w = f
    }

    // Format output.
    switch cfg.Format {
    case "json":
        return output.JSON(w, result)
    case "table":
        return output.Table(w, result)
    default:
        return fmt.Errorf("unknown format: %s (supported: json, table)", cfg.Format)
    }
}

const usage = `rivus - Go function side-effect checker

Usage:
    rivus [flags] [patterns...]

Flags:
    -f, --format <json|table>   Output format (default: table)
    -o, --output <file>         Output file (default: stdout)
    -d, --dir <dir>             Working directory
    --no-cache                  Disable cache
    -h, --help                  Show this help

Commands:
    cache clear                 Clear the analysis cache

Examples:
    rivus                       Analyze current module
    rivus ./cmd/...             Analyze specific packages
    rivus --format json         Output as JSON
    rivus --format json -o result.json  Save JSON to file
`
```

- [ ] **Step 2: Build and test the CLI**

```bash
cd /home/paul/rivus-linter-go
go build -o rivus ./cmd/rivus/
./rivus --help
```

Expected: Usage text printed.

- [ ] **Step 3: Run on testdata**

```bash
cd /home/paul/rivus-linter-go
./rivus --format table ./testdata/
```

Expected: Table output showing functions with their flags.

- [ ] **Step 4: Commit**

```bash
git add cmd/rivus/main.go
git commit -m "feat(cli): add rivus CLI with check and cache clear commands"
```

---

### Task 13: End-to-End Verification

**Files:**
- No new files (verification only)

- [ ] **Step 1: Run all tests**

```bash
cd /home/paul/rivus-linter-go
go test ./... -v -timeout 120s
```

Expected: All tests PASS.

- [ ] **Step 2: Run the tool on testdata with JSON output**

```bash
cd /home/paul/rivus-linter-go
./rivus --format json ./testdata/ | jq .
```

Expected: Valid JSON with function flags.

- [ ] **Step 3: Run the tool on testdata with table output**

```bash
cd /home/paul/rivus-linter-go
./rivus --format table ./testdata/
```

Expected: Human-readable table with function names, flags, and source locations.

- [ ] **Step 4: Test cache behavior**

```bash
cd /home/paul/rivus-linter-go
# First run (populates cache)
./rivus --format table ./testdata/
# Second run (should use cache)
./rivus --format table ./testdata/
# Clear cache
./rivus cache clear
# Third run (rebuilds cache)
./rivus --format table ./testdata/
```

Expected: All three runs produce the same output.

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat: complete Go side-effect checker v1.0"
```
