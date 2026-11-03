package loneanalyzer

import (
	"fmt"
	loneast "lone/pkg2/lone-parser-ast"
	"strings"
	"unicode"
)

// TypeEvaluator performs type checking, symbol resolution, and immutability validation.
type TypeEvaluator struct {
	stack           *ScopeStack
	diags           []Diagnostic
	structs         map[string]*StructType
	traits          map[string]*TraitType
	currentSelfType Type
}

func NewTypeEvaluator() *TypeEvaluator {
	return &TypeEvaluator{
		stack:   NewScopeStack(),
		structs: make(map[string]*StructType),
		traits:  make(map[string]*TraitType),
	}
}

func (te *TypeEvaluator) Diagnostics() []Diagnostic {
	return te.diags
}

func (te *TypeEvaluator) addDiag(sev Severity, span loneast.Span, rule, msg string) {
	te.diags = append(te.diags, Diagnostic{
		Severity: sev,
		Span:     span,
		Rule:     rule,
		Message:  msg,
	})
}

// EvaluateProgram executes a full semantic evaluation pass on a slice of AST declarations.
func (te *TypeEvaluator) EvaluateProgram(decls []any) *TProgram {
	// Pass 1: Hoist structs, traits, and aliases
	for _, decl := range decls {
		if tdec, ok := decl.(loneast.TYPE_DEC); ok {
			te.hoistTypeDec(&tdec)
		} else if ptdec, ok := decl.(*loneast.TYPE_DEC); ok {
			te.hoistTypeDec(ptdec)
		}
	}

	// Pass 2: Hoist function signatures, impl blocks, global variables, and host capability directives
	for _, decl := range decls {
		switch d := decl.(type) {
		case loneast.FN_DEC:
			te.hoistFn(&d)
		case *loneast.FN_DEC:
			te.hoistFn(d)
		case loneast.BIND_DEC:
			te.hoistVar(&d)
		case *loneast.BIND_DEC:
			te.hoistVar(d)
		case loneast.WasmDirective:
			te.hoistWasmDirective(&d)
		case *loneast.WasmDirective:
			te.hoistWasmDirective(d)
		case loneast.ImportDecl:
			te.hoistImport(&d)
		case *loneast.ImportDecl:
			te.hoistImport(d)
		case loneast.IMPL_DEC:
			te.hoistImpl(&d)
		case *loneast.IMPL_DEC:
			te.hoistImpl(d)
		}
	}

	// Pass 3: Analyze bodies
	var tDecls []TNode
	for _, decl := range decls {
		td := te.analyzeTopLevel(decl)
		if td != nil {
			tDecls = append(tDecls, td)
		}
	}

	return &TProgram{Decls: tDecls}
}

// =====================
// HOISTING
// =====================

func (te *TypeEvaluator) hoistTypeDec(t *loneast.TYPE_DEC) {
	// Validate Trait Naming Convention: Tr + PascalCase
	if t.Kind == loneast.TYPE_DEC_TRAIT {
		if !isTrNaming(t.Name) {
			te.addDiag(SeverityError, t.Span_, "convention/trait-naming",
				fmt.Sprintf("trait %q must begin with 'Tr' followed by an uppercase letter (e.g. 'Tr%s')", t.Name, t.Name))
		}
	} else {
		// Non-trait types must NOT start with Tr + uppercase
		if isTrNaming(t.Name) {
			te.addDiag(SeverityError, t.Span_, "convention/non-trait-naming",
				fmt.Sprintf("non-trait type %q must not begin with 'Tr' prefix reserved for traits", t.Name))
		}
	}

	if t.Kind == loneast.TYPE_DEC_STRUCT {
		st := &StructType{
			Name:     t.Name,
			FieldMap: make(map[string]StructFieldInfo),
			Methods:  make(map[string]*FnType),
		}

		offset := 0
		maxAlign := 1
		for _, f := range t.Fields {
			fType := te.resolveType(f.Type)
			sz := TypeSize(fType)
			al := TypeAlign(fType)
			if al > maxAlign {
				maxAlign = al
			}
			// Align offset
			if al > 0 && offset%al != 0 {
				offset += al - (offset % al)
			}
			info := StructFieldInfo{Name: f.Name, Type: fType, Offset: offset}
			st.Fields = append(st.Fields, info)
			st.FieldMap[f.Name] = info
			offset += sz
		}

		if maxAlign > 0 && offset%maxAlign != 0 {
			offset += maxAlign - (offset % maxAlign)
		}
		st.TotalSize = offset
		st.TotalAlign = maxAlign

		for _, m := range t.Methods {
			var mParams []FnParamInfo
			for _, p := range m.Params {
				mParams = append(mParams, FnParamInfo{Name: p.Name, Type: te.resolveType(p.Type)})
			}
			mRet := Type(TypeVoid)
			if m.ReturnType != nil {
				mRet = te.resolveType(*m.ReturnType)
			}
			st.Methods[m.Name] = &FnType{Params: mParams, Return: mRet}
		}

		te.structs[t.Name] = st
		_ = te.stack.Define(&Symbol{Name: t.Name, Kind: SymbolType, Type: st, Node: t})
	}

	if t.Kind == loneast.TYPE_DEC_TRAIT {
		tr := &TraitType{
			Name:    t.Name,
			Methods: make(map[string]*FnType),
		}
		for _, sig := range t.TraitSigs {
			var params []FnParamInfo
			for _, p := range sig.Params {
				pType := te.resolveType(p.Type)
				params = append(params, FnParamInfo{Name: p.Name, Type: pType})
			}
			retType := Type(TypeVoid)
			if sig.ReturnType != nil {
				retType = te.resolveType(*sig.ReturnType)
			}
			tr.Methods[sig.Name] = &FnType{Params: params, Return: retType}
		}
		te.traits[t.Name] = tr
		_ = te.stack.Define(&Symbol{Name: t.Name, Kind: SymbolType, Type: tr, Node: t})
	}
}

