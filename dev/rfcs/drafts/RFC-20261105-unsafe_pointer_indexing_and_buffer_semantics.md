# RFC-20261105: Unsafe Pointer Indexing, Typestate Buffer Separation, and Memory Safety

- **RFC**: `RFC-20261105-unsafe_pointer_indexing_and_buffer_semantics`
- **Title**: Unsafe Pointer Indexing, Typestate Buffer Separation, and Memory Safety
- **Author**: Human & AI Pair
- **Status**: Draft
- **Created**: 2026-11-05
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
2. **Raw Multi-Element Buffer Pointer (`ptr<[]T>`)**: Explicitly declares an unmanaged, unbounded contiguous buffer in linear memory (e.g. from `#alloc`, framebuffers, or FFI). Indexing `buf[i]` is **permitted** with zero runtime overhead.
3. **Explicit Type Casting**: Transitioning from a single pointer to a buffer pointer requires an explicit cast (`p as ptr<[]T>`), ensuring developers consciously acknowledge unbounded memory traversal.
4. **Safe Slice Promotion (`#slice(p, len)`)**: Provides an ergonomic bridge from unmanaged `ptr<[]T>` to safe, bounds-checked dynamic slices `[]T`.

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
| **Zig** | `*T` vs `[*]T` | **Separated** | `*T`: **Forbidden**<br/>`[*]T`: **Allowed** | Typestate enforcement: single pointer cannot be indexed without explicit cast |
| **Go** | `*T` | Conflated | Forbidden on `*T`; requires `unsafe.Pointer` + `uintptr` arithmetic | High-friction escape hatch |
| **Lone (Status Quo v0.6.0)** | `ptr<T>` | Conflated | Allowed directly on all `ptr<T>` | Low friction, but vulnerable to Innocent Syntax trap |
| **Lone (Proposed v0.6.1)** | `ptr<T>` vs `ptr<[]T>` | **Separated** | `ptr<T>`: **Forbidden** (`type/single-pointer-indexing-rejected`)<br/>`ptr<[]T>`: **Allowed** | Invariant IL1: Single reference cannot masquerade as buffer |

Zig's separation of single-item pointer `*T` from many-item pointer `[*]T` has proven exceptionally successful in production systems programming. It eliminates over 90% of unintended buffer-overflow bugs at compile time without adding runtime overhead or ugly syntactic friction.

---

## 4. Proposed Architectural Design

### 4.1 Typestate Separation: `ptr<T>` vs `ptr<[]T>`

Lone refines its pointer type grammar to distinguish pointer cardinality:

1. **Single-Item Pointer (`ptr<T>`)**:
   - Semantics: Points to a single, isolated value of type `T` in linear memory.
   - Operations Allowed:
     - Dereference read: `let val = *p`
     - Dereference write: `*p = val` (requires `ptr<T>`, rejected on `ptr<ro T>`)
   - Operations Forbidden:
     - Indexing `p[i]`: **Compile-time Error** (`type/single-pointer-indexing-rejected`).
     - Invariant: A pointer obtained via `&raw x` on a variable is always `ptr<T>`. It cannot be indexed as an array.

2. **Buffer Pointer (`ptr<[]T>`)**:
   - Semantics: Points to an unmanaged, contiguous sequence of values of type `T` with unknown runtime length.
   - Operations Allowed:
     - Index read: `let elem = buf[i]`
     - Index write: `buf[i] = val` (requires `ptr<[]T>`, rejected on `ptr<ro []T>`)
     - Dereference: `*buf` accesses index 0 (`buf[0]`)
   - Origin:
     - Memory allocators: `#alloc(size) as ptr<[]u8>`
     - Fixed buffer addresses: `0x10000 as ptr<[]u32>`
     - Explicit pointer cast: `single_ptr as ptr<[]T>`

