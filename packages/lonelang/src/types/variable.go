package types

type VariableDec struct {
	Name    string  `json:"name"`
	Type    TypeRef `json:"type"`
	IsConst bool    `json:"is_const"`
	Value   Expr    `json:"value"`
}

type TypeRef struct {
	Name string    `json:"name"`
	Args []TypeRef `json:"args"`
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
