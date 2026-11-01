package transpile

import (
	"falone/mirtypes"
	"fmt"
	"strings"
)

// TranspileToWat converts a Falone MirModule into WebAssembly Text format (WAT).
func TranspileToWat(mod *mirtypes.MirModule) (string, error) {
	if mod == nil {
		return "(module)", nil
	}

	var sb strings.Builder
	sb.WriteString("(module\n")
	sb.WriteString("  (memory (export \"memory\") 1)\n")

	// Global variables
	for name, typ := range mod.Globals {
		gName := cleanIdentifier(name)
		wType := toWasmType(typ)
		sb.WriteString(fmt.Sprintf("  (global $%s (mut %s) (%s.const 0))\n", gName, wType, wType))
	}

	// Functions
	for _, fn := range mod.Functions {
		fnWat, err := transpileFunction(fn)
		if err != nil {
			return "", err
		}
		sb.WriteString("\n" + fnWat + "\n")
	}

	sb.WriteString(")\n")
	return sb.String(), nil
}

func transpileFunction(fn *mirtypes.MirFunction) (string, error) {
	fnName := cleanIdentifier(fn.Name)
	var sb strings.Builder

	// Export main by default or if function is exported
	exportStr := ""
	if fnName == "main" {
		exportStr = " (export \"main\")"
	}

	sb.WriteString(fmt.Sprintf("  (func $%s%s", fnName, exportStr))

	// Parameters
	for _, p := range fn.Params {
		pName := cleanIdentifier(p.Name)
		sb.WriteString(fmt.Sprintf(" (param $%s %s)", pName, toWasmType(p.Type)))
	}

	// Return type
	if fn.ReturnType != mirtypes.MirVoid {
		sb.WriteString(fmt.Sprintf(" (result %s)", toWasmType(fn.ReturnType)))
	}
	sb.WriteString("\n")

	// Collect local variables: all registers in fn.Locals not present in params
	paramSet := make(map[string]bool)
	for _, p := range fn.Params {
		paramSet[cleanIdentifier(p.Name)] = true
	}

	for reg, typ := range fn.Locals {
		cleanReg := cleanIdentifier(reg)
		if !paramSet[cleanReg] {
			sb.WriteString(fmt.Sprintf("    (local $%s %s)\n", cleanReg, toWasmType(typ)))
		}
	}

	// Emit basic blocks
	if len(fn.Blocks) == 0 {
		sb.WriteString("  )")
		return sb.String(), nil
	}

	if len(fn.Blocks) == 1 {
		// Single block
		emitBlockBody(&sb, fn.Blocks[0], 2)
	} else {
		// Multi-block control flow: wrap forward blocks in nested WebAssembly block labels
		emitStructuredBlocks(&sb, fn.Blocks, 2)
	}

	sb.WriteString("  )")
	return sb.String(), nil
}

func emitStructuredBlocks(sb *strings.Builder, blocks []*mirtypes.MirBlock, indent int) {
	// If there are N blocks, open block labels for blocks 1..N-1
	// e.g. for [entry, if_block, exit]
	// (block $exit
	//   (block $if_block
	//     entry insts...
	//   )
	//   if_block insts...
	// )
	// exit insts...

	total := len(blocks)
	for i := total - 1; i >= 1; i-- {
		blkName := cleanIdentifier(blocks[i].Name)
		sb.WriteString(fmt.Sprintf("%s(block $%s\n", strings.Repeat("  ", indent+total-1-i), blkName))
	}

	currentIndent := indent + total - 1
	for i, blk := range blocks {
		emitBlockBody(sb, blk, currentIndent)
		if i < total-1 {
			currentIndent--
			sb.WriteString(fmt.Sprintf("%s)\n", strings.Repeat("  ", currentIndent)))
		}
	}
}

func emitBlockBody(sb *strings.Builder, blk *mirtypes.MirBlock, indent int) {
	pad := strings.Repeat("  ", indent)
	sb.WriteString(fmt.Sprintf("%s;; label %s\n", pad, blk.Name))

	for _, inst := range blk.Insts {
		emitInstruction(sb, inst, indent)
	}

	if blk.Terminator != nil {
		emitTerminator(sb, blk.Terminator, indent)
	}
}

