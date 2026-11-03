# XOR Operator (`xor` / `^`)

**Date:** 2026-11-03  
**Status:** Completed  
**Branch:** `feat/import-and-std-io`

---

## 1. Overview & Motivation

Following the user proposal, Lone now supports the `xor` operator keyword alongside `and` and `or`. It functions symmetrically across both boolean logical expressions (exclusive-or) and integer types (bitwise exclusive-or), matching the symbolic `^` operator.

---

## 2. Key Additions & Features

### 2.1 Grammar & Parser
- **Keywords**: Added `KW_XOR <- "xor" ![a-zA-Z0-9_]` (along with `KW_AND` and `KW_OR`) to `KEYWORD`.
- **Precedence Hierarchy**:
  1. Equality (`==`, `!=`)
  2. Bitwise AND (`&`)
  3. Bitwise XOR (`^`)
  4. Bitwise OR (`|`)
  5. Logical AND (`and`, `&&`)
  6. Logical XOR (`xor`)
  7. Logical OR (`or`, `||`)
- **AST Nodes**: Added `ir.BINARY_OP_XOR ("xor")` and `ir.BINARY_OP_CARET ("^")`.

### 2.2 Type Evaluator & Lowering
- **Type Evaluator**:
  - For booleans, evaluates to `TypeBool`.
  - For integers, evaluates to the operand integer type (`i32`, `i64`, etc.).
- **MIR Lowering**:
  - Lowers `"xor"` and `"^"` to `MirBinOp{ Op: "xor", ... }`.
  - Lowers directly to Wasm instructions `i32.xor` and `i64.xor`.

### 2.3 Editor & Tooling
- Updated VS Code syntax highlighting grammar (`extra/ovsx-lsp/syntaxes/lone.tmLanguage.json`).
- Updated language specification (`drafts/SPEC.md`).
- Repackaged LSP VSIX (`extra/ovsx-lsp/lone-lsp-0.3.0.vsix`).

---

## 3. Verification & Testing

- `pkg/lone-parser/parser_test.go`:
  - `TestXorOperatorParser` verifying precedence `a and b xor c or d` => `((a and b) xor c) or d`.
- `pkg/lone-compiler/std_io_e2e_test.go`:
  - `TestXorOperatorWasmtime` verifying:
    - Boolean truth table (`true xor false == true`, `true xor true == false`, etc.).
    - Integer bitwise execution (`5 xor 3 == 6`, `12 xor 10 == 6`, `5 ^ 3 == 6`).
    - Operator precedence in compound condition statements on Wasmtime.
- All test suites passing (`go test -count=1 ./...`).
