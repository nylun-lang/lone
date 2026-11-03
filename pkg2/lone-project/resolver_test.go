package loneproject_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lonelexer "lone/pkg2/lone-lexer"
	loneparser "lone/pkg2/lone-parser"
	loneproject "lone/pkg2/lone-project"
)

func parseSnippet(t *testing.T, source string) []any {
	t.Helper()
	tokens, err := lonelexer.NewLexer([]byte(source)).Tokenize()
	if err != nil {
		t.Fatalf("tokenize error: %v", err)
	}
	parser := loneparser.NewParser(tokens)
	ast, err := parser.ParseProgram()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return ast
}

func TestResolveStdImport(t *testing.T) {
	code := `
import .print, .println from :std:io

fn main() void {
	print("Resolved successfully!\n")
}
`
	ast := parseSnippet(t, code)
	combined, diags := loneproject.ResolveAST("test.lone", ast)
	if len(diags) > 0 {
		var msgs []string
		for _, d := range diags {
			msgs = append(msgs, d.String())
		}
		t.Fatalf("unexpected resolver diags: %s", strings.Join(msgs, "\n"))
	}

	if len(combined) <= len(ast) {
		t.Errorf("expected combined AST to have imported std declarations, got %d <= %d", len(combined), len(ast))
	}
}

func TestCircularImportDetection(t *testing.T) {
	tmpDir := t.TempDir()
	modA := filepath.Join(tmpDir, "modA.lone")
	modB := filepath.Join(tmpDir, "modB.lone")

	if err := os.WriteFile(modA, []byte("import .b from .modB\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modB, []byte("import .a from .modA\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ast := parseSnippet(t, "import .a from .modA\n")
	_, diags := loneproject.ResolveAST(filepath.Join(tmpDir, "main.lone"), ast)

	hasCycleError := false
	for _, d := range diags {
		if d.Rule == "import/cycle" {
			hasCycleError = true
			break
		}
	}

	if !hasCycleError {
		t.Errorf("expected import/cycle diagnostic, got: %+v", diags)
	}
}
