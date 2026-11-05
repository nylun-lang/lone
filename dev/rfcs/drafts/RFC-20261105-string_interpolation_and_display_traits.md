# RFC-20261105: Formatted String Interpolation (`f"..."`) and Display Traits (`TrDisplay`)

- **RFC**: `RFC-20261105-string_interpolation_and_display_traits`
- **Title**: Formatted String Interpolation (`f"..."`) and Display Traits (`TrDisplay`)
- **Author**: Human & AI Pair
- **Status**: Draft
- **Created**: 2026-11-05
- **Target Version**: v0.6.0
- **Related RFCs**:
  - [`RFC-20261104-interface_data_contracts`](./RFC-20261104-interface_data_contracts.md)
  - [`RFC-20261105-reference_and_pointer_semantics`](./RFC-20261105-reference_and_pointer_semantics.md)
  - [`RFC-20261105-grouped_declarations_anonymous_functions_and_tuples`](./RFC-20261105-grouped_declarations_anonymous_functions_and_tuples.md)
- **Applies to**: `pkg2/lone-lexer`, `pkg2/lone-parser`, `pkg2/lone-analyzer`, `pkg2/lone-compiler-wir`, `dev/SPEC.md`

---

## 1. Summary

This RFC introduces first-class **Formatted String Interpolation** to Lone via the **`f"..."`** syntax, backed by a zero-allocation **Stack-Buffered Single-Write Architecture** and the standard **`TrDisplay`** trait contract.

Key architectural pillars:
1. **Explicit Format String Syntax (`f"Hello, ${name}!"`)**:
   Prefixing interpolated strings with `f` distinguishes them from regular string literals, ensuring normal strings (`"Price: $100"`) require zero escaping for `$`.
2. **Stack-Buffered Single-Write Output**:
   When passed to output functions (`println`, `eprintln`), the formatted string is assembled into a temporary Page 0 datastack scratch buffer and flushed in **exactly ONE atomic WASI `fd_write` syscall**.
   - **Zero Log Tearing**: Eliminates interleaved output hazards across concurrent writers.
   - **Pipe Chunking Safety**: Downstream pipes and line-buffered readers always receive complete, untorn lines.
   - **Zero Heap Allocations**: Memory is instantly reclaimed upon statement completion via datastack pointer rewind.
3. **First-Class `str` Value Evaluation**:
   When used in value position (`let s: str = f"..."`), it constructs a contiguous string on the Managed Store (heap `#alloc`), deterministically freed when references drop.
4. **The `TrDisplay` Trait Contract**:
   Interpolation evaluates types implementing `TrDisplay` via zero-copy buffer streaming (`fn fmt(self, buf: ref<StringBuilder>) void`). All primitive types provide inlined zero-cost implementations out of the box.

---

## 2. Motivation & Architectural Rationale

In Lone v0.5.0-beta, formatting values requires manual sequencing of standalone functions:
```lone
#print("Player #")
print_int(id)
#print(": ")
#print(name)
#print(" (score: ")
print_int(score)
println(")")
```
While functional, this approach is error-prone, verbose, and difficult to maintain.

### The Naive Desugaring Pitfall: Why Not Multiple Print Calls?
A naive compiler optimization might expand `println(f"Hello ${name}, score: ${score}")` directly into 5 separate calls to `print`. However, this introduces **three catastrophic systems hazards**:
1. **Output Tearing**: Concurrent tasks or stderr writes interleave into stdout, corrupting log lines (`Hello ERROR: disk full! Alice, score: 95`).
2. **Syscall Overhead & Pipe Fragmentation**: Emitting 5 separate WASI `fd_write` syscalls per line instead of 1 degrades throughput by 5x. Piped commands (`lone_app | grep`) or JSON log parsers reading non-blocking sockets receive incomplete chunks and fail with unexpected EOF.
3. **Expression Invalidation**: It prevents `f"..."` from being a real expression. You could not write `let s = f"Hello ${name}"`.

To eliminate all three hazards, Lone establishes the **Stack-Buffered Single-Write Architecture**.

---

## 3. Syntax & Lexical Grammar

### 3.1 Lexical Form
Formatted string literals begin with the character `'f'` immediately followed by double quotes:

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

### 3.2 Escaping Rules inside `f"..."`
- `\${`: Escapes the literal sequence `${` without triggering interpolation.
- `\"`: Escapes a double quote.
- `\\`: Escapes a backslash.
- Inside regular `"..."` strings, `$` has no special meaning and does not require escaping.

