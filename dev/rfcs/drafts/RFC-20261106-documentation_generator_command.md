# RFC-20261106: CLI Documentation Generator via `lone doc-gen json`

- **RFC**: `RFC-20261106-documentation_generator_command`
- **Title**: CLI Documentation Generator via `lone doc-gen json`
- **Author**: Human & AI Pair
- **Status**: Draft
- **Created**: 2026-11-06
- **Target Version**: v0.7.0
- **Related RFCs**:
  - [`RFC-20261106-symbol_documentation_directive`](../accepted/RFC-20261106-symbol_documentation_directive.md)
  - [`RFC-20261106-trait_implementation_labels`](../accepted/RFC-20261106-trait_implementation_labels.md)
  - [`RFC-20261106-function_and_method_overloading`](../accepted/RFC-20261106-function_and_method_overloading.md)
  - [`RFC-20261106-asymmetric_field_visibility`](../accepted/RFC-20261106-asymmetric_field_visibility.md)
  - [`RFC-20261104-interface_data_contracts`](../accepted/RFC-20261104-interface_data_contracts.md)
- **Applies to**: `cmd/lone`, `pkg/lone-analyzer`, `pkg/lone-project`, `docs/scripts`, `dev/SPEC.md`

---

## 1. Summary

This RFC proposes a new official compiler CLI subcommand, **`lone doc-gen json <file.lone>`**, which extracts complete semantic API documentation, symbol signatures, data contracts, and attached `@fa:docs` docstrings into a structured, machine-readable JSON representation.

