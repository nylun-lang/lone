# Milestone Report: Greenfield Reimplementation of WIR Lowerer, Compiler, CLI Build, and Lib2

**Date:** 2026-11-03  
**Branch:** `reimpl`  
**Commit:** `6f3792d`  
**Status:** Completed & 100% Passing Tests  

---

## 1. Executive Summary

Following the completion of the greenfield front-end (`pkg2/lone-lexer`, `pkg2/lone-parser-ast`, `pkg2/lone-parser`, `pkg2/lone-analyzer`), we implemented the greenfield backend compiler pipeline:
1. **`pkg2/lone-compiler-wir`**: WebAssembly AST representation, AST-directed structured control flow lowering, linear-scan local coalescing (solving the virtual register explosion), and memory-safe linear memory layout.
2. **`pkg2/lone-compiler`**: High-level compiler driver coordinating lexing, Pratt parsing, strict semantic analysis, lowering, peephole optimization, and WebAssembly text format emission.
3. **`cmd2/lone build`**: Full CLI compilation subcommand supporting `-o <out.wat>`, `--target wasi|web`, and `-O`. Verified with live `wasmtime` execution.
4. **`lib2/std`**: Core next-generation standard library written in pure Lone:
   - `sys.lone`: WebAssembly WASI Snapshot Preview 1 bridge.
   - `io.lone`: Pure-Lone standard I/O implementing the new `TrWriter` and `TrReader` traits with immutable `let` bindings.
   - `string.lone`: Idiomatic string operations (`len`, `equals`, `starts_with`).

---

## 2. Key Architecture & Resolutions

### 2.1 Elimination of Fragile CFG Reconstruction (RFC Section 3.1)
- **Problem in Legacy Compiler**: Lowering to flat MIR basic blocks and attempting to reconstruct structured Wasm control flow using block name string matching (`"loop_"`, `"while_then"`) was brittle and caused misalignment on non-trivial control flow.
- **Greenfield Solution**: Direct AST-directed lowering from `loneanalyzer.TProgram`:
  - `while cond { body }` lowers directly to an outer `InstBlock $break` wrapping an inner `InstLoop $continue`. The condition check branches conditionally to `$break`, and the end of the body branches back to `$continue`.
  - `loop { body } while: [cond]` lowers to an outer `InstBlock $break` wrapping an inner `InstLoop $continue`. The post-condition evaluates at the loop tail, branching back to `$continue` when non-zero.
  - `if: [let x = 0] cond { then } else { else }` evaluates header bindings in the outer scope, pushes the condition onto the operand stack, and lowers directly into `InstIf { Then, Else }`.
  - `break` and `continue` resolve to targeted structured labels (`$break`, `$continue`) without heuristic guessing.

### 2.2 Elimination of Virtual Register Explosion (RFC Section 3.2)
- **Problem in Legacy Compiler**: Every intermediate expression evaluation generated a fresh virtual register (`%t0`, `%t1`, ... `%t5492`), creating thousands of Wasm locals.
- **Greenfield Solution**:
  - **Stack Machine Direct Emission**: Intermediate expression trees evaluate directly on the WebAssembly value stack, requiring **zero** temporary locals for expression evaluation.
  - **Local Allocator & Coalescer (`LocalAllocator`)**: Variables and spilled temporaries are pooled by Wasm type (`$i32_0`, `$i32_1`, `$f64_0`). When lexical scopes exit, variable slots are returned to the pool, allowing sibling blocks and sequential scopes to reuse the exact same physical Wasm locals.

### 2.3 Memory Safety & Static Scratchpad Layout (RFC Section 3.3)
- **Scratchpad Reservation**: Offsets `[0 .. 64)` are strictly reserved for WASI `fd_write` `ciovec` structs and runtime scratchpad memory.
- **Data Segments**: Static string literals start at offset `64` with 4-byte/16-byte alignment and null termination.
- **Runtime Pointers**:
  - `$__stack_pointer`: Initializes to `total_memory_bytes` (131,072 bytes / 2 pages), growing downwards.
  - `$__heap_pointer`: Initializes to `heapStart = (dataEnd + 15) & ^15`, growing upwards.

### 2.4 Next-Generation Standard Library (`lib2/std`)
- Implemented `lib2/std/sys.lone` using authorized `@fa:wasm` imports from `:wasi:snapshot_preview1`.
- Implemented `lib2/std/io.lone` adopting:
  - `TrWriter` and `TrReader` (conforming to rule IL3: `Tr` + PascalCase).
  - Immutable `let` bindings (rule IL1).
  - Pure WASI scatter/gather output via `sys_write`.
- Implemented `lib2/std/string.lone` with clean string manipulation.

---

## 3. Verification & Live Execution

### 3.1 Live WASI Execution via Wasmtime
Compiled `test/behavior/hello.lone` using `cmd2/lone build test/behavior/hello.lone -o /tmp/hello_greenfield.wat`:
```wat
(module
  (import "wasi_snapshot_preview1" "fd_write" (func $__wasi_fd_write (param i32 i32 i32 i32) (result i32)))
  (memory (export "memory") 2)
  (global $__stack_pointer (mut i32) (i32.const 131072))
  (global $__heap_pointer (mut i32) (i32.const 96))
  (data (i32.const 64) "Hello from Lone!\0a\00")

  ;; Inlined WASI #print runtime implementation
  (func $__lone_rt_print (param $ptr i32) (param $len i32)
    (i32.store (i32.const 0) (local.get $ptr))
    (i32.store (i32.const 4) (local.get $len))
    (drop (call $__wasi_fd_write (i32.const 1) (i32.const 0) (i32.const 1) (i32.const 8)))
  )

  (func $main (export "_start") (export "main")
    i32.const 64
    i32.const 17
    call $__lone_rt_print
    return
  )
)
```
Executed with `wasmtime /tmp/hello_greenfield.wat`:
```
Hello from Lone!
```

### 3.2 Full Test Suite Results
```
ok  	lone/cmd2/lone	0.891s
ok  	lone/pkg/lone-analyzer	(cached)
ok  	lone/pkg/lone-analyzer-diag	(cached)
ok  	lone/pkg/lone-analyzer-project	(cached)
ok  	lone/pkg/lone-compiler	(cached)
ok  	lone/pkg/lone-compiler-adapter	(cached)
ok  	lone/pkg/lone-compiler-mir	(cached)
ok  	lone/pkg/lone-compiler-wir	(cached)
ok  	lone/pkg/lone-lsp	(cached)
ok  	lone/pkg/lone-parser	(cached)
ok  	lone/pkg/lone-runtime	(cached)
ok  	lone/pkg/lone-shared-logger	(cached)
ok  	lone/pkg2/lone-analyzer	0.003s
ok  	lone/pkg2/lone-compiler	0.061s
ok  	lone/pkg2/lone-compiler-wir	0.018s
ok  	lone/pkg2/lone-lexer	(cached)
ok  	lone/pkg2/lone-parser	0.004s
```
**Zero regressions across legacy and greenfield codebases.**
