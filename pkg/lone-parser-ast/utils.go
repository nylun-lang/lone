package loneast

import lonelogger "lone/pkg/lone-shared-logger"

type Position struct {
	Col    int
	Line   int
	Offset int
}

// ToSpan converts an AST Position into a lonelogger SourceSpan for diagnostic reporting.
func (p Position) ToSpan(file string, label ...string) lonelogger.SourceSpan {
	return lonelogger.PointSpan(file, p.Line, p.Col, label...)
}

