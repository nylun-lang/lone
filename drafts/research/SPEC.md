# The Lone Programming Language

## 1. Introduction

Lone, is an explicit system & ui oriented programming language that compiles
to web assembly as primary target. It takes heavy inspiration on mature language
such as Go, Typescript, Python and Rust/Zig.

The languages is meant to be `Easy to read, Easy to write and Hard to misunderstand`.

## 2. Invarients

- **IL1 - Explicit over implicit** Every name reference must be explicit on `what it is`, and
  `where it came from`

- **IL2 - No Implicit built in** Every built in symbol/fn must be distinguishable from user defined code.

- **IF1 — Syntax level feedback** Punctuation shall not rescue readability that grammar already provide

- **IF2 - Progressive Information** Lone should progressively display information from left to right whenever possible.

- **IUI1 — Declared topology is stable** A UI declaration establishes a persistent topology known to the compiler, except at explicitly declared dynamic branches.

## 3. Lexical Structure

### 3.1 Identifyers

#### 3.1.1 General Definition

An Identifier is a resource identifier in code that follows a specific naming convention.

**Regex Constraint:** All identifiers must adhere to the pattern `[a-zA-Z_][a-zA-Z0-9_]`.

- The first character must be uppercase or lowercase letter or an underscore.
- Subsequent characters can be letters, digits, or underscores.

#### 3.1.2 Type-Specific Constraints

Different code types (and keywords) impose distinct casing requirements.

| Context                   | Name Convention | Example                            |
| :------------------------ | :-------------- | :--------------------------------- |
| **Functions / Variables** | `snake_case`    | `calculate`, `user_input`, `score` |
| **Globals**               | `ALL_CAPS`      | `MY_GLOBAL_VAR`, `CONFIG_ALL`      |
| **Types & Classes**       | `CammelCase`    | `UserData`, `MyClass`              |

#### 3.1.3 **Special Restrictions**

- **No Numbers:** Identifiers cannot be restricted to a numeric prefix (e.g., `x`). They must start with an alphanumeric character, underscore, or letter.

- **No Keywords:** The word "identifier" and the function keywords are not allowed to use these rules as names for code blocks.

### 3.2 Literals

#### 3.2.1 Strings

String literal can be formed as:

```Lone
"inline string"
f"format: {value}"

"""
multiline
string
"""
```

#### 3.2.2 Number

Number literal can be formed as:

```lone
10
10.5
1.787887
10.787_887
10_000
1_000_000
```

#### 3.2.3 Boolean

Boolean is expressed by the `true` and `false` keywords

#### 3.2.4 None

The `none` keyword express literal absence of value

#### 3.2.5 Object literal

Object are instantiated as ether anonymous or named object

##### Record

A record is a dumb data structure

```lone
type UserData = {
  name string
  mail string
}

let x UserData = {
 .name: "John",
 .mail: "some@mail.com"
}
```

##### Struct

A struct, in lone a struct behave like a class

```lone
type UserData = struct {
  name string
  mail string
  role string

  fn #init (
    (name, mail) string, 
    role ?string = 'guest'
  ) Self {
    return Self::{name, mail, role}
  }
}

let x = UserData::("John", "some@mail.com")
```

### 3.3 Comments

In Lone, comments falls in two categories, simple comments and compiler
directive.

#### 3.3.1 Simple comment

```lone
-- this is an inline comment

---
this is a multiline
comment
---
```

#### 3.3.2 Compiler Directive

A compiler directive is just a `simple comment` affixed of the namespace prefix
`fa:<directive>`.

```lone
--fa:<directive> INLINE_CONTENT
---fa:<directive>
MULTILINE_CONTENT
---
```

> [!NOTE] The Lone compiler is named `Falone`

The currently planned directive are:

- doc
- embed

##### 3.3.2.1 Doc Directive

A doc directive, document a symbol in the code, and can be used to generate

