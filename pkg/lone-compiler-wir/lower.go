package lonewir

import (
	"fmt"
	"strings"

	lonemir "lone/pkg/lone-compiler-mir"
	loneruntime "lone/pkg/lone-runtime"
)

// LowerToWasmModule translates a Falone MirModule into a full WebAssembly AST (WasmModule).
func LowerToWasmModule(mod *lonemir.MirModule, target loneruntime.Target, rt *loneruntime.Table) (*WasmModule, error) {
	if mod == nil {
		return NewModule(), nil
	}
	if rt == nil {
		rt = loneruntime.NewDefaultTable()
	}
	if target == "" {
		target = loneruntime.TargetWasi
	}

	wasmMod := NewModule()

	// 1. Resolve intrinsics and register required imports & inlined runtime code
	needsPrint := usesPrintIntrinsic(mod)
	if needsPrint {
		spec, ok := rt.Lookup("#print", target)
		if ok {
			for _, imp := range spec.Imports {
				var pTypes []WasmType
				for _, p := range imp.Params {
					pTypes = append(pTypes, WasmType(p))
				}
				wasmMod.Imports = append(wasmMod.Imports, Import{
					Module: imp.Module,
					Name:   imp.Name,
					Alias:  imp.Alias,
					Params: pTypes,
					Result: WasmType(imp.Result),
				})
			}
			if spec.InlinedWatFunc != "" {
				wasmMod.InlinedRuntime = append(wasmMod.InlinedRuntime, spec.InlinedWatFunc)
			}
		}
	}

	// 2. Data segments (linear memory)
	for _, seg := range mod.DataSegments {
		wasmMod.DataSegments = append(wasmMod.DataSegments, DataSegment{
			Offset: seg.Offset,
			RawStr: seg.Value,
		})
	}

	// 3. Lower functions
	for _, fn := range mod.Functions {
		wfn, err := lowerFunction(fn)
		if err != nil {
			return nil, err
		}
		wasmMod.AddFunction(wfn)
	}

	return wasmMod, nil
}

// TranspileToWat converts a MirModule into WebAssembly Text format via the WIR AST.
func TranspileToWat(mod *lonemir.MirModule) (string, error) {
	return TranspileToWatWithTarget(mod, loneruntime.TargetWasi)
}

// TranspileToWatWithTarget converts a MirModule into WebAssembly Text format targeting WASI or Web.
func TranspileToWatWithTarget(mod *lonemir.MirModule, target loneruntime.Target) (string, error) {
	ast, err := LowerToWasmModule(mod, target, loneruntime.NewDefaultTable())
	if err != nil {
		return "", err
	}

	// Run optimizer pass on the AST
	opt := NewOptimizer()
	opt.OptimizeModule(ast)

	return ast.FormatWat(), nil
}

func usesPrintIntrinsic(mod *lonemir.MirModule) bool {
	if mod == nil {
		return false
	}
	for _, fn := range mod.Functions {
		for _, blk := range fn.Blocks {
			for _, inst := range blk.Insts {
				if call, ok := inst.(lonemir.MirCallInst); ok {
					callee := cleanIdentifier(call.Callee)
					if callee == "__lone_rt_print" || callee == "#print" || callee == "print" {
						return true
					}
				}
			}
		}
	}
	return false
}

