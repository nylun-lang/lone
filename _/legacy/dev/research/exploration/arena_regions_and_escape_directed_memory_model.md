# Exploration: Memory Model Synthesis — Escape-Directed Lexical Destruction & First-Class Arenas

**Date**: 2026-11-06  
**Status**: Draft / Exploratory  
**Author**: Human & AI Pair  
**Location**: `dev/research/exploration/arena_regions_and_escape_directed_memory_model.md`  

---

## 1. Executive Summary

Lone occupies a distinct and deliberate position in programming language design: **it is neither a tracing garbage-collected language nor a full reference-counting language**.

- **Tracing GC** (Go, Java, JS) is rejected: it introduces non-deterministic latency, GC pauses, binary bloat, and breaks direct, transparent control over WebAssembly linear memory (**IL1**).
- **Full ARC** (Swift, Python, Nim/Mojo baseline) is rejected: atomic/non-atomic counter churn on every assignment, copy, or parameter pass destroys WebAssembly throughput, while cyclic references require cumbersome `weak` pointer handling.

This research exploration synthesizes two complementary models into a unified, zero-overhead memory architecture for Lone:

1. **Escape-Directed Lexical Destruction (Speed 2)**: For individual objects and dynamic collections, non-escaping values reside on the Page 0 datastack, while escaping values are promoted to the Page 1+ Managed Store. Because Lone enforces strict single-ownership value semantics, the compiler statically inserts deallocation (`$__lone_rt_free`) at the exact lexical point where the owner variable exits scope (or when explicitly `drop`-ped).
2. **First-Class Arenas / Regions (Speed 3)**: For high-frequency, complex object graphs (UI component trees, ASTs, game tick loops, network requests), allocations are bound to an explicit or scoped Arena. Allocations use $O(1)$ bump-pointers, and the entire memory region is reclaimed in $O(1)$ when the arena/region scope ends.

Crucially, **Lone's strict value semantics (pass-by-value aggregate copying) solves the classic "Return Problem" of arenas**: a function can construct a 50 MB temporary object graph inside an arena, extract a final aggregate struct result, and return it *by value* to the caller, allowing the entire arena to be wiped in $O(1)$ immediately upon return.

---

## 2. The Architectural Reality: The Three Speeds of Memory

Lone organizes WebAssembly linear memory (`env.memory`) into a three-tier hierarchy balancing safety, determinism, and raw execution speed:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                       The Three Speeds of Lone Memory                       │
└─────────────────────────────────────────────────────────────────────────────┘

 Speed 1: Frame Stack (Page 0 Datastack)
 ┌───────────────────────────────────────────────────────────────────────────┐
 │ • Scope: Non-escaping structs, local variables with &x, transient strings │
 │ • Allocation: Subtraction on $__stack_pointer in function prologue        │
 │ • Deallocation: O(1) zero-cost pointer rewind in function epilogue        │
 │ • Cost: 0 heap allocations, 0 fragmentation, 0 runtime bookkeeping        │
 └───────────────────────────────────────────────────────────────────────────┘
                                   │
                                   │ (value escapes function scope)
                                   ▼
 Speed 2: Lexical Heap (Page 1+ Managed Store)
 ┌───────────────────────────────────────────────────────────────────────────┐
 │ • Scope: Individual escaping objects, dynamic collections, fallible errors│
 │ • Allocation: Free-List Allocator with splitting ($__lone_rt_alloc)       │
 │ • Deallocation: Compiler statically inserts $__lone_rt_free at the        │
 │   owner's lexical scope exit (or at explicit `drop <expr>`)               │
 │ • Cost: Deterministic per-object free, 0 retain/release counter churn     │
 └───────────────────────────────────────────────────────────────────────────┘
                                   │
                                   │ (high-throughput / batch / phase graphs)
                                   ▼
 Speed 3: First-Class Arenas (Bulk Regions)
 ┌───────────────────────────────────────────────────────────────────────────┐
 │ • Scope: UI component trees, parser ASTs, game frames, worker requests    │
 │ • Allocation: O(1) bump-pointer increment within designated memory block  │
 │ • Deallocation: O(1) bulk pointer rewind wiping thousands of objects at   │
 │   once without individual frees or header traversal                       │
 └───────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Speed 2: Escape-Directed Lexical Destruction

