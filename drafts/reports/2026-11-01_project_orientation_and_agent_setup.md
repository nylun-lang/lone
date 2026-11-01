# Project Orientation & Agent Setup Report

- **Date**: 2026-11-01
- **Author**: Antigravity (AI Assistant)
- **Status**: Completed

---

## 1. Executive Summary

This report documents the orientation process for the **Lone Programming Language** repository, the population of `AGENT.md`, and the establishment of repository conventions for future interactions. During inspection, a workspace module path discrepancy in `go.work` was also resolved, confirming that the current PEG-based parser executes successfully on sample Lone code.

---

## 2. Project Understanding

### 2.1 What is Lone?
Lone is an explicit, systems- and UI-oriented programming language compiling to **WebAssembly (Wasm)** as its primary target. It synthesizes design philosophies from Go, TypeScript, Python, and Rust/Zig with a foundational goal:
> *"Easy to read, Easy to write, and Hard to misunderstand."*

### 2.2 Core Invariants
- **IL1 (Explicit over implicit)**: All references are unambiguous regarding identifier source and identity.
- **IL2 (No implicit built-ins)**: Built-in symbols, functions, or compiler intrinsics are prefixed (e.g. `#print`, `#init`), ensuring clear distinction from user definitions.
- **IF1 (Syntax-level feedback)**: Punctuation supports clarity without relying on punctuation gymnastics.
- **IF2 (Progressive information)**: Information flows progressively from left to right.
- **IUI1 (Declared topology is stable)**: Compiler-level awareness of stable UI topologies for UI rendering.

### 2.3 Repository Map
- **`packages/lone-lang`**: Go-based parser and intermediate representation (`irtypes`), utilizing Pigeon for PEG parsing.
- **`packages/falone`**: The upcoming Lone compiler targeting WebAssembly.
- **`packages/lone-alyzer`**: Static analysis and language tooling.
- **`drafts/research/`**: Design sketches, language specifications (`SPEC.md`), and proposals for what Lone could become.
- **`drafts/under-the-hood/`**: Technical documentation describing how system components operate under the hood.
- **`drafts/reports/`**: Task completion and milestone reports.
- **`_old/`**: Historical and legacy prototypes, specs, and code. Kept for reference.
- **`assets/`**: Visual assets, logos, and banners.

---

## 3. Actions Taken

1. **Codebase & Architecture Analysis**:
   - Reviewed `drafts/research/SPEC.md`, `README.md`, `_old/` history, and `packages/lone-lang/parser/parser.peg`.
   - Inspected `irtypes` (expressions, declarations, type references, type constraints).

2. **Go Workspace Fix & Verification**:
   - Identified that `go.work` referenced `./packages/lonelang` while the directory on disk is `./packages/lone-lang`.
   - Updated `go.work` to point to `./packages/lone-lang`.
   - Verified functionality by running `go run ./packages/lone-lang/cmd/parse.go ./packages/lone-lang/samples/misc.lone`, which successfully parsed variable declarations and type constraints into JSON IR.

3. **Populated `AGENT.md`**:
   - Outlined project overview, invariants, and directory responsibilities.
   - Formalized operational rules:
     - All feature commits must be tagged `[Ai-assisted] ...`.
     - End of requests must conclude with a report in `drafts/reports/`.
     - `drafts/under-the-hood/` is reserved for internal architectural deep-dives.
     - `drafts/research/` is designated for forward-looking design drafts and ideas.
     - `_old/` is designated as read-only legacy reference.
   - Documented build workflows, pigeon PEG generation commands, and syntax style rules.

---

## 4. Next Steps & Recommendations

- **Parser Expansion**: Continue grammar coverage in `packages/lone-lang/parser/parser.peg` (functions, struct declarations, records, control flow, UI blocks).
- **Test Suite**: Establish structured test suites in `packages/lone-lang` comparing parsed ASTs against expected outputs.
- **Compiler Inception (`packages/falone`)**: Set up the initial translation pipeline from `irtypes` to Wasm AST.
