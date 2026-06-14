package output

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/BingyanStudio/rivus-linter-go/internal/model"
)

// pkgEntry holds a package's flagged functions for table output.
type pkgEntry struct {
	Path string
	Fns  []fnEntry
}

// fnEntry holds a function's flag summary for table output.
type fnEntry struct {
	Name      string
	FlagStr   string
	FlagItems []flagItem
}

// flagItem holds a single flag's display info for table output.
type flagItem struct {
	Letter string // "P", "I", etc.
	Source string // "at handler.go:12" or "via ReadFile (io.go:45)"
}

// Table formats the analysis result as a human-readable table.
//
// The output groups functions by package, shows each function's flag set
// in compact {P, I} notation, and lists each flag with its source.
// Inherited flags show the source function name and call-site location.
//
// Example:
//
//	rivus side-effect report
//	─────────────────────────────────────────────────────────────
//
//	  example.com/pkg
//
//	    ProcessData  {P, I}
//	      P  panic at handler.go:12
//	      I  via ReadFile (io.go:45)
//
//	    ReadFile     {I}
//	      I  os.ReadFile at io.go:45
func Table(w io.Writer, result *model.AnalysisResult) error {
	if len(result.Packages) == 0 {
		fmt.Fprintln(w, "No packages analyzed.")
		return nil
	}

	var packages []pkgEntry

	for _, pkg := range result.Packages {
		var fns []fnEntry
		for _, fn := range pkg.Functions {
			if len(fn.Flags) == 0 {
				continue
			}

			// Build compact flag string: {P, I, S}
			flagSet := make(map[model.FlagType]bool)
			for _, f := range fn.Flags {
				flagSet[f.Type] = true
			}
			var letters []string
			for ft := range flagSet {
				letters = append(letters, ft.String())
			}
			sort.Strings(letters)
			flagStr := "{" + strings.Join(letters, ", ") + "}"

			// Build flag items with source info.
			var items []flagItem
			for _, f := range fn.Flags {
				item := flagItem{Letter: f.Type.String()}
				if f.Via != nil {
					// Inherited flag: show source function and call site.
					item.Source = fmt.Sprintf("via %s (%s:%d)",
						*f.Via, f.Position.File, f.Position.Line)
				} else {
					// Own flag: show function name and location.
					item.Source = fmt.Sprintf("%s at %s:%d",
						f.FuncName, f.Position.File, f.Position.Line)
				}
				items = append(items, item)
			}

			fns = append(fns, fnEntry{
				Name:      fn.FuncName,
				FlagStr:   flagStr,
				FlagItems: items,
			})
		}

		if len(fns) > 0 {
			// Sort functions by name.
			sort.Slice(fns, func(i, j int) bool {
				return fns[i].Name < fns[j].Name
			})
			packages = append(packages, pkgEntry{Path: pkg.Path, Fns: fns})
		}
	}

	if len(packages) == 0 {
		fmt.Fprintln(w, "No side effects detected.")
		return nil
	}

	// Sort packages by path.
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Path < packages[j].Path
	})

	// Find the longest function name for alignment.
	maxNameLen := 0
	for _, pkg := range packages {
		for _, fn := range pkg.Fns {
			if len(fn.Name) > maxNameLen {
				maxNameLen = len(fn.Name)
			}
		}
	}
	if maxNameLen < 12 {
		maxNameLen = 12
	}

	// Print header.
	fmt.Fprintln(w, "rivus side-effect report")
	fmt.Fprintln(w, strings.Repeat("─", 60))
	fmt.Fprintln(w)

	// Print each package.
	for _, pkg := range packages {
		// Package header.
		if pkg.Path != "" {
			fmt.Fprintf(w, "  %s\n\n", pkg.Path)
		}

		// Print each function.
		for _, fn := range pkg.Fns {
			fmt.Fprintf(w, "    %-*s  %s\n", maxNameLen, fn.Name, fn.FlagStr)
			for _, item := range fn.FlagItems {
				fmt.Fprintf(w, "      %s  %s\n", item.Letter, item.Source)
			}
		}
		fmt.Fprintln(w)
	}

	return nil
}
