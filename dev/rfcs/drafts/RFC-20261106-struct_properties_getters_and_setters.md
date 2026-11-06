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

### 3.1 Directive Syntax: Dual Form Support (Block & Inline)

To maximize developer flexibility, both **block directive** and **inline statement / decorator** forms are fully supported:

#### Form A: Block Directive (Multi-Property Grouping)
Encloses one or more getter/setter declarations in a scoped block:
```lone
pub type Game = struct {
    _attempts: i32,
    ro max_val: i32,
    attempts: i32, -- Optional explicit property header contract

    -- Block Getter: intercepts `game.attempts`
    @fa:struct_getter {
        fn attempts(self: ref<ro Self>) i32 {
            return self._attempts
        }
    }

    -- Block Setter: intercepts `game.attempts = val`
    @fa:struct_setter {
        fn attempts(self: ref<Self>, val: i32) void {
            if val >= 0 {
                self._attempts = val
            }
        }
    }
}
```

#### Form B: Inline Statement / Decorator Form (Single-Property Conciseness)
Decorates an individual method directly without block braces:
```lone
pub type Game = struct {
    _attempts: i32,
    ro max_val: i32,

    -- Inline Getter (omitting redundant property header line)
    @fa:struct_getter
    pub fn attempts(self: ref<ro Self>) i32 {
        return self._attempts
    }

    -- Inline Setter
    @fa:struct_setter
    pub fn attempts(self: ref<Self>, val: i32) void {
        self._attempts = val
    }
}
```

### 3.2 The Anti-Nesting Invariant (Compile-Time Error)

Directive nesting within struct property blocks is strictly prohibited:
1. Nesting `@fa:struct_getter` inside `@fa:struct_getter` is a compile-time error (`syn/nested-struct-directive`).
2. Nesting `@fa:struct_setter` inside `@fa:struct_getter` (or vice-versa) is a compile-time error (`syn/nested-struct-directive`).

```lone
-- FORBIDDEN (Compile error: syn/nested-struct-directive)
@fa:struct_getter {
    fn attempts(self: ref<ro Self>) i32 { return self._attempts }

    @fa:struct_setter {  -- ERROR: cannot nest @fa:struct_setter inside @fa:struct_getter!
        fn attempts(self: ref<Self>, val: i32) void { self._attempts = val }
    }
}
```

### 3.3 Declaration Freedom (Explicit Header vs Inferred Property)

Developers have complete freedom in how virtual properties are declared in the struct:
1. **Explicit Property Header**: Listing `attempts: i32` or `ro attempts: i32` in the struct field list clearly communicates the public API contract alongside other fields.
2. **Inferred Property**: Omission of the field line is equally valid—the virtual property name and return type are inferred directly from the getter function signature.

In both styles, the property is **virtual**:
- **0-Byte Physical Allocation**: A field backed by `@fa:struct_getter` does **not** allocate linear memory (`sizeof` only measures physical backing fields).
- **Aggregate Literal Exclusion**: Attempting to assign a virtual property in aggregate literal `Game::{ .attempts: 10 }` produces a compile error.

### 3.4 Receiver Rules & Immutability

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

## 6. Resolved Design Decisions

1. **Explicit Long Names**: Retain `@fa:struct_getter` and `@fa:struct_setter` in full to maintain clear namespace identity and future compiler extensibility.
2. **Dual Form Syntax**: Both block form (`@fa:struct_getter { ... }`) and inline statement/decorator form (`@fa:struct_getter fn ...`) are supported.
3. **Anti-Nesting Rule**: Nesting of any `@fa:struct_...` directives inside one another is strictly forbidden and produces a compile error (`syn/nested-struct-directive`).
4. **Header Line Freedom**: The property header line (e.g. `attempts: i32`) inside the struct field list is optional. Developers are free to specify it for explicit documentation or omit it in favor of direct getter declaration.
