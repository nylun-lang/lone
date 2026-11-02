package loneproject_test

import (
	"strings"
	"testing"

	loneanalyzer "lone/pkg2/lone-analyzer"
	lonecompiler "lone/pkg2/lone-compiler"
	loneproject "lone/pkg2/lone-analyzer-project"
)

func TestProjectTwoFilesOrdering(t *testing.T) {
	proj := loneproject.NewProject(".")

	mathCode := `
fn add(a: i32, b: i32) i32 {
    return a + b
}
`
	mainCode := `
fn main() void {
    drop add(10, 20)
}
`
	// Add main.lone first, then math.lone
	if _, err := proj.AddFile("main.lone", []byte(mainCode)); err != nil {
		t.Fatalf("failed to add main.lone: %v", err)
	}
	if _, err := proj.AddFile("math.lone", []byte(mathCode)); err != nil {
		t.Fatalf("failed to add math.lone: %v", err)
	}

	merged, diags, err := proj.AnalyzeAndOrder()
	if err != nil {
		t.Fatalf("expected order to succeed, got %v (diags: %v)", err, diags)
	}

	if len(merged.OrderedFiles) != 2 {
		t.Fatalf("expected 2 ordered files, got %d", len(merged.OrderedFiles))
	}

	// math.lone MUST precede main.lone because main depends on math
	if merged.OrderedFiles[0].Path != "math.lone" {
		t.Errorf("expected math.lone first, got %s", merged.OrderedFiles[0].Path)
	}
	if merged.OrderedFiles[1].Path != "main.lone" {
		t.Errorf("expected main.lone second, got %s", merged.OrderedFiles[1].Path)
	}

	// Now verify the merged AST passes semantic analysis!
	analyzer := loneanalyzer.NewAnalyzer("merged.lone", loneanalyzer.DefaultConfig())
	typedProg, analyzerDiags := analyzer.AnalyzeTyped(merged.MergedAst)

	hasErrors := false
	for _, d := range analyzerDiags {
		if d.Severity == loneanalyzer.SeverityError {
			hasErrors = true
			t.Errorf("unexpected analyzer error on merged project: %s", d.Message)
		}
	}
	if hasErrors {
		t.Fatalf("merged project failed semantic analysis")
	}

	if len(typedProg.Decls) != 2 {
		t.Errorf("expected 2 declarations in typed program, got %d", len(typedProg.Decls))
	}
}

func TestProjectCircularDependency(t *testing.T) {
	proj := loneproject.NewProject(".")

	codeA := `
fn func_a() void {
    func_b()
}
`
	codeB := `
fn func_b() void {
    func_a()
}
`
	if _, err := proj.AddFile("a.lone", []byte(codeA)); err != nil {
		t.Fatalf("failed to add a.lone: %v", err)
	}
	if _, err := proj.AddFile("b.lone", []byte(codeB)); err != nil {
		t.Fatalf("failed to add b.lone: %v", err)
	}

	_, _, err := proj.AnalyzeAndOrder()
	if err == nil {
		t.Fatalf("expected error on circular dependency, got nil")
	}

	if !strings.Contains(err.Error(), "circular dependency detected") {
		t.Errorf("expected circular dependency error, got %v", err)
	}
}

func TestProjectDuplicateSymbol(t *testing.T) {
	proj := loneproject.NewProject(".")

	code1 := `
fn compute() i32 {
    return 1
}
`
	code2 := `
fn compute() i32 {
    return 2
}
`
	if _, err := proj.AddFile("mod1.lone", []byte(code1)); err != nil {
		t.Fatalf("failed to add mod1.lone: %v", err)
	}
	if _, err := proj.AddFile("mod2.lone", []byte(code2)); err != nil {
		t.Fatalf("failed to add mod2.lone: %v", err)
	}

	_, diags, err := proj.AnalyzeAndOrder()
	if err == nil {
		t.Fatalf("expected error on duplicate symbol, got nil")
	}

	if len(diags) == 0 {
		t.Fatalf("expected duplicate symbol diagnostic, got 0")
	}

	if !strings.Contains(diags[0].Message, "duplicate declaration of symbol \"compute\"") {
		t.Errorf("expected duplicate symbol message, got %s", diags[0].Message)
	}
}

func TestProjectEndToEndCompile(t *testing.T) {
	proj := loneproject.NewProject(".")

	libCode := `
fn greet(msg: str) void {
    #print(msg)
}
`
	appCode := `
fn main() void {
    greet("Hello Multi-File Lone!\n")
}
`
	if _, err := proj.AddFile("app/main.lone", []byte(appCode)); err != nil {
		t.Fatalf("failed to add app: %v", err)
	}
	if _, err := proj.AddFile("lib/greet.lone", []byte(libCode)); err != nil {
		t.Fatalf("failed to add lib: %v", err)
	}

	merged, _, err := proj.AnalyzeAndOrder()
	if err != nil {
		t.Fatalf("failed to order: %v", err)
	}

	// Verify lib/greet.lone is first, app/main.lone is second
	if merged.OrderedFiles[0].Path != "lib/greet.lone" {
		t.Errorf("expected lib/greet.lone first, got %s", merged.OrderedFiles[0].Path)
	}

	comp := lonecompiler.NewCompiler(lonecompiler.Config{
		Target: lonecompiler.TargetWasi,
	})

	mirMod, err := comp.CompileSource("merged.lone", []byte(appCode+"\n"+libCode))
	if err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	if !strings.Contains(mirMod.WatOutput, "(func $main") {
		t.Errorf("expected main func in output")
	}
}
