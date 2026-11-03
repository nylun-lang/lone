# Lone Language Specification

This specification documents the current implementation of the **Lone Programming Language** based on the active compiler toolchain, parser grammar (`pkg/lone-parser`), abstract syntax tree (`pkg/lone-parser-ast`), semantic analyzer (`pkg/lone-analyzer`), and WebAssembly code generator (`pkg/lone-compiler`).

---

## 1. Introduction & Philosophy

**Lone** is an explicit, systems- and UI-oriented programming language designed to compile directly to **WebAssembly (Wasm)** with zero runtime overhead as its primary target. It takes inspiration from Go, TypeScript, Python, and Rust/Zig.

### Core Philosophy
> *"Easy to read, Easy to write, and Hard to misunderstand."*

### Key Invariants
- **IL1 - Explicit over implicit**: Every name reference, type indirection, and memory allocation must be unambiguous regarding *what it is* and *where it came from*.
- **IL2 - No implicit built-ins**: Compiler intrinsics are prefixed with `#` (e.g. `#panic`, `#print`) to cleanly distinguish them from user code.
- **IL3 - Strict Resource & Result Accounting**: Non-void expressions in statement position must be consumed or explicitly dropped (`drop <expr>`). Unconsumed values trigger a static analysis error (`type/unused-result`).
- **IF1 - Syntax-level feedback**: Grammar layout and keywords provide readability without resorting to confusing punctuation.
- **IF2 - Progressive information**: Information displays progressively from left to right wherever possible.
- **IUI1 - Declared topology is stable**: UI declarations establish a persistent topology known to the compiler.

---

## 2. Lexical Structure

### 2.1 Identifiers
Identifiers name variables, functions, types, fields, parameters, and labels.
- **Pattern**: `[a-zA-Z_][a-zA-Z0-9_]*`
- **Intrinsics / Built-in Names**: May optionally be prefixed with `#` (e.g. `#panic`, `#print`).
- **Naming Conventions**:
  - **Functions & Local Variables**: `snake_case` (e.g. `calculate_hash`, `user_id`).
  - **Globals & Constants**: `ALL_CAPS` or `snake_case` (e.g. `MAX_BUFFER_SIZE`, `default_timeout`).
  - **Types & Structs**: `CamelCase` / PascalCase (e.g. `UserData`, `TokenStream`).
  - **Trait Types**: **Must** end with the exact suffix `Trait` (e.g. `ComparableTrait`, `WriterTrait`, `ReaderTrait`).
  - **Non-Trait Identifiers**: Any type or symbol that is **not** a trait must **not** end with `trait` (case-insensitive, e.g. `_trait`, `_TRAIT`, `Trait`) or be named `Trait`/`trait`. Legitimate English words ending in "trait" (such as `portrait`, `strait`, `distrait`) are preserved.

### 2.2 Keywords
The following keywords are reserved and guarded by word boundaries (`![a-zA-Z0-9_]`):
```
import    from      as        pub       var       let
type      struct    trait     impl      fn        return
break     if        else      while     loop      drop
and       or        true      false     none      ro
comptime
```

### 2.3 Comments
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

### 2.4 Compiler Directives (`@fa:name`)
Compiler directives are first-class language constructs prefixed with `@` and namespaced under `fa:` (Falone compiler engine):

- **Block Directive (`@fa:name { ... }`)**: Encloses structured declarations, imports, or compiler configurations:
  ```lone
  @fa:wasm {
      import .fd_write, .fd_read from :wasi:snapshot_preview1
  }
  ```
- **Statement Directive (`@fa:name <stmt>`)**: Directs an individual statement or import:
  ```lone
  @fa:wasm import .random_get from :wasi:snapshot_preview1
  ```
