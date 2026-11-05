# RFC-20261105: Reference (`ref<T>`) vs. Pointer (`ptr<T>`) Semantics & Memory Model

- **RFC**: `RFC-20261105-reference_and_pointer_semantics`
- **Title**: Reference (`ref<T>`) vs. Pointer (`ptr<T>`) Semantics & Memory Model
- **Author**: Human & AI Pair
- **Status**: Draft
- **Created**: 2026-11-05
- **Target Version**: v0.6.0
- **Related RFCs**:
  - [`RFC-20261104-interface_data_contracts`](./RFC-20261104-interface_data_contracts.md)
  - [`RFC-20261104-struct_initialization_and_field_modifiers`](./RFC-20261104-struct_initialization_and_field_modifiers.md)
- **Applies to**: `pkg2/lone-parser`, `pkg2/lone-analyzer`, `pkg2/lone-compiler-wir`, `dev/SPEC.md`

---

## 1. Summary

This RFC formalizes the complete semantics, type rules, syntax invariants, and WebAssembly runtime implementation for indirect memory addressing in Lone.

In accordance with **IL1 (Explicit over implicit)** and **IL2 (No implicit built-ins)**, Lone establishes a strict two-tier memory architecture:
1. **`ref<T>` (Managed Reference)**: Safe, guaranteed non-null, borrow-checked reference. Participates in automatic escape promotion to the Managed Store (heap `#alloc`) when outliving stack scopes.
2. **`ptr<T>` (Raw Memory Address)**: Unchecked, nullable (`0` / `none`), zero-overhead byte offset into WebAssembly linear memory (`i32`). Designed for low-level systems code, custom allocators, hardware/WASI buffers, and FFI syscalls.

---

## 2. Motivation

In Lone v0.5.0-beta, the parser and type analyzer represent `ref<T>` and `ptr<T>` as distinct types, and differentiate `&expr`, `&raw expr`, and `*expr`. However, several critical semantic and architectural aspects remained informal or unimplemented in `pkg2`:

1. **Nullability Ambiguity**: Whether pointers and references could be null, and how `none` interacted with WebAssembly memory offset 0.
2. **Buffer Traversal**: How `ptr<T>` could access contiguous buffers without clumsy integer arithmetic.
3. **Backend Register Spilling (`AddressTaken`)**: In WebAssembly MVP, virtual registers (`local.get`, `local.set`) have no memory address. Taking `&x` or `&raw x` on a local variable requires spilling it onto the Page 0 datastack frame, which was implemented in legacy `lone-compiler-mir` but not yet wired into `pkg2/lone-compiler-wir`.
4. **Unified Escape Safety**: Harmonizing `ref<T>` with the automatic heap promotion rules established in Edge Case 10 of the Soundness Audit.

---

## 3. Syntax & Grammar Specification

### 3.1 Operators
Lone provides three primary operators for indirect addressing:

| Syntax | Kind | Precedence | AST Node | Produces |
| :--- | :--- | :---: | :--- | :--- |
| **`&expr`** | Managed Reference | Unary (`PrecUnary`) | `UNARY_OP_REF` | `ref<T>` (or `ref<ro T>`) |
| **`&raw expr`** | Raw Memory Address | Unary (`PrecUnary`) | `UNARY_OP_RAW_REF` | `ptr<T>` (or `ptr<ro T>`) |
| **`*expr`** | Dereference | Unary (`PrecUnary`) | `UNARY_OP_DEREF` | Target value `T` |

### 3.2 Buffer Indexing Syntax (`p[i]`)
To provide intuitive, safe-syntax traversal over contiguous raw memory buffers, `ptr<T>` supports indexing:

$$\text{address} = \mathbf{p} + \mathbf{i} \times \mathbf{sizeof}(T)$$

```lone
fn fill_zeros(buf: ptr<u8>, count: i32) void {
    var i = 0
    while i < count {
        buf[i] = 0   -- Writes to (buf + i * 1)
        i = i + 1
    }
}
```

- Reading `let val = p[i]` loads the element of type `T` at offset `p + i * sizeof(T)`.
- Writing `p[i] = val` stores the value of type `T` at offset `p + i * sizeof(T)`.
- `ref<T>` **strictly forbids** indexing (`p[i]`). References represent single objects, not arrays.

### 3.3 Explicit Pointer Methods
For explicit pointer arithmetic without array indexing, `ptr<T>` provides compiler-recognized built-in methods:
- `p.offset(count: i32) ptr<T>`: Advances the pointer by `count * sizeof(T)` bytes.
- `p.byte_offset(bytes: i32) ptr<T>`: Advances the pointer by `bytes` raw bytes.

---

## 4. Semantic Rules & Invariants

### 4.1 Dimension 1: Nullability & Sentinel Values
WebAssembly linear memory addresses are 32-bit unsigned offsets (`0` to `2^32 - 1`). In Lone's memory layout:
- **Reserved Scratch (0..63)**: Offsets `0..63` are reserved for WASI scratch (`iovec`).
- **Sentinel Rule**: Address `0` is **never** allocated to user objects. It serves as the canonical `NULL` sentinel.

