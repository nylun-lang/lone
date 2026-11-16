# Work Session Report: Guessing Game Example & Stdlib Numeric I/O Extensions

**Date**: 2026-11-03  
**Branch**: `feat/import-and-std-io`  
**Tags**: `[Ai-assisted]`, `example`, `guessing-game`, `std:io`, `random`, `parse_int`, `print_int`

---

## 1. Request Overview

The user requested:
> "ok ok ok ok ok, let's put it to the test in exemple impl a guessing game in lone"

This exercised the complete pure-Lone toolchain end-to-end:
- Loading modules from `:std:io` (`print`, `println`, `read_line`, etc.)
- Constructing structs and calling instance methods (`Game.new(...)`, `game.check(guess)`)
- WASI Preview 1 random number generation (`random_get` via `:std:sys`)
- Interactive stdin input parsing (`read_line()`, `parse_int()`)
- Formatted integer output (`print_int()`, `println_int()`, `int_to_str()`)
- Game state tracking and control flow (`while`, `if`/`else`, `break`, early surrender option)

---

## 2. Changes Made

### 2.1 Standard Library Enhancements (`libraries/std/`)
- **System Bridge ([sys.lone](../../libraries/std/sys.lone))**:
  - Bound `:wasi:snapshot_preview1` `random_get` under the `---fa:wasm` directive.
  - Implemented `pub fn sys_random(buf: ptr<any>, len: i32) i32`.
  - Updated [pkg/lone-runtime/host.go](../../pkg/lone-runtime/host.go) to parameterize `random_get`'s buffer with `ptr<any>`.
- **Numeric & Stream I/O ([io.lone](../../libraries/std/io.lone), [std.lone](../../libraries/std/std.lone))**:
  - `pub fn parse_int(s: str) i32`: Parses signed/unsigned integers from strings, skipping leading whitespace and handling optional `+`/`-`.
  - `pub fn int_to_str(n: i32) str`: Converts an `i32` to a null-terminated heap-allocated string (`#heap_alloc` + `#ptr_to_str`).
  - `pub fn print_int(n: i32) void` and `pub fn println_int(n: i32) void`: Writes integers directly to standard output.
  - `pub fn random_i32() i32`: Returns a 32-bit random signed integer via WASI `random_get`.
  - `pub fn random_range(min: i32, max: i32) i32`: Returns an integer uniformly distributed in `[min, max]`.

### 2.2 Grammar & Lowering Robustness (`pkg/lone-parser/`, `pkg/lone-compiler-mir/`, `pkg/lone-analyzer/`)
- In [parser.peg](../../pkg/lone-parser/parser.peg), updated bitwise `&` and `|` to require negative lookahead `!"&"` and `!"|"` so that compound boolean operators (`&&`, `||`) or unary address-of operators (`&foo`) are not accidentally split.
- Added support for `&&` and `||` in `LOGICAL_AND_BINARY_EXPR` and `LOGICAL_OR_BINARY_EXPR` alongside native `and` and `or`.

### 2.3 Number Guessing Game ([exemple/guess.lone](../../exemple/guess.lone))
- Authored the complete interactive guessing game in `exemple/guess.lone`:
  - `type Game = struct { secret: i32, attempts: i32, max_val: i32 }`
  - Associated constructor `Game.new(max_num: i32) Game` using `random_range(1, max_num)`
  - Instance method `fn check(self, guess: i32) i32` returning `-1` (too low), `1` (too high), or `0` (correct)
  - Interactive loop reading guesses via `read_line()` and `parse_int()`
  - Handles surrender (`0`), counts attempts, and awards skill rankings based on attempt count.

---

## 3. Verification & Testing

1. **Workspace Tests**:
   - `go test -count=1 ./...` passed **100%** across all packages.
2. **End-to-End Tests ([std_io_e2e_test.go](../../pkg/lone-compiler/std_io_e2e_test.go))**:
   - `TestGuessingGameExampleE2EWasmtime`: Compiles `exemple/guess.lone` and runs it with piped stdin `0\n` in `wasmtime`, verifying prompt, surrender handling, and clean process exit.
   - `TestParseIntAndIntToStrE2EWasmtime`: Verifies `parse_int` with negative numbers and whitespace, integer arithmetic, and `int_to_str` formatting in `wasmtime`.
3. **Live Execution (`lone run`)**:
   - Tested running `go run ./cmd/lone run exemple/guess.lone` with piped inputs `50\n75\n0\n`:
     ```text
     ==========================================
         Welcome to the Lone Guessing Game!    
     ==========================================
     I have chosen a secret number between 1 and 100.
     Can you guess what it is? (Enter 0 to surrender)

     Your guess: -> Too low! Try a higher number.

     Your guess: -> Too low! Try a higher number.

     Your guess: Giving up? The secret number was: 97
     Better luck next time!
     ```
