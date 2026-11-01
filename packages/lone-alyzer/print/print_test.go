package print_test

import (
	"strings"
	"testing"

	"lonealyzer/analyze"
	"lonealyzer/print"
	"lonelang/parser"
)

func TestPrintProgram(t *testing.T) {
	code := `
var count = 42

fn add(a: i32, b: i32) i32 {
	drop 100
	return a + b
}
`
	ast, err := parser.Parse("print_test.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := analyze.NewAnalyzer("print_test.lone", analyze.DefaultConfig())
	prog, diags := analyzer.AnalyzeTyped(ast)

	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got: %v", diags)
	}

	out := print.PrintProgram(prog)
	if !strings.Contains(out, "var count: i32 = 42") {
		t.Errorf("expected var declaration in printout, got:\n%s", out)
	}
	if !strings.Contains(out, "fn add(a: i32, b: i32) i32 {") {
		t.Errorf("expected fn signature in printout, got:\n%s", out)
	}
	if !strings.Contains(out, "drop 100") {
		t.Errorf("expected drop stmt in printout, got:\n%s", out)
	}
	if !strings.Contains(out, "return (a + b)") {
		t.Errorf("expected return in printout, got:\n%s", out)
	}
}
