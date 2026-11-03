# Disallow Comparison Chaining (`a < b < c`) Implementation Report

**Date**: 2026-11-03  
**Status**: Completed & Verified  

## 1. Overview
In languages like C, writing `a < b < c` is parsed as `(a < b) < c`, evaluating the boolean result of `a < b` against `c`, which is a notorious source of subtle bugs. In Python, it is desugared to `a < b and b < c`. Lone prohibits comparison chaining entirely: expressions such as `a < b < c`, `a <= b <= c`, or `a < b > c` are now rejected at compile time as errors.

## 2. Changes Made

### 2.1 Grammar & Parser
- **File**: `pkg/lone-parser/parser.peg`
  - In `COMPARISION_BINARY_EXPR`, inspected the rest list of operators. If more than one comparison operator is present in the sequence (`len(tail) > 1`), parsing fails immediately with:
    `comparison operators cannot be chained; use 'and' instead (e.g. 'a < b and b < c')`
- **File**: `pkg/lone-parser/generated-parser.go`
  - Regenerated via `pigeon`.

### 2.2 Semantic Analysis (Defense-in-Depth)
- **File**: `pkg/lone-analyzer/eval_type.go`
  - In `evalBinaryExpr`, added checks for relational comparison operators (`<`, `<=`, `>`, `>=`):
    - If either operand is of type `TypeBool` (e.g., in parenthesized forms like `(a < b) < c`), diagnostic `type/invalid-comparison` is raised:
      `cannot compare bool with comparison operator "<"; comparison operators cannot be chained`

### 2.3 Documentation
- **File**: `drafts/SPEC.md`
  - Updated Section 7.4: Relational comparison operators (`<`, `<=`, `>`, `>=`) are now documented as **Non-associative**.
  - Added an explanatory note detailing that comparison chaining is rejected at compile time and recommending `a < b and b < c`.

### 2.4 Testing
- **File**: `pkg/lone-parser/parser_test.go`
  - `TestComparisonChainingRejected`: Verifies that `a < b < c`, `a <= b <= c`, `a > b > c`, `a >= b >= c`, `a < b > c`, and `a > b <= c >= d` produce parse errors containing `"comparison operators cannot be chained"`, while valid single comparisons and `a < b and b < c` continue to parse properly.
- **File**: `pkg/lone-analyzer/analyzer_test.go`
  - `TestComparisonChainingAnalyzerRejection`: Verifies that parenthesized comparisons like `(a < b) < c` are flagged with `type/invalid-comparison`.
- **File**: `pkg/lone-compiler/std_io_e2e_test.go`
  - `TestComparisonChainingCompileError`: Verifies that the compiler fails when compiling a Lone file with `a < b < c`.
