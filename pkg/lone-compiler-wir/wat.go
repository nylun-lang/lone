package lonewir

import (
	"fmt"
	"strings"
)

// FormatWat serializes a WasmModule AST into WebAssembly Text format.
func (m *WasmModule) FormatWat() string {
	if m == nil {
		return "(module)\n"
	}

	var sb strings.Builder
	sb.WriteString("(module\n")

	// 1. Imports
	for _, imp := range m.Imports {
		var paramStr strings.Builder
		for _, p := range imp.Params {
			paramStr.WriteString(" " + string(p))
		}
		resultStr := ""
		if imp.Result != "" && imp.Result != TypeVoid {
			resultStr = fmt.Sprintf(" (result %s)", imp.Result)
		}

		paramsPart := ""
		if paramStr.Len() > 0 {
			paramsPart = fmt.Sprintf(" (param%s)", paramStr.String())
		}

		sb.WriteString(fmt.Sprintf("  (import %q %q (func %s%s%s))\n",
			imp.Module, imp.Name, imp.Alias, paramsPart, resultStr))
	}

	// 2. Memory
	if m.MemoryPages > 0 {
		if m.MemoryExport != "" {
			sb.WriteString(fmt.Sprintf("  (memory (export %q) %d)\n", m.MemoryExport, m.MemoryPages))
		} else {
			sb.WriteString(fmt.Sprintf("  (memory %d)\n", m.MemoryPages))
		}
	}

	// 3. Globals
	for _, g := range m.Globals {
		typeSpec := string(g.Type)
		if g.Mutable {
			typeSpec = fmt.Sprintf("(mut %s)", g.Type)
		}
		sb.WriteString(fmt.Sprintf("  (global %s %s (%s))\n", g.Name, typeSpec, g.InitVal))
	}

	// 4. Data segments
	for _, data := range m.DataSegments {
		escaped := escapeWatString(data.RawStr)
		sb.WriteString(fmt.Sprintf("  (data (i32.const %d) \"%s\")\n", data.Offset, escaped))
	}

	if len(m.DataSegments) > 0 {
		sb.WriteString("\n")
	}

	// 5. Inlined runtime functions
	for _, inlined := range m.InlinedRuntime {
		if strings.TrimSpace(inlined) != "" {
			sb.WriteString(inlined)
			sb.WriteString("\n\n")
		}
	}

	// 6. Functions
	for _, fn := range m.Functions {
		sb.WriteString(formatFunction(fn))
		sb.WriteString("\n")
	}

	sb.WriteString(")\n")
	return sb.String()
}

func formatFunction(fn *WasmFunc) string {
	var sb strings.Builder

	// Export annotations on the function signature
	var exportParts strings.Builder
	for _, exp := range fn.Exports {
		exportParts.WriteString(fmt.Sprintf(" (export %q)", exp))
	}

	// Parameters
	var paramParts strings.Builder
	for _, p := range fn.Params {
		paramParts.WriteString(fmt.Sprintf(" (param %s %s)", p.Name, p.Type))
	}

	// Result
	resultPart := ""
	if fn.Result != "" && fn.Result != TypeVoid {
		resultPart = fmt.Sprintf(" (result %s)", fn.Result)
	}

	sb.WriteString(fmt.Sprintf("  (func %s%s%s%s\n", fn.Name, exportParts.String(), paramParts.String(), resultPart))

	// Locals
	for _, loc := range fn.Locals {
		sb.WriteString(fmt.Sprintf("    (local %s %s)\n", loc.Name, loc.Type))
	}

	// Body instructions
	for _, inst := range fn.Body {
		sb.WriteString(inst.FormatWat(2) + "\n")
	}

	sb.WriteString("  )")
	return sb.String()
}

func escapeWatString(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		switch b {
		case '\n':
			sb.WriteString(`\0a`)
		case '\r':
			sb.WriteString(`\0d`)
		case '\t':
			sb.WriteString(`\09`)
		case '\\':
			sb.WriteString(`\5c`)
		case '"':
			sb.WriteString(`\22`)
		case 0:
			sb.WriteString(`\00`)
		default:
			if b >= 32 && b <= 126 {
				sb.WriteByte(b)
			} else {
				sb.WriteString(fmt.Sprintf("\\%02x", b))
			}
		}
	}
	sb.WriteString(`\00`)
	return sb.String()
}
