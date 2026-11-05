# Research Exploration: Reference (`ref<T>`) vs. Pointer (`ptr<T>`) Semantics & Memory Model

**Date**: 2026-11-05  
**Scope**: Language Design, Memory Architecture, Syntax & Compiler Codegen  
**Status**: Active Exploration  
**Location**: `dev/research/exploration/ref_vs_ptr_semantics_and_memory_model.md`  
**Related**:
- `dev/SPEC.md` (Sections 4.3, 7.3, 8.1, 10.1)
- `dev/rfcs/drafts/RFC-20261104-interface_data_contracts.md`
- `dev/rfcs/drafts/RFC-20261104-struct_initialization_and_field_modifiers.md`
- `dev/research/audits/2026-11-04_rfc_edge_cases_and_soundness_audit.md`

---

## 1. Executive Summary

Lone distinguishes itself from conventional systems and application languages by establishing two strictly separated tiers of indirect memory addressing:
1. **`ref<T>` (Managed Reference)**: Safe, non-null, borrow-checked, lifetime-aware references that participate in automatic escape promotion to the Managed Store (heap) when escaping stack scopes.
2. **`ptr<T>` (Raw Memory Address)**: Low-level, zero-cost, unchecked byte offsets into WebAssembly linear memory (`i32`), designed for memory allocators, foreign function interfaces (FFI/WASI), and hardware-adjacent buffer management.

This exploration maps the **existing behaviors** across the parser, analyzer, and compiler backends, contrasts them with **prior art** (Rust, Zig, Go, C++), analyzes open questions regarding **nullability, arithmetic, auto-dereference, and immutability**, and proposes concrete semantics for **v0.6.0**.

---

## 2. Current State & Existing Behaviors in Lone (`v0.5.0-beta`)

### 2.1 Grammar & Syntax
In the Pratt expression parser (`pkg2/lone-parser`):
- **Managed Address-Of (`&expr`)**: Parsed as unary operator `UNARY_OP_REF` (`&`).
- **Raw Address-Of (`&raw expr`)**: Parsed as unary operator `UNARY_OP_RAW_REF` (`&raw`).
- **Dereference (`*expr`)**: Parsed as unary operator `UNARY_OP_DEREF` (`*`).
- **Type Grammar**: Both `ref<T>` and `ptr<T>` are parsed as `loneast.TypeBase` with generic arguments `[T]`.

### 2.2 Static Analysis & Typing (`pkg2/lone-analyzer`)
- **Type Representation** (`types.go`):
  - `RefType`: Struct `{ Elem: Type, ReadOnly: bool }` (printed as `ref<T>` or `ro ref<T>`).
  - `PtrType`: Struct `{ Elem: Type, ReadOnly: bool }` (printed as `ptr<T>` or `ro ptr<T>`).
- **Type Compatibility & Assignability**:
  - `ref<T>` is **never** implicitly assignable to `ptr<T>`.
  - `ptr<T>` is **never** implicitly assignable to `ref<T>`.
  - Covariant read-only conversion is supported: `ref<T>` is assignable to `ref<ro T>`, and `ptr<T>` is assignable to `ptr<ro T>`.
- **Unary Expression Evaluation** (`eval_type.go`):
  - `&x` on an expression of type `T` yields `RefType{Elem: T}`.
  - `&raw x` on an expression of type `T` yields `PtrType{Elem: T}`.
  - `*r` on `ref<T>` yields `T`.
  - `*p` on `ptr<T>` yields `T`.
  - Dereferencing any non-pointer/non-reference type emits diagnostic `type/invalid-deref`.
- **Explicit Type Casts (`as`)**:
  - `ref<T> as ptr<T>`: **Permitted** (explicit degradation from safe reference to raw address).
  - `ptr<T> as ptr<U>`: **Permitted** (pointer bitcast/transmutation).
  - `ptr<T> as ref<T>`: **Rejected** (`type/invalid-cast`).
