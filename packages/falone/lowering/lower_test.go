package lowering_test

import (
	"strings"
	"testing"

	"falone/lowering"
	"falone/mirtypes"
	"lonealyzer/analyze"
	"lonelang/parser"
)

func TestLowerFunctionToMir(t *testing.T) {
	code := `
fn add(a: i32, b: i32) i32 {
	return a + b
}

fn check_value(x: i32) i32 {
	if x < 10 {
		return 1
	} else {
		return 2
	}
}
`
	ast, err := parser.Parse("test.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := analyze.NewAnalyzer("test.lone", analyze.DefaultConfig())
	prog, diags := analyzer.AnalyzeTyped(ast)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	mirMod, err := lowering.LowerProgram(prog)
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	if len(mirMod.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(mirMod.Functions))
	}

	mirText := mirtypes.FormatModule(mirMod)
	if !strings.Contains(mirText, "fn @add(%a: i32, %b: i32) -> i32") {
		t.Errorf("expected @add function signature, got:\n%s", mirText)
	}
	if !strings.Contains(mirText, "add i32") {
		t.Errorf("expected add i32 instruction, got:\n%s", mirText)
	}
	if !strings.Contains(mirText, "fn @check_value(%x: i32) -> i32") {
		t.Errorf("expected @check_value signature, got:\n%s", mirText)
	}
	if !strings.Contains(mirText, "branch") {
		t.Errorf("expected branch instruction, got:\n%s", mirText)
	}
}

func TestLowerPrintIntrinsicToMir(t *testing.T) {
	code := `
fn main() void {
	#print("Hello from Lone!\n")
}
`
	ast, err := parser.Parse("hello.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := analyze.NewAnalyzer("hello.lone", analyze.DefaultConfig())
	prog, diags := analyzer.AnalyzeTyped(ast)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	mirMod, err := lowering.LowerProgram(prog)
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	if len(mirMod.DataSegments) != 1 {
		t.Fatalf("expected 1 data segment, got %d", len(mirMod.DataSegments))
	}
	if mirMod.DataSegments[0].Offset != 16 {
		t.Errorf("expected data offset 16, got %d", mirMod.DataSegments[0].Offset)
	}
	if mirMod.DataSegments[0].Value != "Hello from Lone!\n" {
		t.Errorf("expected string 'Hello from Lone!\n', got %q", mirMod.DataSegments[0].Value)
	}

	mirText := mirtypes.FormatModule(mirMod)
	if !strings.Contains(mirText, "data @16 \"Hello from Lone!\\n\"") {
		t.Errorf("expected data segment in mir text, got:\n%s", mirText)
	}
	if !strings.Contains(mirText, "call @__lone_rt_print 16 17") {
		t.Errorf("expected call @__lone_rt_print 16 17, got:\n%s", mirText)
	}
}
