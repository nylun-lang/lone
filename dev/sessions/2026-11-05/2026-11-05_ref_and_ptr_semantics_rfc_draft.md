# Implementation Report: Formal RFC Draft for Reference & Pointer Semantics

**Date**: 2026-11-05  
**Scope**: Language RFC Authoring & Compiler Architecture  
**Status**: Completed & Documented  
**Location**: `dev/sessions/2026-11-05/2026-11-05_ref_and_ptr_semantics_rfc_draft.md`  

---

## 1. Overview & Objectives

Following user approval of the memory model recommendations in the research exploration, drafted a formal language RFC:
- [`RFC-20261105-reference_and_pointer_semantics`](../../rfcs/drafts/RFC-20261105-reference_and_pointer_semantics.md)

---

## 2. Key Resolutions Formalized

1. **Non-Null References (`ref<T>`)**:
   - `ref<T>` is guaranteed non-null. Optional references are expressed via `?ref<T>`.
2. **Nullable Raw Addresses (`ptr<T>`)**:
   - `ptr<T>` is nullable (sentinel `0` / `none`). Offset 0 in WebAssembly linear memory is within the reserved scratch page (`0..63` for WASI scratch), ensuring user objects are never placed at 0.
   - Pointers can be checked directly with `if p == none`.
3. **Buffer Indexing & Arithmetic**:
   - `ptr<T>` supports contiguous array indexing `p[i]` (reading/writing at `p + i * sizeof(T)`).
   - Explicit stepping supported via `p.offset(count)` and `p.byte_offset(bytes)`.
   - `ref<T>` strictly forbids indexing and pointer arithmetic.
4. **Binding Mutability Invariant**:
   - Taking the address of a `let` binding yields `ref<ro T>` or `ptr<ro T>`, preventing mutation bypass of immutable bindings.
5. **Backend Implementation Architecture (`AddressTaken` Spilling)**:
   - Evaluated `&x` and `&raw x` sets `AddressTaken = true` on the variable symbol.
   - At function entry in `pkg2/lone-compiler-wir`, `AddressTaken` variables are allocated slots on the Page 0 datastack frame, and `&x` lowers to `sp + slotOffset`.
   - Escaping managed references (`ref<T>`) are automatically promoted to the Managed Store (heap `#alloc`), while escaping unmanaged raw pointers (`ptr<T>`) are rejected at compile time.

---

## 3. Verification

- All 22 test packages pass cleanly (`go test ./...`).
- RFC cataloged under `dev/rfcs/drafts/`.
