package lonecompiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	lonecompiler "lone/pkg/lone-compiler"
)

func runWat(t *testing.T, wat string) string {
	t.Helper()
	tmpDir := t.TempDir()
	watPath := filepath.Join(tmpDir, "prog.wat")
	if err := os.WriteFile(watPath, []byte(wat), 0644); err != nil {
		t.Fatalf("failed to write temp wat: %v", err)
	}

	cmd := exec.Command("wasmtime", watPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime execution failed: %v\noutput:\n%s\nWAT:\n%s", err, string(out), wat)
	}
	return string(out)
}

func TestCompileHelloLoneFile(t *testing.T) {
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileFile("../../test/behavior/hello.lone")
	if err != nil {
		t.Fatalf("CompileFile failed: %v", err)
	}

	if !strings.Contains(res.WatOutput, "(func $main") {
		t.Errorf("expected $main in wat output, got:\n%s", res.WatOutput)
	}

	got := runWat(t, res.WatOutput)
	want := "Hello from Lone!\n"
	if got != want {
		t.Errorf("unexpected stdout from wasmtime:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCompileSourceControlFlow(t *testing.T) {
	code := `
fn compute_loop(n: int) int {
	var total = 0
	var i = 0
	while i < n {
		total = total + i
		i = i + 1
	}
	return total
}

fn main() void {
	var result = compute_loop(5)
	#print("Done computing loop!\n")
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileSource("loop_test.lone", []byte(code))
	if err != nil {
		t.Fatalf("CompileSource failed: %v", err)
	}

	got := runWat(t, res.WatOutput)
	want := "Done computing loop!\n"
	if got != want {
		t.Errorf("unexpected stdout:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCompilePostConditionLoop(t *testing.T) {
	code := `
fn main() void {
	var x = 3
	loop {
		x = x - 1
	} while: [x > 0]
	#print("Post-condition loop completed!\n")
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileSource("post_loop.lone", []byte(code))
	if err != nil {
		t.Fatalf("CompileSource failed: %v", err)
	}

	got := runWat(t, res.WatOutput)
	want := "Post-condition loop completed!\n"
	if got != want {
		t.Errorf("unexpected stdout:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCompileScopedHeaderIf(t *testing.T) {
	code := `
fn main() void {
	if: [let x = 42] x == 42 {
		#print("Scoped header let binding matched!\n")
	}
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileSource("header_if.lone", []byte(code))
	if err != nil {
		t.Fatalf("CompileSource failed: %v", err)
	}

	got := runWat(t, res.WatOutput)
	want := "Scoped header let binding matched!\n"
	if got != want {
		t.Errorf("unexpected stdout:\ngot:  %q\nwant: %q", got, want)
	}
}
