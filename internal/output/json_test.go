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
