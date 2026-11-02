package lonemir

import "fmt"

// MirT represents primitive data types in Falone MIR.
type MirT int

const (
	MirVoid MirT = iota
	MirI32
	MirI64
	MirF32
	MirF64
)

func (t MirT) String() string {
	switch t {
	case MirVoid:
		return "void"
	case MirI32:
		return "i32"
	case MirI64:
		return "i64"
	case MirF32:
		return "f32"
	case MirF64:
		return "f64"
	default:
		return "unknown"
	}
}

// MirOperand represents a value source in instructions: register, constant, or global.
type MirOperand interface {
	OperandString() string
}

type MirReg struct {
	Name string
}

func (r MirReg) OperandString() string {
	if len(r.Name) > 0 && r.Name[0] == '%' {
		return r.Name
	}
	return "%" + r.Name
}

type MirConst struct {
	Type  MirT
	Value any
}

func (c MirConst) OperandString() string {
	return fmt.Sprintf("%v", c.Value)
}

type MirGlobal struct {
	Name string
}

func (g MirGlobal) OperandString() string {
	if len(g.Name) > 0 && g.Name[0] == '$' {
		return g.Name
	}
	return "$" + g.Name
}

// =====================
// INSTRUCTIONS
// =====================

type MirInst interface {
	InstString() string
}

// MirBinOp represents binary arithmetic: %dest = add/sub/mul/div <type> <left>, <right>
type MirBinOp struct {
	Op    string // add, sub, mul, div
	Type  MirT
	Dest  string
	Left  MirOperand
	Right MirOperand
}

func (b MirBinOp) InstString() string {
	dest := b.Dest
	if len(dest) > 0 && dest[0] != '%' {
		dest = "%" + dest
	}
	return fmt.Sprintf("%s = %s %s %s, %s", dest, b.Op, b.Type.String(), b.Left.OperandString(), b.Right.OperandString())
}

// MirCmpOp represents comparisons: %dest = cmp.lt/gt/eq/le/ge/ne <left> <right>
type MirCmpOp struct {
	Op    string // lt, gt, eq, le, ge, ne
	Type  MirT
	Dest  string
	Left  MirOperand
	Right MirOperand
}

func (c MirCmpOp) InstString() string {
	dest := c.Dest
	if len(dest) > 0 && dest[0] != '%' {
		dest = "%" + dest
	}
	return fmt.Sprintf("%s = cmp.%s %s %s", dest, c.Op, c.Left.OperandString(), c.Right.OperandString())
}

// MirConstInst represents constant assignment: %dest = const <type> <value>
type MirConstInst struct {
	Dest  string
	Type  MirT
	Value any
}

func (c MirConstInst) InstString() string {
	dest := c.Dest
	if len(dest) > 0 && dest[0] != '%' {
		dest = "%" + dest
	}
	return fmt.Sprintf("%s = const %s %v", dest, c.Type.String(), c.Value)
}

// MirCopyInst represents register copy: %dest = %src
type MirCopyInst struct {
	Dest string
	Src  MirOperand
}

func (c MirCopyInst) InstString() string {
	dest := c.Dest
	if len(dest) > 0 && dest[0] != '%' {
		dest = "%" + dest
	}
	return fmt.Sprintf("%s = %s", dest, c.Src.OperandString())
}

// MirCallInst represents a function call: [%dest =] call @callee <args...>
type MirCallInst struct {
	Dest   string // empty if void call
	Callee string
	Args   []MirOperand
}

func (c MirCallInst) InstString() string {
	callee := c.Callee
	if len(callee) > 0 && callee[0] != '@' {
		callee = "@" + callee
	}
	var argsStr string
	for _, a := range c.Args {
		argsStr += " " + a.OperandString()
	}
	if c.Dest != "" {
		dest := c.Dest
		if len(dest) > 0 && dest[0] != '%' {
			dest = "%" + dest
		}
		return fmt.Sprintf("%s = call %s%s", dest, callee, argsStr)
	}
	return fmt.Sprintf("call %s%s", callee, argsStr)
}

// =====================
// TERMINATORS
// =====================

type MirTerminator interface {
	TerminatorString() string
}

type MirReturn struct {
	Value MirOperand // nil for void return
}

func (r MirReturn) TerminatorString() string {
	if r.Value != nil {
		return fmt.Sprintf("return %s", r.Value.OperandString())
	}
	return "return"
}

type MirJump struct {
	Target string
}

func (j MirJump) TerminatorString() string {
	return fmt.Sprintf("jump %s", j.Target)
}

type MirBranch struct {
	Cond       MirOperand
	ThenTarget string
	ElseTarget string
}

func (b MirBranch) TerminatorString() string {
	return fmt.Sprintf("branch %s, %s, %s", b.Cond.OperandString(), b.ThenTarget, b.ElseTarget)
}

// =====================
// BASIC BLOCK & FUNCTION
// =====================

type MirBlock struct {
	Name       string
	IsLoop     bool   // true if this block is a loop header
	LoopExit   string // label of the loop exit block
	Insts      []MirInst
	Terminator MirTerminator
}

type MirParam struct {
	Name string
	Type MirT
}

type MirFunction struct {
	Name       string
	Params     []MirParam
	ReturnType MirT
	Blocks     []*MirBlock
	Locals     map[string]MirT // All allocated registers and their types
}

func (f *MirFunction) AddBlock(name string) *MirBlock {
	blk := &MirBlock{
		Name:  name,
		Insts: make([]MirInst, 0),
	}
	f.Blocks = append(f.Blocks, blk)
	return blk
}

func (f *MirFunction) AddLocal(reg string, typ MirT) {
	if len(reg) > 0 && reg[0] == '%' {
		reg = reg[1:]
	}
	f.Locals[reg] = typ
}

type MirDataSegment struct {
	Offset int
	Value  string
}

type MirModule struct {
	Functions    []*MirFunction
	Globals      map[string]MirT
	DataSegments []MirDataSegment
}
