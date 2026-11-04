package lonewir

import (
	"fmt"

	loneanalyzer "lone/pkg2/lone-analyzer"
	loneast "lone/pkg2/lone-parser-ast"
)

// LowerOptions configures the code generation behavior.
type LowerOptions struct {
	Target       string // "wasi" (default) or "web"
	MemoryPages  int
	MemoryExport string
}

// DefaultLowerOptions returns the default compilation target options.
func DefaultLowerOptions() LowerOptions {
	return LowerOptions{
		Target:       "wasi",
		MemoryPages:  2, // 128 KiB
		MemoryExport: "memory",
	}
}

type loopContext struct {
	label    string
	breakLbl string
	contLbl  string
}

type stringEntry struct {
	offset int
	length int
}

type lowerer struct {
	opts         LowerOptions
	mod          *WasmModule
	strings      map[string]stringEntry
	stringVarLen map[string]int
	nextOffset   int
	loopStack    []loopContext
	labelStack   map[string]string // user label -> wasm block label
	labelId      int
	usesPrint    bool
	usesStrlen   bool
}

// LowerProgram compiles a typed program AST directly into a structured WebAssembly AST.
func LowerProgram(prog *loneanalyzer.TProgram, opts LowerOptions) (*WasmModule, error) {
	if opts.MemoryPages <= 0 {
		opts.MemoryPages = 2
	}
	if opts.MemoryExport == "" {
		opts.MemoryExport = "memory"
	}

	l := &lowerer{
		opts:         opts,
		mod:          NewModule(),
		strings:      make(map[string]stringEntry),
		stringVarLen: make(map[string]int),
		nextOffset:   64, // [0..64) reserved for WASI iovec / runtime scratchpad
		labelStack:   make(map[string]string),
	}
	l.mod.MemoryPages = opts.MemoryPages
	l.mod.MemoryExport = opts.MemoryExport

	// Pass 1: Collect globals and functions
	var fnDecls []*loneanalyzer.TFnDec
	for _, decl := range prog.Decls {
		switch d := decl.(type) {
		case *loneanalyzer.TVarDec:
			wType := toWasmType(d.Type)
			initVal := l.formatGlobalInit(d.Value, d.Type)
			l.mod.Globals = append(l.mod.Globals, Global{
				Name:    "$g_" + d.Name,
				Type:    wType,
				Mutable: d.IsMutable,
				InitVal: initVal,
			})
		case *loneanalyzer.TFnDec:
			fnDecls = append(fnDecls, d)
		case *loneanalyzer.TImplDec:
			for _, m := range d.Methods {
				fnDecls = append(fnDecls, m)
			}
		}
	}

	// Pass 2: Lower functions
	for _, fn := range fnDecls {
		wfn, err := l.lowerFunction(fn)
		if err != nil {
			return nil, err
		}
		l.mod.AddFunction(wfn)
	}

	// Pass 3: Setup runtime imports and helpers
	if l.usesPrint {
		if opts.Target == "web" {
			l.mod.Imports = append(l.mod.Imports, Import{
				Module: "env",
				Name:   "print",
				Alias:  "$__lone_rt_print",
				Params: []WasmType{TypeI32, TypeI32},
				Result: TypeVoid,
			})
		} else {
			// WASI target
			l.mod.Imports = append(l.mod.Imports, Import{
				Module: "wasi_snapshot_preview1",
				Name:   "fd_write",
				Alias:  "$__wasi_fd_write",
				Params: []WasmType{TypeI32, TypeI32, TypeI32, TypeI32},
				Result: TypeI32,
			})
			l.mod.InlinedRuntime = append(l.mod.InlinedRuntime, `  ;; Inlined WASI #print runtime implementation
  (func $__lone_rt_print (param $ptr i32) (param $len i32)
    (i32.store (i32.const 0) (local.get $ptr))
    (i32.store (i32.const 4) (local.get $len))
    (drop (call $__wasi_fd_write (i32.const 1) (i32.const 0) (i32.const 1) (i32.const 8)))
  )`)
		}
	}

	if l.usesStrlen {
		l.mod.InlinedRuntime = append(l.mod.InlinedRuntime, `  ;; Inlined runtime strlen
  (func $__lone_rt_strlen (param $ptr i32) (result i32)
    (local $curr i32)
    (local.set $curr (local.get $ptr))
    (block $done
      (loop $loop
        (i32.eqz (i32.load8_u (local.get $curr)))
        (br_if $done)
        (local.set $curr (i32.add (local.get $curr) (i32.const 1)))
        (br $loop)
      )
    )
    (i32.sub (local.get $curr) (local.get $ptr))
  )`)
	}

	// Compute heap pointer after data segments
	dataEnd := l.nextOffset
	heapStart := (dataEnd + 15) & ^15
	totalMemBytes := opts.MemoryPages * 65536

	// Emit memory runtime globals: stack pointer and heap pointer
	l.mod.Globals = append([]Global{
		{
			Name:    "$__stack_pointer",
			Type:    TypeI32,
			Mutable: true,
			InitVal: fmt.Sprintf("i32.const %d", totalMemBytes),
		},
		{
			Name:    "$__heap_pointer",
			Type:    TypeI32,
			Mutable: true,
			InitVal: fmt.Sprintf("i32.const %d", heapStart),
		},
	}, l.mod.Globals...)

	return l.mod, nil
}

