package lonelsp_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	lonelsp "lone/pkg2/lone-lsp"
)

func TestLspInitialize(t *testing.T) {
	clientRead, serverWrite := io.Pipe()
	serverRead, clientWrite := io.Pipe()

	server := lonelsp.NewServer()
	go server.Serve(serverRead, serverWrite)

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
	if !strings.Contains(msg, "cannot return value of type") {
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

	code := `
type Point = struct {
    x: i32,
    y: i32,
    pub fn new() Point { return Point::{ .x: 0, .y: 0 } }
}

type IfcReader = interface {
    ro count: i32,
}

type Status = enum {
    Ready,
    Error(code: i32),
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
				"uri": "file:///symbols.lone",
				"languageId": "lone",
				"version": 1,
				"text": %q
			}
		}
	}`, code)
	sendLspMessage(t, clientWrite, didOpen)
	_ = readLspMessage(t, clientRead) // drain diagnostics

	symReq := `{"jsonrpc":"2.0","id":2,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":"file:///symbols.lone"}}}`
	sendLspMessage(t, clientWrite, symReq)

	resp := readLspMessage(t, clientRead)
	var r lonelsp.Response
	if err := json.Unmarshal(resp, &r); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	syms, ok := r.Result.([]any)
	if !ok || len(syms) != 4 {
		t.Fatalf("expected 4 document symbols, got %d (%v)", len(syms), r.Result)
	}

	clientWrite.Close()
	serverWrite.Close()
}

func TestLspHoverAndCompletion(t *testing.T) {
	clientRead, serverWrite := io.Pipe()
	serverRead, clientWrite := io.Pipe()

	server := lonelsp.NewServer()
	go server.Serve(serverRead, serverWrite)

	didOpen := `{
		"jsonrpc": "2.0",
		"method": "textDocument/didOpen",
		"params": {
			"textDocument": {
				"uri": "file:///test.lone",
				"languageId": "lone",
				"version": 1,
				"text": "fn test() void {\n    println(\"hello\")\n}\n"
			}
		}
	}`
	sendLspMessage(t, clientWrite, didOpen)
	_ = readLspMessage(t, clientRead) // drain diagnostics

	// Hover on println
	hoverReq := `{"jsonrpc":"2.0","id":3,"method":"textDocument/hover","params":{"textDocument":{"uri":"file:///test.lone"},"position":{"line":1,"character":6}}}`
	sendLspMessage(t, clientWrite, hoverReq)

	resp := readLspMessage(t, clientRead)
	var r lonelsp.Response
	if err := json.Unmarshal(resp, &r); err != nil {
		t.Fatalf("failed to unmarshal hover response: %v", err)
	}
	resMap, ok := r.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected hover result map, got %v", r.Result)
	}
	contents := resMap["contents"].(map[string]any)
	val := contents["value"].(string)
	if !strings.Contains(val, "println") {
		t.Errorf("expected hover text to mention println, got %s", val)
	}

	// Completion
	compReq := `{"jsonrpc":"2.0","id":4,"method":"textDocument/completion","params":{"textDocument":{"uri":"file:///test.lone"},"position":{"line":1,"character":4}}}`
	sendLspMessage(t, clientWrite, compReq)

	respComp := readLspMessage(t, clientRead)
	var rComp lonelsp.Response
	if err := json.Unmarshal(respComp, &rComp); err != nil {
		t.Fatalf("failed to unmarshal completion response: %v", err)
	}
	compMap := rComp.Result.(map[string]any)
	items := compMap["items"].([]any)
	if len(items) == 0 {
		t.Errorf("expected non-empty completions list")
	}

	clientWrite.Close()
	serverWrite.Close()
}

func sendLspMessage(t *testing.T, w io.Writer, jsonBody string) {
	t.Helper()
	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(jsonBody), jsonBody)
	if _, err := w.Write([]byte(msg)); err != nil {
		t.Fatalf("failed to write LSP message: %v", err)
	}
}

func readLspMessage(t *testing.T, r io.Reader) []byte {
	t.Helper()
	reader := bufio.NewReader(r)

	contentLength := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("failed to read LSP header: %v", err)
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
		t.Fatalf("did not find valid Content-Length header")
	}

	buf := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, buf); err != nil {
		t.Fatalf("failed to read LSP payload: %v", err)
	}
	return buf
}
