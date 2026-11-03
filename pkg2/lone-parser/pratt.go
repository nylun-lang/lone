package loneparser

import (
	"fmt"
	lonelexer "lone/pkg2/lone-lexer"
	loneast "lone/pkg2/lone-parser-ast"
	"strconv"
)

type Precedence int

const (
	PrecNone Precedence = iota
	PrecAssign
	PrecLogicalOr
	PrecLogicalXor
	PrecLogicalAnd
	PrecBitwiseOr
	PrecBitwiseXor
	PrecBitwiseAnd
	PrecEquality
	PrecComparison
	PrecCast
	PrecTerm
	PrecFactor
	PrecUnary
	PrecPostfix
)

func tokenPrecedence(kind lonelexer.TokenKind) Precedence {
	switch kind {
	case lonelexer.TokenAssign:
		return PrecAssign
	case lonelexer.TokenOr:
		return PrecLogicalOr
	case lonelexer.TokenXor:
		return PrecLogicalXor
	case lonelexer.TokenAnd:
		return PrecLogicalAnd
	case lonelexer.TokenPipe:
		return PrecBitwiseOr
	case lonelexer.TokenCaret:
		return PrecBitwiseXor
	case lonelexer.TokenAmpersand:
		return PrecBitwiseAnd
	case lonelexer.TokenEq, lonelexer.TokenNotEq:
		return PrecEquality
	case lonelexer.TokenLt, lonelexer.TokenLtEq, lonelexer.TokenGt, lonelexer.TokenGtEq, lonelexer.TokenIs:
		return PrecComparison
	case lonelexer.TokenAs:
		return PrecCast
	case lonelexer.TokenPlus, lonelexer.TokenMinus:
		return PrecTerm
	case lonelexer.TokenStar, lonelexer.TokenSlash, lonelexer.TokenPercent:
		return PrecFactor
	case lonelexer.TokenDot, lonelexer.TokenColon, lonelexer.TokenColon2, lonelexer.TokenColon2Pipe,
		lonelexer.TokenLParen, lonelexer.TokenLBracket:
		return PrecPostfix
	default:
		return PrecNone
	}
}

// ParseExpr parses an expression using Pratt top-down operator precedence.
func (p *Parser) ParseExpr(minPrec Precedence) (loneast.Expr, error) {
	prefixExpr, err := p.parsePrefix()
	if err != nil {
		return nil, err
	}

	for !p.isAtEnd() {
		prec := tokenPrecedence(p.peek().Kind)
		if prec == PrecNone || prec < minPrec {
			break
		}

		op := p.peek()
		// Special check: assignment '=' in expression position
		if op.Kind == lonelexer.TokenAssign {
			break
		}

		nextExpr, err := p.parseInfix(prefixExpr)
		if err != nil {
			return nil, err
		}
		prefixExpr = nextExpr
	}

	return prefixExpr, nil
}

func (p *Parser) parsePrefix() (loneast.Expr, error) {
	tok := p.advance()

	switch tok.Kind {
	case lonelexer.TokenInt:
		val, _ := strconv.ParseInt(stringsReplace(tok.Text, "_", ""), 0, 64)
		return loneast.Literal{
			Kind:  loneast.LiteralInt,
			Value: val,
			Span_: tok.Span(),
		}, nil

	case lonelexer.TokenFloat:
		val, _ := strconv.ParseFloat(stringsReplace(tok.Text, "_", ""), 64)
		return loneast.Literal{
			Kind:  loneast.LiteralFloat,
			Value: val,
			Span_: tok.Span(),
		}, nil

	case lonelexer.TokenString:
		return loneast.Literal{
			Kind:  loneast.LiteralString,
			Value: tok.Text,
			Span_: tok.Span(),
		}, nil

	case lonelexer.TokenChar:
		return loneast.Literal{
			Kind:  loneast.LiteralChar,
			Value: tok.Text,
			Span_: tok.Span(),
		}, nil

	case lonelexer.TokenTrue:
		return loneast.Literal{
			Kind:  loneast.LiteralBool,
			Value: true,
			Span_: tok.Span(),
		}, nil

	case lonelexer.TokenFalse:
		return loneast.Literal{
			Kind:  loneast.LiteralBool,
			Value: false,
			Span_: tok.Span(),
		}, nil

	case lonelexer.TokenNone:
		return loneast.Literal{
			Kind:  loneast.LiteralNone,
			Value: nil,
			Span_: tok.Span(),
		}, nil

	case lonelexer.TokenIdent:
		return loneast.NameRef{
			Name:  tok.Text,
			Span_: tok.Span(),
		}, nil

	case lonelexer.TokenIntrinsic:
		return loneast.NameRef{
			Name:  tok.Text,
			Span_: tok.Span(),
		}, nil

	case lonelexer.TokenLParen:
		inner, err := p.ParseExpr(PrecNone)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lonelexer.TokenRParen); err != nil {
			return nil, err
		}
		return inner, nil

	case lonelexer.TokenLBracket:
		// List literal: [1, 2, 3] or []
		return p.parseListLiteral(tok)

	case lonelexer.TokenLBrace:
		// Object literal: { .x: 1, .y: 2 } or {}
		return p.parseObjectLiteral(tok)

	// Unary operators
	case lonelexer.TokenPlus:
		return p.parseUnary(loneast.UNARY_OP_POSITIVE, tok, PrecUnary)
	case lonelexer.TokenMinus:
		return p.parseUnary(loneast.UNARY_OP_NEGATIVE, tok, PrecUnary)
	case lonelexer.TokenExcl:
		return p.parseUnary(loneast.UNARY_OP_NOT, tok, PrecUnary)
	case lonelexer.TokenStar:
		return p.parseUnary(loneast.UNARY_OP_DEREF, tok, PrecUnary)
	case lonelexer.TokenAmpersand:
		return p.parseUnary(loneast.UNARY_OP_REF, tok, PrecUnary)
	case lonelexer.TokenAmpRaw:
		return p.parseUnary(loneast.UNARY_OP_RAW_REF, tok, PrecUnary)

	default:
		return nil, fmt.Errorf("unexpected token %s (%q) at %s", tok.Kind, tok.Text, tok.Start)
	}
}