- **Member Access Auto-Dereference**:
  - Accessing `obj.field` on a struct, `ref<Struct>`, or `ptr<Struct>` automatically dereferences the pointer/reference to resolve the field.

### 2.3 Compilation to WebAssembly (`pkg2/lone-compiler-wir`)
- **Runtime Representation**: In WebAssembly 32-bit linear memory, both `ref<T>` and `ptr<T>` evaluate to primitive `i32` byte offsets.
- **Current Backend Gap**:
  - In `pkg2/lone-compiler-wir/lower.go`, `&`, `&raw`, and `*` currently fall into the default unary expression handler without generating address loads or memory read/write instructions.
  - Wasm local variables (`local.get`, `local.set`) are virtual registers and cannot be addressed directly.
  - *Contrast with legacy `pkg/lone-compiler-mir`*: The legacy compiler tracked `AddressTaken = true` on symbols. Variables with taken addresses were allocated stack slots on the Page 0 datastack frame, and `&raw` emitted the slot address.

### 2.4 Existing Standard Library & Syscall Usage
In `dev/SPEC.md` and runtime headers:
- Syscalls and I/O rely exclusively on `ptr<u8>`:
  - `read(self, buf: ptr<u8>, max_len: i32) i32`
  - `read_line(in: ptr<u8>) str`
  - `int_to_str(val: i32, out_buf: ptr<u8>) str`
  - `Ciovec = struct { buf: ptr<u8>, len: i32 }`
- `ref<T>` is intended for high-level object references and generic contracts (`Ifc*`), while `ptr<u8>` / `ptr<any>` is the systems transport type.

---

## 3. The Core Architectural Divide: Safe vs. Unsafe Memory

Lone's core invariant **IL1 (Explicit over implicit)** demands that safe code and unsafe low-level mechanics remain visually and semantically distinct.

```
                    ┌──────────────────────────────────────────────┐
                    │            Lone Memory Semantics             │
                    └──────────────────────┬───────────────────────┘
                                           │
                   ┌───────────────────────┴───────────────────────┐
                   ▼                                               ▼
         Safe References:                                Raw Addresses:
             ref<T>                                         ptr<T>
  • Created via &x                               • Created via &raw x
  • Guaranteed non-null                          • Nullable (0 / none)
  • No pointer arithmetic                        • Indexed access (p[i]) or offset
  • Participates in escape promotion             • Unmanaged raw Wasm i32 offset
  • Checked by borrow / lifetime rules           • Developer assumes safety burden
  • Dereferenced via *r or r.field               • Dereferenced via *p or p.field
```

### Prior Art Comparison Matrix

| Feature | Lone `ref<T>` | Lone `ptr<T>` | Rust `&T` / `&mut T` | Rust `*const T` / `*mut T` | Zig `*T` / `[*]T` | Go `*T` | C++ `T&` |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: |
| **Creation** | `&x` | `&raw x` | `&x` / `&mut x` | `&raw const x` / `&raw mut x` | `&x` | `&x` | `x` (bind) |
| **Nullability** | Non-null | Nullable (`0`) | Non-null | Nullable (`0`) | Non-null (`?*T` for null) | Nullable (`nil`) | Non-null |
| **Arithmetic** | No | Yes (`p[i]`) | No | Via methods (`.add()`) | Yes (`p + i` for `[*]T`) | Via `unsafe` | No |
| **Escape Action** | Promoted to Heap | Dangling / Error | Blocked by borrow checker | Allowed (unchecked) | Allowed (unchecked) | Promoted to Heap | Dangling |
| **Dereference** | `*r` / `r.f` | `*p` / `p.f` | `*r` / `r.f` | `*p` (in `unsafe`) | `p.*` / `p.f` | `*p` / `p.f` | `r` (implicit) |
| **Underlying Wasm**| `i32` offset | `i32` offset | `i32` offset | `i32` offset | `i32` offset | `i32` offset | `i32` offset |

