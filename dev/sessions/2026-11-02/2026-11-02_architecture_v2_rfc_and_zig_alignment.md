# Work Session Report: Architecture v2 RFC & Zig Alignment Migration

- **Date:** 2026-11-02
- **Topic:** Architecture v2 RFC & In-Place Migration
- **Author:** Antigravity AI Pair

---

## 1. Request Summary
The user requested an architectural review of a revised repository structure inspired by the Zig programming language repository (`_local/zig`), addressing rapid AI code sprawl and establishing a cleaner, more comprehensible codebase organization. Upon confirmation, the migration to Architecture v2 was executed in-place on `feat/polish`.

---

## 2. Changes Made

1. **Architecture RFC**:
   - Authored [`drafts/research/architecture_v2.md`](file:///home/char-c/Projects/lone/drafts/research/architecture_v2.md) defining the Zig-inspired separation of host compiler (`pkg/`, `cmd/`) vs target language libraries (`lib/`).

2. **Unified Single Root Go Module**:
   - Created root [`go.mod`](file:///home/char-c/Projects/lone/go.mod) (`module lone`).

3. **Compiler Subsystems Consolidated under `pkg/`**:
   - `pkg/ast/`: Abstract Syntax Tree nodes (`dec.go`, `expr.go`, `utils.go`).
   - `pkg/parser/`: PEG grammar (`parser.peg`), Pigeon generator, and test suite.
   - `pkg/sema/`: Type checker, Analyzed IR (AIR), scope resolution, and naming/consume-or-drop conventions.
   - `pkg/mir/`: Mid-level IR, basic blocks, control flow graph, and AIR lowering.
   - `pkg/wir/`: WebAssembly Text format transpiler, memory data segments, and loops.
   - `pkg/adapter/`: Node.js and Browser JavaScript adapter runtime glue.
   - `pkg/diag/`: Formatted diagnostic and AST/AIR printers.

4. **Target Code Established under `lib/`**:
   - `lib/std/`: Standard library written in Lone (`lib/std/std.lone`).
   - `lib/runtime/`: C print runtime and WASI snapshot preview 1 stubs (`lib/runtime/lone_rt_print.c`).

5. **Consolidated Test Harness under `test/`**:
   - `test/behavior/`: Functional test files (`hello.lone`, `valid.lone`, `control_flow.lone`, `functions.lone`, etc.).
   - `test/compile_errors/`: Negative tests asserting compile-time diagnostics (`diagnostics.lone`).

6. **Unified CLI Driver (`cmd/lone`)**:
   - Single binary providing `lone build` (with `--target`, `--with-adapter`, `--mir`, `--wat`), `lone check` (with `--ast-json`, `--air-json`, `--diag-json`), `lone run`, and `lone lsp`.

7. **Documentation Updates**:
   - Updated [`AGENT.md`](file:///home/char-c/Projects/lone/AGENT.md) to document Architecture v2 layouts and commands.
   - Cleaned up scratch/draft directories (`packages2/` and empty `library/`).

---

## 3. Verification & Testing

- **Package Tests**:
  - `go test ./pkg/...` executed across all packages:
    - `lone/pkg/adapter` OK
    - `lone/pkg/diag` OK
    - `lone/pkg/mir` OK
    - `lone/pkg/parser` OK
    - `lone/pkg/sema` OK
    - `lone/pkg/wir` OK
- **CLI Verification**:
  - `go run ./cmd/lone check test/behavior/hello.lone`: Exits 0 ("Check passed with 0 errors").
  - `go run ./cmd/lone build test/behavior/hello.lone`: Emits full WebAssembly Text format with inlined WASI preview 1 printing and memory data segments.
  - `go run ./cmd/lone check test/behavior/valid.lone`: Exits 0 ("Check passed with 0 errors").
  - `go run ./cmd/lone check test/compile_errors/diagnostics.lone`: Correctly catches naming warnings, type mismatches, and consume-or-drop violations.

---

## 4. Next Steps
- Implement `lone run` integration with `wasmtime` / `node` execution harness.
- Gradually build out `lib/std/` (`io.lone`, `math.lone`, `string.lone`).
- Safely archive or remove legacy `packages/` once verified across all workflows.
