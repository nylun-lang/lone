package loneanalyzer

import (
	"strings"
	"testing"

	ir "lone/pkg/lone-parser-ast"
	"lone/pkg/lone-parser"
)

func TestScopeStack(t *testing.T) {
	stack := NewScopeStack("main.lone")
	if stack.Current().Kind != ScopeFile {
		t.Fatalf("expected root scope to be file, got %v", stack.Current().Kind)
	}

	sym1 := &Symbol{Name: "GLOBAL_VAL", Kind: SymbolVar}
	if err := stack.Define(sym1); err != nil {
		t.Fatalf("failed to define global: %v", err)
	}

	// Duplicate error
	if err := stack.Define(sym1); err == nil {
		t.Fatalf("expected error on duplicate definition")
	}

	// Push function
	stack.Push(ScopeFunction, "main")
	if stack.Current().Kind != ScopeFunction {
		t.Fatalf("expected function scope, got %v", stack.Current().Kind)
	}
	if !stack.HasEnclosing(ScopeFile) {
		t.Fatalf("expected enclosing file scope")
	}

	// Define local
	sym2 := &Symbol{Name: "local_val", Kind: SymbolVar}
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
	if stack.Current().Kind != ScopeFile {
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
	ast, err := loneparser.Parse("test.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := NewAnalyzer("test.lone", DefaultConfig())
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
	ast, err := loneparser.Parse("test.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := NewAnalyzer("test.lone", DefaultConfig())
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

	analyzer := NewAnalyzer("test.lone", DefaultConfig())
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
	ast, err := loneparser.Parse("infer.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := NewAnalyzer("infer.lone", DefaultConfig())
	prog, diags := analyzer.AnalyzeTyped(ast)

	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got: %v", diags)
	}

	if prog == nil || len(prog.Decls) != 4 {
		t.Fatalf("expected 4 declarations in typed IR, got: %+v", prog)
	}

	// 1. var num = 42 -> TypeI32
	varNum, ok := prog.Decls[0].(*TVarDec)
	if !ok || !varNum.Type.Equals(TypeI32) {
		t.Errorf("expected varNum to be resolved to i32, got: %+v", varNum)
	}

	// 2. var text = "hello" -> TypeStr
	varText, ok := prog.Decls[1].(*TVarDec)
	if !ok || !varText.Type.Equals(TypeStr) {
		t.Errorf("expected varText to be resolved to str, got: %+v", varText)
	}

	// 3. var flag = true -> TypeBool
	varFlag, ok := prog.Decls[2].(*TVarDec)
	if !ok || !varFlag.Type.Equals(TypeBool) {
		t.Errorf("expected varFlag to be resolved to bool, got: %+v", varFlag)
	}

	// 4. fn compute() i32 -> local_infer is i32
	fnCompute, ok := prog.Decls[3].(*TFnDec)
	if !ok || !fnCompute.ReturnType.Equals(TypeI32) {
		t.Errorf("expected fnCompute return type to be i32, got: %+v", fnCompute)
	}
	localVar, ok := fnCompute.Body.Stmts[0].(*TVarDec)
	if !ok || !localVar.Type.Equals(TypeI32) {
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
	astMismatch, err := loneparser.Parse("mismatch.lone", []byte(codeMismatch))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := NewAnalyzer("mismatch.lone", DefaultConfig())
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
	astMissing, err := loneparser.Parse("missing.lone", []byte(codeMissing))
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
	astUnused, err := loneparser.Parse("unused.lone", []byte(codeUnused))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := NewAnalyzer("unused.lone", DefaultConfig())
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
	astDropped, err := loneparser.Parse("dropped.lone", []byte(codeDropped))
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
	astConsumed, err := loneparser.Parse("consumed.lone", []byte(codeConsumed))
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
	astVoid, err := loneparser.Parse("void.lone", []byte(codeVoid))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	diagsVoid := analyzer.Analyze(astVoid)
	if len(diagsVoid) != 0 {
		t.Fatalf("expected 0 diagnostics when calling void fn, got: %v", diagsVoid)
	}
}

func TestPrintIntrinsicAnalysis(t *testing.T) {
	code := `
fn main() void {
	#print("Hello, world!\n")
}
`
	ast, err := loneparser.Parse("print.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := NewAnalyzer("print.lone", DefaultConfig())
	prog, diags := analyzer.AnalyzeTyped(ast)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for #print call, got: %v", diags)
	}

	if len(prog.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(prog.Decls))
	}
	fnMain, ok := prog.Decls[0].(*TFnDec)
	if !ok {
		t.Fatalf("expected TFnDec, got %T", prog.Decls[0])
	}
	if len(fnMain.Body.Stmts) != 1 {
		t.Fatalf("expected 1 stmt, got %d", len(fnMain.Body.Stmts))
	}
	exprStmt, ok := fnMain.Body.Stmts[0].(*TExprStmt)
	if !ok {
		t.Fatalf("expected TExprStmt, got %T", fnMain.Body.Stmts[0])
	}
	callExpr, ok := exprStmt.Expr.(*TCallExpr)
	if !ok {
		t.Fatalf("expected TCallExpr, got %T", exprStmt.Expr)
	}
	ref, ok := callExpr.Callee.(*TVarRefExpr)
	if !ok || ref.Name != "#print" {
		t.Fatalf("expected callee '#print', got %+v", callExpr.Callee)
	}
}

func TestTraitNamingConventions(t *testing.T) {
	// 1. Valid trait with Trait suffix and legitimate English words (portrait, strait)
	validCode := `
type ComparableTrait = trait {
	fn compare(self, other: i32) i32
}

type Canvas = struct {
	portrait: i32,
	bering_strait: i32,
}

fn draw_portrait(strait_width: i32) void {
	var local_portrait = 10
	var obj = { .portrait: 42 }
}
`
	astValid, err := loneparser.Parse("valid_trait.lone", []byte(validCode))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	diagsValid := NewAnalyzer("valid_trait.lone", DefaultConfig()).Analyze(astValid)
	for _, d := range diagsValid {
		if d.Severity == SeverityWarning || d.Severity == SeverityError {
			t.Fatalf("unexpected diagnostic in valid_trait.lone: %v", d)
		}
	}

	// 2. Trait missing "Trait" suffix
	badTraitCode := `
type Comparable = trait {
	fn compare(self, other: i32) i32
}
`
	astBadTrait, err := loneparser.Parse("bad_trait.lone", []byte(badTraitCode))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	diagsBadTrait := NewAnalyzer("bad_trait.lone", DefaultConfig()).Analyze(astBadTrait)
	if !hasDiagRule(diagsBadTrait, "naming/trait") {
		t.Fatalf("expected naming/trait warning for trait not ending with Trait, got: %v", diagsBadTrait)
	}

	// 3. Non-trait types (struct, alias) ending in Trait or named Trait
	badNonTraitTypes := `
type PointTrait = struct {
	x: i32,
}

type Trait = struct {
	y: i32,
}

type NumTrait = i32
`
	astBadTypes, err := loneparser.Parse("bad_types.lone", []byte(badNonTraitTypes))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	diagsBadTypes := NewAnalyzer("bad_types.lone", DefaultConfig()).Analyze(astBadTypes)
	var typeWarnCount int
	for _, d := range diagsBadTypes {
		if d.Rule == "naming/type" {
			typeWarnCount++
		}
	}
	if typeWarnCount < 3 {
		t.Fatalf("expected at least 3 naming/type warnings for non-trait types ending in Trait, got %d: %v", typeWarnCount, diagsBadTypes)
	}

	// 4. Non-trait function, variables, fields, params ending in trait
	badOtherDecls := `
var global_trait = 1
var TRAIT = 2

fn run_trait(param_trait: i32) void {
	var local_trait = 3
	lbl_trait: {
		break lbl_trait: 0
	}
	var o = { .bad_trait: 4 }
}
`
	astBadOther, err := loneparser.Parse("bad_other.lone", []byte(badOtherDecls))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	diagsBadOther := NewAnalyzer("bad_other.lone", DefaultConfig()).Analyze(astBadOther)

	expectedRules := map[string]bool{
		"naming/global":    false,
		"naming/function":  false,
		"naming/parameter": false,
		"naming/variable":  false,
		"naming/label":     false,
		"naming/field":     false,
	}
	for _, d := range diagsBadOther {
		if _, ok := expectedRules[d.Rule]; ok {
			expectedRules[d.Rule] = true
		}
	}
	for rule, found := range expectedRules {
		if !found {
			t.Errorf("expected diagnostic for rule %q in badOtherDecls, got: %v", rule, diagsBadOther)
		}
	}
}

func TestComparisonChainingAnalyzerRejection(t *testing.T) {
	code := `
fn check(a: i32, b: i32, c: i32) void {
	var x = (a < b) < c
}
`
	ast, err := loneparser.Parse("chained_analyzer.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed unexpectedly: %v", err)
	}
	diags := NewAnalyzer("chained_analyzer.lone", DefaultConfig()).Analyze(ast)
	found := false
	for _, d := range diags {
		if d.Rule == "type/invalid-comparison" && strings.Contains(d.Message, "cannot compare bool with comparison operator") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected diagnostic 'type/invalid-comparison' for parenthesized comparison chaining, got: %v", diags)
	}
}
