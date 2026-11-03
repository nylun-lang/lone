package loneparser_test

import (
	ir "lone/pkg/lone-parser-ast"
	"lone/pkg/lone-parser"
	"strings"
	"testing"
)

func parseLone(t *testing.T, code string) []ir.BIND_DEC {
	t.Helper()
	res, err := loneparser.Parse("test.lone", []byte(code))
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
	res, err := loneparser.Parse("test.lone", []byte(code))
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
	res, err := loneparser.Parse("test_label.lone", []byte(code))
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
	res, err := loneparser.Parse("test_control_flow.lone", []byte(code))
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

func TestPackedIfStmt(t *testing.T) {
	code := `
fn classify(x: i32) void {
	if {
		x > 100 => #print("Large")
		x > 50 => #print("Medium")
		x > 0 => #print("Small")
		else => {
			#print("Invalid")
		}
	}
}

fn check_optional_else(x: i32) void {
	if {
		x == 1 => #print("One")
		x == 2 => #print("Two")
	}
}

fn check_with_delimiters(x: i32) void {
	if {
		x > 10 => #print("High"),
		x > 5 => #print("Mid");
		else => #print("Low")
	}
}
`
	ast, err := loneparser.Parse("packed_if.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to parse packed if: %v", err)
	}

	rawList, ok := ast.([]any)
	if !ok || len(rawList) != 3 {
		t.Fatalf("expected 3 function declarations, got: %T (%d)", ast, len(rawList))
	}

	// 1. classify(x: i32)
	fnClassify := rawList[0].([]any)[1].(ir.FN_DEC)
	if len(fnClassify.Body.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in classify, got %d", len(fnClassify.Body.Stmts))
	}

	// Root IfStmt: x > 100
	if1, ok := fnClassify.Body.Stmts[0].(ir.IfStmt)
	if !ok {
		t.Fatalf("expected IfStmt, got %T", fnClassify.Body.Stmts[0])
	}
	if len(if1.Then.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in if1 then, got %d", len(if1.Then.Stmts))
	}

	// Second IfStmt in Else: x > 50
	if2, ok := if1.Else.(ir.IfStmt)
	if !ok {
		t.Fatalf("expected chained IfStmt in if1.Else, got %T", if1.Else)
	}
	if len(if2.Then.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in if2 then, got %d", len(if2.Then.Stmts))
	}

	// Third IfStmt in Else: x > 0
	if3, ok := if2.Else.(ir.IfStmt)
	if !ok {
		t.Fatalf("expected chained IfStmt in if2.Else, got %T", if2.Else)
	}
	if len(if3.Then.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in if3 then, got %d", len(if3.Then.Stmts))
	}

	// Final Else: Block { #print("Invalid") }
	elseBlock, ok := if3.Else.(ir.Block)
	if !ok {
		t.Fatalf("expected ir.Block in if3.Else, got %T", if3.Else)
	}
	if len(elseBlock.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in elseBlock, got %d", len(elseBlock.Stmts))
	}

	// 2. check_optional_else(x: i32) - no else branch
	fnOpt := rawList[1].([]any)[1].(ir.FN_DEC)
	optIf1 := fnOpt.Body.Stmts[0].(ir.IfStmt)
	optIf2, ok := optIf1.Else.(ir.IfStmt)
	if !ok {
		t.Fatalf("expected chained IfStmt in optIf1.Else, got %T", optIf1.Else)
	}
	if optIf2.Else != nil {
		t.Errorf("expected nil else for optIf2, got %+v", optIf2.Else)
	}

	// 3. check_with_delimiters(x: i32)
	fnDelim := rawList[2].([]any)[1].(ir.FN_DEC)
	dIf1 := fnDelim.Body.Stmts[0].(ir.IfStmt)
	dIf2, ok := dIf1.Else.(ir.IfStmt)
	if !ok {
		t.Fatalf("expected chained IfStmt in dIf1.Else, got %T", dIf1.Else)
	}
	dElseBlock, ok := dIf2.Else.(ir.Block)
	if !ok {
		t.Fatalf("expected ir.Block in dIf2.Else, got %T", dIf2.Else)
	}
	if len(dElseBlock.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in dElseBlock, got %d", len(dElseBlock.Stmts))
	}
}

