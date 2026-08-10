# Lone Language Draft

## Introduction

Lone is an **explicit UI oriented compiled** programming language. 
It's to meant be readable and dead simple in interpretation. At it's core the language takes deep inspiration of Languages than are Go, Zig, Typescript, And Rust.

## Language Construct

### Variables & Types


#### Builtin Types

- int8
- int16
- int32
- int64
- uint8
- uint16
- uint32
- uint64
- float32
- float64
- bool
- string
- literal<T>
- fn(paramsTypes) ReturnType
- `infer` inference type 
- `opt<T>` for optional T
- `res<T, E>` for result type
- `ref<T>` reference type
- `ro<T>` readonly type

#### Variable Declaration

In lone a variable is bound by it's type through the Readonly type that is `ro<T>`.

Example: 
```lone
let x int8 = 1          // x is a mutable int8
let y ro<infer> = foo() // y is a readonly integer
const r = "hello lone"  // equivalent to ro<string> but value must come from a literal string
```

A constant is a literal value, where a let is variable whose capability a defined by it's type.

#### Reference & Dereferencing 

In Lone a reference is equivalent to an indirect access to memory. 

Example:
```lone
let x int = 1
let y ro<ref<ro<int>>> = &x
let z = *y 
```

#### Type Declaration

syntax: type foo = T

Where `T` is etheir a type or an object. lone define 4 kind of object.

- record : plain data type, cannot have methods attach to it. expect an annon object literal
- struct : record on which behavior can be attached. expect a named object literal
- state : struct that cannot be mutated outside of dedicated method on the type
- mutex : state that implement a lock unlock mechanish. cannot be read, nor mutated outside of dedicated methods.

> [!IMPORTANT] `[WIP]` state & mutex can be observed with builtin fn `#watchObject(v, |changed []string, prev ro<ref<infer>>| {})`

#### ObjectLiteral

type User = record {
    Name string
    Avatar opt<ref<Image>>
}

let u User = {
    @Name = "Giovanni"
    @Avatar = none
}

let u2 User = { @Name = "Giovanni" }

type Color = struct {
    R, G, B int32
    read String() string {
        return "#" + R + G + B
    }
}

let c = Color{
    @R = 1,
    @G = 2,
    @B = 3
}

the `@` prefix on object literal is construct is not sugar, it's here for lsp trigger.

#### UI declaration

##### `[WIP]` Node Declaration

##### Component Declaration

A Component is a tree of ui node. the specific UI may vary as the language aim for flexibilty on ui definition.

```lone
import std.ui.html5

type Count = state {
    
    Value int32

    new create(start int32) Counter {
        return Counter{
            @Value = start
        }
    }

    mut decrement(step uint32) void {
        Value -= step
    }

    mut increment(step uint32) void {
        Value += step
    }
}

ui<html5.UiNode> Counter(start int32) with (
    own c = Count.create(start)
) {
    .init({ @callback = ||{} })
    .render({ @throttle = #unitHertzPerSec(30) }) {
        .div({ @id = "container" }) {
            // compiler magic the resulting ui tree impl a notify like logic that rebuilt this specific node on state change
            .text(c.Value)
            .div({ @class = "flex" }) {
                .button({ @onclick = || c.increment(1) }){
                    .text("increment")
                }
                .button({ @onclick = || c.increment(-1) }){
                    .text("increment")
                }

            }
        }
    }
}
```

#### Abscence And Error handleling 

absence of value is exressed by `opt<T>`

let y int32 = 1 // here is an integer
let x opt<int32> = foo() // here may be an integer

match x {
    none => /* do someting */
    _ => {}
}

failure is expressed by the res<T, E> type and try/must/fail operators

std.io

fn crash() void!(anyerror) {
    fail Error{ @msg = "" }
}

fn main() {

    let result res<void, anyerror> = try crash()
    
    must crash() // early return on failure

}


















