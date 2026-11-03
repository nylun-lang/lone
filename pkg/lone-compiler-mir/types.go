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

// MirBinOp represents binary arithmetic: %dest = add/sub/mul/div/mod/and/or <type> <left>, <right>
type MirBinOp struct {
	Op    string // add, sub, mul, div, mod, and, or
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

// MirSlotAddr computes the linear memory address of a datastack frame slot: %dest = slot.addr +<offset>
type MirSlotAddr struct {
	Dest   string
	Offset int
}

func (s MirSlotAddr) InstString() string {
	dest := s.Dest
	if len(dest) > 0 && dest[0] != '%' {
		dest = "%" + dest
	}
	return fmt.Sprintf("%s = slot.addr +%d", dest, s.Offset)
}

// MirLoad loads a value of Size bytes from linear memory at Addr + Offset.
type MirLoad struct {
	Dest   string
	Type   MirT
	Size   int
	Signed bool
	Addr   MirOperand
	Offset int
}

func (l MirLoad) InstString() string {
	dest := l.Dest
	if len(dest) > 0 && dest[0] != '%' {
		dest = "%" + dest
	}
	sign := "u"
	if l.Signed {
		sign = "s"
	}
	return fmt.Sprintf("%s = load.%d%s %s [%s+%d]", dest, l.Size, sign, l.Type.String(), l.Addr.OperandString(), l.Offset)
}

// MirStore stores a value of Size bytes into linear memory at Addr + Offset.
type MirStore struct {
	Type   MirT
	Size   int
	Addr   MirOperand
	Offset int
	Value  MirOperand
}

func (s MirStore) InstString() string {
	return fmt.Sprintf("store.%d %s [%s+%d], %s", s.Size, s.Type.String(), s.Addr.OperandString(), s.Offset, s.Value.OperandString())
}

// MirGlobalSet assigns a value to a mutable module global.
type MirGlobalSet struct {
	Name  string
	Type  MirT
	Value MirOperand
}

func (g MirGlobalSet) InstString() string {
	return fmt.Sprintf("global.set $%s, %s", g.Name, g.Value.OperandString())
}

// MirHeapAlloc bump-allocates Size bytes from the linear memory heap arena.
type MirHeapAlloc struct {
	Dest string
	Size MirOperand
}

func (h MirHeapAlloc) InstString() string {
	dest := h.Dest
	if len(dest) > 0 && dest[0] != '%' {
		dest = "%" + dest
	}
	return fmt.Sprintf("%s = heap.alloc %s", dest, h.Size.OperandString())
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
	JoinTarget string // structured join block for if/else reconstruction
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
	IsDoWhile  bool   // true if this block is a post-condition (do-while) loop
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
	FrameSize  int // aligned datastack frame size in bytes (0 if no stack slots used)
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

type MirHostImport struct {
	WasmModule string
	WasmName   string
	LocalName  string
	Params     []string
	Result     string
}

type MirModule struct {
	Functions    []*MirFunction
	Globals      map[string]MirT
	GlobalInits  map[string]any
	DataSegments []MirDataSegment
	HostImports  []MirHostImport
	UsesHeap     bool
}
