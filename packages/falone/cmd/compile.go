package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"falone/adapter"
	"falone/lowering"
	"falone/mirtypes"
	"falone/transpile"
	"lonealyzer/analyze"
	p "lonelang/parser"

	"github.com/spf13/cobra"
)

var (
	compileMirFlag     bool
	compileWatFlag     bool
	compileOutFlag     string
	compileTargetFlag  string
	compileAdapterFlag string
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
	compileCmd.PersistentFlags().StringVar(&compileAdapterFlag, "with-adapter", "", "Generate runtime glue adapter ('node' or 'browser')")
	compileCmd.PersistentFlags().StringVar(&compileAdapterFlag, "with-adater", "", "Alias for --with-adapter")
	_ = compileCmd.PersistentFlags().MarkHidden("with-adater")
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

	// 5. Generate runtime adapter if requested
	var adapterCode string
	var adapterKind adapter.AdapterKind
	if compileAdapterFlag != "" {
		adapterKind, err = adapter.ParseAdapterKind(compileAdapterFlag)
		if err != nil {
			return err
		}

		wasmFileName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)) + ".wasm"
		if compileOutFlag != "" {
			ext := filepath.Ext(compileOutFlag)
			if ext == ".wat" || ext == ".wasm" {
				wasmFileName = strings.TrimSuffix(filepath.Base(compileOutFlag), ext) + ".wasm"
			} else if ext == ".js" {
				wasmFileName = strings.TrimSuffix(filepath.Base(compileOutFlag), ".js") + ".wasm"
			}
		}

		adapterCode, err = adapter.GenerateAdapter(adapterKind, wasmFileName)
		if err != nil {
			return fmt.Errorf("adapter generation error: %w", err)
		}
	}

	// If output file specified
	if compileOutFlag != "" {
		watOutPath := compileOutFlag
		var adapterOutPath string
		if adapterCode != "" {
			ext := filepath.Ext(compileOutFlag)
			if ext == ".js" {
				adapterOutPath = compileOutFlag
				watOutPath = strings.TrimSuffix(compileOutFlag, ".js") + ".wat"
			} else {
				base := strings.TrimSuffix(compileOutFlag, ext)
				adapterOutPath = base + ".adapter.js"
			}
		}

		if err := os.WriteFile(watOutPath, []byte(watText), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		if adapterOutPath != "" {
			if err := os.WriteFile(adapterOutPath, []byte(adapterCode), 0644); err != nil {
				return fmt.Errorf("failed to write adapter file: %w", err)
			}
			fmt.Printf("Successfully compiled to %s and generated %s adapter %s\n", watOutPath, adapterKind, adapterOutPath)
		} else {
			fmt.Printf("Successfully compiled to %s\n", watOutPath)
		}
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

	if adapterCode != "" {
		if printMir || printWat {
			fmt.Println()
		}
		fmt.Printf("=== RUNTIME ADAPTER (%s) ===\n", adapterKind)
		fmt.Print(adapterCode)
	}

	return nil
}
