package loneanalyzerdiag

import (
	"encoding/json"
	"fmt"
	"strings"

	"lone/pkg/lone-analyzer"
	
)

// PrintNodesJSON serializes the Typed IR node tree into formatted JSON.
func PrintNodesJSON(node any) (string, error) {
	bytes, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// PrintNodeTree renders a structured indented tree representation of the Typed IR nodes.
func PrintNodeTree(node loneanalyzer.TNode, indent int) string {
	if node == nil {
		return "<nil>\n"
	}

	pad := strings.Repeat("  ", indent)

	switch n := node.(type) {
	case *loneanalyzer.TProgram:
		var sb strings.Builder
		sb.WriteString(pad + "TProgram\n")
		for _, d := range n.Decls {
			sb.WriteString(PrintNodeTree(d, indent+1))
		}
		return sb.String()

	case *loneanalyzer.TVarDec:
		return fmt.Sprintf("%s├── TVarDec [name=%q, type=%s]\n%s", pad, n.Name, n.Type.String(), PrintNodeTree(n.Value, indent+1))

	case *loneanalyzer.TFnDec:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s├── TFnDec [name=%q, return=%s]\n", pad, n.Name, n.ReturnType.String()))
		for _, p := range n.Params {
			sb.WriteString(fmt.Sprintf("%s  ├── TFnParam [name=%q, type=%s]\n", pad, p.Name, p.Type.String()))
		}
		sb.WriteString(PrintNodeTree(&n.Body, indent+1))
		return sb.String()

	case *loneanalyzer.TBlock:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s├── TBlock [stmts=%d]\n", pad, len(n.Stmts)))
		for _, s := range n.Stmts {
			sb.WriteString(PrintNodeTree(s, indent+1))
		}
		return sb.String()

	case *loneanalyzer.TReturnStmt:
		if n.Value != nil {
			return fmt.Sprintf("%s├── TReturnStmt\n%s", pad, PrintNodeTree(n.Value, indent+1))
		}
		return fmt.Sprintf("%s├── TReturnStmt (void)\n", pad)

	case *loneanalyzer.TDropStmt:
		return fmt.Sprintf("%s├── TDropStmt\n%s", pad, PrintNodeTree(n.Expr, indent+1))

	case *loneanalyzer.TBreakStmt:
		return fmt.Sprintf("%s├── TBreakStmt [label=%q]\n", pad, n.Label)

	case *loneanalyzer.TExprStmt:
		return fmt.Sprintf("%s├── TExprStmt\n%s", pad, PrintNodeTree(n.Expr, indent+1))

	case *loneanalyzer.TWhileStmt:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s├── TWhileStmt\n", pad))
		sb.WriteString(fmt.Sprintf("%s  ├── Cond:\n%s", pad, PrintNodeTree(n.Cond, indent+2)))
		sb.WriteString(PrintNodeTree(&n.Body, indent+1))
		return sb.String()

	case *loneanalyzer.TDoWhileStmt:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s├── TDoWhileStmt\n", pad))
		sb.WriteString(PrintNodeTree(&n.Body, indent+1))
		sb.WriteString(fmt.Sprintf("%s  ├── Cond:\n%s", pad, PrintNodeTree(n.Cond, indent+2)))
		return sb.String()

	case *loneanalyzer.TIfStmt:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s├── TIfStmt\n", pad))
		sb.WriteString(fmt.Sprintf("%s  ├── Cond:\n%s", pad, PrintNodeTree(n.Cond, indent+2)))
		sb.WriteString(PrintNodeTree(&n.Then, indent+1))
		if n.Else != nil {
			sb.WriteString(fmt.Sprintf("%s  ├── Else:\n", pad))
			sb.WriteString(PrintNodeTree(n.Else, indent+2))
		}
		return sb.String()

	case *loneanalyzer.TLabelDec:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s├── TLabelDec [name=%q]\n", pad, n.Name))
		sb.WriteString(PrintNodeTree(&n.Body, indent+1))
		return sb.String()

	case *loneanalyzer.TBinaryExpr:
		return fmt.Sprintf("%s└── TBinaryExpr [op=%s, type=%s]\n%s%s", pad, n.Op, n.Typ.String(), PrintNodeTree(n.Left, indent+1), PrintNodeTree(n.Right, indent+1))

	case *loneanalyzer.TIsExpr:
		opStr := "is"
		if n.Negated {
			opStr = "is not"
		}
		return fmt.Sprintf("%s└── TIsExpr [%s %s, type=%s]\n%s", pad, opStr, n.TargetType.String(), n.Typ.String(), PrintNodeTree(n.Expr, indent+1))

	case *loneanalyzer.TUnaryExpr:
		return fmt.Sprintf("%s└── TUnaryExpr [op=%s, type=%s]\n%s", pad, n.Op, n.Typ.String(), PrintNodeTree(n.Expr, indent+1))

	case *loneanalyzer.TCallExpr:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s└── TCallExpr [type=%s]\n", pad, n.Typ.String()))
		sb.WriteString(fmt.Sprintf("%s  ├── Callee:\n%s", pad, PrintNodeTree(n.Callee, indent+2)))
		for i, a := range n.Args {
			sb.WriteString(fmt.Sprintf("%s  ├── Arg[%d]:\n%s", pad, i, PrintNodeTree(a, indent+2)))
		}
		return sb.String()

	case *loneanalyzer.TLiteralExpr:
		return fmt.Sprintf("%s└── TLiteralExpr [val=%v, type=%s]\n", pad, n.Value, n.Typ.String())

	case *loneanalyzer.TVarRefExpr:
		return fmt.Sprintf("%s└── TVarRefExpr [name=%q, type=%s]\n", pad, n.Name, n.Typ.String())

	case *loneanalyzer.TObjectExpr:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s└── TObjectExpr [type=%s]\n", pad, n.Typ.String()))
		for _, f := range n.Fields {
			sb.WriteString(fmt.Sprintf("%s  ├── .%s:\n%s", pad, f.Name, PrintNodeTree(f.Expr, indent+2)))
		}
		return sb.String()

	case *loneanalyzer.TListExpr:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s└── TListExpr [type=%s]\n", pad, n.Typ.String()))
		for i, it := range n.Items {
			sb.WriteString(fmt.Sprintf("%s  ├── [%d]:\n%s", pad, i, PrintNodeTree(it, indent+2)))
		}
		return sb.String()

	case *loneanalyzer.THostImport:
		return fmt.Sprintf("%s├── THostImport [module=%q, name=%q, local=%q, return=%s]\n",
			pad, n.WasmModule, n.WasmName, n.LocalName, n.ReturnType.String())

	case *loneanalyzer.TTypeDec:
		return fmt.Sprintf("%s├── TTypeDec [name=%q, type=%s]\n", pad, n.Name, n.Typ.String())

	case *loneanalyzer.TAssignStmt:
		return fmt.Sprintf("%s├── TAssignStmt\n%s%s", pad, PrintNodeTree(n.Target, indent+1), PrintNodeTree(n.Value, indent+1))

	case *loneanalyzer.TIndexExpr:
		return fmt.Sprintf("%s└── TIndexExpr [type=%s]\n%s%s", pad, n.Typ.String(), PrintNodeTree(n.Base, indent+1), PrintNodeTree(n.Index, indent+1))

	case *loneanalyzer.TMemberExpr:
		return fmt.Sprintf("%s└── TMemberExpr [member=%q, offset=%d, type=%s]\n%s", pad, n.Member, n.Offset, n.Typ.String(), PrintNodeTree(n.Base, indent+1))

	case *loneanalyzer.TStructLitExpr:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s└── TStructLitExpr [type=%s]\n", pad, n.StructType.Name))
		for _, f := range n.Fields {
			sb.WriteString(fmt.Sprintf("%s  ├── .%s:\n%s", pad, f.Name, PrintNodeTree(f.Expr, indent+2)))
		}
		return sb.String()

	case *loneanalyzer.TTraitMatExpr:
		return fmt.Sprintf("%s└── TTraitMatExpr [type=%s]\n", pad, n.Typ.String())

	default:
		return fmt.Sprintf("%s└── %T\n", pad, node)
	}
}

// FormatDiagnostic returns a user-readable string for a diagnostic.
func FormatDiagnostic(d loneanalyzer.Diagnostic) string {
	return fmt.Sprintf("[%s] %d:%d (%s): %s", d.Severity, d.Pos.Line, d.Pos.Col, d.Rule, d.Message)
}

// FormatDiagnostics formats multiple diagnostics joined by newlines.
func FormatDiagnostics(diags []loneanalyzer.Diagnostic) string {
	var lines []string
	for _, d := range diags {
		lines = append(lines, FormatDiagnostic(d))
	}
	return strings.Join(lines, "\n")
}

// PrintProgram pretty-prints a Typed IR program with concrete type annotations.
func PrintProgram(prog *loneanalyzer.TProgram) string {
	if prog == nil {
		return "<nil program>"
	}

	var sb strings.Builder
	for i, decl := range prog.Decls {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(PrintNode(decl, 0))
	}
	return sb.String()
}

// PrintNode pretty-prints any Typed IR node at a given indentation level.
func PrintNode(node loneanalyzer.TNode, indent int) string {
	if node == nil {
		return ""
	}

	pad := strings.Repeat("  ", indent)

	switch n := node.(type) {
	case *loneanalyzer.TVarDec:
		return fmt.Sprintf("%svar %s: %s = %s", pad, n.Name, n.Type.String(), PrintExpr(n.Value))

	case *loneanalyzer.THostImport:
		return fmt.Sprintf("%simport %s.%s as %s", pad, n.WasmModule, n.WasmName, n.LocalName)

	case *loneanalyzer.TTypeDec:
		return fmt.Sprintf("%stype %s = %s", pad, n.Name, n.Typ.String())

	case *loneanalyzer.TAssignStmt:
		return fmt.Sprintf("%s%s = %s", pad, PrintExpr(n.Target), PrintExpr(n.Value))

	case *loneanalyzer.TFnDec:
		var params []string
		for _, p := range n.Params {
			params = append(params, fmt.Sprintf("%s: %s", p.Name, p.Type.String()))
		}
		sig := fmt.Sprintf("%sfn %s(%s) %s {\n", pad, n.Name, strings.Join(params, ", "), n.ReturnType.String())
		for _, stmt := range n.Body.Stmts {
			sig += PrintNode(stmt, indent+1) + "\n"
		}
		sig += fmt.Sprintf("%s}", pad)
		return sig

	case *loneanalyzer.TReturnStmt:
		if n.Value != nil {
			return fmt.Sprintf("%sreturn %s", pad, PrintExpr(n.Value))
		}
		return fmt.Sprintf("%sreturn", pad)

	case *loneanalyzer.TDropStmt:
		return fmt.Sprintf("%sdrop %s", pad, PrintExpr(n.Expr))

	case *loneanalyzer.TBreakStmt:
		if n.Label != "" && n.Value != nil {
			return fmt.Sprintf("%sbreak %s: %s", pad, n.Label, PrintExpr(n.Value))
		} else if n.Label != "" {
			return fmt.Sprintf("%sbreak %s:", pad, n.Label)
		} else if n.Value != nil {
			return fmt.Sprintf("%sbreak %s", pad, PrintExpr(n.Value))
		}
		return fmt.Sprintf("%sbreak", pad)

	case *loneanalyzer.TExprStmt:
		return fmt.Sprintf("%s%s", pad, PrintExpr(n.Expr))

	case *loneanalyzer.TWhileStmt:
		res := fmt.Sprintf("%swhile %s {\n", pad, PrintExpr(n.Cond))
		for _, s := range n.Body.Stmts {
			res += PrintNode(s, indent+1) + "\n"
		}
		res += fmt.Sprintf("%s}", pad)
		return res

	case *loneanalyzer.TDoWhileStmt:
		res := fmt.Sprintf("%sloop {\n", pad)
		for _, s := range n.Body.Stmts {
			res += PrintNode(s, indent+1) + "\n"
		}
		res += fmt.Sprintf("%s} while: [%s]", pad, PrintExpr(n.Cond))
		return res

	case *loneanalyzer.TIfStmt:
		res := fmt.Sprintf("%sif %s {\n", pad, PrintExpr(n.Cond))
		for _, s := range n.Then.Stmts {
			res += PrintNode(s, indent+1) + "\n"
		}
		res += fmt.Sprintf("%s}", pad)
		if n.Else != nil {
			res += " else {\n"
			for _, s := range n.Else.Stmts {
				res += PrintNode(s, indent+1) + "\n"
			}
			res += fmt.Sprintf("%s}", pad)
		}
		return res

	case *loneanalyzer.TLabelDec:
		res := fmt.Sprintf("%s%s: {\n", pad, n.Name)
		for _, s := range n.Body.Stmts {
			res += PrintNode(s, indent+1) + "\n"
		}
		res += fmt.Sprintf("%s}", pad)
		return res

	default:
		if expr, ok := node.(loneanalyzer.TExpr); ok {
			return fmt.Sprintf("%s%s", pad, PrintExpr(expr))
		}
		return fmt.Sprintf("%s<unknown node %T>", pad, node)
	}
}

// PrintExpr prints a Typed IR expression with its value/operator representation.
func PrintExpr(expr loneanalyzer.TExpr) string {
	if expr == nil {
		return "<nil>"
	}

	switch e := expr.(type) {
	case *loneanalyzer.TLiteralExpr:
		return fmt.Sprintf("%v", e.Value)

	case *loneanalyzer.TVarRefExpr:
		return e.Name

	case *loneanalyzer.TBinaryExpr:
		return fmt.Sprintf("(%s %s %s)", PrintExpr(e.Left), e.Op, PrintExpr(e.Right))

	case *loneanalyzer.TIsExpr:
		opStr := "is"
		if e.Negated {
			opStr = "is not"
		}
		return fmt.Sprintf("(%s %s %s)", PrintExpr(e.Expr), opStr, e.TargetType.String())

	case *loneanalyzer.TUnaryExpr:
		return fmt.Sprintf("%s%s", e.Op, PrintExpr(e.Expr))

	case *loneanalyzer.TCallExpr:
		var args []string
		for _, a := range e.Args {
			args = append(args, PrintExpr(a))
		}
		return fmt.Sprintf("%s(%s)", PrintExpr(e.Callee), strings.Join(args, ", "))

	case *loneanalyzer.TObjectExpr:
		var fields []string
		for _, f := range e.Fields {
			fields = append(fields, fmt.Sprintf(".%s: %s", f.Name, PrintExpr(f.Expr)))
		}
		return fmt.Sprintf("{ %s }", strings.Join(fields, ", "))

	case *loneanalyzer.TListExpr:
		var items []string
		for _, item := range e.Items {
			items = append(items, PrintExpr(item))
		}
		return fmt.Sprintf("{ %s }", strings.Join(items, ", "))

	case *loneanalyzer.TDropStmt:
		return fmt.Sprintf("drop %s", PrintExpr(e.Expr))

	case *loneanalyzer.TIndexExpr:
		return fmt.Sprintf("%s[%s]", PrintExpr(e.Base), PrintExpr(e.Index))

	case *loneanalyzer.TMemberExpr:
		return fmt.Sprintf("%s.%s", PrintExpr(e.Base), e.Member)

	case *loneanalyzer.TStructLitExpr:
		var fields []string
		for _, f := range e.Fields {
			fields = append(fields, fmt.Sprintf(".%s: %s", f.Name, PrintExpr(f.Expr)))
		}
		return fmt.Sprintf("%s::{ %s }", e.StructType.Name, strings.Join(fields, ", "))

	case *loneanalyzer.TTraitMatExpr:
		return e.Typ.String()

	default:
		return fmt.Sprintf("<expr %T: %s>", expr, expr.ExprType().String())
	}
}
