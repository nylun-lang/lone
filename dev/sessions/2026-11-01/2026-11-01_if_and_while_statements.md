# Feature Report: If Statements & While Loops

- **Branch**: `feat/better-type`
- **Date**: 2026-11-01
- **Status**: Completed & Verified

---

## 1. Overview

Implemented grammar and intermediate representation for conditional branching (`IfStmt`, supporting `else if` and `else`) and iterative loops (`WhileStmt`), integrated with Lone's labeled block/expression model.

---

## 2. Implementation Summary

### 2.1 IR Additions ([`packages/lone-lang/irtypes/expr.go`](file:///home/char-c/Projects/lone/packages/lone-lang/irtypes/expr.go))
- Added `EXPR_IF` and `EXPR_WHILE` to `ExprKind`.
- Added `ir.IfStmt`:
  - `Cond Expr`: Condition expression.
  - `Then Block`: Consequence block.
  - `Else any`: Optional `ir.Block` or nested `ir.IfStmt` for chained `else if`.
  - Implements `ir.Expr`.
- Added `ir.WhileStmt`:
  - `Cond Expr`: Loop condition.
  - `Body Block`: Loop body block.
  - Implements `ir.Expr`.

### 2.2 PEG Grammar ([`packages/lone-lang/parser/parser.peg`](file:///home/char-c/Projects/lone/packages/lone-lang/parser/parser.peg))
- Added `KW_IF`, `KW_ELSE`, `KW_WHILE` with boundary checks (`![a-zA-Z0-9_]`) to `KEYWORD`.
- Fixed comparison operator precedence: ordered `>=` and `<=` before `>` and `<` in `Comparision_Op`.
- Added `ELSE_BRANCH <- KW_ELSE _ branch:(IF_STMT / BLOCK)`.
- Added `IF_STMT <- KW_IF _ cond:EXPR _ then:BLOCK el:(_ ELSE_BRANCH)?`.
- Added `WHILE_STMT <- KW_WHILE _ cond:EXPR _ body:BLOCK`.
- Integrated `IF_STMT` and `WHILE_STMT` into `PRIMARY_EXPR`, `STMT`, `LABELABLE`, and `TOP_LEVEL`.
- Recompiled parser via Pigeon.

### 2.3 Testing & Verification
- Added `TestIfStmtAndWhileStmt` in [`packages/lone-lang/parser/parser_test.go`](file:///home/char-c/Projects/lone/packages/lone-lang/parser/parser_test.go) verifying:
  - `if` with `else if` and `else` branches (`fn classify(x: int) int`).
  - `while` loops with nested `if` and `break` statements (`fn count_up(limit: int) int`).
  - Labeled `while` loops evaluated as expressions: `var res = loop: while true { break loop: 42 }`.
- Created sample [`packages/lone-lang/samples/control_flow.lone`](file:///home/char-c/Projects/lone/packages/lone-lang/samples/control_flow.lone) and verified JSON output.
- All 10 unit tests pass (`go test -v ./...`).
