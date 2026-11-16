package loneast

type ExprKind int

const (
	ExprLiteral ExprKind = iota
	ExprObjectLiteral
	ExprListLiteral
	ExprNameRef
	ExprUnaryOp
	ExprBinaryOp
	ExprCast
	ExprIs
	ExprPostfix
	ExprIf
	ExprPackedIf
	ExprWhile
	ExprLoop
	ExprFor
	ExprBreak
	ExprReturn
	ExprDrop
	ExprAssign
	ExprLabelDec
	ExprMatch
	ExprMust
	ExprTry
	ExprTuple
	ExprAnonFn
	ExprInterpolatedString
)

type Expr interface {
	Node
	IsExpr() ExprKind
}

// =====================
// LITERALS
// =====================

type LiteralKind int

const (
	LiteralInt LiteralKind = iota
	LiteralFloat
	LiteralString
	LiteralChar
	LiteralBool
	LiteralNone
	LiteralDynamicSize
	LiteralUninitialized
)

type Literal struct {
	Kind  LiteralKind
	Value any
	Span_ Span
}

func (Literal) IsExpr() ExprKind { return ExprLiteral }
func (l Literal) NodeSpan() Span  { return l.Span_ }

type ListLiteral struct {
	Elements []Expr
	Span_    Span
}

func (ListLiteral) IsExpr() ExprKind { return ExprListLiteral }
func (l ListLiteral) NodeSpan() Span  { return l.Span_ }

type FieldAssign struct {
	Name  string
	Expr  Expr
	Span_ Span
}

func (f FieldAssign) NodeSpan() Span { return f.Span_ }

type ObjectLiteral struct {
	Fields []FieldAssign
	Span_  Span
}

func (ObjectLiteral) IsExpr() ExprKind { return ExprObjectLiteral }
func (o ObjectLiteral) NodeSpan() Span  { return o.Span_ }

type NameRef struct {
	Name  string
	Span_ Span
}

func (NameRef) IsExpr() ExprKind { return ExprNameRef }
func (n NameRef) NodeSpan() Span  { return n.Span_ }

// =====================
// OPERATORS & CASTS
// =====================

type BinaryOp string

const (
	BINARY_OP_PLUS       BinaryOp = "+"
	BINARY_OP_MINUS      BinaryOp = "-"
	BINARY_OP_TIME       BinaryOp = "*"
	BINARY_OP_DIV        BinaryOp = "/"
	BINARY_OP_MOD        BinaryOp = "%"
	BINARY_OP_AND        BinaryOp = "and"
	BINARY_OP_OR         BinaryOp = "or"
	BINARY_OP_XOR        BinaryOp = "xor"
	BINARY_OP_CARET      BinaryOp = "^"
	BINARY_OP_LESS       BinaryOp = "<"  // Fixed
	BINARY_OP_LESS_EQ    BinaryOp = "<=" // Fixed
	BINARY_OP_EQ         BinaryOp = "=="
	BINARY_OP_NOT_EQ     BinaryOp = "!="
	BINARY_OP_GREATER    BinaryOp = ">"  // Fixed
	BINARY_OP_GREATER_EQ BinaryOp = ">=" // Fixed
)

type BinaryExpr struct {
	Op    BinaryOp
	Left  Expr
	Right Expr
	Span_ Span
}

func (BinaryExpr) IsExpr() ExprKind { return ExprBinaryOp }
func (b BinaryExpr) NodeSpan() Span  { return b.Span_ }

type UnaryOp string

const (
	UNARY_OP_POSITIVE = "+"
	UNARY_OP_NEGATIVE = "-"
	UNARY_OP_NOT      = "!"
	UNARY_OP_RAW_REF  = "&raw"
	UNARY_OP_REF      = "&"
	UNARY_OP_DEREF    = "*"
)

type UnaryExpr struct {
	Op    UnaryOp
	Expr  Expr
	Span_ Span
}

func (UnaryExpr) IsExpr() ExprKind { return ExprUnaryOp }
func (u UnaryExpr) NodeSpan() Span  { return u.Span_ }

// CastExpr represents explicit type casting: expr as Type
type CastExpr struct {
	Expr       Expr
	TargetType TYPE_REF
	Span_      Span
}

func (CastExpr) IsExpr() ExprKind { return ExprCast }
func (c CastExpr) NodeSpan() Span  { return c.Span_ }

// IsExpr represents runtime/compile-time type testing: expr is [not] Type
type IsExpr struct {
	Expr       Expr
	TargetType TYPE_REF
	Negated    bool
	Span_      Span
}

func (IsExpr) IsExpr() ExprKind { return ExprIs }
func (i IsExpr) NodeSpan() Span  { return i.Span_ }

