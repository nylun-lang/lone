# Under the Hood: WebAssembly IR (WIR) & Code Generation Architecture

This document details the code generation engine in `pkg2/lone-compiler-wir`, explaining how Lone's typed AST is transformed into structured WebAssembly (WAT/Wasm).

---

## 1. Overview & Pipeline

The WIR compiler lowers typed AST nodes (`pkg2/lone-analyzer`) directly into a structured WebAssembly AST without requiring third-party assembler dependencies:

```
┌────────────────────────┐
│ Typed AST (loneanalyzer)│
└───────────┬────────────┘
            │
            ▼
┌────────────────────────┐
│ Lowerer (lower.go)     │ ◄─── Inlined Runtime Library
│  • Frame Slot Spiller  │      ($__lone_rt_alloc, memcpy, strlen, print)
│  • Local Allocator     │
│  • Structured CFG      │
└───────────┬────────────┘
            │
            ▼
┌────────────────────────┐
│ WasmModule AST (wir.go) │
│  • WasmFunction        │
│  • Structured WasmInst │
└───────────┬────────────┘
            │
            ▼
┌────────────────────────┐
│ Text Emitter (wat.go)  │ ──► .wat output (wasmtime / browser)
└────────────────────────┘
```

---

## 2. WebAssembly AST Model (`wir.go`)

Rather than emitting raw strings, `pkg2/lone-compiler-wir` constructs a strongly typed WebAssembly instruction tree:

### 2.1 Instruction Hierarchy (`WasmInst`)
- **Control Flow**: `InstBlock`, `InstLoop`, `InstIf`, `InstBranch`, `InstBranchIf`, `InstReturn`, `InstUnreachable`.
- **Variable Access**: `InstLocalGet`, `InstLocalSet`, `InstLocalTee`, `InstGlobalGet`, `InstGlobalSet`.
- **Constants**: `InstConstI32`, `InstConstI64`, `InstConstF32`, `InstConstF64`.
- **Memory Operations**: `InstLoad`, `InstStore`, `InstMemorySize`, `InstMemoryGrow`.
- **Operations**: `InstBinaryOp`, `InstUnaryOp`, `InstCastOp`, `InstDrop`.
- **Calls**: `InstCall`, `InstCallIndirect`.

---

## 3. Local Variable Pooling & Register Allocation (`allocator.go`)

To avoid virtual register explosion in large functions, `LocalAllocator` manages typed local registers using a free-pool coalescing algorithm:

### 3.1 Allocation Strategy
- **Parameters**: Mapped to `$p_<name>`.
- **Named Local Variables (`AllocVar`)**:
  - Bound to a scoped Wasm local name (e.g. `$i32_0`).
  - When entering an inner block (`EnterScope`), new local variable names are pushed onto the scope stack.
  - When exiting the block (`ExitScope`), variable bindings are removed and returned to the typed pool (`pool[typ]`) for immediate reuse by subsequent sibling blocks.
- **Ephemeral Temporaries (`AllocTemp` / `FreeTemp`)**:
  - Reclaims intermediate evaluation registers.
  - Recycled across statement evaluations to minimize function local declarations.

---

## 4. Structured Control Flow Lowering

WebAssembly does not have arbitrary gotos; all control flow must be structured into nested `block`, `loop`, and `if` constructs:

### 4.1 While Loops
`lowerWhileStmt` lowers `while cond { body }` into:
```wat
block $break_loop
  loop $continue_loop
    ;; 1. Evaluate condition
    <cond_instructions>
    i32.eqz
    br_if $break_loop  ;; Exit loop if condition is false

    ;; 2. Execute body
    <body_instructions>

    ;; 3. Repeat loop
    br $continue_loop
  end
end
```

### 4.2 Labeled Blocks & Breaks
Labeled blocks `label: { ... break label: value }` map to named WebAssembly blocks:
- The block is tagged with result type: `(block $label (result i32) ...)`.
- Break statements evaluate `value` and branch to the label:
  ```wat
  <value_instructions>
  br $label
  ```

---

## 5. Inlined WebAssembly Runtime

The compiler embeds a zero-dependency runtime library directly into compiled modules:

1. **`$__lone_rt_alloc(size: i32) -> i32`**:
   - Free-list memory allocator managing the Managed Store (`>= 0x50000`).
   - Scans free blocks, splits oversized chunks, and issues `memory.grow` when capacity is exhausted.
2. **`$__lone_rt_free(ptr: i32)`**:
   - Marks memory blocks as free and coalesces adjacent free blocks to prevent fragmentation.
3. **`$__lone_rt_memcpy(dst: i32, src: i32, len: i32)`**:
   - High-throughput byte copying used for struct pass-by-value copies, dereference assignments (`*r = val`), and string construction.
4. **`$__lone_rt_strlen(ptr: i32) -> i32`**:
   - Calculates null-terminated string byte length.
5. **`$__lone_rt_print(ptr: i32, len: i32)`**:
   - Populates a WASI `iovec` at scratchpad `0x00` and issues `fd_write` to `stdout` (`fd = 1`).

---

## 6. String Formatting & Dual-Path Emission

Formatted string interpolation `f"Hello ${name}"` is lowered according to statement vs. value context:

1. **Output Statement Context (`println(f"...")`)**:
   - Targets the Page 0 Datastack (`$__stack_pointer`).
   - Formats literals and integer/string expressions directly into a transient 256-byte stack buffer.
   - Emits a single atomic WASI `fd_write` syscall, guaranteeing thread safety and eliminating heap allocation.
2. **Value Context (`let s = f"..."`)**:
   - Computes total static + dynamic length.
   - Allocates contiguous memory via `$__lone_rt_alloc`.
   - Formats expressions into the heap buffer and returns the resulting string pointer.
