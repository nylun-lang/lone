# Implementation Milestone: Unsafe Pointer Indexing & Typestate Buffer Semantics

**Date**: 2026-11-05  
**Scope**: Acceptance & Full Implementation of [`RFC-20261105-unsafe_pointer_indexing_and_buffer_semantics`](../../rfcs/accepted/RFC-20261105-unsafe_pointer_indexing_and_buffer_semantics.md)  
**Status**: Completed & Verified Across All 22 Packages  
**Location**: `dev/sessions/2026-11-05/2026-11-05_unsafe_pointer_indexing_and_buffer_semantics_implementation.md`  

---

## 1. Overview & Acceptance

Following consensus on typestate pointer separation, fat slice references, field writes through references, and caller-side pass-by-value struct copies, [`RFC-20261105-unsafe_pointer_indexing_and_buffer_semantics.md`](../../rfcs/accepted/RFC-20261105-unsafe_pointer_indexing_and_buffer_semantics.md) was officially moved to **Accepted** status.

The implementation was applied across `pkg2/lone-analyzer` and `pkg2/lone-compiler-wir`, delivering 100% test pass rate with zero regressions.

---

## 2. Implemented Subsystems & Capabilities

### 2.1 Pointer Hierarchy & Typestate Indexing Rules (`pkg2/lone-analyzer`)
- **`ptr<T>` (Single-Item Pointer)**:
  - Indexing `p[i]` is statically forbidden, raising `type/single-pointer-indexing-rejected`.
  - Dereference stores `*p = val` remain permitted on mutable pointers.
- **`ptr<[N]T>` (Sized Buffer Pointer)**:
  - Constant indices $i \ge N$ or $i < 0$ raise compile-time error `type/index-out-of-bounds`.
  - Dynamic indices $p[i]$ are permitted and compiled with dynamic checks.
  - Safe decay: `ptr<[N]T>` is assignable to `ptr<[]T>`.
- **`ptr<[]T>` (Unsized Buffer Pointer)**:
  - Permitted without bounds check; dereference `*buf` accesses index 0 (`buf[0]`).
- **`ref<[]T> ≡ []T` (Fat References)**:
  - `ref<[]T>` is structurally compatible with `[]T`.
  - Indexing is allowed on `ref<[N]T>` and `ref<[]T>`; rejected on scalar `ref<T>` (`type/invalid-index`).
- **Read-Only Enforcements**:
  - `type/immutable-pointer-write` and `type/immutable-reference-write` prevent writes to `ro` pointer/reference buffers.
- **Explicit Casts**:
  - `evalCastExpr` permits integer $\leftrightarrow$ pointer casts (e.g. `#alloc(64) as ptr<[]int>`).

### 2.2 WebAssembly IR Lowering (`pkg2/lone-compiler-wir`)
- **Dynamic Bounds Checking**:
  - Lowering `p[i]` on `ptr<[N]T>` or `ref<[N]T>` for dynamic indices emits an unsigned comparison against immediate $N$ (`i32.ge_u N`) that traps via `unreachable` on out-of-bounds access.
  - Constant indices fold statically with zero runtime checks.
  - Lowering `p[i]` on `ptr<[]T>` emits direct zero-overhead address calculations (`base + index * elemSz`).
- **List Literals**:
  - `[10, 20, 30]` allocates contiguous memory and stores initialized elements.
- **Address-Of Arrays**:
  - `&scores` and `&raw scores` return the base linear memory address of the array.
  - `&p[i]` and `&raw p[i]` compute `base + index * elemSz` without loading.

---

## 3. Verification & Test Evidence

1. **`pkg2/lone-analyzer/analyzer_test.go`**:
   - `TestSinglePointerIndexingRejected`: Verified rejection of `p[0]` on `ptr<int>`.
   - `TestSizedPointerConstantBoundsCheck`: Verified compile-time rejection of `p[6]` on `ptr<[6]int>`.
   - `TestUnsizedBufferPointerAllowed`: Verified zero diagnostics on `ptr<[]int>`.
   - `TestSizedPointerDecayToUnsized`: Verified safe decay `ptr<[6]int>` $\rightarrow$ `ptr<[]int>`.
   - `TestRefBufferIndexingAndSingleRefRejection`: Verified acceptance on `ref<[4]int>` / `ref<[]int>` and rejection on `ref<int>`.
   - `TestReadOnlyBufferWriteRejection`: Verified immutability rejection on `ptr<ro []int>` and `ref<ro []int>`.
2. **`pkg2/lone-compiler-wir/wir_test.go` (Wasmtime Execution)**:
   - `TestSizedBufferPointerExecutionAndDynamicTrap`: Verified in-bounds dynamic read returns 30; out-of-bounds dynamic access traps via `unreachable`.
   - `TestUnsizedBufferPointerExecution`: Verified raw memory indexing returns 888.
   - `TestSizedPointerDecayExecution`: Verified safe decay argument passing returns 600.
3. **Full Suite**: 100% pass across all 22 packages (`go test ./...`).