---

## 4. The Stack-Buffered Single-Write Architecture

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

### 4.1 Temporary Output Scope (`println`, `eprintln`)
When an interpolated string is used in statement output position:
1. The compiler reserves a temporary scratch buffer on the **current Page 0 datastack frame** (default 128 bytes, or statically bounded based on literal length).
2. Formats all elements directly into this contiguous stack buffer.
3. Invokes the underlying WASI `fd_write` **EXACTLY ONCE** with `{ ptr: stack_buf, len: total_len }`.
4. The datastack pointer rewinds immediately upon statement exit.

* **Guarantees**: Exactly **1 atomic syscall**, **0 bytes of heap memory**, **zero log tearing**, and downstream pipes receive the complete line intact.

### 4.2 Escaping Value Scope (`let s: str = ...`)
When an interpolated string is assigned to a variable, passed to a function expecting `str`, or returned:
1. The compiler allocates the formatted string in the **Managed Store** (dynamic heap `#alloc`).
2. Populates the string sequentially and returns the fat pointer `str` `{ ptr, len }`.
3. Deterministically reclaimed via `#free` when all references to `s` drop.

---

## 5. The `TrDisplay` Trait Contract

### 5.1 Trait Definition
Any type can be formatted or interpolated by implementing `TrDisplay`:

```lone
pub type TrDisplay = trait {
    fn fmt(self, buf: ref<StringBuilder>) void
}
```

- **Streaming Efficiency**: By writing to `ref<StringBuilder>`, types format themselves **without allocating intermediate strings**.

### 5.2 User-Defined Struct Implementation
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

-- Now Point can be interpolated anywhere:
let pt = Point::{ .x: 10, .y: 20 }
println(f"Target coordinate is: ${pt}")
```

### 5.3 Built-In Primitive Implementations
Out of the box, the compiler and standard library provide inlined zero-cost implementations of `TrDisplay` for all primitive types:

| Type | Formatted Representation |
| :--- | :--- |
| `str` | Direct slice copy |
| `int`, `i32` | Decimal integer string |
| `i64`, `u64` | 64-bit integer string |
| `f32`, `f64` | Floating-point string |
| `bool` | `"true"` or `"false"` |
| `ptr<T>` | Hexadecimal memory address `0x00010040` |
| `?T` (Option) | `"Some(...)"` or `"none"` |

---

## 6. Static Analysis & Diagnostics

| Diagnostic Code | Condition | Severity |
| :--- | :--- | :--- |
| `type/missing-trait-impl` | Type in `${expr}` does not implement `TrDisplay`. | Error |
| `syntax/unclosed-interpolation` | `${` without matching `}` inside `f"..."`. | Error |
| `syntax/empty-interpolation` | `${}` with no expression. | Error |

---

## 7. Compiler & Lexer Architecture (`pkg2`)

### 7.1 Lexer State Machine (`pkg2/lone-lexer`)
1. Encountering `f"` emits `TokenFStringStart`.
2. Reads text chunks until `${` or closing `"`.
3. Emits `TokenStringPart` with literal text.
4. On `${`, pushes `LexerModeInterpolation` and emits `TokenInterpolateStart`.
5. Resumes normal expression lexing for `${expr}`.
6. On `}`, pops mode and resumes string scanning until next `${` or closing `"`.
7. Encountering closing `"` emits `TokenFStringEnd`.

### 7.2 Parser AST Representation (`pkg2/lone-parser-ast`)
```go
type InterpolatedStringExpr struct {
    Parts []Expr // Alternating LiteralString and general Expr
    Span_ lonelexer.Span
}
```

---

## 8. Prior Art & Comparison

| Language | Syntax | Mechanism | Atomic Output? | Allocates Heap in Print? |
| :--- | :--- | :--- | :---: | :---: |
| **Lone (Proposed)** | `f"Hello ${name}"` | Stack-Buffered Single Write + `TrDisplay` | **Yes (1 syscall)** | **No (0 bytes)** |
| **Rust** | `println!("Hello {name}")` | `write!` macro + `Display` trait | Yes (stream lock) | No |
| **Go** | `fmt.Sprintf("Hello %v", name)` | Reflection / dynamic interface | Yes (buffered) | Yes |
| **Python** | `f"Hello {name}"` | Formatted string opcode | Yes (buffer) | Yes |
| **JavaScript** | `` `Hello ${name}` `` | Template literal heap evaluation | Yes (buffer) | Yes |
