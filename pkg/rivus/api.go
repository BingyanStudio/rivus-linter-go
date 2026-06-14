// Package rivus provides the public API for the Go side-effect checker.
// This package is designed to be consumed by VSCode extensions and other tools.
package rivus

import (
	"context"
	"encoding/json"

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

// FromJSON parses JSON bytes into an AnalysisResult.
func FromJSON(data []byte) (*model.AnalysisResult, error) {
	var result model.AnalysisResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