func (p *Parser) parseUnary(op loneast.UnaryOp, opTok lonelexer.Token, prec Precedence) (loneast.Expr, error) {
	expr, err := p.ParseExpr(prec)
	if err != nil {
		return nil, err
	}
	return loneast.UnaryExpr{
		Op:    op,
		Expr:  expr,
		Span_: lonelexer.Span{Start: opTok.Start, End: expr.NodeSpan().End},
	}, nil
}

func appendSuffix(left loneast.Expr, suffix loneast.Suffix, end lonelexer.Position) loneast.PostFixExpr {
	if pfe, ok := left.(loneast.PostFixExpr); ok {
		pfe.Suffix = append(pfe.Suffix, suffix)
		pfe.Span_.End = end
		return pfe
	}
	return loneast.PostFixExpr{
		Base:   left,
		Suffix: []loneast.Suffix{suffix},
		Span_:  lonelexer.Span{Start: left.NodeSpan().Start, End: end},
	}
}

func (p *Parser) parseInfix(left loneast.Expr) (loneast.Expr, error) {
	tok := p.peek()
	prec := tokenPrecedence(tok.Kind)

	switch tok.Kind {
	// Postfix member access: left.name
	case lonelexer.TokenDot:
		p.advance()
		ident, err := p.expect(lonelexer.TokenIdent)
		if err != nil {
			return nil, err
		}
		return appendSuffix(left, loneast.SuffixMemberAccess{Name: ident.Text, Span_: ident.Span()}, ident.End), nil

	// Postfix namespace access: left:name
	case lonelexer.TokenColon:
		p.advance()
		ident, err := p.expect(lonelexer.TokenIdent)
		if err != nil {
			return nil, err
		}
		return appendSuffix(left, loneast.SuffixNamespaceAccess{Name: ident.Text, Span_: ident.Span()}, ident.End), nil

	// Postfix associated access: left::name, left::{ ... }, left::[ ... ]
	case lonelexer.TokenColon2:
		p.advance()
		if p.peek().Kind == lonelexer.TokenLBrace {
			braceTok := p.advance()
			obj, err := p.parseObjectLiteral(braceTok)
			if err != nil {
				return nil, err
			}
			return appendSuffix(left, loneast.SuffixCallTypeMake{Data: obj, Span_: obj.NodeSpan()}, obj.NodeSpan().End), nil
		}
		if p.peek().Kind == lonelexer.TokenLBracket {
			brackTok := p.advance()
			list, err := p.parseListLiteral(brackTok)
			if err != nil {
				return nil, err
			}
			return appendSuffix(left, loneast.SuffixCallTypeMake{Data: list, Span_: list.NodeSpan()}, list.NodeSpan().End), nil
		}

		ident, err := p.expect(lonelexer.TokenIdent)
		if err != nil {
			return nil, err
		}
		return appendSuffix(left, loneast.SuffixAssociatedAccess{Name: ident.Text, Span_: ident.Span()}, ident.End), nil

	// Postfix trait materialization: left::|TargetType|
	case lonelexer.TokenColon2Pipe:
		p.advance()
		tr, err := p.parseTypeRef()
		if err != nil {
			return nil, err
		}
		pipe, err := p.expect(lonelexer.TokenPipe)
		if err != nil {
			return nil, err
		}
		return appendSuffix(left, loneast.SuffixTraitMat{TargetType: tr, Span_: lonelexer.Span{Start: tok.Start, End: pipe.End}}, pipe.End), nil

	// Postfix Call: left(args...)
	case lonelexer.TokenLParen:
		p.advance()
		args := []loneast.Expr{}
		if p.peek().Kind != lonelexer.TokenRParen {
			for {
				arg, err := p.ParseExpr(PrecNone)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
				if p.peek().Kind == lonelexer.TokenComma {
					p.advance()
					if p.peek().Kind == lonelexer.TokenRParen {
						break
					}
				} else {
					break
				}
			}
		}
		rparen, err := p.expect(lonelexer.TokenRParen)
		if err != nil {
			return nil, err
		}
		return appendSuffix(left, loneast.SuffixCall{Args: args, Span_: lonelexer.Span{Start: tok.Start, End: rparen.End}}, rparen.End), nil

	// Postfix Index: left[index]
	case lonelexer.TokenLBracket:
		p.advance()
		idx, err := p.ParseExpr(PrecNone)
		if err != nil {
			return nil, err
		}
		rbrack, err := p.expect(lonelexer.TokenRBracket)
		if err != nil {
			return nil, err
		}
		return appendSuffix(left, loneast.SuffixIndex{Index: idx, Span_: lonelexer.Span{Start: tok.Start, End: rbrack.End}}, rbrack.End), nil

	// Explicit cast: expr as TargetType
	case lonelexer.TokenAs:
		p.advance()
		targetType, err := p.parseTypeRef()
		if err != nil {
			return nil, err
		}
		return loneast.CastExpr{
			Expr:       left,
			TargetType: targetType,
			Span_:      lonelexer.Span{Start: left.NodeSpan().Start, End: targetType.NodeSpan().End},
		}, nil

	// Type test: expr is [not] TargetType
	case lonelexer.TokenIs:
		p.advance()
		negated := false
		if p.peek().Kind == lonelexer.TokenNot {
			p.advance()
			negated = true
		}
		targetType, err := p.parseTypeRef()
		if err != nil {
			return nil, err
		}
		return loneast.IsExpr{
			Expr:       left,
			TargetType: targetType,
			Negated:    negated,
			Span_:      lonelexer.Span{Start: left.NodeSpan().Start, End: targetType.NodeSpan().End},
		}, nil

	// Standard binary arithmetic / comparison / logical operators
	default:
		opTok := p.advance()
		right, err := p.ParseExpr(prec + 1)
		if err != nil {
			return nil, err
		}
		binOp := loneast.BinaryOp(opTok.Text)
		return loneast.BinaryExpr{
			Op:    binOp,
			Left:  left,
			Right: right,
			Span_: lonelexer.Span{Start: left.NodeSpan().Start, End: right.NodeSpan().End},
		}, nil
	}
}

