package loneanalyzer

import (
	"fmt"
)

// ScopeKind identifies the syntactic structure that owns a lexical scope.
type ScopeKind string

const (
	ScopeFile     ScopeKind = "file"
	ScopeFunction ScopeKind = "function"
	ScopeBlock    ScopeKind = "block"
	ScopeLabel    ScopeKind = "label"
	ScopeLoop     ScopeKind = "loop"
)

// SymbolKind classifies declared symbols in the AST.
type SymbolKind string

const (
	SymbolVar   SymbolKind = "var"
	SymbolFn    SymbolKind = "fn"
	SymbolType  SymbolKind = "type"
	SymbolLabel SymbolKind = "label"
)

// Symbol represents a declared identifier in a scope.
type Symbol struct {
	Name         string
	MangledName  string // Optional lowered symbol name (e.g. for methods or aliased imports)
	Kind         SymbolKind
	Type         Type
	Scope        *Scope
	Node         any   // original AST node reference
	DeclNode     TNode // typed IR declaration node (e.g. *TVarDec or *TFnParam)
	AddressTaken bool  // true if &raw or & was applied to this variable
}

// Scope represents a lexical environment holding symbols and nesting relationships.
type Scope struct {
	Kind     ScopeKind
	Name     string
	Parent   *Scope
	Children []*Scope
	Symbols  map[string]*Symbol
}

// NewScope creates a new child scope under parent.
func NewScope(kind ScopeKind, name string, parent *Scope) *Scope {
	s := &Scope{
		Kind:     kind,
		Name:     name,
		Parent:   parent,
		Symbols:  make(map[string]*Symbol),
		Children: make([]*Scope, 0),
	}
	if parent != nil {
		parent.Children = append(parent.Children, s)
	}
	return s
}

// Define adds a symbol to this specific scope. Returns an error if already defined in this scope.
func (s *Scope) Define(sym *Symbol) error {
	if _, exists := s.Symbols[sym.Name]; exists {
		return fmt.Errorf("symbol %q already declared in current scope", sym.Name)
	}
	sym.Scope = s
	s.Symbols[sym.Name] = sym
	return nil
}

// LookupLocal finds a symbol defined strictly in this scope (non-recursive).
func (s *Scope) LookupLocal(name string) *Symbol {
	return s.Symbols[name]
}

// Lookup resolves a symbol by climbing the lexical scope chain.
func (s *Scope) Lookup(name string) *Symbol {
	for cur := s; cur != nil; cur = cur.Parent {
		if sym, ok := cur.Symbols[name]; ok {
			return sym
		}
	}
	return nil
}

// ScopeStack manages nested scopes during AST traversal.
type ScopeStack struct {
	root    *Scope
	current *Scope
	all     []*Scope
}

// NewScopeStack initializes a stack with a top-level file scope.
func NewScopeStack(fileName string) *ScopeStack {
	root := NewScope(ScopeFile, fileName, nil)
	registerBuiltins(root)
	return &ScopeStack{
		root:    root,
		current: root,
		all:     []*Scope{root},
	}
}

func registerBuiltins(root *Scope) {
	_ = root.Define(&Symbol{
		Name: "#print",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamType{
				{Name: "str", Type: TypeStr},
			},
			ReturnType: TypeVoid,
		},
	})
	_ = root.Define(&Symbol{
		Name: "#heap_alloc",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamType{
				{Name: "size", Type: TypeI32},
			},
			ReturnType: &PtrType{Elem: TypeU8},
		},
	})
	_ = root.Define(&Symbol{
		Name: "#ptr_to_str",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamType{
				{Name: "ptr", Type: &PtrType{Elem: TypeU8}},
			},
			ReturnType: TypeStr,
		},
	})
	_ = root.Define(&Symbol{
		Name: "#str_to_ptr",
		Kind: SymbolFn,
		Type: &FnType{
			Params: []FnParamType{
				{Name: "s", Type: TypeStr},
			},
			ReturnType: &PtrType{Elem: TypeU8},
		},
	})
}

// Root returns the root file scope.
func (ss *ScopeStack) Root() *Scope {
	return ss.root
}

// All returns all lexical scopes created during AST traversal.
func (ss *ScopeStack) All() []*Scope {
	return ss.all
}

// Current returns the current scope at the top of the stack.
func (ss *ScopeStack) Current() *Scope {
	return ss.current
}

// Push enters a new child scope.
func (ss *ScopeStack) Push(kind ScopeKind, name string) *Scope {
	child := NewScope(kind, name, ss.current)
	ss.current = child
	ss.all = append(ss.all, child)
	return child
}

// Pop exits the current scope and returns to its parent.
func (ss *ScopeStack) Pop() *Scope {
	if ss.current == nil || ss.current.Parent == nil {
		return ss.current
	}
	popped := ss.current
	ss.current = ss.current.Parent
	return popped
}

// Define defines a symbol in the current scope.
func (ss *ScopeStack) Define(sym *Symbol) error {
	return ss.current.Define(sym)
}

// Resolve looks up a symbol starting from current scope.
func (ss *ScopeStack) Resolve(name string) *Symbol {
	return ss.current.Lookup(name)
}

// HasEnclosing checks if any ancestor in the stack matches the given ScopeKind.
func (ss *ScopeStack) HasEnclosing(kind ScopeKind) bool {
	for cur := ss.current; cur != nil; cur = cur.Parent {
		if cur.Kind == kind {
			return true
		}
	}
	return false
}

// FindEnclosing finds the closest ancestor of the given ScopeKind.
func (ss *ScopeStack) FindEnclosing(kind ScopeKind) *Scope {
	for cur := ss.current; cur != nil; cur = cur.Parent {
		if cur.Kind == kind {
			return cur
		}
	}
	return nil
}
