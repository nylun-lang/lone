# Feature Report: Function Declarations (`fn name (args) T { }`)

- **Branch**: `feat/better-type`
- **Date**: 2026-11-01
- **Status**: Completed & Verified

---

## 1. Request Overview

Added grammar and IR support for top-level function declarations of the form:
```lone
fn name (arg1: T, ...) T { }
```

---

## 2. Implementation Summary

### 2.1 IR Representation ([`packages/lone-lang/irtypes/dec.go`](file:///home/char-c/Projects/lone/packages/lone-lang/irtypes/dec.go))
- Added `FN_DEC`:
  - `Name string`: Function name (supports identifiers and `#`-prefixed built-in constructors like `#init`).
  - `Pos Position`: Source position.
  - `Params []FnParam`: Parameter list with names and `TYPE_REF`s.
  - `ReturnType *TYPE_REF`: Optional return type (nil for void).
  - `Body Block`: Block containing statements.
- Added `Block`: `Stmts []any`, `Pos Position`.
- Added `ReturnStmt`: `Value Expr`, `Pos Position`.

### 2.2 PEG Grammar ([`packages/lone-lang/parser/parser.peg`](file:///home/char-c/Projects/lone/packages/lone-lang/parser/parser.peg))
- Updated `TOP_LEVEL` to dispatch across `BIND_DEC / FN_DEC`.
- Added `KW_RETURN` to `KEYWORD`.
- Added `FN_NAME <- "#"? IDENTIFIER`.
- Added `RETURN_STMT <- KW_RETURN expr:(_ EXPR)?`.
- Added `STMT <- BIND_DEC / RETURN_STMT / EXPR`.
- Added `BLOCK <- "{" _ stmts:(_ STMT)* _ "}"`.
- Added `FN_DEC` with parameter list, optional return type, and body block.
- Recompiled PEG parser via Pigeon.

### 2.3 Verification & Testing
- Added `TestFunctionDeclaration` in [`packages/lone-lang/parser/parser_test.go`](file:///home/char-c/Projects/lone/packages/lone-lang/parser/parser_test.go) covering:
  - Void functions with empty parameter lists (`fn noop() { }`)
  - Typed parameters with return types and return statements (`fn add(a: int, b: int) int { return a }`)
  - Complex types (`[]string`, `?float`, `[]float`) with local bindings (`var x = 10`)
  - `#`-prefixed constructor names (`fn #init(name: string) Self { }`)
  - Higher-order function arguments (`cb: fn (x: int) bool`)
- Created sample file [`packages/lone-lang/samples/functions.lone`](file:///home/char-c/Projects/lone/packages/lone-lang/samples/functions.lone) and verified JSON output with `cmd/parse.go`.
- All 8 unit tests pass (`go test -v ./...`).
