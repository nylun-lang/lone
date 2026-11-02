package loneruntime

// Target indicates the compilation execution environment.
type Target string

const (
	TargetAny  Target = "any"
	TargetWasi Target = "wasi"
	TargetWeb  Target = "web"
)

// WasmImportSpec specifies an external WebAssembly function import needed by an intrinsic.
type WasmImportSpec struct {
	Module string
	Name   string
	Alias  string
	Params []string
	Result string
}

// IntrinsicSpec defines an inlined or runtime-provided built-in function.
type IntrinsicSpec struct {
	Name            string           // e.g. "#print"
	Target          Target           // "wasi", "web", or "any"
	Params          []string         // e.g. ["i32", "i32"]
	ReturnType      string           // e.g. "void"
	Imports         []WasmImportSpec // imports required in the Wasm module
	InlinedWatFunc  string           // Inlined WAT function body if applicable
	FunctionName    string           // e.g. "$__lone_rt_print"
}
