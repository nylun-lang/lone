---
title: "Variables & Types"
order: 1
---

# Variables & Types

Lone is statically typed with powerful type inference, explicit memory representations, and structured data primitives.

## Variable Declarations

Variables are declared using the `var` keyword. Types can be written explicitly or inferred:

```rust
-- Explicitly typed
var counter: i32 = 0
var ratio: f64 = 3.14159
var message: str = "Welcome to Lone\n"
var is_active: bool = true

-- Inferred types
var total = 100         -- Inferred as i32
var greeting = "Hello"  -- Inferred as str
```

## Primitive Types

Lone supports standard fixed-width numeric types matching WebAssembly primitives:

| Type | Description | WebAssembly Equivalent |
| :--- | :--- | :--- |
| `i8`, `i16`, `i32` | Signed integers | `i32` |
| `u8`, `u16`, `u32` | Unsigned integers | `i32` |
| `i64`, `u64` | 64-bit integers | `i64` |
| `f32` | 32-bit floating point | `f32` |
| `f64` | 64-bit floating point | `f64` |
| `bool` | Boolean (`true` / `false`) | `i32` (`0` or `1`) |
| `str` | UTF-8 string literal | Linear memory offset (`i32`) |
| `void` | Empty return type | No return value |
| `none` | Nil/null value | `0` |

## Optional Types (`?T`)

An optional type represents a value of type `T` or `none`:

```rust
var opt_name: ?str = "Alice"
var empty_val: ?i32 = none
```

Optional types can be tested and narrowed using `is` / `is not`:
```rust
if opt_name is not none {
    println(opt_name) -- Automatically narrowed to str
}
```

## Structs & Traits

### Struct Declarations

```rust
type Point = struct {
    x: i32,
    y: i32,
}
```

### Trait Declarations

Traits define interfaces. Trait names **must** follow the `PascalCase` convention ending with `Trait`:

```rust
type DrawableTrait = trait {
    fn draw(self) void
}
```

### Implementing Traits

```rust
impl DrawableTrait for Point {
    fn draw(self) void {
        println_int(self.x)
    }
}
```

## Pointers and References

Lone distinguishes between safe references and raw linear memory pointers:

- `ref<T>`: Safe managed reference (`&x`).
- `ptr<T>`: Unsafe linear memory address pointer (`&raw x`).
- `ro`: Read-only memory modifier (`ref<ro T>`, `ptr<ro T>`).
