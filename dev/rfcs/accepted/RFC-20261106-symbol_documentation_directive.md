# RFC-20261106: Symbol Documentation via `@fa:docs <string>`

- **RFC**: `RFC-20261106-symbol_documentation_directive`
- **Title**: Symbol Documentation via `@fa:docs <string>`
- **Author**: Human & AI Pair
- **Status**: Accepted
- **Created**: 2026-11-06
- **Target Version**: v0.7.0
- **Related RFCs**:
  - [`RFC-20261104-interface_data_contracts`](./RFC-20261104-interface_data_contracts.md)
  - [`RFC-20261104-struct_initialization_and_field_modifiers`](./RFC-20261104-struct_initialization_and_field_modifiers.md)
  - [`RFC-20261106-function_and_method_overloading`](../drafts/RFC-20261106-function_and_method_overloading.md)
- **Applies to**: `pkg/lone-lexer`, `pkg/lone-parser-ast`, `pkg/lone-parser`, `pkg/lone-analyzer`, `pkg/lone-lsp`, `cmd/lone`, `dev/SPEC.md`

---

## 1. Summary

This RFC introduces first-class symbol documentation in Lone via the `@fa:docs <string>` compiler directive.

Key capabilities:
1. **Explicit Symbol Attachment**: Documentation is declared as a first-class compiler directive bound directly to the subsequent declaration, avoiding fragile comment-parsing heuristics.
2. **Permitted Declarations**: Exclusively permitted on:
   - **`type` declarations**: `struct`, `enum`, `interface`, and type aliases.
   - **`global` declarations**: Top-level module variables (`let` and `var`).
   - **`fn` declarations**: Standalone module functions, inherent struct methods, and `__init__` constructors.
3. **IDE & LSP Integration**: The documentation string is preserved in the Abstract Syntax Tree (AST) and Typed IR, enabling instant rich Markdown hover tooltips (`textDocument/hover`) and symbol outlines in the Language Server (`pkg/lone-lsp`).
4. **Toolchain Inspection**: Exposed via `lone debug <file> [symbol] [--json]` and automated documentation site generators.
5. **Zero WebAssembly Overhead**: Does not emit instructions or linear memory allocations into compiled `.wat` / `.wasm` output.

---

## 2. Motivation

In modern programming environments, readable, maintainable, and self-documenting code is a necessity. Developers rely heavily on Language Server Protocol (LSP) tooltips, signature help, and automated documentation platforms.

### 2.1 The Shortcomings of Comment-Based Documentation
Most conventional languages use "doc comments" (e.g. `///` in Rust, `/**` in Java/TypeScript, or `#` docstrings in Python):
- **Lexical Fragility**: In typical compiler architectures, comments are discarded during lexical scanning. Associating detached comments with AST nodes requires fragile whitespace tracking, line-number heuristics, or trailing comment lookahead.
- **Accidental Detachment**: A single blank line or intermediate pragma can inadvertently detach a doc comment from its target symbol.
- **Non-Standardization**: Comment formats frequently vary between tools (e.g. Doxygen vs JSDoc vs Sphinx).

### 2.2 Lone's Philosophy: Explicit over Implicit (**IL1**)
Lone rejects magical hidden comments in favor of explicit compiler constructs:
- Earlier in the language design, directives transitioned from "magic comments" (`--fa:wasm`) to first-class syntax (`@fa:wasm`).
- Attaching documentation via `@fa:docs <string>` makes symbol documentation a **first-class semantic construct**:
  - The compiler parses and validates the directive as part of the formal grammar.
  - The AST stores documentation directly on declaration nodes.
  - Invalid usage (e.g. attaching documentation to a statement inside a loop) produces immediate, clear compiler errors.

---

## 3. Detailed Specification

### 3.1 Syntax & Placement

The directive `@fa:docs` accepts a string literal argument and precedes the target declaration:

$$\mathbf{@fa:docs}\;\; \text{<string>}\;\; \mathbf{[\textbackslash n]}\;\; \text{<Declaration>}$$

```lone
@fa:docs "Calculates the Euclidean distance between two points in 2D space."
pub fn distance(p1: Point, p2: Point) f64 {
    let dx = p1.x - p2.x
    let dy = p1.y - p2.y
    return (dx * dx + dy * dy) as f64
}
```

### 3.2 Permitted Declarations

`@fa:docs` is strictly permitted on exactly three declaration categories:

#### 1. Type Declarations (`type`)
Applicable to `struct`, `enum`, `interface`, and type aliases:

