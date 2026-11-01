package lowering

import (
	"falone/mirtypes"
	"fmt"
	air "lonealyzer/irtypes"
	ir "lonelang/irtypes"
)

// LowerProgram translates a Typed IR program from lone-alyzer into a Falone MirModule.
func LowerProgram(prog *air.TProgram) (*mirtypes.MirModule, error) {
	if prog == nil {
		return &mirtypes.MirModule{
			Functions: make([]*mirtypes.MirFunction, 0),
			Globals:   make(map[string]mirtypes.MirT),
		}, nil
	}

	lowerer := newLowerer()
	return lowerer.lower(prog)
}

type lowerer struct {
	mod           *mirtypes.MirModule
	fn            *mirtypes.MirFunction
	curBlk        *mirtypes.MirBlock
	regId         int
	blockId       int
	vars          map[string]string // maps variable name to register name
	loopExitStack []string          // tracks enclosing loop exit block labels
}

func newLowerer() *lowerer {
	return &lowerer{
		mod: &mirtypes.MirModule{
			Functions: make([]*mirtypes.MirFunction, 0),
			Globals:   make(map[string]mirtypes.MirT),
		},
		vars:          make(map[string]string),
		loopExitStack: make([]string, 0),
	}
}

func (l *lowerer) freshReg(prefix string, typ mirtypes.MirT) string {
	l.regId++
	name := fmt.Sprintf("%s%d", prefix, l.regId)
	l.fn.AddLocal(name, typ)
	return name
}

func (l *lowerer) freshBlock(name string) *mirtypes.MirBlock {
	l.blockId++
	blkName := fmt.Sprintf("%s_%d", name, l.blockId)
	return l.fn.AddBlock(blkName)
}

func (l *lowerer) lower(prog *air.TProgram) (*mirtypes.MirModule, error) {
	// First pass: collect top-level declarations
	for _, decl := range prog.Decls {
		switch d := decl.(type) {
		case *air.TVarDec:
			mType := toMirType(d.Type)
			l.mod.Globals[d.Name] = mType
		case *air.TFnDec:
			fn, err := l.lowerFunction(d)
			if err != nil {
				return nil, err
			}
			l.mod.Functions = append(l.mod.Functions, fn)
		}
	}
	return l.mod, nil
}

func (l *lowerer) lowerFunction(d *air.TFnDec) (*mirtypes.MirFunction, error) {
	l.fn = &mirtypes.MirFunction{
		Name:       d.Name,
		Params:     make([]mirtypes.MirParam, 0, len(d.Params)),
		ReturnType: toMirType(d.ReturnType),
		Blocks:     make([]*mirtypes.MirBlock, 0),
		Locals:     make(map[string]mirtypes.MirT),
	}
	l.vars = make(map[string]string)
	l.regId = 0
	l.blockId = 0

	// Add params
	for _, p := range d.Params {
		pType := toMirType(p.Type)
		pReg := p.Name
		l.fn.Params = append(l.fn.Params, mirtypes.MirParam{
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
			if l.fn.ReturnType == mirtypes.MirVoid {
				lastBlk.Terminator = mirtypes.MirReturn{}
			} else {
				// Return default zero
				lastBlk.Terminator = mirtypes.MirReturn{
					Value: mirtypes.MirConst{Type: l.fn.ReturnType, Value: 0},
				}
			}
		}
	}

	return l.fn, nil
}

func (l *lowerer) lowerBlock(b *air.TBlock) {
	for _, stmt := range b.Stmts {
		l.lowerStmt(stmt)
	}
}

func (l *lowerer) lowerStmt(stmt air.TStmt) {
	if l.curBlk == nil {
		return
	}

	switch s := stmt.(type) {
	case *air.TVarDec:
		valOp := l.lowerExpr(s.Value)
		mType := toMirType(s.Type)
		reg := l.freshReg("v_"+s.Name, mType)
		l.vars[s.Name] = reg

		l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirCopyInst{
			Dest: reg,
			Src:  valOp,
		})

	case *air.TReturnStmt:
		var retOp mirtypes.MirOperand
		if s.Value != nil {
			retOp = l.lowerExpr(s.Value)
		}
		l.curBlk.Terminator = mirtypes.MirReturn{Value: retOp}
		// Any code following return in same block will need a fresh dead block or gets omitted
		l.curBlk = nil

	case *air.TDropStmt:
		// Evaluate for side-effects, discard result
		l.lowerExpr(s.Expr)

	case *air.TExprStmt:
		l.lowerExpr(s.Expr)

	case *air.TIfStmt:
		condOp := l.lowerExpr(s.Cond)
		thenBlk := l.freshBlock("if_then")
		joinBlk := l.freshBlock("if_join")

		elseTarget := joinBlk.Name
		var elseBlk *mirtypes.MirBlock
		if s.Else != nil && len(s.Else.Stmts) > 0 {
			elseBlk = l.freshBlock("if_else")
			elseTarget = elseBlk.Name
		}

		l.curBlk.Terminator = mirtypes.MirBranch{
			Cond:       condOp,
			ThenTarget: thenBlk.Name,
			ElseTarget: elseTarget,
		}

		// Lower then block
		l.curBlk = thenBlk
		l.lowerBlock(&s.Then)
		if l.curBlk != nil && l.curBlk.Terminator == nil {
			l.curBlk.Terminator = mirtypes.MirJump{Target: joinBlk.Name}
		}

		// Lower else block if exists
		if elseBlk != nil {
			l.curBlk = elseBlk
			l.lowerBlock(s.Else)
			if l.curBlk != nil && l.curBlk.Terminator == nil {
				l.curBlk.Terminator = mirtypes.MirJump{Target: joinBlk.Name}
			}
		}

		l.curBlk = joinBlk

	case *air.TWhileStmt:
		headBlk := l.freshBlock("while_head")
		headBlk.IsLoop = true
		bodyBlk := l.freshBlock("while_body")
		exitBlk := l.freshBlock("while_exit")
		headBlk.LoopExit = exitBlk.Name

		l.curBlk.Terminator = mirtypes.MirJump{Target: headBlk.Name}

		// Header: evaluate condition
		l.curBlk = headBlk
		condOp := l.lowerExpr(s.Cond)
		l.curBlk.Terminator = mirtypes.MirBranch{
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
			l.curBlk.Terminator = mirtypes.MirJump{Target: headBlk.Name}
		}

		l.loopExitStack = l.loopExitStack[:len(l.loopExitStack)-1]

		l.curBlk = exitBlk

	case *air.TBreakStmt:
		if len(l.loopExitStack) > 0 {
			target := l.loopExitStack[len(l.loopExitStack)-1]
			l.curBlk.Terminator = mirtypes.MirJump{Target: target}
		} else {
			l.curBlk.Terminator = mirtypes.MirReturn{}
		}
		l.curBlk = nil
	}
}

