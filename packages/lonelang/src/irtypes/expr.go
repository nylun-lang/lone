package irtypes

// =====================
// EXPR
// =====================

type ExprKind int

const (
	EXPR_LITERAL = iota
	EXPR_OBJ_LITERAL
	EXPR_TRAIT_MATERIALIZATION
	EXPR_NAME_REFERENCE
	EXPR_UNARY_OP
	EXPR_BINARY_OP
)

type Expr interface {
	IsExpr() ExprKind
}

// =====================
// BINARY EXPR
// =====================

type BinaryOp string

const (
	BINARY_OP_PLUS  BinaryOp = "+"
	BINARY_OP_MINUS BinaryOp = "-"
	BIARY_OP_TIME   BinaryOp = "*"
	BINARY_OP_DIV   BinaryOp = "/"
	BINARY_OP_MOD   BinaryOp = "*"
)

type BinaryExpr struct {
	Op   BinaryOp
	Expr Expr
	Pos  Position
}

func (BinaryExpr) IsExpr() ExprKind {
	return EXPR_BINARY_OP
}

// =====================
// UNARY EXPR
// =====================

type UnaryOp string

const (
	UNARY_OP_POSITIVE = "+"
	UNARY_OP_NEGATIVE = "-"
	UNARY_OP_NOT      = "!"
	UNARY_OP_REF      = "&"
	UNARY_OP_DEREF    = "*"
)

type UnaryExpr struct {
	Op   UnaryOp
	Expr Expr
	Pos  Position
}

func (UnaryExpr) IsExpr() ExprKind {
	return EXPR_UNARY_OP
}

// =====================
// POSTFIX EXPR
// =====================

type SuffixKind int

const (
	SUFFIX_KIND_MEMBER = iota
	SUFFIX_KIND_ASSOCIATED
	SUFFIX_KIND_NAMESPACE
	SUFFIX_KIND_CALL
)

type Suffix interface {
	IsSuffix() SuffixKind
}

type PostFixExpr struct {
	Base   Expr
	Suffix []Suffix
	Pos    Position
}

func (PostFixExpr) IsExpr() ExprKind {
	return EXPR_NAME_REFERENCE
}

type SuffixCall struct {
	Args []Expr
	Pos  Position
}

func (SuffixCall) IsSuffix() SuffixKind {
	return SUFFIX_KIND_CALL
}

type SuffixMemberAccess struct {
	Name string
	Pos  Position
}

func (SuffixMemberAccess) IsSuffix() SuffixKind {
	return SUFFIX_KIND_MEMBER
}

type SuffixAssociatedAccess struct {
	Name string
	Pos  Position
}

func (SuffixAssociatedAccess) IsSuffix() SuffixKind {
	return SUFFIX_KIND_ASSOCIATED
}

type SuffixNamespaceAccess struct {
	Name string
	Pos  Position
}

func (SuffixNamespaceAccess) IsSuffix() SuffixKind {
	return SUFFIX_KIND_NAMESPACE
}

// =====================
// NAMEREF EXPR
// =====================

type NameRef struct {
	Name string
	Pos  Position
}

func (NameRef) IsExpr() ExprKind {
	return EXPR_NAME_REFERENCE
}

// =====================
// TRAIT MAT EXPR
// =====================

type TraitMat struct {
	TraitName string
	TypeName  string
	Pos       Position
}

func (TraitMat) IsExpr() ExprKind {
	return EXPR_TRAIT_MATERIALIZATION
}

// =====================
// LITERAL EXPR
// =====================

type LiteralKind int

const (
	LITERAL_BOOL = iota
	LITERAL_NONE
	LITERAL_STRING
	LITERAL_INT
	LITERAL_FLOAT
	LITERAL_UNINITIALIZED
)

type Literal struct {
	Kind  LiteralKind
	Value any
	Pos   Position
}

func (Literal) IsExpr() ExprKind {
	return EXPR_LITERAL
}

// =====================
// OBJ LITERAL EXPR
// =====================

type ObjectLiteral struct {
	Name   string        `json:"name"`
	Fields []FieldAssign `json:"fields"`
	Pos    Position
}

func (ObjectLiteral) IsExpr() ExprKind {
	return EXPR_OBJ_LITERAL
}

type FieldAssign struct {
	Name string `json:"name"`
	Expr Expr   `json:"expr"`
}