```lone
@fa:docs "Encapsulates player credentials and active game metrics."
pub type UserSession = struct {
    _token: str,
    ro(pub) user_id: i32,
    pub attempts: i32,
}

@fa:docs "Discriminant error variants for fallible I/O operations."
pub type IOError = enum {
    NotFound,
    PermissionDenied,
    UnexpectedEOF,
}

@fa:docs "Contract for types that can serialize their internal state to JSON."
pub type IfcJsonSerializable = interface {
    ro id: i32
    ro json_payload: str
}

@fa:docs "Type alias representing a millisecond timestamp duration."
pub type Milliseconds = i64
```

#### 2. Global Declarations (`let` / `var` at Module Level)
Applicable to top-level module constants and global variables:

```lone
@fa:docs "Global maximum connection timeout in milliseconds."
pub let MAX_CONNECTION_TIMEOUT: i32 = 5000

@fa:docs "Tracks the total number of active concurrent worker tasks."
var global_active_workers: i32 = 0
```

#### 3. Function & Method Declarations (`fn`)
Applicable to standalone functions, inherent struct methods, and `__init__` constructors:

```lone
@fa:docs "Initializes a new Game session with default limits."
pub fn __init__(max_val: i32) Game {
    return Game { max_val: max_val, attempts: 0 }
}

@fa:docs "Increments the session attempt counter by one."
pub fn record_attempt(self: ref<var Game>) void {
    self.attempts = self.attempts + 1
}
```

---

## 4. Multi-Line Documentation & Formatting

Because documentation frequently requires detailed paragraphs, argument lists, and Markdown examples, `@fa:docs` supports multi-line strings:

### 4.1 Standard Escaped Multi-Line Strings
```lone
@fa:docs "Generates a cryptographically secure random integer.\n\n# Parameters\n- `min`: Inclusive lower bound.\n- `max`: Inclusive upper bound.\n\n# Return\nA pseudo-random 32-bit integer uniformly distributed in `[min, max]`."
pub fn random_range(min: i32, max: i32) i32 { ... }
```

### 4.2 Consecutive `@fa:docs` Directives (Block Concatenation)
To prevent massive single-line string escapes, multiple consecutive `@fa:docs` directives immediately preceding a declaration are automatically concatenated with newline separation (`\n`):

```lone
@fa:docs "Parses an integer from a UTF-8 string buffer."
@fa:docs ""
@fa:docs "Returns `Result::Ok(val)` on success, or `Result::Err(Error::Generic)`"
@fa:docs "if non-numeric characters are encountered or overflow occurs."
pub fn parse_int(s: str) Result<i32> { ... }
```

When evaluated, the AST merges the string literals into a single cohesive Markdown documentation block.

---

## 5. Compiler & Toolchain Architecture Integration

### 5.1 Abstract Syntax Tree (`pkg/lone-parser-ast/dec.go`)
Declaration AST nodes are updated to include a `Docs` field:

```go
type FN_DEC struct {
    Name       string
    Docs       string // Documentation from @fa:docs, if present
    Params     []PARAM
    ReturnType TYPE_REF
    Body       []STMT
    Span_      Span
}

type TYPE_DEC struct {
    Name  string
    Docs  string // Documentation from @fa:docs, if present
    Type  TYPE_DEF
    Span_ Span
}

type BIND_DEC struct {
    IsMutable bool
    Docs      string // Documentation from @fa:docs (globals only)
    Bindings  []BIND_TARGET
    Type      TYPE_REF
    Value     EXPR
    Span_     Span
}
```

### 5.2 Parser Grammar Lowering (`pkg/lone-parser/parser.go`)
During recursive descent parsing:
1. When `@fa:docs <string>` is encountered, the parser records the string literal and expects the immediate next declaration.
2. If the next node is `fn`, `type`, or a module-level `let`/`var`, the `Docs` field is assigned.
3. If any other token is encountered, the parser reports a compile error (`syn/invalid-doc-target`).

### 5.3 Semantic Analyzer (`pkg/lone-analyzer/eval_type.go`)
The Type Evaluator propagates documentation into symbol representations:
- `StructType.Docs string`
- `EnumType.Docs string`
- `InterfaceType.Docs string`
- `FunctionType.Docs string`
- `TVarDec.Docs string`

### 5.4 Language Server Protocol (`pkg/lone-lsp/server.go`)
When a client queries `textDocument/hover`:
1. The LSP server resolves the symbol at the requested `Position`.
2. The server formats the hover response combining the signature and docs:
   ```markdown
   ```lone
   fn parse_int(s: str) Result<i32>
   ```
   Parses an integer from a UTF-8 string buffer.
   
   Returns `Result::Ok(val)` on success, or `Result::Err(Error::Generic)`
   if non-numeric characters are encountered.
   ```
