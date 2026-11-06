# RFC-20261106: Struct Properties, Getters, and Setters via `@fa:struct_getter` & `@fa:struct_setter`

- **RFC**: `RFC-20261106-struct_properties_getters_and_setters`
- **Title**: Struct Properties, Getters, and Setters via `@fa:struct_getter` & `@fa:struct_setter`
- **Author**: Human & AI Pair
- **Status**: Draft
- **Created**: 2026-11-06
- **Target Version**: v0.7.0
- **Related RFCs**:
  - [`RFC-20261104-struct_initialization_and_field_modifiers`](../accepted/RFC-20261104-struct_initialization_and_field_modifiers.md)
  - [`RFC-20261104-interface_data_contracts`](../accepted/RFC-20261104-interface_data_contracts.md)
  - [`RFC-20261105-reference_and_pointer_semantics`](../accepted/RFC-20261105-reference_and_pointer_semantics.md)
- **Applies to**: `pkg2/lone-parser`, `pkg2/lone-analyzer`, `pkg2/lone-compiler-wir`, `dev/SPEC.md`

---

## 1. Summary

This RFC introduces **Virtual Struct Properties** in Lone, powered by explicit compiler directives `@fa:struct_getter` and `@fa:struct_setter`. 

Key capabilities:
1. **Property Interception**: Accessing `instance.property` can transparently invoke a getter function instead of performing a direct linear memory load (`i32.load`).
2. **Encapsulated Computed State**: Structs can expose computed or guarded public properties backed by private fields (`_field`) without forcing callers to use function call syntax (`instance.get_field()`).
3. **Zero-Byte Memory Footprint (Virtual Property Invariant)**: Properties declared with getters do not allocate physical storage in the struct's linear memory layout.
4. **Zero-Cost Static Interface Satisfaction**: Structs with getters can satisfy `ro field: T` requirements in `interface` data contracts without vtables or dynamic dispatch under generic monomorphization.
5. **Const Read-Only Receivers**: Getters take `self: ref<ro Self>` or `self: Self`, guaranteeing by construction that reading a property cannot produce side-effects or mutate instance state.

---

## 2. Motivation

In Lone v0.6.0:
- Struct fields directly correspond to static offsets in WebAssembly linear memory (`i32.load offset=N`).
- Private fields (`_secret`, `_attempts`) are strictly encapsulated inside the struct module.
- To expose a private or computed value to external callers, developers are currently forced to author accessor methods:
  ```lone
  pub fn get_secret(self: ref<Game>) i32 {
      return self._secret
  }
  ```
  Callers must invoke `game.get_secret()` with parentheses.

This design introduces several friction points:
1. **Ergonomic Inconsistency**: Public data is accessed as `game.max_val`, but protected or computed state requires `game.get_secret()`. If an implementation changes from a stored field to a computed property, every call site across the codebase breaks.
2. **Interface Data Contract Inflexibility**: An interface contract like `interface { ro attempts: i32 }` can only be satisfied by structs that physically store `attempts` at a known memory offset. A struct that counts attempts dynamically or computes a cached value cannot satisfy the interface.
3. **Unintended Mutation**: If an API author wants to expose an internal data structure (e.g. array length or item count), exposing a public field allows external callers to read it, but if they declare it `var`, callers can corrupt it. Declaring `ro` prevents mutation, but still requires physical storage.

---

## 3. Detailed Specification

### 3.1 Directive Syntax & Declaration

Getters and setters are declared inside a struct declaration using the `@fa:struct_getter` and `@fa:struct_setter` compiler directive blocks:

```lone
pub type Game = struct {
    _attempts: i32,
    ro max_val: i32,
    attempts: i32, -- Declares the public virtual property contract

    -- Intercepts member read access `game.attempts`
    @fa:struct_getter {
        fn attempts(self: ref<ro Self>) i32 {
            return self._attempts
        }
    }

    -- Intercepts member write access `game.attempts = val`
    @fa:struct_setter {
        fn attempts(self: ref<Self>, val: i32) void {
            if val >= 0 {
                self._attempts = val
            }
        }
    }
}
```

### 3.2 The Virtual Property Invariant (Memory Layout)

A property backed by `@fa:struct_getter` is **virtual**:
1. **0-Byte Physical Allocation**: A field that has an associated getter does **not** allocate bytes in the struct layout. Its offset does not advance the struct frame offset counter.
   $$\text{sizeof}(\text{Game}) = \text{sizeof}(\_attempts) + \text{sizeof}(max\_val) = 4 + 4 = 8\text{ bytes}$$
   *(The virtual property `attempts` contributes 0 bytes).*
