package lonecompiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	loneanalyzer "lone/pkg/lone-analyzer"
	loneproject "lone/pkg/lone-analyzer-project"
	loneadapter "lone/pkg/lone-compiler-adapter"
	lonemir "lone/pkg/lone-compiler-mir"
	lonewir "lone/pkg/lone-compiler-wir"
	loneparser "lone/pkg/lone-parser"
	loneruntime "lone/pkg/lone-runtime"
)

// Target represents the execution platform for the emitted WebAssembly.
type Target = loneruntime.Target

const (
	TargetWasi = loneruntime.TargetWasi
	TargetWeb  = loneruntime.TargetWeb
)

// Config defines the options for a compilation run.
type Config struct {
	Target          Target
	EmitMir         bool
	Optimize        bool
	WithAdapter     string // "node" or "browser"
	OutFilePath     string
}

// DefaultConfig provides recommended compilation options.
func DefaultConfig() Config {
	return Config{
		Target:   TargetWasi,
		EmitMir:  false,
		Optimize: true,
	}
}

// Result holds the compiled artifacts produced across all compiler stages.
type Result struct {
	TypedProgram *loneanalyzer.TProgram
	Diagnostics  []loneanalyzer.Diagnostic
	MirModule    *lonemir.MirModule
	WasmModule   *lonewir.WasmModule
	WatOutput    string
	AdapterGlue  string
	AdapterFile  string
}

// Compiler coordinates the end-to-end compilation pipeline.
type Compiler struct {
	cfg Config
	rt  *loneruntime.Table
}

// NewCompiler creates a new Compiler with the given configuration.
func NewCompiler(cfg Config) *Compiler {
	return &Compiler{
		cfg: cfg,
		rt:  loneruntime.NewDefaultTable(),
	}
}

// CompileFile reads a .lone file from disk and runs it through the full compiler pipeline.
func (c *Compiler) CompileFile(filePath string) (*Result, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	return c.CompileSource(filePath, content)
}

// CompileSource executes the compiler pipeline on in-memory Lone source code.
func (c *Compiler) CompileSource(fileName string, source []byte) (*Result, error) {
	// 1. Parsing
	rawAst, err := loneparser.Parse(fileName, source)
	if err != nil {
		return nil, fmt.Errorf("syntax error: %w", err)
	}

	// 1.5. Module & Host Capability Resolution
	resolvedAst, resolveDiags := loneproject.ResolveAST(fileName, rawAst)

	// 2. Semantic Analysis & Type Inference
	analyzer := loneanalyzer.NewAnalyzer(fileName, loneanalyzer.DefaultConfig())
	typedProg, diags := analyzer.AnalyzeTyped(resolvedAst)
	diags = append(resolveDiags, diags...)

	hasErrors := false
	for _, d := range diags {
		if d.Severity == loneanalyzer.SeverityError {
			hasErrors = true
		}
	}
	if hasErrors {
		return &Result{
			TypedProgram: typedProg,
			Diagnostics:  diags,
		}, fmt.Errorf("compilation failed with type/semantic errors")
	}

	// 3. Lowering to MIR (Mid-level IR)
	mirMod, err := lonemir.LowerProgram(typedProg)
	if err != nil {
		return nil, fmt.Errorf("MIR lowering error: %w", err)
	}

	res := &Result{
		TypedProgram: typedProg,
		Diagnostics:  diags,
		MirModule:    mirMod,
	}

	// If MIR output requested, format and return early
	if c.cfg.EmitMir {
		res.WatOutput = lonemir.FormatModule(mirMod)
		return res, nil
	}

	// 4. Lowering to WIR (Wasm AST)
	wasmMod, err := lonewir.LowerToWasmModule(mirMod, c.cfg.Target, c.rt)
	if err != nil {
		return nil, fmt.Errorf("WIR AST construction error: %w", err)
	}
	res.WasmModule = wasmMod

	// 5. Optimization pass
	if c.cfg.Optimize {
		opt := lonewir.NewOptimizer()
		opt.OptimizeModule(wasmMod)
	}

	// 6. Formatting WAT
	res.WatOutput = wasmMod.FormatWat()

	// 7. Adapter Glue generation
	if c.cfg.WithAdapter != "" {
		kind, err := loneadapter.ParseAdapterKind(c.cfg.WithAdapter)
		if err != nil {
			return nil, err
		}
		wasmBaseName := "out.wasm"
		if c.cfg.OutFilePath != "" {
			wasmBaseName = filepath.Base(c.cfg.OutFilePath)
			if strings.HasSuffix(wasmBaseName, ".wat") {
				wasmBaseName = strings.TrimSuffix(wasmBaseName, ".wat") + ".wasm"
			}
		}
		glue, err := loneadapter.GenerateAdapter(kind, wasmBaseName)
		if err != nil {
			return nil, fmt.Errorf("adapter generation failed: %w", err)
		}
		res.AdapterGlue = glue
		ext := ".mjs"
		if kind == loneadapter.AdapterBrowser {
			ext = ".html"
		}
		res.AdapterFile = strings.TrimSuffix(wasmBaseName, filepath.Ext(wasmBaseName)) + ext
	}

	return res, nil
}