func (l *lowerer) lowerExpr(expr air.TExpr) mirtypes.MirOperand {
	if expr == nil {
		return mirtypes.MirConst{Type: mirtypes.MirI32, Value: 0}
	}

	mType := toMirType(expr.ExprType())

	switch e := expr.(type) {
	case *air.TLiteralExpr:
		return mirtypes.MirConst{
			Type:  mType,
			Value: e.Value,
		}

	case *air.TVarRefExpr:
		if reg, ok := l.vars[e.Name]; ok {
			return mirtypes.MirReg{Name: reg}
		}
		if _, ok := l.mod.Globals[e.Name]; ok {
			return mirtypes.MirGlobal{Name: e.Name}
		}
		return mirtypes.MirReg{Name: e.Name}

	case *air.TBinaryExpr:
		leftOp := l.lowerExpr(e.Left)
		rightOp := l.lowerExpr(e.Right)
		destReg := l.freshReg("t_bin", mType)

		switch string(e.Op) {
		case "+":
			l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirBinOp{
				Op: "add", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "-":
			l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirBinOp{
				Op: "sub", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "*":
			l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirBinOp{
				Op: "mul", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "/":
			l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirBinOp{
				Op: "div", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "<":
			l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirCmpOp{
				Op: "lt", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "<=":
			l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirCmpOp{
				Op: "le", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case ">":
			l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirCmpOp{
				Op: "gt", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case ">=":
			l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirCmpOp{
				Op: "ge", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "==":
			l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirCmpOp{
				Op: "eq", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		case "!=":
			l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirCmpOp{
				Op: "ne", Type: toMirType(e.Left.ExprType()), Dest: destReg, Left: leftOp, Right: rightOp,
			})
		default:
			l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirBinOp{
				Op: "add", Type: mType, Dest: destReg, Left: leftOp, Right: rightOp,
			})
		}
		return mirtypes.MirReg{Name: destReg}

	case *air.TUnaryExpr:
		subOp := l.lowerExpr(e.Expr)
		destReg := l.freshReg("t_un", mType)

		if e.Op == ir.UNARY_OP_NEGATIVE {
			l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirBinOp{
				Op: "sub", Type: mType, Dest: destReg,
				Left:  mirtypes.MirConst{Type: mType, Value: 0},
				Right: subOp,
			})
		} else if e.Op == ir.UNARY_OP_NOT {
			l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirCmpOp{
				Op: "eq", Type: mType, Dest: destReg,
				Left:  subOp,
				Right: mirtypes.MirConst{Type: mType, Value: 0},
			})
		} else {
			return subOp
		}
		return mirtypes.MirReg{Name: destReg}

	case *air.TCallExpr:
		calleeName := "unknown"
		if ref, ok := e.Callee.(*air.TVarRefExpr); ok {
			calleeName = ref.Name
		}

		var args []mirtypes.MirOperand
		for _, arg := range e.Args {
			args = append(args, l.lowerExpr(arg))
		}

		var destReg string
		if mType != mirtypes.MirVoid {
			destReg = l.freshReg("t_call", mType)
		}

		l.curBlk.Insts = append(l.curBlk.Insts, mirtypes.MirCallInst{
			Dest:   destReg,
			Callee: calleeName,
			Args:   args,
		})

		if destReg != "" {
			return mirtypes.MirReg{Name: destReg}
		}
		return mirtypes.MirConst{Type: mirtypes.MirVoid, Value: 0}

	case *air.TDropStmt:
		return l.lowerExpr(e.Expr)

	default:
		return mirtypes.MirConst{Type: mirtypes.MirI32, Value: 0}
	}
}

func toMirType(t air.Type) mirtypes.MirT {
	if t == nil {
		return mirtypes.MirI32
	}
	switch t.String() {
	case "i8", "i16", "i32", "u8", "u16", "u32", "bool":
		return mirtypes.MirI32
	case "i64", "u64", "i128", "u128":
		return mirtypes.MirI64
	case "f32":
		return mirtypes.MirF32
	case "f64":
		return mirtypes.MirF64
	case "void":
		return mirtypes.MirVoid
	default:
		return mirtypes.MirI32
	}
}
