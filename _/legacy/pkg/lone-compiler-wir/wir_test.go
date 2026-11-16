package lonewir_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	loneanalyzer "lone/pkg/lone-analyzer"
	lonewir "lone/pkg/lone-compiler-wir"
	lonelexer "lone/pkg/lone-lexer"
	loneparser "lone/pkg/lone-parser"
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

func TestStructConstructorLowering(t *testing.T) {
	code := `
type Account = struct {
    id: int,
    _balance: int,

    pub fn __init__(id: int, initial_deposit: int) Account {
        return Account::{
            .id: id,
            ._balance: initial_deposit,
        }
    }

    pub fn get_id(self) int {
        return self.id
    }
}

fn main() void {
    let acc = Account::(101, 500)
    let my_id = acc.get_id()
}
`
	wat := compileSourceToWat(t, code)
	if !strings.Contains(wat, "(func $Account.__init__") {
		t.Errorf("expected $Account.__init__ in wat:\n%s", wat)
	}
	if !strings.Contains(wat, "call $Account.__init__") {
		t.Errorf("expected call $Account.__init__ in wat:\n%s", wat)
	}
}

func TestRefAndPtrLowering(t *testing.T) {
	code := `
fn main() int {
    var x = 10
    var r = &x
    *r = 42

    var p = &raw x as ptr<[]int>
    p[0] = *r + 5
    return *p
}
`
	wat := compileSourceToWat(t, code)
	if !strings.Contains(wat, "i32.store") {
		t.Errorf("expected i32.store for reference/pointer deref write in wat:\n%s", wat)
	}
	if !strings.Contains(wat, "i32.load") {
		t.Errorf("expected i32.load for reference/pointer deref read in wat:\n%s", wat)
	}

	tmpDir := t.TempDir()
	watPath := filepath.Join(tmpDir, "ref_ptr.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0644); err != nil {
		t.Fatalf("failed to write temp wat: %v", err)
	}

	cmd := exec.Command("wasmtime", "--invoke", "main", watPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime failed: %v, output: %s\nwat:\n%s", err, string(out), wat)
	}

	got := strings.TrimSpace(string(out))
	if !strings.HasSuffix(got, "47") {
		t.Fatalf("unexpected wasmtime output:\ngot:  %q\nwant suffix: 47", got)
	}
}

func TestGroupedTuplesAndAnonFnLowering(t *testing.T) {
	code := `
fn add([x, y]: int) int {
    return x + y
}

fn main() int {
    var [a, b]: int = 10
    let sum1 = add(a, b)

    let pair = (5, 15)
    let (u, v) = pair
    let sum2 = u + v

    let p0 = pair.0
    let p1 = pair.1

    let double = fn(n: int) int {
        return n * 2
    }
    let res = double(sum1 + sum2 + p0 + p1)
    return res
}
`
	wat := compileSourceToWat(t, code)

	tmpDir := t.TempDir()
	watPath := filepath.Join(tmpDir, "grouped_tuples.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0644); err != nil {
		t.Fatalf("failed to write temp wat: %v", err)
	}

	cmd := exec.Command("wasmtime", "--invoke", "main", watPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime failed: %v, output: %s\nwat:\n%s", err, string(out), wat)
	}

	got := strings.TrimSpace(string(out))
	if !strings.HasSuffix(got, "120") {
		t.Fatalf("unexpected wasmtime output:\ngot:  %q\nwant suffix: 120", got)
	}
}

func TestInterfaceConstraintMonomorphization(t *testing.T) {
	code := `
type IfcVector = interface {
    x: int,
    y: int,
}

type Point3D = struct {
    x: int,
    y: int,
    z: int,
    _secret: int,

    pub fn __init__(x: int, y: int, z: int) Point3D {
        return Point3D::{ .x: x, .y: y, .z: z, ._secret: 999 }
    }
}

fn dot_product<T where { T satisfies IfcVector }>(p: T) int {
    return p.x * 10 + p.y
}

fn main() int {
    let p = Point3D::(7, 8, 9)
    return dot_product(p)
}
`
	wat := compileSourceToWat(t, code)

	tmpDir := t.TempDir()
	watPath := filepath.Join(tmpDir, "ifc_mono.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0644); err != nil {
		t.Fatalf("failed to write temp wat: %v", err)
	}

	cmd := exec.Command("wasmtime", "--invoke", "main", watPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime failed: %v, output: %s\nwat:\n%s", err, string(out), wat)
	}

	got := strings.TrimSpace(string(out))
	if !strings.HasSuffix(got, "78") {
		t.Fatalf("unexpected wasmtime output:\ngot:  %q\nwant suffix: 78", got)
	}
}

