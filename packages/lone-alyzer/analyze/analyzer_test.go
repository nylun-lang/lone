package analyze_test

import (
	"testing"

	"lonealyzer/analyze"
	air "lonealyzer/irtypes"
	ir "lonelang/irtypes"
	"lonelang/parser"
)

func TestScopeStack(t *testing.T) {
	stack := analyze.NewScopeStack("main.lone")
	if stack.Current().Kind != analyze.ScopeFile {
		t.Fatalf("expected root scope to be file, got %v", stack.Current().Kind)
	}

	sym1 := &analyze.Symbol{Name: "GLOBAL_VAL", Kind: analyze.SymbolVar}
	if err := stack.Define(sym1); err != nil {
		t.Fatalf("failed to define global: %v", err)
	}

	// Duplicate error
	if err := stack.Define(sym1); err == nil {
		t.Fatalf("expected error on duplicate definition")
	}

	// Push function
	stack.Push(analyze.ScopeFunction, "main")
	if stack.Current().Kind != analyze.ScopeFunction {
		t.Fatalf("expected function scope, got %v", stack.Current().Kind)
	}
	if !stack.HasEnclosing(analyze.ScopeFile) {
		t.Fatalf("expected enclosing file scope")
	}

	// Define local
	sym2 := &analyze.Symbol{Name: "local_val", Kind: analyze.SymbolVar}
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
	if stack.Current().Kind != analyze.ScopeFile {
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

	analyzer := analyze.NewAnalyzer("test.lone", analyze.DefaultConfig())
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

	analyzer := analyze.NewAnalyzer("test.lone", analyze.DefaultConfig())
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

	analyzer := analyze.NewAnalyzer("test.lone", analyze.DefaultConfig())
	diags := analyzer.Analyze([]any{fn})

	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for intrinsic function, got %d: %v", len(diags), diags)
	}
}

func TestTypeInferenceToConcreteIR(t *testing.T) {
	code := `
var num = 42
var text = "hello"
var flag = true

fn compute() i32 {
	var local_infer = 100
	return local_infer
}
`
	ast, err := parser.Parse("infer.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := analyze.NewAnalyzer("infer.lone", analyze.DefaultConfig())
	prog, diags := analyzer.AnalyzeTyped(ast)

	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got: %v", diags)
	}

	if prog == nil || len(prog.Decls) != 4 {
		t.Fatalf("expected 4 declarations in typed IR, got: %+v", prog)
	}

	// 1. var num = 42 -> TypeI32
	varNum, ok := prog.Decls[0].(*air.TVarDec)
	if !ok || !varNum.Type.Equals(air.TypeI32) {
		t.Errorf("expected varNum to be resolved to i32, got: %+v", varNum)
	}

	// 2. var text = "hello" -> TypeStr
	varText, ok := prog.Decls[1].(*air.TVarDec)
	if !ok || !varText.Type.Equals(air.TypeStr) {
		t.Errorf("expected varText to be resolved to str, got: %+v", varText)
	}

	// 3. var flag = true -> TypeBool
	varFlag, ok := prog.Decls[2].(*air.TVarDec)
	if !ok || !varFlag.Type.Equals(air.TypeBool) {
		t.Errorf("expected varFlag to be resolved to bool, got: %+v", varFlag)
	}

	// 4. fn compute() i32 -> local_infer is i32
	fnCompute, ok := prog.Decls[3].(*air.TFnDec)
	if !ok || !fnCompute.ReturnType.Equals(air.TypeI32) {
		t.Errorf("expected fnCompute return type to be i32, got: %+v", fnCompute)
	}
	localVar, ok := fnCompute.Body.Stmts[0].(*air.TVarDec)
	if !ok || !localVar.Type.Equals(air.TypeI32) {
		t.Errorf("expected local_infer to be resolved to i32, got: %+v", localVar)
	}
}

func TestFunctionReturnCheck(t *testing.T) {
	// Function returning wrong type
	codeMismatch := `
fn get_num() i32 {
	return "not a number"
}
`
	astMismatch, err := parser.Parse("mismatch.lone", []byte(codeMismatch))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := analyze.NewAnalyzer("mismatch.lone", analyze.DefaultConfig())
	diags := analyzer.Analyze(astMismatch)

	hasMismatch := false
	for _, d := range diags {
		if d.Rule == "type/return-mismatch" {
			hasMismatch = true
		}
	}
	if !hasMismatch {
		t.Fatalf("expected type/return-mismatch diagnostic, got: %v", diags)
	}

	// Function missing return
	codeMissing := `
fn need_num() i32 {
	var x = 10
}
`
	astMissing, err := parser.Parse("missing.lone", []byte(codeMissing))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	diagsMissing := analyzer.Analyze(astMissing)
	hasMissing := false
	for _, d := range diagsMissing {
		if d.Rule == "type/missing-return" {
			hasMissing = true
		}
	}
	if !hasMissing {
		t.Fatalf("expected type/missing-return diagnostic, got: %v", diagsMissing)
	}
}

func TestConsumeOrDropRule(t *testing.T) {
	// 1. Unused return value from function without drop -> ERROR
	codeUnused := `
fn compute_val() i32 {
	return 42
}

fn test_main() void {
	compute_val()
}
`
	astUnused, err := parser.Parse("unused.lone", []byte(codeUnused))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := analyze.NewAnalyzer("unused.lone", analyze.DefaultConfig())
	diagsUnused := analyzer.Analyze(astUnused)

	hasUnusedDiag := false
	for _, d := range diagsUnused {
		if d.Rule == "type/unused-result" {
			hasUnusedDiag = true
		}
	}
	if !hasUnusedDiag {
		t.Fatalf("expected type/unused-result diagnostic for bare call, got: %v", diagsUnused)
	}

	// 2. Explicitly dropped with 'drop' keyword -> SUCCESS (0 diagnostics)
	codeDropped := `
fn compute_val() i32 {
	return 42
}

fn test_main() void {
	drop compute_val()
}
`
	astDropped, err := parser.Parse("dropped.lone", []byte(codeDropped))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	diagsDropped := analyzer.Analyze(astDropped)
	if len(diagsDropped) != 0 {
		t.Fatalf("expected 0 diagnostics when drop is used, got: %v", diagsDropped)
	}

	// 3. Consumed by variable binding -> SUCCESS (0 diagnostics)
	codeConsumed := `
fn compute_val() i32 {
	return 42
}

fn test_main() void {
	var result = compute_val()
}
`
	astConsumed, err := parser.Parse("consumed.lone", []byte(codeConsumed))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	diagsConsumed := analyzer.Analyze(astConsumed)
	if len(diagsConsumed) != 0 {
		t.Fatalf("expected 0 diagnostics when consumed by var, got: %v", diagsConsumed)
	}

	// 4. Calling void function directly -> SUCCESS (void does not require drop)
	codeVoid := `
fn do_logging() void {
}

fn test_main() void {
	do_logging()
}
`
	astVoid, err := parser.Parse("void.lone", []byte(codeVoid))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	diagsVoid := analyzer.Analyze(astVoid)
	if len(diagsVoid) != 0 {
		t.Fatalf("expected 0 diagnostics when calling void fn, got: %v", diagsVoid)
	}
}
