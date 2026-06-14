package output

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/BingyanStudio/rivus-linter-go/internal/model"
)

// Table formats the analysis result as a human-readable table.
func Table(w io.Writer, result *model.AnalysisResult) error {
	if len(result.Packages) == 0 {
		fmt.Fprintln(w, "No packages analyzed.")
		return nil
	}

	// Collect all functions with flags.
	type row struct {
		FuncName string
		Flags    string
		Details  string
	}

	var rows []row
	for _, pkg := range result.Packages {
		for _, fn := range pkg.Functions {
			if len(fn.Flags) == 0 {
				continue
			}

			// Collect unique flag types.
			flagTypes := make(map[model.FlagType]bool)
			for _, f := range fn.Flags {
				flagTypes[f.Type] = true
			}
			var flagNames []string
			for ft := range flagTypes {
				flagNames = append(flagNames, ft.String())
			}
			sort.Strings(flagNames)

			// Build details.
			var details []string
			for _, f := range fn.Flags {
				if f.Via != nil {
					details = append(details, fmt.Sprintf("%s via %s (%s:%d)",
						f.Type, *f.Via, f.Position.File, f.Position.Line))
				} else {
					details = append(details, fmt.Sprintf("%s at %s:%d",
						f.Type, f.Position.File, f.Position.Line))
				}
			}

			fullName := fn.FuncName
			if pkg.Path != "" {
				fullName = pkg.Path + "." + fn.FuncName
			}

			rows = append(rows, row{
				FuncName: fullName,
				Flags:    strings.Join(flagNames, ", "),
				Details:  strings.Join(details, "; "),
			})
		}
	}

	if len(rows) == 0 {
		fmt.Fprintln(w, "No side effects detected.")
		return nil
	}

	// Sort by function name.
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].FuncName < rows[j].FuncName
	})

	// Print header.
	fmt.Fprintf(w, "%-40s %-10s %s\n", "Function", "Flags", "Details")
	fmt.Fprintln(w, strings.Repeat("─", 80))

	// Print rows.
	for _, r := range rows {
		fmt.Fprintf(w, "%-40s %-10s %s\n", r.FuncName, r.Flags, r.Details)
	}

	return nil
}
