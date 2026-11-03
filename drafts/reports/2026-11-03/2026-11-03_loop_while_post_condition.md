# Post-Condition Loop (`loop { ... } while [cond]`) Implementation Report

**Date**: 2026-11-03  
**Status**: Completed & Verified  

## 1. Overview
Lone introduces an unambiguous, semicolon-free post-condition (do-while) loop construct using the syntax:
```lone
loop {
    var x = read()
} while: [x > 0]
```

Key attributes:
1. **At Least One Execution**: The loop body executes before the condition is ever evaluated.
2. **Semicolon-Free**: Enclosing the condition in brackets `[...]` with a `while:` clause label creates a hard syntactic boundary, preventing line-bleed without requiring a trailing semicolon.
3. **No Collision with Consecutive Loops**: A subsequent `while count < 10 { ... }` lacks `:` and `[` and is unambiguously parsed as a pre-condition while loop rather than attaching to the prior `loop`.
4. **Smart Scoping**: Variables declared inside the `loop` block are accessible inside the condition `while: [cond]`, but strictly out of scope after the loop terminates.

## 2. Changes Made

### 2.1 AST & Parser
- **File**: `pkg/lone-parser-ast/expr.go`
  - Added `EXPR_DO_WHILE` to `ExprKind`.
  - Added `type DoWhileStmt struct { Body Block; Cond Expr; Pos Position }`.
- **File**: `pkg/lone-parser/parser.peg`
  - Added `LOOP_WHILE_TAIL <- _ KW_WHILE _ ":" !":" _ "[" _ cond:EXPR _ "]"`.
  - Updated `LOOP_STMT` to parse optional `whilePart:LOOP_WHILE_TAIL?`.
  - When absent, desugars into `ir.WhileStmt` with `true` (infinite loop).
  - When present, returns `ir.DoWhileStmt{ Body, Cond, Pos }`.
- **File**: `pkg/lone-parser/generated-parser.go`
  - Regenerated via `pigeon`.

### 2.2 Semantic Analysis & Smart Scope
- **File**: `pkg/lone-analyzer/typed_ir.go`
  - Added `TDoWhileStmt struct { Body TBlock; Cond TExpr; Pos_ ir.Position }`.
- **File**: `pkg/lone-analyzer/eval_type.go`
  - Added `analyzeDoWhileStmt`: maintains the loop block's scope stack during the evaluation of `s.Cond` so inner variables are visible, popping the scope when the loop exits.
- **File**: `pkg/lone-analyzer/pass_conventions.go`
  - Added `checkDoWhileStmt`.
- **File**: `pkg/lone-analyzer-diag/print.go`
  - Added `TDoWhileStmt` node printing and string formatting.

### 2.3 MIR & WIR Lowering
- **File**: `pkg/lone-compiler-mir/types.go`
  - Added `IsDoWhile bool` flag to `MirBlock`.
- **File**: `pkg/lone-compiler-mir/lower.go`
  - Lowered `TDoWhileStmt` by jumping directly to the body block (`dowhile_body`), jumping to `dowhile_cond`, and branching back to `dowhile_body` on true or `dowhile_exit` on false.
- **File**: `pkg/lone-compiler-wir/lower.go`
  - Structured Wasm loop lowering wrapping `(block $exit (loop $head ...))`, checking `MirBranch` back-edges into `$head`.

### 2.4 Verification & Tests
- `pkg/lone-parser/parser_test.go`:
  - `TestLoopWhilePostConditionParser`: Verifies parsing of `loop { } while [cond]` and absence of collision with subsequent `while` loops.
- `pkg/lone-analyzer/analyzer_test.go`:
  - `TestLoopWhileSmartScopeAndAnalysis`: Verifies that variables declared in the body are accessible in `while [cond]` and rejected outside.
- `pkg/lone-compiler/std_io_e2e_test.go`:
  - `TestLoopWhilePostConditionWasmtime`: End-to-end execution on Wasmtime testing single-execution on initial false condition, multi-iteration execution with inner variable access, and `break` handling.