#### The Nullability Rules:
1. **`ref<T>` is strictly NON-NULL**:
   - A `ref<T>` can **never** be null or zero.
   - If an optional reference is needed, developers write `?ref<T>` (which evaluates to `Option::Some(&x)` or `Option::None`).
2. **`ptr<T>` is inherently NULLABLE**:
   - `ptr<T>` can hold address `0`.
   - In comparisons, `none` or `0` tests for null:
     ```lone
     if p == none {
         -- Null pointer handling
     }
     if p != none {
         -- Valid address
     }
     ```

### 4.2 Dimension 2: Immutability & Binding Interaction (`let` vs. `var`)
Taking the address of a variable honors Lone's immutability guarantees:

1. **Managed References (`&x`)**:
   - `let x: i32 = 42` $\implies$ `&x` produces **`ref<ro i32>`**.
   - `var x: i32 = 42` $\implies$ `&x` produces **`ref<i32>`**.
   - Attempting to pass `ref<ro T>` where mutable `ref<T>` is expected fails with compile-time diagnostic `type/call-arg-type`.
   - Attempting to write `*r = 10` on `ref<ro T>` triggers `type/immutable-reference-write`.
2. **Raw Pointers (`&raw x`)**:
   - `let x: i32 = 42` $\implies$ `&raw x` produces **`ptr<ro i32>`**.
   - `var x: i32 = 42` $\implies$ `&raw x` produces **`ptr<i32>`**.
   - Attempting to write `*p = 10` or `p[i] = 10` on `ptr<ro T>` triggers `type/immutable-pointer-write`.

### 4.3 Dimension 3: Dereferencing & Auto-Dereference Invariants

1. **Member Access (`.` operator)**:
   - **Auto-dereference is applied automatically**:
     - If `r` is `ref<Point>`, `r.x` is sugar for `(*r).x`.
     - If `p` is `ptr<Point>`, `p.x` is sugar for `(*p).x`.
   - Eliminates cumbersome `->` or `(*p).x` boilerplate while maintaining exact offset calculation.
2. **Scalar Value Reads & Writes**:
   - Must use explicit unary `*`:
     ```lone
     var num: i32 = 10
     var r: ref<i32> = &num
     
     let v: i32 = *r   -- Explicit dereference read
     *r = 20           -- Explicit dereference write
     ```
3. **Rebinding vs. Mutating**:
   ```lone
   var a: i32 = 10
   var b: i32 = 20
   
   var r: ref<i32> = &a
   r = &b       -- Rebinding: 'r' now points to 'b'
   *r = 99      -- Mutation: 'b' is updated to 99 ('a' remains 10)
   ```

### 4.4 Dimension 4: Type Degradation & Transmutation Matrix

Lone strictly forbids implicit conversions between references and pointers:

| Source Type | Target Type | Syntax | Safety | Evaluation |
| :--- | :--- | :---: | :---: | :--- |
| `ref<T>` | `ptr<T>` | `r as ptr<T>` | **Safe** | Decays to raw Wasm `i32` address |
| `ref<T>` | `ref<ro T>` | Implicit / `r` | **Safe** | Subtyping: drops mutation capability |
| `ptr<T>` | `ptr<ro T>` | Implicit / `p` | **Safe** | Subtyping: drops mutation capability |
| `ptr<T>` | `ptr<U>` | `p as ptr<U>` | **Unsafe** | Pointer bitcast/transmutation |
| `ptr<T>` | `i32` | `p as i32` | **Unsafe** | Exposes raw integer linear memory offset |
| `i32` | `ptr<T>` | `addr as ptr<T>`| **Unsafe** | Treats integer offset as typed pointer |
| `ptr<T>` | `ref<T>` | `#ref_from_ptr(p)` | **Unsafe** | Intrinsic: asserts `p != 0` and produces safe reference |

---

## 5. Unified Memory Placement & Escape Analysis

Lone coordinates indirect memory across its two-tier WebAssembly linear memory layout:

```
+-------------------------------------------------------------+ 0x00000000 (0)
| Page 0: Reserved Scratch (iovec 0..63) & Static Constant Pool|
+-------------------------------------------------------------+ dataEnd
| Page 0: Stack Guard Page                                    | $__stack_limit
|                                                             |
| v  Page 0: Datastack (grows downward)                    v  |
|    - Stack slots for AddressTaken variables                 |
|    - Non-escaping &local references                         |
|    - Caller SRET frames                                     |
+-------------------------------------------------------------+ $__stack_pointer
| Free Linear Memory Region                                   |
+-------------------------------------------------------------+ 0x00010000 (64 KiB)
| Page 1+: Managed Store (Dynamic Heap)                       |
|    - Escaping ref<T> allocations (promoted via #alloc)      |
|    - Free-List Allocator with bi-directional coalescing    |
+-------------------------------------------------------------+ memory.size
```

### 5.1 The Lifecycle of `ref<T>`
1. **Local Non-Escaping References**:
   - Variables whose reference `&x` is only passed down the call tree or used locally reside on the local Page 0 datastack frame.
   - **Cost**: Zero heap allocations, zero GC overhead. Instant stack rewind upon return.
