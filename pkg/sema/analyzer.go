package sema


// Pass represents an analysis pass operating on AST declarations.
type Pass interface {
	Run(decls []any) []Diagnostic
}

// AnalyzerConfig specifies options for configuring the analyzer.
type AnalyzerConfig struct {
	EnforceNamingConventions bool
	EnforceTypeChecking      bool
	EnforceConsumeOrDrop     bool
}

// DefaultConfig returns the standard analyzer configuration.
func DefaultConfig() AnalyzerConfig {
	return AnalyzerConfig{
		EnforceNamingConventions: true,
		EnforceTypeChecking:      true,
		EnforceConsumeOrDrop:     true,
	}
}

// Analyzer orchestrates multiple analysis passes over a Lone file AST.
type Analyzer struct {
	fileName string
	config   AnalyzerConfig
	passes   []Pass
	typePass *PassTypes
}

// NewAnalyzer creates a configured analyzer for a given file.
func NewAnalyzer(fileName string, config AnalyzerConfig) *Analyzer {
	a := &Analyzer{
		fileName: fileName,
		config:   config,
		passes:   make([]Pass, 0),
	}

	if config.EnforceNamingConventions {
		a.passes = append(a.passes, NewPassNamingConventions(fileName))
	}

	if config.EnforceTypeChecking {
		a.typePass = NewPassTypes(fileName, config.EnforceConsumeOrDrop)
		a.passes = append(a.passes, a.typePass)
	}

	return a
}

// Analyze runs all registered passes on the AST declarations and returns collected diagnostics.
func (a *Analyzer) Analyze(ast any) []Diagnostic {
	decls := toDeclSlice(ast)
	var diagnostics []Diagnostic

	for _, pass := range a.passes {
		diags := pass.Run(decls)
		diagnostics = append(diagnostics, diags...)
	}

	return diagnostics
}

// AnalyzeTyped runs all passes and returns both the generated Typed IR and collected diagnostics.
func (a *Analyzer) AnalyzeTyped(ast any) (*TProgram, []Diagnostic) {
	diags := a.Analyze(ast)
	var prog *TProgram
	if a.typePass != nil {
		prog = a.typePass.TypedProgram()
	}
	return prog, diags
}

func toDeclSlice(ast any) []any {
	if ast == nil {
		return nil
	}
	switch v := ast.(type) {
	case []any:
		return v
	default:
		return []any{v}
	}
}
