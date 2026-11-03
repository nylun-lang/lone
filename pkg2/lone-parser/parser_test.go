package loneparser

import (
	loneast "lone/pkg2/lone-parser-ast"
	"testing"
)

func TestParseLetAndVar(t *testing.T) {
	code := `
let x = 10
var y: i32 = 20
pub let PI = 3.14
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decls) != 3 {
		t.Fatalf("expected 3 decls, got %d", len(decls))
	}

	b1 := decls[0].(loneast.BIND_DEC)
	if b1.Name != "x" || b1.IsMutable != false || b1.IsPublic != false {
		t.Errorf("expected immutable 'let x', got %+v", b1)
	}

	b2 := decls[1].(loneast.BIND_DEC)
	if b2.Name != "y" || b2.IsMutable != true || b2.IsPublic != false {
		t.Errorf("expected mutable 'var y', got %+v", b2)
	}

	b3 := decls[2].(loneast.BIND_DEC)
	if b3.Name != "PI" || b3.IsMutable != false || b3.IsPublic != true {
		t.Errorf("expected public immutable 'pub let PI', got %+v", b3)
	}
}

func TestParseListAndObjectLiterals(t *testing.T) {
	code := `
let items = [1, 2, 3]
let empty = []
let record = { .x: 10, .y: 20 }
let empty_rec = {}
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decls) != 4 {
		t.Fatalf("expected 4 decls, got %d", len(decls))
	}

	b1 := decls[0].(loneast.BIND_DEC)
	list1, ok := b1.Value.(loneast.ListLiteral)
	if !ok || len(list1.Elements) != 3 {
		t.Errorf("expected ListLiteral with 3 elements, got %#v", b1.Value)
	}

	b2 := decls[1].(loneast.BIND_DEC)
	list2, ok := b2.Value.(loneast.ListLiteral)
	if !ok || len(list2.Elements) != 0 {
		t.Errorf("expected empty ListLiteral, got %#v", b2.Value)
	}

	b3 := decls[2].(loneast.BIND_DEC)
	obj1, ok := b3.Value.(loneast.ObjectLiteral)
	if !ok || len(obj1.Fields) != 2 {
		t.Errorf("expected ObjectLiteral with 2 fields, got %#v", b3.Value)
	}

	b4 := decls[3].(loneast.BIND_DEC)
	obj2, ok := b4.Value.(loneast.ObjectLiteral)
	if !ok || len(obj2.Fields) != 0 {
		t.Errorf("expected empty ObjectLiteral, got %#v", b4.Value)
	}
}

func TestParseScopedConditionHeaderIf(t *testing.T) {
	code := `
if: [let x = 0] x != 9 {
    break
}
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decls) != 1 {
		t.Fatalf("expected 1 stmt, got %d", len(decls))
	}

	ifStmt, ok := decls[0].(loneast.IfStmt)
	if !ok {
		t.Fatalf("expected IfStmt, got %#v", decls[0])
	}

	if len(ifStmt.Header) != 1 {
		t.Fatalf("expected 1 header binding, got %d", len(ifStmt.Header))
	}

	h := ifStmt.Header[0]
	if h.Name != "x" || h.IsMutable != false {
		t.Errorf("expected immutable header binding 'let x', got %+v", h)
	}

	bin, ok := ifStmt.Cond.(loneast.BinaryExpr)
	if !ok || bin.Op != "!=" {
		t.Errorf("expected condition 'x != 9', got %#v", ifStmt.Cond)
	}
}

func TestParseScopedConditionHeaderPackedIf(t *testing.T) {
	code := `
if: [let x = 0, let y = 7] {
    x is int and y is int => {
        drop 1
    },
    else => {
        drop 0
    }
}
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decls) != 1 {
		t.Fatalf("expected 1 stmt, got %d", len(decls))
	}

	packed, ok := decls[0].(loneast.PackedIfStmt)
	if !ok {
		t.Fatalf("expected PackedIfStmt, got %#v", decls[0])
	}

	if len(packed.Header) != 2 {
		t.Fatalf("expected 2 header bindings, got %d", len(packed.Header))
	}

	if packed.Header[0].Name != "x" || packed.Header[1].Name != "y" {
		t.Errorf("expected header bindings 'x' and 'y', got %+v, %+v", packed.Header[0], packed.Header[1])
	}

	if len(packed.Arms) != 2 {
		t.Fatalf("expected 2 arms, got %d", len(packed.Arms))
	}
}

