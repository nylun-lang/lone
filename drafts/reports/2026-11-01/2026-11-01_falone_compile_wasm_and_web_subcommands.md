# Implementation Report: `falone compile wasm` and `falone compile web` Subcommands

**Date**: 2026-11-01  
**Branch**: `feat/better-type`  
**Scope**: `packages/falone/cmd`

---

## 1. Executive Summary

Added dedicated subcommands under `compile` in the Falone CLI:
1. **`falone compile wasm <file.lone>`**:
   - Aliases: `wasi`, `standalone`
   - Explicitly compiles Lone code targeting standalone WebAssembly with the WASI snapshot preview 1 system interface.
   - Emits WASI `fd_write` import and inlines `$__lone_rt_print`.
2. **`falone compile web <file.lone>`**:
   - Aliases: `browser`, `js`
   - Explicitly compiles Lone code targeting web/browser execution.
   - Emits `(import "env" "print" (func $__lone_rt_print (param i32 i32)))`.
3. **`falone compile <file.lone>`**:
   - Retained as a shorthand that defaults to standalone WebAssembly (`wasm`), or accepts `--target` / `-t` flag.
4. **Persistent Flags**:
   - `--mir` (`-m`), `--wat` (`-w`), `--output` (`-o`) are persistent flags on `compileCmd` and inherited by both `wasm` and `web` subcommands.

---

## 2. CLI Usage Examples

```bash
# Compile for standalone WASI
falone compile wasm sample.lone -o sample.wat

# Compile for web browsers
falone compile web sample.lone -o sample.wat

# Shorthand (defaults to wasm)
falone compile sample.lone
```

---

## 3. Test Verification

All workspace test suites continue to pass:
- `lone-lang`: 12 test suites passing
- `lone-alyzer`: 8 test suites passing
- `falone`: 5 test suites passing