- **Inline / Capability Directive (`@fa:name`)**: Marks file-level capability authorization:
  ```lone
  @fa:wasm
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

Type references represent primitive, nominal, parameterized, array, or function signatures, accompanied by orthogonal modifiers.

### 4.1 Primitive Types
- **Signed Integers**: `i8`, `i16`, `i32`, `i64`, `i128`, `int` (alias for `i32`)
- **Unsigned Integers**: `u8`, `u16`, `u32`, `u64`, `u128`, `uint` (alias for `u32`)
- **Floating-point**: `f32`, `f64`, `float` (alias for `f32`)
- **Other Primitives**: `bool`, `str`, `string`, `char`, `void`, `none`, `any`

### 4.2 Type Modifiers & Constraints (`TypeConstrain`)
- **`ro`**: Read-only access constraint.
- **`?`**: Optional / nullable constraint (value may be `none`).
- **`comptime`**: Compile-time evaluated type constraint.

Modifiers compose left-to-right before the type:
```lone
var a: ?i32 = none
var b: comptime ro str = "constant"
var c: comptime ro ?[]i32 = none
```

### 4.3 Reference & Pointer Types: `ref<T>` vs. `ptr<T>`

In accordance with **IL1 (Explicit over implicit)**, Lone makes a strict distinction between managed references and unmanaged raw memory addresses:

| Type | Name | Semantics | Creation Syntax | Target Memory |
| :---: | :--- | :--- | :---: | :--- |
| `ref<T>` | **Managed Reference** | Safe reference tracked by the escape analyzer and memory model. | `&x` | Datastack frame or promoted to Managed Store. |
| `ptr<T>` | **Raw Memory Address** | Unsafe raw byte offset in WebAssembly linear memory (`i32`). Zero runtime overhead. | `&raw x` | WebAssembly linear memory (datastack / shadow stack). |

```lone
var val: i32 = 42

-- Managed reference to val
var r: ref<i32> = &val

-- Raw linear memory address (i32 byte offset)
var p: ptr<i32> = &raw val
```

### 4.4 List & Array Types (`TypeList`)
- **Dynamic Slice (`[]T`)**: Contiguous dynamic list of items:
  ```lone
  var numbers: []i32
  var grid: [][]f32
  var opt_items: ?[]i32
  ```
- **Fixed-Size Array (`[size]T`)**: Fixed-length array where `size` is an integer literal or constant expression:
  ```lone
  var buffer: [1024]u8
  var fixed_grid: [9][10]i32
  ```

### 4.5 Function Signatures (`TypeFn`)
First-class function types with named or anonymous parameters and an optional return type:
```lone
var callback: fn (event: Event) bool
var runner: fn () void
var compute: fn (a: i32, b: i32) i32
var higher_order: fn (cb: fn (x: i32) bool) []str
```

---

## 5. Declarations (`TOP_LEVEL`)

### 5.1 Import Declarations (`ImportDecl`)
Imports bring symbols into the current file scope from namespaces or relative files. Lone uses flat, curly-brace-free import syntax:

```lone
-- Import specific symbols from standard library namespaces
import .print, .println, .Stdout, .WriterTrait from :std:io
import .random_i32, .random_range from :std:sys

-- Import from a relative file in the same directory
import .helper_fn from .helpers

-- Import and alias an entire namespace
import :std:io as io
```

Imported symbols can be used directly (`print("hello")`) or accessed through namespace aliases (`io.println("hello")`).

### 5.2 Visibility (`pub`)
Declarations are private to their declaring file or module by default. The `pub` keyword exports them:
```lone
pub var DEFAULT_BUFFER_SIZE = 4096

pub fn helper() void {
    -- exported function
}

pub type WriterTrait = trait { ... }
pub type Stdout = struct { ... }
```

### 5.3 Variable Declarations (`BIND_DEC`)
Variables are introduced with `var` or `let`, with optional explicit type annotations and initializers:
```lone
var count: i32 = 0
let title = "Lone"          -- Type inferred as 'infer' -> lowered to concrete 'str'
var uninit: ?str            -- Uninitialized, defaults to none
```

### 5.4 Function Declarations (`FN_DEC`)
Top-level functions are introduced with `fn`:
```lone
fn add(a: i32, b: i32) i32 {
    return a + b
}

fn noop() void {
    -- Void return type
}

fn #panic(msg: str) void {
    -- Intrinsic function with # prefix
}
```

### 5.5 Struct Declarations (`TYPE_DEC_STRUCT`)
Structs encapsulate data fields, associated constructors, and instance methods:
```lone
pub type Vec2 = struct {
    x: i32,
    y: i32,

    -- Associated Constructor
    pub fn new(x: i32, y: i32) Vec2 {
        return Vec2::{ .x: x, .y: y }
    }

    -- Instance Method (receiver 'self')
    pub fn dot(self, other: Vec2) i32 {
        return self.x * other.x + self.y * other.y
    }
}
```

#### Struct Instantiation
1. **Bare Struct Construct (`T::{ ... }`)**: Directly initializes fields without invoking a constructor:
   ```lone
   var v = Vec2::{ .x: 10, .y: 20 }
   ```
2. **Associated Method Call (`T.new(...)`)**: Invokes an associated factory constructor:
   ```lone
   var v = Vec2.new(10, 20)
   ```

### 5.6 Trait Declarations (`TYPE_DEC_TRAIT`)
Traits define behavioral contracts that types can satisfy.
- **Convention Rule**: Trait type names **must** end with the `Trait` suffix (e.g. `WriterTrait`, `ReaderTrait`, `ComparableTrait`).

```lone
pub type WriterTrait = trait {
    fn write(self, msg: str) i32
}

