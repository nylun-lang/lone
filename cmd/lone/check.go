package main

import (
	"encoding/json"
	"fmt"
	"os"

	"lone/pkg/diag"
	p "lone/pkg/parser"
	"lone/pkg/sema"

	"github.com/spf13/cobra"
)

var (
	checkAstJsonFlag  bool
	checkAirJsonFlag  bool
	checkDiagJsonFlag bool
)

var checkCmd = &cobra.Command{
	Use:     "check <file.lone>",
	Aliases: []string{"analyze", "lint"},
	Short:   "Perform syntax verification, type checking, and semantic analysis",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", filePath, err)
		}

		rawAst, err := p.Parse(filePath, content)
		if err != nil {
			return fmt.Errorf("syntax error: %w", err)
		}

		if checkAstJsonFlag {
			data, err := json.MarshalIndent(rawAst, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		analyzer := sema.NewAnalyzer(filePath, sema.DefaultConfig())
		typedProg, diags := analyzer.AnalyzeTyped(rawAst)

		if checkAirJsonFlag {
			out, err := diag.PrintNodesJSON(typedProg)
			if err != nil {
				return err
			}
			fmt.Println(out)
			return nil
		}

		if checkDiagJsonFlag {
			data, err := json.MarshalIndent(diags, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		hasErrors := false
		for _, d := range diags {
			prefix := "INFO"
			switch d.Severity {
			case sema.SeverityError:
				prefix = "ERROR"
				hasErrors = true
			case sema.SeverityWarning:
				prefix = "WARN"
			case sema.SeverityHint:
				prefix = "HINT"
			}
			fmt.Fprintf(os.Stderr, "[%s] %s:%d:%d: %s\n", prefix, filePath, d.Pos.Line, d.Pos.Col, d.Message)
		}

		if hasErrors {
			return fmt.Errorf("check failed with errors")
		}

		fmt.Println("Check passed with 0 errors.")
		return nil
	},
}

func init() {
	checkCmd.Flags().BoolVar(&checkAstJsonFlag, "ast-json", false, "Output raw AST as JSON")
	checkCmd.Flags().BoolVar(&checkAirJsonFlag, "air-json", false, "Output Typed IR (AIR) as JSON")
	checkCmd.Flags().BoolVar(&checkDiagJsonFlag, "diag-json", false, "Output diagnostics as JSON")
}
