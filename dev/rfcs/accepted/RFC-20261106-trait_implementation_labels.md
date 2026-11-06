# RFC-20261106: Trait Implementation Labels via `@fa:impl_label`

- **RFC**: `RFC-20261106-trait_implementation_labels`
- **Title**: Trait Implementation Labels via `@fa:impl_label`
- **Author**: Human & AI Pair
- **Status**: Accepted
- **Created**: 2026-11-06
- **Target Version**: v0.7.0
- **Related RFCs**:
  - [`RFC-20261104-interface_data_contracts`](./RFC-20261104-interface_data_contracts.md)
  - [`RFC-20261106-symbol_documentation_directive`](./RFC-20261106-symbol_documentation_directive.md)
- **Applies to**: `pkg/lone-lexer`, `pkg/lone-parser-ast`, `pkg/lone-parser`, `pkg/lone-analyzer`, `pkg/lone-compiler-wir`, `dev/SPEC.md`

---

## 1. Summary

This RFC introduces **Trait Implementation Labels** via the `@fa:impl_label <string>` compiler directive, paired with the arrow specialization syntax `Type->label`.

Key capabilities:
1. **Alternative Trait Implementations**: Allows a single struct or type to define multiple distinct implementation strategies for the **exact same trait `TrT`** (e.g. buffered vs unbuffered I/O, synchronous vs asynchronous write, compact vs verbose formatting).
2. **Strict Signature Invariance**: Every labeled implementation **must adhere 100% to the trait's declared method signatures**. No method signature divergence is permitted, ensuring complete stability and type safety.
3. **Mandatory Default Implementation**: A type **must** provide an unlabeled (default) implementation of `TrT` before or alongside any labeled implementation. A type cannot declare labeled variations in isolation.
4. **Explicit Arrow Materialization (`Type->label`)**: Callers select between the default implementation and labeled alternatives using clean arrow specialization syntax:
   - Default (unlabeled): `TrT::|TargetType|`
   - Labeled alternative: `TrT::|TargetType->label|`
5. **Call-Site Generic Witness Specialization (`func<TargetType->label>(arg)`)**:
   - Generic function authors write decoupled code using standard `TrT::|T|`.
   - Callers select the implementation witness at the call site via `func<TargetType->label>(arg)`.
   - Generic type parameters `T` are strictly forbidden from specifying labels in materialization (`TrT::|T->label|` is a compile error).
6. **Zero Call-Site Ambiguity**: Because Lone traits are pure contracts that do not inject methods into type namespaces (**Invariant IL4**), having multiple implementations never creates ambiguous member resolution.
7. **Deterministic Wasm Symbol Mangling**: Labeled methods are deterministically mangled as `$TrTrait::method$TargetType$label` in WebAssembly, compiling to static direct `call` instructions with zero vtable overhead.

---

## 2. Motivation

In trait-based type systems (such as Rust or Haskell), a type can only implement a trait once:
```rust
// Rust: only one Display implementation permitted for User
impl Display for User { ... }
```

When a developer needs alternative implementation strategies of the same trait for the same type (for example, a `Logger` implementing `TrWriter` with standard buffering vs direct synchronous disk flush, or a `User` implementing `TrDisplay` with standard output vs compact JSON serialization), traditional languages force developers into the **"Newtype Pattern"**:
```rust
// Newtype workaround in Rust:
struct SyncLogger<'a>(&'a Logger);
impl<'a> Write for SyncLogger<'a> { ... }
```

This workaround introduces:
- Redundant wrapper struct definitions.
- Runtime wrapping and unwrapping boilerplate.
- Friction in parameter passing and borrowing.

### Why Lone Can Solve This Cleanly
In languages with implicit method dispatch (`val.method()`), two implementations of `TrWriter` for `Logger` would make `logger.write(...)` ambiguous.

In Lone, **trait methods are never implicitly dispatched on structs** (**IL4 - Explicit Trait Materialization**). Trait methods must be explicitly materialized:
```lone
let w = TrWriter::|Logger|
w.write(logger, "message")
```

Because materialization is already explicit, Lone can support multiple labeled implementations without any call-site ambiguity whatsoever.

---

## 3. Detailed Specification

### 3.1 Directive Syntax

The `@fa:impl_label` directive accepts a string literal label and precedes an `impl` block:

$$\mathbf{@fa:impl\_label}\;\; \text{"<label>"}\;\; \mathbf{[\textbackslash n]}\;\; \mathbf{impl}\;\; \text{TrTrait}\;\; \mathbf{for}\;\; \text{TargetType}\;\; \mathbf{\{ ... \}}$$

