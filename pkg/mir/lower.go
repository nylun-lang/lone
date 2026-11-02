package mir

import (
	"fmt"
	"lone/pkg/ast"
	"lone/pkg/sema"
)

// LowerProgram translates a Typed IR program from lone-alyzer into a Falone MirModule.
func LowerProgram(prog *sema.TProgram) (*MirModule, error) {
	if prog == nil {
		return &MirModule{
			Functions: make([]*MirFunction, 0),
			Globals:   make(map[string]MirT),
		}, nil
	}

	lowerer := newLowerer()
	return lowerer.lower(prog)
}

type lowerer struct {
	mod           *MirModule
	fn            *MirFunction
	curBlk        *MirBlock
	regId         int
	blockId       int
	vars          map[string]string // maps variable name to register name
	loopExitStack []string          // tracks enclosing loop exit block labels
	dataOffset    int               // tracks linear memory allocation offset
	stringOffsets map[string]int    // deduplicates string literals to data offsets
	stringVarLens map[string]int    // tracks known string length for variables
}

func newLowerer() *lowerer {
	return &lowerer{
		mod: &MirModule{
			Functions:    make([]*MirFunction, 0),
			Globals:      make(map[string]MirT),
			DataSegments: make([]MirDataSegment, 0),
		},
		vars:          make(map[string]string),
		loopExitStack: make([]string, 0),
		dataOffset:    16, // offsets 0..15 reserved for WASI ciovec_t scratchpad
		stringOffsets: make(map[string]int),
		stringVarLens: make(map[string]int),
	}
}

func (l *lowerer) allocString(val string) int {
	if offset, exists := l.stringOffsets[val]; exists {
		return offset
	}
	offset := l.dataOffset
	l.stringOffsets[val] = offset
	l.mod.DataSegments = append(l.mod.DataSegments, MirDataSegment{
		Offset: offset,
		Value:  val,
	})
	// Advance offset by string byte length + 1 (null terminator) and align to 4 bytes
	l.dataOffset += len(val) + 1
	l.dataOffset = (l.dataOffset + 3) & ^3
	return offset
}

func (l *lowerer) freshReg(prefix string, typ MirT) string {
	l.regId++
	name := fmt.Sprintf("%s%d", prefix, l.regId)
	l.fn.AddLocal(name, typ)
	return name
}

func (l *lowerer) freshBlock(name string) *MirBlock {
	l.blockId++
	blkName := fmt.Sprintf("%s_%d", name, l.blockId)
	return l.fn.AddBlock(blkName)
}

func (l *lowerer) lower(prog *sema.TProgram) (*MirModule, error) {
	// First pass: collect top-level declarations
	for _, decl := range prog.Decls {
		switch d := decl.(type) {
		case *sema.TVarDec:
			mType := toMirType(d.Type)
			l.mod.Globals[d.Name] = mType
		case *sema.TFnDec:
			fn, err := l.lowerFunction(d)
			if err != nil {
				return nil, err
			}
			l.mod.Functions = append(l.mod.Functions, fn)
		}
	}
	return l.mod, nil
}

func (l *lowerer) lowerFunction(d *sema.TFnDec) (*MirFunction, error) {
	l.fn = &MirFunction{
		Name:       d.Name,
		Params:     make([]MirParam, 0, len(d.Params)),
		ReturnType: toMirType(d.ReturnType),
		Blocks:     make([]*MirBlock, 0),
		Locals:     make(map[string]MirT),
	}
	l.vars = make(map[string]string)
	l.regId = 0
	l.blockId = 0

	// Add params
	for _, p := range d.Params {
		pType := toMirType(p.Type)
		pReg := p.Name
		l.fn.Params = append(l.fn.Params, MirParam{
			Name: pReg,
			Type: pType,
		})
		l.vars[p.Name] = pReg
	}

	// Create entry block
	l.curBlk = l.fn.AddBlock("entry")

	// Lower body
	l.lowerBlock(&d.Body)

	// Ensure entry block or final block has a terminator
	if len(l.fn.Blocks) > 0 {
		lastBlk := l.fn.Blocks[len(l.fn.Blocks)-1]
		if lastBlk.Terminator == nil {
			if l.fn.ReturnType == MirVoid {
				lastBlk.Terminator = MirReturn{}
			} else {
				// Return default zero
				lastBlk.Terminator = MirReturn{
					Value: MirConst{Type: l.fn.ReturnType, Value: 0},
				}
			}
		}
	}

	return l.fn, nil
}

func (l *lowerer) lowerBlock(b *sema.TBlock) {
	for _, stmt := range b.Stmts {
		l.lowerStmt(stmt)
	}
}

