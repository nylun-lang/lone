package sema

import (
	"fmt"
	ir "lone/pkg/ast"
)

type Severity int

const (
	SeverityHint Severity = iota
	SeverityWarning
	SeverityError
)

func (s Severity) String() string {
	switch s {
	case SeverityHint:
		return "HINT"
	case SeverityWarning:
		return "WARNING"
	case SeverityError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type Diagnostic struct {
	Message  string
	Severity Severity
	Pos      ir.Position
	Rule     string
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("[%s] %d:%d (%s): %s", d.Severity, d.Pos.Line, d.Pos.Col, d.Rule, d.Message)
}
