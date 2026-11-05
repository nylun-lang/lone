package loneanalyzer

import (
	loneast "lone/pkg2/lone-parser-ast"
)

type TNode interface {
	Span() loneast.Span
}

type TExpr interface {
	TNode
	ExprType() Type
}

type TStmt interface {
	TNode
}

// =====================
// TYPED DECLARATIONS
// =====================

type TProgram struct {
	Decls []TNode
}

type TVarDec struct {
	Name      string
	Variant   *EnumVariantInfo // non-nil if destructuring variant pattern
	TargetVal TExpr            // target being destructured
	Bindings  []*TVarDec       // bound variables unpacked from variant
	IsMutable    bool             // true for 'var', false for 'let'
	AddressTaken bool             // true if &var or &raw var taken
	Type         Type
	Value     TExpr
	Span_     loneast.Span
}

func (v *TVarDec) Span() loneast.Span { return v.Span_ }

type TFnParam struct {
	Name  string
	Type  Type
	Span_ loneast.Span
}

func (p *TFnParam) Span() loneast.Span { return p.Span_ }

type TFnDec struct {
	Name       string
	Params     []TFnParam
	ReturnType Type
	Body       TBlock
	Span_      loneast.Span
}

func (f *TFnDec) Span() loneast.Span { return f.Span_ }

type TStructDec struct {
	Name    string
	Type    *StructType
	Methods []*TFnDec
	Span_   loneast.Span
}

func (s *TStructDec) Span() loneast.Span { return s.Span_ }

type TTraitDec struct {
	Name  string
	Type  *TraitType
	Span_ loneast.Span
}

func (t *TTraitDec) Span() loneast.Span { return t.Span_ }

type TEnumDec struct {
	Name  string
	Type  *EnumType
	Span_ loneast.Span
}

func (e *TEnumDec) Span() loneast.Span { return e.Span_ }

type TImplDec struct {
	TraitName  string
	TargetType Type
	Methods    []*TFnDec
	Span_      loneast.Span
}

func (i *TImplDec) Span() loneast.Span { return i.Span_ }

// =====================
// TYPED STATEMENTS
// =====================

type TBlock struct {
	Stmts []TStmt
	Typ   Type
	Span_ loneast.Span
}

func (b *TBlock) Span() loneast.Span { return b.Span_ }
func (b *TBlock) ExprType() Type    { return b.Typ }

type TAssignStmt struct {
	Target TExpr
	Value  TExpr
	Span_  loneast.Span
}

func (a *TAssignStmt) Span() loneast.Span { return a.Span_ }

type TReturnStmt struct {
	Value TExpr
	Span_ loneast.Span
}

func (r *TReturnStmt) Span() loneast.Span { return r.Span_ }

type TBreakStmt struct {
	Label string
	Value TExpr
	Span_ loneast.Span
}

func (b *TBreakStmt) Span() loneast.Span { return b.Span_ }

type TDropStmt struct {
	Expr  TExpr
	Span_ loneast.Span
}

func (d *TDropStmt) Span() loneast.Span { return d.Span_ }

type TIfStmt struct {
	Header []*TVarDec
	Cond   TExpr
	Then   TBlock
	Else   TStmt // nil, *TBlock, or *TIfStmt
	Span_  loneast.Span
}

func (i *TIfStmt) Span() loneast.Span { return i.Span_ }

type TPackedIfArm struct {
	Cond  TExpr // nil for else arm
	Body  TStmt
	Span_ loneast.Span
}

type TPackedIfStmt struct {
	Header []*TVarDec
	Arms   []TPackedIfArm
	Span_  loneast.Span
}

func (p *TPackedIfStmt) Span() loneast.Span { return p.Span_ }

type TWhileStmt struct {
	Cond  TExpr
	Body  TBlock
	Span_ loneast.Span
}

func (w *TWhileStmt) Span() loneast.Span { return w.Span_ }

type TLoopStmt struct {
	Body      TBlock
	WhileCond TExpr // nil for infinite loop
	Span_     loneast.Span
}

func (l *TLoopStmt) Span() loneast.Span { return l.Span_ }

type TForStmt struct {
	VarName string
	Iter    TExpr
	Body    TBlock
	Span_   loneast.Span
}

func (f *TForStmt) Span() loneast.Span { return f.Span_ }

type TLabelDec struct {
	Name  string
	Body  TBlock
	Typ   Type
	Span_ loneast.Span
}

func (l *TLabelDec) Span() loneast.Span { return l.Span_ }
func (l *TLabelDec) ExprType() Type     { return l.Typ }

// =====================
// TYPED EXPRESSIONS
// =====================

type TLiteralExpr struct {
	Kind  loneast.LiteralKind
	Value any
	Typ   Type
	Span_ loneast.Span
}

func (l *TLiteralExpr) Span() loneast.Span { return l.Span_ }
func (l *TLiteralExpr) ExprType() Type    { return l.Typ }

type TVarRefExpr struct {
	Name  string
	Typ   Type
	Span_ loneast.Span
}

func (v *TVarRefExpr) Span() loneast.Span { return v.Span_ }
func (v *TVarRefExpr) ExprType() Type    { return v.Typ }

