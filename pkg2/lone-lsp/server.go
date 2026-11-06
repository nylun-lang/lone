package lonelsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"unicode"

	loneanalyzer "lone/pkg2/lone-analyzer"
	loneast "lone/pkg2/lone-parser-ast"
	loneparser "lone/pkg2/lone-parser"
)

// Server implements a JSON-RPC 2.0 Language Server Protocol daemon for Lone.
type Server struct {
	mu        sync.RWMutex
	documents map[string]string // URI -> document text
	writerMu  sync.Mutex
	writer    io.Writer
}

// NewServer creates an initialized Server instance.
func NewServer() *Server {
	return &Server{
		documents: make(map[string]string),
	}
}

// Serve reads JSON-RPC 2.0 messages from in and writes responses to out until EOF.
func (s *Server) Serve(in io.Reader, out io.Writer) error {
	s.writerMu.Lock()
	s.writer = out
	s.writerMu.Unlock()

	reader := bufio.NewReader(in)

	for {
		// Read headers (e.g. Content-Length: 123\r\n\r\n)
		contentLength := -1

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if strings.HasPrefix(line, "Content-Length:") {
				val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
				if n, err := strconv.Atoi(val); err == nil {
					contentLength = n
				}
			}
		}

		if contentLength < 0 {
			continue
		}

		// Read payload
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		s.handleMessage(body)
	}
}

func (s *Server) sendResponse(id any, result any, err *RPCError) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   err,
	}
	s.writeMessage(resp)
}

func (s *Server) sendNotification(method string, params any) {
	pBytes, _ := json.Marshal(params)
	raw := json.RawMessage(pBytes)
	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  &raw,
	}
	s.writeMessage(notif)
}

func (s *Server) writeMessage(msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))

	s.writerMu.Lock()
	defer s.writerMu.Unlock()
	if s.writer != nil {
		_, _ = s.writer.Write([]byte(header))
		_, _ = s.writer.Write(data)
	}
}

func (s *Server) handleMessage(data []byte) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return
	}

	if req.ID == nil {
		s.handleNotification(req)
		return
	}

	s.handleRequest(req)
}

func (s *Server) handleRequest(req Request) {
	switch req.Method {
	case "initialize":
		result := InitializeResult{
			Capabilities: ServerCapabilities{
				TextDocumentSync:       1, // Full sync
				HoverProvider:          true,
				CompletionProvider:     true,
				DefinitionProvider:     true,
				DocumentSymbolProvider: true,
			},
			ServerInfo: ServerInfo{
				Name:    "lone-lsp",
				Version: "0.6.0",
			},
		}
		s.sendResponse(req.ID, result, nil)

	case "shutdown":
		s.sendResponse(req.ID, nil, nil)

	case "textDocument/hover":
		var params HoverParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: "Invalid params"})
			return
		}
		hover := s.handleHover(params.TextDocument.URI, params.Position)
		s.sendResponse(req.ID, hover, nil)

	case "textDocument/completion":
		var params CompletionParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: "Invalid params"})
			return
		}
		completions := s.handleCompletion(params.TextDocument.URI, params.Position)
		s.sendResponse(req.ID, completions, nil)

	case "textDocument/definition":
		var params DefinitionParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: "Invalid params"})
			return
		}
		loc := s.handleDefinition(params.TextDocument.URI, params.Position)
		s.sendResponse(req.ID, loc, nil)

	case "textDocument/documentSymbol":
		var params DocumentSymbolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: "Invalid params"})
			return
		}
		symbols := s.handleDocumentSymbols(params.TextDocument.URI)
		s.sendResponse(req.ID, symbols, nil)

	default:
		s.sendResponse(req.ID, nil, &RPCError{Code: -32601, Message: "Method not found"})
	}
}

