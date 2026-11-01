# Changelog

All notable changes to the Lone language project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.2.0] - 2026-11-01

### Added
- **Falone Compiler & CLI (`packages/falone`)**:
  - Implemented Falone Mid-level Intermediate Representation (MIR) with control flow graphs, basic blocks, and virtual registers.
  - Implemented Typed IR lowering to Falone MIR.
  - Implemented WebAssembly Text (`.wat`) transpiler with linear memory management and structured loops.
  - Added Cobra CLI with `compile`, `compile wasm`, `compile web`, and `analyze` subcommands.
  - Added `--with-adapter <node | browser>` flag generating JavaScript runtime loader glue.
  - Added standalone WASI preview 1 support with inlined `$__lone_rt_print` and `fd_write` syscall.
  - Added Web browser target with `(import "env" "print" ...)`.
  - Added dual entrypoint exports `(export "_start")` and `(export "main")`.
- **Static Analysis & Type Checking (`packages/lone-alyzer`)**:
  - Implemented multi-pass analyzer: Naming Conventions, Type Inference, Return Verification, and Consume-or-Drop rule.
  - Added Typed IR (`air`) definitions and AST-to-Typed-IR transformation.
  - Pre-registered `#print(str: str) void` intrinsic in scope stack.
  - Added CLI for AST pretty-printing, Typed IR output, and JSON diagnostic exports.
- **Parser & Language Features (`packages/lone-lang`)**:
  - Added fixed-size list type syntax `[N]T` and list literals.
  - Added function declaration syntax `fn name(params) return_type`.
  - Added control flow: `if`/`else`, `while` loops, labeled blocks, and `break` with values.
  - Added `drop <expr>` statement syntax.
  - Added `#` intrinsic prefix support in `NAME_REF` expressions.
  - Added string escape sequence handling (`\n`, `\r`, `\t`, `\"`, `\\`).
- **Documentation Platform (`docs/`)**:
  - Configured VitePress static site generator.
  - Authored comprehensive guides for v0.2.0 and historical v0.1.0 documentation under `content/<sem-ver>/`.
  - Added recursive sidebar generator (`docs/scripts/gen-sidebar.js`) with semver ordering and highest-version-as-latest syncing.
  - Added branded hero home page and logo assets.

---

## [0.1.0] - 2026-11-01

### Added
- Initial project structure with PEG parser for the Lone programming language.
- Primitive types (`i8`..`i128`, `u8`..`u128`, `f32`, `f64`, `bool`, `str`, `void`, `none`).
- Basic variable definitions (`var x: T = val`).
- Early AST definitions and JSON parser CLI.