func (te *TypeEvaluator) hoistFn(f *loneast.FN_DEC) {
	var params []FnParamInfo
	for _, p := range f.Params {
		pType := te.resolveType(p.Type)
		params = append(params, FnParamInfo{Name: p.Name, Type: pType})
	}
	retType := Type(TypeVoid)
	if f.ReturnType != nil {
		retType = te.resolveType(*f.ReturnType)
	}
	fnType := &FnType{Params: params, Return: retType}
	_ = te.stack.Define(&Symbol{Name: f.Name, Kind: SymbolFn, Type: fnType, Node: f})
}

func (te *TypeEvaluator) hoistVar(b *loneast.BIND_DEC) {
	varType := te.resolveType(b.Type)
	_ = te.stack.Define(&Symbol{
		Name:      b.Name,
		Kind:      SymbolVar,
		Type:      varType,
		IsMutable: b.IsMutable,
		Node:      b,
	})
}

func (te *TypeEvaluator) hoistWasmDirective(wd *loneast.WasmDirective) {
	if wd == nil {
		return
	}
	for _, imp := range wd.Imports {
		if imp.Source == ":wasi:snapshot_preview1" {
			for _, sym := range imp.Symbols {
				symName := strings.TrimPrefix(sym, ".")
				switch symName {
				case "fd_write":
					_ = te.stack.Define(&Symbol{
						Name: symName,
						Kind: SymbolFn,
						Type: &FnType{
							Params: []FnParamInfo{
								{Name: "fd", Type: TypeI32},
								{Name: "iovs", Type: &PtrType{Elem: TypeAny}},
								{Name: "iovs_len", Type: TypeI32},
								{Name: "nwritten", Type: &PtrType{Elem: TypeI32}},
							},
							Return: TypeI32,
						},
					})
				case "fd_read":
					_ = te.stack.Define(&Symbol{
						Name: symName,
						Kind: SymbolFn,
						Type: &FnType{
							Params: []FnParamInfo{
								{Name: "fd", Type: TypeI32},
								{Name: "iovs", Type: &PtrType{Elem: TypeAny}},
								{Name: "iovs_len", Type: TypeI32},
								{Name: "nread", Type: &PtrType{Elem: TypeI32}},
							},
							Return: TypeI32,
						},
					})
				case "fd_close":
					_ = te.stack.Define(&Symbol{
						Name: symName,
						Kind: SymbolFn,
						Type: &FnType{
							Params: []FnParamInfo{
								{Name: "fd", Type: TypeI32},
							},
							Return: TypeI32,
						},
					})
				case "proc_exit":
					_ = te.stack.Define(&Symbol{
						Name: symName,
						Kind: SymbolFn,
						Type: &FnType{
							Params: []FnParamInfo{
								{Name: "code", Type: TypeI32},
							},
							Return: TypeVoid,
						},
					})
				case "random_get":
					_ = te.stack.Define(&Symbol{
						Name: symName,
						Kind: SymbolFn,
						Type: &FnType{
							Params: []FnParamInfo{
								{Name: "buf", Type: &PtrType{Elem: TypeAny}},
								{Name: "buf_len", Type: TypeI32},
							},
							Return: TypeI32,
						},
					})
				}
			}
		}
	}
}

func (te *TypeEvaluator) hoistImport(imp *loneast.ImportDecl) {
	if imp == nil {
		return
	}
	if imp.Source == ":std:sys" {
		for _, sym := range imp.Symbols {
			symName := strings.TrimPrefix(sym, ".")
			switch symName {
			case "STDIN_FD", "STDOUT_FD", "STDERR_FD":
				_ = te.stack.Define(&Symbol{Name: symName, Kind: SymbolVar, Type: TypeI32, IsMutable: false})
			case "sys_write":
				_ = te.stack.Define(&Symbol{
					Name: symName,
					Kind: SymbolFn,
					Type: &FnType{
						Params: []FnParamInfo{
							{Name: "fd", Type: TypeI32},
							{Name: "buf", Type: &PtrType{Elem: TypeU8}},
							{Name: "len", Type: TypeI32},
						},
						Return: TypeI32,
					},
				})
			case "sys_read":
				_ = te.stack.Define(&Symbol{
					Name: symName,
					Kind: SymbolFn,
					Type: &FnType{
						Params: []FnParamInfo{
							{Name: "fd", Type: TypeI32},
							{Name: "buf", Type: &PtrType{Elem: TypeU8}},
							{Name: "max_len", Type: TypeI32},
						},
						Return: TypeI32,
					},
				})
			case "sys_random":
				_ = te.stack.Define(&Symbol{
					Name: symName,
					Kind: SymbolFn,
					Type: &FnType{
						Params: []FnParamInfo{
							{Name: "buf", Type: &PtrType{Elem: TypeAny}},
							{Name: "len", Type: TypeI32},
						},
						Return: TypeI32,
					},
				})
			case "sys_close":
				_ = te.stack.Define(&Symbol{
					Name: symName,
					Kind: SymbolFn,
					Type: &FnType{
						Params: []FnParamInfo{{Name: "fd", Type: TypeI32}},
						Return: TypeI32,
					},
				})
			case "sys_exit":
				_ = te.stack.Define(&Symbol{
					Name: symName,
					Kind: SymbolFn,
					Type: &FnType{
						Params: []FnParamInfo{{Name: "code", Type: TypeI32}},
						Return: TypeVoid,
					},
				})
			}
		}
	}
}

