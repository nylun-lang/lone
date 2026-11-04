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
  - The encapsulation boundary is **lexical file scope**.
  - Any associated function declared in the **same source file** as the struct can instantiate it using raw aggregate syntax `Self::{ ... }`, setting private fields (`_*`) and `ro` fields during the aggregate expression.
  - Outside callers in other files are strictly restricted to `Type::(...)` (dispatching to `__init__`) or public factory methods.
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
- **Question**: Does `_field` mean private to the file or private to the package/module?
- **Resolution**: **File-Scoped.**
  - Lone uses flat, explicit imports. To avoid mysterious cross-file state access within a directory, fields starting with `_` are private strictly to their declaring `.lone` file.
  - Code in adjacent files in the same project directory must respect the public API of the struct.

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

### Edge Case 10: Object Literals and Datastack Escapes
- **Question**: When an anonymous object literal `{ .x: 10, .y: 20 }` satisfies an interface, where does it live and can it escape?
- **Resolution**:
  - Stack allocation: Object literals live in the caller's Page 0 datastack frame.
  - Escape analysis: Returning `ref<IfcPoint>` pointing to a stack-allocated object literal triggers `borrow/stack-use-after-free`.

---

## 4. Audit Conclusion & Action Plan

Both designs are formally sound, robust, and free of hidden runtime costs.
The resolutions above are incorporated into:
1. `dev/rfcs/drafts/RFC-20261104-interface_data_contracts.md`
2. `dev/rfcs/drafts/RFC-20261104-struct_initialization_and_field_modifiers.md`
