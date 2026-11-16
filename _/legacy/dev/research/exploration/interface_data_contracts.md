# Exploration: Interface Types as Formal Data Contracts in Lone

**Date**: 2026-11-04  
**Status**: Draft / Exploratory  
**Author**: Human & AI Pair  
**Location**: `dev/research/exploration/interface_data_contracts.md`  

---

## 1. Executive Summary & Problem Statement

In Lone v0.5.0-beta, the language provides:
- **Traits (`Tr*`)**: Pure behavioral contracts defining method signatures (`fn write(self, msg: str) void`), implemented explicitly and materialized via `TrTrait::|T|`.
- **Structs**: Concrete nominal records encapsulating fields, constructors, and methods in linear memory.
- **Object Literals**: Anonymous ad-hoc data instances (`{ .name: "Alice", .role: "Admin" }`).

### What Is Missing?
Currently, there is no mechanism to define a **formal, reusable description of data**—a data schema or structural shape contract.
Today, if a function needs access to `{ x: int, y: int }`, it must hard-code a specific concrete struct (e.g. `Point2D`). Any other struct that also contains `x` and `y` (e.g. `Vector2`, `NodeCoordinates`, `MousePosition`) cannot be passed without manual conversion or wrapping.

This exploration investigates introducing **Interfaces** to Lone:
> **Traits (`Tr*`) define *what a type can DO* (behavior).  
> **Interfaces (`It*`) define *what data a type MUST HAVE* (shape).**

---

## 2. Naming Conventions: `It*` vs. `Itr*` vs. Alternatives

In alignment with Lone's naming invariants (**IL1 - Explicit over implicit** and the `Tr` trait rule), interfaces require an unambiguous prefix:

| Prefix | Example | Pros | Cons | Recommendation |
| :--- | :--- | :--- | :--- | :---: |
| **`It*`** | `ItUser`, `ItPoint`, `ItConfig` | Exact 2-letter symmetry with `Tr*` (`Tr` vs `It`). Minimal, clean. | "It" is an English pronoun (`ItState`). | **Primary Candidate** |
| **`Itr*`** | `ItrUser`, `ItrPoint` | Distinct 3 letters. | Universally conflated with **Iterator** (`TrIterator` vs `Itr`). | *Discouraged* |
| **`Ifc*`** | `IfcUser`, `IfcPoint` | Standard 3-letter abbreviation for Interface. No word collision. | One letter longer than `Tr`. | **Strong Alternative** |

### Proposed Rule
- Every interface identifier **must** start with the prefix `It` (or `Ifc`) followed by an uppercase letter (e.g. `ItPoint`, `ItUserData`).
- Non-interface types (structs, enums, traits) must **not** start with `It` followed by an uppercase letter.

---

## 3. Syntax & Grammar

### 3.1 Interface Declaration
Interfaces declare named data fields with explicit type annotations and optional modifiers:

```lone
pub type ItPoint = interface {
    x: int,
    y: int,
}

pub type ItUser = interface {
    id: int,
    username: str,
    email: ?str,
    ro created_at: i64,
}
```

### 3.2 Field Modifiers
- **Read-Only (`ro`)**: The field can only be read through this interface, not assigned to.
- **Optional (`?T`)**: The field may hold a value of `T` or `none`.
- **Nested Interfaces**: An interface field can itself be an interface:
  ```lone
  pub type ItEmployee = interface {
      personal_info: ItUser,
      department: str,
  }
  ```

### 3.3 Interface Embedding (Composition)
Interfaces can embed other interfaces to compose larger schemas:

```lone
pub type ItEntity = interface {
    id: int,
    uuid: str,
}

pub type ItProduct = interface {
    ItEntity,            -- Embeds 'id' and 'uuid'
    title: str,
    price_cents: int,
}
```

---

## 4. Conformance & Satisfaction Model

How does a concrete struct or object literal satisfy an interface?

### Model A: Structural Conformance (Duck Typing at Compile-Time)
A type `T` satisfies `ItPoint` if `T` declares fields with compatible names and types, regardless of where or how `T` was declared:

```lone
-- Concrete struct
pub type Vector2 = struct {
    x: int,
    y: int,
    label: str,
}

-- Anonymous object literal
var pt = { .x: 10, .y: 20 }

-- Both 'Vector2' and 'pt' structurally satisfy 'ItPoint'!
```

### Model B: Explicit Nominal Conformance
A struct must explicitly declare that it satisfies an interface:
```lone
pub type Vector2 = struct satisfies ItPoint {
    x: int,
    y: int,
    label: str,
}
```

### Comparison:
| Criterion | Structural Conformance (Model A) | Nominal Conformance (Model B) |
| :--- | :--- | :--- |
| **Object Literals** | Seamless: anonymous records `{ .x: 1, .y: 2 }` work instantly | Incompatible without explicit cast syntax |
| **Coupling** | Decoupled: consumer defines the interface it needs | Tightly coupled: producer must import the interface |
| **Accidental Conformance** | Minor risk: coincidentally identical field names match | Zero risk |

> **Recommendation**: **Structural Conformance** at compile-time matches Lone's philosophy of ergonomic object literals and composable UI topologies, provided it is checked statically.