func TestLoopStmt(t *testing.T) {
	code := `
fn run_forever() void {
	loop {
		#print("tick\n")
		break
	}
}

fn run_labeled() void {
	outer: loop {
		break outer:
	}
}
`
	ast, err := loneparser.Parse("loop.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to parse loop statement: %v", err)
	}

	rawList, ok := ast.([]any)
	if !ok || len(rawList) != 2 {
		t.Fatalf("expected 2 declarations, got %T (%d)", ast, len(rawList))
	}

	// 1. run_forever()
	fn1 := rawList[0].([]any)[1].(ir.FN_DEC)
	if len(fn1.Body.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in fn1, got %d", len(fn1.Body.Stmts))
	}
	whileStmt, ok := fn1.Body.Stmts[0].(ir.WhileStmt)
	if !ok {
		t.Fatalf("expected WhileStmt for loop, got %T", fn1.Body.Stmts[0])
	}
	condLit, ok := whileStmt.Cond.(ir.Literal)
	if !ok || condLit.Kind != ir.LITERAL_BOOL || condLit.Value != true {
		t.Errorf("expected boolean true literal condition, got: %+v", whileStmt.Cond)
	}
	if len(whileStmt.Body.Stmts) != 2 {
		t.Errorf("expected 2 stmts in loop body, got %d", len(whileStmt.Body.Stmts))
	}

	// 2. run_labeled()
	fn2 := rawList[1].([]any)[1].(ir.FN_DEC)
	labelDec, ok := fn2.Body.Stmts[0].(ir.LabelDec)
	if !ok {
		t.Fatalf("expected LabelDec for outer: loop, got %T", fn2.Body.Stmts[0])
	}
	if labelDec.Name != "outer" {
		t.Errorf("expected label name 'outer', got %q", labelDec.Name)
	}
	labeledLoop, ok := labelDec.Body.Stmts[0].(ir.WhileStmt)
	if !ok {
		t.Fatalf("expected WhileStmt in labeled body, got %T", labelDec.Body.Stmts[0])
	}
	if len(labeledLoop.Body.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in labeledLoop body, got %d", len(labeledLoop.Body.Stmts))
	}
	brk, ok := labeledLoop.Body.Stmts[0].(ir.BreakStmt)
	if !ok || brk.Label != "outer" {
		t.Errorf("expected break outer, got: %+v", brk)
	}
}

func TestForStmtAndImplFor(t *testing.T) {
	code := `
type IterableTrait<T> = trait {
	fn iterator(self) Self
}

impl IterableTrait<i32> for Range {
	fn iterator(self) Self {
		return self
	}
}

fn iterate() void {
	for i in range(1, 10) {
		#print("item\n")
	}

	outer: for x in items {
		break outer
	}
}
`
	ast, err := loneparser.Parse("for.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to parse for statement and impl for: %v", err)
	}

	rawList, ok := ast.([]any)
	if !ok || len(rawList) != 3 {
		t.Fatalf("expected 3 declarations, got %T (%d)", ast, len(rawList))
	}

	// 1. impl IterableTrait<i32> for Range
	implDec := rawList[1].([]any)[1].(ir.IMPL_DEC)
	if implDec.TraitName != "IterableTrait" {
		t.Errorf("expected TraitName 'IterableTrait', got %q", implDec.TraitName)
	}
	if implDec.TargetType.AsBase().Name != "Range" {
		t.Errorf("expected TargetType 'Range', got %q", implDec.TargetType.AsBase().Name)
	}

	// 2. iterate() fn
	fnDec := rawList[2].([]any)[1].(ir.FN_DEC)
	if len(fnDec.Body.Stmts) != 2 {
		t.Fatalf("expected 2 stmts in fnDec, got %d", len(fnDec.Body.Stmts))
	}

	forStmt1, ok := fnDec.Body.Stmts[0].(ir.ForStmt)
	if !ok {
		t.Fatalf("expected ForStmt for stmt 0, got %T", fnDec.Body.Stmts[0])
	}
	if forStmt1.VarName != "i" {
		t.Errorf("expected VarName 'i', got %q", forStmt1.VarName)
	}
	if len(forStmt1.Body.Stmts) != 1 {
		t.Errorf("expected 1 stmt in forStmt1 body, got %d", len(forStmt1.Body.Stmts))
	}

	labelDec, ok := fnDec.Body.Stmts[1].(ir.LabelDec)
	if !ok {
		t.Fatalf("expected LabelDec for outer: for, got %T", fnDec.Body.Stmts[1])
	}
	if labelDec.Name != "outer" {
		t.Errorf("expected label 'outer', got %q", labelDec.Name)
	}
	forStmt2, ok := labelDec.Body.Stmts[0].(ir.ForStmt)
	if !ok {
		t.Fatalf("expected ForStmt inside label, got %T", labelDec.Body.Stmts[0])
	}
	if forStmt2.VarName != "x" {
		t.Errorf("expected VarName 'x', got %q", forStmt2.VarName)
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
	ast, err := loneparser.Parse("test_drop.lone", []byte(code))
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

func TestIntrinsicCall(t *testing.T) {
	code := `
fn main() void {
	#print("Hello, world!\n")
}
`
	ast, err := loneparser.Parse("test_intrinsic.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	rawList := ast.([]any)
	if len(rawList) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(rawList))
	}

	mainFn := rawList[0].([]any)[1].(ir.FN_DEC)
	if len(mainFn.Body.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in mainFn, got %d", len(mainFn.Body.Stmts))
	}

	postExpr, ok := mainFn.Body.Stmts[0].(ir.PostFixExpr)
	if !ok {
		t.Fatalf("expected PostFixExpr, got %T", mainFn.Body.Stmts[0])
	}

	nameRef, ok := postExpr.Base.(ir.NameRef)
	if !ok {
		t.Fatalf("expected NameRef for Base, got %T", postExpr.Base)
	}
	if nameRef.Name != "#print" {
		t.Errorf("expected '#print', got %q", nameRef.Name)
	}
}

