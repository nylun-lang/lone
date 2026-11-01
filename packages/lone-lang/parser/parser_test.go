package parser_test

import (
	ir "lonelang/irtypes"
	"lonelang/parser"
	"testing"
)

func parseLone(t *testing.T, code string) []ir.BIND_DEC {
	t.Helper()
	res, err := parser.Parse("test.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to parse %q: %v", code, err)
	}

	rawList, ok := res.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", res)
	}

	var decls []ir.BIND_DEC
	for _, item := range rawList {
		pair, ok := item.([]any)
		if !ok || len(pair) < 2 {
			continue
		}
		dec, ok := pair[1].(ir.BIND_DEC)
		if ok {
			decls = append(decls, dec)
		}
	}
	return decls
}

func TestTypeBase(t *testing.T) {
	code := `
var a: int
var b: list<string>
var c: Map<string, int>
`
	decls := parseLone(t, code)
	if len(decls) != 3 {
		t.Fatalf("expected 3 declarations, got %d", len(decls))
	}

	// a: int
	if decls[0].Type.Kind != ir.TYPE_BASE {
		t.Errorf("expected TYPE_BASE, got %v", decls[0].Type.Kind)
	}
	baseA := decls[0].Type.AsBase()
	if baseA == nil || baseA.Name != "int" || len(baseA.Args) != 0 {
		t.Errorf("unexpected base type: %+v", baseA)
	}

	// b: list<string>
	baseB := decls[1].Type.AsBase()
	if baseB == nil || baseB.Name != "list" || len(baseB.Args) != 1 {
		t.Fatalf("unexpected base type: %+v", baseB)
	}
	if baseB.Args[0].AsBase().Name != "string" {
		t.Errorf("expected string arg, got %v", baseB.Args[0].AsBase().Name)
	}

	// c: Map<string, int>
	baseC := decls[2].Type.AsBase()
	if baseC == nil || baseC.Name != "Map" || len(baseC.Args) != 2 {
		t.Fatalf("unexpected base type: %+v", baseC)
	}
	if baseC.Args[0].AsBase().Name != "string" || baseC.Args[1].AsBase().Name != "int" {
		t.Errorf("unexpected type args: %+v", baseC.Args)
	}
}

func TestTypeList(t *testing.T) {
	code := `
var numbers: []int
var matrix: [][]int
var opt_list: ?[]int
var list_opt: []?string
`
	decls := parseLone(t, code)
	if len(decls) != 4 {
		t.Fatalf("expected 4 declarations, got %d", len(decls))
	}

	// numbers: []int
	if decls[0].Type.Kind != ir.TYPE_LIST {
		t.Errorf("expected TYPE_LIST, got %v", decls[0].Type.Kind)
	}
	list0 := decls[0].Type.AsList()
	if list0 == nil || list0.ItemType.AsBase().Name != "int" {
		t.Errorf("unexpected list0 item type: %+v", list0)
	}
	if list0.IsFixed() {
		t.Errorf("expected dynamic list, got fixed")
	}

	// matrix: [][]int
	list1 := decls[1].Type.AsList()
	if list1 == nil || list1.ItemType.Kind != ir.TYPE_LIST {
		t.Fatalf("expected inner TYPE_LIST, got %v", list1)
	}
	innerList := list1.ItemType.AsList()
	if innerList.ItemType.AsBase().Name != "int" {
		t.Errorf("expected inner int, got %v", innerList.ItemType.AsBase().Name)
	}

	// opt_list: ?[]int
	if !decls[2].Type.Constrain.Optional || decls[2].Type.Kind != ir.TYPE_LIST {
		t.Errorf("expected optional TYPE_LIST: %+v", decls[2].Type)
	}

	// list_opt: []?string
	list3 := decls[3].Type.AsList()
	if !list3.ItemType.Constrain.Optional || list3.ItemType.AsBase().Name != "string" {
		t.Errorf("expected inner optional string: %+v", list3.ItemType)
	}
}

