---
title: "Type System & Safety Invariants"
order: 9
---

# Type System & Safety Invariants

Lone enforces strict safety rules during static analysis before code is lowered to WebAssembly Intermediate Representation (WIR).

---

## Multi-Pass Static Analysis

The static analyzer performs rigorous semantic checks across multiple passes:

1. **Pass 1: Naming Conventions**:
   - **Traits**: Must strictly begin with the `Tr` prefix followed by an uppercase letter (e.g. `TrWriter`, `TrReader`).
   - **Non-Traits**: Any non-trait type, function, variable, or field must **not** begin with `Tr` followed by an uppercase letter.
   - **Functions, Locals, Parameters, Fields**: Must follow `snake_case` (or `#` for intrinsics).
   - **Globals & Constants**: Must be `ALL_CAPS` or `snake_case`.
   - **Nominal Types (Structs & Enums)**: Must follow `PascalCase`.

2. **Pass 2: Type Inference & Trait Checking**:
   - Resolves inferred bindings (`let x = ...`) to concrete types.
   - Verifies explicit trait materialization (`TrTrait::|T|`) against registered `impl` declarations.
   - Validates generic constraints declared in `where { ... }` blocks (`satisfies`, `is`).

3. **Pass 3: Return Verification & Consume-or-Drop**:
   - Ensures all code paths in non-void functions return a matching type.
   - Strictly enforces the Consume-or-Drop invariant.

---

## The Consume-or-Drop Rule (Invariant IL3)

> **Key Invariant**: Every expression evaluating to a non-void value must be consumed by a variable binding or explicitly dropped with `drop`.

### Disallowed: Unconsumed Expression Result

```rust
fn compute() i32 {
    return 42
}

fn run() void {
    compute() -- ERROR: result of expression of type "i32" is not consumed
}
```

Static analysis rejects this with:
```
[error] 6:5 (type/unused-result): result of expression of type "i32" is not consumed; use 'drop' to discard it
```

### Allowed: Consumed by Variable
```rust
fn run() void {
    var val = compute() -- OK: consumed by variable binding
}
```

### Allowed: Explicit Drop
```rust
fn run() void {
    drop compute() -- OK: explicitly discarded with 'drop'
}
```

This invariant guarantees that discarded computations or unhandled results are always explicit and intentional.
