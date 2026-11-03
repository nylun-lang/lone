# Work Session Report: Compiler Directive Migration (`@fa:name`)

**Date**: 2026-11-03  
**Branch**: `feat/import-and-std-io`  
**Topic**: Migrate Compiler Directives from Magic Comments (`--fa:wasm`) to First-Class Syntax (`@fa:wasm`)

---

## 1. Motivation & Summary

Previously, compiler directives were encoded inside comments (e.g. `---fa:wasm ... ---` or `--fa:wasm`). This violated Lone's core design philosophy:
- **IL1 (Explicit over implicit)**: Comments should be for humans, not machine semantics. Code formatters and linters should be able to touch comments without altering compilation.
- **Tooling & Editor Ergonomics**: Editors treated directives as grayed-out comments rather than active compiler directives.

Compiler directives have been promoted to first-class syntax prefixed with `@` and namespaced under `fa:` (Falone compiler engine).

### Syntax Forms

1. **Block Directive**:
   ```lone
   @fa:wasm {
       import .fd_write, .fd_read from :wasi:snapshot_preview1
   }
   ```
2. **Statement Directive**:
   ```lone
   @fa:wasm import .proc_exit from :wasi:snapshot_preview1
   ```
3. **Inline / Capability Directive**:
   ```lone
   @fa:wasm
   ```

*(Legacy syntax `---fa:wasm ... ---` and `--fa:wasm` is preserved in the parser for backward compatibility).*

---

## 2. Modified Files

- `pkg/lone-parser-ast/dec.go`: Added `Name string` to `ir.WasmDirective`.
- `pkg/lone-parser/parser.peg`: Added rules `FA_DIRECTIVE`, `FA_DIRECTIVE_BLOCK`, `FA_DIRECTIVE_STMT`, `FA_DIRECTIVE_BARE`.
- `pkg/lone-parser/generated-parser.go`: Regenerated PEG parser with pigeon.
- `pkg/lone-parser/parser_test.go`: Added `TestFaCompilerDirectives` verifying block, statement, and bare directive forms.
- `libraries/std/sys.lone`: Migrated WASI host import declarations from `---fa:wasm` to `@fa:wasm { ... }`.
- `pkg/lone-analyzer-project/resolver.go` & `eval_type.go`: Updated diagnostic error messages to reference `@fa:wasm`.
- `pkg/lone-analyzer-project/resolver_test.go`: Added test case verifying `@fa:wasm` capability authorization.
- `extra/ovsx-lsp/syntaxes/lone.tmLanguage.json`: Added TextMate rule for `@fa:name` directives (`keyword.control.directive.lone`).
- `extra/nvim-lsp/syntax/lone.vim`: Added Vim syntax match and highlight (`PreProc`) for `@fa:name` directives.
- `drafts/SPEC.md`: Updated Section 2.4 to document `@fa:name` compiler directives.

---

## 3. Verification & Test Results

All packages and end-to-end tests pass cleanly:
```bash
go test -count=1 ./...
# ok  	lone/pkg/lone-analyzer	0.077s
# ok  	lone/pkg/lone-analyzer-diag	0.020s
# ok  	lone/pkg/lone-analyzer-project	0.538s
# ok  	lone/pkg/lone-compiler	0.759s
# ok  	lone/pkg/lone-compiler-adapter	0.005s
# ok  	lone/pkg/lone-compiler-mir	0.013s
# ok  	lone/pkg/lone-compiler-wir	0.009s
# ok  	lone/pkg/lone-lsp	0.129s
# ok  	lone/pkg/lone-parser	0.049s
# ok  	lone/pkg/lone-runtime	0.005s
# ok  	lone/pkg/lone-shared-logger	0.003s
```