func (l *lowerer) allocString(val string) (int, int) {
	if entry, exists := l.strings[val]; exists {
		return entry.offset, entry.length
	}
	offset := l.nextOffset
	length := len(val)
	l.strings[val] = stringEntry{offset: offset, length: length}
	l.mod.DataSegments = append(l.mod.DataSegments, DataSegment{
		Offset: offset,
		RawStr: val,
	})
	// Null-terminated string + 4-byte alignment
	l.nextOffset = (offset + length + 1 + 3) & ^3
	return offset, length
}

func (l *lowerer) lowerFunction(fn *loneanalyzer.TFnDec) (*WasmFunc, error) {
	wfn := &WasmFunc{
		Name:   "$" + fn.Name,
		Result: toWasmType(fn.ReturnType),
	}

	if fn.Name == "main" {
		wfn.Exports = []string{"_start", "main"}
	}

	alloc := NewLocalAllocator()
	for _, p := range fn.Params {
		pName := alloc.AddParam(p.Name)
		wfn.Params = append(wfn.Params, Param{
			Name: pName,
			Type: toWasmType(p.Type),
		})
	}

	bodyInsts := l.lowerBlock(fn.Body, alloc)

	// Ensure function ends with return if needed
	if fn.ReturnType == loneanalyzer.TypeVoid || fn.ReturnType == nil {
		bodyInsts = append(bodyInsts, InstReturn{})
	} else {
		// Non-void function: if it does not end with an explicit return or unreachable,
		// append unreachable to satisfy Wasm stack validation.
		endsInReturn := false
		if len(bodyInsts) > 0 {
			switch bodyInsts[len(bodyInsts)-1].(type) {
			case InstReturn, InstUnreachable:
				endsInReturn = true
			}
		}
		if !endsInReturn {
			bodyInsts = append(bodyInsts, InstUnreachable{})
		}
	}

	wfn.Locals = alloc.Locals()
	wfn.Body = bodyInsts
	return wfn, nil
}

func (l *lowerer) lowerBlock(b loneanalyzer.TBlock, alloc *LocalAllocator) []WasmInst {
	alloc.EnterScope()
	defer alloc.ExitScope()

	var insts []WasmInst
	for _, stmt := range b.Stmts {
		sInsts := l.lowerStmt(stmt, alloc)
		insts = append(insts, sInsts...)
	}
	return insts
}

func (l *lowerer) lowerStmt(stmt loneanalyzer.TStmt, alloc *LocalAllocator) []WasmInst {
	switch s := stmt.(type) {
	case *loneanalyzer.TVarDec:
		var insts []WasmInst
		valInsts := l.lowerExpr(s.Value, alloc)
		insts = append(insts, valInsts...)

		// Track string variable lengths for #print
		if lit, ok := s.Value.(*loneanalyzer.TLiteralExpr); ok {
			if strVal, ok := lit.Value.(string); ok {
				l.stringVarLen[s.Name] = len(strVal)
			}
		}

		wType := toWasmType(s.Type)
		localName := alloc.AllocVar(s.Name, wType)
		insts = append(insts, InstLocalSet{Name: localName})
		return insts

	case *loneanalyzer.TAssignStmt:
		var insts []WasmInst
		valInsts := l.lowerExpr(s.Value, alloc)
		insts = append(insts, valInsts...)

		if ref, ok := s.Target.(*loneanalyzer.TVarRefExpr); ok {
			if localName, found := alloc.Resolve(ref.Name); found {
				insts = append(insts, InstLocalSet{Name: localName})
			} else {
				// Global variable
				insts = append(insts, InstGlobalSet{Name: "$g_" + ref.Name})
			}
		}
		return insts

	case *loneanalyzer.TReturnStmt:
		var insts []WasmInst
		if s.Value != nil {
			insts = append(insts, l.lowerExpr(s.Value, alloc)...)
		}
		insts = append(insts, InstReturn{})
		return insts

	case *loneanalyzer.TDropStmt:
		var insts []WasmInst
		exprInsts := l.lowerExpr(s.Expr, alloc)
		insts = append(insts, exprInsts...)
		if s.Expr.ExprType() != loneanalyzer.TypeVoid && s.Expr.ExprType() != nil {
			insts = append(insts, InstDrop{})
		}
		return insts

	case *loneanalyzer.TIfStmt:
		return l.lowerIfStmt(s, alloc)

	case *loneanalyzer.TPackedIfStmt:
		return l.lowerPackedIfStmt(s, alloc)

	case *loneanalyzer.TWhileStmt:
		return l.lowerWhileStmt(s, alloc)

	case *loneanalyzer.TLoopStmt:
		return l.lowerLoopStmt(s, alloc)

	case *loneanalyzer.TBreakStmt:
		var insts []WasmInst
		target := ""
		if s.Label != "" {
			if mapped, ok := l.labelStack[s.Label]; ok {
				target = mapped
			} else {
				target = "$" + s.Label
			}
		} else if len(l.loopStack) > 0 {
			target = l.loopStack[len(l.loopStack)-1].breakLbl
		}
		if s.Value != nil {
			insts = append(insts, l.lowerExpr(s.Value, alloc)...)
		}
		if target != "" {
			insts = append(insts, InstBr{Target: target})
		}
		return insts

	case *loneanalyzer.TLabelDec:
		bodyInsts := l.lowerBlock(s.Body, alloc)
		wType := toWasmType(s.Typ)
		return []WasmInst{
			InstBlock{
				Label:  "$" + s.Name,
				Result: wType,
				Body:   bodyInsts,
			},
		}

	case loneanalyzer.TExpr:
		var insts []WasmInst
		insts = append(insts, l.lowerExpr(s, alloc)...)
		if s.ExprType() != loneanalyzer.TypeVoid && s.ExprType() != nil {
			insts = append(insts, InstDrop{})
		}
		return insts

	default:
		return nil
	}
}

