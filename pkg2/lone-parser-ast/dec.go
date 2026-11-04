package loneast

// ImportDecl represents a flat import statement: import .sym1, .sym2 from :source
type ImportDecl struct {
	Symbols     []string
	Source      string
	IsNamespace bool
	Alias       string
	Span_       Span
}

func (i ImportDecl) NodeSpan() Span { return i.Span_ }

// WasmDirective represents @fa:wasm or legacy directive.
type WasmDirective struct {
	Name    string
	Imports []ImportDecl
	Inline  bool
	Span_   Span
}

func (w WasmDirective) NodeSpan() Span { return w.Span_ }

// BIND_DEC represents variable and constant bindings (let and var).
type BIND_DEC struct {
	Name      string
	Pattern   MatchPattern // non-nil for destructuring: let Shape::Circle(r) = s
	IsPublic  bool
	IsMutable bool // true for 'var' (mutable), false for 'let' (immutable)
	Value     Expr
	Type      TYPE_REF
	Span_     Span
}

func (b BIND_DEC) NodeSpan() Span { return b.Span_ }

// FnParam represents a typed function parameter.
type FnParam struct {
	Name  string
	Type  TYPE_REF
	Span_ Span
}

func (p FnParam) NodeSpan() Span { return p.Span_ }

// TypeParam represents a generic type parameter: T or U = DefaultType
type TypeParam struct {
	Name    string
	Default *TYPE_REF
	Span_   Span
}

func (tp TypeParam) NodeSpan() Span { return tp.Span_ }

// FN_DEC represents a function declaration.
type FN_DEC struct {
	Name       string
	IsPublic   bool
	TypeParams []TypeParam
	Params     []FnParam
	ReturnType *TYPE_REF
	Body       Block
	Span_      Span
}

func (f FN_DEC) NodeSpan() Span { return f.Span_ }

// StructField represents a field inside a struct declaration.
type StructField struct {
	Name  string
	Type  TYPE_REF
	Span_ Span
}

func (s StructField) NodeSpan() Span { return s.Span_ }

// TraitMethodSig represents a signature inside a trait declaration.
type TraitMethodSig struct {
	Name       string
	Params     []FnParam
	ReturnType *TYPE_REF
	Span_      Span
}

func (t TraitMethodSig) NodeSpan() Span { return t.Span_ }

type TypeDecKind int

const (
	TYPE_DEC_ALIAS TypeDecKind = iota
	TYPE_DEC_STRUCT
	TYPE_DEC_TRAIT
	TYPE_DEC_ENUM
)

// EnumVariant represents a variant within an enum declaration.
type EnumVariant struct {
	Name   string
	Fields []StructField
	Tag    int
	Span_  Span
}

func (v EnumVariant) NodeSpan() Span { return v.Span_ }

// TYPE_DEC represents struct, trait, enum, or alias declarations.
type TYPE_DEC struct {
	Name       string
	IsPublic   bool
	TypeParams []TypeParam
	Kind       TypeDecKind
	AliasType  *TYPE_REF
	Fields     []StructField
	Variants   []EnumVariant
	Methods    []FN_DEC
	TraitSigs  []TraitMethodSig
	Span_      Span
}

func (t TYPE_DEC) NodeSpan() Span { return t.Span_ }

// IMPL_DEC represents trait implementation on a type: impl TrTrait for TargetType { ... }
type IMPL_DEC struct {
	TraitName  string
	TraitArgs  []TYPE_REF
	TargetType TYPE_REF
	Methods    []FN_DEC
	Span_      Span
}

func (i IMPL_DEC) NodeSpan() Span { return i.Span_ }

// Block represents a sequence of statements enclosed in braces.
type Block struct {
	Stmts []any
	Span_ Span
}

func (b Block) NodeSpan() Span { return b.Span_ }

// =====================
// TYPE REFERENCES
// =====================

type TypeRefKind int

const (
	TYPE_BASE TypeRefKind = iota
	TYPE_LIST
	TYPE_FN
)

type TypeConstrain struct {
	ReadOnly bool
	Optional bool
	Comptime bool
}

type TYPE_REF struct {
	Kind      TypeRefKind
	Constrain TypeConstrain
	Value     any // *TypeBase, *TypeList, *TypeFn
	Span_     Span
}

func (t TYPE_REF) NodeSpan() Span { return t.Span_ }

type TypeBase struct {
	Name    string
	Variant string // non-empty if associated variant e.g. Shape::Circle
	Args    []TYPE_REF
}

type TypeList struct {
	Size     Expr // nil or dynamic size literal for slice []T, constant Expr for fixed [N]T
	ItemType TYPE_REF
}

func (l TypeList) IsFixed() bool {
	return l.Size != nil
}

type TypeFn struct {
	Params     []FnParam
	ReturnType *TYPE_REF
}
