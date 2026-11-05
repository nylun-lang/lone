package lonelexer

import (
	"fmt"
	"strings"
	"unicode"
)

// Lexer scans source code bytes into tokens.
type Lexer struct {
	src    []byte
	offset int
	line   int
	col    int
}

// NewLexer creates an initialized Lexer.
func NewLexer(src []byte) *Lexer {
	return &Lexer{
		src:    src,
		offset: 0,
		line:   1,
		col:    1,
	}
}

// Tokenize processes the entire source and returns all tokens ending with TokenEOF.
func (l *Lexer) Tokenize() ([]Token, error) {
	var tokens []Token
	for {
		tok, err := l.NextToken()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
		if tok.Kind == TokenEOF {
			break
		}
	}
	return tokens, nil
}

// NextToken scans and returns the next lexical token.
func (l *Lexer) NextToken() (Token, error) {
	l.skipWhitespaceAndComments()

	start := l.pos()
	if l.isEOF() {
		return Token{Kind: TokenEOF, Text: "", Start: start, End: start}, nil
	}

	ch := l.peek()

	// 1. Directives: @fa:name
	if ch == '@' {
		return l.scanAtDirective()
	}

	// 2. Intrinsics: #name
	if ch == '#' {
		return l.scanIntrinsic()
	}

	// 3. String literal or formatted string (f"...")
	if ch == '"' {
		return l.scanString()
	}
	if ch == 'f' && l.peekN(1) == '"' {
		return l.scanFString()
	}

	// 4. Character literal
	if ch == '\'' {
		return l.scanChar()
	}

	// 5. Number literal: integer or float
	if unicode.IsDigit(rune(ch)) {
		return l.scanNumber()
	}

	// 6. Identifier or Keyword
	if isIdentStart(rune(ch)) {
		return l.scanIdentOrKeyword()
	}

	// 7. Operators and Punctuation
	l.advance()
	end := l.pos()

	switch ch {
	case '+':
		return Token{Kind: TokenPlus, Text: "+", Start: start, End: end}, nil
	case '-':
		return Token{Kind: TokenMinus, Text: "-", Start: start, End: end}, nil
	case '*':
		return Token{Kind: TokenStar, Text: "*", Start: start, End: end}, nil
	case '/':
		return Token{Kind: TokenSlash, Text: "/", Start: start, End: end}, nil
	case '%':
		return Token{Kind: TokenPercent, Text: "%", Start: start, End: end}, nil
	case '^':
		return Token{Kind: TokenCaret, Text: "^", Start: start, End: end}, nil
	case '|':
		return Token{Kind: TokenPipe, Text: "|", Start: start, End: end}, nil
	case ',':
		return Token{Kind: TokenComma, Text: ",", Start: start, End: end}, nil
	case ';':
		return Token{Kind: TokenSemicolon, Text: ";", Start: start, End: end}, nil
	case '(':
		return Token{Kind: TokenLParen, Text: "(", Start: start, End: end}, nil
	case ')':
		return Token{Kind: TokenRParen, Text: ")", Start: start, End: end}, nil
	case '[':
		return Token{Kind: TokenLBracket, Text: "[", Start: start, End: end}, nil
	case ']':
		return Token{Kind: TokenRBracket, Text: "]", Start: start, End: end}, nil
	case '{':
		return Token{Kind: TokenLBrace, Text: "{", Start: start, End: end}, nil
	case '}':
		return Token{Kind: TokenRBrace, Text: "}", Start: start, End: end}, nil
	case '.':
		return Token{Kind: TokenDot, Text: ".", Start: start, End: end}, nil
	case '?':
		return Token{Kind: TokenQuestion, Text: "?", Start: start, End: end}, nil

	case '&':
		// Check for '&raw'
		if l.matchesString("raw") && !isIdentPart(rune(l.peekN(3))) {
			l.advanceN(3)
			return Token{Kind: TokenAmpRaw, Text: "&raw", Start: start, End: l.pos()}, nil
		}
		return Token{Kind: TokenAmpersand, Text: "&", Start: start, End: end}, nil

	case '!':
		if l.peek() == '=' {
			l.advance()
			return Token{Kind: TokenNotEq, Text: "!=", Start: start, End: l.pos()}, nil
		}
		return Token{Kind: TokenExcl, Text: "!", Start: start, End: end}, nil

	case '=':
		if l.peek() == '=' {
			l.advance()
			return Token{Kind: TokenEq, Text: "==", Start: start, End: l.pos()}, nil
		}
		if l.peek() == '>' {
			l.advance()
			return Token{Kind: TokenFatArrow, Text: "=>", Start: start, End: l.pos()}, nil
		}
		return Token{Kind: TokenAssign, Text: "=", Start: start, End: end}, nil

	case '<':
		if l.peek() == '=' {
			l.advance()
			return Token{Kind: TokenLtEq, Text: "<=", Start: start, End: l.pos()}, nil
		}
		return Token{Kind: TokenLt, Text: "<", Start: start, End: end}, nil

	case '>':
		if l.peek() == '=' {
			l.advance()
			return Token{Kind: TokenGtEq, Text: ">=", Start: start, End: l.pos()}, nil
		}
		return Token{Kind: TokenGt, Text: ">", Start: start, End: end}, nil

	case ':':
		if l.peek() == ':' {
			l.advance()
			if l.peek() == '|' {
				l.advance()
				return Token{Kind: TokenColon2Pipe, Text: "::|", Start: start, End: l.pos()}, nil
			}
			return Token{Kind: TokenColon2, Text: "::", Start: start, End: l.pos()}, nil
		}
		return Token{Kind: TokenColon, Text: ":", Start: start, End: end}, nil

	default:
		return Token{Kind: TokenIllegal, Text: string(ch), Start: start, End: end},
			fmt.Errorf("unexpected character %q at line %d:%d", ch, start.Line, start.Col)
	}
}

