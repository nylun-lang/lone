package loneparser

import (
	"fmt"
	lonelexer "lone/pkg2/lone-lexer"
	loneast "lone/pkg2/lone-parser-ast"
)

// Parser parses a token stream into an AST.
type Parser struct {
	tokens []lonelexer.Token
	cursor int
}

// NewParser creates a new Parser instance.
func NewParser(tokens []lonelexer.Token) *Parser {
	return &Parser{
		tokens: tokens,
		cursor: 0,
	}
}

// Parse parses source code directly from byte slice.
func Parse(src []byte) ([]any, error) {
	lex := lonelexer.NewLexer(src)
	tokens, err := lex.Tokenize()
	if err != nil {
		return nil, err
	}
	p := NewParser(tokens)
	return p.ParseProgram()
}

// ParseProgram parses top-level declarations until EOF.
func (p *Parser) ParseProgram() ([]any, error) {
	var decls []any

	for !p.isAtEnd() {
		decl, err := p.parseTopLevel()
		if err != nil {
			return nil, err
		}
		if decl != nil {
			decls = append(decls, decl)
		}
	}

	return decls, nil
}

func (p *Parser) parseTopLevel() (any, error) {
	tok := p.peek()

	// 1. Directive @fa:name
	if tok.Kind == lonelexer.TokenFaDirective {
		return p.parseDirective()
	}

	// 2. Import statement: import ...
	if tok.Kind == lonelexer.TokenImport {
		return p.parseImport()
	}

	// 3. Public visibility prefix
	isPub := false
	if tok.Kind == lonelexer.TokenPub {
		p.advance()
		isPub = true
	}

	// 4. Type declaration: type Name = ...
	if p.peek().Kind == lonelexer.TokenType {
		return p.parseTypeDec(isPub)
	}

	// 5. Impl declaration: impl Trait for Target { ... }
	if p.peek().Kind == lonelexer.TokenImpl {
		return p.parseImplDec()
	}

	// 6. Function declaration: fn name(...) ...
	if p.peek().Kind == lonelexer.TokenFn {
		return p.parseFnDec(isPub)
	}

	// 7. Binding declaration: let name = ... or var name = ...
	if p.peek().Kind == lonelexer.TokenLet || p.peek().Kind == lonelexer.TokenVar {
		return p.parseBindDec(isPub)
	}

	// 8. Top-level statements
	return p.parseStmt()
}

// =====================
// DECLARATIONS
// =====================

func (p *Parser) parseDirective() (loneast.WasmDirective, error) {
	dirTok := p.advance()
	name := dirTok.Text
	if len(name) > 4 && name[:4] == "@fa:" {
		name = name[4:]
	}

	var imports []loneast.ImportDecl
	isInline := true

	if p.peek().Kind == lonelexer.TokenLBrace {
		p.advance() // consume '{'
		isInline = false
		for p.peek().Kind != lonelexer.TokenRBrace && !p.isAtEnd() {
			if p.peek().Kind == lonelexer.TokenImport {
				imp, err := p.parseImport()
				if err != nil {
					return loneast.WasmDirective{}, err
				}
				imports = append(imports, imp)
			} else {
				p.advance()
			}
		}
		rbrace, err := p.expect(lonelexer.TokenRBrace)
		if err != nil {
			return loneast.WasmDirective{}, err
		}
		return loneast.WasmDirective{
			Name:    name,
			Imports: imports,
			Inline:  isInline,
			Span_:   lonelexer.Span{Start: dirTok.Start, End: rbrace.End},
		}, nil
	}

	// Single line directive
	return loneast.WasmDirective{
		Name:    name,
		Imports: imports,
		Inline:  isInline,
		Span_:   dirTok.Span(),
	}, nil
}

func (p *Parser) parseImport() (loneast.ImportDecl, error) {
	importTok := p.advance() // consume 'import'

	// Alias import: import :source as alias
	if p.peek().Kind == lonelexer.TokenColon {
		src, err := p.parseImportSource()
		if err != nil {
			return loneast.ImportDecl{}, err
		}
		asTok, err := p.expect(lonelexer.TokenAs)
		if err != nil {
			return loneast.ImportDecl{}, err
		}
		_ = asTok
		aliasTok, err := p.expect(lonelexer.TokenIdent)
		if err != nil {
			return loneast.ImportDecl{}, err
		}
		return loneast.ImportDecl{
			Symbols:     nil,
			Source:      src,
			IsNamespace: len(src) > 0 && src[0] == ':',
			Alias:       aliasTok.Text,
			Span_:       lonelexer.Span{Start: importTok.Start, End: aliasTok.End},
		}, nil
	}

	// Symbol list import: import .sym1, .sym2 from :source
	var symbols []string
	for {
		dot, err := p.expect(lonelexer.TokenDot)
		if err != nil {
			return loneast.ImportDecl{}, err
		}
		_ = dot
		symTok, err := p.expect(lonelexer.TokenIdent)
		if err != nil {
			return loneast.ImportDecl{}, err
		}
		symbols = append(symbols, symTok.Text)

		if p.peek().Kind == lonelexer.TokenComma {
			p.advance()
			if p.peek().Kind == lonelexer.TokenFrom {
				break
			}
		} else {
			break
		}
	}

	fromTok, err := p.expect(lonelexer.TokenFrom)
	if err != nil {
		return loneast.ImportDecl{}, err
	}
	_ = fromTok

	src, err := p.parseImportSource()
	if err != nil {
		return loneast.ImportDecl{}, err
	}

	return loneast.ImportDecl{
		Symbols:     symbols,
		Source:      src,
		IsNamespace: len(src) > 0 && src[0] == ':',
		Alias:       "",
		Span_:       lonelexer.Span{Start: importTok.Start, End: p.previous().End},
	}, nil
}