func TestImportDeclaration(t *testing.T) {
	code := `
import .readline from :std:io
import .foo, .bar, from .std
import :wasi:cli as cli
`
	ast, err := loneparser.Parse("test_import.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	rawList, ok := ast.([]any)
	if !ok || len(rawList) != 3 {
		t.Fatalf("expected 3 declarations, got %d (%T)", len(rawList), ast)
	}

	// 1. import .readline from :std:io
	imp1, ok := rawList[0].([]any)[1].(ir.ImportDecl)
	if !ok {
		t.Fatalf("expected ImportDecl, got %T", rawList[0].([]any)[1])
	}
	if len(imp1.Symbols) != 1 || imp1.Symbols[0] != "readline" {
		t.Errorf("expected symbols ['readline'], got %v", imp1.Symbols)
	}
	if imp1.Source != ":std:io" || !imp1.IsNamespace {
		t.Errorf("expected namespace :std:io, got source=%q isNamespace=%v", imp1.Source, imp1.IsNamespace)
	}

	// 2. import .foo, .bar, from .std
	imp2, ok := rawList[1].([]any)[1].(ir.ImportDecl)
	if !ok {
		t.Fatalf("expected ImportDecl, got %T", rawList[1].([]any)[1])
	}
	if len(imp2.Symbols) != 2 || imp2.Symbols[0] != "foo" || imp2.Symbols[1] != "bar" {
		t.Errorf("expected symbols ['foo', 'bar'], got %v", imp2.Symbols)
	}
	if imp2.Source != ".std" || imp2.IsNamespace {
		t.Errorf("expected relative .std, got source=%q isNamespace=%v", imp2.Source, imp2.IsNamespace)
	}

	// 3. import :wasi:cli as cli
	imp3, ok := rawList[2].([]any)[1].(ir.ImportDecl)
	if !ok {
		t.Fatalf("expected ImportDecl, got %T", rawList[2].([]any)[1])
	}
	if imp3.Alias != "cli" || imp3.Source != ":wasi:cli" || !imp3.IsNamespace {
		t.Errorf("expected alias cli for :wasi:cli, got alias=%q source=%q isNs=%v", imp3.Alias, imp3.Source, imp3.IsNamespace)
	}
}

func TestRawPointerUnaryExpr(t *testing.T) {
	code := `
fn test() void {
	var a = &raw b
	var c = &d
}
`
	ast, err := loneparser.Parse("test_raw_ptr.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	rawList := ast.([]any)
	fnDec := rawList[0].([]any)[1].(ir.FN_DEC)
	if len(fnDec.Body.Stmts) != 2 {
		t.Fatalf("expected 2 stmts, got %d", len(fnDec.Body.Stmts))
	}

	// var a = &raw b
	stmt1 := fnDec.Body.Stmts[0].(ir.BIND_DEC)
	un1, ok := stmt1.Value.(ir.UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", stmt1.Value)
	}
	if un1.Op != ir.UNARY_OP_RAW_REF {
		t.Errorf("expected op %q, got %q", ir.UNARY_OP_RAW_REF, un1.Op)
	}

	// var c = &d
	stmt2 := fnDec.Body.Stmts[1].(ir.BIND_DEC)
	un2, ok := stmt2.Value.(ir.UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", stmt2.Value)
	}
	if un2.Op != ir.UNARY_OP_REF {
		t.Errorf("expected op %q, got %q", ir.UNARY_OP_REF, un2.Op)
	}
}

func TestWasmDirectiveStructTraitAndIndex(t *testing.T) {
	code := `
-- normal inline comment
---
normal multiline comment
---
---fa:wasm
import .fd_write, .fd_read from :wasi:snapshot_preview1
---

pub type Ciovec = struct {
	buf: ptr<u8>,
	len: i32,

	pub fn make(b: ptr<u8>, l: i32) Ciovec {
		return Ciovec::{ .buf: b, .len: l, }
	}
}

pub type WriterTrait = trait {
	fn write(self, msg: str) i32
}

impl WriterTrait on Ciovec {
	fn write(self, msg: str) i32 {
		return self.len
	}
}

pub fn demo(s: str) i32 {
	let w = WriterTrait::|Ciovec|
	let i = 0
	var b = s[i]
	i = (i + 1) % 4
	return i
}
`
	ast, err := loneparser.Parse("test_full_grammar.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	rawList := ast.([]any)
	if len(rawList) != 5 {
		t.Fatalf("expected 5 top-level decls, got %d", len(rawList))
	}

	wd, ok := rawList[0].([]any)[1].(ir.WasmDirective)
	if !ok || wd.Inline || len(wd.Imports) != 1 {
		t.Fatalf("expected block WasmDirective with 1 import, got %+v", rawList[0].([]any)[1])
	}
	if wd.Imports[0].Source != ":wasi:snapshot_preview1" || len(wd.Imports[0].Symbols) != 2 {
		t.Errorf("unexpected directive import: %+v", wd.Imports[0])
	}

	stDec, ok := rawList[1].([]any)[1].(ir.TYPE_DEC)
	if !ok || stDec.Kind != ir.TYPE_DEC_STRUCT || !stDec.IsPublic || stDec.Name != "Ciovec" {
		t.Fatalf("expected pub struct Ciovec, got %+v", rawList[1].([]any)[1])
	}
	if len(stDec.Fields) != 2 || len(stDec.Methods) != 1 {
		t.Errorf("expected 2 fields and 1 method, got fields=%d methods=%d", len(stDec.Fields), len(stDec.Methods))
	}

	trDec, ok := rawList[2].([]any)[1].(ir.TYPE_DEC)
	if !ok || trDec.Kind != ir.TYPE_DEC_TRAIT || trDec.Name != "WriterTrait" || len(trDec.TraitSigs) != 1 {
		t.Fatalf("expected trait WriterTrait with 1 sig, got %+v", rawList[2].([]any)[1])
	}
	if len(trDec.TraitSigs[0].Params) != 2 || trDec.TraitSigs[0].Params[0].Name != "self" {
		t.Errorf("expected self param on trait sig, got %+v", trDec.TraitSigs[0].Params)
	}

	implDec, ok := rawList[3].([]any)[1].(ir.IMPL_DEC)
	if !ok || implDec.TraitName != "WriterTrait" || len(implDec.Methods) != 1 {
		t.Fatalf("expected impl WriterTrait on Ciovec, got %+v", rawList[3].([]any)[1])
	}

	fnDec, ok := rawList[4].([]any)[1].(ir.FN_DEC)
	if !ok || !fnDec.IsPublic || len(fnDec.Body.Stmts) != 5 {
		t.Fatalf("expected pub fn demo with 5 stmts, got %+v", rawList[4].([]any)[1])
	}

	// let w = Writer::|Ciovec|
	wBind := fnDec.Body.Stmts[0].(ir.BIND_DEC)
	wPf := wBind.Value.(ir.PostFixExpr)
	if len(wPf.Suffix) != 1 || wPf.Suffix[0].IsSuffix() != ir.SUFFIX_KIND_TRAIT_MAT {
		t.Errorf("expected SUFFIX_KIND_TRAIT_MAT, got %+v", wPf.Suffix)
	}

	// var b = s[i]
	bBind := fnDec.Body.Stmts[2].(ir.BIND_DEC)
	bPf := bBind.Value.(ir.PostFixExpr)
	if len(bPf.Suffix) != 1 || bPf.Suffix[0].IsSuffix() != ir.SUFFIX_KIND_INDEX {
		t.Errorf("expected SUFFIX_KIND_INDEX, got %+v", bPf.Suffix)
	}

	// i = (i + 1) % 4
	if _, ok := fnDec.Body.Stmts[3].(ir.AssignStmt); !ok {
		t.Errorf("expected AssignStmt, got %T", fnDec.Body.Stmts[3])
	}
}

func TestFaCompilerDirectives(t *testing.T) {
	code := `
@fa:wasm {
	import .fd_write, .fd_read from :wasi:snapshot_preview1
	import .random_get from :wasi:snapshot_preview1
}

@fa:wasm import .proc_exit from :wasi:snapshot_preview1

@fa:wasm
`
	ast, err := loneparser.Parse("test_directives.lone", []byte(code))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	rawList := ast.([]any)
	if len(rawList) != 3 {
		t.Fatalf("expected 3 directives, got %d", len(rawList))
	}

	// 1. Block directive
	d1, ok := rawList[0].([]any)[1].(ir.WasmDirective)
	if !ok || d1.Name != "wasm" || d1.Inline || len(d1.Imports) != 2 {
		t.Fatalf("expected block @fa:wasm with 2 imports, got %+v", rawList[0].([]any)[1])
	}
	if len(d1.Imports[0].Symbols) != 2 || d1.Imports[0].Source != ":wasi:snapshot_preview1" {
		t.Errorf("unexpected first import: %+v", d1.Imports[0])
	}

	// 2. Statement directive
	d2, ok := rawList[1].([]any)[1].(ir.WasmDirective)
	if !ok || d2.Name != "wasm" || d2.Inline || len(d2.Imports) != 1 {
		t.Fatalf("expected stmt @fa:wasm with 1 import, got %+v", rawList[1].([]any)[1])
	}
	if d2.Imports[0].Symbols[0] != "proc_exit" {
		t.Errorf("unexpected import symbol: %v", d2.Imports[0].Symbols)
	}

	// 3. Bare inline directive
	d3, ok := rawList[2].([]any)[1].(ir.WasmDirective)
	if !ok || d3.Name != "wasm" || !d3.Inline {
		t.Fatalf("expected bare @fa:wasm inline directive, got %+v", rawList[2].([]any)[1])
	}
}

func TestXorOperatorParser(t *testing.T) {
	code := `
fn test_xor(a: bool, b: bool, c: bool, d: bool) bool {
	return a and b xor c or d
}

fn test_caret(x: i32, y: i32) i32 {
	return x ^ y
}
`
	ast, err := loneparser.Parse("xor.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to parse xor operator: %v", err)
	}

	rawList, ok := ast.([]any)
	if !ok || len(rawList) != 2 {
		t.Fatalf("expected 2 declarations, got %T (%d)", ast, len(rawList))
	}

	fn1 := rawList[0].([]any)[1].(ir.FN_DEC)
	ret1 := fn1.Body.Stmts[0].(ir.ReturnStmt)
	// Outer is "or": (a and b xor c) or d
	binOr, ok := ret1.Value.(ir.BinaryExpr)
	if !ok || binOr.Op != "or" {
		t.Fatalf("expected outer binary 'or', got %+v", ret1.Value)
	}
	// Left of "or" is "xor": (a and b) xor c
	binXor, ok := binOr.Left.(ir.BinaryExpr)
	if !ok || binXor.Op != "xor" {
		t.Fatalf("expected left of 'or' to be 'xor', got %+v", binOr.Left)
	}
	// Left of "xor" is "and": a and b
	binAnd, ok := binXor.Left.(ir.BinaryExpr)
	if !ok || binAnd.Op != "and" {
		t.Fatalf("expected left of 'xor' to be 'and', got %+v", binXor.Left)
	}

	fn2 := rawList[1].([]any)[1].(ir.FN_DEC)
	ret2 := fn2.Body.Stmts[0].(ir.ReturnStmt)
	binCaret, ok := ret2.Value.(ir.BinaryExpr)
	if !ok || binCaret.Op != "^" {
		t.Fatalf("expected binary '^', got %+v", ret2.Value)
	}
}

func TestIsOperatorParser(t *testing.T) {
	code := `
fn check(x: ?i32) bool {
	if x is i32 {
		return true
	}
	if x is not none {
		return true
	}
	if !(x is none) {
		return true
	}
	return x is none
}
`
	ast, err := loneparser.Parse("is.lone", []byte(code))
	if err != nil {
		t.Fatalf("failed to parse is operator: %v", err)
	}

	rawList, ok := ast.([]any)
	if !ok || len(rawList) != 1 {
		t.Fatalf("expected 1 declaration, got %T (%d)", ast, len(rawList))
	}

	fn := rawList[0].([]any)[1].(ir.FN_DEC)
	if len(fn.Body.Stmts) != 4 {
		t.Fatalf("expected 4 stmts, got %d", len(fn.Body.Stmts))
	}

	// 1. if x is i32
	if1 := fn.Body.Stmts[0].(ir.IfStmt)
	is1, ok := if1.Cond.(ir.IsExpr)
	if !ok || is1.Negated || is1.TargetType.AsBase().Name != "i32" {
		t.Errorf("expected is i32, got %+v", if1.Cond)
	}

	// 2. if x is not none
	if2 := fn.Body.Stmts[1].(ir.IfStmt)
	is2, ok := if2.Cond.(ir.IsExpr)
	if !ok || !is2.Negated || is2.TargetType.AsBase().Name != "none" {
		t.Errorf("expected is not none, got %+v", if2.Cond)
	}

	// 3. if !(x is none)
	if3 := fn.Body.Stmts[2].(ir.IfStmt)
	un3, ok := if3.Cond.(ir.UnaryExpr)
	if !ok || un3.Op != "!" {
		t.Fatalf("expected UnaryExpr !, got %+v", if3.Cond)
	}
	var innerExpr any = un3.Expr
	if pf, ok := innerExpr.(ir.PostFixExpr); ok && len(pf.Suffix) == 0 {
		innerExpr = pf.Base
	}
	is3, ok := innerExpr.(ir.IsExpr)
	if !ok || is3.Negated || is3.TargetType.AsBase().Name != "none" {
		t.Errorf("expected inner is none, got %+v", innerExpr)
	}

	// 4. return x is none
	ret4 := fn.Body.Stmts[3].(ir.ReturnStmt)
	is4, ok := ret4.Value.(ir.IsExpr)
	if !ok || is4.Negated || is4.TargetType.AsBase().Name != "none" {
		t.Errorf("expected return x is none, got %+v", ret4.Value)
	}
}

func TestComparisonChainingRejected(t *testing.T) {
	chainedCases := []string{
		"fn test() void { var x = a < b < c }",
		"fn test() void { if a < b < c {} }",
		"fn test() void { if a <= b <= c {} }",
		"fn test() void { if a > b > c {} }",
		"fn test() void { if a >= b >= c {} }",
		"fn test() void { if a < b > c {} }",
		"fn test() void { if a > b <= c >= d {} }",
	}

	for _, code := range chainedCases {
		_, err := loneparser.Parse("chained.lone", []byte(code))
		if err == nil {
			t.Fatalf("expected compile-time parse error for chained comparison %q, but got nil", code)
		}
		if !strings.Contains(err.Error(), "comparison operators cannot be chained") {
			t.Fatalf("expected error message to contain 'comparison operators cannot be chained' for %q, got: %v", code, err)
		}
	}

	// Valid single comparisons and compound logical expressions must pass
	validCases := []string{
		"fn test() void { if a < b and b < c {} }",
		"fn test() void { if (a < b) < c {} }",
		"fn test() void { if a < b {} }",
		"fn test() void { if a <= b {} }",
		"fn test() void { if a > b {} }",
		"fn test() void { if a >= b {} }",
	}
	for _, code := range validCases {
		_, err := loneparser.Parse("valid.lone", []byte(code))
		if err != nil {
			t.Fatalf("expected valid parse for %q, got error: %v", code, err)
		}
	}
}
