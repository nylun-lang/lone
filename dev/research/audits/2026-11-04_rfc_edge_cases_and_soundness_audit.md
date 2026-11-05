# Research Audit: Soundness & Edge Case Stress-Test for Data Contracts & Struct Initialization

**Date**: 2026-11-04  
**Scope**: RFC Stress-Testing & Soundness Verification  
**Audited RFCs**:
- `RFC-20261104-interface_data_contracts`
- `RFC-20261104-struct_initialization_and_field_modifiers`
**Status**: Completed & Approved  
**Location**: `dev/research/audits/2026-11-04_rfc_edge_cases_and_soundness_audit.md`  

---

## 1. Executive Summary

This audit rigorously tests the soundness, boundary conditions, and potential semantic ambiguities of the two newly proposed language mechanisms:
1. **Interface Data Contracts (`Ifc*`)**
2. **Struct Initialization Modes, `__init__`, and Field Modifiers**

Across 10 investigated edge cases, the underlying architecture proves exceptionally sound when grounded in Lone's core invariants: **IL1 (Explicit over implicit)**, **IL2 (No implicit built-ins)**, and **zero-cost WebAssembly execution**.

---

## 2. Struct Initialization & Encapsulation Edge Cases

### Edge Case 1: Factory Methods & Additional Constructors
- **Question**: If `ro` fields can only be set in `__init__`, how can structs provide alternative named constructors (e.g. `TcpSocket::connect(...)` or `from_fd(...)`)?
- **Analysis**: If factory methods could never set `ro` fields, constructors would be restricted to a single signature, forcing clumsy wrapping.
- **Resolution**:
  - The encapsulation boundary is **associated struct scope (Option A)**.
  - Any associated function or constructor declared on the struct `T` itself (such as `pub fn connect(...) Self` or `__init__`) can instantiate it using raw aggregate syntax `Self::{ ... }`, setting private fields (`_*`) and `ro` fields during the aggregate expression.
  - Outside callers in non-associated code or other files are strictly restricted to `Type::(...)` (dispatching to `__init__`) or public factory methods.
  - *Invariant*: Once the `Self::{ ... }` aggregate expression finishes evaluating, its `ro` fields are permanently frozen.

### Edge Case 2: Reassignment of `ro` Fields Inside `__init__`
- **Question**: Can an author mutate an `ro` field via assignment inside `__init__` before returning?
  ```lone
  pub fn __init__() Self {
      var s = Self::{ .ro_id: 1 }
      s.ro_id = 2 -- Is this permitted inside __init__?
      return s
  }
  ```
- **Resolution**: **Strictly Forbidden.**
  - `ro` fields can **only** be initialized within the `Self::{ ... }` literal itself.
  - Once constructed, `s.ro_id = 2` triggers `type/immutable-field-write`, even inside `__init__`.
  - *Rationale*: Guarantees that passing `&s` to a helper function within `__init__` never encounters a mutating `ro` field.

### Edge Case 3: Scope of Privacy (`_`)
- **Question**: Does `_field` mean private to the file, private to the package, or private to the struct itself?
- **Resolution**: **Struct-Private Scope (Option A - mirroring JavaScript `#field`).**
  - Fields prefixed with `_` are strictly private to the declaring struct `T` itself.
  - Access to `_field` is permitted **only** within methods and associated functions declared on `T` (via `self._field` or `Self::__init__`).
  - Standalone functions—even within the exact same `.lone` source file—**cannot** access `instance._field`.
  - External code and non-associated functions attempting to read or write `instance._field` receive compile-time diagnostic `type/private-field-access`.
  - This provides strict, uncompromising encapsulation without accidental state leaks across functions in the same module.

### Edge Case 4: Zero-Field Structs (`type Unit = struct {}`)
- **Resolution**:
  - `Unit::{}` has zero private fields, so it is legal from anywhere.
  - `Unit::()` is **illegal** unless the struct explicitly declares `pub fn __init__() Unit`. Lone never synthesizes magic implicit constructors (**IL1**).

---

## 3. Interface Data Contracts (`Ifc*`) Edge Cases

### Edge Case 5: Methods vs. Data Fields (No Implicit Getters!)
- **Question**: If an interface declares `x: int`, can a struct declaring a method `pub fn x(self) int` satisfy it?
- **Analysis**: In languages with dynamic dispatch or properties (e.g. JavaScript/TypeScript), method getters can satisfy properties. In a WebAssembly systems language, fields are static linear memory byte offsets (`i32.load offset=N`), whereas methods are function invocations.
- **Resolution**: **Strictly Prohibited.**
  - Method signatures **never** satisfy data fields.
  - The compiler will never synthesize synthetic getter wrappers or dynamic vtables.
  - *Rule*: Interface fields can only be satisfied by physical struct fields or object literal fields.

