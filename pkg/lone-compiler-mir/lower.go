package lonemir

import (
	"fmt"
	"lone/pkg/lone-analyzer"
	"lone/pkg/lone-parser-ast"
)

// LowerProgram translates a Typed IR program from lone-alyzer into a Falone MirModule.
func LowerProgram(prog *loneanalyzer.TProgram) (*MirModule, error) {
	if prog == nil {
		return &MirModule{
			Functions:   make([]*MirFunction, 0),
			Globals:     make(map[string]MirT),
			GlobalInits: make(map[string]any),
		}, nil
	}

	lowerer := newLowerer()
	return lowerer.lower(prog)
}

type varSlotInfo struct {
	offset      int
	typ         loneanalyzer.Type
	isAggregate bool
}

type lowerer struct {
	mod           *MirModule
	fn            *MirFunction
	curBlk        *MirBlock
	regId         int
	blockId       int
	frameOffset   int
	curStructRet  *loneanalyzer.StructType
	vars          map[string]string      // maps variable name to register name
	varSlots      map[string]varSlotInfo // maps stack-allocated scalar/aggregate var name to datastack slot
	structReturns map[string]*loneanalyzer.StructType
	loopExitStack []string       // tracks enclosing loop exit block labels
	dataOffset    int            // tracks linear memory allocation offset
	stringOffsets map[string]int // deduplicates string literals to data offsets
	stringVarLens map[string]int // tracks known string length for variables
}

func newLowerer() *lowerer {
	return &lowerer{
		mod: &MirModule{
			Functions:    make([]*MirFunction, 0),
			Globals:      make(map[string]MirT),
			GlobalInits:  make(map[string]any),
			DataSegments: make([]MirDataSegment, 0),
			HostImports:  make([]MirHostImport, 0),
		},
		vars:          make(map[string]string),
		varSlots:      make(map[string]varSlotInfo),
		structReturns: make(map[string]*loneanalyzer.StructType),
		loopExitStack: make([]string, 0),
		dataOffset:    16, // offsets 0..15 reserved for WASI ciovec_t scratchpad
		stringOffsets: make(map[string]int),
		stringVarLens: make(map[string]int),
	}
}

func isAggregateType(t loneanalyzer.Type) bool {
	if t == nil {
		return false
	}
	switch v := t.(type) {
	case *loneanalyzer.StructType:
		return true
	case *loneanalyzer.ListType:
		return v.Size >= 0
	default:
		return false
	}
}

func alignOffset(off, align int) int {
	if align <= 1 {
		return off
	}
	rem := off % align
	if rem == 0 {
		return off
	}
	return off + (align - rem)
}

func (l *lowerer) allocSlot(size, align int) int {
	if size <= 0 {
		size = 4
	}
	if align <= 0 {
		align = 4
	}
	off := alignOffset(l.frameOffset, align)
	l.frameOffset = off + size
	return off
}

func (l *lowerer) emitSlotAddr(offset int) string {
	reg := l.freshReg("t_slot", MirI32)
	l.curBlk.Insts = append(l.curBlk.Insts, MirSlotAddr{
		Dest:   reg,
		Offset: offset,
	})
	return reg
}

func (l *lowerer) emitMemCopy(dstAddr, srcAddr MirOperand, size int) {
	offset := 0
	for offset+4 <= size {
		tmp := l.freshReg("t_cpy", MirI32)
		l.curBlk.Insts = append(l.curBlk.Insts,
			MirLoad{Dest: tmp, Type: MirI32, Size: 4, Signed: false, Addr: srcAddr, Offset: offset},
			MirStore{Type: MirI32, Size: 4, Addr: dstAddr, Offset: offset, Value: MirReg{Name: tmp}},
		)
		offset += 4
	}
	for offset < size {
		tmp := l.freshReg("t_cpyb", MirI32)
		l.curBlk.Insts = append(l.curBlk.Insts,
			MirLoad{Dest: tmp, Type: MirI32, Size: 1, Signed: false, Addr: srcAddr, Offset: offset},
			MirStore{Type: MirI32, Size: 1, Addr: dstAddr, Offset: offset, Value: MirReg{Name: tmp}},
		)
		offset++
	}
}

func (l *lowerer) emitMemZero(dstAddr MirOperand, size int) {
	offset := 0
	for offset+4 <= size && offset < 64 {
		l.curBlk.Insts = append(l.curBlk.Insts,
			MirStore{Type: MirI32, Size: 4, Addr: dstAddr, Offset: offset, Value: MirConst{Type: MirI32, Value: 0}},
		)
		offset += 4
	}
	for offset < size && offset < 64 {
		l.curBlk.Insts = append(l.curBlk.Insts,
			MirStore{Type: MirI32, Size: 1, Addr: dstAddr, Offset: offset, Value: MirConst{Type: MirI32, Value: 0}},
		)
		offset++
	}
}

