# Specification Update Report: Populated `drafts/SPEC.md`

- **Branch**: `feat/better-type`
- **Date**: 2026-11-01
- **Status**: Completed

---

## 1. Overview

Populated [`drafts/SPEC.md`](file:///home/char-c/Projects/lone/drafts/SPEC.md) to provide an authoritative, comprehensive specification of the Lone Programming Language representing the **current implementation** in `packages/lone-lang`.

---

## 2. Contents of `drafts/SPEC.md`

1. **Philosophy & Invariants**:
   - Explicit over implicit (`IL1`), distinguished built-in `#` symbols (`IL2`), progressive left-to-right syntax (`IF2`), stable UI topologies (`IUI1`).
2. **Lexical Structure**:
   - Identifiers and conventions (`snake_case`, `CamelCase`, `ALL_CAPS`, `#` prefix for intrinsics/constructors).
   - Word-boundary guarded keywords.
   - Single-line, multi-line, and compiler directives (`--fa:`).
   - Primitive literals (integers, floats, booleans, `none`, escaped strings).
3. **Data Structure Literals**:
   - Object literals (`{ .foo: "bar" }`).
   - List literals (`{ 1, 2, 3 }`).
4. **Type System (`TYPE_REF`)**:
   - Constraints (`comptime`, `ro`, `?`).
   - Three-way structural split:
     - `TypeBase`: Identifiers and generic arguments (`list<int>`, `Map<K, V>`).
     - `TypeList`: Dynamic slices (`[]T`) and fixed-size arrays (`[size]T`).
     - `TypeFn`: First-class function signatures (`fn (args) Ret`).
5. **Declarations (`TOP_LEVEL`)**:
   - Variable declarations with inference and uninitialized states (`BIND_DEC`).
   - Function declarations (`FN_DEC`) with constructors (`#init`).
   - Labeled blocks and control expressions (`LABEL_DEC`).
6. **Statements & Control Flow**:
   - Return statements (`RETURN_STMT`).
   - Break statements (`BREAK_STMT`), with explicit label targeting and value-yielding semantics (`break name: expr`).
   - If statements (`IF_STMT`) with `else if` chaining and `else` branches.
   - While loops (`WHILE_STMT`).
7. **Expressions & Operator Precedence**:
   - Labeled expressions.
   - Postfix operations (member, namespace, associated, call, constructor init, type make, chaining).
   - Unary operators (`-`, `+`, `!`, `&`, `*`).
   - Binary operator precedence hierarchy (multiplicative, additive, relational, equality, logical and/or).
8. **Intermediate Representation Reference**:
   - Complete mapping table between source syntax and AST/IR types in `packages/lone-lang/irtypes`.