### 3.1 Value Semantics & Single Ownership
In Lone, values have single ownership by default:
- Assigning `let b = a` or passing `process(a)` passes aggregates by value (shallow copy of struct fields and scalars).
- Managed references (`ref<T>`) are strictly **non-owning borrows**:
  - Guaranteed non-null and lifetime-bounded.
  - Cannot outlive the underlying value.
  - **Never modify reference counts** because they do not participate in ownership.

### 3.2 Escape Analysis & Promotion
When a local value or struct is constructed:
1. **Local-Only**: If the compiler determines that neither the value nor its reference escapes the local function, it is placed on the **Page 0 Datastack**.
2. **Escaping**: If the value is returned from the function, stored into a caller-provided reference (`out.item = val`), or captured by an escaping closure, the compiler automatically rewrites its allocation to the **Managed Store** (`$__lone_rt_alloc`).

### 3.3 Static Scope-Exit Deallocation (Auto-Drop)
Because the receiving variable in the caller becomes the sole owner of the promoted heap slot, the compiler statically schedules its deallocation:

```lone
fn build_session(user_id: i32) UserSession {
    -- Escapes build_session -> Promoted to Managed Store ($__lone_rt_alloc)
    return UserSession::(user_id, "token_xyz")
}

pub fn main() void {
    let session = build_session(42)
    println_int(session.user_id)

    -- <-- End of main scope
    -- Compiler statically inserts: $__lone_rt_free(session._ptr)
}
```

### 3.4 Integration with `drop` (Invariant IL3)
Lone already requires non-void expressions in statement position to be consumed or explicitly dropped:
```lone
pub fn handle_request() void {
    let temp_data = fetch_payload()
    process(&temp_data)
    
    -- Explicit early reclamation:
    drop temp_data
    -- Compiler immediately emits $__lone_rt_free(temp_data._ptr) here!
    
    perform_long_running_task()
}
```
Explicit `drop` provides deterministic, fine-grained reclamation without waiting for the closing brace.

---

## 4. Speed 3: First-Class Arenas (Bulk Regions)

While Escape-Directed Auto-Drop is ideal for individual long-lived objects, it exhibits poor performance characteristics for **complex, short-lived object graphs**:
- Compiling an AST with 50,000 nodes requires 50,000 calls to `$__lone_rt_alloc` and 50,000 individual calls to `$__lone_rt_free`.
- Allocator headers (8 bytes each) introduce 400 KiB of metadata overhead.
- Heap fragmentation occurs when nodes are freed in non-contiguous orders.

### 4.1 Arena Mechanics
An **Arena** pre-allocates a contiguous slice of linear memory (or dynamically grows pages on demand):
- **Allocation**: Adding $N$ bytes to the arena's `offset` pointer ($O(1)$).
- **Deallocation**: Resetting `offset` back to `0` or its starting mark ($O(1)$).
- **Zero Header Overhead**: Individual nodes require no 8-byte status/size headers.

### 4.2 The Return Problem: Solved by Value Semantics
A notorious problem in arena-based languages (such as C or Zig) is **"The Return Problem"**: how can a function allocate a large temporary graph in an arena, but return a final result to a caller whose lifetime outlives that arena?

In C or Zig, returning an arena-allocated object either:
1. Requires the caller to keep the entire arena alive indefinitely.
2. Requires manual deep-cloning into a secondary allocator.

In Lone, **structs and tuples are pure value types**:

```lone
pub type Config = struct {
    host: str,
    port: i32,
}

fn load_configuration(yaml_source: str) Config {
    arena {
        -- 10,000 temporary nodes allocated on the bump arena:
        let tokens = tokenize(yaml_source)
        let ast = parse_yaml_ast(tokens)
        let resolved = validate_schema(ast)
        
        -- RETURN BY VALUE:
        -- Lone copies the 16-byte `Config` struct out to the caller's stack frame!
        return Config {
            host: resolved.host,
            port: resolved.port,
        }
    }
    -- <-- Exiting the `arena` block wipes the entire 10,000-node graph in O(1)!
    -- The caller safely receives the `Config` copy with ZERO dangling references.
}
```

