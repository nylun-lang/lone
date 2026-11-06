package loneparser

import (
	loneast "lone/pkg/lone-parser-ast"
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
	if tr2.Name != "TrIterator" || len(tr2.TypeParams) != 1 || tr2.TypeParams[0].Name != "T" {
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

func TestParseDiscriminantIsHeaderDestructureAndMustTry(t *testing.T) {
	code := `
fn test_features(s: Shape, r: Result) void {
    let check = s is Shape::Circle and s is not Shape::Point

    if: [let Shape::Circle(radius) = s] {
        #print("circle\n")
    } else {
        #print("other\n")
    }

    let val1 = must r
    let val2 = try r
}
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decls) != 1 {
		t.Fatalf("expected 1 fn decl, got %d", len(decls))
	}

	fnDec := decls[0].(loneast.FN_DEC)
	stmts := fnDec.Body.Stmts
	if len(stmts) != 4 {
		t.Fatalf("expected 4 statements in test_features, got %d", len(stmts))
	}

	// Stmt 1: let check = ...
	bind1 := stmts[0].(loneast.BIND_DEC)
	if bind1.Name != "check" {
		t.Errorf("expected bind check, got %q", bind1.Name)
	}

	// Stmt 2: if: [let Shape::Circle(radius) = s]
	ifStmt := stmts[1].(loneast.IfStmt)
	if len(ifStmt.Header) != 1 {
		t.Fatalf("expected 1 header binding, got %d", len(ifStmt.Header))
	}
	vp, ok := ifStmt.Header[0].Pattern.(loneast.VariantPattern)
	if !ok || vp.Variant != "Circle" || len(vp.Bindings) != 1 {
		t.Fatalf("expected VariantPattern Circle with 1 binding, got %#v", ifStmt.Header[0].Pattern)
	}

	// Stmt 3: let val1 = must r
	bind3 := stmts[2].(loneast.BIND_DEC)
	if _, ok := bind3.Value.(loneast.MustExpr); !ok {
		t.Fatalf("expected MustExpr, got %#v", bind3.Value)
	}

	// Stmt 4: let val2 = try r
	bind4 := stmts[3].(loneast.BIND_DEC)
	if _, ok := bind4.Value.(loneast.TryExpr); !ok {
		t.Fatalf("expected TryExpr, got %#v", bind4.Value)
	}
}

func TestParseGenericsAndInfer(t *testing.T) {
	code := `
pub type Result<V, U = Error> = enum {
    Ok(val: V),
    Err(err: U),
}

fn wrap<T>(val: T) Result<T, infer> {
    return Result::Ok(val)
}
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(decls) != 2 {
		t.Fatalf("expected 2 decls, got %d", len(decls))
	}

	tdec := decls[0].(loneast.TYPE_DEC)
	if tdec.Name != "Result" || len(tdec.TypeParams) != 2 {
		t.Fatalf("expected Result with 2 type params, got %+v", tdec)
	}
	if tdec.TypeParams[0].Name != "V" || tdec.TypeParams[0].Default != nil {
		t.Errorf("expected param V with no default")
	}
	if tdec.TypeParams[1].Name != "U" || tdec.TypeParams[1].Default == nil {
		t.Fatalf("expected param U with default")
	}
	tb := tdec.TypeParams[1].Default.Value.(*loneast.TypeBase)
	if tb.Name != "Error" {
		t.Errorf("expected default type Error, got %q", tb.Name)
	}

	fdec := decls[1].(loneast.FN_DEC)
	if fdec.Name != "wrap" || len(fdec.TypeParams) != 1 || fdec.TypeParams[0].Name != "T" {
		t.Fatalf("expected fn wrap<T>, got %+v", fdec)
	}
	if fdec.ReturnType == nil {
		t.Fatalf("expected return type for wrap")
	}
	retTb := fdec.ReturnType.Value.(*loneast.TypeBase)
	if retTb.Name != "Result" || len(retTb.Args) != 2 {
		t.Fatalf("expected Result<T, infer>, got %+v", retTb)
	}
	inferTb := retTb.Args[1].Value.(*loneast.TypeBase)
	if inferTb.Name != "infer" {
		t.Errorf("expected arg 1 to be infer, got %q", inferTb.Name)
	}
}

func TestParseGenericConstraints(t *testing.T) {
	code := `
pub type Result<V, U = Error where { U is Error }> = enum {
    Ok(val: V),
    Err(err: U),
}

fn write_data<T where { T satisfies TrWriter }>(writer: T, msg: str) void {
}
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(decls) != 2 {
		t.Fatalf("expected 2 decls, got %d", len(decls))
	}

	tdec := decls[0].(loneast.TYPE_DEC)
	if len(tdec.Constraints) != 1 {
		t.Fatalf("expected 1 constraint on Result, got %d", len(tdec.Constraints))
	}
	c1 := tdec.Constraints[0]
	if c1.TypeParam != "U" || c1.Op != loneast.ConstraintIs {
		t.Errorf("expected U is ..., got %+v", c1)
	}
	tb1 := c1.Target.Value.(*loneast.TypeBase)
	if tb1.Name != "Error" {
		t.Errorf("expected target Error, got %q", tb1.Name)
	}

	fdec := decls[1].(loneast.FN_DEC)
	if len(fdec.Constraints) != 1 {
		t.Fatalf("expected 1 constraint on write_data, got %d", len(fdec.Constraints))
	}
	c2 := fdec.Constraints[0]
	if c2.TypeParam != "T" || c2.Op != loneast.ConstraintSatisfies {
		t.Errorf("expected T satisfies ..., got %+v", c2)
	}
	tb2 := c2.Target.Value.(*loneast.TypeBase)
	if tb2.Name != "TrWriter" {
		t.Errorf("expected target TrWriter, got %q", tb2.Name)
	}
}

func TestParseStructModifiersAndConstructorCall(t *testing.T) {
	code := `
type Point = struct {
    ro id: int,
    [x, y]: int,
    ro [w, h]: f32,
    _secret: str,

    pub fn __init__(id: int, x: int, y: int) Point {
        return Point::{ .id: id, .x: x, .y: y, .w: 0.0, .h: 0.0, ._secret: "hidden" }
    }
}

fn main() void {
    let p = Point::(1, 10, 20)
}
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(decls) != 2 {
		t.Fatalf("expected 2 decls, got %d", len(decls))
	}

	st := decls[0].(loneast.TYPE_DEC)
	if len(st.Fields) != 6 {
		t.Fatalf("expected 6 fields (id, x, y, w, h, _secret), got %d: %+v", len(st.Fields), st.Fields)
	}

	if !st.Fields[0].IsReadOnly || st.Fields[0].Name != "id" {
		t.Errorf("expected ro id, got %+v", st.Fields[0])
	}
	if st.Fields[1].Name != "x" || st.Fields[2].Name != "y" {
		t.Errorf("expected [x, y], got %s, %s", st.Fields[1].Name, st.Fields[2].Name)
	}
	if !st.Fields[3].IsReadOnly || st.Fields[3].Name != "w" || !st.Fields[4].IsReadOnly || st.Fields[4].Name != "h" {
		t.Errorf("expected ro [w, h], got %+v, %+v", st.Fields[3], st.Fields[4])
	}
	if st.Fields[5].Name != "_secret" {
		t.Errorf("expected _secret, got %s", st.Fields[5].Name)
	}

	fn := decls[1].(loneast.FN_DEC)
	bind := fn.Body.Stmts[0].(loneast.BIND_DEC)
	pf := bind.Value.(loneast.PostFixExpr)
	if len(pf.Suffix) != 1 {
		t.Fatalf("expected 1 suffix, got %d", len(pf.Suffix))
	}
	constr, ok := pf.Suffix[0].(loneast.SuffixConstructorCall)
	if !ok {
		t.Fatalf("expected SuffixConstructorCall, got %T", pf.Suffix[0])
	}
	if len(constr.Args) != 3 {
		t.Errorf("expected 3 constructor args, got %d", len(constr.Args))
	}
}

func TestParseGroupedDeclarationsTuplesAndAnonFn(t *testing.T) {
	code := `
fn draw_rect([x, y, w, h]: int, [point_a, point_b]: (int, int), color: str) void {
    var [top, bottom, left, right]: int = 0
    let pair: (int, str) = (42, "hello")
    let (num, msg) = pair
    let val0 = pair.0
    let val1 = pair.1

    let sq = fn(x: int) int {
        return x * x
    }
}
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(decls) != 1 {
		t.Fatalf("expected 1 fn decl, got %d", len(decls))
	}

	fn := decls[0].(loneast.FN_DEC)
	// Parameters: [x, y, w, h] (4) + [point_a, point_b] (2) + color (1) = 7 params
	if len(fn.Params) != 7 {
		t.Fatalf("expected 7 expanded params, got %d", len(fn.Params))
	}
	if fn.Params[0].Name != "x" || fn.Params[3].Name != "h" {
		t.Errorf("unexpected params: %+v", fn.Params)
	}
	if fn.Params[4].Type.Kind != loneast.TYPE_TUPLE {
		t.Errorf("expected tuple type for point_a, got %v", fn.Params[4].Type.Kind)
	}

	stmts := fn.Body.Stmts
	// Stmt 0: var [top, bottom, left, right]: int = 0
	b0 := stmts[0].(loneast.BIND_DEC)
	if len(b0.Names) != 4 || b0.Names[0] != "top" || b0.Names[3] != "right" {
		t.Errorf("expected 4 grouped names, got %+v", b0.Names)
	}

	// Stmt 1: let pair = (42, "hello")
	b1 := stmts[1].(loneast.BIND_DEC)
	tupExpr, ok := b1.Value.(loneast.TupleExpr)
	if !ok || len(tupExpr.Elements) != 2 {
		t.Fatalf("expected TupleExpr with 2 elements, got %#v", b1.Value)
	}

	// Stmt 2: let (num, msg) = pair
	b2 := stmts[2].(loneast.BIND_DEC)
	tupPat, ok := b2.Pattern.(loneast.TuplePattern)
	if !ok || len(tupPat.Elements) != 2 {
		t.Fatalf("expected TuplePattern with 2 elements, got %#v", b2.Pattern)
	}

	// Stmt 3 & 4: pair.0 and pair.1
	b3 := stmts[3].(loneast.BIND_DEC)
	pf3 := b3.Value.(loneast.PostFixExpr)
	mem3 := pf3.Suffix[0].(loneast.SuffixMemberAccess)
	if mem3.Name != "0" {
		t.Errorf("expected member name '0', got %q", mem3.Name)
	}

	// Stmt 5: let sq = fn(x: int) int { ... }
	b5 := stmts[5].(loneast.BIND_DEC)
	anonFn, ok := b5.Value.(loneast.AnonFnExpr)
	if !ok {
		t.Fatalf("expected AnonFnExpr, got %#v", b5.Value)
	}
	if len(anonFn.Params) != 1 || anonFn.Params[0].Name != "x" {
		t.Errorf("expected param 'x', got %+v", anonFn.Params)
	}
}

func TestParseInterpolatedString(t *testing.T) {
	code := `
let msg = f"Hello, ${name}! You have ${count + 1} items."
let esc = f"Price is \${price}"
`
	decls, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(decls) != 2 {
		t.Fatalf("expected 2 decls, got %d", len(decls))
	}

	b1 := decls[0].(loneast.BIND_DEC)
	interp, ok := b1.Value.(loneast.InterpolatedStringExpr)
	if !ok {
		t.Fatalf("expected InterpolatedStringExpr, got %#v", b1.Value)
	}
	// Parts should be: "Hello, ", `name`, "! You have ", `count + 1`, " items."
	if len(interp.Parts) != 5 {
		t.Fatalf("expected 5 parts, got %d", len(interp.Parts))
	}

	b2 := decls[1].(loneast.BIND_DEC)
	interp2, ok := b2.Value.(loneast.InterpolatedStringExpr)
	if !ok {
		t.Fatalf("expected InterpolatedStringExpr for esc, got %#v", b2.Value)
	}
	// Should have literal parts with "${price}"
	if len(interp2.Parts) != 1 {
		t.Fatalf("expected 1 part for escaped string, got %d", len(interp2.Parts))
	}
	lit, ok := interp2.Parts[0].(loneast.Literal)
	if !ok || lit.Value != "Price is ${price}" {
		t.Errorf("expected escaped text 'Price is ${price}', got %#v", interp2.Parts[0])
	}

	// Error tests
	_, err = Parse([]byte(`let bad = f"empty ${}"`))
	if err == nil {
		t.Errorf("expected error for empty ${}, got nil")
	}

	_, err = Parse([]byte(`let bad = f"unclosed ${val"`))
	if err == nil {
		t.Errorf("expected error for unclosed ${, got nil")
	}
}



