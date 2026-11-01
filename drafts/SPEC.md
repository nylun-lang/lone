# Lone Language Specification

This specification documents the current implementation of the **Lone Programming Language** based on the active parser grammar and intermediate representation (`irtypes`) in `packages/lone-lang`.

---

## 1. Introduction & Philosophy

**Lone** is an explicit, systems- and UI-oriented programming language designed to compile to **WebAssembly (Wasm)** as its primary target. It takes inspiration from Go, TypeScript, Python, and Rust/Zig.

### Core Philosophy
> *"Easy to read, Easy to write, and Hard to misunderstand."*

### Key Invariants
- **IL1 - Explicit over implicit**: Every name reference must be unambiguous regarding *what it is* and *where it came from*.
- **IL2 - No implicit built-ins**: Built-in symbols, functions, or intrinsics are prefixed with `#` (e.g. `#init`, `#print`) to cleanly distinguish them from user code.
- **IF1 - Syntax-level feedback**: Grammar layout and keywords provide readability without resorting to confusing punctuation.
- **IF2 - Progressive information**: Information displays progressively from left to right wherever possible.
- **IUI1 - Declared topology is stable**: UI declarations establish a persistent topology known to the compiler.

---

## 2. Lexical Structure

### 2.1 Identifiers
Identifiers name variables, functions, types, and labels.
- **Pattern**: `[a-zA-Z_][a-zA-Z0-9_]*`
- **Intrinsics / Built-in Names**: May optionally be prefixed with `#` (e.g. `#init`).
- **Naming Conventions**:
  - Functions & Local Variables: `snake_case` (e.g. `calculate_hash`, `user_id`)
  - Globals & Constants: `ALL_CAPS` (e.g. `MAX_BUFFER_SIZE`)
  - Types & Structs: `CamelCase` (e.g. `UserData`, `TokenStream`)

### 2.2 Keywords
The following keywords are reserved and guarded by word boundaries (`![a-zA-Z0-9_]`):
```
var       type      fn        return    break
if        else      while     and       or
true      false     none      ro        comptime
```

### 2.3 Comments & Directives
- **Single-line comments**: Begin with `--` and extend to end-of-line:
  ```lone
  -- This is a single line comment
  ```
- **Multi-line comments**: Enclosed in `--- ... ---`:
  ```lone
  ---
  This is a multiline
  comment block
  ---
  ```
- **Compiler Directives**: Comments with the `fa:` namespace prefix (e.g. `Falone` compiler directives):
  ```lone
  --fa:doc Single-line doc directive
  ---fa:doc
  Multiline documentation directive
  ---
  ```

---

## 3. Literals & Primary Values

### 3.1 Primitive Literals
- **Booleans**: `true`, `false`
- **None**: `none` (absence of a value)
- **Integers**: Decimal integers with optional digit-separating underscores:
  ```lone
  0
  42
  1_000_000
  ```
- **Floating-point Numbers**: Decimal numbers containing a fractional dot:
  ```lone
  3.14
  10.5
  10.787_887
  ```
- **Strings**: Double-quoted strings supporting escape sequences (`\n`, `\r`, `\t`, `\"`, `\\`):
  ```lone
  "Hello, World!\n"
  ```

### 3.2 Object Literals
Object literals instantiate records or key-value fields using dot-prefixed field names:
```lone
var data = { .name: "Alice", .role: "Admin" }
```

### 3.3 List Literals
List literals express a sequence of values enclosed in braces `{ ... }`:
```lone
var numbers = { 1, 2, 3, 4, 5 }
var matrix = { { 1, 0 }, { 0, 1 } }
```

---

## 4. Type System (`TYPE_REF`)

Type references represent named, parameterized, array, or function signatures, accompanied by orthogonal modifiers.

### 4.1 Type Modifiers & Constraints (`TypeConstrain`)
- **`ro`**: Read-only access constraint.
- **`?`**: Optional / nullable constraint (value may be `none`).
- **`comptime`**: Compile-time evaluated type constraint.

Modifiers compose left-to-right before the type:
```lone
var a: ?int = none
var b: comptime ro string = "constant"
var c: comptime ro ?[]int = none
```

### 4.2 Type Classifications

#### 4.2.1 Base Types (`TypeBase`)
Named types with optional generic type arguments:
```lone
var x: int
var y: string
var items: list<string>
var cache: Map<string, int>
```

#### 4.2.2 List & Array Types (`TypeList`)
- **Dynamic Slice (`[]T`)**: Contiguous dynamic list of items:
  ```lone
  var numbers: []int
  var grid: [][]float
  var opt_items: ?[]int
  ```
- **Fixed-Size Array (`[size]T`)**: Fixed-length array where `size` is an integer literal or constant expression:
  ```lone
  var buffer: [1024]byte
  var fixed_grid: [9][10]int
  var named_size: [BUFFER_CAPACITY]string
  ```

#### 4.2.3 Function Signatures (`TypeFn`)
First-class function types with named or anonymous parameters and an optional return type:
```lone
var callback: fn (event: Event) bool
var runner: fn ()
var compute: fn (a: int, b: int) int
var higher_order: fn (cb: fn (x: int) bool) []string
```

---

## 5. Declarations (`TOP_LEVEL`)

### 5.1 Variable Declarations (`BIND_DEC`)
Variables are introduced with `var`, with optional explicit type annotations and initializers:
```lone
var count: int = 0
var title = "Lone"          -- Type inferred as 'infer'
var uninit: ?string         -- Uninitialized, defaults to LITERAL_UNINITIALIZED
```

