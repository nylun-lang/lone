# Research Exploration: Grouped Declarations, Anonymous Functions, and Tuples

**Date**: 2026-11-05  
**Scope**: Language Design, Syntax Grammar, Closure Semantics & Type System  
**Status**: Completed & Approved (Formalized into Accepted RFC)  
**Location**: `dev/research/exploration/anonymous_functions_grouped_declarations_and_tuples.md`  
**RFC**: [`RFC-20261105-grouped_declarations_anonymous_functions_and_tuples`](../rfcs/accepted/RFC-20261105-grouped_declarations_anonymous_functions_and_tuples.md)  
**Related**:
- `dev/SPEC.md`
- `dev/rfcs/accepted/RFC-20261105-reference_and_pointer_semantics.md`
- `dev/research/audits/2026-11-05_ref_ptr_and_closure_soundness_audit.md` (Edge Case 21)

---

## 1. Executive Summary

This exploration develops three cohesive, ergonomic language extensions that build directly on Lone's memory model and recent soundness audits:

1. **Grouped Declarations (`[id1, id2, ..., idN]: Type [= default]`)**:
   A uniform bracket syntax allowing developers to declare $N \ge 2$ identifiers sharing the exact same type, modifiers, and default values across:
   - Function parameter lists: `fn draw([x, y, w, h]: int) void` or `fn draw_line([point_a, point_b]: (int, int))`
   - Struct field definitions: `struct { [w, x, y, z]: f32 = 0.0 }` or `struct { [pos, target]: (f32, f32) }`
   - Local variable bindings: `var [r, g, b, a]: u8 = 255`
   - *Core Invariant*: Every identifier in `[id1, ..., idN]` receives the exact declared `Type`. Writing `[point_a, point_b]: (int, int)` cleanly declares two distinct tuple parameters, each of type `(int, int)`.
   - *Design Invariant*: Uses square brackets `[...]` to prevent any syntactic or visual ambiguity with tuple expressions `(a, b)` and parenthesized types.
2. **First-Class Anonymous Functions (`let f = fn(...) { ... }`)**:
   Expression-position function literals with formal closure capture rules (borrowing stack data for non-escaping helpers, value-capture for escaping closures) adhering to **Edge Case 21**.
3. **Lightweight Tuples & Destructuring (`let (a, b) = pair`)**:
   Anonymous ordered product types `(T1, T2, ..., TN)`, literal construction `(10, "ok")`, and pattern destructuring `let (inc, get) = make_tools()`.

---

## 2. Grouped Declarations (`[id1, id2, ..., idN]: Type`)

### 2.1 The Problem in Existing Systems & Disambiguation with Tuples
In many languages, declaring multiple variables or fields of the same type requires either:
- Repetitive boilerplate: `x: int, y: int, z: int, w: int`
- Ambiguous comma-separated syntax: Go's `x, y, z int` can become visually noisy and difficult to parse when mixed with multi-return types or complex expressions.
- If parenthesized syntax `(a, b): int` were used, it would directly collide with tuple patterns and parenthesized expressions.

By adopting **square brackets `[id1, id2, ..., idN]`** for grouped identifier packs:
- `(...)` is reserved exclusively for **tuples** (values, types, and destructuring patterns).
- `[...]` is reserved for **grouped identifier packs**, list literals, and indexing.
- The parser enjoys deterministic LL(1) parsing without backtracking.

### 2.2 Lone Grouped Syntax Specification
A grouped declaration wraps $N \ge 2$ identifiers in square brackets, followed by their common type and optional shared default initializer:

$$\mathbf{[modifiers]\;\; [id_1, id_2, \dots, id_N]\;:\; Type\;\; [=\; default]}$$

#### The Single Universal Rule:
**Every identifier in the pack receives the declared `Type`.**

There are no confusing special cases. Whatever type is written on the right of the colon is assigned to all identifiers in the bracket:
- `[w, x, y, z]: int` $\implies$ `w: int`, `x: int`, `y: int`, `z: int`.
- `[point_a, point_b]: (int, int)` $\implies$ `point_a: (int, int)` and `point_b: (int, int)` (two parameters/fields, both tuples!).
- `[start_vec, end_vec]: (f32, f32, f32)` $\implies$ both are 3D vector tuples.
- `[name, fallback]: str = "guest"` $\implies$ both are `str` initialized to `"guest"`.

