package loneproject

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	loneanalyzer "lone/pkg/lone-analyzer"
	loneparser "lone/pkg/lone-parser"
	loneast "lone/pkg/lone-parser-ast"
	loneruntime "lone/pkg/lone-runtime"
)

// Resolver resolves module imports (:std:*, relative .foo) and enforces ---fa:wasm host capability gating per file.
type Resolver struct {
	StdLibDir          string
	visited            map[string]*loneast.ModuleScopeDecl
	visiting           map[string]bool
	visitStack         []string
	orderedModules     []*loneast.ModuleScopeDecl
	hostModules        map[string]*loneast.ModuleScopeDecl
	orderedHostModules []*loneast.ModuleScopeDecl
	diags              []loneanalyzer.Diagnostic
}

// NewResolver creates a new module and host capability resolver.
func NewResolver(stdLibDir string) *Resolver {
	return &Resolver{
		StdLibDir:   stdLibDir,
		visited:     make(map[string]*loneast.ModuleScopeDecl),
		visiting:    make(map[string]bool),
		hostModules: make(map[string]*loneast.ModuleScopeDecl),
	}
}

// ResolveAST resolves all imports and ---fa:wasm directives starting from entryFile's parsed AST.
// If entryFile contains no imports or wasm directives, rawAst is returned unchanged.
func ResolveAST(entryFile string, rawAst any) (any, []loneanalyzer.Diagnostic) {
	decls := extractDeclarations(rawAst)
	hasImportsOrDirectives := false
	for _, d := range decls {
		switch d.(type) {
		case loneast.ImportDecl, *loneast.ImportDecl, loneast.WasmDirective, *loneast.WasmDirective:
			hasImportsOrDirectives = true
		}
	}
	if !hasImportsOrDirectives {
		return rawAst, nil
	}

	r := NewResolver("")
	return r.ResolveEntry(entryFile, decls)
}

// ResolveEntry resolves an entry file's declarations and returns a unified AST declaration slice.
func (r *Resolver) ResolveEntry(entryFile string, decls []any) ([]any, []loneanalyzer.Diagnostic) {
	absEntry := entryFile
	if entryFile != "" {
		if abs, err := filepath.Abs(entryFile); err == nil {
			absEntry = filepath.Clean(abs)
		}
	}

	r.visiting[absEntry] = true
	r.visitStack = append(r.visitStack, absEntry)
	entryDecls, _ := r.resolveFileDecls(absEntry, decls)
	r.visitStack = r.visitStack[:len(r.visitStack)-1]
	delete(r.visiting, absEntry)

	var combined []any
	for _, hostMod := range r.orderedHostModules {
		combined = append(combined, *hostMod)
	}
	for _, mod := range r.orderedModules {
		combined = append(combined, *mod)
	}
	combined = append(combined, entryDecls...)
	return combined, r.diags
}

func (r *Resolver) addError(pos loneast.Position, rule, msg string) {
	r.diags = append(r.diags, loneanalyzer.Diagnostic{
		Severity: loneanalyzer.SeverityError,
		Pos:      pos,
		Rule:     rule,
		Message:  msg,
	})
}

func (r *Resolver) loadModule(absPath, moduleID string, importPos loneast.Position) *loneast.ModuleScopeDecl {
	cleanPath := filepath.Clean(absPath)
	if existing, ok := r.visited[cleanPath]; ok {
		return existing
	}

	if r.visiting[cleanPath] {
		chain := append([]string{}, r.visitStack...)
		chain = append(chain, cleanPath)
		for i := range chain {
			chain[i] = filepath.Base(chain[i])
		}
		r.addError(importPos, "import/cycle",
			fmt.Sprintf("circular import detected: %s", strings.Join(chain, " -> ")))
		return nil
	}

	content, err := os.ReadFile(cleanPath)
	if err != nil {
		r.addError(importPos, "import/module-not-found",
			fmt.Sprintf("cannot read imported module %q (%s): %v", moduleID, cleanPath, err))
		return nil
	}

	rawAst, err := loneparser.Parse(cleanPath, content)
	if err != nil {
		r.addError(importPos, "import/parse-error",
			fmt.Sprintf("syntax error in imported module %q (%s): %v", moduleID, filepath.Base(cleanPath), err))
		return nil
	}

	decls := extractDeclarations(rawAst)

	r.visiting[cleanPath] = true
	r.visitStack = append(r.visitStack, cleanPath)
	resolvedDecls, exports := r.resolveFileDecls(cleanPath, decls)
	r.visitStack = r.visitStack[:len(r.visitStack)-1]
	delete(r.visiting, cleanPath)

	mod := &loneast.ModuleScopeDecl{
		ModuleID: moduleID,
		FilePath: cleanPath,
		Decls:    resolvedDecls,
		Exports:  exports,
	}
	r.visited[cleanPath] = mod
	r.orderedModules = append(r.orderedModules, mod)
	return mod
}

