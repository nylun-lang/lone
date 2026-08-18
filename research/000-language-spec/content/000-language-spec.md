```tree
Lone Language Specification
│
├── 1. Design principles
│
├── 2. Lexical structure
│   ├── identifiers
│   ├── literals
│   ├── comments
│   └── keywords
│
├── 3. Syntax
│   ├── declarations
│   ├── expressions
│   ├── types
│   ├── functions
│   └── UI syntax
│
├── 4. Static semantics
│   ├── name resolution
│   ├── type checking
│   ├── mutability
│   ├── references
│   └── scopes
│
├── 5. Runtime semantics
│   ├── evaluation
│   ├── memory
│   ├── errors
│   └── concurrency
│
├── 6. UI semantics
│   ├── context
│   ├── topology
│   ├── properties
│   └── dynamic branches
│
└── 7. Compilation
    ├── lowering
    ├── WASM representation
    └── runtime requirementss
```

# Lone Language Specification

<!--toc:start-->
- [Lone Language Specification](#lone-language-specification)
  - [1. Design principles](#1-design-principles)
    - [Principle](#principle)
    - [Invariant](#invariant)
  - [2. Lexical Structure](#2-lexical-structure)
    - [Identifiers](#identifiers)
    - [Literals](#literals)
      - [String](#string)
      - [Number](#number)
      - [Boolean](#boolean)
      - [None](#none)
      - [Object literal](#object-literal)
        - [Anon Object](#anon-object)
        - [Named Object](#named-object)
    - [Comments](#comments)
      - [Simple comment](#simple-comment)
      - [Compiler hint](#compiler-hint)
        - [Doc Hint](#doc-hint)
        - [Embed Hint](#embed-hint)
    - [Keywords](#keywords)
  - [3. Syntax](#3-syntax)
    - [Declarations](#declarations)
      - [Variable](#variable)
      - [Type](#type)
        - [Type Ref](#type-ref)
          - [Modifiers](#modifiers)
          - [Construct](#construct)
          - [Base Type](#base-type)
      - [Function](#function)
      - [Expressions](#expressions)
      - [UI](#ui)
        - [Simple Loum Declaration](#simple-loum-declaration)
        - [Complex Loum Declaration](#complex-loum-declaration)
<!--toc:end-->

## 1. Design principles

### Principle

Lone is a compiled, explicit UI oriented programming language. At Core it takes
deep inspiration from language such as Go, Ts, Zig and Rust while adopting it's
own philosophy. That of being easy to read, easy to write and hard to
misunderstand

### Invariant

- **Invariant L1 — Explicit declaration** Every variable has an explicitly
  declared or inferred type.

- **Invariant L2 — No implicit imports** Names outside the current scope are not
  implicitly introduced.

- **Invariant L3 — Lexical scope** Names introduced in a lexical scope are
  accessible according to the language's lexical scoping rules.

- **Invariant L4 — Explicit mutability** Mutation requires a mutable binding.

- **Invariant L5 — ref<T> is managed** A ref<T> refers to an independently-lived
  object whose lifetime is governed by the reference-management system.

- **Invariant L6 — ptr<T> is unsafe** Raw pointers do not participate in the
  managed reference lifetime model.

- **Invariant L7 — No implicit built in** Every built in symbol/fn must be
  distinguishable from user defined code.

- **Invariant UI1 — Declared topology is stable** A UI declaration establishes a
  persistent topology known to the compiler, except at explicitly declared
  dynamic branches.

- **Invariant UI2 — Dynamic branches are local** A dynamic branch may replace
  its children without reconstructing unrelated portions of the UI graph

## 2. Lexical Structure

### Identifiers

An Identifiers identify a resource in the code. It follows common pattern:

- cannot be keyword
- cannot start by a number

It's regex : `[a-zA-Z_][a-zA-Z0-9_]*`

> Identifiers starting by underscore `_` are special case, they express hidden
> name/field/behavior

### Literals

#### String

String literal can be formed as:

```Lone
"inline string"

"""
multiline
string
"""
```

#### Number

Number literal can be formed as:

```lone
10
10.5
1.787887
10.787_887
10_000
1_000_000
```

#### Boolean

Boolean is expressed by the `true` and `false` keywords

#### None

The `none` keyword express literal absence of value

#### Object literal

Object are instantiated as etheir anonymous or named object

##### Anon Object

Anon object intervain when creating a `record` (dumb data)

```lone
let mike infer = {
    @name "mike"
    @pass "1234"
}
```

##### Named Object

A named object intervain when `instantiating` a complex data structure such as a
`struct` or it's varients

```lone
let color infer = Color{
    @R 80
    @G 80
    @B 255
}
```

### Comments

In Lone comment fall into two categories, simple comments and compiler hint
comment.

#### Simple comment

```lone
-- this is an inline comment
---
this is a multiline
comment
---
```

#### Compiler hint

Currently the planned hint are `doc`, `embed`.

##### Doc Hint

A doc comment, document a symbol or entity in the code.

```lone
---doc 
create an asci colored text for the shell

@param R, G, B
Integer from 0 to 255 expressing intensity of Red | Green | Blue
in the text output

@param text
The text being colored

@exemple build
color(1, 1, 200, "hello world")
---
fn color | R, G, B int text string | string {
    return #fmt("\x1b[38;2;{};{};{}m{} \x1b[0m", R, G, B, text)
}
```

```lone
--doc A really important constant
const FOO = "BAR"
```

> [!NOTE] It's content may require dedicated grammar

##### Embed Hint

Inspired by Go's directive `//go:embed` it's meant as for embedding file in
output binary.

```lone
---embed
@path <filepath>
@mode 0
@perm "read,write"
---
let file infer
```

> [!NOTE] It's content may require dedicated grammar

### Keywords

|  Keywords   | Purpose                       |
| :---------: | ----------------------------- |
|     fn      | function declarations keyword |
|    loum     | ui type declarations keyword  |
|  ui, with   | ui declarations keyword       |
|     let     | let declarations keyword      |
|    const    | const declarations keyword    |
|    type     | type declarations             |
|    state    | state type construct          |
|   struct    | struct type construct         |
|   record    | record type construct         |
|     ro      | readonly type modifier        |
|     lit     | literal type modifier         |
| true, false | boolean literal               |
|    none     | none literal                  |
|     if      | if statement                  |
|    else     | else clause                   |
|     for     | for loop                      |
|    range    | range operator                |
|    while    | while loop                    |
|  must, try  | error handling strategy       |

## 3. Syntax

### Declarations

#### Variable

Lone distinguish `let` and `const` variable.

**Constant**

A `const` is a binding to an inferred type `T` that cannot change and **must be
a literal**

```lone
const Y = 10
const X = foo() -- is illegal
```

> [!NOTE] Constant are always in uppercase

**Let**

A `let` is a binding to an explicit type `T` and it's capability.

```lone
-- x is an immutable runtime integer
let x ro int = foo()

-- y is a runtime integer
let y int = 1
```

For convenience the `infer` type is available **ONLY** at variable & functions
return declarations

```lone
-- x is inferred as a runtime integer
let x infer = 1

-- y is inferred as immutable whatever's the return type of foo
let y ro infer = foo()
```

#### Type

In Lone, a type define what a value is, As well as how the compiler interact
with it.

##### Type Ref

A type reference is a call upon a specific type definition, As well as tweaking
it's capability. It's made by combining a set of base types, type modifier and
type constructor.

###### Modifiers

| Symbol | Purpose                                       |
| :----: | --------------------------------------------- |
|   ro   | set a `T` as immutable                        |
|  lit   | require `T` to be a literal known in the code |

```lone
let x ro T = foo() -- x is a readonly T
```

###### Construct

|   Symbol    | Purpose                    |
| :---------: | -------------------------- |
|  `opt<T>`   | `T` may be `none`          |
| `res<T, E>` | `T` may be `E` error       |
|  `ref<T>`   | managed memory holdind `T` |
|  `ptr<T>`   | unsafe memory holding `T`  |

```lone
let x ro T = foo() -- x is a readonly T
```

###### Base Type

|     Symbol     | Purpose                            |
| :------------: | ---------------------------------- |
|   int, int32   | 32 bit signed integer              |
|      int8      | 8 bit signed integer               |
|     int16      | 16 bit signed integer              |
|     int64      | 64 bit signed integer              |
|  uint, uint32  | 32 bit unsigned integer            |
|     uint8      | 8 bit unsigned integer             |
|     uint16     | 16 bit unsigned integer            |
|     uint64     | 64 bit unsigned integer            |
|     string     | chain of character, []u8 in memory |
|      bool      | boolean                            |
| float, float32 | 32 bit floating point number       |
|    float64     | 64 bit floating point number       |

#### Function

Named function

```lone
fn greet |text string| string {
    return #fmt("hello {}", text)
}
```

#### Expressions

#### UI

**Loum** — Lone UI Model, pronounced _loom_. Is Lone's built-in syntax for
declaring and composing UI trees.

> [!IMPORTANT]+
> A `Loum` declaration describes a static UI tree in memory.
> The tree itself is composed of ordinary data structures; 
> reactivity, when present, is provided by an associated context
> rather than the node themselves.

##### Simple Loum Declaration

```lone
import std.ui.html5

---doc
create an html div greeting the user with a provided name

@param name
User provided name
---
ui<html5.HTML5> Greet |name string| {
    .render{
        .div({ @class "bold p-4 center" }) {
            .text(#fmt("Hello {}", name))
        }
    }
}
```

> Greet is a Loum declaration producing a tree of `html5.HTML5` node.

##### Complex Loum Declaration

More complex or reactive Loum declarations can be defined by attaching a context
to the UI tree through a `with {}` scope.

The `with` scope declares the state and computations available to the Loum tree.
The compiler tracks their use within the tree and can therefore determine which
nodes are affected when that state changes.

```lone
ui<UiType> Name |--- parameters ---| with {
    -- context
} {
    .render{
        -- Loum model
    }
}
```

Because Loum declarations are statically associated with their context, the
compiler can correlate state mutations with the nodes that depend on the mutated
state, allowing only the affected portions of the UI tree to be marked dirty and
updated.

This also gives the `state` type construct a direct purpose within the UI model:
`state` provides the reactive context from which Loum derives its update
dependencies.

```lone
import std.ui.html5

type Count = state {
    value int
    _step int
}

fn (c Count::new) createFrom | start, step int | Counter {
    return Counter{ 
        @value start
        @_step step
    }
}
fn (c Count::mut) decrement || {
    c.value -= c._step
}
fn (c Count::mut) increment || {
    c.value += c._step
}

ui <html5.HTML5> Counter |start int| with {
    let count infer = Counter::createFrom(start, 1)
} {
    .render({ @throttle #unitHertzPerSec(30) }) {
        .div({ @id "counter" @class "flex-h" }) {
            .text(count.value)
            .button({ @onclick count.increment }) {
                .text("increment")
            }
            .button({ @onclick count.decrement }) {
                .text("decrement")
            }
        }
    }
}
```
