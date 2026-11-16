# RFC-20261104: Struct Initialization Modes, `__init__` Constructors, and Field Modifiers

- **RFC**: `RFC-20261104-struct_initialization_and_field_modifiers`
- **Title**: Struct Initialization Modes, `__init__` Constructors, and Field Modifiers
- **Author**: Human & AI Pair
- **Status**: Accepted
- **Created**: 2026-11-04
- **Accepted**: 2026-11-05
- **Target Version**: v0.6.0
- **Related RFCs**:
  - [`RFC-20261104-interface_data_contracts`](./RFC-20261104-interface_data_contracts.md)
  - [`RFC-20261105-reference_and_pointer_semantics`](./RFC-20261105-reference_and_pointer_semantics.md)
  - [`RFC-20261105-grouped_declarations_anonymous_functions_and_tuples`](./RFC-20261105-grouped_declarations_anonymous_functions_and_tuples.md)
  - [`RFC-20261105-string_interpolation_and_display_traits`](./RFC-20261105-string_interpolation_and_display_traits.md)
- **Applies to**: `pkg2/lone-parser`, `pkg2/lone-analyzer`, `pkg2/lone-compiler-wir`, `dev/SPEC.md`

---

## 1. Summary

This RFC formalizes the rules governing struct field visibility, immutability, instantiation semantics, and encapsulation invariants in Lone:

1. **Visibility by Convention (Struct-Private Scope)**: Fields without a leading underscore (e.g. `name`) are **public**. Fields with a leading underscore (e.g. `_socket`) are **struct-private** (accessible exclusively within methods on `T` itself and `__init__`, mirroring JavaScript `#field`).
2. **Field Immutability (`ro`)**:
   - `ro field: T` on public fields allows "set-once, read-only forever" DTOs without constructor boilerplate.
   - `ro _field: T` on private fields requires an explicit `__init__` constructor.
3. **Dual Construction Modes**:
   - **`Type::{ .f: v }` (Raw Aggregate Construct)**: Non-enforcing field assignment. Strictly forbidden from outside if the struct contains hidden fields.
   - **`Type::(args...)` (Enforcing Constructor)**: First-class constructor syntax dispatching directly to `Type::__init__(args...)`.
4. **Encapsulation Invariant**: Structs with private fields **cannot** be instantiated with raw aggregate syntax `Type::{ ... }` outside the struct's own associated initializers/methods.
5. **No Magic Constructors (IL1)**: Calling `Type::(...)` on a struct without an explicit `__init__` method is a compile-time error.

---

## 2. Motivation

Lone v0.5.0-beta allows struct instantiation primarily through raw field syntax (`Point::{ .x: 10, .y: 20 }`). While ergonomic for open data structures, this model presents critical shortcomings when building robust systems and encapsulated domain models:

1. **State Invariant Bypass**: If a struct declares private internal state (e.g. `_socket_fd: int`, `_buffer: ptr<u8>`), allowing outside callers to use raw aggregate initialization lets them bypass initialization invariants, leaving memory zeroed or uninitialized.
2. **Ambiguity Between DTOs and Encapsulated Objects**: Without clear constructor semantics, there is no standardized way to distinguish between an open plain-old-data record and a guarded domain entity.
3. **Inconsistent Immutability**: Fields that should never be mutated after construction (IDs, creation timestamps, security hashes) either required clumsy getter methods or were left vulnerable to accidental reassignment.

---

## 3. Detailed Specification

### 3.1 Field Anatomy & Syntax
A struct field declaration follows Lone's progressive left-to-right information flow (**IF2**):

$$\mathbf{[ro]}\;\; \text{name}\mathbf{:}\;\; \text{Type}\;\; \mathbf{[=\; default]}$$

```lone
pub type Account = struct {
    ro id: int,             -- Public, permanently read-only
    owner: str,             -- Public, mutable
    _balance: int,          -- Private, mutable
    ro _created_at: i64,    -- Private, permanently read-only
}
```

### 3.2 Visibility: The Leading Underscore Rule (Struct-Private Scope)
To eliminate repetitive `pub` keywords on individual fields while maintaining bulletproof encapsulation:
- **Public Field**: Any field name that does **not** begin with `_`. Accessible everywhere the struct is imported.
- **Private/Hidden Field (`_field`)**: Any field name that begins with `_`. Accessible **strictly within methods and associated functions declared on `T` itself** (via `self._field` or `Self::__init__`), mirroring JavaScript `#field` encapsulation.
- **Strict Boundary**: Even standalone functions declared in the exact same `.lone` source file cannot access `instance._field`. All external and non-associated access attempts trigger `type/private-field-access`.