func (te *TypeEvaluator) hoistImpl(imp *loneast.IMPL_DEC) {
	if imp == nil {
		return
	}
	targetType := te.resolveType(imp.TargetType)
	if st, ok := targetType.(*StructType); ok {
		for _, m := range imp.Methods {
			var mParams []FnParamInfo
			for _, p := range m.Params {
				pType := te.resolveType(p.Type)
				if p.Name == "self" && (pType == TypeInfer || pType == TypeUnknown) {
					pType = targetType
				}
				mParams = append(mParams, FnParamInfo{Name: p.Name, Type: pType})
			}
			mRet := Type(TypeVoid)
			if m.ReturnType != nil {
				mRet = te.resolveType(*m.ReturnType)
			}
			st.Methods[m.Name] = &FnType{Params: mParams, Return: mRet}
		}
	}
}

// =====================
// ANALYSIS
// =====================

func (te *TypeEvaluator) analyzeTopLevel(decl any) TNode {
	switch d := decl.(type) {
	case loneast.BIND_DEC:
		return te.analyzeVarDec(&d)
	case *loneast.BIND_DEC:
		return te.analyzeVarDec(d)

	case loneast.FN_DEC:
		return te.analyzeFnDec(&d)
	case *loneast.FN_DEC:
		return te.analyzeFnDec(d)

	case loneast.TYPE_DEC:
		if st, ok := te.structs[d.Name]; ok {
			return &TStructDec{Name: d.Name, Type: st, Span_: d.Span_}
		}
		if tr, ok := te.traits[d.Name]; ok {
			return &TTraitDec{Name: d.Name, Type: tr, Span_: d.Span_}
		}
		return nil

	case loneast.IMPL_DEC:
		return te.analyzeImplDec(&d)
	case *loneast.IMPL_DEC:
		return te.analyzeImplDec(d)

	case loneast.IfStmt:
		return te.analyzeIfStmt(&d, TypeVoid)
	case *loneast.IfStmt:
		return te.analyzeIfStmt(d, TypeVoid)

	case loneast.PackedIfStmt:
		return te.analyzePackedIfStmt(&d, TypeVoid)
	case *loneast.PackedIfStmt:
		return te.analyzePackedIfStmt(d, TypeVoid)

	case loneast.WhileStmt:
		return te.analyzeWhileStmt(&d, TypeVoid)
	case *loneast.WhileStmt:
		return te.analyzeWhileStmt(d, TypeVoid)

	case loneast.LoopStmt:
		return te.analyzeLoopStmt(&d, TypeVoid)
	case *loneast.LoopStmt:
		return te.analyzeLoopStmt(d, TypeVoid)

	case loneast.ForStmt:
		return te.analyzeForStmt(&d, TypeVoid)
	case *loneast.ForStmt:
		return te.analyzeForStmt(d, TypeVoid)

	case loneast.AssignStmt:
		return te.analyzeAssignStmt(&d)
	case *loneast.AssignStmt:
		return te.analyzeAssignStmt(d)

	case loneast.DropStmt:
		expr := te.EvalExpr(d.Expr)
		return &TDropStmt{Expr: expr, Span_: d.Span_}
	case *loneast.DropStmt:
		expr := te.EvalExpr(d.Expr)
		return &TDropStmt{Expr: expr, Span_: d.Span_}

	case loneast.LabelDec:
		return te.analyzeLabelDec(&d, TypeVoid)
	case *loneast.LabelDec:
		return te.analyzeLabelDec(d, TypeVoid)

	default:
		return nil
	}
}

func (te *TypeEvaluator) analyzeVarDec(b *loneast.BIND_DEC) *TVarDec {
	declaredType := te.resolveType(b.Type)
	tVal := te.EvalExpr(b.Value)

	resolvedType := declaredType
	if declaredType.Equals(TypeInfer) || declaredType.Equals(TypeUnknown) {
		if tVal != nil && tVal.ExprType() != TypeUnknown {
			resolvedType = tVal.ExprType()
		} else {
			resolvedType = TypeUnknown
		}
	} else if tVal != nil && tVal.ExprType() != TypeUnknown && !tVal.ExprType().Equals(TypeInfer) {
		// Type mismatch check
		if !tVal.ExprType().AssignableTo(declaredType) {
			te.addDiag(SeverityError, b.Span_, "type/mismatch",
				fmt.Sprintf("cannot assign value of type %q to binding %q of type %q",
					tVal.ExprType().String(), b.Name, declaredType.String()))
		}
	}

	if existing := te.stack.LookupLocal(b.Name); existing != nil {
		existing.Type = resolvedType
		existing.IsMutable = b.IsMutable
	} else {
		sym := &Symbol{
			Name:      b.Name,
			Kind:      SymbolVar,
			Type:      resolvedType,
			IsMutable: b.IsMutable,
			Node:      b,
		}
		_ = te.stack.Define(sym)
	}

	return &TVarDec{
		Name:      b.Name,
		IsMutable: b.IsMutable,
		Type:      resolvedType,
		Value:     tVal,
		Span_:     b.Span_,
	}
}

