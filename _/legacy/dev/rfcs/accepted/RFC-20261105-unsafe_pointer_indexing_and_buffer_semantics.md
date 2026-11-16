# RFC-20261105: Unsafe Pointer Indexing, Typestate Buffer Separation, and Memory Safety

- **RFC**: `RFC-20261105-unsafe_pointer_indexing_and_buffer_semantics`
- **Title**: Unsafe Pointer Indexing, Typestate Buffer Separation, and Memory Safety
- **Author**: Human & AI Pair
- **Status**: Accepted
- **Created**: 2026-11-05
- **Accepted**: 2026-11-05
- **Target Version**: v0.6.1 / v0.7.0
- **Related RFCs**:
  - [`RFC-20261105-reference_and_pointer_semantics`](../accepted/RFC-20261105-reference_and_pointer_semantics.md)
  - [`RFC-20261104-struct_initialization_and_field_modifiers`](../accepted/RFC-20261104-struct_initialization_and_field_modifiers.md)
  - [`RFC-20261105-grouped_declarations_anonymous_functions_and_tuples`](../accepted/RFC-20261105-grouped_declarations_anonymous_functions_and_tuples.md)
- **Applies to**: `pkg2/lone-parser`, `pkg2/lone-analyzer`, `pkg2/lone-compiler-wir`, `dev/SPEC.md`
- **Exploration Reference**: [`dev/research/exploration/unsafe-ptr-indexing.md`](../../research/exploration/unsafe-ptr-indexing.md)

---

## 1. Summary

This RFC investigates and formalizes the design space for **Unsafe Pointer Indexing (`p[i]`)** in Lone. 

Following the implementation of [`RFC-20261105-reference_and_pointer_semantics`](../accepted/RFC-20261105-reference_and_pointer_semantics.md), Lone strictly distinguishes safe, non-indexable references (`ref<T>`) from raw linear memory pointers (`ptr<T>`). While `ptr<T>` currently permits direct bracket indexing (`p[i]`), research documented in `unsafe-ptr-indexing.md` reveals a critical systems hazard: **The "Innocent Syntax" Trap** within WebAssembly's flat, unsegmented linear memory sandbox.

This proposal introduces a cohesive safety model centered on **Typestate Pointer Separation**:
1. **Single-Element Pointer (`ptr<T>`)**: Points to exactly one instance of `T` (e.g. `let p = &raw x`). Array-style indexing `p[i]` is **statically forbidden** (`type/single-pointer-indexing-rejected`). Only dereference `*p` is allowed.
2. **Sized Buffer Pointer (`ptr<[N]T>`)**: Points to a fixed-size contiguous array of $N$ elements in linear memory (e.g. `let p = &raw scores` where `scores` is `[6]int`). 
   - **Compile-Time Bounds Check**: Constant indices $i \ge N$ are rejected at compile time (`type/index-out-of-bounds`).
   - **Zero-Cost Dynamic Bounds Check**: Variable indices `p[i]` are verified via a single unsigned WebAssembly instruction (`i32.ge_u N`), cleanly trapping before linear memory corruption can occur.
   - **Safe Decay**: `ptr<[N]T>` safely coerces into an unsized buffer `ptr<[]T>`.
3. **Unsized Multi-Element Buffer Pointer (`ptr<[]T>`)**: Explicitly declares an unmanaged, unbounded contiguous buffer in linear memory (e.g. from `#alloc`, framebuffers, or FFI). Indexing `buf[i]` is **permitted** with zero runtime overhead.
4. **Explicit Type Casting**: Transitioning from a single pointer to a buffer pointer requires an explicit cast (`p as ptr<[]T>`), ensuring developers consciously acknowledge unbounded memory traversal.
5. **Safe Slice Promotion (`#slice(p, len)`)**: Provides an ergonomic bridge from unmanaged `ptr<[]T>` to safe, bounds-checked dynamic slices `[]T`.

---

## 2. Motivation & The WebAssembly Sandbox Hazard

### 2.1 The "Innocent Syntax" Trap
In modern programming languages (Go, Python, TypeScript, Rust), the bracket operator `container[i]` conveys an implicit guarantee of **spatial safety**: either the collection knows its length and panics on out-of-bounds access, or the compiler statically proves the access is within bounds.

With raw pointers (`ptr<T>`), indexing is a thin byte offset calculation:
$$\text{Address} = \text{base} + (i \times \operatorname{sizeof}(T))$$

It carries **zero runtime bounds metadata**.

