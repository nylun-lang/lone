# Work Session Report: Audit Resolutions & RFC Drafting

**Date**: 2026-11-03  
**Branch**: `audit/language-inconsistencies-and-improvements`  
**Authors**: Lone Compiler Engineering Team & AI Pair  
**RFC Document**: [`drafts/research/rfc_language_refinements_and_compiler_architecture.md`](file:///home/char-c/Projects/lone/drafts/research/rfc_language_refinements_and_compiler_architecture.md)

---

## 1. Request Overview

Following the comprehensive language and compiler audit, the user requested a full RFC design draft resolving each item identified during the audit:
1. **`let` vs. `var`**: Formalize immutability by default.
2. **Collection Literals vs. Types**: Harmonize list literals to square brackets `[1, 2, 3]` and empty list `[]`, reserving curly braces `{ ... }` for struct literals and blocks.
3. **AST Comparison Operators**: Correct inverted operator constants in [`expr.go`](file:///home/char-c/Projects/lone/pkg/lone-parser-ast/expr.go).
4. **Control Flow Invariants**: Maintain existing control flow syntax as-is, while documenting the precise rationale for each.
5. **Numeric Coercions**: Eliminate implicit conversions; specify the explicit `as` cast operator and its Wasm instruction lowering.
6. **Pointers, References, and Intrinsics**: Clarify `ref<T>` vs. `ptr<T>`, eliminate `#refGrab`, and migrate `#ptr_to_str` into `:std:str`.
7. **Trait Naming Convention**: Adopt `Tr` + PascalCase (e.g. `TrWriter`, `TrIterator`) to eliminate dictionary whitelist/blacklist hacks.
8. **Pratt Expression Parser**: Draft the transition from 15-tier PEG cascades to top-down operator precedence.
9. **Virtual Register Explosion**: Demystify the Wasm local bloat problem and outline a linear scan register allocator.
10. **Structured Control Flow in WIR**: In-depth pros and cons discussion of Relooper vs. AST-directed structured lowering.
11. **Linear Memory Architecture**: Draft safe memory layout, stack-heap collision detection, dynamic `memory.grow`, and free lists.
12. **Language Evolution**: Draft specifications for Generics/Monomorphization, Tagged Unions (`enum`), Pattern Matching (`match`), and `Result<T, E>` with the `?` operator.

---

## 2. Key Accomplishments & Deliverables

1. **Delivered Comprehensive RFC**: Created [`drafts/research/rfc_language_refinements_and_compiler_architecture.md`](file:///home/char-c/Projects/lone/drafts/research/rfc_language_refinements_and_compiler_architecture.md) containing full specifications, code snippets, grammar definitions, and algorithm designs across all 12 areas.
2. **Detailed Architectural Pros & Cons**: Analyzed options for structured WebAssembly generation (Relooper vs AST-directed lowering vs heuristic region matching), establishing AST-directed lowering as the optimal path for Lone.
3. **Memory Safety Roadmap**: Formulated a partitioned linear memory layout with guard pages, collision detection, and dynamic page growth to eliminate stack smashing.
4. **Clean Phased Implementation Plan**: Structured the implementation into 4 sequential phases (Phase 0 Quick Fixes $\rightarrow$ Phase 1 Core Subsystems $\rightarrow$ Phase 2 Engine Upgrades $\rightarrow$ Phase 3 Language Expansion).

---

## 3. Verification & Current Repository State

- Working directory is clean and tests pass across all packages:
  ```bash
  go test -count=1 ./...
  ```
- All documentation follows repository conventions defined in [`AGENT.md`](file:///home/char-c/Projects/lone/AGENT.md).