// =====================
// POSTFIX SUFFIXES
// =====================

type SuffixKind int

const (
	SuffixKindMember SuffixKind = iota
	SuffixKindAssociated
	SuffixKindNamespace
	SuffixKindCall
	SuffixKindCallTypeMake
	SuffixKindIndex
	SuffixKindTraitMat
	SuffixKindConstructorCall
)

type Suffix interface {
	Node
	IsSuffix() SuffixKind
}

type SuffixMemberAccess struct {
	Name  string
	Span_ Span
}

func (SuffixMemberAccess) IsSuffix() SuffixKind { return SuffixKindMember }
func (s SuffixMemberAccess) NodeSpan() Span     { return s.Span_ }

type SuffixAssociatedAccess struct {
	Name  string
	Span_ Span
}

func (SuffixAssociatedAccess) IsSuffix() SuffixKind { return SuffixKindAssociated }
func (s SuffixAssociatedAccess) NodeSpan() Span     { return s.Span_ }

type SuffixNamespaceAccess struct {
	Name  string
	Span_ Span
}

func (SuffixNamespaceAccess) IsSuffix() SuffixKind { return SuffixKindNamespace }
func (s SuffixNamespaceAccess) NodeSpan() Span     { return s.Span_ }

type SuffixCall struct {
	Args  []Expr
	Span_ Span
}

func (SuffixCall) IsSuffix() SuffixKind { return SuffixKindCall }
func (s SuffixCall) NodeSpan() Span     { return s.Span_ }

type SuffixCallTypeMake struct {
	Data  any // ObjectLiteral or ListLiteral
	Span_ Span
}

func (SuffixCallTypeMake) IsSuffix() SuffixKind { return SuffixKindCallTypeMake }
func (s SuffixCallTypeMake) NodeSpan() Span     { return s.Span_ }

type SuffixConstructorCall struct {
	Args  []Expr
	Span_ Span
}

func (SuffixConstructorCall) IsSuffix() SuffixKind { return SuffixKindConstructorCall }
func (s SuffixConstructorCall) NodeSpan() Span     { return s.Span_ }

type SuffixIndex struct {
	Index Expr
	Span_ Span
}

func (SuffixIndex) IsSuffix() SuffixKind { return SuffixKindIndex }
func (s SuffixIndex) NodeSpan() Span     { return s.Span_ }

type SuffixTraitMat struct {
	TargetType TYPE_REF
	Span_      Span
}

func (SuffixTraitMat) IsSuffix() SuffixKind { return SuffixKindTraitMat }
func (s SuffixTraitMat) NodeSpan() Span     { return s.Span_ }

type PostFixExpr struct {
	Base   Expr
	Suffix []Suffix
	Span_  Span
}

func (PostFixExpr) IsExpr() ExprKind { return ExprPostfix }
func (p PostFixExpr) NodeSpan() Span  { return p.Span_ }

// =====================
// CONTROL FLOW
// =====================

// IfStmt represents standard if statement with optional scoped header bindings: if: [let x = 0] x > 0 { ... }
type IfStmt struct {
	Header []BIND_DEC // Optional scoped header bindings
	Cond   Expr
	Then   Block
	Else   any // nil, Block, or IfStmt
	Span_  Span
}

func (IfStmt) IsExpr() ExprKind { return ExprIf }
func (i IfStmt) NodeSpan() Span  { return i.Span_ }

type PackedIfArm struct {
	Cond  Expr // nil if else arm
	Body  any  // Block or Expr
	Span_ Span
}

func (p PackedIfArm) NodeSpan() Span { return p.Span_ }

// PackedIfStmt represents packed if statement with optional scoped header bindings: if: [let x = 0] { ... }
type PackedIfStmt struct {
	Header []BIND_DEC // Optional scoped header bindings
	Arms   []PackedIfArm
	Span_  Span
}

func (PackedIfStmt) IsExpr() ExprKind { return ExprPackedIf }
func (p PackedIfStmt) NodeSpan() Span  { return p.Span_ }

type WhileStmt struct {
	Cond  Expr
	Body  Block
	Span_ Span
}

func (WhileStmt) IsExpr() ExprKind { return ExprWhile }
func (w WhileStmt) NodeSpan() Span  { return w.Span_ }

// LoopStmt represents infinite loop or post-condition loop (loop { ... } while: [cond])
type LoopStmt struct {
	Body      Block
	WhileCond Expr // nil if infinite loop, non-nil if post-condition loop
	Span_     Span
}

func (LoopStmt) IsExpr() ExprKind { return ExprLoop }
func (l LoopStmt) NodeSpan() Span  { return l.Span_ }