When an ordinary pointer to a single local stack variable allows array indexing:
```lone
var count: int = 10
let p = &raw count
p[1] = 999 -- WHAT DOES THIS WRITE TO?
```
The caller innocently believes they are accessing an array, but in reality, they are silently corrupting adjacent memory.

### 2.2 The WebAssembly Hazard: Silent Corruption vs. Hardware MMU Traps
In native OS architectures (x86_64, ARM64), an out-of-bounds pointer write that crosses page boundaries triggers a hardware `SIGSEGV` or MMU exception, halting the program and pinpointing the bug.

In **WebAssembly Linear Memory**, the execution environment is fundamentally different:
1. **Flat Unsegmented Address Space**: WebAssembly linear memory is a single contiguous byte array. In Lone's memory layout:
   - Page 0 holds globals, data segments, and the downward-growing **datastack frame** (`$__stack_pointer`).
   - Higher pages host dynamic heap allocations (`$__heap_base`, `$__free_list`).
2. **Silent Cascading Corruption**: Writing past the end of `p` in Wasm does not crash the host unless it exceeds the total allocated WebAssembly page count (`memory.size`). Instead, it silently overwrites:
   - Adjacent stack frames (corrupting local variables and return registers of parent callers).
   - Dynamic heap metadata (corrupting free-list block headers, causing subsequent `#alloc` calls to crash or corrupt memory).
   - Global variable tables.
3. **No Hardware Canaries**: Pure Wasm runtimes execute without hardware memory protection rings. An errant `p[i]` write can run for millions of cycles in a silently corrupted state before manifesting as an inexplicable logic failure far from the source.

---

## 3. Prior Art & Comparative Analysis

| Language | Pointer Syntax | Single vs Buffer | Indexing Rule (`p[i]`) | Bounds Safety |
| :--- | :---: | :---: | :--- | :--- |
| **C / C++** | `T*` | Conflated | Allowed directly (`p[i]`) | None (Undefined Behavior; primary source of CVEs) |
| **Rust** | `*const T`, `*mut T` | Conflated | Forbidden via `[]`; requires `*p.add(i)` inside `unsafe` block | Strict unsafe demarcation; standard type is slice `&[T]` |
| **Zig** | `*T` vs `[*]T` vs `*[N]T` | **Separated** | `*T`: **Forbidden**<br/>`[*]T`: **Allowed** (unsized)<br/>`*[N]T`: **Allowed** (bounds checked) | Typestate enforcement: single pointer cannot be indexed without explicit cast |
| **Go** | `*T` | Conflated | Forbidden on `*T`; requires `unsafe.Pointer` + `uintptr` arithmetic | High-friction escape hatch |
| **Lone (Status Quo v0.6.0)** | `ptr<T>` | Conflated | Allowed directly on all `ptr<T>` | Low friction, but vulnerable to Innocent Syntax trap |
| **Lone (Proposed v0.6.1)** | `ptr<T>`<br/>`ptr<[N]T>`<br/>`ptr<[]T>` | **Separated** | `ptr<T>`: **Forbidden** (`type/single-pointer-indexing-rejected`)<br/>`ptr<[N]T>`: **Checked** (static $i < N$, 1-inst dynamic)<br/>`ptr<[]T>`: **Allowed** (unbounded raw) | Invariant IL1: Single reference cannot masquerade as buffer |

Zig's separation of single-item pointer `*T`, sized array pointer `*[N]T`, and many-item pointer `[*]T` has proven exceptionally successful in production systems programming. It eliminates over 90% of unintended buffer-overflow bugs at compile time without adding runtime overhead or ugly syntactic friction.

---

## 4. Proposed Architectural Design

### 4.1 The 3-Tier Typestate Pointer Hierarchy

Lone refines its pointer type grammar into three distinct tiers based on spatial cardinality:

1. **Single-Item Pointer (`ptr<T>`)**:
   - Semantics: Points to a single, isolated value of type `T` in linear memory.
   - Operations Allowed:
     - Dereference read: `let val = *p`
     - Dereference write: `*p = val` (requires `ptr<T>`, rejected on `ptr<ro T>`)
   - Operations Forbidden:
     - Indexing `p[i]`: **Compile-time Error** (`type/single-pointer-indexing-rejected`).
     - Invariant: A pointer obtained via `&raw x` on a non-array variable is always `ptr<T>`. It cannot be indexed as an array.

