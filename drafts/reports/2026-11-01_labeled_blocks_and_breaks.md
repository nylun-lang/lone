# Feature Report: Labeled Blocks & Break Statements

- **Branch**: `feat/better-type`
- **Date**: 2026-11-01
- **Status**: Completed & Verified

---

## 1. Overview

Implemented labeled declarations (`LabelDec`, e.g. `blk: { ... }`) and break statements (`BreakStmt`, e.g. `break name: expr`, `break expr`, `break`), allowing blocks to evaluate to expressions.

---

## 2. Implementation Summary

### 2.1 IR Additions ([`packages/lone-lang/irtypes/expr.go`](file:///home/char-c/Projects/lone/packages/lone-lang/irtypes/expr.go))
- Added `EXPR_LABEL_DEC` and `EXPR_BREAK` to `ExprKind`.
- Added `ir.LabelDec`:
  - `Name string`: Label identifier.
  - `Body Block`: Statement block.
  - Implements `ir.Expr` so labeled blocks can appear anywhere expressions are valid (variable assignments, arguments, return statements).
- Added `ir.BreakStmt`:
  - `Label string`: Target label (or empty string).
  - `Value Expr`: Evaluated return/break value (or nil).
  - Implements `ir.Expr` and functions as a `STMT`.

### 2.2 PEG Grammar ([`packages/lone-lang/parser/parser.peg`](file:///home/char-c/Projects/lone/packages/lone-lang/parser/parser.peg))
- Added `KW_BREAK <- "break" ![a-zA-Z0-9_]` and registered in `KEYWORD`.
- Added `LABEL_DEC <- name:IDENTIFIER _ ":" !":" _ body:BLOCK`.
- Integrated `LABEL_DEC` into `PRIMARY_EXPR` and `TOP_LEVEL`.
- Added `BREAK_LABEL <- id:IDENTIFIER _INLINE_WS ":" !":"`.
- Added `BREAK_STMT <- KW_BREAK label:(_INLINE_WS BREAK_LABEL)? val:(_INLINE_WS EXPR)?`.
- Enforced inline whitespace (`_INLINE_WS`) for statement-level breaks so bare breaks do not swallow expressions across statement newlines.
- Recompiled parser via Pigeon.

### 2.3 Testing & Verification
- Added `TestLabelDecAndBreakStmt` in [`packages/lone-lang/parser/parser_test.go`](file:///home/char-c/Projects/lone/packages/lone-lang/parser/parser_test.go) verifying:
  - Labeled block assigned as an expression: `var res = blk: { var x = 10; break blk: x + 1 }`.
  - Labeled block inside a function.
  - Labeled break without value: `break eval:`.
  - Unlabeled break with value: `break 100`.
  - Bare break: `break`.
- Created sample [`packages/lone-lang/samples/labels_and_breaks.lone`](file:///home/char-c/Projects/lone/packages/lone-lang/samples/labels_and_breaks.lone) and verified JSON output.
- All 9 unit tests pass (`go test -v ./...`).
