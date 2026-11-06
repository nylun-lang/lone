# The Lone Programming Language

<img src="./assets/lone-banner.png" alt="Lone Logo">

[![Build Status](https://img.shields.io/badge/version-v0.6.0-blue.svg)](./dev/SPEC.md)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen.svg)](./test)

**Lone** is an explicit, systems- and UI-oriented programming language designed from the ground up to compile directly to **WebAssembly (Wasm)** with zero runtime overhead as its primary target. It takes inspiration from Go, TypeScript, Python, and Rust/Zig.

> *"Easy to read, Easy to write, and Hard to misunderstand."*

---

## Key Invariants

- **IL1 - Explicit over implicit**: Every name reference, type indirection, and memory allocation is unambiguous regarding what it is and where it came from.
- **IL2 - No implicit built-ins**: Compiler intrinsics are prefixed with `#` (e.g. `#print`, `#panic`, `#alloc`, `#free`) to cleanly separate language primitives from user code.
- **IL3 - Strict Resource & Result Accounting**: Non-void expressions in statement position must be bound or explicitly dropped (`drop <expr>`). Unconsumed results trigger static analysis errors (`type/unused-result`).
- **IL4 - Explicit Trait Materialization**: Traits are pure contracts and do not inject methods into type namespaces. Trait methods are explicitly materialized via `TrTrait::|TargetType|`.
- **IF1 - Syntax-level feedback**: Grammar layout and keywords provide readability without cryptic punctuation.
- **IF2 - Progressive information**: Information flows progressively from left to right wherever possible.

---

## Language Highlights (v0.6.0)

```lone
-- Import I/O utilities
import .println, .println_int from :std:io

-- Tagged union enums with variant payloads
pub type MathOp = enum {
    Add(a: int, b: int),
    Multiply(a: int, b: int),
    Negate(x: int),
    Zero,
}

-- Exhaustive pattern matching
fn evaluate(op: MathOp) int {
    return match op {
        MathOp::Add(a, b) => a + b,
        MathOp::Multiply(a, b) => a * b,
        MathOp::Negate(x) => 0 - x,
        MathOp::Zero => 0,
    }
}

-- Fallible functions using Result<V, U>
fn safe_divide(numerator: int, denominator: int) Result<int, str> {
    if denominator == 0 {
        return Result::Err("Division by zero")
    }
    return Result::Ok(numerator / denominator)
}

-- Formal interface data contract
pub type IfcNamed = interface {
    ro name: str
}

-- Struct with asymmetric visibility: internally mutable, externally read-only
pub type User = struct {
    ro(pub) name: str
    pub age: int

    pub fn __init__(name: str, age: int) User {
        return User { name: name, age: age }
    }
}

pub fn main() void {
    let u = User::("Alice", 30)
    println(f"User ${u.name} initialized")

    let op = MathOp::Add(20, 22)
    let total = evaluate(op)
    
    -- Error handling: 'must' unwraps Ok or panics with Wasm unreachable
    let quotient = must safe_divide(total, 2)
    println_int(quotient) -- 21
}
```

- **Asymmetric Field Visibility (`ro(pub)`)**: Expose fields as externally read-only while retaining internal mutability within the defining module.
- **Formal Interface Contracts (`interface`)**: Structural duck-typing at compile time for decoupled contracts.
- **Tagged Union Enums**: Full algebraic data types (`Option<T>`, `Result<V, U = Error where { U is Error }>`).
- **Pattern Matching (`match`)**: Destructuring payload bindings with compiler-verified exhaustiveness.
- **Error Handling with `must` & `try`**:
  - `must <expr>`: Unwraps `Result::Ok` or immediately panics via WebAssembly `unreachable`.
  - `try <expr>`: Propagates `Result::Err` up the call stack with early return.
- **Reference & Pointer Semantics**: Safe typed references `ref<T>` with immutability by default, raw pointers `ptr<T>` for low-level memory operations, and fat slice references `ref<[]T>`.
- **Parametric Polymorphism & Constraints**: Generics with `where` constraint blocks using `satisfies` (trait contract) and `is` (type match), plus `infer` escape hatches.
- **Formatted String Interpolation**: Compile-time desugared `f"Hello ${name}!"` format strings.
- **Explicit Trait Materialization**: `TrTrait::|T|` syntax with mandatory `Tr` prefix naming convention.
- **Linear Memory & Free-List Allocator**: Two-tier memory model: Page 0 downward datastack frame allocation alongside Page 1+ dynamic heap managed by an inlined free-list allocator with bi-directional coalescing and `memory.grow`.
- **Modern Control Flow**: Pre-condition `while`, infinite `loop`, post-condition `loop { ... } while: [cond]` with smart variable scoping, and multi-branch packed `if`.

---

## Getting Started

### Prerequisites
- **Go 1.22+**
- **Wasmtime** (for executing `.wat` / `.wasm`)

### Building the Toolchain
```bash
git clone https://github.com/lone-lang/lone.git
cd lone

# Build the Lone compiler and CLI
go build -o lone ./cmd/lone
```

### Checking, Compiling & Running
```bash
# Static analysis and type checking
./lone check program.lone

# Compile to standalone WebAssembly Text (.wat)
./lone build program.lone -o program.wat

# Immediate in-process execution with Wasmtime
./lone run program.lone
```

---

## Toolchain & CLI Commands

- `lone build <file.lone> [-o out.wat]`: Compile source into WebAssembly WAT.
- `lone run <file.lone>`: Compile and execute immediately in Wasmtime.
- `lone check <file.lone>`: Analyze file for semantic and type correctness.
- `lone parse <file.lone>`: Parse file using Pratt parser and verify syntax.
- `lone debug <file.lone> [symbol] [--json]`: Inspect struct memory layout, interfaces, enums, and functions.
- `lone lsp`: Launch Language Server Protocol daemon for editor integrations.
- `lone version`: Display Lone compiler version.

---

## Documentation

Full documentation is available under `docs/`:
- **Language Specification**: [`dev/SPEC.md`](./dev/SPEC.md)
- **Documentation Website**: Run `npm run docs:dev` inside `docs/` to browse the interactive VitePress documentation.
- **Internal RFCs & Architecture**: Check [`dev/rfcs/`](./dev/rfcs/) and [`dev/internals/`](./dev/internals/).

---

## License

Lone is open source software licensed under the [MIT License](./LICENSE).
