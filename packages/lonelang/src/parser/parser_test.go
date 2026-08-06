package parser

import (
	"testing"
)

const txt = `module main
import std.io
import std.ffi
// test
const zz int32
var x int64 = 10
var y float32 = 10
var z int64 = 10_230_39
var m ro<ptr<opt<bool>>>
var test infer = E{ @passed = true, @todoes = "many" }

`

func TestParser(t *testing.T) {
	v, err := Parse("simple.lone", []byte(txt))

	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	t.Logf("ParseResult: %v", v)
}
