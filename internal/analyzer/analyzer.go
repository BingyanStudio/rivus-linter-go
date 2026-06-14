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
		pkgPath := pkg.PkgPath
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

// ClearCache clears the analysis cache.
func (a *Analyzer) ClearCache() error {
	return a.cache.Clear()
}
