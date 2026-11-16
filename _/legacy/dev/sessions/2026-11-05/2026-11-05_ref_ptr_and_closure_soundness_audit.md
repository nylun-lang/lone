# Implementation Report: Reference, Pointer & Closure Soundness Audit (E11–E21)

**Date**: 2026-11-05  
**Scope**: RFC Stress-Testing, Soundness Verification & Closure Capture Boundary  
**Status**: Completed & Documented  
**Location**: `dev/sessions/2026-11-05/2026-11-05_ref_ptr_and_closure_soundness_audit.md`  

---

## 1. Overview & Objectives

Conducted a thorough soundness audit across 11 boundary cases (**E11–E21**) evaluating `RFC-20261105-reference_and_pointer_semantics`, `RFC-20261104-interface_data_contracts`, and `RFC-20261104-struct_initialization_and_field_modifiers`.

A primary objective was resolving the user's inquiry regarding **reference captures in inner function scopes**, preventing captured objects from living "rent-free" on the Managed Store.

---

## 2. Key Resolutions Formalized

1. **Inner Closure Captures (E21)**:
   - **Non-Escaping Closures**: Synchronous closures (e.g. `items.for_each(fn)`) are permitted to borrow local stack references with **zero heap allocations**. The datastack frame remains valid.
   - **Escaping Closures**: Closures that outlive their defining function (returned, stored in persistent structs) are **strictly forbidden** from capturing local stack variables by reference (`ref<T>`). Must capture by value or reference explicitly managed objects. Prevents zombie heap allocations and eliminates race conditions.
2. **Null Pointer Traps in Wasm (E11)**:
   - `ref<T>` is guaranteed non-null (zero runtime checks).
   - `ptr<T>` traps on address `0` in safe mode to prevent silent reads from the reserved WASI scratch space.
3. **Interior Reference Whole-Object Promotion (E12)**:
   - Escaping `&s.field` promotes the entire enclosing struct `s` to the Managed Store.
4. **Untyped Pointer Safety (E14)**:
   - Dereference `*p` and indexing `p[i]` are strictly forbidden on `ptr<any>` without an explicit cast.
5. **Loop Frame Reuse (E16)**:
   - Datastack frames for all variables with `AddressTaken == true` are reserved once at prologue, preventing datastack growth inside loops.
6. **Interface Non-Conformance for Raw Pointers (E19)**:
   - Raw pointers `ptr<T>` do not satisfy interfaces; callers must pass `ref<T>` or value `*p`.

---

## 3. Documentation & Verification

- Published complete audit in [`dev/research/audits/2026-11-05_ref_ptr_and_closure_soundness_audit.md`](../../research/audits/2026-11-05_ref_ptr_and_closure_soundness_audit.md).
- Updated [`dev/rfcs/drafts/RFC-20261105-reference_and_pointer_semantics.md`](../../rfcs/drafts/RFC-20261105-reference_and_pointer_semantics.md) with Section 5.3 and diagnostic `closure/escaping-reference-capture`.
- All 22 test packages pass (`go test ./...`).
