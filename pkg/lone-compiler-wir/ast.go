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
	TypeVoid WasmType = "void"
)

// WasmInst represents an individual WebAssembly instruction node.
type WasmInst interface {
	FormatWat(indent int) string
	IsPure() bool // Returns true if instruction has no side-effects
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
	Kind string // "func", "memory", "table"
	Ref  string // e.g. "$main" or "0"
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
	DataSegments   []DataSegment
	Functions      []*WasmFunc
	Exports        []Export
	InlinedRuntime []string
}

// NewModule initializes an empty WasmModule AST.
func NewModule() *WasmModule {
	return &WasmModule{
		MemoryPages:  1,
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

// InstConst represents an i32.const instruction.
type InstConst struct {
	Value int32
}

func (i InstConst) FormatWat(indent int) string {
	return fmt.Sprintf("%si32.const %d", strings.Repeat("  ", indent), i.Value)
}
func (i InstConst) IsPure() bool { return true }

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

// InstBinaryOp represents a binary operation instruction (e.g. i32.add).
type InstBinaryOp struct {
	Op string // "i32.add", "i32.sub", "i32.mul", etc.
}

func (i InstBinaryOp) FormatWat(indent int) string {
	return fmt.Sprintf("%s%s", strings.Repeat("  ", indent), i.Op)
}
func (i InstBinaryOp) IsPure() bool { return true }

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

// InstBlock represents a structured block.
type InstBlock struct {
	Label string
	Body  []WasmInst
}

func (i InstBlock) FormatWat(indent int) string {
	pad := strings.Repeat("  ", indent)
	labelStr := ""
	if i.Label != "" {
		labelStr = " " + i.Label
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%sblock%s\n", pad, labelStr))
	for _, inst := range i.Body {
		sb.WriteString(inst.FormatWat(indent+1) + "\n")
	}
	sb.WriteString(fmt.Sprintf("%send", pad))
	return sb.String()
}
func (i InstBlock) IsPure() bool { return false }

// InstLoop represents a structured loop.
type InstLoop struct {
	Label string
	Body  []WasmInst
}

func (i InstLoop) FormatWat(indent int) string {
	pad := strings.Repeat("  ", indent)
	labelStr := ""
	if i.Label != "" {
		labelStr = " " + i.Label
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%sloop%s\n", pad, labelStr))
	for _, inst := range i.Body {
		sb.WriteString(inst.FormatWat(indent+1) + "\n")
	}
	sb.WriteString(fmt.Sprintf("%send", pad))
	return sb.String()
}
func (i InstLoop) IsPure() bool { return false }

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
