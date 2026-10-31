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

<div align=center>
    <img src="../../../assets/lone-banner.png" align="center"/>
    <h1>System & UI Design</h1>
</div>

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
      - [Compiler Directive](#compiler-directive)
        - [Doc Directive](#doc-directive)
        - [Embed Directive](#embed-directive)
    - [Keywords](#keywords)
  - [3. Syntax](#3-syntax)
    - [Declarations](#declarations)
      - [Variable](#variable)
      - [Type \[WIP\]](#type-wip)
        - [Type Ref](#type-ref)
          - [Modifiers](#modifiers)
          - [Construct](#construct)
          - [Base Type](#base-type)
          - [Fn Type](#fn-type)
          - [Any vs Infer](#any-vs-infer)
          - [Type \& Data structure](#type--data-structure)
          - [Type \& Associated Fn](#type--associated-fn)
      - [Function \[WIP\]](#function-wip)
      - [UI](#ui)
        - [Vocabulary](#vocabulary)
          - [Loum](#loum)
          - [Loum Node Type](#loum-node-type)
          - [Loum Node Space](#loum-node-space)
          - [Loum Tree](#loum-tree)
          - [Loum Declaration](#loum-declaration)
        - [Loum Syntax](#loum-syntax)
          - [Simple Loum Declaration](#simple-loum-declaration)
          - [Complex Loum Declaration](#complex-loum-declaration)
          - [Control Flow](#control-flow)
          - [Loum Node Space \[WIP\]](#loum-node-space-wip)
    - [Lone type system](#lone-type-system)
      - [Enum \& Error](#enum--error)
      - [Materialization](#materialization)
        - [Type Mat](#type-mat)
        - [Trait](#trait)

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
- **Invariant L5 — ref<T></t> is managed** A ref<T></t> refers to an independently-lived
  object whose lifetime is governed by the reference-management system.
- **Invariant L6 — ptr<T></t> is unsafe** Raw pointers do not participate in the
  managed reference lifetime model.
- **Invariant L7 — No implicit built in** Every built in symbol/fn must be
  distinguishable from user defined code.
- **Invariant F1 — Syntax level feedback** Punctuation shall not rescue
  readability that grammar already provide
- **Invariant F2 — Progressive Information** Lone should progressively display
  information, from left to right whenever possible.
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

Object are instantiated as ether anonymous or named object

##### Anon Object

Anon object intervein when creating a `record` (dumb data)

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

In Lone, comments falls in two categories, simple comments and compiler
directive.

#### Simple comment

```lone
-- this is an inline comment
---
this is a multiline
comment
---
```

#### Compiler Directive

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

##### Doc Directive

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
fn color | 
    R, G, B int 
    text string 
| string {
    return #fmt("\x1b[38;2;{};{};{}m{} \x1b[0m", R, G, B, text)
}
```

```lone
--fa:doc A really important constant
const FOO = "BAR"
```

> [!NOTE] It's content may be subject dedicated/stricter grammar/layout

##### Embed Directive

Inspired by Go's directive `//go:embed` it's meant as for embedding file in
output binary.

```lone
--fa:embed "asset/bin/foo"
let file infer
```

> [!NOTE] It's content may require dedicated grammar

### Keywords

|  Keywords  | Purpose                       |
| :---------: | ----------------------------- |
|     fn     | function declarations keyword |
|    loum    | ui type declarations keyword  |
|  ui, with  | ui declarations keyword       |
|     let     | let declarations keyword      |
|    const    | const declarations keyword    |
|    type    | type declarations             |
|    state    | state type construct          |
|   struct   | struct type construct         |
|   record   | record type construct         |
|     ro     | readonly type modifier        |
|     lit     | literal type modifier         |
| true, false | boolean literal               |
|    none    | none literal                  |
|     if     | if statement                  |
|    else    | else clause                   |
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

#### Type [WIP]

In Lone, a type define what a value is, As well as how the compiler interact
with it.

##### Type Ref

A type reference is a call upon a specific type definition, As well as tweaking
it's capability. It's made by combining a set of base types, type modifier and
type constructor.

###### Modifiers

| Symbol | Purpose                                        |
| :----: | ---------------------------------------------- |
|   ro   | set a`T` as immutable                        |
|  lit  | require`T` to be a literal known in the code |

```lone
let x ro T = foo() -- x is a readonly T
let y lit ro int = 1 -- y is literal readonly runtime integer, but that's redundant, use const instead
```

###### Construct

|    Symbol    | Purpose                     |
| :-----------: | --------------------------- |
|    `?T`    | `T` may be `none`       |
|  `[size]T`  | Array of T element          |
| `res<T, E>` | `T` may be `E` error    |
|  `ref<T>`  | managed memory holding`T` |
|  `ptr<T>`  | unsafe memory holding`T`  |

```lone
-- optional int
let x ?int = none

-- optional array of readonly optional integer
let y ?[]ro ?int

-- ref to readonly optional array of map of string => readonly int 
let z ref<ro ?[]map<string, ro int>>
```

###### Base Type

|     Symbol     | Purpose                            |
| :------------: | ---------------------------------- |
|     int, int32   | 32 bit signed integer              |
|      int8      | 8 bit signed integer               |
|     int16     | 16 bit signed integer              |
|     int64     | 64 bit signed integer              |
|  uint, uint32  | 32 bit unsigned integer            |
|     uint8     | 8 bit unsigned integer             |
|     uint16     | 16 bit unsigned integer            |
|     uint64     | 64 bit unsigned integer            |
|     string     | chain of character, []u8 in memory |
|      bool      | boolean                            |
| float, float32 | 32 bit floating point number       |
|    float64    | 64 bit floating point number       |
|      any      | value of any type                  |

###### Fn Type

```lone
type len = fn |a1 string| int
```

###### Any vs Infer

###### Type & Data structure

Lone admits three kind of data structures. 3 kind of data structures.

**The Record**

A **record** is a dumb data structure

```lone
type User = record {
    name string
    pass ?string
}

let X User = {
    @name "mike"
}
```

**The Struct**

A **struct** is a data structures with behavior bolted on top.

```lone
type Color = struct {
    R, G, B int

    fn createFromInt | R, G, B int | Color {
        return Color{
            @R R
            @G G
            @B B
        }
    }
}

let y infer = Color::createFromInt(100, 100, 255)
```

**The State**

A state is a data that cannot be modified outside it's construct.

> **`MyState.foo = bar` is illegal outside MyState type definition**

```lone
type Counter = state {
    value int

    fn increment |
        self    ref<Self> 
        new_val int
    | void {
        self.value = new_val 
    }
}
```

###### Type & Associated Fn

A given `T` type data structure may possess associated function.
How they are accessed depends on how they use
the `self` argument parameter and the self type.

**Without self**

The fn live on the type `T::foo(bar)`

**With self**

The fn live in the instance of `T`:

`my_var.foo(bar)`

**Self Access Model**

The self parameter is special

`self Self` - you want a mutable copy of the instance of Self
`self ref<ro Self>` - you want a references to an immutable binding to the instance of Self
`self ref<Self>` - you want a reference to a mutable binding to the instance of Self

#### Function [WIP]

Named function

```lone
fn greet |text string| string {
    return #fmt("hello {}", text)
}
```

#### UI

##### Vocabulary

###### Loum

**Loum** — Lone UI Model, pronounced _loom_, is a subset of Lone for declaring
and composing UI trees.

###### Loum Node Type

A **Loum node type** describes the dumb data and shape of a UI element such as
text, container or a button within a Loum tree, it does not provide reactivity
or managed state.

###### Loum Node Space

A Loum node space defines the node types available within a UI model and the
data associated with each node, or in simpler words it's the vocabulary from
which a Loum tree may be composed.

###### Loum Tree

A Loum tree is a composition is of Loum nodes representing a UI structure.

The structure of the Loom tree is statically described by its Loum declaration,
Runtime state may determine which nodes are active or how many instances of a
known node shape exist, but it cannot produce new shapes.

###### Loum Declaration

A Loum declaration describe a topology of a Loum tree from node types defined by
a Loum node space.

The topology describes what shapes can exists and where they can occur within
the tree.

> [!IMPORTANT] ❗ + A `Loum` declaration describes a static UI tree in memory. The
> tree itself is composed of ordinary data structures; reactivity, when present,
> is provided by an associated _context_ rather than the node themselves.

##### Loum Syntax

###### Simple Loum Declaration

```lone
import std.ui.html5

---fa:doc
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

> Greet is a Loum declaration producing a tree of Loum nodes defined in
> `html5.HTML5`.

###### Complex Loum Declaration

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

    fn createFrom | start, step int | Self {
        return Counter{ 
            @value start
            @_step step
        }
    }

    fn decrement |self ref<Self>| void {
        self.value -= c._step
    }

    fn increment |self ref<Self>| void {
        self.value += c._step
    }
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

###### Control Flow

**Loum DOES NOT allow arbitrary nor dynamic node injection**.

There is no mechanism for constructing an arbitrary Loum node from ordinary Lone
code:

```pseudocode
parent.addChildren(...)
```

Will never be a Loum construct.

> **A good UI is a dumb UI**

Instead of dynamically constructing it's structure, a Loum tree explicitly
declares all of it's possible node.

Control flows may then determine witch of these nodes are active or how many
instances of a known shape exist.

```lone
import std.ui.html5 { HTML5 }

ui <HTML5> Greet | grumpy bool | {
    .render{
        if (grumpy) {
            .text("Good morning")
        } else {
            .text("Greetings fellow human")
        }
    }
}
```

The supported control flow constructs within a Loom tree are :

- **if/else**: conditionally select
- **switch**: select based on cases
- **for**: create multiple instances of statically declared nodes

```lone
ui <HTML5> Users |users []User| {
    .render {
        .div {
            for user in users {
                .div({ @class "user" }) {
                    .text(user.name)
                }
            }
        }
    }
}
```

> [NOTE]+ None of these control flow constructs allows the program to invent new
> shape at runtime.
>
> The complete `topology` of a Loum tree is statically known. The compiler
> `falone`, never has to run lone code in order to discover Loum structures.

###### Loum Node Space [WIP]

If you where to look at the content of the `std.ui.html5` library, you would see
something similar to :

```lone
import std:platform:dom { :Event }

loum HTML5 {
  
    node _base {
        attr {
            id    opt<string>
            class opt<string>
            onclick fn |e Event| void   
        }
        children true
    }

    node div extends _base {}
  
    node select extends _base {
        children {
            accept { optgroup, option }
        }
    }

    node text(content string)
}
```

### Lone type system

#### Enum & Error

Enum and error are two side of same coint. in memory they both just integers.

```lone
type Foo = enum {
    fancy
    good
    mid
    lame
}
```

```lone
type BarErr = error {
    OutOfLuck
    LuckyAsHeck
}
```

#### Materialization

Lone define the concept of materialization as a compile time operation that produce
a type or runtime value.

Lone conceive two form of materialization `type materialization` `trait materialization`

##### Type Mat

In case of type declarations, lone generic admit a second kind of parameters.
`generic materialization parameters`

```lone
type Matrix<D>|R, C| where {
    D    is any
    D satisfies Add, Sub, Mul, ScalarMul
    R, C is int
} = struct {
   data [N * M]D
}

type T32 = Tensor<float64>|2, 2|
```

##### Trait

A define a set of behavior independent of the associated YET NOT PART of a type.

```lone
type BasicArithmetic<T> where { 
    T satisfies Add<T>
    T satisfies Sub<T>
    T satisfies Div<T>
    T satisfies Mul<T>
} = trait {}
```

In Lone a trait implementation for a specific type belong to the pkg in which
it's declared.

> the type and it's trait impl must be in the same pkg

```lone
import std:math {
    :type Ordering
    :pkg
}

type Foo = []int

impl Comparable<Foo> for Foo {
    fn less |a Foo b Foo| Foo {}
    fn greater |a Foo b Foo| Foo {}
    fn equal |a Foo b Foo| Foo {}
    fn ordering |a Foo b Foo| Ordering {}
}
```

In lone, **a trait can only be used by materializing it**, for it does not
belong to the type that impl it

`
