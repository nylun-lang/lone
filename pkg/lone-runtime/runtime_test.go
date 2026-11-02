package loneruntime_test

import (
	"testing"

	loneruntime "lone/pkg/lone-runtime"
)

func TestTableLookupWasi(t *testing.T) {
	tbl := loneruntime.NewDefaultTable()

	spec, ok := tbl.Lookup("#print", loneruntime.TargetWasi)
	if !ok {
		t.Fatalf("expected #print to be registered for wasi")
	}

	if spec.FunctionName != "$__lone_rt_print" {
		t.Errorf("expected function name $__lone_rt_print, got %s", spec.FunctionName)
	}

	if len(spec.Imports) != 1 || spec.Imports[0].Module != "wasi_snapshot_preview1" {
		t.Errorf("expected wasi_snapshot_preview1 import, got %+v", spec.Imports)
	}

	if spec.InlinedWatFunc == "" {
		t.Errorf("expected non-empty inlined WAT body for wasi #print")
	}
}

func TestTableLookupWeb(t *testing.T) {
	tbl := loneruntime.NewDefaultTable()

	spec, ok := tbl.Lookup("#print", loneruntime.TargetWeb)
	if !ok {
		t.Fatalf("expected #print to be registered for web")
	}

	if len(spec.Imports) != 1 || spec.Imports[0].Module != "env" {
		t.Errorf("expected env import for web, got %+v", spec.Imports)
	}

	if spec.InlinedWatFunc != "" {
		t.Errorf("expected web #print to be imported directly, not inlined")
	}
}
