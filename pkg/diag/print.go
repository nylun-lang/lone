package diag

import (
	"encoding/json"
	"fmt"
	"strings"

	"lone/pkg/sema"
	
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
func PrintNodeTree(node sema.TNode, indent int) string {
	if node == nil {
		return "<nil>\n"
	}

	pad := strings.Repeat("  ", indent)

	switch n := node.(type) {
	case *sema.TProgram:
		var sb strings.Builder
		sb.WriteString(pad + "TProgram\n")
		for _, d := range n.Decls {
			sb.WriteString(PrintNodeTree(d, indent+1))
		}
		return sb.String()

	case *sema.TVarDec:
		return fmt.Sprintf("%s├── TVarDec [name=%q, type=%s]\n%s", pad, n.Name, n.Type.String(), PrintNodeTree(n.Value, indent+1))

	case *sema.TFnDec:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s├── TFnDec [name=%q, return=%s]\n", pad, n.Name, n.ReturnType.String()))
		for _, p := range n.Params {
			sb.WriteString(fmt.Sprintf("%s  ├── TFnParam [name=%q, type=%s]\n", pad, p.Name, p.Type.String()))
		}
		sb.WriteString(PrintNodeTree(&n.Body, indent+1))
		return sb.String()

	case *sema.TBlock:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s├── TBlock [stmts=%d]\n", pad, len(n.Stmts)))
		for _, s := range n.Stmts {
			sb.WriteString(PrintNodeTree(s, indent+1))
		}
		return sb.String()

	case *sema.TReturnStmt:
		if n.Value != nil {
			return fmt.Sprintf("%s├── TReturnStmt\n%s", pad, PrintNodeTree(n.Value, indent+1))
		}
		return fmt.Sprintf("%s├── TReturnStmt (void)\n", pad)

	case *sema.TDropStmt:
		return fmt.Sprintf("%s├── TDropStmt\n%s", pad, PrintNodeTree(n.Expr, indent+1))

	case *sema.TBreakStmt:
		return fmt.Sprintf("%s├── TBreakStmt [label=%q]\n", pad, n.Label)

	case *sema.TExprStmt:
		return fmt.Sprintf("%s├── TExprStmt\n%s", pad, PrintNodeTree(n.Expr, indent+1))

	case *sema.TWhileStmt:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s├── TWhileStmt\n", pad))
		sb.WriteString(fmt.Sprintf("%s  ├── Cond:\n%s", pad, PrintNodeTree(n.Cond, indent+2)))
		sb.WriteString(PrintNodeTree(&n.Body, indent+1))
		return sb.String()

	case *sema.TIfStmt:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s├── TIfStmt\n", pad))
		sb.WriteString(fmt.Sprintf("%s  ├── Cond:\n%s", pad, PrintNodeTree(n.Cond, indent+2)))
		sb.WriteString(PrintNodeTree(&n.Then, indent+1))
		if n.Else != nil {
			sb.WriteString(fmt.Sprintf("%s  ├── Else:\n", pad))
			sb.WriteString(PrintNodeTree(n.Else, indent+2))
		}
		return sb.String()

	case *sema.TLabelDec:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s├── TLabelDec [name=%q]\n", pad, n.Name))
		sb.WriteString(PrintNodeTree(&n.Body, indent+1))
		return sb.String()

	case *sema.TBinaryExpr:
		return fmt.Sprintf("%s└── TBinaryExpr [op=%s, type=%s]\n%s%s", pad, n.Op, n.Typ.String(), PrintNodeTree(n.Left, indent+1), PrintNodeTree(n.Right, indent+1))

	case *sema.TUnaryExpr:
		return fmt.Sprintf("%s└── TUnaryExpr [op=%s, type=%s]\n%s", pad, n.Op, n.Typ.String(), PrintNodeTree(n.Expr, indent+1))

	case *sema.TCallExpr:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s└── TCallExpr [type=%s]\n", pad, n.Typ.String()))
		sb.WriteString(fmt.Sprintf("%s  ├── Callee:\n%s", pad, PrintNodeTree(n.Callee, indent+2)))
		for i, a := range n.Args {
			sb.WriteString(fmt.Sprintf("%s  ├── Arg[%d]:\n%s", pad, i, PrintNodeTree(a, indent+2)))
		}
		return sb.String()

	case *sema.TLiteralExpr:
		return fmt.Sprintf("%s└── TLiteralExpr [val=%v, type=%s]\n", pad, n.Value, n.Typ.String())

	case *sema.TVarRefExpr:
		return fmt.Sprintf("%s└── TVarRefExpr [name=%q, type=%s]\n", pad, n.Name, n.Typ.String())

	case *sema.TObjectExpr:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s└── TObjectExpr [type=%s]\n", pad, n.Typ.String()))
		for _, f := range n.Fields {
			sb.WriteString(fmt.Sprintf("%s  ├── .%s:\n%s", pad, f.Name, PrintNodeTree(f.Expr, indent+2)))
		}
		return sb.String()

	case *sema.TListExpr:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s└── TListExpr [type=%s]\n", pad, n.Typ.String()))
		for i, it := range n.Items {
			sb.WriteString(fmt.Sprintf("%s  ├── [%d]:\n%s", pad, i, PrintNodeTree(it, indent+2)))
		}
		return sb.String()

	default:
		return fmt.Sprintf("%s└── %T\n", pad, node)
	}
}

// FormatDiagnostic returns a user-readable string for a diagnostic.
func FormatDiagnostic(d sema.Diagnostic) string {
	return fmt.Sprintf("[%s] %d:%d (%s): %s", d.Severity, d.Pos.Line, d.Pos.Col, d.Rule, d.Message)
}

// FormatDiagnostics formats multiple diagnostics joined by newlines.
func FormatDiagnostics(diags []sema.Diagnostic) string {
	var lines []string
	for _, d := range diags {
		lines = append(lines, FormatDiagnostic(d))
	}
	return strings.Join(lines, "\n")
}

// PrintProgram pretty-prints a Typed IR program with concrete type annotations.
func PrintProgram(prog *sema.TProgram) string {
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
func PrintNode(node sema.TNode, indent int) string {
	if node == nil {
		return ""
	}

	pad := strings.Repeat("  ", indent)

	switch n := node.(type) {
	case *sema.TVarDec:
		return fmt.Sprintf("%svar %s: %s = %s", pad, n.Name, n.Type.String(), PrintExpr(n.Value))

	case *sema.TFnDec:
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

	case *sema.TReturnStmt:
		if n.Value != nil {
			return fmt.Sprintf("%sreturn %s", pad, PrintExpr(n.Value))
		}
		return fmt.Sprintf("%sreturn", pad)

	case *sema.TDropStmt:
		return fmt.Sprintf("%sdrop %s", pad, PrintExpr(n.Expr))

	case *sema.TBreakStmt:
		if n.Label != "" && n.Value != nil {
			return fmt.Sprintf("%sbreak %s: %s", pad, n.Label, PrintExpr(n.Value))
		} else if n.Label != "" {
			return fmt.Sprintf("%sbreak %s:", pad, n.Label)
		} else if n.Value != nil {
			return fmt.Sprintf("%sbreak %s", pad, PrintExpr(n.Value))
		}
		return fmt.Sprintf("%sbreak", pad)

	case *sema.TExprStmt:
		return fmt.Sprintf("%s%s", pad, PrintExpr(n.Expr))

	case *sema.TWhileStmt:
		res := fmt.Sprintf("%swhile %s {\n", pad, PrintExpr(n.Cond))
		for _, s := range n.Body.Stmts {
			res += PrintNode(s, indent+1) + "\n"
		}
		res += fmt.Sprintf("%s}", pad)
		return res

	case *sema.TIfStmt:
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

	case *sema.TLabelDec:
		res := fmt.Sprintf("%s%s: {\n", pad, n.Name)
		for _, s := range n.Body.Stmts {
			res += PrintNode(s, indent+1) + "\n"
		}
		res += fmt.Sprintf("%s}", pad)
		return res

	default:
		if expr, ok := node.(sema.TExpr); ok {
			return fmt.Sprintf("%s%s", pad, PrintExpr(expr))
		}
		return fmt.Sprintf("%s<unknown node %T>", pad, node)
	}
}

// PrintExpr prints a Typed IR expression with its value/operator representation.
func PrintExpr(expr sema.TExpr) string {
	if expr == nil {
		return "<nil>"
	}

	switch e := expr.(type) {
	case *sema.TLiteralExpr:
		return fmt.Sprintf("%v", e.Value)

	case *sema.TVarRefExpr:
		return e.Name

	case *sema.TBinaryExpr:
		return fmt.Sprintf("(%s %s %s)", PrintExpr(e.Left), e.Op, PrintExpr(e.Right))

	case *sema.TUnaryExpr:
		return fmt.Sprintf("%s%s", e.Op, PrintExpr(e.Expr))

	case *sema.TCallExpr:
		var args []string
		for _, a := range e.Args {
			args = append(args, PrintExpr(a))
		}
		return fmt.Sprintf("%s(%s)", PrintExpr(e.Callee), strings.Join(args, ", "))

	case *sema.TObjectExpr:
		var fields []string
		for _, f := range e.Fields {
			fields = append(fields, fmt.Sprintf(".%s: %s", f.Name, PrintExpr(f.Expr)))
		}
		return fmt.Sprintf("{ %s }", strings.Join(fields, ", "))

	case *sema.TListExpr:
		var items []string
		for _, item := range e.Items {
			items = append(items, PrintExpr(item))
		}
		return fmt.Sprintf("{ %s }", strings.Join(items, ", "))

	case *sema.TDropStmt:
		return fmt.Sprintf("drop %s", PrintExpr(e.Expr))

	default:
		return fmt.Sprintf("<expr %T: %s>", expr, expr.ExprType().String())
	}
}
