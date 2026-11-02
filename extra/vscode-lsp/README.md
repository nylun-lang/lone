# Lone Language Support (VS Code Extension)

Official Visual Studio Code extension providing syntax highlighting and Language Server Protocol (LSP) diagnostics, hover information, and document symbols for the **Lone Programming Language**.

## Features

- **Language Server Integration**: Real-time syntax and semantic diagnostics, hover documentation, and document symbol outlines powered by `lone lsp`.
- **Zero Config**: Automatically uses the `lone` binary present in your system `PATH`.
- **Syntax Highlighting**: Full TextMate grammar for keywords, types, intrinsics (`#print`, `#init`), strings, comments (`--` and `---`), and operators.
- **Language Configuration**: Brackets, indentation, and comment toggling.

## Requirements

Ensure that the Lone compiler CLI `lone` is installed and available in your `PATH`:

```bash
# Verify lone is in PATH
lone --help
lone lsp --help
```

If you are developing locally inside the Lone repository, you can compile and install `lone` into your PATH:

```bash
go install ./cmd/lone
```

## Extension Settings

This extension contributes the following settings:

* `lone.lsp.command`: Path or command for the Lone executable (default: `"lone"`).
* `lone.lsp.args`: Arguments passed to start the language server in stdio mode (default: `["lsp"]`).
* `lone.trace.server`: Traces communication between VS Code and the Lone language server (`"off"`, `"messages"`, or `"verbose"`).

## Building & Installation

To build and package the extension locally:

```bash
cd extra/vscode-lsp
npm install
npm run build
```

To install directly into VS Code or VSCodium for local development:

```bash
# Link extension for development
ln -s "$(pwd)/extra/vscode-lsp" ~/.vscode/extensions/lone-lsp
```
or package into a `.vsix` file using `vsce`:

```bash
npx @vscode/vsce package
code --install-extension lone-lsp-0.1.0.vsix
```