func (te *TypeEvaluator) analyzeFnDec(f *loneast.FN_DEC) *TFnDec {
	te.stack.Push(ScopeFunction)
	defer te.stack.Pop()

	var tParams []TFnParam
	for _, p := range f.Params {
		pType := te.resolveType(p.Type)
		if p.Name == "self" && (pType == TypeUnknown || pType == TypeInfer || p.Type.Value == nil) {
			if te.currentSelfType != nil {
				pType = te.currentSelfType
			}
		}
		_ = te.stack.Define(&Symbol{Name: p.Name, Kind: SymbolVar, Type: pType, IsMutable: false})
		tParams = append(tParams, TFnParam{Name: p.Name, Type: pType, Span_: p.Span_})
	}

	retType := Type(TypeVoid)
	if f.ReturnType != nil {
		retType = te.resolveType(*f.ReturnType)
	}

	body := te.analyzeBlock(&f.Body, retType)

	return &TFnDec{
		Name:       f.Name,
		Params:     tParams,
		ReturnType: retType,
		Body:       body,
		Span_:      f.Span_,
	}
}

func (te *TypeEvaluator) analyzeImplDec(i *loneast.IMPL_DEC) *TImplDec {
	targetType := te.resolveType(i.TargetType)
	oldSelf := te.currentSelfType
	te.currentSelfType = targetType
	defer func() { te.currentSelfType = oldSelf }()

	var tMethods []*TFnDec

	for _, m := range i.Methods {
		tMethod := te.analyzeFnDec(&m)
		tMethods = append(tMethods, tMethod)
	}

	return &TImplDec{
		TraitName:  i.TraitName,
		TargetType: targetType,
		Methods:    tMethods,
		Span_:      i.Span_,
	}
}

func (te *TypeEvaluator) analyzeBlock(b *loneast.Block, expectedRet Type) TBlock {
	te.stack.Push(ScopeBlock)
	defer te.stack.Pop()

	var stmts []TStmt
	blockType := Type(TypeVoid)
	for _, s := range b.Stmts {
		tStmt := te.analyzeStmt(s, expectedRet)
		if tStmt != nil {
			stmts = append(stmts, tStmt)
			if brk, ok := tStmt.(*TBreakStmt); ok && brk.Value != nil {
				blockType = brk.Value.ExprType()
			} else if expr, ok := tStmt.(TExpr); ok {
				blockType = expr.ExprType()
			}
		}
	}

	return TBlock{
		Stmts: stmts,
		Typ:   blockType,
		Span_: b.Span_,
	}
}

func (te *TypeEvaluator) analyzeStmt(raw any, expectedRet Type) TStmt {
	switch s := raw.(type) {
	case loneast.BIND_DEC:
		return te.analyzeVarDec(&s)
	case *loneast.BIND_DEC:
		return te.analyzeVarDec(s)

	case loneast.AssignStmt:
		return te.analyzeAssignStmt(&s)
	case *loneast.AssignStmt:
		return te.analyzeAssignStmt(s)

	case loneast.ReturnStmt:
		var tVal TExpr
		if s.Value != nil {
			tVal = te.EvalExpr(s.Value)
			if expectedRet != TypeVoid && !tVal.ExprType().AssignableTo(expectedRet) {
				te.addDiag(SeverityError, s.Span_, "type/return-mismatch",
					fmt.Sprintf("cannot return value of type %q from function expecting %q",
						tVal.ExprType().String(), expectedRet.String()))
			}
		}
		return &TReturnStmt{Value: tVal, Span_: s.Span_}

	case loneast.BreakStmt:
		var tVal TExpr
		if s.Value != nil {
			tVal = te.EvalExpr(s.Value)
		}
		return &TBreakStmt{Label: s.Label, Value: tVal, Span_: s.Span_}

	case loneast.DropStmt:
		tExpr := te.EvalExpr(s.Expr)
		return &TDropStmt{Expr: tExpr, Span_: s.Span_}

	case loneast.IfStmt:
		return te.analyzeIfStmt(&s, expectedRet)
	case *loneast.IfStmt:
		return te.analyzeIfStmt(s, expectedRet)

	case loneast.PackedIfStmt:
		return te.analyzePackedIfStmt(&s, expectedRet)
	case *loneast.PackedIfStmt:
		return te.analyzePackedIfStmt(s, expectedRet)

	case loneast.WhileStmt:
		return te.analyzeWhileStmt(&s, expectedRet)
	case *loneast.WhileStmt:
		return te.analyzeWhileStmt(s, expectedRet)

	case loneast.LoopStmt:
		return te.analyzeLoopStmt(&s, expectedRet)
	case *loneast.LoopStmt:
		return te.analyzeLoopStmt(s, expectedRet)

	case loneast.ForStmt:
		return te.analyzeForStmt(&s, expectedRet)
	case *loneast.ForStmt:
		return te.analyzeForStmt(s, expectedRet)

	case loneast.LabelDec:
		return te.analyzeLabelDec(&s, expectedRet)
	case *loneast.LabelDec:
		return te.analyzeLabelDec(s, expectedRet)

	case loneast.Expr:
		return te.EvalExpr(s)

	default:
		return nil
	}
}

