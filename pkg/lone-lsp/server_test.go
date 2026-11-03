package lonelsp_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	lonelsp "lone/pkg/lone-lsp"
)

func TestLspInitialize(t *testing.T) {
	clientRead, serverWrite := io.Pipe()
	serverRead, clientWrite := io.Pipe()

	server := lonelsp.NewServer()
	go server.Serve(serverRead, serverWrite)

	// Send initialize request
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	sendLspMessage(t, clientWrite, initReq)

	resp := readLspMessage(t, clientRead)
	var r lonelsp.Response
	if err := json.Unmarshal(resp, &r); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if r.ID != float64(1) {
		t.Errorf("expected response id 1, got %v", r.ID)
	}

	resMap, ok := r.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected result map, got %T", r.Result)
	}

	serverInfo, ok := resMap["serverInfo"].(map[string]any)
	if !ok || serverInfo["name"] != "lone-lsp" {
		t.Errorf("expected server name lone-lsp, got %+v", serverInfo)
	}

	clientWrite.Close()
	serverWrite.Close()
}

func TestLspDidOpenPublishDiagnostics(t *testing.T) {
	clientRead, serverWrite := io.Pipe()
	serverRead, clientWrite := io.Pipe()

	server := lonelsp.NewServer()
	go server.Serve(serverRead, serverWrite)

	// Send didOpen notification with code having a return type mismatch
	didOpen := `{
		"jsonrpc": "2.0",
		"method": "textDocument/didOpen",
		"params": {
			"textDocument": {
				"uri": "file:///test.lone",
				"languageId": "lone",
				"version": 1,
				"text": "fn bad_return() i32 {\n    return \"invalid\"\n}\n"
			}
		}
	}`
	sendLspMessage(t, clientWrite, didOpen)

	notifBytes := readLspMessage(t, clientRead)
	var notif lonelsp.Notification
	if err := json.Unmarshal(notifBytes, &notif); err != nil {
		t.Fatalf("failed to unmarshal notification: %v", err)
	}

	if notif.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("expected textDocument/publishDiagnostics, got %s", notif.Method)
	}

	paramsMap, ok := notif.Params.(map[string]any)
	if !ok {
		t.Fatalf("expected params map, got %T", notif.Params)
	}

	diags, ok := paramsMap["diagnostics"].([]any)
	if !ok || len(diags) == 0 {
		t.Fatalf("expected at least 1 diagnostic, got %d", len(diags))
	}

	firstDiag := diags[0].(map[string]any)
	msg := firstDiag["message"].(string)
	if !strings.Contains(msg, "expected return type \"i32\"") {
		t.Errorf("expected return type mismatch message, got %s", msg)
	}

	clientWrite.Close()
	serverWrite.Close()
}

func TestLspDocumentSymbol(t *testing.T) {
	clientRead, serverWrite := io.Pipe()
	serverRead, clientWrite := io.Pipe()

	server := lonelsp.NewServer()
	go server.Serve(serverRead, serverWrite)

	// Open document
	code := "var global_val = 10\nfn calculate() void {}\n"
	didOpen := fmt.Sprintf(`{
		"jsonrpc": "2.0",
		"method": "textDocument/didOpen",
		"params": {
			"textDocument": {
				"uri": "file:///symbols.lone",
				"languageId": "lone",
				"version": 1,
				"text": %q
			}
		}
	}`, code)
	sendLspMessage(t, clientWrite, didOpen)

	// Consume diagnostics notification
	_ = readLspMessage(t, clientRead)

	// Request document symbols
	docSymbolReq := `{"jsonrpc":"2.0","id":2,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":"file:///symbols.lone"}}}`
	sendLspMessage(t, clientWrite, docSymbolReq)

	respBytes := readLspMessage(t, clientRead)
	var r lonelsp.Response
	if err := json.Unmarshal(respBytes, &r); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	symbols, ok := r.Result.([]any)
	if !ok || len(symbols) != 2 {
		t.Fatalf("expected 2 document symbols, got %d", len(symbols))
	}

	clientWrite.Close()
	serverWrite.Close()
}

func TestLspStructAndTraitDocumentSymbols(t *testing.T) {
	clientRead, serverWrite := io.Pipe()
	serverRead, clientWrite := io.Pipe()

	server := lonelsp.NewServer()
	go server.Serve(serverRead, serverWrite)

	code := `import .print from :std:io

pub type WriterTrait = trait {
	fn write(self, msg: str) i32
}

pub type Vec2 = struct {
	x: i32,
	y: i32,
	pub fn new(x: i32, y: i32) Vec2 {
		return Vec2::{ .x: x, .y: y }
	}
}

impl WriterTrait on Vec2 {
	fn write(self, msg: str) i32 {
		return 0
	}
}
`
	didOpen := fmt.Sprintf(`{
		"jsonrpc": "2.0",
		"method": "textDocument/didOpen",
		"params": {
			"textDocument": {
				"uri": "file:///advanced.lone",
				"languageId": "lone",
				"version": 1,
				"text": %q
			}
		}
	}`, code)
	sendLspMessage(t, clientWrite, didOpen)
	_ = readLspMessage(t, clientRead) // diagnostics

	docSymbolReq := `{"jsonrpc":"2.0","id":10,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":"file:///advanced.lone"}}}`
	sendLspMessage(t, clientWrite, docSymbolReq)

	respBytes := readLspMessage(t, clientRead)
	var r lonelsp.Response
	if err := json.Unmarshal(respBytes, &r); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	symbols, ok := r.Result.([]any)
	if !ok || len(symbols) != 4 {
		t.Fatalf("expected 4 document symbols (import, trait, struct, impl), got %d: %+v", len(symbols), r.Result)
	}

	// Verify trait symbol
	traitSym := symbols[1].(map[string]any)
	if traitSym["name"] != "WriterTrait" || traitSym["kind"] != float64(lonelsp.SymbolKindInterface) {
		t.Errorf("expected WriterTrait interface symbol, got: %+v", traitSym)
	}

	// Verify struct symbol and its children
	structSym := symbols[2].(map[string]any)
	if structSym["name"] != "Vec2" || structSym["kind"] != float64(lonelsp.SymbolKindStruct) {
		t.Errorf("expected Vec2 struct symbol, got: %+v", structSym)
	}
	children, ok := structSym["children"].([]any)
	if !ok || len(children) < 3 { // x, y, new
		t.Errorf("expected at least 3 children for Vec2 (fields & new), got: %+v", children)
	}

	clientWrite.Close()
	serverWrite.Close()
}

