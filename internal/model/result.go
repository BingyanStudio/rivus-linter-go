package model

import "time"

// Flag represents a single side-effect occurrence in a function.
type Flag struct {
	Type     FlagType `json:"type"`
	Position Position `json:"position"`
	// FuncName is the function that produces this flag.
	// For own flags, this is the function being analyzed.
	// For inherited flags, this is the source function (same as Via).
	FuncName string  `json:"func_name"`
	Via      *string `json:"via,omitempty"` // non-nil if inherited from another function
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