func TestInterpolatedStringPrinting(t *testing.T) {
	code := `
fn main() void {
    let name = "Lone"
    let ver = 6
    let active = true
    println(f"Running ${name} v0.${ver}.0 (active: ${active})")
}
`
	wat := compileSourceToWat(t, code)

	tmpDir := t.TempDir()
	watPath := filepath.Join(tmpDir, "interp.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0644); err != nil {
		t.Fatalf("failed to write temp wat: %v", err)
	}

	cmd := exec.Command("wasmtime", watPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime failed: %v, output: %s\nwat:\n%s", err, string(out), wat)
	}

	got := string(out)
	expected := "Running Lone v0.6.0 (active: true)\n"
	if got != expected {
		t.Fatalf("unexpected wasmtime output:\ngot:  %q\nwant: %q\nwat:\n%s", got, expected, wat)
	}
}

func TestInterpolatedStringValueScope(t *testing.T) {
	code := `
fn main() void {
    let name = "Lone"
    let count = 100
    let msg = f"Built with ${name} for ${count} users!"
    #print(msg)
}
`
	wat := compileSourceToWat(t, code)

	tmpDir := t.TempDir()
	watPath := filepath.Join(tmpDir, "interp_val.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0644); err != nil {
		t.Fatalf("failed to write temp wat: %v", err)
	}

	cmd := exec.Command("wasmtime", watPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime failed: %v, output: %s\nwat:\n%s", err, string(out), wat)
	}

	got := string(out)
	expected := "Built with Lone for 100 users!"
	if got != expected {
		t.Fatalf("unexpected wasmtime output:\ngot:  %q\nwant: %q\nwat:\n%s", got, expected, wat)
	}
}

func TestRefStructFieldMutationExecution(t *testing.T) {
	code := `
type Point = struct {
    x: int,
    y: int,
}

fn move_point(p: ref<Point>) void {
    p.x = p.x + 5
    p.y = p.y + 10
}

fn main() int {
    var pt = Point::{ .x: 1, .y: 2 }
    move_point(&pt)
    return pt.x * 100 + pt.y
}
`
	wat := compileSourceToWat(t, code)

	tmpDir := t.TempDir()
	watPath := filepath.Join(tmpDir, "ref_struct_field.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0644); err != nil {
		t.Fatalf("failed to write temp wat: %v", err)
	}

	cmd := exec.Command("wasmtime", "--invoke", "main", watPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime failed: %v, output: %s\nwat:\n%s", err, string(out), wat)
	}

	got := strings.TrimSpace(string(out))
	if !strings.HasSuffix(got, "612") {
		t.Fatalf("unexpected wasmtime output:\ngot:  %q\nwant suffix: 612", got)
	}
}

func TestPassByValueStructIsolationExecution(t *testing.T) {
	code := `
type Point = struct {
    x: int,
    y: int,
}

fn mutate_alias(val: Point, r: ref<Point>) int {
    r.x = 999
    r.y = 888
    return val.x * 10 + val.y
}

fn main() int {
    var a = Point::{ .x: 1, .y: 2 }
    let res = mutate_alias(a, &a)
    return res * 1000 + a.x
}
`
	wat := compileSourceToWat(t, code)

	tmpDir := t.TempDir()
	watPath := filepath.Join(tmpDir, "struct_pass_val.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0644); err != nil {
		t.Fatalf("failed to write temp wat: %v", err)
	}

	cmd := exec.Command("wasmtime", "--invoke", "main", watPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime failed: %v, output: %s\nwat:\n%s", err, string(out), wat)
	}

	got := strings.TrimSpace(string(out))
	if !strings.HasSuffix(got, "12999") {
		t.Fatalf("unexpected wasmtime output:\ngot:  %q\nwant suffix: 12999", got)
	}
}

func TestRefStructMemcpyAssignmentExecution(t *testing.T) {
	code := `
type Point = struct {
    x: int,
    y: int,
}

fn reset(p: ref<Point>) void {
    *p = Point::{ .x: 42, .y: 84 }
}

fn main() int {
    var pt = Point::{ .x: 1, .y: 2 }
    reset(&pt)
    return pt.x * 100 + pt.y
}
`
	wat := compileSourceToWat(t, code)

	tmpDir := t.TempDir()
	watPath := filepath.Join(tmpDir, "ref_struct_memcpy.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0644); err != nil {
		t.Fatalf("failed to write temp wat: %v", err)
	}

	cmd := exec.Command("wasmtime", "--invoke", "main", watPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime failed: %v, output: %s\nwat:\n%s", err, string(out), wat)
	}

	got := strings.TrimSpace(string(out))
	if !strings.HasSuffix(got, "4284") {
		t.Fatalf("unexpected wasmtime output:\ngot:  %q\nwant suffix: 4284", got)
	}
}

