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
)

type Expr interface {
	IsExpr() ExprKind
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
