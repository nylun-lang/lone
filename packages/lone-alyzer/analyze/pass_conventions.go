package analyze

import (
	"fmt"
	ir "lonelang/irtypes"
)

// PassNamingConventions checks that identifiers throughout the AST satisfy Lone naming conventions.
type PassNamingConventions struct {
	fileName    string
	stack       *ScopeStack
	diagnostics []Diagnostic
}

// NewPassNamingConventions creates a new naming convention pass.
func NewPassNamingConventions(fileName string) *PassNamingConventions {
	return &PassNamingConventions{
		fileName:    fileName,
		stack:       NewScopeStack(fileName),
		diagnostics: make([]Diagnostic, 0),
	}
}

// Run executes the naming convention pass on the parsed AST declarations.
func (p *PassNamingConventions) Run(decls []any) []Diagnostic {
	p.stack = NewScopeStack(p.fileName)
	p.diagnostics = make([]Diagnostic, 0)
	for _, decl := range decls {
		p.walkNode(decl)
	}
	return p.diagnostics
}

func (p *PassNamingConventions) addDiagnostic(sev Severity, pos ir.Position, rule, message string) {
	p.diagnostics = append(p.diagnostics, Diagnostic{
		Message:  message,
		Severity: sev,
		Pos:      pos,
		Rule:     rule,
	})
}

func (p *PassNamingConventions) walkNode(node any) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case ir.BIND_DEC:
		p.checkBindDec(&n)
	case *ir.BIND_DEC:
		p.checkBindDec(n)

	case ir.FN_DEC:
		p.checkFnDec(&n)
	case *ir.FN_DEC:
		p.checkFnDec(n)

	case ir.Block:
		p.checkBlock(&n)
	case *ir.Block:
		p.checkBlock(n)

	case ir.ReturnStmt:
		p.walkNode(n.Value)
	case *ir.ReturnStmt:
		p.walkNode(n.Value)

	case ir.LabelDec:
		p.checkLabelDec(&n)
	case *ir.LabelDec:
		p.checkLabelDec(n)

	case ir.BreakStmt:
		p.checkBreakStmt(&n)
	case *ir.BreakStmt:
		p.checkBreakStmt(n)

	case ir.DropStmt:
		p.walkNode(n.Expr)
	case *ir.DropStmt:
		p.walkNode(n.Expr)

	case ir.IfStmt:
		p.checkIfStmt(&n)
	case *ir.IfStmt:
		p.checkIfStmt(n)

	case ir.WhileStmt:
		p.checkWhileStmt(&n)
	case *ir.WhileStmt:
		p.checkWhileStmt(n)

	case ir.BinaryExpr:
		p.walkNode(n.Left)
		p.walkNode(n.Right)
	case *ir.BinaryExpr:
		p.walkNode(n.Left)
		p.walkNode(n.Right)

	case ir.UnaryExpr:
		p.walkNode(n.Expr)
	case *ir.UnaryExpr:
		p.walkNode(n.Expr)

	case ir.PostFixExpr:
		p.walkNode(n.Base)
		for _, s := range n.Suffix {
			p.walkSuffix(s)
		}
	case *ir.PostFixExpr:
		p.walkNode(n.Base)
		for _, s := range n.Suffix {
			p.walkSuffix(s)
		}

	case ir.ObjectLiteral:
		p.checkObjectLiteral(&n)
	case *ir.ObjectLiteral:
		p.checkObjectLiteral(n)

	case ir.ListLiteral:
		for _, item := range n.Content {
			p.walkNode(item)
		}
	case *ir.ListLiteral:
		for _, item := range n.Content {
			p.walkNode(item)
		}

	case ir.TYPE_REF:
		p.checkTypeRef(&n)
	case *ir.TYPE_REF:
		p.checkTypeRef(n)

	case []any:
		for _, item := range n {
			p.walkNode(item)
		}
	}
}

func (p *PassNamingConventions) walkSuffix(suffix ir.Suffix) {
	if suffix == nil {
		return
	}
	switch s := suffix.(type) {
	case ir.SuffixCall:
		for _, arg := range s.Args {
			p.walkNode(arg)
		}
	case *ir.SuffixCall:
		for _, arg := range s.Args {
			p.walkNode(arg)
		}
	case ir.SuffixCallTypeInit:
		for _, arg := range s.Args {
			p.walkNode(arg)
		}
	case *ir.SuffixCallTypeInit:
		for _, arg := range s.Args {
			p.walkNode(arg)
		}
	case ir.SuffixCallTypeMake:
		p.walkNode(s.Arg)
	case *ir.SuffixCallTypeMake:
		p.walkNode(s.Arg)
	}
}

