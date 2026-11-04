# AGENT.md - AI Operating Guide for Lone

Welcome to the **Lone Programming Language** repository. This guide defines project context, architectural guidelines, directory structures, and mandatory operating conventions for AI assistants collaborating on this codebase.

---

## 1. Project Overview & Philosophy

**Lone** is an explicit, systems- and UI-oriented programming language designed to compile to **WebAssembly (Wasm)** as its primary target. It draws inspiration from Go, TypeScript, Python, and Rust/Zig.

### Core Philosophy
> *"Easy to read, Easy to write, and Hard to misunderstand."*

### Key Language Invariants
- **IL1 - Explicit over implicit**: Every reference must be unambiguous about *what it is* and *where it originated*.
- **IL2 - No implicit built-ins**: Built-in symbols, functions, or compiler intrinsics are explicitly distinguished (e.g., `#init`, `#print`) from user-defined identifiers.
- **IF1 - Syntax-level feedback**: Punctuation reinforces grammar rather than compensating for ambiguity.
- **IF2 - Progressive information**: Information flows progressively from left to right wherever possible.
- **IUI1 - Declared topology is stable**: UI declarations establish persistent topologies known to the compiler, maintaining stable component structures except at explicitly declared dynamic branches.

---

## 2. Directory Layout & Organization (Architecture v2)

Inspired by Zig (`_local/zig`), Lone uses a clean separation between **compiler host code** (`pkg/`, `cmd/`) and **target language libraries** (`lib/`):

| Directory / File | Description & Purpose |
| :--- | :--- |
| `cmd/lone/` | **Single unified CLI driver** (`lone build`, `lone check`, `lone run`, `lone lsp`). |
| `pkg/` | **Unified Compiler Engine** (single Go module root `lone`): |
| `pkg/ast/` | Abstract Syntax Tree node definitions and source position types. |
| `pkg/parser/` | PEG grammar (`parser.peg`) and generated Pigeon parser. |
| `pkg/sema/` | Semantic analysis, type checking, scope resolution, and Analyzed IR (AIR). |
| `pkg/mir/` | Mid-level IR: basic blocks, CFG, virtual registers, and AIR lowering. |
| `pkg/wir/` | WebAssembly IR (WIR) and WAT transpilation engine. |
| `pkg/adapter/` | Runtime adapter glue generator for Node.js and Browser environments. |
| `pkg/diag/` | Formatted diagnostic messages, error reporters, and AST/AIR printers. |
| `lib/` | **Target Code (shipped with compiler)**: |
| `lib/std/` | Lone Standard Library written in `.lone` (e.g., `lib/std/std.lone`, `io.lone`). |
| `lib/runtime/` | Host runtime glue, WASI preview 1 stubs, and adapter harnesses. |
| `test/` | **Consolidated Test Harness**: |
| `test/behavior/` | End-to-end `.lone` source files verifying compiler and execution outputs. |
| `test/compile_errors/` | Negative `.lone` files asserting diagnostic errors and warnings. |
| `packages/` | *Legacy multi-module implementation (kept for backwards compatibility during migration).* |
| `dev/` | Central engineering hub for specifications, RFCs, research, and session logs. Follows [`dev/POLICY.md`](file:///home/char-c/Projects/lone/dev/POLICY.md). |
| `dev/SPEC.md` | **Living Language Specification (v0.5.0-beta)** (Single source of truth for active features). |
| `dev/adr/` | **Architectural Decision Records** (Immutable decisions on compiler architecture). |
| `dev/rfcs/` | **Formal Proposals & RFCs** (`drafts/`, `accepted/`, `rejected/`). |
| `dev/research/` | **Investigative Studies** (`exploration/`, `audits/`, `benchmarks/`). |
| `dev/sessions/` | **Session History** (Daily roll-up `YYYY-MM-DD.md` and task reports in `YYYY-MM-DD/`). |
| `dev/internals/`| **Compiler Subsystems** (Deep-dive documentation on IR, lowering, and memory). |
| `dev/archive/` | **Historical Archive** (Pre-0.5.0 RFCs and legacy 2026-08 documents). |
| `assets/` | Brand assets, icons, banners, and vector artwork. |

---

## 3. Mandatory AI Operating Directives

Whenever assisting on this repository, you must strictly follow these rules and adhere to [`dev/POLICY.md`](file:///home/char-c/Projects/lone/dev/POLICY.md):

### 3.1 Feature Commits
- When asked to work on a feature, any git commit must be prefixed with:
  ```
  [Ai-assisted] <type>(<scope>): <short description>
  ```
  *Examples:*
  - `[Ai-assisted] feat(parser): parse record declarations`
  - `[Ai-assisted] fix(irtypes): correct type constraint flags for comptime`

### 3.2 End-of-Request Reports
- At the end of every user request or milestone, **write a report** in `dev/sessions/`.
- File naming convention: `dev/sessions/YYYY-MM-DD/<topic>.md` or `dev/sessions/YYYY-MM-DD_<topic>.md`.
- Maintain or update the daily summary index: `dev/sessions/YYYY-MM-DD.md`.
- The report should summarize:
  - What was requested.
  - What changes were made or investigated.
  - Any verification/testing done.
  - Pending questions, next steps, or open considerations.

### 3.3 Documentation Boundaries & Dev Lifecycles
- Consult [`dev/POLICY.md`](file:///home/char-c/Projects/lone/dev/POLICY.md) for the mandatory 5-stage feature pipeline.
- **Language Ideas & Proposals:** Draft in `dev/rfcs/drafts/`. Only move to `dev/rfcs/accepted/` upon user agreement.
- **Exploratory Sketches & Audits:** Place in `dev/research/exploration/` or `dev/research/audits/`.
- **Architectural Decisions:** Record in `dev/adr/`.
- **Internal Component Docs:** Place in `dev/internals/` (e.g. Pratt parser precedence, memory layout, WIR lowering).
- **Active Specification:** Update `dev/SPEC.md` only when features are fully implemented and passing in `pkg2/` and `test2/`.
- **Historical Reference:** Consult `dev/archive/` for historical context, but treat it as read-only.

---

## 4. Development Workflows & Commands

### Unified Toolchain Commands (`cmd/lone`)
1. **Check / Semantic Analysis**:
   ```bash
   go run ./cmd/lone check test/behavior/hello.lone
   ```
2. **Build WebAssembly (WAT)**:
   ```bash
   go run ./cmd/lone build test/behavior/hello.lone
   ```
3. **Build with JavaScript Runtime Glue**:
   ```bash
   go run ./cmd/lone build test/behavior/hello.lone -o out.wat --with-adapter node
   ```
4. **Regenerate Parser**:
   ```bash
   go run github.com/mna/pigeon -o ./pkg/parser/generated-parser.go ./pkg/parser/parser.peg
   ```
5. **Run All Tests**:
   ```bash
   go test ./pkg/...
   ```

---

## 5. Naming & Style Conventions (Lone Language)

When generating or evaluating Lone code:
- **Functions & Local Variables**: `snake_case` (e.g., `calculate_hash`, `user_input`)
- **Constants / Globals**: `ALL_CAPS` (e.g., `MAX_BUFFER_SIZE`)
- **Types, Structs & Records**: `CamelCase` (e.g., `UserData`, `TokenStream`)
- **Built-in Intrinsics / Directives**: `#` prefix (e.g., `#init`, `#print`)
- **Compiler Directives**: `--fa:<directive>` or `---fa:<directive>`
- **Comments**: `--` for single-line comments, `--- ... ---` for multi-line comments.
