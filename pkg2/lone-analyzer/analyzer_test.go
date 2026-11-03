package loneanalyzer

import (
	"strings"
	"testing"
)

func TestLetImmutabilityEnforcement(t *testing.T) {
	code := `
let max = 100
max = 200

var count = 0
count = 10
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(diags), diags)
	}

	d := diags[0]
	if d.Rule != "type/immutable-reassignment" {
		t.Errorf("expected rule 'type/immutable-reassignment', got %q", d.Rule)
	}
	if !strings.Contains(d.Message, "cannot reassign immutable binding \"max\"") {
		t.Errorf("unexpected error message: %q", d.Message)
	}
}

func TestVarMutabilityAllowed(t *testing.T) {
	code := `
var total = 0
total = 10
total = total + 5
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestScopedConditionHeaderScope(t *testing.T) {
	code := `
fn test_scoped() void {
    if: [let scoped_var = 42] scoped_var > 0 {
        let inside = scoped_var
    }
    let leak = scoped_var
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	// 'scoped_var' must be undefined outside the 'if' construct
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic (scope leak), got %d: %v", len(diags), diags)
	}

	d := diags[0]
	if d.Rule != "type/undefined-identifier" || !strings.Contains(d.Message, "scoped_var") {
		t.Errorf("expected undefined identifier 'scoped_var', got %v", d)
	}
}

func TestScopedConditionHeaderPackedIf(t *testing.T) {
	code := `
fn test_packed() void {
    if: [let x = 10, let y = 20] {
        x > 0 and y > 0 => {
            drop x + y
        },
        else => {
            drop 0
        }
    }
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestTrTraitNamingConvention(t *testing.T) {
	// 1. Valid Tr trait
	validCode := `
pub type TrWriter = trait {
    fn write(self, buf: []u8) i32
}
pub type Stdout = struct {
    fd: i32
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(validCode))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for valid TrWriter, got: %v", diags)
	}

	// 2. Trait without Tr prefix
	invalidTraitCode := `
pub type WriterTrait = trait {
    fn write(self) void
}
`
	analyzer2 := NewAnalyzer()
	_, diags2, err := analyzer2.AnalyzeSource([]byte(invalidTraitCode))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags2) != 1 || diags2[0].Rule != "convention/trait-naming" {
		t.Errorf("expected convention/trait-naming error, got: %v", diags2)
	}

	// 3. Struct with Tr prefix
	invalidStructCode := `
pub type TrMyStruct = struct {
    x: i32
}
`
	analyzer3 := NewAnalyzer()
	_, diags3, err := analyzer3.AnalyzeSource([]byte(invalidStructCode))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags3) != 1 || diags3[0].Rule != "convention/non-trait-naming" {
		t.Errorf("expected convention/non-trait-naming error, got: %v", diags3)
	}
}

func TestExplicitCastOperator(t *testing.T) {
	code := `
let i: i32 = 100
let f: f32 = i as f32
let w: i64 = i as i64
`
	analyzer := NewAnalyzer()
	prog, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for explicit 'as' casts, got: %v", diags)
	}

	fVar := prog.Decls[1].(*TVarDec)
	if !fVar.Type.Equals(TypeF32) {
		t.Errorf("expected type f32, got %q", fVar.Type.String())
	}
}

func TestImplicitNumericMismatchRejected(t *testing.T) {
	code := `
var f: f32 = 100
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic (type mismatch for implicit int to float), got %d: %v", len(diags), diags)
	}
	if diags[0].Rule != "type/mismatch" {
		t.Errorf("expected rule 'type/mismatch', got %q", diags[0].Rule)
	}
}

func TestEnumAndMatchAnalysis(t *testing.T) {
	code := `
type Shape = enum {
    Circle(radius: f32),
    Rect(width: f32, height: f32),
    Point,
}

fn area(s: Shape) f32 {
    return match s {
        Shape::Circle(r) => 3.14 * r * r,
        Shape::Rect(w, h) => w * h,
        Shape::Point => 0.0,
    }
}

fn test_construct() Shape {
    let c = Shape::Circle(10.5)
    let p = Shape::Point
    return c
}
`
	analyzer := NewAnalyzer()
	prog, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %v", len(diags), diags)
	}

	enumDec, ok := prog.Decls[0].(*TEnumDec)
	if !ok {
		t.Fatalf("expected TEnumDec, got %#v", prog.Decls[0])
	}
	// Tag (4) + Rect payload (4+4) = 12 bytes
	if enumDec.Type.TotalSize != 12 {
		t.Errorf("expected Shape size 12, got %d", enumDec.Type.TotalSize)
	}
}

func TestNonExhaustiveMatchError(t *testing.T) {
	code := `
type Shape = enum {
    Circle(radius: f32),
    Rect(width: f32, height: f32),
    Point,
}

fn area(s: Shape) f32 {
    return match s {
        Shape::Circle(r) => 3.14 * r * r,
        Shape::Point => 0.0,
    }
}
`
	analyzer := NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for missing Rect variant, got %d: %v", len(diags), diags)
	}
	if diags[0].Rule != "type/non-exhaustive-match" {
		t.Errorf("expected rule 'type/non-exhaustive-match', got %q", diags[0].Rule)
	}
	if !strings.Contains(diags[0].Message, "Rect") {
		t.Errorf("expected message to mention missing variant 'Rect', got %q", diags[0].Message)
	}
}
