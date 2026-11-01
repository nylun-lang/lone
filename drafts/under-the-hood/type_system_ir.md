# Under the Hood: Type Reference IR Architecture

This document describes how type references are structured in the intermediate representation (`irtypes`) and parsed by the PEG grammar in `packages/lone-lang`.

---

## 1. Motivation & Architecture

In Lone, types are first-class, expressive syntactic constructs. Rather than treating all types as flat identifiers, `TYPE_REF` models three distinct structural forms:

1. **`TypeBase`**: Named types with optional parameterized type arguments (e.g. `int`, `string`, `list<int>`, `Map<string, int>`).
2. **`TypeList`**: Slice / dynamic array types of arbitrary items using prefix syntax `[]TYPE_REF` (e.g. `[]int`, `[][]float`, `[]?string`).
3. **`TypeFn`**: First-class function signatures with typed parameters and an optional return type (e.g. `fn (a: int, b: string) bool`, `fn ()`, `fn (cb: fn (x: int) bool) []string`).

Orthogonally to their structural kind, type references carry modifiers/constraints (`TypeConstrain`):
- `ro`: Read-only constraint.
- `?`: Nullable / optional constraint.
- `comptime`: Evaluated or required at compile time.

---

## 2. IR Data Structures (`irtypes/dec.go`)

```go
type TypeRefKind int

const (
    TYPE_BASE TypeRefKind = iota
    TYPE_LIST
    TYPE_FN
)

type TypeConstrain struct {
    ReadOnly bool
    Optional bool
    Comptime bool
}

type TYPE_REF struct {
    Pos       Position
    Kind      TypeRefKind
    Constrain TypeConstrain
    Value     any // Holds TypeBase, TypeList, or TypeFn
}
```

### Specialized Payloads

- **`TypeBase`**:
  ```go
  type TypeBase struct {
      Name string
      Args []TYPE_REF
  }
  ```
- **`TypeList`**:
  ```go
  type TypeList struct {
      ItemType TYPE_REF
  }
  ```
- **`TypeFn`**:
  ```go
  type FnParam struct {
      Name string
      Type TYPE_REF
  }

  type TypeFn struct {
      Params     []FnParam
      ReturnType *TYPE_REF // nil if void
  }
  ```

---

## 3. PEG Grammar Mapping (`parser/parser.peg`)

The PEG grammar evaluates types with the following hierarchy:

```peg
TYPE_ACTUAL <- TYPE_FN / TYPE_LIST / TYPE_BASE

TYPE_REF <- 
    is_comptime:KW_COMPTIME?
    _
    is_readonly:KW_READONLY? 
    _ 
    is_optional:"?"? 
    _
    actual:TYPE_ACTUAL
```

### Disambiguation & Left-to-Right Clarity
1. `TYPE_FN` begins with keyword `fn`, preventing collision with identifiers.
2. `TYPE_LIST` begins with delimiter `[]`.
3. `TYPE_BASE` matches an `IDENTIFIER` followed by optional `<...>` arguments.
4. Because each payload accepts `TYPE_REF` recursively, arbitrary nested compositions (e.g. `[]fn (x: []int) ?string`) parse deterministically.
