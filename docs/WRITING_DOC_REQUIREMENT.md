# Documentation Requirements Specification (`WRITING_DOC_REQUIREMENT.md`)

This document defines the **mandatory structural and content requirements** for Lone documentation suites. Every release documentation set (e.g. `docs/docs/content/<version>/`) must satisfy these requirements to be considered complete, authoritative, and compliant with Lone project standards.

---

## 1. Documentation Philosophy: The 4-Pillar Standard

Lone is a statically-typed systems language targeting WebAssembly with zero runtime overhead. Its documentation must reflect its core motto:

> *"Easy to read, Easy to write, and Hard to misunderstand."*

Documentation is not simply an API catalog or a brief getting started tutorial; it is an integrated, progressive learning system. Every release documentation suite **MUST** be structured around the **Four Pillars of Lone Documentation**:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    THE FOUR PILLARS OF LONE DOCUMENTATION                    │
├──────────────────────┬──────────────────────┬────────────────────────────────┤
│ 1. GETTING STARTED   │ 2. GUIDE & REFERENCE │ 3. ADVANCED & LONE'S QUIRKS    │
│    - Prerequisites   │    - Values & Types  │    - Core Invariants (IL1-IL4) │
│    - Basic CLI       │    - Full Keywords   │    - Two-Tier Linear Memory    │
│    - "Hello World"   │    - Control Flow    │    - Asymmetric Visibility     │
│    - First Run       │    - Functions & Ops │    - Trait Specialization      │
├──────────────────────┴──────────────────────┴────────────────────────────────┤
│ 4. DEBUGGING WITH `lone debug`                                               │
│    - Struct Memory Layout & Offsets    - Enum Discriminant Payloads          │
│    - Interface Contract Verification   - Inherent Method Inspection          │
│    - Symbol Table Dumps                - Machine-Readable `--json` Export    │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Pillar 1: GETTING STARTED (`guide/`)

The **Getting Started** section is the primary on-ramp for developers. It must remove all friction and guide the reader from an empty directory to a running WebAssembly binary.

