---
title: "Functions & Generics"
order: 2
---

# Functions & Generics

Functions in Lone are declared with the `fn` keyword, typed parameters, explicit return types, and support generic parameters with compile-time constraints.

---

## Function Declarations

```rust
fn add(a: i32, b: i32) i32 {
    return a + b
}

fn greet(name: str) void {
    #print("Hello, ")
    #print(name)
    #print("\n")
}
```

- **Parameters**: Must have explicit type annotations `(name: type, ...)`. Parameter names must follow `snake_case`.
- **Return Type**: Every function must specify a return type or `void`. Non-void functions must guarantee a return value along all code branches.
- **Intrinsic Functions**: Built-in compiler primitives are prefixed with `#` (e.g. `#print`, `#panic`, `#alloc`).

---

## Generic Functions

Functions can declare one or more generic type parameters enclosed in angle brackets `<...>`:

```rust
fn identity<T>(item: T) T {
    return item
}
```

When called, the compiler infers the concrete type arguments or monomorphizes the function specifically for each invoked type signature.

---

## Generic Constraints (`where`)

Lone supports compile-time generic constraints declared within a `where { ... }` block:

```rust
fn log_data<T where { T satisfies TrWriter }>(writer: T, msg: str) void {
    -- Materialize the trait for type T
    let w = TrWriter::|T|
    w.write(writer, msg)
}
```

### Constraint Operators:
1. **`satisfies`**: Verifies that the concrete type implements a required trait:
   ```rust
   T where { T satisfies TrClone }
   ```
2. **`is`**: Constrains a type parameter to a specific nominal type or base category:
   ```rust
   pub type Result<V, U = Error where { U is Error }> = enum {
       Ok(val: V),
       Err(err: U),
   }
   ```

---

## The `infer` Keyword

Lone uses the dedicated `infer` keyword as an explicit type escape hatch:

- **Error Type Inference**: In fallible functions using `try`, `infer` allows the compiler to deduce the unified error union:
  ```rust
  fn process_job(id: int) Result<Job, infer> {
      let rec = try fetch_job(id)
      let parsed = try parse_job(rec)
      return Result::Ok(parsed)
  }
  ```
- Lone intentionally reserves `infer` for explicit inference placeholders and rejects `_` in type positions.

---

## Monomorphization & Performance

All generic functions and generic types are **monomorphized at compile time**:
- Each instantiation with concrete types generates dedicated WebAssembly instructions tailored to that exact layout.
- Zero runtime vtables or boxing overhead.
- Direct static calls are generated for all trait methods.
