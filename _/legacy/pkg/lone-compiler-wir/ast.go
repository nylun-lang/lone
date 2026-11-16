package lonewir

import (
	"fmt"
	"strings"
)

// WasmType represents a primitive WebAssembly value type.
type WasmType string

const (
	TypeI32  WasmType = "i32"
	TypeI64  WasmType = "i64"
	TypeF32  WasmType = "f32"
	TypeF64  WasmType = "f64"
	TypeVoid WasmType = ""
)

// WasmInst represents an individual WebAssembly instruction node.
type WasmInst interface {
	FormatWat(indent int) string
	IsPure() bool
}

// Param represents a named function parameter.
type Param struct {
	Name string
	Type WasmType
}

// Local represents a function local variable.
type Local struct {
	Name string
	Type WasmType
}

// Global represents a module-level WebAssembly global variable.
type Global struct {
	Name    string
	Type    WasmType
	Mutable bool
	InitVal string
}

// Import represents a Wasm module import.
type Import struct {
	Module string
	Name   string
	Alias  string
	Params []WasmType
	Result WasmType
}

// Export represents an exported symbol.
type Export struct {
	Name string
	Kind string // "func", "memory", "table", "global"
	Ref  string // e.g. "$main"
}

// DataSegment represents linear memory initial data.
type DataSegment struct {
	Offset int
	Bytes  []byte
	RawStr string
}

// WasmFunc represents a function declaration in Wasm AST.
type WasmFunc struct {
	Name    string
	Params  []Param
	Result  WasmType
	Locals  []Local
	Body    []WasmInst
	Exports []string
}

// WasmModule represents the top-level WebAssembly Module AST.
type WasmModule struct {
	Imports        []Import
	MemoryPages    int
	MemoryExport   string
	Globals        []Global
	DataSegments   []DataSegment
	Functions      []*WasmFunc
	Exports        []Export
	InlinedRuntime []string
}

// NewModule initializes an empty WasmModule AST.
func NewModule() *WasmModule {
	return &WasmModule{
		MemoryPages:  2, // Default 128 KiB
		MemoryExport: "memory",
	}
}

// AddFunction adds a WasmFunc to the module.
func (m *WasmModule) AddFunction(fn *WasmFunc) {
	m.Functions = append(m.Functions, fn)
}

// =====================
// Instructions
// =====================

// InstConstI32 represents an i32.const instruction.
type InstConstI32 struct {
	Value int32
}

func (i InstConstI32) FormatWat(indent int) string {
	return fmt.Sprintf("%si32.const %d", strings.Repeat("  ", indent), i.Value)
}
func (i InstConstI32) IsPure() bool { return true }

// InstConstI64 represents an i64.const instruction.
type InstConstI64 struct {
	Value int64
}

func (i InstConstI64) FormatWat(indent int) string {
	return fmt.Sprintf("%si64.const %d", strings.Repeat("  ", indent), i.Value)
}
func (i InstConstI64) IsPure() bool { return true }

// InstConstF32 represents an f32.const instruction.
type InstConstF32 struct {
	Value float32
}

func (i InstConstF32) FormatWat(indent int) string {
	return fmt.Sprintf("%sf32.const %g", strings.Repeat("  ", indent), i.Value)
}
func (i InstConstF32) IsPure() bool { return true }

// InstConstF64 represents an f64.const instruction.
type InstConstF64 struct {
	Value float64
}

func (i InstConstF64) FormatWat(indent int) string {
	return fmt.Sprintf("%sf64.const %g", strings.Repeat("  ", indent), i.Value)
}
func (i InstConstF64) IsPure() bool { return true }

// InstLocalGet represents a local.get instruction.
type InstLocalGet struct {
	Name string
}

func (i InstLocalGet) FormatWat(indent int) string {
	return fmt.Sprintf("%slocal.get %s", strings.Repeat("  ", indent), i.Name)
}
func (i InstLocalGet) IsPure() bool { return true }

// InstLocalSet represents a local.set instruction.
type InstLocalSet struct {
	Name string
}

func (i InstLocalSet) FormatWat(indent int) string {
	return fmt.Sprintf("%slocal.set %s", strings.Repeat("  ", indent), i.Name)
}
func (i InstLocalSet) IsPure() bool { return false }

