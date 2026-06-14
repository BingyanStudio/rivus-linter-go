package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/BingyanStudio/rivus-linter-go/internal/model"
)

func TestTableOutput(t *testing.T) {
	via := "ReadFile"
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
							{Type: model.FlagPanic, Position: model.Position{File: "handler.go", Line: 12, Col: 1}},
							{Type: model.FlagIO, Position: model.Position{File: "io.go", Line: 45, Col: 3}, Via: &via},
						},
					},
					{
						FuncName: "ReadFile",
						Flags: []model.Flag{
							{Type: model.FlagIO, Position: model.Position{File: "io.go", Line: 45, Col: 3}},
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

	// Check header.
	if !strings.Contains(output, "rivus side-effect report") {
		t.Error("expected header 'rivus side-effect report'")
	}

	// Check package name.
	if !strings.Contains(output, "example.com/pkg") {
		t.Error("expected package name 'example.com/pkg'")
	}

	// Check function names.
	if !strings.Contains(output, "ProcessData") {
		t.Error("expected 'ProcessData' in output")
	}
	if !strings.Contains(output, "ReadFile") {
		t.Error("expected 'ReadFile' in output")
	}

	// Check flag set notation.
	if !strings.Contains(output, "{I, P}") {
		t.Error("expected flag set '{I, P}' for ProcessData")
	}
	if !strings.Contains(output, "{I}") {
		t.Error("expected flag set '{I}' for ReadFile")
	}

	// Check inherited flag shows source function.
	if !strings.Contains(output, "via ReadFile") {
		t.Error("expected 'via ReadFile' for inherited I flag")
	}

	// Check own flag shows location.
	if !strings.Contains(output, "at handler.go:12") {
		t.Error("expected 'at handler.go:12' for own P flag")
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

func TestTableMultiplePackages(t *testing.T) {
	result := &model.AnalysisResult{
		Version:   "1.0",
		Timestamp: time.Now(),
		Packages: []model.PackageResult{
			{
				Path: "example.com/a",
				Functions: []model.FuncResult{
					{
						FuncName: "Alpha",
						Flags: []model.Flag{
							{Type: model.FlagIO, Position: model.Position{File: "a.go", Line: 10, Col: 1}},
						},
					},
				},
			},
			{
				Path: "example.com/b",
				Functions: []model.FuncResult{
					{
						FuncName: "Beta",
						Flags: []model.Flag{
							{Type: model.FlagPanic, Position: model.Position{File: "b.go", Line: 20, Col: 1}},
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

	// Both packages should appear.
	if !strings.Contains(output, "example.com/a") {
		t.Error("expected 'example.com/a' in output")
	}
	if !strings.Contains(output, "example.com/b") {
		t.Error("expected 'example.com/b' in output")
	}

	// Both functions should appear.
	if !strings.Contains(output, "Alpha") {
		t.Error("expected 'Alpha' in output")
	}
	if !strings.Contains(output, "Beta") {
		t.Error("expected 'Beta' in output")
	}
}
