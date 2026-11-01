package irtypes

type ExprKind int

const (
	EXPR_LITERAL ExprKind = iota
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
