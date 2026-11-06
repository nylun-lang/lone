# RFC-20261106: Modules, Package Entry Points (`_entry.lone`), and Re-exporting via `pub import`

- **RFC**: `RFC-20261106-modules_namespaces_and_reexporting`
- **Title**: Modules, Package Entry Points (`_entry.lone`), and Re-exporting via `pub import`
- **Author**: Human & AI Pair
- **Status**: Draft
- **Created**: 2026-11-06
- **Target Version**: v0.7.0
- **Related RFCs**:
  - [`RFC-20261106-symbol_documentation_directive`](../accepted/RFC-20261106-symbol_documentation_directive.md)
  - [`RFC-20261106-asymmetric_field_visibility`](../accepted/RFC-20261106-asymmetric_field_visibility.md)
  - [`RFC-20261106-documentation_generator_command`](../accepted/RFC-20261106-documentation_generator_command.md)
- **Applies to**: `pkg/lone-lexer`, `pkg/lone-parser-ast`, `pkg/lone-parser`, `pkg/lone-project`, `pkg/lone-analyzer`, `cmd/lone`, `dev/SPEC.md`

---

## 1. Summary

This RFC formalizes the module and package architecture of the Lone programming language:
1. **Conceptual Delineation (Module vs. Namespace/Package)**:
   - A **Module** is a single Lone source file (`.lone`) with its own private lexical scope.
   - A **Package / Namespace** is a directory tree of modules, referenced via colon-delimited paths (e.g. `:std:io`, `:math:linalg`).
2. **Directory Package Entry Point (`_entry.lone` / `_root.lone`)**:
   - Establishes canonical directory resolution. When a directory path is imported, the compiler automatically locates and evaluates `_entry.lone` (with `_root.lone` supported as a secondary fallback).
3. **Explicit Re-exporting via `pub import`**:
   - Extends the existing `pub` visibility modifier to import statements:
     ```lone
     pub import .Vector2, .Vector3 from .vectors
     pub import .Matrix4 as Mat4 from .matrices
     pub import :std:io as io
     ```
   - Brings symbols or namespace aliases into the current scope **and exports them** as part of the module's public interface.
4. **Strict Alignment with Invariant IL1 (Explicit Over Implicit)**:
   - Mandates explicit symbol lists (`pub import .sym1, .sym2 from ...`).
   - Strictly prohibits wildcard re-exports (`pub import .*`) to prevent namespace pollution and invisible dependency hazards.

---

## 2. Motivation

### 2.1 The Library Decomposition Problem
In Lone v0.6.0, codebases face an awkward structural trade-off:
- **Option A (Monolithic Files)**: Put all types, functions, and helpers into a single huge file (e.g. `math.lone`) so callers can import everything from `:math`.
- **Option B (Leaky Internal Decomposition)**: Break code cleanly into submodules (`math/vector.lone`, `math/matrix.lone`, `math/quaternion.lone`), but force every caller to know the internal directory structure:
  ```lone
  import .Vector3 from :math:vector
  import .Matrix4 from :math:matrix
  import .Quaternion from :math:quaternion
  ```

If the library author wants to reorganize `math/vector.lone` into `math/vec2.lone` and `math/vec3.lone`, **all downstream consumer code breaks**.

### 2.2 Why Existing Workarounds Are Insufficient
Currently, developers attempt to emulate re-exporting using type aliases and wrapper functions:
```lone
-- Tedious, error-prone workaround in math.lone:
import .Vector3 from .internal_vec
pub type Vector3 = internal_vec.Vector3 -- type alias duplicate

import .dot_product from .internal_vec
pub fn dot_product(a: Vector3, b: Vector3) f32 {
    return internal_vec.dot_product(a, b) -- redundant wrapper trampoline
}
```
This workaround:
- Produces redundant wrapper functions in generated WebAssembly bytecode.
- Requires maintaining duplicate signatures in two files.
- Clutters compiler symbol tables and hampers inlining heuristics.

### 2.3 Ergonomic Synergy with `pub`
Lone already uses the `pub` keyword to designate exported declarations:
- `pub type User = struct { ... }`
- `pub fn parse() i32 { ... }`
- `pub const MAX: i32 = 100`

Extending `pub` to imports is intuitive, minimal, and immediately understandable:
- `import .sym from .file` $\longrightarrow$ Private import (visible only in this file).
- `pub import .sym from .file` $\longrightarrow$ Public re-export (visible in this file and to external consumers).

---

## 3. Conceptual Model: Modules vs. Namespaces

