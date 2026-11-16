---
title: "Built-in Intrinsics"
order: 5
---

# Built-in Intrinsics

Intrinsics in Lone are primitive operations baked into the compiler that provide direct access to runtime facilities without external library overhead.

## The `#print` Intrinsic

Lone provides `#print` as a built-in intrinsic for string output:

```rust
fn main() void {
    #print("Hello from Lone!\n")
}
```

### Signature
`#print(str: str) void`

Because `#print` returns `void`, calling it as a statement does not require the `drop` keyword.

## Memory Lowering

When `#print` is compiled:
1. The string literal `"Hello from Lone!\n"` is placed into linear memory as a static data segment.
2. Identical strings are automatically deduplicated.
3. In Falone MIR, `#print` is lowered into:
   ```text
   call @__lone_rt_print %ptr %len
   ```
4. In WebAssembly:
   - For WASI target: inlines an `fd_write` syscall helper using linear memory scratchpad.
   - For Web target: imports `env.print(ptr, len)`.
