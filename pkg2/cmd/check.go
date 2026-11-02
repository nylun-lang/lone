package main

import (
	"encoding/json"
	"fmt"
	"os"

	loneanalyzer "lone/pkg2/lone-analyzer"
	loneanalyzerdiag "lone/pkg2/lone-analyzer-diag"
	loneparser "lone/pkg2/lone-parser"
	lonelogger "lone/pkg2/lone-shared-logger"

	"github.com/spf13/cobra"
)

var (
	checkAstJsonFlag  bool
	checkAirJsonFlag  bool
	checkTreeFlag     bool
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

		rawAst, err := loneparser.Parse(filePath, content)
		if err != nil {
			emitter := lonelogger.NewEmitter(os.Stderr, lonelogger.DefaultConfig())
			diag := lonelogger.NewError(err.Error(), lonelogger.PointSpan(filePath, 1, 1))
			_ = emitter.Emit(diag, content)
			return fmt.Errorf("syntax error in %s", filePath)
		}

		if checkAstJsonFlag {
			data, err := json.MarshalIndent(rawAst, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		analyzer := loneanalyzer.NewAnalyzer(filePath, loneanalyzer.DefaultConfig())
		typedProg, diags := analyzer.AnalyzeTyped(rawAst)

		if checkAirJsonFlag {
			out, err := loneanalyzerdiag.PrintNodesJSON(typedProg)
			if err != nil {
				return err
			}
			fmt.Println(out)
			return nil
		}

		if checkTreeFlag {
			fmt.Print(loneanalyzerdiag.PrintNodeTree(typedProg, 0))
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

		emitter := lonelogger.NewEmitter(os.Stderr, lonelogger.DefaultConfig())
		hasErrors := false

		for _, d := range diags {
			if d.Severity == loneanalyzer.SeverityError {
				hasErrors = true
			}
			_ = emitter.Emit(d.ToLoggerDiag(filePath), content)
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
	checkCmd.Flags().BoolVar(&checkTreeFlag, "tree", false, "Output Typed IR as an indented structural tree")
	checkCmd.Flags().BoolVar(&checkDiagJsonFlag, "diag-json", false, "Output diagnostics as JSON")
}