func (l *lowerer) allocString(val string) int {
	if offset, exists := l.stringOffsets[val]; exists {
		return offset
	}
	offset := l.dataOffset
	l.stringOffsets[val] = offset
	l.mod.DataSegments = append(l.mod.DataSegments, MirDataSegment{
		Offset: offset,
		Value:  val,
	})
	// Advance offset by string byte length + 1 (null terminator) and align to 4 bytes
	l.dataOffset += len(val) + 1
	l.dataOffset = (l.dataOffset + 3) & ^3
	return offset
}

func (l *lowerer) freshReg(prefix string, typ MirT) string {
	l.regId++
	name := fmt.Sprintf("%s%d", prefix, l.regId)
	l.fn.AddLocal(name, typ)
	return name
}

func (l *lowerer) freshBlock(name string) *MirBlock {
	l.blockId++
	blkName := fmt.Sprintf("%s_%d", name, l.blockId)
	return l.fn.AddBlock(blkName)
}

func (l *lowerer) lower(prog *loneanalyzer.TProgram) (*MirModule, error) {
	// Pre-pass: record functions returning StructType by value (SRET ABI) and globals/host imports
	for _, decl := range prog.Decls {
		switch d := decl.(type) {
		case *loneanalyzer.TFnDec:
			if st, ok := d.ReturnType.(*loneanalyzer.StructType); ok {
				l.structReturns[d.Name] = st
			}
		case *loneanalyzer.THostImport:
			l.mod.HostImports = append(l.mod.HostImports, MirHostImport{
				WasmModule: d.WasmModule,
				WasmName:   d.WasmName,
				LocalName:  d.LocalName,
				Params:     d.WasmParams,
				Result:     d.ReturnWasmType,
			})
		case *loneanalyzer.TVarDec:
			mType := toMirType(d.Type)
			l.mod.Globals[d.Name] = mType
			if lit, ok := d.Value.(*loneanalyzer.TLiteralExpr); ok {
				if strVal, isStr := lit.Value.(string); isStr && lit.Kind == loneast.LITERAL_STRING {
					l.mod.GlobalInits[d.Name] = l.allocString(strVal)
					l.stringVarLens[d.Name] = len(strVal)
				} else {
					l.mod.GlobalInits[d.Name] = lit.Value
				}
			}
		}
	}

	// Second pass: lower functions
	for _, decl := range prog.Decls {
		if d, ok := decl.(*loneanalyzer.TFnDec); ok {
			fn, err := l.lowerFunction(d)
			if err != nil {
				return nil, err
			}
			l.mod.Functions = append(l.mod.Functions, fn)
		}
	}
	return l.mod, nil
}

func (l *lowerer) lowerFunction(d *loneanalyzer.TFnDec) (*MirFunction, error) {
	retMirType := toMirType(d.ReturnType)
	l.curStructRet = nil
	if st, ok := d.ReturnType.(*loneanalyzer.StructType); ok {
		l.curStructRet = st
		retMirType = MirI32
	}

	l.fn = &MirFunction{
		Name:       d.Name,
		Params:     make([]MirParam, 0, len(d.Params)+1),
		ReturnType: retMirType,
		Blocks:     make([]*MirBlock, 0),
		Locals:     make(map[string]MirT),
	}
	l.vars = make(map[string]string)
	l.varSlots = make(map[string]varSlotInfo)
	l.regId = 0
	l.blockId = 0
	l.frameOffset = 0

	if l.curStructRet != nil {
		l.fn.Params = append(l.fn.Params, MirParam{
			Name: "__sret",
			Type: MirI32,
		})
	}

	// Add params
	for _, p := range d.Params {
		pType := toMirType(p.Type)
		pReg := p.Name
		l.fn.Params = append(l.fn.Params, MirParam{
			Name: pReg,
			Type: pType,
		})
		l.vars[p.Name] = pReg
	}

	// Create entry block
	l.curBlk = l.fn.AddBlock("entry")

	// Spill scalar parameters whose address is taken (&raw param / &param) onto the datastack frame
	for _, p := range d.Params {
		if p.AddressTaken && !isAggregateType(p.Type) {
			slotOff := l.allocSlot(loneanalyzer.TypeSize(p.Type), loneanalyzer.TypeAlign(p.Type))
			l.varSlots[p.Name] = varSlotInfo{
				offset:      slotOff,
				typ:         p.Type,
				isAggregate: false,
			}
			addrReg := l.emitSlotAddr(slotOff)
			l.curBlk.Insts = append(l.curBlk.Insts, MirStore{
				Type:   toMirType(p.Type),
				Size:   loneanalyzer.TypeSize(p.Type),
				Addr:   MirReg{Name: addrReg},
				Offset: 0,
				Value:  MirReg{Name: p.Name},
			})
		}
	}

	// Lower body
	l.lowerBlock(&d.Body)

	// Ensure entry block or final block has a terminator
	if len(l.fn.Blocks) > 0 {
		lastBlk := l.fn.Blocks[len(l.fn.Blocks)-1]
		if lastBlk.Terminator == nil {
			if l.fn.ReturnType == MirVoid {
				lastBlk.Terminator = MirReturn{}
			} else {
				lastBlk.Terminator = MirReturn{
					Value: MirConst{Type: l.fn.ReturnType, Value: 0},
				}
			}
		}
	}

	if l.frameOffset > 0 {
		l.fn.FrameSize = alignOffset(l.frameOffset, 16)
	}

	return l.fn, nil
}

