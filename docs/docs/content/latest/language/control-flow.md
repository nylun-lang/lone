---
title: "Control Flow & Loops"
order: 3
---

# Control Flow & Loops

Lone provides conditional branching and looping constructs that lower directly to structured WebAssembly control-flow instructions.

## Conditional Branching (`if` / `else`)

```rust
fn check_status(score: i32) i32 {
    if score >= 50 {
        return 1
    } else {
        return 0
    }
}
```

In Falone MIR, `if` statements are lowered into `entry`, `then`, `else`, and `join` basic blocks, which transpile cleanly into WebAssembly `(block ...)` structures.

## While Loops

Loops use the `while` keyword:

```rust
fn count_to(max: i32) void {
    var i: i32 = 0
    while i < max {
        #print("iteration\n")
        i = i + 1
    }
}
```

### WebAssembly Loop Architecture

In WebAssembly, a `block` only branches forward, whereas a `loop` branches backward to its head. Falone automatically emits the canonical Wasm loop nesting structure:

```wasm
(block $while_exit
  (loop $while_head
    ;; 1. Evaluate condition
    local.get $i
    local.get $max
    i32.lt_s

    ;; 2. Invert condition and exit if false
    i32.eqz
    br_if $while_exit

    ;; 3. Body instructions
    ...

    ;; 4. Repeat loop
    br $while_head
  )
)
```

## Labeled Blocks & Breaks

Loops can be labeled, and `break` statements can specify target labels or return break values:

```rust
outer: while running {
    while active {
        if critical_error {
            break outer
        }
    }
}
```
