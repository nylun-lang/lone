# Work Session Report: Standard I/O Architecture & Formal Import System

**Date**: 2026-11-02  
**Tags**: `[Ai-assisted]`, `rfc`, `std:io`, `import-system`, `wasi`, `fa:wasm`, `runtime`, `architecture`

---

## 1. Request Overview
The user provided clarification and architectural refinement:
- Rather than an ad-hoc keyword like `require`, Lone should formalize its **import system**.
- Key syntax & semantic rules:
  - `import .readline from :std:io`
  - Flat comma-separated symbols: `import .foo, .bar from :somewhere` (no `{}` curly braces).
  - Colon prefix (`:`) signifies a **namespace** (e.g., `:std:io`, `:std:math`, `:wasi:cli`).
  - Dot prefix (`.`) signifies a **relative file/module** at the same directory level (e.g., `import .foo from .std`).
  - Leading dot on symbols (`.foo`, `.bar`) signifies member projection from the target source without curly brace noise.
  - **Explicit Indirection (`&` vs `&raw`)**:
    - `&foo`: Creates a managed reference `ref<T>` (tracked by lifetime/escape analysis).
    - `&raw foo`: Creates an unmanaged raw memory address `ptr<T>` (WebAssembly linear memory byte offset `i32`). Promotes the variable to the linear datastack so it possesses a memory address for WASI syscalls (`iovec`, `nread`, buffer pointers).
- This unifies both standard library usage in user code and host capability binding inside `---fa:wasm` blocks.

---

## 2. Work Conducted & Deliverables

1. **RFC Expanded & Formalized**:
   - Updated [std_io_and_fa_wasm_directive.md](../research/std_io_and_fa_wasm_directive.md) to title: *RFC: Lone Module Import System & Standard I/O (`std:io`)*.
   - Formalized the syntax for namespace imports (`:`) vs relative file imports (`.`).
   - Flat, brace-free symbol projection syntax (`import .foo, .bar from :somewhere`).
   - Defined explicit indirection semantics: `&foo` (`ref<T>`) vs `&raw foo` (`ptr<T>`).
   - Clarified the physical distinction between the bump-allocated datastack ($O(1)$ rewind on return) and the anchored Managed Store.
   - Conceptualized structs as linear memory layouts and materialized traits as zero-sized function bundles.

2. **Authored Comprehensive Implementation Roadmap**:
   - Created [roadmap_import_system_memory_and_std_io.md](../research/roadmap_import_system_memory_and_std_io.md) covering 6 distinct phases from flat imports to pure Lone standard I/O.

3. **Implemented Phase 1 & `&raw` Operator**:
   - Created feature branch: `feat/import-and-std-io`.
   - **AST**: Added `loneast.ImportDecl` to `pkg/lone-parser-ast/dec.go` and `UNARY_OP_RAW_REF = "&raw"` to `pkg/lone-parser-ast/expr.go`.
   - **PEG Grammar**: Updated `pkg/lone-parser/parser.peg` to parse:
     - `import .foo, .bar from :std:io` (namespace imports)
     - `import .foo from .std` (relative module imports)
     - `import :std:io as io` (aliased imports)
     - `&raw foo` prefix unary expression.
   - **Regenerated Parser**: Generated `pkg/lone-parser/generated-parser.go` using Pigeon.
   - **Semantic & Project Integration**:
     - Added `TImportDecl` in `pkg/lone-analyzer/typed_ir.go`.
     - Integrated `ImportDecl` in `pkg/lone-analyzer/eval_type.go` and `pass_conventions.go`.
     - Added import symbol requirement tracking in `pkg/lone-analyzer-project/project.go`.
   - **Tests Added**:
     - `TestImportDeclaration` in `pkg/lone-parser/parser_test.go`.
     - `TestRawPointerUnaryExpr` in `pkg/lone-parser/parser_test.go`.
     - All package tests passing (`100% PASS`).

---

## 3. Pending Questions & Next Steps
- Implement Phase 2: Host capability directives (`---fa:wasm`) and WIR import emission.
- Implement Phase 3: Datastack (shadow stack) bump/rewind in WIR/MIR and `&raw` address lowering.
