# Research Dilemma: Unsafe Pointer Indexing (`ptr<T>[i]`)

In systems language design—and specifically in Lone’s distinction between safe references (`ref<T>`) and unmanaged raw pointers (`ptr<T>`)—allowing array-style indexing on raw pointers (`p[i] = val`) exposes a classic foundational dilemma: **Low-Level Systems Ergonomics vs. Perceptual Safety Guarantees**.

---

### 1. The Core Tension: The "Innocent Syntax" Trap

When a developer sees `container[i]`, modern language intuition (from Python, Go, Rust, TypeScript) assumes **spatial safety**: either the collection knows its length and panics on out-of-bounds access, or the compiler statically verifies the index range.

With raw pointers (`ptr<T>`), `p[i]` is a thin pointer:
$$\text{Address} = \text{base} + (i \times \operatorname{sizeof}(T))$$

It possesses **zero runtime bounds metadata**. 

#### The Dilemma
* **Option A (Allow `p[i]` directly)**: Maximizes ergonomics for systems programming (allocators, SIMD buffers, framebuffers, bytecode interpreters). However, it introduces an **asymmetric visual risk**: safe, bounds-checked slice indexing and memory-corrupting raw indexing look 100% identical at the call site.
* **Option B (Forbid `p[i]`, require explicit pointer arithmetic)**: Forces developers to write `*(p.offset(i))` or `*(p + i)`. While this destroys the illusion of safety and makes danger explicit, it makes low-level memory loops cumbersome, noisy, and prone to manual arithmetic mistakes.
* **Option C (Require an explicit unsafe marker)**: E.g., `unsafe { p[i] }` or distinct syntax like `p.[i]`. This solves the perceptual issue but adds syntax fragmentation and ceremony.

---

### 2. The WebAssembly Sandbox Hazard: Silent Corruption vs. Hardware Traps

In native environments (x86_64 / ARM with MMUs), wild pointer indexing past page boundaries frequently triggers a hardware `SIGSEGV` or OS trap, surfacing bugs immediately.

In **WebAssembly Linear Memory**, the stakes are radically different:
1. **Flat Unsegmented Address Space**: Wasm linear memory is a contiguous byte array starting from index `0`. Page 0 holds globals and the datastack frame, while higher addresses host dynamic heap memory (`#alloc`).
2. **Silent Corruption**: Indexing `p[i]` beyond `p` does not crash the process unless it crosses the total allocated memory page limit (`memory.size`). Instead, it silently overwrites:
   - Adjacent stack frames (corrupting local variables of callers).
   - Heap allocation headers (breaking free lists).
   - Global variable tables.
3. **No Hardware Canaries**: Pure Wasm engines execute sandboxed within host runtimes, meaning out-of-bounds raw writes can run indefinitely in a poisoned state before an inexplicable logical failure occurs.

---

### 3. Comparison of Prior Art & Language Solutions

| Language | Raw Indexing Syntax | Safety Mechanism | Trade-off / Cost |
| :--- | :--- | :--- | :--- |
| **C / C++** | `p[i]` | None (Undefined Behavior) | Rampant buffer overflows; zero visual warning. |
| **Rust** | `*p.add(i)` or `*p.offset(i)` | Forbidden outside `unsafe`; preferred type is slice `&[T]`. | Strict safety, but high syntactic overhead when implementing allocators or raw hardware buffers. |
| **Zig** | `p[i]` on `[*]T` (many-pointer) | Allowed on multi-pointers; single pointers `*T` disallow indexing. | Distinguishes single-item pointers `*T` from multi-item pointers `[*]T` in the type system. |
| **Go** | Disallowed on `*T` | Requires `unsafe.Pointer` + `uintptr` arithmetic. | High friction; language intentionally discourages raw memory access. |

---

### 4. The Architectural Trade-Off in Lone

In Lone's greenfield architecture:
1. **References (`ref<T>`)**: Safe, non-null, tracked by the compiler. Indexing is **strictly rejected** compile-time (`type/invalid-index`). If developers want indexing with references, they must use slices (`[]T`) or fixed arrays (`[N]T`).
2. **Pointers (`ptr<T>`)**: Unmanaged `i32` linear memory offsets. Lone permits `p[i]` strictly on `ptr<T>` for zero-cost performance in systems intrinsics, but enforces dereference rules (`*p = val`, `p[i] = val`).
3. **Future Consideration (The Zig-style Typestate Hierarchy)**: A potential evolution is splitting pointers into a 3-tier cardinality typestate:
   - `ptr<T>`: Single-element pointer (indexing strictly forbidden).
   - `ptr<[N]T>`: Sized array pointer (compile-time checked for constant indices, 1-instruction `ge_u N` dynamic check).
   - `ptr<[]T>`: Unsized raw buffer pointer (unbounded raw access).
   Ensures that an innocent single reference address cannot be indexed as an array without an explicit cast.

---

### 5. Formal RFC Proposal
This research exploration has been formalized into:
👉 [`dev/rfcs/drafts/RFC-20261105-unsafe_pointer_indexing_and_buffer_semantics.md`](../../rfcs/drafts/RFC-20261105-unsafe_pointer_indexing_and_buffer_semantics.md)
