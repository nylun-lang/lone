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
- `ptr<T>` : Unsafe raw memory address to T
- `list<T>`: Dynamically sized contiguous collection of T
- `map<K, V>`: Associative key-value table

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

```lone
type Ordering = enum{Less Equal Greater}

type Comparable<U where { U is unknown }> = trait {
  compare fn(ref<ro U>, ref<ro U>) Ordering
}

type Rect = struct {
  (w,h) int = 0
}

impl Comparable<Rect> on Rect {
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

let cmp = Comparable::|Rect|
let res = cmp.compare(&Rect::(1,2), &Rect::(1,2))
```

```lone
import { .Ordering, .Comparable, } from :std:math

fn binary_search<U where {
  U satisfies Comparable
  let D is list<U>
}>(A: D, x: U) ?U {
  
  let cmp = Comparable|U|
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


i

## 5 Memory Model [WIP]

### 5.1 Concept

Lone's memory model can be summarized as this:

- A value `definied` in global scope, is availlable for the entire execution of the program
- A value `used` in local scope are `anchored` to that scope.

### 5.2 Notion of Anchoring

An `anchor` is an implicit or explicit `ressource` that prevent a value from being freed.
An object with no anchor is automatically freed.

> **THIS IS NOT GC BASED SOLUTION**

#### 5.2.1 Implicit Vs Explicit Anchor

Again an anchor concerned values at local scope.

An implicit anchor is an anchor owned by the scope defined the value.

```lone
fn foo() {
  let x = Point::(0, 1) -- implicit anchor owed by the scope of fn, die with the fn
}
```

An explicit `anchor` is an **active** `reference` to the value

```lone
fn foo() {
  let x = [1, 2, 3, 4, 5, 6] 
  return &x -- explicit anchor that escape to caller's scope
}
```

The piece above is `illegal` code in most low level language, 
as the reference outlive it's value, by escaping the function.

But this is valid in lone, as in lone a value is not `owned` by the scope that defines it,
It is `anchored` to it, the `implicit` anchor dies with the fn, but the explicit one that is the `ref` keep the value alives as it escape.

#### 5.2.2 Limit of Explicit Anchoring

Anchoring as limit to it's usefullness, beware of the term  **active** `reference`.

```lone

fn foo() {
  let x = [1, 2, 3, 4, 5, 6]
  fn bar () {
    return  &x -- implicit anchor owed by the scope of fn, die with the fn
  } 
}

let y = foo()
let z = y()
```

IS ILLEGAL, tr the simple reason that an `active reference` is needed.
by active we means a reference immediatly avaible to runtime.

And here bar does capture the a ref to x, but that ref only exist when bar is called,
by the time foo as returned it's implicite ref as alrdey been destroyed so calling bar would return a ref to invalid memory 
value is `anchored` to one or many execution scope.

> The insipiraction came from `Go` and more recently `Goose` languages.

An `anchor` is just of way for the compiler to know wether























## 5. Runtime Semantics: Memory Model

### 5.1 Architecture Overview

Lone splits memory execution into two cooperating tiers:

1. **The Linear Datastack (Bump-Allocated Scratchpad):** Per-task/per-thread linear memory slabs where local variables, intermediate buffers, and callee scopes live.

2. **The Managed Store (Anchored Objects):** A compiler-managed heap tier where values that escape their defining scopes reside under **Explicit Anchors** (`ref<T>`).

This architecture avoids both stop-the-world tracing GC pauses and the pervasive fragmentation overhead of traditional dynamic allocators (`malloc`/`free`).

### 5.2 The Datastack (Scope-Bound Allocation)

By default, every value declared within a function or block is allocated on the current execution thread's **datastack**.

#### 5.2.1 Bump Allocation Mechanics

- A datastack maintains a single monotonic pointer: `stack_top`.
- Instantiating arrays, records, or local structs simply increments the pointer:
  $$\text{stack\_top} \mathrel{+}= \text{aligned\_sizeof}(\text{T})$$
- **Zero Allocation Cost:** Allocations require neither free-list traversals nor lock contention.

#### 5.2.2 Frame Reset & Callee Promotion

When a function `callee` returns:

- **Discarded Locals:** If local data does not escape, `stack_top` rewinds to the caller frame's base boundary, reclaiming all temporary scratchpad memory in $O(1)$.

- **Direct Value Return (Append to Caller):** When a value is returned by value (such as a structural record or primitive), the compiler arranges for the bytes to be positioned directly at the caller's stack boundary, automatically adopting it into the caller's datastack.


### 5.3 Anchoring (Escaping Lifetimes)

An **Anchor** is an active constraint recognized by Falone that prevents an object from being reclaimed when an execution scope unwinds.

#### 5.3.1 Implicit Anchors (Stack Lifetime)
An implicit anchor is an ephemeral lease owned strictly by the lexical block that declared the value.

```lone
fn process() {
  let temp = [1, 2, 3, 4]
  -- `temp` is implicitly anchored to `process`.
  -- Upon return, `stack_top` rewinds; `temp` is instantly discarded.
}
```

#### 5.3.2 Explicit Anchors (`ref<T>`)

An explicit anchor is an active, compiler-tracked reference handle (`ref<T>`).
Whenever a reference to a locally declared value escapes the lexical boundary (via return, closure capture, or cross-thread dispatch), Falone's escape analyzer promotes the value out of the temporary datastack and into the **Managed Store**:

```lone
fn generate_buffer() ref<[6]int> {
  let data = [1, 2, 3, 4, 5, 6]
  
  -- Compiler Escape Promotion:
  -- Falone detects `&data` escapes the frame.
  -- `data` is promoted to the Managed Store with an explicit anchor count of 1.
  return &data
}