pub type ReaderTrait = trait {
    fn read(self, buf: ptr<u8>, max_len: i32) i32
}
```

### 5.7 Implementation Declarations (`IMPL_DEC`)
Implementations attach trait methods to concrete types:
```lone
impl WriterTrait on Stdout {
    fn write(self, msg: str) i32 {
        return sys_write(self.fd, &raw msg[0], str_len(msg))
    }
}

impl ReaderTrait on Stdin {
    fn read(self, buf: ptr<u8>, max_len: i32) i32 {
        return sys_read(self.fd, buf, max_len)
    }
}
```

---

## 6. Statements & Control Flow (`STMT`)

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

#### 6.3.1 Packed If (`PACKED_IF_STMT`)
Multi-branch conditional block grouping conditions and actions using `=>` fat arrows, eliminating verbose `else if` ladders:
```lone
if {
    x > 100 => print("Large")
    x > 50  => print("Medium")
    x > 0   => print("Small")
    else    => {
        print("Invalid")
    }
}
```
- **Arms**: One or more condition arms (`<cond> => <stmt|block>`) followed by an optional fallback `else => <stmt|block>`.
- **Delimiters**: Arms can be separated by newlines, optional commas (`,`), or optional semicolons (`;`).
- **Desugaring**: Desugars 1:1 in the AST into nested `ir.IfStmt` nodes, incurring zero additional runtime or MIR/WIR overhead.
- **Statement context**: `else =>` is optional (falls through if no condition matches).
- **Expression context**: `else =>` is required for exhaustiveness; blocks yield values via `break <expr>`.

### 6.4 While Loop (`WHILE_STMT`)
Repeats a block of statements while a condition is truthy:
```lone
while low <= high {
    var mid = low + (high - low) / 2
    if target == mid {
        break
    }
    low = low + 1
}
```

#### 6.4.1 Infinite Loop (`LOOP_STMT`)
Runs an infinite loop until terminated by `break` or `return`:
```lone
loop {
    var line = read_line()
    if line == "quit" {
        break
    }
    println(line)
}
```
- Can also be labeled for targeted breaks: `outer: loop { inner: loop { break outer } }`.
- Desugars cleanly into an `ir.WhileStmt` with constant `true` condition.

### 6.5 Drop Statement (`DROP_STMT`)
Evaluates an expression and explicitly discards its result:
```lone
drop compute_expensive_value()
drop out.write("hello")
```
In Lone, any statement evaluating to a non-void value must be consumed (e.g. bound via `var`/`let`, returned, or passed to a function) or discarded with `drop`. Ignoring return values triggers a static analysis error (`type/unused-result`).

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
```

### 7.2 Postfix Expressions (`POSTFIX_EXPR`)
Postfix operators chain onto any primary expression from left to right:

| Suffix Syntax | Meaning | Example |
| :--- | :--- | :--- |
| `.name` | Member property access | `user.name` |
| `:name` | Namespace / module symbol access | `math:sin` |
| `::name` | Associated function / type access | `Vec2::new` |
| `(args...)` | Function / method call | `calc(1, 2)` |
| `::{ ... }` | Type-make construct (struct/record) | `Vec2::{ .x: 1, .y: 2 }` |
| `::|Type|` | **Trait Materialization**: Binds a trait contract to a concrete type | `WriterTrait::|Stdout|` |

#### Trait Materialization (`::|T|`)
Traits are materialized by binding the contract to a concrete implementation type:
```lone
var out = Stdout.new()
let writer = WriterTrait::|Stdout|
drop writer.write(out, "Hello, world!\n")
```
Trait materialization has zero runtime overhead; it resolves to direct call dispatch passing the receiver struct pointer.

### 7.3 Unary Operators (`UNARY_EXPR`)
Evaluated right-to-left before binary expressions:
- `-` : Negation (arithmetic minus)
- `+` : Unary positive
- `!` : Logical NOT
- `&` : **Managed Reference** operator (produces `ref<T>`)
- `&raw` : **Raw Memory Address** operator (produces `ptr<T>`)
- `*` : Dereference operator

### 7.4 Binary Operators & Precedence
From highest precedence (evaluated first) to lowest:

