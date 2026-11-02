package sema


// PassTypes performs type analysis, inferring untyped variables, verifying function returns,
// enforcing that non-void expression results are consumed or dropped, and generating a Typed IR.
type PassTypes struct {
	fileName             string
	enforceConsumeOrDrop bool
	program              *TProgram
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
	evaluator := NewTypeEvaluator(p.fileName, p.enforceConsumeOrDrop)
	p.program = evaluator.Evaluate(decls)
	return evaluator.Diagnostics()
}

// TypedProgram returns the evaluated Typed IR program.
func (p *PassTypes) TypedProgram() *TProgram {
	return p.program
}