---

## 5. WebAssembly Lowering & Zero-Cost Abstraction

In WebAssembly, memory access is performed via explicit byte offsets (`i32.load offset=N`). How can interface fields be accessed efficiently?

### Strategy 1: Monomorphized Generic Constraints (Recommended - Zero Runtime Overhead)
Interfaces are primarily used in generic type constraints via `where`:

```lone
fn render_point<T where { T satisfies ItPoint }>(p: T) void {
    println_int(p.x)
    println_int(p.y)
}
```

#### Compilation Mechanics:
1. When `render_point(vec)` is called with `Vector2`, the compiler monomorphizes `render_point` for `Vector2`.
2. The compiler knows the exact static offset of `x` (e.g. offset `0`) and `y` (offset `4`) in `Vector2`.
3. Emitted WebAssembly uses direct, immediate offsets:
   ```wat
   (i32.load offset=0 (local.get $p_ptr)) ;; Loads p.x directly!
   (i32.load offset=4 (local.get $p_ptr)) ;; Loads p.y directly!
   ```
4. **Performance**: Zero runtime overhead, zero vtable, zero fat pointers, zero indirection.

---

### Strategy 2: Dynamic Boxed Interface (Fat Pointer in Linear Memory)
If an interface is used as a direct, non-generic parameter type:
```lone
fn log_id(entity: ItEntity) void
```
Because different structs have field `id` at different byte offsets, `ItEntity` cannot be a raw single pointer `i32`. It would require a **fat pointer** in linear memory:
```
+--------------------------+--------------------------+
| Data Pointer (i32)       | Field Offset Table (i32) |
+--------------------------+--------------------------+
```
Accessing `entity.id` would first load the offset from the offset table, then add it to the base pointer:
$$\text{address} = \text{data\_ptr} + \text{offset\_table}[\text{FIELD\_ID}]$$

> **Design Choice**: In accordance with **IL1 (Explicit over implicit)** and Lone's zero-overhead WebAssembly focus, **Strategy 1 (Compile-Time Monomorphization)** should be the standard. Dynamic fat pointers should only exist if explicitly requested via a dedicated reference type (e.g. `dyn ItEntity` or `ref<ItEntity>`).

---

## 6. Orthogonal Coexistence: Traits (`Tr*`) and Interfaces (`It*`)

A major strength of this architecture is complete orthogonality between data and behavior:

```lone
-- Data Contract (Interface)
pub type IfcLoggableEvent = interface {
    event_id: int,
    timestamp: i64,
    payload: str,
}

-- Behavioral Contract (Trait)
pub type TrSerializer = trait {
    fn serialize(self) str
}

-- Generic function requiring BOTH via declarative comma-separated bounds:
fn dispatch_event<T where {
    T satisfies IfcLoggableEvent,
    T satisfies TrSerializer,
}>(event: T) void {
    -- Access data directly:
    let id = event.event_id
    
    -- Materialize trait behavior explicitly:
    let s = TrSerializer::|T|
    let encoded = s.serialize(event)
    
    #print(encoded)
}
```

- Data fields are accessed naturally via `event.field`.
- Trait methods are invoked explicitly via `TrSerializer::|T|.serialize(event)`.
- The compiler verifies both contracts statically.

---

## 7. Constraint Bounds Syntax: Why Boolean `and` is Prohibited

Generic type constraints in `where { ... }` use **comma-separated declarative bounds**, strictly prohibiting boolean conjunctions like `and`:

```lone
-- Disallowed / Anti-pattern:
T where { T satisfies A and T satisfies B }

-- Canonical Lone Syntax:
T where { T satisfies A, T satisfies B }
```

### Rationale:
1. **Avoiding the Disjunction Slippery Slope**: Allowing `and` falsely implies that `or`, `xor`, and `not` are valid constraint operators.
2. **Preserving Static Narrowing**: In a zero-overhead language without runtime type tags, disjunctive constraints (`A or B`) break static type narrowing. The compiler cannot know which contract is valid at a given call site, preventing safe field or method dispatch.
3. **Set-Based Conjunction**: Multiple bounds in a `where` block represent a simultaneous set of requirements (an intersection). Separating them with commas (`,`) treats them as a clean declarative list, identical to struct fields or parameter lists.

---

## 8. Open Questions for Discussion

1. **Prefix Choice**: Do you prefer **`It*`** (`ItUser`) or **`Ifc*`** (`IfcUser`)?
2. **Width Subtyping**: If an interface requires `{ x: int, y: int }`, should a struct with `{ x: int, y: int, z: int }` satisfy it? (Width subtyping is standard in structural typing).
3. **Mutability**: Should interfaces specify whether fields are mutable or read-only (`ro`) by default?
4. **Declaration Keyword**: Is `type ItName = interface { ... }` preferred, or a top-level `interface ItName { ... }`? (Lone standard has `type Name = struct { ... }` and `type Name = trait { ... }`, so `type ItName = interface { ... }` is perfectly uniform).

---

## 8. Next Steps

- Collect feedback on prefix choice (`It` vs `Ifc`) and structural subtyping rules.
- Transition this exploration into a formal RFC in `dev/rfcs/drafts/` when aligned.