| Level | Operator Category | Operators | Associativity |
| :---: | :--- | :--- | :---: |
| 1 | Multiplicative | `*`, `/`, `%` | Left-to-right |
| 2 | Additive | `+`, `-` | Left-to-right |
| 3 | Relational Comparison | `>=`, `<=`, `>`, `<` | Left-to-right |
| 4 | Equality Comparison | `==`, `!=` | Left-to-right |
| 5 | Bitwise Operators | `&`, `\|`, `^`, `<<`, `>>` | Left-to-right |
| 6 | Logical AND | `&&`, `and` | Left-to-right (short-circuiting) |
| 7 | Logical OR | `\|\|`, `or` | Left-to-right (short-circuiting) |

---

## 8. AST / Intermediate Representation Reference

| Source Syntax | IR Node (`pkg/lone-parser-ast`) |
| :--- | :--- |
| `import .a, .b from :ns` | `ir.ImportDecl` |
| `@fa:wasm { ... }` | `ir.WasmDirective` |
| `var x: T = val`, `let x = val` | `ir.BIND_DEC` |
| `fn name(args) T { ... }` | `ir.FN_DEC` |
| `type Name = struct { ... }` | `ir.TYPE_DEC` (Kind `TYPE_DEC_STRUCT`) |
| `type NameTrait = trait { ... }` | `ir.TYPE_DEC` (Kind `TYPE_DEC_TRAIT`) |
| `impl Trait on Type { ... }` | `ir.IMPL_DEC` |
| `name: { ... }` | `ir.LabelDec` (Kind `EXPR_LABEL_DEC`) |
| `break name: expr` | `ir.BreakStmt` (Kind `EXPR_BREAK`) |
| `if cond { } else { }` | `ir.IfStmt` (Kind `EXPR_IF`) |
| `while cond { }` | `ir.WhileStmt` (Kind `EXPR_WHILE`) |
| `drop expr` | `ir.DropStmt` (Kind `EXPR_DROP`) |
| `return expr` | `ir.ReturnStmt` |
| `{ .field: expr }` | `ir.ObjectLiteral` (Kind `EXPR_OBJ_LITERAL`) |
| `{ 1, 2, 3 }` | `ir.ListLiteral` (Kind `EXPR_LIST_LITERAL`) |
| `base.member`, `base(...)` | `ir.PostFixExpr` with `ir.Suffix` slice |
| `Trait::\|Type\|` | `ir.PostFixExpr` with `ir.SuffixTraitMat` |
| `a + b`, `a && b` | `ir.BinaryExpr` (Kind `EXPR_BINARY_OP`) |
| `-x`, `!x`, `&x`, `&raw x` | `ir.UnaryExpr` (Kind `EXPR_UNARY_OP`) |
| `int`, `ptr<u8>`, `ref<T>` | `ir.TYPE_REF` (Kind `TYPE_BASE`, `ir.TypeBase`) |
| `[]T`, `[size]T` | `ir.TYPE_REF` (Kind `TYPE_LIST`, `ir.TypeList`) |
| `fn (T) T` | `ir.TYPE_REF` (Kind `TYPE_FN`, `ir.TypeFn`) |

---

## 9. Static Analysis & Typed IR (`pkg/lone-analyzer`)

The Lone Static Analyzer translates parsed ASTs into a **Typed IR** (`TProgram`), resolving placeholder types (`infer`) into concrete types, validating control flow, and enforcing safety rules.

### 9.1 Naming Conventions (`PassNamingConventions`)
1. **Trait Types (`naming/trait`)**:
   - Every trait declaration must end with the exact suffix `Trait` (e.g. `ComparableTrait`, `WriterTrait`).
2. **Non-Trait Identifiers**:
   - Non-trait types (structs, aliases) must not end in `trait` or be named `Trait`/`trait` (`naming/type`).
   - Non-trait declarations (functions, globals, local variables, parameters, fields, and labels) must not end in `trait` or be named `Trait`/`trait`.
   - Legitimate English words (e.g. `portrait`, `strait`, `distrait`) are preserved.
3. **General Identifiers**:
   - Functions, locals, parameters, labels, struct fields: `snake_case`.
   - Globals and constants: `ALL_CAPS` or `snake_case`.
   - Nominal types: `CamelCase`.

### 9.2 Key Analysis Passes & Safety Invariants
1. **Type Inference**:
   - Variables declared without an explicit type (`var x = expr`) are evaluated and lowered in Typed IR to concrete types matching the initializer expression.
2. **Function Return Validation (`type/return-mismatch`, `type/missing-return`)**:
   - Return statements are checked against declared function return types. Non-void functions must guarantee a return path on every branch.