// analyzeAssignStmt enforces IL1 mutability invariant: cannot reassign immutable 'let' bindings.
func (te *TypeEvaluator) analyzeAssignStmt(a *loneast.AssignStmt) *TAssignStmt {
	tTarget := te.EvalExpr(a.Target)
	tVal := te.EvalExpr(a.Value)

	// Check if target is a variable reference and whether it is immutable
	if vref, ok := tTarget.(*TVarRefExpr); ok {
		sym := te.stack.Lookup(vref.Name)
		if sym != nil && !sym.IsMutable {
			te.addDiag(SeverityError, a.Span_, "type/immutable-reassignment",
				fmt.Sprintf("cannot reassign immutable binding %q declared with 'let'; use 'var' if mutation is intended", sym.Name))
		}
	}

	if tTarget.ExprType() != TypeUnknown && tVal.ExprType() != TypeUnknown {
		if !tVal.ExprType().AssignableTo(tTarget.ExprType()) {
			te.addDiag(SeverityError, a.Span_, "type/assignment-mismatch",
				fmt.Sprintf("cannot assign value of type %q to target of type %q",
					tVal.ExprType().String(), tTarget.ExprType().String()))
		}
	}

	return &TAssignStmt{
		Target: tTarget,
		Value:  tVal,
		Span_:  a.Span_,
	}
}

// analyzeIfStmt handles standard if with optional scoped header bindings: if: [let x = 0] x != 9 { ... }
func (te *TypeEvaluator) analyzeIfStmt(i *loneast.IfStmt, expectedRet Type) *TIfStmt {
	hasHeader := len(i.Header) > 0
	if hasHeader {
		te.stack.Push(ScopeHeader)
	}

	var tHeaders []*TVarDec
	for _, h := range i.Header {
		tVar := te.analyzeVarDec(&h)
		tHeaders = append(tHeaders, tVar)
	}

	tCond := te.EvalExpr(i.Cond)
	if !tCond.ExprType().Equals(TypeBool) && tCond.ExprType() != TypeUnknown {
		te.addDiag(SeverityError, i.Cond.NodeSpan(), "type/condition-not-bool",
			fmt.Sprintf("if condition must be boolean, got %q", tCond.ExprType().String()))
	}

	tThen := te.analyzeBlock(&i.Then, expectedRet)

	var tElse TStmt
	if i.Else != nil {
		switch el := i.Else.(type) {
		case loneast.Block:
			b := te.analyzeBlock(&el, expectedRet)
			tElse = &b
		case *loneast.Block:
			b := te.analyzeBlock(el, expectedRet)
			tElse = &b
		case loneast.IfStmt:
			tElse = te.analyzeIfStmt(&el, expectedRet)
		case *loneast.IfStmt:
			tElse = te.analyzeIfStmt(el, expectedRet)
		}
	}

	if hasHeader {
		te.stack.Pop()
	}

	return &TIfStmt{
		Header: tHeaders,
		Cond:   tCond,
		Then:   tThen,
		Else:   tElse,
		Span_:  i.Span_,
	}
}

// analyzePackedIfStmt handles packed if with optional scoped header bindings: if: [let x = 0, let y = 7] { ... }
func (te *TypeEvaluator) analyzePackedIfStmt(p *loneast.PackedIfStmt, expectedRet Type) *TPackedIfStmt {
	hasHeader := len(p.Header) > 0
	if hasHeader {
		te.stack.Push(ScopeHeader)
	}

	var tHeaders []*TVarDec
	for _, h := range p.Header {
		tVar := te.analyzeVarDec(&h)
		tHeaders = append(tHeaders, tVar)
	}

	var tArms []TPackedIfArm
	for _, arm := range p.Arms {
		var tCond TExpr
		if arm.Cond != nil {
			tCond = te.EvalExpr(arm.Cond)
			if !tCond.ExprType().Equals(TypeBool) && tCond.ExprType() != TypeUnknown {
				te.addDiag(SeverityError, arm.Cond.NodeSpan(), "type/condition-not-bool",
					fmt.Sprintf("packed if arm condition must be boolean, got %q", tCond.ExprType().String()))
			}
		}
		var tBody TStmt
		if blk, ok := arm.Body.(loneast.Block); ok {
			b := te.analyzeBlock(&blk, expectedRet)
			tBody = &b
		} else if pblk, ok := arm.Body.(*loneast.Block); ok {
			b := te.analyzeBlock(pblk, expectedRet)
			tBody = &b
		} else {
			tBody = te.analyzeStmt(arm.Body, expectedRet)
		}
		tArms = append(tArms, TPackedIfArm{
			Cond:  tCond,
			Body:  tBody,
			Span_: arm.Span_,
		})
	}

	if hasHeader {
		te.stack.Pop()
	}

	return &TPackedIfStmt{
		Header: tHeaders,
		Arms:   tArms,
		Span_:  p.Span_,
	}
}

func (te *TypeEvaluator) analyzeWhileStmt(w *loneast.WhileStmt, expectedRet Type) *TWhileStmt {
	tCond := te.EvalExpr(w.Cond)
	tBody := te.analyzeBlock(&w.Body, expectedRet)
	return &TWhileStmt{Cond: tCond, Body: tBody, Span_: w.Span_}
}

