package lonelogger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

// Config configures styling and formatting behavior for the diagnostic emitter.
type Config struct {
	EnableColor  bool
	ContextLines int
	TabWidth     int
}

// DefaultConfig returns a recommended default configuration.
func DefaultConfig() Config {
	// Auto-detect NO_COLOR environment variable (https://no-color.org)
	noColor := os.Getenv("NO_COLOR") != ""
	term := os.Getenv("TERM")

	enableColor := !noColor && term != "dumb"
	return Config{
		EnableColor:  enableColor,
		ContextLines: 1,
		TabWidth:     4,
	}
}

// PlainConfig returns a plain-text configuration with colors disabled.
func PlainConfig() Config {
	return Config{
		EnableColor:  false,
		ContextLines: 1,
		TabWidth:     4,
	}
}

// Emitter renders diagnostics with source code excerpts, line numbers, and carets.
type Emitter struct {
	out io.Writer
	cfg Config
}

// NewEmitter creates a new Emitter writing to out with the given config.
func NewEmitter(out io.Writer, cfg Config) *Emitter {
	if cfg.TabWidth <= 0 {
		cfg.TabWidth = 4
	}
	if cfg.ContextLines < 0 {
		cfg.ContextLines = 0
	}
	return &Emitter{
		out: out,
		cfg: cfg,
	}
}

// ANSI Escape sequences
const (
	ansiReset     = "\033[0m"
	ansiBold      = "\033[1m"
	ansiRed       = "\033[31m"
	ansiBoldRed   = "\033[1;31m"
	ansiBoldYellow = "\033[1;33m"
	ansiBoldCyan  = "\033[1;36m"
	ansiBoldGreen = "\033[1;32m"
	ansiBoldBlue  = "\033[1;34m"
)

func (e *Emitter) color(code string, text string) string {
	if !e.cfg.EnableColor {
		return text
	}
	return code + text + ansiReset
}

func (e *Emitter) severityColor(sev Severity) string {
	switch sev {
	case SeverityError:
		return ansiBoldRed
	case SeverityWarning:
		return ansiBoldYellow
	case SeverityNote:
		return ansiBoldCyan
	case SeverityHint:
		return ansiBoldGreen
	default:
		return ansiBold
	}
}

// Emit writes the formatted diagnostic to the configured output writer.
func (e *Emitter) Emit(diag *Diagnostic, sourceCode []byte) error {
	formatted := e.Format(diag, sourceCode)
	_, err := io.WriteString(e.out, formatted)
	return err
}

// Format returns the diagnostic rendered as a formatted string.
func (e *Emitter) Format(diag *Diagnostic, sourceCode []byte) string {
	if diag == nil {
		return ""
	}

	var buf bytes.Buffer
	sevStr := diag.Severity.String()
	sevColor := e.severityColor(diag.Severity)

	// Header: error[E001]: message
	header := e.color(sevColor, sevStr)
	if diag.Code != "" {
		header += fmt.Sprintf("[%s]", e.color(ansiBold, diag.Code))
	}
	header += ": " + e.color(ansiBold, diag.Message) + "\n"
	buf.WriteString(header)

	// If no primary span exists, just emit notes/hints
	if diag.PrimarySpan.File == "" && diag.PrimarySpan.StartLine <= 0 {
		e.appendFooters(&buf, diag, 0)
		return buf.String()
	}

	span := diag.PrimarySpan
	if span.StartLine <= 0 {
		span.StartLine = 1
	}
	if span.StartCol <= 0 {
		span.StartCol = 1
	}
	if span.EndLine <= 0 {
		span.EndLine = span.StartLine
	}
	if span.EndCol <= 0 {
		span.EndCol = span.StartCol
	}

	// Split source into lines
	lines := splitLines(sourceCode)
	if len(lines) == 0 {
		// No source available, emit simple location header
		loc := fmt.Sprintf("  --> %s:%d:%d\n", span.File, span.StartLine, span.StartCol)
		buf.WriteString(e.color(ansiBoldBlue, loc))
		e.appendFooters(&buf, diag, 0)
		return buf.String()
	}

	startIdx := span.StartLine - 1 - e.cfg.ContextLines
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := span.EndLine - 1 + e.cfg.ContextLines
	if endIdx >= len(lines) {
		endIdx = len(lines) - 1
	}

	maxLineNum := endIdx + 1
	gutterWidth := len(fmt.Sprintf("%d", maxLineNum))
	gutterPad := strings.Repeat(" ", gutterWidth)

	// Location indicator:   --> file:line:col
	arrow := e.color(ansiBoldBlue, fmt.Sprintf("%s--> %s:%d:%d\n", gutterPad, span.File, span.StartLine, span.StartCol))
	buf.WriteString(arrow)

	emptyGutter := e.color(ansiBoldBlue, fmt.Sprintf("%s |\n", gutterPad))
	buf.WriteString(emptyGutter)

	for i := startIdx; i <= endIdx; i++ {
		lineNum := i + 1
		rawLine := lines[i]
		expandedLine, colMap := expandTabs(rawLine, e.cfg.TabWidth)

		numStr := fmt.Sprintf("%*d", gutterWidth, lineNum)
		linePrefix := fmt.Sprintf("%s | ", e.color(ansiBoldBlue, numStr))
		buf.WriteString(linePrefix)
		buf.WriteString(expandedLine)
		buf.WriteString("\n")

		// Check if this line needs carets
		if lineNum >= span.StartLine && lineNum <= span.EndLine {
			caretBuf := e.renderCaretLine(lineNum, span, colMap, len(expandedLine), gutterPad, sevColor)
			buf.WriteString(caretBuf)
		}
	}

	buf.WriteString(emptyGutter)
	e.appendFooters(&buf, diag, gutterWidth)

	return buf.String()
}

