---
title: "Getting Started"
order: 2
---

# Getting Started with Lone

This guide covers building the Lone v0.6.0 toolchain, writing your first program with algebraic types, formatted string interpolation, pattern matching, and running it on WebAssembly.

---

## Prerequisites

- **Go 1.22+**: Required to build the Lone compiler and CLI tools.
- **Wasmtime**: For executing compiled WebAssembly modules (`.wat` or `.wasm`).

---

## Building the Toolchain

Clone the repository and build the unified `lone` executable:

```bash
git clone https://github.com/lone-lang/lone.git
cd lone

# Build the Lone compiler and CLI
go build -o lone ./cmd/lone
```

Verify your installation:

```bash
./lone version
# lone v0.6.0
```

---

## Your First Lone Program

Create a file named `calc.lone`:

```rust
-- Import I/O utilities
import .println, .println_int from :std:io

-- Define a tagged union enum
pub type MathOp = enum {
    Add(a: int, b: int),
    Multiply(a: int, b: int),
    Negate(x: int),
    Zero,
}

-- Compute using exhaustive pattern matching
fn evaluate(op: MathOp) int {
    return match op {
        MathOp::Add(a, b) => a + b,
        MathOp::Multiply(a, b) => a * b,
        MathOp::Negate(x) => 0 - x,
        MathOp::Zero => 0,
    }
}

pub fn main() void {
    let sum_op = MathOp::Add(15, 27)
    let result = evaluate(sum_op)
    
    println(f"Calculation result: ${result}")
}
```

---

## Checking and Compiling

### 1. Static Analysis & Type Checking
Verify syntax, types, and invariants with `lone check`:

```bash
./lone check calc.lone
```

If there are unconsumed expressions or non-exhaustive matches, `lone check` will report detailed error locations.

### 2. Compiling to WebAssembly Text (`.wat`)
Compile to standalone WebAssembly Text:

```bash
./lone build calc.lone -o calc.wat
```

### 3. Immediate In-Process Execution
Run directly using embedded Wasmtime:

```bash
./lone run calc.lone
```

Output:
```text
Calculation result: 42
```

---

## Using Fallible Operations with `must` and `try`

Lone provides first-class `must` and `try` operators for handling `Result`:

```rust
pub type MathError = enum {
    DivisionByZero,
    Overflow,
}

fn safe_divide(numerator: int, denominator: int) Result<int, MathError> {
    if denominator == 0 {
        return Result::Err(MathError::DivisionByZero)
    }
    return Result::Ok(numerator / denominator)
}

fn calculate_average(sum: int, count: int) Result<int, infer> {
    -- 'try' unwraps Ok or early-returns Err up to caller
    let avg = try safe_divide(sum, count)
    return Result::Ok(avg)
}

pub fn main() void {
    -- 'must' unwraps Ok or panics on Err with wasm unreachable
    let value = must calculate_average(100, 4)
    println_int(value) -- 25
}
```