2. **Sized Array Pointer (`ptr<[N]T>`)**:
   - Semantics: Points to a contiguous buffer of exactly $N$ elements in linear memory.
   - Origin:
     - Taking raw address of a fixed-size array: `let p = &raw my_array` where `my_array` is `[N]T`.
     - Explicit sized cast: `raw_mem as ptr<[6]int>`.
   - Bounds Safety Guarantees:
     - **Compile-Time Constant Checking**: If $i$ is a constant and $i \ge N$ (or $i < 0$), the compiler raises a static error: `type/index-out-of-bounds`.
     - **Single-Instruction Dynamic Checking**: If $i$ is a runtime variable, the compiler emits an unsigned comparison against immediate $N$ (`i32.ge_u N`), trapping before memory corruption occurs.
     - **Safe Decay / Coercion**: A `ptr<[N]T>` can safely and implicitly coerce to an unsized buffer `ptr<[]T>`.

3. **Unsized Buffer Pointer (`ptr<[]T>`)**:
   - Semantics: Points to an unmanaged, contiguous sequence of values of type `T` with unknown runtime length.
   - Operations Allowed:
     - Index read: `let elem = buf[i]`
     - Index write: `buf[i] = val` (requires `ptr<[]T>`, rejected on `ptr<ro []T>`)
     - Dereference: `*buf` accesses index 0 (`buf[0]`)
   - Origin:
     - Memory allocators: `#alloc(size) as ptr<[]u8>`
     - Fixed buffer addresses: `0x10000 as ptr<[]u32>`
     - Explicit pointer cast: `single_ptr as ptr<[]T>`
     - Coercion from sized pointer: `sized_ptr as ptr<[]T>`

```lone
-- 1. Single pointer: indexing forbidden!
var x: int = 42
let p: ptr<int> = &raw x

*p = 100    -- OK: dereference store updates x
-- p[0] = 5 -- COMPILE ERROR: cannot index single-item pointer 'ptr<int>'

-- 2. Sized array pointer: bounds checked!
var scores: [6]int = [10, 20, 30, 40, 50, 60]
let sized_p: ptr<[6]int> = &raw scores

sized_p[0] = 99 -- OK: within [0..5]
sized_p[5] = 99 -- OK: within [0..5]
-- sized_p[6] = 99 -- COMPILE ERROR: index 6 is out of bounds for 'ptr<[6]int>' of size 6

-- Safe decay to unsized buffer:
let buf_view: ptr<[]int> = sized_p -- OK: [6] safely coerces to []

-- 3. Unsized buffer pointer: unmanaged raw access!
let raw_mem = #alloc(1024) as ptr<[]int>
raw_mem[0] = 10
raw_mem[1] = 20
raw_mem[2] = 30
```

---

### 4.2 Alternative Syntax Explored: The Call-Site Marker (`p.[i]`)

The research exploration also considered whether indexing unmanaged buffers should look syntactically different from safe slice indexing:

- **Option A (Typestate Only, Standard `buf[i]`)**:
  - `buf[i]` is used for both `ptr<[]T>` and `[]T`.
  - Type signature makes the danger explicit (`ptr<[]T>` vs `[]T`).
  - Maximizes familiarity and cleaner loops in low-level kernels.

- **Option B (Dotted Indexing `p.[i]`)**:
  - Safe slice / array: `arr[i]`
  - Unmanaged buffer: `buf.[i]`
  - Pro: Every single read or write visually screams that it is raw memory.
  - Con: Non-standard punctuation, visual clutter in math-heavy algorithms (SIMD, graphics, crypto).

- **Option C (Hash-Prefix `buf#[i]`)**:
  - Follows Lone's `#` intrinsic convention (`#print`, `#alloc`).
  - Pro: Clear indicator of unsafe compiler intrinsics.
  - Con: Highly unusual syntax for an indexing operator.

#### Recommendation:
Adopt **Option A (Typestate Separation `ptr<[]T>`)** as the primary language feature, with the compiler strictly rejecting indexing on `ptr<T>`. Because the type signature `ptr<[]T>` already explicitly documents that the value is an unmanaged buffer, standard brackets `buf[i]` preserve clean ergonomics.

---

### 4.3 Safe Slice Promotion (`#slice`)

To encourage transitioning from raw unmanaged memory to safe, bounds-checked Lone slices, the compiler provides the `#slice` intrinsic:

