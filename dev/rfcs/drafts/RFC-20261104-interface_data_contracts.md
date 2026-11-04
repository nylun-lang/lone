# RFC-20261104: Formal Data Contracts via Interface Types (`Ifc*`)

- **RFC**: `RFC-20261104-interface_data_contracts`
- **Title**: Formal Data Contracts via Interface Types (`Ifc*`)
- **Author**: Human & AI Pair
- **Status**: Draft
- **Created**: 2026-11-04
- **Target Version**: v0.6.0
- **Related RFCs**: [`RFC-20261104-struct_initialization_and_field_modifiers`](./RFC-20261104-struct_initialization_and_field_modifiers.md)
- **Applies to**: `pkg2/lone-parser`, `pkg2/lone-analyzer`, `pkg2/lone-compiler-wir`, `dev/SPEC.md`

---

## 1. Summary

This RFC introduces **Interfaces** (`Ifc*`) to Lone as first-class, structural **Data Contracts**.

While **Traits (`Tr*`)** define pure *behavioral contracts* (method signatures) that are explicitly implemented and materialized, **Interfaces (`Ifc*`)** define *data shape contracts* (field requirements) that are satisfied via compile-time **structural typing (duck typing)** with width subtyping.

---

## 2. Motivation & Core Separation of Concerns

Lone v0.5.0-beta establishes a strict boundary for traits:
- Traits contain only functions/methods.
- Traits do not inject into a type's namespace; they must be materialized via `TrTrait::|T|`.

However, systems and UI programming frequently require abstracting over **data shape**, not just behavior:
1. **UI Component Props & Topology**: A component needs access to `{ label: str, count: int }` regardless of whether the caller passes `ButtonModel`, `MenuItem`, or an anonymous object literal `{ .label: "Save", .count: 3 }`.
2. **Partial Record Views**: A logger needs `{ id: int, timestamp: i64 }` from an entity with 20 other domain fields.
3. **Decoupled API Boundaries**: Library functions can specify the exact fields they consume without forcing callers to adopt a specific nominal struct type.

### The Lone Contract Division:

| Concept | Keyword | Prefix | Focus | Contents | Satisfaction |
| :--- | :---: | :---: | :--- | :--- | :--- |
| **Trait** | `trait` | `Tr*` | **Behavior** | Function/method signatures | Explicit nominal `impl` + `Tr::\|T\|` |
| **Interface** | `interface` | `Ifc*` | **Data** | Named field types & mutability | Compile-time **structural duck typing** |

---

## 3. Syntax & Lexical Conventions

### 3.1 Naming Convention: The `Ifc` Rule
To maintain visual clarity and instant recognition:
- **Interface Types**: **Must** begin with prefix `Ifc` followed by an uppercase letter (e.g. `IfcPoint`, `IfcUser`, `IfcAppConfig`).
- **Non-Interface Identifiers**: Any type or symbol that is not an interface must **not** begin with prefix `Ifc` followed by an uppercase letter.

### 3.2 Declaration Syntax
Interfaces follow Lone's uniform type definition grammar:

```lone
pub type IfcPoint = interface {
    x: int,
    y: int,
}

pub type IfcUser = interface {
    id: int,
    username: str,
    email: ?str,
    ro created_at: i64,
}
```

### 3.3 Interface Embedding (Composition)
Interfaces can embed other interfaces to compose schemas without inheritance hierarchies:

```lone
pub type IfcIdentifiable = interface {
    id: int,
}

pub type IfcNamed = interface {
    name: str,
}

pub type IfcEntity = interface {
    IfcIdentifiable,
    IfcNamed,
    ro created_at: i64,
}
```

---

## 4. Semantic Rules & Duck Typing Conformance

### 4.1 Structural Duck Typing
A concrete type `T` (struct or object literal) satisfies `IfcSchema` if:
1. For every field `f: Type_I` declared in `IfcSchema`, `T` contains a field named `f` with a compatible type `Type_T`.
2. **Width Subtyping**: `T` may contain additional fields not declared in `IfcSchema`. Extra fields are safely ignored.

```lone
pub type Vector3 = struct {
    x: int,
    y: int,
    z: int,
    label: str,
}

-- Vector3 automatically satisfies IfcPoint!
```

### 4.2 Object Literal Conformance
Anonymous object literals satisfy interfaces without explicit casting or type tags:

```lone
fn render_point<T where { T satisfies IfcPoint }>(p: T) void {
    println_int(p.x)
    println_int(p.y)
}

pub fn main() void {
    -- Object literal structurally satisfies IfcPoint:
    render_point({ .x: 10, .y: 20 })
}
```

---

## 5. Field Mutability & Safety (`ro`)

Field mutability in interfaces mirrors struct field semantics:

### 5.1 Read-Only Fields (`ro field: T`)
- Declaring `ro field: T` indicates the consumer only intends to **read** the field.
- **Covariant Satisfaction**: Both mutable fields (`x: int`) and read-only fields (`ro x: int`) on the target struct satisfy `ro x: int`.
- Mutating a field through an `ro` interface reference is rejected at compile time (`type/immutable-field-write`).

### 5.2 Mutable Fields (`field: T`)
- Declaring `field: T` indicates the consumer requires write access (`target.field = new_val`).
- A struct declaring `ro field: T` **cannot** satisfy a mutable interface field (`type/field-mutability-mismatch`).
- The target struct field must be mutable, and the variable binding must be mutable (`var`).

```lone
pub type IfcMutablePoint = interface {
    x: int,
    y: int,
}

pub type ImmutablePoint = struct {
    ro x: int,
    ro y: int,
}

-- Fails static analysis: ImmutablePoint cannot satisfy IfcMutablePoint
```

