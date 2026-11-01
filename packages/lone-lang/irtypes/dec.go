package irtypes

type BIND_DEC struct {
	Name  string
	Pos   Position
	Value Expr
	Type  TYPE_REF
}

type TYPE_REF struct {
	Pos       Position
	Name      string
	Args      []TYPE_REF
	Constrain TypeConstrain
}

type TypeConstrain struct {
	ReadOnly bool
	Optional bool
	Comptime bool
}