### 3.3 Immutability: The `ro` Modifier
- **Default (Mutable)**: If the struct variable is declared with `var`, the field can be read and reassigned (`acc.owner = "Bob"`). If declared with `let`, the entire instance is immutable.
- **Permanently Read-Only (`ro`)**: Once initialized during construction, the field can **never** be reassigned, even on a mutable `var` binding. Attempting to write `acc.id = 99` triggers a compile-time error (`type/immutable-field-write`).
- **Strict Immutability Invariant**: Reassigning an `ro` field via assignment syntax (`s.ro_field = val`) is prohibited **everywhere**, including inside `__init__`. `ro` fields can only receive values during the initial aggregate literal expression.

---

## 4. Construction Modes & The Encapsulation Invariant

Lone provides two distinct instantiation mechanisms:

### 4.1 Raw Aggregate Construct (`Type::{ ... }`)
- **Semantics**: Assigns named fields directly. Fields not provided take their default values or zero/none representations.
- **Enforcement Level**: Non-enforcing / lenient.
- **Access Rule (The Encapsulation Invariant)**:
  - **Inside struct associated methods / initializers**: Permitted on all structs (used by `__init__` and factory methods declared on `T` to assemble state).
  - **Outside struct methods**: **FORBIDDEN** if the struct contains one or more private fields (`_*`).
  - **Allowed outside ONLY IF** all fields on the struct are public (open DTOs).

```lone
-- Attempted raw construction from outside:
var acc = Account::{ .owner: "Alice" }
-- COMPILE ERROR: struct 'Account' contains private field(s) ('_balance', '_created_at')
-- and cannot be instantiated with raw aggregate syntax '::{ ... }'; use Account::(...) instead.
```

### 4.2 Constructor Invocation (`Type::(...)`)
- **Semantics**: Invokes the struct's explicit `pub fn __init__(...) Self` method.
- **Enforcement Level**: Enforcing. Evaluates arguments, enforces type checks, executes constructor logic, and ensures all private and `ro` fields are soundly initialized.
- **Access Rule**: Accessible from anywhere the struct and its `__init__` method are visible.

```lone
-- Outside callers construct via the enforcing constructor:
var acc = Account::(101, "Alice", 500)
```

---

## 5. The `__init__` Constructor Hook

### 5.1 Definition & Syntax
A struct provides a constructor by declaring an associated function named `__init__` returning `Self`:

```lone
pub type Account = struct {
    ro id: int,
    owner: str,
    _balance: int,
    ro _created_at: i64,

    pub fn __init__(id: int, owner: str, initial_deposit: int) Account {
        return Account::{
            .id: id,
            .owner: owner,
            ._balance: initial_deposit,
            ._created_at: #now(),
        }
    }
}
```

### 5.2 Direct Invocation Dispatch: `Type::(args...)`
Calling `Type::(arg1, arg2)` desugars 1:1 into `Type::__init__(arg1, arg2)`.

### 5.3 Prohibition of Magic Constructors (IL1)
Calling `Type::(...)` on a struct that does **not** explicitly declare an `__init__` method is a compile-time error (`type/unresolved-constructor`):

```lone
pub type Point = struct {
    x: int,
    y: int,
    -- No __init__ declared
}

pub fn main() void {
    -- COMPILE ERROR: Struct 'Point' defines no '__init__' constructor.
    -- Use raw aggregate Point::{ .x: 10, .y: 20 } instead.
    var p = Point::(10, 20)
}
```
*Rationale*: Lone never generates implicit magic constructors. Code must be explicit about whether it is constructing an aggregate or calling a constructor.

### 5.4 The Private `ro` Requirement Rule
- **Public `ro` fields (`ro id: int`)**: Do **not** require an `__init__`. External callers can initialize them once via `Type::{ .id: 10 }`.
- **Private `ro` fields (`ro _hash: str`)**: **MUST** have an `__init__` constructor.
  - If a struct declares private `ro` fields without providing an `__init__`, the compiler flags a static analysis error (`type/missing-init-for-ro-private-field`).
  - *Rationale*: A private `ro` field can never be initialized from outside. Without an `__init__`, it would remain uninitializable.

### 5.5 Factory Methods & Associated Constructors
Structs often provide alternative named constructors (e.g. `TcpSocket::connect(host, port)` or `TcpSocket::from_raw_fd(fd)`):
- Any associated function declared on the struct `T` itself can use raw aggregate syntax `Self::{ ... }`, populating both private `_*` fields and `ro` fields during the aggregate expression.
- Outside callers access these factory methods as standard functions (`TcpSocket.connect(...)`), while direct raw construction from outside remains blocked.

### 5.6 Zero-Field Structs (`type Unit = struct {}`)
- `Unit::{}` declares zero private fields and is legal from anywhere.
- `Unit::()` is **illegal** unless the struct explicitly declares `pub fn __init__() Unit`. Lone never synthesizes magic parameterless constructors (**IL1**).

