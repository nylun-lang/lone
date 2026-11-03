package lonewir_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	loneanalyzer "lone/pkg2/lone-analyzer"
	lonewir "lone/pkg2/lone-compiler-wir"
	lonelexer "lone/pkg2/lone-lexer"
	loneparser "lone/pkg2/lone-parser"
)

func compileSourceToWat(t *testing.T, source string) string {
	t.Helper()
	tokens, err := lonelexer.NewLexer([]byte(source)).Tokenize()
	if err != nil {
		t.Fatalf("lexing failed: %v", err)
	}
	p := loneparser.NewParser(tokens)
	ast, err := p.ParseProgram()
	if err != nil {
		t.Fatalf("parsing failed: %v", err)
	}
	analyzer := loneanalyzer.NewTypeEvaluator()
	typedProg := analyzer.EvaluateProgram(ast)
	if len(analyzer.Diagnostics()) > 0 {
		var msgs []string
		for _, d := range analyzer.Diagnostics() {
			msgs = append(msgs, d.String())
		}
		t.Fatalf("analyzer errors:\n%s", strings.Join(msgs, "\n"))
	}
	mod, err := lonewir.LowerProgram(typedProg, lonewir.DefaultLowerOptions())
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	return mod.FormatWat()
}

func TestBasicWatGeneration(t *testing.T) {
	code := `
fn add(a: int, b: int) int {
	return a + b
}
`
	wat := compileSourceToWat(t, code)
	if !strings.Contains(wat, "(func $add (param $p_a i32) (param $p_b i32) (result i32)") {
		t.Errorf("expected $add signature, got:\n%s", wat)
	}
	if !strings.Contains(wat, "i32.add") {
		t.Errorf("expected i32.add, got:\n%s", wat)
	}
}

func TestWhileStructuredLowering(t *testing.T) {
	code := `
fn test_while() void {
	var count = 0
	while count < 10 {
		count = count + 1
	}
}
`
	wat := compileSourceToWat(t, code)
	if !strings.Contains(wat, "block $while_break_") {
		t.Errorf("expected outer structured block $while_break_, got:\n%s", wat)
	}
	if !strings.Contains(wat, "loop $while_loop_") {
		t.Errorf("expected inner structured loop $while_loop_, got:\n%s", wat)
	}
	if !strings.Contains(wat, "br_if $while_break_") {
		t.Errorf("expected conditional break, got:\n%s", wat)
	}
}

func TestLoopWithPostConditionLowering(t *testing.T) {
	code := `
fn test_post_cond() void {
	var x = 10
	loop {
		x = x - 1
	} while: [x > 0]
}
`
	wat := compileSourceToWat(t, code)
	if !strings.Contains(wat, "block $loop_break_") {
		t.Errorf("expected outer structured block $loop_break_, got:\n%s", wat)
	}
	if !strings.Contains(wat, "loop $loop_cont_") {
		t.Errorf("expected inner structured loop $loop_cont_, got:\n%s", wat)
	}
	if !strings.Contains(wat, "br_if $loop_cont_") {
		t.Errorf("expected post-condition branch back to loop, got:\n%s", wat)
	}
}

func TestScopedHeaderIfLowering(t *testing.T) {
	code := `
fn test_scoped_header() void {
	if: [let x = 0] x != 9 {
		var y = x + 1
	}
}
`
	wat := compileSourceToWat(t, code)
	if !strings.Contains(wat, "if") {
		t.Errorf("expected structured if, got:\n%s", wat)
	}
	if !strings.Contains(wat, "i32.const 0") {
		t.Errorf("expected init of scoped header x, got:\n%s", wat)
	}
}

func TestExplicitAsCastLowering(t *testing.T) {
	code := `
fn test_casts(x: int) float {
	var f = x as float
	var back = f as int
	var d = x as f64
	var back_d = d as int
	return f
}
`
	wat := compileSourceToWat(t, code)
	if !strings.Contains(wat, "f32.convert_i32_s") {
		t.Errorf("expected f32.convert_i32_s for x as float, got:\n%s", wat)
	}
	if !strings.Contains(wat, "i32.trunc_f32_s") {
		t.Errorf("expected i32.trunc_f32_s for f as int, got:\n%s", wat)
	}
	if !strings.Contains(wat, "f64.convert_i32_s") {
		t.Errorf("expected f64.convert_i32_s for x as f64, got:\n%s", wat)
	}
	if !strings.Contains(wat, "i32.trunc_f64_s") {
		t.Errorf("expected i32.trunc_f64_s for d as int, got:\n%s", wat)
	}
}

func TestPrintIntrinsicAndDataSegment(t *testing.T) {
	code := `
fn main() void {
	#print("Hello from Greenfield!\n")
}
`
	wat := compileSourceToWat(t, code)
	if !strings.Contains(wat, "(import \"wasi_snapshot_preview1\" \"fd_write\"") {
		t.Errorf("expected fd_write import, got:\n%s", wat)
	}
	if !strings.Contains(wat, "(func $__lone_rt_print") {
		t.Errorf("expected inlined print helper, got:\n%s", wat)
	}
	if !strings.Contains(wat, `(data (i32.const 64) "Hello from Greenfield!\0a\00")`) {
		t.Errorf("expected data segment at offset >= 64, got:\n%s", wat)
	}
	if !strings.Contains(wat, "(export \"_start\")") {
		t.Errorf("expected _start export for main, got:\n%s", wat)
	}
}

func TestLocalAllocatorPooling(t *testing.T) {
	alloc := lonewir.NewLocalAllocator()
	alloc.EnterScope()
	l1 := alloc.AllocVar("a", lonewir.TypeI32)
	alloc.ExitScope()

	// Sibling scope should reuse the exact same local
	alloc.EnterScope()
	l2 := alloc.AllocVar("b", lonewir.TypeI32)
	alloc.ExitScope()

	if l1 != l2 {
		t.Errorf("expected local slot reuse between sibling scopes: got %s and %s", l1, l2)
	}
	if len(alloc.Locals()) != 1 {
		t.Errorf("expected exactly 1 local allocated, got %d", len(alloc.Locals()))
	}
}

func TestWasiExecutionWithWasmtime(t *testing.T) {
	code := `
fn main() void {
	#print("Hello from Greenfield Wasm!\n")
}
`
	wat := compileSourceToWat(t, code)

	tmpDir := t.TempDir()
	watPath := filepath.Join(tmpDir, "test.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0644); err != nil {
		t.Fatalf("failed to write wat: %v", err)
	}

	cmd := exec.Command("wasmtime", watPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime failed: %v, output: %s\nwat:\n%s", err, string(out), wat)
	}

	want := "Hello from Greenfield Wasm!\n"
	if string(out) != want {
		t.Fatalf("unexpected wasmtime output:\ngot:  %q\nwant: %q", string(out), want)
	}
}