---

## 5. Ergonomic Models for Arenas in Lone

We explore three syntax models for integrating Arenas into Lone:

### Model A: Scoped Lexical Block (`arena { ... }`)
The simplest and most declarative approach:

```lone
pub fn render_frame(scene: Scene) void {
    arena {
        let vdom = generate_ui_nodes(scene)
        let draw_calls = layout_and_batch(vdom)
        gpu_submit(draw_calls)
    } 
    -- Bump pointer reset to entry mark in O(1)
}
```
- **Pros**: Zero boilerplate; impossible to forget resetting or freeing the arena.
- **Cons**: Allocations inside the block implicitly redirect to the arena unless explicitly designated otherwise.

### Model B: First-Class `Arena` Type with Explicit Passing
Explicit and systems-oriented (**IL1 - Explicit over implicit**):

```lone
import .Arena from :std:sys

pub fn main() void {
    let frame_arena = Arena::(64 * 1024) -- 64 KiB buffer
    
    loop {
        let event = poll_input()
        process_event(&frame_arena, event)
        frame_arena.reset() -- O(1) bulk rewind for next tick
    }
}
```
- **Pros**: Completely transparent; caller controls buffer size, reuse, and lifetime.
- **Cons**: Requires passing `&arena` as a parameter to participating functions.

### Model C: Contextual Capability (`using arena`)
Functions declare that they operate within an active arena context without parameter clutter:

```lone
fn parse_node(source: str) Node using arena {
    return Node { ... } -- allocated in the ambient arena
}

pub fn main() void {
    with arena {
        let root = parse_node("root")
    }
}
```

---

## 6. Borrowing Rules Across Scopes & Arenas

To maintain safety without introducing complex lifetime annotations:

1. **Stack References Cannot Escape**:
   `&x` of a Page 0 stack variable cannot be returned or stored in a longer-lived struct (`borrow/stack-use-after-free`).
2. **Arena References Cannot Outlive the Arena**:
   A reference `ref<T>` pointing to data within an arena block cannot escape outside that block:
   ```lone
   var leaked: ?ref<Node> = none
   arena {
       let node = Node::()
       leaked = &node -- COMPILE ERROR: cannot assign arena borrow to outer scope!
   }
   ```
3. **Values Always Can Escape**:
   Passing or returning by value (`T`) is **always legal** because it copies the aggregate data across the boundary.

---

## 7. Comparative Analysis with Other Languages

| Feature | Tracing GC (Go/JS) | Full ARC (Swift/Python) | Rust | Zig | **Lone (Speed 2 + Speed 3)** |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Runtime Pauses** | Yes (non-deterministic) | No | No | No | **No (0 pauses)** |
| **Reference Churn** | None | High (atomic counters) | None | None | **None (0 counter updates)** |
| **Cyclic Leaks** | Handled by GC | Yes (requires `weak`) | No cycles without `Rc` | Manual | **Impossible for values/borrows** |
| **Cognitive Friction** | Lowest | Low | Highest (lifetimes `'a`) | Medium (manual free) | **Low (Value semantics + Auto-drop)** |
| **Bulk Phase Wipe** | No (individual sweep) | No (cascading dec) | Arena crates | First-class arenas | **First-class O(1) Arenas** |
| **WebAssembly Fit** | Poor (large runtime) | Moderate (code bloat) | Excellent | Excellent | **Optimal (Wasm native)** |

---

## 8. Roadmap & Next Steps

1. **Step 1: Escape-Directed Lexical Destruction in Analyzer**:
   - Enhance `pkg/lone-analyzer` to track scope exits for owned heap values.
   - Emit `$__lone_rt_free` calls in `pkg/lone-compiler-wir` for promoted heap variables upon scope exit and explicit `drop`.
2. **Step 2: Core Arena Implementation in `:std:sys`**:
   - Implement `Arena` struct backed by dynamic Page 1+ linear memory chunks.
   - Provide `alloc(size: i32) ptr<u8>` and `reset() void` intrinsics.
3. **Step 3: Syntax Exploration RFC**:
   - Draft formal RFC comparing Model A (`arena { ... }`) vs Model B (`Arena::()`) for community and user review.
