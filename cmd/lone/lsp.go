package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var lspCmd = &cobra.Command{
	Use:   "lsp",
	Short: "Start the Lone Language Server Protocol daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Lone LSP server initializing (stdio mode)...")
		// LSP daemon loop will be implemented in pkg/lsp
		return nil
	},
}