func (r *Resolver) resolveFileDecls(filePath string, decls []any) ([]any, map[string]bool) {
	exports := computeModuleExports(decls)
	var out []any
	inlineWasmAuthorized := false

	for _, d := range decls {
		switch node := d.(type) {
		case loneast.WasmDirective:
			if node.Inline {
				inlineWasmAuthorized = true
			} else {
				for _, imp := range node.Imports {
					out = append(out, r.resolveSingleImport(filePath, imp, true)...)
				}
			}
		case *loneast.WasmDirective:
			if node.Inline {
				inlineWasmAuthorized = true
			} else {
				for _, imp := range node.Imports {
					out = append(out, r.resolveSingleImport(filePath, imp, true)...)
				}
			}
		case loneast.ImportDecl:
			out = append(out, r.resolveSingleImport(filePath, node, inlineWasmAuthorized)...)
		case *loneast.ImportDecl:
			out = append(out, r.resolveSingleImport(filePath, *node, inlineWasmAuthorized)...)
		default:
			out = append(out, d)
		}
	}

	return out, exports
}

func (r *Resolver) resolveSingleImport(filePath string, imp loneast.ImportDecl, wasmAuthorized bool) []any {
	if loneruntime.IsHostNamespace(imp.Source) {
		if !wasmAuthorized {
			r.addError(imp.Pos, "capability/unauthorized-host-import",
				fmt.Sprintf("unauthorized host capability import from %q in %s: wrap host imports in an explicit @fa:wasm directive",
					imp.Source, filepath.Base(filePath)))
			return nil
		}

		ns, ok := loneruntime.LookupHostNamespace(imp.Source)
		if !ok {
			r.addError(imp.Pos, "import/unknown-host-namespace",
				fmt.Sprintf("unknown host capability namespace %q", imp.Source))
			return nil
		}

		if imp.Alias != "" {
			if _, exists := r.hostModules[imp.Source]; !exists {
				var hostDecls []any
				hostExports := make(map[string]bool)
				var symNames []string
				for symName := range ns {
					symNames = append(symNames, symName)
				}
				sort.Strings(symNames)
				for _, symName := range symNames {
					spec := ns[symName]
					hostDecls = append(hostDecls, hostSpecToDecl(spec, symName, imp.Pos))
					hostExports[symName] = true
				}
				hm := &loneast.ModuleScopeDecl{
					ModuleID: imp.Source,
					FilePath: imp.Source,
					Decls:    hostDecls,
					Exports:  hostExports,
				}
				r.hostModules[imp.Source] = hm
				r.orderedHostModules = append(r.orderedHostModules, hm)
			}
			return []any{loneast.ResolvedImportDecl{
				ModuleID: imp.Source,
				Source:   imp.Source,
				Alias:    imp.Alias,
				Pos:      imp.Pos,
			}}
		}

		var hostDecls []any
		for _, sym := range imp.Symbols {
			spec, ok := ns[sym]
			if !ok {
				r.addError(imp.Pos, "import/undefined-symbol",
					fmt.Sprintf("host capability %q does not exist in namespace %q", sym, imp.Source))
				continue
			}
			hostDecls = append(hostDecls, hostSpecToDecl(spec, sym, imp.Pos))
		}
		return hostDecls
	}

	// Regular Lone module (:std:* or .relative)
	targetPath, moduleID, err := r.resolveModulePath(filePath, imp.Source)
	if err != nil {
		r.addError(imp.Pos, "import/module-not-found", err.Error())
		return nil
	}

	targetMod := r.loadModule(targetPath, moduleID, imp.Pos)
	if targetMod == nil {
		return nil
	}

	if imp.Alias == "" {
		for _, sym := range imp.Symbols {
			if !targetMod.Exports[sym] {
				r.addError(imp.Pos, "import/undefined-symbol",
					fmt.Sprintf("symbol %q is not exported by module %q", sym, imp.Source))
			}
		}
	}

	return []any{loneast.ResolvedImportDecl{
		ModuleID: targetMod.ModuleID,
		Source:   imp.Source,
		Symbols:  imp.Symbols,
		Alias:    imp.Alias,
		Pos:      imp.Pos,
	}}
}

func hostSpecToDecl(spec loneruntime.HostFuncSpec, localName string, pos loneast.Position) loneast.HostImportDecl {
	var params []loneast.HostImportParam
	for _, p := range spec.Params {
		params = append(params, loneast.HostImportParam{
			Name:     p.Name,
			LoneType: p.LoneType,
			WasmType: p.WasmType,
		})
	}
	return loneast.HostImportDecl{
		Namespace:      spec.Namespace,
		WasmModule:     spec.WasmModule,
		WasmName:       spec.Name,
		LocalName:      localName,
		Params:         params,
		ReturnLoneType: spec.ReturnLoneType,
		ReturnWasmType: spec.ReturnWasmType,
		Pos:            pos,
	}
}

