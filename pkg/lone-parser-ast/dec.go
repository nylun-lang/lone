package loneast

type ImportDecl struct {
	Symbols     []string // List of imported symbol names (e.g. ["readline", "print"])
	Source      string   // Target namespace or relative path (e.g. ":std:io", ".std")
	IsNamespace bool     // True if Source starts with ':'
	Alias       string   // Optional alias (e.g. for "import :std:io as io")
	Pos         Position
}

type WasmDirective struct {
	Name    string       // Directive identifier (e.g. "wasm", "doc")
	Imports []ImportDecl
	Inline  bool // True for single-line directive
	Pos     Position
}

type HostImportParam struct {
	Name     string
	LoneType string
	WasmType string
}

type HostImportDecl struct {
	Namespace      string
	WasmModule     string
	WasmName       string
	LocalName      string
	Params         []HostImportParam
	ReturnLoneType string
	ReturnWasmType string
	Pos            Position
}

type ModuleScopeDecl struct {
	ModuleID string
	FilePath string
	Decls    []any
	Exports  map[string]bool
}

type ResolvedImportDecl struct {
	ModuleID string
	Source   string
	Symbols  []string
	Alias    string
	Pos      Position
}

type BIND_DEC struct {
	Name     string
	IsPublic bool
	Pos      Position
	Value    Expr
	Type     TYPE_REF
}

type FN_DEC struct {
	Name       string
	IsPublic   bool
	Pos        Position
	Params     []FnParam
	ReturnType *TYPE_REF
	Body       Block
}

type AssignStmt struct {
	Target Expr
	Value  Expr
	Pos    Position
}

type TypeDecKind int

const (
	TYPE_DEC_ALIAS TypeDecKind = iota
	TYPE_DEC_STRUCT
	TYPE_DEC_TRAIT
)

type StructField struct {
	Name string
	Type TYPE_REF
	Pos  Position
}

type TraitMethodSig struct {
	Name       string
	Params     []FnParam
	ReturnType *TYPE_REF
	Pos        Position
}

type TYPE_DEC struct {
	Name       string
	IsPublic   bool
	TypeParams []string
	Kind       TypeDecKind
	AliasType  *TYPE_REF
	Fields     []StructField
	Methods    []FN_DEC
	TraitSigs  []TraitMethodSig
	Pos        Position
}

type IMPL_DEC struct {
	TraitName  string
	TraitArgs  []TYPE_REF
	TargetType TYPE_REF
	Methods    []FN_DEC
	Pos        Position
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
