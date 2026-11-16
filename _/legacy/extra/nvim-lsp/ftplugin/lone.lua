-- Settings for Lone buffers
vim.bo.commentstring = "-- %s"
vim.bo.shiftwidth = 4
vim.bo.tabstop = 4
vim.bo.softtabstop = 4
vim.bo.expandtab = true

-- Automatically start LSP if 'lone' is in PATH
local ok, lone_lsp = pcall(require, "lone-lsp")
if ok then
  lone_lsp.start()
else
  if vim.fn.executable("lone") == 1 then
    local root = nil
    if vim.fs and vim.fs.root then
      root = vim.fs.root(0, { "lone.toml", "go.mod", ".git" })
    end
    vim.lsp.start({
      name = "lone-lsp",
      cmd = { "lone", "lsp" },
      root_dir = root or vim.fs.dirname(vim.api.nvim_buf_get_name(0)),
    })
  end
end
