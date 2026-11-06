package lonewir

import (
	"fmt"
)

// LocalAllocator manages Wasm local variables and pools them by type to eliminate
// virtual register explosion.
type LocalAllocator struct {
	params    map[string]string // Lone param name -> Wasm param name ($p_name)
	vars      map[string]string // Lone var name -> Wasm local name
	varTypes  map[string]WasmType
	scopeVars [][]string       // Stack of variable names declared per lexical scope level
	pool      map[WasmType][]string
	counts    map[WasmType]int
	allLocals []Local
}

// NewLocalAllocator creates a new LocalAllocator for a function.
func NewLocalAllocator() *LocalAllocator {
	return &LocalAllocator{
		params:    make(map[string]string),
		vars:      make(map[string]string),
		varTypes:  make(map[string]WasmType),
		scopeVars: [][]string{nil}, // Root scope level
		pool:      make(map[WasmType][]string),
		counts:    make(map[WasmType]int),
	}
}

// AddParam registers a function parameter.
func (la *LocalAllocator) AddParam(name string) string {
	wasmName := "$p_" + name
	la.params[name] = wasmName
	return wasmName
}

// EnterScope pushes a new scope level for tracking scoped local lifetimes.
func (la *LocalAllocator) EnterScope() {
	la.scopeVars = append(la.scopeVars, nil)
}

// ExitScope pops the current scope level and reclaims locals into the free pool.
func (la *LocalAllocator) ExitScope() {
	if len(la.scopeVars) <= 1 {
		return
	}
	top := la.scopeVars[len(la.scopeVars)-1]
	la.scopeVars = la.scopeVars[:len(la.scopeVars)-1]

	for _, name := range top {
		localName, ok := la.vars[name]
		if ok {
			typ := la.varTypes[name]
			delete(la.vars, name)
			delete(la.varTypes, name)
			// Return to pool for reuse by subsequent sibling scopes
			la.pool[typ] = append(la.pool[typ], localName)
		}
	}
}

// AllocVar binds a named variable (e.g. let/var or if-header binding) to a Wasm local.
func (la *LocalAllocator) AllocVar(name string, typ WasmType) string {
	if local, exists := la.vars[name]; exists {
		return local
	}

	var localName string
	avail := la.pool[typ]
	if len(avail) > 0 {
		// Reuse coalesced local slot from free pool
		localName = avail[len(avail)-1]
		la.pool[typ] = avail[:len(avail)-1]
	} else {
		// Allocate a new typed slot
		idx := la.counts[typ]
		la.counts[typ]++
		localName = fmt.Sprintf("$%s_%d", typ, idx)
		la.allLocals = append(la.allLocals, Local{
			Name: localName,
			Type: typ,
		})
	}

	la.vars[name] = localName
	la.varTypes[name] = typ
	currentScope := len(la.scopeVars) - 1
	la.scopeVars[currentScope] = append(la.scopeVars[currentScope], name)
	return localName
}

// AllocTemp allocates an ephemeral temporary local slot.
func (la *LocalAllocator) AllocTemp(typ WasmType) string {
	avail := la.pool[typ]
	if len(avail) > 0 {
		localName := avail[len(avail)-1]
		la.pool[typ] = avail[:len(avail)-1]
		return localName
	}

	idx := la.counts[typ]
	la.counts[typ]++
	localName := fmt.Sprintf("$%s_%d", typ, idx)
	la.allLocals = append(la.allLocals, Local{
		Name: localName,
		Type: typ,
	})
	return localName
}

// FreeTemp returns an ephemeral temporary local slot back to the free pool.
func (la *LocalAllocator) FreeTemp(name string, typ WasmType) {
	la.pool[typ] = append(la.pool[typ], name)
}

// Resolve finds the Wasm identifier for a parameter or local variable.
func (la *LocalAllocator) Resolve(name string) (string, bool) {
	if p, ok := la.params[name]; ok {
		return p, true
	}
	if v, ok := la.vars[name]; ok {
		return v, true
	}
	return "", false
}

// Locals returns the complete slice of unique declared locals for the function.
func (la *LocalAllocator) Locals() []Local {
	return la.allLocals
}
