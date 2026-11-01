package main

import (
	"flag"
	"fmt"
	"os"

	"falone/lowering"
	"falone/mirtypes"
	"falone/transpile"
	"lonealyzer/analyze"
	p "lonelang/parser"
)

func main() {
	var (
		mirFlag = flag.Bool("mir", false, "Print Falone MIR assembly")
		watFlag = flag.Bool("wat", false, "Print WebAssembly Text (WAT)")
		outFlag = flag.String("o", "", "Write output to specified file")
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
		fmt.Fprintf(os.Stderr, "error: file '%s' does not exist: %s\n", filePath, err.Error())
		os.Exit(1)
	}

	// 1. Parse Lone source into AST
	rawAst, err := p.ParseFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %s\n", err.Error())
		os.Exit(1)
	}

	// 2. Run static analysis & type evaluation -> Typed IR
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
			os.Exit(1)
		}
	}

	// 3. Lower Typed IR to Falone MIR
	mirMod, err := lowering.LowerProgram(prog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lowering error: %s\n", err.Error())
		os.Exit(1)
	}

	// 4. Transpile Falone MIR to WAT
	watText, err := transpile.TranspileToWat(mirMod)
	if err != nil {
		fmt.Fprintf(os.Stderr, "transpile error: %s\n", err.Error())
		os.Exit(1)
	}

	// Write to file if -o specified
	if *outFlag != "" {
		if err := os.WriteFile(*outFlag, []byte(watText), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write error: %s\n", err.Error())
			os.Exit(1)
		}
		fmt.Printf("Wrote WebAssembly output to %s\n", *outFlag)
		return
	}

	printMir := false
	printWat := false

	if *mirFlag && !*watFlag {
		printMir = true
	} else if *watFlag && !*mirFlag {
		printWat = true
	} else {
		// Default: print both
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
}
