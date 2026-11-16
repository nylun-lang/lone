# Compiler Package Architecture (v0.6.0)

Lone is implemented as an end-to-end, zero-dependency compiler pipeline targeting WebAssembly (WASI and Web), featuring a hand-crafted Lexer, Pratt expression parser, single-pass type analyzer, and direct WIR (WebAssembly Intermediate Representation) emitter with linear memory management and register allocation.

## Compiler Pipeline Subsystems

```
Source (.lone) ──► lone-lexer ──► lone-parser ──► lone-parser-ast
                                                       │
                                                       ▼
                                                 lone-project (Module Resolver)
                                                       │
                                                       ▼
                                                 lone-analyzer (Types & AST Lowering)
                                                       │
                                                       ▼
                                                 lone-compiler-wir (Linear Scan & WIR)
                                                       │
                                                       ▼
                                                 WebAssembly (.wat / .wasm)
```

### 1. `lone-lexer`
- **Role**: High-speed, zero-allocation token scanner.
- **Features**: Scans identifiers, keywords (`let`, `var`, `struct`, `interface`, `enum`, `match`, `where`, `satisfies`, `must`, `try`, `as`), numeric/hex/binary/float literals, byte-string and character literals, formatted strings `f"..."` with `${...}` expressions, and produces byte-accurate line/column/offset span positioning.

### 2. `lone-parser-ast`
- **Role**: Concrete and Abstract Syntax Tree data structures.
- **Features**: AST node representations for declarations, statements, expressions, type signatures, interfaces, and enums. Supports asymmetric visibility `ro(pub)`, generic type parameters, and labeled control flow.

### 3. `lone-parser`
- **Role**: Syntax parser combining recursive descent and Pratt operator precedence.
- **Features**: $O(N)$ top-down operator precedence for binary, unary, postfix, cast (`as`), and call expressions. Strict validation preventing invalid nested constructs, with clear and actionable syntax error recovery.

### 4. `lone-project`
- **Role**: Multi-file project resolver and dependency graph analyzer.
- **Features**: Resolves relative file imports (`import .symbol from "path"`), namespace imports (`:std:io`, `:std:sys`), and locates the Lone standard library (`lib/std`). Performs topological sorting and circular dependency detection.

### 5. `lone-analyzer`
- **Role**: Type evaluation, semantic checking, and Typed IR generation.
- **Features**:
  - Scope stack and lexical symbol tables.
  - Asymmetric struct field visibility: internal mutability with external `ro(pub)`.
  - Formal interface contracts (`interface`) with compile-time duck-typing / structural conformance.
  - Tagged union enums and pattern-matching exhaustiveness checks.
  - Generic monomorphization across functions, structs, and enums.
  - Error propagation mechanics (`Result<T, E>`, `must`, `try`).
  - Struct memory layout calculation (byte sizes, alignment, and field offsets).

### 6. `lone-compiler-wir`
- **Role**: WebAssembly Intermediate Representation (WIR), linear memory management, and code generator.
- **Features**:
  - Structured WebAssembly AST (blocks, loops, branches, calls, locals, globals).
  - Linear scan register allocator mapping virtual variables to Wasm locals.
  - Dynamic linear memory manager: bump allocator, free-list allocator, coalescing, and dynamic `memory.grow`.
  - Emits clean, human-readable WebAssembly Text (`.wat`).

### 7. `lone-compiler`
- **Role**: Compiler driver and coordinator.
- **Features**: Unifies parsing, multi-file resolution, semantic analysis, and WIR lowering into single-invocation APIs (`CompileFile`, `CompileSource`). Collects diagnostics and returns compile artifacts.

### 8. `lone-lsp`
- **Role**: Language Server Protocol (LSP) daemon for IDE integration.
- **Features**: Standard JSON-RPC 2.0 server supporting `initialize`, `textDocument/didOpen`, `textDocument/didChange`, `textDocument/didSave`, `textDocument/hover`, `textDocument/definition`, and `textDocument/documentSymbol`. Seamlessly integrates with Neovim (`extra/nvim-lsp`) and Open VSX / VS Code (`extra/ovsx-lsp`).
