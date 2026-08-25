package irtypes

// ============
// VAR DEC
// ============

type VariableDec struct {
	Name  string   `json:"name"`
	Type  TypeRef  `json:"type"`
	Const bool     `json:"is_const"`
	Value Expr     `json:"value"`
	Pos   Position `json:"position"`
}

// ============
// TYPE DEC
// ============

type TypeDecKind int

const (
	TYPE_DEC_KINDOF_ALIAS TypeDecKind = iota
	TYPE_DEC_KINDOF_COMPOSITE
)

type TypeDec struct {
	Name  string      `json:"name"`
	Kind  TypeDecKind `json:"kind"`
	Value any         `json:"type"`
	Pos   Position    `json:"position"`
}

// TYPE REF

type TypeRef struct {
	Name     string    `json:"name"`
	Args     []TypeRef `json:"args"`
	Modifier TypeRefModifier
	Pos      Position
}

type TypeRefModifier struct {
	ReadOnly    bool
	LiteralOnly bool
}

// COMPOSITE TYPE

type CompositeTypeKind int

const (
	COMPOSITE_TYPEOF_STATE = iota
	COMPOSITE_TYPEOF_STRUCT
	COMPOSITE_TYPEOF_RECORD
)

type CompositeType struct {
	Kind   CompositeTypeKind    `json:"kind"`
	Fields []CompositeTypeField `json:"fields"`
	Pos    Position
}

type CompositeTypeField struct {
	Name string  `json:"name"`
	Type TypeRef `json:"type"`
}
