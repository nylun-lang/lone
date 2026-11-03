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

	loneanalyzer "lone/pkg/lone-analyzer"
	loneproject "lone/pkg/lone-analyzer-project"
	loneast "lone/pkg/lone-parser-ast"
	loneparser "lone/pkg/lone-parser"
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
				// Empty line marks end of header section
				break
			}
			if strings.HasPrefix(line, "Content-Length:") {
				val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
				if cl, err := strconv.Atoi(val); err == nil {
					contentLength = cl
				}
			}
		}

		if contentLength <= 0 {
			continue
		}

		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			return err
		}

		var req Request
		if err := json.Unmarshal(body, &req); err != nil {
			continue
		}

		s.handleMessage(&req)
	}
}

func (s *Server) handleMessage(req *Request) {
	// If no ID, it is a notification
	if req.ID == nil {
		s.handleNotification(req)
		return
	}

	switch req.Method {
	case "initialize":
		res := InitializeResult{
			Capabilities: ServerCapabilities{
				TextDocumentSync:       1, // Full document sync
				HoverProvider:          true,
				DocumentSymbolProvider: true,
				CompletionProvider:     true,
				DefinitionProvider:     true,
			},
			ServerInfo: ServerInfo{
				Name:    "lone-lsp",
				Version: "0.3.0",
			},
		}
		s.sendResponse(req.ID, res, nil)

	case "shutdown":
		s.sendResponse(req.ID, map[string]any{}, nil)

	case "textDocument/documentSymbol":
		var params DocumentSymbolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: err.Error()})
			return
		}
		symbols := s.handleDocumentSymbols(params.TextDocument.URI)
		s.sendResponse(req.ID, symbols, nil)

	case "textDocument/hover":
		var params HoverParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: err.Error()})
			return
		}
		hover := s.handleHover(params.TextDocument.URI, params.Position)
		s.sendResponse(req.ID, hover, nil)

	case "textDocument/completion":
		var params CompletionParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: err.Error()})
			return
		}
		completions := s.handleCompletion(params.TextDocument.URI, params.Position)
		s.sendResponse(req.ID, completions, nil)

	case "textDocument/definition":
		var params DefinitionParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: err.Error()})
			return
		}
		loc := s.handleDefinition(params.TextDocument.URI, params.Position)
		s.sendResponse(req.ID, loc, nil)

	default:
		// Method not found
		s.sendResponse(req.ID, nil, &RPCError{Code: -32601, Message: fmt.Sprintf("method %s not found", req.Method)})
	}
}

func (s *Server) handleNotification(req *Request) {
	switch req.Method {
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
		if err := json.Unmarshal(req.Params, &params); err == nil && len(params.ContentChanges) > 0 {
			text := params.ContentChanges[len(params.ContentChanges)-1].Text
			s.mu.Lock()
			s.documents[params.TextDocument.URI] = text
			s.mu.Unlock()
			s.validateDocument(params.TextDocument.URI, text)
		}

	case "textDocument/didClose":
		var params struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(req.Params, &params); err == nil {
			s.mu.Lock()
			delete(s.documents, params.TextDocument.URI)
			s.mu.Unlock()
		}
	}
}

