---
title: "Variables & Types"
order: 1
---

# Variables & Types

Lone is a statically typed systems language featuring powerful type inference, explicit memory models, tagged union enums, and compile-time constraints.

---

## Variable Declarations

Variables are declared using `var` (mutable) or `let` (immutable binding). Types can be explicitly annotated or inferred:

```rust
-- Explicitly typed
var counter: i32 = 0
let max_size: u32 = 1024
var ratio: f64 = 3.14159
var message: str = "Welcome to Lone\n"
var is_active: bool = true

-- Inferred types
let total = 100         -- Inferred as i32
var greeting = "Hello"  -- Inferred as str
```

---

## Primitive Types

Lone supports standard fixed-width numeric types directly matching WebAssembly primitives:

| Type | Description | WebAssembly Equivalent |
| :--- | :--- | :--- |
| `i8`, `i16`, `i32` | Signed integers | `i32` |
| `u8`, `u16`, `u32` | Unsigned integers | `i32` |
| `i64`, `u64` | 64-bit integers | `i64` |
| `i128`, `u128` | 128-bit integers | Double-word representation |
| `int`, `uint` | Aliases for `i32` and `u32` | `i32` |
| `f32`, `f64` | Single and double-precision floats | `f32`, `f64` |
| `float` | Alias for `f32` | `f32` |
| `bool` | Boolean (`true` / `false`) | `i32` (`1` or `0`) |
| `str` | UTF-8 string pointer / linear offset | `i32` |
| `char` | Unicode code point | `i32` |
| `void` | Absence of return value | No value on stack |
| `none` | Nil / empty representation | `0` |

---

## Type Modifiers & Constraints

Types can be modified with orthogonal prefixes:

- **`ro`**: Read-only access constraint (`ro str`, `ref<ro T>`).
- **`?`**: Optional / nullable modifier (`?i32`, `?str`). A value of type `?T` can hold a value of `T` or `none`.
- **`comptime`**: Compile-time evaluated constant constraint.

```rust
var opt_name: ?str = "Alice"
var empty_val: ?i32 = none
var constant_table: comptime ro []i32 = { 1, 2, 3 }
```

### Type Testing & Narrowing (`is` / `is not`)

Optional types and polymorphic types can be tested with `is` and `is not`. Lone automatically narrows the type in conditional branches:

```rust
if opt_name is not none {
    -- opt_name is statically narrowed from ?str to str
    println(opt_name)
}
```

---

## References (`ref<T>`) vs. Raw Pointers (`ptr<T>`)

In accordance with **IL1 (Explicit over implicit)**, Lone makes a strict distinction between managed references and unmanaged raw memory offsets:

| Type | Meaning | Creation Syntax | Target Memory |
| :--- | :--- | :---: | :--- |
| `ref<T>` | **Managed Reference** | `&x` | Tracked datastack or managed store. |
| `ptr<T>` | **Raw Memory Address** | `&raw x` | Raw byte offset in WebAssembly linear memory (`i32`). Zero runtime overhead. |

```rust
var val: i32 = 42

-- Safe managed reference
var r: ref<i32> = &val

-- Raw WebAssembly linear memory address
var p: ptr<i32> = &raw val
```

Passing a `ref<T>` where a `ptr<T>` is expected triggers a compile-time type error (`type/call-arg-type`).

---

## Struct Declarations

Structs encapsulate fields, constructors, and instance methods:

```rust
pub type Point = struct {
    x: i32,
    y: i32,

    pub fn new(x: i32, y: i32) Point {
        return Point::{ .x: x, .y: y }
    }

    pub fn distance_squared(self, other: Point) i32 {
        var dx = self.x - other.x
        var dy = self.y - other.y
        return dx * dx + dy * dy
    }
}
```

Structs can be instantiated via associated construct (`Point::{ .x: 10, .y: 20 }`) or constructor method (`Point.new(10, 20)`).

---

## Tagged Union Enums

Enums in Lone are algebraic data types (tagged unions). Variants can be empty or carry named payload fields:

```rust
pub type Shape = enum {
    Circle(radius: int),
    Rectangle(width: int, height: int),
    Point,
}
```

### Variant Construction

Variants are constructed using `Enum::Variant(...)`:

```rust
let c = Shape::Circle(10)
let r = Shape::Rectangle(20, 30)
let p = Shape::Point
```

### Memory Representation
Enums are laid out with a 4-byte integer discriminant tag at offset `0`, followed by payload fields aligned to the maximum payload alignment. Dynamic instances are allocated on the linear heap via `$__lone_rt_alloc`.

---

## Standard Enums: `Option` and `Result`

Lone defines standard tagged unions for optionality and error handling:

### `Option<T>`
```rust
pub type Option<T> = enum {
    Some(val: T),
    None,
}
```

### `Result<V, U = Error where { U is Error }>`
```rust
pub type Result<V, U = Error where { U is Error }> = enum {
    Ok(val: V),
    Err(err: U),
}
```

- When the second type argument is omitted, it defaults to `Error`.
- Fallible functions returning `Result` can be unwrapped cleanly using `must` and `try`.