2. **Automatic Escape Promotion**:
   - When the compiler's escape analyzer detects that a reference outlives the function (e.g. `return &x` or storing into a persistent heap-backed struct), the allocation is **automatically promoted** to the Managed Store (heap `#alloc`).
   - The compiler emits deterministic reference drop cleanup (`$__lone_rt_free`), coalescing memory immediately without a stop-the-world garbage collector.

### 5.2 The Lifecycle of `ptr<T>`
1. **Unchecked Raw Address**:
   - `ptr<T>` is an unmanaged address and **never** triggers automatic heap promotion.
2. **Escaping Stack Address Detection**:
   - If static analysis detects `&raw local_var` escaping the declaring function scope, the compiler raises a compile-time error: `borrow/stack-use-after-free`.
   - Raw pointers intended to outlive a stack frame must be explicitly allocated via `#alloc`.

---

## 6. WebAssembly Compilation & Codegen Pipeline

### 6.1 Address-Taken Stack Spilling
In WebAssembly, local variables declared with `local $var` exist only in virtual execution registers and cannot have their linear memory address taken.

#### The Codegen Transformation:
1. **Analysis Phase (`pkg2/lone-analyzer`)**:
   - When evaluating `&x` or `&raw x`, the analyzer marks `sym.AddressTaken = true`.
2. **Function Frame Layout (`pkg2/lone-compiler-wir`)**:
   - At function prologue, the compiler scans all local declarations.
   - For every variable with `AddressTaken == true`:
     $$\text{frameSize} = \sum \text{alignTo}(\text{sizeof}(T), \text{alignof}(T))$$
     $$\text{offset}(x) = \text{slotOffset}$$
   - The prologue adjusts the datastack pointer:
     ```wat
     global.get $__stack_pointer
     i32.const <frameSize>
     i32.sub
     global.set $__stack_pointer
     ```
3. **Address-Of Lowering (`&x` and `&raw x`)**:
   - Emits the linear memory address of the stack slot:
     ```wat
     global.get $__stack_pointer
     i32.const <offset_x>
     i32.add
     ```
4. **Dereference Lowering (`*r` and `*p`)**:
   - Read: `lowerExpr(expr)` followed by `i32.load` (or `i64.load` / `f64.load` depending on `T`).
   - Write (`*r = val`): `lowerExpr(r)` + `lowerExpr(val)` + `i32.store`.
5. **Function Epilogue**:
   - Rewinds `$__stack_pointer` by `<frameSize>`.

---

## 7. Static Analysis & Diagnostics

| Diagnostic Code | Condition | Severity |
| :--- | :--- | :--- |
| `type/invalid-deref` | Attempting `*expr` on non-pointer, non-reference type. | Error |
| `type/immutable-reference-write` | Attempting `*r = val` on `ref<ro T>`. | Error |
| `type/immutable-pointer-write` | Attempting `*p = val` or `p[i] = val` on `ptr<ro T>`. | Error |
| `type/invalid-cast` | Attempting `ptr<T> as ref<T>` or cast between incompatible types. | Error |
| `type/call-arg-type` | Passing `ref<T>` where `ptr<T>` expected (or vice versa). | Error |
| `borrow/stack-use-after-free` | Returning `&raw stack_var` escaping function frame. | Error |

---

## 8. Prior Art & Rationale

- **Rust**: Uses `&T` / `&mut T` for safe borrows and `*const T` / `*mut T` for raw pointers. Lone's `ref<T>` / `ptr<T>` achieves a similar safety boundary, but pairs safe references with automatic Managed Store promotion to eliminate lifetime annotation friction for application developers.
- **Zig**: Uses `*T` for single-item pointers and `[*]T` for multi-item (sliceable) pointers. Lone achieves this by reserving indexing `p[i]` exclusively for `ptr<T>`, keeping `ref<T>` cleanly single-item.
- **Go**: Uses `*T` for garbage-collected references and `unsafe.Pointer` / `uintptr` for raw systems access. Lone's `ref<T>` provides Go-like ergonomic reference safety, while `ptr<T>` gives C/Zig-like bare-metal linear memory control without runtime garbage collection pauses.

---

## 9. Next Steps & Compiler Roadmap

1. **`pkg2/lone-analyzer`**:
   - Mark `AddressTaken = true` on symbols when `&expr` or `&raw expr` is evaluated.
   - Enforce read-only write protection on `ref<ro T>` and `ptr<ro T>`.
   - Implement `p[i]` type checking on `ptr<T>`.
2. **`pkg2/lone-compiler-wir`**:
   - Implement datastack frame allocation and spilling for variables with `AddressTaken == true`.
   - Lower `&x` and `&raw x` to `sp + offset`.
   - Lower `*x` scalar reads to `load` and `*x = v` to `store`.
   - Lower `p[i]` to `p + i * sizeof(T)` load/store.
3. **`test2`**:
   - Add unit tests for `ref` vs `ptr`, `ro` mutation prevention, indexing `p[i]`, and address-taken stack codegen.
