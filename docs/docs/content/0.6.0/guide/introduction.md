---
title: "Introduction"
order: 1
---

# Introduction to Lone (v0.6.0)

**Lone** is an explicit, systems- and UI-oriented programming language designed from the ground up to compile directly to **WebAssembly (Wasm)** with zero runtime overhead as its primary target. It takes inspiration from Go, TypeScript, Python, and Rust/Zig.

## Core Philosophy & Key Invariants

> *"Easy to read, Easy to write, and Hard to misunderstand."*

- **IL1 - Explicit over implicit**: Every name reference, type indirection, and memory allocation must be unambiguous regarding *what it is* and *where it came from*.
- **IL2 - No implicit built-ins**: Compiler intrinsics are prefixed with `#` (e.g. `#panic`, `#print`, `#alloc`, `#free`) to cleanly distinguish them from user code.
- **IL3 - Strict Resource & Result Accounting**: Non-void expressions in statement position must be consumed by a binding or explicitly dropped (`drop <expr>`). Unconsumed values trigger a compile-time static analysis error (`type/unused-result`).
- **IL4 - Explicit Trait Materialization**: Traits are pure contracts and do not inject methods into type namespaces. Trait methods must be explicitly materialized via `TrTrait::|TargetType|`.
- **IF1 - Syntax-level feedback**: Grammar layout and keywords provide readability without cryptic punctuation.
- **IF2 - Progressive information**: Information flows progressively from left to right wherever possible.

---

## Compiler Architecture (v0.6.0)

The Lone toolchain implements a zero-dependency, high-performance architecture:

```
[Lone Source (.lone)]
         │
         ▼  (pkg/lone-lexer & lone-parser / Pratt precedence)
     [Raw AST]
         │
         ▼  (pkg/lone-project / multi-file resolver)
 [Resolved AST Graph]
         │
         ▼  (pkg/lone-analyzer / multi-pass analysis)
  [Typed IR (TProgram)]  <── Validates types, interface contracts, and drop rules
         │
         ▼  (pkg/lone-compiler-wir / WIR lowering & monomorphization)
 [WebAssembly (WAT / Wasm)]
         │
         ├── Page 0: Downward Datastack (Shadow Stack) + Constants
         └── Page 1+: Upward Heap + Free-List Allocator ($__lone_rt_alloc)
```

---

## Key Features in v0.6.0

- **Asymmetric Field Visibility (`ro(pub)`)**: Physical struct fields are publicly accessible as read-only outside the declaring module, while remaining fully mutable internally.
- **Formal Interface Contracts (`interface`)**: Compile-time duck-typing and structural contracts that decoupled callers from concrete implementations.
- **Canonical Struct Constructors (`__init__`)**: Direct construction syntax `Type::(...)` desugars into invocations of `pub fn __init__(...) Type`.
- **Reference & Pointer Semantics**:
  - Safe typed references `ref<T>` with immutability by default and `ref<var T>`.
  - Raw pointers `ptr<T>` for low-level memory operations and sized array pointers `ptr<[N]T>`.
  - Fat slice references `ref<[]T>` encapsulating pointer and length.
- **Formatted String Interpolation (`f"..."`)**: Compile-time desugared format strings with `${...}` expressions and `TrDisplay` trait integration.
- **Grouped Variable Declarations**: `[a, b]: Type` syntax for declaring multiple variables cleanly.
- **Tagged Union Enums**: Full algebraic data types with variant payloads, discriminant tagging, and standard types like `Option<T>` and `Result<V, U>`.
- **Exhaustive Pattern Matching**: First-class `match` expressions with pattern destructuring and compiler-checked exhaustiveness.
- **Error Handling with `must` & `try`**:
  - `must <expr>`: Unwraps `Result::Ok` or panics via WebAssembly `unreachable`.
  - `try <expr>`: Propagates `Result::Err` up the call stack with early return.
- **Parametric Polymorphism & Constraints**: Generics with `where` constraint blocks using `satisfies` (trait contract) and `is` (type match), plus `infer` escape hatches.
- **Linear Memory & Free-List Allocator**: Two-tier memory model: Page 0 downward datastack frame allocation alongside Page 1+ dynamic heap managed by an inlined free-list allocator with bi-directional coalescing and `memory.grow`.
- **Full LSP Protocol Daemon (`lone lsp`)**: JSON-RPC 2.0 language server providing diagnostics, hover tooltips, definition jumps, and document symbols.
