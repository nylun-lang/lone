---
title: "Control Flow & Loops"
order: 3
---

# Control Flow & Loops

Lone provides a rich set of conditional branching and looping constructs that lower directly to structured WebAssembly control-flow instructions.

## Standard Conditional (`if` / `else`)

```rust
fn check_status(score: i32) i32 {
    if score >= 50 {
        return 1
    } else {
        return 0
    }
}
```

Parentheses around conditions are optional. In MIR, `if` statements are lowered into `entry`, `then`, `else`, and `join` basic blocks, which transpile cleanly into WebAssembly `(if ...)` structures.

## Packed `if` Statement

For multi-way branching without cascading `else if` chains, Lone supports **packed `if`**:

```rust
if {
    x > 100 => println("Large")
    x > 50  => println("Medium")
    x > 0   => println("Small")
    else    => {
        println("Non-positive")
    }
}
```

- Each arm specifies a condition, `=>`, and a body (single statement or block `{ ... }`).
- Semicolons or commas between arms are optional.
- The `else` arm is optional; if omitted and no conditions match, execution falls through.

## Pre-Condition While Loops (`while`)

A standard pre-condition loop tests its condition before each iteration:

```rust
fn count_to(max: i32) void {
    var i = 0
    while i < max {
        println_int(i)
        i = i + 1
    }
}
```

## Infinite Loops (`loop`)

For loops intended to run until an explicit `break` or `return`:

```rust
loop {
    var line = read_line()
    if line == "quit" {
        break
    }
    println(line)
}
```

Infinite loops desugar cleanly into an `ir.WhileStmt` with constant `true`.

## Post-Condition Loops (`loop { ... } while: [cond]`)

A post-condition loop executes the loop body **at least once** before evaluating the condition:

```rust
var count = 0
loop {
    count = count + 1
    var step = count
    println_int(step)
} while: [step < 5]
```

- **Semicolon-Free**: The condition is prefixed with `while:` and enclosed in brackets `[...]`, creating a distinct syntactic boundary without requiring a trailing semicolon.
- **Smart Scoping**: Variables declared inside the `loop { ... }` body (`step` above) remain in scope and are accessible in `while: [cond]`. Once the loop exits, those inner variables fall out of scope.
- **Wasm Lowering**: Emits a structured Wasm `(block $exit (loop $body ... (if (then (br $body)))))` where the body executes unconditionally on the first pass.

## For-In Loops (`for`)

Iterates over values using Lone's standard iterator protocol (`IteratorTrait<T>` and `IterableTrait<T>` in `:std:builtin`):

```rust
import .range, .range_step from :std:builtin
import .println_int from :std:io

pub fn main() void {
    for i in range(1, 5) {
        println_int(i)
    }
}
```

Desugars into standard iterator advancement:
```rust
var it = iterable.iterator()
while it.has_next() {
    var i = it.next()
    -- loop body
}
```

## Labeled Blocks & Breaks

Loops can be labeled, and `break` statements can target specific labels or yield values:

```rust
outer: loop {
    inner: while active {
        if critical_error {
            break outer
        }
    }
} while: [running]
```

Break with value (yielding an expression):
```rust
var result = lbl: {
    if fast_path {
        break lbl: 10
    }
    break lbl: 20
}
```
