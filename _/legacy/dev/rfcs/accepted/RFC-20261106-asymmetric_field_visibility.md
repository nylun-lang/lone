# RFC-20261106: Asymmetric Field Visibility (`ro(pub)`)

- **RFC**: `RFC-20261106-asymmetric_field_visibility`
- **Title**: Asymmetric Field Visibility via `ro(pub)` (Public Read, Struct-Private Write)
- **Author**: Human & AI Pair
- **Status**: Accepted
- **Created**: 2026-11-06
- **Accepted**: 2026-11-06
- **Target Version**: v0.7.0
- **Related RFCs**:
  - [`RFC-20261104-struct_initialization_and_field_modifiers`](../accepted/RFC-20261104-struct_initialization_and_field_modifiers.md)
  - [`RFC-20261104-interface_data_contracts`](../accepted/RFC-20261104-interface_data_contracts.md)
  - [`RFC-20261106-struct_properties_getters_and_setters`](../rejected/RFC-20261106-struct_properties_getters_and_setters.md)
- **Applies to**: `pkg2/lone-parser`, `pkg2/lone-analyzer`, `pkg2/lone-compiler-wir`, `dev/SPEC.md`

---

## 1. Summary

This RFC introduces **Asymmetric Field Visibility** in Lone via the `ro(pub)` modifier. It allows a struct field to be **read-only to external callers**, while remaining **fully mutable to the struct's own inherent methods**:

```lone
pub type Game = struct {
    ro(pub) attempts: i32,  -- Public can read; only Game methods can write
    ro max_val: i32,        -- Permanently read-only everywhere
    _secret: i32,           -- Private (hidden from public view)
}
```

Key guarantees:
1. **Physical Storage**: `ro(pub)` fields are real, physical WebAssembly linear memory offsets. Access is a 0-cost, direct `i32.load offset=N`.
2. **Transparent References**: Taking references (`&game.attempts`) is fully supported.
3. **Zero Boilerplate**: Eliminates synthetic getter functions (`_attempts` + `get_attempts()`).
4. **Compile-Time Enforcement**: External assignments (`game.attempts = 10`) are statically rejected with `type/asymmetric-field-write`. Inherent methods (`self.attempts = self.attempts + 1`) compile without restriction.
5. **Interface Satisfaction**: Naturally satisfies `interface { ro attempts: i32 }` data contracts without vtables or dynamic dispatch.

---

## 2. Motivation

In Lone v0.6.0:
- Open fields (`attempts: i32`) are mutable by anyone with access to the struct.
- Private fields (`_attempts: i32`) are completely invisible outside the struct.
- Read-only fields (`ro max_val: i32`) are permanently immutable everywhere, including inside the struct's own methods (set-once at construction).

This leaves a critical gap: **How does a struct expose internal state (such as counters, lengths, status flags, and cached progress) for inspection without allowing external callers to corrupt it?**

### The Getter Dilemma
In [`RFC-20261106-struct_properties_getters_and_setters`](../rejected/RFC-20261106-struct_properties_getters_and_setters.md), synthetic property getters (`@fa:struct_getter`) were evaluated and rejected because:
1. They introduced cognitive dissonance on error handling (`Result<T>` vs assignment).
2. They broke address-taking transparency (`&real_field` works, `&getter` fails).
3. They hid function calls and dynamic control flow behind innocent dot access (`game.attempts`).

### The Solution: Asymmetric Field Visibility
Instead of generating synthetic functions to wrap private fields, Lone directly models the access matrix on the physical field itself.

---

## 3. The 5-Quadrant Access Matrix

With `ro(pub)`, Lone provides complete, orthogonal coverage of all field access patterns:

| Field Declaration | Public Caller Read | Public Caller Write | Struct Method Read | Struct Method Write |
| :--- | :---: | :---: | :---: | :---: |
| `field: T` | Yes | Yes | Yes | Yes |
| **`ro(pub) field: T`** | **Yes** | **No (Compile Error)** | **Yes** | **Yes** |
| `ro field: T` | Yes | No (Compile Error) | Yes | No (Set-Once) |
| `_field: T` | No (Compile Error) | No (Compile Error) | Yes | Yes |
| `ro _field: T` | No (Compile Error) | No (Compile Error) | Yes | No (Set-Once) |

