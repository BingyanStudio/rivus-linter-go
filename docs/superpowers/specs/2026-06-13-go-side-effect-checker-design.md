# Go Function Side-Effect Checker Design

**Date**: 2026-06-13
**Status**: Draft
**Author**: Paul Liu

## Overview

A Go tool that analyzes functions using SSA (Static Single Assignment) to detect side effects and impurity. It identifies whether a function is "pure" by checking for various side-effect flags, and supports caching to avoid redundant computation.

## Goals

1. Analyze Go functions using `golang.org/x/tools/go/ssa`
2. Detect side effects: panic, goroutine, dangling context, I/O, global state, unsafe, etc.
3. Support flag inheritance (child function flags propagate to callers)
4. Cache analysis results based on file content hashing
5. Report source locations (file:line) for each flag
6. Provide both CLI and library API (for future VSCode extension)
7. Output in JSON and human-readable table formats

## Non-Goals (Phase 1)

- Watch mode (file change monitoring)
- IDE integration (will be added later)
- Fix suggestions

## Architecture

### Overview

The tool uses a two-phase analysis approach:

```
Phase 1: Build SSA + Call Graph
Phase 2: Bottom-up traversal to mark flags
```

This approach naturally supports flag inheritance and is cache-friendly.

### Project Structure

```
rivus-linter-go/
├── cmd/rivus/              # CLI entry point
│   └── main.go
├── internal/
│   ├── analyzer/           # Core analysis engine
│   │   ├── ssa.go          # SSA construction
│   │   ├── callgraph.go    # Call Graph construction
│   │   ├── scanner.go      # Side-effect scanner
│   │   └── cache.go        # Cache management
│   ├── model/              # Data models
│   │   ├── flag.go         # Flag definitions (P, G, D, I, S, U, T, X, R, C)
│   │   ├── result.go       # Analysis results
│   │   └── position.go     # Source position info
│   ├── output/             # Output formatters
│   │   ├── json.go         # JSON output
│   │   └── table.go        # Table output
│   └── config/             # Configuration
│       └── config.go
└── pkg/
    └── rivus/              # Public API (for VSCode extension)
        └── api.go
```

### Data Flow

1. `cmd/rivus` parses CLI args, calls `pkg/rivus` API
2. `analyzer` loads Go packages, builds SSA
3. Builds Call Graph from SSA
4. Traverses Call Graph bottom-up, runs `scanner` on each function
5. `scanner` traverses SSA instructions, matches side-effect patterns
6. Aggregates results, formats output via `output`

## Flag Model

### Flag Types

| Flag | Name | Description |
|------|------|-------------|
| P | Panic | Calls `panic()` or function with P flag |
| G | Goroutine | Launches goroutine (`go` statement) or function with G flag |
| D | Dangling Context | Calls `context.Background()` or `context.TODO()` without cancel |
| I | I/O | Performs I/O (network, file, database) |
| S | Side Effect | Reads/writes global variables, env vars, random numbers |
| U | Unsafe | Uses `unsafe.Pointer` or `unsafe.*` functions |
| T | Time | Calls `time.Now()`, `time.After()`, `time.Sleep()` |
| X | Exit | Calls `os.Exit()`, `log.Fatal*()` |
| R | Reflection | Calls `reflect.*` functions |
| C | CGO | Uses CGO (`import "C"`) |

### Data Structures

```go
// Flag represents a side-effect marker
type Flag struct {
    Type     FlagType  // Flag type (P, G, D, I, S, U, T, X, R, C)
    Position Position  // Source location where the flag originates
    Via      *string   // If inherited, the source function name (optional)
}

// FlagType defines all possible side-effect types
type FlagType string

const (
    FlagPanic      FlagType = "P"
    FlagGoroutine  FlagType = "G"
    FlagContext    FlagType = "D"
    FlagIO         FlagType = "I"
    FlagSideEffect FlagType = "S"
    FlagUnsafe     FlagType = "U"
    FlagTime       FlagType = "T"
    FlagExit       FlagType = "X"
    FlagReflect    FlagType = "R"
    FlagCGO        FlagType = "C"
)

// Position represents a source code location
type Position struct {
    File string
    Line int
    Col  int
}

// FuncResult represents the analysis result for a function
type FuncResult struct {
    FuncName string
    Flags    []Flag
    Children []FuncResult  // Results of called functions
}
```

