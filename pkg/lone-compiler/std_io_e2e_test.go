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
import .Stdout, .Writer from :std:io
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
	let w = Writer::|Stdout|
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
