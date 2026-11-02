package sema

import (
	"fmt"
	ir "lone/pkg/ast"
)

// TypeEvaluator analyzes an AST, infers types, and produces a concrete Typed IR (TProgram).
type TypeEvaluator struct {
	stack                *ScopeStack
	diagnostics          []Diagnostic
	enforceConsumeOrDrop bool
}

// NewTypeEvaluator creates a new TypeEvaluator for a given file.
func NewTypeEvaluator(fileName string, enforceConsumeOrDrop bool) *TypeEvaluator {
	return &TypeEvaluator{
		stack:                NewScopeStack(fileName),
		diagnostics:          make([]Diagnostic, 0),
		enforceConsumeOrDrop: enforceConsumeOrDrop,
	}
}

// Diagnostics returns all diagnostics gathered during type evaluation.
func (te *TypeEvaluator) Diagnostics() []Diagnostic {
	return te.diagnostics
}

func (te *TypeEvaluator) addDiag(sev Severity, pos ir.Position, rule, msg string) {
	te.diagnostics = append(te.diagnostics, Diagnostic{
		Message:  msg,
		Severity: sev,
		Pos:      pos,
		Rule:     rule,
	})
}

// Evaluate performs type analysis over top-level AST declarations.
func (te *TypeEvaluator) Evaluate(rawDecls []any) *TProgram {
	decls := toDeclSlice(rawDecls)
	program := &TProgram{
		Decls: make([]TNode, 0),
	}

	// Pass 2a: Hoist top-level function signatures & globals
	for _, raw := range decls {
		te.hoistTopLevel(raw)
	}

	// Pass 2b: Analyze bodies & expressions
	for _, raw := range decls {
		node := te.analyzeTopLevel(raw)
		if node != nil {
			program.Decls = append(program.Decls, node)
		}
	}

	return program
}

func (te *TypeEvaluator) hoistTopLevel(node any) {
	if node == nil {
		return
	}

	// Handle pigeon wrapped tuple `[0, decl]`
	if tuple, ok := node.([]any); ok && len(tuple) == 2 {
		node = tuple[1]
	}

	switch n := node.(type) {
	case ir.FN_DEC:
		te.hoistFn(&n)
	case *ir.FN_DEC:
		te.hoistFn(n)
	case ir.BIND_DEC:
		te.hoistVar(&n)
	case *ir.BIND_DEC:
		te.hoistVar(n)
	}
}

func (te *TypeEvaluator) hoistFn(fn *ir.FN_DEC) {
	var params []FnParamType
	for _, p := range fn.Params {
		params = append(params, FnParamType{
			Name: p.Name,
			Type: ParseTypeRef(&p.Type),
		})
	}

	var retType Type = TypeVoid
	if fn.ReturnType != nil {
		retType = ParseTypeRef(fn.ReturnType)
	}

	fnType := &FnType{
		Params:     params,
		ReturnType: retType,
	}

	sym := &Symbol{
		Name: fn.Name,
		Kind: SymbolFn,
		Type: fnType,
		Node: fn,
	}

	_ = te.stack.Define(sym)
}

func (te *TypeEvaluator) hoistVar(b *ir.BIND_DEC) {
	declaredType := ParseTypeRef(&b.Type)
	sym := &Symbol{
		Name: b.Name,
		Kind: SymbolVar,
		Type: declaredType,
		Node: b,
	}
	_ = te.stack.Define(sym)
}

func (te *TypeEvaluator) analyzeTopLevel(node any) TNode {
	if node == nil {
		return nil
	}

	if tuple, ok := node.([]any); ok && len(tuple) == 2 {
		node = tuple[1]
	}

	switch n := node.(type) {
	case ir.FN_DEC:
		return te.analyzeFnDec(&n)
	case *ir.FN_DEC:
		return te.analyzeFnDec(n)

	case ir.BIND_DEC:
		return te.analyzeVarDec(&n)
	case *ir.BIND_DEC:
		return te.analyzeVarDec(n)

	case ir.DropStmt:
		tExpr := te.EvalExpr(n.Expr)
		return &TDropStmt{Expr: tExpr, Pos_: n.Pos}
	case *ir.DropStmt:
		tExpr := te.EvalExpr(n.Expr)
		return &TDropStmt{Expr: tExpr, Pos_: n.Pos}

	default:
		if expr, ok := node.(ir.Expr); ok {
			tExpr := te.EvalExpr(expr)
			if te.enforceConsumeOrDrop {
				typ := tExpr.ExprType()
				if typ != TypeVoid && typ != TypeNone && typ != TypeUnknown {
					te.addDiag(SeverityError, tExpr.Pos(), "type/unused-result",
						fmt.Sprintf("result of expression of type %q is not consumed; use 'drop' to discard it", typ.String()))
				}
			}
			return &TExprStmt{Expr: tExpr, Pos_: tExpr.Pos()}
		}
	}
	return nil
}

