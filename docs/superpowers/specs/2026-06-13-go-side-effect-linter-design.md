# Go Function Side-Effect Linter — Design Specification

## Overview

A Go linter that analyzes functions for side effects and runtime behaviors, inspired by [rivus-linter](https://github.com/jyi2ya/rivus-linter) (Rust). The linter detects capabilities (panic, goroutine, I/O, etc.) and propagates them through the call graph, reporting which functions are "pure" and which have side effects.

**Module**: `github.com/BingyanStudio/rivus-linter-go`

## Capability Letters

Nine capabilities, each independently detected and propagated:

| Letter | Name | Detection Method | Propagates |
|--------|------|-----------------|------------|
| **P** | Panic | AST: `panic()`, `log.Fatal*()`, `os.Exit()` | Yes |
| **G** | Goroutine | AST: `go` keyword | Yes |
| **D** | Dangling Context | AST: `context.Background()/TODO()` not passed to `WithCancel/WithTimeout/WithDeadline` in same function | Yes |
| **I** | IO | Capsmap: leaf nodes like `os.Open`, `net.Dial` | Yes |
| **S** | SideEffect | AST: global var writes, `os.Setenv`, `rand.Seed`, `sync.Once.Do` | Yes |
| **U** | Unsafe | AST: `unsafe.Pointer`, `reflect.SliceHeader`, cgo | Yes |
| **B** | Blocking | Capsmap: `sync.Mutex.Lock`, channel ops, `time.Sleep` | Yes |
| **M** | Mutable | Signature: `*T` parameters | No |
| **T** | ThreadLocal | Capsmap: `sync.Pool.Get`, `sync.Once.Do` | Yes |

**Key differences from the Rust version:**
- **G** replaces **A** — Go uses goroutines, not async/await
- **D** is new — `context.Background()/TODO()` without cancellation is a Go-specific anti-pattern
- **M** does not propagate — Go's mutability is less explicit; only `*T` params are flagged
- **T** detection uses Go's `sync.Pool`/`sync.Once` instead of `thread_local!`

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLI Layer                                 │
│  Commands: check, report, why, infer-capsmap, generate-stdlib    │
│  Flags: --json, --output, --packages, --verbose                  │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│                     Core Engine                                  │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │
│  │  Loader      │  │  Analyzer    │  │  Reporter            │   │
│  │              │  │              │  │                      │   │
│  │ go/parser    │  │ SSA builder  │  │ Text formatter       │   │
│  │ go/types     │  │ Call graph   │  │ JSON serializer      │   │
│  │ go/ssa       │  │ Cap detector │  │ Diagnostic struct    │   │
│  │ Package load │  │ Propagation  │  │                      │   │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────┘   │
│         │                 │                      │               │
│         └────────┬────────┘                      │               │
│                  │                               │               │
│         ┌────────▼────────┐                      │               │
│         │   CapsMap       │                      │               │
│         │                 │                      │               │
│         │ std (pre-built) │                      │               │
│         │ user caps/      │──────────────────────┘               │
│         │ inferred cache  │                                      │
│         └─────────────────┘                                      │
└──────────────────────────────────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│                     Cache Layer                                  │
│                                                                  │
│  .rivus-cache/                                                   │
│  ├── file-hashes.json      (content hash per file)               │
│  ├── package-caps.json     (per-package capability results)      │
│  ├── callgraph.json        (cached call graph)                   │
│  └── inferred-capsmap.txt  (generated caps for analyzed pkgs)    │
└──────────────────────────────────────────────────────────────────┘
```

### Data Flow — `check` Command

1. Load capsmap (pre-built std + user `caps/` directory)
2. Load packages via `go/packages` → build SSA
3. For each function in analyzed packages:
   a. Detect direct capabilities from AST/SSA (P, G, D, S, U, M)
   b. Look up external calls in capsmap (I, B, T)
   c. Record call graph edges
4. Run fixpoint propagation (union callee caps into caller)
5. Report diagnostics

### Data Flow — `why <function>` Command

1. Load cached call graph
2. Show function's capabilities + which callees contributed each

### Data Flow — `generate-stdlib` Command

1. Read Go stdlib source from `$GOROOT/src`
2. Build SSA, analyze all exported functions
3. Write capsmap to `caps/std`

## SSA Analysis Pipeline

### Package Loading

Using `golang.org/x/tools/go/ssa` and `golang.org/x/tools/go/packages`:

1. `packages.Load(cfg, patterns...)` → `[]*packages.Package`
2. `ssautil.AllPackages(pkgs)` → `[]*ssa.Package`
3. `pkg.Build()` for each analyzed package
4. For each package, iterate `pkg.Members` → find `*ssa.Function`

### Direct Capability Detection

Each `*ssa.Function` is analyzed for direct capabilities:

**P (Panic):**
- `ssa.Panic` instruction in the function body
- Calls to `log.Fatal`, `log.Fatalf`, `log.Fatalln`
- Calls to `os.Exit`
- Note: `error` returns are NOT panics — only explicit panic/exit

**G (Goroutine):**
- `ssa.Go` instruction (the `go` keyword)
- Calls to functions that spawn goroutines (from capsmap)

**D (Dangling Context):**
- Calls to `context.Background()` or `context.TODO()` where the result is NOT passed to `context.WithCancel`, `context.WithTimeout`, `context.WithDeadline` within the same function
- Detection: track the `ssa.Value` returned by Background/TODO, check if it flows into a With* call
- Note: D is an AST-level pattern, not a capsmap entry. `context.WithCancel` and friends have empty caps — they resolve the dangling context, but the D flag is on the caller.

**S (SideEffect):**
- `ssa.Store` to a global variable (`*ssa.Global`)
- Calls to `os.Setenv`, `os.Unsetenv`, `os.Chdir`
- Calls to `rand.Seed`, `rand.New` (writes to global state)
- `sync.Once.Do` calls (one-time side effect)

**U (Unsafe):**
- Use of `unsafe.Pointer` operations
- Use of `reflect.SliceHeader`, `reflect.StringHeader`
- cgo calls (`import "C"`)

**M (Mutable):**
- Parameter types containing `*T` where T is not an interface
- Does NOT propagate — only detected from signature

**B (Blocking):** NOT detected from AST — comes from capsmap propagation

**I (IO):** NOT detected from AST — comes from capsmap propagation

**T (ThreadLocal):** NOT detected from AST — comes from capsmap propagation

### Call Graph Construction

From SSA, build a call graph:
- `ssa.Call` and `ssa.Defer` and `ssa.Go` instructions → edges
- For interface calls (`ssa.MakeInterface` + `ssa.Call`), resolve to all concrete implementations
- Store as `map[FunctionID] → []CalleeEntry`

```go
type CalleeEntry struct {
    FunctionID  string    // e.g., "mypkg.HandleRequest" or "io.Reader.Read"
    CallSite    Position  // file:line where the call happens
    IsInterface bool      // true if this is an interface call
    IsExternal  bool      // true if from capsmap (not analyzed)
}
```

### Fixpoint Propagation

1. **Initial assignment**: For each function, assign direct capabilities
2. **Capsmap injection**: Any callee in capsmap → assign its caps
3. **Fixpoint iteration** (up to 16 iterations): For each function, union its caps with all callees' caps. Stop when no new caps are added.
4. **Interface resolution**: For interface calls, majority vote (≥50% of implementations) determines capabilities. M and U are excluded from the vote (they are signature-only). Only implementations found in analyzed packages are considered; if no implementations are found, the interface method is treated as unknown.

## CapsMap

### Format

Same as the Rust version — a text file with one entry per line:

```
# Format: package_path.FunctionName=CAPS
# Comments with #
fmt.Println=I
os.Open=BI
net/http.ListenAndServe=BIG
sync.Mutex.Lock=B
context.Background=
context.TODO=
context.WithCancel=
context.WithTimeout=
log.Fatal=PIS
os.Setenv=S
unsafe.Pointer.Sizeof=U
```

### Lookup

When the analyzer encounters a call to an external function:
1. **Exact match**: `pkgpath.FuncName` → direct lookup
2. **Method match**: `pkgpath.Type.Method` → lookup
3. **Interface resolution**: If calling an interface method, find all implementations, take majority vote (≥50%)
4. **Not found**: Emit `UNKNOWN_CALLEE` diagnostic

### Directory Structure

```
caps/
├── std         # Pre-built: Go standard library (shipped with tool)
├── user        # User-defined caps for their dependencies
└── suppress    # Overrides for incorrect entries
```

Loading order: `std → user → suppress` (later overrides earlier).

### Go-Specific Considerations

- **Package paths**: Use Go import paths — `net/http`, `os`, `sync`, etc.
- **Methods**: `net/http.(*ServeMux).Handle` (pointer receiver) vs `net/http.ServeMux.HandleFunc`
- **Interface methods**: `io.Reader.Read` — the interface definition, resolved via majority vote at analysis time
- **Generics**: Go 1.18+ generics — analyze the instantiated version, not the generic definition

## Caching Strategy

### Cache Structure

```
.rivus-cache/
├── v1/
│   ├── files.json          # {filepath: sha256_hash}
│   ├── packages.json       # {pkgpath: {caps, callees, hash}}
│   ├── callgraph.json      # Full call graph for propagation
│   └── inferred-capsmap.txt # Generated caps for analyzed packages
```

### Invalidation Rules

**File-level**: SHA-256 hash of each `.go` file. If hash unchanged → skip re-parse.

**Package-level**: A package needs re-analysis if ANY of:
- Its own files changed (hash mismatch)
- Any imported package's API changed (exported function signatures or caps changed)
- Capsmap entries used by the package changed

**Propagation-level**: Re-run fixpoint if:
- Any package's direct capabilities changed
- Any capsmap entry changed

### Algorithm

1. Load cache from `.rivus-cache/v1/`
2. For each package in scope:
   a. Hash all `.go` files
   b. If all hashes match cached → use cached package caps
   c. Else → re-analyze package (SSA build + detection)
3. Merge all package caps + capsmap
4. If any package changed → re-run fixpoint propagation
5. Write updated cache

### Why This Works

- **File unchanged**: The Go AST and types are deterministic for the same source. If the file hash matches, the SSA and capabilities are identical.
- **Import unchanged**: Go's type system is structural. If an imported package's exported API hasn't changed, the caller's analysis is unaffected.
- **Capsmap unchanged**: External function capabilities are fixed until the capsmap file changes.

## Output Format

### Diagnostic Structure

```go
type Diagnostic struct {
    Pos      Position   // file:line:column
    Severity Severity   // error, warning, info
    Message  string     // Human-readable description
    Code     string     // "RIVUS_P", "RIVUS_G", etc.
    Function string     // Function being analyzed
    Details  *Details   // Optional: call chain, inheritance info
}

type Details struct {
    Capability   rune       // Which capability is flagged
    Source       Position   // Where the capability originates
    Via          string     // If inherited: which callee, at what call site
    CallChain    []CallSite // Full chain from caller to source
}
```

### Text Output (default)

```
mypkg/handler.go:42:1: warning: HandleRequest has capability P (panic)
    via: log.Fatal at mypkg/handler.go:45:3
    call chain: HandleRequest → log.Fatal

mypkg/handler.go:42:1: warning: HandleRequest has capability G (goroutine)
    via: go worker() at mypkg/handler.go:50:2

mypkg/handler.go:42:1: warning: HandleRequest has capability I (io)
    via: os.Open at mypkg/handler.go:55:10
    inherited from: readConfig (mypkg/config.go:12:1)
```

### JSON Output (`--json`)

```json
{
  "diagnostics": [
    {
      "pos": {"file": "mypkg/handler.go", "line": 42, "column": 1},
      "severity": "warning",
      "code": "RIVUS_P",
      "message": "HandleRequest has capability P (panic)",
      "function": "mypkg.HandleRequest",
      "details": {
        "capability": "P",
        "source": {"file": "mypkg/handler.go", "line": 45, "column": 3},
        "via": "log.Fatal",
        "callChain": [
          {"function": "mypkg.HandleRequest", "pos": {"file": "mypkg/handler.go", "line": 42, "column": 1}},
          {"function": "log.Fatal", "pos": {"file": "mypkg/handler.go", "line": 45, "column": 3}}
        ]
      }
    }
  ],
  "summary": {
    "total_functions": 150,
    "pure_functions": 89,
    "good_functions": 120,
    "capabilities": {"P": 12, "G": 5, "D": 3, "I": 20, "S": 8, "U": 1, "B": 15, "T": 2}
  }
}
```

## Interfaces for VSCode Extension

### Core Interfaces

```go
// Loader loads and parses Go packages.
type Loader interface {
    Load(patterns []string) ([]*Package, error)
    LoadFromFiles(files []string) ([]*Package, error)
}

// Analyzer analyzes packages and returns capabilities.
type Analyzer interface {
    Analyze(pkgs []*Package) (*AnalysisResult, error)
    AnalyzeFunction(fn *Function) (*FunctionCaps, error)
}

// Reporter formats diagnostics for output.
type Reporter interface {
    Format(diags []Diagnostic) string
    FormatJSON(diags []Diagnostic) ([]byte, error)
}

// Cache manages analysis cache.
type Cache interface {
    Load(dir string) (*CacheState, error)
    Save(dir string, state *CacheState) error
    IsStale(pkgPath string, files []string) bool
}
```

### VSCode Extension Integration Points

The VSCode extension would:
1. Implement `Loader` using LSP document symbols (no disk reads)
2. Use the same `Analyzer` (the core engine)
3. Implement `Reporter` using LSP `publishDiagnostics`
4. Use `Cache` with workspace state persistence

## Error Handling

```go
type AnalysisError struct {
    Package string
    File    string
    Line    int
    Message string
    Cause   error
}

type CapsMapError struct {
    Path    string
    Message string
}
```

Errors are non-fatal where possible:
- Unknown callee → warning diagnostic (not a crash)
- Parse error in one file → skip that file, continue with others
- Capsmap syntax error → warning, skip entry

## Testing Strategy

### Unit Tests

Each capability detector has tests with fixture Go files:

```
testdata/
├── panic_basic.go        # P: direct panic call
├── panic_log_fatal.go    # P: log.Fatal
├── goroutine_basic.go    # G: go keyword
├── context_dangling.go   # D: context.Background without cancel
├── context_valid.go      # negative: properly used context
├── io_basic.go           # I: os.Open
└── ...
```

### Integration Tests

Full pipeline on small Go programs:

```
testdata/integration/
├── simple_pure/          # No capabilities
├── mixed_caps/           # Multiple capabilities
└── interface_calls/      # Interface resolution
```

### Snapshot Tests

Output is compared against golden files (like the Rust version's `test_out/`).

### Capsmap Tests

Verify that stdlib capsmap entries are correct.

## Project Structure

```
rivus-linter-go/
├── cmd/
│   └── rivus/
│       └── main.go              # CLI entry point
├── internal/
│   ├── caps/                    # Capability definitions
│   │   ├── caps.go              # Capability type, set operations
│   │   └── detection.go         # Direct capability detection
│   ├── capsmap/                 # CapsMap loading and lookup
│   │   ├── capsmap.go           # CapsMap type
│   │   ├── loader.go            # File loading
│   │   └── lookup.go            # Lookup logic
│   ├── engine/                  # Core analysis engine
│   │   ├── loader.go            # Package loading (go/ssa)
│   │   ├── analyzer.go          # Main analysis pipeline
│   │   ├── callgraph.go         # Call graph construction
│   │   ├── propagation.go       # Fixpoint propagation
│   │   └── interfaces.go        # Interface resolution
│   ├── cache/                   # Cache layer
│   │   ├── cache.go             # Cache interface
│   │   └── filecache.go         # File-based cache implementation
│   └── report/                  # Output formatting
│       ├── text.go              # Text reporter
│       └── json.go              # JSON reporter
├── caps/                        # Capsmap data files
│   ├── std                      # Pre-built stdlib caps
│   └── user                     # User caps (example)
├── testdata/                    # Test fixtures
├── docs/
│   └── superpowers/
│       └── specs/               # Design specs
├── go.mod
└── go.sum
```

## Dependencies

- `golang.org/x/tools/go/packages` — package loading
- `golang.org/x/tools/go/ssa` — SSA construction
- `golang.org/x/tools/go/ssa/ssautil` — SSA utilities
- `github.com/spf13/cobra` — CLI framework (optional, or use stdlib `flag`)
- Standard library: `go/ast`, `go/parser`, `go/types`, `crypto/sha256`, `encoding/json`
