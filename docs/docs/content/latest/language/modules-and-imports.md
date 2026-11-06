---
title: "Modules, Imports & Standard Library"
order: 8
---

# Modules, Imports & Standard Library

Lone organizes reusable code into modules, namespaces, and standard library components.

---

## Importing Symbols

Symbols are imported using the `import` statement:

```rust
-- Import specific symbols from a namespace
import .print, .println, .println_int, .read_line from :std:io

-- Import from standard builtin
import .range, .range_step from :std:builtin
```

### Namespace Aliases

You can also import an entire namespace under an alias:

```rust
import :std:io as io

pub fn main() void {
    io.println("Hello, World!")
}
```

---

## Standard Library Modules

### 1. `:std:io`
Provides formatted and unformatted standard I/O backed by WebAssembly WASI:

- `print(s: str) void`: Prints a string to standard output without newline.
- `println(s: str) void`: Prints a string to standard output followed by `\n`.
- `println_int(n: i32) void`: Prints an integer as decimal text followed by `\n`.
- `read_line() str`: Reads a line of input from standard input up to `\n`.
- `int_to_str(n: i32) str`: Formats an integer to string representation.
- `parse_int(s: str) Result<i32>`: Parses decimal text into an integer, returning `Result::Ok(n)` or `Result::Err(Error::Generic)`.
- `TrWriter`: Writer contract (`write(self, msg: str) void`).
- `TrReader`: Reader contract (`read(self, buf: ptr<u8>, max_len: i32) i32`).

### 2. `:std:sys`
Provides low-level system and runtime utilities:

- `random_i32() i32`: Generates a pseudo-random 32-bit integer backed by WASI `random_get`.
- `random_range(min: i32, max: i32) i32`: Generates an integer uniformly in `[min, max]`.

### 3. `:std:builtin`
Provides core language primitives, ranges, and iteration traits:

- `range(start: i32, end: i32) Range`: Creates an iterable integer range `[start, end)`.
- `range_step(start: i32, end: i32, step: i32) Range`: Creates a stepped range.
- `TrIterator<T>`: Core iterator protocol (`next(self) T`, `has_next(self) bool`).
- `TrIterable<T>`: Core iterable protocol (`iterator(self) Self`).

---

## Compiler Directives (`@fa:wasm`)

To bind low-level WebAssembly host imports directly:

```rust
@fa:wasm {
    import .fd_write, .fd_read from :wasi_snapshot_preview1
}
```

Directives inform the compiler of external module signatures without requiring runtime glue code.