---

## 6. WebAssembly Compilation & Zero-Cost Abstraction

### 6.1 Generic Constraints & Monomorphization (Zero-Cost Standard)
Interfaces are primarily consumed via generic type constraints:

```lone
fn scale_point<T where { T satisfies IfcMutablePoint }>(p: ref<T>, factor: int) void {
    p.x = p.x * factor
    p.y = p.y * factor
}
```

#### WIR Lowering:
1. When `scale_point` is invoked for a concrete type `T`, the compiler monomorphizes the function for `T`.
2. The byte offsets for `x` and `y` are resolved statically based on `T`'s linear memory struct layout.
3. Direct `i32.load` and `i32.store` instructions with hardcoded offsets are emitted.
4. **Guarantees**:
   - Zero runtime vtables.
   - Zero fat pointers.
   - Zero heap allocation.
   - Direct linear memory load/store operations.

---

## 7. Orthogonality with Traits

Interfaces and Traits compose cleanly in generic where-clauses:

```lone
fn process_record<T where { T satisfies IfcUser, T satisfies TrSerializable }>(record: T) void {
    -- Data field access (direct):
    let user_id = record.id
    
    -- Trait method dispatch (materialized):
    let serializer = TrSerializable::|T|
    let payload = serializer.serialize(record)
    
    #print(payload)
}
```

- Data access remains natural: `record.id`.
- Method calls remain explicit and collision-free: `TrSerializable::|T|.serialize(record)`.

---

## 8. Integration with Struct Encapsulation (Cross-RFC Synthesis)

This RFC directly interoperates with [**RFC-20261104-struct_initialization_and_field_modifiers**](./RFC-20261104-struct_initialization_and_field_modifiers.md):

### 8.1 Private Fields Are Transparent to Interfaces
- Interfaces only describe **public data contracts**. They cannot declare fields starting with `_`.
- When an encapsulated struct contains private fields (e.g. `_socket_fd: int`), those private fields are **ignored** by structural matching.
- As long as the struct exposes the required public fields with matching types and mutability, it satisfies the interface.

### 8.2 End-to-End Cross-Compilation Scenario

```lone
-- ==========================================
-- Module A: Encapsulated Domain Struct
-- ==========================================
pub type UserSession = struct {
    ro session_id: int,    -- Public read-only
    username: str,         -- Public mutable
    _token_hash: str,      -- Private internal state
    ro _created_at: i64,   -- Private read-only state

    pub fn __init__(id: int, user: str, secret: str) UserSession {
        return UserSession::{
            .session_id: id,
            .username: user,
            ._token_hash: #sha256(secret),
            ._created_at: #now(),
        }
    }
}

-- ==========================================
-- Module B: Data Interface & Generic Function
-- ==========================================
pub type IfcSessionSummary = interface {
    ro session_id: int,    -- Covariant read-only requirement
    username: str,         -- Mutable or read-only access
}

fn print_session<T where { T satisfies IfcSessionSummary }>(s: ref<T>) void {
    #print("Session ID: ")
    println_int(s.session_id)
    #print("User: ")
    println(s.username)
}

-- ==========================================
-- Consumer Execution & Monomorphization
-- ==========================================
pub fn main() void {
    -- 1. Construct via enforcing constructor:
    var sess = UserSession::(1001, "alice", "my_secret")

    -- 2. Pass to interface-constrained generic function:
    -- UserSession satisfies IfcSessionSummary!
    print_session(&sess)

    -- 3. Anonymous object literals also satisfy IfcSessionSummary:
    print_session(&{ .session_id: 2002, .username: "bob" })
}
```

### 8.3 Compiler Diagnostics & Verification
The compiler guarantees that:
1. `UserSession::{ .session_id: 1001 }` from outside triggers `type/raw-construct-private-fields`.
2. Inside `print_session`, accesses `s.session_id` and `s.username` are lowered to direct `i32.load` WebAssembly instructions using `UserSession`'s static linear memory offsets.
3. Private fields `_token_hash` and `_created_at` remain safely encapsulated and untouched.

---

## 9. Compiler Implementation Plan

1. **`pkg2/lone-lexer`**:
   - Add keyword `interface`.
2. **`pkg2/lone-parser`**:
   - Parse `TYPE_DEC_INTERFACE` within `type Name = interface { ... }`.
   - Parse field declarations with optional `ro` modifier and type references.
3. **`pkg2/lone-analyzer`**:
   - Enforce `Ifc` prefix naming convention (`naming/interface`).
   - Implement structural duck-typing evaluator (`TypeSatisfiesInterface(target, ifc)`).
   - Validate field compatibility, width subtyping, and `ro` mutability rules.
4. **`pkg2/lone-compiler-wir`**:
   - Monomorphize generic functions constrained by `Ifc*` interfaces, resolving fields to concrete linear memory offsets.
5. **`test2`**:
   - Unit and behavioral tests for structural matching, object literals, width subtyping, and mutability errors.

---

## 10. Alternatives & Prior Art

- **TypeScript Interfaces**: Structural typing with width subtyping. Lone adapts this for a statically compiled WebAssembly systems language via monomorphization.
- **Go Interfaces**: Go interfaces define method sets rather than data fields. Lone reserves method sets for **Traits** (`Tr*`) and uses **Interfaces** (`Ifc*`) exclusively for data shapes.
- **Rust Traits with Associated Types**: Rust achieves data-like polymorphism by requiring getter/setter methods. Lone's approach eliminates method boilerplate for simple data access.