func (te *TypeEvaluator) analyzeFnDec(fn *ir.FN_DEC) *TFnDec {
	var retType Type = TypeVoid
	if fn.ReturnType != nil {
		retType = ParseTypeRef(fn.ReturnType)
	}

	te.stack.Push(ScopeFunction, fn.Name)
	defer te.stack.Pop()

	var tParams []TFnParam
	for _, p := range fn.Params {
		pType := ParseTypeRef(&p.Type)
		tParams = append(tParams, TFnParam{
			Name: p.Name,
			Type: pType,
			Pos_: p.Type.Pos,
		})
		_ = te.stack.Define(&Symbol{
			Name: p.Name,
			Kind: SymbolVar,
			Type: pType,
			Node: p,
		})
	}

	hasReturn := false
	tBody := te.analyzeBlock(&fn.Body, retType, &hasReturn)

	if retType != TypeVoid && retType != TypeNone && retType != TypeUnknown && !hasReturn {
		te.addDiag(SeverityError, fn.Pos, "type/missing-return",
			fmt.Sprintf("function %q expects return type %q but contains no return statement", fn.Name, retType.String()))
	}

	return &TFnDec{
		Name:       fn.Name,
		Params:     tParams,
		ReturnType: retType,
		Body:       tBody,
		Pos_:       fn.Pos,
	}
}

func (te *TypeEvaluator) analyzeVarDec(b *ir.BIND_DEC) *TVarDec {
	declaredType := ParseTypeRef(&b.Type)
	tVal := te.EvalExpr(b.Value)

	resolvedType := declaredType
	if declaredType == TypeInfer || declaredType == TypeUnknown {
		resolvedType = tVal.ExprType()
	} else if !tVal.ExprType().AssignableTo(declaredType) {
		te.addDiag(SeverityError, b.Pos, "type/assignment-mismatch",
			fmt.Sprintf("cannot assign value of type %q to variable %q of type %q",
				tVal.ExprType().String(), b.Name, declaredType.String()))
	}

	// Update symbol in current scope
	if sym := te.stack.Current().LookupLocal(b.Name); sym != nil {
		sym.Type = resolvedType
	} else {
		_ = te.stack.Define(&Symbol{
			Name: b.Name,
			Kind: SymbolVar,
			Type: resolvedType,
			Node: b,
		})
	}

	return &TVarDec{
		Name:  b.Name,
		Type:  resolvedType,
		Value: tVal,
		Pos_:  b.Pos,
	}
}

func (te *TypeEvaluator) analyzeBlock(b *ir.Block, expectedReturn Type, hasReturn *bool) TBlock {
	te.stack.Push(ScopeBlock, "")
	defer te.stack.Pop()

	tBlock := TBlock{
		Stmts: make([]TStmt, 0),
		Pos_:  b.Pos,
		Typ:   TypeVoid,
	}

	for _, rawStmt := range b.Stmts {
		stmt := te.analyzeStmt(rawStmt, expectedReturn, hasReturn)
		if stmt != nil {
			tBlock.Stmts = append(tBlock.Stmts, stmt)
		}
	}

	return tBlock
}

