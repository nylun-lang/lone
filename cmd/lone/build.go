package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lone/pkg/adapter"
	"lone/pkg/mir"
	p "lone/pkg/parser"
	"lone/pkg/sema"
	"lone/pkg/wir"

	"github.com/spf13/cobra"
)

var (
	buildMirFlag     bool
	buildWatFlag     bool
	buildOutFlag     string
	buildTargetFlag  string
	buildAdapterFlag string
)

var buildCmd = &cobra.Command{
	Use:     "build [target] <file.lone>",
	Aliases: []string{"compile", "c"},
	Short:   "Compile Lone source code to Falone MIR or WebAssembly (WAT)",
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		target := wir.Target(buildTargetFlag)
		if target == "" {
			target = wir.TargetWasi
		}
		return runBuild(args[0], target)
	},
}

var buildWasmCmd = &cobra.Command{
	Use:     "wasm <file.lone>",
	Aliases: []string{"wasi", "standalone"},
	Short:   "Compile Lone source code targeting standalone WebAssembly (WASI)",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuild(args[0], wir.TargetWasi)
	},
}

var buildWebCmd = &cobra.Command{
	Use:     "web <file.lone>",
	Aliases: []string{"browser", "js"},
	Short:   "Compile Lone source code targeting Web/Browser environments",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuild(args[0], wir.TargetWeb)
	},
}

func init() {
	buildCmd.Flags().BoolVar(&buildMirFlag, "mir", false, "Output lowered Falone MIR instead of WAT")
	buildCmd.Flags().BoolVar(&buildWatFlag, "wat", false, "Output WebAssembly Text format (.wat)")
	buildCmd.Flags().StringVarP(&buildOutFlag, "out", "o", "", "Output file path (default: stdout or <file>.wat)")
	buildCmd.Flags().StringVarP(&buildTargetFlag, "target", "t", "wasi", "Target runtime environment: wasi | web")
	buildCmd.Flags().StringVar(&buildAdapterFlag, "with-adapter", "", "Generate JavaScript runtime loader glue (node | browser)")

	buildWasmCmd.Flags().BoolVar(&buildMirFlag, "mir", false, "Output lowered Falone MIR instead of WAT")
	buildWasmCmd.Flags().StringVarP(&buildOutFlag, "out", "o", "", "Output file path (default: stdout or <file>.wat)")
	buildWasmCmd.Flags().StringVar(&buildAdapterFlag, "with-adapter", "", "Generate JavaScript runtime loader glue (node)")

	buildWebCmd.Flags().BoolVar(&buildMirFlag, "mir", false, "Output lowered Falone MIR instead of WAT")
	buildWebCmd.Flags().StringVarP(&buildOutFlag, "out", "o", "", "Output file path (default: stdout or <file>.wat)")
	buildWebCmd.Flags().StringVar(&buildAdapterFlag, "with-adapter", "browser", "Generate JavaScript runtime loader glue (browser | node)")

	buildCmd.AddCommand(buildWasmCmd)
	buildCmd.AddCommand(buildWebCmd)
}

func runBuild(filePath string, target wir.Target) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	rawAst, err := p.Parse(filePath, content)
	if err != nil {
		return fmt.Errorf("syntax error: %w", err)
	}

	analyzer := sema.NewAnalyzer(filePath, sema.DefaultConfig())
	typedProg, diags := analyzer.AnalyzeTyped(rawAst)

	hasErrors := false
	for _, diag := range diags {
		if diag.Severity == sema.SeverityError {
			hasErrors = true
			fmt.Fprintf(os.Stderr, "Error [%s:%d:%d]: %s\n", filePath, diag.Pos.Line, diag.Pos.Col, diag.Message)
		} else if diag.Severity == sema.SeverityWarning {
			fmt.Fprintf(os.Stderr, "Warning [%s:%d:%d]: %s\n", filePath, diag.Pos.Line, diag.Pos.Col, diag.Message)
		}
	}
	if hasErrors {
		return fmt.Errorf("compilation failed with type/semantic errors")
	}

	mirMod, err := mir.LowerProgram(typedProg)
	if err != nil {
		return fmt.Errorf("lowering error: %w", err)
	}

	var output string
	if buildMirFlag {
		output = mir.FormatModule(mirMod)
	} else {
		output, err = wir.TranspileToWatWithTarget(mirMod, target)
		if err != nil {
			return fmt.Errorf("transpiler error: %w", err)
		}
	}

	if buildOutFlag != "" {
		if err := os.WriteFile(buildOutFlag, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Printf("Wrote output to %s\n", buildOutFlag)

		if buildAdapterFlag != "" && !buildMirFlag {
			kind, err := adapter.ParseAdapterKind(buildAdapterFlag)
			if err != nil {
				return err
			}
			wasmBaseName := filepath.Base(buildOutFlag)
			if strings.HasSuffix(wasmBaseName, ".wat") {
				wasmBaseName = strings.TrimSuffix(wasmBaseName, ".wat") + ".wasm"
			}
			glue, err := adapter.GenerateAdapter(kind, wasmBaseName)
			if err != nil {
				return fmt.Errorf("failed to generate adapter: %w", err)
			}
			adapterExt := ".mjs"
			if kind == adapter.AdapterBrowser {
				adapterExt = ".html"
			}
			adapterPath := strings.TrimSuffix(buildOutFlag, filepath.Ext(buildOutFlag)) + adapterExt
			if err := os.WriteFile(adapterPath, []byte(glue), 0644); err != nil {
				return fmt.Errorf("failed to write adapter glue: %w", err)
			}
			fmt.Printf("Generated %s adapter runtime glue at %s\n", kind, adapterPath)
		}
	} else {
		fmt.Print(output)
	}

	return nil
}
