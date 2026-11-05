# Lone Language Specification (v0.6.0)

This specification documents the implementation of the **Lone Programming Language** (v0.6.0) based on the Greenfield compiler toolchain (`pkg2/lone-lexer`, `pkg2/lone-parser`, `pkg2/lone-analyzer`, `pkg2/lone-compiler-wir`, and `test2`).

---

## 1. Introduction & Philosophy

**Lone** is an explicit, systems- and UI-oriented programming language designed to compile directly to **WebAssembly (Wasm)** with zero runtime overhead as its primary target. It takes inspiration from Go, TypeScript, Python, and Rust/Zig.

### Core Philosophy
> *"Easy to read, Easy to write, and Hard to misunderstand."*

### Key Invariants
- **IL1 - Explicit over implicit**: Every name reference, type indirection, and memory allocation must be unambiguous regarding *what it is* and *where it came from*.
- **IL2 - No implicit built-ins**: Compiler intrinsics are prefixed with `#` (e.g. `#panic`, `#print`, `#alloc`, `#free`) to cleanly distinguish them from user code.
- **IL3 - Strict Resource & Result Accounting**: Non-void expressions in statement position must be consumed or explicitly dropped (`drop <expr>`). Unconsumed values trigger a static analysis error (`type/unused-result`).
- **IL4 - Explicit Trait Materialization**: Traits are pure contracts and do not inject methods into type namespaces. Trait methods must be explicitly materialized via `TrTrait::|TargetType|`.
- **IL5 - Structural Interface Contracts**: Interfaces define pure public data contracts via duck typing. Types satisfying interface contracts can be passed generically with zero runtime boxing overhead.
- **IL6 - Zero Output Tearing**: Formatted string output is stack-buffered and emitted in exactly ONE atomic WASI `fd_write` syscall.
- **IF1 - Syntax-level feedback**: Grammar layout and keywords provide readability without resorting to confusing punctuation.
- **IF2 - Progressive information**: Information displays progressively from left to right wherever possible.
- **IUI1 - Declared topology is stable**: UI declarations establish a persistent topology known to the compiler.

---

## 2. Lexical Structure

### 2.1 Identifiers
Identifiers name variables, functions, types, fields, parameters, variants, and labels.
- **Pattern**: `[a-zA-Z_][a-zA-Z0-9_]*`
- **Intrinsics / Built-in Names**: Prefixed with `#` (e.g. `#panic`, `#print`, `#alloc`, `#free`).
- **Naming Conventions**:
  - **Functions & Local Variables**: `snake_case` (e.g. `calculate_hash`, `user_id`).
  - **Globals & Constants**: `ALL_CAPS` or `snake_case` (e.g. `MAX_BUFFER_SIZE`, `default_timeout`).
  - **Types, Structs & Enums**: `CamelCase` / PascalCase (e.g. `UserData`, `TokenStream`, `Shape`).
  - **Trait Types**: **Must** begin with prefix `Tr` followed by an uppercase letter (e.g. `TrWriter`, `TrReader`, `TrClonable`, `TrDisplay`).
  - **Interface Types**: **Must** begin with prefix `Ifc` followed by an uppercase letter (e.g. `IfcReader`, `IfcVector`, `IfcDrawable`).
  - **Non-Trait / Non-Interface Identifiers**: Any type or symbol that is **not** a trait or interface must **not** begin with prefixes `Tr` or `Ifc` followed by an uppercase letter.
  - **Private Struct Fields**: Struct fields prefixed with an underscore `_` (e.g. `_secret`, `_id`) are private to the declaring module.

### 2.2 Keywords
The following keywords are reserved in Lone:
```
import    from      as        pub       var       let
type      struct    interface enum      trait     impl      fn
return    break     if        else      while     loop
for       in        drop      match     must      try
and       or        xor       is        satisfies where
infer     not       true      false     none      ro
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

### 3.4 Formatted String Interpolation Literals (`f"..."`)
Formatted string literals begin with `f"` and interpolate expressions enclosed in `${...}`:
```lone
let name = "Alice"
let score = 95
let msg = f"Player ${name} scored ${score} points (next: ${score + 5})"
let price = f"Total is \${price}" -- '\${' escapes interpolation
```
Inside regular string literals (`"..."`), `$` is an ordinary literal character and requires no escaping.

