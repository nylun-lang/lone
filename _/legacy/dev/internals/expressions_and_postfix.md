# Under the Hood: Expressions, Literals & Postfix Architecture

This document describes how expressions, object/list literals, and postfix chaining are represented in `irtypes` and parsed by the Lone PEG grammar in `packages/lone-lang`.

---

## 1. Overview

Lone expressions evaluate from primary expressions through postfix operators and suffixes. The grammar handles:
- **Literals**: Boolean, integer, float, string, `none`, object literals, and list literals.
- **Postfix Operations**: Member access (`.`), namespace access (`:`), associated item access (`::`), function calls (`(...)`), type initialization calls (`::(...)`), and type construction (`::{ ... }`).

---

## 2. Object & List Literals

### Object Literal
Represents record or structured data instantiation using dot-prefixed fields:
```lone
var data = { .foo: "bar", .count: 10 }
```
- **IR Node**: `ir.ObjectLiteral` (`Fields []FieldAssign`)
- **Field Node**: `ir.FieldAssign` (`Name string`, `Expr Expr`)

### List Literal
Represents comma-separated literal elements enclosed in braces `{ ... }`:
```lone
var ids = { 1, 2, 3, 4 }
```
- **IR Node**: `ir.ListLiteral` (`Content []Expr`)

---

## 3. Postfix Expression Architecture

In Lone, postfix operations chain onto a base `PRIMARY_EXPR`:

```go
type PostFixExpr struct {
    Base   Expr
    Suffix []Suffix
    Pos    Position
}
```

### Suffix Kinds (`ir.Suffix`)

| Suffix Form | IR Node | Description | Example |
| :--- | :--- | :--- | :--- |
| `.name` | `ir.SuffixMemberAccess` | Field / property member access | `user.name` |
| `:name` | `ir.SuffixNamespaceAccess` | Namespace / module symbol access | `math:sin` |
| `::name` | `ir.SuffixAssociatedAccess` | Associated type/function access | `Color::Default` |
| `(args...)` | `ir.SuffixCall` | Standard function/method call | `calc(1, 2)` |
| `::(args...)` | `ir.SuffixCallTypeInit` | Associated constructor initialization | `Color::(255, 0, 0)` |
| `::{ ... }` | `ir.SuffixCallTypeMake` | Type make for object or list literal | `User::{ .name: "Alice" }` |

---

## 4. Chaining & Precedence

All postfix suffixes chain iteratively from left to right:
```lone
var result = obj.calc(1).val
```
Yields a `PostFixExpr` with:
- `Base`: `NameRef("obj")`
- `Suffix`:
  1. `SuffixMemberAccess("calc")`
  2. `SuffixCall(args=[1])`
  3. `SuffixMemberAccess("val")`
