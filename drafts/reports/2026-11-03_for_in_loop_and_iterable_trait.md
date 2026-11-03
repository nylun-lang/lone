# For-In Loop (`FOR_STMT`) & Standard Iterator Protocol

**Date:** 2026-11-03  
**Status:** Completed  
**Branch:** `feat/import-and-std-io`

---

## 1. Overview & Motivation

To provide ergonomic and type-safe iteration over ranges and custom collections, Lone now supports `for <var> in <iterable> { ... }` syntax backed by the standard iterator protocol (`IteratorTrait<T>` and `IterableTrait<T>` in `:std:builtin`), optional types (`?T` / `none`), and `impl Trait for Target` syntax.

---

## 2. Key Additions & Features

### 2.1 Grammar & Parser
- **Keywords**: Added `for` and `in` as guarded keywords (`KW_FOR`, `KW_IN`).
- **AST Node**: Added `ir.ForStmt`:
  ```go
  type ForStmt struct {
      VarName string
      Iter    Expr
      Body    Block
      Pos     Position
  }
  ```
- **Rule**: `FOR_STMT <- KW_FOR _ varName:IDENTIFIER _ KW_IN _ iter:EXPR _ body:BLOCK`.
- **Labelable**: `FOR_STMT` can be directly labeled (e.g. `outer: for i in iter { ... }`).
- **Impl Syntax**: Extended `IMPL_TRAIT_DEC` to support `(KW_ON / KW_FOR)`, allowing both `impl Trait for Target` and `impl Trait on Target`.

### 2.2 Desugaring & Type Analysis
- **Protocol Inspection**:
  - Checks if the iterable expression implements `iterator()`. If so, desugars `var __it = iterable.iterator()`.
  - If the iterable directly implements `has_next() bool` and `next() T`, it is bound as `var __it = iterable`.
- **Desugared Expansion**:
  ```lone
  {
      var __it = <iterable-expr>
      while __it.has_next() {
          var <varName> = __it.next()
          <user-body>
      }
  }
  ```
- **Lowering**:
  - `lowerStmt` and `lowerExpr` in MIR now natively lower `*loneanalyzer.TBlock`.
  - Supports `break` (both unlabeled and labeled) and `return` within for loops.

### 2.3 Optional Types (`?T` & `none`)
- Added `TypeKindOptional` and `OptionalType` (`?T`).
- Integrated with `AssignableTo`, `TypeSize`, `TypeAlign`, `ParseTypeRefWithResolver`, and `ParseTypeString`.
- `none` and concrete values of type `T` are assignable to `?T`.

### 2.4 Builtin Standard Library Module (`:std:builtin`)
- Created `libraries/std/builtin.lone`:
  - `IteratorTrait<T>` with `has_next(self: ptr<Self>) bool` and `next(self: ptr<Self>) T`.
  - `IterableTrait<T>` with `iterator(self: Self) Range`.
  - `Range` struct with `.new(start, end)`, `.new_with_step(start, end, step)`, `has_next()`, `next()`, and `iterator()`.
  - Functions `range(start, end) Range` and `range_step(start, end, step) Range`.
  - Implements `IteratorTrait<i32>` and `IterableTrait<i32>` on `Range`.
- Re-exported via `libraries/std/std.lone` (`:std`).
- Registered `:std:builtin` in LSP server namespaces.

---

## 3. Verification & Testing

- `pkg/lone-parser/parser_test.go`:
  - Added `TestForStmtAndImplFor` verifying parsing of `for i in range(...)`, labeled `outer: for ...`, and `impl Trait for Target`.
- `pkg/lone-compiler/std_io_e2e_test.go`:
  - Added `TestForRangeWasmtime` verifying execution of `for i in range(1, 5)`, `range_step(0, 10, 2)`, and early `break` with Wasmtime.
  - Added `TestCustomIteratorAndOptionalTypes` verifying custom struct iteration and optional types `?i32` / `none`.
- Full test suite: `go test -count=1 ./...` passed across all packages.
