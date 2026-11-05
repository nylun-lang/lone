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
1. **Ergonomic Template Syntax (`"Hello, ${name}! You scored ${score}."`)**:
   Unambiguous `${expr}` syntax inside double-quoted strings.
2. **Dual Desugaring Modes**:
   - **Direct Output / Streaming (Zero Allocation)**: When passed directly to `print`, `println`, or any stream writer, interpolation desugars into consecutive, in-place writes without allocating intermediate strings on the heap.
   - **Value Construction**: When assigned to a `str` variable, it desugars into an efficient buffer construction.
3. **Trait-Grounded Printable Contract (`TrDisplay` / `TrSerializable`)**:
   Any type `T` can be interpolated if it implements a standard formatting/display trait. Built-in types (`int`, `f64`, `bool`, `str`) provide inlined zero-cost implementations out of the box.

---

## 2. Syntax: The `${expr}` Interpolation Standard

### 2.1 Lexical Form
Interpolated strings use standard double quotes with the `${...}` escape sequence:

$$\mathbf{"\dots\$\{expression\}\dots"}$$

```lone
let name: str = "Alice"
let score: int = 95

-- Simple variable interpolation:
println("Player: ${name}, Score: ${score}")

-- Complex expression interpolation:
println("Next level in: ${100 - score} points")
```

### 2.2 Why `${expr}` over `{expr}` or `\(expr)`
- **No Collision with Braces**: Lone uses `{ ... }` extensively for struct literals, object records, match expressions, and blocks. Requiring `{` to be escaped as `{{` inside strings causes friction.
- **Instant Lexer Recognition**: The `$` sign followed immediately by `{` provides unambiguous $O(1)$ lookahead for the lexer.
- **Familiarity**: Matches JavaScript/TypeScript, Dart, and Kotlin template strings.

---

## 3. The Dual Desugaring Architecture

The fundamental question in a systems language is: *Does string interpolation allocate heap memory?*

In Lone, we can achieve **Zero-Allocation Formatting** by letting the compiler distinguish between *streaming to output* and *constructing a string value*.

```
                          ┌─────────────────────────────┐
                          │   "Hello ${name}, ${age}"   │
                          └──────────────┬──────────────┘
                                         │
                 ┌───────────────────────┴───────────────────────┐
                 ▼                                               ▼
         In Output Position                              In Value Position
     println("Hello ${name}")                       let s: str = "Hello ${name}"
                 │                                               │
                 ▼                                               ▼
     Zero-Allocation Streaming                      Buffered Heap Construction
   print("Hello ")                                var b = StringBuilder::new()
   print(name)                                    b.write("Hello ")
   println()                                      b.write(name)
                                                  let s = b.finish()
```

---

### 3.1 Mode 1: Zero-Allocation Streaming (Direct Output)

When an interpolated string literal is passed directly to `print`, `println`, `eprint`, or a writer method:

```lone
println("User #${id}: ${name} (active: ${is_active})")
```

The compiler **never allocates an intermediate string on the heap**. Instead, the compiler expands the single call into a sequence of zero-cost primitive writes:

```lone
-- Desugared WIR Lowering:
#print("User #")
println_int(id)
#print(": ")
#print(name)
#print(" (active: ")
if is_active { #print("true") } else { #print("false") }
#print(")\n")
```

* **Execution Cost**: **Zero heap allocations**, zero garbage, zero copies.
* Directly leverages existing WASI/WebAssembly output primitives.

---

### 3.2 Mode 2: Value Construction (`let s: str = ...`)

When an interpolated string is assigned to a variable, passed to a function expecting `str`, or returned:

```lone
fn format_entry(user: User) str {
    return "User[${user.id}]: ${user.name}"
}
```

The compiler desugars this into a temporary datastack/heap string builder:

```lone
-- Desugared WIR Lowering:
var _buf = StringBuilder::with_capacity(64)
_buf.write("User[")
_buf.write_int(user.id)
_buf.write("]: ")
_buf.write(user.name)
let s = _buf.finish()
```

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
When the lexer encounters `"`:
1. Reads regular string characters until `$` or `"`.
2. If `$` is followed by `{`:
   - Emits `TokenStringPart` with the text chunk.
   - Pushes `LexerModeInterpolation` onto the lexer state stack.
   - Emits `TokenInterpolateStart` (`${`).
3. Resumes normal expression lexing for the expression inside the braces.
4. When matching closing `}` in `LexerModeInterpolation`:
   - Pops the interpolation mode.
   - Resumes scanning the string until the next `${` or closing `"`.
5. Emits `TokenStringEnd`.

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
| **Lone (Proposed)** | `"Hello ${name}"` | Desugars to `print` streaming or `StringBuilder` via `TrDisplay` | **No** (in print) / Yes (in `let s`) |
| **Rust** | `println!("Hello {name}")` | `std::fmt::Display` via `write!` macro | No (zero allocation) |
| **Go** | `fmt.Sprintf("Hello %v", name)` | Reflection / interface dynamic dispatch | Yes (allocates every call) |
| **JavaScript** | `` `Hello ${name}` `` | Evaluates to new heap string | Yes |
| **Zig** | `std.fmt.allocPrint(alloc, "Hello {s}", .{name})` | Explicit allocator + writer format | Explicit |
