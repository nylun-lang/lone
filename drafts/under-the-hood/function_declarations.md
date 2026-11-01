# Under the Hood: Function Declarations & Blocks

This document describes the intermediate representation and grammar rules for function declarations (`FN_DEC`), statement blocks (`ir.Block`), and return statements in Lone.

---

## 1. Syntax Overview

Function declarations use the following grammar:
```lone
fn name (arg1: T, ...) T {
    -- statements...
}
```

Key aspects:
- **`name`**: Can be a regular identifier (e.g. `add`, `process`) or prefixed with `#` for built-ins / constructors (e.g. `#init`).
- **`params`**: Zero or more typed parameters (`arg_name: TYPE_REF`), comma-separated, with optional trailing commas.
- **`return_type`**: Optional `TYPE_REF` placed after the parameter closing parenthesis `)`. When omitted, `ReturnType` is `nil` (void).
- **`body`**: Enclosed in curly braces `{ ... }`, containing zero or more statements (`STMT`).

---

## 2. IR Structures (`packages/lone-lang/irtypes/dec.go`)

```go
type FN_DEC struct {
    Name       string
    Pos        Position
    Params     []FnParam
    ReturnType *TYPE_REF
    Body       Block
}

type Block struct {
    Stmts []any
    Pos   Position
}

type ReturnStmt struct {
    Value Expr
    Pos   Position
}
```

---

## 3. PEG Grammar (`packages/lone-lang/parser/parser.peg`)

### Top-Level Dispatch
`TOP_LEVEL` parses either variable bindings or function declarations:
```peg
TOP_LEVEL <- BIND_DEC / FN_DEC
```

### Block & Statements
Inside `{ ... }`, statements are delimited by whitespace, newlines, and comments:
```peg
RETURN_STMT <- KW_RETURN expr:(_ EXPR)?
STMT <- BIND_DEC / RETURN_STMT / EXPR
BLOCK <- "{" _ stmts:(_ STMT)* _ "}"
```

### Function Declaration
```peg
FN_NAME <- "#"? IDENTIFIER

FN_DEC <- 
    KW_FN 
    _ 
    name:FN_NAME 
    _ 
    "(" 
    _ 
    params:FN_PARAM_LIST? 
    _ 
    ")" 
    ret:(_ TYPE_REF)? 
    _ 
    body:BLOCK
```
