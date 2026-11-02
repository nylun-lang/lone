package lonelogger

// Severity represents the importance or classification of a diagnostic.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityNote
	SeverityHint
)

// String returns a human-readable lowercase string representation.
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityNote:
		return "note"
	case SeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}

// UpperString returns an uppercase string representation.
func (s Severity) UpperString() string {
	switch s {
	case SeverityError:
		return "ERROR"
	case SeverityWarning:
		return "WARNING"
	case SeverityNote:
		return "NOTE"
	case SeverityHint:
		return "HINT"
	default:
		return "UNKNOWN"
	}
}
