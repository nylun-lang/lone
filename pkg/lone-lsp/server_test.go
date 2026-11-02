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
