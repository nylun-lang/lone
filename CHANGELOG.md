# Changelog

All notable changes to the Lone language project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.6.0] - 2026-11-06

### Added
- **Greenfield Toolchain Promotion**:
  - Promoted the Pratt-based, zero-allocation compiler pipeline (`pkg2/`, `cmd2/`, `lib2/`, `test2/`) to primary repository root (`pkg/`, `cmd/`, `lib/`, `test/`).
  - Retired legacy Pigeon PEG parser, MIR transpiler, and C runtime stubs (`libraries/c-wasi-runtime`).
- **Asymmetric Field Visibility (`ro(pub)`)**:
  - Added `ro(pub)` modifier to struct definitions: fields are publicly accessible as read-only outside the declaring module, while retaining full read-write mutability within module methods and functions.
  - Compile-time error prevention when attempting external field assignments to `ro(pub)` fields.
- **Formal Interface Contracts (`interface`)**:
  - Introduced `interface` types as structural data contracts for static type checking and compile-time duck-typing.
  - Interfaces enforce required field types and read-only constraints (`ro`).
- **Struct Construction & Initialization (`__init__`)**:
  - Direct construction syntax `Type::(...)` desugars into invocations of `pub fn __init__(...) Type`.
  - Static compile error when attempting `Type::(...)` on structs without an explicit `__init__` constructor.
- **Reference & Pointer Semantics**:
  - Typed references `ref<T>` with immutability by default and mutable reference bindings `ref<var T>`.
  - Unsafe raw pointers `ptr<T>` with explicit pointer dereferencing and indexing.
  - Fat slice references `ref<[]T>` encapsulating pointer and length.
  - By-value aggregate struct copies across assignment and parameter passing.
- **Formatted String Interpolation (`f"..."`)**:
  - String interpolation syntax `f"Hello ${name}!"` desugared at compile time into sequential buffer writes.
  - Integration with `TrDisplay` trait for custom object string formatting.
- **Grouped Variable Declarations**:
  - Grouped declaration syntax `[a, b]: Type` for concise multiple variable declarations.
- **Standard Library I/O Hardening (`:std:io`)**:
  - Added `parse_int(s: str) Result<i32>` with `Result::Ok` and `Result::Err(Error::Generic)` return variants.
  - Added buffered line reading (`read_line`), terminal output (`print`, `println`), and WASI random number generation (`random_i32`, `random_range` in `:std:sys`).
- **Modernized Language Server Protocol (`pkg/lone-lsp`)**:
  - Fully implemented JSON-RPC 2.0 LSP server integrated with `lone-parser` and `lone-analyzer`.
  - Byte-accurate 0-indexed diagnostics, hover tooltips, definition lookup, and document symbol outlines.
  - Updated Neovim syntax (`extra/nvim-lsp`) and Open VSX / VS Code extension (`extra/ovsx-lsp`).
- **Enhanced CLI Subcommands (`cmd/lone`)**:
  - `lone debug <file> [symbol] [--json]`: Inspect struct memory layouts, byte offsets, interface requirements, and enum variants.
  - `lone lsp`: Launch background LSP daemon.
  - `lone run`: Direct in-process execution via Wasmtime.

---

## [0.5.0] - 2026-11-04

### Added
- **Greenfield Compiler Architecture**:
  - Hand-crafted zero-allocation Lexer and $O(N)$ Pratt operator precedence parser replacing PEG-generated parser.
  - Unified `lone-analyzer` with single-pass arena scopes, struct memory layout calculation, and Typed IR generation.
  - Direct `lone-compiler-wir` code generator with linear scan register allocation and human-readable WebAssembly Text (`.wat`) emission.
- **Algebraic Data Types & Tagged Union Enums**:
  - `pub type Name = enum { Variant(payload), Empty }` with payload-bearing variants and discriminant tagging.
  - Discriminant testing operator `val is Variant`.
- **Pattern Matching (`match`)**:
  - Exhaustive pattern matching on enums, literals, and wildcards (`_`).
  - Scoped variable destructuring in `if` and `while` condition headers (`if let Variant(x) = expr`).
- **Error Propagation Operators (`must` & `try`)**:
  - Built-in `Result<V, U = Error where { U is Error }>` and `Option<T>`.
  - `must <expr>`: Panics with WebAssembly `unreachable` on `Err`/`None`.
  - `try <expr>`: Propagates `Err` with early return up the call stack.
