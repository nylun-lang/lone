package wir

import (
	"strings"
	"testing"

	"lone/pkg/mir"
	
)

func TestTranspileSyntaxExampleToWat(t *testing.T) {
	// Construct the exact MIR example from syntax.txt:
	// fn @add(%a: i32, %b: i32) -> i32
	fnAdd := &mir.MirFunction{
		Name: "@add",
		Params: []mir.MirParam{
			{Name: "%a", Type: mir.MirI32},
			{Name: "%b", Type: mir.MirI32},
		},
		ReturnType: mir.MirI32,
		Locals: map[string]mir.MirT{
			"r3": mir.MirI32,
		},
	}
	entryAdd := fnAdd.AddBlock("entry")
	entryAdd.Insts = append(entryAdd.Insts, mir.MirBinOp{
		Op:    "add",
		Type:  mir.MirI32,
		Dest:  "r3",
		Left:  mir.MirReg{Name: "a"},
		Right: mir.MirReg{Name: "b"},
	})
	exitAdd := fnAdd.AddBlock("exit")
	exitAdd.Terminator = mir.MirReturn{Value: mir.MirReg{Name: "r3"}}

	// fn main() -> void
	fnMain := &mir.MirFunction{
		Name:       "main",
		Params:     nil,
		ReturnType: mir.MirVoid,
		Locals: map[string]mir.MirT{
			"r1":        mir.MirI32,
			"condition": mir.MirI32,
		},
	}
	entryMain := fnMain.AddBlock("entry")
	entryMain.Insts = append(entryMain.Insts,
		mir.MirCallInst{
			Dest:   "r1",
			Callee: "@add",
			Args: []mir.MirOperand{
				mir.MirConst{Type: mir.MirI32, Value: 1},
				mir.MirConst{Type: mir.MirI32, Value: 2},
			},
		},
		mir.MirCmpOp{
			Op:    "lt",
			Type:  mir.MirI32,
			Dest:  "condition",
			Left:  mir.MirReg{Name: "r1"},
			Right: mir.MirConst{Type: mir.MirI32, Value: 10},
		},
	)
	entryMain.Terminator = mir.MirBranch{
		Cond:       mir.MirReg{Name: "condition"},
		ThenTarget: "if_block",
		ElseTarget: "exit",
	}

	ifBlock := fnMain.AddBlock("if_block")
	ifBlock.Insts = append(ifBlock.Insts, mir.MirCallInst{
		Callee: "@do_foo",
	})
	ifBlock.Terminator = mir.MirJump{Target: "exit"}

	exitMain := fnMain.AddBlock("exit")
	exitMain.Terminator = mir.MirReturn{}

	mod := &mir.MirModule{
		Functions: []*mir.MirFunction{fnAdd, fnMain},
	}

	// 1. Verify MIR Assembly formatting
	mirText := mir.FormatModule(mod)
	if !strings.Contains(mirText, "fn @add(%a: i32, %b: i32) -> i32") {
		t.Errorf("unexpected MIR text: %s", mirText)
	}
	if !strings.Contains(mirText, "%r3 = add i32 %a, %b") {
		t.Errorf("unexpected MIR add: %s", mirText)
	}

	// 2. Transpile to WAT
	watText, err := TranspileToWat(mod)
	if err != nil {
		t.Fatalf("failed to transpile to WAT: %v", err)
	}

	if !strings.Contains(watText, "(func $add") {
		t.Errorf("expected (func $add in WAT, got:\n%s", watText)
	}
	if !strings.Contains(watText, `(func $main (export "_start") (export "main")`) {
		t.Errorf("expected (func $main (export \"_start\") (export \"main\") in WAT, got:\n%s", watText)
	}
	if !strings.Contains(watText, "i32.add") {
		t.Errorf("expected i32.add in WAT, got:\n%s", watText)
	}
	if !strings.Contains(watText, "i32.lt_s") {
		t.Errorf("expected i32.lt_s in WAT, got:\n%s", watText)
	}
	if !strings.Contains(watText, "call $add") {
		t.Errorf("expected call $add in WAT, got:\n%s", watText)
	}
}

