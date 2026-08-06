package types

import "strconv"

type Literal interface {
	Expr
	IsLiteral()
}

type NumKind int8

const (
	NUM_KIND_FLOAT NumKind = iota
	NUM_KIND_INT
)

type Literal_Number struct {
	Kind NumKind `json:"kind"`
	Text string  `json:"text"`
}

func (Literal_Number) IsLiteral() {}
func (Literal_Number) IsExpr()    {}

func (n Literal_Number) ParseInt64() (int64, error) {
	return strconv.ParseInt(n.Text, 10, 64)
}

func (n Literal_Number) ParseFloat64() (float64, error) {
	return strconv.ParseFloat(n.Text, 64)
}

type Literal_String struct {
	Text string `json:"text"`
}

func (Literal_String) IsExpr()    {}
func (Literal_String) IsLiteral() {}

type Literal_Bool struct {
	Value bool `json:"value"`
}

func (Literal_Bool) IsExpr()    {}
func (Literal_Bool) IsLiteral() {}

type Literal_Nil struct{}

func (Literal_Nil) IsExpr()    {}
func (Literal_Nil) IsLiteral() {}

type Literal_Unitialised struct{}

func (Literal_Unitialised) IsExpr()    {}
func (Literal_Unitialised) IsLiteral() {}

type ObjectLiteral struct {
	Name   string        `json:"name"`
	Fields []FieldAssign `json:"fields"`
}

func (ObjectLiteral) IsExpr() {}

type FieldAssign struct {
	Name string `json:"name"`
	Expr Expr   `json:"expr"`
}