func TestFixedTypeList(t *testing.T) {
	code := `
var fixed_arr: [9]int
var fixed_named: [BUFFER_SIZE]string
var multi_fixed: [9][10]int
var fixed_dynamic: [9][]int
`
	decls := parseLone(t, code)
	if len(decls) != 4 {
		t.Fatalf("expected 4 declarations, got %d", len(decls))
	}

	// fixed_arr: [9]int
	list0 := decls[0].Type.AsList()
	if list0 == nil || !list0.IsFixed() {
		t.Fatalf("expected fixed list: %+v", list0)
	}
	size0 := list0.Size
	if pf, ok := size0.(ir.PostFixExpr); ok {
		size0 = pf.Base
	}
	litSize, ok := size0.(ir.Literal)
	if !ok || litSize.Kind != ir.LITERAL_INT || litSize.Value != 9 {
		t.Errorf("expected int literal 9 for size, got %+v", list0.Size)
	}
	if list0.ItemType.AsBase().Name != "int" {
		t.Errorf("expected item type int, got %+v", list0.ItemType)
	}

	// fixed_named: [BUFFER_SIZE]string
	list1 := decls[1].Type.AsList()
	if list1 == nil || !list1.IsFixed() {
		t.Fatalf("expected fixed list: %+v", list1)
	}
	size1 := list1.Size
	if pf, ok := size1.(ir.PostFixExpr); ok {
		size1 = pf.Base
	}
	nameRef, ok := size1.(ir.NameRef)
	if !ok || nameRef.Name != "BUFFER_SIZE" {
		t.Errorf("expected name ref BUFFER_SIZE, got %+v", list1.Size)
	}
	if list1.ItemType.AsBase().Name != "string" {
		t.Errorf("expected item type string, got %+v", list1.ItemType)
	}

	// multi_fixed: [9][10]int
	list2 := decls[2].Type.AsList()
	if list2 == nil || !list2.IsFixed() {
		t.Fatalf("expected fixed list: %+v", list2)
	}
	innerList2 := list2.ItemType.AsList()
	if innerList2 == nil || !innerList2.IsFixed() {
		t.Fatalf("expected inner fixed list: %+v", innerList2)
	}
	size2 := innerList2.Size
	if pf, ok := size2.(ir.PostFixExpr); ok {
		size2 = pf.Base
	}
	innerLit, ok := size2.(ir.Literal)
	if !ok || innerLit.Value != 10 {
		t.Errorf("expected inner size 10, got %+v", innerList2.Size)
	}

	// fixed_dynamic: [9][]int
	list3 := decls[3].Type.AsList()
	if list3 == nil || !list3.IsFixed() {
		t.Fatalf("expected outer fixed list: %+v", list3)
	}
	innerList3 := list3.ItemType.AsList()
	if innerList3 == nil || innerList3.IsFixed() {
		t.Fatalf("expected inner dynamic list: %+v", innerList3)
	}
}