```lone
---fa:doc
create an asci colored text for the shell

@param R, G, B
Integer from 0 to 255 expressing intensity of Red | Green | Blue
in the text output

@param text
The text being colored

@example build
color(1, 1, 200, "hello world")
---
fn color ((R, G, B) int, text string) string {
    return #print(f"\x1b[38;2;{R};{G};{B}m{text} \x1b[0m")
}
```

```lone
--fa:doc A really important constant
const FOO = "BAR"
```

> [!NOTE] It's content may be subject dedicated/stricter grammar/layout

##### 3.3.2.2 Embed Directive

Inspired by Go's directive `//go:embed` it's meant as for embedding file in
output binary.

```lone
--fa:embed "asset/bin/foo"
let file infer
```

> [!NOTE] It's content may require dedicated grammar

## 4 Typing

In Lone, a type define what a value is, As well as how the compiler interact
with it.

There a three concept to mast in order to understand lone's typing:

- base type
- structural access, modifiyer & indirection
- structs

### 4.1 Base Type

A base type is a `built in type` or `user defined` type eg:

```lone
type U = int
type Foo = {
  bar string
}
```

|     Symbol     | Purpose                      |
| :------------: | ---------------------------- |
|      int       | 32 bit signed integer        |
|      int8      | 8 bit signed integer         |
|     int16      | 16 bit signed integer        |
|     int64      | 64 bit signed integer        |
|  uint, uint32  | 32 bit unsigned integer      |
|     uint8      | 8 bit unsigned integer       |
|     uint16     | 16 bit unsigned integer      |
|     uint64     | 64 bit unsigned integer      |
|     string     | chain of character           |
|      bool      | boolean                      |
| float, float32 | 32 bit floating point number |
|    float64     | 64 bit floating point number |
|      any       | value of any type            |

### 4.2 Structural Access & Modifyer

#### 4.2.1 Type Modifyers

| Symbol | Purpose                                                      |
| :----: | ------------------------------------------------------------ |
| `ro T` | the value of type `T` cannot be modified through this access |
|  `?T`  | the value of type `T` may be none                            |

```lone
let x ?int = none
x = 2
```

#### 4.2.2 Type Construct & Indirection

A Type Construct is a built-in parameterized type constructed from other types.
Indirection constructs do not implicitly alias or dereference; manipulating
`indirection` requires explicit syntax and operations.

The core `indirection` and collection constructs are:

- `ref<T>` : Managed, lifetime-tracked reference to T
- `ptr<T>` : Unsafe raw memory address to T (Wasm linear memory `i32` offset)
- `list<T>`: Dynamically sized contiguous collection of T
- `map<K, V>`: Associative key-value table

##### Address-of Operators: `&` vs. `&raw`

In accordance with **IL1 (Explicit over implicit)**, taking an address explicitly specifies whether it creates a managed reference or an unmanaged raw memory address:

| Syntax | Resulting Type | Description |
| :---: | :---: | :--- |
| `&foo` | `ref<T>` | **Managed Reference**: Tracked by the escape analyzer and memory model (datastack frame or promoted to Managed Store). |
| `&raw foo` | `ptr<T>` | **Raw Memory Address**: Raw byte offset in WebAssembly linear memory (`i32`). Forces `foo` to be allocated in linear memory (datastack/shadow stack). Unsafe, zero runtime overhead. |

Example: Explicit Managed Allocation & Access

```lone
type Data = { (name, mail) string }

-- Explicitly allocate a managed reference to a read-only Data record
let x ref<ro Data> = &{ 
  .name "john",
  .mail ""
}

-- Explicitly acquire/dereference the managed reference
let y = #refGrab(x)

-- Compile Error: Cannot mutate field 'name' through read-only modifier 'ro'
y.name = "max"
```

Example: Raw Pointer for Systems & WASI Syscalls

```lone
let nread = 0
-- Explicitly take the raw linear memory address (ptr<int>) for a WASI syscall
let ptr_nread ptr<int> = &raw nread
```

#### 4.3 Fn Type

```lone
type len = fn () int
```

#### 4.4 Structs