// InstLocalTee represents a local.tee instruction.
type InstLocalTee struct {
	Name string
}

func (i InstLocalTee) FormatWat(indent int) string {
	return fmt.Sprintf("%slocal.tee %s", strings.Repeat("  ", indent), i.Name)
}
func (i InstLocalTee) IsPure() bool { return false }

// InstGlobalGet represents a global.get instruction.
type InstGlobalGet struct {
	Name string
}

func (i InstGlobalGet) FormatWat(indent int) string {
	return fmt.Sprintf("%sglobal.get %s", strings.Repeat("  ", indent), i.Name)
}
func (i InstGlobalGet) IsPure() bool { return true }

// InstGlobalSet represents a global.set instruction.
type InstGlobalSet struct {
	Name string
}

func (i InstGlobalSet) FormatWat(indent int) string {
	return fmt.Sprintf("%sglobal.set %s", strings.Repeat("  ", indent), i.Name)
}
func (i InstGlobalSet) IsPure() bool { return false }

// InstLoad represents a linear memory load instruction.
type InstLoad struct {
	Op     string // "i32.load", "i32.load8_u", etc.
	Offset int
}

func (i InstLoad) FormatWat(indent int) string {
	if i.Offset > 0 {
		return fmt.Sprintf("%s%s offset=%d", strings.Repeat("  ", indent), i.Op, i.Offset)
	}
	return fmt.Sprintf("%s%s", strings.Repeat("  ", indent), i.Op)
}
func (i InstLoad) IsPure() bool { return false }

// InstStore represents a linear memory store instruction.
type InstStore struct {
	Op     string // "i32.store", "i32.store8", etc.
	Offset int
}

func (i InstStore) FormatWat(indent int) string {
	if i.Offset > 0 {
		return fmt.Sprintf("%s%s offset=%d", strings.Repeat("  ", indent), i.Op, i.Offset)
	}
	return fmt.Sprintf("%s%s", strings.Repeat("  ", indent), i.Op)
}
func (i InstStore) IsPure() bool { return false }

// InstBinaryOp represents a binary operation instruction.
type InstBinaryOp struct {
	Op string // "i32.add", "i32.sub", "f64.mul", etc.
}

func (i InstBinaryOp) FormatWat(indent int) string {
	return fmt.Sprintf("%s%s", strings.Repeat("  ", indent), i.Op)
}
func (i InstBinaryOp) IsPure() bool { return true }

// InstUnaryOp represents a unary operation instruction.
type InstUnaryOp struct {
	Op string // "i32.eqz", "f64.neg", "f64.convert_i32_s", "i32.trunc_f64_s", etc.
}

func (i InstUnaryOp) FormatWat(indent int) string {
	return fmt.Sprintf("%s%s", strings.Repeat("  ", indent), i.Op)
}
func (i InstUnaryOp) IsPure() bool { return true }

// InstCall represents a call instruction.
type InstCall struct {
	Target string
}

func (i InstCall) FormatWat(indent int) string {
	return fmt.Sprintf("%scall %s", strings.Repeat("  ", indent), i.Target)
}
func (i InstCall) IsPure() bool { return false }

// InstDrop represents a drop instruction.
type InstDrop struct{}

func (i InstDrop) FormatWat(indent int) string {
	return fmt.Sprintf("%sdrop", strings.Repeat("  ", indent))
}
func (i InstDrop) IsPure() bool { return false }

// InstReturn represents a return instruction.
type InstReturn struct{}

func (i InstReturn) FormatWat(indent int) string {
	return fmt.Sprintf("%sreturn", strings.Repeat("  ", indent))
}
func (i InstReturn) IsPure() bool { return false }

// InstNop represents a nop instruction.
type InstNop struct{}

func (i InstNop) FormatWat(indent int) string {
	return fmt.Sprintf("%snop", strings.Repeat("  ", indent))
}
func (i InstNop) IsPure() bool { return true }

// InstUnreachable represents an unreachable instruction.
type InstUnreachable struct{}

func (i InstUnreachable) FormatWat(indent int) string {
	return fmt.Sprintf("%sunreachable", strings.Repeat("  ", indent))
}
func (i InstUnreachable) IsPure() bool { return false }