func (s *Server) validateDocument(uri string, text string) {
	fileName := uriToPath(uri)
	rawAst, err := loneparser.Parse(fileName, []byte(text))

	lspDiags := make([]Diagnostic, 0)

	if err != nil {
		// Syntax error from parser
		lspDiags = append(lspDiags, Diagnostic{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 0, Character: 1},
			},
			Severity: SeverityError,
			Source:   "lone-parser",
			Message:  err.Error(),
		})
	} else {
		// Module resolution & semantic analysis
		resolvedAst, resolveDiags := loneproject.ResolveAST(fileName, rawAst)
		analyzer := loneanalyzer.NewAnalyzer(fileName, loneanalyzer.DefaultConfig())
		_, diags := analyzer.AnalyzeTyped(resolvedAst)
		diags = append(resolveDiags, diags...)

		for _, d := range diags {
			line := d.Pos.Line - 1
			if line < 0 {
				line = 0
			}
			col := d.Pos.Col - 1
			if col < 0 {
				col = 0
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
					End:   Position{Line: line, Character: col + 1},
				},
				Severity: sev,
				Code:     d.Rule,
				Source:   "lone-analyzer",
				Message:  d.Message,
			})
		}
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

	rawAst, err := loneparser.Parse(uriToPath(uri), []byte(text))
	if err != nil {
		return nil
	}

	rawList, ok := rawAst.([]any)
	if !ok {
		return nil
	}

	var symbols []DocumentSymbol
	for _, item := range rawList {
		decl := unwrapDecl(item)
		if decl == nil {
			continue
		}

		switch d := decl.(type) {
		case loneast.FN_DEC:
			symbols = append(symbols, fnToSymbol(&d))
		case *loneast.FN_DEC:
			symbols = append(symbols, fnToSymbol(d))

		case loneast.BIND_DEC:
			symbols = append(symbols, bindToSymbol(&d))
		case *loneast.BIND_DEC:
			symbols = append(symbols, bindToSymbol(d))

		case loneast.TYPE_DEC:
			symbols = append(symbols, typeDecToSymbol(&d))
		case *loneast.TYPE_DEC:
			symbols = append(symbols, typeDecToSymbol(d))

		case loneast.IMPL_DEC:
			symbols = append(symbols, implDecToSymbol(&d))
		case *loneast.IMPL_DEC:
			symbols = append(symbols, implDecToSymbol(d))

		case loneast.ImportDecl:
			symbols = append(symbols, importToSymbol(&d))
		case *loneast.ImportDecl:
			symbols = append(symbols, importToSymbol(d))
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

	// 1. Check if hovering on a standard library symbol
	ident := wordAtPosition(text, pos)
	if stdDoc := getStdLibHover(ident); stdDoc != "" {
		return &Hover{
			Contents: MarkupContent{
				Kind:  "markdown",
				Value: stdDoc,
			},
		}
	}

	rawAst, err := loneparser.Parse(uriToPath(uri), []byte(text))
	if err != nil {
		return nil
	}

	rawList, ok := rawAst.([]any)
	if !ok {
		return nil
	}

	// 2. Check if hovering on or matching a declaration
	for _, item := range rawList {
		decl := unwrapDecl(item)
		if decl == nil {
			continue
		}

		switch d := decl.(type) {
		case loneast.FN_DEC:
			if h := hoverFn(&d, pos, ident); h != nil {
				return h
			}
		case *loneast.FN_DEC:
			if h := hoverFn(d, pos, ident); h != nil {
				return h
			}

		case loneast.BIND_DEC:
			if h := hoverBind(&d, pos, ident); h != nil {
				return h
			}
		case *loneast.BIND_DEC:
			if h := hoverBind(d, pos, ident); h != nil {
				return h
			}

		case loneast.TYPE_DEC:
			if h := hoverTypeDec(&d, pos, ident); h != nil {
				return h
			}
		case *loneast.TYPE_DEC:
			if h := hoverTypeDec(d, pos, ident); h != nil {
				return h
			}

		case loneast.IMPL_DEC:
			if h := hoverImplDec(&d, pos, ident); h != nil {
				return h
			}
		case *loneast.IMPL_DEC:
			if h := hoverImplDec(d, pos, ident); h != nil {
				return h
			}

		case loneast.ImportDecl:
			if d.Pos.Line-1 == pos.Line || (ident != "" && containsString(d.Symbols, ident)) {
				sig := fmt.Sprintf("```lone\nimport .%s from %s\n```", strings.Join(d.Symbols, ", ."), d.Source)
				return &Hover{Contents: MarkupContent{Kind: "markdown", Value: sig}}
			}
		case *loneast.ImportDecl:
			if d.Pos.Line-1 == pos.Line || (ident != "" && containsString(d.Symbols, ident)) {
				sig := fmt.Sprintf("```lone\nimport .%s from %s\n```", strings.Join(d.Symbols, ", ."), d.Source)
				return &Hover{Contents: MarkupContent{Kind: "markdown", Value: sig}}
			}
		}
	}

	return nil
}

func (s *Server) handleCompletion(uri string, pos Position) CompletionList {
	items := make([]CompletionItem, 0)

	// 1. Keywords
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
		{"trait", "Trait contract definition"},
		{"impl", "Trait implementation on type"},
		{"fn", "Function declaration"},
		{"return", "Return from function"},
		{"break", "Break from loop or block"},
		{"if", "Conditional branch"},
		{"else", "Alternative conditional branch"},
		{"while", "While loop statement"},
		{"drop", "Explicitly discard return value"},
		{"ro", "Read-only modifier"},
		{"comptime", "Compile-time modifier"},
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

	// 2. Builtin Types
	types := []string{
		"i8", "i16", "i32", "i64", "i128",
		"u8", "u16", "u32", "u64", "u128",
		"f32", "f64", "int", "bool", "str", "string", "char",
		"void", "any", "ptr", "ref",
	}
	for _, t := range types {
		items = append(items, CompletionItem{
			Label:  t,
			Kind:   CompletionItemKindClass,
			Detail: "Built-in Type",
		})
	}

	// 3. Standard Library Namespaces
	namespaces := []string{":std", ":std:io", ":std:sys"}
	for _, ns := range namespaces {
		items = append(items, CompletionItem{
			Label:  ns,
			Kind:   CompletionItemKindModule,
			Detail: "Standard Library Namespace",
		})
	}

	// 4. Standard Library Symbols
	stdSymbols := []struct {
		name   string
		kind   CompletionItemKind
		detail string
	}{
		{"print", CompletionItemKindFunction, "fn print(msg: str) void"},
		{"println", CompletionItemKindFunction, "fn println(msg: str) void"},
		{"eprint", CompletionItemKindFunction, "fn eprint(msg: str) void"},
		{"eprintln", CompletionItemKindFunction, "fn eprintln(msg: str) void"},
		{"read_line", CompletionItemKindFunction, "fn read_line(in: ptr<u8>) str"},
		{"str_len", CompletionItemKindFunction, "fn str_len(s: str) i32"},
		{"parse_int", CompletionItemKindFunction, "fn parse_int(s: str) i32"},
		{"int_to_str", CompletionItemKindFunction, "fn int_to_str(val: i32, buf: ptr<u8>) str"},
		{"print_int", CompletionItemKindFunction, "fn print_int(val: i32) void"},
		{"println_int", CompletionItemKindFunction, "fn println_int(val: i32) void"},
		{"random_i32", CompletionItemKindFunction, "fn random_i32() i32"},
		{"random_range", CompletionItemKindFunction, "fn random_range(min: i32, max: i32) i32"},
		{"Stdout", CompletionItemKindStruct, "type Stdout = struct { fd: i32 }"},
		{"Stderr", CompletionItemKindStruct, "type Stderr = struct { fd: i32 }"},
		{"Stdin", CompletionItemKindStruct, "type Stdin = struct { fd: i32 }"},
		{"WriterTrait", CompletionItemKindInterface, "pub type WriterTrait = trait { fn write(self, msg: str) i32 }"},
		{"ReaderTrait", CompletionItemKindInterface, "pub type ReaderTrait = trait { fn read(self, buf: ptr<u8>, max: i32) i32 }"},
	}
	for _, sym := range stdSymbols {
		items = append(items, CompletionItem{
			Label:  sym.name,
			Kind:   sym.kind,
			Detail: sym.detail,
		})
	}

	// 5. Symbols in Current Document
	docSymbols := s.handleDocumentSymbols(uri)
	for _, ds := range docSymbols {
		k := CompletionItemKindFunction
		if ds.Kind == SymbolKindVariable || ds.Kind == SymbolKindConstant {
			k = CompletionItemKindVariable
		} else if ds.Kind == SymbolKindStruct {
			k = CompletionItemKindStruct
		} else if ds.Kind == SymbolKindInterface {
			k = CompletionItemKindInterface
		}
		items = append(items, CompletionItem{
			Label:  ds.Name,
			Kind:   k,
			Detail: fmt.Sprintf("Declared in %s", uriToPath(uri)),
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

	ident := wordAtPosition(text, pos)
	if ident == "" {
		return nil
	}

	rawAst, err := loneparser.Parse(uriToPath(uri), []byte(text))
	if err != nil {
		return nil
	}

	rawList, ok := rawAst.([]any)
	if !ok {
		return nil
	}

	for _, item := range rawList {
		decl := unwrapDecl(item)
		if decl == nil {
			continue
		}

		switch d := decl.(type) {
		case loneast.FN_DEC:
			if d.Name == ident {
				line := max(0, d.Pos.Line-1)
				return &Location{
					URI: uri,
					Range: Range{
						Start: Position{Line: line, Character: 0},
						End:   Position{Line: line, Character: len(d.Name)},
					},
				}
			}
		case *loneast.FN_DEC:
			if d.Name == ident {
				line := max(0, d.Pos.Line-1)
				return &Location{
					URI: uri,
					Range: Range{
						Start: Position{Line: line, Character: 0},
						End:   Position{Line: line, Character: len(d.Name)},
					},
				}
			}
		case loneast.BIND_DEC:
			if d.Name == ident {
				line := max(0, d.Pos.Line-1)
				return &Location{
					URI: uri,
					Range: Range{
						Start: Position{Line: line, Character: 0},
						End:   Position{Line: line, Character: len(d.Name)},
					},
				}
			}
		case *loneast.BIND_DEC:
			if d.Name == ident {
				line := max(0, d.Pos.Line-1)
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
				line := max(0, d.Pos.Line-1)
				return &Location{
					URI: uri,
					Range: Range{
						Start: Position{Line: line, Character: 0},
						End:   Position{Line: line, Character: len(d.Name)},
					},
				}
			}
			for _, f := range d.Fields {
				if f.Name == ident {
					line := max(0, f.Pos.Line-1)
					return &Location{
						URI: uri,
						Range: Range{
							Start: Position{Line: line, Character: 0},
							End:   Position{Line: line, Character: len(f.Name)},
						},
					}
				}
			}
			for i := range d.Methods {
				if d.Methods[i].Name == ident {
					line := max(0, d.Methods[i].Pos.Line-1)
					return &Location{
						URI: uri,
						Range: Range{
							Start: Position{Line: line, Character: 0},
							End:   Position{Line: line, Character: len(d.Methods[i].Name)},
						},
					}
				}
			}
		case *loneast.TYPE_DEC:
			if d.Name == ident {
				line := max(0, d.Pos.Line-1)
				return &Location{
					URI: uri,
					Range: Range{
						Start: Position{Line: line, Character: 0},
						End:   Position{Line: line, Character: len(d.Name)},
					},
				}
			}
			for _, f := range d.Fields {
				if f.Name == ident {
					line := max(0, f.Pos.Line-1)
					return &Location{
						URI: uri,
						Range: Range{
							Start: Position{Line: line, Character: 0},
							End:   Position{Line: line, Character: len(f.Name)},
						},
					}
				}
			}
			for i := range d.Methods {
				if d.Methods[i].Name == ident {
					line := max(0, d.Methods[i].Pos.Line-1)
					return &Location{
						URI: uri,
						Range: Range{
							Start: Position{Line: line, Character: 0},
							End:   Position{Line: line, Character: len(d.Methods[i].Name)},
						},
					}
				}
			}
		}
	}

	return nil
}

// Helpers

func unwrapDecl(item any) any {
	if item == nil {
		return nil
	}
	switch n := item.(type) {
	case []any:
		if len(n) == 2 {
			return n[1]
		}
		if len(n) > 0 {
			return n[len(n)-1]
		}
	}
	return item
}

func formatTypeRef(tr *loneast.TYPE_REF) string {
	if tr == nil {
		return "void"
	}
	switch tr.Kind {
	case loneast.TYPE_BASE:
		base := tr.AsBase()
		if base == nil || base.Name == "" {
			return "infer"
		}
		if len(base.Args) == 0 {
			return base.Name
		}
		var args []string
		for i := range base.Args {
			args = append(args, formatTypeRef(&base.Args[i]))
		}
		return fmt.Sprintf("%s<%s>", base.Name, strings.Join(args, ", "))
	case loneast.TYPE_LIST:
		list := tr.AsList()
		if list == nil {
			return "[]any"
		}
		return fmt.Sprintf("[]%s", formatTypeRef(&list.ItemType))
	case loneast.TYPE_FN:
		fn := tr.AsFn()
		if fn == nil {
			return "fn()"
		}
		var params []string
		for _, p := range fn.Params {
			params = append(params, formatTypeRef(&p.Type))
		}
		ret := "void"
		if fn.ReturnType != nil {
			ret = formatTypeRef(fn.ReturnType)
		}
		return fmt.Sprintf("fn(%s) %s", strings.Join(params, ", "), ret)
	default:
		return "any"
	}
}

func fnToSymbol(fn *loneast.FN_DEC) DocumentSymbol {
	line := max(0, fn.Pos.Line-1)
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
	line := max(0, b.Pos.Line-1)
	kind := SymbolKindVariable
	if loneanalyzer.IsAllCaps(b.Name) {
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

func typeDecToSymbol(td *loneast.TYPE_DEC) DocumentSymbol {
	line := max(0, td.Pos.Line-1)
	kind := SymbolKindStruct
	if td.Kind == loneast.TYPE_DEC_TRAIT {
		kind = SymbolKindInterface
	} else if td.Kind == loneast.TYPE_DEC_ALIAS {
		kind = SymbolKindClass
	}

	sym := DocumentSymbol{
		Name: td.Name,
		Kind: kind,
		Range: Range{
			Start: Position{Line: line, Character: 0},
			End:   Position{Line: line, Character: len(td.Name)},
		},
		SelectionRange: Range{
			Start: Position{Line: line, Character: 0},
			End:   Position{Line: line, Character: len(td.Name)},
		},
	}

	for _, f := range td.Fields {
		fLine := max(0, f.Pos.Line-1)
		sym.Children = append(sym.Children, DocumentSymbol{
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

	for i := range td.Methods {
		m := &td.Methods[i]
		mLine := max(0, m.Pos.Line-1)
		mKind := SymbolKindMethod
		if m.Name == "new" || m.Name == "#init" {
			mKind = SymbolKindConstructor
		}
		sym.Children = append(sym.Children, DocumentSymbol{
			Name: m.Name,
			Kind: mKind,
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

	for _, sig := range td.TraitSigs {
		sLine := max(0, sig.Pos.Line-1)
		sym.Children = append(sym.Children, DocumentSymbol{
			Name: sig.Name,
			Kind: SymbolKindMethod,
			Range: Range{
				Start: Position{Line: sLine, Character: 0},
				End:   Position{Line: sLine, Character: len(sig.Name)},
			},
			SelectionRange: Range{
				Start: Position{Line: sLine, Character: 0},
				End:   Position{Line: sLine, Character: len(sig.Name)},
			},
		})
	}

	return sym
}

func implDecToSymbol(im *loneast.IMPL_DEC) DocumentSymbol {
	line := max(0, im.Pos.Line-1)
	targetName := formatTypeRef(&im.TargetType)
	name := fmt.Sprintf("impl %s on %s", im.TraitName, targetName)

	sym := DocumentSymbol{
		Name: name,
		Kind: SymbolKindNamespace,
		Range: Range{
			Start: Position{Line: line, Character: 0},
			End:   Position{Line: line, Character: len(name)},
		},
		SelectionRange: Range{
			Start: Position{Line: line, Character: 0},
			End:   Position{Line: line, Character: len(name)},
		},
	}

	for i := range im.Methods {
		m := &im.Methods[i]
		mLine := max(0, m.Pos.Line-1)
		sym.Children = append(sym.Children, DocumentSymbol{
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

	return sym
}

func importToSymbol(imp *loneast.ImportDecl) DocumentSymbol {
	line := max(0, imp.Pos.Line-1)
	name := fmt.Sprintf("import from %s", imp.Source)
	if len(imp.Symbols) > 0 {
		name = fmt.Sprintf("import .%s from %s", strings.Join(imp.Symbols, ", ."), imp.Source)
	}
	return DocumentSymbol{
		Name: name,
		Kind: SymbolKindModule,
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

func hoverFn(fn *loneast.FN_DEC, pos Position, ident string) *Hover {
	if fn.Pos.Line-1 == pos.Line || (ident != "" && fn.Name == ident) {
		var params []string
		for _, p := range fn.Params {
			params = append(params, fmt.Sprintf("%s: %s", p.Name, formatTypeRef(&p.Type)))
		}
		ret := "void"
		if fn.ReturnType != nil {
			ret = formatTypeRef(fn.ReturnType)
		}
		pubPrefix := ""
		if fn.IsPublic {
			pubPrefix = "pub "
		}
		sig := fmt.Sprintf("```lone\n%sfn %s(%s) %s\n```", pubPrefix, fn.Name, strings.Join(params, ", "), ret)
		return &Hover{Contents: MarkupContent{Kind: "markdown", Value: sig}}
	}
	return nil
}

func hoverBind(b *loneast.BIND_DEC, pos Position, ident string) *Hover {
	if b.Pos.Line-1 == pos.Line || (ident != "" && b.Name == ident) {
		typeStr := formatTypeRef(&b.Type)
		pubPrefix := ""
		if b.IsPublic {
			pubPrefix = "pub "
		}
		sig := fmt.Sprintf("```lone\n%svar %s: %s\n```", pubPrefix, b.Name, typeStr)
		return &Hover{Contents: MarkupContent{Kind: "markdown", Value: sig}}
	}
	return nil
}

func hoverTypeDec(td *loneast.TYPE_DEC, pos Position, ident string) *Hover {
	pubPrefix := ""
	if td.IsPublic {
		pubPrefix = "pub "
	}

	if td.Kind == loneast.TYPE_DEC_TRAIT {
		if td.Pos.Line-1 == pos.Line || (ident != "" && td.Name == ident) {
			var sigs []string
			for _, s := range td.TraitSigs {
				var pStrs []string
				for _, p := range s.Params {
					pStrs = append(pStrs, fmt.Sprintf("%s: %s", p.Name, formatTypeRef(&p.Type)))
				}
				rStr := "void"
				if s.ReturnType != nil {
					rStr = formatTypeRef(s.ReturnType)
				}
				sigs = append(sigs, fmt.Sprintf("    fn %s(%s) %s", s.Name, strings.Join(pStrs, ", "), rStr))
			}
			sig := fmt.Sprintf("```lone\n%stype %s = trait {\n%s\n}\n```", pubPrefix, td.Name, strings.Join(sigs, "\n"))
			return &Hover{Contents: MarkupContent{Kind: "markdown", Value: sig}}
		}
	} else if td.Kind == loneast.TYPE_DEC_STRUCT {
		if td.Pos.Line-1 == pos.Line || (ident != "" && td.Name == ident) {
			var fields []string
			for _, f := range td.Fields {
				fields = append(fields, fmt.Sprintf("    %s: %s", f.Name, formatTypeRef(&f.Type)))
			}
			for i := range td.Methods {
				m := &td.Methods[i]
				var pStrs []string
				for _, p := range m.Params {
					pStrs = append(pStrs, fmt.Sprintf("%s: %s", p.Name, formatTypeRef(&p.Type)))
				}
				rStr := "void"
				if m.ReturnType != nil {
					rStr = formatTypeRef(m.ReturnType)
				}
				fields = append(fields, fmt.Sprintf("    fn %s(%s) %s", m.Name, strings.Join(pStrs, ", "), rStr))
			}
			sig := fmt.Sprintf("```lone\n%stype %s = struct {\n%s\n}\n```", pubPrefix, td.Name, strings.Join(fields, "\n"))
			return &Hover{Contents: MarkupContent{Kind: "markdown", Value: sig}}
		}
		// Also check specific struct fields or methods
		for _, f := range td.Fields {
			if f.Pos.Line-1 == pos.Line || (ident != "" && f.Name == ident) {
				sig := fmt.Sprintf("```lone\n%s.%s: %s\n```", td.Name, f.Name, formatTypeRef(&f.Type))
				return &Hover{Contents: MarkupContent{Kind: "markdown", Value: sig}}
			}
		}
		for i := range td.Methods {
			m := &td.Methods[i]
			if m.Pos.Line-1 == pos.Line || (ident != "" && m.Name == ident) {
				return hoverFn(m, pos, ident)
			}
		}
	} else if td.Kind == loneast.TYPE_DEC_ALIAS {
		if td.Pos.Line-1 == pos.Line || (ident != "" && td.Name == ident) {
			sig := fmt.Sprintf("```lone\n%stype %s = %s\n```", pubPrefix, td.Name, formatTypeRef(td.AliasType))
			return &Hover{Contents: MarkupContent{Kind: "markdown", Value: sig}}
		}
	}

	return nil
}

func hoverImplDec(im *loneast.IMPL_DEC, pos Position, ident string) *Hover {
	targetName := formatTypeRef(&im.TargetType)
	if im.Pos.Line-1 == pos.Line {
		sig := fmt.Sprintf("```lone\nimpl %s on %s\n```", im.TraitName, targetName)
		return &Hover{Contents: MarkupContent{Kind: "markdown", Value: sig}}
	}
	for i := range im.Methods {
		m := &im.Methods[i]
		if m.Pos.Line-1 == pos.Line || (ident != "" && m.Name == ident) {
			return hoverFn(m, pos, ident)
		}
	}
	return nil
}

func wordAtPosition(text string, pos Position) string {
	lines := strings.Split(text, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return ""
	}
	line := lines[pos.Line]
	if pos.Character < 0 || pos.Character > len(line) {
		return ""
	}

	start := pos.Character
	for start > 0 && isIdentChar(line[start-1]) {
		start--
	}
	end := pos.Character
	for end < len(line) && isIdentChar(line[end]) {
		end++
	}
	if start >= end {
		return ""
	}
	return line[start:end]
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '#'
}

func containsString(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func getStdLibHover(name string) string {
	switch name {
	case "print":
		return "```lone\npub fn print(msg: str) void\n```\nWrites `msg` to standard output without a trailing newline."
	case "println":
		return "```lone\npub fn println(msg: str) void\n```\nWrites `msg` to standard output followed by a newline."
	case "eprint":
		return "```lone\npub fn eprint(msg: str) void\n```\nWrites `msg` to standard error without a trailing newline."
	case "eprintln":
		return "```lone\npub fn eprintln(msg: str) void\n```\nWrites `msg` to standard error followed by a newline."
	case "read_line":
		return "```lone\npub fn read_line(in: ptr<u8>) str\n```\nReads a line from standard input into a buffer."
	case "str_len":
		return "```lone\npub fn str_len(s: str) i32\n```\nReturns the byte length of string `s`."
	case "parse_int":
		return "```lone\npub fn parse_int(s: str) i32\n```\nParses a decimal integer from string `s`."
	case "int_to_str":
		return "```lone\npub fn int_to_str(val: i32, out_buf: ptr<u8>) str\n```\nFormats an integer into a decimal string buffer."
	case "print_int":
		return "```lone\npub fn print_int(val: i32) void\n```\nPrints an integer to standard output."
	case "println_int":
		return "```lone\npub fn println_int(val: i32) void\n```\nPrints an integer followed by a newline."
	case "random_i32":
		return "```lone\npub fn random_i32() i32\n```\nGenerates a 32-bit cryptographically secure random integer via WASI `random_get`."
	case "random_range":
		return "```lone\npub fn random_range(min: i32, max: i32) i32\n```\nGenerates a random integer uniformly distributed in `[min, max]`."
	case "Stdout":
		return "```lone\npub type Stdout = struct {\n    fd: i32,\n    pub fn new() Stdout\n}\n```\nStandard output stream handle (fd 1)."
	case "Stderr":
		return "```lone\npub type Stderr = struct {\n    fd: i32,\n    pub fn new() Stderr\n}\n```\nStandard error stream handle (fd 2)."
	case "Stdin":
		return "```lone\npub type Stdin = struct {\n    fd: i32,\n    pub fn new() Stdin\n}\n```\nStandard input stream handle (fd 0)."
	case "WriterTrait":
		return "```lone\npub type WriterTrait = trait {\n    fn write(self, msg: str) i32\n}\n```\nStream writing trait contract."
	case "ReaderTrait":
		return "```lone\npub type ReaderTrait = trait {\n    fn read(self, buf: ptr<u8>, max_len: i32) i32\n}\n```\nStream reading trait contract."
	default:
		return ""
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
	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	s.writeMessage(notif)
}

func (s *Server) writeMessage(msg any) {
	s.writerMu.Lock()
	defer s.writerMu.Unlock()

	if s.writer == nil {
		return
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	s.writer.Write([]byte(header))
	s.writer.Write(body)
}

func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Path == "" {
		return uri
	}
	return u.Path
}
