package loneanalyzer


// PassTypes performs type analysis, inferring untyped variables, verifying function returns,
// enforcing that non-void expression results are consumed or dropped, and generating a Typed IR.
type PassTypes struct {
	fileName             string
	enforceConsumeOrDrop bool
	program              *TProgram
	evaluator            *TypeEvaluator
}

// NewPassTypes creates a new PassTypes instance.
func NewPassTypes(fileName string, enforceConsumeOrDrop bool) *PassTypes {
	return &PassTypes{
		fileName:             fileName,
		enforceConsumeOrDrop: enforceConsumeOrDrop,
	}
}

// Run executes type evaluation over the AST declarations.
func (p *PassTypes) Run(decls []any) []Diagnostic {
	p.evaluator = NewTypeEvaluator(p.fileName, p.enforceConsumeOrDrop)
	p.program = p.evaluator.Evaluate(decls)
	return p.evaluator.Diagnostics()
}

// TypedProgram returns the evaluated Typed IR program.
func (p *PassTypes) TypedProgram() *TProgram {
	return p.program
}

// Evaluator returns the TypeEvaluator instance after Run is called.
func (p *PassTypes) Evaluator() *TypeEvaluator {
	return p.evaluator
}
