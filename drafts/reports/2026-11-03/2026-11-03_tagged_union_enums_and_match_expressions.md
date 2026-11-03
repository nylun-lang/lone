# Milestone Report: Tagged Union Enums and Pattern Match Expressions

**Date:** 2026-11-03  
**Branch:** `reimpl`  
**Status:** Implemented & Verified  

---

## 1. Executive Summary

This milestone adds native tagged union (`enum`) declarations and pattern matching expressions (`match`) to the Lone 2.0 greenfield compiler architecture, adhering directly to RFC Section 4.2 ("Tagged Unions & Pattern Matching"). The implementation spans the full compiler toolchain:
1. **Lexical Analysis (`pkg2/lone-lexer`)**: Added reserved keywords `TokenEnum` (`"enum"`) and `TokenMatch` (`"match"`).
2. **Parser & AST (`pkg2/lone-parser-ast`, `pkg2/lone-parser`)**: Extended `TYPE_DEC` with `TYPE_DEC_ENUM` and `EnumVariant` support for named and positional payloads; implemented `parseMatchExpr` and `parseMatchPattern` (wildcard, enum variants, literals) in the Pratt parser.
3. **Semantic Analysis (`pkg2/lone-analyzer`)**: Implemented `EnumType` and `EnumVariantInfo` with C/Rust-standard memory layout calculation (`[tag: i32] [payload: max_variant_payload]`), variant constructor type checking (`Shape::Circle(...)`, `Shape::Point`), payload binding propagation, match arm result type unification, and semantic exhaustiveness analysis (`type/non-exhaustive-match`).
4. **Compiler WIR Lowering (`pkg2/lone-compiler-wir`)**: Lowered enum constructors to linear memory bump allocation with store operations; lowered match expressions to structured `InstIf` decision trees extracting payload bindings and yielding stack values without control-flow graph (CFG) reconstruction string matching.
5. **Behavior Tests & Validation (`test2/`)**: Added `test2/behavior/enums.lone` and `test2/behavior/match_expr.lone`, fully passing under `wasmtime` in automated tests.

---

## 2. Technical Architecture & Memory Layout

### 2.1 Enum Memory Layout

Each enum instance is stored in linear memory as a contiguous block:
```text
+-------------------+---------------------------------------------+
|    Tag: i32       | Variant Payload: max(variant_payload_bytes) |
|   (Offset 0..4)   | (Offset 4..TotalSize)                       |
+-------------------+---------------------------------------------+
```
For example, for:
```lone
pub type Shape = enum {
    Circle(radius: i32),
    Rect(width: i32, height: i32),
    Point,
}
```
- Tag size: 4 bytes (`i32`).
- `Point`: tag 0, 0 bytes payload.
- `Circle`: tag 1, 4 bytes payload (`i32` at offset 4).
- `Rect`: tag 2, 8 bytes payload (`width` at offset 4, `height` at offset 8).
- Alignment: 4 bytes.
- Total size: $4 + 8 = 12$ bytes.

When constructed via `Shape::Circle(5)` or `Shape::Point`:
1. Pointer `$ptr` is allocated via heap bump: `$__heap_pointer -> $ptr; $__heap_pointer += TotalSize`.
2. Tag is stored at `$ptr + 0` (`i32.store offset=0`).
3. Payload arguments are evaluated and stored at their respective offsets (`f32.store`, `i32.store`, etc.).
4. `$ptr` is placed on the operand stack.

### 2.2 Pattern Matching & Exhaustiveness

Pattern matching syntax supports:
```lone
let a = match s {
    Shape::Circle(r) => 3 * r * r,
    Shape::Rect(w, h) => w * h,
    Shape::Point => 0,
    _ => 0,
}
```
- **Semantic Exhaustiveness Analysis**:
  - The analyzer tracks all variants defined on the target enum.
  - If any variant is omitted and no wildcard arm (`_ =>`) is present, compilation is halted with diagnostic `type/non-exhaustive-match`:
    `non-exhaustive match on enum "Shape": missing variants Rect`.
- **Payload Extraction**:
  - Each variant arm automatically extracts fields from the target pointer (`InstLoad offset=4`, etc.) and binds them as immutable locals in the arm scope.
- **Arm Result Unification**:
  - All arms are validated to yield compatible return types.
  - The match expression evaluates to the unified type on the Wasm operand stack.

---

## 3. Verification & Results

### 3.1 Behavior Test Execution

The behavior test suite was executed live using `wasmtime`:
```bash
go test -v ./test2/...
```
**Results:**
- `TestHello`: PASS
- `TestLetVar`: PASS
- `TestScopedHeaders`: PASS
- `TestLoopWhile`: PASS
- `TestExplicitCasts`: PASS
- `TestLabeledBlocks`: PASS
- `TestEnums`: PASS (verifies multi-field variants, unit variants, and pattern match field extraction)
- `TestMatchExpr`: PASS (verifies match on enums, literals, and wildcard branches)
- `TestCheckAllTest2Suite`: PASS (verifies all 12 behavior test files compile without diagnostics)

### 3.2 Regression & Package Status

All 22 Go packages across legacy and greenfield trees pass tests cleanly:
- `lone/cmd2/lone`: PASS
- `lone/pkg2/lone-lexer`: PASS
- `lone/pkg2/lone-parser`: PASS
- `lone/pkg2/lone-analyzer`: PASS
- `lone/pkg2/lone-compiler-wir`: PASS
- `lone/pkg2/lone-compiler`: PASS
- `lone/pkg2/lone-project`: PASS
- `lone/test2`: PASS
- All legacy `pkg/*` packages: PASS