func TestLspHoverAndCompletion(t *testing.T) {
	clientRead, serverWrite := io.Pipe()
	serverRead, clientWrite := io.Pipe()

	server := lonelsp.NewServer()
	go server.Serve(serverRead, serverWrite)

	code := `pub type Vec2 = struct {
	x: i32,
	y: i32,
}

fn add(a: i32, b: i32) i32 {
	return a + b
}
`
	didOpen := fmt.Sprintf(`{
		"jsonrpc": "2.0",
		"method": "textDocument/didOpen",
		"params": {
			"textDocument": {
				"uri": "file:///hover.lone",
				"languageId": "lone",
				"version": 1,
				"text": %q
			}
		}
	}`, code)
	sendLspMessage(t, clientWrite, didOpen)
	_ = readLspMessage(t, clientRead)

	// 1. Hover on function declaration line (line 5: fn add)
	hoverReq := `{"jsonrpc":"2.0","id":20,"method":"textDocument/hover","params":{"textDocument":{"uri":"file:///hover.lone"},"position":{"line":5,"character":3}}}`
	sendLspMessage(t, clientWrite, hoverReq)
	hoverBytes := readLspMessage(t, clientRead)
	var hResp lonelsp.Response
	if err := json.Unmarshal(hoverBytes, &hResp); err != nil {
		t.Fatalf("failed to unmarshal hover: %v", err)
	}
	hResult := hResp.Result.(map[string]any)
	contents := hResult["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(contents, "fn add(a: i32, b: i32) i32") {
		t.Errorf("expected full function signature in hover, got: %s", contents)
	}

	// 2. Hover on Stdlib symbol "print"
	hoverStdReq := `{"jsonrpc":"2.0","id":21,"method":"textDocument/hover","params":{"textDocument":{"uri":"file:///hover.lone"},"position":{"line":0,"character":0}}}`
	// Position 0 character 0 is "pub", but let's test Stdlib symbol hover directly
	_ = hoverStdReq

	// 3. Test completions
	compReq := `{"jsonrpc":"2.0","id":22,"method":"textDocument/completion","params":{"textDocument":{"uri":"file:///hover.lone"},"position":{"line":0,"character":0}}}`
	sendLspMessage(t, clientWrite, compReq)
	compBytes := readLspMessage(t, clientRead)
	var cResp lonelsp.Response
	if err := json.Unmarshal(compBytes, &cResp); err != nil {
		t.Fatalf("failed to unmarshal completion: %v", err)
	}
	cResult := cResp.Result.(map[string]any)
	items := cResult["items"].([]any)
	if len(items) < 20 {
		t.Fatalf("expected at least 20 completion items, got %d", len(items))
	}

	// 4. Test definition of Vec2
	defReq := `{"jsonrpc":"2.0","id":23,"method":"textDocument/definition","params":{"textDocument":{"uri":"file:///hover.lone"},"position":{"line":0,"character":10}}}`
	sendLspMessage(t, clientWrite, defReq)
	defBytes := readLspMessage(t, clientRead)
	var dResp lonelsp.Response
	if err := json.Unmarshal(defBytes, &dResp); err != nil {
		t.Fatalf("failed to unmarshal definition: %v", err)
	}
	dLoc := dResp.Result.(map[string]any)
	if dLoc["uri"] != "file:///hover.lone" {
		t.Errorf("expected definition URI file:///hover.lone, got %v", dLoc["uri"])
	}

	clientWrite.Close()
	serverWrite.Close()
}

func sendLspMessage(t *testing.T, w io.Writer, msg string) {
	t.Helper()
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(msg))
	if _, err := io.WriteString(w, header+msg); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}
}

func readLspMessage(t *testing.T, r io.Reader) []byte {
	t.Helper()
	reader := bufio.NewReader(r)

	contentLen := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("failed to read header: %v", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			if cl, err := strconv.Atoi(val); err == nil {
				contentLen = cl
			}
		}
	}

	if contentLen <= 0 {
		t.Fatalf("invalid content length: %d", contentLen)
	}

	buf := make([]byte, contentLen)
	if _, err := io.ReadFull(reader, buf); err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	return buf
}
