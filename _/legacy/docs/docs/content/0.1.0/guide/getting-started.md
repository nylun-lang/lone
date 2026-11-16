---
title: "Getting Started (v0.1.0)"
order: 2
---

# Getting Started (v0.1.0)

## Installation

In Lone v0.1.0, the core parser package was provided under `lonelang/parser`:

```bash
cd packages/lone-lang
go test -v ./...
```

## Parsing Lone Files

Run the early AST parser:

```bash
go run ./packages/lone-lang/cmd file.lone
```
