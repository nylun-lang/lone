package loneproject_test

import (
	"os"
	"path/filepath"
	"testing"

	loneanalyzer "lone/pkg/lone-analyzer"
	lonecompiler "lone/pkg/lone-compiler"
)

func hasRule(diags []loneanalyzer.Diagnostic, rule string) bool {
	for _, d := range diags {
		if d.Rule == rule {
			return true
		}
	}
	return false
}

func TestCapabilityGatingRejectsUnauthorizedHostImport(t *testing.T) {
	code := `
import .fd_write from :wasi:snapshot_preview1

fn main() void {
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	res, err := comp.CompileSource("unauthorized.lone", []byte(code))
	if err == nil {
		t.Fatalf("expected compilation to fail for unauthorized host import")
	}
	if res == nil || !hasRule(res.Diagnostics, "capability/unauthorized-host-import") {
		t.Fatalf("expected capability/unauthorized-host-import diagnostic, got: %+v", res)
	}
}

func TestCapabilityIsolationAcrossStdImport(t *testing.T) {
	// 1. User file importing :std:io WITHOUT ---fa:wasm succeeds
	validUser := `
import .println from :std:io

fn main() void {
	println("Safe sandboxed user code")
}
`
	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	if _, err := comp.CompileSource("app.lone", []byte(validUser)); err != nil {
		t.Fatalf("expected user app importing :std:io to compile cleanly, got: %v", err)
	}

	// 2. Importing :std:io MUST NOT leak fd_write into user scope
	leakCheck := `
import .println from :std:io

fn main() void {
	drop fd_write(1, 0, 0, 0)
}
`
	resLeak, err := comp.CompileSource("leak.lone", []byte(leakCheck))
	if err == nil {
		t.Fatalf("expected compilation to fail when calling unimported fd_write")
	}
	if resLeak == nil || !hasRule(resLeak.Diagnostics, "type/undefined-identifier") {
		t.Fatalf("expected type/undefined-identifier for leaked fd_write, got: %+v", resLeak)
	}

	// 3. Importing :std:io MUST NOT leak ---fa:wasm permission to the user file
	permLeakCheck := `
import .println from :std:io
import .fd_write from :wasi:snapshot_preview1

fn main() void {
	println("hi")
}
`
	resPerm, err := comp.CompileSource("perm_leak.lone", []byte(permLeakCheck))
	if err == nil {
		t.Fatalf("expected compilation to fail when user file imports :wasi:snapshot_preview1 without ---fa:wasm")
	}
	if resPerm == nil || !hasRule(resPerm.Diagnostics, "capability/unauthorized-host-import") {
		t.Fatalf("expected capability/unauthorized-host-import, got: %+v", resPerm)
	}
}

func TestRelativeModuleVisibilityAndCycles(t *testing.T) {
	tmpDir := t.TempDir()

	utilsCode := `
fn secret_num() i32 {
	return 7
}

pub fn double_secret(x: i32) i32 {
	return x + secret_num()
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "utils.lone"), []byte(utilsCode), 0644); err != nil {
		t.Fatalf("write utils.lone: %v", err)
	}

	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}

	nestedMain := `
import .double_secret from ..utils
import :std:io as io

fn main() void {
	var v = double_secret(35)
	if v == 42 {
		io.println("nested ok")
	}
}
`
	nestedPath := filepath.Join(subDir, "main.lone")
	if err := os.WriteFile(nestedPath, []byte(nestedMain), 0644); err != nil {
		t.Fatalf("write sub/main.lone: %v", err)
	}

	comp := lonecompiler.NewCompiler(lonecompiler.DefaultConfig())
	if _, err := comp.CompileFile(nestedPath); err != nil {
		t.Fatalf("expected sub/main.lone with ..utils and aliased :std:io to compile, got: %v", err)
	}

	// Importing private symbol `secret_num` from `utils` must fail
	privateImport := `
import .secret_num from .utils

fn main() void {
	drop secret_num()
}
`
	privPath := filepath.Join(tmpDir, "priv.lone")
	if err := os.WriteFile(privPath, []byte(privateImport), 0644); err != nil {
		t.Fatalf("write priv.lone: %v", err)
	}
	resPriv, err := comp.CompileFile(privPath)
	if err == nil {
		t.Fatalf("expected private symbol import to fail")
	}
	if resPriv == nil || !hasRule(resPriv.Diagnostics, "import/undefined-symbol") {
		t.Fatalf("expected import/undefined-symbol for private symbol, got: %+v", resPriv)
	}

	// Circular import detection
	cycA := filepath.Join(tmpDir, "cyc_a.lone")
	cycB := filepath.Join(tmpDir, "cyc_b.lone")
	_ = os.WriteFile(cycA, []byte("import .b_fn from .cyc_b\npub fn a_fn() i32 { return b_fn() }\n"), 0644)
	_ = os.WriteFile(cycB, []byte("import .a_fn from .cyc_a\npub fn b_fn() i32 { return a_fn() }\n"), 0644)

	resCyc, err := comp.CompileFile(cycA)
	if err == nil {
		t.Fatalf("expected circular import to fail")
	}
	if resCyc == nil || !hasRule(resCyc.Diagnostics, "import/cycle") {
		t.Fatalf("expected import/cycle diagnostic, got: %+v", resCyc)
	}
}
