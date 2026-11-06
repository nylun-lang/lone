# RFC-20261106: Trait Implementation Labels via `@fa:impl_label`

- **RFC**: `RFC-20261106-trait_implementation_labels`
- **Title**: Trait Implementation Labels via `@fa:impl_label`
- **Author**: Human & AI Pair
- **Status**: Draft
- **Created**: 2026-11-06
- **Target Version**: v0.7.0
- **Related RFCs**:
  - [`RFC-20261104-interface_data_contracts`](../accepted/RFC-20261104-interface_data_contracts.md)
  - [`RFC-20261106-symbol_documentation_directive`](../accepted/RFC-20261106-symbol_documentation_directive.md)
- **Applies to**: `pkg/lone-parser-ast`, `pkg/lone-parser`, `pkg/lone-analyzer`, `pkg/lone-compiler-wir`, `dev/SPEC.md`

---

## 1. Summary

This RFC introduces **Trait Implementation Labels** via the `@fa:impl_label <string>` compiler directive.

Key capabilities:
1. **Alternative Trait Implementations**: Allows a single struct or type to define multiple distinct implementation strategies for the **exact same trait `TrT`** (e.g. buffered vs unbuffered I/O, synchronous vs asynchronous write, compact vs verbose formatting).
2. **Strict Signature Invariance**: Every labeled implementation **must adhere 100% to the trait's declared method signatures**. No method signature divergence is permitted, ensuring complete stability and type safety.
3. **Explicit Materialization Selection**: Callers select between the default implementation and labeled alternatives using explicit materialization syntax:
   - Default (unlabeled): `TrT::|TargetType|`
   - Labeled alternative: `TrT::|TargetType, "label"|`
4. **Zero Call-Site Ambiguity**: Because Lone traits are pure contracts that do not inject methods into type namespaces (**Invariant IL4**), having multiple implementations never creates ambiguous member resolution.
5. **Deterministic Wasm Symbol Mangling**: Labeled methods are deterministically mangled as `$TrTrait::method$TargetType$label` in WebAssembly, guaranteeing collision-free code generation.

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

-- 1. Default (unlabeled) implementation: standard buffered writing
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

## 4. Invariant Trait Contract & Anti-Divergence Rule

### Rule 1: Strict Signature Matching
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

### Rule 2: Complete Implementation
A labeled `impl` block must implement **all** required methods of the trait. Partial implementations are rejected (`type/incomplete-trait-impl`).

### Rule 3: Unique Label per (Trait, Type) Pair
A type cannot declare two implementations with the same label for the same trait:
```lone
@fa:impl_label "fast"
impl TrMath for Calculator { ... }

@fa:impl_label "fast" -- ERROR: duplicate label "fast"
impl TrMath for Calculator { ... }
```

---

## 5. Materialization Mechanics

Callers select between the default implementation and labeled alternatives using explicit materialization syntax:

### 5.1 Syntax

| Target | Syntax | Description |
| :--- | :--- | :--- |
| **Default Implementation** | `TrTrait::\|TargetType\|` | Resolves the primary, unlabeled `impl TrTrait for TargetType`. |
| **Labeled Implementation** | `TrTrait::\|TargetType, "<label>"\|` | Resolves the alternative `impl` annotated with `@fa:impl_label "<label>"`. |

```lone
pub fn main() void {
    let logger = FileLogger::("app.log")

    -- 1. Use default buffered implementation:
    let default_writer = TrWriter::|FileLogger|
    default_writer.write(&logger, "Standard log entry\n")

    -- 2. Use direct synchronous implementation:
    let sync_writer = TrWriter::|FileLogger, "direct_sync"|
    sync_writer.write(&logger, "CRITICAL ERROR: flush immediately\n")
}
```

Both `default_writer` and `sync_writer` have the **exact same type signature** (`MaterializedTraitType` of `TrWriter`).

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
| **Lone `@fa:impl_label`** | **Lowest (Single directive)** | **Zero (Explicit Materialization)** | **Strictly Enforced** | **Zero (Direct static call)** |

---

## 8. Summary of Benefits

1. **Eliminates Boilerplate**: Eliminates synthetic wrapper types (`SyncLogger`, `JsonUser`) whose sole purpose is satisfying the single-impl restriction.
2. **Contract Stability**: Guarantees that all implementations strictly honor the trait contract without method signature divergence.
3. **Transparent Ergonomics**: Reuses Lone's explicit `TrTrait::|Target, "label"|` materialization cleanly.
