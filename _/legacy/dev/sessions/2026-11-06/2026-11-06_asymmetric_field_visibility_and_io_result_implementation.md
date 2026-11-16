# Session Milestone: Asymmetric Field Visibility (`ro(pub)`) & Standard I/O Result Implementation

**Date**: 2026-11-06  
**Status**: Completed  
**Branch**: `reimpl`  
**Toolchain Modules**: `pkg2/lone-parser-ast`, `pkg2/lone-parser`, `pkg2/lone-analyzer`, `lib2/std/io.lone`, `exemple/guess_v0.6.0.lone`, `test2`, `dev/SPEC.md`

---

## 1. Objectives

1. Adopt idiomatic `Result<i32>` error handling across `:std:io` (`parse_int`), updating `exemple/guess_v0.6.0.lone` to handle non-numeric user inputs cleanly.
2. Accept and implement [`RFC-20261106-asymmetric_field_visibility`](../../rfcs/accepted/RFC-20261106-asymmetric_field_visibility.md), introducing `ro(pub) field: T` for physical struct fields with public read and struct-private write permissions.
3. Validate end-to-end execution against `wasmtime` and maintain 100% test pass rate across all 22 packages.

---

## 2. Technical Decisions & Implementations

### 2.1 Standard I/O `Result<i32>` Adoption
- **`lib2/std/io.lone`**:
  - Refactored `pub fn parse_int(s: str) Result<i32>`.
  - Returns `Result::Ok(n)` on valid digits and `Result::Err(Error::Generic)` when no numeric digits are parsed.
- **`exemple/guess_v0.6.0.lone`**:
  - Replaced raw integer parsing with error checking via `parsed is Result::Err` and `must parsed`.
  - Non-numeric input (such as typing `"abc"`) prompts the player with `-> Invalid input! Please enter a valid number (or 0 to surrender)` instead of silently surrendering.

### 2.2 Asymmetric Field Visibility (`ro(pub)`)
- **Parser (`pkg2/lone-parser-ast`, `pkg2/lone-parser`)**:
  - Added `IsRoPub bool` to `loneast.StructField`.
  - Extended parser to recognize `ro(pub)` on individual fields (`ro(pub) attempts: i32`) and grouped field declarations (`ro(pub) [x, y]: i32`) across structs and interfaces.
- **Semantic Analyzer (`pkg2/lone-analyzer`)**:
  - Added `IsRoPub bool` to `StructFieldInfo`.
  - Rejects redundant `ro(pub)` on private fields (`_field`) with `syn/redundant-field-modifier`.
  - Enforces write restrictions in `evalBinaryAssign`: writes to `ro(pub)` fields outside inherent methods of the declaring struct trigger `type/asymmetric-field-write`.
  - Taking references `&expr.field` outside the declaring struct yields `ref<ro T>`.
  - In `TypeSatisfiesInterface`, verified that `ro(pub)` fields satisfy read-only interface requirements (`ro field: T`) while rejecting mutable interface requirements (`field: T`).
- **WebAssembly WIR Lowerer (`pkg2/lone-compiler-wir`)**:
  - Zero runtime overhead: `ro(pub)` fields are physical memory offsets accessed directly via `i32.load offset=N` and `i32.store offset=N`.
- **Reference Example & Living Spec**:
  - Upgraded `Game` in `exemple/guess_v0.6.0.lone` to use `ro(pub) attempts: i32`.
  - Synchronized `dev/SPEC.md` Section 5.5.

---

## 3. Verification & Results

- **Unit Test Suite**: `TestAsymmetricFieldVisibility` in `pkg2/lone-analyzer/analyzer_test.go` passed.
- **End-to-End Behavior Test**: `test2/behavior/asymmetric_visibility.lone` compiled and ran under `wasmtime`, outputting:
  `Asymmetric field visibility ro(pub) verified in Wasm!`
- **Interactive Verification**: `guess_v0.6.0.lone` compiled and executed with error recovery on `"abc"` and in-place attempt counting via `ro(pub)`.
- **Full Workspace Test**: `go test ./...` passed across all 22 packages.