func (l *lowerer) lowerIfStmt(s *loneanalyzer.TIfStmt, alloc *LocalAllocator) []WasmInst {
	alloc.EnterScope()
	defer alloc.ExitScope()

	var insts []WasmInst
	var patternChecks []WasmInst
	var patternBinds []WasmInst

	// 1. Lower scoped header bindings if any (e.g. if: [let x = 0] ... or if: [let Shape::Circle(r) = s])
	for _, h := range s.Header {
		if h.Variant != nil {
			targetLocal := alloc.AllocTemp(TypeI32)
			defer alloc.FreeTemp(targetLocal, TypeI32)

			insts = append(insts, l.lowerExpr(h.TargetVal, alloc)...)
			insts = append(insts, InstLocalSet{Name: targetLocal})

			// Tag check: targetLocal.tag == h.Variant.Tag
			patternChecks = append(patternChecks, InstLocalGet{Name: targetLocal})
			patternChecks = append(patternChecks, InstLoad{Op: "i32.load", Offset: 0})
			patternChecks = append(patternChecks, InstConstI32{Value: int32(h.Variant.Tag)})
			patternChecks = append(patternChecks, InstBinaryOp{Op: "i32.eq"})

			// In Then block: bind fields
			for i, b := range h.Bindings {
				if i < len(h.Variant.Fields) {
					fInfo := h.Variant.Fields[i]
					bLocal := alloc.AllocVar(b.Name, toWasmType(fInfo.Type))
					patternBinds = append(patternBinds, InstLocalGet{Name: targetLocal})
					patternBinds = append(patternBinds, InstLoad{Op: loadOpForType(fInfo.Type), Offset: fInfo.Offset})
					patternBinds = append(patternBinds, InstLocalSet{Name: bLocal})
				}
			}
		} else {
			hInsts := l.lowerStmt(h, alloc)
			insts = append(insts, hInsts...)
		}
	}

	// 2. Evaluate condition
	if len(patternChecks) > 0 {
		insts = append(insts, patternChecks...)
		if s.Cond != nil {
			insts = append(insts, l.lowerExpr(s.Cond, alloc)...)
			insts = append(insts, InstBinaryOp{Op: "i32.and"})
		}
	} else if s.Cond != nil {
		condInsts := l.lowerExpr(s.Cond, alloc)
		insts = append(insts, condInsts...)
	}

	// 3. Lower Then and Else blocks
	thenInsts := append(patternBinds, l.lowerBlock(s.Then, alloc)...)

	var elseInsts []WasmInst
	if s.Else != nil {
		switch el := s.Else.(type) {
		case *loneanalyzer.TBlock:
			elseInsts = l.lowerBlock(*el, alloc)
		case *loneanalyzer.TIfStmt:
			elseInsts = l.lowerIfStmt(el, alloc)
		}
	}

	insts = append(insts, InstIf{
		Then: thenInsts,
		Else: elseInsts,
	})
	return insts
}

func (l *lowerer) lowerPackedIfStmt(p *loneanalyzer.TPackedIfStmt, alloc *LocalAllocator) []WasmInst {
	alloc.EnterScope()
	defer alloc.ExitScope()

	var insts []WasmInst
	for _, h := range p.Header {
		hInsts := l.lowerStmt(h, alloc)
		insts = append(insts, hInsts...)
	}

	// Chain arms as structured nested if/else
	insts = append(insts, l.lowerPackedArms(p.Arms, 0, alloc)...)
	return insts
}

func (l *lowerer) lowerPackedArms(arms []loneanalyzer.TPackedIfArm, index int, alloc *LocalAllocator) []WasmInst {
	if index >= len(arms) {
		return nil
	}
	arm := arms[index]
	if arm.Cond == nil {
		// Else arm
		return l.lowerStmtOrBlock(arm.Body, alloc)
	}

	var insts []WasmInst
	condInsts := l.lowerExpr(arm.Cond, alloc)
	insts = append(insts, condInsts...)

	thenInsts := l.lowerStmtOrBlock(arm.Body, alloc)
	elseInsts := l.lowerPackedArms(arms, index+1, alloc)

	insts = append(insts, InstIf{
		Then: thenInsts,
		Else: elseInsts,
	})
	return insts
}

func (l *lowerer) lowerStmtOrBlock(stmt loneanalyzer.TStmt, alloc *LocalAllocator) []WasmInst {
	if b, ok := stmt.(*loneanalyzer.TBlock); ok {
		return l.lowerBlock(*b, alloc)
	}
	return l.lowerStmt(stmt, alloc)
}

func (l *lowerer) lowerWhileStmt(s *loneanalyzer.TWhileStmt, alloc *LocalAllocator) []WasmInst {
	l.labelId++
	breakLbl := fmt.Sprintf("$while_break_%d", l.labelId)
	loopLbl := fmt.Sprintf("$while_loop_%d", l.labelId)

	l.loopStack = append(l.loopStack, loopContext{
		breakLbl: breakLbl,
		contLbl:  loopLbl,
	})
	defer func() {
		l.loopStack = l.loopStack[:len(l.loopStack)-1]
	}()

	var loopBody []WasmInst

	// Evaluate condition
	condInsts := l.lowerExpr(s.Cond, alloc)
	loopBody = append(loopBody, condInsts...)
	loopBody = append(loopBody, InstUnaryOp{Op: "i32.eqz"})
	loopBody = append(loopBody, InstBrIf{Target: breakLbl})

	// Body statements
	bodyInsts := l.lowerBlock(s.Body, alloc)
	loopBody = append(loopBody, bodyInsts...)

	// Back branch
	loopBody = append(loopBody, InstBr{Target: loopLbl})

	return []WasmInst{
		InstBlock{
			Label: breakLbl,
			Body: []WasmInst{
				InstLoop{
					Label: loopLbl,
					Body:  loopBody,
				},
			},
		},
	}
}