### Edge Case 6: Reference Immutability Transitivity
- **Question**: What happens if an interface declares a mutable field `count: int`, the struct declares `count: int`, but the consumer passes an immutable reference `ref<ro Counter>`?
- **Resolution**:
  - The struct `Counter` satisfies the interface `IfcWritable`.
  - However, when `ref<ro Counter>` is passed to a generic function `fn inc<T where { T satisfies IfcWritable }>(c: ref<T>)`, attempting to write `c.count = c.count + 1` triggers a static analysis error: `type/immutable-reference-write`.
  - *Rule*: Mutation through an interface requires all three conditions:
    1. Interface field is mutable (no `ro`).
    2. Struct field is mutable (no `ro`).
    3. The reference binding itself is mutable (not `ro`).

### Edge Case 7: Fixed Arrays (`[N]T`) vs. Dynamic Slices (`[]T`)
- **Question**: Can a struct declaring `data: [1024]u8` satisfy an interface declaring `data: []u8`?
- **Resolution**: **Incompatible.**
  - `[1024]u8` is an inline flat array of 1024 bytes.
  - `[]u8` is an 8-byte fat-pointer slice `{ ptr: i32, len: i32 }`.
  - Duck-typing requires identical memory layout. They are incompatible without explicit slicing (`&data[..]`).

### Edge Case 8: Embedding Name Collisions
- **Question**: What happens when an interface embeds two interfaces that share a field name?
- **Resolution**:
  - If both fields have the **exact same type and mutability**, they deduplicate cleanly into a single requirement.
  - If field types or mutability conflict (e.g. `id: int` vs `id: str`, or `ro id: int` vs `id: int`), the compiler emits a compile-time error: `type/interface-embedding-collision`.

### Edge Case 9: Enums Cannot Satisfy Interfaces
- **Question**: If an enum variant contains fields matching an interface, can the enum satisfy it?
- **Resolution**: **Prohibited.**
  - Enums are tagged unions with discriminant tags at offset 0. Monomorphized direct memory loads without discriminant checks would cause memory unsafety.
  - Enums can only be inspected via exhaustive `match` expressions. Only nominal structs and object literals can satisfy interfaces.

### Edge Case 10: Object Literals, Reference Escapes, and Memory Placement
- **Question**: When an anonymous object literal `{ .x: 10, .y: 20 }` or local struct satisfies an interface, where does it live and what happens if a reference to it escapes? Can the language "cheat" using caller stack offsets or automatic heap promotion?
- **Analysis**:
  - In WebAssembly, Page 0 is a downward-growing datastack that rewinds upon function return. Returning a raw pointer to a callee stack frame causes a dangling pointer / use-after-free bug.
  - However, modern systems languages employ two complementary, leak-free memory mechanisms:
    1. **Caller-Allocated Datastack Storage (SRET / "Goose" strategy)** for values.
    2. **Automatic Escape Promotion to Managed Store** for escaping managed references (`ref<T>`).
- **Resolution: Unified Memory Strategy**:
  1. **Value Return (Caller Datastack SRET)**:
     - When a function returns a concrete struct or interface-satisfying object by value (`fn f() Point` or `fn f() { .x: int, .y: int }`), the caller allocates the space directly on its own datastack frame before calling the callee.
     - The caller passes a hidden `sret_ptr` pointer to the callee, which writes fields directly into the caller's frame.
     - *Cost*: Zero heap allocation, zero GC/ref-counting overhead, zero memory leaks. Memory is instantly reclaimed when the caller's frame rewinds.
  2. **Non-Escaping Local References (`&local`)**:
     - When `&obj` does not escape the local scope (e.g. passed downwards to `render_point(&pt)`), it resides safely on the local function's Page 0 datastack frame. Zero allocation overhead.
  3. **Escaping Managed References (`ref<T>` Promotion)**:
     - When the compiler's escape analyzer detects that a reference escapes the declaring function (e.g. `return &Point::{ .x: 10, .y: 20 }` or storing into a long-lived struct field), it automatically promotes the allocation to the **Managed Store** (dynamic heap via `$__lone_rt_alloc` / `#alloc` in Page 1+).
     - **Deterministic Reclamation**: The compiler inserts automatic reference drop cleanup (`$__lone_rt_free` / `#free`). Memory is immediately coalesced by the free-list allocator with bi-directional coalescing.
     - *No stop-the-world GC*: Completely deterministic, zero pause times.
  4. **Unmanaged Raw Pointers (`ptr<T>`)**:
     - Raw pointers obtained via `&raw` bypass the managed reference safety model. If a raw pointer to a stack-local frame is returned or escapes, the static analyzer flags `borrow/stack-use-after-free`.

---

## 4. Audit Conclusion & Action Plan

Both designs are formally sound, robust, and free of hidden runtime costs.
The resolutions above are incorporated into:
1. `dev/rfcs/accepted/RFC-20261104-interface_data_contracts.md`
2. `dev/rfcs/accepted/RFC-20261104-struct_initialization_and_field_modifiers.md`
