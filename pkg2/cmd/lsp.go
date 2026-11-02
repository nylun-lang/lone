package main

import (
	"os"

	lonelsp "lone/pkg2/lone-lsp"

	"github.com/spf13/cobra"
)

var lspCmd = &cobra.Command{
	Use:   "lsp",
	Short: "Start the Lone Language Server Protocol daemon (stdio mode)",
	RunE: func(cmd *cobra.Command, args []string) error {
		srv := lonelsp.NewServer()
		return srv.Serve(os.Stdin, os.Stdout)
	},
}
