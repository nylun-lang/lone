# Under the Hood: Semantic Analyzer & Type System Architecture

This document describes the design, scoping rules, structural subtyping engine, and immutability validation system implemented in `pkg2/lone-analyzer`.

---

## 1. Two-Pass Compiler Pipeline

`pkg2/lone-analyzer` uses a deterministic two-pass architecture to support forward referencing without ordering restrictions:

```
┌────────────────────────────────────────────────────────┐
│ Parse Tree (loneast.Program)                           │
└───────────────────────────┬────────────────────────────┘
                            │
                            ▼
┌────────────────────────────────────────────────────────┐
│ Pass 1: Declaration Hoisting                           │
│  • Struct, Interface, Trait & Enum type registration   │
│  • Function signatures and top-level global variables  │
│  • WASI / Standard library foreign imports (:wasi:*)   │
└───────────────────────────┬────────────────────────────┘
                            │
                            ▼
┌────────────────────────────────────────────────────────┐
│ Pass 2: Type Checking & AST Lowering                   │
│  • Lexical scope traversal & symbol resolution         │
│  • Expression type evaluation & operator inference     │
│  • Structural duck-typing & interface validation       │
│  • Immutability & reference-safety enforcement         │
└───────────────────────────┬────────────────────────────┘
                            │
                            ▼
┌────────────────────────────────────────────────────────┐
│ Typed IR (TProgram, TFnDec, TStmt, TExpr)              │
└────────────────────────────────────────────────────────┘
```

---

## 2. Lexical Scope Stack & Symbol Tracking (`scope.go`)

Symbol scoping is managed by `SymbolStack`:

### 2.1 Scope Hierarchy
- **`ScopeGlobal`**: Contains top-level types, functions, and global constants.
- **`ScopeFunction`**: Encloses function parameters and local declarations.
- **`ScopeBlock`**: Pushed for nested blocks (`if`, `while`, labeled blocks), ensuring lexical variable shadowing and lifetime management.

### 2.2 Symbol Metadata (`Symbol`)
Each symbol entry contains:
- `Name`: Identifier string.
- `Kind`: `SymbolType`, `SymbolFn`, or `SymbolVar`.
- `Type`: Evaluated type instance (`loneanalyzer.Type`).
- `IsMutable`: `true` for `var` bindings; `false` for `let` bindings and function parameters.
- `AddressTaken`: Set to `true` when `&x` or `&raw x` is applied, signaling to the compiler backend that the variable must spill into the Page 0 datastack frame.

---

## 3. Structural Duck-Typing & Width Subtyping Engine

Lone implements zero-overhead structural duck-typing for interface data contracts (`type IfcName = interface { ... }`).

### 3.1 Satisfaction Algorithm (`TypeSatisfiesInterface`)
When checking whether a concrete type `T` satisfies interface `Ifc`:
1. **Naming Invariant**: Interfaces must start with prefix `Ifc` (enforced via rule `naming/interface`).
2. **Width Subtyping**: The target struct may declare additional fields not present in `Ifc`.
3. **Field Matching**: Every field declared in `Ifc` must exist on `T`:
   - Names must match exactly.
   - Types must be assignable (`targetField.Type.AssignableTo(ifcField.Type)`).
4. **Covariant Read-Only Compatibility**:
   - If an interface field is declared `ro` (`ro name: Type`), it can be satisfied by either a mutable or read-only field on `T`.
   - If an interface field is mutable, the target struct field must also be mutable (preventing writes through an interface to a read-only field).
5. **Generic Constraint Evaluation**:
   When evaluating generic functions constrained by `where { T satisfies IfcName }`, the analyzer verifies that the caller's concrete type argument structurally satisfies all interface requirements at compile time.

---

## 4. Immutability & Reference Safety Rules

Lone enforces safety through compile-time diagnostic rules:

| Diagnostic Rule | Trigger Condition | Severity |
| :--- | :--- | :--- |
| `type/immutable-reassignment` | Attempting to mutate an immutable binding declared with `let` or a by-value function parameter (`p.x = val`). | Error |
| `type/immutable-field-write` | Attempting to assign to a struct field declared with `ro` outside of its `__init__` constructor. | Error |
| `type/immutable-reference-write`| Attempting to write through a read-only reference (`ref<ro T>`) or pointer (`ptr<ro T>`). | Error |
| `type/invalid-index` | Attempting to index a managed reference (`ref<T>`) or a scalar pointer (`ptr<T>`). | Error |
| `type/private-field-access` | Accessing private fields (`_field`) outside of methods declared on the enclosing struct. | Error |
| `type/raw-construct-private-fields`| Attempting raw aggregate instantiation `Type::{ ... }` on a struct containing private fields. | Error |
| `type/missing-init-for-ro-private-field`| Declaring a struct with private `ro` fields without providing an `__init__` constructor. | Error |
| `type/missing-trait-impl` | Using a type in formatted string interpolation `${expr}` that does not implement `TrDisplay`. | Error |

---

## 5. Auto-Dereferencing on Member Access (`evalPostfixExpr`)

To preserve clean ergonomics, field access (`p.field`) automatically unwraps references:
```go
if refT, ok := curr.ExprType().(*RefType); ok {
    if st, ok := refT.Elem.(*StructType); ok {
        targetStruct = st
    }
} else if ptrT, ok := curr.ExprType().(*PtrType); ok {
    if st, ok := ptrT.Elem.(*StructType); ok {
        targetStruct = st
    }
}
```
Field offset lookup, type extraction, and visibility checks occur against the unwrapped `*StructType`. Lowering emits direct offset loads without requiring explicit dereference syntax (`(*p).field`).