### 3.5 Tuple Literals
Tuples are heterogeneous, fixed-size product types enclosed in parentheses `( ... )`:
```lone
let pair = (10, "hello")
let triple = (true, 42, 3.14)

-- Positional member access:
let first = pair.0   -- 10
let second = pair.1  -- "hello"
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

### 4.3 Reference & Pointer Hierarchy: `ref<T>` vs. `ptr<T>`

In accordance with **IL1 (Explicit over implicit)** and the accepted pointer hierarchy ([`RFC-20261105-unsafe_pointer_indexing_and_buffer_semantics`](./rfcs/accepted/RFC-20261105-unsafe_pointer_indexing_and_buffer_semantics.md)), Lone strictly separates single-item pointers, sized array pointers, unsized buffer pointers, and managed references:

| Type | Kind | Nullable | Indexing (`[i]`) | Bounds Safety | Target Memory |
| :---: | :--- | :---: | :---: | :--- | :--- |
| `ref<T>` | **Managed Reference** | No | **Rejected** (`type/invalid-index`) | N/A (single item) | Datastack frame or Managed Store. |
| `ref<[N]T>` | **Sized Array Reference** | No | **Allowed** | Checked ($i < N$, 1-inst dynamic) | Datastack frame or Managed Store. |
| `ref<[]T>` (`[]T`) | **Fat Slice Reference** | No | **Allowed** | Checked ($i < \text{len}$) | 12-byte slice header. |
| `ptr<T>` | **Scalar Pointer** | Yes | **Rejected** (`type/single-pointer-indexing-rejected`) | N/A (must use `*p`) | Linear memory (`i32`). |
| `ptr<[N]T>` | **Sized Buffer Pointer** | Yes | **Allowed** | Checked ($i < N$, 1-inst dynamic) | Linear memory (`i32`). |
| `ptr<[]T>` | **Unsized Buffer Pointer**| Yes | **Allowed** | Unchecked (raw systems escape) | Linear memory (`i32`). |

#### Dereference, Indexing & Mutation Rules:
- **Single-Item Pointers**: `ptr<T>` points to an isolated scalar. Indexing `p[i]` is statically rejected (`type/single-pointer-indexing-rejected`). Dereferencing `*p = val` is permitted (rejected on `ptr<ro T>`).
- **Sized Buffers (`ptr<[N]T>`, `ref<[N]T>`)**: Indexing `p[i]` is verified at compile-time for constant indices ($i \ge N$ triggers `type/index-out-of-bounds`) and verified at runtime with a single unsigned instruction (`i32.ge_u N`) that cleanly traps on out-of-bounds access. Sized pointers safely decay into unsized buffer pointers `ptr<[]T>`.
- **Unsized Buffers (`ptr<[]T>`)**: Array indexing `buf[i]` executes with zero runtime overhead without bounds checks.
- **Fat References (`ref<[]T> ≡ []T`)**: Structurally equivalent to the 12-byte slice header `{ ptr, len, cap }`.
- **Field Writes Through References**: Mutating fields through references (`r.x = val` where `r: ref<Point>`) is supported. Mutating through `ref<ro Point>` emits `type/immutable-reference-write`, and mutating fields declared `ro` emits `type/immutable-field-write`.
- **Pass-By-Value Aggregate Copies**: Function arguments are passed strictly by value. When an lvalue struct argument is passed by value to a function that may alias it via a reference parameter (e.g. `f(a, &a)`), the compiler emits a caller-side memory copy to preserve the original struct value within the callee.

```lone
var val: i32 = 42

-- Managed reference to val
var r: ref<i32> = &val
*r = 99 -- Updates val to 99

-- Scalar linear memory address: indexing rejected!
var p: ptr<i32> = &raw val
*p = 100 -- Updates val to 100
-- p[0] = 101 -- ERROR: type/single-pointer-indexing-rejected

