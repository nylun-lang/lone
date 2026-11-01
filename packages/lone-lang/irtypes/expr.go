package irtypes

type ExprKind int

const (
	EXPR_LITERAL ExprKind = iota
	EXPR_OBJ_LITERAL
	EXPR_LIST_LITERAL
	EXPR_TRAIT_MATERIALIZATION
	EXPR_NAME_REFERENCE
	EXPR_UNARY_OP
	EXPR_BINARY_OP
)

type Expr interface {
	IsExpr() ExprKind
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
	SUFFIX_KIND_CALL_TYPE_INIT
	SUFFIX_KIND_CALL_TYPE_MAKE
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

type SuffixCallTypeInit struct {
	Args []Expr
	Pos  Position
}

func (SuffixCallTypeInit) IsSuffix() SuffixKind {
	return SUFFIX_KIND_CALL_TYPE_INIT
}

type TypeMakeKind = int

const (
	CALL_TYPE_MAKE_OBJ TypeMakeKind = iota
	CALL_TYPE_MAKE_LIST
)

type SuffixCallTypeMake struct {
	Kind TypeMakeKind
	Arg  any
	Pos  Position
}

func (SuffixCallTypeMake) IsSuffix() SuffixKind {
	return SUFFIX_KIND_CALL_TYPE_MAKE
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
	LITERAL_DYNAMIC_SIZE
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
// NAME REF EXPR
// =====================

type NameRef struct {
	Name string
	Pos  Position
}

func (NameRef) IsExpr() ExprKind {
	return EXPR_NAME_REFERENCE
}

// =====================
// OBJ LITERAL EXPR
// =====================

type ObjectLiteral struct {
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

// =====================
// LIST LITERAL EXPR
// =====================

type ListLiteral struct {
	Content []Expr
	Pos     Position
}

func (ListLiteral) IsExpr() ExprKind {
	return EXPR_LIST_LITERAL
}
