# RFC-20261105: Grouped Declarations (`[...]`), Anonymous Functions, and Tuples

- **RFC**: `RFC-20261105-grouped_declarations_anonymous_functions_and_tuples`
- **Title**: Grouped Declarations (`[...]`), Anonymous Functions, and Tuples
- **Author**: Human & AI Pair
- **Status**: Draft
- **Created**: 2026-11-05
- **Target Version**: v0.6.0
- **Related RFCs**:
  - [`RFC-20261104-struct_initialization_and_field_modifiers`](./RFC-20261104-struct_initialization_and_field_modifiers.md)
  - [`RFC-20261105-reference_and_pointer_semantics`](./RFC-20261105-reference_and_pointer_semantics.md)
  - [`RFC-20261105-string_interpolation_and_display_traits`](./RFC-20261105-string_interpolation_and_display_traits.md)
- **Applies to**: `pkg2/lone-lexer`, `pkg2/lone-parser`, `pkg2/lone-analyzer`, `pkg2/lone-compiler-wir`, `dev/SPEC.md`

---

## 1. Summary

This RFC formalizes three complementary, highly ergonomic language capabilities for Lone v0.6.0:

1. **Grouped Declarations (`[id1, id2, ..., idN]: Type [= default]`)**:
   A uniform bracket syntax allowing developers to declare $N \ge 2$ identifiers sharing the exact same type, modifiers, and default values across:
   - Function parameter lists: `fn draw_line([point_a, point_b]: (int, int))`
   - Struct field definitions: `struct { [w, x, y, z]: f32 = 0.0 }`
   - Local variable bindings: `var [top, bottom, left, right]: int = 0`
   - **The Single Universal Rule**: Every identifier in `[id1, ..., idN]` receives the exact declared `Type`.
   - **Syntax Disambiguation**: Uses square brackets `[...]` to prevent any confusion with tuple expressions `(a, b)` and parenthesized types.
2. **First-Class Anonymous Functions (`let f = fn(...) { ... }`)**:
   Expression-position function literals with formal closure capture rules:
   - **Non-escaping closures**: Can borrow local stack variables directly with **zero heap allocations**.
   - **Escaping closures**: Strictly forbidden from capturing local stack variables by reference (`ref<T>`). Must capture by **value (copy)** or hold explicit `#alloc` heap cells, preventing "rent-free" zombie memory leaks (**Edge Case 21**).
3. **Lightweight Tuples & Destructuring (`let (a, b) = pair`)**:
   Anonymous ordered product types `(T1, T2, ..., TN)`, literal construction `(10, "ok")`, and pattern destructuring `let (inc, get) = make_tools()`.

---

## 2. Motivation

In Lone v0.5.0-beta:
1. Declaring multiple fields or parameters of the same type requires repetitive copy-pasting (`x: int, y: int, z: int, w: int`).
2. Functions cannot be created inline as expressions, limiting higher-order functional patterns like `list.map` or local callbacks.
3. Returning multiple values requires defining one-off nominal structs or returning loosely typed primitives.

This RFC resolves all three limitations under Lone's strict invariants: **IL1 (Explicit over implicit)** and **zero-cost WebAssembly execution**.

---

## 3. Grouped Declarations (`[id1, id2, ..., idN]: Type`)

### 3.1 Syntax
A grouped declaration wraps $N \ge 2$ identifiers in square brackets, followed by their common type and optional shared default initializer:

$$\mathbf{[modifiers]\;\; [id_1, id_2, \dots, id_N]\;:\; Type\;\; [=\; default]}$$

### 3.2 The Single Universal Rule
**Every identifier in the pack receives the declared `Type`.**

Whatever type is written on the right of the colon is assigned to all identifiers in the bracket:
- `[w, x, y, z]: int` $\implies$ `w: int`, `x: int`, `y: int`, `z: int`.
- `[point_a, point_b]: (int, int)` $\implies$ `point_a: (int, int)` and `point_b: (int, int)` (two parameters/fields, both tuples).
- `[start_vec, end_vec]: (f32, f32, f32)` $\implies$ both are 3D vector tuples.
- `[name, fallback]: str = "guest"` $\implies$ both are `str` initialized to `"guest"`.

