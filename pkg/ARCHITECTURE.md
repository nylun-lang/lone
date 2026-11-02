# Package Architecture

- **lone-parser-ast** AST nodes and source span positioning.

- **lone-parser-ast** AST nodes and source span positioning.

- **lone-shared-logger** Rust/Zig-style source excerpts with carets and line numbers.

- **lone-analyzer** Multi-pass type inference, scope stack, return verification, and consume-or-drop rules.

- **lone-analyzer-diag** Typed IR JSON and indented tree printers.

- **lone-analyzer-project** Multi-file dependency graph, cycle detection, and topological compilation ordering.

- **lone-runtime** Inlined intrinsic table registry (#print for WASI and Web).

- **lone-compiler-mir** Assembly-like Mid-level IR and lowerer.

- **lone-compiler-wir** Structured WebAssembly AST, peephole optimizer, and WAT serializer.

- **lone-compiler-adapter** Node.js and Browser JavaScript glue generators.

- **lone-compiler** End-to-end compiler coordinator.

- **lone-lsp** JSON-RPC 2.0 Language Server Protocol daemon.