### 2.1 Prerequisites & Toolchain Installation
The documentation must explicitly document:
- **Host Prerequisites**:
  - Go 1.22+ (for building the compiler from source).
  - WebAssembly execution environment: [Wasmtime](https://wasmtime.dev/) (recommended), Node.js WASI preview 1, or modern Web browsers.
- **Source Compilation**:
  ```bash
  git clone https://github.com/lone-lang/lone.git
  cd lone
  go build -o lone ./cmd/lone
  ```
- **Installation Verification**:
  ```bash
  ./lone version
  ```

### 2.2 Basic CLI Operations
Must introduce the 4 fundamental CLI operations:
- `lone build <file.lone> [-o out.wat]`: Compile source to WebAssembly Text representation.
- `lone run <file.lone>`: Compile and execute immediately using in-process Wasmtime.
- `lone check <file.lone>`: Perform static analysis, type checking, and invariant verification without generating code.
- `lone parse <file.lone>`: Parse source into AST to verify grammar syntax.

### 2.3 Canonical "Hello World"
Must provide the canonical minimal program with full explanation:
```lone
import .println from :std:io

pub fn main() void {
    println("Hello from Lone!")
}
```
**Required Explanations**:
1. `import .println from :std:io`: Module import syntax, colon-delimited path `:std:io`, and leading dot `.println` for explicit symbol extraction.
2. `pub fn main() void`: Entry-point declaration, mandatory `pub` visibility, and explicit `void` return type.
3. Execution command and expected terminal output:
   ```bash
   lone run hello.lone
   # Hello from Lone!
   ```

---

## 3. Pillar 2: GUIDE & LANGUAGE REFERENCE (`language/`)

The Language Reference provides an exhaustive, authoritative breakdown of every syntactic and semantic construct in the language.

### 3.1 Values & Literals
Must document all literal representations:
- **Integers**: Decimal (`42`), Hexadecimal (`0x2A`), Octal (`0o52`), Binary (`0b101010`), and digit separators (`1_000_000`).
- **Floating-Point**: `3.14159`, scientific notation `1e-6`.
- **Booleans**: `true`, `false`.
- **Strings**:
  - Standard UTF-8 string literals with escape sequences (`\n`, `\t`, `\\`, `\"`).
  - Formatted String Interpolation `f"Value: ${expr}"`.
- **Aggregates**:
  - Struct literals: `Point { x: 10, y: 20 }`.
  - Canonical constructor calls: `Point::(10, 20)`.
  - Enum variants: `Option::Some(42)`, `Option::None`.
  - Fixed-size list literals: `[1, 2, 3, 4]`.

### 3.2 Complete Keyword & Lexical Dictionary
Must provide a comprehensive glossary covering all reserved keywords:
- **Declarations**: `pub`, `type`, `fn`, `let`, `var`, `const`, `ro`.
- **Control Flow**: `if`, `else`, `while`, `loop`, `match`, `return`, `break`, `continue`.
- **Error Handling**: `must`, `try`.
- **Type System**: `struct`, `enum`, `interface`, `satisfies`, `where`, `is`, `infer`, `ref`, `ptr`.
- **Directives**: `@fa:docs`, `@fa:impl_label`, `@fa:fn_overload`, `@fa:wasm`.
- **Intrinsics**: `#print`, `#panic`, `#alloc`, `#free`.
- **Resource Accounting**: `drop`.
- **Object Context**: `self`, `Self`.

### 3.3 The Type System
Must exhaustively detail:
- **Primitives**:
  - Sized signed integers: `i8`, `i16`, `i32`, `i64`, `i128`.
  - Sized unsigned integers: `u8`, `u16`, `u32`, `u64`, `u128`.
  - Platform integer: `int` (defaults to `i32` in 32-bit Wasm).
  - Floating-point: `f32`, `f64`.
  - Logic & text: `bool`, `str`, `void`.
- **Nominal Structs**:
  - Physical fields, field types, default field values.
  - Inherent methods with `self: ref<T>` or `self: T`.
  - Canonical constructors (`pub fn __init__(...)`).
- **Tagged Union Enums**:
  - Algebraic data types with variant payloads:
    ```lone
    pub type Shape = enum {
        Circle(radius: f32),
        Rectangle(w: f32, h: f32),
        Point,
    }
    ```
  - Standard library enums: `Option<T>`, `Result<V, U>`.
- **Interface Data Contracts**:
  - Pure structural duck-typing for fields and methods:
    ```lone
    pub type IfcNamed = interface {
        ro name: str
        fn get_id() i32
    }
    ```
- **References & Pointers**:
  - Safe references: `ref<T>` (immutable by default), `ref<ro T>`.
  - Raw linear memory pointers: `ptr<T>`.
  - Sized buffer pointers: `ptr<[N]T>` (typestate with compile-time bounds checking).
  - Fat slice references: `ref<[]T>`.

### 3.4 Control Flow
Must cover all branching and looping constructs:
- **`if` / `else`**: Conditional execution, multi-branch packed `if`, statement vs expression semantics.
- **Loops**:
  - Pre-condition `while [cond] { ... }`.
  - Infinite `loop { ... }`.
  - Post-condition `loop { ... } while: [cond]` with scoped bindings.
- **Pattern Matching (`match`)**:
  - Exhaustive matching on tagged unions.
  - Payload binding destructuring: `match shape { Shape::Circle(r) => ..., ... }`.
  - Compiler error on non-exhaustive matches (`type/non-exhaustive-match`).
- **Loop Control**: `break`, `continue`, labeled blocks, and returning values from blocks.

### 3.5 Functions, Methods & Modules
- Function signatures, parameter types, explicit return types.
- Inherent methods and receiver parameters (`self: ref<T>`, `self: T`).
- File-based modules and namespace paths (`:std:io`, `:math:geometry`).
- Module exports (`pub`) and encapsulation.

### 3.6 Error Handling Architecture
- The standard error paradigm using `Result<V, U>` and `Option<T>`.
- `must <expr>`: Unwraps `Result::Ok(val)` or panics immediately via WebAssembly `unreachable`.
- `try <expr>`: Unwraps `Result::Ok(val)` or early-returns `Result::Err(err)` to the caller.

---

## 4. Pillar 3: ADVANCED FANCY STUFF & LONE'S QUIRKS (`language/` & `runtime/`)

Lone contains unique architectural invariants and systems mechanisms that distinguish it from C, Rust, Go, or Python. Documentation must highlight these "fancy" features and quirks to prevent false assumptions.

### 4.1 The Core Invariants (The "Lone Way")
Must prominently explain:
- **IL1 - Explicit Over Implicit**:
  - **Zero implicit type widening or numeric coercion**. `i8` does NOT implicitly become `i32`. All conversions require explicit casts (`x as i32`).
  - No hidden vtables, no invisible boxing, and no magical operator hijacking.
- **IL2 - Intrinsics Separation**:
  - All built-in compiler intrinsics are prefixed with `#` (e.g. `#print`, `#panic`, `#alloc`, `#free`) so they never clash with user-defined identifiers.
- **IL3 - Strict Resource & Result Accounting**:
  - Non-void expressions in statement position **must be bound or explicitly dropped**:
    ```lone
    -- COMPILE ERROR: type/unused-result
    calculate_total()

    -- VALID: Explicit drop
    drop calculate_total()
    ```
- **IL4 - Explicit Trait Materialization**:
  - Traits are pure contracts that **never inject methods into type namespaces**.
  - Methods must be explicitly materialized using `TrTrait::|TargetType|`:
    ```lone
    TrSerializable::|User|.serialize(user)
    ```

### 4.2 Two-Tier Linear Memory Model
Must detail the physical memory organization:
```
0x0000 ┌──────────────────────────────────────────────┐
       │ Reserved Scratchpad (0..15)                   │
       ├──────────────────────────────────────────────┤
       │ Static Strings & Constants                    │
       ├──────────────────────────────────────────────┤
       │ Page 0: Downward Datastack (Frame Stack)      │
       │ ($__lone_stack_ptr moves downward)            │
       │ - O(1) Zero-cost frame allocation & rewind    │
0xFFFF ├──────────────────────────────────────────────┤  Boundary: Page 1 (64KB)
       │ Page 1+: Upward Dynamic Heap                  │
       │ - Free-List Allocator ($__lone_rt_alloc)      │
       │ - Bi-directional header coalescing            │
       │ - Dynamic expansion via memory.grow           │
       └──────────────────────────────────────────────┘
```
**Quirks to Document**:
- Why aggregate return values (structs, tuples) are copied by value from Page 0, eliminating lifetime/escape bugs.
- Why heap allocation is strictly separated from stack scratchpads.

### 4.3 Asymmetric Field Visibility (`ro(pub)`)
Must document how Lone solves the "getter/setter boilerplate" problem:
```lone
pub type Account = struct {
    ro(pub) balance: i64    -- Publicly readable, privately mutable
    pub id: str             -- Publicly readable and mutable
    _internal_key: i32      -- Struct-private
}
```
- **Access Rule**: Outside the declaring module, `account.balance` is strictly read-only. External assignments trigger `type/asymmetric-field-write`.
- **Reference Rule**: External callers can only take `ref<ro i64>` references, never mutable `ref<i64>`.
- **Zero Overhead**: Direct linear memory load without method call or vtable overhead.

### 4.4 Trait Implementation Labels (`@fa:impl_label`, `Type->label`)
Must document specialized trait materialization:
- **The Problem**: A single struct often needs multiple serialization formats (e.g. JSON vs Compact Binary).
- **Declaration**:
  ```lone
  @fa:impl_label "json"
  impl TrSerialize for User { ... }
  ```
- **Arrow Specialization**: Callers specify the witness using `Type->label`:
  ```lone
  TrSerialize::|User->json|.serialize(u)
  serialize_payload<User->json>(u)
  ```
- **Strict Invariants**:
  1. Struct-only: labels are forbidden on primitives, enums, and interfaces.
  2. Mandatory default implementation before labeled variations can be defined.
  3. Generic Anti-Coupling Invariant: Generic parameters cannot specify labels (`TrT::|T->label|` is forbidden).

### 4.5 Deterministic Function & Constructor Overloading (`@fa:fn_overload`)
Must document constructor and function overloading:
- Guarded explicitly under the `@fa:fn_overload` directive.
- Multiple constructor arities (`__init__()` vs `__init__(max: i32)`).
- Deterministic resolution based on exact arity and exact types (zero ambiguity due to no implicit promotions).
- Strict prohibition of return-type-only overloading.
- Deterministic WebAssembly symbol mangling (`$Game.__init__$i32`).

### 4.6 First-Class Symbol Documentation (`@fa:docs`)
Must document docstring directives:
- Target scope: `type`, `global`, `fn`.
- Block concatenation of consecutive `@fa:docs` directives.
- Zero runtime footprint (0 bytes emitted in WebAssembly binary).

---

## 5. Pillar 4: DEBUGGING WITH `lone debug` (`toolchain/lone-debug.md`)

Systems programmers need deep, immediate visibility into memory layouts and compiler lowerings. The documentation suite **MUST** contain a dedicated, thorough guide to `lone debug`.

### 5.1 Overview of `lone debug`
- Explain that `lone debug` is a built-in static introspection engine that analyzes source files and outputs deep structural, memory, and semantic data without needing GDB, LLDB, or a running WebAssembly process.

### 5.2 Struct Memory Layout & Offsets Inspection
Must demonstrate how to inspect struct byte layouts:
```bash
lone debug program.lone Game
```
**Expected Documentation Content**:
- Table of field offsets, field types, and alignments.
- Automatic padding bytes added for alignment.
- Total struct byte size.
- Inherent methods and constructor signatures attached to the struct.
- Practical use case: optimizing struct layout to eliminate padding waste in WebAssembly linear memory.

### 5.3 Interface Contract Satisfaction
Must demonstrate contract validation:
- Displays all interfaces defined in the module.
- Displays whether a target struct satisfies an interface contract structurally.
- Highlights missing fields, type mismatches, or mutability conflicts.

### 5.4 Enum Discriminant & Payload Layouts
Must show how tagged unions are represented in memory:
- Tag discriminant size and index (e.g. `tag: i32` at offset 0).
- Variant payload layout and shared memory union size.
- Total memory footprint of the enum.

### 5.5 Machine-Readable JSON Export (`--json`)
Must document how to use `--json` for tooling, CI, and IDE integration:
```bash
lone debug program.lone --json
```
**Required Schema Explanations**:
- `types`: Structs, fields (name, type, offset, size, visibility), enums (tags, variants, payloads), interfaces.
- `functions`: Signatures, parameters, return types, mangled Wasm symbol identifiers.
- `globals`: Names, types, addresses, mutability.
- `directives`: Associated `@fa:docs`, `@fa:impl_label`, `@fa:fn_overload` metadata.

### 5.6 Interactive Debugging & Diagnostic Workflows
Must provide step-by-step diagnostic workflows:
1. **Diagnosing Alignment Waste**: How to rearrange struct fields to minimize linear memory usage.
2. **Diagnosing Interface Mismatches**: How to use `lone debug` to pinpoint why a struct fails a generic `satisfies` constraint.
3. **Diagnosing Overload Clashes**: Inspecting mangled symbols to resolve signature collisions.

---

## 6. Documentation Compliance & Auditing Matrix

Every version documentation set (`docs/docs/content/<version>/`) must be audited against this checklist before release:

| Section | Required Topic | Target File | Status |
| :--- | :--- | :--- | :--- |
| **Pillar 1** | Prerequisites, Toolchain Setup, Go 1.22+ | `guide/getting-started.md` | Required |
| **Pillar 1** | Basic CLI (`build`, `run`, `check`, `parse`) | `toolchain/lone-cli.md` | Required |
| **Pillar 1** | Canonical "Hello World" & execution | `guide/getting-started.md` | Required |
| **Pillar 2** | Values & Literals (numbers, strings, `f"..."`) | `language/variables-and-types.md` | Required |
| **Pillar 2** | Complete Keyword & Lexical Dictionary | `language/type-system.md` | Required |
| **Pillar 2** | Primitive Types & Compound Structs | `language/variables-and-types.md` | Required |
| **Pillar 2** | Tagged Union Enums & Option/Result | `language/type-system.md` | Required |
| **Pillar 2** | Interface Data Contracts | `language/traits.md` | Required |
| **Pillar 2** | References (`ref<T>`) & Pointers (`ptr<T>`) | `language/memory.md` | Required |
| **Pillar 2** | Control Flow (`if`, `while`, `loop`, `match`) | `language/control-flow.md` | Required |
| **Pillar 2** | Error Handling Architecture (`must`, `try`) | `language/control-flow.md` | Required |
| **Pillar 3** | Core Invariants (IL1–IL4, zero coercion) | `guide/introduction.md` | Required |
| **Pillar 3** | Resource Accounting (`drop <expr>`) | `language/type-system.md` | Required |
| **Pillar 3** | Two-Tier Linear Memory (Page 0 vs Page 1+) | `toolchain/wasm-and-wasi.md` | Required |
| **Pillar 3** | Asymmetric Field Visibility (`ro(pub)`) | `language/variables-and-types.md` | Required |
| **Pillar 3** | Trait Implementation Labels (`Type->label`) | `language/traits.md` | Required |
| **Pillar 3** | Function & Constructor Overloading | `language/functions.md` | Required |
| **Pillar 3** | First-Class Symbol Docs (`@fa:docs`) | `language/intrinsics.md` | Required |
| **Pillar 4** | `lone debug` Overview & Usage | `toolchain/lone-debug.md` | Required |
| **Pillar 4** | Struct Memory Layout, Offsets & Padding | `toolchain/lone-debug.md` | Required |
| **Pillar 4** | Interface Satisfaction Inspection | `toolchain/lone-debug.md` | Required |
| **Pillar 4** | Enum Discriminants & Payload Inspection | `toolchain/lone-debug.md` | Required |
| **Pillar 4** | Machine-Readable JSON Export (`--json`) | `toolchain/lone-debug.md` | Required |
