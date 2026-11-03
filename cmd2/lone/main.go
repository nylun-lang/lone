package main

import (
	"fmt"
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
	fmt.Println("  parse <file.lone>    Parse file using Pratt parser and verify syntax")
	fmt.Println("  version              Display lone2 version")
}
