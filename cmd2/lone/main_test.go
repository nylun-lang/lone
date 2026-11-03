package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmd2LoneVersion(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("version command failed: %v, output: %s", err, string(out))
	}
	if !strings.Contains(string(out), "lone v2") {
		t.Errorf("expected 'lone v2', got:\n%s", string(out))
	}
}

func TestCmd2LoneParseAndCheck(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "parse", "../../test/behavior/hello.lone")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("parse command failed: %v, output: %s", err, string(out))
	}
	if !strings.Contains(string(out), "Successfully parsed") {
		t.Errorf("expected 'Successfully parsed', got:\n%s", string(out))
	}

	cmd = exec.Command("go", "run", ".", "check", "../../test/behavior/hello.lone")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("check command failed: %v, output: %s", err, string(out))
	}
	if !strings.Contains(string(out), "0 errors") {
		t.Errorf("expected '0 errors', got:\n%s", string(out))
	}
}

func TestCmd2LoneBuild(t *testing.T) {
	tmpDir := t.TempDir()
	outWat := filepath.Join(tmpDir, "hello.wat")

	cmd := exec.Command("go", "run", ".", "build", "../../test/behavior/hello.lone", "-o", outWat)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build command failed: %v, output: %s", err, string(out))
	}

	if _, err := os.Stat(outWat); err != nil {
		t.Fatalf("expected output wat file to exist at %s: %v", outWat, err)
	}

	// Verify running with wasmtime
	wasmCmd := exec.Command("wasmtime", outWat)
	wOut, err := wasmCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime execution failed: %v, output: %s", err, string(wOut))
	}

	want := "Hello from Lone!\n"
	if string(wOut) != want {
		t.Errorf("unexpected stdout from built wat:\ngot:  %q\nwant: %q", string(wOut), want)
	}
}