A struct is a nominal data structure with associated behavior and constructors bolted on top.

> Lone structs encapsulate data and behavior similarly to classes, while retaining explicit control over layout and reference semantics.

```lone
type Color = struct {
  (R, G, B) int8

  -- Associated Constructor (#init)
  fn #init((R, G, B) int8) Self {
    return Self::{ .R R, .G G, .B B }
  }

  -- Instance Method (immutable reference receiver)
  fn apply_on_text(self ref<ro Self>, text string) string {
    let s = #refGrab(self)
    return f"\x1b[38;2;{s.R};{s.G};{s.B}m{text}\x1b[0m"
  }
}

-- Instantiation using init construct
let blue = Color::(0, 0, 255)
#print(blue.apply_on_text("Hello World"))
```

##### Instantiation: Bare Construct vs. Init Construct

A struct can be instantiated in two ways:

1. **Bare Construct (`T::{ ... }`):**
Directly initializes the struct fields without invoking `#init`. Useful for raw layout control, internal initialization, or serialization.

```lone
let raw = Color::{ .R 255, .G 0, .B 0 }
```

2. **Init Construct (`T::(...)`):**
Invokes the associated `#init` constructor, executing validation, field assignments, or default mapping.

```lone
let safe = Color::(255, 0, 0)
```

###### Type & Associated Functions

A type `T` may define associated functions. How they are dispatched and called depends on the presence and form of the `self` parameter:

- **Static (Type-level) Functions:**
When a function does not declare a `self` parameter, it lives directly on the type namespace:

```lone
T::foo(bar)
```

- **Instance Methods:**
When a function declares a `self` parameter as its first argument, it lives on the instance:

```lone
my_var.foo(bar)
```

###### Self Access Model

The `self` parameter explicitly declares the receiver's ownership and mutability semantics:

- `self Self` — Pass-by-value: the method operates on a local copy of the instance.
- `self ref<ro Self>` — Read-only reference: the method borrows an immutable managed reference to the instance.
- `self ref<Self>` — Mutable reference: the method borrows a mutable managed reference capable of mutating the instance.


#### 4.5 Generics

The notions of generics in lone is bound to that of materialization.

##### Trait & Materialization

A trait is a set of behavior a given `T` struct must provide.

###### Trait Naming Convention
- **Trait Types**: A trait type name **must** end with the suffix `Trait` (e.g. `ComparableTrait`, `WriterTrait`, `ReaderTrait`).
- **Non-Trait Identifiers**: Any type or symbol that is **not** a trait must not end with `trait` (case-insensitive, e.g. `_trait`, `_TRAIT`, `Trait`) or be named `Trait`/`trait`. Legitimate English words (e.g. `portrait`, `strait`) are preserved.

```lone
type Ordering = enum{Less Equal Greater}

type ComparableTrait<U where { U is unknown }> = trait {
  compare fn(ref<ro U>, ref<ro U>) Ordering
}

type Rect = struct {
  (w,h) int = 0
}

impl ComparableTrait<Rect> on Rect {
  fn compare(left, right) Ordering {
    let (l, r) = #refGrab(left, right)
    if l.w + l.h > r.w + r.h {
      return Ordering::Greater
    } else if l.w + l.h < r.w + r.h {
      return Ordering::Less
    }
    return Ordering::Equal
  }
}

let cmp = ComparableTrait::|Rect|
let res = cmp.compare(&Rect::(1,2), &Rect::(1,2))
```

```lone
import .Ordering, .ComparableTrait from :std:math

fn binary_search<U where {
  U satisfies ComparableTrait
  let D is list<U>
}>(A: D, x: U) ?U {
  
  let cmp = ComparableTrait|U|
  let (start, end) = 0, #len(A, -1)

  while start <= end {
    let mid = start + ((end - start) / 2)
    match cmp.compare(A[mid], x) {
      Ordering::Less => { end = mid - 1 },
      Ordering::Equal => { return A[mid] },
      Ordering::Greater => { start = mid + 1 },
    }
  }

  return none
}
```
