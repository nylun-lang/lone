# Milestone Report: WebAssembly Linear Memory Management & Free-List Allocator Runtime

**Date:** November 4, 2026  
**Branch:** `reimpl`  
**Commit:** `ae0ab03`  
**Test Suite:** 22/22 Go packages passing (100% pass rate)

---

## 1. Overview & Objectives

This milestone established the core **WebAssembly Linear Memory Management** architecture for Greenfield Lone (`pkg2`):
1. **Memory Partitioning & Isolation**:
   - **Page 0 (0 .. 65,535)**:
     - `0 .. 63`: Reserved runtime scratchpad / WASI I/O buffer.
     - `64 .. dataEnd`: Static string literals and compile-time constants.
     - `dataEnd .. 65,535`: Downward-growing execution datastack (`$__stack_pointer`).
     - `$__stack_limit`: Stack underflow guard threshold (prevents stack from corrupting static data).
   - **Page 1+ (65,536 .. memory.size * 65,536)**:
     - Dynamic Heap: Upward-growing heap managed by a first-fit Free-List Allocator.
     - Dynamic Memory Expansion: Automatic `memory.grow` invocations when heap capacity is exceeded.
   - **Zero Collision Guarantee**: Because stack grows downward in Page 0 and heap grows upward starting at Page 1, stack and heap never collide.
2. **First-Fit Free-List Allocator (`$__lone_rt_alloc`, `$__lone_rt_free`, `$__lone_rt_realloc`)**:
   - 8-byte aligned block headers tracking block payload size and allocation status.
   - Address-sorted free list supporting bi-directional coalescing (merging adjacent free blocks both left and right).
   - Splitting of large free blocks when residual capacity meets minimum header/payload thresholds.
3. **Linear Memory Intrinsics**:
   - `#alloc(size: int) int`: Dynamic heap allocation via free-list runtime.
   - `#free(ptr: int) void`: Return block to free-list with automatic adjacent coalescing.
   - `#realloc(ptr: int, new_size: int) int`: Resizing with in-place reuse or alloc-copy-free.
   - `#memory_grow(pages: int) int`: Explicit WebAssembly `memory.grow`.
   - `#memory_size() int`: Current WebAssembly memory page count.
   - `#stack_pointer() int`: Current linear stack pointer offset.
   - `#stack_alloc(size: int) int`: Bump datastack allocation with stack limit guard check.
   - `#stack_free(size: int) void`: Constant-time stack rewind.
4. **Integration with Language Features**:
   - Enum constructor lowering (`lowerEnumConstructExpr`) migrated from raw bump allocation to `$__lone_rt_alloc`.

---

## 2. Key Architecture & Changes

### 2.1 Compiler AST (`pkg2/lone-compiler-wir/ast.go`)
- Added `InstMemoryGrow` (`memory.grow`).
- Added `InstMemorySize` (`memory.size`).

### 2.2 Analyzer Built-in Intrinsics (`pkg2/lone-analyzer/scope.go`)
- Registered function symbols: `#alloc`, `#free`, `#realloc`, `#memory_grow`, `#memory_size`, `#stack_pointer`, `#stack_alloc`, and `#stack_free`.

### 2.3 WIR Lowering & Inlined Runtime (`pkg2/lone-compiler-wir/lower.go`)
- **Globals Emitted**:
  - `$__stack_pointer (mut i32)`: Initialized to `heapBase` (e.g. 65,536).
  - `$__stack_limit (mut i32)`: Set to 16-byte aligned `dataEnd` (minimum 1,024).
  - `$__heap_base i32`: Initialized to `heapBase`.
  - `$__heap_top (mut i32)`: Heap allocation cursor initialized to `heapBase`.
  - `$__free_list_head (mut i32)`: Head of address-sorted free list.
  - `$__heap_pointer (mut i32)`: Backwards-compatible alias.
- **Inlined Runtime Implementations**:
  - `$__lone_rt_memcpy`: 4-byte word and 1-byte trailing loop memory copy.
  - `$__lone_rt_memset`: Byte-level memory fill.
  - `$__lone_rt_alloc`: First-fit free block search, block splitting, heap top bump, and dynamic `memory.grow`.
  - `$__lone_rt_free`: Address-sorted insertion with immediate left and right coalescing.
  - `$__lone_rt_realloc`: In-place reuse check, expansion, memory copy, and old block reclamation.
- **Enum Construction**:
  - Updated `lowerEnumConstructExpr` to allocate variant memory via `$__lone_rt_alloc` and store tags/fields cleanly using `InstLocalSet`.
- **Intrinsic Dispatch**:
  - Lowered `#alloc`, `#free`, `#realloc`, `#memory_grow`, `#memory_size`, `#stack_pointer`, `#stack_alloc`, and `#stack_free` in `lowerCallExpr`.

---

## 3. Verification & Testing

### 3.1 Allocator Verification Scratch Testing
- Validated block splitting, reallocations, and adjacent block coalescing via direct `wasmtime` invocation.

### 3.2 Behavior Execution Test (`test2/behavior/linear_memory.lone`)
- End-to-end Wasm runtime test verifying:
  - Initial memory size ($\ge 2$ pages).
  - Datastack allocation (`#stack_alloc`) and rewind (`#stack_free`).
  - Heap allocation of consecutive blocks `p1`, `p2`, `p3`.
  - Deallocation of `p1` and `p2`, verifying address-sorted coalescing into a single block.
  - Verification that allocating 120 bytes reuses the coalesced block at `p1`.
  - Reallocation of merged block to 200 bytes.
  - Dynamic `memory.grow` expanding memory by 2 pages.
  - Full output assertion verified via `wasmtime`.

### 3.3 Full Test Suite Pass Rate
All 22 packages across `pkg`, `pkg2`, and `test2` (including all 17 behavior tests) pass with 100% success:
```bash
go test ./...
# PASS across all 22 packages
```
