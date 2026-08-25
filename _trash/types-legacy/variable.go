package types

type VariableDec struct {
	Name  string  `json:"name"`
	Type  TypeRef `json:"type"`
	Const bool    `json:"is_const"`
	Value Expr    `json:"value"`
}

type TypeDec struct {
	Name  string `json:"name"`
	Value any    `json:"type"`
	IsObj bool   `json:"is_obj"`
}

type TypeRef struct {
	Name     string    `json:"name"`
	Args     []TypeRef `json:"args"`
	Modifier TypeRefModifier
}

type TypeRefModifier struct {
	ReadOnly    bool
	LiteralOnly bool
}

type ObjectKind uint8

const (
	OBJ_KIND_STRUCT ObjectKind = iota
	OBJ_KIND_STATE
	OBJ_KIND_RECORD
)

type ObjectType struct {
	Kind   ObjectKind `json:"kind"`
	Fields []FieldDec `json:"fields"`
}

type FieldDec struct {
	Name string  `json:"name"`
	Type TypeRef `json:"type"`
}
