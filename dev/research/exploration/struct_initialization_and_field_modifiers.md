# Exploration: Struct Initialization, Constructors (`__init__`), and Field Modifiers

**Date**: 2026-11-04  
**Status**: Draft / Exploratory  
**Author**: Human & AI Pair  
**Location**: `dev/research/exploration/struct_initialization_and_field_modifiers.md`  

---

## 1. Executive Summary

This exploration formalizes the rules governing **field modifiers**, **visibility**, and **initialization mechanics** for Lone structs and interfaces:

1. **Visibility via Prefix**: Fields without a leading underscore (e.g. `name`) are **public**. Fields with a leading underscore (e.g. `_hash`) are **private/hidden** to the declaring module.
2. **Field Modifiers (`ro`)**:
   - `ro field: T` on public fields allows "initialize-once, read-only forever" DTOs without boilerplate.
   - `ro _field: T` on private fields requires an explicit constructor (`__init__`).
3. **The `__init__` Constructor Hook**: Special constructor method invoked via `Type::(args...)`.
4. **Encapsulation Invariant**: Structs containing hidden fields (`_*`) **cannot** be instantiated via raw aggregate construct `Type::{ ... }` from outside their declaring module. Outside callers must use `Type::(...)`.

---

## 2. Field Anatomy & Naming Rules

A struct field declaration follows Lone's progressive left-to-right information flow (**IF2**):

$$\mathbf{[ro]}\;\; \text{name}\mathbf{:}\;\; \text{Type}\;\; \mathbf{[=\; default]}$$

### 2.1 Visibility: The Leading Underscore (`_`)
Lone avoids repetitive `pub` keywords on individual fields:
- **Public Field**: Any field whose identifier does **not** begin with `_` (e.g. `id`, `username`, `x`, `y`). Accessible across modules if the enclosing struct is `pub`.
- **Private/Hidden Field**: Any field whose identifier begins with `_` (e.g. `_socket_fd`, `_token_hash`, `_ref_count`). Accessible only within the declaring file/module.

### 2.2 Immutability: The `ro` Modifier
- **Mutable (Default)**: If the struct instance is declared with `var`, the field can be read and reassigned (`s.field = new_val`). If declared with `let`, all fields are immutable.
- **Permanently Read-Only (`ro`)**: Once initialized during construction, the field can **never** be reassigned, even on a mutable `var` instance. Attempting to write `s.ro_field = val` is a compile-time error (`type/immutable-field-write`).

---

## 3. Two Modes of Struct Construction

Lone strictly separates raw aggregate data instantiations from enforcing constructor calls:

| Syntax | Kind | Enforcement | External Access |
| :--- | :--- | :--- | :--- |
| **`Type::{ .x: 10 }`** | **Raw Aggregate Construct** | **Non-Enforcing** (Lenient; missing fields take default or zero values) | **Only permitted if the struct has NO hidden fields (`_*`)**. |
| **`Type::(arg1, arg2)`** | **Constructor Call** | **Enforcing** (Dispatches directly to `Type::__init__(...)`) | **Universally permitted** if `__init__` is public. |

---

## 4. The Encapsulation Invariant: Hidden Field Protection

> **Core Invariant**: Any struct declaring one or more **private/hidden fields (`_*`)** CANNOT be instantiated with raw aggregate syntax `Type::{ ... }` from outside its declaring module.

### Why this rule is essential:
If an outside caller could execute:
```lone
var session = Session::{ .label: "admin" }
```
on a struct declaring `_hash: str`, the private `_hash` field would be bypassed, leaving internal state zeroed or uninitialized. This would violate memory safety and class invariants.

### Compile-Time Diagnostic:
Attempting raw construction on a struct with hidden fields from outside produces:
```
[error] (type/raw-construct-private-fields): struct 'Session' contains private field(s) ('_hash') and cannot be instantiated with raw aggregate syntax '::{ ... }'; invoke its constructor Session::(...) instead.
```

