package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	loneanalyzer "lone/pkg2/lone-analyzer"
	lonecompiler "lone/pkg2/lone-compiler"
	lonelexer "lone/pkg2/lone-lexer"
	lonelsp "lone/pkg2/lone-lsp"
	loneparser "lone/pkg2/lone-parser"
	loneproject "lone/pkg2/lone-project"
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

	case "run":
		handleRun(os.Args[2:])

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
		tokens, err := lonelexer.NewLexer(src).Tokenize()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Lexer error in %s: %v\n", filePath, err)
			os.Exit(1)
		}
		rawAst, err := loneparser.NewParser(tokens).ParseProgram()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Syntax error in %s: %v\n", filePath, err)
			os.Exit(1)
		}
		resolvedAst, resolveDiags := loneproject.ResolveAST(filePath, rawAst)
		evaluator := loneanalyzer.NewTypeEvaluator()
		_ = evaluator.EvaluateProgram(resolvedAst)
		diags := append(resolveDiags, evaluator.Diagnostics()...)
		if len(diags) > 0 {
			for _, d := range diags {
				fmt.Fprintf(os.Stderr, "%s\n", d)
			}
			os.Exit(1)
		}
		fmt.Printf("Checked %s: 0 errors\n", filePath)

	case "debug":
		handleDebug(os.Args[2:])

	case "lsp":
		srv := lonelsp.NewServer()
		if err := srv.Serve(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "LSP server error: %v\n", err)
			os.Exit(1)
		}

	case "version":
		fmt.Println("lone v0.6.0")

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

func handleRun(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: lone run <file.lone>\n")
		os.Exit(1)
	}

	filePath := args[0]
	tmpDir, err := os.MkdirTemp("", "lone-run-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Temp dir error: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	watPath := filepath.Join(tmpDir, "out.wat")
	cfg := lonecompiler.Config{
		Target:      "wasi",
		Optimize:    true,
		OutFilePath: watPath,
	}

	comp := lonecompiler.NewCompiler(cfg)
	res, err := comp.CompileFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compilation error: %v\n", err)
		if res != nil && len(res.Diagnostics) > 0 {
			for _, d := range res.Diagnostics {
				fmt.Fprintf(os.Stderr, "%s\n", d)
			}
		}
		os.Exit(1)
	}

	wasmtimePath, err := exec.LookPath("wasmtime")
	if err == nil {
		cmd := exec.Command(wasmtimePath, watPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}
		return
	}

	fmt.Fprintf(os.Stderr, "Compiled to %s\nNotice: Install 'wasmtime' to execute programs directly via 'lone run'.\n", watPath)
}

