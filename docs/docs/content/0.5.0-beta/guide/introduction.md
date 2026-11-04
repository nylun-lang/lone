---
title: "Introduction"
order: 1
---

# Introduction to Lone (v0.5.0-beta)

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

## Compiler Architecture (v0.5.0-beta)

The modern Lone toolchain implements a clean, high-performance Greenfield architecture:

```
[Lone Source (.lone)]
         │
         ▼  (pkg2/lone-lexer & lone-parser / Pratt precedence)
     [Raw AST]
         │
         ▼  (pkg2/lone-analyzer / multi-pass analysis)
  [Typed IR (TProgram)]  <── Validates types, drop rule, and naming rules
         │
         ▼  (pkg2/lone-compiler-wir / WIR lowering & monomorphization)
 [WebAssembly (WAT / Wasm)]
         │
         ├── Page 0: Downward Datastack (Shadow Stack) + Constants
         └── Page 1+: Upward Heap + Free-List Allocator ($__lone_rt_alloc)
```

---

## Key Features in v0.5.0-beta

- **Tagged Union Enums**: Full algebraic data types with variant payloads, discriminant tagging, and standard types like `Option<T>` and `Result<V, U>`.
- **Exhaustive Pattern Matching**: First-class `match` expressions with pattern destructuring and compiler-checked exhaustiveness.
- **Error Handling with `must` & `try`**:
  - `must <expr>`: Unwraps `Result::Ok` or panics via WebAssembly `unreachable`.
  - `try <expr>`: Propagates `Result::Err` up the call stack with early return.
- **Parametric Polymorphism & Constraints**: Generics with `where` constraint blocks using `satisfies` (trait contract) and `is` (type match), plus `infer` escape hatches.
- **Explicit Trait Materialization**: `TrTrait::|T|` syntax for unambiguous, zero-cost trait method dispatch. Trait names strictly enforce the `Tr` prefix convention.
- **Linear Memory & Free-List Allocator**: Two-tier memory model: Page 0 downward datastack frame allocation (zero-cost rewind) alongside Page 1+ dynamic heap managed by an inlined free-list allocator with bi-directional coalescing and `memory.grow`.
- **Modern Control Flow**: Pre-condition `while`, infinite `loop`, post-condition `loop { ... } while: [cond]` with smart variable scoping, and multi-branch packed `if`.
