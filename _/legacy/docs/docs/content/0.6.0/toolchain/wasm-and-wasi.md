---
title: "WebAssembly & WASI"
order: 2
---

# WebAssembly & WASI Integration

Lone compiles directly to standard WebAssembly Text (`.wat`) and binary (`.wasm`), targeting standalone WASI runtimes (Node.js, Wasmtime, Wasmer) and browser engines with zero external C/runtime dependencies.

---

## Two-Tier Linear Memory Layout

Every compiled Lone module exports standard WebAssembly linear memory:

```wasm
(memory (export "memory") 2)
```

Linear memory is partitioned into dedicated regions:

```
0                   64               dataEnd        65536 ($__heap_base)
┌───────────────────┬────────────────┬──────────────┬─────────────────────────┐
│ Scratchpad (iovec)│ Static Data    │ Datastack    │ Dynamic Free-List Heap  │
│ WASI syscall args │ String tables  │ Shadow stack │ Managed objects, enums  │
│                   │                │ (downward)   │ (upward with grow)      │
└───────────────────┴────────────────┴──────────────┴─────────────────────────┘
```

1. **Reserved Scratch (0..63)**:
   - `0..7`: `ciovec[0]` (`buf` at offset 0, `len` at offset 4).
   - `8..11`: `nwritten` (bytes written result from `fd_write`).
2. **Static Data (64..dataEnd)**:
   - Immutable string literals and read-only lookup tables.
3. **Execution Datastack (dataEnd..65536)**:
   - Downward-growing stack frame storage for structs and address-taken values (`&raw`).
   - Constant-time rewind on return; zero leaks.
4. **Dynamic Free-List Heap (65536+)**:
   - Upward-growing heap managed by `$__lone_rt_alloc` and `$__lone_rt_free`.
   - Automatic `memory.grow` expansions.

---

## Inlined WASI System Calls

For WASI targets, Lone inlines syscall invocations directly:

```wasm
(import "wasi_snapshot_preview1" "fd_write" 
  (func $__wasi_fd_write (param i32 i32 i32 i32) (result i32)))

(func $__lone_rt_print (param $ptr i32) (param $len i32)
  ;; Store buffer pointer in ciovec[0].buf
  (i32.store (i32.const 0) (local.get $ptr))
  ;; Store buffer length in ciovec[0].len
  (i32.store (i32.const 4) (local.get $len))
  ;; Call WASI fd_write(fd=1, iovs=0, iovs_len=1, nwritten=8)
  (drop (call $__wasi_fd_write (i32.const 1) (i32.const 0) (i32.const 1) (i32.const 8)))
)
```

The emitted `.wat` or `.wasm` files are 100% self-contained and run on any WASI-compliant engine.
