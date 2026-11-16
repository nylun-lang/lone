---
title: "Lone Debug Toolchain"
order: 4
---

# Lone Debug Toolchain (`lone debug`)

The `lone debug` command suite provides compile-time semantic inspection and introspection for Lone projects. It allows developers and IDE tools to deeply inspect scopes, symbols, memory layouts, trait contracts, and storage allocations without running a debugger or executing binaries.

---

## Command Overview

```bash
# Scope Overview (Dump)
lone debug dump <scope>
lone debug <scope>                  # Shorthand

# Symbol Inspection (Whatis)
lone debug whatis <scope> <symbol>
lone debug <scope> <symbol>         # Shorthand
lone debug inspect <scope> <symbol> # Alias
```

### Options & Flags

| Flag | Description |
| :--- | :--- |
| `--json` | Output results in structured JSON format for tooling and IDEs |

---

## Specifying Scopes

A `<scope>` represents the compilation unit or module namespace being queried:

1. **Source Files**: Relative or absolute path to a `.lone` source file:
   ```bash
   lone debug main.lone
   lone debug src/game.lone
   ```
2. **Standard Library Namespaces**: Any standard library module starting with `:`:
   ```bash
   lone debug :std:io
   lone debug :std:builtin
   ```

---

## Specifying Symbols

Symbols can be queried globally within the scope, or scoped using `->`, `:`, or `.`:

| Syntax | Target | Example |
| :--- | :--- | :--- |
| `<name>` | Top-level symbol | `Stdout`, `Point`, `main` |
| `<fn>-><var>` | Variable / parameter inside a function | `main->guess`, `run->total` |
| `<struct>:<field>` | Field within a struct | `Stdout:fd`, `Point:x` |
| `<struct>:<method>` | Method within a struct | `Stdout:write`, `Point:dist` |

---

## 1. Scope Overview (`lone debug dump`)

Dumps an overview of all declarations, imports, and definitions in a file or namespace:

```bash
lone debug :std:io
```

### Example Output

```text
=== Scope Overview: :std:io ===

[Structs]
  • Ciovec (size: 8 bytes, align: 4 bytes)
  • Stderr (size: 4 bytes, align: 4 bytes)
  • Stdin (size: 4 bytes, align: 4 bytes)
  • Stdout (size: 4 bytes, align: 4 bytes)

[Traits]
  • ReaderTrait (1 methods)
  • WriterTrait (1 methods)

[Functions]
  • int_to_str(n: i32) str
  • parse_int(s: str) i32
  • print(s: str) void
  • println(s: str) void
  • println_int(n: i32) void
  • read_line() str

[Globals]
  • STDERR_FD: i32
  • STDIN_FD: i32
  • STDOUT_FD: i32

[Imports]
  • sys_write, sys_read, sys_close, sys_exit, sys_random from :std:sys
```

---

## 2. Symbol Inspection (`lone debug whatis`)

### A. Struct Inspection (Memory Layout & Traits)

Inspecting a struct displays its byte size, alignment, field offsets, constructors, methods, and implemented trait contracts:

```bash
lone debug :std:io Stdout
```

```text
=== Struct: Stdout ===
Defined: :std:io:18:1
Layout:  size: 4 bytes, align: 4 bytes

Fields:
  • fd: i32 (offset: +0, size: 4 bytes)

Constructors:
  • make(fd: i32) Stdout

Methods:
  • write(self: ref<Stdout>, buf: str) i32

Implemented Traits:
  • WriterTrait
      ↳ write(self, buf: str) i32 (matches contract)
```

### B. Trait Inspection (Contracts & Implementors)

Inspecting a trait displays required method signatures and every concrete struct in the scope that implements it:

```bash
lone debug :std:io WriterTrait
```

```text
=== Trait: WriterTrait ===
Defined: :std:io:27:1

Required Methods:
  • write(self, buf: str) i32

Implemented By:
  • Stderr
  • Stdout
```

### C. Variable & Storage Inspection

Variables are inspected with storage locations, initialization states, and address-taken tracking:

```bash
lone debug main.lone main->guess
```

```text
=== Variable: guess ===
Enclosing: Function 'main'
Defined:   main.lone:42:5
Type:      i32 (inferred)
Init:      Initialized
Storage:   datastack frame (slot)
Address:   normal (not address-taken)
```

#### Address-Taken Tracking (`&raw`)
When a variable has its address taken via `&raw x`, the compiler marks it:
```text
Storage:   linear memory (address-taken: requires stack frame offset)
Address:   AddressTaken (passed to ptr<T> / linear memory)
```

### D. Function Inspection

Inspecting a function displays parameters, return type, visibility, and local variable declarations:

```bash
lone debug :std:io println_int
```

```text
=== Function: println_int ===
Defined:    :std:io:58:1
Visibility: public
Signature:  fn println_int(n: i32) void

Parameters:
  • n: i32

Local Variables:
  • s: str
```

---

## 3. JSON Output Mode (`--json`)

Adding `--json` outputs a clean machine-readable representation ideal for IDE language servers, build diagnostics, and custom scripts:

```bash
lone debug --json :std:io Stdout
```

```json
{
  "kind": "struct",
  "name": "Stdout",
  "pos": { "line": 18, "col": 1, "offset": 320 },
  "struct": {
    "size": 4,
    "align": 4,
    "fields": [
      { "name": "fd", "type": "i32", "offset": 0, "size": 4 }
    ],
    "methods": [
      { "name": "write", "signature": "fn write(self: ref<Stdout>, buf: str) i32" }
    ],
    "traits": [
      {
        "trait_name": "WriterTrait",
        "methods": ["write"]
      }
    ]
  }
}
```
