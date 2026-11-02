# RFC: Architecture v2 — Zig-Inspired Unified Toolchain

- **Author:** Lone Core Team / AI Pair
- **Date:** 2026-11-02
- **Status:** Proposed & In-Progress
- **Inspiration:** Zig Programming Language (`_local/zig`)

---

## 1. Motivation & Context

On 2026-11-01, the Lone project expanded rapidly across three Go modules (`packages/lone-lang`, `packages/lone-alyzer`, and `packages/falone`), an external C runtime (`packages/c-lone-runtime`), multiple independent CLIs, and two distinct intermediate representations with duplicated AST/IR structures.

While functional, this rapid growth created:
1. **Cognitive Overhead:** Hard to follow the boundaries between what executes on the compiler host versus the target WebAssembly runtime.
2. **Module Sprawl:** Multi-module `go.work` setups with circular dependencies and `replace` directives.
3. **Pipeline Fragmentation:** AST definition (`lone-lang/irtypes`) split from Analyzed IR (`lone-alyzer/irtypes`) and Mid-level IR (`falone/mirtypes`).
4. **Scattered Standard Library:** No clean location for standard library code written in `.lone`.

Taking architectural inspiration from the self-hosted Zig toolchain (`_local/zig`), this RFC establishes **Architecture v2** for Lone.

---

## 2. Zig Architectural Principles Applied to Lone

Inspecting `_local/zig` demonstrates three fundamental rules:

1. **`lib/` vs `src/` (or `pkg/`) Separation:**
   - `lib/`: Code intended for the **target program** (Standard library written in `.lone`, compiler runtime intrinsics, and WASI/JS host glue).
   - `pkg/` & `cmd/`: Code intended for the **host compiler** (Parser, Semantic Analyzer, IRs, Codegen, Driver).
2. **Unified Pipeline, Not Micro-Modules:**
   - In Zig, `Air`, `Sema`, `Ast`, and `codegen` are pipeline stages, not isolated external packages. In Go, they belong as clean packages under a single root Go module (`lone`), eliminating `replace` directives and module version synchronization.
3. **Single Cohesive Binary (`cmd/lone`):**
   - A single toolchain entrypoint: `lone build`, `lone check`, `lone run`, `lone lsp`.

---

## 3. Directory Layout (Architecture v2)

```text
lone/
├── cmd/
│   └── lone/                 # Single unified CLI driver (main.go)
│       ├── root.go           # Root command & global flags (--verbose, --lib-dir)
│       ├── build.go          # 'lone build <file.lone>' (emits .wasm / .wat)
│       ├── check.go          # 'lone check <file.lone>' (diagnostics & type-checking)
│       ├── run.go            # 'lone run <file.lone>' (compile & execute)
│       └── lsp.go            # 'lone lsp' (Language Server Protocol)
│
├── pkg/                      # Compiler Core (single Go module)
│   ├── ast/                  # AST definitions, token positions, source spans
│   ├── parser/               # PEG grammar & generated Pigeon parser
│   ├── sema/                 # Semantic analysis, type checking, Analyzed IR (AIR)
│   ├── mir/                  # Control Flow Graph (CFG), Basic Blocks, Virtual Registers
│   ├── wir/                  # WebAssembly IR & WAT transpilation engine
│   ├── adapter/              # JS glue generators for Node and Browser runtimes
│   ├── project/              # Lone project configuration and workspace manifest
│   └── diag/                 # Formatted terminal diagnostics and error reporting
│
├── lib/                      # Target Code (shipped alongside compiler)
│   ├── std/                  # Standard Library written in Lone (.lone)
│   │   ├── io.lone           # Standard I/O (#print wrapper, file descriptors)
│   │   ├── sys.lone          # System calls & memory grow primitives
│   │   └── string.lone       # String manipulation & slices
│   └── runtime/              # Host glue & runtime templates
│       ├── wasi_stub.wat     # Minimal WASI preview 1 stubs
│       └── web_adapter.js    # Browser/Node execution harness
│
├── test/                     # End-to-End Test Harness
│   ├── behavior/             # Functional tests checking runtime output
│   └── compile_errors/       # Negative tests checking compile-time errors
│
├── docs/                     # VitePress documentation
├── drafts/                   # RFCs, design sketches, session reports
├── go.mod                    # Single root Go module (module lone)
└── README.md
```

---

## 4. Pipeline Stages & Data Flow

$$\text{Source Code (.lone)} \xrightarrow{\text{pkg/parser}} \text{AST} \xrightarrow{\text{pkg/sema}} \text{AIR (Typed IR)} \xrightarrow{\text{pkg/mir}} \text{MIR (CFG)} \xrightarrow{\text{pkg/wir}} \text{Wasm / WAT}$$

1. **`pkg/ast`**: Raw syntax tree nodes directly emitted by `pkg/parser`.
2. **`pkg/sema`**: Name resolution, type inference, consume-or-drop checks, and emission of typed `AIR`.
3. **`pkg/mir`**: Lowering `AIR` into linear basic blocks with explicit branch targets, temporaries, and register allocations.
4. **`pkg/wir`**: Lowering `MIR` into structured WebAssembly statements (`loop`, `block`, `br`, `call`, memory data segments).
5. **`lib/std`**: Core language features written natively in Lone, compiled and linked by default unless `--no-std` is passed.

---

## 5. Migration Strategy

1. **Root Module Initialization**: Setup single root `go.mod` for `lone`.
2. **Extract `lib/std` & `lib/runtime`**: Move runtime artifacts and start standard library in `lib/std/`.
3. **Consolidate Packages into `pkg/`**:
   - `packages/lone-lang/parser` + AST $\rightarrow$ `pkg/ast` & `pkg/parser`
   - `packages/lone-alyzer/analyze` + Typed IR $\rightarrow$ `pkg/sema` & `pkg/diag`
   - `packages/falone/lowering` + `mirtypes` $\rightarrow$ `pkg/mir`
   - `packages/falone/transpile` + `adapter` $\rightarrow$ `pkg/wir` & `pkg/adapter`
4. **Unified CLI**: Implement `cmd/lone` using Cobra, offering `build`, `check`, `run`.
5. **Clean up**: Remove `packages2/`, retire old multi-module `go.work`, update documentation and `AGENT.md`.