To eliminate ambiguity across the compiler pipeline and documentation, Lone defines:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            LONE CODE ORGANIZATION                           │
├──────────────────────────────────────┬──────────────────────────────────────┤
│               MODULE                 │         PACKAGE / NAMESPACE          │
├──────────────────────────────────────┼──────────────────────────────────────┤
│ • A single .lone file                │ • A directory containing .lone files │
│ • Atomic unit of lexical scope       │ • Logical grouping of modules        │
│ • Private by default                 │ • Identified by colon path (:pkg:sub)│
│ • Imported relatively:               │ • Imported by namespace:             │
│   import .helper from .submod        │   import .io from :std:io            │
│ • Canonical entry file of a package  │ • Evaluated via package entry point: │
│   is named `_entry.lone`             │   `path/to/pkg/_entry.lone`          │
└──────────────────────────────────────┴──────────────────────────────────────┘
```

---

## 4. Detailed Specification

### 4.1 Package Entry Point Resolution (`_entry.lone` / `_root.lone`)

When the project resolver (`pkg/lone-project`) encounters an import source string:

#### A. Namespace Paths (`:namespace:subpackage`)
Given an import `from :foo:bar`:
1. Search root directories (`LONE_STD_DIR`, project workspace root, or library paths).
2. Resolve target according to the following order:
   1. **Direct Module File**: `<root>/foo/bar.lone`
   2. **Primary Package Entry Point**: `<root>/foo/bar/_entry.lone`
   3. **Secondary Package Entry Point**: `<root>/foo/bar/_root.lone`
3. If none of these exist, report compile error `import/module-not-found`.

#### B. Relative Paths (`.subpackage` or `..subpackage`)
Given an import `from .geometry`:
1. Search relative to the importing file's directory.
2. Resolve target in order:
   1. **Direct Module File**: `geometry.lone`
   2. **Primary Package Entry Point**: `geometry/_entry.lone`
   3. **Secondary Package Entry Point**: `geometry/_root.lone`
3. If none exist, report `import/module-not-found`.

#### Rationale for Leading Underscore (`_entry.lone` / `_root.lone`):
- **Collision Immunity**: Developers routinely name domain files `entry.lone` (e.g. A blog entry, accounting ledger entry). A leading underscore guarantees no naming clashes with domain models.
- **Visual Distinction**: In file trees, `_entry.lone` sorts to the top or stands out clearly as the package facade/manifest.
- **Internal Alignment**: Lone already uses leading underscores for internal conventions (`_secret` private fields, `__init__` constructors).

---

### 4.2 Re-exporting Syntax (`pub import`)

#### Grammar
```ebnf
ImportDecl    ::= [ "pub" ] "import" ( SymbolImport | NamespaceImport )
SymbolImport  ::= DotSymbol ( "," DotSymbol )* "from" ImportSource
DotSymbol     ::= "." IDENTIFIER [ "as" IDENTIFIER ]
NamespaceImport ::= ImportSource "as" IDENTIFIER
ImportSource  ::= NAMESPACE_PATH | RELATIVE_PATH
```

#### Form A: Explicit Symbol Re-export
```lone
-- Re-exports Vector2 and Vector3 under their original names
pub import .Vector2, .Vector3 from .vectors
```
External consumers importing this module can access `.Vector2` and `.Vector3` as direct public symbols.

#### Form B: Renamed / Aliased Re-export
```lone
-- Re-exports internal RawByteBuffer under the clean public name Buffer
pub import .RawByteBuffer as Buffer from .internal_buffer
```
External consumers import `.Buffer`. The private origin name `RawByteBuffer` is neither required nor leaked.

#### Form C: Namespace Alias Re-export
```lone
-- Re-exports an entire submodule namespace under an alias
pub import :std:io as io
```
External consumers can access functions via qualified namespace syntax: `MyModule.io.println(...)`.

---

### 4.3 Documentation Propagation & Override

A `pub import` statement may optionally be decorated with an `@fa:docs` directive:

```lone
@fa:docs "High-performance 3D spatial vector with SIMD acceleration."
pub import .Vector3 from .internal_vectors
```

#### Documentation Resolution Hierarchy:
1. **Facade Override**: If `@fa:docs` is attached to the `pub import` statement in `_entry.lone`, this documentation string is surfaced in the package's public API documentation.
2. **Origin Fallback**: If no `@fa:docs` is attached to the `pub import`, the documentation generator (`lone doc-gen json`) automatically inherits the `@fa:docs` attached to the symbol in its originating module.

---

### 4.4 Invariants & Semantic Rules

1. **Private Symbol Re-export Prohibition**:
   A module **cannot** re-export a symbol that is not declared `pub` in its origin module:
   ```lone
   -- In internal.lone:
   fn _secret_helper() void { ... } -- NOT pub

   -- In _entry.lone:
   -- COMPILE ERROR: import/cannot-reexport-private
   pub import ._secret_helper from .internal
   ```

2. **Wildcard Re-export Prohibition (Anti-Wildcard Invariant)**:
   Wildcard re-exports (`pub import .* from ...`) are strictly prohibited and trigger a compile-time syntax error (`syn/wildcard-import-prohibited`).
   *Rationale (IL1 - Explicit over implicit)*: Wildcards introduce silent namespace clashes, make dependency auditing impossible, and break downstream consumers whenever an internal file adds a new symbol.

3. **Diamond Re-export Canonicalization**:
   If module `A` and module `B` both re-export `User` from `core`, and a third module imports `User` from both `A` and `B`:
   $$\text{TypeID}(A.\text{User}) \equiv \text{TypeID}(B.\text{User}) \equiv \text{TypeID}(\text{core}.\text{User})$$
   Nominal type identity is preserved globally. There is no duplicate type instantiation or nominal incompatibility.

4. **Circular Re-export Guard**:
   The module resolver detects circular re-export dependency loops (e.g. `A` re-exports `B` which re-exports `A`) and reports `import/reexport-cycle` with the full dependency chain.

---

## 5. Concrete End-to-End Walkthrough

### 5.1 Package Layout (`src/math/`)
```
src/math/
├── _entry.lone            # Public facade of the :math package
├── vector.lone            # Internal vector implementations
├── matrix.lone            # Internal matrix implementations
└── _simd_intrinsics.lone  # Unexported hardware accelerator
```

### 5.2 Internal Module (`src/math/vector.lone`)
```lone
@fa:docs "A 3D coordinate in Euclidean space."
pub type Vector3 = struct {
    pub x: f32
    pub y: f32
    pub z: f32

    pub fn __init__(x: f32, y: f32, z: f32) Vector3 {
        return Vector3 { x: x, y: y, z: z }
    }
}

