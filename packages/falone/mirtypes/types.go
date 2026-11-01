package mirtypes

type MirNode interface{}

type MirT int

const (
	MirI32 MirT = iota
	MirI64
	MirF32
	MirF64
)

type MirAdd struct {
	Type MirT
	Arg1 MirNode
	Arg2 MirNode
}

type MirNum struct {
	Type  MirT
	Value int
}

type MirSetReg struct {
	Name  string
	Type  MirT
	Value MirNode
}

type MirGetReg struct {
	Name string
}

type MirGetGlobal struct {
	Name string
}

type MirSetGlobal struct {
	Name  string
	Type  MirT
	Value MirNode
}
