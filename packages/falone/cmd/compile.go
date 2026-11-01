package main

import (
	"fmt"
	"os"

	"falone/lowering"
	"falone/mirtypes"
	"falone/transpile"
	"lonealyzer/analyze"
	p "lonelang/parser"

	"github.com/spf13/cobra"
)

var (
	compileMirFlag    bool
	compileWatFlag    bool
	compileOutFlag    string
	compileTargetFlag string
)

var compileCmd = &cobra.Command{
	Use:     "compile [target] <file.lone>",
	Aliases: []string{"build", "c"},
	Short:   "Compile Lone source code to Falone MIR or WebAssembly (WAT)",
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		target := transpile.Target(compileTargetFlag)
		if target == "" {
			target = transpile.TargetWasi
		}
		return runCompile(args[0], target)
	},
}

var compileWasmCmd = &cobra.Command{
	Use:     "wasm <file.lone>",
	Aliases: []string{"wasi", "standalone"},
	Short:   "Compile Lone source code targeting standalone WebAssembly (WASI)",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCompile(args[0], transpile.TargetWasi)
	},
}

var compileWebCmd = &cobra.Command{
	Use:     "web <file.lone>",
	Aliases: []string{"browser", "js"},
	Short:   "Compile Lone source code targeting Web/Browser environments",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCompile(args[0], transpile.TargetWeb)
	},
}

func init() {
	compileCmd.PersistentFlags().BoolVarP(&compileMirFlag, "mir", "m", false, "Print Falone MIR assembly")
	compileCmd.PersistentFlags().BoolVarP(&compileWatFlag, "wat", "w", false, "Print WebAssembly Text (WAT)")
	compileCmd.PersistentFlags().StringVarP(&compileOutFlag, "output", "o", "", "Write WebAssembly output to specified file")
	compileCmd.Flags().StringVarP(&compileTargetFlag, "target", "t", "wasi", "Compilation target ('wasi' or 'web')")

	compileCmd.AddCommand(compileWasmCmd)
	compileCmd.AddCommand(compileWebCmd)
}

func runCompile(filePath string, target transpile.Target) error {
	if _, err := os.Lstat(filePath); err != nil {
		return fmt.Errorf("file '%s' does not exist: %w", filePath, err)
	}

	// 1. Parse AST
	rawAst, err := p.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// 2. Type analysis with lone-alyzer
	analyzer := analyze.NewAnalyzer(filePath, analyze.DefaultConfig())
	prog, diags := analyzer.AnalyzeTyped(rawAst)

	if len(diags) > 0 {
		hasError := false
		for _, d := range diags {
			fmt.Fprintf(os.Stderr, "[%s] %d:%d (%s): %s\n", d.Severity, d.Pos.Line, d.Pos.Col, d.Rule, d.Message)
			if d.Severity == analyze.SeverityError {
				hasError = true
			}
		}
		if hasError {
			return fmt.Errorf("compilation aborted due to static analysis errors")
		}
	}

	// 3. Lower Typed IR to Falone MIR
	mirMod, err := lowering.LowerProgram(prog)
	if err != nil {
		return fmt.Errorf("lowering error: %w", err)
	}

	// 4. Transpile Falone MIR to WAT
	watText, err := transpile.TranspileToWatWithTarget(mirMod, target)
	if err != nil {
		return fmt.Errorf("transpilation error: %w", err)
	}

	// If output file specified
	if compileOutFlag != "" {
		if err := os.WriteFile(compileOutFlag, []byte(watText), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Successfully compiled to %s\n", compileOutFlag)
		return nil
	}

	// Determine what to print
	printMir := compileMirFlag
	printWat := compileWatFlag

	if !printMir && !printWat {
		// Default: print both MIR and WAT
		printMir = true
		printWat = true
	}

	if printMir {
		fmt.Println("=== FALONE MIR ASSEMBLY ===")
		fmt.Println(mirtypes.FormatModule(mirMod))
		fmt.Println()
	}

	if printWat {
		fmt.Println("=== WEBASSEMBLY TEXT (WAT) ===")
		fmt.Print(watText)
	}

	return nil
}