---

## 6. Archetype Classification Matrix

| Struct Archetype | Field Signatures | Allowed External Construction | Requires `__init__`? |
| :--- | :--- | :---: | :---: |
| **Open Data DTO** | `x: int, y: int` | `Point::{ .x: 10, .y: 20 }` or `Point::(10, 20)` (if provided) | No |
| **Public Immutable Record** | `ro id: int, ro name: str` | `Record::{ .id: 1, .name: "Alice" }` (Set once, immutable) | No |
| **Encapsulated Entity** | `id: int, _cache: ?str` | **Only `Entity::(...)`** (`::{ ... }` is forbidden) | **Yes** (`pub fn __init__`) |
| **Secure Protected Object** | `ro _hash: str, title: str`| **Only `Secure::(...)`** (`::{ ... }` is forbidden) | **Yes** (`pub fn __init__`) |

---

## 7. Static Analysis & Diagnostics

The compiler (`pkg2/lone-analyzer`) enforces the following invariants:

| Diagnostic Code | Condition | Severity |
| :--- | :--- | :--- |
| `type/raw-construct-private-fields` | Attempting `Type::{ ... }` on a struct with `_*` fields from outside the struct's associated methods. | Error |
| `type/unresolved-constructor` | Invoking `Type::(...)` when `Type` has no `__init__` method. | Error |
| `type/missing-init-for-ro-private-field` | Struct declares `ro _*` field but provides no `__init__` method. | Error |
| `type/immutable-field-write` | Attempting reassignment to an `ro` field (`s.ro_f = v`) outside construction. | Error |
| `type/private-field-access` | Attempting direct member access `s._f` from outside methods on `T`. | Error |

---

## 8. WebAssembly Compilation & Runtime Mechanics

1. **Stack Frame SRET Allocation (Return-by-Value)**:
   - `Type::__init__(args...)` returns the struct by value using Lone's standard SRET (Structure Return) protocol.
   - The caller allocates a stack frame slot on its own Page 0 datastack frame and passes a hidden `sret_ptr` as the first argument.
   - Inside `__init__`, fields are written directly to `sret_ptr` offsets.
   - Reclaiming stack memory is zero-cost: rewinding the caller's stack pointer frees it instantly with zero leaks and zero heap operations.
2. **Automatic Escape Promotion for Escaping References (`ref<T>`)**:
   - If an initialized struct reference escapes the function scope (`return &instance`), the escape analyzer automatically promotes the allocation to the **Managed Store** (dynamic heap via `$__lone_rt_alloc`).
   - The compiler inserts deterministic `#free` calls when all references to the instance are dropped, coalesced immediately by the free-list allocator.
3. **Zero Runtime Overhead for Method Calls**:
   - `Type::(...)` resolves statically to direct call dispatch: `call $ModuleName.Type.__init__`.
   - No vtables, dynamic dispatch, or heap allocation required for stack-allocated structs.

---

## 9. Integration with Data Interfaces (Cross-RFC Synthesis)

This RFC directly interoperates with [**RFC-20261104-interface_data_contracts**](./RFC-20261104-interface_data_contracts.md):

### 9.1 Encapsulated Structs Satisfy Interfaces Seamlessly
- While an encapsulated struct with private fields (`_*`) restricts external construction to `Type::(...)`, its **public fields** naturally satisfy structural duck-typing for `Ifc*` interfaces.
- The struct's private fields remain hidden and unexposed:
  ```lone
  pub type TcpClient = struct {
      ro host: str,           -- Public read-only
      port: int,              -- Public mutable
      _socket_fd: int,        -- Private internal socket descriptor
      _rx_buf: ptr<u8>,       -- Private receive buffer

      pub fn __init__(host: str, port: int) TcpClient {
          return TcpClient::{
              .host: host,
              .port: port,
              ._socket_fd: #sys_socket_connect(host, port),
              ._rx_buf: #alloc(4096),
          }
      }
  }

  -- Data Contract:
  pub type IfcNetworkTarget = interface {
      ro host: str,
      port: int,
  }

  -- TcpClient automatically satisfies IfcNetworkTarget!
  fn log_target<T where { T satisfies IfcNetworkTarget }>(t: ref<T>) void {
      #print(t.host)
      #print(":")
      println_int(t.port)
  }
  ```

### 9.2 Complete Lifecycle Workflow
1. **Creation**: Caller executes `var client = TcpClient::("10.0.0.1", 443)`. Direct field access to `_socket_fd` is rejected.
2. **Passing**: Caller passes `&client` to `log_target(&client)`.
3. **Monomorphization**: `log_target` is monomorphized for `TcpClient`, reading `host` and `port` at their exact linear memory offsets.
4. **Safety & Zero Leaks**: Invariants remain completely intact, with zero runtime indirection or fat pointers.
