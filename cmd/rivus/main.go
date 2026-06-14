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
			fmt.Fprint(stdout, usage)
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