func computeModuleExports(decls []any) map[string]bool {
	exports := make(map[string]bool)
	allLocal := make(map[string]bool)
	pubLocal := make(map[string]bool)

	for _, d := range decls {
		switch n := d.(type) {
		case loneast.FN_DEC:
			allLocal[n.Name] = true
			if n.IsPublic {
				pubLocal[n.Name] = true
			}
		case loneast.BIND_DEC:
			allLocal[n.Name] = true
			if n.IsPublic {
				pubLocal[n.Name] = true
			}
		case loneast.TYPE_DEC:
			allLocal[n.Name] = true
			if n.IsPublic {
				pubLocal[n.Name] = true
			}
		}
	}

	if len(pubLocal) > 0 {
		return pubLocal
	}
	if len(allLocal) > 0 {
		return allLocal
	}

	// Pure barrel file (no local definitions): re-export non-host imported symbols
	for _, d := range decls {
		if imp, ok := d.(loneast.ImportDecl); ok && !loneruntime.IsHostNamespace(imp.Source) {
			for _, sym := range imp.Symbols {
				exports[sym] = true
			}
		}
	}
	return exports
}

func (r *Resolver) resolveModulePath(fromFile, source string) (string, string, error) {
	if strings.HasPrefix(source, ":") {
		parts := strings.Split(strings.TrimPrefix(source, ":"), ":")
		if len(parts) == 0 || parts[0] == "" {
			return "", "", fmt.Errorf("invalid namespace import %q", source)
		}
		if parts[0] != "std" {
			return "", "", fmt.Errorf("unknown module namespace %q", source)
		}

		stdDir := r.findStdLibDir(fromFile)
		if stdDir == "" {
			return "", "", fmt.Errorf("could not locate Lone standard library directory for %q", source)
		}

		var candidates []string
		if len(parts) == 1 {
			candidates = append(candidates, filepath.Join(stdDir, "std.lone"))
		} else {
			subPath := filepath.Join(parts[1:]...)
			candidates = append(candidates,
				filepath.Join(stdDir, subPath+".lone"),
				filepath.Join(stdDir, subPath, "mod.lone"),
				filepath.Join(stdDir, subPath, parts[len(parts)-1]+".lone"),
			)
		}

		for _, cand := range candidates {
			if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
				abs, err := filepath.Abs(cand)
				if err == nil {
					cand = abs
				}
				return filepath.Clean(cand), source, nil
			}
		}
		return "", "", fmt.Errorf("standard library module %q not found in %s", source, stdDir)
	}

	if strings.HasPrefix(source, ".") {
		baseDir := "."
		if fromFile != "" {
			baseDir = filepath.Dir(fromFile)
		}

		dots := 0
		for dots < len(source) && source[dots] == '.' {
			dots++
		}
		rest := source[dots:]

		var candidates []string
		if strings.HasPrefix(rest, "/") {
			rel := strings.TrimSuffix(source, ".lone")
			candidates = append(candidates,
				filepath.Join(baseDir, rel+".lone"),
				filepath.Join(baseDir, rel, "mod.lone"),
			)
		} else {
			dir := baseDir
			for k := 1; k < dots; k++ {
				dir = filepath.Join(dir, "..")
			}
			rel := strings.TrimSuffix(rest, ".lone")
			rel = strings.ReplaceAll(rel, ".", string(filepath.Separator))
			candidates = append(candidates,
				filepath.Join(dir, rel+".lone"),
				filepath.Join(dir, rel, "mod.lone"),
			)
		}

		for _, cand := range candidates {
			if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
				abs, err := filepath.Abs(cand)
				if err == nil {
					cand = abs
				}
				return filepath.Clean(cand), filepath.Clean(cand), nil
			}
		}
		return "", "", fmt.Errorf("relative module %q not found from %s", source, filepath.Base(fromFile))
	}

	return "", "", fmt.Errorf("invalid import source %q: expected ':' namespace or '.' relative path", source)
}

func (r *Resolver) findStdLibDir(fromFile string) string {
	if r.StdLibDir != "" {
		if fi, err := os.Stat(r.StdLibDir); err == nil && fi.IsDir() {
			return r.StdLibDir
		}
	}

	var searchStarts []string
	if fromFile != "" {
		if abs, err := filepath.Abs(filepath.Dir(fromFile)); err == nil {
			searchStarts = append(searchStarts, abs)
		}
	}
	if wd, err := os.Getwd(); err == nil {
		searchStarts = append(searchStarts, wd)
	}
	if _, callerFile, _, ok := runtime.Caller(0); ok {
		repoRoot := filepath.Clean(filepath.Join(filepath.Dir(callerFile), "..", ".."))
		searchStarts = append(searchStarts, repoRoot)
	}

	for _, start := range searchStarts {
		cur := start
		for {
			for _, rel := range []string{filepath.Join("libraries", "std"), filepath.Join("lib", "std")} {
				cand := filepath.Join(cur, rel)
				if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
					return cand
				}
			}
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}
	}

	return ""
}
