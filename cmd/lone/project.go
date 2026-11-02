package main

import (
	"fmt"
	"os"

	loneproject "lone/pkg/lone-analyzer-project"
	lonelogger "lone/pkg/lone-shared-logger"

	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project [dir]",
	Short: "Analyze and topologically order a multi-file Lone project",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		proj := loneproject.NewProject(dir)
		if err := proj.ScanDir(dir); err != nil {
			return fmt.Errorf("failed to scan directory: %w", err)
		}

		if len(proj.Files) == 0 {
			return fmt.Errorf("no .lone files discovered in %s", dir)
		}

		fmt.Printf("Discovered %d source file(s) in %s\n", len(proj.Files), dir)

		merged, diags, err := proj.AnalyzeAndOrder()
		if err != nil {
			emitter := lonelogger.NewEmitter(os.Stderr, lonelogger.DefaultConfig())
			for _, d := range diags {
				_ = emitter.Emit(d.ToLoggerDiag("project"), nil)
			}
			return fmt.Errorf("project dependency error: %w", err)
		}

		fmt.Println("\nTopological Compilation Order:")
		for i, f := range merged.OrderedFiles {
			fmt.Printf("  %d. %s (provides %d symbols, requires %d)\n",
				i+1, f.Path, len(f.Provides), len(f.Requires))
		}

		fmt.Printf("\nProject successfully ordered (0 conflicts, 0 cycles).\n")
		return nil
	},
}