```lone
pub trait TrWriter {
    fn write(self: ref<Self>, msg: str) void
}

-- 1. Default (unlabeled) implementation: standard buffered writing (MANDATORY)
impl TrWriter for FileLogger {
    fn write(self: ref<FileLogger>, msg: str) void {
        self.buffer.append(msg)
        if self.buffer.len >= 4096 {
            self.flush()
        }
    }
}

-- 2. Labeled implementation: direct unbuffered synchronous write
@fa:impl_label "direct_sync"
impl TrWriter for FileLogger {
    fn write(self: ref<FileLogger>, msg: str) void {
        self.flush()
        #write_raw_fd(self.fd, msg)
        #fsync(self.fd)
    }
}
```

---

## 4. Semantic Rules & Invariants

### Rule 1: Mandatory Default Implementation
A type **must** provide an unlabeled (default) implementation of `TrTrait` before or alongside any `@fa:impl_label` implementation:

```lone
@fa:impl_label "json"
impl TrDisplay for User { ... } -- ERROR if no default impl TrDisplay for User exists!
```

**Diagnostic**:
```text
error[type/missing-default-trait-impl]: labeled implementation '@fa:impl_label "json"' requires a default implementation of 'TrDisplay' for 'User'
  --> src/user.lone:15:1
   |
15 | @fa:impl_label "json"
   | ^^^^^^^^^^^^^^^^^^^^^ type 'User' must provide an unlabeled default implementation before declaring labeled variations
   = note: default implementations guarantee that 'User' unconditionally satisfies 'TrDisplay' in generic contexts
```

**Rationale**:
- Guarantees that any type with labeled implementations **always satisfies the base trait unconditionally**.
- Prevents fragmented types where conformance depends on whether a caller knows a private label.
- Guarantees that generic libraries calling standard `TrTrait::|T|` will never encounter missing implementations.

### Rule 2: Strict Signature Invariance (Anti-Divergence)
Every method declared within a labeled `impl` block **must match the trait definition exactly**:
- Identical method names.
- Identical parameter counts and parameter types.
- Identical return type.

Attempting to change parameter types or return types in a labeled implementation is a compile-time static error (`type/trait-signature-mismatch`):

```lone
@fa:impl_label "bad"
impl TrWriter for FileLogger {
    fn write(self: ref<FileLogger>, msg: str, priority: i32) void { -- ERROR!
        ...
    }
}
```

**Diagnostic**:
```text
error[type/trait-signature-mismatch]: method 'write' in @fa:impl_label "bad" does not match trait 'TrWriter'
  --> src/logger.lone:25:5
   |
25 |     fn write(self: ref<FileLogger>, msg: str, priority: i32) void {
   |     ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ expected 2 parameters (self, str), got 3
```

### Rule 3: Complete Implementation
A labeled `impl` block must implement **all** required methods of the trait. Partial implementations are rejected (`type/incomplete-trait-impl`).

### Rule 4: Unique Label per (Trait, Type) Pair
A type cannot declare two implementations with the same label for the same trait (`sym/duplicate-impl-label`).

---

## 5. Materialization Mechanics & Generic Interaction

Callers select between the default implementation and labeled alternatives using explicit arrow materialization syntax:

### 5.1 Direct Concrete Materialization (`TrTrait::|TargetType->label|`)

| Target | Syntax | Description |
| :--- | :--- | :--- |
| **Default Implementation** | `TrTrait::\|TargetType\|` | Resolves the primary, unlabeled `impl TrTrait for TargetType`. |
| **Labeled Implementation** | `TrTrait::\|TargetType->label\|` | Resolves the alternative `impl` annotated with `@fa:impl_label "label"`. |

```lone
pub fn main() void {
    let logger = FileLogger::("app.log")

    -- 1. Use default buffered implementation:
    let default_writer = TrWriter::|FileLogger|
    default_writer.write(&logger, "Standard log entry\n")

    -- 2. Use direct synchronous implementation via arrow syntax:
    let sync_writer = TrWriter::|FileLogger->direct_sync|
    sync_writer.write(&logger, "CRITICAL ERROR: flush immediately\n")
}
```

Both `default_writer` and `sync_writer` have the **exact same type signature** (`MaterializedTraitType` of `TrWriter`).

---

### 5.2 Generic Functions: Decoupling & The Anti-Coupling Invariant

When developing generic libraries, the author defines contracts using generic constraints:
```lone
fn process_output<T where { T satisfies TrDisplay }>(item: T) void {
    -- Invariant: Generic parameters CANNOT permit label materialization!
    let d = TrDisplay::|T|
    println(d.display(item))
}
```

#### Rule 5: Generic Type Parameters CANNOT Permit Label Materialization
Inside a generic function body, attempting to qualify a generic type parameter `T` with a label during materialization is a **strict compile-time error** (`type/generic-parameter-label-forbidden`):

```lone
fn bad_generic<T where { T satisfies TrDisplay }>(item: T) void {
    let d = TrDisplay::|T->json| -- COMPILE ERROR!
}
```

