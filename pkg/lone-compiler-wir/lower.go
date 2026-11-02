package lonewir

import (
	"fmt"
	"sort"
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
	importedAliases := make(map[string]bool)

	// 1a. Emit authorized host imports from ---fa:wasm directives
	for _, himp := range mod.HostImports {
		alias := "$" + cleanIdentifier(himp.LocalName)
		if importedAliases[alias] {
			continue
		}
		importedAliases[alias] = true

		var pTypes []WasmType
		for _, p := range himp.Params {
			pTypes = append(pTypes, WasmType(p))
		}
		wasmMod.Imports = append(wasmMod.Imports, Import{
			Module: himp.WasmModule,
			Name:   himp.WasmName,
			Alias:  alias,
			Params: pTypes,
			Result: WasmType(himp.Result),
		})
	}

	// 1b. Resolve legacy intrinsics (#print) and register required imports & inlined runtime code
	needsPrint := usesPrintIntrinsic(mod)
	if needsPrint {
		spec, ok := rt.Lookup("#print", target)
		if ok {
			for _, imp := range spec.Imports {
				if importedAliases[imp.Alias] {
					continue
				}
				importedAliases[imp.Alias] = true
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
	dataEnd := 32
	for _, seg := range mod.DataSegments {
		wasmMod.DataSegments = append(wasmMod.DataSegments, DataSegment{
			Offset: seg.Offset,
			RawStr: seg.Value,
		})
		end := seg.Offset + len(seg.Value) + 1
		if end > dataEnd {
			dataEnd = end
		}
	}
	heapStart := (dataEnd + 15) & ^15

	// 2.5. Linear memory datastack & heap arena globals (emitted when frame slots or heap are used)
	if needsStackOrHeap(mod) {
		wasmMod.MemoryPages = 2 // 128 KiB: [0..heapStart) static, [heapStart..sp) heap, (sp..131072] datastack
		wasmMod.Globals = append(wasmMod.Globals,
			Global{
				Name:    "$__stack_pointer",
				Type:    TypeI32,
				Mutable: true,
				InitVal: "i32.const 131072",
			},
			Global{
				Name:    "$__heap_pointer",
				Type:    TypeI32,
				Mutable: true,
				InitVal: fmt.Sprintf("i32.const %d", heapStart),
			},
		)
	}

	// User-defined globals
	if len(mod.Globals) > 0 {
		var globalNames []string
		for gName := range mod.Globals {
			globalNames = append(globalNames, gName)
		}
		sort.Strings(globalNames)
		for _, gName := range globalNames {
			gType := mod.Globals[gName]
			wType := toWasmType(gType)
			initExpr := formatGlobalInit(mod.GlobalInits[gName], gType)
			wasmMod.Globals = append(wasmMod.Globals, Global{
				Name:    "$" + cleanIdentifier(gName),
				Type:    wType,
				Mutable: true,
				InitVal: initExpr,
			})
		}
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

func needsStackOrHeap(mod *lonemir.MirModule) bool {
	if mod.UsesHeap {
		return true
	}
	for _, fn := range mod.Functions {
		if fn.FrameSize > 0 {
			return true
		}
	}
	return false
}

func formatGlobalInit(val any, typ lonemir.MirT) string {
	switch typ {
	case lonemir.MirI64:
		switch v := val.(type) {
		case int:
			return fmt.Sprintf("i64.const %d", v)
		case int64:
			return fmt.Sprintf("i64.const %d", v)
		default:
			return "i64.const 0"
		}
	case lonemir.MirF32:
		if f, ok := val.(float64); ok {
			return fmt.Sprintf("f32.const %g", f)
		}
		return "f32.const 0"
	case lonemir.MirF64:
		if f, ok := val.(float64); ok {
			return fmt.Sprintf("f64.const %g", f)
		}
		return "f64.const 0"
	default:
		switch v := val.(type) {
		case int:
			return fmt.Sprintf("i32.const %d", v)
		case int32:
			return fmt.Sprintf("i32.const %d", v)
		case bool:
			if v {
				return "i32.const 1"
			}
			return "i32.const 0"
		default:
			return "i32.const 0"
		}
	}
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
					if call.Callee == "#print" || cleanIdentifier(call.Callee) == "__lone_rt_print" {
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

	regTypes := make(map[string]lonemir.MirT)

	// Parameters
	paramSet := make(map[string]bool)
	for _, p := range fn.Params {
		pName := cleanIdentifier(p.Name)
		paramSet[pName] = true
		regTypes[pName] = p.Type
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

	// Frame pointer local if function allocates on the linear datastack
	if fn.FrameSize > 0 {
		wfn.Locals = append(wfn.Locals, Local{
			Name: "$__fp",
			Type: TypeI32,
		})
	}

	// Locals (virtual registers minus parameters, sorted deterministically)
	var localNames []string
	for reg := range fn.Locals {
		localNames = append(localNames, reg)
	}
	sort.Strings(localNames)
	for _, reg := range localNames {
		typ := fn.Locals[reg]
		cleanReg := cleanIdentifier(reg)
		regTypes[cleanReg] = typ
		if !paramSet[cleanReg] {
			wfn.Locals = append(wfn.Locals, Local{
				Name: "$" + cleanReg,
				Type: toWasmType(typ),
			})
		}
	}

	// Instructions
	if len(fn.Blocks) > 0 {
		cl := newCfgLowerer(fn, regTypes)
		var body []WasmInst

		if fn.FrameSize > 0 {
			body = append(body,
				InstGlobalGet{Name: "$__stack_pointer"},
				InstConst{Value: int32(fn.FrameSize)},
				InstBinaryOp{Op: "i32.sub"},
				InstLocalTee{Name: "$__fp"},
				InstGlobalSet{Name: "$__stack_pointer"},
			)
		}

		body = append(body, cl.lowerRegion(fn.Blocks[0].Name, map[string]bool{})...)
		wfn.Body = body
	}

	return wfn, nil
}

type cfgLowerer struct {
	fn          *lonemir.MirFunction
	blockMap    map[string]*lonemir.MirBlock
	regTypes    map[string]lonemir.MirT
	loopHeads   map[string]bool
	loopExits   map[string]bool
}

func newCfgLowerer(fn *lonemir.MirFunction, regTypes map[string]lonemir.MirT) *cfgLowerer {
	bm := make(map[string]*lonemir.MirBlock, len(fn.Blocks))
	for _, b := range fn.Blocks {
		bm[b.Name] = b
	}
	return &cfgLowerer{
		fn:        fn,
		blockMap:  bm,
		regTypes:  regTypes,
		loopHeads: make(map[string]bool),
		loopExits: make(map[string]bool),
	}
}

func copySetWith(base map[string]bool, extras ...string) map[string]bool {
	out := make(map[string]bool, len(base)+len(extras))
	for k, v := range base {
		if v {
			out[k] = true
		}
	}
	for _, e := range extras {
		if e != "" {
			out[e] = true
		}
	}
	return out
}

func (cl *cfgLowerer) lowerRegion(startName string, stopTargets map[string]bool) []WasmInst {
	var insts []WasmInst
	curName := startName

	for curName != "" && !stopTargets[curName] {
		blk, ok := cl.blockMap[curName]
		if !ok {
			break
		}

		// Loop header block: wrap in structured (block $exit (loop $head ...))
		if blk.IsLoop || strings.HasPrefix(blk.Name, "while_head") {
			exitName := blk.LoopExit
			var condOp lonemir.MirOperand
			var bodyStart string
			if br, isBr := blk.Terminator.(lonemir.MirBranch); isBr {
				condOp = br.Cond
				bodyStart = br.ThenTarget
				if exitName == "" {
					exitName = br.ElseTarget
				}
			}

			loopHead := cleanIdentifier(blk.Name)
			loopExit := cleanIdentifier(exitName)

			var loopBody []WasmInst
			for _, inst := range blk.Insts {
				loopBody = append(loopBody, cl.lowerInstruction(inst)...)
			}
			if condOp != nil {
				loopBody = append(loopBody,
					cl.lowerLoadOperand(condOp, lonemir.MirI32)...,
				)
				loopBody = append(loopBody,
					InstRaw{Code: "i32.eqz"},
					InstBrIf{Target: "$" + loopExit},
				)
			}

			cl.loopHeads[blk.Name] = true
			if exitName != "" {
				cl.loopExits[exitName] = true
			}

			innerStops := copySetWith(stopTargets, blk.Name, exitName)
			bodyInsts := cl.lowerRegion(bodyStart, innerStops)
			loopBody = append(loopBody, bodyInsts...)
			loopBody = append(loopBody, InstBr{Target: "$" + loopHead})

			delete(cl.loopHeads, blk.Name)
			if exitName != "" {
				delete(cl.loopExits, exitName)
			}

			insts = append(insts, InstBlock{
				Label: "$" + loopExit,
				Body: []WasmInst{
					InstLoop{
						Label: "$" + loopHead,
						Body:  loopBody,
					},
				},
			})

			curName = exitName
			continue
		}

		insts = append(insts, InstRaw{Code: ";; label " + blk.Name})
		for _, inst := range blk.Insts {
			insts = append(insts, cl.lowerInstruction(inst)...)
		}

		if blk.Terminator == nil {
			break
		}

		switch t := blk.Terminator.(type) {
		case lonemir.MirReturn:
			if t.Value != nil {
				insts = append(insts, cl.lowerLoadOperand(t.Value, cl.fn.ReturnType)...)
			}
			if cl.fn.FrameSize > 0 {
				insts = append(insts,
					InstLocalGet{Name: "$__fp"},
					InstConst{Value: int32(cl.fn.FrameSize)},
					InstBinaryOp{Op: "i32.add"},
					InstGlobalSet{Name: "$__stack_pointer"},
				)
			}
			insts = append(insts, InstReturn{})
			return insts

		case lonemir.MirJump:
			if cl.loopExits[t.Target] {
				insts = append(insts, InstBr{Target: "$" + cleanIdentifier(t.Target)})
				return insts
			}
			if cl.loopHeads[t.Target] {
				if !stopTargets[t.Target] {
					insts = append(insts, InstBr{Target: "$" + cleanIdentifier(t.Target)})
				}
				return insts
			}
			if stopTargets[t.Target] {
				return insts
			}
			curName = t.Target

		case lonemir.MirBranch:
			insts = append(insts, cl.lowerLoadOperand(t.Cond, lonemir.MirI32)...)
			joinName := t.JoinTarget
			branchStops := copySetWith(stopTargets, joinName)
			thenInsts := cl.lowerRegion(t.ThenTarget, branchStops)
			var elseInsts []WasmInst
			if t.ElseTarget != "" && t.ElseTarget != joinName {
				elseInsts = cl.lowerRegion(t.ElseTarget, branchStops)
			}
			insts = append(insts, InstIf{
				Then: thenInsts,
				Else: elseInsts,
			})
			curName = joinName
		}
	}

	return insts
}

func (cl *cfgLowerer) regMirType(name string) lonemir.MirT {
	if t, ok := cl.regTypes[cleanIdentifier(name)]; ok {
		return t
	}
	return lonemir.MirI32
}

func (cl *cfgLowerer) lowerInstruction(inst lonemir.MirInst) []WasmInst {
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

		insts = append(insts, cl.lowerLoadOperand(n.Left, n.Type)...)
		insts = append(insts, cl.lowerLoadOperand(n.Right, n.Type)...)
		insts = append(insts, InstBinaryOp{Op: wasmOp})
		insts = append(insts, InstLocalSet{Name: "$" + cleanIdentifier(n.Dest)})

	case lonemir.MirCmpOp:
		wType := toWasmType(n.Type)
		isFloat := n.Type == lonemir.MirF32 || n.Type == lonemir.MirF64
		wasmOp := string(wType) + "." + n.Op
		switch n.Op {
		case "lt":
			if isFloat {
				wasmOp = string(wType) + ".lt"
			} else {
				wasmOp = string(wType) + ".lt_s"
			}
		case "gt":
			if isFloat {
				wasmOp = string(wType) + ".gt"
			} else {
				wasmOp = string(wType) + ".gt_s"
			}
		case "le":
			if isFloat {
				wasmOp = string(wType) + ".le"
			} else {
				wasmOp = string(wType) + ".le_s"
			}
		case "ge":
			if isFloat {
				wasmOp = string(wType) + ".ge"
			} else {
				wasmOp = string(wType) + ".ge_s"
			}
		case "eq":
			wasmOp = string(wType) + ".eq"
		case "ne":
			wasmOp = string(wType) + ".ne"
		}
		insts = append(insts, cl.lowerLoadOperand(n.Left, n.Type)...)
		insts = append(insts, cl.lowerLoadOperand(n.Right, n.Type)...)
		insts = append(insts, InstBinaryOp{Op: wasmOp})
		insts = append(insts, InstLocalSet{Name: "$" + cleanIdentifier(n.Dest)})

	case lonemir.MirConstInst:
		insts = append(insts, lowerConstant(n.Value, n.Type)...)
		insts = append(insts, InstLocalSet{Name: "$" + cleanIdentifier(n.Dest)})

	case lonemir.MirCallInst:
		callee := cleanIdentifier(n.Callee)
		if n.Callee == "#print" || callee == "__lone_rt_print" {
			callee = "__lone_rt_print"
		}
		for _, arg := range n.Args {
			argType := lonemir.MirI32
			if r, ok := arg.(lonemir.MirReg); ok {
				argType = cl.regMirType(r.Name)
			} else if c, ok := arg.(lonemir.MirConst); ok && c.Type != lonemir.MirVoid {
				argType = c.Type
			}
			insts = append(insts, cl.lowerLoadOperand(arg, argType)...)
		}
		insts = append(insts, InstCall{Target: "$" + callee})
		if n.Dest != "" {
			insts = append(insts, InstLocalSet{Name: "$" + cleanIdentifier(n.Dest)})
		}

	case lonemir.MirCopyInst:
		destType := cl.regMirType(n.Dest)
		insts = append(insts, cl.lowerLoadOperand(n.Src, destType)...)
		insts = append(insts, InstLocalSet{Name: "$" + cleanIdentifier(n.Dest)})

	case lonemir.MirSlotAddr:
		insts = append(insts, InstLocalGet{Name: "$__fp"})
		if n.Offset != 0 {
			insts = append(insts,
				InstConst{Value: int32(n.Offset)},
				InstBinaryOp{Op: "i32.add"},
			)
		}
		insts = append(insts, InstLocalSet{Name: "$" + cleanIdentifier(n.Dest)})

	case lonemir.MirLoad:
		insts = append(insts, cl.lowerLoadOperand(n.Addr, lonemir.MirI32)...)
		op := wasmLoadOp(n.Type, n.Size, n.Signed)
		insts = append(insts, InstLoad{Op: op, Offset: n.Offset})
		insts = append(insts, InstLocalSet{Name: "$" + cleanIdentifier(n.Dest)})

	case lonemir.MirStore:
		insts = append(insts, cl.lowerLoadOperand(n.Addr, lonemir.MirI32)...)
		insts = append(insts, cl.lowerLoadOperand(n.Value, n.Type)...)
		op := wasmStoreOp(n.Type, n.Size)
		insts = append(insts, InstStore{Op: op, Offset: n.Offset})

	case lonemir.MirGlobalSet:
		insts = append(insts, cl.lowerLoadOperand(n.Value, n.Type)...)
		insts = append(insts, InstGlobalSet{Name: "$" + cleanIdentifier(n.Name)})

	case lonemir.MirHeapAlloc:
		destName := "$" + cleanIdentifier(n.Dest)
		insts = append(insts,
			InstGlobalGet{Name: "$__heap_pointer"},
			InstLocalSet{Name: destName},
			InstGlobalGet{Name: "$__heap_pointer"},
		)
		insts = append(insts, cl.lowerLoadOperand(n.Size, lonemir.MirI32)...)
		insts = append(insts,
			InstConst{Value: 7},
			InstBinaryOp{Op: "i32.add"},
			InstConst{Value: -8},
			InstBinaryOp{Op: "i32.and"},
			InstBinaryOp{Op: "i32.add"},
			InstGlobalSet{Name: "$__heap_pointer"},
		)
	}

	return insts
}

func wasmLoadOp(typ lonemir.MirT, size int, signed bool) string {
	switch typ {
	case lonemir.MirI64:
		return "i64.load"
	case lonemir.MirF32:
		return "f32.load"
	case lonemir.MirF64:
		return "f64.load"
	default:
		switch size {
		case 1:
			if signed {
				return "i32.load8_s"
			}
			return "i32.load8_u"
		case 2:
			if signed {
				return "i32.load16_s"
			}
			return "i32.load16_u"
		default:
			return "i32.load"
		}
	}
}

func wasmStoreOp(typ lonemir.MirT, size int) string {
	switch typ {
	case lonemir.MirI64:
		return "i64.store"
	case lonemir.MirF32:
		return "f32.store"
	case lonemir.MirF64:
		return "f64.store"
	default:
		switch size {
		case 1:
			return "i32.store8"
		case 2:
			return "i32.store16"
		default:
			return "i32.store"
		}
	}
}

func (cl *cfgLowerer) lowerLoadOperand(op lonemir.MirOperand, typ lonemir.MirT) []WasmInst {
	switch o := op.(type) {
	case lonemir.MirReg:
		return []WasmInst{InstLocalGet{Name: "$" + cleanIdentifier(o.Name)}}
	case lonemir.MirGlobal:
		return []WasmInst{InstGlobalGet{Name: "$" + cleanIdentifier(o.Name)}}
	case lonemir.MirConst:
		cType := typ
		if o.Type != lonemir.MirVoid && cType == lonemir.MirVoid {
			cType = o.Type
		}
		return lowerConstant(o.Value, cType)
	default:
		return nil
	}
}

func lowerConstant(val any, typ lonemir.MirT) []WasmInst {
	switch typ {
	case lonemir.MirI64:
		switch v := val.(type) {
		case int:
			return []WasmInst{InstRaw{Code: fmt.Sprintf("i64.const %d", v)}}
		case int64:
			return []WasmInst{InstRaw{Code: fmt.Sprintf("i64.const %d", v)}}
		default:
			return []WasmInst{InstRaw{Code: "i64.const 0"}}
		}
	case lonemir.MirF32:
		switch v := val.(type) {
		case float64:
			return []WasmInst{InstRaw{Code: fmt.Sprintf("f32.const %g", v)}}
		case int:
			return []WasmInst{InstRaw{Code: fmt.Sprintf("f32.const %d", v)}}
		default:
			return []WasmInst{InstRaw{Code: "f32.const 0"}}
		}
	case lonemir.MirF64:
		switch v := val.(type) {
		case float64:
			return []WasmInst{InstRaw{Code: fmt.Sprintf("f64.const %g", v)}}
		case int:
			return []WasmInst{InstRaw{Code: fmt.Sprintf("f64.const %d", v)}}
		default:
			return []WasmInst{InstRaw{Code: "f64.const 0"}}
		}
	}

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
	name = strings.ReplaceAll(name, "#", "_")
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
