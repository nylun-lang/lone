# Architectural Evaluation: C-Lone-Runtime, WASI, and WebAssembly Linkage Strategies

**Author**: Lone Architectural Team  
**Date**: 2026-11-01  
**Status**: Active RFC & Evaluation  
**Subject**: Evaluation of `packages/c-lone-runtime`, WASI integration, static vs dynamic linking, and dual-target deployment (Web vs Standalone).

---

## 1. Executive Summary

The proposal to author Lone's runtime in C (`packages/c-lone-runtime`) targeting the WebAssembly System Interface (WASI `wasi_snapshot_preview1`) and link it with Falone's output addresses a classic language implementation challenge: **how to supply built-in runtime intrinsics (`#print`, `#panic`, memory allocation) without reimplementing low-level OS/WASI primitives by hand in the compiler**.

This document evaluates:
1. The technical mechanics of linking `lone_rt_print.c` with Falone's generated WAT/Wasm.
2. The core challenges (Memory sharing, relocatable Wasm linking, and Browser vs Standalone compatibility).
3. Four concrete alternative architectures, concluding with a recommended hybrid approach that keeps the Lone toolchain dependency-free while accommodating both web and standalone targets.

---

## 2. Technical Evaluation of the Current Proposal

### 2.1 The Current Implementation (`packages/c-lone-runtime`)
The existing prototype in `packages/c-lone-runtime/core/lone_rt_print.c`:
```c
__attribute__((import_module("wasi_snapshot_preview1"), import_name("fd_write")))
int __wasi_fd_write(int fd, const void *iovs, int iovs_len, int *nwritten);

typedef struct {
    const char *buf;
    int len;
} ciovec_t;

__attribute__((export_name("__lone_rt_print")))
void __lone_rt_print(const char *str, int len) {
    ciovec_t iov = { .buf = str, .len = len };
    int written = 0;
    __wasi_fd_write(1, &iov, 1, &written);
}
```
And the compilation in `justfile`:
```sh
clang --target=wasm32-wasip1 -O3 -nostdlib \
    -Wl,--no-entry -Wl,--export-all \
    -c core/lone_rt_print.c -o targets/wasm-wasi32/lone_rt_print.wasm
```

### 2.2 Strengths of this Strategy
1. **Leverages LLVM / Clang Optimization**: C code compiled with `-O3` produces highly optimized WebAssembly bytecode.
2. **Access to C Ecosystem**: Future runtime facilities (e.g., `dlmalloc` or `mimalloc`, UTF-8 decoding, floating-point string conversions) can leverage tested C implementations rather than being written from scratch.
3. **Decoupled Development**: The runtime team can iterate on C runtime functions independently from compiler development in Falone.

---

### 2.3 The Critical Challenges & Constraints

#### Challenge 1: WebAssembly Linear Memory Isolation
In WebAssembly, each instantiated module owns its **own isolated linear memory** by default:
- If `app.wasm` (compiled from Lone) creates a string at memory address `0x1000` and calls `__lone_rt_print(0x1000, 5)` in `runtime.wasm`, `runtime.wasm` will read offset `0x1000` in **its own linear memory**, reading uninitialized or garbage memory!
- **Requirement**: `app.wasm` and `runtime.wasm` MUST share a single imported memory:
  ```wat
  ;; Both modules must declare:
  (import "env" "memory" (memory 1))
  ```
  Furthermore, the C runtime uses a shadow stack in linear memory for local variables (such as `&iov` and `&written` in `lone_rt_print`). If memory is shared, the C stack pointer (`__stack_pointer`) and the Lone heap/stack must not collide!

#### Challenge 2: "Linking" Clang Outputs with Falone's WAT Output
In traditional systems, `.c` compiles to `.o` and `ld` links them. In WebAssembly:
- `clang -c` produces an unlinked ELF-like WebAssembly relocatable object file with custom sections (`linking`, `reloc.CODE`, `reloc.DATA`). It cannot be run directly.
- Falone produces human-readable **WebAssembly Text format (`WAT`)**.
- You **cannot concatenate or append** a binary `.wasm` file to a text `.wat` file.
- To link them statically:
  1. Falone would need to emit `.wasm` object files with relocation headers and call `wasm-ld` (LLVM linker), requiring `clang` and `lld` installed on the host machine.
  2. Or Falone would need an internal tool like `wasm-merge` (from Binaryen).

#### Challenge 3: WASI is NOT Natively Supported in Web Browsers
- **Standalone Engines (Wasmtime, Wasmer, WasmEdge, Node.js `--wasi`)**:
  Provide `wasi_snapshot_preview1` host imports natively. `fd_write` writes directly to stdout/console.
- **Web Browsers (Chrome, Firefox, Safari)**:
  Have **NO native WASI support**. Running a WASI module in a browser requires:
  - Bundling a 50KB+ JavaScript WASI polyfill (such as `@wasmer/wasi` or browser polyfills).
  - Web developers expect to instantiate Wasm simply with `WebAssembly.instantiate(bytes, { env: { print: ... } })`.

---

## 3. Alternative Architectural Strategies