func (p *PassNamingConventions) checkBindDec(b *ir.BIND_DEC) {
	isTopLevel := p.stack.Current().Kind == ScopeFile

	if isTopLevel {
		// Globals / constants should be ALL_CAPS or snake_case
		if !IsAllCaps(b.Name) && !IsSnakeCase(b.Name) {
			p.addDiagnostic(
				SeverityWarning,
				b.Pos,
				"naming/global",
				fmt.Sprintf("global variable %q should follow ALL_CAPS or snake_case convention", b.Name),
			)
		}
	} else {
		// Locals must be snake_case
		if !IsSnakeCase(b.Name) {
			p.addDiagnostic(
				SeverityWarning,
				b.Pos,
				"naming/variable",
				fmt.Sprintf("local variable %q should follow snake_case convention", b.Name),
			)
		}
	}

	p.checkTypeRef(&b.Type)
	p.walkNode(b.Value)
}

func (p *PassNamingConventions) checkFnDec(fn *ir.FN_DEC) {
	// Function name should be snake_case or #intrinsic
	if !IsSnakeCase(fn.Name) && !IsIntrinsic(fn.Name) {
		p.addDiagnostic(
			SeverityWarning,
			fn.Pos,
			"naming/function",
			fmt.Sprintf("function %q should follow snake_case or #intrinsic convention", fn.Name),
		)
	}

	p.stack.Push(ScopeFunction, fn.Name)
	defer p.stack.Pop()

	// Parameters
	for _, param := range fn.Params {
		if !IsSnakeCase(param.Name) {
			p.addDiagnostic(
				SeverityWarning,
				param.Type.Pos,
				"naming/parameter",
				fmt.Sprintf("parameter %q should follow snake_case convention", param.Name),
			)
		}
		p.checkTypeRef(&param.Type)
	}

	if fn.ReturnType != nil {
		p.checkTypeRef(fn.ReturnType)
	}

	p.checkBlock(&fn.Body)
}

func (p *PassNamingConventions) checkBlock(b *ir.Block) {
	p.stack.Push(ScopeBlock, "")
	defer p.stack.Pop()

	for _, stmt := range b.Stmts {
		p.walkNode(stmt)
	}
}

func (p *PassNamingConventions) checkLabelDec(lbl *ir.LabelDec) {
	if !IsSnakeCase(lbl.Name) {
		p.addDiagnostic(
			SeverityWarning,
			lbl.Pos,
			"naming/label",
			fmt.Sprintf("label %q should follow snake_case convention", lbl.Name),
		)
	}

	p.stack.Push(ScopeLabel, lbl.Name)
	defer p.stack.Pop()

	for _, stmt := range lbl.Body.Stmts {
		p.walkNode(stmt)
	}
}

func (p *PassNamingConventions) checkBreakStmt(brk *ir.BreakStmt) {
	if brk.Label != "" && !IsSnakeCase(brk.Label) {
		p.addDiagnostic(
			SeverityWarning,
			brk.Pos,
			"naming/label",
			fmt.Sprintf("break target label %q should follow snake_case convention", brk.Label),
		)
	}
	p.walkNode(brk.Value)
}

func (p *PassNamingConventions) checkIfStmt(stmt *ir.IfStmt) {
	p.walkNode(stmt.Cond)
	p.checkBlock(&stmt.Then)
	if stmt.Else != nil {
		p.walkNode(stmt.Else)
	}
}

func (p *PassNamingConventions) checkWhileStmt(stmt *ir.WhileStmt) {
	p.walkNode(stmt.Cond)
	p.stack.Push(ScopeLoop, "")
	defer p.stack.Pop()

	for _, s := range stmt.Body.Stmts {
		p.walkNode(s)
	}
}

func (p *PassNamingConventions) checkObjectLiteral(obj *ir.ObjectLiteral) {
	for _, f := range obj.Fields {
		if !IsSnakeCase(f.Name) {
			p.addDiagnostic(
				SeverityWarning,
				obj.Pos,
				"naming/field",
				fmt.Sprintf("field %q should follow snake_case convention", f.Name),
			)
		}
		p.walkNode(f.Expr)
	}
}

func (p *PassNamingConventions) checkTypeRef(tr *ir.TYPE_REF) {
	if tr == nil {
		return
	}

	switch tr.Kind {
	case ir.TYPE_BASE:
		base := tr.AsBase()
		if base != nil && base.Name != "" {
			if !IsPrimitiveType(base.Name) && !IsCamelCase(base.Name) {
				p.addDiagnostic(
					SeverityWarning,
					tr.Pos,
					"naming/type",
					fmt.Sprintf("type name %q should follow CamelCase convention", base.Name),
				)
			}
			for i := range base.Args {
				p.checkTypeRef(&base.Args[i])
			}
		}

	case ir.TYPE_LIST:
		list := tr.AsList()
		if list != nil {
			p.checkTypeRef(&list.ItemType)
		}

	case ir.TYPE_FN:
		fn := tr.AsFn()
		if fn != nil {
			for _, param := range fn.Params {
				if param.Name != "" && !IsSnakeCase(param.Name) {
					p.addDiagnostic(
						SeverityWarning,
						param.Type.Pos,
						"naming/parameter",
						fmt.Sprintf("parameter %q should follow snake_case convention", param.Name),
					)
				}
				p.checkTypeRef(&param.Type)
			}
			if fn.ReturnType != nil {
				p.checkTypeRef(fn.ReturnType)
			}
		}
	}
}
