# Under the Hood: If Statements & While Loops

This document details the syntax, intermediate representation (`irtypes`), and grammar for conditional branching (`ir.IfStmt`) and iterative loops (`ir.WhileStmt`) in Lone.

---

## 1. Syntax Overview

### If Statements
Lone conditionals require no parentheses around conditions and support `else if` chaining and `else` branches:
```lone
if x > 0 {
    return 1
} else if x == 0 {
    return 0
} else {
    return -1
}
```

### While Loops
Loops iterate while a condition evaluates to true:
```lone
while low <= high {
    var mid = low + 1
    if arr.get(mid) == target {
        break
    }
}
```

### Labeled Control Flow
Because `if` and `while` can be labeled using `label:`, they can evaluate as expressions yielding a value through targeted `break` statements:
```lone
var first_match = loop: while true {
    if found {
        break loop: result
    }
}
```

---

## 2. IR Data Structures (`packages/lone-lang/irtypes/expr.go`)

### `ir.IfStmt`
```go
type IfStmt struct {
    Cond Expr
    Then Block
    Else any // nil, ir.Block, or ir.IfStmt
    Pos  Position
}

func (IfStmt) IsExpr() ExprKind {
    return EXPR_IF
}
```

- When there is no `else`, `Else` is `nil`.
- When an `else` block is present, `Else` holds `ir.Block`.
- When an `else if` branch is chained, `Else` holds another `ir.IfStmt`.

### `ir.WhileStmt`
```go
type WhileStmt struct {
    Cond Expr
    Body Block
    Pos  Position
}

func (WhileStmt) IsExpr() ExprKind {
    return EXPR_WHILE
}
```

---

## 3. PEG Grammar Architecture (`packages/lone-lang/parser/parser.peg`)

### Conditional Rules
```peg
ELSE_BRANCH <- KW_ELSE _ branch:(IF_STMT / BLOCK)

IF_STMT <- KW_IF _ cond:EXPR _ then:BLOCK el:(_ ELSE_BRANCH)?
```

### Loop Rules
```peg
WHILE_STMT <- KW_WHILE _ cond:EXPR _ body:BLOCK
```

### Expression & Statement Integration
- `PRIMARY_EXPR`: Includes `IF_STMT` and `WHILE_STMT`.
- `STMT`: Supports `IF_STMT` and `WHILE_STMT` inside functions and blocks.
- `LABEL_DEC`: Labels can wrap `BLOCK`, `IF_STMT`, or `WHILE_STMT`.
- Comparison operators order: `">=" / "<=" / ">" / "<"` ensures multi-character comparison operators (`<=`, `>=`) take precedence over single-character operators (`<`, `>`).