func (p *Parser) parseListLiteral(startTok lonelexer.Token) (loneast.Expr, error) {
	var elements []loneast.Expr
	if p.peek().Kind != lonelexer.TokenRBracket {
		for {
			elem, err := p.ParseExpr(PrecNone)
			if err != nil {
				return nil, err
			}
			elements = append(elements, elem)
			if p.peek().Kind == lonelexer.TokenComma {
				p.advance()
				if p.peek().Kind == lonelexer.TokenRBracket {
					break
				}
			} else {
				break
			}
		}
	}

	rbrack, err := p.expect(lonelexer.TokenRBracket)
	if err != nil {
		return nil, err
	}

	return loneast.ListLiteral{
		Elements: elements,
		Span_:    lonelexer.Span{Start: startTok.Start, End: rbrack.End},
	}, nil
}

func (p *Parser) parseObjectLiteral(startTok lonelexer.Token) (loneast.Expr, error) {
	var fields []loneast.FieldAssign
	if p.peek().Kind != lonelexer.TokenRBrace {
		for {
			// Field is '.fieldName: value' or '.fieldName value'
			dot, err := p.expect(lonelexer.TokenDot)
			if err != nil {
				return nil, err
			}
			nameTok, err := p.expect(lonelexer.TokenIdent)
			if err != nil {
				return nil, err
			}
			if p.peek().Kind == lonelexer.TokenColon {
				p.advance()
			}
			valExpr, err := p.ParseExpr(PrecNone)
			if err != nil {
				return nil, err
			}

			fields = append(fields, loneast.FieldAssign{
				Name:  nameTok.Text,
				Expr:  valExpr,
				Span_: lonelexer.Span{Start: dot.Start, End: valExpr.NodeSpan().End},
			})

			if p.peek().Kind == lonelexer.TokenComma {
				p.advance()
				if p.peek().Kind == lonelexer.TokenRBrace {
					break
				}
			} else {
				break
			}
		}
	}

	rbrace, err := p.expect(lonelexer.TokenRBrace)
	if err != nil {
		return nil, err
	}

	return loneast.ObjectLiteral{
		Fields: fields,
		Span_:  lonelexer.Span{Start: startTok.Start, End: rbrace.End},
	}, nil
}

func stringsReplace(s, old, new string) string {
	var res string
	for i := 0; i < len(s); i++ {
		if string(s[i]) != old {
			res += string(s[i])
		} else {
			res += new
		}
	}
	return res
}
