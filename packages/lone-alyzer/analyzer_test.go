package lonealyzer_test

import (
	"testing"

	"lonealyzer"
	ir "lonelang/irtypes"
	"lonelang/parser"
)

func TestScopeStack(t *testing.T) {
	stack := lonealyzer.NewScopeStack("main.lone")
	if stack.Current().Kind != lonealyzer.ScopeFile {
		t.Fatalf("expected root scope to be file, got %v", stack.Current().Kind)
	}

	sym1 := &lonealyzer.Symbol{Name: "GLOBAL_VAL", Kind: lonealyzer.SymbolVar}
	if err := stack.Define(sym1); err != nil {
		t.Fatalf("failed to define global: %v", err)
	}

	// Duplicate error
	if err := stack.Define(sym1); err == nil {
		t.Fatalf("expected error on duplicate definition")
	}

	// Push function
	stack.Push(lonealyzer.ScopeFunction, "main")
	if stack.Current().Kind != lonealyzer.ScopeFunction {
		t.Fatalf("expected function scope, got %v", stack.Current().Kind)
	}
	if !stack.HasEnclosing(lonealyzer.ScopeFile) {
		t.Fatalf("expected enclosing file scope")
	}

	// Define local
	sym2 := &lonealyzer.Symbol{Name: "local_val", Kind: lonealyzer.SymbolVar}
	if err := stack.Define(sym2); err != nil {
		t.Fatalf("failed to define local: %v", err)
	}

	// Lookup local and global from inner scope
	if resolved := stack.Resolve("local_val"); resolved == nil {
		t.Fatalf("failed to resolve local_val")
	}
	if resolved := stack.Resolve("GLOBAL_VAL"); resolved == nil {
		t.Fatalf("failed to resolve GLOBAL_VAL from inner scope")
	}

	// Pop
	stack.Pop()
	if stack.Current().Kind != lonealyzer.ScopeFile {
		t.Fatalf("expected to return to file scope, got %v", stack.Current().Kind)
	}
	if resolved := stack.Resolve("local_val"); resolved != nil {
		t.Fatalf("expected local_val not to resolve in file scope")
	}
}

func TestNamingConventionsValidCode(t *testing.T) {
	code := `
var MAX_USERS = 100
var app_name = "lone"

fn calculate_total(item_count: i32, unit_price: f32) f32 {
	var sub_total = 0
	blk: {
		break blk: 42
	}
	return 0
}
`
	ast, err := parser.Parse("test.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := lonealyzer.NewAnalyzer("test.lone", lonealyzer.DefaultConfig())
	diags := analyzer.Analyze(ast)

	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for valid code, got %d: %v", len(diags), diags)
	}
}

func TestNamingConventionsInvalidIdentifiers(t *testing.T) {
	code := `
var BadGlobalName = 10

fn BadFunctionName(BadParam: i32) bad_custom_type {
	var BadLocalVar = 20
	BadLabel: {
		break BadLabel: 1
	}
	var obj = { .BadField: 1 }
	return 0
}
`
	ast, err := parser.Parse("test.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := lonealyzer.NewAnalyzer("test.lone", lonealyzer.DefaultConfig())
	diags := analyzer.Analyze(ast)

	expectedRules := map[string]bool{
		"naming/global":    false,
		"naming/function":  false,
		"naming/parameter": false,
		"naming/type":      false,
		"naming/variable":  false,
		"naming/label":     false,
		"naming/field":     false,
	}

	for _, d := range diags {
		if _, ok := expectedRules[d.Rule]; ok {
			expectedRules[d.Rule] = true
		}
	}

	for rule, found := range expectedRules {
		if !found {
			t.Errorf("expected diagnostic for rule %q, but none was emitted. Emitted diags: %v", rule, diags)
		}
	}
}

func TestNamingConventionsIntrinsicFunction(t *testing.T) {
	// Intrinsic function using direct AST node
	fn := ir.FN_DEC{
		Name: "#intrinsic_print",
		Body: ir.Block{Stmts: nil},
	}

	analyzer := lonealyzer.NewAnalyzer("test.lone", lonealyzer.DefaultConfig())
	diags := analyzer.Analyze([]any{fn})

	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for intrinsic function, got %d: %v", len(diags), diags)
	}
}
