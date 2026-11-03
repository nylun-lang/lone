package loneruntime

import "strings"

// HostParamSpec describes a typed parameter of a host capability function.
type HostParamSpec struct {
	Name     string
	LoneType string
	WasmType string
}

// HostFuncSpec describes a host-imported WebAssembly function signature.
type HostFuncSpec struct {
	Namespace      string
	WasmModule     string
	Name           string
	Params         []HostParamSpec
	ReturnLoneType string
	ReturnWasmType string
}

var defaultHostRegistry = map[string]map[string]HostFuncSpec{
	":wasi:snapshot_preview1": {
		"fd_write": {
			Namespace:  ":wasi:snapshot_preview1",
			WasmModule: "wasi_snapshot_preview1",
			Name:       "fd_write",
			Params: []HostParamSpec{
				{Name: "fd", LoneType: "i32", WasmType: "i32"},
				{Name: "iovs", LoneType: "ptr<any>", WasmType: "i32"},
				{Name: "iovs_len", LoneType: "i32", WasmType: "i32"},
				{Name: "nwritten", LoneType: "ptr<i32>", WasmType: "i32"},
			},
			ReturnLoneType: "i32",
			ReturnWasmType: "i32",
		},
		"fd_read": {
			Namespace:  ":wasi:snapshot_preview1",
			WasmModule: "wasi_snapshot_preview1",
			Name:       "fd_read",
			Params: []HostParamSpec{
				{Name: "fd", LoneType: "i32", WasmType: "i32"},
				{Name: "iovs", LoneType: "ptr<any>", WasmType: "i32"},
				{Name: "iovs_len", LoneType: "i32", WasmType: "i32"},
				{Name: "nread", LoneType: "ptr<i32>", WasmType: "i32"},
			},
			ReturnLoneType: "i32",
			ReturnWasmType: "i32",
		},
		"fd_close": {
			Namespace:  ":wasi:snapshot_preview1",
			WasmModule: "wasi_snapshot_preview1",
			Name:       "fd_close",
			Params: []HostParamSpec{
				{Name: "fd", LoneType: "i32", WasmType: "i32"},
			},
			ReturnLoneType: "i32",
			ReturnWasmType: "i32",
		},
		"proc_exit": {
			Namespace:  ":wasi:snapshot_preview1",
			WasmModule: "wasi_snapshot_preview1",
			Name:       "proc_exit",
			Params: []HostParamSpec{
				{Name: "code", LoneType: "i32", WasmType: "i32"},
			},
			ReturnLoneType: "void",
			ReturnWasmType: "",
		},
		"args_get": {
			Namespace:  ":wasi:snapshot_preview1",
			WasmModule: "wasi_snapshot_preview1",
			Name:       "args_get",
			Params: []HostParamSpec{
				{Name: "argv", LoneType: "ptr<any>", WasmType: "i32"},
				{Name: "argv_buf", LoneType: "ptr<u8>", WasmType: "i32"},
			},
			ReturnLoneType: "i32",
			ReturnWasmType: "i32",
		},
		"args_sizes_get": {
			Namespace:  ":wasi:snapshot_preview1",
			WasmModule: "wasi_snapshot_preview1",
			Name:       "args_sizes_get",
			Params: []HostParamSpec{
				{Name: "argc", LoneType: "ptr<i32>", WasmType: "i32"},
				{Name: "argv_buf_size", LoneType: "ptr<i32>", WasmType: "i32"},
			},
			ReturnLoneType: "i32",
			ReturnWasmType: "i32",
		},
		"random_get": {
			Namespace:  ":wasi:snapshot_preview1",
			WasmModule: "wasi_snapshot_preview1",
			Name:       "random_get",
			Params: []HostParamSpec{
				{Name: "buf", LoneType: "ptr<any>", WasmType: "i32"},
				{Name: "buf_len", LoneType: "i32", WasmType: "i32"},
			},
			ReturnLoneType: "i32",
			ReturnWasmType: "i32",
		},
	},
	":env": {
		"print": {
			Namespace:  ":env",
			WasmModule: "env",
			Name:       "print",
			Params: []HostParamSpec{
				{Name: "ptr", LoneType: "ptr<u8>", WasmType: "i32"},
				{Name: "len", LoneType: "i32", WasmType: "i32"},
			},
			ReturnLoneType: "void",
			ReturnWasmType: "",
		},
	},
}

// IsHostNamespace returns true if source refers to a host Wasm capability namespace (:wasi:* or :env).
func IsHostNamespace(source string) bool {
	return source == ":env" || strings.HasPrefix(source, ":env:") ||
		source == ":wasi" || strings.HasPrefix(source, ":wasi:")
}

// LookupHostNamespace returns the table of host functions for a given host namespace.
func LookupHostNamespace(source string) (map[string]HostFuncSpec, bool) {
	ns, ok := defaultHostRegistry[source]
	return ns, ok
}

// LookupHostFunc looks up a single host function specification in a host namespace.
func LookupHostFunc(source, name string) (HostFuncSpec, bool) {
	ns, ok := defaultHostRegistry[source]
	if !ok {
		return HostFuncSpec{}, false
	}
	fn, ok := ns[name]
	return fn, ok
}
