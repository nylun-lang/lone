package loneanalyzer

import "fmt"

type ScopeKind string

const (
	ScopeFile     ScopeKind = "file"
	ScopeFunction ScopeKind = "function"
	ScopeBlock    ScopeKind = "block"
	ScopeHeader   ScopeKind = "header" // Scoped header clause in if: [bindings]
	ScopeLoop     ScopeKind = "loop"
)

type SymbolKind string

const (
	SymbolVar   SymbolKind = "var"
	SymbolFn    SymbolKind = "fn"
	SymbolType  SymbolKind = "type"
	SymbolLabel SymbolKind = "label"
)

type Symbol struct {
	Name         string
	Kind         SymbolKind
	Type         Type
	IsMutable    bool // true for 'var' (mutable), false for 'let' (immutable)
	Scope        *Scope
	Node         any
	AddressTaken bool
}

type Scope struct {
	Kind     ScopeKind
	Parent   *Scope
	Children []*Scope
	Symbols  map[string]*Symbol
}

func NewScope(kind ScopeKind, parent *Scope) *Scope {
	s := &Scope{
		Kind:     kind,
		Parent:   parent,
		Children: make([]*Scope, 0),
		Symbols:  make(map[string]*Symbol),
	}
	if parent != nil {
		parent.Children = append(parent.Children, s)
	}
	return s
}

func (s *Scope) Define(sym *Symbol) error {
	if _, exists := s.Symbols[sym.Name]; exists {
		return fmt.Errorf("symbol %q already declared in current scope", sym.Name)
	}
	sym.Scope = s
	s.Symbols[sym.Name] = sym
	return nil
}

func (s *Scope) LookupLocal(name string) *Symbol {
	return s.Symbols[name]
}

func (s *Scope) Lookup(name string) *Symbol {
	for cur := s; cur != nil; cur = cur.Parent {
		if sym, ok := cur.Symbols[name]; ok {
			return sym
		}
	}
	return nil
}

type ScopeStack struct {
	root    *Scope
	current *Scope
}

func NewScopeStack() *ScopeStack {
	root := NewScope(ScopeFile, nil)
	registerBuiltins(root)
	return &ScopeStack{
		root:    root,
		current: root,
	}
}

func (ss *ScopeStack) Push(kind ScopeKind) *Scope {
	child := NewScope(kind, ss.current)
	ss.current = child
	return child
}

func (ss *ScopeStack) Pop() {
	if ss.current != nil && ss.current.Parent != nil {
		ss.current = ss.current.Parent
	}
}

func (ss *ScopeStack) Current() *Scope {
	return ss.current
}

func (ss *ScopeStack) Define(sym *Symbol) error {
	return ss.current.Define(sym)
}

func (ss *ScopeStack) Lookup(name string) *Symbol {
	return ss.current.Lookup(name)
}

func (ss *ScopeStack) LookupLocal(name string) *Symbol {
	return ss.current.LookupLocal(name)
}

func registerBuiltins(root *Scope) {
	// Standard types
	prims := []*PrimitiveType{
		TypeVoid, TypeNone, TypeAny, TypeBool, TypeStr, TypeChar,
		TypeI8, TypeI16, TypeI32, TypeI64,
		TypeU8, TypeU16, TypeU32, TypeU64,
		TypeF32, TypeF64,
	}
	for _, p := range prims {
		_ = root.Define(&Symbol{Name: p.Name, Kind: SymbolType, Type: p})
	}
	// Aliases
	_ = root.Define(&Symbol{Name: "int", Kind: SymbolType, Type: TypeI32})
	_ = root.Define(&Symbol{Name: "uint", Kind: SymbolType, Type: TypeU32})
	_ = root.Define(&Symbol{Name: "float", Kind: SymbolType, Type: TypeF32})
	_ = root.Define(&Symbol{Name: "string", Kind: SymbolType, Type: TypeStr})

	// Built-in intrinsics
	_ = root.Define(&Symbol{
		Name: "#print",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamInfo{{Name: "s", Type: TypeStr}},
			Return: TypeVoid,
		},
	})
	_ = root.Define(&Symbol{
		Name: "print",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamInfo{{Name: "s", Type: TypeStr}},
			Return: TypeVoid,
		},
	})
	_ = root.Define(&Symbol{
		Name: "println",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamInfo{{Name: "s", Type: TypeStr}},
			Return: TypeVoid,
		},
	})
	_ = root.Define(&Symbol{
		Name: "#panic",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamInfo{{Name: "msg", Type: TypeStr}},
			Return: TypeVoid,
		},
	})
	_ = root.Define(&Symbol{
		Name: "#alloc",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamInfo{{Name: "size", Type: TypeI32}},
			Return: TypeI32,
		},
	})
	_ = root.Define(&Symbol{
		Name: "#free",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamInfo{{Name: "ptr", Type: TypeI32}},
			Return: TypeVoid,
		},
	})
	_ = root.Define(&Symbol{
		Name: "#realloc",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamInfo{{Name: "ptr", Type: TypeI32}, {Name: "new_size", Type: TypeI32}},
			Return: TypeI32,
		},
	})
	_ = root.Define(&Symbol{
		Name: "#memory_grow",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamInfo{{Name: "pages", Type: TypeI32}},
			Return: TypeI32,
		},
	})
	_ = root.Define(&Symbol{
		Name: "#memory_size",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamInfo{},
			Return: TypeI32,
		},
	})
	_ = root.Define(&Symbol{
		Name: "#stack_pointer",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamInfo{},
			Return: TypeI32,
		},
	})
	_ = root.Define(&Symbol{
		Name: "#stack_alloc",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamInfo{{Name: "size", Type: TypeI32}},
			Return: TypeI32,
		},
	})
	_ = root.Define(&Symbol{
		Name: "#stack_free",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamInfo{{Name: "size", Type: TypeI32}},
			Return: TypeVoid,
		},
	})
}
