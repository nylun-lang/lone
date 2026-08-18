# UI Node Construct

## 1. Context

Memory speaking a node in a `loum` (Lone UI Model) is just a dumb data structure.
Therefore, it's declaration should be that of dumb data structure.

## 2. Syntax Proposal

### Namespace like

A UI type is just namespace of node description.

```loum
import std.plateform.dom

loum HTML5 {

    node _base {
        id       opt<string>
        class    opt<string>
        onclick  opt<fn |e dom.Event| void>
        children opt<[]HTML5>
    }

    node div extends _base {}

    node text(txt string)

}
```

## 3. Rendering Proposal

The current syntax for `loum` tree declaration being :

```lone
import std.ui.html5

ui<html5.HTML5> Greet |name string| {
    .render{
        .div({ @id "hello" }) {
            .text(#fmt("hello {}", name"))
        }
    }
}
```

That is the fancy declaration. but technicaly it could lowered to a static array

```pseudocode
[
    Node1,
    Node2,
    ...,
    NodeN
]
```

The only link between them would the child having the index of it's parent or the parent having the index of it's childs.
The compiler could inline a piece of code that update the affected nodes of data changes.

A render could techincaly be notified of changes occuring in specfic index and rendering them to the user.

## Hypthosis

```lone
---fa:doc
greet the user
---
ui <html5.HTML5> Greet | name string | with {
    computed n | prev, curr string = name | {

    }
}
```
