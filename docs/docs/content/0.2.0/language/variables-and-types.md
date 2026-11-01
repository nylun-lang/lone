---
title: "Variables & Types"
order: 1
---

# Variables & Types

Lone is statically typed with powerful type inference, explicit memory representations, and structured data primitives.

## Variable Declarations

Variables are declared using the `var` keyword. Types can be written explicitly or inferred:

```rust
// Explicitly typed
var counter: i32 = 0
var ratio: f64 = 3.14159
var message: str = "Welcome to Lone\n"
var is_active: bool = true

// Inferred types (resolved during lone-alyzer type evaluation)
var total = 100         // Inferred as i32
var greeting = "Hello"  // Inferred as str
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

## Lists

### Fixed-Size Lists

Fixed-size lists have their length checked at compile time:

```rust
var scores: [3]i32 = [95, 88, 100]
```

### Dynamic Lists

Dynamic lists represent growable sequences:

```rust
var items: []str = ["apple", "banana", "cherry"]
```

## Object Literals

Object literals use dot-prefixed fields:

```rust
var point = { .x: 10, .y: 20 }
var config = { .max_retries: 3, .timeout: 5000 }
```
