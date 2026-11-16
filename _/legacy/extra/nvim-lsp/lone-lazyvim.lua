return {
  -- 1. Lone syntax and filetype detection
  {
    dir = vim.fn.expand("~/Projects/lone/extra/nvim-lsp"),
    name = "lone-lsp",
    ft = { "lone" },
  },

  -- 2. LazyVim / nvim-lspconfig integration (Mason compatible)
  {
    "neovim/nvim-lspconfig",
    opts = {
      servers = {
        lone = {
          -- Tells Mason this is provided by PATH/source, bypassing registry lookup
          mason = false,
          cmd = { "lone", "lsp" },
          filetypes = { "lone" },
          root_dir = function(fname)
            local root = vim.fs.root(fname, { "lone.toml", "go.mod", ".git" })
            return root or vim.fs.dirname(fname)
          end,
          single_file_support = true,
        },
      },
      setup = {
        lone = function(_, opts)
          local configs = require("lspconfig.configs")
          if not configs.lone then
            configs.lone = {
              default_config = {
                cmd = { "lone", "lsp" },
                filetypes = { "lone" },
                root_dir = function(fname)
                  local root = vim.fs.root(fname, { "lone.toml", "go.mod", ".git" })
                  return root or vim.fs.dirname(fname)
                end,
                single_file_support = true,
              },
            }
          end
        end,
      },
    },
  },
}