---

## 4. Key Semantic Dimensions & Design Questions

### Dimension 1: Nullability & Sentinel Values
- **The Problem**: In WebAssembly, linear memory addresses are 32-bit unsigned integers (`0x00000000` to `0xFFFFFFFF`). Offset `0` is a physical byte address. However, conventional languages use `0` as `NULL`.
- **Existing Lone Architecture**:
  - In `dev/SPEC.md` Section 8.1, offsets `0..63` are reserved for WASI scratch (`iovec`).
  - Offset `0` is therefore **never a valid user object address**.
- **Proposal**:
  1. `ref<T>` is **guaranteed non-null**. There is no "null reference". If an optional reference is needed, developers write `?ref<T>`, which represents `Option::Some(ref)` or `Option::None`.
  2. `ptr<T>` is a raw address. An address of `0` represents a null pointer.
  3. `ptr<T>` can be compared with `none` or `0`:
     ```lone
     if p == none { -- Checks if p == 0
         return Error::NullPointer
     }
     ```

### Dimension 2: Immutability & Binding Interaction (`let` vs. `var`)
How does taking the address of a binding affect reference mutability?

1. **Managed References (`&x`)**:
   - `let x: i32 = 42` $\implies$ `&x` produces **`ref<ro i32>`** (read-only reference).
   - `var x: i32 = 42` $\implies$ `&x` produces **`ref<i32>`** (mutable reference).
   - *Soundness Guarantee*: You cannot obtain a mutable `ref<T>` to an immutable `let` binding. Attempting to pass `&let_var` to a function expecting `ref<T>` triggers `type/call-arg-type`.
2. **Raw Pointers (`&raw x`)**:
   - `let x: i32 = 42` $\implies$ `&raw x` produces **`ptr<ro i32>`**.
   - `var x: i32 = 42` $\implies$ `&raw x` produces **`ptr<i32>`**.
   - Writing `*p = 100` when `p` is `ptr<ro i32>` is a compile-time error (`type/immutable-pointer-write`).

### Dimension 3: Dereferencing & Auto-Dereference Invariants
When is explicit `*` required, and when is auto-dereferencing applied?

1. **Field & Method Access (`.` operator)**:
   - **Auto-Dereference Applied**: `r.field` and `p.field` automatically dereference.
   - If `p` is `ptr<Point>`, `p.x` desugars statically to `(*p).x`. This eliminates cumbersome C-style `->` or `(*p).x` boilerplate.
2. **Scalar Value Reads & Writes**:
   - **Explicit `*` Required**:
     ```lone
     var num: i32 = 10
     var r = &num
     
     -- Read:
     let val: i32 = *r     -- Explicit dereference
     
     -- Write:
     *r = 20               -- Mutates 'num' through reference
     ```
3. **Rebinding vs. Mutating**:
   ```lone
   var a: i32 = 1
   var b: i32 = 2
   
   var r: ref<i32> = &a
   r = &b       -- REBINDING: 'r' now points to 'b'
   *r = 99      -- MUTATION: 'b' is now 99 ('a' is still 1)
   ```

### Dimension 4: Pointer Arithmetic & Array Traversal
`ref<T>` strictly prohibits pointer arithmetic. How should `ptr<T>` navigate contiguous buffers?

#### Option 4A: C-Style Binary Addition (`p + offset`)
- `p + 1` advances by `sizeof(T)` bytes.
- *Issue*: Hides byte arithmetic; ambiguous when `T` is `u8` vs `struct Big`.

#### Option 4B: Array Indexing Syntax (`p[i]`) (Recommended)
- Treats raw pointers as potential contiguous arrays:
  ```lone
  fn sum_buffer(buf: ptr<i32>, count: i32) i32 {
      var total = 0
      var i = 0
      while i < count {
          total = total + buf[i] -- Loads from (buf + i * sizeof(i32))
          i = i + 1
      }
      return total
  }
  ```
