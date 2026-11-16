# Research Audit: Reference & Pointer Semantics, Closure Captures, and Cross-RFC Soundness

**Date**: 2026-11-05  
**Scope**: RFC Stress-Testing & Soundness Verification  
**Audited RFCs**:
- `RFC-20261105-reference_and_pointer_semantics`
- `RFC-20261104-interface_data_contracts`
- `RFC-20261104-struct_initialization_and_field_modifiers`
**Status**: Completed & Approved  
**Location**: `dev/research/audits/2026-11-05_ref_ptr_and_closure_soundness_audit.md`  

---

## 1. Executive Summary

This audit continues the rigorous stress-testing initiated in the first audit (`2026-11-04_rfc_edge_cases_and_soundness_audit.md`, E1–E10), focusing on the newly proposed **Reference & Pointer Semantics (`ref<T>` vs. `ptr<T>`)** and its interaction with **Interface Data Contracts**, **Struct Initialization**, and **Inner Closure Captures**.

Across 11 additional investigated edge cases (**E11 through E21**), the language maintains strict adherence to **IL1 (Explicit over implicit)**, **IL2 (No implicit built-ins)**, zero hidden runtime allocations, and deterministic memory safety.

---

## 2. Pointer & Reference Boundary Edge Cases

### Edge Case 11: Null Pointer Dereference in WebAssembly (`*p` when `p == 0`)
- **Question**: WebAssembly linear memory offsets `0..63` are reserved for WASI scratch (`iovec`). If code executes `*p` or `p[i]` when `p == 0`, what happens at runtime?
- **Analysis**:
  - In C/C++, dereferencing null triggers a hardware page fault. In WebAssembly, offset 0 is physically accessible memory. A blind `i32.load offset=0` would silently read bytes from WASI scratch without trapping, causing subtle memory corruption.
- **Resolution**:
  1. **`ref<T>` (Safe Tier)**: Guaranteed non-null at compile time. Never requires a runtime null check. Zero execution overhead.
  2. **`ptr<T>` (Systems Tier)**:
     - In debug/safe compilation modes: The compiler emits an `i32.eqz` check before raw dereference/index load instructions; if `p == 0`, it immediately executes `unreachable` (Wasm trap).
     - In release/unchecked mode: Direct memory loads/stores are emitted for bare-metal performance.
     - Developers are expected to guard nullable pointers with `if p != none`.

### Edge Case 12: Interior Pointers & References (`&s.field`, `&arr[i]`)
- **Question**: Can a developer take the address of an individual struct field (`&s.field` or `&raw s.field`) or array element (`&arr[i]`)? What happens if that interior reference escapes?
- **Analysis**:
  - If `s` is a stack-allocated struct, its field `s.field` resides at address `sp + slotOffset + fieldOffset`.
  - Taking `&s.field` yields an interior reference `ref<FieldType>`.
  - If `&s.field` escapes the function (e.g. `return &s.field`), the runtime cannot allocate just a partial field on the heap.
- **Resolution**:
  - **Whole-Object Escape Promotion**: When an interior reference `&s.field` escapes, the escape analyzer automatically promotes the **entire enclosing struct `s`** to the Managed Store (heap `#alloc`).
  - The returned `ref<FieldType>` points into the heap allocation of `s`. The heap block's reference count is bound to the interior reference, preventing use-after-free until all interior references are dropped.

### Edge Case 13: Multi-Level Indirection (`ptr<ptr<T>>` vs. `ref<ref<T>>`)
- **Question**: Are multi-level pointers (`ptr<ptr<T>>`) and multi-level references (`ref<ref<T>>`) supported?
- **Analysis**:
  - `ptr<ptr<T>>` is fundamental for systems programming (e.g. string tables, 2D arrays, or dynamic pointer tables). In Wasm, an array of pointers is simply a contiguous sequence of `i32` addresses.
  - In safe application code, `ref<ref<T>>` introduces unnecessary indirection layers and ambiguity regarding which reference is being rebound vs mutated.
- **Resolution**:
  - `ptr<ptr<T>>` is fully supported: `*p` yields `ptr<T>`, and `**p` yields `T`.
  - `ref<ref<T>>` is **auto-flattened** to `ref<T>` (transparent alias) or flagged as a static warning (`type/redundant-reference`).

### Edge Case 14: Untyped Raw Pointers (`ptr<any>`)
- **Question**: What operations are permitted on `ptr<any>` (the equivalent of `void*`)?
- **Analysis**:
  - `ptr<any>` represents a raw linear memory address whose target type and byte width are unknown.
- **Resolution**:
  - **Dereferencing `*p` is Strictly Prohibited**: The compiler cannot emit a Wasm memory instruction because the type size (`i32`, `i64`, `f64`) is unknown (`type/invalid-deref`).
  - **Indexing `p[i]` is Strictly Prohibited**: The stride step size is unknown.
  - Operations on `ptr<any>` are limited to:
    1. Null checks: `if p == none`.
    2. Pointer equality: `p == other`.
    3. Explicit cast: `p as ptr<u8>` (byte-level access) or `p as ptr<ConcreteType>`.

