package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lone",
	Short: "Lone is an explicit, systems- and UI-oriented programming language",
	Long: `Lone compiles Lone source code (.lone) to WebAssembly (Wasm/WAT) and provides
tooling for semantic analysis, type verification, and runtime execution.
Inspired by Zig, Go, and TypeScript.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return runBuild(args[0], "wasi")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(lspCmd)
}
