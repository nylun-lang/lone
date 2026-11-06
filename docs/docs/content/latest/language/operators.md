---
title: "Operators & Precedence"
order: 7
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

---

## Explicit Casting (`as`)

Lone prohibits implicit widening or narrowing numeric conversions. All type conversions must be explicit using `as`:

```rust
let small: i32 = 42
let wide: i64 = small as i64
let byte: u8 = small as u8
```

---

## Error Propagation Operators (`must`, `try`)

Lone provides first-class operators for unwrapping and propagating `Result<V, U>`:

- **`must <expr>`**: Asserts that `expr` is `Result::Ok(val)` or `Option::Some(val)` and evaluates to the inner payload `val`. If the value is `Result::Err` or `Option::None`, execution terminates immediately with a WebAssembly `unreachable` trap.
- **`try <expr>`**: Unwraps `Result::Ok(val)` into `val`. If the value is `Result::Err(err)`, the enclosing function immediately returns `Result::Err(err)`.

```rust
let val = must parse_int("123") -- i32
let item = try fetch_item(id)   -- propagates Err on failure
```
