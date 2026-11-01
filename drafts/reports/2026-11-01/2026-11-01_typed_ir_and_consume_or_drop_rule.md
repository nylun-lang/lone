# Report: Typed IR, Expression Type Evaluation, and Consume-or-Drop Invariant

**Date**: 2026-11-01  
**Author**: AI Assistant & Lone Team  
**Branch**: `feat/better-type`  
**Tags**: `[Ai-assisted]`, `lone-alyzer`, `typed-ir`, `type-inference`, `drop-keyword`, `safety`

---

## 1. Overview & Objectives

In response to language safety and analyzer requirements, this update introduces:
1. **Typed Intermediate Representation (Typed IR)** for `packages/lone-alyzer`: Transforming parsed AST nodes where types are placeholder `infer` into concrete `TNode` structures with fully resolved static types (e.g., lowering `var x: infer = 42` into `TVarDec{Name: "x", Type: TypeI32}`).
2. **Expression Type Evaluator (`TypeEvaluator`)**: An inference and type-checking engine capable of evaluating expression types across literals, binary operations, unary operators, function calls, labeled blocks, and objects.
3. **Function Return Checking**: Static verification that all return statements in a function match the declared return type, and that non-void functions cannot silently exit without returning a value.
4. **Consume-or-Drop Language Rule**: Any function call or expression evaluating to a non-void type inside a statement context must either be consumed (bound to a `var`, returned, or passed as an argument) or explicitly discarded using the newly introduced `drop` statement (`drop <expr>`).

---

## 2. Changes & Architectural Implementation

### 2.1 Grammar & Parser Extension (`packages/lone-lang`)
- Added `KW_DROP <- "drop" ![a-zA-Z0-9_]` to `packages/lone-lang/parser/parser.peg` and registered `KW_DROP` in the keyword table.
- Added `DROP_STMT <- KW_DROP _ expr:EXPR` to `PRIMARY_EXPR`, `STMT`, and `TOP_LEVEL`.
- Added `ir.DropStmt` (`ir.EXPR_DROP`) to `packages/lone-lang/irtypes/expr.go`.
- Regenerated parser using Pigeon and added `TestDropStmt` in `parser_test.go`.

### 2.2 Static Type Representation (`packages/lone-alyzer/types.go`)
- Defined `Type` interface with `Kind()`, `String()`, `Equals(Type)`, and `AssignableTo(Type)`.
- Predefined singletons for primitives: `TypeVoid`, `TypeNone`, `TypeAny`, `TypeBool`, `TypeStr`, `TypeChar`, `TypeI8`-`TypeI128`, `TypeU8`-`TypeU128`, `TypeF32`, `TypeF64`, `TypeInfer`, `TypeUnknown`.
- Structural & nominal types: `CustomType`, `ListType`, `FnType`, and `ObjectType`.
- Numeric assignment compatibility allowing float and int interoperability where valid.
- `ParseTypeRef`: Bridges parser `ir.TYPE_REF` into static analyzer `Type`.

### 2.3 Typed IR (`packages/lone-alyzer/typed_ir.go`)
- `TNode`: Base interface providing source position (`Pos()`).
- `TExpr`: Expression interface providing concrete evaluated type (`ExprType() Type`).
- Key nodes:
  - `TVarDec`: Carries resolved `Type` (never `infer`).
  - `TFnDec`: Contains typed parameter list, return type, and typed body block.
  - `TBlock`: Scoped statement list.
  - `TReturnStmt`: Return statement with typed value.
  - `TDropStmt`: Represents an explicit drop of an expression value.
  - `TBreakStmt`: Break statement with optional labeled target and value.
  - `TIfStmt` & `TWhileStmt`: Typed conditionals and loops.
  - `TCallExpr`: Fully typed function invocation node with verified callee and argument types.
  - `TLiteralExpr`, `TVarRefExpr`, `TBinaryExpr`, `TUnaryExpr`, `TObjectExpr`, `TListExpr`.

### 2.4 Type Evaluator & Rule Enforcement (`packages/lone-alyzer/eval_type.go`)
- **Symbol Hoisting**: Scans top-level declarations to pre-populate function signatures and global bindings before body analysis.
- **Variable Inference**: When `var x = expr` has no explicit type, the analyzer evaluates `expr.ExprType()` and binds the variable to that concrete type.
- **Return Type Verification**: Compares returned expression types against function return types (`type/return-mismatch`) and warns if non-void functions lack return statements (`type/missing-return`).
- **Consume-or-Drop Invariant**: Standalone expression statements that evaluate to non-void/non-none produce `type/unused-result` diagnostics unless wrapped in `drop <expr>`.

### 2.5 Multi-Pass Integration (`packages/lone-alyzer/pass_types.go` & `analyzer.go`)
- `PassTypes` integrates seamlessly into `Analyzer`'s pass pipeline.
- `Analyzer.AnalyzeTyped(ast any) (*TProgram, []Diagnostic)` allows callers to retrieve both the generated Typed IR and any emitted diagnostics.

---

## 3. Test Coverage

Comprehensive unit tests were implemented in `packages/lone-alyzer/analyzer_test.go`:
- `TestTypeInferenceToConcreteIR`: Verifies lowering of integer, string, boolean, and function-local inferred variables into Typed IR.
- `TestFunctionReturnCheck`: Verifies detection of return type mismatches and missing returns in non-void functions.
- `TestConsumeOrDropRule`:
  - Standalone call to `compute_val() i32` -> flags error `type/unused-result`.
  - `drop compute_val()` -> passes with 0 diagnostics.
  - `var res = compute_val()` -> passes with 0 diagnostics.
  - Standalone call to `do_logging() void` -> passes without requiring `drop`.

All tests pass:
- `lone-lang`: 11 / 11 test suites passing.
- `lone-alyzer`: 7 / 7 test suites passing.

---

## 4. Next Steps

1. Implement object member access type checking in chained postfix expressions.
2. Add control-flow graph (CFG) reachability analysis for exhaustive return checks across complex if-else / loop constructs.
3. Wire the analyzer and typed IR output into a CLI command in `cmd/lone`.