func TestSizedBufferPointerExecutionAndDynamicTrap(t *testing.T) {
	// 1. In-bounds dynamic access
	codeOk := `
fn get_elem(p: ptr<[4]int>, idx: int) int {
    return p[idx]
}

fn main() int {
    var arr: [4]int = [10, 20, 30, 40]
    let p: ptr<[4]int> = &raw arr
    return get_elem(p, 2)
}
`
	watOk := compileSourceToWat(t, codeOk)

	tmpDir := t.TempDir()
	watPathOk := filepath.Join(tmpDir, "sized_ptr_ok.wat")
	if err := os.WriteFile(watPathOk, []byte(watOk), 0644); err != nil {
		t.Fatalf("failed to write temp wat: %v", err)
	}

	cmdOk := exec.Command("wasmtime", "--invoke", "main", watPathOk)
	outOk, err := cmdOk.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime failed: %v, output: %s\nwat:\n%s", err, string(outOk), watOk)
	}
	gotOk := strings.TrimSpace(string(outOk))
	if !strings.HasSuffix(gotOk, "30") {
		t.Fatalf("unexpected wasmtime output for in-bounds access:\ngot: %q\nwant suffix: 30", gotOk)
	}

	// 2. Out-of-bounds dynamic access must trap (unreachable)
	codeTrap := `
fn get_elem(p: ptr<[4]int>, idx: int) int {
    return p[idx]
}

fn main() int {
    var arr: [4]int = [10, 20, 30, 40]
    let p: ptr<[4]int> = &raw arr
    return get_elem(p, 4) -- OOB: size is 4, valid is [0..3]
}
`
	watTrap := compileSourceToWat(t, codeTrap)
	watPathTrap := filepath.Join(tmpDir, "sized_ptr_trap.wat")
	if err := os.WriteFile(watPathTrap, []byte(watTrap), 0644); err != nil {
		t.Fatalf("failed to write temp wat: %v", err)
	}

	cmdTrap := exec.Command("wasmtime", "--invoke", "main", watPathTrap)
	outTrap, err := cmdTrap.CombinedOutput()
	if err == nil {
		t.Fatalf("expected wasmtime to trap on OOB index access, but succeeded with output: %s", string(outTrap))
	}
	if !strings.Contains(string(outTrap), "unreachable") && !strings.Contains(string(outTrap), "trap") {
		t.Logf("wasmtime output on trap: %s", string(outTrap))
	}
}

func TestUnsizedBufferPointerExecution(t *testing.T) {
	code := `
fn main() int {
    let mem = #alloc(64) as ptr<[]int>
    mem[0] = 111
    mem[1] = 222
    mem[5] = 555
    return mem[0] + mem[1] + mem[5]
}
`
	wat := compileSourceToWat(t, code)

	tmpDir := t.TempDir()
	watPath := filepath.Join(tmpDir, "unsized_ptr.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0644); err != nil {
		t.Fatalf("failed to write temp wat: %v", err)
	}

	cmd := exec.Command("wasmtime", "--invoke", "main", watPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime failed: %v, output: %s\nwat:\n%s", err, string(out), wat)
	}

	got := strings.TrimSpace(string(out))
	if !strings.HasSuffix(got, "888") {
		t.Fatalf("unexpected wasmtime output:\ngot: %q\nwant suffix: 888", got)
	}
}

func TestSizedPointerDecayExecution(t *testing.T) {
	code := `
fn sum3(buf: ptr<[]int>) int {
    return buf[0] + buf[1] + buf[2]
}

fn main() int {
    var arr: [3]int = [100, 200, 300]
    let p: ptr<[3]int> = &raw arr
    return sum3(p)
}
`
	wat := compileSourceToWat(t, code)

	tmpDir := t.TempDir()
	watPath := filepath.Join(tmpDir, "sized_ptr_decay.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0644); err != nil {
		t.Fatalf("failed to write temp wat: %v", err)
	}

	cmd := exec.Command("wasmtime", "--invoke", "main", watPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime failed: %v, output: %s\nwat:\n%s", err, string(out), wat)
	}

	got := strings.TrimSpace(string(out))
	if !strings.HasSuffix(got, "600") {
		t.Fatalf("unexpected wasmtime output:\ngot: %q\nwant suffix: 600", got)
	}
}



