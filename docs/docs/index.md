---
layout: home

hero:
  name: "Lone"
  text: "Statically Typed Systems Language for WebAssembly"
  tagline: "High-performance compiler with Pratt parsing, linear memory management, WIR backend, and standalone WASI/Web targets."
  image:
    src: /logo.svg
    alt: Lone Language Logo
  actions:
    - theme: brand
      text: Get Started
      link: /content/latest/guide/getting-started
    - theme: alt
      text: Language Tour
      link: /content/latest/language/variables-and-types
    - theme: alt
      text: Toolchain & CLI
      link: /content/latest/toolchain/lone-cli

features:
  - title: Predictable Type System
    details: Static typing with algebraic data types, pattern matching, interface contracts, and asymmetric field visibility ro(pub).
  - title: Direct WIR & Zero-Dep Wasm
    details: Clean lowering to WebAssembly Intermediate Representation with linear scan register allocation and inlined WASI syscalls.
  - title: Safe Linear Memory Architecture
    details: Downward Page 0 datastack (shadow stack) paired with Page 1+ free-list heap allocator with coalescing and dynamic memory.grow.
  - title: Built-in Intrinsics & Standard Library
    details: High-speed compiler intrinsics prefixed with # alongside a pure-Lone standard library (:std:io, :std:sys).
---
