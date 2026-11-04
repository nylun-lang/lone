# Report: lone-alyzer Architecture Research & Draft Implementation

**Date**: 2026-11-01  
**Author**: AI Assistant & Lone Team  
**Branch**: `feat/better-type`  
**Tags**: `[Ai-assisted]`, `lone-alyzer`, `static-analysis`, `linter`, `naming-conventions`

---

## 1. Executive Summary

This report documents the architectural research, design, and draft implementation of `packages/lone-alyzer`, the static analysis and verification engine for the Lone language. 

Building on the stabilized parser and IR in `packages/lone-lang`, `lone-alyzer` operates over a single file AST through an extensible multi-pass pipeline backed by a lexical `ScopeStack`. The initial implementation delivers Pass 1, which enforces Lone's naming conventions across global/local variables, functions, intrinsic procedures, parameters, types, labels, and object literal fields.

---

## 2. Research & Architecture Design

The full architectural RFC was authored and saved to [drafts/research/lone_alyzer_architecture.md](../../drafts/research/lone_alyzer_architecture.md).

### 2.1 Multi-Pass Pipeline Design

The analyzer is designed around $N$ isolated passes executed sequentially over the AST:
1. **Pass 1 — Naming Conventions**: Enforces style rules and syntactic identifier patterns without needing full type knowledge.
2. **Pass 2 — Lexical Scoping & Symbol Table**: Populates symbol tables across file, function, block, label, and loop scopes.
3. **Pass 3 — Label & Control-Flow Validation**: Ensures `break` statements target valid enclosing labels or loops, and flags dead code.
4. **Pass 4 — Type Checking & Inference**: Solves bidirectional type constraints, validating assignment compatibility and function calls.

```
       AST ([]any)
            │
            ▼
┌───────────────────────┐
│ Pass 1: Conventions   │ ──► Diagnostics (Warnings/Hints)
└───────────────────────┘
            │
            ▼
┌───────────────────────┐
│ Pass 2: Symbol Scopes │ ──► Scope Tree & Duplicate Symbol Errors
└───────────────────────┘
            │
            ▼
┌───────────────────────┐
│ Pass 3: Control-Flow  │ ──► Illegal Break / Return Diagnostics
└───────────────────────┘
            │
            ▼
┌───────────────────────┐
│ Pass 4: Type Checker  │ ──► Type Mismatch Diagnostics
└───────────────────────┘
```

### 2.2 Stack-Based Lexical Scoping

To track contextual state during traversal, `ScopeStack` provides:
- Lexical scopes: `ScopeFile`, `ScopeFunction`, `ScopeBlock`, `ScopeLabel`, `ScopeLoop`.
- Scope queries: `HasEnclosing(kind)` and `FindEnclosing(kind)` to easily validate statements like `break blk: expr` against target labels or loops.
- Lexical resolution: `Resolve(name)` climbs up the scope hierarchy to locate symbols.

---

## 3. Implementation Details

The new module `lonealyzer` was established under `packages/lone-alyzer/` and integrated into the root `go.work` workspace:

1. **`go.mod`**: Configured with `replace lonelang => ../lone-lang`.
2. **`diagnostic.go`**: Defines `Diagnostic`, `Severity` (`SeverityHint`, `SeverityWarning`, `SeverityError`), and source location references (`irtypes.Position`).
3. **`scope.go`**: Implements `ScopeKind`, `SymbolKind`, `Symbol`, `Scope`, and `ScopeStack` with push/pop, symbol definition, and ancestor queries.
4. **`conventions.go`**: Implements regex matching and classifier predicates:
   - `IsSnakeCase`: `^[a-z_][a-z0-9_]*$`
   - `IsAllCaps`: `^[A-Z_][A-Z0-9_]*$`
   - `IsCamelCase`: `^[A-Z][a-zA-Z0-9]*$`
   - `IsIntrinsic`: `^#[a-z_][a-z0-9_]*$`
   - `IsPrimitiveType`: Handles primitive types (`i8`-`i128`, `u8`-`u128`, `f32`, `f64`, `bool`, `str`, `string`, `char`, `void`, `any`, and compiler placeholder `infer`).
5. **`pass_conventions.go`**: AST visitor that verifies:
   - Global variables follow `ALL_CAPS` or `snake_case` (`naming/global`).
   - Local variables follow `snake_case` (`naming/variable`).
   - Functions follow `snake_case` or `#intrinsic` (`naming/function`).
   - Parameters follow `snake_case` (`naming/parameter`).
   - Type references follow `CamelCase` (`naming/type`).
   - Labeled blocks and break targets follow `snake_case` (`naming/label`).
   - Object literal field keys follow `snake_case` (`naming/field`).
6. **`analyzer.go`**: High-level runner accepting an AST and orchestrating configured passes.
7. **`analyzer_test.go`**: Comprehensive test suite verifying scope stack operations, positive cases (clean code), negative cases (naming convention violations across all AST node types), and intrinsic functions.

---

## 4. Verification & Testing

Both modules were validated against their test suites:
- `cd packages/lone-alyzer && go test -v ./...` -> **PASS**
  - `TestScopeStack`: Passed
  - `TestNamingConventionsValidCode`: Passed
  - `TestNamingConventionsInvalidIdentifiers`: Passed
  - `TestNamingConventionsIntrinsicFunction`: Passed
- `cd packages/lone-lang && go test -v ./...` -> **PASS** (all 10 parser test suites passing)

---

## 5. Next Steps

1. Expand `Pass 2`: Populate symbol tables with variable, function, and label bindings, detecting shadowed or duplicate identifiers.
2. Implement `Pass 3`: Verify label targets for `break <label>: <expr>` statements and loop breaks.
3. Integrate CLI support in `cmd/` for running the analyzer over `.lone` source files.