type ForStmt struct {
	VarName string
	Iter    Expr
	Body    Block
	Span_   Span
}

func (ForStmt) IsExpr() ExprKind { return ExprFor }
func (f ForStmt) NodeSpan() Span  { return f.Span_ }

type BreakStmt struct {
	Label string
	Value Expr
	Span_ Span
}

func (BreakStmt) IsExpr() ExprKind { return ExprBreak }
func (b BreakStmt) NodeSpan() Span  { return b.Span_ }

type ReturnStmt struct {
	Value Expr
	Span_ Span
}

func (ReturnStmt) IsExpr() ExprKind { return ExprReturn }
func (r ReturnStmt) NodeSpan() Span  { return r.Span_ }

type DropStmt struct {
	Expr  Expr
	Span_ Span
}

func (DropStmt) IsExpr() ExprKind { return ExprDrop }
func (d DropStmt) NodeSpan() Span  { return d.Span_ }

type AssignStmt struct {
	Target Expr
	Value  Expr
	Span_  Span
}

func (AssignStmt) IsExpr() ExprKind { return ExprAssign }
func (a AssignStmt) NodeSpan() Span  { return a.Span_ }

type LabelDec struct {
	Name  string
	Body  Block
	Span_ Span
}

func (LabelDec) IsExpr() ExprKind { return ExprLabelDec }
func (l LabelDec) NodeSpan() Span  { return l.Span_ }

// =====================
// PATTERN MATCHING
// =====================

type PatternKind int

const (
	PatternWildcard PatternKind = iota // _
	PatternVariant                     // Shape::Circle(r) or Circle(r) or Shape::Point
	PatternLiteral                     // 1, "foo", true
	PatternTuple                       // (a, b)
)

type MatchPattern interface {
	Node
	IsPattern() PatternKind
}

type TuplePattern struct {
	Elements []MatchPattern
	Span_    Span
}

func (TuplePattern) IsPattern() PatternKind { return PatternTuple }
func (t TuplePattern) NodeSpan() Span       { return t.Span_ }

type WildcardPattern struct {
	Span_ Span
}

func (WildcardPattern) IsPattern() PatternKind { return PatternWildcard }
func (w WildcardPattern) NodeSpan() Span       { return w.Span_ }

type VariantPattern struct {
	TypeName string   // e.g. "Shape" (optional, can be empty if inferred)
	Variant  string   // e.g. "Circle"
	Bindings []string // e.g. ["r"] or ["w", "h"]
	Span_    Span
}

func (VariantPattern) IsPattern() PatternKind { return PatternVariant }
func (v VariantPattern) NodeSpan() Span       { return v.Span_ }

type LiteralPattern struct {
	Literal Literal
	Span_   Span
}

func (LiteralPattern) IsPattern() PatternKind { return PatternLiteral }
func (l LiteralPattern) NodeSpan() Span       { return l.Span_ }

type MatchArm struct {
	Pattern MatchPattern
	Body    any // Expr or Block
	Span_   Span
}

func (m MatchArm) NodeSpan() Span { return m.Span_ }

type MatchExpr struct {
	Target Expr
	Arms   []MatchArm
	Span_  Span
}

func (MatchExpr) IsExpr() ExprKind { return ExprMatch }
func (m MatchExpr) NodeSpan() Span { return m.Span_ }

type MustExpr struct {
	Expr  Expr
	Span_ Span
}

func (MustExpr) IsExpr() ExprKind { return ExprMust }
func (m MustExpr) NodeSpan() Span { return m.Span_ }

type TryExpr struct {
	Expr  Expr
	Span_ Span
}

func (TryExpr) IsExpr() ExprKind { return ExprTry }
func (t TryExpr) NodeSpan() Span { return t.Span_ }

type TupleExpr struct {
	Elements []Expr
	Span_    Span
}

func (TupleExpr) IsExpr() ExprKind { return ExprTuple }
func (t TupleExpr) NodeSpan() Span { return t.Span_ }

type AnonFnExpr struct {
	TypeParams  []TypeParam
	Constraints []GenericConstraint
	Params      []FnParam
	ReturnType  *TYPE_REF
	Body        Block
	Span_       Span
}

func (AnonFnExpr) IsExpr() ExprKind { return ExprAnonFn }
func (a AnonFnExpr) NodeSpan() Span { return a.Span_ }

type InterpolatedStringExpr struct {
	Parts []Expr
	Span_ Span
}

func (InterpolatedStringExpr) IsExpr() ExprKind { return ExprInterpolatedString }
func (i InterpolatedStringExpr) NodeSpan() Span  { return i.Span_ }