-- Buffer pointer: indexing allowed!
var buf: ptr<[]i32> = &raw val as ptr<[]i32>
buf[0] = 101 -- OK on buffer pointer
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

### 4.5 Tuple Types (`TupleType`)
Tuples represent fixed-size heterogeneous product types:
```lone
type PointPair = (int, int)
type KeyVal = (str, ?int)
```
Elements are laid out contiguously in memory with native alignment padding.

### 4.6 Interface Data Contracts (`Ifc*`)
Interfaces declare pure public data contracts validated via compile-time **structural duck typing**:

```lone
pub type IfcVector = interface {
    x: int,
    y: int,
}

pub type Ifc3D = interface {
    IfcVector, -- Interface embedding
    z: int,
}
```

- **Naming Rule**: Interface names **must** begin with `Ifc` followed by an uppercase letter. Non-interface types must not begin with `Ifc`.
- **Duck Typing (Width Subtyping)**: Any struct containing all required fields with compatible types satisfies the interface. Extra fields are allowed.
- **Read-Only Covariance**: A mutable struct field satisfies a read-only (`ro`) interface field requirement.
- **Private Fields Forbidden**: Interfaces describe public APIs and cannot contain private fields (`_*`).
- **Enums Prohibited**: Enums cannot satisfy interfaces (`type/interface-enum-rejected`).

### 4.7 Function Signatures (`TypeFn`)
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

-- Grouped variable declaration:
let [a, b, c]: int = 10     -- Binds a=10, b=10, c=10

-- Tuple pattern destructuring:
let pair = (42, "hello")
let (id, name) = pair
```

### 5.4 Function Declarations (`FN_DEC`)
Top-level functions are introduced with `fn`:
```lone
fn add(a: i32, b: i32) i32 {
    return a + b
}

-- Grouped function parameters:
fn draw_rect([x, y, w, h]: int, color: str) void {
    -- x, y, w, h are all int
}

fn noop() void {
    -- Void return type
}

fn #panic(msg: str) void {
    -- Intrinsic function with # prefix
}
```

### 5.5 Struct Declarations (`TYPE_DEC_STRUCT`)
Structs encapsulate data fields, field modifiers, constructors, and instance methods:
```lone
pub type Point3D = struct {
    -- Grouped fields:
    [x, y]: int,
    
    -- Read-only field (immutable after construction):
    ro z: int,
    
    -- Private field (accessible only in declaring module):
    _secret: int,

    -- Canonical Constructor (__init__):
    pub fn __init__(x: int, y: int, z: int) Point3D {
        return Point3D::{ .x: x, .y: y, .z: z, ._secret: 999 }
    }

    -- Instance Method (receiver 'self')
    pub fn distance_squared(self) int {
        return self.x * self.x + self.y * self.y + self.z * self.z
    }
}
```

#### Struct Rules & Enforcements:
- **Constructor Dispatch `Type::(...)`**: Calling `Point3D::(1, 2, 3)` desugars directly to `Point3D::__init__(1, 2, 3)`.
- **Encapsulation Protection**: Structs with private fields (`_*`) cannot be instantiated via raw aggregate syntax `Point3D::{ ... }` from outside; callers must use `Point3D::(...)`.
- **Initialization Guarantees**: Structs with read-only private fields must declare an `__init__` constructor (`type/missing-init-for-ro-private-field`).
- **Immutability Protection**: Writes to `ro` fields after instantiation trigger `type/immutable-field-write`.

### 5.6 Interface Declarations (`TYPE_DEC_INTERFACE`)
Interfaces declare pure data contracts without implementation:
```lone
pub type IfcVector = interface {
    [x, y]: int,
}

pub type IfcTransform = interface {
    IfcVector,       -- Embedding expands all fields from IfcVector
    ro scale: float,
}
```
Embedded interfaces must not declare conflicting field types (`type/interface-embedding-collision`).

#### Struct Instantiation
1. **Associated Construct (`T::{ ... }`)**: Directly initializes fields without invoking a constructor:
   ```lone
   var v = Vec2::{ .x: 10, .y: 20 }
   ```
2. **Associated Method Call (`T.new(...)`)**: Invokes an associated factory constructor:
   ```lone
   var v = Vec2.new(10, 20)
   ```

### 5.6 Tagged Union Enums (`TYPE_DEC_ENUM`)
Enums define tagged unions whose variants may optionally hold named payload fields:
```lone
pub type Shape = enum {
    Circle(radius: int),
    Rectangle(width: int, height: int),
    Point,
}