func (p *Parser) parseImportSource() (string, error) {
	var sb string
	// Namespace: :std:io
	if p.peek().Kind == lonelexer.TokenColon {
		for p.peek().Kind == lonelexer.TokenColon {
			p.advance()
			sb += ":"
			id, err := p.expect(lonelexer.TokenIdent)
			if err != nil {
				return "", err
			}
			sb += id.Text
		}
		return sb, nil
	}

	// Relative path: .helper or ..utils
	if p.peek().Kind == lonelexer.TokenDot {
		for p.peek().Kind == lonelexer.TokenDot {
			p.advance()
			sb += "."
		}
		id, err := p.expect(lonelexer.TokenIdent)
		if err != nil {
			return "", err
		}
		sb += id.Text
		return sb, nil
	}

	return "", fmt.Errorf("expected import source at %s", p.peek().Start)
}

func (p *Parser) parseBindDec(isPub bool) (loneast.BIND_DEC, error) {
	kwTok := p.advance() // 'let' or 'var'
	isMut := kwTok.Kind == lonelexer.TokenVar

	// Check for pattern destructuring: let Shape::Circle(...) = expr or let Circle(...) = expr
	if p.peek().Kind == lonelexer.TokenIdent && (p.peekN(1).Kind == lonelexer.TokenColon2 || p.peekN(1).Kind == lonelexer.TokenLParen) {
		pat, err := p.parseMatchPattern()
		if err != nil {
			return loneast.BIND_DEC{}, err
		}
		var val loneast.Expr
		if p.peek().Kind == lonelexer.TokenAssign {
			p.advance()
			initVal, err := p.ParseExpr(PrecNone)
			if err != nil {
				return loneast.BIND_DEC{}, err
			}
			val = initVal
		}
		endPos := pat.NodeSpan().End
		if val != nil {
			endPos = val.NodeSpan().End
		}
		varName := ""
		if vp, ok := pat.(loneast.VariantPattern); ok {
			varName = vp.Variant
		}
		return loneast.BIND_DEC{
			Name:      varName,
			Pattern:   pat,
			IsPublic:  isPub,
			IsMutable: isMut,
			Value:     val,
			Type:      loneast.TYPE_REF{Kind: loneast.TYPE_BASE, Value: &loneast.TypeBase{Name: "infer"}, Span_: pat.NodeSpan()},
			Span_:     lonelexer.Span{Start: kwTok.Start, End: endPos},
		}, nil
	}

	nameTok, err := p.expect(lonelexer.TokenIdent)
	if err != nil {
		return loneast.BIND_DEC{}, err
	}

	// Optional type annotation: : Type
	var tr loneast.TYPE_REF
	if p.peek().Kind == lonelexer.TokenColon {
		p.advance()
		parsedTr, err := p.parseTypeRef()
		if err != nil {
			return loneast.BIND_DEC{}, err
		}
		tr = parsedTr
	} else {
		tr = loneast.TYPE_REF{
			Kind: loneast.TYPE_BASE,
			Value: &loneast.TypeBase{
				Name: "infer",
			},
			Span_: nameTok.Span(),
		}
	}

	// Optional initializer: = expr
	var val loneast.Expr = loneast.Literal{
		Kind:  loneast.LiteralUninitialized,
		Span_: nameTok.Span(),
	}
	if p.peek().Kind == lonelexer.TokenAssign {
		p.advance()
		initVal, err := p.ParseExpr(PrecNone)
		if err != nil {
			return loneast.BIND_DEC{}, err
		}
		val = initVal
	}

	endPos := nameTok.End
	if val != nil {
		endPos = val.NodeSpan().End
	}

	return loneast.BIND_DEC{
		Name:      nameTok.Text,
		IsPublic:  isPub,
		IsMutable: isMut,
		Value:     val,
		Type:      tr,
		Span_:     lonelexer.Span{Start: kwTok.Start, End: endPos},
	}, nil
}

