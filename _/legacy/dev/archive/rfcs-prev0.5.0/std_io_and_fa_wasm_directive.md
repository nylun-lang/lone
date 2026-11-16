# RFC: Lone Module Import System & Standard I/O (`std:io`)

- **Author:** Lone Architecture Team & AI Pair
- **Date:** 2026-11-02
- **Status:** Draft / Active RFC
- **Related Specs:** [SPEC.md](SPEC.md), [c_runtime_and_wasi_evaluation.md](c_runtime_and_wasi_evaluation.md), [architecture_v2.md](architecture_v2.md)
- **Primary References:** `_local/std_lib_impl`, `libraries/std/std.lone`, `pkg/lone-runtime/`

---

## 1. Context & Motivation

### 1.1 The Runtime Dilemma
Lone is designed to compile directly to **WebAssembly (Wasm)** as its native target. To build a standard I/O library (`std:io`), programs must perform operations like writing to `stdout`, reading from `stdin`, or interacting with files. In WebAssembly, these operations do not exist in the base instruction set—they require host system calls provided by **WASI (WebAssembly System Interface)** or web host imports.

Historically in the project, two approaches were attempted:
1. **The External C Runtime (`libraries/c-wasi-runtime` / `packages/c-lone-runtime`)**:
   - *Approach*: Write WASI wrappers in C (`lone_rt_print.c`) and compile them to Wasm using `clang --target=wasm32-wasip1 -nostdlib`.
   - *Problem*: Demands that the user or CI environment install LLVM/Clang and `wasm-ld`. It introduces dual linear-memory management headaches, stack pointer collisions (`__stack_pointer`), and relocatable object file linking complexity. Lone was forced to ask: *"Why does Lone require C just to print or read a line?"*
2. **Inlined Go Compiler Templates (`pkg/lone-runtime`)**:
   - *Approach*: The Go compiler internally maintains hardcoded string templates of WebAssembly Text (WAT) instructions (such as storing an `iovec` into scratchpad memory and calling `fd_write`).
   - *Problem*: Doesn't scale. Every standard library function would need to be hand-written inside the Go compiler engine, leaving the target standard library (`lib/std/`) hollow and unable to be authored in Lone itself.
3. **Exposing Raw Inline Wasm Assembly in User Code**:
   - *Approach*: Allowing blocks like `asm { (call $fd_read ...) }` inside `.lone` files.
   - *Problem*: Violates Lone's core design invariant: *"Easy to read, Easy to write, and Hard to misunderstand."* It leaks low-level stack/register mechanics into high-level Lone code and makes code unportable across targets.

### 1.2 The Breakthrough: Unifying Directives & The Formal Import System
The note in `_local/std_lib_impl` outlined an elegant alternative:
```lone
why does lone require C 

```lone
---fa:wasm
require wasi:cli
---
fn read_line() str {
    cli.stdout
}
```

By evolving this from an ad-hoc `require` into **Lone's formal `import` grammar**, we unify standard library imports, user package imports, and low-level WebAssembly/WASI capability bindings under a single, cohesive syntax.

---

## 2. Formal Import Syntax & Namespace Grammar

Lone's import system follows the language's core invariants:
- **IL1 (Explicit over implicit)**: Every reference must be unambiguous about *what it is* and *where it originated*.
- **IF1 (Syntax-level feedback)**: Punctuation reinforces grammar rather than compensating for ambiguity.

### 2.1 The Source Prefix Rule: Namespaces vs. Relative Files

The first character of the import source specifies its resolution mechanism with zero ambiguity:

| Prefix | Category | Examples | Resolution Mechanism |
| :---: | :--- | :--- | :--- |
| `:` | **Namespace / Package** | `:std:io`<br>`:std:math`<br>`:wasi:cli` | Resolved via standard library root (`lib/std/`) or package registry (`lone.mod`). |
| `.` | **Relative File / Module** | `.std`<br>`.utils`<br>`..common` | Resolved relative to the current file's directory on the filesystem. |

#### Examples:
```lone
-- Importing from a global / standard library namespace:
import .readline from :std:io

-- Importing from a local file at the same directory level (e.g., ./std.lone):
import .foo from .std

-- Importing from a parent directory module (e.g., ../config.lone):
import .Config from ..config
```

### 2.2 Flat Member Projection: Leading Dot (`.`) & No Braces (`{}`)

Lone deliberately adopts a **flat, brace-free import syntax**:
```lone
import .readline from :std:io
import .foo, .bar from :somewhere
import .Ordering, .Comparable from :std:math
```

**Why Flat Syntax without `{}`?**
1. **Preserving `{}` Semantics**: In Lone, curly braces `{}` are strictly reserved for executable code blocks (functions, control flow) and record/struct type definitions. Keeping imports flat avoids overloading `{}` as punctuation grouping.
2. **Minimal Syntax Noise**: Comma-delimited symbol projections with leading dots (`.foo, .bar`) provide high contrast and clean visual scanning without closing brackets.
3. **Progressive Left-to-Right Flow (IF2)**:
   ```lone
   import .read_line, .print, .println from :std:io
   ```
   Or across multiple lines for longer lists:
   ```lone
   import
       .stdin,
       .stdout,
       .stderr
   from :wasi:cli
   ```

### 2.3 Import Forms Supported

1. **Single Member Import**:
   ```lone
   import .readline from :std:io
   ```
2. **Multi-Member Import (Flat Comma-Separated)**:
   ```lone
   import .read_line, .print, .println from :std:io
   ```
3. **Namespace / Module Alias Import**:
   ```lone
   import :std:io as io
   -- Usage: io.println("Hello, World!")
   ```
4. **Relative Module Import**:
   ```lone
   import .helper, .validate from .my_module
   ```

---

## 3. Host Capability Imports via `---fa:wasm`

Rather than inventing a separate keyword (like `require`), low-level capability bindings use the exact same flat `import` grammar, guarded by the compiler directive `---fa:wasm`.

### 3.1 Directive Specification
Inside a `---fa:wasm` block, a library author imports WebAssembly host interfaces:

```lone
---fa:wasm
import .stdout, .stdin from :wasi:cli
---
```

Or for fine-grained WASI Preview 1 POSIX syscalls:
```lone
---fa:wasm
import .fd_write, .fd_read from :wasi:snapshot_preview1
---
```

### 3.2 Raw Memory Addresses (`&raw`) vs. Managed References (`&`)

WASI functions operate directly on 32-bit linear memory offsets (`i32`). To support this without implicit magic, Lone establishes an explicit distinction between managed references and raw pointers:

| Expression | Type | Role in Runtime |
| :---: | :---: | :--- |
| `&foo` | `ref<T>` | Safe, compiler-managed reference tracked by escape analysis. |
| `&raw foo` | `ptr<T>` | Raw byte offset in WebAssembly linear memory (`i32`). Signals to the compiler that `foo` must be allocated in linear memory (on the shadow datastack) rather than kept in virtual Wasm registers. |

### 3.3 Low-Level WASI Syscall Driver Implementation

Using `---fa:wasm` and `&raw`, low-level WASI preview 1 syscalls are written directly in Lone:

```lone
---fa:wasm
import .fd_write, .fd_read from :wasi:snapshot_preview1
---

