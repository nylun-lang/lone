package lonelogger_test

import (
	"strings"
	"testing"

	lonelogger "lone/pkg2/lone-shared-logger"
)

func TestEmitterSinglePoint(t *testing.T) {
	source := `fn main() void {
    var x = 10
    drop x
}`

	diag := lonelogger.NewError("undefined variable", lonelogger.PointSpan("main.lone", 2, 9, "not found in this scope")).
		WithCode("E002").
		WithNote("declared variables must be in scope").
		WithHint("did you mean 'y'?")

	emitter := lonelogger.NewEmitter(nil, lonelogger.PlainConfig())
	output := emitter.Format(diag, []byte(source))

	if !strings.Contains(output, "error[E002]: undefined variable") {
		t.Errorf("expected header with code, got:\n%s", output)
	}
	if !strings.Contains(output, "--> main.lone:2:9") {
		t.Errorf("expected location line, got:\n%s", output)
	}
	if !strings.Contains(output, "var x = 10") {
		t.Errorf("expected source line, got:\n%s", output)
	}
	if !strings.Contains(output, "^ not found in this scope") {
		t.Errorf("expected caret with label, got:\n%s", output)
	}
	if !strings.Contains(output, "= note: declared variables must be in scope") {
		t.Errorf("expected footer note, got:\n%s", output)
	}
	if !strings.Contains(output, "= help: did you mean 'y'?") {
		t.Errorf("expected footer hint, got:\n%s", output)
	}
}

func TestEmitterRangeSpan(t *testing.T) {
	source := `fn calculate() i32 {
    return "invalid_string"
}`

	diag := lonelogger.NewError("type mismatch", lonelogger.RangeSpan("calc.lone", 2, 12, 2, 27, "expected i32, found str"))
	emitter := lonelogger.NewEmitter(nil, lonelogger.PlainConfig())
	output := emitter.Format(diag, []byte(source))

	if !strings.Contains(output, "^^^^^^^^^^^^^^^^ expected i32, found str") {
		t.Errorf("expected range underline for string, got:\n%s", output)
	}
}

func TestEmitterTabHandling(t *testing.T) {
	// Source with tab indentation
	source := "fn test() void {\n\tvar z = 5\n}"

	diag := lonelogger.NewWarning("unused variable", lonelogger.PointSpan("test.lone", 2, 6, "consider removing or using 'drop'"))
	emitter := lonelogger.NewEmitter(nil, lonelogger.Config{
		EnableColor:  false,
		ContextLines: 0,
		TabWidth:     4,
	})
	output := emitter.Format(diag, []byte(source))

	// The tab should be expanded to 4 spaces, so column 6 (after \t and 'var ') lines up accurately
	if !strings.Contains(output, "warning: unused variable") {
		t.Errorf("expected warning output, got:\n%s", output)
	}
	if !strings.Contains(output, "    var z = 5") {
		t.Errorf("expected tab expanded to spaces, got:\n%s", output)
	}
}

func TestEmitterEmptySource(t *testing.T) {
	diag := lonelogger.NewError("general error without file", lonelogger.PointSpan("unknown.lone", 1, 1))
	emitter := lonelogger.NewEmitter(nil, lonelogger.PlainConfig())
	output := emitter.Format(diag, nil)

	if !strings.Contains(output, "error: general error without file") {
		t.Errorf("expected fallback header, got:\n%s", output)
	}
	if !strings.Contains(output, "--> unknown.lone:1:1") {
		t.Errorf("expected file location fallback, got:\n%s", output)
	}
}
