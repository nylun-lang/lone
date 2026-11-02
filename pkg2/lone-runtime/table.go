package loneruntime

import (
	"strings"
	"sync"
)

// Table manages the registry of built-in intrinsic functions and their inlined Wasm representations.
type Table struct {
	mu         sync.RWMutex
	intrinsics map[string]map[Target]IntrinsicSpec
}

// NewDefaultTable constructs a Table pre-populated with standard Lone intrinsics.
func NewDefaultTable() *Table {
	t := &Table{
		intrinsics: make(map[string]map[Target]IntrinsicSpec),
	}

	// Register #print for WASI target (uses fd_write with inlined iovec packing)
	t.Register(IntrinsicSpec{
		Name:       "#print",
		Target:     TargetWasi,
		Params:     []string{"i32", "i32"},
		ReturnType: "void",
		FunctionName: "$__lone_rt_print",
		Imports: []WasmImportSpec{
			{
				Module: "wasi_snapshot_preview1",
				Name:   "fd_write",
				Alias:  "$__wasi_fd_write",
				Params: []string{"i32", "i32", "i32", "i32"},
				Result: "i32",
			},
		},
		InlinedWatFunc: `  ;; Inlined WASI #print runtime implementation
  (func $__lone_rt_print (param $ptr i32) (param $len i32)
    (i32.store (i32.const 0) (local.get $ptr))
    (i32.store (i32.const 4) (local.get $len))
    (drop (call $__wasi_fd_write (i32.const 1) (i32.const 0) (i32.const 1) (i32.const 8)))
  )`,
	})

	// Register #print for Web target (imported from host 'env.print')
	t.Register(IntrinsicSpec{
		Name:       "#print",
		Target:     TargetWeb,
		Params:     []string{"i32", "i32"},
		ReturnType: "void",
		FunctionName: "$__lone_rt_print",
		Imports: []WasmImportSpec{
			{
				Module: "env",
				Name:   "print",
				Alias:  "$__lone_rt_print",
				Params: []string{"i32", "i32"},
				Result: "",
			},
		},
		InlinedWatFunc: "", // Provided directly by import
	})

	return t
}

// Register adds or updates an intrinsic specification.
func (t *Table) Register(spec IntrinsicSpec) {
	t.mu.Lock()
	defer t.mu.Unlock()

	norm := strings.ToLower(strings.TrimSpace(spec.Name))
	if t.intrinsics[norm] == nil {
		t.intrinsics[norm] = make(map[Target]IntrinsicSpec)
	}
	t.intrinsics[norm][spec.Target] = spec
}

// Lookup finds the appropriate intrinsic specification for a given name and target.
func (t *Table) Lookup(name string, target Target) (*IntrinsicSpec, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	norm := strings.ToLower(strings.TrimSpace(name))
	byTarget, exists := t.intrinsics[norm]
	if !exists {
		return nil, false
	}

	// Exact target match
	if spec, ok := byTarget[target]; ok {
		return &spec, true
	}

	// Fallback to TargetAny
	if spec, ok := byTarget[TargetAny]; ok {
		return &spec, true
	}

	return nil, false
}
