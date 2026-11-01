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

func TestLabelDecAndBreakStmt(t *testing.T) {
	code := `
var result = blk: {
    var x = 10
    break blk: x + 1
}

fn test_block() int {
    var computed = eval: {
        break eval: 42
    }
    break eval:
    break 100
    break
    return computed
}
`
	res, err := parser.Parse("test_label.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	rawList := res.([]any)
	if len(rawList) != 2 {
		t.Fatalf("expected 2 top-level items, got %d", len(rawList))
	}

	// 1. var result = blk: { ... }
	varDec := rawList[0].([]any)[1].(ir.BIND_DEC)
	if varDec.Name != "result" {
		t.Errorf("expected var 'result', got %q", varDec.Name)
	}

	pf, ok := varDec.Value.(ir.PostFixExpr)
	if !ok {
		t.Fatalf("expected PostFixExpr for value, got %T", varDec.Value)
	}
	labelDec, ok := pf.Base.(ir.LabelDec)
	if !ok {
		t.Fatalf("expected LabelDec, got %T", pf.Base)
	}
	if labelDec.Name != "blk" {
		t.Errorf("expected label name 'blk', got %q", labelDec.Name)
	}
	if len(labelDec.Body.Stmts) != 2 {
		t.Fatalf("expected 2 statements in blk, got %d", len(labelDec.Body.Stmts))
	}

	// Statement 1 in blk: var x = 10
	if _, ok := labelDec.Body.Stmts[0].(ir.BIND_DEC); !ok {
		t.Errorf("expected BIND_DEC in blk, got %T", labelDec.Body.Stmts[0])
	}

	// Statement 2 in blk: break blk: x + 1
	break1, ok := labelDec.Body.Stmts[1].(ir.BreakStmt)
	if !ok {
		t.Fatalf("expected BreakStmt, got %T", labelDec.Body.Stmts[1])
	}
	if break1.Label != "blk" {
		t.Errorf("expected break label 'blk', got %q", break1.Label)
	}
	if break1.Value == nil {
		t.Errorf("expected break value, got nil")
	}

	// 2. fn test_block()
	fnDec := rawList[1].([]any)[1].(ir.FN_DEC)
	if fnDec.Name != "test_block" {
		t.Errorf("expected fn 'test_block', got %q", fnDec.Name)
	}
	if len(fnDec.Body.Stmts) != 5 {
		t.Fatalf("expected 5 stmts in fn, got %d", len(fnDec.Body.Stmts))
	}

	// Stmt 1: var computed = eval: { break eval: 42 }
	varComputed := fnDec.Body.Stmts[0].(ir.BIND_DEC)
	pfEval := varComputed.Value.(ir.PostFixExpr)
	labelEval := pfEval.Base.(ir.LabelDec)
	if labelEval.Name != "eval" {
		t.Errorf("expected label 'eval', got %q", labelEval.Name)
	}

	// Stmt 2: break eval: (label only, no value)
	break2 := fnDec.Body.Stmts[1].(ir.BreakStmt)
	if break2.Label != "eval" || break2.Value != nil {
		t.Errorf("unexpected break2: %+v", break2)
	}

	// Stmt 3: break 100 (unlabeled with value)
	break3 := fnDec.Body.Stmts[2].(ir.BreakStmt)
	if break3.Label != "" || break3.Value == nil {
		t.Errorf("unexpected break3: %+v", break3)
	}

	// Stmt 4: break (bare break)
	break4 := fnDec.Body.Stmts[3].(ir.BreakStmt)
	if break4.Label != "" || break4.Value != nil {
		t.Errorf("unexpected break4: %+v", break4)
	}

	// Stmt 5: return computed
	if _, ok := fnDec.Body.Stmts[4].(ir.ReturnStmt); !ok {
		t.Errorf("expected ReturnStmt for stmt 5, got %T", fnDec.Body.Stmts[4])
	}
}

func TestIfStmtAndWhileStmt(t *testing.T) {
	code := `
fn classify(x: int) int {
    if x > 0 {
        return 1
    } else if x == 0 {
        return 0
    } else {
        return -1
    }
}

fn count_up(limit: int) int {
    var count = 0
    while count < limit {
        if count == 5 {
            break
        }
        var next = count + 1
    }
    return count
}

var res = loop: while true {
    break loop: 42
}
`
	res, err := parser.Parse("test_control_flow.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	rawList := res.([]any)
	if len(rawList) != 3 {
		t.Fatalf("expected 3 top-level items, got %d", len(rawList))
	}

	// 1. fn classify(x: int) int
	fnClassify := rawList[0].([]any)[1].(ir.FN_DEC)
	if fnClassify.Name != "classify" {
		t.Errorf("expected fn 'classify', got %q", fnClassify.Name)
	}
	if len(fnClassify.Body.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in classify, got %d", len(fnClassify.Body.Stmts))
	}

	ifStmt, ok := fnClassify.Body.Stmts[0].(ir.IfStmt)
	if !ok {
		t.Fatalf("expected IfStmt, got %T", fnClassify.Body.Stmts[0])
	}
	if ifStmt.Cond == nil || len(ifStmt.Then.Stmts) != 1 {
		t.Errorf("unexpected ifStmt then branch: %+v", ifStmt)
	}

	// Else should be chained IfStmt: else if x == 0
	elseIf, ok := ifStmt.Else.(ir.IfStmt)
	if !ok {
		t.Fatalf("expected nested IfStmt in Else, got %T", ifStmt.Else)
	}
	if elseIf.Cond == nil || len(elseIf.Then.Stmts) != 1 {
		t.Errorf("unexpected elseIf then branch: %+v", elseIf)
	}

	// Inner else should be Block
	elseBlock, ok := elseIf.Else.(ir.Block)
	if !ok {
		t.Fatalf("expected Block for final else, got %T", elseIf.Else)
	}
	if len(elseBlock.Stmts) != 1 {
		t.Errorf("expected 1 stmt in elseBlock, got %d", len(elseBlock.Stmts))
	}

	// 2. fn count_up(limit: int) int
	fnCount := rawList[1].([]any)[1].(ir.FN_DEC)
	if fnCount.Name != "count_up" {
		t.Errorf("expected fn 'count_up', got %q", fnCount.Name)
	}
	if len(fnCount.Body.Stmts) != 3 {
		t.Fatalf("expected 3 stmts in count_up, got %d", len(fnCount.Body.Stmts))
	}

	// Stmt 1: var count = 0 (BIND_DEC)
	if _, ok := fnCount.Body.Stmts[0].(ir.BIND_DEC); !ok {
		t.Errorf("expected BIND_DEC for stmt 0, got %T", fnCount.Body.Stmts[0])
	}

	// Stmt 2: while count < limit
	whileStmt, ok := fnCount.Body.Stmts[1].(ir.WhileStmt)
	if !ok {
		t.Fatalf("expected WhileStmt for stmt 1, got %T", fnCount.Body.Stmts[1])
	}
	if whileStmt.Cond == nil {
		t.Errorf("expected while condition, got nil")
	}
	if len(whileStmt.Body.Stmts) != 2 {
		t.Fatalf("expected 2 stmts in while body, got %d", len(whileStmt.Body.Stmts))
	}

	// Nested IfStmt inside while body
	nestedIf, ok := whileStmt.Body.Stmts[0].(ir.IfStmt)
	if !ok {
		t.Fatalf("expected IfStmt in while body, got %T", whileStmt.Body.Stmts[0])
	}
	if nestedIf.Else != nil {
		t.Errorf("expected nil else for nestedIf, got %+v", nestedIf.Else)
	}
	if len(nestedIf.Then.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in nestedIf, got %d", len(nestedIf.Then.Stmts))
	}
	if _, ok := nestedIf.Then.Stmts[0].(ir.BreakStmt); !ok {
		t.Errorf("expected BreakStmt inside nestedIf, got %T", nestedIf.Then.Stmts[0])
	}

	// 3. var res = loop: while true { break loop: 42 }
	varLoop := rawList[2].([]any)[1].(ir.BIND_DEC)
	pfLoop := varLoop.Value.(ir.PostFixExpr)
	labelLoop := pfLoop.Base.(ir.LabelDec)
	if labelLoop.Name != "loop" {
		t.Errorf("expected label name 'loop', got %q", labelLoop.Name)
	}
	if len(labelLoop.Body.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in labeled while, got %d", len(labelLoop.Body.Stmts))
	}
	labeledWhile, ok := labelLoop.Body.Stmts[0].(ir.WhileStmt)
	if !ok {
		t.Fatalf("expected WhileStmt in labeled body, got %T", labelLoop.Body.Stmts[0])
	}
	if len(labeledWhile.Body.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in labeledWhile body, got %d", len(labeledWhile.Body.Stmts))
	}
	breakStmt := labeledWhile.Body.Stmts[0].(ir.BreakStmt)
	if breakStmt.Label != "loop" || breakStmt.Value == nil {
		t.Errorf("expected break loop: 42, got %+v", breakStmt)
	}
}

func TestDropStmt(t *testing.T) {
	code := `
fn compute() i32 {
	return 42
}

fn main() void {
	drop compute()
	drop 100
}
`
	ast, err := parser.Parse("test_drop.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	rawList := ast.([]any)
	if len(rawList) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(rawList))
	}

	mainFn := rawList[1].([]any)[1].(ir.FN_DEC)
	if len(mainFn.Body.Stmts) != 2 {
		t.Fatalf("expected 2 stmts in mainFn, got %d", len(mainFn.Body.Stmts))
	}

	drop1, ok := mainFn.Body.Stmts[0].(ir.DropStmt)
	if !ok {
		t.Fatalf("expected DropStmt for stmt 0, got %T", mainFn.Body.Stmts[0])
	}
	if drop1.Expr == nil {
		t.Errorf("expected non-nil expr in drop1")
	}

	drop2, ok := mainFn.Body.Stmts[1].(ir.DropStmt)
	if !ok {
		t.Fatalf("expected DropStmt for stmt 1, got %T", mainFn.Body.Stmts[1])
	}
	if drop2.Expr == nil {
		t.Errorf("expected non-nil expr in drop2")
	}
}