type TBinaryExpr struct {
	Op    loneast.BinaryOp
	Left  TExpr
	Right TExpr
	Typ   Type
	Span_ loneast.Span
}

func (b *TBinaryExpr) Span() loneast.Span { return b.Span_ }
func (b *TBinaryExpr) ExprType() Type    { return b.Typ }

type TUnaryExpr struct {
	Op    loneast.UnaryOp
	Expr  TExpr
	Typ   Type
	Span_ loneast.Span
}

func (u *TUnaryExpr) Span() loneast.Span { return u.Span_ }
func (u *TUnaryExpr) ExprType() Type    { return u.Typ }

type TCastExpr struct {
	Expr       TExpr
	TargetType Type
	Span_      loneast.Span
}

func (c *TCastExpr) Span() loneast.Span { return c.Span_ }
func (c *TCastExpr) ExprType() Type    { return c.TargetType }

type TIsExpr struct {
	Expr          TExpr
	TargetType    Type
	TargetVariant *EnumVariantInfo // non-nil if testing variant discriminant tag
	Negated       bool
	Span_         loneast.Span
}

func (i *TIsExpr) Span() loneast.Span { return i.Span_ }
func (i *TIsExpr) ExprType() Type    { return TypeBool }

type TCallExpr struct {
	Callee TExpr
	Args   []TExpr
	Typ    Type
	Span_  loneast.Span
}

func (c *TCallExpr) Span() loneast.Span { return c.Span_ }
func (c *TCallExpr) ExprType() Type    { return c.Typ }

type TMemberAccessExpr struct {
	Base   TExpr
	Field  string
	Offset int
	Typ    Type
	Span_  loneast.Span
}

func (m *TMemberAccessExpr) Span() loneast.Span { return m.Span_ }
func (m *TMemberAccessExpr) ExprType() Type    { return m.Typ }

type TIndexExpr struct {
	Base  TExpr
	Index TExpr
	Typ   Type
	Span_ loneast.Span
}

func (i *TIndexExpr) Span() loneast.Span { return i.Span_ }
func (i *TIndexExpr) ExprType() Type    { return i.Typ }

type TEnumConstructExpr struct {
	EnumType *EnumType
	Variant  *EnumVariantInfo
	Args     []TExpr
	Typ      Type
	Span_    loneast.Span
}

func (e *TEnumConstructExpr) Span() loneast.Span { return e.Span_ }
func (e *TEnumConstructExpr) ExprType() Type    { return e.Typ }

type TMatchArm struct {
	Pattern   loneast.MatchPattern
	Variant   *EnumVariantInfo // if matching an enum variant
	Bindings  []*TVarDec       // variables bound by variant fields
	BodyExpr  TExpr            // non-nil if arm body is an expression
	BodyBlock *TBlock          // non-nil if arm body is a block
	Span_     loneast.Span
}

func (a *TMatchArm) Span() loneast.Span { return a.Span_ }

type TMatchExpr struct {
	Target TExpr
	Arms   []TMatchArm
	Typ    Type
	Span_  loneast.Span
}

func (m *TMatchExpr) Span() loneast.Span { return m.Span_ }
func (m *TMatchExpr) ExprType() Type    { return m.Typ }

type TMustExpr struct {
	Expr       TExpr
	ResultType *EnumType
	OkVariant  *EnumVariantInfo
	OkType     Type
	Span_      loneast.Span
}

func (m *TMustExpr) Span() loneast.Span { return m.Span_ }
func (m *TMustExpr) ExprType() Type    { return m.OkType }

type TTryExpr struct {
	Expr       TExpr
	ResultType *EnumType
	OkVariant  *EnumVariantInfo
	OkType     Type
	Span_      loneast.Span
}

func (t *TTryExpr) Span() loneast.Span { return t.Span_ }
func (t *TTryExpr) ExprType() Type    { return t.OkType }

type TTraitMatExpr struct {
	TraitName  string
	TargetType Type
	Typ        Type
	Span_      loneast.Span
}

func (t *TTraitMatExpr) Span() loneast.Span { return t.Span_ }
func (t *TTraitMatExpr) ExprType() Type    { return t.Typ }
func (t *TTraitMatExpr) tnode()            {}
func (t *TTraitMatExpr) texpr()            {}

type TTupleExpr struct {
	Elements []TExpr
	Typ      Type
	Span_    loneast.Span
}

func (t *TTupleExpr) Span() loneast.Span { return t.Span_ }
func (t *TTupleExpr) ExprType() Type     { return t.Typ }
func (t *TTupleExpr) tnode()             {}
func (t *TTupleExpr) texpr()             {}

type TAnonFnExpr struct {
	Name       string
	Params     []*TFnParam
	ReturnType Type
	Body       *TBlock
	Captures   []*Symbol
	Typ        Type
	Span_      loneast.Span
}

func (a *TAnonFnExpr) Span() loneast.Span { return a.Span_ }
func (a *TAnonFnExpr) ExprType() Type     { return a.Typ }
func (a *TAnonFnExpr) tnode()             {}
func (a *TAnonFnExpr) texpr()             {}

