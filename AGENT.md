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

## 2. Directory Layout & Organization

| Directory / File | Description & Purpose |
| :--- | :--- |
| `AGENT.md` | AI instructions, repository conventions, and workflow rules (this file). |
| `README.md` | Main project entry point and high-level introduction. |
| `go.work` / `go.work.sum` | Go workspace configuration linking active Go packages. |
| `packages/` | Core implementation packages of Lone. |
| `packages/lone-lang/` | PEG grammar (`parser/parser.peg`), generated parser (`mna/pigeon`), AST/IR data structures (`irtypes`), and sample test runners (`cmd/parse.go`). |
| `packages/falone/` | The official Lone compiler (`Falone`), translating Lone IR to WebAssembly. |
| `packages/lone-alyzer/` | Static analysis, LSP, and developer tooling for Lone. |
| `drafts/` | Working documentation, designs, reports, and internal specs. |
| `drafts/research/` | **Ideas and drafts of what the project could be** (language specifications, sketches, RFCs, memory model explorations). |
| `drafts/under-the-hood/` | **Internal architectural documentation** explaining how the project and its components work under the hood. |
| `drafts/reports/` | **Work session reports** generated at the conclusion of tasks and feature requests. |
| `drafts/archive/` | Historical specifications and archived documentation. |
| `_old/` | **Legacy or old implementations of Lone.** Kept strictly for reference; do not modify unless migrating legacy functionality. |
| `assets/` | Brand assets, icons, banners, and vector artwork. |

---

## 3. Mandatory AI Operating Directives

Whenever assisting on this repository, you must strictly follow these rules:

### 3.1 Feature Commits
- When asked to work on a feature, any git commit must be prefixed with:
  ```
  [Ai-assisted] <type>(<scope>): <short description>
  ```
  *Examples:*
  - `[Ai-assisted] feat(parser): parse record declarations`
  - `[Ai-assisted] fix(irtypes): correct type constraint flags for comptime`

### 3.2 End-of-Request Reports
- At the end of every user request or milestone, **write a report** in `drafts/reports/`.
- File naming convention: `drafts/reports/YYYY-MM-DD_<brief-topic>.md`.
- The report should summarize:
  - What was requested.
  - What changes were made or investigated.
  - Any verification/testing done.
  - Pending questions, next steps, or open considerations.

### 3.3 Documentation Boundaries
- **Language Ideas & Proposals:** Place in `drafts/research/` (e.g., grammar sketches, type system proposals).
- **Internal Component Docs:** Place in `drafts/under-the-hood/` (e.g., how the PEG parser builds the AST, symbol resolution stages, memory layout).
- **Legacy Reference:** Consult `_old/` when researching earlier design iterations, but write all new code in `packages/`.

---

## 4. Development Workflows & Commands

The project uses Go (1.27+) with a multi-module workspace (`go.work`).

### Working on the Parser (`packages/lone-lang`)
1. **Grammar definition**: `packages/lone-lang/parser/parser.peg`
2. **Generate parser**:
   ```bash
   cd packages/lone-lang
   go tool pigeon -o ./parser/generated-parser.go ./parser/parser.peg
   ```
   *(or `just gen-parser` if `just` is installed)*
3. **Run parser against sample files**:
   ```bash
   go run ./packages/lone-lang/cmd/parse.go ./packages/lone-lang/samples/misc.lone
   ```
   *(or `cd packages/lone-lang && just parse samples/misc.lone`)*

---

## 5. Naming & Style Conventions (Lone Language)

When generating or evaluating Lone code:
- **Functions & Local Variables**: `snake_case` (e.g., `calculate_hash`, `user_input`)
- **Constants / Globals**: `ALL_CAPS` (e.g., `MAX_BUFFER_SIZE`)
- **Types, Structs & Records**: `CamelCase` (e.g., `UserData`, `TokenStream`)
- **Built-in Intrinsics / Directives**: `#` prefix (e.g., `#init`, `#print`)
- **Compiler Directives**: `--fa:<directive>` or `---fa:<directive>`
- **Comments**: `--` for single-line comments, `--- ... ---` for multi-line comments.
