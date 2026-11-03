package loneproject

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	loneanalyzer "lone/pkg2/lone-analyzer"
	lonelexer "lone/pkg2/lone-lexer"
	loneparser "lone/pkg2/lone-parser"
	loneast "lone/pkg2/lone-parser-ast"
)

// Resolver resolves module imports (:std:*, relative .foo) across projects.
type Resolver struct {
	StdLibDir    string
	visited      map[string][]any
	visiting     map[string]bool
	visitStack   []string
	orderedFiles []string
	diags        []loneanalyzer.Diagnostic
}

// NewResolver creates a new module resolver.
func NewResolver(stdLibDir string) *Resolver {
	return &Resolver{
		StdLibDir: stdLibDir,
		visited:   make(map[string][]any),
		visiting:  make(map[string]bool),
	}
}

// ResolveAST is a high-level helper to resolve all imports in rawAst starting from entryFile.
func ResolveAST(entryFile string, rawAst []any) ([]any, []loneanalyzer.Diagnostic) {
	r := NewResolver("")
	return r.ResolveEntry(entryFile, rawAst)
}

// ResolveEntry resolves an entry file's declarations and returns an ordered slice of all declarations.
func (r *Resolver) ResolveEntry(entryFile string, decls []any) ([]any, []loneanalyzer.Diagnostic) {
	absEntry := entryFile
	if entryFile != "" {
		if abs, err := filepath.Abs(entryFile); err == nil {
			absEntry = filepath.Clean(abs)
		}
	}

	r.visiting[absEntry] = true
	r.visitStack = append(r.visitStack, absEntry)
	r.resolveDecls(absEntry, decls)
	r.visitStack = r.visitStack[:len(r.visitStack)-1]
	delete(r.visiting, absEntry)

	var combined []any
	for _, file := range r.orderedFiles {
		if file != absEntry {
			combined = append(combined, r.visited[file]...)
		}
	}
	combined = append(combined, decls...)
	return combined, r.diags
}

func (r *Resolver) resolveDecls(filePath string, decls []any) {
	for _, d := range decls {
		var imp *loneast.ImportDecl
		switch id := d.(type) {
		case loneast.ImportDecl:
			imp = &id
		case *loneast.ImportDecl:
			imp = id
		}

		if imp == nil {
			continue
		}

		targetPath, err := r.resolveModulePath(filePath, imp.Source)
		if err != nil {
			r.diags = append(r.diags, loneanalyzer.Diagnostic{
				Severity: loneanalyzer.SeverityError,
				Span:     imp.Span_,
				Rule:     "import/module-not-found",
				Message:  err.Error(),
			})
			continue
		}

		r.loadModule(targetPath, imp.Span_)
	}
}

func (r *Resolver) loadModule(absPath string, impSpan loneast.Span) {
	cleanPath := filepath.Clean(absPath)
	if _, ok := r.visited[cleanPath]; ok {
		return
	}

	if r.visiting[cleanPath] {
		chain := append([]string{}, r.visitStack...)
		chain = append(chain, cleanPath)
		for i := range chain {
			chain[i] = filepath.Base(chain[i])
		}
		r.diags = append(r.diags, loneanalyzer.Diagnostic{
			Severity: loneanalyzer.SeverityError,
			Span:     impSpan,
			Rule:     "import/cycle",
			Message:  fmt.Sprintf("circular import detected: %s", strings.Join(chain, " -> ")),
		})
		return
	}

	content, err := os.ReadFile(cleanPath)
	if err != nil {
		r.diags = append(r.diags, loneanalyzer.Diagnostic{
			Severity: loneanalyzer.SeverityError,
			Span:     impSpan,
			Rule:     "import/read-error",
			Message:  fmt.Sprintf("failed to read module file %s: %v", cleanPath, err),
		})
		return
	}

	tokens, err := lonelexer.NewLexer(content).Tokenize()
	if err != nil {
		r.diags = append(r.diags, loneanalyzer.Diagnostic{
			Severity: loneanalyzer.SeverityError,
			Span:     impSpan,
			Rule:     "import/lex-error",
			Message:  fmt.Sprintf("lexer error in imported module %s: %v", filepath.Base(cleanPath), err),
		})
		return
	}

	parser := loneparser.NewParser(tokens)
	decls, err := parser.ParseProgram()
	if err != nil {
		r.diags = append(r.diags, loneanalyzer.Diagnostic{
			Severity: loneanalyzer.SeverityError,
			Span:     impSpan,
			Rule:     "import/parse-error",
			Message:  fmt.Sprintf("syntax error in imported module %s: %v", filepath.Base(cleanPath), err),
		})
		return
	}

	r.visiting[cleanPath] = true
	r.visitStack = append(r.visitStack, cleanPath)
	r.resolveDecls(cleanPath, decls)
	r.visitStack = r.visitStack[:len(r.visitStack)-1]
	delete(r.visiting, cleanPath)

	r.visited[cleanPath] = decls
	r.orderedFiles = append(r.orderedFiles, cleanPath)
}

