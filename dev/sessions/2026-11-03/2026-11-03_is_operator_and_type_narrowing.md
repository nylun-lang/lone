# Type-Check Operator (`is` / `is not`) & Automatic Type Narrowing

**Date:** 2026-11-03  
**Status:** Completed  
**Branch:** `feat/import-and-std-io`

---

## 1. Overview & Motivation

To provide clean, idiomatic, and type-safe inspection of values—particularly optional types (`?T`) and static types—Lone now supports the `is` and `is not` operators with automatic type narrowing (smart casting) in branching blocks.

---

## 2. Key Additions & Features

### 2.1 Grammar & Parser
- **Keywords**: Added `KW_IS <- "is" ![a-zA-Z0-9_]` and `KW_NOT <- "not" ![a-zA-Z0-9_]` to `KEYWORD`.
- **AST Node**: Added `ir.IsExpr`:
  ```go
  type IsExpr struct {
      Expr       Expr
      TargetType TYPE_REF
      Negated    bool
      Pos        Position
  }
  ```
- **Operator Precedence**:
  Placed `IS_EXPR` at Level 4 (between Relational Comparisons `<`, `>`, `<=`, `>=` and Equality `==`, `!=`).
  - Supports `x is T` and `x is not T`.
  - Also composes naturally with negation: `!(x is T)`.
- **Type References**: Updated `TYPE_BASE` to accept `none` as a valid type reference (`name: (IDENTIFIER / KW_NONE)`).

### 2.2 Automatic Type Narrowing
- When inspecting an optional variable `x: ?T` in conditional statements (`if` / `else` / packed `if`):
  - `if x is T`: in `then` block, `x` is narrowed to concrete type `T`. In `else` block, `x` is narrowed to `none`.
  - `if x is not none` (or `!(x is none)`): in `then` block, `x` is narrowed to `T`. In `else` block, `x` is narrowed to `none`.
  - `if x is none`: in `then` block, `x` is `none`. In `else` block, `x` is narrowed to `T`.
- Implemented via scoped symbol shadowing (`te.analyzeBlockWithNarrowing` and `te.applyNarrowing`), seamlessly restoring the outer type when exiting the block without mutating outer scope state.

### 2.3 MIR Lowering & WebAssembly Codegen
- **Optional Types (`?T`)**:
  - `x is none`: lowers to zero comparison (`MirCmpOp{ Op: "eq", ... }`).
  - `x is not none`: lowers to non-zero comparison (`MirCmpOp{ Op: "ne", ... }`).
  - `x is T`: lowers to non-zero comparison (`MirCmpOp{ Op: "ne", ... }`).
- **Static Types**:
  - `x is T`: evaluated at compile time; compiles directly into `MirConst{ Value: 1 }` or `0` with zero runtime penalty.
  - Side effects in `x` (e.g. calls) are preserved.

### 2.4 Tooling & Editor Support
- Added `is` and `not` to VS Code TextMate syntax highlighting grammar (`extra/ovsx-lsp/syntaxes/lone.tmLanguage.json`).
- Rebuilt LSP extension package (`extra/ovsx-lsp/lone-lsp-0.3.0.vsix`).
- Documented in language specification (`drafts/SPEC.md`).

---

## 3. Verification & Testing

- `pkg/lone-parser/parser_test.go`:
  - Added `TestIsOperatorParser` verifying parsing of `x is i32`, `x is not none`, `!(x is none)`, and `return x is none`.
- `pkg/lone-compiler/std_io_e2e_test.go`:
  - Added `TestIsOperatorAndNarrowingWasmtime` executing on Wasmtime verifying:
    - `var a: ?i32 = 42; if a is i32 { println_int(a) }` passes strictly typed `println_int` via automatic type narrowing.
    - `var b: ?i32 = none; if b is none { println(...) }`
    - `var c: ?i32 = 100; if c is not none { println_int(c) }`
    - `var d: ?i32 = 200; if !(d is none) { println_int(d) }`
    - `var e: ?i32 = 300; if e is none { ... } else { println_int(e) }`
    - Static type checks `n is i32` and `n is not str`.
- Full workspace test suite passed (`go test -count=1 ./...`).
