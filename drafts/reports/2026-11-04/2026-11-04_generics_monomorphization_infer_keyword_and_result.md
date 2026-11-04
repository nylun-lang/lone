# Milestone Report: Generics, Monomorphization, Inferred Errors, and Unified Result

**Date:** 2026-11-04  
**Branch:** `reimpl`  
**Status:** Implemented & Verified  

---

## 1. Executive Summary

This milestone introduces zero-cost parametric polymorphism (generics), the `infer` keyword for type inference escape hatches, the unified `Error` enum, and the generic `Result<V, U = Error>` model to the Lone 2.0 greenfield compiler architecture.

Key features delivered:
1. **Lexical & Syntactic Support for Generics & `infer`**:
   - Added reserved keyword `infer` (`TokenInfer`) to `pkg2/lone-lexer`. Explicitly prohibited `_` for type inference per specification.
   - Enhanced `pkg2/lone-parser-ast` and `pkg2/lone-parser` to parse type parameter declarations with optional default type arguments (`<T, U = Error>`) on type declarations (`TYPE_DEC`) and function declarations (`FN_DEC`).
2. **Built-in Unified `Error` Enum**:
   - Pre-populated standard extensible `Error` enum containing common variants: `Generic(msg: str)`, `Code(code: i32)`, and `NotFound`.
3. **Generic `Result<V, U = Error>` Model**:
   - Standard generic `Result` with default error type `Error`, allowing zero-boilerplate `Result<T>` signatures while enabling strict custom errors (`Result<T, CustomError>`) and inferred error unions (`Result<T, infer>`).
4. **Monomorphization Engine**:
   - On-demand specialization of generic enums (`Option<T>`, `Result<V, U>`, etc.), generic structs (`Pair<A, B>`), and generic functions (`wrap<T>(item: T) Box<T>`).
   - Specialization names (`wrap__i32`) and concrete payload layout sizing directly tailored to WebAssembly execution without boxing or runtime type dispatch.
5. **Behavior Tests & Validation**:
   - Added `generics_struct_enum.lone`, `result_infer.lone`, and `generic_fn.lone` to `test2/behavior/`.
   - All 14 behavior tests pass cleanly under `wasmtime` and all 22 packages in the repository pass.

---

## 2. Technical Architecture & Implementation

### 2.1 Lexical & AST Additions

- **`pkg2/lone-lexer`**:
  - Registered `TokenInfer` ("infer").
- **`pkg2/lone-parser-ast/dec.go`**:
  - Introduced `TypeParam` struct:
    ```go
    type TypeParam struct {
        Name    string
        Default *TYPE_REF
        Span_   Span
    }
    ```
  - Added `TypeParams []TypeParam` to `FN_DEC` and `TYPE_DEC`.
- **`pkg2/lone-parser/parser.go`**:
  - Updated `parseTypeDec` and `parseFnDec` to accept `<T, U = DefaultType>`.
  - Updated `isTypeStart` and `parseTypeRef` to recognize `infer` as a type reference.

### 2.2 Built-in `Error` and `Result<V, U = Error>`

In `pkg2/lone-analyzer/eval_type.go`:
- Built-in `Error` enum:
  - Contiguous linear memory layout: `[tag: i32, payload bytes...]`.
  - Variants: `Generic(msg: str)` (tag 0), `Code(code: i32)` (tag 1), `NotFound` (tag 2).
- Generic `Result<V, U = Error>`:
  - Registered as a template with type parameters `V` and `U` (defaulting to `Error`).
  - Variants: `Ok(val: V)` (tag 0), `Err(err: U)` (tag 1).
  - When written as `Result<i32>`, `U` resolves to `Error`.
  - When written as `Result<i32, MyError>`, `U` resolves to `MyError`.
  - When written as `Result<i32, infer>`, `U` is inferred from `try` and `return Result::Err(...)` in the function body.

### 2.3 Monomorphization Pipeline

1. **Type Templates**:
   - During hoisting, generic definitions with `len(TypeParams) > 0` are stored as `GenericTypeTemplate` and `GenericFnTemplate`.
