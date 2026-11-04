---
title: "Built-in Intrinsics"
order: 6
---

# Built-in Intrinsics

Intrinsics in Lone are primitive operations baked directly into the compiler, prefixed with `#` to clearly distinguish them from user-defined functions (**IL2 - No implicit built-ins**).

---

## Output & Diagnostics

### `#print(msg: str) void`
Outputs a UTF-8 string directly to standard output via WebAssembly linear memory:
```rust
#print("Hello from Lone!\n")
```

### `#panic(msg: str) void`
Prints an error diagnostic string and immediately aborts execution by emitting WebAssembly `unreachable`:
```rust
#panic("Critical assertion failed!\n")
```

---

## Dynamic Heap Memory Intrinsics

These intrinsics directly interface with the inlined Free-List allocator runtime on WebAssembly linear memory:

### `#alloc(size: int) int`
Allocates a block of `size` bytes on the dynamic heap (Page 1+) and returns the 32-bit linear memory pointer:
```rust
var buf_ptr = #alloc(256)
```

### `#free(ptr: int) void`
Frees a previously allocated block at `ptr` and triggers bi-directional coalescing with adjacent free blocks:
```rust
#free(buf_ptr)
```

### `#realloc(ptr: int, new_size: int) int`
Resizes the allocated block at `ptr` to `new_size` bytes. If sufficient space is available in-place or in an adjacent right-hand free block, it expands without copying; otherwise, it allocates a new block, copies existing payload data, and frees the old block:
```rust
buf_ptr = #realloc(buf_ptr, 512)
```

### `#memory_grow(pages: int) int`
Executes the WebAssembly `memory.grow` instruction, expanding linear memory by the given number of 64 KiB pages. Returns the previous page count, or `-1` on failure:
```rust
var prev_pages = #memory_grow(2)
```

### `#memory_size() int`
Returns the current number of 64 KiB WebAssembly linear memory pages:
```rust
var total_pages = #memory_size()
```

---

## Datastack (Shadow Stack) Intrinsics

These intrinsics inspect and manage the downward-growing execution datastack in Page 0:

### `#stack_pointer() int`
Returns the current linear memory byte offset of `$__stack_pointer`:
```rust
var sp = #stack_pointer()
```

### `#stack_alloc(size: int) int`
Allocates `size` bytes by bumping the datastack downwards with guard-page bounds verification, returning the pointer to the allocated frame memory:
```rust
var local_buf = #stack_alloc(64)
```

### `#stack_free(size: int) void`
Rewinds the datastack upwards by `size` bytes in constant time:
```rust
#stack_free(64)
```
