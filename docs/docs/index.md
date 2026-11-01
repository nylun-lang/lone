---
layout: home

hero:
  name: "Lone"
  text: "Statically Typed Systems Language for WebAssembly"
  tagline: "High-performance compiler with multi-pass static analysis, Falone MIR, and standalone WASI/Web targets."
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
      link: /content/latest/toolchain/falone-cli

features:
  - title: Predictable Type System
    details: Static typing with automatic inference (infer), fixed-size lists ([N]T), and a strict consume-or-drop invariant.
  - title: Falone MIR & Zero-Dep Wasm
    details: Clean lowering to control-flow graphs with basic blocks, inlined WASI syscalls, and structured Wasm loops.
  - title: Auto-Generated Runtime Adapters
    details: Built-in --with-adapter flag generates production-ready JavaScript glue for Node.js and web browsers.
  - title: Built-in Intrinsics
    details: High-speed intrinsics like #print mapped directly to linear memory data segments and optimized syscalls.
---