func (l *Lexer) scanAtDirective() (Token, error) {
	start := l.pos()
	l.advance() // skip '@'

	// scan "fa:name"
	startOffset := l.offset
	for !l.isEOF() && (isIdentPart(rune(l.peek())) || l.peek() == ':') {
		l.advance()
	}
	text := "@" + string(l.src[startOffset:l.offset])
	return Token{Kind: TokenFaDirective, Text: text, Start: start, End: l.pos()}, nil
}

func (l *Lexer) scanIntrinsic() (Token, error) {
	start := l.pos()
	l.advance() // skip '#'

	startOffset := l.offset
	for !l.isEOF() && isIdentPart(rune(l.peek())) {
		l.advance()
	}
	text := "#" + string(l.src[startOffset:l.offset])
	return Token{Kind: TokenIntrinsic, Text: text, Start: start, End: l.pos()}, nil
}

func (l *Lexer) scanIdentOrKeyword() (Token, error) {
	start := l.pos()
	startOffset := l.offset

	for !l.isEOF() && isIdentPart(rune(l.peek())) {
		l.advance()
	}

	text := string(l.src[startOffset:l.offset])
	end := l.pos()

	if kw, isKw := Keywords[text]; isKw {
		return Token{Kind: kw, Text: text, Start: start, End: end}, nil
	}

	return Token{Kind: TokenIdent, Text: text, Start: start, End: end}, nil
}

func (l *Lexer) scanNumber() (Token, error) {
	start := l.pos()
	startOffset := l.offset

	// Check hex (0x...), binary (0b...), octal (0o...)
	if l.peek() == '0' && l.offset+1 < len(l.src) {
		next := l.src[l.offset+1]
		if next == 'x' || next == 'X' {
			l.advanceN(2)
			for !l.isEOF() && (isHexDigit(l.peek()) || l.peek() == '_') {
				l.advance()
			}
			return Token{Kind: TokenInt, Text: string(l.src[startOffset:l.offset]), Start: start, End: l.pos()}, nil
		}
		if next == 'b' || next == 'B' {
			l.advanceN(2)
			for !l.isEOF() && (l.peek() == '0' || l.peek() == '1' || l.peek() == '_') {
				l.advance()
			}
			return Token{Kind: TokenInt, Text: string(l.src[startOffset:l.offset]), Start: start, End: l.pos()}, nil
		}
	}

	// Standard decimal or float
	isFloat := false
	for !l.isEOF() {
		ch := l.peek()
		if unicode.IsDigit(rune(ch)) || ch == '_' {
			l.advance()
		} else if ch == '.' && !isFloat && l.offset+1 < len(l.src) && unicode.IsDigit(rune(l.src[l.offset+1])) {
			isFloat = true
			l.advance() // consume '.'
		} else {
			break
		}
	}

	kind := TokenInt
	if isFloat {
		kind = TokenFloat
	}
	return Token{Kind: kind, Text: string(l.src[startOffset:l.offset]), Start: start, End: l.pos()}, nil
}

func (l *Lexer) scanString() (Token, error) {
	start := l.pos()
	l.advance() // skip opening '"'

	var sb strings.Builder
	for !l.isEOF() {
		ch := l.peek()
		if ch == '"' {
			l.advance() // skip closing '"'
			return Token{Kind: TokenString, Text: sb.String(), Start: start, End: l.pos()}, nil
		}
		if ch == '\n' || ch == '\r' {
			return Token{Kind: TokenIllegal, Text: "", Start: start, End: l.pos()},
				fmt.Errorf("unterminated string literal at line %d:%d", start.Line, start.Col)
		}
		if ch == '\\' {
			l.advance()
			if l.isEOF() {
				break
			}
			esc := l.peek()
			l.advance()
			switch esc {
			case 'n':
				sb.WriteByte('\n')
			case 'r':
				sb.WriteByte('\r')
			case 't':
				sb.WriteByte('\t')
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			case '0':
				sb.WriteByte(0)
			default:
				sb.WriteByte(esc)
			}
			continue
		}

		sb.WriteByte(ch)
		l.advance()
	}

	return Token{Kind: TokenIllegal, Text: "", Start: start, End: l.pos()},
		fmt.Errorf("unterminated string literal at line %d:%d", start.Line, start.Col)
}

