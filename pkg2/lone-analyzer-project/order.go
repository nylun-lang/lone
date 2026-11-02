package loneproject

import (
	"fmt"
	"sort"
	"strings"

	loneanalyzer "lone/pkg2/lone-analyzer"
	loneast "lone/pkg2/lone-parser-ast"
	lonelogger "lone/pkg2/lone-shared-logger"
)

// MergedProject contains the unified, topologically ordered declarations of a multi-file project.
type MergedProject struct {
	OrderedFiles []*ProjectFile
	MergedAst    any               // Standard []any AST representation containing all declarations in order
	SymbolMap    map[string]string // Symbol name -> file defining it
}

// AnalyzeAndOrder resolves multi-file dependencies, checks for conflicts, and orders declarations topologically.
func (p *Project) AnalyzeAndOrder() (*MergedProject, []loneanalyzer.Diagnostic, error) {
	var diags []loneanalyzer.Diagnostic

	// 1. Build Global Symbol Map & check for duplicate/conflicting symbols
	symbolToFile := make(map[string]string)
	for _, file := range p.Files {
		for _, sym := range file.Provides {
			if existingFile, exists := symbolToFile[sym]; exists && existingFile != file.Path {
				pos := file.Positions[sym]
				diags = append(diags, loneanalyzer.Diagnostic{
					Severity: loneanalyzer.SeverityError,
					Message:  fmt.Sprintf("duplicate declaration of symbol %q: already defined in %s", sym, existingFile),
					Pos:      pos,
					Rule:     "project/duplicate-symbol",
				})
			} else {
				symbolToFile[sym] = file.Path
			}
		}
	}

	if len(diags) > 0 {
		return nil, diags, fmt.Errorf("project has conflicting symbol declarations")
	}

	// 2. Build File Dependency Graph
	// deps[A] = set of files that A depends on (A requires symbols provided by B)
	deps := make(map[string]map[string]bool)
	for path, file := range p.Files {
		deps[path] = make(map[string]bool)
		for _, req := range file.Requires {
			providerFile, exists := symbolToFile[req]
			if exists && providerFile != path {
				deps[path][providerFile] = true
			}
		}
	}

	// 3. Topological Sort (Kahn's algorithm with stable sorting for determinism)
	sortedPaths, err := topologicalSort(deps)
	if err != nil {
		return nil, diags, err
	}

	// 4. Construct Merged Project in topological order
	var orderedFiles []*ProjectFile
	var mergedDecls []any

	for _, path := range sortedPaths {
		file := p.Files[path]
		orderedFiles = append(orderedFiles, file)

		// Pack each declaration as [pos, decl] tuple matching loneparser structure
		for _, decl := range file.Decls {
			pos := loneast.Position{}
			if fn, ok := decl.(loneast.FN_DEC); ok {
				pos = fn.Pos
			} else if bd, ok := decl.(loneast.BIND_DEC); ok {
				pos = bd.Pos
			}
			mergedDecls = append(mergedDecls, []any{pos, decl})
		}
	}

	return &MergedProject{
		OrderedFiles: orderedFiles,
		MergedAst:    mergedDecls,
		SymbolMap:    symbolToFile,
	}, diags, nil
}

func topologicalSort(deps map[string]map[string]bool) ([]string, error) {
	// inDegree[node] = number of dependencies node is waiting on
	inDegree := make(map[string]int)
	// dependents[node] = list of nodes that depend on node
	dependents := make(map[string][]string)

	for node := range deps {
		inDegree[node] = len(deps[node])
		for dep := range deps[node] {
			dependents[dep] = append(dependents[dep], node)
		}
	}

	// Collect nodes with 0 in-degree
	var queue []string
	for node, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, node)
		}
	}

	// Sort queue for deterministic ordering
	sort.Strings(queue)

	var result []string
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		result = append(result, curr)

		for _, dep := range dependents[curr] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
				sort.Strings(queue) // keep stable
			}
		}
	}

	// If result length doesn't match total nodes, there is a cycle
	if len(result) < len(deps) {
		cycle := findCycle(deps)
		return nil, fmt.Errorf("circular dependency detected in project: %s", strings.Join(cycle, " -> "))
	}

	return result, nil
}

func findCycle(deps map[string]map[string]bool) []string {
	visited := make(map[string]int) // 0: unvisited, 1: visiting, 2: visited
	var path []string
	var cycle []string

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = 1
		path = append(path, node)

		for dep := range deps[node] {
			if visited[dep] == 1 {
				// Found cycle
				idx := -1
				for i, n := range path {
					if n == dep {
						idx = i
						break
					}
				}
				if idx != -1 {
					cycle = append(cycle, path[idx:]...)
					cycle = append(cycle, dep)
				}
				return true
			} else if visited[dep] == 0 {
				if dfs(dep) {
					return true
				}
			}
		}

		visited[node] = 2
		path = path[:len(path)-1]
		return false
	}

	for node := range deps {
		if visited[node] == 0 {
			if dfs(node) {
				break
			}
		}
	}

	if len(cycle) == 0 {
		// Fallback
		for node := range deps {
			cycle = append(cycle, node)
		}
	}
	return cycle
}

// FormatDiagnostics renders project diagnostics using lone-shared-logger.
func (p *Project) FormatDiagnostics(diags []loneanalyzer.Diagnostic) string {
	emitter := lonelogger.NewEmitter(nil, lonelogger.DefaultConfig())
	var sb strings.Builder

	for _, d := range diags {
		var content []byte
		if d.Pos.Col > 0 && d.Pos.Line > 0 {
			// Find file content
			for _, f := range p.Files {
				if f.Positions[d.Rule] == d.Pos {
					content = f.Content
					break
				}
			}
		}
		diagObj := d.ToLoggerDiag("project")
		sb.WriteString(emitter.Format(diagObj, content))
		sb.WriteString("\n")
	}
	return sb.String()
}
