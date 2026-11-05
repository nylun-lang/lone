# Research Exploration: String Interpolation, Formatter Desugaring, and Traits

**Date**: 2026-11-05  
**Scope**: Language Syntax, Lexer Architecture, Trait Design & Zero-Allocation Formatting  
**Status**: Active Exploration  
**Location**: `dev/research/exploration/string_interpolation_and_display_traits.md`  
**Related**:
- `dev/SPEC.md` (Sections 5.7, 5.9, 11.3)
- `dev/rfcs/drafts/RFC-20261104-interface_data_contracts.md`
- `dev/research/exploration/anonymous_functions_grouped_declarations_and_tuples.md`

---

## 1. Executive Summary

In Lone v0.5.0-beta, formatting values requires manual sequencing of standalone functions (`print("x: ")`, `print_int(x)`, `println("")`).

This exploration formulates a cohesive design for **String Interpolation** grounded in Lone's core principles:
1. **Explicit Format String Syntax (`f"Hello, ${name}! You scored ${score}."`)**:
   Explicit `f` prefix distinguishes formatted strings from normal string literals, upholding **IL1 (Explicit over implicit)** and ensuring normal strings `"Price: $100"` never require escaping for `$`.
2. **Stack-Buffered Single-Write Architecture**:
   - **Temporary Output Position (`println(f"...")`)**: Assembled into a caller-reserved Page 0 datastack scratch buffer and flushed in **one single atomic syscall** (`fd_write`). Guarantees zero heap allocations, single-syscall throughput, and complete untorn lines for downstream pipes.
   - **Persistent Value Position (`let s: str = f"..."`)**: Assembled into the Managed Store (heap `#alloc`) for persistent lifetime.
3. **Trait-Grounded Printable Contract (`TrDisplay` / `TrSerializable`)**:
   Any type `T` can be interpolated if it implements a standard formatting/display trait. Built-in types (`int`, `f64`, `bool`, `str`) provide inlined zero-cost implementations out of the box.

---

## 2. Syntax: The `f"${expr}"` Formatted String Standard

### 2.1 Lexical Form
Interpolated strings explicitly use the **`f` prefix** preceding the opening double quote:

$$\mathbf{f"\dots\$\{expression\}\dots"}$$

```lone
let name: str = "Alice"
let score: int = 95

-- Explicit format string literal:
println(f"Player: ${name}, Score: ${score}")

-- Complex expression interpolation:
println(f"Next level in: ${100 - score} points")

-- Regular string literals remain 100% literal with zero escaping needed:
println("The ticket costs $50 per person.") -- Plain string, no interpolation!
```

### 2.2 Why `f"..."` is Superior for Systems Languages
- **IL1 (Explicit over implicit)**: Normal strings are never accidentally scanned or altered. The developer explicitly opts in to interpolation with `f"..."`.
- **Zero Dollar Escaping in Normal Strings**: Code containing shell variables, prices, regexes, or documentation (`$PATH`, `$100`, `$__stack_pointer`) can use normal `"..."` without escaping `$` as `\$`.
- **Deterministic Lexer Performance**: The lexer immediately knows at character `'f'` followed by `'"'` to enter the format string tokenization mode. Standard strings bypass this logic entirely for maximum scanning speed.

---

## 3. The Stack-Buffered Single-Write Architecture

### 3.1 The Danger of Naive Multi-Print Desugaring
A naive systems desugaring would expand `println(f"Hello ${name}, score: ${score}")` into multiple sequential calls: `print("Hello ")`, `print(name)`, `print(", score: ")`, `print_int(score)`, `println("")`.

While this avoids heap allocations on paper, it introduces **three catastrophic systems hazards**:
1. **Output Tearing & Interleaving**:
   If another thread, async task, or error handler writes to stdout/stderr while the sequence executes, its output gets spliced directly into the middle of the sentence (e.g. `Hello ERROR: disk full! Alice, score: 95`).
