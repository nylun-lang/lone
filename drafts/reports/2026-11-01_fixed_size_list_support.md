# Feature Report: Fixed-Size List Support (`[size]T`)

- **Branch**: `feat/better-type`
- **Date**: 2026-11-01
- **Status**: Completed & Verified

---

## 1. Overview

Addressed the missing `[size]T` case (e.g. `[9]T`, `[BUFFER_SIZE]T`) for `TypeList` in Lone's parser and intermediate representation.

---

## 2. Changes Made

### 2.1 IR Additions (`packages/lone-lang/irtypes/`)
- Updated `TypeList` in [`irtypes/dec.go`](file:///home/char-c/Projects/lone/packages/lone-lang/irtypes/dec.go) to include `Size Expr` and an `IsFixed() bool` helper method.
- Added `NameRef` (`irtypes/expr.go`) implementing `Expr` (`EXPR_NAME_REFERENCE`) so constant / named sizes (like `[MAX_BUFFER]T`) can be represented as expressions.
- Utilized `ir.LITERAL_DYNAMIC_SIZE` for unconstrained slice types (`[]T`).

### 2.2 PEG Grammar (`packages/lone-lang/parser/parser.peg`)
- Updated `TYPE_LIST` to match `"[" _ size:EXPR? _ "]"` followed by `elem:TYPE_REF`.
- If `size` is present, it is stored in `TypeList.Size`. If omitted, `TypeList.Size` defaults to `ir.Literal{ Kind: ir.LITERAL_DYNAMIC_SIZE }`.
- Added `NAME_REF` rule to `PRIMARY_EXPR` so named constants work in expressions and type bracket bounds.
- Regenerated `generated-parser.go` via Pigeon.

### 2.3 Documentation & Testing
- Added `TestFixedTypeList` in [`packages/lone-lang/parser/parser_test.go`](file:///home/char-c/Projects/lone/packages/lone-lang/parser/parser_test.go) testing:
  - Single fixed array: `[9]int`
  - Named constant size: `[BUFFER_SIZE]string`
  - Multi-dimensional fixed array: `[9][10]int`
  - Fixed array of dynamic slices: `[9][]int`
- Updated sample file [`packages/lone-lang/samples/types.lone`](file:///home/char-c/Projects/lone/packages/lone-lang/samples/types.lone).
- Updated internal architecture doc [`drafts/under-the-hood/type_system_ir.md`](file:///home/char-c/Projects/lone/drafts/under-the-hood/type_system_ir.md).
- All unit tests pass (`go test -v ./...`).
