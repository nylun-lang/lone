package lonewir_test

import (
	"strings"
	"testing"

	lonemir "lone/pkg2/lone-compiler-mir"
	lonewir "lone/pkg2/lone-compiler-wir"
)

func TestWasmModuleAST(t *testing.T) {
	mod := lonewir.NewModule()
	mod.MemoryPages = 1

	fn := &lonewir.WasmFunc{
		Name:    "$add",
		Params:  []lonewir.Param{{Name: "$a", Type: lonewir.TypeI32}, {Name: "$b", Type: lonewir.TypeI32}},
		Result:  lonewir.TypeI32,
		Exports: []string{"add"},
		Body: []lonewir.WasmInst{
			lonewir.InstLocalGet{Name: "$a"},
			lonewir.InstLocalGet{Name: "$b"},
			lonewir.InstBinaryOp{Op: "i32.add"},
			lonewir.InstReturn{},
		},
	}
	mod.AddFunction(fn)

	wat := mod.FormatWat()

	if !strings.Contains(wat, "(func $add (export \"add\") (param $a i32) (param $b i32) (result i32)") {
		t.Errorf("expected func declaration in wat, got:\n%s", wat)
	}
	if !strings.Contains(wat, "i32.add") {
		t.Errorf("expected i32.add instruction, got:\n%s", wat)
	}
}

func TestOptimizerSuppressNaiveDrop(t *testing.T) {
	opt := lonewir.NewOptimizer()

	insts := []lonewir.WasmInst{
		lonewir.InstNop{},
		lonewir.InstConst{Value: 42},
		lonewir.InstDrop{}, // naive const followed by drop
		lonewir.InstLocalGet{Name: "$x"},
		lonewir.InstNop{},
	}

	optimized := opt.OptimizeInsts(insts)

	if len(optimized) != 1 {
		t.Fatalf("expected 1 instruction remaining (local.get), got %d: %+v", len(optimized), optimized)
	}

	if get, ok := optimized[0].(lonewir.InstLocalGet); !ok || get.Name != "$x" {
		t.Errorf("expected local.get $x remaining, got %+v", optimized[0])
	}
}

func TestLowerMirToWat(t *testing.T) {
	mirFn := &lonemir.MirFunction{
		Name: "@calculate",
		Params: []lonemir.MirParam{
			{Name: "%x", Type: lonemir.MirI32},
		},
		ReturnType: lonemir.MirI32,
		Locals: map[string]lonemir.MirT{
			"r1": lonemir.MirI32,
		},
	}
	entry := mirFn.AddBlock("entry")
	entry.Insts = append(entry.Insts, lonemir.MirBinOp{
		Dest:  "r1",
		Op:    "add",
		Type:  lonemir.MirI32,
		Left:  lonemir.MirReg{Name: "%x"},
		Right: lonemir.MirConst{Type: lonemir.MirI32, Value: 10},
	})
	entry.Terminator = lonemir.MirReturn{Value: lonemir.MirReg{Name: "r1"}}

	mirMod := &lonemir.MirModule{
		Functions: []*lonemir.MirFunction{mirFn},
	}

	wat, err := lonewir.TranspileToWat(mirMod)
	if err != nil {
		t.Fatalf("failed to transpile MIR to WAT: %v", err)
	}

	if !strings.Contains(wat, "(func $calculate (param $x i32) (result i32)") {
		t.Errorf("expected func $calculate signature, got:\n%s", wat)
	}
	if !strings.Contains(wat, "i32.add") {
		t.Errorf("expected i32.add, got:\n%s", wat)
	}
}
