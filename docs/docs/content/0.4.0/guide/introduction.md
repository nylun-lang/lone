---
title: "Introduction"
order: 1
---

# Introduction to Lone

**Lone** is a modern, statically typed systems language designed from the ground up to compile to WebAssembly (Wasm) with zero runtime dependencies.

Lone combines the safety and rigor of static typing with a clean, expressive syntax and a predictable compilation pipeline:

```
[Lone Source (.lone)]
         │
         ▼  (lone-lang / PEG parser)
    [Raw AST]
         │
         ▼  (lone-alyzer / multi-pass analysis)
  [Typed IR (air)]   <── Enforces types, naming, & drop rule
         │
         ▼  (falone / lowering)
   [Falone MIR]      <── Control-flow graphs & linear memory
         │
         ▼  (falone / transpile)
 [WebAssembly (WAT)] <── Standalone WASI or Web target
```

## Key Highlights

- **Predictable Type System**: Type inference (`infer`), primitive integer/float/bool types, fixed-size lists (`[N]T`), dynamic lists (`[]T`), and object literals.
- **Consume-or-Drop Invariant**: Functions returning values must have their return values consumed by a binding or explicitly discarded with the `drop` keyword, eliminating accidental ignored results.
- **Dual Compilation Targets**:
  - **Standalone WASI (`wasm`)**: Compiles with inlined WASI `fd_write` syscalls, capable of running in Node.js, Wasmtime, Wasmer, or edge runtimes.
  - **Web (`web`)**: Compiles with imports for browser environments.
- **Auto-Generated Runtime Glue**: Built-in `--with-adapter node` and `--with-adapter browser` flags generate complete JavaScript/TypeScript loader glue with UTF-8 linear memory decoding.
- **Intrinsic Functions**: High-performance built-in operations prefixed with `#` (e.g. `#print`), lowered directly into native WebAssembly instructions or system calls.
