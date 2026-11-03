# Lone Debug CLI and Query Engine

**Date:** 2026-11-03  
**Status:** Completed  
**Branch:** `feat/import-and-std-io`

---

## 1. Executive Summary

Implemented the `lone debug` command suite providing deep inspection of Lone symbols and scopes:
- `lone debug dump <scope>`: Full overview of structs, traits, functions, globals, and imports in a file or standard namespace (e.g. `:std:io`).
- `lone debug whatis <scope> <symbol>`: Detailed inspection of symbols:
  - **Structs**: Memory layout (size, align, field offsets/sizes), constructors, instance methods, and implemented traits with fulfilling method contracts.
  - **Traits**: Required method contracts and list of concrete structs implementing them.
  - **Variables**: Declared vs inferred/resolved types, initialization state, defined location, storage location (`datastack frame` vs `static linear memory`), and address-taken tracking (`&raw`).
  - **Functions**: Signatures, parameters, return types, public visibility, and local variables.
  - **Scoped Symbols**: Supports `foo:bar` and `foo.bar` to inspect inner function variables (`fn:var`) or struct fields/methods (`struct:field`, `struct:method`).
  - **Shorthands**: `lone debug <scope>` aliases `dump`, and `lone debug <scope> <symbol>` aliases `whatis`.
  - **JSON Output**: `--json` flag for machine-readable integrations.

---

## 2. Changes Implemented

1. **`pkg/lone-analyzer`**:
   - Exposed `All() []*Scope` on `ScopeStack`.
   - Added getters on `TypeEvaluator`: `Stack()`, `TraitImpls()`, `MethodImpls()`, `Modules()`.
   - Exposed `Evaluator() *TypeEvaluator` on `PassTypes` and `Analyzer`.

2. **`pkg/lone-analyzer-project`**:
   - Created `debug.go`:
     - `ResolveScopePath`: Resolves `:std:*` standard library namespaces and `.lone` file paths.
     - `InspectSymbol`: Semantic inspector supporting both unscoped and scoped (`parent:child`, `parent.child`) symbol queries.
     - `DumpScope`: Overview builder for all structs, traits, functions, globals, and imports.
   - Added unit test suite in `debug_test.go`:
     - Struct inspection with memory layout and implemented traits (`Stdout` implementing `WriterTrait`).
     - Trait inspection with method contracts and concrete implementors (`WriterTrait` implemented by `Stderr` and `Stdout`).
     - Scoped symbol inspection (`Stdout:fd`, `Stdout:write`, `Stdout.write`).
     - Variable address-taken detection (`&raw`).
     - Scope dump verification for `:std:io`.

3. **`cmd/lone`**:
   - Created `cmd/lone/debug.go` with `debugCmd`, `debugDumpCmd`, `debugWhatisCmd`.
   - Registered `debugCmd` in `cmd/lone/root.go`.
   - Formatted human-readable output and JSON serialization (`--json`).

---

## 3. Verification

All tests pass cleanly:
```bash
go test -count=1 ./...
```
Manual CLI tests:
```bash
lone debug :std:io
lone debug :std:io Stdout
lone debug :std:io WriterTrait
lone debug :std:io Stdout:fd
lone debug :std:io Stdout:write
lone debug exemple/guess.lone main:game
lone debug --json :std:io Stdout
```
