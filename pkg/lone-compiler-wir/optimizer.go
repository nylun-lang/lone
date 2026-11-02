package lonewir

// Optimizer performs peephole optimizations and dead instruction elimination on a WasmModule AST.
type Optimizer struct{}

// NewOptimizer returns a new Optimizer instance.
func NewOptimizer() *Optimizer {
	return &Optimizer{}
}

// OptimizeModule optimizes all functions in the given WasmModule.
func (opt *Optimizer) OptimizeModule(mod *WasmModule) {
	if mod == nil {
		return
	}
	for _, fn := range mod.Functions {
		fn.Body = opt.OptimizeInsts(fn.Body)
	}
}

// OptimizeInsts recursively cleans and optimizes a slice of instructions.
func (opt *Optimizer) OptimizeInsts(insts []WasmInst) []WasmInst {
	if len(insts) == 0 {
		return insts
	}

	var optimized []WasmInst

	for i := 0; i < len(insts); i++ {
		curr := insts[i]

		// 1. Strip pure nop instructions
		if _, isNop := curr.(InstNop); isNop {
			continue
		}

		// 2. Peephole: Suppress naive constant followed immediately by drop (e.g. i32.const -> drop)
		if i+1 < len(insts) {
			next := insts[i+1]
			if curr.IsPure() {
				if _, isDrop := next.(InstDrop); isDrop {
					// Both the pure instruction and the drop can be suppressed!
					i++ // Skip next (the drop)
					continue
				}
			}
		}

		// 3. Recurse into structured blocks, loops, and conditionals
		switch block := curr.(type) {
		case InstBlock:
			block.Body = opt.OptimizeInsts(block.Body)
			curr = block
		case InstLoop:
			block.Body = opt.OptimizeInsts(block.Body)
			curr = block
		case InstIf:
			block.Then = opt.OptimizeInsts(block.Then)
			block.Else = opt.OptimizeInsts(block.Else)
			curr = block
		}

		optimized = append(optimized, curr)
	}

	return optimized
}
