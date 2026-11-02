package adapter_test

import (
	"strings"
	"testing"

	"lone/pkg/adapter"
)

func TestParseAdapterKind(t *testing.T) {
	tests := []struct {
		input   string
		want    adapter.AdapterKind
		wantErr bool
	}{
		{"node", adapter.AdapterNode, false},
		{"Node", adapter.AdapterNode, false},
		{"nodejs", adapter.AdapterNode, false},
		{"browser", adapter.AdapterBrowser, false},
		{"Browser", adapter.AdapterBrowser, false},
		{"web", adapter.AdapterBrowser, false},
		{"unknown", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		got, err := adapter.ParseAdapterKind(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseAdapterKind(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseAdapterKind(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateNodeAdapter(t *testing.T) {
	code, err := adapter.GenerateAdapter(adapter.AdapterNode, "hello.wasm")
	if err != nil {
		t.Fatalf("GenerateAdapter failed: %v", err)
	}

	if !strings.Contains(code, "createEnv(getMemory, hooks = {})") {
		t.Errorf("missing createEnv in node adapter:\n%s", code)
	}
	if !strings.Contains(code, "process.stdout.write(text)") {
		t.Errorf("missing process.stdout.write in node adapter:\n%s", code)
	}
	if !strings.Contains(code, "module.exports = { instantiate, run, createEnv }") {
		t.Errorf("missing module.exports in node adapter:\n%s", code)
	}
	if !strings.Contains(code, `"hello.wasm"`) {
		t.Errorf("missing hello.wasm reference in node adapter:\n%s", code)
	}
}

func TestGenerateBrowserAdapter(t *testing.T) {
	code, err := adapter.GenerateAdapter(adapter.AdapterBrowser, "app.wasm")
	if err != nil {
		t.Fatalf("GenerateAdapter failed: %v", err)
	}

	if !strings.Contains(code, "export function createEnv") {
		t.Errorf("missing export function createEnv in browser adapter:\n%s", code)
	}
	if !strings.Contains(code, "WebAssembly.instantiateStreaming") {
		t.Errorf("missing instantiateStreaming in browser adapter:\n%s", code)
	}
	if !strings.Contains(code, "console.log(text)") {
		t.Errorf("missing console.log in browser adapter:\n%s", code)
	}
	if !strings.Contains(code, `"app.wasm"`) {
		t.Errorf("missing app.wasm reference in browser adapter:\n%s", code)
	}
}
