---
title: "Getting Started"
order: 2
---

# Getting Started with Lone

This guide covers building the Lone toolchain from source, writing your first program, and compiling it to WebAssembly.

## Prerequisites

- **Go 1.22+**: Used by `lone-lang`, `lone-alyzer`, and `falone`.
- **Node.js 18+** (or Wasmtime / Wasmer): For running generated WebAssembly modules.

## Building the Toolchain

Clone the repository and build the `falone` CLI:

```bash
git clone https://github.com/lone-lang/lone.git
cd lone

# Build the falone compiler
go build -o falone ./packages/falone/cmd
```

## Your First Lone Program

Create a file named `hello.lone`:

```rust
fn main() void {
    #print("Hello from Lone!\n")
}
```

## Compiling for Standalone WASI

Compile your program targeting standalone WebAssembly with the WASI interface:

```bash
./falone compile wasm hello.lone -o hello.wat
```

This generates `hello.wat`, featuring an inlined WASI preview 1 runtime that directly outputs strings to stdout via linear memory.

## Running with Node.js WASI

You can run your compiled `.wat` file directly using Node.js:

```javascript
const { WASI } = require("node:wasi");
const fs = require("node:fs");

// Assemble or load wasm binary
const wasmBuffer = fs.readFileSync("hello.wasm");
const wasi = new WASI({ version: "preview1" });

WebAssembly.instantiate(wasmBuffer, wasi.getImportObject()).then(({ instance }) => {
  wasi.start(instance); // Prints: Hello from Lone!
});
```

## Compiling for the Web with Adapters

To run inside a browser or Node without full WASI permissions, compile with the `web` target and generate the runtime adapter:

```bash
# Generate WebAssembly Text and Node.js adapter
./falone compile web hello.lone -o hello_web.wat --with-adapter node

# Execute the generated adapter
node hello_web.adapter.js hello_web.wasm
```
