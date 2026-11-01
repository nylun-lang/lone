package print

import (
	"fmt"
	"strings"

	"lonealyzer/analyze"
	air "lonealyzer/irtypes"
)

// FormatDiagnostic returns a user-readable string for a diagnostic.
func FormatDiagnostic(d analyze.Diagnostic) string {
	return fmt.Sprintf("[%s] %d:%d (%s): %s", d.Severity, d.Pos.Line, d.Pos.Col, d.Rule, d.Message)
}

// FormatDiagnostics formats multiple diagnostics joined by newlines.
func FormatDiagnostics(diags []analyze.Diagnostic) string {
	var lines []string
	for _, d := range diags {
		lines = append(lines, FormatDiagnostic(d))
	}
	return strings.Join(lines, "\n")
}

// PrintProgram pretty-prints a Typed IR program with concrete type annotations.
func PrintProgram(prog *air.TProgram) string {
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
func PrintNode(node air.TNode, indent int) string {
	if node == nil {
		return ""
	}

	pad := strings.Repeat("  ", indent)

	switch n := node.(type) {
	case *air.TVarDec:
		return fmt.Sprintf("%svar %s: %s = %s", pad, n.Name, n.Type.String(), PrintExpr(n.Value))

	case *air.TFnDec:
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

	case *air.TReturnStmt:
		if n.Value != nil {
			return fmt.Sprintf("%sreturn %s", pad, PrintExpr(n.Value))
		}
		return fmt.Sprintf("%sreturn", pad)

	case *air.TDropStmt:
		return fmt.Sprintf("%sdrop %s", pad, PrintExpr(n.Expr))

	case *air.TBreakStmt:
		if n.Label != "" && n.Value != nil {
			return fmt.Sprintf("%sbreak %s: %s", pad, n.Label, PrintExpr(n.Value))
		} else if n.Label != "" {
			return fmt.Sprintf("%sbreak %s:", pad, n.Label)
		} else if n.Value != nil {
			return fmt.Sprintf("%sbreak %s", pad, PrintExpr(n.Value))
		}
		return fmt.Sprintf("%sbreak", pad)

	case *air.TExprStmt:
		return fmt.Sprintf("%s%s", pad, PrintExpr(n.Expr))

	case *air.TWhileStmt:
		res := fmt.Sprintf("%swhile %s {\n", pad, PrintExpr(n.Cond))
		for _, s := range n.Body.Stmts {
			res += PrintNode(s, indent+1) + "\n"
		}
		res += fmt.Sprintf("%s}", pad)
		return res

	case *air.TIfStmt:
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

	case *air.TLabelDec:
		res := fmt.Sprintf("%s%s: {\n", pad, n.Name)
		for _, s := range n.Body.Stmts {
			res += PrintNode(s, indent+1) + "\n"
		}
		res += fmt.Sprintf("%s}", pad)
		return res

	default:
		if expr, ok := node.(air.TExpr); ok {
			return fmt.Sprintf("%s%s", pad, PrintExpr(expr))
		}
		return fmt.Sprintf("%s<unknown node %T>", pad, node)
	}
}

// PrintExpr prints a Typed IR expression with its value/operator representation.
func PrintExpr(expr air.TExpr) string {
	if expr == nil {
		return "<nil>"
	}

	switch e := expr.(type) {
	case *air.TLiteralExpr:
		return fmt.Sprintf("%v", e.Value)

	case *air.TVarRefExpr:
		return e.Name

	case *air.TBinaryExpr:
		return fmt.Sprintf("(%s %s %s)", PrintExpr(e.Left), e.Op, PrintExpr(e.Right))

	case *air.TUnaryExpr:
		return fmt.Sprintf("%s%s", e.Op, PrintExpr(e.Expr))

	case *air.TCallExpr:
		var args []string
		for _, a := range e.Args {
			args = append(args, PrintExpr(a))
		}
		return fmt.Sprintf("%s(%s)", PrintExpr(e.Callee), strings.Join(args, ", "))

	case *air.TObjectExpr:
		var fields []string
		for _, f := range e.Fields {
			fields = append(fields, fmt.Sprintf(".%s: %s", f.Name, PrintExpr(f.Expr)))
		}
		return fmt.Sprintf("{ %s }", strings.Join(fields, ", "))

	case *air.TListExpr:
		var items []string
		for _, item := range e.Items {
			items = append(items, PrintExpr(item))
		}
		return fmt.Sprintf("{ %s }", strings.Join(items, ", "))

	case *air.TDropStmt:
		return fmt.Sprintf("drop %s", PrintExpr(e.Expr))

	default:
		return fmt.Sprintf("<expr %T: %s>", expr, expr.ExprType().String())
	}
}
