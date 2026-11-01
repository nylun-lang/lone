# Under the Hood: Labeled Blocks & Break Statements

This document describes how labeled expressions (`ir.LabelDec`) and break statements (`ir.BreakStmt`) are modeled in Lone's intermediate representation and parsed by the PEG grammar.

---

## 1. Overview & Motivation

In Lone, blocks and control constructs can yield values, turning what are traditionally statements into expressions.
A **labeled block** declares an identifier followed by a colon and a block:
```lone
var result = blk: {
    var a = 10
    var b = 20
    break blk: a + b
}
```

The `break` statement targets the enclosing label and supplies the expression value yielded by that block:
```lone
break name: expr
```

Lone's syntax design rules apply here:
- **Explicit over implicit (`IL1`)**: The label is explicitly referenced at break sites (`break blk: expr`).
- **Disambiguation**: The trailing colon on the label (`name:`) distinguishes label targets from expression variables and namespace access (`:` vs `::`).

---

## 2. IR Data Structures (`packages/lone-lang/irtypes/`)

### `ir.LabelDec`
```go
type LabelDec struct {
    Name string
    Body Block
    Pos  Position
}

func (LabelDec) IsExpr() ExprKind {
    return EXPR_LABEL_DEC
}
```

### `ir.BreakStmt`
```go
type BreakStmt struct {
    Label string // Target label or empty string if unlabeled
    Value Expr   // Evaluated break expression or nil
    Pos   Position
}

func (BreakStmt) IsExpr() ExprKind {
    return EXPR_BREAK
}
```

---

## 3. PEG Grammar Rules (`packages/lone-lang/parser/parser.peg`)

### Labeled Declaration
```peg
LABEL_DEC <- name:IDENTIFIER _ ":" !":" _ body:BLOCK {
    return ir.LabelDec{
        Name: string(AnyValToType[[]uint8]("LABEL_DEC.name", name)),
        Body: AnyValToType[ir.Block]("LABEL_DEC.body", body),
        Pos: currentPosition(c),
    }, nil
}
```

### Break Statement & Label Disambiguation
```peg
BREAK_LABEL <- id:IDENTIFIER _INLINE_WS ":" !":" {
    return string(AnyValToType[[]uint8]("BREAK_LABEL.id", id)), nil
}

BREAK_STMT <- KW_BREAK label:(_INLINE_WS BREAK_LABEL)? val:(_INLINE_WS EXPR)?
```

- `_INLINE_WS` prevents break target values from unintentionally matching across subsequent newlines when a break statement appears alone on a line.
- `!":"` prevents `::` (associated access or constructor calls) from being erroneously parsed as label colons.