### 3.3 Use Cases

#### 1. Function Parameter Lists
```lone
-- Declares four distinct parameters 'x', 'y', 'w', 'h', each of type 'int':
fn draw_rect([x, y, w, h]: int, color: str) void {
    #print("Drawing at: ")
    println_int(x)
}

-- Declares two distinct parameters 'point_a' and 'point_b', both of type '(int, int)':
fn draw_line([point_a, point_b]: (int, int)) void {
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
    var [top, bottom, left, right]: int = 0
    var [point_a, point_b]: (int, int) = (0, 0)
}
```

---

## 4. First-Class Anonymous Functions (Lambdas & Closures)

### 4.1 Syntax
Anonymous functions use the `fn` keyword in expression position:

$$\mathbf{fn\; ([params])\; [RetType]\; \{\; body\; \}}$$

```lone
pub fn main() void {
    let square = fn(x: int) int {
        return x * x
    }
    
    let result = square(5)
    println_int(result) -- 25
}
```

### 4.2 Closure Capture Invariants (Adhering to Edge Case 21)

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

#### 1. Non-Escaping Closures (Stack Borrowing)
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

#### 2. Escaping Closures (Value Capture / Explicit Heap Cell)
```lone
-- Value Capture:
fn make_adder(addend: int) fn(int) int {
    return fn(x: int) int {
        return x + addend -- 'addend' is copied into the closure payload
    }
}

-- Shared Mutable State (Explicit Heap Cell):
pub type Cell<T> = struct { val: T }

fn make_shared_counter() (fn() void, fn() i32) {
    var count: ref<Cell<i32>> = #alloc(Cell<i32>::{ .val: 0 })
    
    let inc = fn() void { count.val = count.val + 1 }
    let get = fn() i32  { return count.val }
    
    return (inc, get)
}
```

---

## 5. Lightweight Tuples & Destructuring

### 5.1 Syntax & Type Grammar

| Construct | Syntax | Example |
| :--- | :--- | :--- |
| **Tuple Type** | `(T1, T2, ..., TN)` | `(int, str, bool)` |
| **Tuple Value Literal** | `(e1, e2, ..., eN)` | `(101, "Alice", true)` |
| **Unit / Void Tuple** | `()` | Equivalent to `void` / `Unit` |
| **Single-Element Tuple**| `(e1,)` | Trailing comma disambiguates from `(e1)` |

### 5.2 Pattern Destructuring (`let (a, b) = pair`)
```lone
fn get_screen_bounds() (int, int) {
    return (1920, 1080)
}

pub fn main() void {
    let (width, height) = get_screen_bounds()
    println_int(width)
    println_int(height)
}
```

### 5.3 Positional Access
```lone
let entry: (str, int) = ("Port", 8080)
#print(entry.0)
println_int(entry.1)
```

### 5.4 Zero-Cost SRET Lowering
Tuples are lowered identically to anonymous structs:
- Stack-allocated on the caller's Page 0 datastack frame via standard SRET (Structure Return).
- **Zero heap allocations** for tuple returns.

---

## 6. Static Analysis & Diagnostics

| Diagnostic Code | Condition | Severity |
| :--- | :--- | :--- |
| `closure/escaping-reference-capture` | Escaping closure captures local stack variable by reference. | Error |
| `type/tuple-arity-mismatch` | Destructuring pattern arity does not match tuple arity. | Error |
| `type/tuple-index-out-of-bounds`| Positional index `t.N` exceeds tuple arity. | Error |

---

## 7. Prior Art & Alternatives

- **Go**: Uses `a, b int` for parameters, but lacks grouped struct fields and general tuples.
- **Rust**: Uses tuples `(A, B)` and closures `|x| x * x`, but lacks grouped parameter/field declarations.
- **Zig**: Omits closures entirely; uses anonymous structs `. { a, b }`.