func (te *TypeEvaluator) analyzeStmt(raw any, expectedReturn Type, hasReturn *bool) TStmt {
	if raw == nil {
		return nil
	}

	switch s := raw.(type) {
	case ir.BIND_DEC:
		return te.analyzeVarDec(&s)
	case *ir.BIND_DEC:
		return te.analyzeVarDec(s)

	case ir.ReturnStmt:
		return te.analyzeReturnStmt(&s, expectedReturn, hasReturn)
	case *ir.ReturnStmt:
		return te.analyzeReturnStmt(s, expectedReturn, hasReturn)

	case ir.DropStmt:
		tExpr := te.EvalExpr(s.Expr)
		return &TDropStmt{Expr: tExpr, Pos_: s.Pos}
	case *ir.DropStmt:
		tExpr := te.EvalExpr(s.Expr)
		return &TDropStmt{Expr: tExpr, Pos_: s.Pos}

	case ir.BreakStmt:
		var tVal TExpr
		if s.Value != nil {
			tVal = te.EvalExpr(s.Value)
		}
		return &TBreakStmt{Label: s.Label, Value: tVal, Pos_: s.Pos}
	case *ir.BreakStmt:
		var tVal TExpr
		if s.Value != nil {
			tVal = te.EvalExpr(s.Value)
		}
		return &TBreakStmt{Label: s.Label, Value: tVal, Pos_: s.Pos}

	case ir.IfStmt:
		tCond := te.EvalExpr(s.Cond)
		tThen := te.analyzeBlock(&s.Then, expectedReturn, hasReturn)
		var tElse *TBlock
		if s.Else != nil {
			if elseBlock, ok := s.Else.(ir.Block); ok {
				eb := te.analyzeBlock(&elseBlock, expectedReturn, hasReturn)
				tElse = &eb
			} else if elseIf, ok := s.Else.(ir.IfStmt); ok {
				stmt := te.analyzeStmt(elseIf, expectedReturn, hasReturn)
				if ts, ok := stmt.(TStmt); ok {
					eb := TBlock{Stmts: []TStmt{ts}, Pos_: elseIf.Pos, Typ: TypeVoid}
					tElse = &eb
				}
			}
		}
		return &TIfStmt{Cond: tCond, Then: tThen, Else: tElse, Pos_: s.Pos, Typ: tThen.Typ}

	case ir.WhileStmt:
		tCond := te.EvalExpr(s.Cond)
		tBody := te.analyzeBlock(&s.Body, expectedReturn, hasReturn)
		return &TWhileStmt{Cond: tCond, Body: tBody, Pos_: s.Pos}

	case ir.LabelDec:
		tBody := te.analyzeBlock(&s.Body, expectedReturn, hasReturn)
		return &TLabelDec{Name: s.Name, Body: tBody, Pos_: s.Pos, Typ: tBody.Typ}

	default:
		if expr, ok := raw.(ir.Expr); ok {
			tExpr := te.EvalExpr(expr)
			// Rule: Fn/expression returning non-void must be consumed or dropped
			if te.enforceConsumeOrDrop {
				typ := tExpr.ExprType()
				if typ != TypeVoid && typ != TypeNone && typ != TypeUnknown {
					te.addDiag(SeverityError, tExpr.Pos(), "type/unused-result",
						fmt.Sprintf("result of expression of type %q is not consumed; use 'drop' to discard it", typ.String()))
				}
			}
			return &TExprStmt{Expr: tExpr, Pos_: tExpr.Pos()}
		}
	}

	return nil
}

func (te *TypeEvaluator) analyzeReturnStmt(ret *ir.ReturnStmt, expectedReturn Type, hasReturn *bool) *TReturnStmt {
	if hasReturn != nil {
		*hasReturn = true
	}

	var tVal TExpr
	var actualType Type = TypeVoid
	if ret.Value != nil {
		tVal = te.EvalExpr(ret.Value)
		actualType = tVal.ExprType()
	}

	if expectedReturn != nil && expectedReturn != TypeInfer && expectedReturn != TypeUnknown {
		if !actualType.AssignableTo(expectedReturn) {
			te.addDiag(SeverityError, ret.Pos, "type/return-mismatch",
				fmt.Sprintf("expected return type %q, got %q", expectedReturn.String(), actualType.String()))
		}
	}

	return &TReturnStmt{
		Value: tVal,
		Pos_:  ret.Pos,
	}
}

