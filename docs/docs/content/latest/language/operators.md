---
title: "Operators & Precedence"
order: 5
---

# Operators & Precedence

Lone features a clean operator precedence hierarchy designed to prevent ambiguities and subtle programming errors.

## Operator Precedence Table

From highest precedence (evaluated first) to lowest:

| Level | Category | Operators | Associativity | Description |
| :---: | :--- | :--- | :---: | :--- |
| **1** | Multiplicative | `*`, `/`, `%` | Left-to-right | Multiplication, division, remainder |
| **2** | Additive | `+`, `-` | Left-to-right | Addition, subtraction |
| **3** | Relational | `<`, `<=`, `>`, `>=` | **Non-associative** | Ordering comparisons (**no chaining**) |
| **4** | Type Test | `is`, `is not` | **Non-associative** | Type test & smart cast |
| **5** | Equality | `==`, `!=` | Left-to-right | Equality and inequality |
| **6** | Bitwise AND | `&` | Left-to-right | Bitwise AND |
| **7** | Bitwise XOR | `^` | Left-to-right | Bitwise exclusive OR |
| **8** | Bitwise OR | `\|` | Left-to-right | Bitwise inclusive OR |
| **9** | Logical AND | `&&`, `and` | Left-to-right | Short-circuit boolean AND |
| **10** | Logical XOR | `xor` | Left-to-right | Boolean exclusive OR |
| **11** | Logical OR | `\|\|`, `or` | Left-to-right | Short-circuit boolean OR |

---

## Logical `xor` Operator

The `xor` operator evaluates to `true` if exactly one operand is `true`, and `false` otherwise:

```rust
var both_false = false xor false -- false
var one_true   = true xor false  -- true
var both_true  = true xor true   -- false
```

In compound expressions, `xor` sits directly between `and` and `or`:
```rust
a and b xor c or d
-- Evaluates as: ((a and b) xor c) or d
```

---

## Type Test Operators (`is`, `is not`) & Smart Narrowing

Tests if an expression matches a given type reference (`is`) or does not match (`is not`):

```rust
if val is i32 {
    println_int(val) -- Statically narrowed to i32
}

if opt is not none {
    println(opt)     -- Narrowed from ?str to str
}

if !(opt is none) {
    println(opt)     -- Narrowed to inner type
}
```

### Automatic Type Narrowing (Smart Casting)
- Inside the positive branch of `if x is T`, `x` is automatically narrowed to `T`.
- In the `else` branch of `if x is none`, `x` is automatically narrowed from `?T` to `T`.
- In the `else` branch of `if x is not none`, `x` is narrowed to `none`.
- For optional types (`?T`), runtime lowering performs null/tag checks.
- For concrete static types, checks resolve at compile-time with zero runtime overhead.

---

## Prohibition of Comparison Chaining

In languages like C, `a < b < c` compiles silently into `(a < b) < c`, evaluating the boolean result of `a < b` against `c`.

In Lone, **comparison operators cannot be chained**:
```rust
var ok = a < b and b < c  -- OK: explicit logical conjunction
var bad = a < b < c       -- COMPILE ERROR: comparison operators cannot be chained
```

Attempting to chain comparisons (`a < b < c`, `a <= b <= c`, `a < b > c`) is rejected immediately at compile time.
Parenthesized boolean comparisons against numbers like `(a < b) < c` are also rejected during semantic analysis (`type/invalid-comparison`).
