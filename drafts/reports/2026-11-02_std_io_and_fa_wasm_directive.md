# Work Session Report: Module Import System, `---fa:wasm` Capabilities, Memory Model & Pure-Lone `std:io`

**Date**: 2026-11-02  
**Branch**: `feat/import-and-std-io`  
**Tags**: `[Ai-assisted]`, `std:io`, `import-system`, `wasi`, `fa:wasm`, `datastack`, `structs`, `traits`, `mir`, `wir`

---

## 1. Request Overview

Implement all 6 phases of the agreed Lone module import, memory model, struct/trait, and pure-Lone Standard I/O roadmap on `feat/import-and-std-io`:
1. **Flat Import System & Module Resolver**: `import .foo, .bar from :std:io`, `import .foo from .local`, `import :std:io as io`, visibility isolation (`pub`), and cycle detection.
2. **Host Capability Directives (`---fa:wasm`)**: Gate low-level host imports (`:wasi:snapshot_preview1`, `:env`) strictly to files with `---fa:wasm ... ---` or `--fa:wasm`, preventing host capability leakage into user application files.
3. **Explicit Indirection (`&` vs `&raw`) & Linear Datastack**: Distinguish managed references (`&x` -> `ref<T>`) from raw WebAssembly linear memory offsets (`&raw x` -> `ptr<T>`), backed by a frame-scoped bump datastack (`$__stack_pointer`, $O(1)$ prologue/epilogue rewind) and heap arena (`$__heap_pointer`).
4. **Struct Memory Layout, Pointer Arithmetic & SRET ABI**: Compute exact field byte offsets, sizes, and alignments; lower field access/mutation and fixed-size arrays (`[N]T`) to linear memory loads/stores; support associated functions, `self` instance methods, and hidden `__sret` return-by-value slots.
5. **Trait Materialization (`Trait::|Type|`)**: Compile materialized traits as zero-sized compile-time function bundles with static dispatch (`Trait__Type__method`) and zero runtime allocation.
6. **Pure-Lone Standard Library (`:std:sys` & `:std:io`)**: Implement `libraries/std/sys.lone`, `libraries/std/io.lone`, and `libraries/std/std.lone` purely in Lone targeting WASI Preview 1 (`fd_write`, `fd_read`, `fd_close`, `proc_exit`) without C dependencies or inline WAT strings.

---

## 2. Changes Made

### 2.1 Parser & AST (`pkg/lone-parser-ast/`, `pkg/lone-parser/`)
- Extended AST (`dec.go`, `expr.go`) and PEG grammar (`parser.peg`, regenerated `generated-parser.go` via Pigeon) with:
  - `WasmDirective` (`---fa:wasm ... ---` block and `--fa:wasm` inline forms).
  - `TYPE_DEC` (`struct`, `trait`, structural `{ ... }`, type aliases), `IMPL_DEC` (`impl Trait on Target { ... }` and inherent `impl Target { ... }`).
  - `AssignStmt`, `SuffixTraitMat` (`::|Type|`), `SuffixIndex` (`[expr]`), `%` modulo, bare `self` parameters, `pub` visibility modifier, `let`/`var` bindings, and synthetic resolver AST nodes (`HostImportDecl`, `ModuleScopeDecl`, `ResolvedImportDecl`).

### 2.2 Host Registry & Multi-File Module Resolver (`pkg/lone-runtime/host.go`, `pkg/lone-analyzer-project/resolver.go`)
- Registered canonical WASI Preview 1 signatures (`fd_write`, `fd_read`, `fd_close`, `proc_exit`, `args_get`, `args_sizes_get`, `random_get`) and `:env` (`print`) in `pkg/lone-runtime/host.go`.
- Built `Resolver` in `pkg/lone-analyzer-project/resolver.go` and wired `ResolveAST` into `pkg/lone-compiler/compiler.go`, `cmd/lone/check.go`, and `pkg/lone-lsp/server.go`:
  - Resolves `:std`, `:std:io`, `:std:sys`, and relative `.foo` / `..bar` modules.
  - Enforces per-file `---fa:wasm` authorization (`capability/unauthorized-host-import`), export visibility (`import/undefined-symbol`), and circular import detection (`import/cycle`).
  - Wraps each imported file in an isolated `ModuleScopeDecl` so private symbols and `---fa:wasm` authority never leak to importing modules.

### 2.3 Semantic Analysis & Type System (`pkg/lone-analyzer/`, `pkg/lone-analyzer-diag/`)
- Added `RefType` (`ref<T>`, `ref<ro T>`), `PtrType` (`ptr<T>`, assignable to `ptr<any>`, strictly distinct from `ref<T>` and `i32`), `StructType` (`ComputeLayout()` byte offsets/padding/alignment), `TraitType`, `TraitMatType` (`Trait::|Target|`), and `ModuleAliasType`.
- Implemented `AddressTaken` analysis on local variables and scalar parameters, read-only (`ro`) mutation rejection (`type/readonly-mutation`), pointer arithmetic (`ptr<T> +/- i32`), indexing (`[N]T`, `ptr<T>`, `str` -> `u8`), and memory intrinsics (`#heap_alloc`, `#ptr_to_str`, `#str_to_ptr`, `#refGrab`).