// EvalExpr evaluates an ir.Expr and returns its typed counterpart TExpr with concrete Type.
func (te *TypeEvaluator) EvalExpr(raw ir.Expr) TExpr {
	if raw == nil {
		return &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: TypeUnknown}
	}

	switch e := raw.(type) {
	case ir.Literal:
		return te.evalLiteral(&e)
	case *ir.Literal:
		return te.evalLiteral(e)

	case ir.NameRef:
		return te.evalNameRef(&e)
	case *ir.NameRef:
		return te.evalNameRef(e)

	case ir.BinaryExpr:
		return te.evalBinaryExpr(&e)
	case *ir.BinaryExpr:
		return te.evalBinaryExpr(e)

	case ir.UnaryExpr:
		return te.evalUnaryExpr(&e)
	case *ir.UnaryExpr:
		return te.evalUnaryExpr(e)

	case ir.PostFixExpr:
		return te.evalPostFixExpr(&e)
	case *ir.PostFixExpr:
		return te.evalPostFixExpr(e)

	case ir.ObjectLiteral:
		return te.evalObjectLiteral(&e)
	case *ir.ObjectLiteral:
		return te.evalObjectLiteral(e)

	case ir.ListLiteral:
		return te.evalListLiteral(&e)
	case *ir.ListLiteral:
		return te.evalListLiteral(e)

	case ir.DropStmt:
		inner := te.EvalExpr(e.Expr)
		return &TDropStmt{Expr: inner, Pos_: e.Pos}
	case *ir.DropStmt:
		inner := te.EvalExpr(e.Expr)
		return &TDropStmt{Expr: inner, Pos_: e.Pos}

	case ir.LabelDec:
		tBody := te.analyzeBlock(&e.Body, nil, nil)
		return &TLabelDec{Name: e.Name, Body: tBody, Pos_: e.Pos, Typ: tBody.Typ}
	case *ir.LabelDec:
		tBody := te.analyzeBlock(&e.Body, nil, nil)
		return &TLabelDec{Name: e.Name, Body: tBody, Pos_: e.Pos, Typ: tBody.Typ}

	case ir.BreakStmt:
		var tVal TExpr
		if e.Value != nil {
			tVal = te.EvalExpr(e.Value)
		}
		return &TBreakStmt{Label: e.Label, Value: tVal, Pos_: e.Pos}
	case *ir.BreakStmt:
		var tVal TExpr
		if e.Value != nil {
			tVal = te.EvalExpr(e.Value)
		}
		return &TBreakStmt{Label: e.Label, Value: tVal, Pos_: e.Pos}

	default:
		return &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: TypeUnknown}
	}
}

func (te *TypeEvaluator) evalLiteral(lit *ir.Literal) *TLiteralExpr {
	var typ Type = TypeUnknown
	switch lit.Kind {
	case ir.LITERAL_INT:
		typ = TypeI32
	case ir.LITERAL_FLOAT:
		typ = TypeF64
	case ir.LITERAL_STRING:
		typ = TypeStr
	case ir.LITERAL_BOOL:
		typ = TypeBool
	case ir.LITERAL_NONE:
		typ = TypeNone
	case ir.LITERAL_UNINITIALIZED, ir.LITERAL_DYNAMIC_SIZE:
		typ = TypeUnknown
	}

	return &TLiteralExpr{
		Kind:  lit.Kind,
		Value: lit.Value,
		Typ:   typ,
		Pos_:  lit.Pos,
	}
}

func (te *TypeEvaluator) evalNameRef(nr *ir.NameRef) *TVarRefExpr {
	sym := te.stack.Resolve(nr.Name)
	if sym == nil {
		if IsIntrinsic(nr.Name) {
			return &TVarRefExpr{Name: nr.Name, Typ: TypeAny, Pos_: nr.Pos}
		}
		te.addDiag(SeverityError, nr.Pos, "type/undefined-identifier",
			fmt.Sprintf("undefined identifier %q", nr.Name))
		return &TVarRefExpr{Name: nr.Name, Typ: TypeUnknown, Pos_: nr.Pos}
	}
	return &TVarRefExpr{Name: nr.Name, Typ: sym.Type, Pos_: nr.Pos}
}

func (te *TypeEvaluator) evalBinaryExpr(b *ir.BinaryExpr) *TBinaryExpr {
	tLeft := te.EvalExpr(b.Left)
	tRight := te.EvalExpr(b.Right)

	var resType Type = TypeUnknown
	switch b.Op {
	case ir.BINARY_OP_PLUS, ir.BINARY_OP_MINUS, ir.BINARY_OP_TIME, ir.BINARY_OP_DIV, ir.BINARY_OP_MOD:
		// Arithmetic: returns left type
		resType = tLeft.ExprType()
	case ir.BINARY_OP_EQ, ir.BINARY_OP_NOT_EQ, ir.BINARY_OP_LESS, ir.BINARY_OP_LESS_EQ, ir.BINARY_OP_GREATER, ir.BINARY_OP_GREATER_EQ:
		resType = TypeBool
	case ir.BINARY_OP_AND, ir.BINARY_OP_OR:
		resType = TypeBool
	default:
		resType = tLeft.ExprType()
	}

	return &TBinaryExpr{
		Op:    b.Op,
		Left:  tLeft,
		Right: tRight,
		Typ:   resType,
		Pos_:  b.Pos,
	}
}