- **Parametric Polymorphism & Generic Constraints**:
  - Generic structs, enums, and functions (`fn map<T, R>(val: T) R`).
  - `where` constraint blocks with `satisfies` (trait contract) and `is` (exact type match).
  - Whole-program monomorphization pass specializing generic AST nodes.
- **Linear Memory Management**:
  - Two-tier memory model: Page 0 downward datastack and Page 1+ dynamic heap.
  - Free-list allocator with bi-directional coalescing and dynamic `memory.grow`.
- **Control Flow Enhancements**:
  - Post-condition loops: `loop { ... } while: [condition]`.
  - Labeled block expressions with value breaks: `let x = 'blk: { ... break 'blk val }`.
  - Explicit type casts using `as` keyword (`x as i64`).

---

## [0.4.0] - 2026-11-03

### Added
- **Module Resolution & Dependency Graph (`lone-project`)**:
  - Multi-file project resolver with topological sorting and circular dependency detection.
  - Namespace imports (`import .symbol from :std:io`).
- **Self-Contained Standard Library (`lib/std`)**:
  - Implemented `:std:io`, `:std:sys`, and `:std:string` in pure Lone.
  - Direct WASI Preview 1 syscall bindings via `@fa:wasm` imports.
- **Language Soundness & Specification Alignment**:
  - Formalized memory invariants and stack/heap collision boundaries.
  - Comprehensive edge-case audit suite.

---

## [0.3.0] - 2026-11-02

### Added
- **Architecture v2 Restructuring**:
  - Reorganized internal packages into modular packages under `pkg/`.
  - Structured error logging with source excerpts and caret diagnostics (`lone-shared-logger`).
  - Mid-level IR (MIR) improvements and WebAssembly Text (WIR) generator refinements.
- **Trait Contracts & Materialization**:
  - Trait declaration syntax and explicit materialization syntax (`TrTrait::|Target|`).

---

## [0.2.0] - 2026-11-01

### Added
- **Falone Compiler & CLI (`packages/falone`)**:
  - Implemented Falone Mid-level Intermediate Representation (MIR) with control flow graphs, basic blocks, and virtual registers.
  - Implemented Typed IR lowering to Falone MIR.
  - Implemented WebAssembly Text (`.wat`) transpiler with linear memory management and structured loops.
  - Added Cobra CLI with `compile`, `compile wasm`, `compile web`, and `analyze` subcommands.
  - Added `--with-adapter <node | browser>` flag generating JavaScript runtime loader glue.
  - Added standalone WASI preview 1 support with inlined `$__lone_rt_print` and `fd_write` syscall.
  - Added Web browser target with `(import "env" "print" ...)`.
  - Added dual entrypoint exports `(export "_start")` and `(export "main")`.
- **Static Analysis & Type Checking (`packages/lone-alyzer`)**:
  - Implemented multi-pass analyzer: Naming Conventions, Type Inference, Return Verification, and Consume-or-Drop rule.
  - Added Typed IR (`air`) definitions and AST-to-Typed-IR transformation.
  - Pre-registered `#print(str: str) void` intrinsic in scope stack.
  - Added CLI for AST pretty-printing, Typed IR output, and JSON diagnostic exports.
- **Parser & Language Features (`packages/lone-lang`)**:
  - Added fixed-size list type syntax `[N]T` and list literals.
  - Added function declaration syntax `fn name(params) return_type`.
  - Added control flow: `if`/`else`, `while` loops, labeled blocks, and `break` with values.
  - Added `drop <expr>` statement syntax.
  - Added `#` intrinsic prefix support in `NAME_REF` expressions.
  - Added string escape sequence handling (`\n`, `\r`, `\t`, `\"`, `\\`).
- **Documentation Platform (`docs/`)**:
  - Configured VitePress static site generator.
  - Authored comprehensive guides for v0.2.0 and historical v0.1.0 documentation under `content/<sem-ver>/`.
  - Added recursive sidebar generator (`docs/scripts/gen-sidebar.js`) with semver ordering and highest-version-as-latest syncing.
  - Added branded hero home page and logo assets.

---

## [0.1.0] - 2026-11-01

### Added
- Initial project structure with PEG parser for the Lone programming language.
- Primitive types (`i8`..`i128`, `u8`..`u128`, `f32`, `f64`, `bool`, `str`, `void`, `none`).
- Basic variable definitions (`var x: T = val`).
- Early AST definitions and JSON parser CLI.