```lone
-- 1. Single pointer: indexing forbidden!
var x: int = 42
let p: ptr<int> = &raw x

*p = 100    -- OK: dereference store updates x
-- p[0] = 5 -- COMPILE ERROR: cannot index single-item pointer 'ptr<int>'

-- 2. Buffer pointer: indexing allowed!
let raw_mem = #alloc(1024) as ptr<[]int>
raw_mem[0] = 10
raw_mem[1] = 20
raw_mem[2] = 30

-- 3. Explicit cast when a single pointer genuinely starts an array:
let buf_view = p as ptr<[]int>
buf_view[0] = 42 -- Explicit and intentional!
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

## 5. Formal Grammar & Type System Rules

### 5.1 Type Grammar
```ebnf
PointerType   = "ptr" "<" [ "ro" ] ( ArrayType | TypeRef ) ">" ;
ArrayType     = "[" "]" TypeRef ;
```

- `ptr<T>`: Single-element raw pointer.
- `ptr<ro T>`: Single-element read-only raw pointer.
- `ptr<[]T>`: Multi-element raw buffer pointer.
- `ptr<ro []T>`: Multi-element read-only raw buffer pointer.

### 5.2 Subtyping & Assignability Rules
1. `ptr<T>` is **not** assignable to `ptr<[]T>` implicitly. Requires explicit cast: `p as ptr<[]T>`.
2. `ptr<[]T>` is **not** assignable to `ptr<T>` implicitly. Requires explicit cast or dereference: `*p` or `p as ptr<T>`.
3. Mutability covariance: `ptr<T>` is assignable to `ptr<ro T>`, and `ptr<[]T>` is assignable to `ptr<ro []T>`. The reverse requires an explicit cast.
4. References (`ref<T>`) cannot be converted directly to buffer pointers without `&raw` and a cast.

---

## 6. Static Analysis & Diagnostics

| Diagnostic Code | Trigger Condition | Severity | Example Message |
| :--- | :--- | :---: | :--- |
| `type/single-pointer-indexing-rejected` | Attempting to index `p[i]` where `p` is `ptr<T>`. | **Error** | `cannot index single-item pointer "ptr<int>"; use "*p" for dereference or cast to buffer pointer "ptr<[]int>" for array indexing` |
| `type/immutable-buffer-write` | Writing to `buf[i] = val` where `buf` is `ptr<ro []T>`. | **Error** | `cannot write to read-only buffer through "ptr<ro []int>"` |
| `type/invalid-index-type` | Index `i` in `buf[i]` is not an integer (`i32`, `u32`, `int`). | **Error** | `buffer index must be an integer, got "str"` |

---

## 7. WebAssembly Lowering (`pkg2/lone-compiler-wir`)

In WebAssembly WIR lowering:
1. Both `ptr<T>` and `ptr<[]T>` lower to `WasmType.TypeI32` (linear memory byte address).
2. For `p[i]` on `ptr<[]T>`:
   ```wat
   ;; base + index * sizeof(T)
   local.get $buf
   local.get $index
   i32.const sizeof(T)
   i32.mul
   i32.add
   ;; load or store
   i32.load (or i32.store)
   ```
3. Zero runtime cost: The typestate check is 100% compile-time. Emitted WebAssembly instructions are identical to direct pointer arithmetic.

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

1. **Parser (`pkg2/lone-parser`)**: Support `ptr<[]T>` and `ptr<ro []T>` in `parseTypeRef`.
2. **Analyzer (`pkg2/lone-analyzer`)**:
   - Add `IsBuffer bool` (or `BufferElem Type`) to `PtrType`.
   - Update `evalPostfixExpr` for `SuffixIndex`:
     - If base is `*PtrType` with `!IsBuffer`: emit `type/single-pointer-indexing-rejected`.
     - If base is `*PtrType` with `IsBuffer`: allow indexing.
3. **Diagnostics & Tests**:
   - Unit tests verifying rejection of `p[i]` on `ptr<T>`.
   - Unit tests verifying acceptance of `buf[i]` on `ptr<[]T>`.
   - Test read-only enforcement on `ptr<ro []T>`.
