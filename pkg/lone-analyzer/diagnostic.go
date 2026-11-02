package loneanalyzer

import (
	"fmt"
	ir "lone/pkg/lone-parser-ast"
	lonelogger "lone/pkg/lone-shared-logger"
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

// ToLoggerDiag converts an analyzer Diagnostic into a rich lonelogger.Diagnostic.
func (d Diagnostic) ToLoggerDiag(file string) *lonelogger.Diagnostic {
	var sev lonelogger.Severity
	switch d.Severity {
	case SeverityError:
		sev = lonelogger.SeverityError
	case SeverityWarning:
		sev = lonelogger.SeverityWarning
	case SeverityHint:
		sev = lonelogger.SeverityHint
	default:
		sev = lonelogger.SeverityNote
	}
	return &lonelogger.Diagnostic{
		Severity:    sev,
		Code:        d.Rule,
		Message:     d.Message,
		PrimarySpan: lonelogger.PointSpan(file, d.Pos.Line, d.Pos.Col),
	}
}