2. **Pipe Chunking & Syscall Explosion**:
   Emitting 5 separate WASI `fd_write` syscalls per line instead of 1 introduces massive 5x syscall overhead. Downstream pipe readers (`lone_app | grep`) or JSON log parsers doing non-blocking reads can receive incomplete chunks and fail with unexpected EOF.
3. **Expression Invalidation**:
   If `f"..."` only worked as desugared `print` calls, code like `let msg = f"User: ${name}"` would be impossible, breaking the fundamental rule that `f"..."` is a first-class expression producing a `str`.

---

### 3.2 The Sound Architecture: Datastack-Buffered Single Write
To guarantee **atomic output**, **single-syscall delivery**, and **zero heap allocations**, Lone adopts a two-tier memory strategy:

```
                          ┌─────────────────────────────┐
                          │   f"Hello ${name}, ${age}"  │
                          └──────────────┬──────────────┘
                                         │
                 ┌───────────────────────┴───────────────────────┐
                 ▼                                               ▼
       Temporary Output Scope                          Escaping Value Scope
     println(f"Hello ${name}")                     let s: str = f"Hello ${name}"
                 │                                               │
                 ▼                                               ▼
     Stack-Buffered Single Write                     Managed Store Heap Allocation
  - Formatted into Page 0 datastack                - Allocated on Managed Store (heap)
  - EXACTLY ONE atomic fd_write syscall            - Persistent contiguous str
  - ZERO output tearing, ZERO heap cost            - Deterministic #free on ref drop
  - Instant datastack frame rewind
```

#### Mode 1: Temporary Output Position (`println`, `eprintln`)
When an interpolated string is passed directly to an output function or used as a statement:
```lone
println(f"User #${id}: ${name} (active: ${is_active})")
```
1. The compiler allocates a temporary scratch frame (e.g. 128 bytes) on the **current Page 0 datastack**.
2. Formats all elements directly into this contiguous stack buffer.
3. Invokes the underlying WASI `fd_write` **EXACTLY ONCE** with the full, cohesive line `{ ptr: stack_buf, len: total_len }`.
4. The datastack pointer rewinds immediately.
* **Guarantees**: **1 single atomic syscall**, **0 bytes of heap memory**, **zero log tearing**, and downstream pipes receive the complete line in a single chunk.

#### Mode 2: Persistent Value Position (`let s: str = ...`)
When an interpolated string is assigned to a variable, passed to a function expecting `str`, or returned:
```lone
fn format_entry(user: User) str {
    return f"User[${user.id}]: ${user.name}"
}
```
1. The compiler allocates the formatted string in the **Managed Store** (dynamic heap `#alloc`).
2. Populates the string sequentially and returns the fat pointer `str` `{ ptr, len }`.
3. Deterministically reclaimed via `#free` when all references to `s` drop.

---

## 4. The Trait Contract: `TrDisplay` vs. `TrSerializable`

To decide which types can be interpolated, Lone uses a trait contract.

### Option A: `TrSerializable` (Single Unified Contract)
```lone
pub type TrSerializable = trait {
    fn serialize(self) str
}
```
* **Pros**: Single trait for everything (printing, logging, JSON, IPC).
* **Cons**: In systems languages, "serialization" often implies machine-readable data (JSON, Protobuf, binary bytes), whereas printing is for human-readable text. For example, a `PasswordHash` might display as `[REDACTED]` in `print`, but serialize as the raw hex bytes.

### Option B: `TrDisplay` for Text + `TrSerializable` for Data (Recommended)
Follows standard systems separation (Rust `Display` vs `Serialize`, Go `Stringer` vs `BinaryMarshaler`):

1. **`TrDisplay` (Human-Readable Formatting)**:
   ```lone
   pub type TrDisplay = trait {
       fn to_string(self) str
   }
   ```
   * Used by: string interpolation (`${x}`), `print(x)`, `println(x)`.
2. **`TrSerializable` (Data Interchange / Persistence)**:
   ```lone
   pub type TrSerializable = trait {
       fn serialize(self) str
   }
   ```
   * Used by: JSON encoders, file storage, network packets.

---

