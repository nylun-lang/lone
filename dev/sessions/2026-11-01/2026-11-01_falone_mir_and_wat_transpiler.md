# Report: Falone Assembly (MIR) and WebAssembly (WAT) Transpiler

**Date**: 2026-11-01  
**Author**: AI Assistant & Lone Team  
**Branch**: `feat/better-type`  
**Tags**: `[Ai-assisted]`, `falone`, `compiler`, `mir`, `assembly`, `wat`, `wasm`

---

## 1. Overview & Objectives

Following the stabilization of `lone-alyzer`'s Typed IR, this work establishes the core of the official Lone compiler, **`Falone`** (`packages/falone`):
1. **Falone MIR (Mid-level Intermediate Representation / Assembly)**: An explicit register-based, basic-block control flow graph representation specified in `packages/falone/syntax.txt`.
2. **Lowering Pipeline (`lowering`)**: Translates `lone-alyzer`'s Typed IR (`air.TProgram`) into Falone MIR (`MirModule`).
3. **WebAssembly Transpiler (`transpile`)**: Transpiles Falone MIR directly into clean WebAssembly Text format (`WAT`).
4. **Compiler CLI (`cmd`)**: An end-to-end driver that accepts Lone source code and emits both Falone MIR assembly and WAT output.

---

## 2. Architecture & Components

```
                Lone Source (.lone)
                        │
                        ▼ (lonelang/parser)
                    Parsed AST
                        │
                        ▼ (lonealyzer/analyze)
                     Typed IR
                        │
                        ▼ (falone/lowering)
                    Falone MIR
                 (Assembly Blocks)
                        │
                        ▼ (falone/transpile)
             WebAssembly Text (WAT)
```

### 2.1 Falone MIR Specification & Types (`mirtypes/`)
- **Types (`MirT`)**: `MirI32`, `MirI64`, `MirF32`, `MirF64`, `MirVoid`.
- **Operands (`MirOperand`)**: Virtual registers (`%r1`, `%result`), constants (`1`, `42`), and global variables (`$max_retries`).
- **Instructions (`MirInst`)**:
  - Binary arithmetic: `add`, `sub`, `mul`, `div` (e.g. `%r3 = add i32 %a, %b`)
  - Comparisons: `cmp.lt`, `cmp.gt`, `cmp.eq`, `cmp.le`, `cmp.ge`, `cmp.ne`
  - Constants & Copies: `%r = const i32 10`, `%dest = %src`
  - Calls: `[%dest =] call @callee <args...>`
- **Terminators (`MirTerminator`)**:
  - `return [%val]`
  - `jump <label>`
  - `branch %cond, <then_label>, <else_label>`
- **Basic Blocks (`MirBlock`)**: Label name, instruction slice, terminator.
- **Assembly Formatter (`printer.go`)**: Renders human-readable Falone assembly exactly per `packages/falone/syntax.txt`.

### 2.2 Lowering Pipeline (`lowering/lower.go`)
- Converts `air.TProgram` declarations into `MirModule`.
- Functions lower to basic blocks with explicit control flow:
  - Statements generate copy, call, and arithmetic instructions.
  - If/Else constructs lower into conditional branches (`branch`), then-blocks, else-blocks, and join-blocks (`jump`).
  - While loops lower into header check blocks, body blocks, and loop exit jumps.

### 2.3 WAT Transpiler (`transpile/wat.go`)
- Emits WebAssembly Text module:
  - Exports memory and `main` function.
  - Generates parameter, result, and local variable declarations for all allocated registers.
  - Implements structured control flow for basic blocks (`block`, `loop`, `br`, `br_if`).
  - Maps binary instructions to corresponding WebAssembly opcodes (`i32.add`, `i32.sub`, `i32.mul`, `i32.div_s`, `i32.lt_s`, etc.).

### 2.4 Compiler CLI (`cmd/main.go`)
- Supports running from the terminal:
  - `go run ./packages/falone/cmd/main.go -mir <file.lone>` (Prints Falone MIR)
  - `go run ./packages/falone/cmd/main.go -wat <file.lone>` (Prints WAT)
  - `go run ./packages/falone/cmd/main.go -o out.wat <file.lone>` (Writes WAT file)
  - Default prints both MIR assembly and WAT.

---

## 3. Verification

- `cd packages/falone && go test -v ./...` -> **PASS**
  - `TestLowerFunctionToMir`: Verified Typed IR translation to Falone basic blocks.
  - `TestTranspileSyntaxExampleToWat`: Verified transpilation of `syntax.txt` specification to WAT.
- End-to-end CLI tested against `./packages/lone-alyzer/sample/valid.lone` -> Emits valid MIR assembly and WAT code.
- Workspace test run: **21 / 21 test suites passing** across `lone-lang`, `lone-alyzer`, and `falone`.
