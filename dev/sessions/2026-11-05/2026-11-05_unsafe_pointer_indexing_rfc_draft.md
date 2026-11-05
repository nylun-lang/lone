# Research & RFC Drafting Report: Unsafe Pointer Indexing & Typestate Buffers

**Date**: 2026-11-05  
**Scope**: Exploration Analysis & Formal RFC Drafting for Unsafe Pointer Indexing (`ptr<T>` vs `ptr<[]T>`)  
**Status**: Draft Completed & Under Review  
**Location**: `dev/sessions/2026-11-05/2026-11-05_unsafe_pointer_indexing_rfc_draft.md`  

---

## 1. Overview & Context

Prompted by research documented in [`dev/research/exploration/unsafe-ptr-indexing.md`](../../research/exploration/unsafe-ptr-indexing.md), reviewed the tension between low-level systems ergonomics and perceptual safety guarantees in pointer indexing (`p[i]`).

In WebAssembly's flat unsegmented linear memory sandbox, out-of-bounds pointer writes lack hardware MMU traps (`SIGSEGV`) and silently corrupt adjacent Page 0 stack frames or heap block headers. Allowing array-style indexing `p[i]` on an ordinary pointer to a single local stack variable (`let p = &raw x`) creates an "Innocent Syntax" trap.

---

## 2. Key Design Decisions in the Draft RFC

Formulated and authored [`RFC-20261105-unsafe_pointer_indexing_and_buffer_semantics.md`](../../rfcs/drafts/RFC-20261105-unsafe_pointer_indexing_and_buffer_semantics.md):

1. **Typestate Cardinality Separation**:
   - `ptr<T>`: Single-element raw pointer. Indexing `p[i]` is **statically forbidden** (`type/single-pointer-indexing-rejected`). Only dereference `*p` is allowed.
   - `ptr<[]T>`: Multi-element raw buffer pointer (unbounded slice in linear memory). Indexing `buf[i]` is **permitted** with zero runtime overhead.
2. **Explicit Intent & Transition**:
   - `&raw x` on a variable always produces `ptr<T>`.
   - Indexing requires an explicit cast: `p as ptr<[]T>`, guaranteeing intentionality.
3. **Safe Promotion (`#slice`)**:
   - Intrinsic `#slice(raw_buf, len)` promotes `ptr<[]T>` into a safe, bounds-checked dynamic slice `[]T`.
4. **Call-site Syntax Evaluation**:
   - Compared typestate standard brackets `buf[i]` against call-site markers (`p.[i]` and `p#[i]`). Concluded that typestate `ptr<[]T>` provides superior language cohesion and ergonomics while resolving the underlying safety hazard.

---

## 3. Documents Created & Updated

- **Draft RFC**: [`dev/rfcs/drafts/RFC-20261105-unsafe_pointer_indexing_and_buffer_semantics.md`](../../rfcs/drafts/RFC-20261105-unsafe_pointer_indexing_and_buffer_semantics.md)
- **Exploration Cross-Reference**: Linked from [`dev/research/exploration/unsafe-ptr-indexing.md`](../../research/exploration/unsafe-ptr-indexing.md)
- **Session Log**: Updated [`dev/sessions/2026-11-05.md`](../2026-11-05.md)
