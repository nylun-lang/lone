
## 5. Runtime Semantics: Memory Model

### 5.1 Architecture Overview

Lone splits memory execution into two cooperating tiers:

1. **The Linear Datastack (Bump-Allocated Scratchpad):** Per-task/per-thread linear memory slabs where local variables, intermediate buffers, and callee scopes live.

2. **The Managed Store (Anchored Objects):** A compiler-managed heap tier where values that escape their defining scopes reside under **Explicit Anchors** (`ref<T>`).

This architecture avoids both stop-the-world tracing GC pauses and the pervasive fragmentation overhead of traditional dynamic allocators (`malloc`/`free`).

### 5.2 The Datastack (Scope-Bound Allocation)

By default, every value declared within a function or block is allocated on the current execution thread's **datastack**.

#### 5.2.1 Bump Allocation Mechanics

- A datastack maintains a single monotonic pointer: `stack_top`.
- Instantiating arrays, records, or local structs simply increments the pointer:
  $$\text{stack\_top} \mathrel{+}= \text{aligned\_sizeof}(\text{T})$$
- **Zero Allocation Cost:** Allocations require neither free-list traversals nor lock contention.

#### 5.2.2 Frame Reset & Callee Promotion

When a function `callee` returns:

- **Discarded Locals:** If local data does not escape, `stack_top` rewinds to the caller frame's base boundary, reclaiming all temporary scratchpad memory in $O(1)$.

- **Direct Value Return (Append to Caller):** When a value is returned by value (such as a structural record or primitive), the compiler arranges for the bytes to be positioned directly at the caller's stack boundary, automatically adopting it into the caller's datastack.


### 5.3 Anchoring (Escaping Lifetimes)

An **Anchor** is an active constraint recognized by Falone that prevents an object from being reclaimed when an execution scope unwinds.

#### 5.3.1 Implicit Anchors (Stack Lifetime)
An implicit anchor is an ephemeral lease owned strictly by the lexical block that declared the value.

```lone
fn process() {
  let temp = [1, 2, 3, 4]
  -- `temp` is implicitly anchored to `process`.
  -- Upon return, `stack_top` rewinds; `temp` is instantly discarded.
}
```

#### 5.3.2 Explicit Anchors (`ref<T>`)

An explicit anchor is an active, compiler-tracked reference handle (`ref<T>`).
Whenever a reference to a locally declared value escapes the lexical boundary (via return, closure capture, or cross-thread dispatch), Falone's escape analyzer promotes the value out of the temporary datastack and into the **Managed Store**:

```lone
fn generate_buffer() ref<[6]int> {
  let data = [1, 2, 3, 4, 5, 6]
  
  -- Compiler Escape Promotion:
  -- Falone detects `&data` escapes the frame.
  -- `data` is promoted to the Managed Store with an explicit anchor count of 1.
  return &data
}

```

#### 5.3.3 Deallocation Invariant

- **Invariant M1 (Reclaim Rule):** When an object has zero active anchors (all explicit `ref<T>` handles dropped and enclosing lexical scope exited), its memory is immediately reclaimed.
- **Invariant M2 (Reference Validity):** An explicit reference `ref<T>` is guaranteed never to point to a rewound datastack address.

---

### 5.4 Closure Captures & Concurrency

When closures capture variables from their defining context, Falone determines the storage strategy based on escape boundaries:

1. **Down-Funarg / Non-Escaping Closures:**
If a closure is consumed entirely within the current stack lifetime (e.g., passed to an inline loop, predicate, or mapping function), it borrows values directly from the datastack with **zero heap promotion**.
2. **Up-Funarg / Escaping Closures:**
If a closure is returned from a function or dispatched to a background thread/task:
- Any implicitly anchored variable referenced by the closure is promoted to the Managed Store.
- The closure’s synthesized capture context acquires an **explicit anchor** (`ref<T>`).
- Even if the defining function returns immediately, the captured data remains valid for the closure's entire execution lifespan across any thread.

### Key Wins for Falone

1. **High Throughput in Loops & Functions:** Algorithmic routines (like `binary_search` or parsing) generate dozens of intermediate lists/records that sit on the datastack and vanish with zero runtime cost.
2. **Seamless UI Integration:** Loum components[cite: 2, 3] and reactive `state` units[cite: 2, 3] naturally live via explicit anchors in the managed tier, while high-frequency render passes run their temporary scratch computations on the thread's datastack.
3. **WASM-Friendly:** WebAssembly's linear memory model maps directly to linear datastack bumping (`i32.add` on a global offset), making compiled Lone binaries small and fast.