3. **Result Consumption Invariant (`type/unused-result`)**:
   - Non-void expressions in statement position must be bound (`var`/`let`), returned, or explicitly dropped (`drop <expr>`).
4. **Reference vs. Raw Pointer Type Safety (`type/call-arg-type`)**:
   - Passing `&x` (`ref<T>`) to a function expecting `ptr<T>` is strictly rejected. `ptr<T>` requires explicit `&raw x`.
5. **Trait Implementation Validation (`type/missing-trait-impl`)**:
   - Trait materialization `Trait::|TargetType|` verifies that an `impl Trait on TargetType` declaration exists and implements all required method signatures.

---

## 10. Memory Model & WebAssembly Execution

Lone targets standard WebAssembly (Wasm) linear memory with an explicit, high-performance execution model:

### 10.1 WebAssembly Linear Memory Layout
```
+------------------------------------+ 0x00000000 (0)
| Static Data & String Literals      |
+------------------------------------+
| ... Free Linear Memory ...         |
+------------------------------------+
| Managed Store (Promoted Allocs)    |
+------------------------------------+ 0x00010000 (64 KiB)
|                                    |
| v  Datastack (Grows Downward)   v  |
|                                    |
+------------------------------------+ 0x00020000 (128 KiB, Initial SP)
```

### 10.2 Datastack (Shadow Stack) & SRET Rewind
- **Shadow Stack Pointer (`__stack_pointer`)**: Initialized at 128 KiB (`0x00020000`) and grows downwards.
- **Function Frame Prologue/Epilogue**: Functions that allocate local structs or take raw addresses (`&raw`) reserve a stack frame by decrementing `__stack_pointer` and restore it before returning:
  ```wat
  global.get $__stack_pointer
  i32.const 16
  i32.sub
  global.set $__stack_pointer
  ...
  global.get $__stack_pointer
  i32.const 16
  i32.add
  global.set $__stack_pointer
  ```
- **SRET (Structure Return)**: Structs returned by value are written directly to a caller-allocated datastack slot passed as an implicit first pointer parameter (`sret_ptr`).
- **Zero Leak Guarantee**: Upon function return, the datastack pointer is rewound to its entry position, reclaiming all frame memory instantly without garbage collection or reference counting overhead.

### 10.3 Struct Layout & Pointer Arithmetic
- Struct fields are laid out contiguously in linear memory with natural alignment.
- Struct member access (`s.field`) compiles to a memory load/store at the base address offset by the field's static byte offset.
- Trait materialization (`Trait::|T|`) compiles to direct function invocation passing the receiver struct pointer.

---

## 11. Standard Library (`:std`, `:std:io`, `:std:sys`)

The Lone standard library provides a pure-Lone, zero-external-dependency standard library for systems programming and console I/O.

### 11.1 Module Structure
- **`:std:io`**: Low-level stream handles, traits, buffered line reading, string formatting, and integer I/O.
- **`:std:sys`**: System-level utilities, WASI syscall bindings, and random number generators.
- **`:std`**: Root convenience entrypoint re-exporting primary I/O functions.

### 11.2 Core I/O Stream Handles & Contracts
- **Handles**: `Stdout` (fd 1), `Stderr` (fd 2), `Stdin` (fd 0).
- **Contracts**:
  ```lone
  pub type WriterTrait = trait {
      fn write(self, msg: str) i32
  }

  pub type ReaderTrait = trait {
      fn read(self, buf: ptr<u8>, max_len: i32) i32
  }
  ```

### 11.3 Standard Library Functions
- **Console Output**:
  - `print(msg: str) void`: Prints to stdout without newline.
  - `println(msg: str) void`: Prints to stdout with trailing newline.
  - `eprint(msg: str) void`: Prints to stderr without newline.
  - `eprintln(msg: str) void`: Prints to stderr with trailing newline.
- **Console Input**:
  - `read_line(in: ptr<u8>) str`: Reads a line of user input from stdin up to the newline character.
- **Numeric & String Utilities**:
  - `str_len(s: str) i32`: Returns string length in bytes.
  - `parse_int(s: str) i32`: Parses a decimal integer from a string (supports negative signs).
  - `int_to_str(val: i32, out_buf: ptr<u8>) str`: Formats an integer into a string buffer.
  - `print_int(val: i32) void`: Prints an integer to stdout.
  - `println_int(val: i32) void`: Prints an integer followed by a newline.
- **System & Random Generation**:
  - `random_i32() i32`: Generates a cryptographically secure 32-bit random integer via WASI `random_get`.
  - `random_range(min: i32, max: i32) i32`: Generates a uniform random integer in `[min, max]`.
