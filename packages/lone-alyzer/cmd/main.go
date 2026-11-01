package main

import (
	"flag"
	"fmt"
	"os"

	"lonealyzer/analyze"
	"lonealyzer/print"
	p "lonelang/parser"
)

func main() {
	var (
		modeFlag  = flag.String("mode", "", "Output mode: 'ir', 'nodes', or 'both'")
		irFlag    = flag.Bool("ir", false, "Print formatted Alyzer IR")
		nodesFlag = flag.Bool("nodes", false, "Print Typed IR node tree")
		bothFlag  = flag.Bool("both", false, "Print both Alyzer IR and node tree")
		jsonFlag  = flag.Bool("json", false, "Print Typed IR nodes formatted as JSON")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <file.lone>\n\nFlags:\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	filePath := args[0]
	if _, err := os.Lstat(filePath); err != nil {
		fmt.Fprintf(os.Stderr, "error: file '%s' does not exist; %s\n", filePath, err.Error())
		os.Exit(1)
	}

	// 1. Parse AST with lonelang/parser
	rawAst, err := p.ParseFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %s\n", err.Error())
		os.Exit(1)
	}

	// 2. Run lone-alyzer
	analyzer := analyze.NewAnalyzer(filePath, analyze.DefaultConfig())
	prog, diags := analyzer.AnalyzeTyped(rawAst)

	// Determine output mode
	printIR := false
	printNodes := false

	if *bothFlag || *modeFlag == "both" {
		printIR = true
		printNodes = true
	} else if *irFlag || *modeFlag == "ir" {
		printIR = true
	} else if *nodesFlag || *modeFlag == "nodes" || *jsonFlag {
		printNodes = true
	} else {
		// Default when no flag is provided: both
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
	if printNodes {
		if *jsonFlag {
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
	if len(diags) > 0 {
		fmt.Println("=== DIAGNOSTICS ===")
		fmt.Println(print.FormatDiagnostics(diags))
		fmt.Println()
	}
}