func (l *lowerer) lowerBlock(b *loneanalyzer.TBlock) {
	for _, stmt := range b.Stmts {
		l.lowerStmt(stmt)
	}
}

func (l *lowerer) lowerStmt(stmt loneanalyzer.TStmt) {
	if l.curBlk == nil {
		return
	}

	switch s := stmt.(type) {
	case *loneanalyzer.TVarDec:
		l.lowerVarDec(s)

	case *loneanalyzer.TAssignStmt:
		l.lowerAssignStmt(s)

	case *loneanalyzer.TReturnStmt:
		var retOp MirOperand
		if s.Value != nil {
			if l.curStructRet != nil {
				srcAddr := l.lowerExpr(s.Value)
				sretOp := MirReg{Name: "__sret"}
				l.emitMemCopy(sretOp, srcAddr, l.curStructRet.Size)
				retOp = sretOp
			} else {
				retOp = l.lowerExpr(s.Value)
			}
		}
		l.curBlk.Terminator = MirReturn{Value: retOp}
		l.curBlk = nil

	case *loneanalyzer.TDropStmt:
		l.lowerExpr(s.Expr)

	case *loneanalyzer.TExprStmt:
		l.lowerExpr(s.Expr)

	case *loneanalyzer.TBlock:
		l.lowerBlock(s)

	case *loneanalyzer.TIfStmt:
		condOp := l.lowerExpr(s.Cond)
		thenBlk := l.freshBlock("if_then")
		var elseBlk *MirBlock
		hasElse := s.Else != nil && len(s.Else.Stmts) > 0
		if hasElse {
			elseBlk = l.freshBlock("if_else")
		}
		joinBlk := l.freshBlock("if_join")

		elseTarget := joinBlk.Name
		if elseBlk != nil {
			elseTarget = elseBlk.Name
		}

		l.curBlk.Terminator = MirBranch{
			Cond:       condOp,
			ThenTarget: thenBlk.Name,
			ElseTarget: elseTarget,
			JoinTarget: joinBlk.Name,
		}

		// Lower then block
		l.curBlk = thenBlk
		l.lowerBlock(&s.Then)
		if l.curBlk != nil && l.curBlk.Terminator == nil {
			l.curBlk.Terminator = MirJump{Target: joinBlk.Name}
		}

		// Lower else block if exists
		if elseBlk != nil {
			l.curBlk = elseBlk
			l.lowerBlock(s.Else)
			if l.curBlk != nil && l.curBlk.Terminator == nil {
				l.curBlk.Terminator = MirJump{Target: joinBlk.Name}
			}
		}

		l.curBlk = joinBlk

	case *loneanalyzer.TWhileStmt:
		headBlk := l.freshBlock("while_head")
		headBlk.IsLoop = true
		bodyBlk := l.freshBlock("while_body")
		exitBlk := l.freshBlock("while_exit")
		headBlk.LoopExit = exitBlk.Name

		l.curBlk.Terminator = MirJump{Target: headBlk.Name}

		// Header: evaluate condition
		l.curBlk = headBlk
		condOp := l.lowerExpr(s.Cond)
		l.curBlk.Terminator = MirBranch{
			Cond:       condOp,
			ThenTarget: bodyBlk.Name,
			ElseTarget: exitBlk.Name,
		}

		// Track loop exit for breaks
		l.loopExitStack = append(l.loopExitStack, exitBlk.Name)

		// Body
		l.curBlk = bodyBlk
		l.lowerBlock(&s.Body)
		if l.curBlk != nil && l.curBlk.Terminator == nil {
			l.curBlk.Terminator = MirJump{Target: headBlk.Name}
		}

		l.loopExitStack = l.loopExitStack[:len(l.loopExitStack)-1]
		l.curBlk = exitBlk

	case *loneanalyzer.TDoWhileStmt:
		bodyBlk := l.freshBlock("dowhile_body")
		bodyBlk.IsDoWhile = true
		condBlk := l.freshBlock("dowhile_cond")
		exitBlk := l.freshBlock("dowhile_exit")
		bodyBlk.LoopExit = exitBlk.Name

		// Jump directly into the body first (do-while guarantee)
		l.curBlk.Terminator = MirJump{Target: bodyBlk.Name}

		// Track loop exit for breaks
		l.loopExitStack = append(l.loopExitStack, exitBlk.Name)

		// Body
		l.curBlk = bodyBlk
		l.lowerBlock(&s.Body)
		if l.curBlk != nil && l.curBlk.Terminator == nil {
			l.curBlk.Terminator = MirJump{Target: condBlk.Name}
		}

		// Condition block: evaluate condition after each iteration
		l.curBlk = condBlk
		condOp := l.lowerExpr(s.Cond)
		l.curBlk.Terminator = MirBranch{
			Cond:       condOp,
			ThenTarget: bodyBlk.Name,
			ElseTarget: exitBlk.Name,
		}

		l.loopExitStack = l.loopExitStack[:len(l.loopExitStack)-1]
		l.curBlk = exitBlk

	case *loneanalyzer.TLabelDec:
		exitBlk := l.freshBlock("label_exit")
		l.loopExitStack = append(l.loopExitStack, exitBlk.Name)
		l.lowerBlock(&s.Body)
		if l.curBlk != nil && l.curBlk.Terminator == nil {
			l.curBlk.Terminator = MirJump{Target: exitBlk.Name}
		}
		l.loopExitStack = l.loopExitStack[:len(l.loopExitStack)-1]
		l.curBlk = exitBlk

	case *loneanalyzer.TBreakStmt:
		if len(l.loopExitStack) > 0 {
			target := l.loopExitStack[len(l.loopExitStack)-1]
			l.curBlk.Terminator = MirJump{Target: target}
		} else {
			l.curBlk.Terminator = MirReturn{}
		}
		l.curBlk = nil
	}
}

