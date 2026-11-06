package loneparser

import (
	"fmt"
	lonelexer "lone/pkg/lone-lexer"
	loneast "lone/pkg/lone-parser-ast"
	"strconv"
	"strings"
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

		// Disambiguate leading prefix / statement tokens on a new line:
		// If the operator starts on a later line than the prefix expression ended,
		// and it could begin a new statement/expression (e.g. *, &, +, -, (, [),
		// it should not greedily bind as an infix operator of the previous statement.
		if op.Start.Line > prefixExpr.NodeSpan().End.Line {
			if op.Kind == lonelexer.TokenStar ||
				op.Kind == lonelexer.TokenAmpersand ||
				op.Kind == lonelexer.TokenPlus ||
				op.Kind == lonelexer.TokenMinus ||
				op.Kind == lonelexer.TokenLParen ||
				op.Kind == lonelexer.TokenLBracket {
				break
			}
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

	case lonelexer.TokenFString:
		return p.parseFString(tok)

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
		if p.peek().Kind == lonelexer.TokenColon && p.peekN(1).Kind == lonelexer.TokenLBrace {
			label := tok.Text
			p.advance() // consume ':'
			blk, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			return loneast.LabelDec{
				Name:  label,
				Body:  blk,
				Span_: lonelexer.Span{Start: tok.Start, End: blk.NodeSpan().End},
			}, nil
		}
		return loneast.NameRef{
			Name:  tok.Text,
			Span_: tok.Span(),
		}, nil

	case lonelexer.TokenSelf:
		return loneast.NameRef{
			Name:  tok.Text,
			Span_: tok.Span(),
		}, nil

	case lonelexer.TokenIntrinsic:
		return loneast.NameRef{
			Name:  tok.Text,
			Span_: tok.Span(),
		}, nil

	case lonelexer.TokenFn:
		// Anonymous function: fn([params]) [RetType] { body }
		var typeParams []loneast.TypeParam
		var constraints []loneast.GenericConstraint
		if p.peek().Kind == lonelexer.TokenLt {
			tps, cs, err := p.parseTypeParamsAndConstraints()
			if err != nil {
				return nil, err
			}
			typeParams = tps
			constraints = cs
		}

		if _, err := p.expect(lonelexer.TokenLParen); err != nil {
			return nil, err
		}

		var params []loneast.FnParam
		if p.peek().Kind != lonelexer.TokenRParen {
			for {
				if p.peek().Kind == lonelexer.TokenSelf {
					selfTok := p.advance()
					if p.peek().Kind == lonelexer.TokenColon {
						p.advance()
						pType, err := p.parseTypeRef()
						if err != nil {
							return nil, err
						}
						params = append(params, loneast.FnParam{
							Name:  "self",
							Type:  pType,
							Span_: loneast.Span{Start: selfTok.Span().Start, End: pType.NodeSpan().End},
						})
					} else {
						params = append(params, loneast.FnParam{
							Name: "self",
							Type: loneast.TYPE_REF{
								Kind:  loneast.TYPE_BASE,
								Value: &loneast.TypeBase{Name: "Self"},
								Span_: selfTok.Span(),
							},
							Span_: selfTok.Span(),
						})
					}
				} else if p.peek().Kind == lonelexer.TokenLBracket {
					lbrack := p.advance()
					var names []string
					for {
						idTok, err := p.expect(lonelexer.TokenIdent)
						if err != nil {
							return nil, err
						}
						names = append(names, idTok.Text)
						if p.peek().Kind == lonelexer.TokenComma {
							p.advance()
							if p.peek().Kind == lonelexer.TokenRBracket {
								break
							}
						} else {
							break
						}
					}
					if _, err := p.expect(lonelexer.TokenRBracket); err != nil {
						return nil, err
					}
					if _, err := p.expect(lonelexer.TokenColon); err != nil {
						return nil, err
					}
					pType, err := p.parseTypeRef()
					if err != nil {
						return nil, err
					}
					for _, n := range names {
						params = append(params, loneast.FnParam{
							Name:  n,
							Names: names,
							Type:  pType,
							Span_: lonelexer.Span{Start: lbrack.Start, End: pType.NodeSpan().End},
						})
					}
				} else {
					pName, err := p.expect(lonelexer.TokenIdent)
					if err != nil {
						return nil, err
					}
					if _, err := p.expect(lonelexer.TokenColon); err != nil {
						return nil, err
					}
					pType, err := p.parseTypeRef()
					if err != nil {
						return nil, err
					}
					params = append(params, loneast.FnParam{
						Name:  pName.Text,
						Type:  pType,
						Span_: lonelexer.Span{Start: pName.Start, End: pType.NodeSpan().End},
					})
				}

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

		if _, err := p.expect(lonelexer.TokenRParen); err != nil {
			return nil, err
		}

		var retType *loneast.TYPE_REF
		if isTypeStart(p.peek().Kind) {
			rt, err := p.parseTypeRef()
			if err != nil {
				return nil, err
			}
			retType = &rt
		}

		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}

		return loneast.AnonFnExpr{
			TypeParams:  typeParams,
			Constraints: constraints,
			Params:      params,
			ReturnType:  retType,
			Body:        body,
			Span_:       lonelexer.Span{Start: tok.Start, End: body.NodeSpan().End},
		}, nil

	case lonelexer.TokenLParen:
		if p.peek().Kind == lonelexer.TokenRParen {
			rp := p.advance()
			return loneast.TupleExpr{
				Elements: nil,
				Span_:    lonelexer.Span{Start: tok.Start, End: rp.End},
			}, nil
		}
		first, err := p.ParseExpr(PrecNone)
		if err != nil {
			return nil, err
		}
		if p.peek().Kind == lonelexer.TokenComma {
			p.advance() // consume ','
			elements := []loneast.Expr{first}
			for p.peek().Kind != lonelexer.TokenRParen && !p.isAtEnd() {
				elem, err := p.ParseExpr(PrecNone)
				if err != nil {
					return nil, err
				}
				elements = append(elements, elem)
				if p.peek().Kind == lonelexer.TokenComma {
					p.advance()
				} else {
					break
				}
			}
			rp, err := p.expect(lonelexer.TokenRParen)
			if err != nil {
				return nil, err
			}
			return loneast.TupleExpr{
				Elements: elements,
				Span_:    lonelexer.Span{Start: tok.Start, End: rp.End},
			}, nil
		}
		rp, err := p.expect(lonelexer.TokenRParen)
		if err != nil {
			return nil, err
		}
		_ = rp
		return first, nil

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

	case lonelexer.TokenMatch:
		return p.parseMatchExpr(tok)

	case lonelexer.TokenMust:
		inner, err := p.ParseExpr(PrecUnary)
		if err != nil {
			return nil, err
		}
		return loneast.MustExpr{
			Expr:  inner,
			Span_: lonelexer.Span{Start: tok.Start, End: inner.NodeSpan().End},
		}, nil

	case lonelexer.TokenTry:
		inner, err := p.ParseExpr(PrecUnary)
		if err != nil {
			return nil, err
		}
		return loneast.TryExpr{
			Expr:  inner,
			Span_: lonelexer.Span{Start: tok.Start, End: inner.NodeSpan().End},
		}, nil

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
		var memberName string
		var endPos lonelexer.Position
		if p.peek().Kind == lonelexer.TokenInt {
			intTok := p.advance()
			memberName = intTok.Text
			endPos = intTok.End
		} else {
			ident, err := p.expect(lonelexer.TokenIdent)
			if err != nil {
				return nil, err
			}
			memberName = ident.Text
			endPos = ident.End
		}
		return appendSuffix(left, loneast.SuffixMemberAccess{Name: memberName, Span_: lonelexer.Span{Start: tok.Start, End: endPos}}, endPos), nil

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
		if p.peek().Kind == lonelexer.TokenLParen {
			p.advance() // consume '('
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
			return appendSuffix(left, loneast.SuffixConstructorCall{Args: args, Span_: lonelexer.Span{Start: tok.Start, End: rparen.End}}, rparen.End), nil
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

func (p *Parser) parseMatchExpr(matchTok lonelexer.Token) (loneast.Expr, error) {
	target, err := p.ParseExpr(PrecNone)
	if err != nil {
		return nil, err
	}

	lbrace, err := p.expect(lonelexer.TokenLBrace)
	if err != nil {
		return nil, err
	}
	_ = lbrace

	var arms []loneast.MatchArm
	for p.peek().Kind != lonelexer.TokenRBrace && !p.isAtEnd() {
		pat, err := p.parseMatchPattern()
		if err != nil {
			return nil, err
		}

		arrow, err := p.expect(lonelexer.TokenFatArrow)
		if err != nil {
			return nil, err
		}
		_ = arrow

		var body any
		if p.peek().Kind == lonelexer.TokenLBrace {
			// Check if object literal: { .field: value }
			if p.peekN(1).Kind == lonelexer.TokenDot {
				braceTok := p.advance()
				bExpr, err := p.parseObjectLiteral(braceTok)
				if err != nil {
					return nil, err
				}
				body = bExpr
			} else {
				blk, err := p.parseBlock()
				if err != nil {
					return nil, err
				}
				body = blk
			}
		} else {
			bExpr, err := p.ParseExpr(PrecNone)
			if err != nil {
				return nil, err
			}
			body = bExpr
		}

		endPos := p.previous().End
		if p.peek().Kind == lonelexer.TokenComma {
			commaTok := p.advance()
			endPos = commaTok.End
		}

		arms = append(arms, loneast.MatchArm{
			Pattern: pat,
			Body:    body,
			Span_:   lonelexer.Span{Start: pat.NodeSpan().Start, End: endPos},
		})
	}

	rbrace, err := p.expect(lonelexer.TokenRBrace)
	if err != nil {
		return nil, err
	}

	return loneast.MatchExpr{
		Target: target,
		Arms:   arms,
		Span_:  lonelexer.Span{Start: matchTok.Start, End: rbrace.End},
	}, nil
}

func (p *Parser) parseMatchPattern() (loneast.MatchPattern, error) {
	tok := p.peek()

	// Wildcard: _
	if tok.Kind == lonelexer.TokenIdent && tok.Text == "_" {
		p.advance()
		return loneast.WildcardPattern{Span_: tok.Span()}, nil
	}

	// Literals: int, float, string, char, bool, none
	if tok.Kind == lonelexer.TokenInt || tok.Kind == lonelexer.TokenFloat ||
		tok.Kind == lonelexer.TokenString || tok.Kind == lonelexer.TokenChar ||
		tok.Kind == lonelexer.TokenTrue || tok.Kind == lonelexer.TokenFalse ||
		tok.Kind == lonelexer.TokenNone {
		litExpr, err := p.parsePrefix()
		if err != nil {
			return nil, err
		}
		if lit, ok := litExpr.(loneast.Literal); ok {
			return loneast.LiteralPattern{Literal: lit, Span_: lit.Span_}, nil
		}
		return nil, fmt.Errorf("expected literal pattern at %s", tok.Start)
	}

	// Variant: Type::Variant(bindings...) or Type::Variant or Variant(bindings...) or Variant
	if tok.Kind == lonelexer.TokenIdent {
		id1 := p.advance()
		typeName := ""
		variantName := id1.Text

		if p.peek().Kind == lonelexer.TokenColon2 {
			p.advance() // consume '::'
			id2, err := p.expect(lonelexer.TokenIdent)
			if err != nil {
				return nil, err
			}
			typeName = id1.Text
			variantName = id2.Text
		}

		var bindings []string
		if p.peek().Kind == lonelexer.TokenLParen {
			p.advance()
			if p.peek().Kind != lonelexer.TokenRParen {
				for {
					bTok, err := p.expect(lonelexer.TokenIdent)
					if err != nil {
						return nil, err
					}
					bindings = append(bindings, bTok.Text)
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
			_, err := p.expect(lonelexer.TokenRParen)
			if err != nil {
				return nil, err
			}
		}

		return loneast.VariantPattern{
			TypeName: typeName,
			Variant:  variantName,
			Bindings: bindings,
			Span_:    lonelexer.Span{Start: id1.Start, End: p.previous().End},
		}, nil
	}

	// Tuple pattern: (p1, p2, ...) or ()
	if tok.Kind == lonelexer.TokenLParen {
		lp := p.advance()
		var elements []loneast.MatchPattern
		if p.peek().Kind != lonelexer.TokenRParen {
			for {
				elem, err := p.parseMatchPattern()
				if err != nil {
					return nil, err
				}
				elements = append(elements, elem)
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
		rp, err := p.expect(lonelexer.TokenRParen)
		if err != nil {
			return nil, err
		}
		return loneast.TuplePattern{
			Elements: elements,
			Span_:    lonelexer.Span{Start: lp.Start, End: rp.End},
		}, nil
	}

	return nil, fmt.Errorf("unexpected pattern token %s (%q) at %s", tok.Kind, tok.Text, tok.Start)
}

func (p *Parser) parseFString(tok lonelexer.Token) (loneast.Expr, error) {
	text := tok.Text
	var parts []loneast.Expr
	var curLit strings.Builder

	flushLit := func() {
		if curLit.Len() > 0 {
			parts = append(parts, loneast.Literal{
				Kind:  loneast.LiteralString,
				Value: curLit.String(),
				Span_: tok.Span(),
			})
			curLit.Reset()
		}
	}

	i := 0
	for i < len(text) {
		// Check for escaped \${
		if i+2 < len(text) && text[i:i+3] == "\\${" {
			curLit.WriteString("${")
			i += 3
			continue
		}

		if i+1 < len(text) && text[i:i+2] == "${" {
			flushLit()
			exprStart := i + 2
			j := exprStart
			braceDepth := 1
			for j < len(text) && braceDepth > 0 {
				if text[j] == '{' {
					braceDepth++
				} else if text[j] == '}' {
					braceDepth--
				}
				if braceDepth > 0 {
					j++
				}
			}
			if braceDepth > 0 {
				return nil, fmt.Errorf("unclosed interpolation '${' in format string at %s", tok.Start)
			}
			exprStr := strings.TrimSpace(text[exprStart:j])
			if exprStr == "" {
				return nil, fmt.Errorf("empty interpolation '${}' in format string at %s", tok.Start)
			}

			subTokens, err := lonelexer.NewLexer([]byte(exprStr)).Tokenize()
			if err != nil {
				return nil, fmt.Errorf("lexing interpolation expression failed at %s: %w", tok.Start, err)
			}
			subParser := NewParser(subTokens)
			subExpr, err := subParser.ParseExpr(PrecNone)
			if err != nil {
				return nil, fmt.Errorf("parsing interpolation expression failed at %s: %w", tok.Start, err)
			}
			parts = append(parts, subExpr)

			i = j + 1
			continue
		}

		curLit.WriteByte(text[i])
		i++
	}

	flushLit()

	return loneast.InterpolatedStringExpr{
		Parts: parts,
		Span_: tok.Span(),
	}, nil
}