func TestTypeFn(t *testing.T) {
	code := `
var fn_empty: fn () int
var fn_void: fn ()
var fn_multi: fn (a: int, b: string) bool
var fn_nested: fn (cb: fn (x: int) bool) []string
`
	decls := parseLone(t, code)
	if len(decls) != 4 {
		t.Fatalf("expected 4 declarations, got %d", len(decls))
	}

	// fn_empty: fn () int
	fn0 := decls[0].Type.AsFn()
	if fn0 == nil || len(fn0.Params) != 0 || fn0.ReturnType == nil || fn0.ReturnType.AsBase().Name != "int" {
		t.Errorf("unexpected fn0: %+v", fn0)
	}

	// fn_void: fn ()
	fn1 := decls[1].Type.AsFn()
	if fn1 == nil || len(fn1.Params) != 0 || fn1.ReturnType != nil {
		t.Errorf("unexpected fn1: %+v", fn1)
	}

	// fn_multi: fn (a: int, b: string) bool
	fn2 := decls[2].Type.AsFn()
	if fn2 == nil || len(fn2.Params) != 2 || fn2.ReturnType == nil || fn2.ReturnType.AsBase().Name != "bool" {
		t.Fatalf("unexpected fn2: %+v", fn2)
	}
	if fn2.Params[0].Name != "a" || fn2.Params[0].Type.AsBase().Name != "int" {
		t.Errorf("unexpected param 0: %+v", fn2.Params[0])
	}
	if fn2.Params[1].Name != "b" || fn2.Params[1].Type.AsBase().Name != "string" {
		t.Errorf("unexpected param 1: %+v", fn2.Params[1])
	}

	// fn_nested: fn (cb: fn (x: int) bool) []string
	fn3 := decls[3].Type.AsFn()
	if fn3 == nil || len(fn3.Params) != 1 || fn3.ReturnType == nil || fn3.ReturnType.Kind != ir.TYPE_LIST {
		t.Fatalf("unexpected fn3: %+v", fn3)
	}
	paramCb := fn3.Params[0].Type.AsFn()
	if paramCb == nil || len(paramCb.Params) != 1 || paramCb.ReturnType.AsBase().Name != "bool" {
		t.Errorf("unexpected nested fn param: %+v", paramCb)
	}
	returnList := fn3.ReturnType.AsList()
	if returnList == nil || returnList.ItemType.AsBase().Name != "string" {
		t.Errorf("unexpected return list item: %+v", returnList)
	}
}

func TestObjectLiteral(t *testing.T) {
	code := `
var data = { .foo: "bar", .num: 42 }
`
	decls := parseLone(t, code)
	if len(decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(decls))
	}

	pf, ok := decls[0].Value.(ir.PostFixExpr)
	if !ok {
		t.Fatalf("expected PostFixExpr, got %T", decls[0].Value)
	}

	obj, ok := pf.Base.(ir.ObjectLiteral)
	if !ok {
		t.Fatalf("expected ObjectLiteral, got %T", pf.Base)
	}

	if len(obj.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(obj.Fields))
	}
	if obj.Fields[0].Name != "foo" {
		t.Errorf("expected field 0 name 'foo', got %q", obj.Fields[0].Name)
	}
	if obj.Fields[1].Name != "num" {
		t.Errorf("expected field 1 name 'num', got %q", obj.Fields[1].Name)
	}
}

func TestListLiteral(t *testing.T) {
	code := `
var ids = { 1, 2, 3, 4 }
`
	decls := parseLone(t, code)
	if len(decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(decls))
	}

	pf, ok := decls[0].Value.(ir.PostFixExpr)
	if !ok {
		t.Fatalf("expected PostFixExpr, got %T", decls[0].Value)
	}

	list, ok := pf.Base.(ir.ListLiteral)
	if !ok {
		t.Fatalf("expected ListLiteral, got %T", pf.Base)
	}

	if len(list.Content) != 4 {
		t.Fatalf("expected 4 elements, got %d", len(list.Content))
	}
}

