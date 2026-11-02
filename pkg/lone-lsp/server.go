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
			},
			ServerInfo: ServerInfo{
				Name:    "lone-lsp",
				Version: "0.2.0",
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
		// Semantic analysis
		analyzer := loneanalyzer.NewAnalyzer(fileName, loneanalyzer.DefaultConfig())
		_, diags := analyzer.AnalyzeTyped(rawAst)

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
		pair, ok := item.([]any)
		if !ok || len(pair) < 2 {
			continue
		}

		switch d := pair[1].(type) {
		case loneast.FN_DEC:
			line := d.Pos.Line - 1
			if line < 0 {
				line = 0
			}
			symbols = append(symbols, DocumentSymbol{
				Name: d.Name,
				Kind: SymbolKindFunction,
				Range: Range{
					Start: Position{Line: line, Character: 0},
					End:   Position{Line: line, Character: len(d.Name)},
				},
				SelectionRange: Range{
					Start: Position{Line: line, Character: 0},
					End:   Position{Line: line, Character: len(d.Name)},
				},
			})
		case loneast.BIND_DEC:
			line := d.Pos.Line - 1
			if line < 0 {
				line = 0
			}
			symbols = append(symbols, DocumentSymbol{
				Name: d.Name,
				Kind: SymbolKindVariable,
				Range: Range{
					Start: Position{Line: line, Character: 0},
					End:   Position{Line: line, Character: len(d.Name)},
				},
				SelectionRange: Range{
					Start: Position{Line: line, Character: 0},
					End:   Position{Line: line, Character: len(d.Name)},
				},
			})
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

	// Simple hover extraction: return declaration signature if cursor on a declared function
	rawAst, err := loneparser.Parse(uriToPath(uri), []byte(text))
	if err != nil {
		return nil
	}

	rawList, ok := rawAst.([]any)
	if !ok {
		return nil
	}

	for _, item := range rawList {
		pair, ok := item.([]any)
		if !ok || len(pair) < 2 {
			continue
		}
		if fn, ok := pair[1].(loneast.FN_DEC); ok {
			if fn.Pos.Line-1 == pos.Line {
				ret := "void"
				if fn.ReturnType != nil && fn.ReturnType.Value != nil {
					ret = fmt.Sprintf("%v", fn.ReturnType.Value)
				}
				sig := fmt.Sprintf("```lone\nfn %s(...) %s\n```", fn.Name, ret)
				return &Hover{
					Contents: MarkupContent{
						Kind:  "markdown",
						Value: sig,
					},
				}
			}
		}
	}

	return nil
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