func (r *Resolver) resolveModulePath(fromFile, source string) (string, error) {
	if strings.HasPrefix(source, ":") {
		// Namespace import: :std:io -> lib2/std/io.lone (or fallback to libraries/std/io.lone)
		parts := strings.Split(strings.TrimPrefix(source, ":"), ":")
		if len(parts) == 0 || parts[0] == "" {
			return "", fmt.Errorf("invalid namespace import %q", source)
		}
		if parts[0] != "std" {
			return "", fmt.Errorf("unknown module namespace %q", source)
		}

		stdDir := r.findStdLibDir(fromFile)
		if stdDir == "" {
			return "", fmt.Errorf("could not locate Lone standard library directory for %q", source)
		}

		var candidates []string
		if len(parts) == 1 {
			candidates = append(candidates, filepath.Join(stdDir, "std.lone"))
		} else {
			subPath := filepath.Join(parts[1:]...)
			candidates = append(candidates,
				filepath.Join(stdDir, subPath+".lone"),
				filepath.Join(stdDir, subPath, "std.lone"),
			)
		}

		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && !info.IsDir() {
				return c, nil
			}
		}
		return "", fmt.Errorf("standard library module %q not found in %s", source, stdDir)
	}

	// Relative import: .helper or ..utils
	baseDir := filepath.Dir(fromFile)
	relPath := source
	if strings.HasPrefix(relPath, ".") {
		relPath = strings.TrimPrefix(relPath, ".")
		target := filepath.Join(baseDir, relPath+".lone")
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			return target, nil
		}
	}

	return "", fmt.Errorf("cannot resolve relative import %q from %s", source, fromFile)
}

func (r *Resolver) findStdLibDir(fromFile string) string {
	if r.StdLibDir != "" {
		return r.StdLibDir
	}

	// Check environment variable
	if env := os.Getenv("LONE_STD_DIR"); env != "" {
		if info, err := os.Stat(env); err == nil && info.IsDir() {
			return env
		}
	}

	// Walk upwards from fromFile looking for lib2/std or libraries/std
	dir := filepath.Dir(fromFile)
	for i := 0; i < 10; i++ {
		// Prefer lib2/std
		lib2Std := filepath.Join(dir, "lib2", "std")
		if info, err := os.Stat(lib2Std); err == nil && info.IsDir() {
			return lib2Std
		}
		// Fallback to libraries/std
		legacyStd := filepath.Join(dir, "libraries", "std")
		if info, err := os.Stat(legacyStd); err == nil && info.IsDir() {
			return legacyStd
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Check current working directory
	if cwd, err := os.Getwd(); err == nil {
		lib2Std := filepath.Join(cwd, "lib2", "std")
		if info, err := os.Stat(lib2Std); err == nil && info.IsDir() {
			return lib2Std
		}
		legacyStd := filepath.Join(cwd, "libraries", "std")
		if info, err := os.Stat(legacyStd); err == nil && info.IsDir() {
			return legacyStd
		}
	}

	return ""
}