```lone
-- Promotes a raw buffer pointer and known length into a safe slice:
let raw_buf: ptr<[]u8> = #alloc(256) as ptr<[]u8>
let safe_slice: []u8 = #slice(raw_buf, 256)

-- safe_slice[i] is bounds-checked:
safe_slice[0] = 1 -- OK
-- safe_slice[300] = 2 -- Panics at runtime with bounds check!
```

---

### 4.4 Safe Unsized Views: `ref<[]T>` ≡ `[]T` (Fat References)

**Rule: a reference to an unsized type is fat; a raw pointer is always thin.**

- `ref<T>` and `ref<[N]T>` have sizes known at compile time, so they are **thin** (1 × `i32`).
- `ref<[]T>` has no length in its type, so the length travels with it: `{ ptr: i32, len: i32 }`. It is the slice header itself, not a pointer to a header.
- `[]T` is sugar for `ref<[]T>`, and `[]ro T` / `ref<ro []T>` is the read-only slice.
- `ptr<[]T>` stays thin and length-less (like Zig's `[*]T`). It is the deliberate escape hatch where the programmer owns the length.

| | Sized (length in type) | Unsized |
| :--- | :--- | :--- |
| **Safe (`ref`)** | `ref<[N]T>`: checked, thin | `ref<[]T>` ≡ `[]T`: checked against `len`, fat |
| **Unsafe (`ptr`)** | `ptr<[N]T>`: checked, thin | `ptr<[]T>`: **unchecked**, thin |

Consequences:
1. `ref<T>` indexing stays rejected (`type/invalid-index`). `ref<[N]T>` and `ref<[]T>` become indexable with bounds checks. This is a small **amendment to `RFC-20261105-reference_and_pointer_semantics`**.
2. **Decay** `ref<[N]T>` → `ref<[]T>` is implicit and attaches `len = N`. The reverse requires a runtime length check and an explicit cast.
3. `#slice(p: ptr<[]T>, len)` is the only place an unverified length enters the safe world. That is where the trust boundary lies.
4. Slices of stack arrays that escape their frame follow the existing automatic escape-promotion rules for `ref`.

---

### 4.5 Parameter Passing Semantics

**Definitions.**
- **By value**: the callee receives its own copy of the argument's bytes. Mutating the parameter never affects the caller's variable.
- **By reference**: the callee receives an address. Writes through it are visible to the caller.

**Lone rule: every argument is passed by value. Sharing is always explicit, by passing a `ref` or `ptr`.**

```lone
fn a(x: int)          -- 4 bytes copied; caller's variable untouched
fn b(p: Point)        -- semantically a copy (see 4.5.1 and 4.5.3)
fn c(p: ref<Point>)   -- address copied; writes visible to caller (see 4.5.2)
fn d(xs: []int)       -- 8-byte header {ptr,len} copied; ELEMENTS are shared
```

`d` looks like a value but behaves like a reference to its elements, because the copied header contains a pointer. This matches Go slices, Zig `[]T` and Rust `&mut [T]`. `xs[i] = v` mutates the caller's array unless the parameter is `[]ro T`.

```lone
fn sum(xs: []int) int { ... }

var arr: [6]int = [1, 2, 3, 4, 5, 6]
sum(&arr)   -- &arr is a thin ref<[6]int>; the compiler attaches len = 6 to form the slice header
```

**Prior art.**

| Language | Default | Notes |
| :--- | :--- | :--- |
| C | By value | Arrays decay to a pointer; structs copied whole |
| Go | By value | Slices, maps and channels are small headers copied by value over shared data |
| Rust | Move/copy by value | `&T`/`&mut T` explicit; `&[T]` is a fat reference |
| Zig | By value | `[]T` is a `{ptr,len}` struct copied by value |
| Python/JS | By object reference | The reference is copied; the object is shared |

#### 4.5.1 Current implementation status (struct parameters)

Findings from the `pkg2` compiler as of v0.6.0:

1. **Lowering**: `toWasmType` maps every non-primitive type (structs, tuples, enums) to a single `i32` **address**. A struct argument is passed as an address and **no copy is made at the call site**.
2. **Parameters are immutable bindings**: mutating a by-value parameter, e.g. `p.x = 99` inside `fn bump(p: Point)`, is rejected by the analyzer (`type/immutable-reassignment`). Even field writes through a `ref<Point>` parameter (`r.x = 5`) are currently rejected for the same reason.
3. **Consequence**: the "callee mutates its copy" hazard cannot currently be written, so by-value semantics are preserved *by construction* rather than by copying. The remaining risk is aliasing: if a callee receives both `p: Point` and a `ref` to the same struct and can write through the `ref`, the by-value parameter would observe the write. A probe using `*r = Point::{...}` returned the old value, but that only replaces the slot's handle and does not mutate the shared memory, so it does **not** prove copy semantics.

#### 4.5.2 Decision: field writes through `ref<T>` are permitted

A `ref<T>` that cannot mutate its target is of limited use, so writes through references are part of the language:

```lone
fn move_right(p: ref<Point>) void {
    p.x = p.x + 1        -- field write through ref: allowed
    *p = Point::{ .x: 0 } -- whole-value store: allowed
}

fn read_only(p: ref<ro Point>) void {
    -- p.x = 1           -- ERROR: type/immutable-field-write (target is ro)
}
```

Rules:
1. Writing `r.f = v` or `*r = v` requires `r: ref<T>` (not `ref<ro T>`), and `T`'s field `f` must not be declared `ro`.
2. Taking `&x` yields `ref<T>` only if `x` is a `var`; `&x` on a `let` yields `ref<ro T>` (unchanged from the accepted reference RFC).
3. A by-value parameter (`p: Point`) remains an immutable binding: `p.x = 1` is still rejected. Only `ref` parameters may mutate.

#### 4.5.3 Consequence: by-value aggregate arguments must be copied

Once writes through `ref` exist, passing an aggregate as an address with no copy becomes observable:

```lone
fn f(p: Point, r: ref<Point>) int {
    r.x = 5
    return p.x      -- must return the ORIGINAL x, because p is a by-value copy
}
var a = Point::{ .x: 1 }
f(a, &a)            -- required result: 1
```

Required lowering:
1. **Lvalue argument** (a variable, field or dereference): the caller copies the aggregate into a datastack temporary and passes that temporary's address. This is the same mechanism as the SRET convention used for returns.
2. **Rvalue argument** (a fresh temporary such as `Point::(1)` or a call result): no copy is needed, because no other name can reach it. It is moved.
3. **Copy elision** (optimization, not semantics): the copy may be skipped when the callee has no `ref`/`ptr` parameter and no captured reference that could alias the argument, since then nothing can observe the difference.
4. Primitives, `ref`, `ptr` and slice headers are already value-sized and need no special handling.

Tests required before acceptance:
1. The `f(a, &a)` aliasing program above returns `1` under wasmtime.
2. `move_right(&a)` mutates `a`.
3. `p.x = 1` on a by-value parameter and on a `ref<ro Point>` is rejected with `type/immutable-field-write`.
4. Rvalue arguments emit no copy (assert on emitted WAT).

---

## 5. Formal Grammar & Type System Rules

### 5.1 Type Grammar
```ebnf
PointerType     = "ptr" "<" [ "ro" ] ( FixedArrayType | ArrayType | TypeRef ) ">" ;
FixedArrayType  = "[" IntLiteral "]" TypeRef ;
ArrayType       = "[" "]" TypeRef ;
```

- `ptr<T>`: Single-element raw pointer.
- `ptr<ro T>`: Single-element read-only raw pointer.
- `ptr<[N]T>`: Sized raw array pointer with compile-time known length $N$.
- `ptr<ro [N]T>`: Sized read-only raw array pointer.
- `ptr<[]T>`: Multi-element unmanaged raw buffer pointer.
- `ptr<ro []T>`: Multi-element read-only raw buffer pointer.

### 5.2 Subtyping & Assignability Rules
1. **Safe Decay**: `ptr<[N]T>` is implicitly assignable to `ptr<[]T>`. A known-size buffer safely decays into an unsized buffer view.
2. **Reverse Restriction**: `ptr<[]T>` is **not** implicitly assignable to `ptr<[N]T>`. Requires explicit cast: `buf as ptr<[N]T>`.
3. **Single Pointer Isolation**: `ptr<T>` is **not** implicitly assignable to either `ptr<[N]T>` or `ptr<[]T>`. Requires explicit cast: `p as ptr<[]T>`.
4. **Mutability Covariance**: `ptr<T>` assigns to `ptr<ro T>`, `ptr<[N]T>` assigns to `ptr<ro [N]T>`, and `ptr<[]T>` assigns to `ptr<ro []T>`. Reversing mutability requires an explicit cast.
5. **References (`ref<T>`)**: Cannot be converted directly to any raw pointer tier without explicit `&raw`.

---

## 6. Static Analysis & Diagnostics

| Diagnostic Code | Trigger Condition | Severity | Example Message |
| :--- | :--- | :---: | :--- |
| `type/single-pointer-indexing-rejected` | Attempting to index `p[i]` where `p` is `ptr<T>`. | **Error** | `cannot index single-item pointer "ptr<int>"; use "*p" for dereference or cast to buffer pointer "ptr<[]int>" for array indexing` |
| `type/index-out-of-bounds` | Constant index $i \ge N$ or $i < 0$ on `ptr<[N]T>`. | **Error** | `index 6 is out of bounds for array pointer "ptr<[6]int>" of size 6` |
| `type/immutable-buffer-write` | Writing to `buf[i] = val` on `ptr<ro [N]T>` or `ptr<ro []T>`. | **Error** | `cannot write to read-only buffer through "ptr<ro []int>"` |
| `type/invalid-index-type` | Index `i` in `buf[i]` is not an integer (`i32`, `u32`, `int`). | **Error** | `buffer index must be an integer, got "str"` |

---

## 7. WebAssembly Lowering (`pkg2/lone-compiler-wir`)

In WebAssembly WIR lowering:
1. `ptr<T>`, `ptr<[N]T>`, and `ptr<[]T>` all lower to `WasmType.TypeI32` (linear memory byte address).
2. For constant index on `ptr<[N]T>` (e.g. `p[2]`):
   - Offset is statically folded at compile-time: `base + (2 * sizeof(T))`.
   - Zero runtime bounds check emitted (compile-time verified).
3. For dynamic index on `ptr<[N]T>` (e.g. `p[i]`):
   - Emits a **single unsigned comparison** instruction (`i32.ge_u N`) that simultaneously validates $i \ge 0$ and $i < N$:
     ```wat
     local.get $index
     i32.const N
     i32.ge_u
     if
       unreachable ;; or call $__lone_rt_panic_oob
     end
     local.get $buf
     local.get $index
     i32.const sizeof(T)
     i32.mul
     i32.add
     i32.load ;; or i32.store
     ```
4. For dynamic index on `ptr<[]T>` (e.g. `buf[i]`):
   - Emits direct linear memory address calculation without bounds check: `base + index * sizeof(T)`.
   - Zero runtime overhead for systems performance.

---

## 8. Migration & Impact on Existing Codebase

1. **Greenfield Code (`pkg2`)**:
   - `&raw x` continues to return `ptr<T>`.
   - Any test or function performing buffer traversal changes type annotation from `ptr<T>` to `ptr<[]T>`.
   - Allocator calls returning memory for arrays change from `ptr<u8>` to `ptr<[]u8>`.
2. **Backwards Compatibility**:
   - Code dereferencing single pointers (`*p`, `*r`) is 100% unaffected.
   - Code indexing `p[i]` will be guided by the compiler diagnostic to either dereference `*p` or annotate as `ptr<[]T>`.

---

## 9. Next Steps & Implementation Plan

1. **Parser (`pkg2/lone-parser`)**: Support `ptr<[N]T>`, `ptr<[]T>`, and their `ro` variants in `parseTypeRef`.
2. **Analyzer (`pkg2/lone-analyzer`)**:
   - Refine `PtrType`: add `IsBuffer bool` and `FixedSize int` (-1 for unsized buffer, $N > 0$ for sized array, 0 for single item).
   - In `evalPostfixExpr` for `SuffixIndex`:
     - If base is single-item pointer `ptr<T>`: emit `type/single-pointer-indexing-rejected`.
     - If base is sized pointer `ptr<[N]T>` and index is a constant $\ge N$ (or $< 0$): emit `type/index-out-of-bounds`.
     - If base is `ptr<[]T>`: allow unbounded raw indexing.
   - Subtyping: Implement safe decay coercion from `ptr<[N]T>` to `ptr<[]T>`.
3. **Lowering (`pkg2/lone-compiler-wir`)**:
   - For `ptr<[N]T>` with dynamic index: emit single unsigned bounds check `i32.ge_u N` with trap/panic.
   - For `ptr<[]T>`: emit direct zero-cost address offset.
4. **Diagnostics & Tests**:
   - Test rejection of `p[i]` on `ptr<T>`.
   - Test compile-time rejection of constant `p[N]` on `ptr<[N]T>`.
   - Test execution of dynamic `p[i]` on `ptr<[N]T>` with in-bounds success and out-of-bounds trap under wasmtime.
   - Test raw buffer indexing on `ptr<[]T>`.
