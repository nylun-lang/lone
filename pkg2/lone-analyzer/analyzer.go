package loneanalyzer

import (
	loneparser "lone/pkg2/lone-parser"
)

// Analyzer orchestrates parsing and semantic evaluation.
type Analyzer struct {
	evaluator *TypeEvaluator
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		evaluator: NewTypeEvaluator(),
	}
}

// Analyze runs semantic analysis on parsed AST declarations and returns collected diagnostics.
func (a *Analyzer) Analyze(decls []any) []Diagnostic {
	_ = a.evaluator.EvaluateProgram(decls)
	return a.evaluator.Diagnostics()
}

// AnalyzeTyped runs semantic analysis and returns both the Typed IR program and diagnostics.
func (a *Analyzer) AnalyzeTyped(decls []any) (*TProgram, []Diagnostic) {
	prog := a.evaluator.EvaluateProgram(decls)
	return prog, a.evaluator.Diagnostics()
}

// AnalyzeSource parses source bytes and performs full type evaluation.
func (a *Analyzer) AnalyzeSource(src []byte) (*TProgram, []Diagnostic, error) {
	decls, err := loneparser.Parse(src)
	if err != nil {
		return nil, nil, err
	}
	prog, diags := a.AnalyzeTyped(decls)
	return prog, diags, nil
}
