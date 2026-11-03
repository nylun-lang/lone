package lonecompiler

import (
	"fmt"
	"os"

	loneanalyzer "lone/pkg2/lone-analyzer"
	lonewir "lone/pkg2/lone-compiler-wir"
	lonelexer "lone/pkg2/lone-lexer"
	loneparser "lone/pkg2/lone-parser"
)

// Config defines configuration options for the compiler pipeline.
type Config struct {
	Target       string // "wasi" (default) or "web"
	Optimize     bool
	OutFilePath  string
	MemoryPages  int
	MemoryExport string
}

// DefaultConfig provides recommended compilation options.
func DefaultConfig() Config {
	return Config{
		Target:       "wasi",
		Optimize:     true,
		MemoryPages:  2,
		MemoryExport: "memory",
	}
}

// Result holds the compiled artifacts produced across all compiler stages.
type Result struct {
	AST          []any
	TypedProgram *loneanalyzer.TProgram
	Diagnostics  []loneanalyzer.Diagnostic
	WasmModule   *lonewir.WasmModule
	WatOutput    string
}

// Compiler coordinates the end-to-end compilation pipeline.
type Compiler struct {
	cfg Config
}

// NewCompiler creates a new Compiler with the given configuration.
func NewCompiler(cfg Config) *Compiler {
	if cfg.Target == "" {
		cfg.Target = "wasi"
	}
	if cfg.MemoryPages <= 0 {
		cfg.MemoryPages = 2
	}
	if cfg.MemoryExport == "" {
		cfg.MemoryExport = "memory"
	}
	return &Compiler{cfg: cfg}
}

// CompileFile reads a file from disk and compiles it.
func (c *Compiler) CompileFile(filePath string) (*Result, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	return c.CompileSource(filePath, content)
}

// CompileSource executes the complete compiler pipeline on in-memory source code.
func (c *Compiler) CompileSource(fileName string, source []byte) (*Result, error) {
	// 1. Lexical Analysis
	tokens, err := lonelexer.NewLexer(source).Tokenize()
	if err != nil {
		return nil, fmt.Errorf("lexer error in %s: %w", fileName, err)
	}

	// 2. Parsing (Pratt + Recursive Descent)
	parser := loneparser.NewParser(tokens)
	ast, err := parser.ParseProgram()
	if err != nil {
		return nil, fmt.Errorf("syntax error in %s: %w", fileName, err)
	}

	// 3. Semantic Analysis & Type Evaluation
	evaluator := loneanalyzer.NewTypeEvaluator()
	typedProg := evaluator.EvaluateProgram(ast)
	diags := evaluator.Diagnostics()

	hasErrors := false
	for _, d := range diags {
		if d.Severity == loneanalyzer.SeverityError {
			hasErrors = true
		}
	}
	if hasErrors {
		return &Result{
			AST:          ast,
			TypedProgram: typedProg,
			Diagnostics:  diags,
		}, fmt.Errorf("compilation failed with type/semantic errors in %s", fileName)
	}

	// 4. Structured Wasm Lowering
	lowerOpts := lonewir.LowerOptions{
		Target:       c.cfg.Target,
		MemoryPages:  c.cfg.MemoryPages,
		MemoryExport: c.cfg.MemoryExport,
	}
	wasmMod, err := lonewir.LowerProgram(typedProg, lowerOpts)
	if err != nil {
		return nil, fmt.Errorf("lowering error in %s: %w", fileName, err)
	}

	// 5. Optimization
	if c.cfg.Optimize {
		opt := lonewir.NewOptimizer()
		opt.OptimizeModule(wasmMod)
	}

	// 6. WAT Code Generation
	wat := wasmMod.FormatWat()

	res := &Result{
		AST:          ast,
		TypedProgram: typedProg,
		Diagnostics:  diags,
		WasmModule:   wasmMod,
		WatOutput:    wat,
	}

	// 7. Write to output file if requested
	if c.cfg.OutFilePath != "" {
		if err := os.WriteFile(c.cfg.OutFilePath, []byte(wat), 0644); err != nil {
			return res, fmt.Errorf("failed to write output to %s: %w", c.cfg.OutFilePath, err)
		}
	}

	return res, nil
}
