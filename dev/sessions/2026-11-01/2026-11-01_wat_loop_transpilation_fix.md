# Report: WebAssembly Loop Transpilation Architecture & Fix

**Date**: 2026-11-01  
**Author**: AI Assistant & Lone Team  
**Branch**: `feat/better-type`  
**Tags**: `[Ai-assisted]`, `falone`, `wat`, `wasm`, `loops`, `control-flow`

---

## 1. Overview & Problem Diagnosis

In WebAssembly, a `block` label represents forward-only branching: branching to a `block` (`br $block`) jumps *out* to the end of the block. A `block` is therefore not a loop.

To implement a `while` loop correctly in WebAssembly Text format (WAT), the standard structure combines an outer `block` with an inner `loop`:
```wat
(block $while_exit
  (loop $while_head

    ;; Evaluate condition
    local.get $counter
    global.get $max_retries
    i32.lt_s

    ;; Exit if condition is false
    i32.eqz
    br_if $while_exit

    ;; Body
    ...

    ;; Repeat
    br $while_head
  )
)
```

The initial Falone transpiler nested all basic blocks inside standard forward `block` constructs, which incorrectly prevented backward branching for loops.

---

## 2. Changes & Implementation

### 2.1 MIR Loop Metadata (`packages/falone/mirtypes/types.go`)
- Updated `MirBlock` to include loop metadata:
  - `IsLoop bool`: Flags whether the block is a loop header.
  - `LoopExit string`: Identifies the label of the loop exit block.

### 2.2 Lowering Pipeline Loop Tracking (`packages/falone/lowering/lower.go`)
- In `TWhileStmt`:
  - `headBlk.IsLoop = true`
  - `headBlk.LoopExit = exitBlk.Name`
  - Added `loopExitStack []string` on `lowerer` to accurately resolve `TBreakStmt` targets to the innermost `LoopExit` block (`jump while_exit`).
- Fixed comparison operator string mapping (`"<"` -> `lt`, `"<="` -> `le`, `">"` -> `gt`, `">="` -> `ge`) avoiding inverted constant lookups.

### 2.3 WebAssembly Transpiler (`packages/falone/transpile/wat.go`)
- Implemented `emitFunctionBlocks` with specialized loop handling:
  - Wraps the loop in `(block $<while_exit> (loop $<while_head> ...))`.
  - Inverts the loop condition with `i32.eqz` followed by `br_if $<while_exit>` to exit when false.
  - Translates `break` statements to `br $<while_exit>`.
  - Repeats the iteration with `br $<while_head>` at the end of the body.
  - Resumes execution after the loop at the `<while_exit>` block.

---

## 3. Verification

- Added `TestTranspileWhileLoopToWat` in `packages/falone/transpile/wat_test.go` asserting presence of:
  - `(block $while_exit ...)`
  - `(loop $while_head ...)`
  - `i32.eqz`
  - `br_if $while_exit`
  - `br $while_head`
- Verified WAT output on `valid.lone`:
  ```wat
  (block $while_exit_3
    (loop $while_head_1
      ;; Evaluate condition
      local.get $v_loop_count5
      global.get $max_retries
      i32.lt_s
      local.set $t_bin6
      ;; Exit if condition is false
      local.get $t_bin6
      i32.eqz
      br_if $while_exit_3

      ;; Body
      local.get $v_loop_count5
      i32.const 1
      i32.add
      local.set $t_bin7
      br $while_exit_3
      ;; Repeat
      br $while_head_1
    )
  )
  ```
- All 22 test suites across the repository (`lone-lang`, `lone-alyzer`, `falone`) pass.