```

#### 5.3.3 Deallocation Invariant

- **Invariant M1 (Reclaim Rule):** When an object has zero active anchors (all explicit `ref<T>` handles dropped and enclosing lexical scope exited), its memory is immediately reclaimed.
- **Invariant M2 (Reference Validity):** An explicit reference `ref<T>` is guaranteed never to point to a rewound datastack address.

---

### 5.4 Closure Captures & Concurrency

When closures capture variables from their defining context, Falone determines the storage strategy based on escape boundaries:

1. **Down-Funarg / Non-Escaping Closures:**
If a closure is consumed entirely within the current stack lifetime (e.g., passed to an inline loop, predicate, or mapping function), it borrows values directly from the datastack with **zero heap promotion**.
2. **Up-Funarg / Escaping Closures:**
If a closure is returned from a function or dispatched to a background thread/task:
- Any implicitly anchored variable referenced by the closure is promoted to the Managed Store.
- The closure’s synthesized capture context acquires an **explicit anchor** (`ref<T>`).
- Even if the defining function returns immediately, the captured data remains valid for the closure's entire execution lifespan across any thread.



```

---1z
### Key Wins for Falone

1. **High Throughput in Loops & Functions:** Algorithmic routines (like `binary_search` or parsing) generate dozens of intermediate lists/records that sit on the datastack and vanish with zero runtime cost.
2. **Seamless UI Integration:** Loum components[cite: 2, 3] and reactive `state` units[cite: 2, 3] naturally live via explicit anchors in the managed tier, while high-frequency render passes run their temporary scratch computations on the thread's datastack.
3. **WASM-Friendly:** WebAssembly's linear memory model maps directly to linear datastack bumping (`i32.add` on a global offset), making compiled Lone binaries small and fast.

```















```lone



let y ref[ro User<Ukind::Admin>] = &User::<'admin'>('john', 'mail.com)

let z ref[ro list[ro int]] = [ 2, 3, 4, 5, 6 ]

type InvalidColor = error{OutOfRange}

type ColorRGB = struct {

(R, G, B) int = 0

fn **init** (self ro ref[ro Self], (R, G, B) uint8) Self!InvalidColor {
self.R = R
self.G = G
self.B = B
return self
}

}

let red = ColorRGB::({ .R 255, .G 0, .B 0 })

let QUEUE_GROUP = Queue::({ })

Theard::({ .daemon: true, .target: fn || {} }){ (.daemon, .x) 0, .target fn || { } })

ui<std.html5.HTML5> Counter || {
.div({ .id: "test", .class: "" }) {
.text_node(f"Count: {count}")
}
}