3. In autocompletion lists (`textDocument/completion`), the first sentence of `@fa:docs` is automatically surfaced in the completion detail pane.

### 5.5 CLI Debug Tooling (`cmd/lone debug`)
`lone debug <file> [symbol] --json` exports documentation attributes directly in its JSON DTO:

```json
{
  "structs": {
    "UserSession": {
      "name": "UserSession",
      "docs": "Encapsulates player credentials and active game metrics.",
      "size": 16,
      "align": 4,
      "fields": [ ... ]
    }
  }
}
```

---

## 6. Validation Rules & Static Diagnostics

The compiler enforces strict placement and argument constraints:

### Rule 1: Restricted Declaration Targets
`@fa:docs` is forbidden on local function statements, control flow blocks, loops, or struct fields directly:

```lone
fn compute() void {
    @fa:docs "Temporary local counter" -- COMPILE ERROR!
    var local_count = 0
}
```

**Diagnostic**:
```text
error[syn/invalid-doc-target]: @fa:docs directive is only permitted on type, global variable, and function declarations
  --> src/game.lone:42:5
   |
42 |     @fa:docs "Temporary local counter"
   |     ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ cannot document local variable in statement position
```

### Rule 2: String Literal Argument Required
The argument to `@fa:docs` must be an explicit string literal:

```lone
@fa:docs 12345 -- COMPILE ERROR!
pub fn bad() void {}
```

**Diagnostic**:
```text
error[syn/invalid-doc-argument]: @fa:docs requires a string literal argument, got integer literal
  --> src/game.lone:10:10
   |
10 | @fa:docs 12345
   |          ^^^^^ expected string literal (e.g. @fa:docs "...")
```

### Rule 3: Dangling Directive Prohibited
An `@fa:docs` directive at the end of a file or without a following declaration triggers a syntax error:

```lone
@fa:docs "Dangling doc at end of file"
-- EOF
```

**Diagnostic**:
```text
error[syn/dangling-doc-directive]: @fa:docs directive must precede a type, global, or function declaration
  --> src/game.lone:50:1
   |
50 | @fa:docs "Dangling doc at end of file"
   | ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ expected declaration after @fa:docs
```

---

## 7. WebAssembly Emission & Zero-Cost Guarantee

In accordance with **IL1** and Lone's zero-overhead mission:
- `@fa:docs` produces **zero bytes** of WebAssembly instructions (`WasmInst`).
- `@fa:docs` occupies **zero bytes** in Page 0 (Datastack) or Page 1+ (Heap).
- Strings supplied to `@fa:docs` are **not** emitted into WebAssembly static data segments (`(data ...)`).
- The compiled `.wat` / `.wasm` binary size is completely unaffected by the presence or length of documentation directives.

*(Optional Future Optimization: A compiler flag `--emit-debug-docs` can store documentation in a standard WebAssembly custom section `(custom "lone.docs" ...)` for third-party debuggers).*

---

## 8. Comparison with Other Languages

| Language | Documentation Syntax | Grammar Level | Detachment Risk | Runtime Overhead |
| :--- | :--- | :---: | :---: | :---: |
| **Rust** | `/// doc` / `#[doc = "..."]` | Attribute / Comment | Low | Zero |
| **Go** | `// Comment above symbol` | Comment heuristic | High (blank line detaches) | Zero |
| **Python** | `"""docstring"""` | Executable AST object | None | Runtime `__doc__` heap object |
| **TypeScript** | `/** JSDoc */` | Comment heuristic | High | Zero |
| **Lone** | `@fa:docs "..."` | **First-class Directive** | **Zero (Formal Syntax Binding)**| **Zero (0 bytes in Wasm)** |

---

## 9. Alternatives Considered & Rejected

1. **Adopting `///` Doc Comments**:
   - Rejected. Requires maintaining comment buffers in the lexer and parser, with tricky heuristics for blank lines, multi-line comments, and whitespace normalization. Directives are consistent with `@fa:wasm` and `@fa:fn_overload`.
2. **Permitting `@fa:docs` on Struct Fields Directly**:
   - Deferred. Documenting individual fields (`@fa:docs "x coordinate" x: i32`) clutters compact struct definitions. Instead, field descriptions can be placed in the parent struct's `@fa:docs` markdown table, or introduced in a future ergonomics pass.
3. **Allowing `@fa:docs` on Local Variables**:
   - Rejected. Local variables within function bodies do not form public API contracts and do not require formal documentation nodes.