pub type Option<T> = enum {
    Some(val: T),
    None,
}

pub type Result<V, U = Error where { U is Error }> = enum {
    Ok(val: V),
    Err(err: U),
}
```

- **Tag Allocation**: Variants are assigned sequential integer discriminant tags starting at `0`.
- **Memory Layout**: 4-byte discriminant tag at offset `0`, followed by payload fields aligned to the maximum payload alignment.
- **Dynamic Allocation**: Constructed on the heap via the Free-List allocator runtime (`$__lone_rt_alloc`).

### 5.7 Trait Declarations (`TYPE_DEC_TRAIT`)
Traits define behavioral contracts that types can satisfy.
- **Naming Rule**: Trait type names **must** start with the `Tr` prefix followed by an uppercase letter (e.g. `TrWriter`, `TrReader`, `TrClonable`).

```lone
pub type TrWriter = trait {
    fn write(self, msg: str) void
}

pub type TrReader = trait {
    fn read(self, buf: ptr<u8>, max_len: i32) i32
}
```

### 5.8 Implementation Declarations (`IMPL_DEC`)
Implementations attach trait contracts to concrete target types:
```lone
impl TrWriter on Console {
    fn write(self, msg: str) void {
        #print(msg)
    }
}
```

### 5.9 Explicit Trait Materialization (`TrTrait::|T|`)
In accordance with **IL4 (Explicit Trait Materialization)**, traits are pure contracts and do not implicitly inject methods into a type's namespace. To invoke a trait method on a conforming type, the trait implementation must be explicitly materialized:

```lone
fn log_data<T where { T satisfies TrWriter }>(writer: T, msg: str) void {
    let w = TrWriter::|T|
    w.write(writer, msg)
}

-- Or directly chained:
TrWriter::|Console|.write(con, "hello\n")
```

The compiler statically checks that `TargetType` implements `TrTrait` via an `impl` declaration, synthesizes a `MaterializedTraitType`, and binds `self` to `TargetType`.

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

#### 6.4.1 Infinite & Post-Condition Loops (`LOOP_STMT`)
Runs an infinite loop until terminated by `break` or `return`, or loops repeatedly while a bracketed post-condition holds:
```lone
-- Infinite loop
loop {
    var line = read_line()
    if line == "quit" {
        break
    }
    println(line)
}

-- Post-condition loop (do-while)
var count = 0
loop {
    count = count + 1
    var step = count
    println_int(step)
} while: [step < 5]
```
- **Post-Condition `while: [cond]`**:
  - Executes the loop body **at least once** before evaluating `cond`.
  - The condition is prefixed with `while:` and enclosed in brackets `[...]`, completely avoiding semicolon requirements and line bleed-through.
  - **Smart Scoping**: Variables declared inside the `loop { ... }` body remain in scope and are accessible inside `while: [cond]`. Once the loop exits, those variables fall out of scope.
  - Can also be labeled for targeted breaks: `outer: loop { inner: loop { break outer } } while: [true]`.
  - Infinite `loop { ... }` desugars into an `ir.WhileStmt` with constant `true` condition; post-condition loops parse as `ir.DoWhileStmt` and lower directly into structured Wasm blocks and branch-back instructions.

#### 6.4.2 For-In Loop (`FOR_STMT`)
Iterates over an iterable value using Lone's standard iterator protocol (`IteratorTrait<T>` and `IterableTrait<T>` in `:std:builtin`):
```lone
import .range, .range_step from :std:builtin

for i in range(1, 5) {
    println_int(i)
}