func (l *lowerer) lowerVarDec(s *loneanalyzer.TVarDec) {
	isUninit := false
	if lit, ok := s.Value.(*loneanalyzer.TLiteralExpr); ok && lit.Kind == loneast.LITERAL_UNINITIALIZED {
		isUninit = true
	}

	// Track string variable lengths for legacy #print intrinsic
	if lit, ok := s.Value.(*loneanalyzer.TLiteralExpr); ok {
		if strVal, ok := lit.Value.(string); ok {
			l.stringVarLens[s.Name] = len(strVal)
		}
	} else if ref, ok := s.Value.(*loneanalyzer.TVarRefExpr); ok {
		if strLen, ok := l.stringVarLens[ref.Name]; ok {
			l.stringVarLens[s.Name] = strLen
		}
	}

	// 1. Struct variable: lives on the linear datastack frame (register holds i32 base address)
	if st, isStruct := s.Type.(*loneanalyzer.StructType); isStruct {
		slotOff := l.allocSlot(st.Size, st.Align)
		addrReg := l.emitSlotAddr(slotOff)
		l.vars[s.Name] = addrReg
		if isUninit {
			l.emitMemZero(MirReg{Name: addrReg}, st.Size)
		} else if stLit, isStLit := s.Value.(*loneanalyzer.TStructLitExpr); isStLit {
			l.populateStructFields(MirReg{Name: addrReg}, st, stLit.Fields)
		} else {
			srcAddr := l.lowerExpr(s.Value)
			l.emitMemCopy(MirReg{Name: addrReg}, srcAddr, st.Size)
		}
		return
	}

	// 2. Fixed-size array variable ([N]T): lives on the linear datastack frame
	if lt, isList := s.Type.(*loneanalyzer.ListType); isList && lt.Size >= 0 {
		totalSize := loneanalyzer.TypeSize(lt)
		align := loneanalyzer.TypeAlign(lt)
		slotOff := l.allocSlot(totalSize, align)
		addrReg := l.emitSlotAddr(slotOff)
		l.vars[s.Name] = addrReg

		if listLit, ok := s.Value.(*loneanalyzer.TListExpr); ok {
			elemSize := loneanalyzer.TypeSize(lt.ItemType)
			elemMirT := toMirType(lt.ItemType)
			for idx, item := range listLit.Items {
				itemOp := l.lowerExpr(item)
				off := idx * elemSize
				if isAggregateType(lt.ItemType) {
					dstElem := l.addConstOffset(MirReg{Name: addrReg}, off)
					l.emitMemCopy(dstElem, itemOp, elemSize)
				} else {
					l.curBlk.Insts = append(l.curBlk.Insts, MirStore{
						Type:   elemMirT,
						Size:   elemSize,
						Addr:   MirReg{Name: addrReg},
						Offset: off,
						Value:  itemOp,
					})
				}
			}
		} else if isUninit {
			l.emitMemZero(MirReg{Name: addrReg}, totalSize)
		}
		return
	}

	// 3. Address-taken scalar variable (&raw x or &x): lives on the linear datastack frame
	if s.AddressTaken {
		sz := loneanalyzer.TypeSize(s.Type)
		al := loneanalyzer.TypeAlign(s.Type)
		slotOff := l.allocSlot(sz, al)
		l.varSlots[s.Name] = varSlotInfo{
			offset:      slotOff,
			typ:         s.Type,
			isAggregate: false,
		}
		var valOp MirOperand = MirConst{Type: toMirType(s.Type), Value: 0}
		if !isUninit {
			valOp = l.lowerExpr(s.Value)
		}
		addrReg := l.emitSlotAddr(slotOff)
		l.curBlk.Insts = append(l.curBlk.Insts, MirStore{
			Type:   toMirType(s.Type),
			Size:   sz,
			Addr:   MirReg{Name: addrReg},
			Offset: 0,
			Value:  valOp,
		})
		return
	}

	// 4. Standard scalar variable: lives in a fast WebAssembly local register
	var valOp MirOperand = MirConst{Type: toMirType(s.Type), Value: 0}
	if !isUninit {
		valOp = l.lowerExpr(s.Value)
	}
	mType := toMirType(s.Type)
	reg := l.freshReg("v_"+s.Name, mType)
	l.vars[s.Name] = reg
	l.curBlk.Insts = append(l.curBlk.Insts, MirCopyInst{
		Dest: reg,
		Src:  valOp,
	})
}