type Ciovec = {
    buf: ptr<uint8>,
    len: uint32,
}

pub fn wasi_write(fd: int, data: []uint8) int {
    let iov = Ciovec{
        buf: &raw data[0],
        len: data.len,
    }
    let nwritten = 0
    -- Explicitly pass raw linear memory addresses to WASI host imports:
    fd_write(fd, &raw iov, 1, &raw nwritten)
    return nwritten
}

pub fn wasi_read(fd: int, buf: &mut []uint8) int {
    let iov = Ciovec{
        buf: &raw buf[0],
        len: buf.len,
    }
    let nread = 0
    fd_read(fd, &raw iov, 1, &raw nread)
    return nread
}
```

### 3.4 Capability Guarding & Safety
- **Compiler Boundary**: The analyzer strictly restricts imports of `:wasi:*` or host modules to files containing an active `---fa:wasm` directive block.
- **Sandboxed Application Code**: Regular application code never imports `:wasi:*` directly. Instead, user code imports `:std:io`, ensuring that application logic remains pure, safe, and portable.

---

## 4. Designing `std:io` (`lib/std/io.lone`)

With this formal import mechanism, `lib/std/io.lone` is authored purely in Lone without external C dependencies:

```lone
---fa:wasm
import .stdout, .stdin from :wasi:cli
---

-- Standard I/O module for Lone
-- Provides buffered and unbuffered terminal and stream access.

pub fn print(msg: string) void {
    stdout.write(msg)
}

pub fn println(msg: string) void {
    stdout.write(msg)
    stdout.write("\n")
}

pub fn read_line() string {
    return stdin.read_line()
}
```

### 4.1 Consuming `std:io` in User Code
```lone
import .println, .read_line from :std:io
import .config from .app_config

fn main() void {
    println("Enter your name:")
    let name = read_line()
    println(f"Welcome, {name}!")
}
```

---

## 5. Architectural Comparison

| Dimension | C Runtime (`packages/c-lone-runtime`) | Inlined Go WAT (`pkg/lone-runtime`) | Formal Import + `---fa:wasm` |
| :--- | :--- | :--- | :--- |
| **Toolchain Dependencies** | Requires `clang`, `lld`, `llvm-ar` | None (pure Go) | **None (pure Go)** |
| **Stdlib Language** | Written in C | Hardcoded WAT in Go strings | **Authored in pure `.lone`** |
| **Import Syntax** | N/A (linker flags) | N/A (compiler intrinsics) | **Unified `import .sym from :ns`** |
| **Namespacing** | Flat C symbol collisions | Internal compiler symbol table | **Explicit `:ns` vs `.file`** |
| **Portability** | Hard to target both Web & WASI | Manual WAT per target | **Directives adapt to target** |

---

## 6. Implementation Plan in Falone

1. **Parser (`pkg/lone-parser/parser.peg`)**:
   - Add `IMPORT_STMT` to `TOP_LEVEL` rules.
   - Flat symbol list: `import ( "." IDENT ) ( "," _ ( "." IDENT ) )* from ( NAMESPACE / REL_PATH )`
   - Alias / whole-module form: `import ( NAMESPACE / REL_PATH ) as IDENT`
   - `NAMESPACE`: `: IDENT (: IDENT)*`
   - `REL_PATH`: `"."+ IDENT? ("/" IDENT)*`
2. **AST (`pkg/lone-parser-ast/`)**:
   - Define `ast.ImportDecl`:
     - `Symbols`: `[]string` (with leading `.` parsed)
     - `Source`: string
     - `IsNamespace`: bool (true if started with `:`)
     - `Alias`: string (optional)
3. **Analyzer (`pkg/lone-analyzer/`)**:
   - Module resolution table:
     - If `Source` starts with `:`, look up in `lib/std/` or registered packages.
     - If `Source` starts with `.`, look up relative to current file's directory.
   - Enforce `---fa:wasm` check when importing `:wasi:*` namespaces.
4. **WIR Generator (`pkg/lone-compiler-wir/`)**:
   - Emit `(import ...)` entries for capabilities declared via `---fa:wasm`.
5. **Standard Library (`lib/std/`)**:
   - Implement `lib/std/io.lone` and `lib/std/std.lone`.