// analyzeLoopStmt handles infinite loops and post-condition loops: loop { ... } while: [cond]
func (te *TypeEvaluator) analyzeLoopStmt(l *loneast.LoopStmt, expectedRet Type) *TLoopStmt {
	te.stack.Push(ScopeLoop)
	defer te.stack.Pop()

	var stmts []TStmt
	for _, s := range l.Body.Stmts {
		tStmt := te.analyzeStmt(s, expectedRet)
		if tStmt != nil {
			stmts = append(stmts, tStmt)
		}
	}
	tBody := TBlock{Stmts: stmts, Typ: TypeVoid, Span_: l.Body.Span_}

	var tWhileCond TExpr
	if l.WhileCond != nil {
		// Post-condition has access to loop-scoped variables!
		tWhileCond = te.EvalExpr(l.WhileCond)
		if !tWhileCond.ExprType().Equals(TypeBool) && tWhileCond.ExprType() != TypeUnknown {
			te.addDiag(SeverityError, l.WhileCond.NodeSpan(), "type/condition-not-bool",
				fmt.Sprintf("post-condition loop condition must be boolean, got %q", tWhileCond.ExprType().String()))
		}
	}

	return &TLoopStmt{
		Body:      tBody,
		WhileCond: tWhileCond,
		Span_:     l.Span_,
	}
}

func (te *TypeEvaluator) analyzeForStmt(f *loneast.ForStmt, expectedRet Type) *TForStmt {
	tIter := te.EvalExpr(f.Iter)

	te.stack.Push(ScopeBlock)
	defer te.stack.Pop()

	elemType := Type(TypeI32) // default fallback
	if lt, ok := tIter.ExprType().(*ListType); ok {
		elemType = lt.Elem
	}
	_ = te.stack.Define(&Symbol{Name: f.VarName, Kind: SymbolVar, Type: elemType, IsMutable: false})

	var stmts []TStmt
	for _, s := range f.Body.Stmts {
		tStmt := te.analyzeStmt(s, expectedRet)
		if tStmt != nil {
			stmts = append(stmts, tStmt)
		}
	}
	tBody := TBlock{Stmts: stmts, Typ: TypeVoid, Span_: f.Body.Span_}

	return &TForStmt{
		VarName: f.VarName,
		Iter:    tIter,
		Body:    tBody,
		Span_:   f.Span_,
	}
}

func (te *TypeEvaluator) analyzeLabelDec(l *loneast.LabelDec, expectedRet Type) *TLabelDec {
	body := te.analyzeBlock(&l.Body, expectedRet)
	return &TLabelDec{
		Name:  l.Name,
		Body:  body,
		Typ:   body.Typ,
		Span_: l.Span_,
	}
}

// =====================
// EXPRESSION EVALUATION
// =====================

func (te *TypeEvaluator) EvalExpr(e loneast.Expr) TExpr {
	if e == nil {
		return nil
	}

	switch expr := e.(type) {
	case loneast.Literal:
		return te.evalLiteral(expr)

	case loneast.ListLiteral:
		return te.evalListLiteral(expr)

	case loneast.ObjectLiteral:
		return te.evalObjectLiteral(expr)

	case loneast.NameRef:
		sym := te.stack.Lookup(expr.Name)
		if sym == nil {
			te.addDiag(SeverityError, expr.Span_, "type/undefined-identifier",
				fmt.Sprintf("undefined identifier %q", expr.Name))
			return &TVarRefExpr{Name: expr.Name, Typ: TypeUnknown, Span_: expr.Span_}
		}
		return &TVarRefExpr{Name: expr.Name, Typ: sym.Type, Span_: expr.Span_}

	case loneast.BinaryExpr:
		return te.evalBinaryExpr(expr)

	case loneast.UnaryExpr:
		return te.evalUnaryExpr(expr)

	case loneast.CastExpr:
		return te.evalCastExpr(expr)

	case loneast.IsExpr:
		return te.evalIsExpr(expr)

	case loneast.PostFixExpr:
		return te.evalPostfixExpr(expr)

	case loneast.LabelDec:
		return te.analyzeLabelDec(&expr, TypeUnknown)
	case *loneast.LabelDec:
		return te.analyzeLabelDec(expr, TypeUnknown)

	default:
		return nil
	}
}

func (te *TypeEvaluator) evalLiteral(l loneast.Literal) TExpr {
	switch l.Kind {
	case loneast.LiteralInt:
		return &TLiteralExpr{Kind: l.Kind, Value: l.Value, Typ: TypeI32, Span_: l.Span_}
	case loneast.LiteralFloat:
		return &TLiteralExpr{Kind: l.Kind, Value: l.Value, Typ: TypeF32, Span_: l.Span_}
	case loneast.LiteralString:
		return &TLiteralExpr{Kind: l.Kind, Value: l.Value, Typ: TypeStr, Span_: l.Span_}
	case loneast.LiteralChar:
		return &TLiteralExpr{Kind: l.Kind, Value: l.Value, Typ: TypeChar, Span_: l.Span_}
	case loneast.LiteralBool:
		return &TLiteralExpr{Kind: l.Kind, Value: l.Value, Typ: TypeBool, Span_: l.Span_}
	case loneast.LiteralNone:
		return &TLiteralExpr{Kind: l.Kind, Value: nil, Typ: TypeNone, Span_: l.Span_}
	case loneast.LiteralUninitialized:
		return &TLiteralExpr{Kind: l.Kind, Value: nil, Typ: TypeInfer, Span_: l.Span_}
	default:
		return &TLiteralExpr{Kind: l.Kind, Value: l.Value, Typ: TypeUnknown, Span_: l.Span_}
	}
}