### 2.4 MIR, WIR & WAT Lowering (`pkg/lone-compiler-mir/`, `pkg/lone-compiler-wir/`)
- **MIR (`types.go`, `lower.go`)**: Added `MirSlotAddr`, `MirLoad`, `MirStore`, `MirGlobalSet`, `MirHeapAlloc`, `MirHostImport`, `FrameSize`, and `JoinTarget`. Implemented datastack slot allocation, SRET struct return-by-value ABI (`__sret`), struct/array construction, address-taken scalar spilling, and byte-scaled pointer arithmetic.
- **WIR & WAT (`ast.go`, `lower.go`, `optimizer.go`, `wat.go`)**:
  - Emits 2-page (128 KiB) linear memory with `$__stack_pointer` (growing down from `131072`) and `$__heap_pointer` (growing up from 16-byte-aligned end of static data segments) whenever frame slots or `#heap_alloc` are used, while preserving 1-page zero-global output for simple modules.
  - Implemented recursive structured CFG lowering (`cfgLowerer.lowerRegion`) for `if/else`, `while`, nested branches, `break`, and datastack frame prologue/epilogue (`$__fp` / `$__stack_pointer` restoration on every `return`).
  - Fixed WAT data segment serialization (`wat.go`) so hex byte escapes (`\0a`, `\00`, `\22`, `\5c`) emit exact bytes in WebAssembly linear memory without double-escaping backslashes, and removed hardcoded `callee == "print"` interception so pure-Lone `std:io.print` executes natively.
- **CLI (`cmd/lone/build.go`, `cmd/lone/run.go`)**: Separated `buildWebCmd`'s `--with-adapter` flag binding so `lone build` and `lone run` default cleanly to standalone WASI without unintended browser HTML adapter generation.

### 2.5 Pure-Lone Standard Library (`libraries/std/`)
- [`libraries/std/sys.lone`](../../libraries/std/sys.lone): `---fa:wasm` WASI Preview 1 syscall bridge (`Ciovec`, `sys_write`, `sys_read`, `sys_close`, `sys_exit`, `STDIN_FD`, `STDOUT_FD`, `STDERR_FD`).
- [`libraries/std/io.lone`](../../libraries/std/io.lone): Pure-Lone `str_len`, `Writer` & `Reader` traits, `Stdout`/`Stderr`/`Stdin` structs + trait implementations, `print`, `println`, `eprint`, `eprintln`, and heap-promoting `read_line()`.
- [`libraries/std/std.lone`](../../libraries/std/std.lone): Root `:std` barrel re-exporting `:std:io` symbols.

---

## 3. Verification & Testing

- **Full Uncached Go Test Suite (`go test -count=1 ./...`)**: **100% PASS** across all packages (`lone-parser`, `lone-analyzer`, `lone-analyzer-diag`, `lone-analyzer-project`, `lone-compiler-mir`, `lone-compiler-wir`, `lone-compiler-adapter`, `lone-compiler`, `lone-lsp`, `lone-runtime`, `lone-shared-logger`).
- **New Unit & Integration Tests**:
  - `pkg/lone-parser/parser_test.go`: `TestWasmDirectiveStructTraitAndIndex`.
  - `pkg/lone-analyzer/mem_struct_trait_test.go`: `TestRefVsRawPtrTypeDistinction`, `TestReadOnlyMutationRejection`, `TestStructLayoutAndByteOffsets`, `TestTraitMaterializationAndMissingImpl`.
  - `pkg/lone-analyzer-project/resolver_test.go`: `TestCapabilityGatingRejectsUnauthorizedHostImport`, `TestCapabilityIsolationAcrossStdImport`, `TestRelativeModuleVisibilityAndCycles`.
  - `pkg/lone-compiler/std_io_e2e_test.go`:
    - `TestPureLoneStdIoPrintAndReadLineWasmtime`: Compiles pure-Lone `:std:io` (`print`, `println`, `read_line`) with zero `__lone_rt_print` inlined WAT, pipes CRLF and LF lines into `wasmtime`, and verifies exact stdout.
    - `TestStructSretDatastackRewindAndTraitMatWasmtime`: Executes 5,000 loop iterations of datastack struct + `&raw` calls in `wasmtime` (verifying leak-free `$__stack_pointer` rewind), SRET struct methods, `Writer::|Stdout|` materialized trait calls, `:std` barrel imports, and `import :std:io as io`.
- **Live CLI Verification**:
  - `go run ./cmd/lone run test/behavior/io_pure_lone.lone` -> outputs:
    ```text
    Hello from Pure Lone std:io!
    Materialized Writer::|Stdout| works!
    ```

---

## 4. Next Steps & Future Considerations

- **Escape Analysis & Reference Counting for Managed Store**: Extend `#heap_alloc` into automatic escape-promoted `ref<T>` allocations with scope/shadow reference release hooks.
- **Buffered `BufReader` / `BufWriter` & Integer Formatting**: Build buffered stream wrappers and `print_int` / string interpolation helpers atop `:std:io`'s `Reader` and `Writer` traits.
- **Generic Structs & Parameterized Traits**: Generalize `StructType` and `TraitMatType` monomorphization for user-defined generic containers (`Vec<T>`, `Option<T>`).
