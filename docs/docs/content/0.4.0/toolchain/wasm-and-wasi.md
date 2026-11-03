---
title: "WebAssembly & WASI"
order: 2
---

# WebAssembly & WASI Integration

Falone compiles Lone code directly to WebAssembly Text (`.wat`), with specialized architectures for standalone WASI systems and browser environments.

## Linear Memory Layout

Every compiled Falone module declares a WebAssembly linear memory:

```wasm
(memory (export "memory") 1)
```

Falone partitions linear memory as follows:

```
0                   16                                    N
┌───────────────────┬─────────────────────────────────────┐
│ Scratchpad        │ Static Data Segments (String data)  │
│ (ciovec_t / iovs) │ "Hello from Lone!\0a\00"            │
└───────────────────┴─────────────────────────────────────┘
```

- **Offset 0..15**: Reserved scratch space for WASI `ciovec_t` structures:
  - `0..3`: `iov[0].buf` (pointer to string buffer, `i32`)
  - `4..7`: `iov[0].len` (byte length of string buffer, `i32`)
  - `8..11`: `nwritten` (number of bytes written returned by `fd_write`, `i32`)
- **Offset 16+**: Static string literal data segments. Each segment is aligned to 4 bytes and null-terminated for C/Wasm interoperability.

## Inlined WASI `#print` Runtime

For WASI targets, Falone inlines the runtime function directly into the module:

```wasm
(import "wasi_snapshot_preview1" "fd_write" 
  (func $__wasi_fd_write (param i32 i32 i32 i32) (result i32)))

(func $__lone_rt_print (param $ptr i32) (param $len i32)
  ;; Store buffer pointer in ciovec_t[0].buf
  (i32.store (i32.const 0) (local.get $ptr))
  ;; Store buffer length in ciovec_t[0].len
  (i32.store (i32.const 4) (local.get $len))
  ;; Call WASI fd_write(fd=1 (stdout), iovs=0, iovs_len=1, nwritten=8)
  (drop (call $__wasi_fd_write (i32.const 1) (i32.const 0) (i32.const 1) (i32.const 8)))
)
```

This design eliminates external runtime libraries: the output `.wat` file is 100% self-contained and immediately executable.