outer: for x in items {
    if x > 10 {
        break outer
    }
}
```
- **Iterator Protocol**:
  - If the iterable implements `iterator()`, `var __it = iterable.iterator()` is called to obtain the iterator.
  - If the iterable directly implements `has_next() bool` and `next() T`, it is used as the iterator directly.
  - Desugars cleanly at analysis into:
    ```lone
    {
        var __it = iterable.iterator()
        while __it.has_next() {
            var i = __it.next()
            -- body
        }
    }
    ```
- Built-in `Range` struct and `range(start, end)`, `range_step(start, end, step)` are available in `:std:builtin` and re-exported via `:std`.
- `impl Trait for Target` and `impl Trait on Target` are both supported.
- Optional types `?T` (e.g. `?i32`, `?str`, `[]?str`, `none`) are supported for nullable and partial values.

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
| `.0`, `.1` | Tuple positional element access | `pair.0`, `triple.1` |
| `:name` | Namespace / module symbol access | `math:sin` |
| `::name` | Associated function / type access | `Vec2::new` |
| `::(...)` | **Canonical Constructor Call**: Dispatches to `Type::__init__(...)` | `Point3D::(1, 2, 3)` |
| `(args...)` | Function / method call | `calc(1, 2)` |
| `[i]` | Buffer indexing (valid on `ptr<[N]T>`, `ptr<[]T>`, `ref<[N]T>`, `ref<[]T>`, `[]T`; rejected on scalar `ptr<T>` and `ref<T>`) | `p[0]` |
| `::{ ... }` | Raw aggregate construct (requires all fields to be public) | `Vec2::{ .x: 1, .y: 2 }` |
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
- `*` : Dereference operator (required for reference / pointer stores: `*r = 10`)

### 7.4 Binary Operators & Precedence
From highest precedence (evaluated first) to lowest:

| Level | Operator Category | Operators | Associativity |
| :---: | :--- | :--- | :---: |
| 1 | Multiplicative | `*`, `/`, `%` | Left-to-right |
| 2 | Additive | `+`, `-` | Left-to-right |
| 3 | Relational Comparison | `>=`, `<=`, `>`, `<` | Non-associative |
| 4 | Type Test | `is`, `is not` | Non-associative |
| 5 | Equality Comparison | `==`, `!=` | Left-to-right |
| 6 | Bitwise AND | `&` | Left-to-right |
| 7 | Bitwise XOR | `^` | Left-to-right |
| 8 | Bitwise OR | `\|` | Left-to-right |
| 9 | Logical AND | `&&`, `and` | Left-to-right |
| 10 | Logical XOR | `xor` | Left-to-right |
| 11 | Logical OR | `\|\|`, `or` | Left-to-right |

> [!NOTE]
> Comparison chaining is disallowed at compile time. Writing chained comparisons such as `a < b < c` or `a <= b <= c` results in a compilation error. Use explicit logical conjunction instead: `a < b and b < c`. Comparing a boolean with a relational operator (e.g. `(a < b) < c`) is also rejected during semantic analysis.

### 7.5 Type Checking & Narrowing (`IS_EXPR`)
Tests if a value matches a given type reference (`is`) or does not match (`is not`):
```lone
if val is i32 {
    println_int(val) -- automatically narrowed to i32
}
```

### 7.6 Anonymous Functions (`ANON_FN_EXPR`)
Anonymous functions can be defined inline as expressions:
```lone
let square = fn(x: int) int {
    return x * x
}
let res = square(5) -- 25
```
- **Closure Capture Analysis**: Closures capturing outer local variables by reference across escaping boundaries trigger `closure/escaping-reference-capture`.
- **Lowering**: Anonymous functions are hoisted to top-level WebAssembly functions (`anon_fn_N`).

### 7.7 Formatted String Interpolation & `TrDisplay`
Formatted string interpolation (`f"..."`) provides zero-allocation, atomic output:
```lone
let name = "Alice"
let count = 42
println(f"Hello ${name}, you have ${count} notifications.")
```

- **Stack-Buffered Single Write Architecture**: When passed to output functions (`println`, `print`), the formatted string is built into a temporary Page 0 datastack buffer and flushed in **exactly ONE atomic WASI `fd_write` syscall**. This guarantees zero output tearing and eliminates fragmented pipe chunks without heap allocations.
- **Escaping Value Scope**: In value position (`let s: str = f"..."`), the formatted string is allocated in the heap.
- **The `TrDisplay` Trait**: Types can be formatted by implementing `TrDisplay`:
  ```lone
  pub type TrDisplay = trait {
      fn fmt(self, buf: ref<StringBuilder>) void
  }
  ```
  All primitive types (`str`, `int`, `float`, `bool`, `char`, `ptr<T>`) provide inlined zero-cost implementations out of the box. Un-displayable types trigger `type/missing-trait-impl`.
}

if opt is not none {
    println(opt)     -- automatically narrowed from ?str to str
}

if !(opt is none) {
    println(opt)     -- automatically narrowed to inner type
}
```
- **Automatic Type Narrowing**: Inside the positive branch of `if x is T` (or negative branch of `if x is none`), `x` is statically narrowed to `T`. In `else` blocks, the opposite narrowing applies.
- For optional types (`?T`), runtime lowering checks the discriminant tag / null-pointer.
- For statically fixed concrete types, `is` tests are resolved at compile time with zero runtime overhead.

