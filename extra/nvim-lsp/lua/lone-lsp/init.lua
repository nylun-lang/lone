local M = {}

--- Default configuration options for lone-lsp
M.default_config = {
  -- The command and arguments to start the language server.
  -- By default, it runs `lone lsp`, looking up `lone` from the system PATH.
  cmd = { "lone", "lsp" },
  filetypes = { "lone" },
  root_markers = { "lone.toml", "go.mod", ".git" },
  name = "lone-lsp",
  single_file_support = true,
  settings = {},
}

--- Resolves the project root directory for a given buffer.
---@param bufnr integer? Buffer number (0 or nil for current buffer)
---@param markers table? List of files or directories identifying the root
---@return string
function M.get_root_dir(bufnr, markers)
  bufnr = bufnr or 0
  markers = markers or M.default_config.root_markers
  local buf_name = vim.api.nvim_buf_get_name(bufnr)
  if buf_name == "" then
    return vim.fn.getcwd()
  end

  local root
  if vim.fs and vim.fs.root then
    root = vim.fs.root(buf_name, markers)
  elseif vim.fs and vim.fs.find then
    local match = vim.fs.find(markers, { upward = true, path = vim.fs.dirname(buf_name) })[1]
    if match then
      root = vim.fs.dirname(match)
    end
  end

  if root then
    return root
  end

  if vim.fs and vim.fs.dirname then
    return vim.fs.dirname(buf_name)
  end

  return vim.fn.fnamemodify(buf_name, ":h")
end

--- Starts or attaches the Lone LSP client to the specified buffer.
---@param opts table? Optional configuration overrides
---@return integer? client_id The LSP client ID or nil on failure
function M.start(opts)
  opts = vim.tbl_deep_extend("force", {}, M.default_config, opts or {})

  local cmd = opts.cmd
  local bin = type(cmd) == "table" and cmd[1] or cmd

  if vim.fn.executable(bin) ~= 1 then
    vim.notify(
      string.format("[lone-lsp] Binary '%s' not found in PATH. Ensure 'lone' is installed in PATH.", bin),
      vim.log.levels.WARN
    )
    return nil
  end

  local bufnr = opts.bufnr or vim.api.nvim_get_current_buf()
  local root_dir = opts.root_dir or M.get_root_dir(bufnr, opts.root_markers)

  local client_config = {
    name = opts.name,
    cmd = opts.cmd,
    root_dir = root_dir,
    capabilities = opts.capabilities,
    on_attach = opts.on_attach,
    settings = opts.settings or {},
  }

  return vim.lsp.start(client_config, { bufnr = bufnr })
end

--- Initializes lone-lsp with user options and registers FileType autocommands.
---@param opts table? User configuration overrides
function M.setup(opts)
  M.default_config = vim.tbl_deep_extend("force", M.default_config, opts or {})

  local group = vim.api.nvim_create_augroup("lone_lsp_autostart", { clear = true })
  vim.api.nvim_create_autocmd("FileType", {
    group = group,
    pattern = "lone",
    callback = function(args)
      M.start({ bufnr = args.buf })
    end,
    desc = "Auto-start Lone LSP for .lone buffers",
  })
end

--- Convenience helper to register Lone LSP with `nvim-lspconfig`
---@param custom_opts table? Additional options passed to lspconfig.lone.setup(...)
function M.setup_lspconfig(custom_opts)
  local ok, lspconfig = pcall(require, "lspconfig")
  if not ok then
    vim.notify("[lone-lsp] nvim-lspconfig is not installed", vim.log.levels.ERROR)
    return
  end

  local configs = require("lspconfig.configs")
  if not configs.lone then
    configs.lone = {
      default_config = {
        cmd = { "lone", "lsp" },
        filetypes = { "lone" },
        root_dir = function(fname)
          return M.get_root_dir(0, { "lone.toml", "go.mod", ".git" })
        end,
        single_file_support = true,
      },
    }
  end

  lspconfig.lone.setup(custom_opts or {})
end

return M