---

## 4. Detailed Specification

### 4.1 Syntax & Grammar
The field declaration syntax in `pkg2/lone-parser` is extended to support qualified `ro` modifiers:

$$\text{FieldDec} \longrightarrow [\mathbf{ro} [(\mathbf{pub})]] \;\;\; \text{Name} \mathbf{:} \;\;\; \text{Type} \;\;\; [= \text{Default}]$$

```lone
pub type Account = struct {
    ro id: i64,             -- Permanently const (set once in constructor)
    ro(pub) balance: i64,   -- Readable by anyone; modified only by deposit/withdraw
    owner: str,             -- Freely mutable by anyone
    _nonce: u32,            -- Strictly internal private state
}
```

*Note*: Applying `ro(pub)` to a private field (`ro(pub) _field: T`) is a compile-time error (`syn/redundant-field-modifier`), as private fields are already invisible to public callers.

### 4.2 Semantic Analysis Rules (`pkg2/lone-analyzer`)

When evaluating a member assignment expression `base.field = expr`:

1. Let `targetType` be the base struct type, and `fInfo` be the target field.
2. If `fInfo.IsRo` (permanent const):
   - Reject with `type/immutable-field-write`: *"cannot assign to read-only field 'field'"*.
3. If `fInfo.IsRoPub` (asymmetric):
   - Check the current semantic scope:
   - If the assignment occurs **inside an inherent method of `targetType`** (where `te.currentSelfType == targetType`):
     - **Permit** assignment.
   - If the assignment occurs **outside `targetType`** (in a free function, external module, or method of a different type):
     - **Reject** with `type/asymmetric-field-write`:
       > *`field 'field' on struct 'Type' is read-only to external callers; mutation is restricted to methods of 'Type'`*.

### 4.3 Reference Rules (`&`)
- Inside `targetType` methods: `&self.balance` yields `ref<i64>`.
- Outside `targetType`:
  - `&acc.balance` yields `ref<ro i64>` (a read-only reference).
  - Attempting to pass `&acc.balance` to a function expecting `ref<i64>` (mutable reference) fails with:
    > *`type/mismatch: cannot pass read-only reference 'ref<ro i64>' as mutable 'ref<i64>'`*.

---

## 5. Interface Satisfaction

An asymmetric field `ro(pub) attempts: i32` seamlessly satisfies an interface requirement:

```lone
pub type IfcSession = interface {
    ro attempts: i32,
}

pub type Game = struct {
    ro(pub) attempts: i32,
    ...
}
```

Because external callers can read `attempts`, `Game` satisfies `IfcSession`. Under monomorphized generic dispatch:
```lone
pub fn log_status<T where { T satisfies IfcSession }>(s: ref<T>) void {
    println(f"Attempts: ${s.attempts}")
}
```
The compiler emits a direct `i32.load offset=N`. No vtable, no function call wrapper, zero overhead.

---

## 6. Comparison with Other Languages

| Language | Equivalent Syntax | Mechanism | Memory Model |
| :--- | :--- | :--- | :--- |
| **Swift** | `public private(set) var x: Int` | Compiler access check | Physical field |
| **C#** | `public int X { get; private set; }` | Synthetic getter/setter | Virtual property |
| **Rust** | `pub fn x(&self) -> i32` | Manual method boilerplate | Method call |
| **Go** | `x int` + `func (s *S) X() int` | Manual method boilerplate | Method call |
| **Lone (Proposed)**| `ro(pub) x: i32` | Compiler access check | **Physical field (0-cost)** |

---

## 7. Open Questions

1. **Naming Alternative**: Is `ro(pub)` the most intuitive phrasing, or would `ro(ext)` (externally read-only) or `pub(read)` be clearer?
   - `ro(pub)` leverages the existing `ro` keyword and specifies who is restricted to read-only access.