func emitInstruction(sb *strings.Builder, inst mirtypes.MirInst, indent int) {
	pad := strings.Repeat("  ", indent)

	switch n := inst.(type) {
	case mirtypes.MirBinOp:
		wType := toWasmType(n.Type)
		wasmOp := wType + "." + n.Op
		if n.Op == "div" {
			if n.Type == mirtypes.MirF32 || n.Type == mirtypes.MirF64 {
				wasmOp = wType + ".div"
			} else {
				wasmOp = wType + ".div_s"
			}
		}

		emitLoadOperand(sb, n.Left, n.Type, indent)
		emitLoadOperand(sb, n.Right, n.Type, indent)
		sb.WriteString(fmt.Sprintf("%s%s\n", pad, wasmOp))
		sb.WriteString(fmt.Sprintf("%slocal.set $%s\n", pad, cleanIdentifier(n.Dest)))

	case mirtypes.MirCmpOp:
		wType := toWasmType(n.Type)
		var wasmCmp string
		switch n.Op {
		case "lt":
			if n.Type == mirtypes.MirF32 || n.Type == mirtypes.MirF64 {
				wasmCmp = wType + ".lt"
			} else {
				wasmCmp = wType + ".lt_s"
			}
		case "gt":
			if n.Type == mirtypes.MirF32 || n.Type == mirtypes.MirF64 {
				wasmCmp = wType + ".gt"
			} else {
				wasmCmp = wType + ".gt_s"
			}
		case "le":
			if n.Type == mirtypes.MirF32 || n.Type == mirtypes.MirF64 {
				wasmCmp = wType + ".le"
			} else {
				wasmCmp = wType + ".le_s"
			}
		case "ge":
			if n.Type == mirtypes.MirF32 || n.Type == mirtypes.MirF64 {
				wasmCmp = wType + ".ge"
			} else {
				wasmCmp = wType + ".ge_s"
			}
		case "eq":
			wasmCmp = wType + ".eq"
		case "ne":
			wasmCmp = wType + ".ne"
		default:
			wasmCmp = wType + ".eq"
		}

		emitLoadOperand(sb, n.Left, n.Type, indent)
		emitLoadOperand(sb, n.Right, n.Type, indent)
		sb.WriteString(fmt.Sprintf("%s%s\n", pad, wasmCmp))
		sb.WriteString(fmt.Sprintf("%slocal.set $%s\n", pad, cleanIdentifier(n.Dest)))

	case mirtypes.MirConstInst:
		wType := toWasmType(n.Type)
		sb.WriteString(fmt.Sprintf("%s%s.const %v\n", pad, wType, n.Value))
		sb.WriteString(fmt.Sprintf("%slocal.set $%s\n", pad, cleanIdentifier(n.Dest)))

	case mirtypes.MirCopyInst:
		emitLoadOperand(sb, n.Src, mirtypes.MirI32, indent)
		sb.WriteString(fmt.Sprintf("%slocal.set $%s\n", pad, cleanIdentifier(n.Dest)))

	case mirtypes.MirCallInst:
		callee := cleanIdentifier(n.Callee)
		for _, arg := range n.Args {
			emitLoadOperand(sb, arg, mirtypes.MirI32, indent)
		}
		sb.WriteString(fmt.Sprintf("%scall $%s\n", pad, callee))
		if n.Dest != "" {
			sb.WriteString(fmt.Sprintf("%slocal.set $%s\n", pad, cleanIdentifier(n.Dest)))
		}
	}
}

func emitTerminator(sb *strings.Builder, term mirtypes.MirTerminator, indent int) {
	pad := strings.Repeat("  ", indent)

	switch t := term.(type) {
	case mirtypes.MirReturn:
		if t.Value != nil {
			emitLoadOperand(sb, t.Value, mirtypes.MirI32, indent)
		}
		sb.WriteString(fmt.Sprintf("%sreturn\n", pad))

	case mirtypes.MirJump:
		sb.WriteString(fmt.Sprintf("%sbr $%s\n", pad, cleanIdentifier(t.Target)))

	case mirtypes.MirBranch:
		emitLoadOperand(sb, t.Cond, mirtypes.MirI32, indent)
		sb.WriteString(fmt.Sprintf("%sbr_if $%s\n", pad, cleanIdentifier(t.ThenTarget)))
		sb.WriteString(fmt.Sprintf("%sbr $%s\n", pad, cleanIdentifier(t.ElseTarget)))
	}
}

func emitLoadOperand(sb *strings.Builder, op mirtypes.MirOperand, defaultType mirtypes.MirT, indent int) {
	pad := strings.Repeat("  ", indent)
	switch o := op.(type) {
	case mirtypes.MirReg:
		sb.WriteString(fmt.Sprintf("%slocal.get $%s\n", pad, cleanIdentifier(o.Name)))
	case mirtypes.MirConst:
		wType := toWasmType(o.Type)
		if wType == "unknown" || wType == "void" {
			wType = toWasmType(defaultType)
		}
		sb.WriteString(fmt.Sprintf("%s%s.const %v\n", pad, wType, o.Value))
	case mirtypes.MirGlobal:
		sb.WriteString(fmt.Sprintf("%sglobal.get $%s\n", pad, cleanIdentifier(o.Name)))
	}
}

func toWasmType(t mirtypes.MirT) string {
	switch t {
	case mirtypes.MirI32:
		return "i32"
	case mirtypes.MirI64:
		return "i64"
	case mirtypes.MirF32:
		return "f32"
	case mirtypes.MirF64:
		return "f64"
	default:
		return "i32"
	}
}

func cleanIdentifier(name string) string {
	name = strings.TrimPrefix(name, "@")
	name = strings.TrimPrefix(name, "%")
	name = strings.TrimPrefix(name, "$")
	return name
}