| Strategy | Description | Build Dependencies | Web vs Standalone | Complexity |
| :--- | :--- | :--- | :--- | :---: |
| **Strategy 1: C Runtime + Static Linker (`wasm-ld`)** | Compile C to `.o`, compile Lone to `.o`, link via `wasm-ld` | `clang`, `wasm-ld` | Requires browser WASI polyfill | High |
| **Strategy 2: Multi-Module Dynamic Link (Host-Loaded)** | Host instantiates `runtime.wasm` & `app.wasm` sharing memory | `clang` (runtime only) | Separate loaders for web & CLI | Medium |
| **Strategy 3: Pure Inlined WAT Runtime** | Small runtime primitives emitted directly in WAT by Falone | **Zero** (Pure Go compiler) | Dual-target profile (`--target=wasi`, `--target=web`) | Low / Clean |
| **Strategy 4: Tiered Hybrid (Recommended)** | Inlined WAT for core primitives + optional C runtime module for heavy components | Zero for basic apps; `clang` for heavy runtime | Native WASI + Native Web | Balanced & Future-Proof |

---

## 4. Deep-Dive on Alternative Strategies

### Strategy 1: C Runtime + Static Linker (`wasm-ld`)
- **Workflow**:
  1. `packages/c-lone-runtime` compiles to `lone_rt.a` (static archive) using `clang` and `llvm-ar`.
  2. Falone compiles Lone code and emits a Wasm object file with symbol relocations.
  3. `wasm-ld lone_rt.a app.o -o output.wasm --no-entry --export-all`.
- **Verdict**: Standard for C/C++ or Rust toolchains, but forces every user of `falone` to install LLVM, `clang`, and `lld`.

---

### Strategy 2: Multi-Module Linking via Host Instantiation
- **Workflow**:
  1. `c-lone-runtime` compiles once into a standalone `lone_rt.wasm`.
  2. Falone compiles Lone code into `app.wat`, which imports `(import "lone_rt" "__lone_rt_print" (func ...))`.
  3. The runner/host instantiates both modules:
     ```js
     // Browser / Standalone JS Runner:
     const memory = new WebAssembly.Memory({ initial: 1 });
     const rt = await WebAssembly.instantiate(rtBytes, { wasi_snapshot_preview1: wasiObj, env: { memory } });
     const app = await WebAssembly.instantiate(appBytes, { lone_rt: rt.instance.exports, env: { memory } });
     app.instance.exports.main();
     ```
- **Verdict**: Clean separation of concerns. Requires runner orchestration.

---

### Strategy 3: Pure Inlined WAT Runtime (No C Dependency)
For lightweight intrinsics like `#print`, WebAssembly needs only a few instructions to invoke WASI `fd_write` directly in WAT!

#### WASI Standalone WAT Header:
```wat
(module
  (import "wasi_snapshot_preview1" "fd_write" 
    (func $__wasi_fd_write (param i32 i32 i32 i32) (result i32)))
  (memory (export "memory") 1)

  ;; Internal WAT implementation of print:
  (func $__lone_rt_print (param $str_ptr i32) (param $str_len i32)
    ;; Store ciovec at scratch address 0: { buf: $str_ptr, len: $str_len }
    (i32.store (i32.const 0) (local.get $str_ptr))
    (i32.store (i32.const 4) (local.get $str_len))
    ;; Call fd_write(fd=1 (stdout), iovs=0, iovs_len=1, nwritten=8)
    (drop (call $__wasi_fd_write (i32.const 1) (i32.const 0) (i32.const 1) (i32.const 8)))
  )
  ...
)
```

#### Browser Target WAT Header:
```wat
(module
  (import "env" "print" (func $__lone_rt_print (param i32 i32)))
  (memory (export "memory") 1)
  ...
)
```
- **Verdict**: Completely self-contained. Falone produces a standalone, runnable `.wat` with zero external dependencies.

---

### Strategy 4: The Recommended Hybrid Roadmap

For Lone's immediate and long-term roadmap, a **Tiered Hybrid Architecture** is recommended:

```
┌────────────────────────────────────────────────────────┐
│                        FALONE                          │
├──────────────────────────┬─────────────────────────────┤
│   Target: Standalone     │        Target: Web          │
│   (WASI preview 1)       │        (Browser env)        │
├──────────────────────────┼─────────────────────────────┤
│ Emits wasi fd_write WAT  │ Emits env.print import WAT  │
└──────────────────────────┴─────────────────────────────┘
                             │
                             ▼ (Future Heavy Extensions)
┌────────────────────────────────────────────────────────┐
│               packages/c-lone-runtime                  │
│  (For heavy facilities: Allocators, GC Anchors, Loum)  │
│  Linked via Multi-Module or merged with wasm-merge     │
└────────────────────────────────────────────────────────┘
```

1. **Phase 1 (Immediate - Pure Toolchain Independence)**:
   - Implement intrinsic `#print` directly in Falone's WAT emitter with support for two profiles:
     - `--target=wasi` (default for CLI / standalone runtimes like `wasmtime` or `wasmer`).
     - `--target=web` (for web browsers, importing `(import "env" "print" ...)`).
   - This makes `falone compile` immediately usable out-of-the-box without requiring `clang` or external build tools.

2. **Phase 2 (Medium Term - C Runtime Integration)**:
   - Retain `packages/c-lone-runtime` for non-trivial features (memory allocation, Loum memory anchor management, string formatting).
   - Use the **Host-linked multi-module pattern** or incorporate `wasm-merge` when distributing pre-bundled standalone binaries.

---

## 5. Conclusion & Action Items

- The user's idea of writing runtime intrinsics in C targeting WASI is sound and standard practice for systems compilers.
- However, attempting to statically "append" raw binary `.wasm` from Clang into Falone's `.wat` output is fundamentally blocked by the binary/text mismatch, relocation section requirements, and memory isolation.
- By adopting **Strategy 4 (Tiered Hybrid)**, Lone gains instant out-of-the-box execution on both WASI CLI hosts and Web browsers, while reserving `packages/c-lone-runtime` for larger, complex runtime features as the compiler matures.
