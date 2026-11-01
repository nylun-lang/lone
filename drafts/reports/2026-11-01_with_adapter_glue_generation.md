# Implementation Report: `--with-adapter` Runtime Glue Generation

**Date**: 2026-11-01  
**Branch**: `feat/better-type`  
**Scope**: `packages/falone/adapter`, `packages/falone/cmd`

---

## 1. Executive Summary

We implemented `--with-adapter` (with alias `--with-adater`) for generating runtime JavaScript glue code when compiling Lone source to WebAssembly:
1. **New Package `falone/adapter`**:
   - `AdapterNode`: Generates a Node.js CommonJS module providing:
     - `createEnv(() => instance.exports.memory, hooks)` providing `print(ptr, len)`.
     - Linear memory UTF-8 string decoding via `TextDecoder` and `process.stdout.write`.
     - `instantiate(wasmInput, options)` and `run(wasmInput, options)` functions.
     - CLI execution support: `node <adapter.js> [file.wasm]`.
   - `AdapterBrowser`: Generates a modern ES module providing:
     - Streaming WebAssembly instantiation (`WebAssembly.instantiateStreaming` with `fetch(wasmSource)`).
     - Linear memory UTF-8 string decoding via `TextDecoder` and `console.log` (overridable via `hooks.print`).
     - `instantiate(wasmSource, options)` and `run(wasmSource, options)`.
2. **CLI Integration**:
   - Added `--with-adapter <node | browser>` (and hidden alias `--with-adater`) as a persistent flag on `compileCmd` and `compile web`.
   - When printing to terminal: emits `=== RUNTIME ADAPTER (<kind>) ===`.
   - When saving to file (`-o <output>`):
     - If `-o file.wat` or `-o file.wasm`: writes WAT to `file.wat` and adapter to `file.adapter.js`.
     - If `-o file.js`: writes adapter to `file.js` and WAT to `file.wat`.

---

## 2. End-to-End Execution Verification

1. Compiled `packages/falone/sample/hello.lone` with Node adapter:
   ```bash
   falone compile web packages/falone/sample/hello.lone -o /tmp/hello_web.wat --with-adapter node
   ```
   Output:
   ```
   Successfully compiled to /tmp/hello_web.wat and generated node adapter /tmp/hello_web.adapter.js
   ```

2. Assembled `/tmp/hello_web.wat` into binary `/tmp/hello_web.wasm`.

3. Executed the generated adapter directly:
   ```bash
   $ node /tmp/hello_web.adapter.js /tmp/hello_web.wasm
   Hello from Lone!
   ```
   The generated runtime adapter successfully instantiated the WebAssembly module, decoded string data from linear memory via `TextDecoder`, executed `main()`, and printed `Hello from Lone!` to stdout.

---

## 3. Test Suite Status

All 30 test suites across all packages pass:
- `lone-lang`: 12 test suites passing
- `lone-alyzer`: 8 test suites passing
- `falone`: 10 test suites passing (lowering, transpile, and adapter suites)
