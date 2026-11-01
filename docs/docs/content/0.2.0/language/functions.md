---
title: "Functions & Signatures"
order: 2
---

# Functions & Signatures

Functions in Lone are declared with the `fn` keyword, typed parameters, and an explicit return type.

## Function Syntax

```rust
fn add(a: i32, b: i32) i32 {
    return a + b
}

fn log_event(name: str) void {
    #print(name)
}
```

## Function Signatures

- Parameters must have explicit type annotations: `(name: type, ...)`.
- Parameter names must follow the `snake_case` naming convention.
- Return types can be any valid Lone type or `void`.

## Intrinsic Functions

Intrinsic functions represent compiler built-ins and are prefixed with `#`:

```rust
fn main() void {
    #print("Running intrinsic operation\n")
}
```

The analyzer allows `#` prefixed identifiers in function calls and definitions without triggering naming convention warnings.
