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
