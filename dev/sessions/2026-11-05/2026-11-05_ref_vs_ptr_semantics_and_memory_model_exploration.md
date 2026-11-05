# Implementation Report: Reference (`ref<T>`) vs. Pointer (`ptr<T>`) Semantics & Memory Model Exploration

**Date**: 2026-11-05  
**Scope**: Language Exploration, Memory Model & Compiler Codegen  
**Status**: Completed & Documented  
**Location**: `dev/sessions/2026-11-05/2026-11-05_ref_vs_ptr_semantics_and_memory_model_exploration.md`  

---

## 1. Overview & Objectives

Investigated and formalized the architectural and semantic distinction between **`ref<T>` (Managed Reference)** and **`ptr<T>` (Raw Linear Memory Address)** in Lone, cataloging existing compiler behaviors, contrasting with systems language prior art (Rust, Zig, Go, C++), and resolving core questions around nullability, arithmetic, auto-dereference, and WebAssembly memory placement.

---

## 2. Key Findings & Existing Behaviors

1. **Parser & Lexer (`pkg2/lone-parser`)**:
   - `&x` parsed as `UNARY_OP_REF` $\implies$ produces `ref<T>`.
   - `&raw x` parsed as `UNARY_OP_RAW_REF` $\implies$ produces `ptr<T>`.
   - `*expr` parsed as `UNARY_OP_DEREF` $\implies$ dereferences both `ref<T>` and `ptr<T>`.
2. **Type System & Analysis (`pkg2/lone-analyzer`)**:
   - `RefType` and `PtrType` are distinct types. Implicit conversion between them is strictly forbidden.
   - Explicit degradation `r as ptr<T>` is supported; `ptr<T> as ref<T>` is rejected.
   - Both support the read-only modifier: `ref<ro T>` and `ptr<ro T>`.
   - Auto-dereference is supported for member access: `r.field` and `p.field`.
3. **WIR Backend (`pkg2/lone-compiler-wir`)**:
   - WebAssembly linear memory addresses are `i32` offsets.
   - Wasm registers/locals (`local.get`, `local.set`) cannot have their address taken.
   - Identified the need to re-introduce `AddressTaken` frame spilling from legacy `lone-compiler-mir` to properly lower `&x` and `&raw x` onto the datastack frame.

---

## 3. Core Proposals Authored

1. **Strict Two-Tier Model**:
   - `ref<T>`: Safe, non-null, borrow-checked, escape-promoted to Managed Store (heap) when escaping functions.
   - `ptr<T>`: Unchecked, nullable (`0`), systems pointer. Escaping a stack-local pointer triggers compile-time use-after-free diagnostics.
2. **Buffer Traversal via Indexing**:
   - Adopt `p[i]` indexing for contiguous buffer access on `ptr<T>`.
3. **Immutability Invariant**:
   - `let` bindings produce `ref<ro T>` or `ptr<ro T>`, preventing accidental mutation of immutable variables.

---

## 4. Documentation & Verification

- Created comprehensive exploration document: [`dev/research/exploration/ref_vs_ptr_semantics_and_memory_model.md`](../../research/exploration/ref_vs_ptr_semantics_and_memory_model.md).
- Verified full test suite passes with 100% success across all packages (`go test ./...`).
