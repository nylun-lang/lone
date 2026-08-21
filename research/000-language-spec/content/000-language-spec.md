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
      - [Compiler Directive](#compiler-directive)
        - [Doc Directive](#doc-directive)
        - [Embed Directive](#embed-directive)
    - [Keywords](#keywords)
  - [3. Syntax](#3-syntax)
    - [Declarations](#declarations)
      - [Variable](#variable)
      - [Type [WIP]](#type-wip)
        - [Type Ref](#type-ref)
          - [Modifiers](#modifiers)
          - [Construct](#construct)
          - [Base Type](#base-type)
      - [Function [WIP]](#function-wip)
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
          - [Loum Node Space](#loum-node-space-1)
    - [Expressions](#expressions)

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

- **Invariant F1 — Syntax level feedback** Punctuation shallnot rescue
  readability that grammar already provide

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
fn color | R, G, B int text string | string {
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

#### Type [WIP]

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
let y lit ro int = 1 -- y is literal readonly runtime integer, but that's redundant, use const instead
```

###### Construct

|   Symbol    | Purpose                    |
| :---------: | -------------------------- |
|  `opt<T>`   | `T` may be `none`          |
| `res<T, E>` | `T` may be `E` error       |
|  `ref<T>`   | managed memory holding `T` |
|  `ptr<T>`   | unsafe memory holding `T`  |

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
|      any       | value of any type                  |


###### Any vs Infer

###### Type & Data struct

Lone admits three kind of data structures. 3 kind of data structures.

**The Record**

A **record** is a dumb data structure

```lone
type User = record {
    name string
    pass opt<string>
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
}

let y infer = Color{
    @R 100 
    @G 100
    @B 255
}
```

**The State**

A state is a data that cannot be modified outside of mutable method.

| Name   | Quirks                                              |
| ------ | --------------------------------------------------- |
| record | pure data structure that has no method bolted on it |
| struct | data structure that can be                          |
| state  |                                                     |

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

> [!IMPORTANT]+ A `Loum` declaration describes a static UI tree in memory. The
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

###### Loum Node Space

If you where to look at the content of the `std.ui.html5` library, you would see
something similar to :

```lone
import std.platform.dom { Event }

type BaseAttr = record 

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

### Expressions

#### Match Expressions

Option Type

"opt<T>" represents a value that may either contain a value of type "T" or represent the absence of a value.

opt<T>
├── value of T
└── none

An "opt<T>" is a distinct type. A value of type "T" may be used to initialize an "opt<T>", while "none" represents the absence of a value.

let a opt<int> = 42
let b opt<int> = none

The value contained by an "opt<T>" cannot be used as a "T" without explicitly handling the optional case.

Lone uses "match" to unpack an option:

```lone
match a {
    value int => print(value)
    none => print("nothing")
}
```

"none" is a value pattern, while "value int" is a typed pattern that both verifies the value's type and binds it to "value".

Match Patterns

A "match" branch accepts either a value pattern or a typed pattern.

A value pattern consists of a single expression:

```lone
match x {
    10 => ...
    SOME_VALUE => ...
    none => ...
}
```

The expression is evaluated and compared against the value being matched.

A typed pattern consists of an identifier followed by a named type:

```lone
match x {
    value int => ...
    text string => ...
    user User => ...
}
```

The typed pattern matches when the value has the specified type and binds that value to the identifier for the duration of the branch.

This distinction is intentional:

```plaintext
X       → value pattern
X T     → typed pattern
```

A type alone is therefore not a valid match pattern. For example:

```lone
match x {
    int => ...        // invalid
    value int => ...  // valid
}
```

This keeps type matching syntactically distinct from value matching and avoids requiring the parser to determine whether a single identifier represents a type, variable, constant, or other value.

Type patterns are restricted to named types. Arbitrary type expressions are not valid typed patterns.

```lone
value int   => ...    // valid
value User  => ...    // valid
value opt<int> => ... // invalid
```

The pattern grammar is therefore conceptually:

```plaintext
pattern
    = value-pattern
    | typed-pattern

value-pattern
    = expression

typed-pattern
    = identifier named-type
```


### Formating

Lone is designed to be a compact language with a hopefully self-explanatory
grammar. The syntax should carry as much meaning as possible without requiring
punctuation to tell the reader what is already clear from the structure.

As a consequence, Lone makes relatively little use of punctuation such as commas
and equal signs. They are used where they carry meaningful syntactic
information, rather than simply to separate things that the grammar can already
distinguish.

For example, function parameters do not require commas:

```lone
fn foo |a int b string c bool| void
```

The grammar already makes the boundaries clear: a parameter consists of an
identifier followed by its type.

This compactness has an intentional consequence: **Lone can become ugly and**
**difficult to read when too much information is written inline.** This is not
considered a flaw, but a feature. The syntax allows compact code, while its
increasing visual density naturally signals when it should be expanded.

Consider:

```lone
fn foo |a opt<ref<Foo>> b, c res<Thing, Error> d string| infer {
    --- content ---
}
```

This is valid, but difficult to scan horizontally.

The formatting rule is therefore simple:

> If a declaration becomes difficult to scan horizontally, give each declaration
> its own line.

```lone
fn foo |
    a opt<ref<Foo>>
    b, c res<Thing, Error>
    d fn |int| string
| infer {
    --- content ---
}
```

This principle applies to any declaration, including object declarations:

```lone
let x infer = User{
    @name "mike"
    @pass 1234
    @mail none
}
```

rather than forcing the same structure onto one line:

```lone
let x infer = User{ @name "mike" @pass 1234 @mail none }
```

The language therefore does not need additional punctuation merely to make dense
inline code easier to read. When the compact form becomes difficult to scan,
formatting should expose the structure instead of adding syntactic noise.