### 4.3 Streaming Trait Formulation: Zero-Copy `to_writer`

If `to_string(self) str` always allocates a new string, calling `${obj}` inside `println` would still force `obj` to create a temporary heap string!

To make user-defined structs format with **zero heap allocation**, we can formulate `TrDisplay` with a writer buffer:

```lone
pub type TrDisplay = trait {
    fn fmt(self, buf: ref<StringBuilder>) void
}
```

```lone
pub type Point = struct {
    x: int,
    y: int,
}

impl TrDisplay on Point {
    fn fmt(self, buf: ref<StringBuilder>) void {
        buf.write("(")
        buf.write_int(self.x)
        buf.write(", ")
        buf.write_int(self.y)
        buf.write(")")
    }
}
```

* When printing `println("Point: ${pt}")`, `pt.fmt(&console_buf)` writes its bytes **directly to the output stream without creating any temporary strings**!

---

## 5. Built-In Primitive Implementations

Out of the box, the compiler and standard library provide built-in `TrDisplay` implementations for all primitive types:

| Primitive Type | Desugared Formatter | Example Output |
| :--- | :--- | :--- |
| `str` | Direct pointer/len slice | `"Hello"` |
| `int`, `i32` | `int_to_str` / `println_int` | `42`, `-100` |
| `i64`, `u64` | `i64_to_str` | `9223372036854775807` |
| `f32`, `f64` | `f64_to_str` | `3.14159` |
| `bool` | Literal constant `"true"` / `"false"` | `true` |
| `ptr<T>` | Hex address formatter | `0x00010040` |
| `?T` (Option) | `"Some(...)"` or `"none"` | `Some(42)` |

Attempting to interpolate a type `T` that does not implement `TrDisplay` fails at compile time:
`type/missing-trait-impl: type 'TcpSocket' does not implement 'TrDisplay' and cannot be interpolated`.

---

## 6. Compiler Pipeline & Lexer Architecture (`pkg2`)

### 6.1 Lexer State Machine (`pkg2/lone-lexer`)
When the lexer encounters `f"`:
1. Emits `TokenFStringStart`.
2. Reads characters until `${` or `"`.
3. If `${` is encountered:
   - Emits `TokenStringPart` with the text chunk.
   - Pushes `LexerModeInterpolation` onto the lexer state stack.
   - Emits `TokenInterpolateStart` (`${`).
4. Resumes normal expression lexing for the expression inside the braces.
5. When matching closing `}` in `LexerModeInterpolation`:
   - Pops the interpolation mode.
   - Resumes scanning the format string until the next `${` or closing `"`.
6. When closing `"` is encountered, emits `TokenFStringEnd`.
7. Regular strings beginning with `"` alone are scanned as traditional flat literals (`TokenString`), completely skipping interpolation overhead.

### 6.2 Parser AST Representation (`pkg2/lone-parser-ast`)
An interpolated string is represented as a dedicated AST node:
```go
type InterpolatedStringExpr struct {
    Parts []Expr // Alternating LiteralString and general Expr
    Span_ lonelexer.Span
}
```

---

## 7. Comparison with Other Languages

| Language | Interpolation Syntax | Underlying Mechanism | Allocates Heap? |
| :--- | :--- | :--- | :---: |
| **Lone (Proposed)** | `f"Hello ${name}"` | Desugars to `print` streaming or `StringBuilder` via `TrDisplay` | **No** (in print) / Yes (in `let s`) |
| **Rust** | `println!("Hello {name}")` | `std::fmt::Display` via `write!` macro | No (zero allocation) |
| **Go** | `fmt.Sprintf("Hello %v", name)` | Reflection / interface dynamic dispatch | Yes (allocates every call) |
| **Python** | `f"Hello {name}"` | String formatting opcode | Yes |
| **JavaScript** | `` `Hello ${name}` `` | Evaluates to new heap string | Yes |
| **Zig** | `std.fmt.allocPrint(alloc, "Hello {s}", .{name})` | Explicit allocator + writer format | Explicit |
