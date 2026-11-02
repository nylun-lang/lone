package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	lonecompiler "lone/pkg/lone-compiler"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <file.lone>",
	Short: "Build and execute a Lone program",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		tmpDir, err := os.MkdirTemp("", "lone-run-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		watPath := filepath.Join(tmpDir, "out.wat")
		buildOutFlag = watPath
		buildAdapterFlag = ""
		buildQuietFlag = true

		if err := runBuild(filePath, lonecompiler.TargetWasi); err != nil {
			return err
		}

		// Try wasmtime if installed
		if wasmtimePath, err := exec.LookPath("wasmtime"); err == nil {
			execCmd := exec.Command(wasmtimePath, watPath)
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
			execCmd.Stdin = os.Stdin
			return execCmd.Run()
		}

		fmt.Printf("Compiled to WebAssembly Text at %s\n", watPath)
		fmt.Println("Note: Install 'wasmtime' or use 'lone build --with-adapter node' to execute directly.")
		return nil
	},
}