---

### 2.3 Use Cases & Grammar Integrations

#### 1. Function Parameter Lists
```lone
-- Declares four distinct parameters 'x', 'y', 'w', 'h', each of type 'int':
fn draw_rect([x, y, w, h]: int, color: str) void {
    #print("Drawing at: ")
    println_int(x)
}

-- Declares two distinct parameters 'point_a' and 'point_b', both of type '(int, int)':
fn draw_line([point_a, point_b]: (int, int)) void {
    -- point_a is (int, int)
    -- point_b is (int, int)
    let (x1, y1) = point_a
    let (x2, y2) = point_b
    ...
}
```

#### 2. Struct Field Definitions
```lone
pub type Transform = struct {
    [pos_x, pos_y, pos_z]: f32 = 0.0,
    [scale_x, scale_y, scale_z]: f32 = 1.0,
    [min_corner, max_corner]: (i32, i32),  -- Both fields are (i32, i32) tuples!
    ro [created_at, updated_at]: i64,
    _ [secret_a, secret_b]: str,          -- Grouped private fields!
}
```
* **Modifier Uniformity**: Modifiers (`ro`, `_`, `pub`) apply uniformly to every identifier in the pack.
* **Layout Invariant**: Fields are laid out sequentially in linear memory in the order written.

#### 3. Local Variable Declarations
```lone
pub fn main() void {
    -- Declares four local integers, all initialized to 0:
    var [top, bottom, left, right]: int = 0
    
    -- Declares two 2D points, both initialized to (0, 0):
    var [point_a, point_b]: (int, int) = (0, 0)
}
```
* Evaluates the initializer expression and assigns it to each variable independently.

---

## 3. First-Class Anonymous Functions (Lambdas / Closures)

### 3.1 Syntax
Anonymous functions use the `fn` keyword in expression position:

$$\mathbf{fn\; ([params])\; [RetType]\; \{\; body\; \}}$$

```lone
pub fn main() void {
    -- Anonymous function bound to variable:
    let square = fn(x: int) int {
        return x * x
    }
    
    let result = square(5)
    println_int(result) -- 25
}
```

### 3.2 Type Representation
Anonymous functions conform to Lone's existing `FnType`:
```lone
var math_op: fn(int, int) int
math_op = fn(a: int, b: int) int { return a + b }
```

### 3.3 Closure Capture Rules (Adhering to Edge Case 21)

```
                    ┌──────────────────────────────────────────────┐
                    │            Inner Closure Captures            │
                    └──────────────────────┬───────────────────────┘
                                           │
                   ┌───────────────────────┴───────────────────────┐
                   ▼                                               ▼
         Non-Escaping Closures:                          Escaping Closures:
        (Synchronous in-place)                         (Returned / Persistent)
  • Can borrow local stack &vars                 • Capturing stack &var is FORBIDDEN
  • Zero heap allocations                        • Prevents "rent-free" heap zombies
  • Datastack frame remains alive                • Must capture by VALUE (copy/move)
  • Passed to array.map, for_each                • Or use explicit #alloc heap Cell
```

#### Example 1: Non-Escaping Closure (Borrowing Stack Frame)
```lone
fn process_scores(scores: []int) int {
    var sum = 0
    
    -- Synchronous call: closure does NOT escape process_scores:
    scores.for_each(fn(score: int) {
        sum = sum + score -- Safe stack reference borrowing! Zero heap allocation.
    })
    
    return sum
}
```

#### Example 2: Escaping Closure (Capture by Value)
```lone
fn make_counter_step(step: int) fn(int) int {
    -- 'step' is captured by VALUE (copied into closure payload):
    return fn(base: int) int {
        return base + step
    }
}
```