func (s *Server) handleNotification(req Request) {
	switch req.Method {
	case "initialized":
		// Ready

	case "textDocument/didOpen":
		var params DidOpenTextDocumentParams
		if err := json.Unmarshal(req.Params, &params); err == nil {
			s.mu.Lock()
			s.documents[params.TextDocument.URI] = params.TextDocument.Text
			s.mu.Unlock()
			s.validateDocument(params.TextDocument.URI, params.TextDocument.Text)
		}

	case "textDocument/didChange":
		var params DidChangeTextDocumentParams
		if err := json.Unmarshal(req.Params, &params); err == nil {
			if len(params.ContentChanges) > 0 {
				newText := params.ContentChanges[len(params.ContentChanges)-1].Text
				s.mu.Lock()
				s.documents[params.TextDocument.URI] = newText
				s.mu.Unlock()
				s.validateDocument(params.TextDocument.URI, newText)
			}
		}

	case "textDocument/didClose":
		var params DidCloseTextDocumentParams
		if err := json.Unmarshal(req.Params, &params); err == nil {
			s.mu.Lock()
			delete(s.documents, params.TextDocument.URI)
			s.mu.Unlock()
		}
	}
}

func (s *Server) validateDocument(uri string, text string) {
	analyzer := loneanalyzer.NewAnalyzer()
	_, diags, err := analyzer.AnalyzeSource([]byte(text))

	lspDiags := make([]Diagnostic, 0)

	if err != nil {
		lspDiags = append(lspDiags, Diagnostic{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 0, Character: 1},
			},
			Severity: SeverityError,
			Source:   "lone-parser",
			Message:  err.Error(),
		})
	}

	for _, d := range diags {
		line := d.Span.Start.Line - 1
		if line < 0 {
			line = 0
		}
		col := d.Span.Start.Col - 1
		if col < 0 {
			col = 0
		}
		endCol := d.Span.End.Col - 1
		if endCol <= col {
			endCol = col + 1
		}

		sev := SeverityError
		if d.Severity == loneanalyzer.SeverityWarning {
			sev = SeverityWarning
		} else if d.Severity == loneanalyzer.SeverityHint {
			sev = SeverityHint
		}

		lspDiags = append(lspDiags, Diagnostic{
			Range: Range{
				Start: Position{Line: line, Character: col},
				End:   Position{Line: line, Character: endCol},
			},
			Severity: sev,
			Code:     d.Rule,
			Source:   "lone-analyzer",
			Message:  d.Message,
		})
	}

	params := PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: lspDiags,
	}
	s.sendNotification("textDocument/publishDiagnostics", params)
}

func (s *Server) handleDocumentSymbols(uri string) []DocumentSymbol {
	s.mu.RLock()
	text, exists := s.documents[uri]
	s.mu.RUnlock()
	if !exists {
		return nil
	}

	decls, err := loneparser.Parse([]byte(text))
	if err != nil {
		return nil
	}

	symbols := make([]DocumentSymbol, 0)
	for _, decl := range decls {
		switch d := decl.(type) {
		case loneast.FN_DEC:
			symbols = append(symbols, fnToSymbol(&d))
		case loneast.TYPE_DEC:
			symbols = append(symbols, typeDecToSymbol(&d))
		case loneast.BIND_DEC:
			symbols = append(symbols, bindToSymbol(&d))
		case loneast.IMPL_DEC:
			symbols = append(symbols, implToSymbol(&d))
		}
	}

	return symbols
}

func (s *Server) handleHover(uri string, pos Position) *Hover {
	s.mu.RLock()
	text, exists := s.documents[uri]
	s.mu.RUnlock()
	if !exists {
		return nil
	}

	ident := getIdentifierAtPosition(text, pos)
	if ident == "" {
		return nil
	}

	// 1. Builtin/Keyword hover
	if doc, ok := lookupKeywordDoc(ident); ok {
		return &Hover{
			Contents: MarkupContent{
				Kind:  "markdown",
				Value: doc,
			},
		}
	}

	// 2. Standard library hover
	if stdDoc, ok := lookupStdDoc(ident); ok {
		return &Hover{
			Contents: MarkupContent{
				Kind:  "markdown",
				Value: stdDoc,
			},
		}
	}

	// 3. User declarations hover
	decls, err := loneparser.Parse([]byte(text))
	if err != nil {
		return nil
	}

	for _, decl := range decls {
		switch d := decl.(type) {
		case loneast.FN_DEC:
			if d.Name == ident {
				return &Hover{
					Contents: MarkupContent{
						Kind:  "markdown",
						Value: fmt.Sprintf("```lone\nfn %s(...)\n```\nFunction declaration", d.Name),
					},
				}
			}
		case loneast.TYPE_DEC:
			if d.Name == ident {
				kindStr := "type"
				switch d.Kind {
				case loneast.TYPE_DEC_STRUCT:
					kindStr = "struct"
				case loneast.TYPE_DEC_INTERFACE:
					kindStr = "interface"
				case loneast.TYPE_DEC_ENUM:
					kindStr = "enum"
				case loneast.TYPE_DEC_TRAIT:
					kindStr = "trait"
				}
				return &Hover{
					Contents: MarkupContent{
						Kind:  "markdown",
						Value: fmt.Sprintf("```lone\ntype %s = %s { ... }\n```\nDefined in this document", d.Name, kindStr),
					},
				}
			}
		}
	}

	return nil
}

