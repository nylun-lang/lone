---
title: "Linear Memory & Allocator"
order: 5
---

# WebAssembly Linear Memory & Allocator

Lone targets standard WebAssembly (Wasm) linear memory using an explicit two-tier architecture designed for deterministic memory safety and zero runtime overhead.

---

## Linear Memory Architecture

Lone divides WebAssembly's 64 KiB linear memory pages into dedicated, non-overlapping zones:

```
+--------------------+ 0x00000000 (0)
| Reserved Scratch   | WASI iovec & system buffers (0..63)
+--------------------+ 0x00000040 (64)
| Static Constants   | String literals & metadata tables (64..dataEnd)
+--------------------+ dataEnd
| Stack Guard Page   | $__stack_limit (minimum 1024)
|                    | 
| Datastack Area     | Downward-growing stack ($__stack_pointer starts at 65536)
+--------------------+ 0x00010000 (65536, $__heap_base)
| Dynamic Heap       | Upward-growing heap managed by Free-List Allocator
|                    | Automatic dynamic expansion via memory.grow
+--------------------+ memory.size * 65536
```

### Zero-Collision Guarantee
Because the datastack grows downward within Page 0 and the dynamic heap begins at Page 1 growing upward, the stack and heap can never collide.

---

## Page 0: The Datastack (Shadow Stack)

Functions allocating local structs or taking raw addresses (`&raw`) reserve frame memory on the datastack:

1. **Prologue**: Decrements `$__stack_pointer` by the required frame size.
2. **Overflow Check**: Validates that `$__stack_pointer` remains above `$__stack_limit`.
3. **Structure Return (SRET)**: Functions returning large structs receive a caller-allocated stack slot as an implicit pointer argument (`sret_ptr`), avoiding unnecessary copies.
4. **Epilogue / Zero-Cost Rewind**: Before returning, `$__stack_pointer` is incremented back to its entry position, instantly reclaiming all stack frame memory without garbage collection or reference counting overhead.

---

## Page 1+: Dynamic Free-List Allocator

Dynamic allocations (such as tagged union enums, dynamic lists, and `#alloc`) are managed by Lone's inlined WebAssembly Free-List allocator runtime:

### 1. 8-Byte Block Headers
Each heap block is prefixed by an 8-byte header:
- **`size` (4 bytes)**: Size of the payload in bytes (aligned to 4 bytes).
- **`status` (4 bytes)**: `0` indicates `FREE`, `1` indicates `ALLOCATED`.

### 2. Allocation Strategy (First-Fit with Splitting)
`$__lone_rt_alloc(size)` traverses the free list from `$__heap_base`:
- Locates the first free block with sufficient capacity.
- If the remaining space after allocation is $\ge 8$ bytes, the block is split into an allocated block and a residual free block.

### 3. Constant-Time Bi-Directional Coalescing
When `#free(ptr)` is invoked:
- The block's status is reset to `0` (`FREE`).
- The allocator immediately inspects adjacent blocks to both the right and left.
- Adjacent free blocks are merged in constant time, preventing heap fragmentation.

### 4. Dynamic Expansion (`memory.grow`)
When available free blocks cannot satisfy an allocation request, `$__lone_rt_alloc` automatically invokes WebAssembly `memory.grow` to expand linear memory before allocating from the newly available page.

---

## References (`ref<T>`) vs. Raw Pointers (`ptr<T>`)

- **`ref<T>` (Safe Managed Reference)**: Safe reference created via `&x`. Immutable by default (`ref<T>`), or explicitly mutable when bound as `ref<var T>`. Cannot escape its enclosing stack scope.
- **`ref<[]T>` (Fat Slice Reference)**: Encapsulates a pointer and length for safe contiguous bounds-checked access.
- **`ptr<T>` (Raw Unsafe Pointer)**: Direct WebAssembly linear memory address (`i32`) created via `&raw x`. Supports raw memory indexing `ptr[i]` and offset arithmetic.
- **`ptr<[N]T>` (Sized Buffer Pointer)**: Typed pointer to a fixed-size buffer of `N` contiguous elements in linear memory, facilitating zero-copy buffer operations and WASI syscall integration.
