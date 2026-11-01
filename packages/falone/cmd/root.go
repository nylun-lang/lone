package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "falone",
	Short: "Falone is the official compiler and toolchain for the Lone language",
	Long: `Falone compiles Lone source code into Falone MIR (Mid-level Intermediate Representation)
and WebAssembly (WAT). It also integrates lone-alyzer for static analysis and type checking.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		// If a file is passed directly to falone (e.g. 'falone file.lone'), default to compile
		return runCompile(args[0])
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(compileCmd)
	rootCmd.AddCommand(analyzeCmd)
}