func (s *Server) handleCompletion(uri string, pos Position) CompletionList {
	items := make([]CompletionItem, 0)

	// Keywords
	keywords := []struct {
		kw     string
		detail string
	}{
		{"import", "import .symbol from :namespace"},
		{"from", "Module import source"},
		{"as", "Namespace alias"},
		{"pub", "Public symbol visibility"},
		{"var", "Mutable variable declaration"},
		{"let", "Immutable/inferred binding"},
		{"type", "Type definition"},
		{"struct", "Struct type definition"},
		{"interface", "Interface data contract definition"},
		{"enum", "Algebraic tagged union enum"},
		{"trait", "Trait contract definition"},
		{"impl", "Trait implementation on type"},
		{"fn", "Function declaration"},
		{"return", "Return from function"},
		{"break", "Break from loop or block"},
		{"if", "Conditional branch"},
		{"else", "Alternative conditional branch"},
		{"while", "While loop statement"},
		{"loop", "Loop statement"},
		{"match", "Exhaustive pattern matching"},
		{"must", "Unwrap Result::Ok or panic with unreachable"},
		{"try", "Early-return Result::Err or unwrap Ok"},
		{"drop", "Explicitly discard return value"},
		{"ro", "Read-only modifier"},
		{"where", "Generic constraint specification"},
		{"satisfies", "Interface/Trait contract satisfaction"},
		{"is", "Type/variant match constraint"},
		{"not", "Logical or constraint negation"},
		{"true", "Boolean true literal"},
		{"false", "Boolean false literal"},
		{"none", "Absence of a value"},
	}
	for _, kw := range keywords {
		items = append(items, CompletionItem{
			Label:  kw.kw,
			Kind:   CompletionItemKindKeyword,
			Detail: kw.detail,
		})
	}

	// Types
	types := []string{
		"i8", "i16", "i32", "i64", "i128",
		"u8", "u16", "u32", "u64", "u128",
		"f32", "f64", "int", "bool", "str", "char", "void", "any", "infer", "ptr", "ref",
	}
	for _, typ := range types {
		items = append(items, CompletionItem{
			Label:  typ,
			Kind:   CompletionItemKindClass,
			Detail: "Built-in Type",
		})
	}

	// Standard library
	stdSymbols := []struct {
		name   string
		detail string
	}{
		{"print", "fn(msg: str) void (:std:io)"},
		{"println", "fn(msg: str) void (:std:io)"},
		{"read_line", "fn() str (:std:io)"},
		{"parse_int", "fn(s: str) Result<i32> (:std:io)"},
		{"random_i32", "fn() i32 (:std:io)"},
		{"random_range", "fn(min: i32, max: i32) i32 (:std:io)"},
		{"Result", "enum Result<V, U = Error>"},
		{"Error", "enum Error"},
	}
	for _, sym := range stdSymbols {
		items = append(items, CompletionItem{
			Label:  sym.name,
			Kind:   CompletionItemKindFunction,
			Detail: sym.detail,
		})
	}

	return CompletionList{
		IsIncomplete: false,
		Items:        items,
	}
}

