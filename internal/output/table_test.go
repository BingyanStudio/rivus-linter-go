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
