package loneast

import (
	lonelexer "lone/pkg2/lone-lexer"
)

type Position = lonelexer.Position
type Span = lonelexer.Span

// Node represents any AST node in the Lone abstract syntax tree.
type Node interface {
	NodeSpan() Span
}