func (te *TypeEvaluator) evalUnaryExpr(u *ir.UnaryExpr) *TUnaryExpr {
	tExpr := te.EvalExpr(u.Expr)
	var resType Type = tExpr.ExprType()

	switch u.Op {
	case ir.UNARY_OP_NOT:
		resType = TypeBool
	case ir.UNARY_OP_POSITIVE, ir.UNARY_OP_NEGATIVE:
		resType = tExpr.ExprType()
	case ir.UNARY_OP_REF, ir.UNARY_OP_DEREF:
		resType = tExpr.ExprType()
	}

	return &TUnaryExpr{
		Op:   u.Op,
		Expr: tExpr,
		Typ:  resType,
		Pos_: u.Pos,
	}
}

func (te *TypeEvaluator) evalPostFixExpr(pf *ir.PostFixExpr) TExpr {
	tBase := te.EvalExpr(pf.Base)

	// If no suffix, unwrap or return base
	if len(pf.Suffix) == 0 {
		return tBase
	}

	var curExpr TExpr = tBase

	for _, s := range pf.Suffix {
		switch suf := s.(type) {
		case ir.SuffixCall:
			var args []TExpr
			for _, a := range suf.Args {
				args = append(args, te.EvalExpr(a))
			}

			var retType Type = TypeUnknown
			baseType := curExpr.ExprType()
			if fnType, ok := baseType.(*FnType); ok {
				retType = fnType.ReturnType
				// Check parameter counts and types
				if len(args) != len(fnType.Params) {
					te.addDiag(SeverityError, suf.Pos, "type/call-arg-count",
						fmt.Sprintf("function expects %d arguments, but got %d", len(fnType.Params), len(args)))
				} else {
					for i, arg := range args {
						paramType := fnType.Params[i].Type
						if !arg.ExprType().AssignableTo(paramType) {
							te.addDiag(SeverityError, arg.Pos(), "type/call-arg-type",
								fmt.Sprintf("argument %d expects type %q, got %q", i+1, paramType.String(), arg.ExprType().String()))
						}
					}
				}
			} else if baseRef, ok := curExpr.(*TVarRefExpr); ok && IsIntrinsic(baseRef.Name) {
				// Intrinsics default to void
				retType = TypeVoid
			} else if baseType == TypeUnknown || baseType == TypeAny {
				retType = TypeAny
			} else {
				te.addDiag(SeverityError, suf.Pos, "type/not-callable",
					fmt.Sprintf("expression of type %q is not callable", baseType.String()))
			}

			curExpr = &TCallExpr{
				Callee: curExpr,
				Args:   args,
				Typ:    retType,
				Pos_:   suf.Pos,
			}

		case ir.SuffixMemberAccess:
			// Object field access
			var memberType Type = TypeUnknown
			if objType, ok := curExpr.ExprType().(*ObjectType); ok {
				if ft, ok := objType.Fields[suf.Name]; ok {
					memberType = ft
				} else {
					te.addDiag(SeverityError, suf.Pos, "type/field-not-found",
						fmt.Sprintf("field %q not found on type %q", suf.Name, objType.String()))
				}
			}
			curExpr = &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: memberType, Pos_: suf.Pos}
		}
	}

	return curExpr
}

func (te *TypeEvaluator) evalObjectLiteral(obj *ir.ObjectLiteral) *TObjectExpr {
	fields := make([]TFieldAssign, 0, len(obj.Fields))
	typeMap := make(map[string]Type)

	for _, f := range obj.Fields {
		tExpr := te.EvalExpr(f.Expr)
		fields = append(fields, TFieldAssign{
			Name: f.Name,
			Expr: tExpr,
		})
		typeMap[f.Name] = tExpr.ExprType()
	}

	return &TObjectExpr{
		Fields: fields,
		Typ:    &ObjectType{Fields: typeMap},
		Pos_:   obj.Pos,
	}
}

func (te *TypeEvaluator) evalListLiteral(list *ir.ListLiteral) *TListExpr {
	items := make([]TExpr, 0, len(list.Content))
	var elemType Type = TypeAny

	for _, c := range list.Content {
		tExpr := te.EvalExpr(c)
		items = append(items, tExpr)
		if elemType == TypeAny {
			elemType = tExpr.ExprType()
		}
	}

	return &TListExpr{
		Items: items,
		Typ:    &ListType{ItemType: elemType, Size: len(items)},
		Pos_:   list.Pos,
	}
}