func TestTranspileWhileLoopToWat(t *testing.T) {
	// Function with while loop:
	// (block $while_exit
	//   (loop $while_head
	//     ;; evaluate cond
	//     ...
	//     i32.eqz
	//     br_if $while_exit
	//     ;; body
	//     ...
	//     br $while_head
	//   )
	// )
	fnLoop := &mir.MirFunction{
		Name:       "test_loop",
		Params:     nil,
		ReturnType: mir.MirVoid,
		Locals: map[string]mir.MirT{
			"counter": mir.MirI32,
			"cond":    mir.MirI32,
		},
	}

	entry := fnLoop.AddBlock("entry")
	entry.Insts = append(entry.Insts, mir.MirConstInst{
		Dest:  "counter",
		Type:  mir.MirI32,
		Value: 0,
	})
	entry.Terminator = mir.MirJump{Target: "while_head"}

	whileHead := fnLoop.AddBlock("while_head")
	whileHead.IsLoop = true
	whileHead.LoopExit = "while_exit"
	whileHead.Insts = append(whileHead.Insts, mir.MirCmpOp{
		Op:    "lt",
		Type:  mir.MirI32,
		Dest:  "cond",
		Left:  mir.MirReg{Name: "counter"},
		Right: mir.MirGlobal{Name: "max_retries"},
	})
	whileHead.Terminator = mir.MirBranch{
		Cond:       mir.MirReg{Name: "cond"},
		ThenTarget: "while_body",
		ElseTarget: "while_exit",
	}

	whileBody := fnLoop.AddBlock("while_body")
	whileBody.Insts = append(whileBody.Insts, mir.MirBinOp{
		Op:    "add",
		Type:  mir.MirI32,
		Dest:  "counter",
		Left:  mir.MirReg{Name: "counter"},
		Right: mir.MirConst{Type: mir.MirI32, Value: 1},
	})
	whileBody.Terminator = mir.MirJump{Target: "while_head"}

	whileExit := fnLoop.AddBlock("while_exit")
	whileExit.Terminator = mir.MirReturn{}

	mod := &mir.MirModule{
		Functions: []*mir.MirFunction{fnLoop},
		Globals: map[string]mir.MirT{
			"max_retries": mir.MirI32,
		},
	}

	watText, err := TranspileToWat(mod)
	if err != nil {
		t.Fatalf("transpile failed: %v", err)
	}

	// Verify the exact loop structure:
	// 1. (block $while_exit
	// 2.   (loop $while_head
	// 3.     i32.eqz
	// 4.     br_if $while_exit
	// 5.     br $while_head
	if !strings.Contains(watText, "(block $while_exit") {
		t.Errorf("missing (block $while_exit in WAT:\n%s", watText)
	}
	if !strings.Contains(watText, "(loop $while_head") {
		t.Errorf("missing (loop $while_head in WAT:\n%s", watText)
	}
	if !strings.Contains(watText, "i32.eqz") {
		t.Errorf("missing i32.eqz in WAT:\n%s", watText)
	}
	if !strings.Contains(watText, "br_if $while_exit") {
		t.Errorf("missing br_if $while_exit in WAT:\n%s", watText)
	}
	if !strings.Contains(watText, "br $while_head") {
		t.Errorf("missing br $while_head in WAT:\n%s", watText)
	}
}

func TestTranspilePrintIntrinsicToWatWasi(t *testing.T) {
	fnMain := &mir.MirFunction{
		Name:       "main",
		Params:     nil,
		ReturnType: mir.MirVoid,
		Locals:     make(map[string]mir.MirT),
	}
	entry := fnMain.AddBlock("entry")
	entry.Insts = append(entry.Insts, mir.MirCallInst{
		Callee: "__lone_rt_print",
		Args: []mir.MirOperand{
			mir.MirConst{Type: mir.MirI32, Value: 16},
			mir.MirConst{Type: mir.MirI32, Value: 17},
		},
	})
	entry.Terminator = mir.MirReturn{}

	mod := &mir.MirModule{
		Functions: []*mir.MirFunction{fnMain},
		DataSegments: []mir.MirDataSegment{
			{Offset: 16, Value: "Hello from Lone!\n"},
		},
	}

	watText, err := TranspileToWatWithTarget(mod, TargetWasi)
	if err != nil {
		t.Fatalf("transpile failed: %v", err)
	}

	if !strings.Contains(watText, `(import "wasi_snapshot_preview1" "fd_write"`) {
		t.Errorf("missing wasi_snapshot_preview1 fd_write import in WAT:\n%s", watText)
	}
	if !strings.Contains(watText, `(data (i32.const 16) "Hello from Lone!\0a\00")`) {
		t.Errorf("missing or incorrect data segment in WAT:\n%s", watText)
	}
	if !strings.Contains(watText, `(func $__lone_rt_print (param $ptr i32) (param $len i32)`) {
		t.Errorf("missing $__lone_rt_print function in WAT:\n%s", watText)
	}
	if !strings.Contains(watText, `call $__lone_rt_print`) {
		t.Errorf("missing call to $__lone_rt_print in WAT:\n%s", watText)
	}
}

func TestTranspilePrintIntrinsicToWatWeb(t *testing.T) {
	fnMain := &mir.MirFunction{
		Name:       "main",
		Params:     nil,
		ReturnType: mir.MirVoid,
		Locals:     make(map[string]mir.MirT),
	}
	entry := fnMain.AddBlock("entry")
	entry.Insts = append(entry.Insts, mir.MirCallInst{
		Callee: "__lone_rt_print",
		Args: []mir.MirOperand{
			mir.MirConst{Type: mir.MirI32, Value: 16},
			mir.MirConst{Type: mir.MirI32, Value: 17},
		},
	})
	entry.Terminator = mir.MirReturn{}

	mod := &mir.MirModule{
		Functions: []*mir.MirFunction{fnMain},
		DataSegments: []mir.MirDataSegment{
			{Offset: 16, Value: "Hello from Lone!\n"},
		},
	}

	watText, err := TranspileToWatWithTarget(mod, TargetWeb)
	if err != nil {
		t.Fatalf("transpile failed: %v", err)
	}

	if !strings.Contains(watText, `(import "env" "print" (func $__lone_rt_print (param i32 i32)))`) {
		t.Errorf("missing env print import in WAT:\n%s", watText)
	}
	if strings.Contains(watText, "wasi_snapshot_preview1") {
		t.Errorf("did not expect wasi import for web target in WAT:\n%s", watText)
	}
}