func TestParsePostConditionLoop(t *testing.T) {
	code := `
loop {
    count = count + 1
} while: [count < 10]
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decls) != 1 {
		t.Fatalf("expected 1 stmt, got %d", len(decls))
	}

	loop, ok := decls[0].(loneast.LoopStmt)
	if !ok {
		t.Fatalf("expected LoopStmt, got %#v", decls[0])
	}

	if loop.WhileCond == nil {
		t.Fatalf("expected non-nil WhileCond in post-condition loop")
	}

	bin, ok := loop.WhileCond.(loneast.BinaryExpr)
	if !ok || bin.Op != "<" {
		t.Errorf("expected condition 'count < 10', got %#v", loop.WhileCond)
	}
}

func TestParseExplicitCastAndOperators(t *testing.T) {
	code := `
let x: f32 = int_val as f32
let cmp = a < b and c >= d
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b1 := decls[0].(loneast.BIND_DEC)
	cast, ok := b1.Value.(loneast.CastExpr)
	if !ok {
		t.Fatalf("expected CastExpr, got %#v", b1.Value)
	}
	tb, ok := cast.TargetType.Value.(*loneast.TypeBase)
	if !ok || tb.Name != "f32" {
		t.Errorf("expected cast to 'f32', got %#v", cast.TargetType)
	}

	b2 := decls[1].(loneast.BIND_DEC)
	bin, ok := b2.Value.(loneast.BinaryExpr)
	if !ok || bin.Op != "and" {
		t.Fatalf("expected binary 'and' expr, got %#v", b2.Value)
	}

	leftCmp, ok := bin.Left.(loneast.BinaryExpr)
	if !ok || leftCmp.Op != loneast.BINARY_OP_LESS {
		t.Errorf("expected '<' (%q), got %q", loneast.BINARY_OP_LESS, leftCmp.Op)
	}

	rightCmp, ok := bin.Right.(loneast.BinaryExpr)
	if !ok || rightCmp.Op != loneast.BINARY_OP_GREATER_EQ {
		t.Errorf("expected '>=' (%q), got %q", loneast.BINARY_OP_GREATER_EQ, rightCmp.Op)
	}
}

func TestParseTraitWithTrPrefix(t *testing.T) {
	code := `
pub type TrWriter = trait {
    fn write(self, buf: []u8) i32,
    fn flush(self) void
}

pub type TrIterator<T> = trait {
    fn has_next(self) bool,
    fn next(self) T
}
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decls) != 2 {
		t.Fatalf("expected 2 trait decls, got %d", len(decls))
	}

	tr1 := decls[0].(loneast.TYPE_DEC)
	if tr1.Name != "TrWriter" || tr1.Kind != loneast.TYPE_DEC_TRAIT || len(tr1.TraitSigs) != 2 {
		t.Errorf("expected TrWriter trait with 2 signatures, got %+v", tr1)
	}

	tr2 := decls[1].(loneast.TYPE_DEC)
	if tr2.Name != "TrIterator" || len(tr2.TypeParams) != 1 || tr2.TypeParams[0] != "T" {
		t.Errorf("expected TrIterator<T> trait, got %+v", tr2)
	}
}

func TestParsePrecedence(t *testing.T) {
	code := `
let res = a + b * c - d / e
let logic = a and b or c and d
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decls) != 2 {
		t.Fatalf("expected 2 decls, got %d", len(decls))
	}

	// a + (b * c) - (d / e)
	b1 := decls[0].(loneast.BIND_DEC)
	sub, ok := b1.Value.(loneast.BinaryExpr)
	if !ok || sub.Op != "-" {
		t.Fatalf("expected '-' at top level, got %#v", b1.Value)
	}

	add, ok := sub.Left.(loneast.BinaryExpr)
	if !ok || add.Op != "+" {
		t.Fatalf("expected '+' on left of '-', got %#v", sub.Left)
	}

	mul, ok := add.Right.(loneast.BinaryExpr)
	if !ok || mul.Op != "*" {
		t.Fatalf("expected '*' on right of '+', got %#v", add.Right)
	}

	div, ok := sub.Right.(loneast.BinaryExpr)
	if !ok || div.Op != "/" {
		t.Fatalf("expected '/' on right of '-', got %#v", sub.Right)
	}
}