func (p *Parser) parseFnDec(isPub bool) (loneast.FN_DEC, error) {
	fnTok := p.advance() // consume 'fn'

	var nameTok lonelexer.Token
	if p.peek().Kind == lonelexer.TokenIntrinsic || p.peek().Kind == lonelexer.TokenIdent {
		nameTok = p.advance()
	} else {
		return loneast.FN_DEC{}, fmt.Errorf("expected function name at %s", p.peek().Start)
	}

	// Generic type parameters: fn foo<T, U = Default where { ... }>(...)
	var typeParams []loneast.TypeParam
	var constraints []loneast.GenericConstraint
	if p.peek().Kind == lonelexer.TokenLt {
		tps, cs, err := p.parseTypeParamsAndConstraints()
		if err != nil {
			return loneast.FN_DEC{}, err
		}
		typeParams = tps
		constraints = cs
	}

	// Parameters: (p1: T1, p2: T2)
	lparen, err := p.expect(lonelexer.TokenLParen)
	if err != nil {
		return loneast.FN_DEC{}, err
	}
	_ = lparen

	var params []loneast.FnParam
	if p.peek().Kind != lonelexer.TokenRParen {
		for {
			// Check for 'self'
			if p.peek().Kind == lonelexer.TokenSelf {
				selfTok := p.advance()
				params = append(params, loneast.FnParam{
					Name: "self",
					Type: loneast.TYPE_REF{
						Kind:  loneast.TYPE_BASE,
						Value: &loneast.TypeBase{Name: "Self"},
						Span_: selfTok.Span(),
					},
					Span_: selfTok.Span(),
				})
			} else {
				pName, err := p.expect(lonelexer.TokenIdent)
				if err != nil {
					return loneast.FN_DEC{}, err
				}
				pColon, err := p.expect(lonelexer.TokenColon)
				if err != nil {
					return loneast.FN_DEC{}, err
				}
				_ = pColon
				pType, err := p.parseTypeRef()
				if err != nil {
					return loneast.FN_DEC{}, err
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

	rparen, err := p.expect(lonelexer.TokenRParen)
	if err != nil {
		return loneast.FN_DEC{}, err
	}
	_ = rparen

	// Optional return type
	var retType *loneast.TYPE_REF
	if p.peek().Kind != lonelexer.TokenLBrace && p.peek().Kind != lonelexer.TokenComma && p.peek().Kind != lonelexer.TokenSemicolon {
		rt, err := p.parseTypeRef()
		if err != nil {
			return loneast.FN_DEC{}, err
		}
		retType = &rt
	}

	// Body block
	body, err := p.parseBlock()
	if err != nil {
		return loneast.FN_DEC{}, err
	}

	return loneast.FN_DEC{
		Name:        nameTok.Text,
		IsPublic:    isPub,
		TypeParams:  typeParams,
		Constraints: constraints,
		Params:      params,
		ReturnType:  retType,
		Body:        body,
		Span_:       lonelexer.Span{Start: fnTok.Start, End: body.NodeSpan().End},
	}, nil
}

func (p *Parser) parseTypeParamsAndConstraints() ([]loneast.TypeParam, []loneast.GenericConstraint, error) {
	_, err := p.expect(lonelexer.TokenLt)
	if err != nil {
		return nil, nil, err
	}

	var typeParams []loneast.TypeParam
	var constraints []loneast.GenericConstraint

	for p.peek().Kind != lonelexer.TokenGt && p.peek().Kind != lonelexer.TokenWhere && !p.isAtEnd() {
		tp, err := p.expect(lonelexer.TokenIdent)
		if err != nil {
			return nil, nil, err
		}
		var defType *loneast.TYPE_REF
		tpEnd := tp.End
		if p.peek().Kind == lonelexer.TokenAssign {
			p.advance()
			def, err := p.parseTypeRef()
			if err != nil {
				return nil, nil, err
			}
			defType = &def
			tpEnd = def.NodeSpan().End
		}
		typeParams = append(typeParams, loneast.TypeParam{
			Name:    tp.Text,
			Default: defType,
			Span_:   lonelexer.Span{Start: tp.Start, End: tpEnd},
		})
		if p.peek().Kind == lonelexer.TokenComma {
			p.advance()
			if p.peek().Kind == lonelexer.TokenGt || p.peek().Kind == lonelexer.TokenWhere {
				break
			}
		} else {
			break
		}
	}

	if p.peek().Kind == lonelexer.TokenWhere {
		p.advance() // consume 'where'
		_, err := p.expect(lonelexer.TokenLBrace)
		if err != nil {
			return nil, nil, err
		}
		for p.peek().Kind != lonelexer.TokenRBrace && !p.isAtEnd() {
			paramTok, err := p.expect(lonelexer.TokenIdent)
			if err != nil {
				return nil, nil, err
			}
			op := loneast.ConstraintIs
			if p.peek().Kind == lonelexer.TokenIs {
				p.advance()
				op = loneast.ConstraintIs
			} else if p.peek().Kind == lonelexer.TokenSatisfies {
				p.advance()
				op = loneast.ConstraintSatisfies
			} else {
				return nil, nil, fmt.Errorf("expected 'is' or 'satisfies' in constraint at %s", p.peek().Start)
			}
			targetType, err := p.parseTypeRef()
			if err != nil {
				return nil, nil, err
			}
			constraints = append(constraints, loneast.GenericConstraint{
				TypeParam: paramTok.Text,
				Op:        op,
				Target:    targetType,
				Span_:     lonelexer.Span{Start: paramTok.Start, End: targetType.NodeSpan().End},
			})
			if p.peek().Kind == lonelexer.TokenComma {
				p.advance()
			}
		}
		_, err = p.expect(lonelexer.TokenRBrace)
		if err != nil {
			return nil, nil, err
		}
	}

	_, err = p.expect(lonelexer.TokenGt)
	if err != nil {
		return nil, nil, err
	}

	return typeParams, constraints, nil
}

func (p *Parser) parseTypeDec(isPub bool) (loneast.TYPE_DEC, error) {
	typeTok := p.advance() // consume 'type'

	nameTok, err := p.expect(lonelexer.TokenIdent)
	if err != nil {
		return loneast.TYPE_DEC{}, err
	}

	// Generic type parameters: <T, U = Default where { ... }>
	var typeParams []loneast.TypeParam
	var constraints []loneast.GenericConstraint
	if p.peek().Kind == lonelexer.TokenLt {
		tps, cs, err := p.parseTypeParamsAndConstraints()
		if err != nil {
			return loneast.TYPE_DEC{}, err
		}
		typeParams = tps
		constraints = cs
	}

	assignTok, err := p.expect(lonelexer.TokenAssign)
	if err != nil {
		return loneast.TYPE_DEC{}, err
	}
	_ = assignTok

	// Struct definition: type Name = struct { ... }
	if p.peek().Kind == lonelexer.TokenStruct {
		p.advance()
		lbrace, err := p.expect(lonelexer.TokenLBrace)
		if err != nil {
			return loneast.TYPE_DEC{}, err
		}
		_ = lbrace

		var fields []loneast.StructField
		var methods []loneast.FN_DEC

		for p.peek().Kind != lonelexer.TokenRBrace && !p.isAtEnd() {
			isMethodPub := false
			if p.peek().Kind == lonelexer.TokenPub {
				p.advance()
				isMethodPub = true
			}

			if p.peek().Kind == lonelexer.TokenFn {
				method, err := p.parseFnDec(isMethodPub)
				if err != nil {
					return loneast.TYPE_DEC{}, err
				}
				methods = append(methods, method)
			} else {
				fName, err := p.expect(lonelexer.TokenIdent)
				if err != nil {
					return loneast.TYPE_DEC{}, err
				}
				fColon, err := p.expect(lonelexer.TokenColon)
				if err != nil {
					return loneast.TYPE_DEC{}, err
				}
				_ = fColon
				fType, err := p.parseTypeRef()
				if err != nil {
					return loneast.TYPE_DEC{}, err
				}
				fields = append(fields, loneast.StructField{
					Name:  fName.Text,
					Type:  fType,
					Span_: lonelexer.Span{Start: fName.Start, End: fType.NodeSpan().End},
				})
				if p.peek().Kind == lonelexer.TokenComma {
					p.advance()
				}
			}
		}

		rbrace, err := p.expect(lonelexer.TokenRBrace)
		if err != nil {
			return loneast.TYPE_DEC{}, err
		}

		return loneast.TYPE_DEC{
			Name:        nameTok.Text,
			IsPublic:    isPub,
			TypeParams:  typeParams,
			Constraints: constraints,
			Kind:        loneast.TYPE_DEC_STRUCT,
			Fields:      fields,
			Methods:     methods,
			Span_:       lonelexer.Span{Start: typeTok.Start, End: rbrace.End},
		}, nil
	}

	// Trait definition: type TrName = trait { ... }
	if p.peek().Kind == lonelexer.TokenTrait {
		p.advance()
		lbrace, err := p.expect(lonelexer.TokenLBrace)
		if err != nil {
			return loneast.TYPE_DEC{}, err
		}
		_ = lbrace

		var traitSigs []loneast.TraitMethodSig
		for p.peek().Kind != lonelexer.TokenRBrace && !p.isAtEnd() {
			fnKw, err := p.expect(lonelexer.TokenFn)
			if err != nil {
				return loneast.TYPE_DEC{}, err
			}
			_ = fnKw
			mName, err := p.expect(lonelexer.TokenIdent)
			if err != nil {
				return loneast.TYPE_DEC{}, err
			}
			lp, err := p.expect(lonelexer.TokenLParen)
			if err != nil {
				return loneast.TYPE_DEC{}, err
			}
			_ = lp

			var sigParams []loneast.FnParam
			if p.peek().Kind != lonelexer.TokenRParen {
				for {
					if p.peek().Kind == lonelexer.TokenSelf {
						selfTok := p.advance()
						sigParams = append(sigParams, loneast.FnParam{
							Name: "self",
							Type: loneast.TYPE_REF{
								Kind:  loneast.TYPE_BASE,
								Value: &loneast.TypeBase{Name: "Self"},
								Span_: selfTok.Span(),
							},
							Span_: selfTok.Span(),
						})
					} else {
						pName, err := p.expect(lonelexer.TokenIdent)
						if err != nil {
							return loneast.TYPE_DEC{}, err
						}
						pColon, err := p.expect(lonelexer.TokenColon)
						if err != nil {
							return loneast.TYPE_DEC{}, err
						}
						_ = pColon
						pType, err := p.parseTypeRef()
						if err != nil {
							return loneast.TYPE_DEC{}, err
						}
						sigParams = append(sigParams, loneast.FnParam{
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
			rp, err := p.expect(lonelexer.TokenRParen)
			if err != nil {
				return loneast.TYPE_DEC{}, err
			}
			_ = rp

			var sigRet *loneast.TYPE_REF
			if p.peek().Kind != lonelexer.TokenComma && p.peek().Kind != lonelexer.TokenRBrace && p.peek().Kind != lonelexer.TokenFn {
				rt, err := p.parseTypeRef()
				if err != nil {
					return loneast.TYPE_DEC{}, err
				}
				sigRet = &rt
			}
			if p.peek().Kind == lonelexer.TokenComma {
				p.advance()
			}

			traitSigs = append(traitSigs, loneast.TraitMethodSig{
				Name:       mName.Text,
				Params:     sigParams,
				ReturnType: sigRet,
				Span_:      lonelexer.Span{Start: mName.Start, End: p.previous().End},
			})
		}

		rbrace, err := p.expect(lonelexer.TokenRBrace)
		if err != nil {
			return loneast.TYPE_DEC{}, err
		}

		return loneast.TYPE_DEC{
			Name:        nameTok.Text,
			IsPublic:    isPub,
			TypeParams:  typeParams,
			Constraints: constraints,
			Kind:        loneast.TYPE_DEC_TRAIT,
			TraitSigs:   traitSigs,
			Span_:       lonelexer.Span{Start: typeTok.Start, End: rbrace.End},
		}, nil
	}

	// Enum definition: type Name = enum { ... }
	if p.peek().Kind == lonelexer.TokenEnum {
		p.advance()
		lbrace, err := p.expect(lonelexer.TokenLBrace)
		if err != nil {
			return loneast.TYPE_DEC{}, err
		}
		_ = lbrace

		var variants []loneast.EnumVariant
		tag := 0
		for p.peek().Kind != lonelexer.TokenRBrace && !p.isAtEnd() {
			vName, err := p.expect(lonelexer.TokenIdent)
			if err != nil {
				return loneast.TYPE_DEC{}, err
			}
			var fields []loneast.StructField
			if p.peek().Kind == lonelexer.TokenLParen {
				p.advance()
				if p.peek().Kind != lonelexer.TokenRParen {
					for {
						if p.peek().Kind == lonelexer.TokenIdent && p.peekN(1).Kind == lonelexer.TokenColon {
							fId := p.advance()
							p.advance() // consume ':'
							fieldType, err := p.parseTypeRef()
							if err != nil {
								return loneast.TYPE_DEC{}, err
							}
							fields = append(fields, loneast.StructField{
								Name:  fId.Text,
								Type:  fieldType,
								Span_: lonelexer.Span{Start: fId.Start, End: fieldType.NodeSpan().End},
							})
						} else {
							fieldType, err := p.parseTypeRef()
							if err != nil {
								return loneast.TYPE_DEC{}, err
							}
							fields = append(fields, loneast.StructField{
								Name:  fmt.Sprintf("_%d", len(fields)),
								Type:  fieldType,
								Span_: fieldType.NodeSpan(),
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
				_, err = p.expect(lonelexer.TokenRParen)
				if err != nil {
					return loneast.TYPE_DEC{}, err
				}
			}

			variants = append(variants, loneast.EnumVariant{
				Name:   vName.Text,
				Fields: fields,
				Tag:    tag,
				Span_:  lonelexer.Span{Start: vName.Start, End: p.previous().End},
			})
			tag++

			if p.peek().Kind == lonelexer.TokenComma {
				p.advance()
			}
		}

		rbrace, err := p.expect(lonelexer.TokenRBrace)
		if err != nil {
			return loneast.TYPE_DEC{}, err
		}

		return loneast.TYPE_DEC{
			Name:        nameTok.Text,
			IsPublic:    isPub,
			TypeParams:  typeParams,
			Constraints: constraints,
			Kind:        loneast.TYPE_DEC_ENUM,
			Variants:    variants,
			Span_:       lonelexer.Span{Start: typeTok.Start, End: rbrace.End},
		}, nil
	}

	// Alias: type Name = TargetType
	aliasTarget, err := p.parseTypeRef()
	if err != nil {
		return loneast.TYPE_DEC{}, err
	}

	return loneast.TYPE_DEC{
		Name:        nameTok.Text,
		IsPublic:    isPub,
		TypeParams:  typeParams,
		Constraints: constraints,
		Kind:        loneast.TYPE_DEC_ALIAS,
		AliasType:   &aliasTarget,
		Span_:       lonelexer.Span{Start: typeTok.Start, End: aliasTarget.NodeSpan().End},
	}, nil
}

func (p *Parser) parseImplDec() (loneast.IMPL_DEC, error) {
	implTok := p.advance() // consume 'impl'

	trName, err := p.expect(lonelexer.TokenIdent)
	if err != nil {
		return loneast.IMPL_DEC{}, err
	}

	// Optional trait args: TrTrait<T>
	var trArgs []loneast.TYPE_REF
	if p.peek().Kind == lonelexer.TokenLt {
		p.advance()
		for {
			arg, err := p.parseTypeRef()
			if err != nil {
				return loneast.IMPL_DEC{}, err
			}
			trArgs = append(trArgs, arg)
			if p.peek().Kind == lonelexer.TokenComma {
				p.advance()
				if p.peek().Kind == lonelexer.TokenGt {
					break
				}
			} else {
				break
			}
		}
		gt, err := p.expect(lonelexer.TokenGt)
		if err != nil {
			return loneast.IMPL_DEC{}, err
		}
		_ = gt
	}

	// 'for' or 'on'
	if p.peek().Kind != lonelexer.TokenFor && p.peek().Kind != lonelexer.TokenOn {
		return loneast.IMPL_DEC{}, fmt.Errorf("expected 'for' or 'on' after trait name at %s", p.peek().Start)
	}
	p.advance()

	targetType, err := p.parseTypeRef()
	if err != nil {
		return loneast.IMPL_DEC{}, err
	}

	lbrace, err := p.expect(lonelexer.TokenLBrace)
	if err != nil {
		return loneast.IMPL_DEC{}, err
	}
	_ = lbrace

	var methods []loneast.FN_DEC
	for p.peek().Kind != lonelexer.TokenRBrace && !p.isAtEnd() {
		m, err := p.parseFnDec(false)
		if err != nil {
			return loneast.IMPL_DEC{}, err
		}
		methods = append(methods, m)
	}

	rbrace, err := p.expect(lonelexer.TokenRBrace)
	if err != nil {
		return loneast.IMPL_DEC{}, err
	}

	return loneast.IMPL_DEC{
		TraitName:  trName.Text,
		TraitArgs:  trArgs,
		TargetType: targetType,
		Methods:    methods,
		Span_:      lonelexer.Span{Start: implTok.Start, End: rbrace.End},
	}, nil
}

// =====================
// STATEMENTS
// =====================

func (p *Parser) parseBlock() (loneast.Block, error) {
	lbrace, err := p.expect(lonelexer.TokenLBrace)
	if err != nil {
		return loneast.Block{}, err
	}

	var stmts []any
	for p.peek().Kind != lonelexer.TokenRBrace && !p.isAtEnd() {
		stmt, err := p.parseStmt()
		if err != nil {
			return loneast.Block{}, err
		}
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}

	rbrace, err := p.expect(lonelexer.TokenRBrace)
	if err != nil {
		return loneast.Block{}, err
	}

	return loneast.Block{
		Stmts: stmts,
		Span_: lonelexer.Span{Start: lbrace.Start, End: rbrace.End},
	}, nil
}

func (p *Parser) parseStmt() (any, error) {
	tok := p.peek()

	switch tok.Kind {
	case lonelexer.TokenLet, lonelexer.TokenVar:
		return p.parseBindDec(false)

	case lonelexer.TokenReturn:
		retTok := p.advance()
		var retVal loneast.Expr
		if p.peek().Kind != lonelexer.TokenRBrace && p.peek().Kind != lonelexer.TokenSemicolon && !p.isAtEnd() {
			val, err := p.ParseExpr(PrecNone)
			if err != nil {
				return nil, err
			}
			retVal = val
		}
		end := retTok.End
		if retVal != nil {
			end = retVal.NodeSpan().End
		}
		return loneast.ReturnStmt{Value: retVal, Span_: lonelexer.Span{Start: retTok.Start, End: end}}, nil

	case lonelexer.TokenBreak:
		brkTok := p.advance()
		var label string
		var val loneast.Expr

		if p.peek().Kind == lonelexer.TokenIdent && p.peekN(1).Kind == lonelexer.TokenColon {
			labelTok := p.advance()
			p.advance() // consume ':'
			label = labelTok.Text
		}

		if p.peek().Kind != lonelexer.TokenRBrace && p.peek().Kind != lonelexer.TokenSemicolon && !p.isAtEnd() {
			v, err := p.ParseExpr(PrecNone)
			if err != nil {
				return nil, err
			}
			val = v
		}
		end := brkTok.End
		if val != nil {
			end = val.NodeSpan().End
		}
		return loneast.BreakStmt{Label: label, Value: val, Span_: lonelexer.Span{Start: brkTok.Start, End: end}}, nil

	case lonelexer.TokenDrop:
		dropTok := p.advance()
		expr, err := p.ParseExpr(PrecNone)
		if err != nil {
			return nil, err
		}
		return loneast.DropStmt{Expr: expr, Span_: lonelexer.Span{Start: dropTok.Start, End: expr.NodeSpan().End}}, nil

	case lonelexer.TokenIf:
		return p.parseIfStmt()

	case lonelexer.TokenWhile:
		return p.parseWhileStmt()

	case lonelexer.TokenLoop:
		return p.parseLoopStmt()

	case lonelexer.TokenFor:
		return p.parseForStmt()

	case lonelexer.TokenLBrace:
		return p.parseBlock()

	default:
		// Expression or assignment or labeled block: label: { ... }
		return p.parseAssignOrExprStmt()
	}
}

// parseIfStmt handles:
// 1. Standard if: if cond { body } else { ... }
// 2. Scoped header if: if: [let x = 0] x != 9 { body }
// 3. Packed if: if { cond => expr, else => expr }
// 4. Scoped header packed if: if: [let x = 0, let y = 7] { ... }
func (p *Parser) parseIfStmt() (loneast.Expr, error) {
	ifTok := p.advance() // consume 'if'

	var headerBindings []loneast.BIND_DEC

	// Check for scoped header clause: if: [let x = 0, ...]
	if p.peek().Kind == lonelexer.TokenColon {
		p.advance() // consume ':'
		lbrack, err := p.expect(lonelexer.TokenLBracket)
		if err != nil {
			return nil, err
		}
		_ = lbrack

		for p.peek().Kind != lonelexer.TokenRBracket && !p.isAtEnd() {
			bind, err := p.parseBindDec(false)
			if err != nil {
				return nil, err
			}
			headerBindings = append(headerBindings, bind)
			if p.peek().Kind == lonelexer.TokenComma {
				p.advance()
				if p.peek().Kind == lonelexer.TokenRBracket {
					break
				}
			} else {
				break
			}
		}

		rbrack, err := p.expect(lonelexer.TokenRBracket)
		if err != nil {
			return nil, err
		}
		_ = rbrack
	}

	// Check if this is a packed if statement: if { ... } or if: [...] { ... }
	if p.peek().Kind == lonelexer.TokenLBrace && p.isPackedIfBrace() {
		p.advance() // consume '{'
		var arms []loneast.PackedIfArm

		for p.peek().Kind != lonelexer.TokenRBrace && !p.isAtEnd() {
			if p.peek().Kind == lonelexer.TokenElse {
				elseTok := p.advance()
				arrow, err := p.expect(lonelexer.TokenFatArrow)
				if err != nil {
					return nil, err
				}
				_ = arrow
				var body any
				if p.peek().Kind == lonelexer.TokenLBrace {
					blk, err := p.parseBlock()
					if err != nil {
						return nil, err
					}
					body = blk
				} else {
					stmt, err := p.parseStmt()
					if err != nil {
						return nil, err
					}
					body = stmt
				}
				arms = append(arms, loneast.PackedIfArm{
					Cond:  nil,
					Body:  body,
					Span_: lonelexer.Span{Start: elseTok.Start, End: p.previous().End},
				})
			} else {
				armCond, err := p.ParseExpr(PrecNone)
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
					blk, err := p.parseBlock()
					if err != nil {
						return nil, err
					}
					body = blk
				} else {
					stmt, err := p.parseStmt()
					if err != nil {
						return nil, err
					}
					body = stmt
				}
				arms = append(arms, loneast.PackedIfArm{
					Cond:  armCond,
					Body:  body,
					Span_: lonelexer.Span{Start: armCond.NodeSpan().Start, End: p.previous().End},
				})
			}

			if p.peek().Kind == lonelexer.TokenComma || p.peek().Kind == lonelexer.TokenSemicolon {
				p.advance()
			}
		}

		rbrace, err := p.expect(lonelexer.TokenRBrace)
		if err != nil {
			return nil, err
		}

		return loneast.PackedIfStmt{
			Header: headerBindings,
			Arms:   arms,
			Span_:  lonelexer.Span{Start: ifTok.Start, End: rbrace.End},
		}, nil
	}

	// Standard if statement (with optional explicit condition)
	var cond loneast.Expr
	if p.peek().Kind != lonelexer.TokenLBrace {
		c, err := p.ParseExpr(PrecNone)
		if err != nil {
			return nil, err
		}
		cond = c
	}

	thenBlock, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	var elseBranch any
	if p.peek().Kind == lonelexer.TokenElse {
		p.advance() // consume 'else'
		if p.peek().Kind == lonelexer.TokenIf {
			nestedIf, err := p.parseIfStmt()
			if err != nil {
				return nil, err
			}
			elseBranch = nestedIf
		} else {
			elseBlock, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			elseBranch = elseBlock
		}
	}

	endPos := thenBlock.NodeSpan().End
	if elseBranch != nil {
		if node, ok := elseBranch.(loneast.Node); ok {
			endPos = node.NodeSpan().End
		}
	}

	return loneast.IfStmt{
		Header: headerBindings,
		Cond:   cond,
		Then:   thenBlock,
		Else:   elseBranch,
		Span_:  lonelexer.Span{Start: ifTok.Start, End: endPos},
	}, nil
}

func (p *Parser) isPackedIfBrace() bool {
	depth := 0
	for i := 1; i < 50; i++ {
		tok := p.peekN(i)
		if tok.Kind == lonelexer.TokenEOF {
			break
		}
		if tok.Kind == lonelexer.TokenLBrace || tok.Kind == lonelexer.TokenLParen || tok.Kind == lonelexer.TokenLBracket {
			depth++
		} else if tok.Kind == lonelexer.TokenRBrace || tok.Kind == lonelexer.TokenRParen || tok.Kind == lonelexer.TokenRBracket {
			if depth == 0 {
				break
			}
			depth--
		} else if tok.Kind == lonelexer.TokenFatArrow && depth == 0 {
			return true
		}
	}
	return false
}

func (p *Parser) parseWhileStmt() (loneast.WhileStmt, error) {
	whileTok := p.advance() // consume 'while'
	cond, err := p.ParseExpr(PrecNone)
	if err != nil {
		return loneast.WhileStmt{}, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return loneast.WhileStmt{}, err
	}
	return loneast.WhileStmt{
		Cond:  cond,
		Body:  body,
		Span_: lonelexer.Span{Start: whileTok.Start, End: body.NodeSpan().End},
	}, nil
}

func (p *Parser) parseLoopStmt() (loneast.LoopStmt, error) {
	loopTok := p.advance() // consume 'loop'
	body, err := p.parseBlock()
	if err != nil {
		return loneast.LoopStmt{}, err
	}

	// Optional post-condition: while: [cond]
	var whileCond loneast.Expr
	endPos := body.NodeSpan().End

	if p.peek().Kind == lonelexer.TokenWhile && p.peekN(1).Kind == lonelexer.TokenColon {
		p.advance() // consume 'while'
		p.advance() // consume ':'
		lbrack, err := p.expect(lonelexer.TokenLBracket)
		if err != nil {
			return loneast.LoopStmt{}, err
		}
		_ = lbrack
		cond, err := p.ParseExpr(PrecNone)
		if err != nil {
			return loneast.LoopStmt{}, err
		}
		whileCond = cond
		rbrack, err := p.expect(lonelexer.TokenRBracket)
		if err != nil {
			return loneast.LoopStmt{}, err
		}
		endPos = rbrack.End
	}

	return loneast.LoopStmt{
		Body:      body,
		WhileCond: whileCond,
		Span_:     lonelexer.Span{Start: loopTok.Start, End: endPos},
	}, nil
}

func (p *Parser) parseForStmt() (loneast.ForStmt, error) {
	forTok := p.advance() // consume 'for'
	vName, err := p.expect(lonelexer.TokenIdent)
	if err != nil {
		return loneast.ForStmt{}, err
	}
	inTok, err := p.expect(lonelexer.TokenIn)
	if err != nil {
		return loneast.ForStmt{}, err
	}
	_ = inTok
	iterExpr, err := p.ParseExpr(PrecNone)
	if err != nil {
		return loneast.ForStmt{}, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return loneast.ForStmt{}, err
	}

	return loneast.ForStmt{
		VarName: vName.Text,
		Iter:    iterExpr,
		Body:    body,
		Span_:   lonelexer.Span{Start: forTok.Start, End: body.NodeSpan().End},
	}, nil
}

func (p *Parser) parseAssignOrExprStmt() (any, error) {
	// Check for labeled block: label: { ... }
	if p.peek().Kind == lonelexer.TokenIdent && p.peekN(1).Kind == lonelexer.TokenColon && p.peekN(2).Kind == lonelexer.TokenLBrace {
		labelTok := p.advance()
		p.advance() // consume ':'
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		return loneast.LabelDec{
			Name:  labelTok.Text,
			Body:  body,
			Span_: lonelexer.Span{Start: labelTok.Start, End: body.NodeSpan().End},
		}, nil
	}

	expr, err := p.ParseExpr(PrecNone)
	if err != nil {
		return nil, err
	}

	// If followed by '=', this is an assignment statement
	if p.peek().Kind == lonelexer.TokenAssign {
		assignTok := p.advance()
		_ = assignTok
		val, err := p.ParseExpr(PrecNone)
		if err != nil {
			return nil, err
		}
		return loneast.AssignStmt{
			Target: expr,
			Value:  val,
			Span_:  lonelexer.Span{Start: expr.NodeSpan().Start, End: val.NodeSpan().End},
		}, nil
	}

	return expr, nil
}

// =====================
// TYPE REFERENCES
// =====================

func (p *Parser) parseTypeRef() (loneast.TYPE_REF, error) {
	startPos := p.peek().Start

	// Constraints: ro, ?, comptime
	var constrain loneast.TypeConstrain
	for {
		if p.peek().Kind == lonelexer.TokenRo {
			p.advance()
			constrain.ReadOnly = true
		} else if p.peek().Kind == lonelexer.TokenQuestion {
			p.advance()
			constrain.Optional = true
		} else if p.peek().Kind == lonelexer.TokenComptime {
			p.advance()
			constrain.Comptime = true
		} else {
			break
		}
	}

	// List type: [N]T or []T
	if p.peek().Kind == lonelexer.TokenLBracket {
		p.advance() // consume '['
		var sizeExpr loneast.Expr
		if p.peek().Kind != lonelexer.TokenRBracket {
			sz, err := p.ParseExpr(PrecNone)
			if err != nil {
				return loneast.TYPE_REF{}, err
			}
			sizeExpr = sz
		}
		rbrack, err := p.expect(lonelexer.TokenRBracket)
		if err != nil {
			return loneast.TYPE_REF{}, err
		}
		_ = rbrack

		elemType, err := p.parseTypeRef()
		if err != nil {
			return loneast.TYPE_REF{}, err
		}

		return loneast.TYPE_REF{
			Kind:      loneast.TYPE_LIST,
			Constrain: constrain,
			Value: &loneast.TypeList{
				Size:     sizeExpr,
				ItemType: elemType,
			},
			Span_: lonelexer.Span{Start: startPos, End: elemType.NodeSpan().End},
		}, nil
	}

	// Function type: fn (p1: T1) T2
	if p.peek().Kind == lonelexer.TokenFn {
		fnTok := p.advance()
		lp, err := p.expect(lonelexer.TokenLParen)
		if err != nil {
			return loneast.TYPE_REF{}, err
		}
		_ = lp

		var fnParams []loneast.FnParam
		if p.peek().Kind != lonelexer.TokenRParen {
			for {
				pName, err := p.expect(lonelexer.TokenIdent)
				if err != nil {
					return loneast.TYPE_REF{}, err
				}
				pColon, err := p.expect(lonelexer.TokenColon)
				if err != nil {
					return loneast.TYPE_REF{}, err
				}
				_ = pColon
				pType, err := p.parseTypeRef()
				if err != nil {
					return loneast.TYPE_REF{}, err
				}
				fnParams = append(fnParams, loneast.FnParam{
					Name:  pName.Text,
					Type:  pType,
					Span_: lonelexer.Span{Start: pName.Start, End: pType.NodeSpan().End},
				})
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
			return loneast.TYPE_REF{}, err
		}
		_ = rp

		var fnRet *loneast.TYPE_REF
		endPos := rp.End
		if isTypeStart(p.peek().Kind) {
			rt, err := p.parseTypeRef()
			if err != nil {
				return loneast.TYPE_REF{}, err
			}
			fnRet = &rt
			endPos = rt.NodeSpan().End
		}

		return loneast.TYPE_REF{
			Kind:      loneast.TYPE_FN,
			Constrain: constrain,
			Value: &loneast.TypeFn{
				Params:     fnParams,
				ReturnType: fnRet,
			},
			Span_: lonelexer.Span{Start: fnTok.Start, End: endPos},
		}, nil
	}

	// Base type: Name<T1, T2> (includes ptr<T>, ref<T>, i32, str, TrWriter, infer, etc.)
	var nameTok lonelexer.Token
	if p.peek().Kind == lonelexer.TokenIdent || p.peek().Kind == lonelexer.TokenNone || p.peek().Kind == lonelexer.TokenInfer {
		nameTok = p.advance()
	} else {
		return loneast.TYPE_REF{}, fmt.Errorf("expected type name at %s, got %s", p.peek().Start, p.peek().Kind)
	}

	var typeArgs []loneast.TYPE_REF
	endPos := nameTok.End

	var variantName string
	if p.peek().Kind == lonelexer.TokenColon2 {
		p.advance() // consume '::'
		vTok, err := p.expect(lonelexer.TokenIdent)
		if err != nil {
			return loneast.TYPE_REF{}, err
		}
		variantName = vTok.Text
		endPos = vTok.End
	}

	if p.peek().Kind == lonelexer.TokenLt {
		p.advance()
		for {
			arg, err := p.parseTypeRef()
			if err != nil {
				return loneast.TYPE_REF{}, err
			}
			typeArgs = append(typeArgs, arg)
			if p.peek().Kind == lonelexer.TokenComma {
				p.advance()
				if p.peek().Kind == lonelexer.TokenGt {
					break
				}
			} else {
				break
			}
		}
		gt, err := p.expect(lonelexer.TokenGt)
		if err != nil {
			return loneast.TYPE_REF{}, err
		}
		endPos = gt.End
	}

	return loneast.TYPE_REF{
		Kind:      loneast.TYPE_BASE,
		Constrain: constrain,
		Value: &loneast.TypeBase{
			Name:    nameTok.Text,
			Variant: variantName,
			Args:    typeArgs,
		},
		Span_: lonelexer.Span{Start: startPos, End: endPos},
	}, nil
}

// =====================
// TOKEN HELPERS
// =====================

func (p *Parser) isAtEnd() bool {
	return p.cursor >= len(p.tokens) || p.tokens[p.cursor].Kind == lonelexer.TokenEOF
}

func (p *Parser) peek() lonelexer.Token {
	if p.cursor >= len(p.tokens) {
		return lonelexer.Token{Kind: lonelexer.TokenEOF}
	}
	return p.tokens[p.cursor]
}

func (p *Parser) peekN(n int) lonelexer.Token {
	idx := p.cursor + n
	if idx >= len(p.tokens) {
		return lonelexer.Token{Kind: lonelexer.TokenEOF}
	}
	return p.tokens[idx]
}

func (p *Parser) advance() lonelexer.Token {
	tok := p.peek()
	if !p.isAtEnd() {
		p.cursor++
	}
	return tok
}

func (p *Parser) previous() lonelexer.Token {
	if p.cursor == 0 {
		return lonelexer.Token{Kind: lonelexer.TokenEOF}
	}
	return p.tokens[p.cursor-1]
}

func (p *Parser) expect(kind lonelexer.TokenKind) (lonelexer.Token, error) {
	tok := p.peek()
	if tok.Kind != kind {
		return tok, fmt.Errorf("expected %s at %s, got %s (%q)", kind, tok.Start, tok.Kind, tok.Text)
	}
	return p.advance(), nil
}

func isTypeStart(k lonelexer.TokenKind) bool {
	return k == lonelexer.TokenIdent ||
		k == lonelexer.TokenNone ||
		k == lonelexer.TokenInfer ||
		k == lonelexer.TokenLBracket ||
		k == lonelexer.TokenFn ||
		k == lonelexer.TokenQuestion ||
		k == lonelexer.TokenRo ||
		k == lonelexer.TokenComptime
}