func (e *Emitter) renderCaretLine(lineNum int, span SourceSpan, colMap []int, expandedLineLen int, gutterPad string, sevColor string) string {
	var startChar, endChar int

	if lineNum == span.StartLine {
		startChar = mapColToExpanded(span.StartCol, colMap)
	} else {
		startChar = 0
	}

	if lineNum == span.EndLine {
		endChar = mapColToExpanded(span.EndCol, colMap)
	} else {
		endChar = expandedLineLen
	}

	if startChar < 0 {
		startChar = 0
	}
	if endChar < startChar {
		endChar = startChar
	}

	caretCount := endChar - startChar + 1
	if caretCount <= 0 {
		caretCount = 1
	}

	leadSpace := strings.Repeat(" ", startChar)
	carets := strings.Repeat("^", caretCount)

	labelSuffix := ""
	if span.Label != "" && lineNum == span.EndLine {
		labelSuffix = " " + span.Label
	}

	caretText := e.color(sevColor, carets+labelSuffix)
	return fmt.Sprintf("%s | %s%s\n", e.color(ansiBoldBlue, gutterPad), leadSpace, caretText)
}

func (e *Emitter) appendFooters(buf *bytes.Buffer, diag *Diagnostic, gutterWidth int) {
	pad := strings.Repeat(" ", gutterWidth)
	for _, note := range diag.Notes {
		notePrefix := e.color(ansiBoldCyan, fmt.Sprintf("%s = note: ", pad))
		buf.WriteString(fmt.Sprintf("%s%s\n", notePrefix, note))
	}
	for _, hint := range diag.Hints {
		hintPrefix := e.color(ansiBoldGreen, fmt.Sprintf("%s = help: ", pad))
		buf.WriteString(fmt.Sprintf("%s%s\n", hintPrefix, hint))
	}
}

// splitLines converts source bytes into an array of string lines without line endings.
func splitLines(src []byte) []string {
	if len(src) == 0 {
		return nil
	}
	normalized := strings.ReplaceAll(string(src), "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.Split(normalized, "\n")
}

// expandTabs expands tab characters to spaces and returns a mapping from original 1-based columns to expanded 0-based columns.
func expandTabs(s string, tabWidth int) (string, []int) {
	var b strings.Builder
	colMap := make([]int, len(s)+2)

	currCol := 0
	for origIdx, ch := range s {
		origCol := origIdx + 1
		colMap[origCol] = currCol

		if ch == '\t' {
			spaces := tabWidth - (currCol % tabWidth)
			for j := 0; j < spaces; j++ {
				b.WriteByte(' ')
				currCol++
			}
		} else {
			b.WriteRune(ch)
			currCol++
		}
	}
	colMap[len(s)+1] = currCol

	return b.String(), colMap
}

func mapColToExpanded(col int, colMap []int) int {
	if col <= 0 {
		return 0
	}
	if col < len(colMap) {
		return colMap[col]
	}
	if len(colMap) > 0 {
		return colMap[len(colMap)-1]
	}
	return col - 1
}