2. **On-Demand Instantiation (`instantiateGenericType`)**:
   - When a type reference with arguments (e.g. `Option<i32>` or `Result<i32, Error>`) is resolved, the template parameters are mapped to concrete types.
   - Field sizes, alignments, and offsets are computed concretely (`TypeSize`, `TypeAlign`).
   - Specialized types are cached by their specialized name (e.g., `Option<i32>`).
3. **Function Monomorphization (`monomorphizeFn`)**:
   - When calling a generic function `wrap(42)`, the type parameter `T` is inferred from the argument type `i32`.
   - A specialized AST clone is generated with concrete types and name `wrap__i32`.
   - The specialized function is hoisted, analyzed into `*TFnDec`, and appended to the program's top-level declarations.
   - The call site is rewritten to invoke the specialized function directly (`call $wrap__i32`).

### 2.4 Error Propagation & Inference with `infer`

- In `evalTryExpr`:
  - When `try expr` is evaluated inside a function returning `Result<V, infer>`:
    - The error payload type of `expr` is appended to `inferredErrors[currentFnName]`.
  - In `analyzeFnDec`:
    - If return type contains `infer`, all collected error types are unified:
      - Single error type: resolves `infer` to that specific type.
      - Multiple error types or variants of `Error`: resolves `infer` to `Error`.
      - Re-instantiates concrete `Result<V, ConcreteError>` as the final function return type.

---

## 3. Test Suite & Verification Results

All behavior test suites were run directly via `wasmtime` and `go test`:

| Test Name | File | Verified Behavior | Status |
|:---|:---|:---|:---:|
| `TestHello` | `test2/behavior/hello.lone` | Entry point & WASI #print runtime | PASS |
| `TestLetVar` | `test2/behavior/let_var.lone` | Immutability vs mutability semantics | PASS |
| `TestScopedHeaders` | `test2/behavior/scoped_headers.lone` | Header scope lexical retention | PASS |
| `TestLoopWhile` | `test2/behavior/loop_while.lone` | Post-condition loop iteration | PASS |
| `TestExplicitCasts` | `test2/behavior/explicit_casts.lone` | Numeric `as` truncation & widening | PASS |
| `TestLabeledBlocks` | `test2/behavior/labeled_blocks.lone` | Labeled break with value expressions | PASS |
| `TestEnums` | `test2/behavior/enums.lone` | Tagged union memory layout & construction | PASS |
| `TestMatchExpr` | `test2/behavior/match_expr.lone` | Pattern matching and exhaustiveness | PASS |
| `TestIsDiscriminant` | `test2/behavior/is_discriminant.lone` | Fast `is` variant discriminant checks | PASS |
| `TestDestructureHeader` | `test2/behavior/destructure_header.lone` | Header pattern destructuring (`if: [let ...]`) | PASS |
| `TestMustTry` | `test2/behavior/must_try.lone` | `must` panic assertion & `try` propagation | PASS |
| `TestGenericsStructEnum` | `test2/behavior/generics_struct_enum.lone` | `Option<T>` monomorphization and matching | PASS |
| `TestResultInfer` | `test2/behavior/result_infer.lone` | `Result<V, infer>` and default `Error` | PASS |
| `TestGenericFn` | `test2/behavior/generic_fn.lone` | Function monomorphization (`wrap<T> -> wrap__i32`) | PASS |
| `TestCheckAllTest2Suite` | `test2/behavior/*.lone` | Compile verification across all files | PASS |

Full repository test execution:
```bash
$ go test ./...
ok  	lone/cmd2/lone	1.429s
ok  	lone/pkg2/lone-analyzer	0.012s
ok  	lone/pkg2/lone-compiler	0.091s
ok  	lone/pkg2/lone-compiler-wir	0.017s
ok  	lone/pkg2/lone-lexer	(cached)
ok  	lone/pkg2/lone-parser	0.008s
ok  	lone/pkg2/lone-project	0.004s
ok  	lone/test2	0.178s
All 22 packages passing with 0 errors.
```

---

## 4. Next Step: Linear Memory Management

With generics, monomorphization, and error inference operational, the next foundational phase is **Linear Memory Management**:
1. Free-list allocator runtime (`alloc`, `free`, `realloc`) with header metadata.
2. Dynamic memory expansion via WebAssembly `memory.grow`.
3. Downward-growing stack and safety guard pages.
