package analyzer

import (
	"go/token"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/ssa"

	"github.com/BingyanStudio/rivus-linter-go/internal/model"
)

// FuncFlags stores the computed flags for each function.
type FuncFlags struct {
	// Flags is the complete set of flags (own + inherited).
	Flags model.FlagSet
	// Details contains the individual flag entries with positions.
	Details []model.Flag
}

// BuildCallGraph builds a call graph using the CHA (Class Hierarchy Analysis) algorithm.
func BuildCallGraph(prog *ssa.Program) *callgraph.Graph {
	return cha.CallGraph(prog)
}

// AnalyzeCallGraph performs bottom-up analysis on the call graph.
// It returns a map from function key ("pkg.Func") to FuncFlags.
func AnalyzeCallGraph(cg *callgraph.Graph, fset *token.FileSet) map[string]*FuncFlags {
	results := make(map[string]*FuncFlags)

	// Collect all functions from the call graph.
	allFuncs := collectFunctions(cg)

	// Compute topological order (bottom-up).
	order := topoSort(allFuncs, cg)

	// Process in bottom-up order.
	for _, fn := range order {
		key := funcKey(fn)
		if _, exists := results[key]; exists {
			continue
		}

		// Scan this function's own instructions.
		ownFlags := ScanFunc(fn, fset)
		ownFlags = append(ownFlags, ScanGlobals(fn, fset)...)

		// Build the FlagSet from own flags.
		var flagSet model.FlagSet
		var details []model.Flag
		for _, f := range ownFlags {
			flagSet = flagSet.Add(f.Type)
			details = append(details, f)
		}

		// Inherit flags from callees.
		if cg.Nodes[fn] != nil {
			for _, edge := range cg.Nodes[fn].Out {
				callee := edge.Callee.Func
				calleeKey := funcKey(callee)
				if calleeFlags, ok := results[calleeKey]; ok {
					// Inherit flags that we don't already have.
					for _, ft := range model.AllFlags() {
						if calleeFlags.Flags.Has(ft) && !flagSet.Has(ft) {
							flagSet = flagSet.Add(ft)
							viaName := callee.Name()
							details = append(details, model.Flag{
								Type:     ft,
								Position: model.PositionFromToken(fset, edge.Pos()),
								Via:      &viaName,
							})
						}
					}
				}
			}
		}

		results[key] = &FuncFlags{
			Flags:   flagSet,
			Details: details,
		}
	}

	return results
}

// funcKey returns a unique key for a function.
func funcKey(fn *ssa.Function) string {
	if fn == nil {
		return ""
	}
	if fn.Pkg != nil {
		return fn.Pkg.Pkg.Path() + "." + fn.Name()
	}
	return fn.Name()
}

// collectFunctions returns all functions in the call graph.
func collectFunctions(cg *callgraph.Graph) []*ssa.Function {
	var funcs []*ssa.Function
	for fn, node := range cg.Nodes {
		if fn != nil && node != nil {
			funcs = append(funcs, fn)
		}
	}
	return funcs
}

// topoSort returns functions in topological order (leaves first).
// Uses Kahn's algorithm.
func topoSort(funcs []*ssa.Function, cg *callgraph.Graph) []*ssa.Function {
	// Compute in-degree for each function.
	inDegree := make(map[*ssa.Function]int)
	for _, fn := range funcs {
		if _, ok := inDegree[fn]; !ok {
			inDegree[fn] = 0
		}
		if node := cg.Nodes[fn]; node != nil {
			for _, out := range node.Out {
				if callee := out.Callee.Func; callee != nil {
					inDegree[callee]++
				}
			}
		}
	}

	// Start with functions that have no callees (leaves).
	var queue []*ssa.Function
	for _, fn := range funcs {
		if inDegree[fn] == 0 {
			queue = append(queue, fn)
		}
	}

	var order []*ssa.Function
	for len(queue) > 0 {
		fn := queue[0]
		queue = queue[1:]
		order = append(order, fn)

		if node := cg.Nodes[fn]; node != nil {
			for _, out := range node.Out {
				if callee := out.Callee.Func; callee != nil {
					inDegree[callee]--
					if inDegree[callee] == 0 {
						queue = append(queue, callee)
					}
				}
			}
		}
	}

	// If there are remaining functions (cycles), add them at the end.
	seen := make(map[*ssa.Function]bool)
	for _, fn := range order {
		seen[fn] = true
	}
	for _, fn := range funcs {
		if !seen[fn] {
			order = append(order, fn)
		}
	}

	return order
}
