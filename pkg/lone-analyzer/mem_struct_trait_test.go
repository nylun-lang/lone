package loneanalyzer

import (
	"testing"

	loneparser "lone/pkg/lone-parser"
)

func hasDiagRule(diags []Diagnostic, rule string) bool {
	for _, d := range diags {
		if d.Rule == rule {
			return true
		}
	}
	return false
}

func TestRefVsRawPtrTypeDistinction(t *testing.T) {
	// 1. Passing &x (ref<i32>) to ptr<i32> must fail
	badRefToPtr := `
fn takes_raw(p: ptr<i32>) void {
	*p = 99
}

fn main() void {
	var x: i32 = 10
	takes_raw(&x)
}
`
	astBad, err := loneparser.Parse("bad_ref.lone", []byte(badRefToPtr))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	diagsBad := NewAnalyzer("bad_ref.lone", DefaultConfig()).Analyze(astBad)
	if !hasDiagRule(diagsBad, "type/call-arg-type") {
		t.Fatalf("expected type/call-arg-type when passing ref<i32> to ptr<i32>, got: %v", diagsBad)
	}

	// 2. Passing i32 directly to ptr<i32> must fail
	badIntToPtr := `
fn takes_raw(p: ptr<i32>) void {
	*p = 99
}

fn main() void {
	takes_raw(1234)
}
`
	astBadInt, err := loneparser.Parse("bad_int_ptr.lone", []byte(badIntToPtr))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	diagsBadInt := NewAnalyzer("bad_int_ptr.lone", DefaultConfig()).Analyze(astBadInt)
	if !hasDiagRule(diagsBadInt, "type/call-arg-type") {
		t.Fatalf("expected type/call-arg-type when passing i32 to ptr<i32>, got: %v", diagsBadInt)
	}

	// 3. Passing &raw x (ptr<i32>) to ptr<i32> and ptr<any> must succeed and mark AddressTaken
	goodRaw := `
fn takes_any_ptr(p: ptr<any>) void {
}

fn takes_raw(p: ptr<i32>) void {
	*p = 99
	takes_any_ptr(p)
}

fn main() void {
	var x: i32 = 10
	takes_raw(&raw x)
}
`
	astGood, err := loneparser.Parse("good_raw.lone", []byte(goodRaw))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	prog, diagsGood := NewAnalyzer("good_raw.lone", DefaultConfig()).AnalyzeTyped(astGood)
	for _, d := range diagsGood {
		if d.Severity == SeverityError {
			t.Fatalf("unexpected error in good_raw.lone: %v", diagsGood)
		}
	}

	var mainFn *TFnDec
	for _, decl := range prog.Decls {
		if fn, ok := decl.(*TFnDec); ok && fn.Name == "main" {
			mainFn = fn
		}
	}
	if mainFn == nil {
		t.Fatalf("main function not found")
	}
	xVar, ok := mainFn.Body.Stmts[0].(*TVarDec)
	if !ok || !xVar.AddressTaken {
		t.Fatalf("expected var x to have AddressTaken=true when &raw x is used, got: %+v", xVar)
	}
}

func TestReadOnlyMutationRejection(t *testing.T) {
	code := `
fn bad_mutate(r: ref<ro i32>) void {
	*r = 42
}
`
	ast, err := loneparser.Parse("ro.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	diags := NewAnalyzer("ro.lone", DefaultConfig()).Analyze(ast)
	if !hasDiagRule(diags, "type/readonly-mutation") {
		t.Fatalf("expected type/readonly-mutation diagnostic, got: %v", diags)
	}
}

func TestStructLayoutAndByteOffsets(t *testing.T) {
	code := `
pub type Ciovec = struct {
	buf: ptr<u8>,
	len: i32,
}

pub type Padded = struct {
	tag: u8,
	count: i32,
	flag: u8,
}
`
	ast, err := loneparser.Parse("layout.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	te := NewTypeEvaluator("layout.lone", true)
	_ = te.Evaluate(toDeclSlice(ast))
	if len(te.Diagnostics()) != 0 {
		t.Fatalf("unexpected diagnostics: %v", te.Diagnostics())
	}

	ciovecSym := te.stack.Resolve("Ciovec")
	if ciovecSym == nil {
		t.Fatalf("Ciovec type symbol not found")
	}
	ciovec, ok := ciovecSym.Type.(*StructType)
	if !ok {
		t.Fatalf("expected StructType for Ciovec, got %T", ciovecSym.Type)
	}
	if ciovec.Size != 8 || ciovec.Align != 4 {
		t.Errorf("expected Ciovec size=8 align=4, got size=%d align=%d", ciovec.Size, ciovec.Align)
	}
	if bufField, ok := ciovec.Field("buf"); !ok || bufField.Offset != 0 {
		t.Errorf("expected Ciovec.buf at offset 0, got %+v", bufField)
	}
	if lenField, ok := ciovec.Field("len"); !ok || lenField.Offset != 4 {
		t.Errorf("expected Ciovec.len at offset 4, got %+v", lenField)
	}

	paddedSym := te.stack.Resolve("Padded")
	padded := paddedSym.Type.(*StructType)
	if padded.Size != 12 || padded.Align != 4 {
		t.Errorf("expected Padded size=12 align=4, got size=%d align=%d", padded.Size, padded.Align)
	}
	fTag, _ := padded.Field("tag")
	fCount, _ := padded.Field("count")
	fFlag, _ := padded.Field("flag")
	if fTag.Offset != 0 || fCount.Offset != 4 || fFlag.Offset != 8 {
		t.Errorf("expected Padded offsets (0, 4, 8), got (%d, %d, %d)", fTag.Offset, fCount.Offset, fFlag.Offset)
	}
}

func TestTraitMaterializationAndMissingImpl(t *testing.T) {
	goodCode := `
type MeasurableTrait = trait {
	fn area(self) i32
}

type Rect = struct {
	w: i32,
	h: i32,
}

impl MeasurableTrait on Rect {
	fn area(self) i32 {
		return self.w * self.h
	}
}

fn check() i32 {
	var r = Rect::{ .w: 6, .h: 7 }
	let m = MeasurableTrait::|Rect|
	return m.area(r)
}
`
	astGood, err := loneparser.Parse("trait_ok.lone", []byte(goodCode))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	diagsGood := NewAnalyzer("trait_ok.lone", DefaultConfig()).Analyze(astGood)
	for _, d := range diagsGood {
		if d.Severity == SeverityError {
			t.Fatalf("unexpected error in trait_ok.lone: %v", diagsGood)
		}
	}

	badCode := `
type MeasurableTrait = trait {
	fn area(self) i32
}

type Circle = struct {
	r: i32,
}

fn check() void {
	let m = MeasurableTrait::|Circle|
	drop m
}
`
	astBad, err := loneparser.Parse("trait_bad.lone", []byte(badCode))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	diagsBad := NewAnalyzer("trait_bad.lone", DefaultConfig()).Analyze(astBad)
	if !hasDiagRule(diagsBad, "type/missing-trait-impl") {
		t.Fatalf("expected type/missing-trait-impl diagnostic, got: %v", diagsBad)
	}
}
