package lonewir

// Optimizer applies safe peephole optimizations to a WebAssembly AST.
type Optimizer struct{}

// NewOptimizer creates an Optimizer instance.
func NewOptimizer() *Optimizer {
	return &Optimizer{}
}

// OptimizeModule runs optimization passes over all functions in the module.
func (opt *Optimizer) OptimizeModule(mod *WasmModule) {
	if mod == nil {
		return
	}
	for _, fn := range mod.Functions {
		fn.Body = opt.OptimizeInsts(fn.Body)
	}
}

// OptimizeInsts optimizes a slice of instructions.
func (opt *Optimizer) OptimizeInsts(insts []WasmInst) []WasmInst {
	var result []WasmInst

	for i := 0; i < len(insts); i++ {
		inst := insts[i]

		// Recurse into structured blocks
		switch b := inst.(type) {
		case InstBlock:
			b.Body = opt.OptimizeInsts(b.Body)
			inst = b
		case InstLoop:
			b.Body = opt.OptimizeInsts(b.Body)
			inst = b
		case InstIf:
			b.Then = opt.OptimizeInsts(b.Then)
			b.Else = opt.OptimizeInsts(b.Else)
			inst = b
		}

		// 1. Strip Nop
		if _, isNop := inst.(InstNop); isNop {
			continue
		}

		// 2. Suppress pure const followed by drop
		if i+1 < len(insts) {
			_, nextIsDrop := insts[i+1].(InstDrop)
			if nextIsDrop && isConstInst(inst) {
				i++ // Skip both const and drop
				continue
			}
		}

		// 3. local.set $x; local.get $x -> local.tee $x
		if i+1 < len(insts) {
			setInst, isSet := inst.(InstLocalSet)
			getInst, isGet := insts[i+1].(InstLocalGet)
			if isSet && isGet && setInst.Name == getInst.Name {
				result = append(result, InstLocalTee{Name: setInst.Name})
				i++ // Skip get
				continue
			}
		}

		result = append(result, inst)

		// 4. Eliminate dead code after unconditional return or br
		if _, isRet := inst.(InstReturn); isRet {
			break
		}
		if _, isBr := inst.(InstBr); isBr {
			break
		}
		if _, isUnreachable := inst.(InstUnreachable); isUnreachable {
			break
		}
	}

	return result
}

func isConstInst(inst WasmInst) bool {
	switch inst.(type) {
	case InstConstI32, InstConstI64, InstConstF32, InstConstF64:
		return true
	default:
		return false
	}
}
