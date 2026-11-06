package loneanalyzer

import (
	"fmt"
	loneast "lone/pkg/lone-parser-ast"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityNote    Severity = "note"
	SeverityHint    Severity = "hint"
)

// Diagnostic represents a compiler error, warning, note, or hint.
type Diagnostic struct {
	Severity Severity
	Span     loneast.Span
	Rule     string
	Message  string
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("[%s] %s (%s): %s", d.Severity, d.Rule, d.Span.Start, d.Message)
}
