# lone-lsp (Neovim Integration)

Official Neovim plugin and LSP configuration for the **Lone Programming Language**.

Provides filetype detection (`*.lone`), syntax highlighting, and Language Server Protocol integration via `lone lsp` (diagnostics, hover, document symbols).

## Requirements

Ensure the Lone compiler CLI binary `lone` is accessible in your system `PATH`:

```bash
# Verify lone is in PATH
lone --help
lone lsp --help
```

If developing locally from source:

```bash
go install ./cmd/lone
```

---

## Installation & Setup

### Option 1: Native Neovim (0.8+) Built-in LSP (Zero Dependencies)

If you clone or add this plugin to your `runtimepath` or `packpath`:

Using **lazy.nvim**:
```lua
{
  dir = "/path/to/lone/extra/nvim-lsp", -- or git repository URL
  ft = "lone",
  config = function()
    require("lone-lsp").setup({
      -- Custom options (optional)
      -- cmd = { "lone", "lsp" },
    })
  end,
}
```

Or manually in your `init.lua`:
```lua
-- Add plugin to runtimepath
vim.opt.rtp:append("/path/to/lone/extra/nvim-lsp")

-- Setup Lone LSP
require("lone-lsp").setup()
```

### Option 2: Using with `nvim-lspconfig`

If you use `nvim-lspconfig`, you can register Lone with:

```lua
local lone_lsp = require("lone-lsp")
lone_lsp.setup_lspconfig({
  on_attach = function(client, bufnr)
    -- Your keymaps (e.g. vim.lsp.buf.hover, vim.lsp.buf.definition)
  end,
  capabilities = capabilities,
})
```

---

## Default Configuration

The default configuration options are:

```lua
{
  cmd = { "lone", "lsp" }, -- Uses 'lone' found in PATH
  filetypes = { "lone" },
  root_markers = { "lone.toml", "go.mod", ".git" },
  name = "lone-lsp",
  single_file_support = true,
}
```

---

## User Commands

* `:LoneLspStart` - Start or attach the Lone LSP client to the current buffer.
* `:LoneLspInfo` - Display active Lone LSP client instances and root paths.