Key capabilities:
1. **Machine-Readable API Contract**: Emits a deterministic JSON document describing all exported and internal declarations (structs, enums, interfaces, functions, methods, constructors, type aliases, and globals).
2. **Canonical Signature Synthesis**: Generates clean, human-readable Lone source signatures for all symbols (e.g. `pub fn parse_int(s: str) Result<i32>`, `pub type User = struct { ro(pub) name: str }`).
3. **First-Class Docstring Extraction**: Ingests `@fa:docs` compiler directives attached to symbols, preserving paragraph breaks and markdown formatting.
4. **Data Contract Introspection**: Captures structural interface requirements, interface satisfaction witnesses, and labeled trait specializations (`@fa:impl_label`, `Type->label`).
5. **Tooling & Docs Integration**: Serves as the official data feed for automated documentation site generators (such as Lone's VitePress site in `docs/`), Language Server Protocol (LSP) offline indexes, and external IDE extensions.

---

## 2. Motivation

Lone v0.6.0 establishes strong foundations for language inspection via `lone debug <file> [symbol] [--json]`. However, `lone debug` is fundamentally an **internal layout and memory debugger**:
- It inspects WebAssembly linear memory offsets, field alignment padding, struct sizes, and raw tag numbers.
- It does **not** synthesize high-level API signatures, parameter names, docstrings, or cross-referenced interface contracts suitable for developer documentation.

With the acceptance of:
- [`RFC-20261106-symbol_documentation_directive`](../accepted/RFC-20261106-symbol_documentation_directive.md) introducing first-class `@fa:docs <string>` directives,
- [`RFC-20261106-asymmetric_field_visibility`](../accepted/RFC-20261106-asymmetric_field_visibility.md) introducing `ro(pub)` fields,
- [`RFC-20261106-trait_implementation_labels`](../accepted/RFC-20261106-trait_implementation_labels.md) introducing `@fa:impl_label` and `Type->label`, and
- [`RFC-20261106-function_and_method_overloading`](../accepted/RFC-20261106-function_and_method_overloading.md) introducing `@fa:fn_overload`,

there is an immediate need for an official, standardized tool to **extract and export public API documentation** directly from Lone source code.

### 2.1 Use Cases
1. **Automated VitePress Documentation (`docs/`)**:
   Instead of manually writing Markdown pages for standard library modules like `:std:io`, scripts can execute `lone doc-gen json lib/std/io.lone` and automatically generate accurate, up-to-date API reference pages.
2. **IDE & LSP Tooling**:
   Third-party tools, code navigators, and offline doc viewers (e.g. Dash, Zeal) can parse a single standardized JSON schema to index symbol documentation without implementing custom parsers.
3. **CI Contract Verification & Golden Testing**:
   Teams can track API surface changes and prevent accidental breaking changes by diffing `lone doc-gen json` outputs in version control.

---

## 3. CLI Command Specification

### 3.1 Grammar & Invocation
```bash
lone doc-gen <format> <file.lone> [options]
```

- `<format>`: Format selector. In v0.7.0, **`json`** is the mandatory, primary format. The design is explicitly open to future formats (e.g. `markdown`, `html`).
- `<file.lone>`: Path to the root Lone source file or entry point to inspect.

### 3.2 Command Options & Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `-o, --output <path>` | string | stdout | Output file path. If omitted, writes formatted JSON directly to standard output. |
| `--pretty` | boolean | `true` | Indents JSON with 2-space formatting for human readability. |
| `--compact` | boolean | `false` | Emits single-line, minified JSON for minimal file size and fast streaming. |
| `--pub-only` | boolean | `false` | Restricts extraction exclusively to exported (`pub`) declarations, omitting private module internals. |
| `--include-stdlib` | boolean | `false` | Recursively includes documentation for imported standard library symbols referenced by the file. |

### 3.3 Example Invocation
```bash
# Output formatted JSON to stdout
lone doc-gen json ./src/game.lone

# Emit minified JSON to a release artifact
lone doc-gen json ./src/game.lone --pub-only --compact -o ./dist/game.doc.json
```

---

## 4. JSON Schema Specification

The command emits a JSON document complying with the following schema:

```typescript
interface LoneDocJSON {
  schema_version: "1.0.0";
  file: string;                 // Path to analyzed source file
  module: string;               // Module namespace (e.g. "game" or "std:io")
  target_version: string;       // Compiler toolchain version (e.g. "v0.7.0")
  docs: string | null;          // Top-level module documentation
  types: TypeDoc[];             // Structs, enums, interfaces, aliases
  functions: FunctionDoc[];     // Standalone module-level functions
  globals: GlobalDoc[];         // Module-level global constants & variables
}
```

### 4.1 Type Document (`TypeDoc`)

```typescript
interface TypeDoc {
  name: string;                         // Symbol identifier (e.g. "User")
  kind: "struct" | "enum" | "interface" | "alias";
  visibility: "pub" | "private";
  docs: string | null;                  // Extracted from @fa:docs
  signature: string;                    // Full canonical source definition string
  location: SourceLocation;

  // Struct-specific metadata (kind == "struct")
  fields?: FieldDoc[];
  methods?: MethodDoc[];
  constructors?: ConstructorDoc[];
  satisfies?: string[];                 // List of satisfied interface names
  impl_labels?: ImplLabelDoc[];         // Labeled trait implementations

  // Enum-specific metadata (kind == "enum")
  variants?: EnumVariantDoc[];

  // Interface-specific metadata (kind == "interface")
  interface_fields?: FieldDoc[];
  interface_methods?: MethodSignatureDoc[];

  // Alias-specific metadata (kind == "alias")
  aliased_type?: string;
}
```

#### Sub-components of `TypeDoc`:
- **`FieldDoc`**:
  ```typescript
  interface FieldDoc {
    name: string;
    type: string;                       // e.g. "str", "ref<ro i64>"
    visibility: "pub" | "private";
    mutability: "mutable" | "ro" | "ro(pub)";
    docs: string | null;
  }
  ```
- **`EnumVariantDoc`**:
  ```typescript
  interface EnumVariantDoc {
    name: string;                       // e.g. "Add" or "None"
    tag: number;                        // Integer discriminant tag
    payload_type: string | null;        // e.g. "(a: int, b: int)" or null
    docs: string | null;
  }
  ```
- **`ImplLabelDoc`**:
  ```typescript
  interface ImplLabelDoc {
    trait_name: string;                 // e.g. "TrSerialize"
    label: string;                      // e.g. "json"
    specialization_syntax: string;      // e.g. "User->json"
    docs: string | null;
  }
  ```

### 4.2 Function & Method Document (`FunctionDoc`, `MethodDoc`, `ConstructorDoc`)

```typescript
interface FunctionDoc {
  name: string;                         // Function identifier (e.g. "parse_int")
  visibility: "pub" | "private";
  docs: string | null;                  // Extracted from @fa:docs
  signature: string;                    // e.g. "pub fn parse_int(s: str) Result<i32>"
  params: ParamDoc[];
  return_type: string;                  // e.g. "Result<i32>"
  is_overloaded: boolean;               // True if declared within @fa:fn_overload
  overload_siblings?: string[];         // Canonical signatures of sibling overloads
  directives: string[];                 // ["@fa:fn_overload", "@fa:docs", ...]
  location: SourceLocation;
}

interface MethodDoc extends FunctionDoc {
  receiver: {
    type: string;                       // e.g. "ref<User>" or "User"
    is_ref: boolean;
    is_ro: boolean;
  };
}

interface ConstructorDoc extends FunctionDoc {
  is_default: boolean;                  // True if arity is 0 (Default constructor)
}

interface ParamDoc {
  name: string;
  type: string;                         // e.g. "ref<Game>" or "i32"
  is_ref: boolean;
  is_ro: boolean;
}
```

### 4.3 Global Document (`GlobalDoc`)

```typescript
interface GlobalDoc {
  name: string;
  kind: "const" | "let" | "var";
  visibility: "pub" | "private";
  type: string;
  docs: string | null;
  signature: string;                    // e.g. "pub const MAX_RETRIES: i32 = 5"
  location: SourceLocation;
}

interface SourceLocation {
  file: string;
  line_start: number;
  line_end: number;
}
```

---

## 5. Concrete Demonstration

### 5.1 Lone Source Input (`game.lone`)
```lone
@fa:docs "Encapsulates player metrics and session state."
pub type Player = struct {
    ro(pub) name: str
    pub score: i32
    _token: i64

    @fa:fn_overload {
        @fa:docs "Creates a default Player with zero score."
        pub fn __init__(name: str) Player {
            return Player { name: name, score: 0, _token: 0 }
        }

        @fa:docs "Creates a Player with an initial score."
        pub fn __init__(name: str, initial_score: i32) Player {
            return Player { name: name, score: initial_score, _token: 0 }
        }
    }

    @fa:docs "Increments the player score by delta."
    pub fn add_score(self: ref<Player>, delta: i32) void {
        self.score = self.score + delta
    }
}

@fa:docs "Calculates the maximum score between two players."
pub fn max_score(p1: ref<Player>, p2: ref<Player>) i32 {
    if p1.score > p2.score {
        return p1.score
    }
    return p2.score
}
```

### 5.2 Emitted JSON (`lone doc-gen json game.lone --pub-only`)
```json
{
  "schema_version": "1.0.0",
  "file": "game.lone",
  "module": "game",
  "target_version": "v0.7.0",
  "docs": null,
  "types": [
    {
      "name": "Player",
      "kind": "struct",
      "visibility": "pub",
      "docs": "Encapsulates player metrics and session state.",
      "signature": "pub type Player = struct {\n    ro(pub) name: str\n    pub score: i32\n    _token: i64\n}",
      "location": {
        "file": "game.lone",
        "line_start": 2,
        "line_end": 22
      },
      "fields": [
        {
          "name": "name",
          "type": "str",
          "visibility": "pub",
          "mutability": "ro(pub)",
          "docs": null
        },
        {
          "name": "score",
          "type": "i32",
          "visibility": "pub",
          "mutability": "mutable",
          "docs": null
        }
      ],
      "constructors": [
        {
          "name": "__init__",
          "visibility": "pub",
          "docs": "Creates a default Player with zero score.",
          "signature": "pub fn __init__(name: str) Player",
          "params": [
            {
              "name": "name",
              "type": "str",
              "is_ref": false,
              "is_ro": false
            }
          ],
          "return_type": "Player",
          "is_overloaded": true,
          "overload_siblings": [
            "pub fn __init__(name: str) Player",
            "pub fn __init__(name: str, initial_score: i32) Player"
          ],
          "directives": ["@fa:fn_overload", "@fa:docs"],
          "location": {
            "file": "game.lone",
            "line_start": 8,
            "line_end": 10
          }
        },
        {
          "name": "__init__",
          "visibility": "pub",
          "docs": "Creates a Player with an initial score.",
          "signature": "pub fn __init__(name: str, initial_score: i32) Player",
          "params": [
            {
              "name": "name",
              "type": "str",
              "is_ref": false,
              "is_ro": false
            },
            {
              "name": "initial_score",
              "type": "i32",
              "is_ref": false,
              "is_ro": false
            }
          ],
          "return_type": "Player",
          "is_overloaded": true,
          "overload_siblings": [
            "pub fn __init__(name: str) Player",
            "pub fn __init__(name: str, initial_score: i32) Player"
          ],
          "directives": ["@fa:fn_overload", "@fa:docs"],
          "location": {
            "file": "game.lone",
            "line_start": 13,
            "line_end": 15
          }
        }
      ],
      "methods": [
        {
          "name": "add_score",
          "visibility": "pub",
          "docs": "Increments the player score by delta.",
          "signature": "pub fn add_score(self: ref<Player>, delta: i32) void",
          "params": [
            {
              "name": "self",
              "type": "ref<Player>",
              "is_ref": true,
              "is_ro": false
            },
            {
              "name": "delta",
              "type": "i32",
              "is_ref": false,
              "is_ro": false
            }
          ],
          "return_type": "void",
          "is_overloaded": false,
          "directives": ["@fa:docs"],
          "location": {
            "file": "game.lone",
            "line_start": 19,
            "line_end": 21
          }
        }
      ]
    }
  ],
  "functions": [
    {
      "name": "max_score",
      "visibility": "pub",
      "docs": "Calculates the maximum score between two players.",
      "signature": "pub fn max_score(p1: ref<Player>, p2: ref<Player>) i32",
      "params": [
        {
          "name": "p1",
          "type": "ref<Player>",
          "is_ref": true,
          "is_ro": false
        },
        {
          "name": "p2",
          "type": "ref<Player>",
          "is_ref": true,
          "is_ro": false
        }
      ],
      "return_type": "i32",
      "is_overloaded": false,
      "directives": ["@fa:docs"],
      "location": {
        "file": "game.lone",
        "line_start": 25,
        "line_end": 30
      }
    }
  ],
  "globals": []
}
```

---

## 6. Architecture & Implementation Pipeline

The implementation leverages Lone's unified 8-package architecture without introducing any external dependencies:

```
[cmd/lone doc-gen json <file>]
             │
             ▼
  [pkg/lone-project: ResolveAST]
             │
             ▼
  [pkg/lone-analyzer: TypeEvaluator]
             │
             ├── Evaluates types, interfaces, methods, and directives
             └── Attaches concatenated Docs strings to AST/Typed nodes
             │
             ▼
  [pkg/lone-analyzer/docgen: DocExtractor]
             │
             ├── Traverses typed declarations (Structs, Enums, Interfaces, Fns)
             ├── Formats canonical signature strings
             ├── Filters based on --pub-only
             └── Constructs LoneDocJSON DTO
             │
             ▼
  [Standard JSON Marshaller (encoding/json)]
             │
             ▼
  [stdout or -o file]
```

### 6.1 Canonical Signature Synthesizer
The extractor includes a dedicated signature formatting helper that converts AST type annotations and parameter lists into standard Lone code syntax. This guarantees that generated signatures accurately reflect current language grammar without manual string manipulation.

### 6.2 Deterministic Ordering Invariant
To ensure reproducible documentation builds and zero churn in Git diffs:
1. `types` are sorted alphabetically by symbol name.
2. `fields` are listed strictly in declaration order.
3. `methods` and `constructors` are sorted by declaration order.
4. `functions` are sorted alphabetically by function name.
5. `globals` are sorted alphabetically by identifier name.

---

## 7. Downstream Tooling Ecosystem

### 7.1 VitePress Integration (`docs/`)
A Node.js script (e.g. `docs/scripts/gen-api-docs.js`) can execute `lone doc-gen json` across standard library packages (`lib/std/`) and automatically populate Markdown files in `docs/docs/content/latest/`:

```javascript
import { execSync } from 'node:child_process';
import fs from 'node:fs';

const jsonStr = execSync('lone doc-gen json lib/std/io.lone --pub-only').toString();
const docData = JSON.parse(jsonStr);

// Convert docData.functions and docData.types into VitePress Markdown tables and codeblocks!
```

### 7.2 Language Server Integration
`pkg/lone-lsp` can cache the output of `lone doc-gen json` during project workspace indexing, enabling rapid full-text symbol searches across thousands of files.

---

## 8. Invariants & Non-Goals

1. **Zero Runtime Impact**:
   `lone doc-gen` is strictly a compile-time static analysis tool. It produces no `.wat` or `.wasm` artifacts and introduces 0 bytes of overhead to binary distribution.
2. **Deterministic Output**:
   Running `lone doc-gen json` on the same source file must produce byte-for-byte identical output across platforms (Linux, macOS, Windows).
3. **Non-Goal: Presentation Rendering in Compiler**:
   The compiler will **not** render HTML, CSS, or complex web layouts. The compiler's responsibility ends at providing an authoritative, structured JSON data feed. Visual presentation is delegated to VitePress, Dash, or custom web frontends.
4. **Error Handling**:
   If the target source file has syntax or semantic type errors, `lone doc-gen` reports the standard compiler diagnostics to `stderr` and exits with status code 1 without emitting corrupted JSON.