#### Example 3: Escaping Shared State (Explicit Heap Cell via `#alloc`)
```lone
pub type Cell<T> = struct { val: T }

fn make_shared_counter() (fn() void, fn() i32) {
    -- Explicit Managed Store allocation:
    var count: ref<Cell<i32>> = #alloc(Cell<i32>::{ .val: 0 })
    
    let inc = fn() void { count.val = count.val + 1 }
    let get = fn() i32  { return count.val }
    
    return (inc, get)
}
```

---

## 4. Lightweight Tuples & Destructuring

Tuples provide anonymous, ordered heterogeneous collections without the boilerplate of defining dedicated one-off nominal structs.

### 4.1 Tuple Syntax & Types

| Construct | Syntax | Example |
| :--- | :--- | :--- |
| **Tuple Type** | `(T1, T2, ..., TN)` | `(int, str, bool)` |
| **Tuple Value Literal** | `(e1, e2, ..., eN)` | `(101, "Alice", true)` |
| **Unit / Void Tuple** | `()` | Equivalent to `void` / `Unit` |
| **Single-Element Tuple**| `(e1,)` | Trailing comma disambiguates from `(e1)` |

### 4.2 Destructuring Assignment (`let (a, b) = ...`)
Tuples integrate directly with Lone's pattern matching and variable binding system:

```lone
fn get_user_coordinates() (int, int) {
    return (1920, 1080)
}

pub fn main() void {
    -- Destructures tuple return into two separate variables:
    let (screen_w, screen_h) = get_user_coordinates()
    
    println_int(screen_w)
    println_int(screen_h)
}
```

### 4.3 Positional Member Access
When not destructured, tuple elements can be accessed via 0-based compile-time integer index:
```lone
let pair: (str, int) = ("Port", 8080)
#print(pair.0)
println_int(pair.1)
```

### 4.4 WebAssembly Memory Representation
Under the hood, a tuple `(T1, T2, ..., TN)` is lowered identically to an anonymous struct:
- Stack-allocated on Page 0 datastack frame.
- Offsets calculated statically based on alignment of members.
- Function return uses Lone's standard SRET (Structure Return) protocol, ensuring **zero heap allocations** for tuple returns.

---

## 5. Lexer & Pratt Parser Integration (`pkg2`)

### 5.1 Pratt Parser Disambiguation
1. **Parenthesized Expressions vs. Tuples**:
   - `(expr)`: Single expression without trailing comma $\implies$ grouped expression (precedence).
   - `(expr1, expr2)`: Comma encountered $\implies$ `loneast.TupleExpr`.
   - `(expr1,)`: Trailing comma after single element $\implies$ single-element tuple.
2. **Anonymous Function Expressions**:
   - In `pkg2/lone-parser/pratt.go`:
     Register `TokenFn` in `parsePrimary`:
     ```go
     case lonelexer.TokenFn:
         return p.parseFnExpr()
     ```
3. **Grouped Identifier Packs**:
   - In parameter parsing: if `peek() == TokenLBracket`, parse comma-separated identifier list up to `TokenRBracket`, followed by `expect(TokenColon)` and `parseTypeRef()`.
   - In struct fields: if `peek() == TokenLBracket`, parse identifier pack `[w, x, y, z]: Type` or `[a, b]: (int, int)`.
   - Zero lookahead ambiguity: `[` signals an identifier pack; `(` signals a tuple or parenthesized expression.

---

## 6. Summary Comparison with Other Systems

| Feature | Lone Proposal | Go | Rust | Zig |
| :--- | :--- | :--- | :--- | :--- |
| **Grouped Params** | `fn([w, x, y, z]: int)` | `w, x, y, z int` | Not supported | Not supported |
| **Grouped Fields** | `struct { [a, b]: (int, int) }` | Not supported | Not supported | Not supported |
| **Anonymous Fn** | `fn(x: int) int { ... }` | `func(x int) int { ... }` | `\|x\| x * x` | Not supported (no closures) |
| **Closure Escapes**| Error on stack ref capture | Promotes silently to heap | Checked by lifetimes | Explicit context struct |
| **Tuples** | `(int, str)` | Multi-return only | `(i32, &str)` | Anonymous struct `struct { i32, []u8 }` |
| **Destructuring** | `let (a, b) = pair` | Multi-assignment `a, b :=` | `let (a, b) = pair;` | Struct destructuring |
