# Report: Evaluation of c-lone-runtime, WASI Integration, and Web/Standalone Targets

**Date**: 2026-11-01  
**Author**: AI Assistant & Lone Team  
**Branch**: `feat/better-type`  
**Tags**: `[Ai-assisted]`, `falone`, `runtime`, `wasi`, `c-lone-runtime`, `architecture`, `wat`

---

## 1. Overview & Context

This report provides a formal evaluation of the strategy to supply Lone's built-in intrinsics (such as `#print`) via an external C runtime (`packages/c-lone-runtime`) compiled to WebAssembly via WASI (`wasm32-wasip1`), and explores how it integrates with `Falone` across both **Web** and **Standalone Binary** execution targets.

---

## 2. Key Findings & Technical Evaluation

### 2.1 The Feasibility of Appending / Linking
1. **Format Incompatibility**: Falone currently emits WebAssembly Text format (`WAT`), while `clang -c` outputs relocatable binary object files (`.wasm` containing custom ELF-like relocation sections). Binary Wasm cannot simply be concatenated or appended to WAT text.
2. **Memory Isolation**: In WebAssembly, separate modules possess distinct linear memories. If `falone` output calls `__lone_rt_print(ptr, len)` in a separate runtime module, they must share a single imported linear memory (`(import "env" "memory" ...)`), otherwise the runtime will attempt to read the string buffer from its own uninitialized memory space.
3. **Shadow Stack Collisions**: `clang` generates code that relies on a shadow stack in linear memory for locals (such as `ciovec_t iov` in `lone_rt_print.c`). The stack pointer (`__stack_pointer`) must be coordinated between Falone and the C runtime.

### 2.2 Web vs. Standalone Execution
- **Standalone CLI (Wasmtime, Wasmer, WasmEdge, Node.js `--wasi`)**:
  - WASI `wasi_snapshot_preview1` is natively supported. `fd_write` writes directly to standard output.
- **Web Browsers (Chrome, Safari, Firefox)**:
  - Browsers do **not** have native WASI support. Running WASI code in a browser requires polyfilling `wasi_snapshot_preview1.fd_write` in JavaScript (e.g. mapping `fd_write` to `console.log`), adding overhead.
  - Idiomatic web Wasm modules prefer importing a lightweight host function: `(import "env" "print" (func (param i32 i32)))`.

---

## 3. Evaluated Architectural Alternatives

| Approach | Build Tooling | Standalone (CLI) | Web Browser | Recommended Use |
| :--- | :--- | :--- | :--- | :--- |
| **A. Pure WAT Inlining** | Zero (Pure Go) | Native WASI `fd_write` | Direct `env.print` | **Immediate / Core Intrinsics (`#print`, `#panic`)** |
| **B. Host Multi-Module Linking** | `clang` (runtime once) | WASI runner config | JS loader config | **Modular Services & Plugins** |
| **C. Static Linker (`wasm-ld`)** | `clang`, `lld`, `wasm-ld` | Single `.wasm` binary | Requires JS WASI shim | **Heavy Native C libraries** |
| **D. Tiered Hybrid** | Zero for basics, Clang for heavy | Full WASI | Clean web imports | **Recommended Architecture** |

---

## 4. Recommended Architectural Roadmap

1. **Phase 1: Inlined Core Intrinsics in Falone (Zero External Tooling)**
   - Emit `#print` directly in Falone's WAT generator.
   - Support target flags in `falone compile`:
     - `--target=wasi` (default): Emits inline WASI `fd_write` setup. Runs on any WASI runtime (`wasmtime`, `wasmer`, Node.js).
     - `--target=web`: Emits `(import "env" "print" ...)`. Runs in browsers with a one-line JS loader (`console.log`).
2. **Phase 2: Use `c-lone-runtime` for Heavy Services**
   - Reserve `packages/c-lone-runtime` for complex future facilities: memory allocators (`dlmalloc`), Loum memory anchor tracking, and string formatting.
   - Link via WebAssembly Multi-Module instantiation or bundle using `wasm-merge` for standalone distribution.

---

## 5. References & Documents
- Detailed RFC: [drafts/research/c_runtime_and_wasi_evaluation.md](../../drafts/research/c_runtime_and_wasi_evaluation.md)
- Runtime Source: [packages/c-lone-runtime/core/lone_rt_print.c](../../packages/c-lone-runtime/core/lone_rt_print.c)
