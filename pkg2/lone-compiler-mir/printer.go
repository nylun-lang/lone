package lonemir

import (
	"fmt"
	"strings"
)

// FormatModule formats a MirModule into readable Falone assembly text.
func FormatModule(mod *MirModule) string {
	if mod == nil {
		return "<nil module>"
	}

	var sb strings.Builder
	for _, seg := range mod.DataSegments {
		sb.WriteString(fmt.Sprintf("data @%d %q\n", seg.Offset, seg.Value))
	}
	if len(mod.DataSegments) > 0 && len(mod.Functions) > 0 {
		sb.WriteString("\n")
	}

	for i, fn := range mod.Functions {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(FormatFunction(fn))
	}
	return sb.String()
}

// FormatFunction formats a single MirFunction into Falone assembly text.
func FormatFunction(fn *MirFunction) string {
	if fn == nil {
		return "<nil function>"
	}

	var params []string
	for _, p := range fn.Params {
		pName := p.Name
		if len(pName) > 0 && pName[0] != '%' {
			pName = "%" + pName
		}
		params = append(params, fmt.Sprintf("%s: %s", pName, p.Type.String()))
	}

	fnName := fn.Name
	if len(fnName) > 0 && fnName[0] != '@' && fnName != "main" {
		fnName = "@" + fnName
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("fn %s(%s) -> %s {\n", fnName, strings.Join(params, ", "), fn.ReturnType.String()))

	for i, blk := range fn.Blocks {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("    %s:\n", blk.Name))
		for _, inst := range blk.Insts {
			sb.WriteString(fmt.Sprintf("        %s\n", inst.InstString()))
		}
		if blk.Terminator != nil {
			sb.WriteString(fmt.Sprintf("        %s\n", blk.Terminator.TerminatorString()))
		}
	}

	sb.WriteString("}")
	return sb.String()
}
