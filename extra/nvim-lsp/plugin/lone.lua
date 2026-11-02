-- Commands for lone-lsp
vim.api.nvim_create_user_command("LoneLspStart", function()
  require("lone-lsp").start()
end, { desc = "Start or restart the Lone LSP client for the current buffer" })

vim.api.nvim_create_user_command("LoneLspInfo", function()
  local clients = vim.lsp.get_clients({ name = "lone-lsp" })
  if #clients == 0 then
    vim.notify("[lone-lsp] No active lone-lsp client running.", vim.log.levels.INFO)
  else
    for _, client in ipairs(clients) do
      vim.notify(
        string.format("[lone-lsp] Client ID: %d | Root: %s", client.id, client.config.root_dir or "nil"),
        vim.log.levels.INFO
      )
    end
  end
end, { desc = "Show Lone LSP client status" })
