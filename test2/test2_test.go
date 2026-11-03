package test2_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	lonecompiler "lone/pkg2/lone-compiler"
)

func runCompiledWat(t *testing.T, wat string) string {
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

func compileAndRun(t *testing.T, loneFile string) string {
	t.Helper()
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileFile(loneFile)
	if err != nil {
		t.Fatalf("CompileFile %s failed: %v", loneFile, err)
	}
	return runCompiledWat(t, res.WatOutput)
}

func TestHello(t *testing.T) {
	got := compileAndRun(t, "behavior/hello.lone")
	want := "Hello from Greenfield Lone (v2)!\n"
	if got != want {
		t.Errorf("unexpected output: got %q, want %q", got, want)
	}
}

func TestLetVar(t *testing.T) {
	got := compileAndRun(t, "behavior/let_var.lone")
	want := "Let and var mutability semantics verified!\n"
	if got != want {
		t.Errorf("unexpected output: got %q, want %q", got, want)
	}
}

func TestScopedHeaders(t *testing.T) {
	got := compileAndRun(t, "behavior/scoped_headers.lone")
	want := "Scoped condition header bindings verified!\n"
	if got != want {
		t.Errorf("unexpected output: got %q, want %q", got, want)
	}
}

func TestLoopWhile(t *testing.T) {
	got := compileAndRun(t, "behavior/loop_while.lone")
	want := "Post-condition loop verified!\n"
	if got != want {
		t.Errorf("unexpected output: got %q, want %q", got, want)
	}
}

func TestExplicitCasts(t *testing.T) {
	got := compileAndRun(t, "behavior/explicit_casts.lone")
	want := "Explicit as cast verified!\n"
	if got != want {
		t.Errorf("unexpected output: got %q, want %q", got, want)
	}
}

func TestLabeledBlocks(t *testing.T) {
	got := compileAndRun(t, "behavior/labeled_blocks.lone")
	want := "Labeled block expression verified!\n"
	if got != want {
		t.Errorf("unexpected output: got %q, want %q", got, want)
	}
}

func TestCheckAllTest2Suite(t *testing.T) {
	files, err := filepath.Glob("behavior/*.lone")
	if err != nil {
		t.Fatal(err)
	}
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())

	for _, f := range files {
		res, err := comp.CompileFile(f)
		if err != nil {
			var diagMsgs []string
			if res != nil {
				for _, d := range res.Diagnostics {
					diagMsgs = append(diagMsgs, d.String())
				}
			}
			t.Errorf("file %s failed to compile: %v (diags: %s)", f, err, strings.Join(diagMsgs, "; "))
		}
	}
}