2. **Aggregate Literal Exclusion**: In aggregate initialization syntax `Game::{ ... }`, virtual properties cannot be initialized because there is no memory slot to store them:
   ```lone
   -- Compile Error: 'attempts' is a virtual property and cannot be initialized in an aggregate literal;
   -- initialize backing field '_attempts' instead.
   var g = Game::{ ._attempts: 0, .max_val: 100, .attempts: 0 }
   ```

### 3.3 Receiver Rules & Immutability

#### Getter Receivers
A getter method declared inside `@fa:struct_getter` must accept `self` as its first parameter and return a non-void type:
- `fn prop(self: ref<ro Self>) T`: Preferred form. Guarantees read-only semantics; mutating `self` inside the getter is a compile-time error.
- `fn prop(self: Self) T`: Pass-by-value form for small primitive-sized structs.

#### Setter Receivers
A setter method declared inside `@fa:struct_setter` must accept `self: ref<Self>` as its first parameter, the new value as its second parameter, and return `void`:
- `fn prop(self: ref<Self>, value: T) void`

#### Read-Only Properties (No Setter)
If a struct declares a getter for `name`, but does not declare a setter in `@fa:struct_setter`:
```lone
var game = Game::(100)
let a = game.attempts       -- OK: invokes getter
game.attempts = 10          -- COMPILE ERROR: property 'attempts' on type 'Game' is read-only (no setter defined)
```

---

## 4. Interaction with Interfaces & Generics

One of the primary benefits of virtual properties is seamless interoperability with Lone's `interface` data contracts:

```lone
pub type IfcCounter = interface {
    ro count: i32,
}

pub type StepTracker = struct {
    _raw_ticks: i32,
    count: i32,

    @fa:struct_getter {
        fn count(self: ref<ro Self>) i32 {
            return self._raw_ticks / 2
        }
    }
}

pub fn print_count<T where { T satisfies IfcCounter }>(c: ref<T>) void {
    println(f"Count: ${c.count}")
}
```

### Zero-Cost Static Dispatch
Because generic functions in Lone are monomorphized at compile time:
- When compiling `print_count<StepTracker>`:
  The analyzer inspects `T = StepTracker`. It sees that `count` is backed by a getter.
  Instead of emitting `i32.load offset=N`, the WIR lowerer emits:
  ```wat
  call $StepTracker.count
  ```
- When compiling `print_count<SimpleCounter>` (where `count` is a physical stored field):
  The WIR lowerer emits:
  ```wat
  i32.load offset=4
  ```

There is **no vtable**, **no dynamic function pointer dispatch**, and **zero heap allocation**.

---

## 5. WebAssembly Lowering Mechanics

### Member Access Lowering (`T.property`)
When lowering `expr.prop`:
1. If `prop` is a physical field on `T`:
   - Emit address of `expr`.
   - Emit `i32.load offset=fieldOffset`.
2. If `prop` is a virtual getter on `T`:
   - Emit address of `expr` (converted to `ref<ro T>`).
   - Emit `call $T.prop`.

### Assignment Lowering (`expr.prop = val`)
When lowering `expr.prop = val`:
1. If `prop` is a physical field on `T`:
   - Emit address of `expr`.
   - Emit `val`.
   - Emit `i32.store offset=fieldOffset`.
2. If `prop` is a virtual setter on `T`:
   - Emit address of `expr` (converted to `ref<T>`).
   - Emit `val`.
   - Emit `call $T.set_prop` (or `$T.prop$set`).

---

## 6. Open Considerations & Alternative Syntaxes

### 6.1 Syntactic Variants
1. **Block Directive (Proposed)**:
   ```lone
   @fa:struct_getter {
       fn prop(self: ref<ro Self>) i32 { ... }
   }
   ```
2. **Decorator Directive**:
   ```lone
   @fa:getter
   pub fn prop(self: ref<ro Self>) i32 { ... }
   ```
3. **Keyword-Level Syntax**:
   ```lone
   pub get prop(self: ref<ro Self>) i32 { ... }
   pub set prop(self: ref<Self>, val: i32) void { ... }
   ```

*Question for review*: Does `@fa:struct_getter` block syntax provide the right balance between explicit Falone engine control and readability, or is an attribute decorator `@fa:getter` preferred?