func (te *TypeEvaluator) evalListLiteral(l loneast.ListLiteral) TExpr {
	var elemType Type = TypeAny
	if len(l.Elements) > 0 {
		first := te.EvalExpr(l.Elements[0])
		elemType = first.ExprType()
	}
	return &TLiteralExpr{
		Kind:  loneast.LiteralInt, // stored as list value
		Value: l.Elements,
		Typ:   &ListType{Elem: elemType, Fixed: true, Size: len(l.Elements)},
		Span_: l.Span_,
	}
}

func (te *TypeEvaluator) evalObjectLiteral(o loneast.ObjectLiteral) TExpr {
	return &TLiteralExpr{
		Kind:  loneast.LiteralInt,
		Value: o.Fields,
		Typ:   TypeAny,
		Span_: o.Span_,
	}
}

func (te *TypeEvaluator) evalBinaryExpr(b loneast.BinaryExpr) TExpr {
	left := te.EvalExpr(b.Left)
	right := te.EvalExpr(b.Right)

	lType := left.ExprType()
	rType := right.ExprType()

	switch b.Op {
	case "+", "-", "*", "/", "%", "^", "&", "|":
		if lType.Equals(rType) {
			return &TBinaryExpr{Op: b.Op, Left: left, Right: right, Typ: lType, Span_: b.Span_}
		}
		// In Lone, binary ops require identical types; no silent int-to-float promotion!
		te.addDiag(SeverityError, b.Span_, "type/mismatch",
			fmt.Sprintf("operator %s cannot be applied to mismatched types %q and %q (use explicit 'as' cast)",
				b.Op, lType.String(), rType.String()))
		return &TBinaryExpr{Op: b.Op, Left: left, Right: right, Typ: lType, Span_: b.Span_}

	case "==", "!=", "<", "<=", ">", ">=":
		if !lType.Equals(rType) && lType != TypeUnknown && rType != TypeUnknown {
			te.addDiag(SeverityError, b.Span_, "type/comparison-mismatch",
				fmt.Sprintf("cannot compare mismatched types %q and %q", lType.String(), rType.String()))
		}
		return &TBinaryExpr{Op: b.Op, Left: left, Right: right, Typ: TypeBool, Span_: b.Span_}

	case "and", "or", "xor":
		if !lType.Equals(TypeBool) || !rType.Equals(TypeBool) {
			te.addDiag(SeverityError, b.Span_, "type/logical-not-bool",
				fmt.Sprintf("logical %s operator requires boolean operands, got %q and %q",
					b.Op, lType.String(), rType.String()))
		}
		return &TBinaryExpr{Op: b.Op, Left: left, Right: right, Typ: TypeBool, Span_: b.Span_}

	default:
		return &TBinaryExpr{Op: b.Op, Left: left, Right: right, Typ: lType, Span_: b.Span_}
	}
}

func (te *TypeEvaluator) evalUnaryExpr(u loneast.UnaryExpr) TExpr {
	sub := te.EvalExpr(u.Expr)
	sType := sub.ExprType()

	switch u.Op {
	case loneast.UNARY_OP_POSITIVE, loneast.UNARY_OP_NEGATIVE:
		return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: sType, Span_: u.Span_}

	case loneast.UNARY_OP_NOT:
		return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: TypeBool, Span_: u.Span_}

	case loneast.UNARY_OP_DEREF: // *expr
		if pt, ok := sType.(*PtrType); ok {
			return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: pt.Elem, Span_: u.Span_}
		}
		if rt, ok := sType.(*RefType); ok {
			return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: rt.Elem, Span_: u.Span_}
		}
		te.addDiag(SeverityError, u.Span_, "type/invalid-deref",
			fmt.Sprintf("cannot dereference non-pointer, non-reference type %q", sType.String()))
		return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: TypeUnknown, Span_: u.Span_}

	case loneast.UNARY_OP_REF: // &expr
		return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: &RefType{Elem: sType, ReadOnly: false}, Span_: u.Span_}

	case loneast.UNARY_OP_RAW_REF: // &raw expr
		return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: &PtrType{Elem: sType, ReadOnly: false}, Span_: u.Span_}

	default:
		return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: sType, Span_: u.Span_}
	}
}

// evalCastExpr validates explicit type casts: expr as TargetType
func (te *TypeEvaluator) evalCastExpr(c loneast.CastExpr) TExpr {
	sub := te.EvalExpr(c.Expr)
	targetType := te.resolveType(c.TargetType)
	srcType := sub.ExprType()

	// Validation rules for explicit casts
	isSrcNum := IsIntegerType(srcType) || IsFloatType(srcType)
	isDstNum := IsIntegerType(targetType) || IsFloatType(targetType)

	if isSrcNum && isDstNum {
		// Valid numeric cast: int to float, float to int, widening, narrowing
		return &TCastExpr{Expr: sub, TargetType: targetType, Span_: c.Span_}
	}

	// Pointer casts: ptr<T> as ptr<U>
	_, isSrcPtr := srcType.(*PtrType)
	_, isDstPtr := targetType.(*PtrType)
	if isSrcPtr && isDstPtr {
		return &TCastExpr{Expr: sub, TargetType: targetType, Span_: c.Span_}
	}

	// Ref to Ptr cast: ref<T> as ptr<T>
	_, isSrcRef := srcType.(*RefType)
	if isSrcRef && isDstPtr {
		return &TCastExpr{Expr: sub, TargetType: targetType, Span_: c.Span_}
	}

	te.addDiag(SeverityError, c.Span_, "type/invalid-cast",
		fmt.Sprintf("invalid explicit cast from type %q to %q", srcType.String(), targetType.String()))
	return &TCastExpr{Expr: sub, TargetType: targetType, Span_: c.Span_}
}

