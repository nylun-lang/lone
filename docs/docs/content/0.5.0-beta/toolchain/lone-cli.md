---
title: "Lone CLI Reference"
order: 1
---

# Lone CLI Reference

`lone` is the official compiler and developer toolchain CLI for the Lone programming language. It manages project verification, compilation to WebAssembly, execution, debugging, and language server operations.

---

## Installation & Building

```bash
# Build the unified Lone CLI
go build -o lone ./cmd/lone

# Build the Greenfield next-gen compiler driver
go build -o lone2 ./cmd2/lone
```

---

## Core Subcommands

### 1. `lone build`
Compiles a Lone source file or project to WebAssembly Text (`.wat`) or binary WebAssembly (`.wasm`):

```bash
lone build <file.lone> [-o output.wat] [-t wasi|web]
```

#### Flags:
- `-o, --output <path>`: Specifies output `.wat` or `.wasm` file path.
- `-t, --target <wasi|web>`: Sets execution target (default: `wasi`).
- `-O`: Enables WebAssembly peephole optimization passes.

---

### 2. `lone check`
Runs static analysis, multi-pass type evaluation, safety invariant verification (Consume-or-Drop rule), and naming convention verification without emitting code:

```bash
lone check <file.lone>
```

Output:
```text
Checked main.lone: 0 errors
```

---

### 3. `lone run`
Compiles and immediately executes a Lone program in-process using the embedded Wasmtime runtime:

```bash
lone run <file.lone>
```

---

### 4. `lone debug`
Compile-time semantic inspection and introspection for Lone scopes and symbols:

```bash
lone debug <scope>                  # Dumps all symbols in scope
lone debug <scope> <symbol>         # Inspects a specific symbol
lone debug dump <scope>             # Explicit dump subcommand
lone debug whatis <scope> <symbol>  # Explicit whatis subcommand
```

Supports `--json` for IDE and editor integrations. See the [Lone Debug Toolchain](./lone-debug.md) guide for details.

---

### 5. `lone lsp`
Starts the official Language Server Protocol daemon over standard I/O:

```bash
lone lsp
```

---

### 6. `lone2 parse` (Greenfield Toolchain)
Validates source syntax and prints declaration statistics using the Greenfield Pratt precedence parser:

```bash
lone2 parse <file.lone>
```

Output:
```text
Successfully parsed calc.lone: 3 top-level declaration(s)
```
