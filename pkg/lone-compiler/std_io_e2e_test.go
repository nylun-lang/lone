package lonecompiler_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	lonecompiler "lone/pkg/lone-compiler"
)

func runWatWithWasmtime(t *testing.T, wat string, stdin string) string {
	t.Helper()
	wasmtimePath, err := exec.LookPath("wasmtime")
	if err != nil {
		t.Skip("wasmtime not installed in PATH; skipping live WASI execution check")
	}

	tmpDir := t.TempDir()
	watPath := filepath.Join(tmpDir, "prog.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0644); err != nil {
		t.Fatalf("failed to write temp wat: %v", err)
	}

	cmd := exec.Command(wasmtimePath, watPath)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("wasmtime execution failed: %v\nstderr:\n%s\nWAT:\n%s", err, stderr.String(), wat)
	}
	return stdout.String()
}

func TestPureLoneStdIoPrintAndReadLineWasmtime(t *testing.T) {
	code := `
import .print, .println, .read_line from :std:io

fn main() void {
	print("Enter name: ")
	var name = read_line()
	print("Welcome, ")
	print(name)
	println("!")

	var second = read_line()
	print("Line2=")
	println(second)
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileSource("echo_app.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to compile pure-Lone std:io program: %v (diags: %+v)", err, res)
	}

	// Ensure zero reliance on legacy inlined #print WAT helper
	if strings.Contains(res.WatOutput, "__lone_rt_print") {
		t.Errorf("expected pure-Lone std:io compilation without __lone_rt_print, got:\n%s", res.WatOutput)
	}
	if !strings.Contains(res.WatOutput, "(import \"wasi_snapshot_preview1\" \"fd_write\"") {
		t.Errorf("expected fd_write host import from :std:sys")
	}
	if !strings.Contains(res.WatOutput, "(import \"wasi_snapshot_preview1\" \"fd_read\"") {
		t.Errorf("expected fd_read host import from :std:sys")
	}

	got := runWatWithWasmtime(t, res.WatOutput, "Lone Developer\r\nPure WASI\n")
	want := "Enter name: Welcome, Lone Developer!\nLine2=Pure WASI\n"
	if got != want {
		t.Errorf("unexpected stdout from wasmtime:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStructSretDatastackRewindAndTraitMatWasmtime(t *testing.T) {
	code := `
import .Stdout, .WriterTrait from :std:io
import .print from :std
import :std:io as io

type Vec2 = struct {
	x: i32,
	y: i32,

	pub fn new(x: i32, y: i32) Vec2 {
		return Vec2::{ .x: x, .y: y }
	}

	pub fn dot(self, other: Vec2) i32 {
		return self.x * other.x + self.y * other.y
	}
}

fn bump_via_raw_ptr(p: ptr<i32>, delta: i32) void {
	*p = *p + delta
}

fn frame_step(i: i32) i32 {
	var v = Vec2.new(i, 1)
	var acc: i32 = v.y
	bump_via_raw_ptr(&raw acc, 1)
	return acc
}

fn main() void {
	-- 1. Stress datastack frame prologue/epilogue across 5000 calls (would overflow 128 KiB if sp leaked)
	var iter: i32 = 0
	var sum: i32 = 0
	while iter < 5000 {
		sum = sum + frame_step(iter)
		iter = iter + 1
	}
	if sum == 10000 {
		print("stack:ok ")
	}

	-- 2. Struct SRET return-by-value & instance method
	var a = Vec2.new(3, 4)
	var b = Vec2.new(5, 6)
	if a.dot(b) == 39 {
		print("vec:ok ")
	}

	-- 3. Materialized zero-sized trait call + aliased module call
	var out = Stdout.new()
	let w = WriterTrait::|Stdout|
	drop w.write(out, "trait:ok ")
	io.println("alias:ok")
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileSource("mem_trait_e2e.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to compile mem_trait_e2e.lone: %v (diags: %+v)", err, res)
	}

	got := runWatWithWasmtime(t, res.WatOutput, "")
	want := "stack:ok vec:ok trait:ok alias:ok\n"
	if got != want {
		t.Errorf("unexpected stdout from wasmtime:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestGuessingGameExampleE2EWasmtime(t *testing.T) {
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileFile("../../exemple/guess.lone")
	if err != nil {
		t.Fatalf("failed to compile exemple/guess.lone: %v (diags: %+v)", err, res)
	}

	output := runWatWithWasmtime(t, res.WatOutput, "0\n")
	if !strings.Contains(output, "Welcome to the Lone Guessing Game!") {
		t.Errorf("expected welcome banner in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Giving up? The secret number was: ") {
		t.Errorf("expected giving up message in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Better luck next time!") {
		t.Errorf("expected better luck message, got:\n%s", output)
	}
}

func TestParseIntAndIntToStrE2EWasmtime(t *testing.T) {
	code := `
import .print, .println, .println_int, .int_to_str, .parse_int from :std:io

fn main() void {
	var a = parse_int("12345")
	var b = parse_int("  -678")
	var sum = a + b
	print("sum=")
	println_int(sum)

	var s = int_to_str(sum)
	print("str=")
	println(s)
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileSource("parse_test.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	output := runWatWithWasmtime(t, res.WatOutput, "")
	want := "sum=11667\nstr=11667\n"
	if output != want {
		t.Errorf("unexpected output from parse_int/int_to_str test:\ngot:  %q\nwant: %q", output, want)
	}
}

func TestPackedIfWasmtime(t *testing.T) {
	code := `
import .println from :std:io

fn classify(x: i32) void {
	if {
		x > 100 => println("Large")
		x > 50  => println("Medium")
		x > 0   => println("Small")
		else    => {
			println("Invalid")
		}
	}
}

fn main() void {
	classify(150)
	classify(75)
	classify(10)
	classify(-5)
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileSource("classify.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	output := runWatWithWasmtime(t, res.WatOutput, "")
	want := "Large\nMedium\nSmall\nInvalid\n"
	if output != want {
		t.Errorf("unexpected output from packed if test:\ngot:  %q\nwant: %q", output, want)
	}
}

func TestLoopWasmtime(t *testing.T) {
	code := `
import .println_int from :std:io

fn main() void {
	var count = 0
	loop {
		count = count + 1
		println_int(count)
		if count >= 3 {
			break
		}
	}
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileSource("loop_test.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to compile loop: %v", err)
	}

	output := runWatWithWasmtime(t, res.WatOutput, "")
	want := "1\n2\n3\n"
	if output != want {
		t.Errorf("unexpected output from loop test:\ngot:  %q\nwant: %q", output, want)
	}
}

func TestForRangeWasmtime(t *testing.T) {
	code := `
import .println_int from :std:io
import .range, .range_step from :std:builtin

fn main() void {
	var sum = 0
	for i in range(1, 5) {
		sum = sum + i
	}
	println_int(sum)

	-- Test range with step
	var step_sum = 0
	for x in range_step(0, 10, 2) {
		step_sum = step_sum + x
	}
	println_int(step_sum)

	-- Test break inside for
	var early_sum = 0
	for y in range(1, 10) {
		if y > 3 {
			break
		}
		early_sum = early_sum + y
	}
	println_int(early_sum)
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileSource("for_test.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to compile for: %v", err)
	}

	output := runWatWithWasmtime(t, res.WatOutput, "")
	want := "10\n20\n6\n"
	if output != want {
		t.Errorf("unexpected output from for test:\ngot:  %q\nwant: %q", output, want)
	}
}

func TestCustomIteratorAndOptionalTypes(t *testing.T) {
	code := `
import .println_int from :std:io
import .IteratorTrait, .IterableTrait from :std:builtin

type Countdown = struct {
	remaining: i32,

	pub fn new(n: i32) Countdown {
		return Countdown::{ .remaining: n }
	}

	pub fn has_next(self: ptr<Countdown>) bool {
		return self.remaining > 0
	}

	pub fn next(self: ptr<Countdown>) i32 {
		var cur = self.remaining
		self.remaining = self.remaining - 1
		return cur
	}
}

fn test_opt() i32 {
	var a: ?i32 = none
	if a == none {
		var b: ?i32 = 42
		if b != none {
			return 42
		}
	}
	return 0
}

fn main() void {
	var cd = Countdown.new(3)
	for val in cd {
		println_int(val)
	}

	var res = test_opt()
	println_int(res)
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileSource("custom_iter.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to compile: %v (diags: %+v)", err, res.Diagnostics)
	}

	output := runWatWithWasmtime(t, res.WatOutput, "")
	want := "3\n2\n1\n42\n"
	if output != want {
		t.Errorf("unexpected output from custom iterator and optional test:\ngot:  %q\nwant: %q", output, want)
	}
}

func TestXorOperatorWasmtime(t *testing.T) {
	code := `
import .println, .println_int from :std:io

fn main() void {
	-- 1. Boolean XOR truth table
	if true xor false {
		println("t_f:ok")
	}
	if false xor true {
		println("f_t:ok")
	}
	if !(true xor true) {
		println("t_t:ok")
	}
	if !(false xor false) {
		println("f_f:ok")
	}

	-- 2. Integer bitwise XOR with 'xor' keyword
	var a = 5 xor 3
	println_int(a)

	var b = 12 xor 10
	println_int(b)

	-- 3. Integer bitwise XOR with '^' symbol
	var c = 5 ^ 3
	println_int(c)

	-- 4. Precedence: 'and' > 'xor' > 'or'
	-- (true and false) xor true or false => (false xor true) or false => true or false => true
	if true and false xor true or false {
		println("prec:ok")
	}
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileSource("xor_test.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to compile xor test: %v (diags: %+v)", err, res.Diagnostics)
	}

	output := runWatWithWasmtime(t, res.WatOutput, "")
	want := "t_f:ok\nf_t:ok\nt_t:ok\nf_f:ok\n6\n6\n6\nprec:ok\n"
	if output != want {
		t.Errorf("unexpected output from xor test:\ngot:  %q\nwant: %q", output, want)
	}
}