func (s *Server) handleDefinition(uri string, pos Position) *Location {
	s.mu.RLock()
	text, exists := s.documents[uri]
	s.mu.RUnlock()
	if !exists {
		return nil
	}

	ident := getIdentifierAtPosition(text, pos)
	if ident == "" {
		return nil
	}

	decls, err := loneparser.Parse([]byte(text))
	if err != nil {
		return nil
	}

	for _, decl := range decls {
		switch d := decl.(type) {
		case loneast.FN_DEC:
			if d.Name == ident {
				line := max(0, d.Span_.Start.Line-1)
				return &Location{
					URI: uri,
					Range: Range{
						Start: Position{Line: line, Character: 0},
						End:   Position{Line: line, Character: len(d.Name)},
					},
				}
			}
		case loneast.TYPE_DEC:
			if d.Name == ident {
				line := max(0, d.Span_.Start.Line-1)
				return &Location{
					URI: uri,
					Range: Range{
						Start: Position{Line: line, Character: 0},
						End:   Position{Line: line, Character: len(d.Name)},
					},
				}
			}
		}
	}

	return nil
}

// Helpers

func fnToSymbol(fn *loneast.FN_DEC) DocumentSymbol {
	line := max(0, fn.Span_.Start.Line-1)
	return DocumentSymbol{
		Name: fn.Name,
		Kind: SymbolKindFunction,
		Range: Range{
			Start: Position{Line: line, Character: 0},
			End:   Position{Line: line, Character: len(fn.Name)},
		},
		SelectionRange: Range{
			Start: Position{Line: line, Character: 0},
			End:   Position{Line: line, Character: len(fn.Name)},
		},
	}
}

func bindToSymbol(b *loneast.BIND_DEC) DocumentSymbol {
	line := max(0, b.Span_.Start.Line-1)
	kind := SymbolKindVariable
	if isAllCaps(b.Name) {
		kind = SymbolKindConstant
	}
	return DocumentSymbol{
		Name: b.Name,
		Kind: kind,
		Range: Range{
			Start: Position{Line: line, Character: 0},
			End:   Position{Line: line, Character: len(b.Name)},
		},
		SelectionRange: Range{
			Start: Position{Line: line, Character: 0},
			End:   Position{Line: line, Character: len(b.Name)},
		},
	}
}

func typeDecToSymbol(t *loneast.TYPE_DEC) DocumentSymbol {
	line := max(0, t.Span_.Start.Line-1)
	kind := SymbolKindStruct
	switch t.Kind {
	case loneast.TYPE_DEC_INTERFACE:
		kind = SymbolKindInterface
	case loneast.TYPE_DEC_ENUM:
		kind = SymbolKindEnum
	case loneast.TYPE_DEC_TRAIT:
		kind = SymbolKindInterface
	}

	children := make([]DocumentSymbol, 0)
	for _, f := range t.Fields {
		fLine := max(0, f.Span_.Start.Line-1)
		children = append(children, DocumentSymbol{
			Name: f.Name,
			Kind: SymbolKindField,
			Range: Range{
				Start: Position{Line: fLine, Character: 0},
				End:   Position{Line: fLine, Character: len(f.Name)},
			},
			SelectionRange: Range{
				Start: Position{Line: fLine, Character: 0},
				End:   Position{Line: fLine, Character: len(f.Name)},
			},
		})
	}
	for _, v := range t.Variants {
		vLine := max(0, v.Span_.Start.Line-1)
		children = append(children, DocumentSymbol{
			Name: v.Name,
			Kind: SymbolKindEnumMember,
			Range: Range{
				Start: Position{Line: vLine, Character: 0},
				End:   Position{Line: vLine, Character: len(v.Name)},
			},
			SelectionRange: Range{
				Start: Position{Line: vLine, Character: 0},
				End:   Position{Line: vLine, Character: len(v.Name)},
			},
		})
	}
	for _, m := range t.Methods {
		mLine := max(0, m.Span_.Start.Line-1)
		children = append(children, DocumentSymbol{
			Name: m.Name,
			Kind: SymbolKindMethod,
			Range: Range{
				Start: Position{Line: mLine, Character: 0},
				End:   Position{Line: mLine, Character: len(m.Name)},
			},
			SelectionRange: Range{
				Start: Position{Line: mLine, Character: 0},
				End:   Position{Line: mLine, Character: len(m.Name)},
			},
		})
	}

	return DocumentSymbol{
		Name:     t.Name,
		Kind:     kind,
		Children: children,
		Range: Range{
			Start: Position{Line: line, Character: 0},
			End:   Position{Line: line, Character: len(t.Name)},
		},
		SelectionRange: Range{
			Start: Position{Line: line, Character: 0},
			End:   Position{Line: line, Character: len(t.Name)},
		},
	}
}

