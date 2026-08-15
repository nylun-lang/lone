package parser

import (
	"testing"
)

const txt = `module main
import std.io
import std.ffi

// test

const zz = 2

type User = record { 
	name string
	email opt<string>
}

let str string = "hello lone"
`

func TestParserLetDec(t *testing.T) {
	v, err := Parse("simple.lone", []byte(txt))

	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	t.Logf("ParseResult: %v", v)
}