### 7.6 Match Expression (`MATCH_EXPR`)
Exhaustive pattern matching across enums, variants, literals, and wildcards:
```lone
let name = match shape {
    Shape::Circle(r) => "Circle",
    Shape::Rectangle(w, h) => "Rectangle",
    Shape::Point => "Point",
    _ => "Unknown",
}
```
- **Exhaustiveness Check**: The compiler validates that all enum variants are handled. Missing variants without a wildcard `_` fallback trigger a static analysis error (`type/non-exhaustive-match`).
- **Pattern Destructuring**: Variant payload fields are extracted directly into arm-scoped variable bindings.

### 7.7 Error Handling: `must` and `try`
In Lone, `Result<V, U = Error where { U is Error }>` is the standard representation for fallible operations:
- **`must <expr>`**: Asserts that `expr` evaluated to `Result::Ok(val)` and returns `val`. If `expr` evaluated to `Result::Err`, it executes WebAssembly `unreachable` (panics immediately with zero overhead):
  ```lone
  let content = must read_file("config.txt")
  ```
- **`try <expr>`**: Early-return error propagation. If `expr` is `Result::Ok(val)`, evaluates to `val`. If `expr` is `Result::Err(err)`, returns early from the enclosing function with `Result::Err(err)`:
  ```lone
  fn calculate(x: i32, y: i32) Result<i32> {
      let d = try divide(x, y)
      return Result::Ok(d + 10)
  }
  ```
  The enclosing function must have a compatible `Result` return type. Using `infer` as the error type (`Result<T, infer>`) automatically unifies all propagated error variants.

### 7.8 Generics & Constraints (`where`)
Functions, structs, and enums support generic type parameters with compile-time constraints:
```lone
-- Generic type with constraint
pub type Container<T where { T satisfies TrClone }> = struct {
    item: T,
}

-- Generic function with trait constraint
fn log_item<T where { T satisfies TrWriter }>(writer: T, msg: str) void {
    let w = TrWriter::|T|
    w.write(writer, msg)
}

-- Multi-parameter constraints
pub type Result<V, U = Error where { U is Error }> = enum {
    Ok(val: V),
    Err(err: U),
}
```
- **Constraint Operators**:
  - `is`: Nominal type/category constraint (e.g. `U is Error`).
  - `satisfies`: Trait contract check (verifying that `impl TrTrait on T` exists).
- **Monomorphization**: Generic functions and types are monomorphized on demand during compilation.

---

## 8. WebAssembly Linear Memory Architecture

### 8.1 Memory Layout & Partitioning
Lone organizes 64 KiB WebAssembly linear memory pages into dedicated, non-overlapping regions:

```
+--------------------+ 0
| Reserved Scratch   | WASI iovec (0..63)
+--------------------+ 64
| Static Constants   | String literals and metadata tables (64..dataEnd)
+--------------------+ dataEnd
| Stack Guard Page   | $__stack_limit (minimum 1024)
|                    | 
| Datastack Area     | Downward-growing stack ($__stack_pointer starts at 65536)
+--------------------+ 65536 ($__heap_base)
| Dynamic Heap       | Upward-growing heap managed by Free-List Allocator
|                    | Automatic dynamic expansion via memory.grow
+--------------------+ memory.size * 65536
```

