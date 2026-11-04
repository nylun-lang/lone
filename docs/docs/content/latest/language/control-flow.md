---
title: "Control Flow & Pattern Matching"
order: 3
---

# Control Flow & Pattern Matching

Lone provides structured branching, loops with zero-cost WebAssembly lowering, exhaustive pattern matching, and dedicated error propagation operators.

---

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

Parentheses around conditions are optional. Conditions must evaluate to a boolean expression.

---

## Packed `if` Statement

Packed `if` groups multiple condition-action arms using `=>` fat arrows, eliminating verbose `else if` ladders:

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

- **Clean Syntax**: Arms can be separated by newlines, commas (`,`), or semicolons (`;`).
- **Desugaring**: Desugars 1:1 in the AST into nested `IfStmt` nodes with zero runtime penalty.
- **Fallthrough**: In statement position, `else =>` is optional. In expression position, `else =>` is required for exhaustiveness.

---

## Exhaustive Pattern Matching (`match`)

Lone features first-class pattern matching for tagged union enums:

```rust
let name = match shape {
    Shape::Circle(r)       => "Circle",
    Shape::Rectangle(w, h) => "Rectangle",
    Shape::Point           => "Point",
}
```

### Compiler Invariants:
1. **Exhaustiveness**: All enum variants must be handled. Missing variants without a wildcard `_` fallback trigger a compile-time static analysis error (`type/non-exhaustive-match`).
2. **Payload Destructuring**: Variant fields are bound directly to arm-scoped variables (`r`, `w`, `h` above).
3. **Wildcard Fallback (`_`)**:
   ```rust
   match event {
       Event::Click(x, y) => handle_click(x, y),
       _ => log_ignored(),
   }
   ```

---

## Error Handling: `must` and `try`

Lone integrates ergonomic error handling for the `Result<V, U>` type:

### 1. `must <expr>` (Assertion & Panic)
Evaluates `expr`. If the result is `Result::Ok(val)`, evaluates to `val`. If `Result::Err`, it immediately triggers a WebAssembly `unreachable` trap:

```rust
let data = must read_file("config.json")
```

### 2. `try <expr>` (Early Return Propagation)
Evaluates `expr`. If `Result::Ok(val)`, evaluates to `val`. If `Result::Err(err)`, returns early from the enclosing function with `Result::Err(err)`:

```rust
fn load_user(id: int) Result<User, infer> {
    let raw = try fetch_record(id)
    let user = try parse_user(raw)
    return Result::Ok(user)
}
```

The enclosing function must return a compatible `Result` type. Using `infer` as the error parameter (`Result<T, infer>`) automatically unifies all propagated error variants.

---

## Looping Constructs

### 1. Pre-Condition Loop (`while`)
Tests its condition before each iteration:

```rust
var i = 0
while i < max {
    println_int(i)
    i = i + 1
}
```

### 2. Infinite Loop (`loop`)
Executes indefinitely until terminated by `break` or `return`:

```rust
loop {
    var line = read_line()
    if line == "quit" {
        break
    }
    println(line)
}
```

### 3. Post-Condition Loop (`loop { ... } while: [cond]`)
Executes the loop body **at least once** before evaluating the condition:

```rust
var count = 0
loop {
    count = count + 1
    var step = count
    println_int(step)
} while: [step < 5]
```

- **Semicolon-Free**: The bracketed syntax `while: [...]` eliminates ambiguity and semicolon requirements.
- **Smart Scoping**: Variables declared inside the `loop` body (`step`) remain in scope and accessible inside the `while: [...]` condition. Once the loop exits, those variables fall out of scope.

### 4. For-In Loop (`for`)
Iterates over an iterable value using Lone's standard iterator protocol (`IteratorTrait<T>`):

```rust
import .range from :std:builtin

for i in range(1, 10) {
    println_int(i)
}
```

---

## Labeled Blocks & Breaks

Blocks and loops can be prefixed with a label (`label:`). The `break` statement can target specific labels and yield values:

```rust
outer: loop {
    inner: while active {
        if need_exit {
            break outer
        }
    }
} while: [running]
```

### Labeled Expression Blocks:
```rust
var result = blk: {
    var a = calculate_a()
    var b = calculate_b()
    break blk: a + b
}
```

---

## The `drop` Statement

Lone enforces strict resource and result accounting. Any statement evaluating to a non-void value must be bound to a variable or explicitly dropped:

```rust
-- Discards the return value intentionally
drop write_bytes(buffer)
```

Ignoring return values triggers a static analysis error (`type/unused-result`).