func (l *lowerer) populateStructFields(dstAddr MirOperand, st *loneanalyzer.StructType, fields []loneanalyzer.TFieldAssign) {
	for _, f := range fields {
		sf, exists := st.Field(f.Name)
		if !exists {
			continue
		}
		fOp := l.lowerExpr(f.Expr)
		if isAggregateType(sf.Type) {
			fieldDst := l.addConstOffset(dstAddr, sf.Offset)
			l.emitMemCopy(fieldDst, fOp, sf.Size)
		} else {
			l.curBlk.Insts = append(l.curBlk.Insts, MirStore{
				Type:   toMirType(sf.Type),
				Size:   sf.Size,
				Addr:   dstAddr,
				Offset: sf.Offset,
				Value:  fOp,
			})
		}
	}
}

func (l *lowerer) addConstOffset(addr MirOperand, offset int) MirOperand {
	if offset == 0 {
		return addr
	}
	reg := l.freshReg("t_off", MirI32)
	l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
		Op:    "add",
		Type:  MirI32,
		Dest:  reg,
		Left:  addr,
		Right: MirConst{Type: MirI32, Value: offset},
	})
	return MirReg{Name: reg}
}

func (l *lowerer) lowerAssignStmt(s *loneanalyzer.TAssignStmt) {
	valOp := l.lowerExpr(s.Value)

	if vRef, ok := s.Target.(*loneanalyzer.TVarRefExpr); ok {
		if slot, hasSlot := l.varSlots[vRef.Name]; hasSlot {
			addrReg := l.emitSlotAddr(slot.offset)
			l.curBlk.Insts = append(l.curBlk.Insts, MirStore{
				Type:   toMirType(slot.typ),
				Size:   loneanalyzer.TypeSize(slot.typ),
				Addr:   MirReg{Name: addrReg},
				Offset: 0,
				Value:  valOp,
			})
			return
		}
		if isAggregateType(vRef.ExprType()) {
			if reg, exists := l.vars[vRef.Name]; exists {
				l.emitMemCopy(MirReg{Name: reg}, valOp, loneanalyzer.TypeSize(vRef.ExprType()))
				return
			}
		}
		if reg, exists := l.vars[vRef.Name]; exists {
			l.curBlk.Insts = append(l.curBlk.Insts, MirCopyInst{
				Dest: reg,
				Src:  valOp,
			})
			return
		}
		if gType, isGlobal := l.mod.Globals[vRef.Name]; isGlobal {
			l.curBlk.Insts = append(l.curBlk.Insts, MirGlobalSet{
				Name:  vRef.Name,
				Type:  gType,
				Value: valOp,
			})
			return
		}
	}

	addr, off := l.lowerAddr(s.Target)
	targetType := s.Target.ExprType()
	if isAggregateType(targetType) {
		dstAddr := l.addConstOffset(addr, off)
		l.emitMemCopy(dstAddr, valOp, loneanalyzer.TypeSize(targetType))
		return
	}

	l.curBlk.Insts = append(l.curBlk.Insts, MirStore{
		Type:   toMirType(targetType),
		Size:   loneanalyzer.TypeSize(targetType),
		Addr:   addr,
		Offset: off,
		Value:  valOp,
	})
}