func (l *lowerer) lowerLoopStmt(s *loneanalyzer.TLoopStmt, alloc *LocalAllocator) []WasmInst {
	l.labelId++
	breakLbl := fmt.Sprintf("$loop_break_%d", l.labelId)
	contLbl := fmt.Sprintf("$loop_cont_%d", l.labelId)

	l.loopStack = append(l.loopStack, loopContext{
		breakLbl: breakLbl,
		contLbl:  contLbl,
	})
	defer func() {
		l.loopStack = l.loopStack[:len(l.loopStack)-1]
	}()

	var loopBody []WasmInst
	bodyInsts := l.lowerBlock(s.Body, alloc)
	loopBody = append(loopBody, bodyInsts...)

	if s.WhileCond != nil {
		// loop { ... } while: [cond]
		// Evaluates post-condition at loop tail: if true, branches back to contLbl
		condInsts := l.lowerExpr(s.WhileCond, alloc)
		loopBody = append(loopBody, condInsts...)
		loopBody = append(loopBody, InstBrIf{Target: contLbl})
	} else {
		// Infinite loop: unconditionally branch back
		loopBody = append(loopBody, InstBr{Target: contLbl})
	}

	return []WasmInst{
		InstBlock{
			Label: breakLbl,
			Body: []WasmInst{
				InstLoop{
					Label: contLbl,
					Body:  loopBody,
				},
			},
		},
	}
}

func (l *lowerer) lowerExpr(expr loneanalyzer.TExpr, alloc *LocalAllocator) []WasmInst {
	switch e := expr.(type) {
	case *loneanalyzer.TLiteralExpr:
		switch e.Kind {
		case loneast.LiteralInt:
			if v, ok := e.Value.(int64); ok {
				if e.Typ == loneanalyzer.TypeI64 || e.Typ == loneanalyzer.TypeU64 {
					return []WasmInst{InstConstI64{Value: v}}
				}
				return []WasmInst{InstConstI32{Value: int32(v)}}
			}
			return []WasmInst{InstConstI32{Value: 0}}

		case loneast.LiteralFloat:
			if v, ok := e.Value.(float64); ok {
				if e.Typ == loneanalyzer.TypeF32 {
					return []WasmInst{InstConstF32{Value: float32(v)}}
				}
				return []WasmInst{InstConstF64{Value: v}}
			}
			return []WasmInst{InstConstF64{Value: 0}}

		case loneast.LiteralBool:
			if b, ok := e.Value.(bool); ok && b {
				return []WasmInst{InstConstI32{Value: 1}}
			}
			return []WasmInst{InstConstI32{Value: 0}}

		case loneast.LiteralString:
			if s, ok := e.Value.(string); ok {
				offset, _ := l.allocString(s)
				return []WasmInst{InstConstI32{Value: int32(offset)}}
			}
			return []WasmInst{InstConstI32{Value: 0}}

		default:
			return []WasmInst{InstConstI32{Value: 0}}
		}

	case *loneanalyzer.TVarRefExpr:
		if localName, found := alloc.Resolve(e.Name); found {
			return []WasmInst{InstLocalGet{Name: localName}}
		}
		// Module global
		return []WasmInst{InstGlobalGet{Name: "$g_" + e.Name}}

	case *loneanalyzer.TBinaryExpr:
		var insts []WasmInst
		insts = append(insts, l.lowerExpr(e.Left, alloc)...)
		insts = append(insts, l.lowerExpr(e.Right, alloc)...)

		opStr := l.mapBinaryOp(e.Op, e.Left.ExprType())
		insts = append(insts, InstBinaryOp{Op: opStr})
		return insts

	case *loneanalyzer.TUnaryExpr:
		var insts []WasmInst
		switch e.Op {
		case loneast.UNARY_OP_NEGATIVE:
			if isFloatType(e.Expr.ExprType()) {
				insts = append(insts, l.lowerExpr(e.Expr, alloc)...)
				insts = append(insts, InstUnaryOp{Op: "f64.neg"})
			} else {
				insts = append(insts, InstConstI32{Value: 0})
				insts = append(insts, l.lowerExpr(e.Expr, alloc)...)
				insts = append(insts, InstBinaryOp{Op: "i32.sub"})
			}
		case loneast.UNARY_OP_NOT:
			insts = append(insts, l.lowerExpr(e.Expr, alloc)...)
			insts = append(insts, InstUnaryOp{Op: "i32.eqz"})
		default:
			insts = append(insts, l.lowerExpr(e.Expr, alloc)...)
		}
		return insts

	case *loneanalyzer.TCastExpr:
		var insts []WasmInst
		insts = append(insts, l.lowerExpr(e.Expr, alloc)...)
		castInst := l.mapCastOp(e.Expr.ExprType(), e.TargetType)
		if castInst != nil {
			insts = append(insts, castInst)
		}
		return insts

	case *loneanalyzer.TCallExpr:
		return l.lowerCallExpr(e, alloc)

	case *loneanalyzer.TBlock:
		// Expression block: calc_blk: { ... }
		return l.lowerBlock(*e, alloc)

	case *loneanalyzer.TLabelDec:
		bodyInsts := l.lowerBlock(e.Body, alloc)
		wType := toWasmType(e.Typ)
		return []WasmInst{
			InstBlock{
				Label:  "$" + e.Name,
				Result: wType,
				Body:   bodyInsts,
			},
		}

	case *loneanalyzer.TEnumConstructExpr:
		return l.lowerEnumConstructExpr(e, alloc)

	case *loneanalyzer.TMatchExpr:
		return l.lowerMatchExpr(e, alloc)

	case *loneanalyzer.TIsExpr:
		var insts []WasmInst
		if e.TargetVariant != nil {
			insts = append(insts, l.lowerExpr(e.Expr, alloc)...)
			insts = append(insts, InstLoad{Op: "i32.load", Offset: 0})
			insts = append(insts, InstConstI32{Value: int32(e.TargetVariant.Tag)})
			if e.Negated {
				insts = append(insts, InstBinaryOp{Op: "i32.ne"})
			} else {
				insts = append(insts, InstBinaryOp{Op: "i32.eq"})
			}
			return insts
		}
		insts = append(insts, InstConstI32{Value: 1})
		return insts

	case *loneanalyzer.TMustExpr:
		return l.lowerMustExpr(e, alloc)

	case *loneanalyzer.TTryExpr:
		return l.lowerTryExpr(e, alloc)

	case *loneanalyzer.TTraitMatExpr:
		return []WasmInst{InstConstI32{Value: 0}}

	case *loneanalyzer.TMemberAccessExpr:
		if e.Offset >= 0 && e.Typ != loneanalyzer.TypeVoid && e.Typ != nil {
			var insts []WasmInst
			insts = append(insts, l.lowerExpr(e.Base, alloc)...)
			insts = append(insts, InstLoad{Op: loadOpForType(e.Typ), Offset: e.Offset})
			return insts
		}
		return []WasmInst{InstConstI32{Value: 0}}

	default:
		return nil
	}
}

