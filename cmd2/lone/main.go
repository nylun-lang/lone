package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	loneanalyzer "lone/pkg2/lone-analyzer"
	lonecompiler "lone/pkg2/lone-compiler"
	loneparser "lone/pkg2/lone-parser"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "build":
		handleBuild(os.Args[2:])

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

func handleBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	outFlag := fs.String("o", "", "Output WAT file path")
	targetFlag := fs.String("target", "wasi", "Compilation target (wasi, web)")
	optFlag := fs.Bool("O", true, "Enable peephole optimization")

	var flagsOnly, posArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagsOnly = append(flagsOnly, arg)
			if (arg == "-o" || arg == "-target" || arg == "--target") && i+1 < len(args) {
				i++
				flagsOnly = append(flagsOnly, args[i])
			}
		} else {
			posArgs = append(posArgs, arg)
		}
	}

	if err := fs.Parse(flagsOnly); err != nil {
		fmt.Fprintf(os.Stderr, "Flag parse error: %v\n", err)
		os.Exit(1)
	}

	if len(posArgs) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: lone build <file.lone> [-o out.wat] [--target wasi|web] [-O]\n")
		os.Exit(1)
	}

	filePath := posArgs[0]
	outFile := *outFlag
	if outFile == "" {
		ext := filepath.Ext(filePath)
		outFile = strings.TrimSuffix(filePath, ext) + ".wat"
	}

	cfg := lonecompiler.Config{
		Target:      *targetFlag,
		Optimize:    *optFlag,
		OutFilePath: outFile,
	}

	comp := lonecompiler.NewCompiler(cfg)
	res, err := comp.CompileFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Build error: %v\n", err)
		if res != nil && len(res.Diagnostics) > 0 {
			for _, d := range res.Diagnostics {
				fmt.Fprintf(os.Stderr, "%s\n", d)
			}
		}
		os.Exit(1)
	}

	fmt.Printf("Compiled %s -> %s\n", filePath, outFile)
}

func printUsage() {
	fmt.Println("Usage: lone2 <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  build <file.lone> [-o out.wat]   Compile Lone source into WebAssembly WAT")
	fmt.Println("  check <file.lone>                Analyze file for semantic and type correctness")
	fmt.Println("  parse <file.lone>                Parse file using Pratt parser and verify syntax")
	fmt.Println("  version                          Display lone2 version")
}
