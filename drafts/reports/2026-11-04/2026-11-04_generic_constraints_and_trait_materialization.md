# Milestone Report: Generic Parameter Constraints (`where`) and Trait Materialization (`TrTrait::|T|`)

**Date:** November 4, 2026  
**Branch:** `reimpl`  
**Commit:** `9759d46`  
**Test Suite:** 22/22 Go packages passing (100% pass rate)

---

## 1. Overview & Objectives

Following the introduction of generic types, enums, functions, and the `infer` keyword, this milestone achieved:
1. **Generic Parameter Constraints with `where` Clause Syntax**:
   - Generic headers support constraints: `<V, U where { U is Error }>` or `<T where { T satisfies TrWriter }>`.
   - Clear semantic separation between:
     - `is`: Nominal type/category constraint (e.g., verifying `U` is nominal type `Error` or an `enum` representing an error).
     - `satisfies`: Trait contract compliance (verifying an `impl TrTrait on Type` implementation exists).
2. **Built-in `Result<V, U = Error where { U is Error }>`**:
   - Anchored `Result` so the error type parameter `U` is strictly constrained to `Error` or enum types (with `infer` allowed for `try` inference). Rejects arbitrary types like `Result<i32, string>`.
3. **Explicit Trait Materialization (`TrTrait::|T|`)**:
   - In accordance with Lone design principles, traits are not implicitly attached to type instances. Trait methods are accessed through explicit trait materialization:
     ```lone
     let w = TrWriter::|T|
     w.write(writer, msg)
     ```
     or chained directly: `TrWriter::|T|.write(writer, msg)`.

---

## 2. Key Architecture & Changes

### 2.1 Lexer (`pkg2/lone-lexer`)
- Registered `satisfies` (`TokenSatisfies`) alongside `is` (`TokenIs`).
- Unit tested lexer tokenization for generic constraint expressions.

### 2.2 AST (`pkg2/lone-parser-ast`)
- Added `ConstraintOp` enum (`ConstraintIs`, `ConstraintSatisfies`).
- Added `GenericConstraint` struct:
  ```go
  type GenericConstraint struct {
      TypeParam string
      Op        ConstraintOp
      Target    TYPE_REF
      Span_     Span
  }
  ```
- Attached `Constraints []GenericConstraint` to both `FN_DEC` and `TYPE_DEC`.
- Added `TTraitMatExpr` to `typed_ir.go`.

### 2.3 Parser (`pkg2/lone-parser`)
- Implemented `parseTypeParamsAndConstraints` to parse `<params where { param is/satisfies Target, ... }>`.
- Ensured trailing comma support in constraint lists.
- Integrated into `parseFnDec` and `parseTypeDec`.

### 2.4 Semantic Analyzer (`pkg2/lone-analyzer`)
- **Template Tracking**: Preserved `Constraints` in `GenericTypeTemplate` and `GenericFnTemplate`.
- **Trait Implementation Registry**: Maintained `traitImpls map[string]map[string]*loneast.IMPL_DEC` populated during `hoistImpl`.
- **Constraint Validation**:
  - In `instantiateGenericType`: Checks each constraint upon instantiation. Violations emit `type/constraint-violation`.
  - In `monomorphizeFn`: Validates arguments against function constraints before monomorphization.
- **Type Scope Propagation**: Defined active type parameters (`currentTypeParams`) into function scope so `T` resolves in local declarations and trait materialization targets.
- **Trait Materialization (`evalPostfixExpr`)**:
  - Handled `SuffixTraitMat` (`::|TargetType|`).
  - Synthesized `MaterializedTraitType` with concrete method signatures binding `self: TargetType`.
  - Verified `missing-trait-impl` if target type has no corresponding `impl`.
  - Handled method member access on `MaterializedTraitType` and `TraitType`.

### 2.5 WIR & Code Generation (`pkg2/lone-compiler-wir`)
- Lowered `TTraitMatExpr` as zero/local pointer.
- Enabled member access calls on materialized traits (`calleeName = mem.Field`), correctly passing arguments (`self`, `args...`) and emitting target function calls.

---

## 3. Verification & Testing

### 3.1 Unit Tests (`pkg2/lone-analyzer`)
- `TestGenericConstraintsIsError`:
  - Verified valid instantiations: `Result<i32>`, `Result<i32, infer>`, and custom `Result<i32, CustomErr>`.
  - Verified rejection of `Result<i32, string>` triggering `type/constraint-violation` (`does not satisfy 'is Error'`).
- `TestGenericConstraintsSatisfiesAndTraitMaterialization`:
  - Verified `log_msg<T where { T satisfies TrWriter }>(writer: T, msg: str)` with `TrWriter::|T|`.
  - Calling with `Console` (implements `TrWriter`) passed.
  - Calling with `Dummy` (does not implement `TrWriter`) triggered `type/constraint-violation`.

### 3.2 Behavior Execution Test (`test2/behavior/generic_constraints.lone`)
- Verified runtime execution via `wasmtime`:
  - `Result` checked with `must`.
  - Generic function `log_info<T where { T satisfies TrWriter }>` monomorphized.
  - Materialized trait `TrWriter::|Console|` dispatched `write`.
  - Output verified: `"Generic constraints and trait materialization verified!\n"`.

### 3.3 Full Suite Verification
All 22 packages across `pkg`, `pkg2`, and `test2` passed with zero errors:
```bash
go test ./...
# PASS across all packages
```
