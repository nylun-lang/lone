package main

import (
	"fmt"
	loneanalyzer "lone/pkg2/lone-analyzer"
	loneparser "lone/pkg2/lone-parser"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "parse":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: lone parse <file.lone>\n")
			os.Exit(1)
		}
		filePath := os.Args[2]
		src, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filePath, err)
			os.Exit(1)
		}
		decls, err := loneparser.Parse(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully parsed %s: %d top-level declaration(s)\n", filePath, len(decls))

	case "check":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: lone check <file.lone>\n")
			os.Exit(1)
		}
		filePath := os.Args[2]
		src, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filePath, err)
			os.Exit(1)
		}
		analyzer := loneanalyzer.NewAnalyzer()
		_, diags, err := analyzer.AnalyzeSource(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Syntax error in %s: %v\n", filePath, err)
			os.Exit(1)
		}
		if len(diags) > 0 {
			for _, d := range diags {
				fmt.Fprintf(os.Stderr, "%s\n", d)
			}
			os.Exit(1)
		}
		fmt.Printf("Checked %s: 0 errors\n", filePath)

	case "version":
		fmt.Println("lone v2 (greenfield re-implementation)")

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: lone2 <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  check <file.lone>    Analyze file for semantic and type correctness")
	fmt.Println("  parse <file.lone>    Parse file using Pratt parser and verify syntax")
	fmt.Println("  version              Display lone2 version")
}