// InstMemoryGrow represents a memory.grow instruction.
type InstMemoryGrow struct{}

func (i InstMemoryGrow) FormatWat(indent int) string {
	return fmt.Sprintf("%smemory.grow", strings.Repeat("  ", indent))
}
func (i InstMemoryGrow) IsPure() bool { return false }

// InstMemorySize represents a memory.size instruction.
type InstMemorySize struct{}

func (i InstMemorySize) FormatWat(indent int) string {
	return fmt.Sprintf("%smemory.size", strings.Repeat("  ", indent))
}
func (i InstMemorySize) IsPure() bool { return false }

// InstBlock represents a structured block.
type InstBlock struct {
	Label  string
	Result WasmType
	Body   []WasmInst
}

func (i InstBlock) FormatWat(indent int) string {
	pad := strings.Repeat("  ", indent)
	labelStr := ""
	if i.Label != "" {
		labelStr = " " + i.Label
	}
	resStr := ""
	if i.Result != "" && i.Result != TypeVoid {
		resStr = fmt.Sprintf(" (result %s)", i.Result)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%sblock%s%s\n", pad, labelStr, resStr))
	for _, inst := range i.Body {
		sb.WriteString(inst.FormatWat(indent+1) + "\n")
	}
	sb.WriteString(fmt.Sprintf("%send", pad))
	return sb.String()
}
func (i InstBlock) IsPure() bool { return false }

// InstLoop represents a structured loop.
type InstLoop struct {
	Label  string
	Result WasmType
	Body   []WasmInst
}

func (i InstLoop) FormatWat(indent int) string {
	pad := strings.Repeat("  ", indent)
	labelStr := ""
	if i.Label != "" {
		labelStr = " " + i.Label
	}
	resStr := ""
	if i.Result != "" && i.Result != TypeVoid {
		resStr = fmt.Sprintf(" (result %s)", i.Result)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%sloop%s%s\n", pad, labelStr, resStr))
	for _, inst := range i.Body {
		sb.WriteString(inst.FormatWat(indent+1) + "\n")
	}
	sb.WriteString(fmt.Sprintf("%send", pad))
	return sb.String()
}
func (i InstLoop) IsPure() bool { return false }

// InstIf represents a structured WebAssembly if/else/end block.
type InstIf struct {
	Label  string
	Result WasmType
	Then   []WasmInst
	Else   []WasmInst
}

func (i InstIf) FormatWat(indent int) string {
	pad := strings.Repeat("  ", indent)
	labelStr := ""
	if i.Label != "" {
		labelStr = " " + i.Label
	}
	resStr := ""
	if i.Result != "" && i.Result != TypeVoid {
		resStr = fmt.Sprintf(" (result %s)", i.Result)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%sif%s%s\n", pad, labelStr, resStr))
	for _, inst := range i.Then {
		sb.WriteString(inst.FormatWat(indent+1) + "\n")
	}
	if len(i.Else) > 0 {
		sb.WriteString(fmt.Sprintf("%selse\n", pad))
		for _, inst := range i.Else {
			sb.WriteString(inst.FormatWat(indent+1) + "\n")
		}
	}
	sb.WriteString(fmt.Sprintf("%send", pad))
	return sb.String()
}
func (i InstIf) IsPure() bool { return false }

// InstBr represents an unconditional branch.
type InstBr struct {
	Target string
}

func (i InstBr) FormatWat(indent int) string {
	return fmt.Sprintf("%sbr %s", strings.Repeat("  ", indent), i.Target)
}
func (i InstBr) IsPure() bool { return false }

// InstBrIf represents a conditional branch.
type InstBrIf struct {
	Target string
}

func (i InstBrIf) FormatWat(indent int) string {
	return fmt.Sprintf("%sbr_if %s", strings.Repeat("  ", indent), i.Target)
}
func (i InstBrIf) IsPure() bool { return false }

// InstRaw represents raw inlined WAT text or comments.
type InstRaw struct {
	Code string
}

func (i InstRaw) FormatWat(indent int) string {
	return fmt.Sprintf("%s%s", strings.Repeat("  ", indent), i.Code)
}
func (i InstRaw) IsPure() bool { return false }