- Writing `buf[i] = val` stores directly into linear memory at offset `buf + i * sizeof(T)`.
- *Advantages*: Familiar, clear distinction between single-object dereference `*p` and indexed buffer access `p[i]`.

#### Option 4C: Explicit Byte Offset Method (`p.offset(bytes)`)
- Built-in method or intrinsic: `p.byte_offset(16)` or `p.advance(elements)`.
- Completely explicit, zero ambiguity.

> **Recommendation**: Adopt **Option 4B (`p[i]`)** for natural buffer traversal, supplemented with **Option 4C (`p.offset(n)`)** for explicit pointer adjustments.

### Dimension 5: Type Degradation & Transmutation Matrix

| Source Type | Target Type | Syntax | Safety | Compiler Action |
| :--- | :--- | :---: | :---: | :--- |
| `ref<T>` | `ptr<T>` | `r as ptr<T>` | Safe | Zero-cost: passes same Wasm `i32` address |
| `ref<T>` | `ref<ro T>` | Implicit / `r` | Safe | Subtyping: strips write privilege |
| `ptr<T>` | `ptr<ro T>` | Implicit / `p` | Safe | Subtyping: strips write privilege |
| `ptr<T>` | `ptr<U>` | `p as ptr<U>` | Unsafe | Zero-cost: reinterprets address as type `U` |
| `ptr<T>` | `i32` | `p as i32` | Unsafe | Zero-cost: exposes linear memory integer |
| `i32` | `ptr<T>` | `addr as ptr<T>`| Unsafe | Zero-cost: treats integer as memory address |
| `ptr<T>` | `ref<T>` | `#ref_from_ptr(p)` | Unsafe | Requires intrinsic check (non-null assert) |

---

## 5. Unified Memory & Escape Model: How `ref` and `ptr` Interact with Wasm Memory

In accordance with RFC-20261104 and the Soundness Audit (Edge Case 10):

```
+-------------------------------------------------------------+ 0x00000000
| Page 0: Reserved & Static Constant Pool                     |
+-------------------------------------------------------------+ dataEnd
| Page 0: Stack Guard Page                                    | $__stack_limit
|                                                             |
| v  Page 0: Datastack (grows downward)                    v  |
|    - Local structs & non-escaping &local                    |
|    - Caller SRET storage frames                             |
+-------------------------------------------------------------+ $__stack_pointer
| Free Linear Memory Region                                   |
+-------------------------------------------------------------+ 0x00010000 (64 KiB)
| Page 1+: Managed Store (Dynamic Heap)                       |
|    - Escaping ref<T> allocations (promoted via #alloc)      |
|    - Inlined Free-List Allocator with bi-directional merge  |
+-------------------------------------------------------------+ memory.size
```

### 5.1 The Lifecycle of `ref<T>`
1. **Local Frame**:
   ```lone
   fn compute() i32 {
       var val: i32 = 42
       let r: ref<i32> = &val
       return *r
   }
   ```
   `val` is stored in the function's local frame. `r` contains the frame address. `r` does not escape, so no heap allocation occurs. Zero cost.
2. **Escape Promotion**:
   ```lone
   fn make_box(val: i32) ref<i32> {
       var v = val
       return &v -- Escapes function scope!
   }
   ```
   The escape analyzer detects that `&v` outlives `make_box`. The compiler rewrites the allocation to `$__lone_rt_alloc(4)` in the Managed Store. When the caller drops the last reference, the compiler inserts `$__lone_rt_free`.

### 5.2 The Lifecycle of `ptr<T>`
1. **Unchecked Systems Pointer**:
   `ptr<T>` never triggers automatic heap promotion. It is a dumb `i32`.
