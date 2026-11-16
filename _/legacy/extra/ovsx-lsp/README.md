# Lone Language Support (Open VSX / VS Code Compatible)

Official extension providing Language Server Protocol (LSP) diagnostics, hover information, document symbols, and syntax highlighting for the **Lone Programming Language**.

Compatible with **Open VSX**, **VSCodium**, **Cursor**, **Windsurf**, **Eclipse Theia**, **Gitpod**, and **Visual Studio Code**.

---

## ⚡ Instant Installation (Pre-built `.vsix`)

A pre-packaged, ready-to-use extension bundle is included directly in this repository:
- **`extra/ovsx-lsp/lone-lsp-0.1.0.vsix`**

### Option A: Install via Editor GUI
1. Open your editor (VSCodium, Cursor, VS Code, etc.).
2. Open the **Extensions** panel (`Ctrl+Shift+X` or `Cmd+Shift+X`).
3. Click the **`...`** (Views and More Actions) menu in the top-right corner.
4. Select **Install from VSIX...**
5. Select `lone-lsp-0.1.0.vsix`.

### Option B: Install via Terminal Command Line
```bash
# For VSCodium
codium --install-extension extra/ovsx-lsp/lone-lsp-0.1.0.vsix

# For Cursor
cursor --install-extension extra/ovsx-lsp/lone-lsp-0.1.0.vsix

# For VS Code
code --install-extension extra/ovsx-lsp/lone-lsp-0.1.0.vsix
```

---

## 🚀 Requirements

Ensure that the Lone compiler CLI `lone` is installed and available in your `PATH`:

```bash
lone --help
lone lsp --help
```

If developing locally inside this repository:

```bash
# Builds 'lone' into ~/bin (ensure ~/bin is in $PATH)
go build -o ~/bin/lone ./cmd/lone
```

---

## ⚙️ Extension Settings

| Setting | Default | Description |
| :--- | :--- | :--- |
| `lone.lsp.command` | `"lone"` | Binary name or path for the Lone executable (found in `$PATH`). |
| `lone.lsp.args` | `["lsp"]` | Arguments passed to run the LSP server daemon over stdio. |
| `lone.trace.server` | `"off"` | Traces communication between the editor and the Lone language server (`"off"`, `"messages"`, or `"verbose"`). |

---

## 🛠️ Building & Publishing

### Build `.vsix`
```bash
cd extra/ovsx-lsp
npm install
npm run build
npm run package
```

### Publish to Open VSX Registry (open-vsx.org)
Using the official Eclipse Open VSX CLI (`ovsx`):
```bash
# Login or set your Personal Access Token
npx ovsx publish lone-lsp-0.1.0.vsix -p <YOUR_OPEN_VSX_PAT>
```