func (l *lowerer) lowerCallExpr(e *loneanalyzer.TCallExpr, alloc *LocalAllocator) []WasmInst {
	calleeName := ""
	if ref, ok := e.Callee.(*loneanalyzer.TVarRefExpr); ok {
		calleeName = ref.Name
	} else if mem, ok := e.Callee.(*loneanalyzer.TMemberAccessExpr); ok {
		calleeName = mem.Field
	}

	// Handle legacy built-in intrinsic: #print
	if calleeName == "#print" {
		l.usesPrint = true
		var insts []WasmInst
		if len(e.Args) == 1 {
			arg := e.Args[0]
			if lit, ok := arg.(*loneanalyzer.TLiteralExpr); ok {
				if strVal, ok := lit.Value.(string); ok {
					offset, length := l.allocString(strVal)
					insts = append(insts, InstConstI32{Value: int32(offset)})
					insts = append(insts, InstConstI32{Value: int32(length)})
					insts = append(insts, InstCall{Target: "$__lone_rt_print"})
					return insts
				}
			}
			// Variable reference
			insts = append(insts, l.lowerExpr(arg, alloc)...)
			if ref, ok := arg.(*loneanalyzer.TVarRefExpr); ok {
				if strLen, hasLen := l.stringVarLen[ref.Name]; hasLen {
					insts = append(insts, InstConstI32{Value: int32(strLen)})
					insts = append(insts, InstCall{Target: "$__lone_rt_print"})
					return insts
				}
			}
			// Dynamic string length via runtime strlen
			l.usesStrlen = true
			ptrLocal := alloc.AllocTemp(TypeI32)
			defer alloc.FreeTemp(ptrLocal, TypeI32)
			var dynInsts []WasmInst
			dynInsts = append(dynInsts, l.lowerExpr(arg, alloc)...)
			dynInsts = append(dynInsts, InstLocalTee{Name: ptrLocal})
			dynInsts = append(dynInsts, InstLocalGet{Name: ptrLocal})
			dynInsts = append(dynInsts, InstCall{Target: "$__lone_rt_strlen"})
			dynInsts = append(dynInsts, InstCall{Target: "$__lone_rt_print"})
			return dynInsts
		} else if len(e.Args) >= 2 {
			// #print(ptr, len)
			insts = append(insts, l.lowerExpr(e.Args[0], alloc)...)
			insts = append(insts, l.lowerExpr(e.Args[1], alloc)...)
			insts = append(insts, InstCall{Target: "$__lone_rt_print"})
			return insts
		}
		return nil
	}

	// User function call
	var insts []WasmInst
	for _, arg := range e.Args {
		insts = append(insts, l.lowerExpr(arg, alloc)...)
	}
	insts = append(insts, InstCall{Target: "$" + calleeName})
	return insts
}