func lowerFunction(fn *lonemir.MirFunction) (*WasmFunc, error) {
	fnName := cleanIdentifier(fn.Name)
	wfn := &WasmFunc{
		Name: "$" + fnName,
	}

	// Export entrypoints
	if fnName == "main" {
		wfn.Exports = append(wfn.Exports, "_start", "main")
	}

	// Parameters
	for _, p := range fn.Params {
		pName := cleanIdentifier(p.Name)
		wfn.Params = append(wfn.Params, Param{
			Name: "$" + pName,
			Type: toWasmType(p.Type),
		})
	}

	// Result
	if fn.ReturnType != lonemir.MirVoid {
		wfn.Result = toWasmType(fn.ReturnType)
	} else {
		wfn.Result = TypeVoid
	}

	// Locals (virtual registers minus parameters)
	paramSet := make(map[string]bool)
	for _, p := range fn.Params {
		paramSet[cleanIdentifier(p.Name)] = true
	}
	for reg, typ := range fn.Locals {
		cleanReg := cleanIdentifier(reg)
		if !paramSet[cleanReg] {
			wfn.Locals = append(wfn.Locals, Local{
				Name: "$" + cleanReg,
				Type: toWasmType(typ),
			})
		}
	}

	// Instructions
	if len(fn.Blocks) > 0 {
		wfn.Body = lowerFunctionBlocks(fn.Blocks)
	}

	return wfn, nil
}

func lowerFunctionBlocks(blocks []*lonemir.MirBlock) []WasmInst {
	if hasLoop(blocks) {
		return lowerLoopBlocks(blocks)
	} else if len(blocks) == 1 {
		return lowerBlockBody(blocks[0])
	} else {
		return lowerStructuredBlocks(blocks)
	}
}

func hasLoop(blocks []*lonemir.MirBlock) bool {
	for _, b := range blocks {
		if b.IsLoop || strings.HasPrefix(b.Name, "while_head") {
			return true
		}
	}
	return false
}

func lowerBlockBody(blk *lonemir.MirBlock) []WasmInst {
	var insts []WasmInst
	insts = append(insts, InstRaw{Code: ";; label " + blk.Name})

	for _, inst := range blk.Insts {
		insts = append(insts, lowerInstruction(inst)...)
	}

	if blk.Terminator != nil {
		insts = append(insts, lowerTerminator(blk.Terminator)...)
	}

	return insts
}

func lowerStructuredBlocks(blocks []*lonemir.MirBlock) []WasmInst {
	var insts []WasmInst
	total := len(blocks)

	for i, blk := range blocks {
		insts = append(insts, InstRaw{Code: ";; label " + blk.Name})
		for _, inst := range blk.Insts {
			insts = append(insts, lowerInstruction(inst)...)
		}
		if blk.Terminator != nil {
			if jmp, ok := blk.Terminator.(lonemir.MirJump); ok {
				if i+1 < total && cleanIdentifier(jmp.Target) == cleanIdentifier(blocks[i+1].Name) {
					// Fallthrough
					continue
				}
			}
			insts = append(insts, lowerTerminator(blk.Terminator)...)
		}
	}

	return insts
}

