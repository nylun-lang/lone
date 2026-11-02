package lonecompiler_test

import (
	"strings"
	"testing"

	lonecompiler "lone/pkg2/lone-compiler"
)

func TestCompilerEndToEndWasi(t *testing.T) {
	code := `
fn main() void {
    #print("Hello from pkg2 Compiler!\n")
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.Config{
		Target:      lonecompiler.TargetWasi,
		Optimize:    true,
		WithAdapter: "node",
	})

	res, err := comp.CompileSource("main.lone", []byte(code))
	if err != nil {
		t.Fatalf("expected successful compilation, got error: %v", err)
	}

	if res.WatOutput == "" {
		t.Fatalf("expected non-empty WAT output")
	}

	if !strings.Contains(res.WatOutput, "(func $main (export \"_start\") (export \"main\")") {
		t.Errorf("expected exported main entrypoint in WAT, got:\n%s", res.WatOutput)
	}

	if !strings.Contains(res.WatOutput, "call $__lone_rt_print") {
		t.Errorf("expected call to inlined #print, got:\n%s", res.WatOutput)
	}

	if res.AdapterGlue == "" {
		t.Errorf("expected non-empty node adapter glue")
	}

	if !strings.Contains(res.AdapterGlue, "createEnv") {
		t.Errorf("expected createEnv in node adapter glue")
	}
}

func TestCompilerEndToEndWeb(t *testing.T) {
	code := `
fn main() void {
    #print("Web target test\n")
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.Config{
		Target:      lonecompiler.TargetWeb,
		Optimize:    true,
		WithAdapter: "browser",
	})

	res, err := comp.CompileSource("web.lone", []byte(code))
	if err != nil {
		t.Fatalf("expected successful web compilation, got error: %v", err)
	}

	if !strings.Contains(res.WatOutput, "(import \"env\" \"print\"") {
		t.Errorf("expected env.print import in web target WAT, got:\n%s", res.WatOutput)
	}

	if !strings.Contains(res.AdapterGlue, "export function createEnv") {
		t.Errorf("expected browser export function in adapter glue")
	}
}

func TestCompilerEmitMir(t *testing.T) {
	code := `
fn multiply(a: i32, b: i32) i32 {
    return a * b
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.Config{
		EmitMir: true,
	})

	res, err := comp.CompileSource("mul.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to compile MIR: %v", err)
	}

	if !strings.Contains(res.WatOutput, "fn @multiply") {
		t.Errorf("expected MIR formatted function, got:\n%s", res.WatOutput)
	}
}
