# Feature Report: Expressive Type References (`better-type`)

- **Branch**: `feat/better-type`
- **Date**: 2026-11-01
- **Status**: Completed & Verified

---

## 1. Request Overview

The goal of this feature was to enhance Lone's type expression syntax and internal representation by splitting `TYPE_REF` into three distinct kinds:
1. **`TypeFn`**: Function type references matching `fn (arg1: TYPE_REF, ...) TYPE_REF` (and void `fn ()`).
2. **`TypeList`**: Array/list type references matching `[]TYPE_REF` (supporting nested `[][]T` and constrained item types).
3. **`TypeBase`**: Base identifier types with optional generic type arguments `Ident Args?` (e.g. `int`, `Map<string, int>`).

---

## 2. Implementation Summary

### 2.1 IR Representation (`packages/lone-lang/irtypes/dec.go`)
- Added `TypeRefKind` enum: `TYPE_BASE`, `TYPE_LIST`, `TYPE_FN`.
- Updated `TYPE_REF` to hold `Kind`, `Constrain`, `Pos`, and `Value` (polymorphic payload).
- Implemented payload structs:
  - `TypeBase`: `Name string`, `Args []TYPE_REF`.
  - `TypeList`: `ItemType TYPE_REF`.
  - `TypeFn`: `Params []FnParam`, `ReturnType *TYPE_REF`.
- Added helper methods `AsBase()`, `AsList()`, and `AsFn()` on `TYPE_REF` for convenient casting.

### 2.2 PEG Grammar (`packages/lone-lang/parser/parser.peg`)
- Added `![a-zA-Z0-9_]` boundary guards on language keywords (`KW_FN`, `KW_VAR`, `KW_RO`, etc.) to prevent keywords from colliding with valid identifiers.
- Added grammar rules:
  - `TYPE_BASE`: Parses identifier and optional `<...>` type arguments.
  - `TYPE_LIST`: Parses `[]` followed by item `TYPE_REF`.
  - `TYPE_FN`: Parses `fn` parameter list `(name: TYPE_REF, ...)` and optional return `TYPE_REF`.
  - `FN_PARAM` and `FN_PARAM_LIST`: Support named parameters (`a: int`), unnamed parameters (`int`), and trailing commas.
  - `TYPE_ACTUAL`: Dispatches across `TYPE_FN / TYPE_LIST / TYPE_BASE`.
  - `TYPE_REF`: Composes constraints (`ro`, `?`, `comptime`) with `TYPE_ACTUAL`.
- Regenerated `generated-parser.go` via Pigeon.

### 2.3 Documentation & Samples
- Created sample test program [packages/lone-lang/samples/types.lone](file:///home/char-c/Projects/lone/packages/lone-lang/samples/types.lone) demonstrating all variations.
- Added architectural documentation in [drafts/under-the-hood/type_system_ir.md](file:///home/char-c/Projects/lone/drafts/under-the-hood/type_system_ir.md).

---

## 3. Verification & Testing

- **Sample Parsing**: Successfully parsed `samples/misc.lone` and `samples/types.lone` through `cmd/parse.go`.
- **Automated Tests**: Created [packages/lone-lang/parser/parser_test.go](file:///home/char-c/Projects/lone/packages/lone-lang/parser/parser_test.go) covering:
  - `TestTypeBase`: Base identifiers and multi-argument generic types.
  - `TestTypeList`: Single, nested, and constrained lists (`?[]int`, `[]?string`).
  - `TestTypeFn`: Empty args, void returns, multi-arguments, and higher-order nested functions.
- All unit tests passed (`go test -v ./...`).
