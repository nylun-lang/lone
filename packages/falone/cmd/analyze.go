package main

import (
	"fmt"
	"os"

	"lonealyzer/analyze"
	"lonealyzer/print"
	p "lonelang/parser"

	"github.com/spf13/cobra"
)

var (
	analyzeIrFlag    bool
	analyzeNodesFlag bool
	analyzeJsonFlag  bool
	analyzeModeFlag  string
)

var analyzeCmd = &cobra.Command{
	Use:     "analyze <file.lone>",
	Aliases: []string{"check", "a"},
	Short:   "Run static analysis and type verification with lone-alyzer",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAnalyze(args[0])
	},
}

func init() {
	analyzeCmd.Flags().BoolVarP(&analyzeIrFlag, "ir", "i", false, "Print formatted Alyzer IR")
	analyzeCmd.Flags().BoolVarP(&analyzeNodesFlag, "nodes", "n", false, "Print Typed IR node tree")
	analyzeCmd.Flags().BoolVarP(&analyzeJsonFlag, "json", "j", false, "Print Typed IR nodes formatted as JSON")
	analyzeCmd.Flags().StringVarP(&analyzeModeFlag, "mode", "m", "", "Output mode: 'ir', 'nodes', or 'both'")
}

func runAnalyze(filePath string) error {
	if _, err := os.Lstat(filePath); err != nil {
		return fmt.Errorf("file '%s' does not exist: %w", filePath, err)
	}

	// 1. Parse AST
	rawAst, err := p.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// 2. Run static analysis
	analyzer := analyze.NewAnalyzer(filePath, analyze.DefaultConfig())
	prog, diags := analyzer.AnalyzeTyped(rawAst)

	// Determine what to output
	printIR := analyzeIrFlag
	printNodes := analyzeNodesFlag
	printJson := analyzeJsonFlag

	if analyzeModeFlag == "both" {
		printIR = true
		printNodes = true
	} else if analyzeModeFlag == "ir" {
		printIR = true
	} else if analyzeModeFlag == "nodes" {
		printNodes = true
	}

	// If no specific print flag is passed, default to printing both IR and nodes
	if !printIR && !printNodes && !printJson && analyzeModeFlag == "" {
		printIR = true
		printNodes = true
	}

	// Print Alyzer-IR
	if printIR {
		fmt.Println("=== ALYZER IR ===")
		fmt.Println(print.PrintProgram(prog))
		fmt.Println()
	}

	// Print Nodes
	if printNodes || printJson {
		if printJson {
			fmt.Println("=== TYPED IR NODES (JSON) ===")
			jsonStr, err := print.PrintNodesJSON(prog)
			if err != nil {
				fmt.Fprintf(os.Stderr, "json error: %s\n", err.Error())
			} else {
				fmt.Println(jsonStr)
			}
		} else {
			fmt.Println("=== TYPED IR NODES (TREE) ===")
			fmt.Print(print.PrintNodeTree(prog, 0))
		}
		fmt.Println()
	}

	// Print Diagnostics
	hasErrors := false
	if len(diags) > 0 {
		fmt.Println("=== DIAGNOSTICS ===")
		fmt.Println(print.FormatDiagnostics(diags))
		fmt.Println()

		for _, d := range diags {
			if d.Severity == analyze.SeverityError {
				hasErrors = true
			}
		}
	} else {
		fmt.Println("=== DIAGNOSTICS ===")
		fmt.Println("All checks passed successfully! (0 diagnostics)")
		fmt.Println()
	}

	if hasErrors {
		return fmt.Errorf("analysis completed with errors")
	}

	return nil
}