func (l *lowerer) lowerStmt(stmt sema.TStmt) {
	if l.curBlk == nil {
		return
	}

	switch s := stmt.(type) {
	case *sema.TVarDec:
		valOp := l.lowerExpr(s.Value)
		mType := toMirType(s.Type)
		reg := l.freshReg("v_"+s.Name, mType)
		l.vars[s.Name] = reg

		// Track string variable lengths for intrinsics like #print
		if lit, ok := s.Value.(*sema.TLiteralExpr); ok {
			if strVal, ok := lit.Value.(string); ok {
				l.stringVarLens[s.Name] = len(strVal)
			}
		} else if ref, ok := s.Value.(*sema.TVarRefExpr); ok {
			if strLen, ok := l.stringVarLens[ref.Name]; ok {
				l.stringVarLens[s.Name] = strLen
			}
		}

		l.curBlk.Insts = append(l.curBlk.Insts, MirCopyInst{
			Dest: reg,
			Src:  valOp,
		})

	case *sema.TReturnStmt:
		var retOp MirOperand
		if s.Value != nil {
			retOp = l.lowerExpr(s.Value)
		}
		l.curBlk.Terminator = MirReturn{Value: retOp}
		// Any code following return in same block will need a fresh dead block or gets omitted
		l.curBlk = nil

	case *sema.TDropStmt:
		// Evaluate for side-effects, discard result
		l.lowerExpr(s.Expr)

	case *sema.TExprStmt:
		l.lowerExpr(s.Expr)

	case *sema.TIfStmt:
		condOp := l.lowerExpr(s.Cond)
		thenBlk := l.freshBlock("if_then")
		joinBlk := l.freshBlock("if_join")

		elseTarget := joinBlk.Name
		var elseBlk *MirBlock
		if s.Else != nil && len(s.Else.Stmts) > 0 {
			elseBlk = l.freshBlock("if_else")
			elseTarget = elseBlk.Name
		}

		l.curBlk.Terminator = MirBranch{
			Cond:       condOp,
			ThenTarget: thenBlk.Name,
			ElseTarget: elseTarget,
		}

		// Lower then block
		l.curBlk = thenBlk
		l.lowerBlock(&s.Then)
		if l.curBlk != nil && l.curBlk.Terminator == nil {
			l.curBlk.Terminator = MirJump{Target: joinBlk.Name}
		}

		// Lower else block if exists
		if elseBlk != nil {
			l.curBlk = elseBlk
			l.lowerBlock(s.Else)
			if l.curBlk != nil && l.curBlk.Terminator == nil {
				l.curBlk.Terminator = MirJump{Target: joinBlk.Name}
			}
		}

		l.curBlk = joinBlk

	case *sema.TWhileStmt:
		headBlk := l.freshBlock("while_head")
		headBlk.IsLoop = true
		bodyBlk := l.freshBlock("while_body")
		exitBlk := l.freshBlock("while_exit")
		headBlk.LoopExit = exitBlk.Name

		l.curBlk.Terminator = MirJump{Target: headBlk.Name}

		// Header: evaluate condition
		l.curBlk = headBlk
		condOp := l.lowerExpr(s.Cond)
		l.curBlk.Terminator = MirBranch{
			Cond:       condOp,
			ThenTarget: bodyBlk.Name,
			ElseTarget: exitBlk.Name,
		}

		// Track loop exit for breaks
		l.loopExitStack = append(l.loopExitStack, exitBlk.Name)

		// Body
		l.curBlk = bodyBlk
		l.lowerBlock(&s.Body)
		if l.curBlk != nil && l.curBlk.Terminator == nil {
			l.curBlk.Terminator = MirJump{Target: headBlk.Name}
		}

		l.loopExitStack = l.loopExitStack[:len(l.loopExitStack)-1]

		l.curBlk = exitBlk

	case *sema.TBreakStmt:
		if len(l.loopExitStack) > 0 {
			target := l.loopExitStack[len(l.loopExitStack)-1]
			l.curBlk.Terminator = MirJump{Target: target}
		} else {
			l.curBlk.Terminator = MirReturn{}
		}
		l.curBlk = nil
	}
}