2. **Escaping Stack Address**:
   ```lone
   fn bad_escape() ptr<i32> {
       var val: i32 = 42
       return &raw val -- COMPILE-TIME ERROR: borrow/stack-use-after-free
   }
   ```
   Because `ptr<T>` does not carry reference counting or managed lifetime metadata, returning a raw pointer to a stack-local variable is strictly rejected by static analysis.

---

## 6. Slices (`[]T`) as the Bridge Between `ref` and `ptr`

Lone's dynamic slice type `[]T` serves as the high-level, safe window into contiguous memory:

### 6.1 Physical Memory Layout
A slice `[]T` is an 8-byte fat structure:
```
+---------------+---------------+
|  ptr: ptr<T>  |   len: i32    |
|   (4 bytes)   |   (4 bytes)   |
+---------------+---------------+
Offset: +0              +4
```

### 6.2 Safe Construction from References & Buffers
1. **From Fixed Arrays (`[N]T`)**:
   ```lone
   var buffer: [1024]u8
   -- Safely produces slice view { ptr: &raw buffer[0], len: 1024 }:
   let slice: []u8 = &buffer[..]
   ```
2. **From Raw Heap Allocations**:
   ```lone
   let raw_mem: ptr<u8> = #alloc(4096)
   let slice: []u8 = slice_from_raw(raw_mem, 4096)
   ```

---

## 7. Concrete Proposals for Lone v0.6.0

### Proposal 1: Formalize `ref<T>` as Purely Safe & Managed
- Enforce non-null guarantee.
- Connect `AddressTaken` in `pkg2/lone-analyzer` so that taking `&x` ensures `x` is assigned a linear datastack slot if not already in memory.
- In `pkg2/lone-compiler-wir`, lower `&x` to `sp + slotOffset`.
- Lower `*r` to `i32.load` (for scalar reads) and `*r = val` to `i32.store`.

### Proposal 2: Formalize `ptr<T>` as the Low-Level Systems Type
- Retain `&raw x` syntax.
- Support index operator `p[i]` for buffer traversal.
- Implement explicit casting `p as ptr<U>` and `p as i32`.
- Retain prohibition on implicit `ptr` to `ref` conversions.

### Proposal 3: Address-Taken Spilling in WIR Backend (`pkg2/lone-compiler-wir`)
- When a variable has `AddressTaken == true`:
  - The function entry allocates a slot on the datastack frame:
    $$\text{slot} = \text{sp} + \text{offset}$$
  - Taking `&x` or `&raw x` emits `i32.const <offset>`, `local.get $__stack_pointer`, `i32.add`.
  - Reads and writes to `x` continue to work seamlessly via memory load/store or local cache.

---

## 8. Summary Table: `ref<T>` vs. `ptr<T>`

| Characteristic | `ref<T>` (Managed Reference) | `ptr<T>` (Raw Pointer) |
| :--- | :--- | :--- |
| **Primary Domain** | High-level application logic, OOP, interfaces | Systems programming, allocators, WASI FFI |
| **Creation** | `&expr` | `&raw expr` |
| **Dereference Syntax** | `*r` (value), `r.field` (auto-deref) | `*p` (value), `p.field` (auto-deref) |
| **Indexing Syntax** | Disallowed | Allowed: `p[i]` |
| **Nullability** | Guaranteed non-null (`?ref<T>` for optional) | Nullable (`0` / `none`) |
| **Pointer Arithmetic** | Forbidden | Permitted via `p[i]` or `p.offset(n)` |
| **Immutability Tracking**| Honors `let` $\implies$ `ref<ro T>` | Honors `let` $\implies$ `ptr<ro T>` |
| **Escape Analysis** | Automatically promoted to Managed Store | Unmanaged; escaping stack slot is compile error |
| **Wasm Codegen** | Linear memory `i32` offset | Linear memory `i32` offset |
| **Casting to Other Kind**| Explicit downgrade: `r as ptr<T>` | Unsafe upgrade: `#ref_from_ptr(p)` |