func (l *Lexer) scanFString() (Token, error) {
	start := l.pos()
	l.advance() // skip 'f'
	l.advance() // skip '"'

	var sb strings.Builder
	for !l.isEOF() {
		ch := l.peek()
		if ch == '"' {
			l.advance() // skip closing '"'
			return Token{Kind: TokenFString, Text: sb.String(), Start: start, End: l.pos()}, nil
		}
		if ch == '\n' || ch == '\r' {
			return Token{Kind: TokenIllegal, Text: "", Start: start, End: l.pos()},
				fmt.Errorf("unterminated format string literal at line %d:%d", start.Line, start.Col)
		}
		if ch == '\\' {
			l.advance()
			if l.isEOF() {
				break
			}
			esc := l.peek()
			l.advance()
			switch esc {
			case 'n':
				sb.WriteByte('\n')
			case 'r':
				sb.WriteByte('\r')
			case 't':
				sb.WriteByte('\t')
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			case '$':
				sb.WriteString("\\${")
				if !l.isEOF() && l.peek() == '{' {
					l.advance()
				}
			case '0':
				sb.WriteByte(0)
			default:
				sb.WriteByte(esc)
			}
			continue
		}

		sb.WriteByte(ch)
		l.advance()
	}

	return Token{Kind: TokenIllegal, Text: "", Start: start, End: l.pos()},
		fmt.Errorf("unterminated format string literal at line %d:%d", start.Line, start.Col)
}

func (l *Lexer) scanChar() (Token, error) {
	start := l.pos()
	l.advance() // skip opening '\''

	if l.isEOF() || l.peek() == '\'' {
		return Token{Kind: TokenIllegal, Text: "", Start: start, End: l.pos()},
			fmt.Errorf("empty character literal at line %d:%d", start.Line, start.Col)
	}

	var ch byte
	if l.peek() == '\\' {
		l.advance()
		esc := l.peek()
		l.advance()
		switch esc {
		case 'n':
			ch = '\n'
		case 'r':
			ch = '\r'
		case 't':
			ch = '\t'
		case '\\':
			ch = '\\'
		case '\'':
			ch = '\''
		case '0':
			ch = 0
		default:
			ch = esc
		}
	} else {
		ch = l.peek()
		l.advance()
	}

	if l.peek() != '\'' {
		return Token{Kind: TokenIllegal, Text: "", Start: start, End: l.pos()},
			fmt.Errorf("unclosed character literal at line %d:%d", start.Line, start.Col)
	}
	l.advance() // skip closing '\''

	return Token{Kind: TokenChar, Text: string(ch), Start: start, End: l.pos()}, nil
}

func (l *Lexer) skipWhitespaceAndComments() {
	for !l.isEOF() {
		ch := l.peek()

		// Whitespace
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			l.advance()
			continue
		}

		// Comment: starts with '--'
		if ch == '-' && l.offset+1 < len(l.src) && l.src[l.offset+1] == '-' {
			// Check multi-line: '---'
			if l.offset+2 < len(l.src) && l.src[l.offset+2] == '-' {
				// Check if it's '---fa:wasm'
				if l.matchesString("---fa:wasm") {
					return // Directive, do not skip
				}

				// Multi-line comment: skip until '---'
				l.advanceN(3)
				for !l.isEOF() {
					if l.matchesString("---") {
						l.advanceN(3)
						break
					}
					l.advance()
				}
				continue
			}

			// Check if single-line comment is '--fa:wasm'
			if l.matchesString("--fa:wasm") {
				return // Directive, do not skip
			}

			// Single line comment: skip until newline
			l.advanceN(2)
			for !l.isEOF() && l.peek() != '\n' {
				l.advance()
			}
			continue
		}

		break
	}
}

func (l *Lexer) pos() Position {
	return Position{Offset: l.offset, Line: l.line, Col: l.col}
}

func (l *Lexer) isEOF() bool {
	return l.offset >= len(l.src)
}

func (l *Lexer) peek() byte {
	if l.isEOF() {
		return 0
	}
	return l.src[l.offset]
}

func (l *Lexer) peekN(n int) byte {
	idx := l.offset + n
	if idx >= len(l.src) {
		return 0
	}
	return l.src[idx]
}

func (l *Lexer) advance() {
	if l.isEOF() {
		return
	}
	ch := l.src[l.offset]
	l.offset++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
}

func (l *Lexer) advanceN(n int) {
	for i := 0; i < n; i++ {
		l.advance()
	}
}

func (l *Lexer) matchesString(s string) bool {
	if l.offset+len(s) > len(l.src) {
		return false
	}
	return string(l.src[l.offset:l.offset+len(s)]) == s
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
