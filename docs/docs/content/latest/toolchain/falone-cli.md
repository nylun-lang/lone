---
title: "Lone CLI Reference"
order: 1
---

# Lone CLI Reference

`lone` is the official compiler and developer toolchain CLI for the Lone programming language.

## Installation

```bash
go build -o lone ./cmd/lone
```

## Subcommands

### `lone build`
Compiles a Lone source file or project to WebAssembly Text (`.wat`) or binary WebAssembly (`.wasm`):

```bash
lone build <file.lone> [-o output.wat] [-t wasi|web]
```

### `lone run`
Compiles a Lone program and immediately executes it using the embedded Wasmtime runtime:

```bash
lone run <file.lone>
```

### `lone check`
Performs static analysis, multi-pass type evaluation, safety invariant verification, and naming convention validation without emitting code:

```bash
lone check <file.lone>
```

### `lone debug`
Deeply inspects scopes, symbols, memory layout, traits, and storage locations:

```bash
lone debug <scope>                  # Dumps all symbols in scope
lone debug <scope> <symbol>         # Inspects a specific symbol
lone debug dump <scope>             # Explicit dump subcommand
lone debug whatis <scope> <symbol>  # Explicit whatis subcommand
```

See the [Lone Debug Toolchain](/content/latest/toolchain/lone-debug) guide for full details.

### `lone lsp`
Starts the official Lone Language Server Protocol daemon on standard I/O:

```bash
lone lsp
```

---

## Global & Build Flags

| Flag | Shorthand | Description | Default |
| :--- | :--- | :--- | :--- |
| `--output` | `-o` | Output file path for compiled WebAssembly | stdout / alongside file |
| `--target` | `-t` | Compilation target (`wasi` or `web`) | `wasi` |
| `--mir` | `-m` | Output Falone MIR assembly text | `false` |
| `--wat` | `-w` | Output WebAssembly Text | `false` |
| `--json` | `-j` | Output diagnostics or debug info in JSON format | `false` |