### Inheritance Logic

- If function A calls function B, and B has flags `P` and `I`
- Then A inherits `P` and `I`, marked with `Via: "B"`
- If A itself also has `P`, keep A's own `P` (no Via marker)

## Scanner Design

The scanner traverses a function's SSA instructions and identifies side effects.

### Detection Rules

| Flag | Trigger Condition |
|------|-------------------|
| P | `panic()` call, or called function has P flag |
| G | `go` statement, or called function has G flag |
| D | `context.Background()` or `context.TODO()` without cancel |
| I | Calls to `os.*`, `net.*`, `database/sql.*`, `io.*` packages |
| S | Access to `ssa.Global`, calls to `os.Getenv()`, `rand.*` |
| U | `unsafe.Pointer` or `unsafe.*` functions |
| T | `time.Now()`, `time.After()`, `time.Sleep()` |
| X | `os.Exit()`, `log.Fatal*()` |
| R | `reflect.*` functions |
| C | CGO usage (`import "C"`) |

### Scanner Implementation

```go
func (s *Scanner) Scan(fn *ssa.Function) []Flag {
    var flags []Flag

    for _, block := range fn.Blocks {
        for _, instr := range block.Instrs {
            switch i := instr.(type) {
            case *ssa.Call:
                if isPanic(i) {
                    flags = append(flags, Flag{Type: FlagPanic, Position: pos(i)})
                }
                if isIO(i) {
                    flags = append(flags, Flag{Type: FlagIO, Position: pos(i)})
                }
                // ... other rules
            case *ssa.Go:
                flags = append(flags, Flag{Type: FlagGoroutine, Position: pos(i)})
            }
        }
    }

    return flags
}
```

### Cross-Function Inheritance

- Scanner only analyzes a function's **own** instructions
- Inheritance is handled by the Analyzer when traversing the Call Graph

## Analyzer & Call Graph Traversal

### Bottom-Up Traversal Algorithm

```
1. Compute reverse topological order of Call Graph (leaf functions first)
2. For each function:
   a. Use Scanner to scan own instructions -> ownFlags
   b. For each called function:
      - Read its flags from cache (already computed)
      - Mark as inherited, set Via field
   c. Merge ownFlags + inheritedFlags
   d. Cache result
3. For target function, return aggregated flags
```

### Handling Recursive Calls

- Detect strongly connected components (SCC) in Call Graph
- For functions in a SCC, mark as "being analyzed"
- If encountering a function being analyzed, skip (avoid infinite recursion)
- Functions in the same SCC share the same flag set

### Cross-Package Analysis

- Use `ssautil.AllPackages()` to load all dependency packages
- For standard library functions, use predefined flag mapping (no SSA analysis needed)
- For third-party libraries, analyze their SSA (if available)

### Standard Library Flag Mapping

The following standard library functions are pre-mapped with flags:

| Package | Function | Flag |
|---------|----------|------|
| `panic` | `panic()` | P |
| `os` | `Exit()` | X |
| `log` | `Fatal()`, `Fatalf()`, `Fatalln()` | X |
| `os` | `Open()`, `Create()`, `ReadFile()`, `WriteFile()` | I |
| `net` | `Dial()`, `Listen()` | I |
| `database/sql` | `Open()` | I |
| `io` | `Read()`, `Write()` | I |
| `os` | `Getenv()`, `Setenv()` | S |
| `rand` | `Int()`, `Intn()`, `Float64()` | S |
| `unsafe` | `Pointer()`, `Sizeof()`, `Offsetof()` | U |
| `time` | `Now()`, `After()`, `Sleep()` | T |
| `reflect` | `TypeOf()`, `ValueOf()` | R |
| `context` | `Background()`, `TODO()` | D |

## Cache Design

### Strategy

- Based on file content hashing (SHA256)
- Persisted to disk (`.rivus-cache/` directory)
- Supports cross-run reuse

### Cache Structure

```
.rivus-cache/
├── index.json              # Cache index
├── <hash1>.json            # Function-level cache
├── <hash2>.json
└── ...
```

### Cache Key

- For each function: cache key = file path + function name + file content hash
- If file content changes, cache automatically invalidates

### Cache Granularity

The cache uses a two-level granularity strategy:

