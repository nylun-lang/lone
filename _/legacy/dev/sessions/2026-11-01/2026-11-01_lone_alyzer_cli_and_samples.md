# Report: lone-alyzer CLI and Samples Implementation

**Date**: 2026-11-01  
**Author**: AI Assistant & Lone Team  
**Branch**: `feat/better-type`  
**Tags**: `[Ai-assisted]`, `lone-alyzer`, `cli`, `samples`, `pretty-print`, `node-tree`

---

## 1. Overview

Taking inspiration from `packages/lone-lang/cmd/parse.go`, this update establishes a dedicated command-line interface in `packages/lone-alyzer/cmd/main.go` and accompanying sample `.lone` files in `packages/lone-alyzer/sample/`.

The CLI provides flexible inspection capabilities:
- Printing formatted **Alyzer-IR** with inferred and verified types.
- Printing the **Typed IR Nodes** (as a hierarchical tree or serialized JSON).
- Printing **both** IR and Nodes simultaneously (default behavior).
- Reporting all collected **Diagnostics** with source locations and severity.

---

## 2. CLI Architecture & Features

### 2.1 Flags & Options

```
Usage: lone-alyzer [flags] <file.lone>

Flags:
  -both
        Print both Alyzer IR and node tree
  -ir
        Print formatted Alyzer IR
  -json
        Print Typed IR nodes formatted as JSON
  -mode string
        Output mode: 'ir', 'nodes', or 'both'
  -nodes
        Print Typed IR node tree
```

### 2.2 Execution Pipeline

1. **Parser Execution**: Loads the target `.lone` file through `lonelang/parser.ParseFile(filePath)`.
2. **Analysis Pipeline**: Passes AST into `lonealyzer/analyze.NewAnalyzer(filePath, config).AnalyzeTyped(rawAst)`, evaluating expressions, inferring variable types, verifying function returns, enforcing the consume-or-drop invariant, and checking naming conventions.
3. **Renderer**:
   - `print.PrintProgram(prog)` renders clean, source-level typed Lone code.
   - `print.PrintNodeTree(prog, 0)` renders an indented tree of the Typed IR nodes.
   - `print.PrintNodesJSON(prog)` outputs formatted JSON.
   - `print.FormatDiagnostics(diags)` renders color-coded/formatted diagnostic messages.

---

## 3. Sample Files

- **`packages/lone-alyzer/sample/valid.lone`**:
  Demonstrates clean Lone code: top-level constants, functions with parameters and returns, dropped expression results (`drop compute_total(...)`), loops, and labeled blocks.
- **`packages/lone-alyzer/sample/diagnostics.lone`**:
  Demonstrates analyzer diagnostics:
  - `naming/global`: `BadGlobalName` violates ALL_CAPS / snake_case.
  - `naming/function`: `BadFunctionName` violates snake_case.
  - `naming/parameter`: `BadParam` violates snake_case.
  - `type/return-mismatch`: function expects `i32` but returns `str`.
  - `type/unused-result`: function call returning `i32` is not consumed or dropped.

---

## 4. Verification

CLI executions tested:
1. `go run ./packages/lone-alyzer/cmd/main.go ./packages/lone-alyzer/sample/valid.lone` -> **OK** (Prints both Alyzer IR and Node tree).
2. `go run ./packages/lone-alyzer/cmd/main.go -ir ./packages/lone-alyzer/sample/valid.lone` -> **OK** (Prints Alyzer IR only).
3. `go run ./packages/lone-alyzer/cmd/main.go -nodes ./packages/lone-alyzer/sample/valid.lone` -> **OK** (Prints Node tree only).
4. `go run ./packages/lone-alyzer/cmd/main.go -json ./packages/lone-alyzer/sample/valid.lone` -> **OK** (Prints JSON nodes).
5. `go run ./packages/lone-alyzer/cmd/main.go ./packages/lone-alyzer/sample/diagnostics.lone` -> **OK** (Flags all 5 expected diagnostics).
6. Full test suite passing across all modules (19 tests).
