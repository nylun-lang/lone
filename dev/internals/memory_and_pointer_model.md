# Under the Hood: Memory Architecture, Pointer Hierarchy & Value Semantics

This document describes the memory layout, stack frame mechanics, reference and pointer type system, and pass-by-value copying architecture implemented in `pkg2/lone-analyzer` and `pkg2/lone-compiler-wir`.

---

## 1. Hybrid Linear Memory Model

Lone compiles to WebAssembly with a segmented linear memory address space (`env.memory`), divided into two distinct zones:

```
0x00000000 ┌──────────────────────────────────────────────────────────┐
           │ System Scratchpad & WASI iovec ([0x00..0x40))            │
           ├──────────────────────────────────────────────────────────┤
           │ Page 0 Datastack ([0x10000..0x50000))                    │
           │  • Grows downward from $__stack_pointer (init 0x50000)    │
           │  • Stack boundary: $__stack_limit = 0x10000              │
           │  • Holds frame slots for local variables with &x taken   │
           │  • Transient string formatting buffers for println       │
0x00050000 ├──────────────────────────────────────────────────────────┤
           │ Managed Store ([0x50000 .. Dynamic Top))                 │
           │  • Free-list heap managed by $__lone_rt_alloc/free       │
           │  • Heap blocks: 8-byte header [size: u32, is_free: u32]  │
           │  • Holds struct instances, enum payloads, heap strings   │
           └──────────────────────────────────────────────────────────┘
```

### 1.1 Page 0 Datastack Frame Allocation
- **Global Variables**:
  - `$__stack_pointer`: Current stack head (`i32`), initialized to `0x50000`.
  - `$__stack_limit`: Lower stack safety bound (`i32`), set to `0x10000`. If `$__stack_pointer < $__stack_limit`, an `unreachable` trap is raised immediately.
- **Frame Spilling (`AddressTaken`)**:
  - Ordinary local variables live in WebAssembly local registers (`local.get`/`local.set`).
  - When the address-of operator `&x` or `&raw x` is applied to a local variable, `pkg2/lone-analyzer` sets `vdec.AddressTaken = true`.
  - The Wasm lowerer (`lowerFunction`) pre-computes frame offsets for all `AddressTaken` variables, aligned to `TypeAlign(typ)` (up to 16 bytes), adjusting `$__stack_pointer` in function prologue and epilogue.

---

## 2. Pointer & Reference Hierarchy

Lone provides a strictly typed, three-tier pointer and reference hierarchy balancing safety, expressiveness, and systems-level memory access:

| Type Syntax | Kind | Nullable | Indexable (`p[i]`) | Bounds-Checked | Representation |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `ref<T>` | Managed Reference | No | No (compiler error) | N/A | `i32` (memory address) |
| `ref<ro T>` | Read-Only Reference| No | No (compiler error) | N/A | `i32` (memory address) |
| `ptr<T>` | Scalar Pointer | Yes | No (`type/invalid-index`) | N/A | `i32` (memory address) |
| `ptr<[N]T>` | Sized Buffer Pointer| Yes| Yes (`0 <= i < N`) | Yes (compile/runtime)| `i32` (base address) |
| `ptr<[]T>` | Raw Buffer Pointer | Yes | Yes (unchecked) | No (manual) | `i32` (base address) |
| `ref<[]T>` (`[]T`)| Fat Slice Header | No | Yes (`0 <= i < len`)| Yes (runtime panic) | 12-byte header |

### 2.1 Scalar vs. Buffer Pointers
- **`ptr<T>` (Scalar)**: Points to a single instance of `T`. Direct indexing `p[i]` is rejected by `pkg2/lone-analyzer` with diagnostic `type/invalid-index`.
- **`ptr<[N]T>` (Sized Buffer)**: Points to a contiguous buffer of exactly `N` elements. Indexing is validated against the static constant bound `N`.
- **`ptr<[]T>` (Unsized Buffer)**: Represents an unmanaged array buffer whose length is tracked out-of-band by the developer. Indexing emits an offset calculation without safety checks:
  $$\text{addr} = p + (i \times \text{sizeof}(T))$$