func (l *lowerer) mapBinaryOp(op loneast.BinaryOp, operandType loneanalyzer.Type) string {
	isFloat := isFloatType(operandType)
	prefix := "i32."
	if operandType == loneanalyzer.TypeF32 {
		prefix = "f32."
	} else if operandType == loneanalyzer.TypeF64 {
		prefix = "f64."
	} else if operandType == loneanalyzer.TypeI64 || operandType == loneanalyzer.TypeU64 {
		prefix = "i64."
	}

	switch op {
	case loneast.BINARY_OP_PLUS:
		return prefix + "add"
	case loneast.BINARY_OP_MINUS:
		return prefix + "sub"
	case loneast.BINARY_OP_TIME:
		return prefix + "mul"
	case loneast.BINARY_OP_DIV:
		if isFloat {
			return prefix + "div"
		}
		return prefix + "div_s"
	case loneast.BINARY_OP_MOD:
		return prefix + "rem_s"
	case loneast.BINARY_OP_EQ:
		return prefix + "eq"
	case loneast.BINARY_OP_NOT_EQ:
		return prefix + "ne"
	case loneast.BINARY_OP_LESS:
		if isFloat {
			return prefix + "lt"
		}
		return prefix + "lt_s"
	case loneast.BINARY_OP_LESS_EQ:
		if isFloat {
			return prefix + "le"
		}
		return prefix + "le_s"
	case loneast.BINARY_OP_GREATER:
		if isFloat {
			return prefix + "gt"
		}
		return prefix + "gt_s"
	case loneast.BINARY_OP_GREATER_EQ:
		if isFloat {
			return prefix + "ge"
		}
		return prefix + "ge_s"
	case loneast.BINARY_OP_AND:
		return "i32.and"
	case loneast.BINARY_OP_OR:
		return "i32.or"
	case loneast.BINARY_OP_XOR:
		return "i32.xor"
	default:
		return prefix + "add"
	}
}

func (l *lowerer) mapCastOp(from, to loneanalyzer.Type) WasmInst {
	fromW := toWasmType(from)
	toW := toWasmType(to)

	if fromW == toW {
		return nil
	}

	if fromW == TypeI32 && toW == TypeF64 {
		return InstUnaryOp{Op: "f64.convert_i32_s"}
	}
	if fromW == TypeI32 && toW == TypeF32 {
		return InstUnaryOp{Op: "f32.convert_i32_s"}
	}
	if fromW == TypeF64 && toW == TypeI32 {
		return InstUnaryOp{Op: "i32.trunc_f64_s"}
	}
	if fromW == TypeF32 && toW == TypeI32 {
		return InstUnaryOp{Op: "i32.trunc_f32_s"}
	}
	if fromW == TypeI32 && toW == TypeI64 {
		return InstUnaryOp{Op: "i64.extend_i32_s"}
	}
	if fromW == TypeI64 && toW == TypeI32 {
		return InstUnaryOp{Op: "i32.wrap_i64"}
	}
	if fromW == TypeF32 && toW == TypeF64 {
		return InstUnaryOp{Op: "f64.promote_f32"}
	}
	if fromW == TypeF64 && toW == TypeF32 {
		return InstUnaryOp{Op: "f32.demote_f64"}
	}
	return nil
}

func (l *lowerer) formatGlobalInit(val loneanalyzer.TExpr, typ loneanalyzer.Type) string {
	if lit, ok := val.(*loneanalyzer.TLiteralExpr); ok {
		switch lit.Kind {
		case loneast.LiteralInt:
			if v, ok := lit.Value.(int64); ok {
				if typ == loneanalyzer.TypeI64 {
					return fmt.Sprintf("i64.const %d", v)
				}
				return fmt.Sprintf("i32.const %d", v)
			}
		case loneast.LiteralFloat:
			if v, ok := lit.Value.(float64); ok {
				return fmt.Sprintf("f64.const %g", v)
			}
		case loneast.LiteralBool:
			if b, ok := lit.Value.(bool); ok && b {
				return "i32.const 1"
			}
			return "i32.const 0"
		case loneast.LiteralString:
			if s, ok := lit.Value.(string); ok {
				offset, _ := l.allocString(s)
				return fmt.Sprintf("i32.const %d", offset)
			}
		}
	}
	return "i32.const 0"
}

func toWasmType(t loneanalyzer.Type) WasmType {
	if t == nil {
		return TypeVoid
	}
	switch t {
	case loneanalyzer.TypeI64, loneanalyzer.TypeU64:
		return TypeI64
	case loneanalyzer.TypeF32:
		return TypeF32
	case loneanalyzer.TypeF64:
		return TypeF64
	case loneanalyzer.TypeVoid:
		return TypeVoid
	default:
		return TypeI32
	}
}

func isFloatType(t loneanalyzer.Type) bool {
	return t == loneanalyzer.TypeF32 || t == loneanalyzer.TypeF64
}

func (l *lowerer) lowerEnumConstructExpr(e *loneanalyzer.TEnumConstructExpr, alloc *LocalAllocator) []WasmInst {
	var insts []WasmInst
	ptrLocal := alloc.AllocTemp(TypeI32)
	defer alloc.FreeTemp(ptrLocal, TypeI32)

	totalSize := e.EnumType.TotalSize
	if totalSize < 4 {
		totalSize = 4
	}

	// 1. Bump heap pointer to allocate memory for enum
	insts = append(insts, InstGlobalGet{Name: "$__heap_pointer"})
	insts = append(insts, InstLocalTee{Name: ptrLocal})
	insts = append(insts, InstConstI32{Value: int32(totalSize)})
	insts = append(insts, InstBinaryOp{Op: "i32.add"})
	insts = append(insts, InstGlobalSet{Name: "$__heap_pointer"})

	// 2. Store tag at offset 0
	insts = append(insts, InstLocalGet{Name: ptrLocal})
	insts = append(insts, InstConstI32{Value: int32(e.Variant.Tag)})
	insts = append(insts, InstStore{Op: "i32.store", Offset: 0})

	// 3. Store payload fields
	for i, arg := range e.Args {
		if i < len(e.Variant.Fields) {
			fieldInfo := e.Variant.Fields[i]
			insts = append(insts, InstLocalGet{Name: ptrLocal})
			insts = append(insts, l.lowerExpr(arg, alloc)...)
			insts = append(insts, InstStore{Op: storeOpForType(fieldInfo.Type), Offset: fieldInfo.Offset})
		}
	}

	// 4. Return pointer to enum on operand stack
	insts = append(insts, InstLocalGet{Name: ptrLocal})
	return insts
}