func handleDebug(args []string) {
	jsonOutput := false
	var posArgs []string
	for _, a := range args {
		if a == "--json" || a == "-json" {
			jsonOutput = true
		} else if !strings.HasPrefix(a, "-") {
			posArgs = append(posArgs, a)
		}
	}

	if len(posArgs) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: lone debug <file.lone> [symbol] [--json]\n")
		os.Exit(1)
	}

	filePath := posArgs[0]
	src, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filePath, err)
		os.Exit(1)
	}

	tokens, err := lonelexer.NewLexer(src).Tokenize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Lexer error in %s: %v\n", filePath, err)
		os.Exit(1)
	}
	rawAst, err := loneparser.NewParser(tokens).ParseProgram()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Syntax error in %s: %v\n", filePath, err)
		os.Exit(1)
	}

	resolvedAst, _ := loneproject.ResolveAST(filePath, rawAst)
	evaluator := loneanalyzer.NewTypeEvaluator()
	typedProg := evaluator.EvaluateProgram(resolvedAst)

	filterSymbol := ""
	if len(posArgs) > 1 {
		filterSymbol = posArgs[1]
	}

	structs := evaluator.Structs()
	interfaces := evaluator.Interfaces()
	enums := evaluator.Enums()

	if jsonOutput {
		type fieldDTO struct {
			Name       string `json:"name"`
			Type       string `json:"type"`
			Offset     int    `json:"offset"`
			ReadOnly   bool   `json:"read_only"`
			PublicRead bool   `json:"public_read"`
		}
		type structDTO struct {
			Name    string     `json:"name"`
			Size    int        `json:"size"`
			Align   int        `json:"align"`
			Fields  []fieldDTO `json:"fields"`
			Methods []string   `json:"methods"`
		}
		type enumVariantDTO struct {
			Name string `json:"name"`
			Tag  int    `json:"tag"`
		}
		type enumDTO struct {
			Name     string           `json:"name"`
			Size     int              `json:"size"`
			Align    int              `json:"align"`
			Variants []enumVariantDTO `json:"variants"`
		}
		type ifcDTO struct {
			Name   string     `json:"name"`
			Fields []fieldDTO `json:"fields"`
		}

		sDTOs := make(map[string]structDTO)
		for name, st := range structs {
			if filterSymbol != "" && !strings.EqualFold(name, filterSymbol) {
				continue
			}
			var fds []fieldDTO
			for _, f := range st.Fields {
				fds = append(fds, fieldDTO{
					Name:       f.Name,
					Type:       f.Type.String(),
					Offset:     f.Offset,
					ReadOnly:   f.IsReadOnly,
					PublicRead: f.IsRoPub,
				})
			}
			var mtds []string
			for mname := range st.Methods {
				mtds = append(mtds, mname)
			}
			sDTOs[name] = structDTO{
				Name:    name,
				Size:    st.TotalSize,
				Align:   st.TotalAlign,
				Fields:  fds,
				Methods: mtds,
			}
		}

		eDTOs := make(map[string]enumDTO)
		for name, en := range enums {
			if filterSymbol != "" && !strings.EqualFold(name, filterSymbol) {
				continue
			}
			var vars []enumVariantDTO
			for _, v := range en.Variants {
				vars = append(vars, enumVariantDTO{Name: v.Name, Tag: v.Tag})
			}
			eDTOs[name] = enumDTO{
				Name:     name,
				Size:     en.TotalSize,
				Align:    en.TotalAlign,
				Variants: vars,
			}
		}

		iDTOs := make(map[string]ifcDTO)
		for name, ifc := range interfaces {
			if filterSymbol != "" && !strings.EqualFold(name, filterSymbol) {
				continue
			}
			var fds []fieldDTO
			for _, f := range ifc.Fields {
				fds = append(fds, fieldDTO{
					Name:       f.Name,
					Type:       f.Type.String(),
					ReadOnly:   f.IsReadOnly,
					PublicRead: f.IsRoPub,
				})
			}
			iDTOs[name] = ifcDTO{
				Name:   name,
				Fields: fds,
			}
		}

		dumpMap := map[string]any{
			"file":       filePath,
			"structs":    sDTOs,
			"interfaces": iDTOs,
			"enums":      eDTOs,
		}
		data, err := json.MarshalIndent(dumpMap, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}

	fmt.Printf("Debug Scope: %s\n\n", filePath)

	// Structs
	fmt.Printf("Structs (%d):\n", len(structs))
	for name, st := range structs {
		if filterSymbol != "" && !strings.EqualFold(name, filterSymbol) {
			continue
		}
		fmt.Printf("  • %s [size: %dB, align: %dB, fields: %d, methods: %d]\n",
			name, st.TotalSize, st.TotalAlign, len(st.Fields), len(st.Methods))
		for _, f := range st.Fields {
			mod := "mut"
			if f.IsReadOnly {
				mod = "ro"
			} else if f.IsRoPub {
				mod = "ro(pub)"
			}
			fmt.Printf("      - %s: %s (offset: %d, %s)\n", f.Name, f.Type.String(), f.Offset, mod)
		}
		for mName, m := range st.Methods {
			fmt.Printf("      + fn %s(...) %s\n", mName, m.Return.String())
		}
	}
	fmt.Println()

	// Interfaces
	if len(interfaces) > 0 {
		fmt.Printf("Interfaces (%d):\n", len(interfaces))
		for name, ifc := range interfaces {
			if filterSymbol != "" && !strings.EqualFold(name, filterSymbol) {
				continue
			}
			fmt.Printf("  • %s [required fields: %d]\n", name, len(ifc.Fields))
			for _, f := range ifc.Fields {
				roStr := ""
				if f.IsReadOnly {
					roStr = "ro "
				}
				fmt.Printf("      - %s%s: %s\n", roStr, f.Name, f.Type.String())
			}
		}
		fmt.Println()
	}

	// Enums
	if len(enums) > 0 {
		fmt.Printf("Enums (%d):\n", len(enums))
		for name, en := range enums {
			if filterSymbol != "" && !strings.EqualFold(name, filterSymbol) {
				continue
			}
			fmt.Printf("  • %s [size: %dB, align: %dB, variants: %d]\n", name, en.TotalSize, en.TotalAlign, len(en.Variants))
			for _, v := range en.Variants {
				fmt.Printf("      - ::%s (tag: %d)\n", v.Name, v.Tag)
			}
		}
		fmt.Println()
	}

	// Functions
	var fns []*loneanalyzer.TFnDec
	for _, decl := range typedProg.Decls {
		if fn, ok := decl.(*loneanalyzer.TFnDec); ok {
			fns = append(fns, fn)
		}
	}
	if len(fns) > 0 {
		fmt.Printf("Functions (%d):\n", len(fns))
		for _, fn := range fns {
			if filterSymbol != "" && !strings.EqualFold(fn.Name, filterSymbol) {
				continue
			}
			fmt.Printf("  • fn %s(...) %s\n", fn.Name, fn.ReturnType.String())
		}
	}
}

func printUsage() {
	fmt.Println("Usage: lone <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  build <file.lone> [-o out.wat]   Compile Lone source into WebAssembly WAT")
	fmt.Println("  run   <file.lone>                Compile and execute immediately in Wasmtime")
	fmt.Println("  check <file.lone>                Analyze file for semantic and type correctness")
	fmt.Println("  parse <file.lone>                Parse file using Pratt parser and verify syntax")
	fmt.Println("  debug <file.lone> [symbol]       Inspect struct memory layout, interfaces, enums")
	fmt.Println("  lsp                              Launch Language Server Protocol daemon")
	fmt.Println("  version                          Display Lone compiler version")
}