func (te *TypeEvaluator) evalIsExpr(i loneast.IsExpr) TExpr {
	sub := te.EvalExpr(i.Expr)
	targetType := te.resolveType(i.TargetType)
	return &TIsExpr{Expr: sub, TargetType: targetType, Negated: i.Negated, Span_: i.Span_}
}

func (te *TypeEvaluator) evalPostfixExpr(p loneast.PostFixExpr) TExpr {
	curr := te.EvalExpr(p.Base)

	for _, s := range p.Suffix {
		switch suff := s.(type) {
		case loneast.SuffixMemberAccess:
			var targetStruct *StructType
			if st, ok := curr.ExprType().(*StructType); ok {
				targetStruct = st
			} else if vref, ok := curr.(*TVarRefExpr); ok {
				if st, ok := te.structs[vref.Name]; ok {
					targetStruct = st
				}
			}

			if targetStruct != nil {
				if fInfo, exists := targetStruct.Field(suff.Name); exists {
					curr = &TMemberAccessExpr{Base: curr, Field: suff.Name, Offset: fInfo.Offset, Typ: fInfo.Type, Span_: suff.Span_}
				} else if mInfo, exists := targetStruct.Method(suff.Name); exists {
					curr = &TMemberAccessExpr{Base: curr, Field: suff.Name, Typ: mInfo, Span_: suff.Span_}
				} else {
					te.addDiag(SeverityError, suff.Span_, "type/undefined-field",
						fmt.Sprintf("struct %q has no field or method %q", targetStruct.Name, suff.Name))
					curr = &TMemberAccessExpr{Base: curr, Field: suff.Name, Typ: TypeUnknown, Span_: suff.Span_}
				}
			} else {
				curr = &TMemberAccessExpr{Base: curr, Field: suff.Name, Typ: TypeUnknown, Span_: suff.Span_}
			}

		case loneast.SuffixCall:
			var tArgs []TExpr
			for _, arg := range suff.Args {
				tArgs = append(tArgs, te.EvalExpr(arg))
			}
			retType := Type(TypeVoid)
			if fnT, ok := curr.ExprType().(*FnType); ok {
				retType = fnT.Return
			}
			curr = &TCallExpr{Callee: curr, Args: tArgs, Typ: retType, Span_: suff.Span_}

		case loneast.SuffixIndex:
			tIdx := te.EvalExpr(suff.Index)
			elemType := Type(TypeUnknown)
			if lt, ok := curr.ExprType().(*ListType); ok {
				elemType = lt.Elem
			}
			curr = &TIndexExpr{Base: curr, Index: tIdx, Typ: elemType, Span_: suff.Span_}

		default:
			// No-op for now
		}
	}

	return curr
}

func (te *TypeEvaluator) resolveType(tr loneast.TYPE_REF) Type {
	base := te.resolveBaseType(tr)
	if tr.Constrain.Optional {
		return &OptionalType{Inner: base}
	}
	return base
}

func (te *TypeEvaluator) resolveBaseType(tr loneast.TYPE_REF) Type {
	switch tr.Kind {
	case loneast.TYPE_LIST:
		if tl, ok := tr.Value.(*loneast.TypeList); ok {
			elemType := te.resolveType(tl.ItemType)
			size := 0
			if tl.Size != nil {
				if lit, ok := tl.Size.(loneast.Literal); ok && lit.Kind == loneast.LiteralInt {
					if v, ok := lit.Value.(int64); ok {
						size = int(v)
					}
				}
			}
			return &ListType{Elem: elemType, Fixed: tl.IsFixed(), Size: size}
		}
	case loneast.TYPE_FN:
		if tf, ok := tr.Value.(*loneast.TypeFn); ok {
			var params []FnParamInfo
			for _, p := range tf.Params {
				params = append(params, FnParamInfo{
					Name: p.Name,
					Type: te.resolveType(p.Type),
				})
			}
			retType := Type(TypeVoid)
			if tf.ReturnType != nil {
				retType = te.resolveType(*tf.ReturnType)
			}
			return &FnType{Params: params, Return: retType}
		}
	case loneast.TYPE_BASE:
		if tb, ok := tr.Value.(*loneast.TypeBase); ok {
			// Handle pointer ptr<T>
			if tb.Name == "ptr" && len(tb.Args) > 0 {
				inner := te.resolveType(tb.Args[0])
				return &PtrType{Elem: inner, ReadOnly: tr.Constrain.ReadOnly}
			}
			// Handle reference ref<T>
			if tb.Name == "ref" && len(tb.Args) > 0 {
				inner := te.resolveType(tb.Args[0])
				return &RefType{Elem: inner, ReadOnly: tr.Constrain.ReadOnly}
			}
			// Primitive lookup
			if prim := PrimitiveByName(tb.Name); prim != nil {
				return prim
			}
			// Scope symbol lookup (struct / trait)
			sym := te.stack.Lookup(tb.Name)
			if sym != nil && sym.Kind == SymbolType {
				return sym.Type
			}
			return &PrimitiveType{Name: tb.Name}
		}
	}
	return TypeUnknown
}

func isTrNaming(name string) bool {
	if len(name) >= 3 && name[:2] == "Tr" && unicode.IsUpper(rune(name[2])) {
		return true
	}
	return false
}
