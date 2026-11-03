package loneproject

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDebugInspectStructWithTraitImpl(t *testing.T) {
	info, err := InspectSymbol(":std:io", "Stdout")
	if err != nil {
		t.Fatalf("InspectSymbol failed: %v", err)
	}

	if info.Name != "Stdout" {
		t.Errorf("expected name Stdout, got %s", info.Name)
	}
	if info.Kind != DebugKindStruct {
		t.Errorf("expected kind struct, got %s", info.Kind)
	}
	if len(info.Fields) == 0 {
		t.Fatalf("expected Stdout to have fields, got 0")
	}
	if info.Fields[0].Name != "fd" {
		t.Errorf("expected field fd, got %s", info.Fields[0].Name)
	}

	// Should implement WriterTrait
	foundWriter := false
	for _, tr := range info.ImplementedTraits {
		if tr.TraitName == "WriterTrait" {
			foundWriter = true
			if len(tr.Methods) == 0 {
				t.Errorf("expected WriterTrait on Stdout to have methods")
			}
		}
	}
	if !foundWriter {
		t.Errorf("expected Stdout to implement WriterTrait, got: %+v", info.ImplementedTraits)
	}
}

func TestDebugInspectTraitWithImplementors(t *testing.T) {
	info, err := InspectSymbol(":std:io", "WriterTrait")
	if err != nil {
		t.Fatalf("InspectSymbol failed: %v", err)
	}

	if info.Name != "WriterTrait" {
		t.Errorf("expected name WriterTrait, got %s", info.Name)
	}
	if info.Kind != DebugKindTrait {
		t.Errorf("expected kind trait, got %s", info.Kind)
	}
	if len(info.TraitMethods) == 0 {
		t.Fatalf("expected WriterTrait to have methods")
	}
	if info.TraitMethods[0].Name != "write" {
		t.Errorf("expected method write, got %s", info.TraitMethods[0].Name)
	}

	// Should have Stdout and Stderr as implementors
	foundStdout := false
	for _, imp := range info.Implementors {
		if imp == "Stdout" {
			foundStdout = true
		}
	}
	if !foundStdout {
		t.Errorf("expected Stdout in implementors of WriterTrait, got: %v", info.Implementors)
	}
}

func TestDebugInspectScopedSymbol(t *testing.T) {
	// 1. Struct field: Stdout:fd
	fInfo, err := InspectSymbol(":std:io", "Stdout:fd")
	if err != nil {
		t.Fatalf("InspectSymbol Stdout:fd failed: %v", err)
	}
	if fInfo.Kind != DebugKindField || fInfo.Name != "fd" || fInfo.ParentStruct != "Stdout" {
		t.Errorf("unexpected field info: %+v", fInfo)
	}

	// 2. Struct method: Stdout:write
	mInfo, err := InspectSymbol(":std:io", "Stdout:write")
	if err != nil {
		t.Fatalf("InspectSymbol Stdout:write failed: %v", err)
	}
	if mInfo.Kind != DebugKindMethod || mInfo.Name != "write" {
		t.Errorf("unexpected method info: %+v", mInfo)
	}

	// 3. Dot syntax: Stdout.write
	dotInfo, err := InspectSymbol(":std:io", "Stdout.write")
	if err != nil {
		t.Fatalf("InspectSymbol Stdout.write failed: %v", err)
	}
	if dotInfo.Kind != DebugKindMethod || dotInfo.Name != "write" {
		t.Errorf("unexpected dot syntax method info: %+v", dotInfo)
	}

	// 4. Arrow syntax for fn->var: str_len->n
	arrowInfo, err := InspectSymbol(":std:io", "str_len->n")
	if err != nil {
		t.Fatalf("InspectSymbol str_len->n failed: %v", err)
	}
	if arrowInfo.Kind != DebugKindVar || arrowInfo.Name != "n" || arrowInfo.EnclosingFn != "str_len" {
		t.Errorf("unexpected arrow syntax info: %+v", arrowInfo)
	}
}

func TestDebugInspectVariableAddressTaken(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "test_raw.lone")
	src := `
fn compute() i32 {
    var num: i32 = 42
    var p = &raw num
    return 0
}
`
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := InspectSymbol(srcFile, "compute:num")
	if err != nil {
		t.Fatalf("InspectSymbol compute:num failed: %v", err)
	}

	if info.Kind != DebugKindVar {
		t.Errorf("expected kind var, got %s", info.Kind)
	}
	if !info.AddressTaken {
		t.Errorf("expected num to have AddressTaken=true due to &raw num")
	}
	if info.Storage != "datastack frame" {
		t.Errorf("expected storage 'datastack frame', got %s", info.Storage)
	}
}

func TestDebugDumpScope(t *testing.T) {
	dump, err := DumpScope(":std:io")
	if err != nil {
		t.Fatalf("DumpScope failed: %v", err)
	}

	if len(dump.Structs) == 0 {
		t.Errorf("expected structs in :std:io, got 0")
	}
	if len(dump.Traits) == 0 {
		t.Errorf("expected traits in :std:io, got 0")
	}
	if len(dump.Functions) == 0 {
		t.Errorf("expected functions in :std:io, got 0")
	}

	foundStdout := false
	for _, s := range dump.Structs {
		if s.Name == "Stdout" {
			foundStdout = true
			if s.TraitCount == 0 {
				t.Errorf("expected Stdout to have traits in dump")
			}
		}
	}
	if !foundStdout {
		t.Errorf("expected Stdout in dump.Structs")
	}
}
