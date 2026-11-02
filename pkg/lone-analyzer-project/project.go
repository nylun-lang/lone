package loneproject

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	loneast "lone/pkg/lone-parser-ast"
	loneparser "lone/pkg/lone-parser"
)

// ProjectFile represents a single source file in a Lone project.
type ProjectFile struct {
	Path        string           // Normalized relative or absolute path
	Content     []byte           // Raw source content
	Ast         any              // Parsed AST declarations
	Decls       []any            // Flat list of declarations in the file
	Imports     []loneast.ImportDecl // Flat list of import declarations in the file
	Provides    []string         // Top-level symbols defined by this file (functions, globals)
	Requires    []string         // Symbols referenced by this file
	Positions   map[string]loneast.Position // Symbol -> defining position
}

// Project represents a collection of source files forming a cohesive compilation unit.
type Project struct {
	RootDir string
	Files   map[string]*ProjectFile
}

// NewProject constructs an empty Project rooted at rootDir.
func NewProject(rootDir string) *Project {
	return &Project{
		RootDir: rootDir,
		Files:   make(map[string]*ProjectFile),
	}
}

// AddFile parses and registers a source file into the project.
func (p *Project) AddFile(filePath string, content []byte) (*ProjectFile, error) {
	normPath := filepath.Clean(filePath)

	rawAst, err := loneparser.Parse(normPath, content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", normPath, err)
	}

	decls := extractDeclarations(rawAst)
	imports := extractImports(decls)
	provides, positions := extractProvidedSymbols(decls)
	requires := extractRequiredSymbols(decls)

	pf := &ProjectFile{
		Path:      normPath,
		Content:   content,
		Ast:       rawAst,
		Decls:     decls,
		Imports:   imports,
		Provides:  provides,
		Requires:  requires,
		Positions: positions,
	}

	p.Files[normPath] = pf
	return pf, nil
}

// ScanDir recursively scans dir for all .lone files and adds them to the project.
func (p *Project) ScanDir(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip hidden directories, git, node_modules, _local, etc.
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "_local" || name == "node_modules" || name == "_old" {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(d.Name(), ".lone") {
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", path, err)
			}
			if _, err := p.AddFile(path, content); err != nil {
				return err
			}
		}
		return nil
	})
}

func extractDeclarations(ast any) []any {
	rawList, ok := ast.([]any)
	if !ok {
		return nil
	}

	var decls []any
	for _, item := range rawList {
		if pair, ok := item.([]any); ok && len(pair) >= 2 {
			decls = append(decls, pair[1])
		} else {
			decls = append(decls, item)
		}
	}
	return decls
}

func extractProvidedSymbols(decls []any) ([]string, map[string]loneast.Position) {
	var symbols []string
	positions := make(map[string]loneast.Position)

	for _, d := range decls {
		switch v := d.(type) {
		case loneast.FN_DEC:
			symbols = append(symbols, v.Name)
			positions[v.Name] = v.Pos
		case loneast.BIND_DEC:
			symbols = append(symbols, v.Name)
			positions[v.Name] = v.Pos
		case loneast.TYPE_DEC:
			symbols = append(symbols, v.Name)
			positions[v.Name] = v.Pos
		}
	}
	return symbols, positions
}

func extractImports(decls []any) []loneast.ImportDecl {
	var imports []loneast.ImportDecl
	for _, d := range decls {
		if imp, ok := d.(loneast.ImportDecl); ok {
			imports = append(imports, imp)
		}
	}
	return imports
}

func extractRequiredSymbols(decls []any) []string {
	seen := make(map[string]bool)
	var required []string

	for _, d := range decls {
		switch v := d.(type) {
		case loneast.ImportDecl:
			for _, sym := range v.Symbols {
				seen[sym] = true
			}
		case loneast.FN_DEC:
			scanBlockForReferences(v.Body, seen)
		case loneast.BIND_DEC:
			scanExprForReferences(v.Value, seen)
		}
	}

	for sym := range seen {
		// Ignore intrinsics starting with '#'
		if !strings.HasPrefix(sym, "#") {
			required = append(required, sym)
		}
	}
	return required
}

func scanBlockForReferences(block loneast.Block, seen map[string]bool) {
	for _, stmt := range block.Stmts {
		switch s := stmt.(type) {
		case loneast.BIND_DEC:
			scanExprForReferences(s.Value, seen)
		case loneast.ReturnStmt:
			scanExprForReferences(s.Value, seen)
		case loneast.Expr:
			scanExprForReferences(s, seen)
		case []any:
			if len(s) >= 2 {
				if expr, ok := s[1].(loneast.Expr); ok {
					scanExprForReferences(expr, seen)
				}
			}
		}
	}
}

func scanExprForReferences(expr loneast.Expr, seen map[string]bool) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case loneast.NameRef:
		seen[e.Name] = true
	case loneast.BinaryExpr:
		scanExprForReferences(e.Left, seen)
		scanExprForReferences(e.Right, seen)
	case loneast.UnaryExpr:
		scanExprForReferences(e.Expr, seen)
	case loneast.PostFixExpr:
		scanExprForReferences(e.Base, seen)
		for _, suf := range e.Suffix {
			if call, ok := suf.(loneast.SuffixCall); ok {
				for _, arg := range call.Args {
					scanExprForReferences(arg, seen)
				}
			}
		}
	case loneast.IfStmt:
		scanExprForReferences(e.Cond, seen)
		scanBlockForReferences(e.Then, seen)
		if elseBlock, ok := e.Else.(loneast.Block); ok {
			scanBlockForReferences(elseBlock, seen)
		} else if elseIf, ok := e.Else.(loneast.IfStmt); ok {
			scanExprForReferences(elseIf, seen)
		}
	case loneast.WhileStmt:
		scanExprForReferences(e.Cond, seen)
		scanBlockForReferences(e.Body, seen)
	case loneast.DropStmt:
		scanExprForReferences(e.Expr, seen)
	case loneast.BreakStmt:
		scanExprForReferences(e.Value, seen)
	}
}