### 2.2 Fat Reference Equivalence (`ref<[]T> ≡ []T`)
In Lone, a reference to a dynamic slice does not allocate a reference-to-a-header pointer; instead, `ref<[]T>` is structurally identical to the slice header itself:
```
┌───────────────────────────────────────┐
│ ref<[]T> / []T Header (12 Bytes)      │
├──────────────┬───────────┬────────────┤
│ ptr: ptr<T>  │ len: i32  │ cap: i32   │
│ (offset 0)   │ (offset 4)│ (offset 8) │
└──────────────┴───────────┴────────────┘
```
- Accessing `slice.len` or `slice[i]` reads directly from the inline header without double-indirection.
- Auto-dereferencing ensures identical access syntax between `[]T` and `ref<[]T>`.

---

## 3. Struct Representation & Field Access

### 3.1 Memory Layout
Structs are laid out contiguously according to declaration order with alignment padding:
```lone
type Point = struct {
    ro id: int,   -- offset 0 (4 bytes)
    x: int,       -- offset 4 (4 bytes)
    y: int,       -- offset 8 (4 bytes)
}                 -- total size: 12 bytes, align: 4
```
At runtime, a struct instance is represented by its base pointer (`i32`).

### 3.2 Auto-Dereferencing (`evalPostfixExpr`)
When accessing struct fields (`p.x`), the compiler automatically dereferences managed references (`*RefType`) and pointers (`*PtrType`):
1. The base type is unwrapped to find the inner `*StructType`.
2. Field validity, visibility (private `_field`), and offset are verified.
3. Lowering emits a load/store instruction with an immediate offset:
   ```wat
   local.get $p
   i32.load offset=4
   ```

### 3.3 Field Writes through References
Assignment through a mutable reference (`r.x = val` where `r: ref<Point>`) is supported:
- **Mutable Reference (`ref<Point>`)**: Permitted.
- **Read-Only Reference (`ref<ro Point>`)**: Rejected with `type/immutable-reference-write`.
- **Read-Only Field (`ro id: int`)**: Rejected with `type/immutable-field-write`.
- **Direct Struct on Immutable Binding**: Rejected with `type/immutable-reassignment` (bindings declared with `let` and function parameters are immutable).

---

## 4. Pass-By-Value Struct Semantics

To eliminate aliasing hazards between by-value parameters and mutable references, Lone enforces caller-side struct copying.

### 4.1 The Aliasing Hazard
Consider a function accepting both a struct value and a reference to the same struct:
```lone
fn mutate_alias(val: Point, r: ref<Point>) int {
    r.x = 999
    return val.x
}

fn main() int {
    var a = Point::{ .x: 1, .y: 2 }
    return mutate_alias(a, &a)
}
```
If `a` were passed as a raw pointer without copying, mutating `r.x` would erroneously overwrite `val.x`, violating pass-by-value semantics.

### 4.2 Caller-Side Cloning Architecture
In `pkg2/lone-compiler-wir/lower.go` (`lowerCallExpr`):
1. For each call argument whose type is `*loneanalyzer.StructType`:
2. If the argument is an lvalue (`TVarRefExpr`, `TMemberAccessExpr`, or dereferenced pointer `*r`):
   - Allocate an ephemeral temporary local slot (`alloc.AllocTemp(TypeI32)`).
   - Allocate a fresh heap block: `$__lone_rt_alloc(st.TotalSize)`.
   - Copy bytes from source to destination: `$__lone_rt_memcpy(copy, src, st.TotalSize)`.
   - Pass the cloned address to the function call.
3. If the argument is an rvalue (e.g. `Point::{ ... }` or another function call returning a struct), it is already a fresh allocation and does not need redundant copying.

Inside the function:
- The parameter binding is immutable (`sym.IsMutable = false`). Direct assignment `val.x = 10` is rejected by the analyzer.
- Any mutations via `r.x = 999` affect the original struct block, leaving `val` completely isolated.