1. **Package level**: First check all file hashes in the package; if all match, read entire package cache
2. **Function level**: If some files in the package changed, only re-analyze functions in changed files

**Selection logic**:
- Always start with package-level check (fast path)
- If package-level cache miss, fall back to function-level check
- Function-level cache is only valid if the function's source file hash matches

### Cache Invalidation Logic

1. Read `.rivus-cache/index.json`
2. For each package to analyze, check all file hashes
3. If hashes match, read cached function results
4. If hashes don't match, re-analyze the package

### Cache Warmup

- First run: analyze all functions and cache
- Subsequent runs: only analyze changed functions

## Output Formats

### JSON Output

```json
{
  "version": "1.0",
  "timestamp": "2026-06-13T10:00:00Z",
  "packages": [
    {
      "path": "github.com/example/pkg",
      "functions": [
        {
          "name": "ProcessData",
          "position": {"file": "handler.go", "line": 42, "col": 1},
          "flags": [
            {
              "type": "I",
              "position": {"file": "handler.go", "line": 45, "col": 5},
              "via": null
            },
            {
              "type": "P",
              "position": {"file": "utils.go", "line": 12, "col": 3},
              "via": "ValidateInput"
            }
          ]
        }
      ]
    }
  ]
}
```

### Table Output

```
Function                    Flags   Details
─────────────────────────────────────────────────────────────────────
ProcessData                 P, I    P via ValidateInput (utils.go:12)
                                    I at handler.go:45
utils.ValidateInput         P       P at utils.go:12 (panic("invalid"))
```

## Public API

For VSCode extension integration:

```go
// pkg/rivus/api.go

// Analyzer is the main analysis interface
type Analyzer interface {
    // Analyze analyzes functions in specified packages
    Analyze(ctx context.Context, opts AnalyzeOptions) (*AnalysisResult, error)

    // AnalyzeFunc analyzes a single function
    AnalyzeFunc(ctx context.Context, funcName string) (*FuncResult, error)

    // ClearCache clears the cache
    ClearCache() error
}

// AnalyzeOptions are analysis options
type AnalyzeOptions struct {
    Packages []string  // Package paths to analyze (empty = current module)
    Dir      string    // Working directory
}

// AnalysisResult is the analysis result
type AnalysisResult struct {
    Packages []PackageResult
    Errors   []error
}

// PackageResult is the result for a package
type PackageResult struct {
    Path      string
    Functions []FuncResult
}
```

## CLI Interface

```bash
# Analyze current module
rivus check

# Analyze specific packages
rivus check ./cmd/... ./internal/...

# Output JSON
rivus check --format json

# Output to file
rivus check --output result.json

# Clear cache
rivus cache clear
```

## Error Handling

- Errors during analysis (e.g., SSA construction failure) are recorded in `AnalysisResult.Errors`
- A single function's analysis failure does not interrupt the entire analysis
- Unresolvable calls (e.g., interface methods) are marked as `Unknown` and analysis continues

## Testing Strategy

### Unit Tests

- Each Scanner rule has corresponding test cases
- Test cases contain code that intentionally triggers each flag

### Integration Tests

- Test the complete analysis flow
- Test cache read/write and invalidation

### Test Data

```
testdata/
├── panic.go          # Test case with panic
├── goroutine.go      # Test case with goroutine
├── io.go             # Test case with I/O
└── ...
```

### Golden Files

- For table output, use golden file testing
- For JSON output, use JSON schema validation

## Implementation Phases

### Phase 1: Core Analysis (Week 1-2)

1. Set up project structure
2. Implement SSA construction
3. Implement Call Graph construction
4. Implement basic Scanner (P, G, I, S flags)
5. Implement bottom-up traversal

### Phase 2: Complete Scanner (Week 3)

1. Implement remaining flags (D, U, T, X, R, C)
2. Implement cross-package analysis
3. Implement standard library flag mapping

### Phase 3: Cache & Output (Week 4)

1. Implement file hash-based cache
2. Implement JSON output
3. Implement table output

### Phase 4: CLI & API (Week 5)

1. Implement CLI interface
2. Implement public API
3. Integration testing

## Future Work

- Watch mode for continuous monitoring
- VSCode extension integration
- Custom rule configuration
- Fix suggestions
- IDE quick-fix actions