### Edge Case 15: Zero-Sized Types (`ref<Unit>`, `ptr<Unit>`)
- **Question**: If a struct has no fields (`type Unit = struct {}`), what happens when someone takes `&Unit::{}` or `&raw Unit::{}`?
- **Analysis**:
  - `sizeof(Unit) == 0`. Reserving stack space for a 0-byte struct costs 0 bytes.
- **Resolution**:
  - Taking the address of a zero-sized type does not consume datastack memory.
  - Codegen emits the current `$__stack_pointer` (or a dedicated aligned non-zero sentinel).
  - Dereferencing `*u` is a compile-time no-op that yields `Unit::{}` without emitting any Wasm load instructions.

### Edge Case 16: Address-Taken Variables Inside Loops
- **Question**: What happens if `&x` or `&raw x` is taken on a variable declared inside a `while` loop body?
  ```lone
  while cond {
      var x: i32 = 10
      process(&x)
  }
  ```
- **Analysis**:
  - If the compiler adjusted `$__stack_pointer` inside the loop body on every iteration, the datastack would rapidly grow and trigger a stack overflow.
- **Resolution**:
  - **Prologue Frame Reservation**: Datastack frame slots for all local variables (including those in nested blocks and loops) are computed and reserved **once at function prologue**.
  - Loop iterations reuse the exact same stack slot, guaranteeing zero stack growth during loop execution.

---

## 3. Compound Structures & Escape Transitivity

### Edge Case 17: References Stored Inside Structs (Graphs & Linked Data)
- **Question**: Can a struct store a managed reference (`next: ?ref<Node>`)? What happens when the enclosing struct escapes?
  ```lone
  pub type Node = struct {
      val: int,
      next: ?ref<Node>,
  }
  ```
- **Analysis**:
  - If struct `A` is allocated on the heap (Managed Store), but its field `a.next` points to a local variable `b` on the datastack frame, when the function returns, `a.next` becomes a dangling pointer.
- **Resolution**:
  - **Transitive Escape Promotion**: Storing a reference `&b` into a struct field `a.next` where `a` outlives the local frame forces `b` to escape as well.
  - The escape analyzer promotes `b` to the Managed Store alongside `a`.

### Edge Case 18: Pointer Traversal with Read-Only Modifier (`ptr<ro T>`)
- **Question**: If `p` is `ptr<ro T>` (e.g. from an immutable `let arr: [10]i32`), what does `p[i]` and `p.offset(n)` produce?
- **Analysis**:
  - If calling `p.offset(1)` yielded a mutable `ptr<T>`, an author could bypass `let` immutability simply by writing `(p.offset(0))[0] = 999`.
- **Resolution**:
  - **Const-Preserving Arithmetic**: Calling `p.offset(n)` or `p.byte_offset(n)` on `ptr<ro T>` strictly returns `ptr<ro T>`.
  - Reading `let v = p[i]` is allowed.
  - Writing `p[i] = val` on `ptr<ro T>` is rejected at compile time: `type/immutable-pointer-write`.

---

## 4. Cross-RFC Interactions (Interfaces, Struct Init, and Pointers)

### Edge Case 19: Raw Pointers vs. Interface Duck Typing
- **Question**: Can a raw pointer `p: ptr<Point>` directly satisfy a data contract `T where { T satisfies IfcPoint }`?
- **Analysis**:
  - An interface describes the data layout of a concrete type (`x: int, y: int`), not an unchecked 4-byte pointer.
- **Resolution**:
  - **Nominal/Structural Data Satisfaction Only**: `ptr<Point>` does **not** satisfy `IfcPoint`. Only `Point` (and object literals `{ .x, .y }`) satisfies it.
  - To pass a raw pointer to an interface-constrained function, the caller must explicitly dereference it (`*p`) or borrow it as a safe reference (`&*p`).

### Edge Case 20: Struct-Private Field Protection Under Raw Memory Casts
- **Question**: Can an author bypass struct-private encapsulation (`_secret_hash`) by casting `ref<Struct>` to `ptr<u8>` and indexing raw byte offsets?
- **Analysis**:
  - At the language type-checker level: `instance._secret_hash` is strictly forbidden outside methods of `T`.
  - However, WebAssembly linear memory is a flat byte space. Once code has a `ptr<u8>`, physical loads can read any byte offset.
- **Resolution**:
  - **Clear Boundary Between Safe and Systems Tiers**:
    - Encapsulation is a **guarantee of the safe type system**. High-level Lone code cannot access private fields or violate constructor invariants.
    - Converting `ref<T>` to `ptr<T>` is an explicit exit from the safe sandbox into systems-level programming.
    - Casting a struct with private fields to raw pointer (`r as ptr<any>`) requires an explicit cast acknowledging that type safety is bypassed.