func lowerLoopBlocks(blocks []*lonemir.MirBlock) []WasmInst {
	var insts []WasmInst

	for i := 0; i < len(blocks); i++ {
		blk := blocks[i]

		if blk.IsLoop || strings.HasPrefix(blk.Name, "while_head") {
			exitName := blk.LoopExit
			if exitName == "" {
				if br, ok := blk.Terminator.(lonemir.MirBranch); ok {
					exitName = br.ElseTarget
				}
			}
			loopHead := cleanIdentifier(blk.Name)
			loopExit := cleanIdentifier(exitName)

			exitIdx := -1
			for j := i + 1; j < len(blocks); j++ {
				if blocks[j].Name == exitName {
					exitIdx = j
					break
				}
			}

			var loopBody []WasmInst
			// Condition evaluation
			if len(blk.Insts) > 0 {
				loopBody = append(loopBody, InstRaw{Code: ";; Evaluate condition"})
				for _, inst := range blk.Insts {
					loopBody = append(loopBody, lowerInstruction(inst)...)
				}
			}

			// Exit if false
			if br, ok := blk.Terminator.(lonemir.MirBranch); ok {
				loopBody = append(loopBody, InstRaw{Code: ";; Exit if condition is false"})
				loopBody = append(loopBody, lowerLoadOperand(br.Cond, lonemir.MirI32)...)
				loopBody = append(loopBody, InstRaw{Code: "i32.eqz"})
				loopBody = append(loopBody, InstBrIf{Target: "$" + loopExit})
			}

			// Loop body
			bodyBlocksEnd := len(blocks)
			if exitIdx != -1 {
				bodyBlocksEnd = exitIdx
			}

			loopBody = append(loopBody, InstRaw{Code: ";; Body"})
			repeated := false
			for j := i + 1; j < bodyBlocksEnd; j++ {
				bodyBlk := blocks[j]
				for _, inst := range bodyBlk.Insts {
					loopBody = append(loopBody, lowerInstruction(inst)...)
				}
				if jmp, ok := bodyBlk.Terminator.(lonemir.MirJump); ok {
					target := cleanIdentifier(jmp.Target)
					if target == loopExit {
						loopBody = append(loopBody, InstBr{Target: "$" + loopExit})
					} else if target == loopHead {
						loopBody = append(loopBody, InstRaw{Code: ";; Repeat"})
						loopBody = append(loopBody, InstBr{Target: "$" + loopHead})
						repeated = true
					}
				} else if ret, ok := bodyBlk.Terminator.(lonemir.MirReturn); ok {
					loopBody = append(loopBody, lowerTerminator(ret)...)
				}
			}

			if !repeated {
				loopBody = append(loopBody, InstRaw{Code: ";; Repeat"})
				loopBody = append(loopBody, InstBr{Target: "$" + loopHead})
			}

			// Wrap in (block $exit (loop $head ...))
			loopInst := InstLoop{
				Label: "$" + loopHead,
				Body:  loopBody,
			}
			blockInst := InstBlock{
				Label: "$" + loopExit,
				Body:  []WasmInst{loopInst},
			}

			insts = append(insts, blockInst)

			if exitIdx != -1 {
				i = exitIdx - 1
			} else {
				i = bodyBlocksEnd - 1
			}
			continue
		}

		insts = append(insts, InstRaw{Code: ";; label " + blk.Name})
		for _, inst := range blk.Insts {
			insts = append(insts, lowerInstruction(inst)...)
		}
		if blk.Terminator != nil {
			insts = append(insts, lowerTerminator(blk.Terminator)...)
		}
	}

	return insts
}

func lowerInstruction(inst lonemir.MirInst) []WasmInst {
	var insts []WasmInst

	switch n := inst.(type) {
	case lonemir.MirBinOp:
		wType := toWasmType(n.Type)
		wasmOp := string(wType) + "." + n.Op
		if n.Op == "div" {
			if n.Type == lonemir.MirF32 || n.Type == lonemir.MirF64 {
				wasmOp = string(wType) + ".div"
			} else {
				wasmOp = string(wType) + ".div_s"
			}
		} else if n.Op == "mod" {
			wasmOp = string(wType) + ".rem_s"
		} else if n.Op == "lt" {
			wasmOp = string(wType) + ".lt_s"
		} else if n.Op == "le" {
			wasmOp = string(wType) + ".le_s"
		} else if n.Op == "gt" {
			wasmOp = string(wType) + ".gt_s"
		} else if n.Op == "ge" {
			wasmOp = string(wType) + ".ge_s"
		}

		insts = append(insts, lowerLoadOperand(n.Left, n.Type)...)
		insts = append(insts, lowerLoadOperand(n.Right, n.Type)...)
		insts = append(insts, InstBinaryOp{Op: wasmOp})
		insts = append(insts, InstLocalSet{Name: "$" + cleanIdentifier(n.Dest)})

	case lonemir.MirCmpOp:
		wType := toWasmType(n.Type)
		wasmOp := string(wType) + "." + n.Op
		switch n.Op {
		case "lt":
			wasmOp = string(wType) + ".lt_s"
		case "gt":
			wasmOp = string(wType) + ".gt_s"
		case "le":
			wasmOp = string(wType) + ".le_s"
		case "ge":
			wasmOp = string(wType) + ".ge_s"
		case "eq":
			wasmOp = string(wType) + ".eq"
		case "ne":
			wasmOp = string(wType) + ".ne"
		}
		insts = append(insts, lowerLoadOperand(n.Left, n.Type)...)
		insts = append(insts, lowerLoadOperand(n.Right, n.Type)...)
		insts = append(insts, InstBinaryOp{Op: wasmOp})
		insts = append(insts, InstLocalSet{Name: "$" + cleanIdentifier(n.Dest)})

	case lonemir.MirConstInst:
		insts = append(insts, lowerConstant(n.Value, n.Type)...)
		insts = append(insts, InstLocalSet{Name: "$" + cleanIdentifier(n.Dest)})

	case lonemir.MirCallInst:
		callee := cleanIdentifier(n.Callee)
		if callee == "__lone_rt_print" || callee == "#print" || callee == "print" {
			callee = "__lone_rt_print"
		}
		for _, arg := range n.Args {
			insts = append(insts, lowerLoadOperand(arg, lonemir.MirI32)...)
		}
		insts = append(insts, InstCall{Target: "$" + callee})
		if n.Dest != "" {
			insts = append(insts, InstLocalSet{Name: "$" + cleanIdentifier(n.Dest)})
		}

	case lonemir.MirCopyInst:
		insts = append(insts, lowerLoadOperand(n.Src, lonemir.MirI32)...)
		insts = append(insts, InstLocalSet{Name: "$" + cleanIdentifier(n.Dest)})
	}

	return insts
}

