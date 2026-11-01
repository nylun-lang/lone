---
title: "Type System & Drop Rule"
order: 4
---

# Type System & Drop Rule

Lone enforces strict safety rules during static analysis by `lone-alyzer` before code is lowered to Falone MIR.

## Multi-Pass Static Analysis

When code is analyzed:
1. **Pass 1: Naming Conventions**:
   - Local variables & parameters must be `snake_case`.
   - Global variables must be `ALL_CAPS` or `snake_case`.
   - Functions must be `snake_case` or `#intrinsic`.
2. **Pass 2: Type Inference & Typed IR Generation**:
   - Infers variable and expression types.
   - Replaces untyped AST nodes with concrete Typed IR nodes (`TProgram`, `TVarDec`, `TFnDec`).
3. **Pass 3: Return Verification & Consume-or-Drop**:
   - Ensures all paths in non-void functions return a matching type.
   - Enforces the consume-or-drop rule.

## The Consume-or-Drop Rule

In Lone, **every expression that returns a non-void value must be consumed by a variable binding or explicitly dropped**.

### Disallowed: Unconsumed Expression Result

```rust
fn compute() i32 {
    return 42
}

fn run() void {
    compute() // ERROR: result of expression of type "i32" is not consumed
}
```

Running `falone analyze` or `falone compile` yields:
```
[error] 6:5 (type/consume-or-drop): result of expression of type "i32" is not consumed; use 'drop' to discard it
```

### Allowed: Consumed by Variable

```rust
fn run() void {
    var val = compute() // OK: consumed by variable binding
}
```

### Allowed: Explicit Drop

If the result is intentionally discarded:

```rust
fn run() void {
    drop compute() // OK: explicitly discarded with 'drop'
}
```

This prevents silent bugs where function return values or error statuses are accidentally ignored.
