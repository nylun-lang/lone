package lonealyzer

import ir "lonelang/irtypes"

// TNode represents any node in the Typed IR.
type TNode interface {
	Pos() ir.Position
}

// TExpr represents an expression in the Typed IR with a concrete evaluated Type.
type TExpr interface {
	TNode
	ExprType() Type
}

// TStmt represents a statement in the Typed IR.
type TStmt interface {
	TNode
	isStmt()
}

// TDecl represents a declaration in the Typed IR.
type TDecl interface {
	TNode
	isDecl()
}

// =====================
// PROGRAM & TOP-LEVEL
// =====================

type TProgram struct {
	Decls []TNode
	Pos_  ir.Position
}

func (p *TProgram) Pos() ir.Position { return p.Pos_ }

// =====================
// DECLARATIONS
// =====================

// TVarDec represents a typed variable declaration where the type is resolved (never infer).
type TVarDec struct {
	Name  string
	Type  Type
	Value TExpr
	Pos_  ir.Position
}

func (v *TVarDec) Pos() ir.Position { return v.Pos_ }
func (v *TVarDec) isDecl()          {}
func (v *TVarDec) isStmt()          {}

// TFnParam represents a typed function parameter.
type TFnParam struct {
	Name string
	Type Type
	Pos_ ir.Position
}

func (p *TFnParam) Pos() ir.Position { return p.Pos_ }

// TFnDec represents a typed function declaration.
type TFnDec struct {
	Name       string
	Params     []TFnParam
	ReturnType Type
	Body       TBlock
	Pos_       ir.Position
}

func (f *TFnDec) Pos() ir.Position { return f.Pos_ }
func (f *TFnDec) isDecl()          {}

// =====================
// STATEMENTS
// =====================

// TBlock represents a scoped block of statements.
type TBlock struct {
	Stmts []TStmt
	Pos_  ir.Position
	Typ   Type
}

func (b *TBlock) Pos() ir.Position { return b.Pos_ }
func (b *TBlock) isStmt()          {}
func (b *TBlock) ExprType() Type   { return b.Typ }

// TReturnStmt represents a return statement.
type TReturnStmt struct {
	Value TExpr
	Pos_  ir.Position
}

func (r *TReturnStmt) Pos() ir.Position { return r.Pos_ }
func (r *TReturnStmt) isStmt()          {}

// TDropStmt represents a statement that evaluates an expression and discards its value.
type TDropStmt struct {
	Expr TExpr
	Pos_ ir.Position
}

func (d *TDropStmt) Pos() ir.Position { return d.Pos_ }
func (d *TDropStmt) isStmt()          {}
func (d *TDropStmt) ExprType() Type   { return TypeVoid }

// TBreakStmt represents a break statement from a labeled block or loop.
type TBreakStmt struct {
	Label string
	Value TExpr
	Pos_  ir.Position
}

func (b *TBreakStmt) Pos() ir.Position { return b.Pos_ }
func (b *TBreakStmt) isStmt()          {}
func (b *TBreakStmt) ExprType() Type {
	if b.Value != nil {
		return b.Value.ExprType()
	}
	return TypeVoid
}

// TIfStmt represents an if/else conditional branch.
type TIfStmt struct {
	Cond TExpr
	Then TBlock
	Else *TBlock
	Pos_ ir.Position
	Typ  Type
}

func (i *TIfStmt) Pos() ir.Position { return i.Pos_ }
func (i *TIfStmt) isStmt()          {}
func (i *TIfStmt) ExprType() Type   { return i.Typ }

// TWhileStmt represents a while loop.
type TWhileStmt struct {
	Cond TExpr
	Body TBlock
	Pos_ ir.Position
}

func (w *TWhileStmt) Pos() ir.Position { return w.Pos_ }
func (w *TWhileStmt) isStmt()          {}
func (w *TWhileStmt) ExprType() Type   { return TypeVoid }

// TLabelDec represents a labeled block expression/statement.
type TLabelDec struct {
	Name string
	Body TBlock
	Pos_ ir.Position
	Typ  Type
}

func (l *TLabelDec) Pos() ir.Position { return l.Pos_ }
func (l *TLabelDec) isDecl()          {}
func (l *TLabelDec) isStmt()          {}
func (l *TLabelDec) ExprType() Type   { return l.Typ }

// TExprStmt represents an expression executed purely for side-effects in a statement position.
type TExprStmt struct {
	Expr TExpr
	Pos_ ir.Position
}

func (e *TExprStmt) Pos() ir.Position { return e.Pos_ }
func (e *TExprStmt) isStmt()          {}

// =====================
// EXPRESSIONS
// =====================

type TLiteralExpr struct {
	Kind  ir.LiteralKind
	Value any
	Typ   Type
	Pos_  ir.Position
}

func (l *TLiteralExpr) Pos() ir.Position { return l.Pos_ }
func (l *TLiteralExpr) ExprType() Type   { return l.Typ }

type TVarRefExpr struct {
	Name string
	Typ  Type
	Pos_ ir.Position
}

func (v *TVarRefExpr) Pos() ir.Position { return v.Pos_ }
func (v *TVarRefExpr) ExprType() Type   { return v.Typ }

type TBinaryExpr struct {
	Op    ir.BinaryOp
	Left  TExpr
	Right TExpr
	Typ   Type
	Pos_  ir.Position
}

func (b *TBinaryExpr) Pos() ir.Position { return b.Pos_ }
func (b *TBinaryExpr) ExprType() Type   { return b.Typ }

type TUnaryExpr struct {
	Op   ir.UnaryOp
	Expr TExpr
	Typ  Type
	Pos_ ir.Position
}

func (u *TUnaryExpr) Pos() ir.Position { return u.Pos_ }
func (u *TUnaryExpr) ExprType() Type   { return u.Typ }

type TCallExpr struct {
	Callee TExpr
	Args   []TExpr
	Typ    Type
	Pos_   ir.Position
}

func (c *TCallExpr) Pos() ir.Position { return c.Pos_ }
func (c *TCallExpr) ExprType() Type   { return c.Typ }

type TFieldAssign struct {
	Name string
	Expr TExpr
}

type TObjectExpr struct {
	Fields []TFieldAssign
	Typ    Type
	Pos_   ir.Position
}

func (o *TObjectExpr) Pos() ir.Position { return o.Pos_ }
func (o *TObjectExpr) ExprType() Type   { return o.Typ }

type TListExpr struct {
	Items []TExpr
	Typ   Type
	Pos_  ir.Position
}

func (l *TListExpr) Pos() ir.Position { return l.Pos_ }
func (l *TListExpr) ExprType() Type   { return l.Typ }