func (l *lowerer) lowerAddr(expr loneanalyzer.TExpr) (MirOperand, int) {
	switch e := expr.(type) {
	case *loneanalyzer.TVarRefExpr:
		if slot, ok := l.varSlots[e.Name]; ok {
			addrReg := l.emitSlotAddr(slot.offset)
			return MirReg{Name: addrReg}, 0
		}
		if reg, ok := l.vars[e.Name]; ok {
			return MirReg{Name: reg}, 0
		}
		return MirConst{Type: MirI32, Value: 0}, 0

	case *loneanalyzer.TMemberExpr:
		if isAggregateType(e.Base.ExprType()) {
			baseAddr, baseOff := l.lowerAddr(e.Base)
			return baseAddr, baseOff + e.Offset
		}
		// Base is ptr<Struct> or ref<Struct>
		basePtr := l.lowerExpr(e.Base)
		return basePtr, e.Offset

	case *loneanalyzer.TIndexExpr:
		baseAddr := l.lowerExpr(e.Base)
		idxOp := l.lowerExpr(e.Index)
		elemSize := loneanalyzer.TypeSize(e.Typ)
		scaledIdx := idxOp
		if elemSize > 1 {
			mulReg := l.freshReg("t_idx", MirI32)
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op:    "mul",
				Type:  MirI32,
				Dest:  mulReg,
				Left:  idxOp,
				Right: MirConst{Type: MirI32, Value: elemSize},
			})
			scaledIdx = MirReg{Name: mulReg}
		}
		elemAddrReg := l.freshReg("t_elem", MirI32)
		l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
			Op:    "add",
			Type:  MirI32,
			Dest:  elemAddrReg,
			Left:  baseAddr,
			Right: scaledIdx,
		})
		return MirReg{Name: elemAddrReg}, 0

	case *loneanalyzer.TUnaryExpr:
		if e.Op == loneast.UNARY_OP_DEREF {
			return l.lowerExpr(e.Expr), 0
		}
	}

	// Fallback: allocate temporary slot on datastack and store value
	valOp := l.lowerExpr(expr)
	sz := loneanalyzer.TypeSize(expr.ExprType())
	al := loneanalyzer.TypeAlign(expr.ExprType())
	slotOff := l.allocSlot(sz, al)
	addrReg := l.emitSlotAddr(slotOff)
	l.curBlk.Insts = append(l.curBlk.Insts, MirStore{
		Type:   toMirType(expr.ExprType()),
		Size:   sz,
		Addr:   MirReg{Name: addrReg},
		Offset: 0,
		Value:  valOp,
	})
	return MirReg{Name: addrReg}, 0
}