func (l *lowerer) lowerMatchExpr(m *loneanalyzer.TMatchExpr, alloc *LocalAllocator) []WasmInst {
	var insts []WasmInst

	targetType := m.Target.ExprType()
	targetWType := toWasmType(targetType)
	targetLocal := alloc.AllocTemp(targetWType)
	defer alloc.FreeTemp(targetLocal, targetWType)

	// Evaluate target
	insts = append(insts, l.lowerExpr(m.Target, alloc)...)
	insts = append(insts, InstLocalSet{Name: targetLocal})

	_, isEnum := targetType.(*loneanalyzer.EnumType)
	tagLocal := ""
	if isEnum {
		tagLocal = alloc.AllocTemp(TypeI32)
		defer alloc.FreeTemp(tagLocal, TypeI32)
		// Load tag from offset 0
		insts = append(insts, InstLocalGet{Name: targetLocal})
		insts = append(insts, InstLoad{Op: "i32.load", Offset: 0})
		insts = append(insts, InstLocalSet{Name: tagLocal})
	}

	var resultLocal string
	if m.Typ != loneanalyzer.TypeVoid && m.Typ != nil {
		resWType := toWasmType(m.Typ)
		resultLocal = alloc.AllocTemp(resWType)
		defer alloc.FreeTemp(resultLocal, resWType)
	}

	armInsts := l.lowerMatchArms(m.Arms, 0, targetLocal, tagLocal, resultLocal, isEnum, targetType, alloc)
	insts = append(insts, armInsts...)

	if resultLocal != "" {
		insts = append(insts, InstLocalGet{Name: resultLocal})
	}

	return insts
}

func (l *lowerer) lowerMatchArms(arms []loneanalyzer.TMatchArm, index int, targetLocal, tagLocal, resultLocal string, isEnum bool, targetType loneanalyzer.Type, alloc *LocalAllocator) []WasmInst {
	if index >= len(arms) {
		return nil
	}

	arm := arms[index]
	alloc.EnterScope()
	defer alloc.ExitScope()

	switch pat := arm.Pattern.(type) {
	case loneast.WildcardPattern:
		var thenInsts []WasmInst
		if arm.BodyExpr != nil {
			thenInsts = append(thenInsts, l.lowerExpr(arm.BodyExpr, alloc)...)
			if resultLocal != "" {
				thenInsts = append(thenInsts, InstLocalSet{Name: resultLocal})
			}
		} else if arm.BodyBlock != nil {
			thenInsts = append(thenInsts, l.lowerBlock(*arm.BodyBlock, alloc)...)
		}
		return thenInsts

	case loneast.VariantPattern:
		var condInsts []WasmInst
		condInsts = append(condInsts, InstLocalGet{Name: tagLocal})
		tagVal := 0
		if arm.Variant != nil {
			tagVal = arm.Variant.Tag
		}
		condInsts = append(condInsts, InstConstI32{Value: int32(tagVal)})
		condInsts = append(condInsts, InstBinaryOp{Op: "i32.eq"})

		var thenInsts []WasmInst
		// Bind payload variables
		if arm.Variant != nil {
			for i, b := range arm.Bindings {
				if i < len(arm.Variant.Fields) {
					fInfo := arm.Variant.Fields[i]
					bLocal := alloc.AllocVar(b.Name, toWasmType(fInfo.Type))
					thenInsts = append(thenInsts, InstLocalGet{Name: targetLocal})
					thenInsts = append(thenInsts, InstLoad{Op: loadOpForType(fInfo.Type), Offset: fInfo.Offset})
					thenInsts = append(thenInsts, InstLocalSet{Name: bLocal})
				}
			}
		}

		if arm.BodyExpr != nil {
			thenInsts = append(thenInsts, l.lowerExpr(arm.BodyExpr, alloc)...)
			if resultLocal != "" {
				thenInsts = append(thenInsts, InstLocalSet{Name: resultLocal})
			}
		} else if arm.BodyBlock != nil {
			thenInsts = append(thenInsts, l.lowerBlock(*arm.BodyBlock, alloc)...)
		}

		elseInsts := l.lowerMatchArms(arms, index+1, targetLocal, tagLocal, resultLocal, isEnum, targetType, alloc)
		var res []WasmInst
		res = append(res, condInsts...)
		res = append(res, InstIf{
			Then: thenInsts,
			Else: elseInsts,
		})
		return res

	case loneast.LiteralPattern:
		var condInsts []WasmInst
		condInsts = append(condInsts, InstLocalGet{Name: targetLocal})
		litWasm := l.lowerExpr(&loneanalyzer.TLiteralExpr{
			Kind:  pat.Literal.Kind,
			Value: pat.Literal.Value,
			Typ:   targetType,
			Span_: pat.Span_,
		}, alloc)
		condInsts = append(condInsts, litWasm...)
		eqOp := "i32.eq"
		if isFloatType(targetType) {
			if targetType == loneanalyzer.TypeF32 {
				eqOp = "f32.eq"
			} else {
				eqOp = "f64.eq"
			}
		} else if targetType == loneanalyzer.TypeI64 || targetType == loneanalyzer.TypeU64 {
			eqOp = "i64.eq"
		}
		condInsts = append(condInsts, InstBinaryOp{Op: eqOp})

		var thenInsts []WasmInst
		if arm.BodyExpr != nil {
			thenInsts = append(thenInsts, l.lowerExpr(arm.BodyExpr, alloc)...)
			if resultLocal != "" {
				thenInsts = append(thenInsts, InstLocalSet{Name: resultLocal})
			}
		} else if arm.BodyBlock != nil {
			thenInsts = append(thenInsts, l.lowerBlock(*arm.BodyBlock, alloc)...)
		}

		elseInsts := l.lowerMatchArms(arms, index+1, targetLocal, tagLocal, resultLocal, isEnum, targetType, alloc)
		var res []WasmInst
		res = append(res, condInsts...)
		res = append(res, InstIf{
			Then: thenInsts,
			Else: elseInsts,
		})
		return res

	default:
		return nil
	}
}