func lowerTerminator(term lonemir.MirTerminator) []WasmInst {
	var insts []WasmInst

	switch t := term.(type) {
	case lonemir.MirReturn:
		if t.Value != nil {
			insts = append(insts, lowerLoadOperand(t.Value, lonemir.MirI32)...)
		}
		insts = append(insts, InstReturn{})

	case lonemir.MirJump:
		insts = append(insts, InstBr{Target: "$" + cleanIdentifier(t.Target)})

	case lonemir.MirBranch:
		insts = append(insts, lowerLoadOperand(t.Cond, lonemir.MirI32)...)
		insts = append(insts, InstBrIf{Target: "$" + cleanIdentifier(t.ThenTarget)})
		if t.ElseTarget != "" {
			insts = append(insts, InstBr{Target: "$" + cleanIdentifier(t.ElseTarget)})
		}
	}

	return insts
}

func lowerLoadOperand(op lonemir.MirOperand, typ lonemir.MirT) []WasmInst {
	switch o := op.(type) {
	case lonemir.MirReg:
		return []WasmInst{InstLocalGet{Name: "$" + cleanIdentifier(o.Name)}}
	case lonemir.MirConst:
		return lowerConstant(o.Value, typ)
	default:
		return nil
	}
}

func lowerConstant(val any, typ lonemir.MirT) []WasmInst {
	switch v := val.(type) {
	case int:
		return []WasmInst{InstConst{Value: int32(v)}}
	case int32:
		return []WasmInst{InstConst{Value: v}}
	case int64:
		return []WasmInst{InstRaw{Code: fmt.Sprintf("i64.const %d", v)}}
	case bool:
		bVal := int32(0)
		if v {
			bVal = 1
		}
		return []WasmInst{InstConst{Value: bVal}}
	default:
		return []WasmInst{InstConst{Value: 0}}
	}
}

func cleanIdentifier(name string) string {
	name = strings.TrimPrefix(name, "%")
	name = strings.TrimPrefix(name, "@")
	name = strings.TrimPrefix(name, "$")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "-", "_")
	return name
}

func toWasmType(t lonemir.MirT) WasmType {
	switch t {
	case lonemir.MirI32:
		return TypeI32
	case lonemir.MirI64:
		return TypeI64
	case lonemir.MirF32:
		return TypeF32
	case lonemir.MirF64:
		return TypeF64
	case lonemir.MirVoid:
		return TypeVoid
	default:
		return TypeI32
	}
}