func (l *lowerer) lowerExpr(expr loneanalyzer.TExpr) MirOperand {
	if expr == nil {
		return MirConst{Type: MirI32, Value: 0}
	}

	mType := toMirType(expr.ExprType())

	switch e := expr.(type) {
	case *loneanalyzer.TLiteralExpr:
		if strVal, ok := e.Value.(string); ok && e.Kind == loneast.LITERAL_STRING {
			offset := l.allocString(strVal)
			return MirConst{
				Type:  MirI32,
				Value: offset,
			}
		}
		return MirConst{
			Type:  mType,
			Value: e.Value,
		}

	case *loneanalyzer.TVarRefExpr:
		if slot, hasSlot := l.varSlots[e.Name]; hasSlot {
			addrReg := l.emitSlotAddr(slot.offset)
			destReg := l.freshReg("t_ld", mType)
			l.curBlk.Insts = append(l.curBlk.Insts, MirLoad{
				Dest:   destReg,
				Type:   mType,
				Size:   loneanalyzer.TypeSize(slot.typ),
				Signed: !loneanalyzer.IsUnsignedType(slot.typ),
				Addr:   MirReg{Name: addrReg},
				Offset: 0,
			})
			return MirReg{Name: destReg}
		}
		if reg, ok := l.vars[e.Name]; ok {
			return MirReg{Name: reg}
		}
		if _, ok := l.mod.Globals[e.Name]; ok {
			return MirGlobal{Name: e.Name}
		}
		return MirReg{Name: e.Name}

	case *loneanalyzer.TStructLitExpr:
		st := e.StructType
		slotOff := l.allocSlot(st.Size, st.Align)
		addrReg := l.emitSlotAddr(slotOff)
		l.populateStructFields(MirReg{Name: addrReg}, st, e.Fields)
		return MirReg{Name: addrReg}

	case *loneanalyzer.TTraitMatExpr:
		// Zero-sized compile-time function bundle
		return MirConst{Type: MirI32, Value: 0}

	case *loneanalyzer.TMemberExpr:
		addr, off := l.lowerAddr(e)
		if isAggregateType(e.Typ) {
			return l.addConstOffset(addr, off)
		}
		destReg := l.freshReg("t_fld", mType)
		l.curBlk.Insts = append(l.curBlk.Insts, MirLoad{
			Dest:   destReg,
			Type:   mType,
			Size:   loneanalyzer.TypeSize(e.Typ),
			Signed: !loneanalyzer.IsUnsignedType(e.Typ),
			Addr:   addr,
			Offset: off,
		})
		return MirReg{Name: destReg}

	case *loneanalyzer.TIndexExpr:
		addr, off := l.lowerAddr(e)
		if isAggregateType(e.Typ) {
			return l.addConstOffset(addr, off)
		}
		destReg := l.freshReg("t_idxld", mType)
		l.curBlk.Insts = append(l.curBlk.Insts, MirLoad{
			Dest:   destReg,
			Type:   mType,
			Size:   loneanalyzer.TypeSize(e.Typ),
			Signed: !loneanalyzer.IsUnsignedType(e.Typ),
			Addr:   addr,
			Offset: off,
		})
		return MirReg{Name: destReg}

	case *loneanalyzer.TBinaryExpr:
		leftOp := l.lowerExpr(e.Left)
		rightOp := l.lowerExpr(e.Right)

		// Pointer arithmetic scaling: ptr<T> +/- n -> ptr + (n * sizeof(T))
		if pt, isPtr := e.Left.ExprType().(*loneanalyzer.PtrType); isPtr && (e.Op == "+" || e.Op == "-") {
			elemSize := loneanalyzer.TypeSize(pt.Elem)
			if elemSize > 1 {
				scaledReg := l.freshReg("t_pscale", MirI32)
				l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
					Op:    "mul",
					Type:  MirI32,
					Dest:  scaledReg,
					Left:  rightOp,
					Right: MirConst{Type: MirI32, Value: elemSize},
				})
				rightOp = MirReg{Name: scaledReg}
			}
		}

		destReg := l.freshReg("t_bin", mType)

		switch string(e.Op) {
		case "+":
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "add", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "-":
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "sub", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "*":
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "mul", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "/":
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "div", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "%":
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "mod", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "and", "&", "&&":
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "and", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "or", "|", "||":
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "or", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "^", "xor":
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "xor", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "<":
			l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
				Op: "lt", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "<=":
			l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
				Op: "le", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case ">":
			l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
				Op: "gt", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case ">=":
			l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
				Op: "ge", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "==":
			l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
				Op: "eq", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "!=":
			l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
				Op: "ne", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		default:
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "add", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		}
		return MirReg{Name: destReg}

	case *loneanalyzer.TUnaryExpr:
		switch e.Op {
		case loneast.UNARY_OP_RAW_REF, loneast.UNARY_OP_REF:
			if isAggregateType(e.Expr.ExprType()) {
				return l.lowerExpr(e.Expr)
			}
			addr, off := l.lowerAddr(e.Expr)
			return l.addConstOffset(addr, off)

		case loneast.UNARY_OP_DEREF:
			ptrOp := l.lowerExpr(e.Expr)
			if isAggregateType(e.Typ) {
				return ptrOp
			}
			destReg := l.freshReg("t_deref", mType)
			l.curBlk.Insts = append(l.curBlk.Insts, MirLoad{
				Dest:   destReg,
				Type:   mType,
				Size:   loneanalyzer.TypeSize(e.Typ),
				Signed: !loneanalyzer.IsUnsignedType(e.Typ),
				Addr:   ptrOp,
				Offset: 0,
			})
			return MirReg{Name: destReg}

		case loneast.UNARY_OP_NEGATIVE:
			subOp := l.lowerExpr(e.Expr)
			destReg := l.freshReg("t_un", mType)
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "sub", Type: mType, Dest: destReg,
				Left:  MirConst{Type: mType, Value: 0},
				Right: subOp,
			})
			return MirReg{Name: destReg}

		case loneast.UNARY_OP_NOT:
			subOp := l.lowerExpr(e.Expr)
			destReg := l.freshReg("t_un", mType)
			l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
				Op: "eq", Type: mType, Dest: destReg,
				Left:  subOp,
				Right: MirConst{Type: mType, Value: 0},
			})
			return MirReg{Name: destReg}

		default:
			return l.lowerExpr(e.Expr)
		}

	case *loneanalyzer.TCallExpr:
		calleeName := "unknown"
		if ref, ok := e.Callee.(*loneanalyzer.TVarRefExpr); ok {
			calleeName = ref.Name
		}

		// Built-in memory intrinsics
		if calleeName == "#heap_alloc" {
			l.mod.UsesHeap = true
			var sizeOp MirOperand = MirConst{Type: MirI32, Value: 0}
			if len(e.Args) > 0 {
				sizeOp = l.lowerExpr(e.Args[0])
			}
			destReg := l.freshReg("t_heap", MirI32)
			l.curBlk.Insts = append(l.curBlk.Insts, MirHeapAlloc{
				Dest: destReg,
				Size: sizeOp,
			})
			return MirReg{Name: destReg}
		}

		if calleeName == "#ptr_to_str" || calleeName == "#str_to_ptr" {
			if len(e.Args) > 0 {
				return l.lowerExpr(e.Args[0])
			}
			return MirConst{Type: MirI32, Value: 0}
		}

		// Handle legacy built-in intrinsic: #print
		if calleeName == "#print" {
			var ptrOp MirOperand = MirConst{Type: MirI32, Value: 0}
			var lenOp MirOperand = MirConst{Type: MirI32, Value: 0}

			if len(e.Args) > 0 {
				arg := e.Args[0]
				if lit, ok := arg.(*loneanalyzer.TLiteralExpr); ok {
					if strVal, ok := lit.Value.(string); ok {
						offset := l.allocString(strVal)
						ptrOp = MirConst{Type: MirI32, Value: offset}
						lenOp = MirConst{Type: MirI32, Value: len(strVal)}
					} else {
						ptrOp = l.lowerExpr(arg)
					}
				} else if ref, ok := arg.(*loneanalyzer.TVarRefExpr); ok {
					ptrOp = l.lowerExpr(arg)
					if strLen, ok := l.stringVarLens[ref.Name]; ok {
						lenOp = MirConst{Type: MirI32, Value: strLen}
					}
				} else {
					ptrOp = l.lowerExpr(arg)
				}
			}

			l.curBlk.Insts = append(l.curBlk.Insts, MirCallInst{
				Dest:   "",
				Callee: "__lone_rt_print",
				Args:   []MirOperand{ptrOp, lenOp},
			})
			return MirConst{Type: MirVoid, Value: 0}
		}

		var args []MirOperand
		var sretAddrReg string
		if stRet, isSret := l.structReturns[calleeName]; isSret {
			slotOff := l.allocSlot(stRet.Size, stRet.Align)
			sretAddrReg = l.emitSlotAddr(slotOff)
			args = append(args, MirReg{Name: sretAddrReg})
		}

		for _, arg := range e.Args {
			args = append(args, l.lowerExpr(arg))
		}

		var destReg string
		if sretAddrReg != "" {
			destReg = l.freshReg("t_sret", MirI32)
		} else if mType != MirVoid {
			destReg = l.freshReg("t_call", mType)
		}

		l.curBlk.Insts = append(l.curBlk.Insts, MirCallInst{
			Dest:   destReg,
			Callee: calleeName,
			Args:   args,
		})

		if sretAddrReg != "" {
			return MirReg{Name: sretAddrReg}
		}
		if destReg != "" {
			return MirReg{Name: destReg}
		}
		return MirConst{Type: MirVoid, Value: 0}

	case *loneanalyzer.TDropStmt:
		return l.lowerExpr(e.Expr)

	case *loneanalyzer.TBlock:
		l.lowerBlock(e)
		return MirConst{Type: MirVoid, Value: 0}

	case *loneanalyzer.TIsExpr:
		destReg := l.freshReg("t_is", MirI32)
		optType, isOpt := e.Expr.ExprType().(*loneanalyzer.OptionalType)

		if isOpt {
			valOp := l.lowerExpr(e.Expr)
			if e.TargetType == loneanalyzer.TypeNone {
				op := "eq"
				if e.Negated {
					op = "ne"
				}
				l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
					Op:    op,
					Type:  toMirType(optType),
					Dest:  destReg,
					Left:  valOp,
					Right: MirConst{Type: toMirType(optType), Value: 0},
				})
				return MirReg{Name: destReg}
			}

			if optType.Inner.AssignableTo(e.TargetType) || e.TargetType.AssignableTo(optType.Inner) {
				op := "ne"
				if e.Negated {
					op = "eq"
				}
				l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
					Op:    op,
					Type:  toMirType(optType),
					Dest:  destReg,
					Left:  valOp,
					Right: MirConst{Type: toMirType(optType), Value: 0},
				})
				return MirReg{Name: destReg}
			}

			val := 0
			if e.Negated {
				val = 1
			}
			return MirConst{Type: MirI32, Value: val}
		}

		if _, isVar := e.Expr.(*loneanalyzer.TVarRefExpr); !isVar {
			_ = l.lowerExpr(e.Expr)
		}
		srcType := e.Expr.ExprType()

		if e.TargetType == loneanalyzer.TypeNone {
			val := 0
			if e.Negated {
				val = 1
			}
			return MirConst{Type: MirI32, Value: val}
		}

		if srcType.AssignableTo(e.TargetType) {
			val := 1
			if e.Negated {
				val = 0
			}
			return MirConst{Type: MirI32, Value: val}
		}

		val := 0
		if e.Negated {
			val = 1
		}
		return MirConst{Type: MirI32, Value: val}

	default:
		return MirConst{Type: MirI32, Value: 0}
	}
}

func toMirType(t loneanalyzer.Type) MirT {
	if t == nil {
		return MirI32
	}
	switch t.String() {
	case "i8", "i16", "i32", "u8", "u16", "u32", "bool", "char", "str":
		return MirI32
	case "i64", "u64", "i128", "u128":
		return MirI64
	case "f32":
		return MirF32
	case "f64":
		return MirF64
	case "void":
		return MirVoid
	default:
		return MirI32
	}
}
