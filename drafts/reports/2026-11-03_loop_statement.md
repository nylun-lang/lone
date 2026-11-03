# Lone Infinite Loop Statement Implementation

**Date:** 2026-11-03  
**Status:** Completed  
**Branch:** `feat/import-and-std-io`

---

## 1. Executive Summary

Implemented the `loop { ... }` infinite loop language statement in Lone:
- Provides clean, idiomatic infinite loop syntax (`loop { ... }`), replacing verbose `while true { ... }`.
- Terminates via standard `break` or `return` statements.
- Supports labeling for targeted breaks from nested loops: `outer: loop { break outer: }`.
- Desugars 1:1 in AST into an `ir.WhileStmt` with constant boolean `true` condition, executing with full native loop optimizations across MIR, WIR, and WebAssembly bytecode generation.

---

## 2. Syntax & Desugaring

### Syntax
```lone
var count = 0
loop {
    count = count + 1
    println_int(count)
    if count >= 3 {
        break
    }
}
```

### Desugaring
Desugars in the parser to:
```lone
while true {
    count = count + 1
    println_int(count)
    if count >= 3 {
        break
    }
}
```

---

## 3. Changes Implemented

1. **`pkg/lone-parser/parser.peg` & `generated-parser.go`**:
   - Added `KW_LOOP <- "loop" ![a-zA-Z0-9_]` to keywords.
   - Added rule `LOOP_STMT <- KW_LOOP _ body:BLOCK`.
   - Included `LOOP_STMT` in `TOP_LEVEL`, `STMT`, and `LABELABLE`.
   - Allowed `LABEL_IDENT` in `BREAK_LABEL` and `LABEL_DEC` for contextual backward-compatibility with label names named `loop:`.
   - Regenerated parser using Pigeon.

2. **`pkg/lone-compiler-mir/lower.go`**:
   - Added support for lowering `*loneanalyzer.TLabelDec` statements to basic blocks and exit tracking.

3. **`extra/ovsx-lsp/syntaxes/lone.tmLanguage.json`**:
   - Added `loop` to syntax highlighting control keywords.

4. **Specifications**:
   - Updated `drafts/SPEC.md` section 2.2 (Keywords) and added section 6.4.1 (Infinite Loop).

5. **Testing**:
   - Added `TestLoopStmt` in `pkg/lone-parser/parser_test.go` covering basic and labeled loop parsing.
   - Added `TestLoopWasmtime` in `pkg/lone-compiler/std_io_e2e_test.go` verifying execution of loop counting with `break` through Wasmtime.

---

## 4. Verification

All tests across the entire repository pass:
```bash
go test -count=1 ./...
```
Output:
```text
ok  	lone/pkg/lone-analyzer	0.063s
ok  	lone/pkg/lone-analyzer-diag	0.008s
ok  	lone/pkg/lone-analyzer-project	0.649s
ok  	lone/pkg/lone-compiler	0.460s
ok  	lone/pkg/lone-compiler-adapter	0.003s
ok  	lone/pkg/lone-compiler-mir	0.014s
ok  	lone/pkg/lone-compiler-wir	0.006s
ok  	lone/pkg/lone-lsp	0.069s
ok  	lone/pkg/lone-parser	0.058s
ok  	lone/pkg/lone-runtime	0.008s
ok  	lone/pkg/lone-shared-logger	0.006s
```