### 5.2 Function Declarations (`FN_DEC`)
Top-level functions are introduced with `fn`:
```lone
fn add(a: int, b: int) int {
    return a + b
}

fn noop() {
    -- Void return type (nil)
}

fn #init(name: string) Self {
    -- Constructor intrinsic with # prefix
}
```

Parameters are typed (`param_name: TYPE_REF`), separated by commas, and allow optional trailing commas:
```lone
fn configure(host: string, port: int, secure: ?bool,) bool {
    return true
}
```

---

## 6. Statements & Control Flow (`STMT`)

Statements reside inside function bodies, labeled blocks, and control branches.

### 6.1 Return Statement (`RETURN_STMT`)
Exits the enclosing function, optionally returning an expression:
```lone
return
return x + 1
```

### 6.2 Break Statement (`BREAK_STMT`)
Breaks out of an enclosing loop or labeled block. Supports explicit labeled breaks and value yielding:
```lone
break                      -- Bare break
break 42                   -- Break with value
break loop:                -- Break to label without value
break blk: x * 2           -- Break to label yielding a value
```

### 6.3 If Statement (`IF_STMT`)
Conditional execution with optional `else if` chaining and `else` branches:
```lone
if x > 0 {
    return 1
} else if x == 0 {
    return 0
} else {
    return -1
}
```

### 6.4 While Loop (`WHILE_STMT`)
Repeats a block of statements while a condition is truthy:
```lone
while low <= high {
    var mid = low + (high - low) / 2
    if target == mid {
        break
    }
}
```

---

## 7. Expressions & Operators (`EXPR`)

### 7.1 Labeled Expressions (`LABEL_DEC`)
Any block or control structure can be prefixed with a label (`name:`). The block becomes an expression whose evaluated value is supplied by a matching `break name: expr`:
```lone
var result = blk: {
    var a = 10
    var b = 20
    break blk: a + b
}

var first = loop: while true {
    if found {
        break loop: current
    }
}
```

### 7.2 Postfix Expressions (`POSTFIX_EXPR`)
Postfix operators chain onto any primary expression from left to right:

| Suffix Syntax | Meaning | Example |
| :--- | :--- | :--- |
| `.name` | Member property access | `user.name` |
| `:name` | Namespace / module symbol access | `math:sin` |
| `::name` | Associated function / type access | `Ordering::Equal` |
| `(args...)` | Function / method call | `calc(1, 2)` |
| `::(args...)` | Constructor / type init call | `Color::(255, 0, 0)` |
| `::{ ... }` | Type-make construct (object/list) | `User::{ .name: "Alice" }` |

Chaining example:
```lone
var val = builder.set_name("Lone").build().version
```

### 7.3 Unary Operators (`UNARY_EXPR`)
Evaluated right-to-left before binary expressions:
- `-` : Negation (arithmetic minus)
- `+` : Unary positive
- `!` : Logical NOT
- `&` : Reference / address operator
- `*` : Dereference operator

### 7.4 Binary Operators & Precedence
From highest precedence (evaluated first) to lowest:

| Level | Operator Category | Operators | Associativity |
| :---: | :--- | :--- | :---: |
| 1 | Multiplicative | `*`, `/` | Left-to-right |
| 2 | Additive | `+`, `-` | Left-to-right |
| 3 | Relational Comparison | `>=`, `<=`, `>`, `<` | Left-to-right |
| 4 | Equality Comparison | `==`, `!=` | Left-to-right |
| 5 | Logical AND | `and` | Left-to-right |
| 6 | Logical OR | `or` | Left-to-right |

---

## 8. AST / Intermediate Representation Reference

| Source Syntax | IR Node (`packages/lone-lang/irtypes`) |
| :--- | :--- |
| `var x: T = val` | `ir.BIND_DEC` |
| `fn name(args) T { ... }` | `ir.FN_DEC` |
| `name: { ... }` | `ir.LabelDec` (Kind `EXPR_LABEL_DEC`) |
| `break name: expr` | `ir.BreakStmt` (Kind `EXPR_BREAK`) |
| `if cond { } else { }` | `ir.IfStmt` (Kind `EXPR_IF`) |
| `while cond { }` | `ir.WhileStmt` (Kind `EXPR_WHILE`) |
| `return expr` | `ir.ReturnStmt` |
| `{ .field: expr }` | `ir.ObjectLiteral` (Kind `EXPR_OBJ_LITERAL`) |
| `{ 1, 2, 3 }` | `ir.ListLiteral` (Kind `EXPR_LIST_LITERAL`) |
| `base.member`, `base(...)`, `base::(...)` | `ir.PostFixExpr` with `ir.Suffix` slice |
| `a + b`, `a * b`, `a <= b` | `ir.BinaryExpr` (Kind `EXPR_BINARY_OP`) |
| `-x`, `!x`, `&x` | `ir.UnaryExpr` (Kind `EXPR_UNARY_OP`) |
| `int`, `Map<K, V>` | `ir.TYPE_REF` (Kind `TYPE_BASE`, `ir.TypeBase`) |
| `[]T`, `[size]T` | `ir.TYPE_REF` (Kind `TYPE_LIST`, `ir.TypeList`) |
| `fn (T) T` | `ir.TYPE_REF` (Kind `TYPE_FN`, `ir.TypeFn`) |
