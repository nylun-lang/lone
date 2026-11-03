package lonelexer

import (
	"testing"
)

func TestLexerBasicTokens(t *testing.T) {
	code := `
let x: i32 = 42
var count = 3.14
#print("hello world\n")
`
	lex := NewLexer([]byte(code))
	tokens, err := lex.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []TokenKind{
		TokenLet, TokenIdent, TokenColon, TokenIdent, TokenAssign, TokenInt,
		TokenVar, TokenIdent, TokenAssign, TokenFloat,
		TokenIntrinsic, TokenLParen, TokenString, TokenRParen,
		TokenEOF,
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, exp := range expected {
		if tokens[i].Kind != exp {
			t.Errorf("token %d: expected %s, got %s (%q)", i, exp, tokens[i].Kind, tokens[i].Text)
		}
	}
}

func TestLexerScopedHeaderAndListLiteral(t *testing.T) {
	code := `
if: [let x = 0] x != 9 {
    break
}
var items: []i32 = [1, 2, 3]
var empty: []str = []
`
	lex := NewLexer([]byte(code))
	tokens, err := lex.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []TokenKind{
		TokenIf, TokenColon, TokenLBracket, TokenLet, TokenIdent, TokenAssign, TokenInt, TokenRBracket,
		TokenIdent, TokenNotEq, TokenInt, TokenLBrace, TokenBreak, TokenRBrace,
		TokenVar, TokenIdent, TokenColon, TokenLBracket, TokenRBracket, TokenIdent, TokenAssign,
		TokenLBracket, TokenInt, TokenComma, TokenInt, TokenComma, TokenInt, TokenRBracket,
		TokenVar, TokenIdent, TokenColon, TokenLBracket, TokenRBracket, TokenIdent, TokenAssign,
		TokenLBracket, TokenRBracket,
		TokenEOF,
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, exp := range expected {
		if tokens[i].Kind != exp {
			t.Errorf("token %d: expected %s, got %s (%q)", i, exp, tokens[i].Kind, tokens[i].Text)
		}
	}
}

func TestLexerComments(t *testing.T) {
	code := `
-- single line comment
let a = 1 -- inline comment
---
multi line
comment
---
let b = 2
`
	lex := NewLexer([]byte(code))
	tokens, err := lex.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []TokenKind{
		TokenLet, TokenIdent, TokenAssign, TokenInt,
		TokenLet, TokenIdent, TokenAssign, TokenInt,
		TokenEOF,
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, exp := range expected {
		if tokens[i].Kind != exp {
			t.Errorf("token %d: expected %s, got %s (%q)", i, exp, tokens[i].Kind, tokens[i].Text)
		}
	}
}

func TestLexerAmpRawAndOperators(t *testing.T) {
	code := `
let p = &raw val
let r = &val
let deref = *p
var ok = a is int and b is not none
`
	lex := NewLexer([]byte(code))
	tokens, err := lex.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []TokenKind{
		TokenLet, TokenIdent, TokenAssign, TokenAmpRaw, TokenIdent,
		TokenLet, TokenIdent, TokenAssign, TokenAmpersand, TokenIdent,
		TokenLet, TokenIdent, TokenAssign, TokenStar, TokenIdent,
		TokenVar, TokenIdent, TokenAssign, TokenIdent, TokenIs, TokenIdent, TokenAnd, TokenIdent, TokenIs, TokenNot, TokenNone,
		TokenEOF,
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, exp := range expected {
		if tokens[i].Kind != exp {
			t.Errorf("token %d: expected %s, got %s (%q)", i, exp, tokens[i].Kind, tokens[i].Text)
		}
	}
}

func TestLexerEnumAndMatch(t *testing.T) {
	code := `
type Shape = enum {
	Circle(radius: f32),
	Rect(width: f32, height: f32),
	Point,
}

let a = match s {
	Shape::Circle(r) => 3.14,
	Shape::Point => 0.0,
	_ => 0.0,
}
`
	lex := NewLexer([]byte(code))
	tokens, err := lex.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundEnum := false
	foundMatch := false
	for _, tok := range tokens {
		if tok.Kind == TokenEnum {
			foundEnum = true
		}
		if tok.Kind == TokenMatch {
			foundMatch = true
		}
	}

	if !foundEnum {
		t.Errorf("expected to find TokenEnum in tokens")
	}
	if !foundMatch {
		t.Errorf("expected to find TokenMatch in tokens")
	}
}

func TestLexerMustAndTry(t *testing.T) {
	code := `
let x = must compute()
let y = try compute()
`
	lex := NewLexer([]byte(code))
	tokens, err := lex.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundMust := false
	foundTry := false
	for _, tok := range tokens {
		if tok.Kind == TokenMust {
			foundMust = true
		}
		if tok.Kind == TokenTry {
			foundTry = true
		}
	}
	if !foundMust {
		t.Errorf("expected TokenMust")
	}
	if !foundTry {
		t.Errorf("expected TokenTry")
	}
}