func TestParseMethodChainingAndCalls(t *testing.T) {
	code := `
let val = user.get_name().to_upper()
let item = list[0].compute(1, 2)
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decls) != 2 {
		t.Fatalf("expected 2 decls, got %d", len(decls))
	}

	b1 := decls[0].(loneast.BIND_DEC)
	p1, ok := b1.Value.(loneast.PostFixExpr)
	if !ok || len(p1.Suffix) != 4 {
		// .get_name, (), .to_upper, ()
		t.Fatalf("expected 4 postfix suffixes, got %d (%#v)", len(p1.Suffix), p1.Suffix)
	}

	b2 := decls[1].(loneast.BIND_DEC)
	p2, ok := b2.Value.(loneast.PostFixExpr)
	if !ok || len(p2.Suffix) != 3 {
		// [0], .compute, (1, 2)
		t.Fatalf("expected 3 postfix suffixes, got %d (%#v)", len(p2.Suffix), p2.Suffix)
	}
}

func TestParseImplBlock(t *testing.T) {
	code := `
impl TrWriter for Stdout {
    fn write(self, buf: []u8) i32 {
        return 1
    }
}
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decls) != 1 {
		t.Fatalf("expected 1 impl block, got %d", len(decls))
	}

	impl, ok := decls[0].(loneast.IMPL_DEC)
	if !ok {
		t.Fatalf("expected IMPL_DEC, got %#v", decls[0])
	}

	if impl.TraitName != "TrWriter" {
		t.Errorf("expected trait 'TrWriter', got %q", impl.TraitName)
	}

	tb, ok := impl.TargetType.Value.(*loneast.TypeBase)
	if !ok || tb.Name != "Stdout" {
		t.Errorf("expected target 'Stdout', got %#v", impl.TargetType)
	}

	if len(impl.Methods) != 1 || impl.Methods[0].Name != "write" {
		t.Errorf("expected method 'write', got %+v", impl.Methods)
	}
}

func TestParseEnumAndMatch(t *testing.T) {
	code := `
pub type Shape = enum {
    Circle(radius: f32),
    Rect(width: f32, height: f32),
    Point,
}

fn area(s: Shape) f32 {
    return match s {
        Shape::Circle(r) => 3.14 * r * r,
        Shape::Rect(w, h) => w * h,
        Shape::Point => 0.0,
        _ => 0.0,
    }
}
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decls) != 2 {
		t.Fatalf("expected 2 decls, got %d", len(decls))
	}

	enumDec, ok := decls[0].(loneast.TYPE_DEC)
	if !ok || enumDec.Kind != loneast.TYPE_DEC_ENUM {
		t.Fatalf("expected TYPE_DEC_ENUM, got %#v", decls[0])
	}
	if enumDec.Name != "Shape" || len(enumDec.Variants) != 3 {
		t.Fatalf("expected Shape with 3 variants, got %#v", enumDec)
	}
	if enumDec.Variants[0].Name != "Circle" || len(enumDec.Variants[0].Fields) != 1 {
		t.Errorf("expected Circle with 1 field, got %#v", enumDec.Variants[0])
	}
	if enumDec.Variants[1].Name != "Rect" || len(enumDec.Variants[1].Fields) != 2 {
		t.Errorf("expected Rect with 2 fields, got %#v", enumDec.Variants[1])
	}
	if enumDec.Variants[2].Name != "Point" || len(enumDec.Variants[2].Fields) != 0 {
		t.Errorf("expected Point with 0 fields, got %#v", enumDec.Variants[2])
	}

	fnDec, ok := decls[1].(loneast.FN_DEC)
	if !ok || fnDec.Name != "area" {
		t.Fatalf("expected fn area, got %#v", decls[1])
	}
	retStmt, ok := fnDec.Body.Stmts[0].(loneast.ReturnStmt)
	if !ok {
		t.Fatalf("expected ReturnStmt, got %#v", fnDec.Body.Stmts[0])
	}
	matchExpr, ok := retStmt.Value.(loneast.MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr, got %#v", retStmt.Value)
	}
	if len(matchExpr.Arms) != 4 {
		t.Fatalf("expected 4 match arms, got %d", len(matchExpr.Arms))
	}
}
