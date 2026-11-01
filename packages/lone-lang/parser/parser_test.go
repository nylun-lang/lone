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
	litSize, ok := list0.Size.(ir.Literal)
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
	nameRef, ok := list1.Size.(ir.NameRef)
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
	innerLit, ok := innerList2.Size.(ir.Literal)
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