func implToSymbol(i *loneast.IMPL_DEC) DocumentSymbol {
	line := max(0, i.Span_.Start.Line-1)
	name := fmt.Sprintf("impl %s", i.TraitName)
	return DocumentSymbol{
		Name: name,
		Kind: SymbolKindClass,
		Range: Range{
			Start: Position{Line: line, Character: 0},
			End:   Position{Line: line, Character: len(name)},
		},
		SelectionRange: Range{
			Start: Position{Line: line, Character: 0},
			End:   Position{Line: line, Character: len(name)},
		},
	}
}

func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		u, err := url.Parse(uri)
		if err == nil {
			return u.Path
		}
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}

func isAllCaps(s string) bool {
	if len(s) == 0 {
		return false
	}
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsUpper(r) {
				return false
			}
		}
	}
	return hasLetter
}

func getIdentifierAtPosition(text string, pos Position) string {
	lines := strings.Split(text, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return ""
	}
	line := lines[pos.Line]
	if pos.Character < 0 || pos.Character > len(line) {
		return ""
	}

	start := pos.Character
	for start > 0 && isIdentChar(rune(line[start-1])) {
		start--
	}

	end := pos.Character
	for end < len(line) && isIdentChar(rune(line[end])) {
		end++
	}

	if start == end {
		return ""
	}
	return line[start:end]
}

func isIdentChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func lookupKeywordDoc(name string) (string, bool) {
	docs := map[string]string{
		"fn":        "### `fn`\nDeclares a function or method:\n```lone\nfn add(a: i32, b: i32) i32 {\n    return a + b\n}\n```",
		"var":       "### `var`\nDeclares a mutable variable:\n```lone\nvar count: i32 = 0\ncount = count + 1\n```",
		"let":       "### `let`\nDeclares an immutable binding or destructuring:\n```lone\nlet [x, y]: i32 = 0\n```",
		"type":      "### `type`\nDeclares a named type definition (struct, interface, enum, trait, alias).",
		"struct":    "### `struct`\nDeclares a WebAssembly aggregate layout with physical fields and inherent methods.",
		"interface": "### `interface`\nDeclares a compile-time structural duck typing data contract.",
		"enum":      "### `enum`\nDeclares a tagged union algebraic data type with payload variants.",
		"must":      "### `must <expr>`\nUnwraps a `Result::Ok(val)` or panics immediately via WebAssembly `unreachable`.",
		"try":       "### `try <expr>`\nUnwraps a `Result::Ok(val)` or returns early from the function with `Result::Err`.",
		"match":     "### `match <expr> { ... }`\nPerforms exhaustive pattern matching over enum variants.",
		"ro":        "### `ro`\nSpecifies a read-only field or reference. Supports `ro(pub)` for public-read, struct-private-write fields.",
	}
	doc, ok := docs[name]
	return doc, ok
}

func lookupStdDoc(name string) (string, bool) {
	docs := map[string]string{
		"print":        "### `:std:io.print(msg: str) void`\nWrites a string to standard output without a trailing newline.",
		"println":      "### `:std:io.println(msg: str) void`\nWrites a string to standard output followed by a newline.",
		"read_line":    "### `:std:io.read_line() str`\nReads a single line from standard input.",
		"parse_int":    "### `:std:io.parse_int(s: str) Result<i32>`\nParses a decimal integer from a string, returning `Result::Ok(n)` or `Result::Err(Error::Generic)`.",
		"random_i32":   "### `:std:io.random_i32() i32`\nReturns a random 32-bit signed integer using WASI Preview 1 `random_get`.",
		"random_range": "### `:std:io.random_range(min: i32, max: i32) i32`\nReturns a pseudo-random integer between min and max (inclusive).",
		"Result":       "### `Result<V, U = Error>`\nStandard representation for fallible operations with `Ok(val: V)` and `Err(err: U)`.",
	}
	doc, ok := docs[name]
	return doc, ok
}