func TestPostfixExpressions(t *testing.T) {
	code := `
var a = user.name
var b = math:sin(3.14)
var c = Type::method
var d = Color::(255, 0, 0)
var e = User::{ .name: "Alice" }
var f = Ints::{ 1, 2, 3 }
var g = obj.calc(1).val
`
	decls := parseLone(t, code)
	if len(decls) != 7 {
		t.Fatalf("expected 7 declarations, got %d", len(decls))
	}

	// a = user.name
	pfA := decls[0].Value.(ir.PostFixExpr)
	if len(pfA.Suffix) != 1 || pfA.Suffix[0].IsSuffix() != ir.SUFFIX_KIND_MEMBER {
		t.Errorf("expected 1 member access suffix, got %+v", pfA.Suffix)
	}
	if memA := pfA.Suffix[0].(ir.SuffixMemberAccess); memA.Name != "name" {
		t.Errorf("expected member name 'name', got %q", memA.Name)
	}

	// b = math:sin(3.14)
	pfB := decls[1].Value.(ir.PostFixExpr)
	if len(pfB.Suffix) != 2 {
		t.Fatalf("expected 2 suffixes (namespace, call), got %d", len(pfB.Suffix))
	}
	if pfB.Suffix[0].IsSuffix() != ir.SUFFIX_KIND_NAMESPACE {
		t.Errorf("expected namespace suffix, got %v", pfB.Suffix[0].IsSuffix())
	}
	if pfB.Suffix[1].IsSuffix() != ir.SUFFIX_KIND_CALL {
		t.Errorf("expected call suffix, got %v", pfB.Suffix[1].IsSuffix())
	}

	// c = Type::method
	pfC := decls[2].Value.(ir.PostFixExpr)
	if len(pfC.Suffix) != 1 || pfC.Suffix[0].IsSuffix() != ir.SUFFIX_KIND_ASSOCIATED {
		t.Errorf("expected associated access, got %+v", pfC.Suffix)
	}

	// d = Color::(255, 0, 0)
	pfD := decls[3].Value.(ir.PostFixExpr)
	if len(pfD.Suffix) != 1 || pfD.Suffix[0].IsSuffix() != ir.SUFFIX_KIND_CALL_TYPE_INIT {
		t.Errorf("expected call type init, got %+v", pfD.Suffix)
	}
	initD := pfD.Suffix[0].(ir.SuffixCallTypeInit)
	if len(initD.Args) != 3 {
		t.Errorf("expected 3 init args, got %d", len(initD.Args))
	}

	// e = User::{ .name: "Alice" }
	pfE := decls[4].Value.(ir.PostFixExpr)
	if len(pfE.Suffix) != 1 || pfE.Suffix[0].IsSuffix() != ir.SUFFIX_KIND_CALL_TYPE_MAKE {
		t.Errorf("expected call type make, got %+v", pfE.Suffix)
	}
	makeE := pfE.Suffix[0].(ir.SuffixCallTypeMake)
	if makeE.Kind != ir.CALL_TYPE_MAKE_OBJ {
		t.Errorf("expected CALL_TYPE_MAKE_OBJ, got %v", makeE.Kind)
	}

	// f = Ints::{ 1, 2, 3 }
	pfF := decls[5].Value.(ir.PostFixExpr)
	if len(pfF.Suffix) != 1 || pfF.Suffix[0].IsSuffix() != ir.SUFFIX_KIND_CALL_TYPE_MAKE {
		t.Errorf("expected call type make, got %+v", pfF.Suffix)
	}
	makeF := pfF.Suffix[0].(ir.SuffixCallTypeMake)
	if makeF.Kind != ir.CALL_TYPE_MAKE_LIST {
		t.Errorf("expected CALL_TYPE_MAKE_LIST, got %v", makeF.Kind)
	}

	// g = obj.calc(1).val
	pfG := decls[6].Value.(ir.PostFixExpr)
	if len(pfG.Suffix) != 3 {
		t.Fatalf("expected 3 suffixes (member, call, member), got %d", len(pfG.Suffix))
	}
	if pfG.Suffix[0].IsSuffix() != ir.SUFFIX_KIND_MEMBER ||
		pfG.Suffix[1].IsSuffix() != ir.SUFFIX_KIND_CALL ||
		pfG.Suffix[2].IsSuffix() != ir.SUFFIX_KIND_MEMBER {
		t.Errorf("unexpected suffix chain: %+v", pfG.Suffix)
	}
}

func parseFunctions(t *testing.T, code string) []ir.FN_DEC {
	t.Helper()
	res, err := parser.Parse("test.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to parse %q: %v", code, err)
	}

	rawList, ok := res.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", res)
	}

	var decls []ir.FN_DEC
	for _, item := range rawList {
		pair, ok := item.([]any)
		if !ok || len(pair) < 2 {
			continue
		}
		dec, ok := pair[1].(ir.FN_DEC)
		if ok {
			decls = append(decls, dec)
		}
	}
	return decls
}