func (l *lowerer) lowerExpr(expr sema.TExpr) MirOperand {
	if expr == nil {
		return MirConst{Type: MirI32, Value: 0}
	}

	mType := toMirType(expr.ExprType())

	switch e := expr.(type) {
	case *sema.TLiteralExpr:
		if strVal, ok := e.Value.(string); ok && e.Kind == ast.LITERAL_STRING {
			offset := l.allocString(strVal)
			return MirConst{
				Type:  MirI32,
				Value: offset,
			}
		}
		return MirConst{
			Type:  mType,
			Value: e.Value,
		}

	case *sema.TVarRefExpr:
		if reg, ok := l.vars[e.Name]; ok {
			return MirReg{Name: reg}
		}
		if _, ok := l.mod.Globals[e.Name]; ok {
			return MirGlobal{Name: e.Name}
		}
		return MirReg{Name: e.Name}

	case *sema.TBinaryExpr:
		leftOp := l.lowerExpr(e.Left)
		rightOp := l.lowerExpr(e.Right)
		destReg := l.freshReg("t_bin", mType)

		switch string(e.Op) {
		case "+":
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "add", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "-":
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "sub", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "*":
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "mul", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "/":
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "div", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "<":
			l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
				Op: "lt", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "<=":
			l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
				Op: "le", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case ">":
			l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
				Op: "gt", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case ">=":
			l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
				Op: "ge", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "==":
			l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
				Op: "eq", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "!=":
			l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
				Op: "ne", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		default:
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "add", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		}
		return MirReg{Name: destReg}

	case *sema.TUnaryExpr:
		subOp := l.lowerExpr(e.Expr)
		destReg := l.freshReg("t_un", mType)

		if e.Op == ast.UNARY_OP_NEGATIVE {
			l.curBlk.Insts = append(l.curBlk.Insts, MirBinOp{
				Op: "sub", Type: mType, Dest: destReg,
				Left:  MirConst{Type: mType, Value: 0},
				Right: subOp,
			})
		} else if e.Op == ast.UNARY_OP_NOT {
			l.curBlk.Insts = append(l.curBlk.Insts, MirCmpOp{
				Op: "eq", Type: mType, Dest: destReg,
				Left:  subOp,
				Right: MirConst{Type: mType, Value: 0},
			})
		} else {
			return subOp
		}
		return MirReg{Name: destReg}

	case *sema.TCallExpr:
		calleeName := "unknown"
		if ref, ok := e.Callee.(*sema.TVarRefExpr); ok {
			calleeName = ref.Name
		}

		// Handle built-in intrinsics: #print
		if calleeName == "#print" {
			var ptrOp MirOperand = MirConst{Type: MirI32, Value: 0}
			var lenOp MirOperand = MirConst{Type: MirI32, Value: 0}

			if len(e.Args) > 0 {
				arg := e.Args[0]
				if lit, ok := arg.(*sema.TLiteralExpr); ok {
					if strVal, ok := lit.Value.(string); ok {
						offset := l.allocString(strVal)
						ptrOp = MirConst{Type: MirI32, Value: offset}
						lenOp = MirConst{Type: MirI32, Value: len(strVal)}
					} else {
						ptrOp = l.lowerExpr(arg)
					}
				} else if ref, ok := arg.(*sema.TVarRefExpr); ok {
					ptrOp = l.lowerExpr(arg)
					if strLen, ok := l.stringVarLens[ref.Name]; ok {
						lenOp = MirConst{Type: MirI32, Value: strLen}
					}
				} else {
					ptrOp = l.lowerExpr(arg)
				}
			}

			l.curBlk.Insts = append(l.curBlk.Insts, MirCallInst{
				Dest:   "",
				Callee: "__lone_rt_print",
				Args:   []MirOperand{ptrOp, lenOp},
			})
			return MirConst{Type: MirVoid, Value: 0}
		}

		var args []MirOperand
		for _, arg := range e.Args {
			args = append(args, l.lowerExpr(arg))
		}

		var destReg string
		if mType != MirVoid {
			destReg = l.freshReg("t_call", mType)
		}

		l.curBlk.Insts = append(l.curBlk.Insts, MirCallInst{
			Dest:   destReg,
			Callee: calleeName,
			Args:   args,
		})

		if destReg != "" {
			return MirReg{Name: destReg}
		}
		return MirConst{Type: MirVoid, Value: 0}

	case *sema.TDropStmt:
		return l.lowerExpr(e.Expr)

	default:
		return MirConst{Type: MirI32, Value: 0}
	}
}

func toMirType(t sema.Type) MirT {
	if t == nil {
		return MirI32
	}
	switch t.String() {
	case "i8", "i16", "i32", "u8", "u16", "u32", "bool":
		return MirI32
	case "i64", "u64", "i128", "u128":
		return MirI64
	case "f32":
		return MirF32
	case "f64":
		return MirF64
	case "void":
		return MirVoid
	default:
		return MirI32
	}
}
