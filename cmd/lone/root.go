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
tooling for semantic analysis, project ordering, and runtime execution.
Driven by the unified pkg2 subsystem architecture.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		// If a file or dir is passed directly, default to build
		return runBuild(args[0], "wasi")
	},
}

// Execute is the main entrypoint for the CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(lspCmd)
}
