---
title: "Falone CLI Reference"
order: 1
---

# Falone CLI Reference

`falone` is the official compiler and developer toolchain CLI for Lone.

## Installation

```bash
go build -o falone ./packages/falone/cmd
```

## Subcommands

### `falone compile`
Compiles Lone source files to Falone MIR and WebAssembly Text (WAT). Defaults to standalone WASI target.

```bash
falone compile <file.lone> [flags]
```

### `falone compile wasm`
Explicitly targets standalone WebAssembly using the WASI preview 1 system interface:

```bash
falone compile wasm <file.lone> [-o output.wat]
```

### `falone compile web`
Explicitly targets Web and JavaScript browser environments:

```bash
falone compile web <file.lone> [--with-adapter node|browser] [-o output.wat]
```

### `falone analyze`
Performs static analysis and type checking using `lone-alyzer` without compiling to WebAssembly:

```bash
falone analyze <file.lone> [flags]
```

## Flags

| Flag | Shorthand | Description |
| :--- | :--- | :--- |
| `--mir` | `-m` | Output Falone MIR assembly text |
| `--wat` | `-w` | Output WebAssembly Text (WAT) |
| `--output` | `-o` | Write WebAssembly output to a file |
| `--target` | `-t` | Compilation target (`wasi` or `web`) |
| `--with-adapter` | | Generate runtime glue adapter (`node` or `browser`) |
| `--nodes` | `-n` | Print AST / IR node tree (in `analyze`) |
| `--json` | `-j` | Print node tree as formatted JSON (in `analyze`) |