-- Private internal helper
fn _fast_inv_sqrt(n: f32) f32 { ... }
```

### 5.3 Package Facade (`src/math/_entry.lone`)
```lone
@fa:docs "Core mathematics package providing vectors, matrices, and spatial primitives."

-- Re-export public vector and matrix primitives
pub import .Vector3 from .vector
pub import .Matrix4 from .matrix

-- Private internal import (used only inside _entry.lone if needed)
import ._simd_intrinsics from ._simd_intrinsics
```

### 5.4 External Application Consumer (`src/main.lone`)
```lone
import .println from :std:io
import .Vector3, .Matrix4 from :math

pub fn main() void {
    let v = Vector3::(1.0, 2.0, 3.0)
    println("Vector initialized via clean package facade!")
}
```
The caller imports directly from `:math`. The internal file organization (`vector.lone` vs `matrix.lone`) is completely encapsulated.

---

## 6. Compiler Pipeline Architecture

```
[Source File: pub import .Foo from .bar]
                   │
                   ▼
       [pkg/lone-parser: parseImport]
       - Recognizes leading TokenPub
       - Sets loneast.ImportDecl.IsPub = true
       - Parses optional 'as <alias>' mappings
                   │
                   ▼
       [pkg/lone-project: Resolver]
       - Looks for bar.lone -> bar/_entry.lone -> bar/_root.lone
       - Traverses imported ASTs
       - Validates absence of re-export cycles
                   │
                   ▼
       [pkg/lone-analyzer: TypeEvaluator]
       - Hoists imported symbols into local scope
       - If IsPub == true:
         - Validates that origin symbol has pub visibility
         - Registers symbol into current module's ExportTable
         - Attaches documentation overrides if present
                   │
                   ▼
       [cmd/lone doc-gen json & pkg/lone-lsp]
       - Marks re-exported symbols with is_reexport: true
       - Surfaced seamlessly on package documentation index
```

---

## 7. Interaction with Existing RFCs

1. **`RFC-20261106-documentation_generator_command` (`lone doc-gen json`)**:
   `TypeDoc` and `FunctionDoc` schemas will include:
   - `is_reexport`: boolean flag.
   - `origin_module`: original source module path.
2. **`RFC-20261106-symbol_documentation_directive` (`@fa:docs`)**:
   Docstrings attached to `pub import` take precedence over origin docstrings when documenting the package facade.
3. **`RFC-20261106-trait_implementation_labels` (`@fa:impl_label`)**:
   When a struct is re-exported, all attached trait implementation witnesses (`Type->label`) are automatically available to callers importing the re-exported symbol.

---

## 8. Non-Goals

1. **Wildcard Imports**:
   Lone will **never** support `import .*` or `pub import .*`. Explicit declaration is a non-negotiable language invariant.
2. **Dynamic / Runtime Modules**:
   Module resolution is 100% static at compile time. No dynamic code loading or runtime `eval` modules will be introduced.
