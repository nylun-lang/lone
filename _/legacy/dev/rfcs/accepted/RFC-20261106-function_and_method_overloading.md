# RFC-20261106: Function and Method Overloading via `@fa:fn_overload`

- **RFC**: `RFC-20261106-function_and_method_overloading`
- **Title**: Function and Method Overloading via `@fa:fn_overload`
- **Author**: Human & AI Pair
- **Status**: Accepted
- **Created**: 2026-11-06
- **Target Version**: v0.7.0
- **Related RFCs**:
  - [`RFC-20261104-struct_initialization_and_field_modifiers`](./RFC-20261104-struct_initialization_and_field_modifiers.md)
  - [`RFC-20261105-grouped_declarations_anonymous_functions_and_tuples`](./RFC-20261105-grouped_declarations_anonymous_functions_and_tuples.md)
- **Applies to**: `pkg/lone-parser`, `pkg/lone-analyzer`, `pkg/lone-compiler-wir`, `dev/SPEC.md`

---

## 1. Summary

This RFC introduces explicit, deterministic **Function and Method Overloading** in Lone, guarded under the `@fa:fn_overload` compiler directive.

Key capabilities:
1. **Multiple Constructor Arities**: Enables structs to declare multiple `__init__` constructors (e.g. default constructor `Game::()` alongside customized `Game::(max_num: i32)`).
2. **Deterministic Overload Resolution**: Because Lone has **no implicit numeric promotions or implicit type coercions**, overload resolution is completely deterministic, matching candidate signatures based on exact arity and exact type identity.
3. **Wasm-Safe Symbol Mangling**: Automatically generates deterministic, unique WebAssembly symbol identifiers for each overloaded signature (e.g. `$Game.__init__$void`, `$Game.__init__$i32`).
4. **Scope of Application**: Supports struct constructors (`__init__`), inherent struct methods, and top-level module functions.
5. **Prohibition of Return-Type-Only Overloading**: Overloading functions differing solely by return type is strictly prohibited.

---

## 2. Motivation

In Lone v0.6.0, all identifiers within a scope must be unique:
- A struct can declare at most one `__init__` constructor method.
- A module can declare at most one function with a given identifier.

This introduces notable ergonomic friction:

### 2.1 Constructor Proliferation
In real-world applications, data types routinely require multiple construction pathways:
```lone
-- Current v0.6.0 workaround:
pub fn __init__(max_num: i32) Game { ... }
pub fn new_default() Game { return Game::(100) }
```
Callers must remember disparate factory names (`Game::(100)` vs `Game.new_default()`) instead of uniform constructor syntax:
```lone
var g1 = Game::()
var g2 = Game::(100)
```

### 2.2 Standard Library & Utility Ergonomics
Formatting, math, and logging functions frequently operate on different primitive types:
```lone
-- Current v0.6.0 workaround:
pub fn print_i32(val: i32) void { ... }
pub fn print_str(val: str) void { ... }
pub fn print_bool(val: bool) void { ... }
```
Forcing callers to encode parameter types into function identifiers clutters public APIs and increases cognitive load.

### 2.3 Why Lone Avoids the "C++ Overloading Nightmare"
In languages like C++, function overloading is notoriously bug-prone and complex due to implicit promotion hierarchies:
- `char` $\rightarrow$ `short` $\rightarrow$ `int` $\rightarrow$ `long` $\rightarrow$ `float` $\rightarrow$ `double`
- Ambiguous conversion operators, pointer promotions, and template specializations.

In Lone:
- **All type casts are explicit** (`val as i32`).
- There is **zero implicit numeric widening or coercion**.
- Therefore, a call site with argument types `(i32, str)` can only match a signature accepting `(i32, str)`. Overload resolution is straightforward, fast, and completely free of subtle type promotion bugs.

---

## 3. Detailed Specification

### 3.1 Directive Syntax: Block Form (Preferred) & Inline Form

#### Form A: Block Directive (Primary / Preferred)
Overloaded functions are enclosed within an `@fa:fn_overload` directive block. This is the canonical form because grouping overloads in a single block makes API contracts clear, prevents accidental name pollution across distant code, and aids tooling/doc generation:

#### Struct Constructor Overloading (`__init__`)
```lone
pub type Game = struct {
    _secret: i32,
    ro max_val: i32,
    attempts: i32,

    @fa:fn_overload {
        -- Overload 1: Default constructor (max_val = 100)
        pub fn __init__() Game {
            return Game::(100)
        }

        -- Overload 2: Custom range constructor
        pub fn __init__(max_num: i32) Game {
            return Game::{
                ._secret: random_range(1, max_num),
                .max_val: max_num,
                .attempts: 0,
            }
        }
    }
}
```

