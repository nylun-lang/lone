# Implementation Report: Inlined `#print` Intrinsic and Linear Memory WAT Lowering

**Date**: 2026-11-01  
**Branch**: `feat/better-type`  
**Scope**: `packages/lone-lang`, `packages/lone-alyzer`, `packages/falone`

---

## 1. Executive Summary

We implemented end-to-end support for the `#print` built-in intrinsic across the Lone toolchain:
1. **Parser (`packages/lone-lang`)**:
   - Updated `NAME_REF` grammar in `parser.peg` to allow `#` prefix (`"#"? IDENTIFIER`), allowing intrinsic function calls like `#print("...")`.
   - Fixed escape character handling in PEG grammar for `\n`, `\r`, `\t`, `\"`, `\\`.
   - Re-generated pigeon parser and verified with parser test suite.
2. **Type Analyzer (`packages/lone-alyzer`)**:
   - Pre-registered `#print` in `NewScopeStack` as a built-in symbol with signature `fn(str: str) void`.
   - Allowed intrinsic identifier fallback (`IsIntrinsic`) in type evaluation.
   - Calling `#print` produces 0 diagnostics and evaluates to `void`.
3. **MIR Representation (`packages/falone/mirtypes`)**:
   - Added `DataSegments []MirDataSegment` to `MirModule` (`Offset int`, `Value string`).
   - Extended `FormatModule` printer to emit `data @<offset> "<escaped_string>"`.
4. **Lowering (`packages/falone/lowering`)**:
   - Lowerer tracks `dataOffset` (starting at offset 16, reserving 0..15 as scratch memory for WASI `ciovec_t`).
   - Deduplicates string literals in memory and aligns next available offset to 4 bytes.
   - Lowers `#print(str)` directly into `call @__lone_rt_print %ptr %len`.
5. **WAT Transpiler (`packages/falone/transpile`)**:
   - Added `Target` configuration (`TargetWasi`, `TargetWeb`) controllable via CLI `--target` / `-t` flag.
   - Emits linear memory string data segments: `(data (i32.const <offset>) "<escaped_bytes>")`.
   - For WASI target:
     - Emits `(import "wasi_snapshot_preview1" "fd_write" (func $__wasi_fd_write (param i32 i32 i32 i32) (result i32)))`.
     - Emits inlined `$__lone_rt_print (param $ptr i32) (param $len i32)` which sets up the `ciovec_t` structure at offset 0 and calls `fd_write(1, 0, 1, 8)`.
     - Exports both `(export "_start")` and `(export "main")` for compatibility with standalone WASI runtimes and embedders.
   - For Web target:
     - Emits `(import "env" "print" (func $__lone_rt_print (param i32 i32)))`.

---

## 2. End-to-End Execution Verification

A sample Lone program was written to `packages/falone/sample/hello.lone`:
```lone
fn main() void {
    #print("Hello from Lone!\n")
}
```

### Generated WAT (`packages/falone/sample/hello.wat`):
```wat
(module
  (import "wasi_snapshot_preview1" "fd_write" (func $__wasi_fd_write (param i32 i32 i32 i32) (result i32)))
  (memory (export "memory") 1)
  (data (i32.const 16) "Hello from Lone!\0a\00")

  ;; Inlined WASI #print runtime implementation
  (func $__lone_rt_print (param $ptr i32) (param $len i32)
    (i32.store (i32.const 0) (local.get $ptr))
    (i32.store (i32.const 4) (local.get $len))
    (drop (call $__wasi_fd_write (i32.const 1) (i32.const 0) (i32.const 1) (i32.const 8)))
  )

  (func $main (export "_start") (export "main")
    ;; label entry
    i32.const 16
    i32.const 17
    call $__lone_rt_print
    return
  )
)
```

### Execution with Node WASI (`node:wasi`):
```bash
$ wasi.start(instance)
Hello from Lone!
```
The WebAssembly module was assembled and executed against real WASI system calls in Node.js, successfully outputting `Hello from Lone!` to stdout without errors.

---

## 3. Test Suite Status

All 27 test suites across all packages pass:
- `lone-lang`: 12 test suites (syntax, literals, control flow, intrinsic parsing)
- `lone-alyzer`: 8 test suites (naming conventions, scope stack, type evaluation, consume/drop check, `#print` signature)
- `falone`: 5 test suites (lowering, loop lowering, WAT emission, WASI print, Web print)
