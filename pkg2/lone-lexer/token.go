package lonelexer

import "fmt"

// Position represents a source code line and column position.
type Position struct {
	Offset int // 0-based byte offset
	Line   int // 1-based line number
	Col    int // 1-based column number
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

// Span represents a region between two positions in source code.
type Span struct {
	Start Position
	End   Position
}

// TokenKind identifies the lexical category of a Token.
type TokenKind int

const (
	TokenEOF TokenKind = iota
	TokenIllegal

	// Identifiers & Intrinsics
	TokenIdent     // e.g. foo, MyType, TrWriter
	TokenIntrinsic // e.g. #print, #init, #panic

	// Literals
	TokenInt    // e.g. 42, 0xFF, 1_000
	TokenFloat  // e.g. 3.14, 0.5
	TokenString // e.g. "hello\n"
	TokenChar   // e.g. 'a'

	// Keywords
	TokenImport
	TokenFrom
	TokenAs
	TokenPub
	TokenVar
	TokenLet
	TokenType
	TokenStruct
	TokenEnum
	TokenTrait
	TokenImpl
	TokenOn
	TokenWhere
	TokenFn
	TokenReturn
	TokenBreak
	TokenIf
	TokenElse
	TokenWhile
	TokenLoop
	TokenFor
	TokenIn
	TokenDrop
	TokenMatch
	TokenAnd
	TokenOr
	TokenXor
	TokenIs
	TokenNot
	TokenTrue
	TokenFalse
	TokenNone
	TokenRo
	TokenComptime
	TokenSelf

	// Directives
	TokenFaDirective // e.g. @fa:wasm or @fa:doc

	// Operators & Punctuation
	TokenPlus     // +
	TokenMinus    // -
	TokenStar     // *
	TokenSlash    // /
	TokenPercent  // %
	TokenCaret    // ^
	TokenAmpersand// &
	TokenAmpRaw   // &raw
	TokenPipe     // |
	TokenExcl     // !

	TokenEq       // ==
	TokenNotEq    // !=
	TokenLt       // <
	TokenLtEq     // <=
	TokenGt       // >
	TokenGtEq     // >=

	TokenAssign   // =
	TokenFatArrow // =>

	TokenDot      // .
	TokenColon    // :
	TokenQuestion // ?
	TokenColon2   // ::
	TokenColon2Pipe // ::|
	TokenComma    // ,
	TokenSemicolon// ;

	TokenLParen   // (
	TokenRParen   // )
	TokenLBracket // [
	TokenRBracket // ]
	TokenLBrace   // {
	TokenRBrace   // }
)

var tokenNames = map[TokenKind]string{
	TokenEOF:        "EOF",
	TokenIllegal:    "ILLEGAL",
	TokenIdent:      "IDENT",
	TokenIntrinsic:  "INTRINSIC",
	TokenInt:        "INT",
	TokenFloat:      "FLOAT",
	TokenString:     "STRING",
	TokenChar:       "CHAR",
	TokenImport:     "import",
	TokenFrom:       "from",
	TokenAs:         "as",
	TokenPub:        "pub",
	TokenVar:        "var",
	TokenLet:        "let",
	TokenType:       "type",
	TokenStruct:     "struct",
	TokenEnum:       "enum",
	TokenTrait:      "trait",
	TokenImpl:       "impl",
	TokenOn:         "on",
	TokenWhere:      "where",
	TokenFn:         "fn",
	TokenReturn:     "return",
	TokenBreak:      "break",
	TokenIf:         "if",
	TokenElse:       "else",
	TokenWhile:      "while",
	TokenLoop:       "loop",
	TokenFor:        "for",
	TokenIn:         "in",
	TokenDrop:       "drop",
	TokenMatch:      "match",
	TokenAnd:        "and",
	TokenOr:         "or",
	TokenXor:        "xor",
	TokenIs:         "is",
	TokenNot:        "not",
	TokenTrue:       "true",
	TokenFalse:      "false",
	TokenNone:       "none",
	TokenRo:         "ro",
	TokenComptime:   "comptime",
	TokenSelf:       "self",
	TokenFaDirective:"@fa:",
	TokenPlus:       "+",
	TokenMinus:      "-",
	TokenStar:       "*",
	TokenSlash:      "/",
	TokenPercent:    "%",
	TokenCaret:      "^",
	TokenAmpersand:  "&",
	TokenAmpRaw:     "&raw",
	TokenPipe:       "|",
	TokenExcl:       "!",
	TokenEq:         "==",
	TokenNotEq:      "!=",
	TokenLt:         "<",
	TokenLtEq:       "<=",
	TokenGt:         ">",
	TokenGtEq:       ">=",
	TokenAssign:     "=",
	TokenFatArrow:   "=>",
	TokenDot:        ".",
	TokenColon:      ":",
	TokenQuestion:   "?",
	TokenColon2:     "::",
	TokenColon2Pipe: "::|",
	TokenComma:      ",",
	TokenSemicolon:  ";",
	TokenLParen:     "(",
	TokenRParen:     ")",
	TokenLBracket:   "[",
	TokenRBracket:   "]",
	TokenLBrace:     "{",
	TokenRBrace:     "}",
}

func (k TokenKind) String() string {
	if name, ok := tokenNames[k]; ok {
		return name
	}
	return fmt.Sprintf("TokenKind(%d)", k)
}

// Token represents a lexical token in Lone source code.
type Token struct {
	Kind  TokenKind
	Text  string
	Start Position
	End   Position
}

func (t Token) Span() Span {
	return Span{Start: t.Start, End: t.End}
}

func (t Token) String() string {
	return fmt.Sprintf("%s(%q) at %s", t.Kind, t.Text, t.Start)
}

// Keywords map reserved words to their token kinds.
var Keywords = map[string]TokenKind{
	"import":   TokenImport,
	"from":     TokenFrom,
	"as":       TokenAs,
	"pub":      TokenPub,
	"var":      TokenVar,
	"let":      TokenLet,
	"type":     TokenType,
	"struct":   TokenStruct,
	"enum":     TokenEnum,
	"trait":    TokenTrait,
	"impl":     TokenImpl,
	"on":       TokenOn,
	"where":    TokenWhere,
	"fn":       TokenFn,
	"return":   TokenReturn,
	"break":    TokenBreak,
	"if":       TokenIf,
	"else":     TokenElse,
	"while":    TokenWhile,
	"loop":     TokenLoop,
	"for":      TokenFor,
	"in":       TokenIn,
	"drop":     TokenDrop,
	"match":    TokenMatch,
	"and":      TokenAnd,
	"or":       TokenOr,
	"xor":      TokenXor,
	"is":       TokenIs,
	"not":      TokenNot,
	"true":     TokenTrue,
	"false":    TokenFalse,
	"none":     TokenNone,
	"ro":       TokenRo,
	"comptime": TokenComptime,
	"self":     TokenSelf,
}