func TestFunctionDeclaration(t *testing.T) {
	code := `
fn noop() {
}

fn add(a: int, b: int) int {
    return a
}

fn compute(items: []string, factor: ?float) []float {
    var x = 10
    return x
}

fn #init(name: string) Self {
}

fn with_cb(cb: fn (x: int) bool) bool {
    return cb(42)
}
`
	fns := parseFunctions(t, code)
	if len(fns) != 5 {
		t.Fatalf("expected 5 function declarations, got %d", len(fns))
	}

	// 1. noop()
	if fns[0].Name != "noop" {
		t.Errorf("expected name 'noop', got %q", fns[0].Name)
	}
	if len(fns[0].Params) != 0 {
		t.Errorf("expected 0 params, got %d", len(fns[0].Params))
	}
	if fns[0].ReturnType != nil {
		t.Errorf("expected nil return type, got %+v", fns[0].ReturnType)
	}
	if len(fns[0].Body.Stmts) != 0 {
		t.Errorf("expected 0 stmts, got %d", len(fns[0].Body.Stmts))
	}

	// 2. add(a: int, b: int) int
	if fns[1].Name != "add" {
		t.Errorf("expected name 'add', got %q", fns[1].Name)
	}
	if len(fns[1].Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(fns[1].Params))
	}
	if fns[1].Params[0].Name != "a" || fns[1].Params[0].Type.AsBase().Name != "int" {
		t.Errorf("unexpected param 0: %+v", fns[1].Params[0])
	}
	if fns[1].Params[1].Name != "b" || fns[1].Params[1].Type.AsBase().Name != "int" {
		t.Errorf("unexpected param 1: %+v", fns[1].Params[1])
	}
	if fns[1].ReturnType == nil || fns[1].ReturnType.AsBase().Name != "int" {
		t.Errorf("expected return type int, got %+v", fns[1].ReturnType)
	}
	if len(fns[1].Body.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in add, got %d", len(fns[1].Body.Stmts))
	}
	retStmt, ok := fns[1].Body.Stmts[0].(ir.ReturnStmt)
	if !ok {
		t.Fatalf("expected ReturnStmt, got %T", fns[1].Body.Stmts[0])
	}
	if retStmt.Value == nil {
		t.Errorf("expected return expression, got nil")
	}

	// 3. compute(items: []string, factor: ?float) []float
	if fns[2].Name != "compute" {
		t.Errorf("expected name 'compute', got %q", fns[2].Name)
	}
	if fns[2].Params[0].Type.Kind != ir.TYPE_LIST {
		t.Errorf("expected param 0 TYPE_LIST, got %v", fns[2].Params[0].Type.Kind)
	}
	if !fns[2].Params[1].Type.Constrain.Optional {
		t.Errorf("expected param 1 to be optional")
	}
	if fns[2].ReturnType.Kind != ir.TYPE_LIST {
		t.Errorf("expected return type TYPE_LIST, got %v", fns[2].ReturnType.Kind)
	}
	if len(fns[2].Body.Stmts) != 2 {
		t.Fatalf("expected 2 stmts in compute, got %d", len(fns[2].Body.Stmts))
	}
	if _, ok := fns[2].Body.Stmts[0].(ir.BIND_DEC); !ok {
		t.Errorf("expected first stmt to be BIND_DEC, got %T", fns[2].Body.Stmts[0])
	}

	// 4. #init(name: string) Self
	if fns[3].Name != "#init" {
		t.Errorf("expected name '#init', got %q", fns[3].Name)
	}
	if fns[3].ReturnType.AsBase().Name != "Self" {
		t.Errorf("expected return type Self, got %+v", fns[3].ReturnType)
	}

	// 5. with_cb(cb: fn (x: int) bool) bool
	if fns[4].Name != "with_cb" {
		t.Errorf("expected name 'with_cb', got %q", fns[4].Name)
	}
	if fns[4].Params[0].Type.Kind != ir.TYPE_FN {
		t.Errorf("expected param 0 TYPE_FN, got %v", fns[4].Params[0].Type.Kind)
	}
}
