package wir

import (
	"lone/pkg/mir"
	"fmt"
	"strings"
)

// Target represents the compilation target for WebAssembly output.
type Target string

const (
	TargetWasi Target = "wasi"
	TargetWeb  Target = "web"
)

// TranspileToWat converts a Falone MirModule into WebAssembly Text format (WAT) targeting WASI.
func TranspileToWat(mod *mir.MirModule) (string, error) {
	return TranspileToWatWithTarget(mod, TargetWasi)
}

// TranspileToWatWithTarget converts a Falone MirModule into WebAssembly Text format for a specific target.
func TranspileToWatWithTarget(mod *mir.MirModule, target Target) (string, error) {
	if mod == nil {
		return "(module)", nil
	}

	if target == "" {
		target = TargetWasi
	}

	var sb strings.Builder
	sb.WriteString("(module\n")

	needsPrint := usesPrintIntrinsic(mod)

	// Target-specific imports
	if needsPrint {
		if target == TargetWeb {
			sb.WriteString("  (import \"env\" \"print\" (func $__lone_rt_print (param i32 i32)))\n")
		} else {
			// WASI snapshot preview 1 fd_write import
			sb.WriteString("  (import \"wasi_snapshot_preview1\" \"fd_write\" (func $__wasi_fd_write (param i32 i32 i32 i32) (result i32)))\n")
		}
	}

	sb.WriteString("  (memory (export \"memory\") 1)\n")

	// Linear memory data segments (string literals)
	for _, seg := range mod.DataSegments {
		sb.WriteString(fmt.Sprintf("  (data (i32.const %d) \"%s\")\n", seg.Offset, escapeWatString(seg.Value)))
	}

	// Global variables
	for name, typ := range mod.Globals {
		gName := cleanIdentifier(name)
		wType := toWasmType(typ)
		sb.WriteString(fmt.Sprintf("  (global $%s (mut %s) (%s.const 0))\n", gName, wType, wType))
	}

	// Inlined runtime built-in functions
	if needsPrint && target != TargetWeb {
		sb.WriteString("\n  ;; Inlined WASI #print runtime implementation\n")
		sb.WriteString("  (func $__lone_rt_print (param $ptr i32) (param $len i32)\n")
		sb.WriteString("    (i32.store (i32.const 0) (local.get $ptr))\n")
		sb.WriteString("    (i32.store (i32.const 4) (local.get $len))\n")
		sb.WriteString("    (drop (call $__wasi_fd_write (i32.const 1) (i32.const 0) (i32.const 1) (i32.const 8)))\n")
		sb.WriteString("  )\n")
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

func usesPrintIntrinsic(mod *mir.MirModule) bool {
	if mod == nil {
		return false
	}
	for _, fn := range mod.Functions {
		for _, blk := range fn.Blocks {
			for _, inst := range blk.Insts {
				if call, ok := inst.(mir.MirCallInst); ok {
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

func escapeWatString(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == '"' {
			sb.WriteString(`\"`)
		} else if b == '\\' {
			sb.WriteString(`\\`)
		} else if b >= 32 && b <= 126 {
			sb.WriteByte(b)
		} else {
			sb.WriteString(fmt.Sprintf("\\%02x", b))
		}
	}
	// Append trailing null-terminator for C interoperability
	sb.WriteString("\\00")
	return sb.String()
}

func transpileFunction(fn *mir.MirFunction) (string, error) {
	fnName := cleanIdentifier(fn.Name)
	var sb strings.Builder

	// Export main by default or if function is exported
	exportStr := ""
	if fnName == "main" {
		exportStr = " (export \"_start\") (export \"main\")"
	}

	sb.WriteString(fmt.Sprintf("  (func $%s%s", fnName, exportStr))

	// Parameters
	for _, p := range fn.Params {
		pName := cleanIdentifier(p.Name)
		sb.WriteString(fmt.Sprintf(" (param $%s %s)", pName, toWasmType(p.Type)))
	}

	// Return type
	if fn.ReturnType != mir.MirVoid {
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

	if hasLoop(fn.Blocks) {
		emitFunctionBlocks(&sb, fn.Blocks, 2)
	} else if len(fn.Blocks) == 1 {
		// Single block
		emitBlockBody(&sb, fn.Blocks[0], 2)
	} else {
		// Multi-block forward control flow
		emitStructuredBlocks(&sb, fn.Blocks, 2)
	}

	sb.WriteString("  )")
	return sb.String(), nil
}

func hasLoop(blocks []*mir.MirBlock) bool {
	for _, b := range blocks {
		if b.IsLoop || strings.HasPrefix(b.Name, "while_head") {
			return true
		}
	}
	return false
}

func emitFunctionBlocks(sb *strings.Builder, blocks []*mir.MirBlock, indent int) {
	pad := strings.Repeat("  ", indent)

	for i := 0; i < len(blocks); i++ {
		blk := blocks[i]

		// Check if this block is a loop header
		if blk.IsLoop || strings.HasPrefix(blk.Name, "while_head") {
			exitName := blk.LoopExit
			if exitName == "" {
				// Infer exit name from branch else target
				if br, ok := blk.Terminator.(mir.MirBranch); ok {
					exitName = br.ElseTarget
				}
			}
			loopHead := cleanIdentifier(blk.Name)
			loopExit := cleanIdentifier(exitName)

			// Find exit block index
			exitIdx := -1
			for j := i + 1; j < len(blocks); j++ {
				if blocks[j].Name == exitName {
					exitIdx = j
					break
				}
			}

			sb.WriteString(fmt.Sprintf("%s(block $%s\n", pad, loopExit))
			sb.WriteString(fmt.Sprintf("%s  (loop $%s\n", pad, loopHead))

			// 1. Evaluate condition
			if len(blk.Insts) > 0 {
				sb.WriteString(fmt.Sprintf("%s    ;; Evaluate condition\n", pad))
				for _, inst := range blk.Insts {
					emitInstruction(sb, inst, indent+2)
				}
			}

			// 2. Exit if condition is false
			if br, ok := blk.Terminator.(mir.MirBranch); ok {
				sb.WriteString(fmt.Sprintf("%s    ;; Exit if condition is false\n", pad))
				emitLoadOperand(sb, br.Cond, mir.MirI32, indent+2)
				sb.WriteString(fmt.Sprintf("%s    i32.eqz\n", pad))
				sb.WriteString(fmt.Sprintf("%s    br_if $%s\n\n", pad, loopExit))
			}

			// 3. Body
			bodyBlocksEnd := len(blocks)
			if exitIdx != -1 {
				bodyBlocksEnd = exitIdx
			}

			sb.WriteString(fmt.Sprintf("%s    ;; Body\n", pad))
			repeated := false
			for j := i + 1; j < bodyBlocksEnd; j++ {
				bodyBlk := blocks[j]
				for _, inst := range bodyBlk.Insts {
					emitInstruction(sb, inst, indent+2)
				}
				if jmp, ok := bodyBlk.Terminator.(mir.MirJump); ok {
					target := cleanIdentifier(jmp.Target)
					if target == loopExit {
						sb.WriteString(fmt.Sprintf("%s    br $%s\n", pad, loopExit))
					} else if target == loopHead {
						sb.WriteString(fmt.Sprintf("%s    ;; Repeat\n", pad))
						sb.WriteString(fmt.Sprintf("%s    br $%s\n", pad, loopHead))
						repeated = true
					}
				} else if ret, ok := bodyBlk.Terminator.(mir.MirReturn); ok {
					emitTerminator(sb, ret, indent+2)
				}
			}

			if !repeated {
				sb.WriteString(fmt.Sprintf("%s    ;; Repeat\n", pad))
				sb.WriteString(fmt.Sprintf("%s    br $%s\n", pad, loopHead))
			}

			sb.WriteString(fmt.Sprintf("%s  )\n", pad))
			sb.WriteString(fmt.Sprintf("%s)\n\n", pad))

			if exitIdx != -1 {
				i = exitIdx - 1
			} else {
				i = bodyBlocksEnd - 1
			}
			continue
		}

		// Non-loop block
		sb.WriteString(fmt.Sprintf("%s;; label %s\n", pad, blk.Name))
		for _, inst := range blk.Insts {
			emitInstruction(sb, inst, indent)
		}

		// Only emit terminator if it's not a jump to the immediately following block
		if blk.Terminator != nil {
			if jmp, ok := blk.Terminator.(mir.MirJump); ok {
				if i+1 < len(blocks) && cleanIdentifier(jmp.Target) == cleanIdentifier(blocks[i+1].Name) {
					// Fallthrough to next block, no need for br
					continue
				}
			}
			emitTerminator(sb, blk.Terminator, indent)
		}
	}
}

func emitStructuredBlocks(sb *strings.Builder, blocks []*mir.MirBlock, indent int) {
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

func emitBlockBody(sb *strings.Builder, blk *mir.MirBlock, indent int) {
	pad := strings.Repeat("  ", indent)
	sb.WriteString(fmt.Sprintf("%s;; label %s\n", pad, blk.Name))

	for _, inst := range blk.Insts {
		emitInstruction(sb, inst, indent)
	}

	if blk.Terminator != nil {
		emitTerminator(sb, blk.Terminator, indent)
	}
}

func emitInstruction(sb *strings.Builder, inst mir.MirInst, indent int) {
	pad := strings.Repeat("  ", indent)

	switch n := inst.(type) {
	case mir.MirBinOp:
		wType := toWasmType(n.Type)
		wasmOp := wType + "." + n.Op
		if n.Op == "div" {
			if n.Type == mir.MirF32 || n.Type == mir.MirF64 {
				wasmOp = wType + ".div"
			} else {
				wasmOp = wType + ".div_s"
			}
		}

		emitLoadOperand(sb, n.Left, n.Type, indent)
		emitLoadOperand(sb, n.Right, n.Type, indent)
		sb.WriteString(fmt.Sprintf("%s%s\n", pad, wasmOp))
		sb.WriteString(fmt.Sprintf("%slocal.set $%s\n", pad, cleanIdentifier(n.Dest)))

	case mir.MirCmpOp:
		wType := toWasmType(n.Type)
		var wasmCmp string
		switch n.Op {
		case "lt":
			if n.Type == mir.MirF32 || n.Type == mir.MirF64 {
				wasmCmp = wType + ".lt"
			} else {
				wasmCmp = wType + ".lt_s"
			}
		case "gt":
			if n.Type == mir.MirF32 || n.Type == mir.MirF64 {
				wasmCmp = wType + ".gt"
			} else {
				wasmCmp = wType + ".gt_s"
			}
		case "le":
			if n.Type == mir.MirF32 || n.Type == mir.MirF64 {
				wasmCmp = wType + ".le"
			} else {
				wasmCmp = wType + ".le_s"
			}
		case "ge":
			if n.Type == mir.MirF32 || n.Type == mir.MirF64 {
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

	case mir.MirConstInst:
		wType := toWasmType(n.Type)
		sb.WriteString(fmt.Sprintf("%s%s.const %v\n", pad, wType, n.Value))
		sb.WriteString(fmt.Sprintf("%slocal.set $%s\n", pad, cleanIdentifier(n.Dest)))

	case mir.MirCopyInst:
		emitLoadOperand(sb, n.Src, mir.MirI32, indent)
		sb.WriteString(fmt.Sprintf("%slocal.set $%s\n", pad, cleanIdentifier(n.Dest)))

	case mir.MirCallInst:
		callee := cleanIdentifier(n.Callee)
		for _, arg := range n.Args {
			emitLoadOperand(sb, arg, mir.MirI32, indent)
		}
		sb.WriteString(fmt.Sprintf("%scall $%s\n", pad, callee))
		if n.Dest != "" {
			sb.WriteString(fmt.Sprintf("%slocal.set $%s\n", pad, cleanIdentifier(n.Dest)))
		}
	}
}

func emitTerminator(sb *strings.Builder, term mir.MirTerminator, indent int) {
	pad := strings.Repeat("  ", indent)

	switch t := term.(type) {
	case mir.MirReturn:
		if t.Value != nil {
			emitLoadOperand(sb, t.Value, mir.MirI32, indent)
		}
		sb.WriteString(fmt.Sprintf("%sreturn\n", pad))

	case mir.MirJump:
		sb.WriteString(fmt.Sprintf("%sbr $%s\n", pad, cleanIdentifier(t.Target)))

	case mir.MirBranch:
		emitLoadOperand(sb, t.Cond, mir.MirI32, indent)
		sb.WriteString(fmt.Sprintf("%sbr_if $%s\n", pad, cleanIdentifier(t.ThenTarget)))
		sb.WriteString(fmt.Sprintf("%sbr $%s\n", pad, cleanIdentifier(t.ElseTarget)))
	}
}

func emitLoadOperand(sb *strings.Builder, op mir.MirOperand, defaultType mir.MirT, indent int) {
	pad := strings.Repeat("  ", indent)
	switch o := op.(type) {
	case mir.MirReg:
		sb.WriteString(fmt.Sprintf("%slocal.get $%s\n", pad, cleanIdentifier(o.Name)))
	case mir.MirConst:
		wType := toWasmType(o.Type)
		if wType == "unknown" || wType == "void" {
			wType = toWasmType(defaultType)
		}
		sb.WriteString(fmt.Sprintf("%s%s.const %v\n", pad, wType, o.Value))
	case mir.MirGlobal:
		sb.WriteString(fmt.Sprintf("%sglobal.get $%s\n", pad, cleanIdentifier(o.Name)))
	}
}

func toWasmType(t mir.MirT) string {
	switch t {
	case mir.MirI32:
		return "i32"
	case mir.MirI64:
		return "i64"
	case mir.MirF32:
		return "f32"
	case mir.MirF64:
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
