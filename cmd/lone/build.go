package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	loneanalyzer "lone/pkg/lone-analyzer"
	loneproject "lone/pkg/lone-analyzer-project"
	lonecompiler "lone/pkg/lone-compiler"
	lonelogger "lone/pkg/lone-shared-logger"

	"github.com/spf13/cobra"
)

var (
	buildMirFlag     bool
	buildOutFlag     string
	buildTargetFlag  string
	buildAdapterFlag string
	buildNoOptFlag   bool
)

var buildCmd = &cobra.Command{
	Use:     "build [target] <file.lone | dir>",
	Aliases: []string{"compile", "c"},
	Short:   "Compile Lone source code or a Lone project to WebAssembly (WAT)",
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		target := lonecompiler.Target(buildTargetFlag)
		if target == "" {
			target = lonecompiler.TargetWasi
		}
		return runBuild(args[0], target)
	},
}

var buildWasmCmd = &cobra.Command{
	Use:     "wasm <file.lone | dir>",
	Aliases: []string{"wasi", "standalone"},
	Short:   "Compile Lone source code targeting standalone WebAssembly (WASI)",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuild(args[0], lonecompiler.TargetWasi)
	},
}

var buildWebCmd = &cobra.Command{
	Use:     "web <file.lone | dir>",
	Aliases: []string{"browser", "js"},
	Short:   "Compile Lone source code targeting Web/Browser environments",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuild(args[0], lonecompiler.TargetWeb)
	},
}

func init() {
	buildCmd.Flags().BoolVar(&buildMirFlag, "mir", false, "Output lowered Falone MIR instead of WAT")
	buildCmd.Flags().StringVarP(&buildOutFlag, "out", "o", "", "Output file path (default: stdout or <file>.wat)")
	buildCmd.Flags().StringVarP(&buildTargetFlag, "target", "t", "wasi", "Target runtime environment: wasi | web")
	buildCmd.Flags().StringVar(&buildAdapterFlag, "with-adapter", "", "Generate JavaScript runtime loader glue (node | browser)")
	buildCmd.Flags().BoolVar(&buildNoOptFlag, "no-opt", false, "Disable WebAssembly AST optimizer passes")

	buildWasmCmd.Flags().BoolVar(&buildMirFlag, "mir", false, "Output lowered Falone MIR instead of WAT")
	buildWasmCmd.Flags().StringVarP(&buildOutFlag, "out", "o", "", "Output file path (default: stdout or <file>.wat)")
	buildWasmCmd.Flags().StringVar(&buildAdapterFlag, "with-adapter", "", "Generate JavaScript runtime loader glue (node)")

	buildWebCmd.Flags().BoolVar(&buildMirFlag, "mir", false, "Output lowered Falone MIR instead of WAT")
	buildWebCmd.Flags().StringVarP(&buildOutFlag, "out", "o", "", "Output file path (default: stdout or <file>.wat)")
	buildWebCmd.Flags().StringVar(&buildAdapterFlag, "with-adapter", "browser", "Generate JavaScript runtime loader glue (browser | node)")

	buildCmd.AddCommand(buildWasmCmd)
	buildCmd.AddCommand(buildWebCmd)
}

func runBuild(path string, target lonecompiler.Target) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to access path %s: %w", path, err)
	}

	compCfg := lonecompiler.Config{
		Target:      target,
		EmitMir:     buildMirFlag,
		Optimize:    !buildNoOptFlag,
		WithAdapter: buildAdapterFlag,
		OutFilePath: buildOutFlag,
	}
	comp := lonecompiler.NewCompiler(compCfg)

	var res *lonecompiler.Result

	if fi.IsDir() {
		// Multi-file project build
		proj := loneproject.NewProject(path)
		if err := proj.ScanDir(path); err != nil {
			return fmt.Errorf("failed to scan project directory %s: %w", path, err)
		}
		if len(proj.Files) == 0 {
			return fmt.Errorf("no .lone files found in directory %s", path)
		}

		merged, diags, err := proj.AnalyzeAndOrder()
		if err != nil {
			emitter := lonelogger.NewEmitter(os.Stderr, lonelogger.DefaultConfig())
			for _, d := range diags {
				_ = emitter.Emit(d.ToLoggerDiag("project"), nil)
			}
			return fmt.Errorf("project resolution error: %w", err)
		}

		// Compile the merged AST by assembling source files in topological order
		var combinedSource strings.Builder
		for _, f := range merged.OrderedFiles {
			combinedSource.WriteString(fmt.Sprintf("// --- %s ---\n", f.Path))
			combinedSource.Write(f.Content)
			combinedSource.WriteString("\n\n")
		}

		res, err = comp.CompileSource(filepath.Join(path, "project.lone"), []byte(combinedSource.String()))
		if err != nil {
			return err
		}
	} else {
		// Single-file build
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		res, err = comp.CompileSource(path, content)
		if err != nil {
			if res != nil && len(res.Diagnostics) > 0 {
				emitter := lonelogger.NewEmitter(os.Stderr, lonelogger.DefaultConfig())
				for _, d := range res.Diagnostics {
					_ = emitter.Emit(d.ToLoggerDiag(path), content)
				}
			}
			return err
		}

		// Print any non-fatal warnings
		if res != nil {
			emitter := lonelogger.NewEmitter(os.Stderr, lonelogger.DefaultConfig())
			for _, d := range res.Diagnostics {
				if d.Severity == loneanalyzer.SeverityWarning {
					_ = emitter.Emit(d.ToLoggerDiag(path), content)
				}
			}
		}
	}

	if buildOutFlag != "" {
		if err := os.WriteFile(buildOutFlag, []byte(res.WatOutput), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Printf("Wrote WebAssembly output to %s\n", buildOutFlag)

		if res.AdapterGlue != "" && res.AdapterFile != "" {
			adapterPath := filepath.Join(filepath.Dir(buildOutFlag), res.AdapterFile)
			if err := os.WriteFile(adapterPath, []byte(res.AdapterGlue), 0644); err != nil {
				return fmt.Errorf("failed to write adapter glue: %w", err)
			}
			fmt.Printf("Generated runtime adapter glue at %s\n", adapterPath)
		}
	} else {
		fmt.Print(res.WatOutput)
	}

	return nil
}