Inside the declaring module (and specifically within `__init__`), `Type::{ ... }` is permitted so the constructor can assemble the complete instance.

---

## 5. The `__init__` Constructor Hook

### 5.1 Definition & Dispatch
A struct provides an enforcing constructor by defining `pub fn __init__(...) Self`:

```lone
pub type Session = struct {
    label: str,
    ro _hash: str,
    ro created_at: i64,

    pub fn __init__(label: str, raw_secret: str) Session {
        return Session::{
            .label: label,
            ._hash: #sha256(raw_secret),
            .created_at: #now(),
        }
    }
}
```

### 5.2 Direct Invocation Syntax: `Type::(...)`
Writing `Session::("admin", "secret123")` desugars directly to `Session::__init__("admin", "secret123")`:

```lone
pub fn main() void {
    -- Enforcing constructor call:
    var sess = Session::("admin", "secret123")
    
    println(sess.label)       -- OK (public)
    println_int(sess.created_at) -- OK (public ro)
    
    -- sess._hash            -- COMPILE ERROR: field '_hash' is private
    -- sess.created_at = 0   -- COMPILE ERROR: field 'created_at' is read-only
}
```

### 5.3 Calling `Type::(...)` Without an `__init__` Method
Calling constructor syntax `Type::(...)` on a struct that does **not** define an `__init__` method is a **compile-time error** (`type/unresolved-constructor`).

```lone
pub type Point = struct {
    x: int,
    y: int,
    -- No __init__ declared!
}

pub fn main() void {
    -- COMPILE ERROR: struct 'Point' defines no '__init__' constructor.
    -- Use raw aggregate Point::{ .x: 10, .y: 20 } instead.
    var p = Point::(10, 20)
}
```

*Rationale (IL1 - Explicit over implicit)*: Lone **never** synthesizes implicit magic constructors. If a developer wants constructor syntax `Point::(...)`, they must explicitly write `pub fn __init__(x: int, y: int) Point`. Otherwise, callers must use the raw aggregate construct `Point::{ .x: 10, .y: 20 }`.

### 5.4 The `ro` Constructor Requirement Rule:
- **Public `ro` fields only (`ro id: int`)**: No `__init__` is required. Outside callers can supply `id` once in `Type::{ .id: 10 }`, and it remains immutable thereafter.
- **Private `ro` fields (`ro _secret: T`)**: An `__init__` constructor is **MANDATORY**. If omitted, the compiler emits a compile-time error (`type/missing-init-for-ro-private-field`), because private `ro` fields could otherwise never be set by callers.

---

## 6. Struct Categorization Matrix

| Struct Archetype | Field Signatures | Allowed Construction from Outside | Requires `__init__`? |
| :--- | :--- | :---: | :---: |
| **Open Data DTO** | `x: int, y: int` | `Point::{ .x: 10, .y: 20 }` or `Point::(10, 20)` (if provided) | No |
| **Public Immutable Record** | `ro id: int, ro name: str` | `Record::{ .id: 1, .name: "Alice" }` (Set once, immutable) | No |
| **Encapsulated Entity** | `id: int, _cache: ?str` | **Only `Entity::(...)`** (`::{ ... }` is forbidden) | **Yes** (`pub fn __init__`) |
| **Secure Protected Object** | `ro _hash: str, title: str`| **Only `Secure::(...)`** (`::{ ... }` is forbidden) | **Yes** (`pub fn __init__`) |

---

## 7. Interaction with Data Interfaces (`Ifc*`)

Interfaces (`Ifc*`) represent external data contracts and therefore:
1. **Interfaces only declare public fields**: Interface fields never start with `_`.
2. **Encapsulated Structs Satisfy Interfaces**: A struct with private fields (`TcpSocket` with `_fd`) still satisfies an interface (`IfcNetworkEndpoint`) as long as its public fields match the interface's contract.
3. The internal hidden fields remain private and untouched in WebAssembly linear memory layout.
