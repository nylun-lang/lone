---
title: "Traits & Materialization"
order: 4
---

# Traits & Explicit Materialization

Traits define behavioral contracts that types can fulfill. In Lone, traits are strictly pure contracts that do not pollute or alter the namespace of implementing types.

---

## The `Tr` Naming Convention

In accordance with Lone's naming rules:
- **Trait Types**: **Must** begin with prefix `Tr` followed by an uppercase letter (e.g. `TrWriter`, `TrReader`, `TrClonable`, `TrDisplayer`).
- **Non-Trait Identifiers**: Any type or declaration that is not a trait must **not** begin with prefix `Tr` followed by an uppercase letter.

This makes trait contracts immediately recognizable at a glance throughout any codebase.

---

## Declaring Traits

Traits specify method signatures without implementations:

```rust
pub type TrWriter = trait {
    fn write(self, msg: str) void
}

pub type TrReader = trait {
    fn read(self, buf: ptr<u8>, max_len: i32) i32
}
```

---

## Implementing Traits (`impl`)

Trait implementations link a trait contract to a concrete target type using `impl Trait on Target` or `impl Trait for Target`:

```rust
pub type Console = struct {
    id: i32,
}

impl TrWriter on Console {
    fn write(self, msg: str) void {
        #print(msg)
    }
}
```

The compiler statically verifies that all methods required by the trait are implemented with matching parameter and return types.

---

## Invariant IL4: Explicit Trait Materialization (`TrTrait::|T|`)

> **Key Invariant**: Traits are pure contracts. They never inject methods into a type's namespace.

To invoke a trait method on a conforming type, the trait must be explicitly materialized using `TrTrait::|TargetType|`:

```rust
pub fn main() void {
    var con = Console::{ .id: 1 }

    -- Explicit trait materialization
    let writer = TrWriter::|Console|
    writer.write(con, "Hello, world!\n")

    -- Or direct chained invocation
    TrWriter::|Console|.write(con, "Chained call\n")
}
```

### Why Explicit Materialization?
1. **No Method Clashes**: Multiple traits can declare a method with the same name without any ambiguity.
2. **Transparent Call Sites**: It is immediately obvious which trait contract is being exercised at every call site.
3. **Zero Runtime Overhead**: Trait materialization resolves statically at compile time into a direct WebAssembly function call, passing the receiver struct pointer.

---

## Generic Functions & `satisfies`

Generic functions require trait implementations using the `satisfies` constraint:

```rust
fn log_message<T where { T satisfies TrWriter }>(writer: T, msg: str) void {
    let w = TrWriter::|T|
    w.write(writer, msg)
}
```

If a type passed to `log_message` does not implement `TrWriter`, compilation fails with a clear static analysis diagnostic (`type/missing-trait-impl`).