**Diagnostic**:
```text
error[type/generic-parameter-label-forbidden]: cannot materialize trait with label on generic type parameter 'T'
  --> src/render.lone:12:13
   |
12 |     let d = TrDisplay::|T->json|
   |             ^^^^^^^^^^^^^^^^^^^^ generic parameter 'T' must use unlabeled materialization TrDisplay::|T|
   = note: generic code cannot assume external types implement specific labels; callers select labels at the call site
```

**Rationale**:
- Generic function authors cannot know what custom labels application developers define on their structs.
- Hardcoding a label like `T->json` inside generic code would couple the function to arbitrary label names and break generic substitution whenever a type without that label is supplied.
- Generic functions must stay 100% decoupled and write standard `TrTrait::|T|`.

---

### 5.3 Call-Site Witness Specialization (`func<TargetType->label>(arg)`)

How does an application developer execute a generic function using a labeled implementation?

Instead of forcing developers to write artificial wrapper structs ("Newtypes"), the caller **specializes the trait witness at the call site**:

```lone
let user = User::("Alice", 101)

-- 1. Standard call: uses default TrDisplay implementation:
process_output<User>(user)

-- 2. Labeled call: specializes T to User using the "json" witness:
process_output<User->json>(user)
```

#### Zero-Cost Value Representation
- `User->json` has the **exact same memory layout, size, and alignment** as `User`.
- The value passed is raw `user` (no wrapper allocations, no conversions).
- The `->json` qualifier exists purely as a compile-time witness selector for trait resolution.

#### Monomorphization & Code Generation
When the compiler monomorphizes `process_output<T>`:
- `process_output<User>(user)` generates `$process_output$User`:
  - `TrDisplay::|T|` compiles to static call `call $TrDisplay::display$User`.
- `process_output<User->json>(user)` generates `$process_output$User$json`:
  - `TrDisplay::|T|` compiles to static call `call $TrDisplay::display$User$json`!

Both compile to direct, static WebAssembly `call` instructions with zero vtables and zero dispatch penalty.

---

### 5.4 The Graceful Fallback Rule

When a generic function requires multiple trait constraints:
```lone
fn persist_and_log<T where { T satisfies TrDisplay, T satisfies TrSerializable }>(item: T) void {
    let d = TrDisplay::|T|
    let s = TrSerializable::|T|
    ...
}
```

If the caller invokes `persist_and_log<User->json>(user)`:
1. **Matching Trait**: For `TrDisplay`, `User` defines `@fa:impl_label "json"` $\rightarrow$ uses `"json"` implementation.
2. **Non-Matching Trait**: For `TrSerializable`, `User` defines no `"json"` label $\rightarrow$ **gracefully falls back to default unlabeled implementation**.
3. **Missing Trait**: If neither a labeled nor a default implementation exists $\rightarrow$ compile error (`type/trait-not-satisfied`).

---

## 6. WebAssembly Code Generation & Mangling

To ensure collision-free function names in WebAssembly Text (`.wat`) and binary (`.wasm`):

1. **Default Method Symbol**:
   $$\mathbf{\$TrTrait::method\$TargetType}$$
   *Example*: `$TrWriter::write$FileLogger`

2. **Labeled Method Symbol**:
   $$\mathbf{\$TrTrait::method\$TargetType\$label}$$
   *Example*: `$TrWriter::write$FileLogger$direct_sync`

3. **Inlined Dispatch**:
   Because materialization is resolved statically at compile time, calls to `writer.write(...)` compile to **direct static `call` instructions**:
   ```wat
   ;; Calling default writer:
   call $TrWriter::write$FileLogger

   ;; Calling direct_sync writer:
   call $TrWriter::write$FileLogger$direct_sync
   ```
   **Zero vtable overhead, zero runtime dispatch penalty.**

---

## 7. Comparison with Other Approaches

| Approach | Declaration Overhead | Call-Site Ambiguity | Signature Invariance | Runtime Cost |
| :--- | :---: | :---: | :---: | :---: |
| **Rust Newtype Wrapper** | High (wrapper struct + conversions) | None (diverted type) | Enforced | Zero |
| **Haskell Named Instances** | Medium | High (in generic contexts) | Enforced | None |
| **Lone `@fa:impl_label` (`T->label`)** | **Lowest (Single directive)** | **Zero (Explicit Materialization)** | **Strictly Enforced** | **Zero (Direct static call)** |

---

## 8. Summary of Benefits

1. **Eliminates Boilerplate**: Eliminates synthetic wrapper types (`SyncLogger`, `JsonUser`) whose sole purpose is satisfying the single-impl restriction.
2. **Contract Stability**: Guarantees that all implementations strictly honor the trait contract without method signature divergence.
3. **Guaranteed Base Conformance**: Mandatory default implementation guarantees that all types unconditionally satisfy the trait contract across all generic code.
4. **Decoupled Generics**: Generic code writes simple `TrT::|T|`, while callers retain call-site power (`foo<User->json>`).
