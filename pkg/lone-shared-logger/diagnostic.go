package lonelogger

// SourceSpan points to a precise region or token inside a Lone source file.
// Lines and Columns are 1-based.
type SourceSpan struct {
	File      string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	Label     string // Inlined annotation pointing to the span (e.g. "expected i32, found str")
}

// PointSpan constructs a single-location span (cursor or 1-character token).
func PointSpan(file string, line, col int, label ...string) SourceSpan {
	lbl := ""
	if len(label) > 0 {
		lbl = label[0]
	}
	return SourceSpan{
		File:      file,
		StartLine: line,
		StartCol:  col,
		EndLine:   line,
		EndCol:    col,
		Label:     lbl,
	}
}

// RangeSpan constructs a range span spanning from StartLine:StartCol to EndLine:EndCol.
func RangeSpan(file string, startLine, startCol, endLine, endCol int, label ...string) SourceSpan {
	lbl := ""
	if len(label) > 0 {
		lbl = label[0]
	}
	return SourceSpan{
		File:      file,
		StartLine: startLine,
		StartCol:  startCol,
		EndLine:   endLine,
		EndCol:    endCol,
		Label:     lbl,
	}
}

// Diagnostic encapsulates a compiler error, warning, note, or suggestion.
type Diagnostic struct {
	Severity       Severity
	Code           string       // Optional code, e.g. "E001" or "rule/type-mismatch"
	Message        string       // Main error message
	PrimarySpan    SourceSpan   // Main code location associated with the diagnostic
	SecondarySpans []SourceSpan // Supplementary related spans (e.g., conflicting definition)
	Notes          []string     // Footer notes explaining context
	Hints          []string     // Footer suggestions or help instructions
}

// NewError creates an Error-level diagnostic.
func NewError(message string, span SourceSpan) *Diagnostic {
	return &Diagnostic{
		Severity:    SeverityError,
		Message:     message,
		PrimarySpan: span,
	}
}

// NewWarning creates a Warning-level diagnostic.
func NewWarning(message string, span SourceSpan) *Diagnostic {
	return &Diagnostic{
		Severity:    SeverityWarning,
		Message:     message,
		PrimarySpan: span,
	}
}

// WithCode sets an optional diagnostic code.
func (d *Diagnostic) WithCode(code string) *Diagnostic {
	d.Code = code
	return d
}

// WithNote appends a footer note to the diagnostic.
func (d *Diagnostic) WithNote(note string) *Diagnostic {
	d.Notes = append(d.Notes, note)
	return d
}

// WithHint appends a hint/help recommendation.
func (d *Diagnostic) WithHint(hint string) *Diagnostic {
	d.Hints = append(d.Hints, hint)
	return d
}

// WithSecondarySpan adds a secondary highlight location.
func (d *Diagnostic) WithSecondarySpan(span SourceSpan) *Diagnostic {
	d.SecondarySpans = append(d.SecondarySpans, span)
	return d
}
