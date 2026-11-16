# Lone Compiler Internals (`dev/internals/`)

This directory contains deep-dive architectural documentation detailing the subsystems, intermediate representations, memory models, and code generation mechanics of the **Lone Compiler**.

---

## 1. Active Subsystem Documentation (`pkg2`)

The following documents reflect the modern compiler pipeline implemented in `pkg2/`:

| Document | Subsystem / Focus | Key Concepts |
| :--- | :--- | :--- |
| [**`memory_and_pointer_model.md`**](./memory_and_pointer_model.md) | **Memory Architecture & Pointer Hierarchy** | Page 0 Datastack (`$__stack_pointer`), Managed Store free-list heap, frame spilling (`AddressTaken`), 3-tier pointer hierarchy (`ptr<T>`, `ptr<[N]T>`, `ptr<[]T>`), fat references `ref<[]T>`, caller-side pass-by-value struct cloning. |
| [**`wir_codegen_architecture.md`**](./wir_codegen_architecture.md) | **WebAssembly IR & Code Generation** | `pkg2/lone-compiler-wir`, structured WebAssembly AST (`WasmInst`), register allocation & local pooling (`LocalAllocator`), inlined runtime library, dual-path string interpolation. |
| [**`semantic_analyzer_architecture.md`**](./semantic_analyzer_architecture.md) | **Semantic Analysis & Type Checking** | `pkg2/lone-analyzer`, two-pass declaration hoisting, symbol stack (`SymbolStack`), structural duck typing (`Ifc*`), width subtyping, immutability diagnostics, auto-dereferencing. |
| [**`v0.6.0_implementation_roadmap.md`**](./v0.6.0_implementation_roadmap.md) | **v0.6.0 Compiler Delivery Roadmap** | Detailed status, deliverables, and test verification across all 6 phases of the Lone v0.6.0 compiler release. |

---

## 2. Historical & Grammar Reference Documents

The following documents describe initial PEG grammar and AST concepts:

| Document | Topic |
| :--- | :--- |
| [**`type_system_ir.md`**](./type_system_ir.md) | Grammar type representations (`TypeBase`, `TypeList`, `TypeFn`) and constraints. |
| [**`expressions_and_postfix.md`**](./expressions_and_postfix.md) | Postfix chaining, literal expressions, and member access. |
| [**`control_flow_if_while.md`**](./control_flow_if_while.md) | Conditional branching (`if`/`else`) and loops (`while`). |
| [**`function_declarations.md`**](./function_declarations.md) | Top-level function syntax and statement block modeling. |
| [**`labeled_blocks_and_breaks.md`**](./labeled_blocks_and_breaks.md) | Labeled blocks as expressions and targeted break statements. |