#### Form B: Inline Statement / Decorator Form (Incremental Overloads)
Individual functions may also be decorated with `@fa:fn_overload` directly:
```lone
@fa:fn_overload
pub fn format(val: i32) str { ... }

@fa:fn_overload
pub fn format(val: f32) str { ... }
```

#### Inherent Method Overloading
```lone
pub type Vector3 = struct {
    x: f32,
    y: f32,
    z: f32,

    @fa:fn_overload {
        -- Scale by uniform scalar
        pub fn scale(self: ref<Vector3>, factor: f32) void {
            self.x = self.x * factor
            self.y = self.y * factor
            self.z = self.z * factor
        }

        -- Scale by non-uniform factors
        pub fn scale(self: ref<Vector3>, fx: f32, fy: f32, fz: f32) void {
            self.x = self.x * fx
            self.y = self.y * fy
            self.z = self.z * fz
        }
    }
}
```

#### Module-Level Function Overloading
```lone
@fa:fn_overload {
    pub fn format(val: i32) str { ... }
    pub fn format(val: f32) str { ... }
    pub fn format(val: bool) str { ... }
}
```

---

## 4. Overload Resolution Algorithm

When the analyzer encounters a call expression `callee(arg0, arg1, ...)` where `callee` is marked as overloaded:

### Step 1: Arity Filtering
Filter candidates to those having the exact same parameter count $N$ as the argument count:
$$\text{Candidates}_1 = \{ f \in \text{Overloads}(\text{callee}) \mid \text{len}(f.\text{Params}) = \text{len}(\text{Args}) \}$$
If $\text{Candidates}_1$ is empty, report:
> `type/overload-no-match: no overload for 'callee' accepts N argument(s)`

### Step 2: Exact Type Matching
Evaluate types of all arguments: $(T_0, T_1, \dots, T_{N-1})$.
Filter $\text{Candidates}_1$ to candidates whose parameter types match the argument types exactly:
$$\text{Candidates}_2 = \{ f \in \text{Candidates}_1 \mid \forall i \in [0, N-1],\; f.\text{Params}[i].\text{Type} = T_i \}$$

- If $|\text{Candidates}_2| = 1$: **Match resolved**. Select candidate $f^*$.
- If $|\text{Candidates}_2| = 0$: Report:
  > `type/overload-mismatch: cannot call overloaded function 'callee' with arguments (T0, T1, ...)`  
  > *Diagnostics list all available candidates and expected signatures.*
- If $|\text{Candidates}_2| > 1$: Report:
  > `type/overload-ambiguous: ambiguous call to overloaded function 'callee'`

### Step 3: Reference/Value Automatic Adaptation
As established in Lone v0.6.0:
- If a parameter expects `ref<T>` and the caller passes an lvalue variable `x: T`, the compiler automatically passes `&x`.
- If an overload requires `ref<ro T>` vs `ref<T>`, the caller's binding mutability is checked.

---

## 5. WebAssembly Symbol Mangling

In WebAssembly, every module function must have a unique alphanumeric identifier. The compiler lowerer synthesizes distinct symbols using parameter type signatures:

### Mangling Schema
$$\text{MangledName} = \$\text{Scope}.\text{FnName}\$[\text{ParamType}_0]\_[\text{ParamType}_1]\dots$$

For example:
- `Game::__init__()` $\longrightarrow$ `$Game.__init__$void`
- `Game::__init__(max_num: i32)` $\longrightarrow$ `$Game.__init__$i32`
- `scale(self: ref<Vector3>, factor: f32)` $\longrightarrow$ `$Vector3.scale$ref_Vector3_f32`
- `scale(self: ref<Vector3>, fx: f32, fy: f32, fz: f32)` $\longrightarrow$ `$Vector3.scale$ref_Vector3_f32_f32_f32`

---

## 6. Prohibitions & Invariants

1. **Anti-Nesting Invariant**:
   Nesting an `@fa:fn_overload` directive block inside another `@fa:fn_overload` directive block is strictly prohibited and triggers a compile-time error (`syn/nested-fn-overload`).
2. **No Return-Type-Only Overloading**:
   ```lone
   -- FORBIDDEN (Compile-time error: type/overload-duplicate-signature)
   @fa:fn_overload {
       fn parse(s: str) i32 { ... }
       fn parse(s: str) f32 { ... }
   }
   ```
   *Rationale*: Call sites like `let x = parse("123")` or `drop parse("123")` cannot infer the expected type without complex bidirectional typing, introducing ambiguity.
3. **Export Restrictions**:
   Exported WASI/C ABI functions (`pub fn main()`, `@fa:wasm` host exports) cannot be overloaded unless explicitly given unique link names.
4. **No Unmarked Clashing**:
   Declaring multiple functions with the same name without the `@fa:fn_overload` directive remains a strict compile error (`sym/duplicate-symbol`).