- **Zero Collisions**: Because stack grows downward in Page 0 and heap grows upward starting at Page 1, stack and heap cannot collide.

### 8.2 Free-List Allocator Runtime
Heap allocations are managed by an inlined WebAssembly runtime:
- **Block Headers (8 bytes)**: Tracks payload size and allocation status (`0 = FREE`, `1 = ALLOCATED`).
- **Bi-directional Coalescing**: When `#free` is called, adjacent free blocks to both the left and right are merged in constant time.
- **Block Splitting**: Allocations split large free blocks if residual space is $\ge 8$ bytes.
- **Dynamic Memory Expansion**: When heap space is exhausted, `$__lone_rt_alloc` automatically invokes `memory.grow`.

### 8.3 Linear Memory Intrinsics
- `#alloc(size: int) int`: Dynamic heap allocation via free-list runtime.
- `#free(ptr: int) void`: Free block and coalesce with adjacent free blocks.
- `#realloc(ptr: int, new_size: int) int`: Resizing with in-place reuse or copy.
- `#memory_grow(pages: int) int`: Dynamic WebAssembly `memory.grow`.
- `#memory_size() int`: Current page count.
- `#stack_pointer() int`: Current linear stack pointer offset.
- `#stack_alloc(size: int) int`: Bump datastack allocation with guard page overflow check.
- `#stack_free(size: int) void`: Constant-time stack rewind.

---

## 9. AST / Intermediate Representation Reference

| Source Syntax | IR Node (`pkg2/lone-parser-ast`) |
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
   - Every trait declaration must begin with the exact prefix `Tr` followed by an uppercase letter (e.g. `TrWriter`, `TrReader`, `TrClonable`, `TrDisplayer`).
2. **Non-Trait Identifiers**:
   - Non-trait types (structs, aliases, enums) must not begin with `Tr` followed by an uppercase letter (`naming/type`).
   - Non-trait declarations (functions, globals, local variables, parameters, fields, and labels) must not begin with `Tr` followed by an uppercase letter.
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

---

## 12. Toolchain & CLI Reference (`lone`)

The unified `lone` CLI driver controls the entire compilation, semantic verification, debugging, and execution lifecycle.

### 12.1 Subcommands
- `lone build <entry.lone> [-o out.wat] [-t wasi|web]`: Compiles Lone sources into WebAssembly Text (`.wat`) or binary (`.wasm`).
- `lone run <entry.lone>`: Compiles and immediately executes the binary in-process via embedded Wasmtime.
- `lone check <entry.lone>`: Runs multi-pass type checking, safety analysis (Consume-or-Drop), and naming convention verification.
- `lone debug <scope> [symbol]`: Deeply inspects symbols, scopes, memory layouts, trait contracts, and storage locations.
- `lone lsp`: Starts the official Language Server Protocol daemon over standard I/O.

### 12.2 Debug Toolchain Subcommands (`lone debug`)
- `lone debug dump <scope>` (shorthand `lone debug <scope>`): Dumps an overview of all structs, traits, functions, globals, and imports in a file or namespace (e.g. `:std:io`).
- `lone debug whatis <scope> <symbol>` (shorthand `lone debug <scope> <symbol>`): Inspects a symbol in depth:
  - **Structs**: Total byte size, alignment, field offsets/sizes/types, constructors, methods, and implemented trait fulfillment.
  - **Traits**: Required method contracts and concrete struct implementors in scope.
  - **Variables**: Declared vs inferred types, initialization state, defined location, storage location (`datastack frame` vs `static linear memory`), and address-taken state (`&raw`).
  - **Scoped Symbols**: Supports `->`, `:`, and `.` to inspect inner function variables (`fn->var`), struct fields (`struct:field`), and methods (`struct:method`).
  - **JSON Output**: Supports `--json` flag for machine-readable integrations.