func loadOpForType(t loneanalyzer.Type) string {
	switch t {
	case loneanalyzer.TypeI64, loneanalyzer.TypeU64:
		return "i64.load"
	case loneanalyzer.TypeF32:
		return "f32.load"
	case loneanalyzer.TypeF64:
		return "f64.load"
	default:
		return "i32.load"
	}
}

func storeOpForType(t loneanalyzer.Type) string {
	switch t {
	case loneanalyzer.TypeI64, loneanalyzer.TypeU64:
		return "i64.store"
	case loneanalyzer.TypeF32:
		return "f32.store"
	case loneanalyzer.TypeF64:
		return "f64.store"
	default:
		return "i32.store"
	}
}

func (l *lowerer) lowerMustExpr(m *loneanalyzer.TMustExpr, alloc *LocalAllocator) []WasmInst {
	var insts []WasmInst
	targetLocal := alloc.AllocTemp(TypeI32)
	defer alloc.FreeTemp(targetLocal, TypeI32)

	insts = append(insts, l.lowerExpr(m.Expr, alloc)...)
	insts = append(insts, InstLocalSet{Name: targetLocal})

	resLocal := ""
	if m.OkType != loneanalyzer.TypeVoid && m.OkType != nil {
		resWType := toWasmType(m.OkType)
		resLocal = alloc.AllocTemp(resWType)
		defer alloc.FreeTemp(resLocal, resWType)
	}

	// Condition: target.tag == OkVariant.Tag
	var condInsts []WasmInst
	condInsts = append(condInsts, InstLocalGet{Name: targetLocal})
	condInsts = append(condInsts, InstLoad{Op: "i32.load", Offset: 0})
	condInsts = append(condInsts, InstConstI32{Value: int32(m.OkVariant.Tag)})
	condInsts = append(condInsts, InstBinaryOp{Op: "i32.eq"})

	var thenInsts []WasmInst
	if resLocal != "" && len(m.OkVariant.Fields) > 0 {
		fInfo := m.OkVariant.Fields[0]
		thenInsts = append(thenInsts, InstLocalGet{Name: targetLocal})
		thenInsts = append(thenInsts, InstLoad{Op: loadOpForType(fInfo.Type), Offset: fInfo.Offset})
		thenInsts = append(thenInsts, InstLocalSet{Name: resLocal})
	}

	// On Err: unreachable (panics in WebAssembly)
	elseInsts := []WasmInst{
		InstUnreachable{},
	}

	insts = append(insts, condInsts...)
	insts = append(insts, InstIf{
		Then: thenInsts,
		Else: elseInsts,
	})

	if resLocal != "" {
		insts = append(insts, InstLocalGet{Name: resLocal})
	}
	return insts
}

func (l *lowerer) lowerTryExpr(t *loneanalyzer.TTryExpr, alloc *LocalAllocator) []WasmInst {
	var insts []WasmInst
	targetLocal := alloc.AllocTemp(TypeI32)
	defer alloc.FreeTemp(targetLocal, TypeI32)

	insts = append(insts, l.lowerExpr(t.Expr, alloc)...)
	insts = append(insts, InstLocalSet{Name: targetLocal})

	resLocal := ""
	if t.OkType != loneanalyzer.TypeVoid && t.OkType != nil {
		resWType := toWasmType(t.OkType)
		resLocal = alloc.AllocTemp(resWType)
		defer alloc.FreeTemp(resLocal, resWType)
	}

	// Condition: target.tag == OkVariant.Tag
	var condInsts []WasmInst
	condInsts = append(condInsts, InstLocalGet{Name: targetLocal})
	condInsts = append(condInsts, InstLoad{Op: "i32.load", Offset: 0})
	condInsts = append(condInsts, InstConstI32{Value: int32(t.OkVariant.Tag)})
	condInsts = append(condInsts, InstBinaryOp{Op: "i32.eq"})

	var thenInsts []WasmInst
	if resLocal != "" && len(t.OkVariant.Fields) > 0 {
		fInfo := t.OkVariant.Fields[0]
		thenInsts = append(thenInsts, InstLocalGet{Name: targetLocal})
		thenInsts = append(thenInsts, InstLoad{Op: loadOpForType(fInfo.Type), Offset: fInfo.Offset})
		thenInsts = append(thenInsts, InstLocalSet{Name: resLocal})
	}

	// On Err: early-return the target (which is Err)
	elseInsts := []WasmInst{
		InstLocalGet{Name: targetLocal},
		InstReturn{},
	}

	insts = append(insts, condInsts...)
	insts = append(insts, InstIf{
		Then: thenInsts,
		Else: elseInsts,
	})

	if resLocal != "" {
		insts = append(insts, InstLocalGet{Name: resLocal})
	}
	return insts
}
