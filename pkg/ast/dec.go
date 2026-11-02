package ast

type BIND_DEC struct {
	Name  string
	Pos   Position
	Value Expr
	Type  TYPE_REF
}

type FN_DEC struct {
	Name       string
	Pos        Position
	Params     []FnParam
	ReturnType *TYPE_REF
	Body       Block
}

func (f FN_DEC) HasReturn() bool {
	return f.ReturnType != nil
}

type Block struct {
	Stmts []any
	Pos   Position
}

type ReturnStmt struct {
	Value Expr
	Pos   Position
}

type TypeRefKind int

const (
	TYPE_BASE TypeRefKind = iota
	TYPE_LIST
	TYPE_FN
)

func (k TypeRefKind) String() string {
	switch k {
	case TYPE_BASE:
		return "TypeBase"
	case TYPE_LIST:
		return "TypeList"
	case TYPE_FN:
		return "TypeFn"
	default:
		return "Unknown"
	}
}

type TypeConstrain struct {
	ReadOnly bool
	Optional bool
	Comptime bool
}

type TYPE_REF struct {
	Pos       Position
	Kind      TypeRefKind
	Constrain TypeConstrain
	Value     any
}

func (t TYPE_REF) AsBase() *TypeBase {
	switch v := t.Value.(type) {
	case *TypeBase:
		return v
	case TypeBase:
		return &v
	default:
		return nil
	}
}

func (t TYPE_REF) AsList() *TypeList {
	switch v := t.Value.(type) {
	case *TypeList:
		return v
	case TypeList:
		return &v
	default:
		return nil
	}
}

func (t TYPE_REF) AsFn() *TypeFn {
	switch v := t.Value.(type) {
	case *TypeFn:
		return v
	case TypeFn:
		return &v
	default:
		return nil
	}
}

type TypeBase struct {
	Name string
	Args []TYPE_REF
}

type TypeList struct {
	Size     Expr
	ItemType TYPE_REF
}

func (l TypeList) IsFixed() bool {
	if l.Size == nil {
		return false
	}
	if lit, ok := l.Size.(Literal); ok && lit.Kind == LITERAL_DYNAMIC_SIZE {
		return false
	}
	return true
}

type FnParam struct {
	Name string
	Type TYPE_REF
	Size Expr
}

type TypeFn struct {
	Params     []FnParam
	ReturnType *TYPE_REF
}