---

## 5. Closure Capture & Lifetime Invariants

### Edge Case 21: Reference Capture in Inner Closures
- **Question**: What happens when an inner closure captures a local variable by reference (`ref<T>`)? Does the captured object live "rent-free" on the Managed Store even if the closure is never called?
  ```lone
  fn outer() fn() void {
      var x = 100
      fn inner() void {
          #print_int(x) -- captures &x
      }
      return inner -- escapes outer scope!
  }
  ```
- **Analysis**:
  - If the compiler silently promoted `x` to the Managed Store whenever captured, a closure held in an event handler, timer, or long-lived struct would keep `x` hostage indefinitely on the heap even if never invoked.
  - Furthermore, capturing a *mutable reference* across asynchronous boundaries creates data race hazards.
- **Resolution (Scoped to Inner Closures)**:
  1. **Non-Escaping Closures (Safe Stack Borrowing)**:
     - When an inner closure does not escape the enclosing function (e.g. passed synchronously to `items.for_each(fn)`), capturing local stack variables by reference is **100% permitted**.
     - **Cost**: **Zero heap allocations**. The enclosing function's datastack frame remains alive throughout the call.
  2. **Escaping Closures (Forbidden Reference Capture)**:
     - When an inner closure escapes its defining scope (returned, stored in a long-lived struct, or registered in a callback queue), **capturing local stack variables by reference (`ref<T>`) is STRICTLY FORBIDDEN**.
     - The compiler emits diagnostic `closure/escaping-reference-capture`.
     - **Allowed Alternatives**:
       - **Capture by Value (Copy / Move)**: The closure takes an owned snapshot of the data.
       - **Explicit Managed Store Allocation**: If shared persistence is genuinely required, the developer must explicitly allocate it via `#alloc` or standard managed structures.

---

## 6. Comprehensive Edge Case Reference (E1–E21)

| Edge Case | Description | Formal Soundness Guarantee |
| :---: | :--- | :--- |
| **E1** | Named Factory Constructors | Associated functions on `T` use `Self::{ ... }`; `ro` fields freeze permanently. |
| **E2** | Reassignment of `ro` in `__init__` | Reassignment `s.ro = v` strictly forbidden everywhere; must initialize in literal. |
| **E3** | Scope of Privacy (`_`) | Struct-private (Option A, mirroring JS `#field`); accessible only to methods of `T`. |
| **E4** | Zero-Field Structs (`Unit`) | `Unit::{}` allowed; `Unit::()` forbidden without explicit `__init__`. |
| **E5** | Methods vs. Data Fields | Methods never satisfy data fields; no implicit getters or hidden vtables. |
| **E6** | Reference Mutability Transitivity | Mutation requires mutable interface field + mutable struct field + mutable `ref<T>`. |
| **E7** | Array vs. Slice Invariance | `[N]T` cannot satisfy `[]T` directly; requires explicit slicing view. |
| **E8** | Interface Embedding Collision | Identical fields deduplicate; conflicting types trigger compile error. |
| **E9** | Enums Prohibited from Ifc | Tagged unions cannot satisfy interfaces; requires pattern matching. |
| **E10** | Datastack SRET & Escape Promotion| Values return via Caller SRET; escaping `ref<T>` promoted to Managed Store. |
| **E11** | Null Pointer Dereference | `ref<T>` never null (zero cost); `ptr<T>` traps on 0 in safe mode. |
| **E12** | Interior References (`&s.f`) | Escaping interior reference promotes entire enclosing struct `s` to heap. |
| **E13** | Multi-Level Indirection | `ptr<ptr<T>>` allowed; `ref<ref<T>>` auto-flattened to `ref<T>`. |
| **E14** | Untyped Pointer (`ptr<any>`) | Dereference `*p` and indexing `p[i]` forbidden without explicit cast. |
| **E15** | Zero-Sized Types (`Unit`) | Zero stack reservation; dereference is compile-time no-op. |
| **E16** | Loops with Taken Addresses | Single stack frame reservation at prologue; zero stack growth in loops. |
| **E17** | References in Struct Fields | Transitive escape promotion: heap struct promotes contained stack references. |
| **E18** | Read-Only Pointer Traversal | `ptr<ro T>.offset()` preserves `ro`; `p[i] = v` rejected at compile time. |
| **E19** | Pointers vs. Interfaces | `ptr<T>` does not satisfy interfaces; must pass `ref<T>` or value `*p`. |
| **E20** | Encapsulation & Raw Pointers | Encapsulation enforced at type level; casting to `ptr` is explicit unsafe opt-out. |
| **E21** | Reference Capture in Closures | Stack borrowing allowed for non-escaping closures; forbidden in escaping closures. |
