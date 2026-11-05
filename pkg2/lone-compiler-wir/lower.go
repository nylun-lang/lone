package lonewir

import (
	"fmt"
	"strings"

	loneanalyzer "lone/pkg2/lone-analyzer"
	loneast "lone/pkg2/lone-parser-ast"
)

// LowerOptions configures the code generation behavior.
type LowerOptions struct {
	Target       string // "wasi" (default) or "web"
	MemoryPages  int
	MemoryExport string
}

// DefaultLowerOptions returns the default compilation target options.
func DefaultLowerOptions() LowerOptions {
	return LowerOptions{
		Target:       "wasi",
		MemoryPages:  2, // 128 KiB
		MemoryExport: "memory",
	}
}

type loopContext struct {
	label    string
	breakLbl string
	contLbl  string
}

type stringEntry struct {
	offset int
	length int
}

type varSlotInfo struct {
	offset int
	typ    loneanalyzer.Type
}

type lowerer struct {
	opts         LowerOptions
	mod          *WasmModule
	strings      map[string]stringEntry
	stringVarLen map[string]int
	nextOffset   int
	loopStack    []loopContext
	labelStack   map[string]string // user label -> wasm block label
	labelId       int
	usesPrint     bool
	usesStrlen    bool
	usesFormatI32 bool
	varSlots      map[string]varSlotInfo
	anonFnMap    map[string]string
	frameSize    int
}

// LowerProgram compiles a typed program AST directly into a structured WebAssembly AST.
func LowerProgram(prog *loneanalyzer.TProgram, opts LowerOptions) (*WasmModule, error) {
	if opts.MemoryPages <= 0 {
		opts.MemoryPages = 2
	}
	if opts.MemoryExport == "" {
		opts.MemoryExport = "memory"
	}

	l := &lowerer{
		opts:         opts,
		mod:          NewModule(),
		strings:      make(map[string]stringEntry),
		stringVarLen: make(map[string]int),
		nextOffset:   64, // [0..64) reserved for WASI iovec / runtime scratchpad
		labelStack:   make(map[string]string),
		varSlots:     make(map[string]varSlotInfo),
		anonFnMap:    make(map[string]string),
	}
	l.mod.MemoryPages = opts.MemoryPages
	l.mod.MemoryExport = opts.MemoryExport

	// Pass 1: Collect globals and functions
	var fnDecls []*loneanalyzer.TFnDec
	for _, decl := range prog.Decls {
		switch d := decl.(type) {
		case *loneanalyzer.TVarDec:
			wType := toWasmType(d.Type)
			initVal := l.formatGlobalInit(d.Value, d.Type)
			l.mod.Globals = append(l.mod.Globals, Global{
				Name:    "$g_" + d.Name,
				Type:    wType,
				Mutable: d.IsMutable,
				InitVal: initVal,
			})
		case *loneanalyzer.TFnDec:
			fnDecls = append(fnDecls, d)
		case *loneanalyzer.TImplDec:
			for _, m := range d.Methods {
				fnDecls = append(fnDecls, m)
			}
		case *loneanalyzer.TStructDec:
			for _, m := range d.Methods {
				fnDecls = append(fnDecls, m)
			}
		}
	}

	// Pass 2: Lower functions
	for _, fn := range fnDecls {
		wfn, err := l.lowerFunction(fn)
		if err != nil {
			return nil, err
		}
		l.mod.AddFunction(wfn)
	}

	// Pass 3: Setup runtime imports and helpers
	if l.usesPrint {
		if opts.Target == "web" {
			l.mod.Imports = append(l.mod.Imports, Import{
				Module: "env",
				Name:   "print",
				Alias:  "$__lone_rt_print",
				Params: []WasmType{TypeI32, TypeI32},
				Result: TypeVoid,
			})
		} else {
			// WASI target
			l.mod.Imports = append(l.mod.Imports, Import{
				Module: "wasi_snapshot_preview1",
				Name:   "fd_write",
				Alias:  "$__wasi_fd_write",
				Params: []WasmType{TypeI32, TypeI32, TypeI32, TypeI32},
				Result: TypeI32,
			})
			l.mod.InlinedRuntime = append(l.mod.InlinedRuntime, `  ;; Inlined WASI #print runtime implementation
  (func $__lone_rt_print (param $ptr i32) (param $len i32)
    (i32.store (i32.const 0) (local.get $ptr))
    (i32.store (i32.const 4) (local.get $len))
    (drop (call $__wasi_fd_write (i32.const 1) (i32.const 0) (i32.const 1) (i32.const 8)))
  )`)
		}
	}

	if l.usesStrlen {
		l.mod.InlinedRuntime = append(l.mod.InlinedRuntime, `  ;; Inlined runtime strlen
  (func $__lone_rt_strlen (param $ptr i32) (result i32)
    (local $curr i32)
    (local.set $curr (local.get $ptr))
    (block $done
      (loop $loop
        (i32.eqz (i32.load8_u (local.get $curr)))
        (br_if $done)
        (local.set $curr (i32.add (local.get $curr) (i32.const 1)))
        (br $loop)
      )
    )
    (i32.sub (local.get $curr) (local.get $ptr))
  )`)
	}

	if l.usesFormatI32 {
		l.mod.InlinedRuntime = append(l.mod.InlinedRuntime, `  ;; Inlined runtime: format_i32(val, buf) -> len
  (func $__lone_rt_format_i32 (param $val i32) (param $buf i32) (result i32)
    (local $start i32)
    (local $v i32)
    (local $d i32)
    (local $i i32)
    (local $j i32)
    (local $temp i32)
    local.get $buf
    local.set $start
    local.get $val
    local.set $v
    local.get $v
    i32.eqz
    if
      local.get $buf
      i32.const 48
      i32.store8
      i32.const 1
      return
    end
    local.get $v
    i32.const 0
    i32.lt_s
    if
      local.get $buf
      i32.const 45
      i32.store8
      local.get $buf
      i32.const 1
      i32.add
      local.set $buf
      i32.const 0
      local.get $v
      i32.sub
      local.set $v
    end
    local.get $buf
    local.set $i
    (block $done
      (loop $loop
        local.get $v
        i32.eqz
        br_if $done
        local.get $v
        i32.const 10
        i32.rem_u
        i32.const 48
        i32.add
        local.set $d
        local.get $i
        local.get $d
        i32.store8
        local.get $i
        i32.const 1
        i32.add
        local.set $i
        local.get $v
        i32.const 10
        i32.div_u
        local.set $v
        br $loop
      )
    )
    local.get $buf
    local.set $j
    local.get $i
    i32.const 1
    i32.sub
    local.set $temp
    (block $rev_done
      (loop $rev_loop
        local.get $j
        local.get $temp
        i32.ge_u
        br_if $rev_done
        local.get $j
        i32.load8_u
        local.set $d
        local.get $j
        local.get $temp
        i32.load8_u
        i32.store8
        local.get $temp
        local.get $d
        i32.store8
        local.get $j
        i32.const 1
        i32.add
        local.set $j
        local.get $temp
        i32.const 1
        i32.sub
        local.set $temp
        br $rev_loop
      )
    )
    local.get $i
    local.get $start
    i32.sub
  )`)
	}

	l.mod.InlinedRuntime = append(l.mod.InlinedRuntime, `  ;; Inlined runtime: memcpy(dst, src, len)
  (func $__lone_rt_memcpy (param $dst i32) (param $src i32) (param $len i32)
    (local $d i32)
    (local $s i32)
    (local $rem i32)
    local.get $dst
    local.set $d
    local.get $src
    local.set $s
    local.get $len
    local.set $rem
    (block $word_done
      (loop $word_loop
        local.get $rem
        i32.const 4
        i32.lt_s
        br_if $word_done
        local.get $d
        local.get $s
        i32.load
        i32.store
        local.get $d
        i32.const 4
        i32.add
        local.set $d
        local.get $s
        i32.const 4
        i32.add
        local.set $s
        local.get $rem
        i32.const 4
        i32.sub
        local.set $rem
        br $word_loop
      )
    )
    (block $byte_done
      (loop $byte_loop
        local.get $rem
        i32.const 0
        i32.le_s
        br_if $byte_done
        local.get $d
        local.get $s
        i32.load8_u
        i32.store8
        local.get $d
        i32.const 1
        i32.add
        local.set $d
        local.get $s
        i32.const 1
        i32.add
        local.set $s
        local.get $rem
        i32.const 1
        i32.sub
        local.set $rem
        br $byte_loop
      )
    )
  )

  ;; Inlined runtime: memset(dst, val, len)
  (func $__lone_rt_memset (param $dst i32) (param $val i32) (param $len i32)
    (local $d i32)
    (local $rem i32)
    local.get $dst
    local.set $d
    local.get $len
    local.set $rem
    (block $done
      (loop $loop
        local.get $rem
        i32.const 0
        i32.le_s
        br_if $done
        local.get $d
        local.get $val
        i32.store8
        local.get $d
        i32.const 1
        i32.add
        local.set $d
        local.get $rem
        i32.const 1
        i32.sub
        local.set $rem
        br $loop
      )
    )
  )

  ;; Inlined runtime: Free-List alloc(size) -> ptr
  (func $__lone_rt_alloc (param $size i32) (result i32)
    (local $req i32)
    (local $curr i32)
    (local $prev i32)
    (local $bsize i32)
    (local $rem i32)
    (local $split_hdr i32)
    (local $next_f i32)
    (local $total_needed i32)
    (local $curr_mem i32)
    (local $pages_needed i32)
    (local $hdr i32)

    local.get $size
    i32.const 0
    i32.le_s
    if
      i32.const 0
      return
    end

    local.get $size
    i32.const 7
    i32.add
    i32.const -8
    i32.and
    local.tee $req
    i32.const 8
    i32.lt_s
    if
      i32.const 8
      local.set $req
    end

    global.get $__free_list_head
    local.set $curr
    i32.const 0
    local.set $prev

    (block $search_done
      (loop $search_loop
        local.get $curr
        i32.const 0
        i32.eq
        br_if $search_done

        local.get $curr
        i32.load
        local.set $bsize
        local.get $bsize
        local.get $req
        i32.ge_s
        if
          local.get $bsize
          local.get $req
          i32.sub
          i32.const 8
          i32.sub
          local.set $rem
          local.get $rem
          i32.const 8
          i32.ge_s
          if
            local.get $curr
            local.get $req
            i32.store
            local.get $curr
            i32.const 8
            i32.add
            local.get $req
            i32.add
            local.set $split_hdr
            local.get $split_hdr
            local.get $rem
            i32.store
            local.get $split_hdr
            i32.const 4
            i32.add
            i32.const 0
            i32.store
            local.get $split_hdr
            i32.const 8
            i32.add
            local.get $curr
            i32.const 8
            i32.add
            i32.load
            i32.store

            local.get $prev
            i32.const 0
            i32.eq
            if
              local.get $split_hdr
              global.set $__free_list_head
            else
              local.get $prev
              i32.const 8
              i32.add
              local.get $split_hdr
              i32.store
            end
          else
            local.get $curr
            i32.const 8
            i32.add
            i32.load
            local.set $next_f
            local.get $prev
            i32.const 0
            i32.eq
            if
              local.get $next_f
              global.set $__free_list_head
            else
              local.get $prev
              i32.const 8
              i32.add
              local.get $next_f
              i32.store
            end
          end

          local.get $curr
          i32.const 4
          i32.add
          i32.const 1
          i32.store

          local.get $curr
          i32.const 8
          i32.add
          return
        end

        local.get $curr
        local.set $prev
        local.get $curr
        i32.const 8
        i32.add
        i32.load
        local.set $curr
        br $search_loop
      )
    )

    local.get $req
    i32.const 8
    i32.add
    local.set $total_needed

    memory.size
    i32.const 16
    i32.shl
    local.set $curr_mem

    global.get $__heap_top
    local.get $total_needed
    i32.add
    local.get $curr_mem
    i32.gt_u
    if
      global.get $__heap_top
      local.get $total_needed
      i32.add
      local.get $curr_mem
      i32.sub
      i32.const 65535
      i32.add
      i32.const 16
      i32.shr_u
      local.set $pages_needed

      local.get $pages_needed
      memory.grow
      i32.const -1
      i32.eq
      if
        i32.const 0
        return
      end
    end

    global.get $__heap_top
    local.set $hdr

    local.get $hdr
    local.get $req
    i32.store

    local.get $hdr
    i32.const 4
    i32.add
    i32.const 1
    i32.store

    global.get $__heap_top
    local.get $total_needed
    i32.add
    global.set $__heap_top

    local.get $hdr
    i32.const 8
    i32.add
    return
  )

  ;; Inlined runtime: Free-List free(ptr)
  (func $__lone_rt_free (param $ptr i32)
    (local $hdr i32)
    (local $curr i32)
    (local $prev i32)

    local.get $ptr
    i32.const 0
    i32.le_s
    if
      return
    end

    local.get $ptr
    i32.const 8
    i32.sub
    local.set $hdr

    local.get $hdr
    i32.const 4
    i32.add
    i32.load
    i32.const 1
    i32.ne
    if
      return
    end

    local.get $hdr
    i32.const 4
    i32.add
    i32.const 0
    i32.store

    global.get $__free_list_head
    local.set $curr
    i32.const 0
    local.set $prev

    (block $find_done
      (loop $find_loop
        local.get $curr
        i32.const 0
        i32.eq
        br_if $find_done

        local.get $curr
        local.get $hdr
        i32.gt_u
        br_if $find_done

        local.get $curr
        local.set $prev
        local.get $curr
        i32.const 8
        i32.add
        i32.load
        local.set $curr
        br $find_loop
      )
    )

    ;; 1. Coalesce right
    local.get $curr
    i32.const 0
    i32.ne
    if
      local.get $hdr
      i32.const 8
      i32.add
      local.get $hdr
      i32.load
      i32.add
      local.get $curr
      i32.eq
      if
        local.get $hdr
        local.get $hdr
        i32.load
        i32.const 8
        i32.add
        local.get $curr
        i32.load
        i32.add
        i32.store

        local.get $curr
        i32.const 8
        i32.add
        i32.load
        local.set $curr
      end
    end

    local.get $hdr
    i32.const 8
    i32.add
    local.get $curr
    i32.store

    ;; 2. Coalesce left
    local.get $prev
    i32.const 0
    i32.ne
    if
      local.get $prev
      i32.const 8
      i32.add
      local.get $prev
      i32.load
      i32.add
      local.get $hdr
      i32.eq
      if
        local.get $prev
        local.get $prev
        i32.load
        i32.const 8
        i32.add
        local.get $hdr
        i32.load
        i32.add
        i32.store

        local.get $prev
        i32.const 8
        i32.add
        local.get $curr
        i32.store
        return
      else
        local.get $prev
        i32.const 8
        i32.add
        local.get $hdr
        i32.store
        return
      end
    end

    local.get $hdr
    global.set $__free_list_head
  )

  ;; Inlined runtime: Free-List realloc(ptr, new_size) -> ptr
  (func $__lone_rt_realloc (param $ptr i32) (param $new_size i32) (result i32)
    (local $hdr i32)
    (local $curr_size i32)
    (local $new_req i32)
    (local $new_ptr i32)
    (local $copy_len i32)

    local.get $ptr
    i32.const 0
    i32.eq
    if
      local.get $new_size
      call $__lone_rt_alloc
      return
    end

    local.get $new_size
    i32.const 0
    i32.le_s
    if
      local.get $ptr
      call $__lone_rt_free
      i32.const 0
      return
    end

    local.get $ptr
    i32.const 8
    i32.sub
    local.set $hdr

    local.get $hdr
    i32.load
    local.set $curr_size

    local.get $new_size
    i32.const 7
    i32.add
    i32.const -8
    i32.and
    local.tee $new_req
    i32.const 8
    i32.lt_s
    if
      i32.const 8
      local.set $new_req
    end

    local.get $new_req
    local.get $curr_size
    i32.le_s
    if
      local.get $ptr
      return
    end

    local.get $new_size
    call $__lone_rt_alloc
    local.set $new_ptr

    local.get $new_ptr
    i32.const 0
    i32.eq
    if
      i32.const 0
      return
    end

    local.get $curr_size
    local.set $copy_len
    local.get $new_size
    local.get $copy_len
    i32.lt_s
    if
      local.get $new_size
      local.set $copy_len
    end

    local.get $new_ptr
    local.get $ptr
    local.get $copy_len
    call $__lone_rt_memcpy

    local.get $ptr
    call $__lone_rt_free

    local.get $new_ptr
    return
  )`)

	// Compute memory layout: stack in Page 0 (growing downward), heap in Page 1+ (growing upward)
	dataEnd := l.nextOffset
	stackLimit := (dataEnd + 15) & ^15
	if stackLimit < 1024 {
		stackLimit = 1024
	}
	heapBase := 65536
	if stackLimit >= heapBase {
		heapBase = (stackLimit + 65535) & ^65535
	}

	// Emit memory runtime globals: stack pointer, stack limit, heap base, heap top, and free list
	l.mod.Globals = append([]Global{
		{
			Name:    "$__stack_pointer",
			Type:    TypeI32,
			Mutable: true,
			InitVal: fmt.Sprintf("i32.const %d", heapBase),
		},
		{
			Name:    "$__stack_limit",
			Type:    TypeI32,
			Mutable: true,
			InitVal: fmt.Sprintf("i32.const %d", stackLimit),
		},
		{
			Name:    "$__heap_base",
			Type:    TypeI32,
			Mutable: false,
			InitVal: fmt.Sprintf("i32.const %d", heapBase),
		},
		{
			Name:    "$__heap_top",
			Type:    TypeI32,
			Mutable: true,
			InitVal: fmt.Sprintf("i32.const %d", heapBase),
		},
		{
			Name:    "$__free_list_head",
			Type:    TypeI32,
			Mutable: true,
			InitVal: "i32.const 0",
		},
		{
			Name:    "$__heap_pointer",
			Type:    TypeI32,
			Mutable: true,
			InitVal: fmt.Sprintf("i32.const %d", heapBase),
		},
	}, l.mod.Globals...)

	return l.mod, nil
}

func (l *lowerer) allocString(val string) (int, int) {
	if entry, exists := l.strings[val]; exists {
		return entry.offset, entry.length
	}
	offset := l.nextOffset
	length := len(val)
	l.strings[val] = stringEntry{offset: offset, length: length}
	l.mod.DataSegments = append(l.mod.DataSegments, DataSegment{
		Offset: offset,
		RawStr: val,
	})
	// Null-terminated string + 4-byte alignment
	l.nextOffset = (offset + length + 1 + 3) & ^3
	return offset, length
}

func (l *lowerer) lowerFunction(fn *loneanalyzer.TFnDec) (*WasmFunc, error) {
	wfn := &WasmFunc{
		Name:   "$" + fn.Name,
		Result: toWasmType(fn.ReturnType),
	}

	if fn.Name == "main" {
		wfn.Exports = []string{"_start", "main"}
	}

	var varSlots = make(map[string]varSlotInfo)
	frameOffset := 0
	for _, stmt := range fn.Body.Stmts {
		if vdec, ok := stmt.(*loneanalyzer.TVarDec); ok && vdec.AddressTaken {
			sz := loneanalyzer.TypeSize(vdec.Type)
			if sz <= 0 {
				sz = 4
			}
			al := loneanalyzer.TypeAlign(vdec.Type)
			if al <= 0 {
				al = 4
			}
			rem := frameOffset % al
			if rem != 0 {
				frameOffset += al - rem
			}
			varSlots[vdec.Name] = varSlotInfo{offset: frameOffset, typ: vdec.Type}
			frameOffset += sz
		}
	}
	if frameOffset%16 != 0 {
		frameOffset += 16 - (frameOffset % 16)
	}

	oldSlots := l.varSlots
	oldFrame := l.frameSize
	l.varSlots = varSlots
	l.frameSize = frameOffset
	defer func() {
		l.varSlots = oldSlots
		l.frameSize = oldFrame
	}()

	alloc := NewLocalAllocator()
	for _, p := range fn.Params {
		pName := alloc.AddParam(p.Name)
		wfn.Params = append(wfn.Params, Param{
			Name: pName,
			Type: toWasmType(p.Type),
		})
	}

	var bodyInsts []WasmInst
	if frameOffset > 0 {
		bodyInsts = append(bodyInsts,
			InstGlobalGet{Name: "$__stack_pointer"},
			InstConstI32{Value: int32(frameOffset)},
			InstBinaryOp{Op: "i32.sub"},
			InstGlobalSet{Name: "$__stack_pointer"},
		)
	}

	bodyInsts = append(bodyInsts, l.lowerBlock(fn.Body, alloc)...)

	// Ensure function ends with return if needed
	if fn.ReturnType == loneanalyzer.TypeVoid || fn.ReturnType == nil {
		if frameOffset > 0 {
			bodyInsts = append(bodyInsts,
				InstGlobalGet{Name: "$__stack_pointer"},
				InstConstI32{Value: int32(frameOffset)},
				InstBinaryOp{Op: "i32.add"},
				InstGlobalSet{Name: "$__stack_pointer"},
			)
		}
		bodyInsts = append(bodyInsts, InstReturn{})
	} else {
		// Non-void function: if it does not end with an explicit return or unreachable,
		// append unreachable to satisfy Wasm stack validation.
		endsInReturn := false
		if len(bodyInsts) > 0 {
			switch bodyInsts[len(bodyInsts)-1].(type) {
			case InstReturn, InstUnreachable:
				endsInReturn = true
			}
		}
		if !endsInReturn {
			if frameOffset > 0 {
				bodyInsts = append(bodyInsts,
					InstGlobalGet{Name: "$__stack_pointer"},
					InstConstI32{Value: int32(frameOffset)},
					InstBinaryOp{Op: "i32.add"},
					InstGlobalSet{Name: "$__stack_pointer"},
				)
			}
			bodyInsts = append(bodyInsts, InstUnreachable{})
		}
	}

	wfn.Locals = alloc.Locals()
	wfn.Body = bodyInsts
	return wfn, nil
}

func (l *lowerer) lowerBlock(b loneanalyzer.TBlock, alloc *LocalAllocator) []WasmInst {
	alloc.EnterScope()
	defer alloc.ExitScope()

	var insts []WasmInst
	for _, stmt := range b.Stmts {
		sInsts := l.lowerStmt(stmt, alloc)
		insts = append(insts, sInsts...)
	}
	return insts
}

func (l *lowerer) lowerStmt(stmt loneanalyzer.TStmt, alloc *LocalAllocator) []WasmInst {
	switch s := stmt.(type) {
	case *loneanalyzer.TVarDec:
		if anon, ok := s.Value.(*loneanalyzer.TAnonFnExpr); ok {
			l.anonFnMap[s.Name] = anon.Name
		}

		// Tuple pattern destructuring: let (a, b) = pair
		if s.TargetVal != nil && len(s.Bindings) > 0 && s.Variant == nil {
			var insts []WasmInst
			tupType, isTuple := s.TargetVal.ExprType().(*loneanalyzer.TupleType)
			targetLocal := alloc.AllocTemp(TypeI32)
			defer alloc.FreeTemp(targetLocal, TypeI32)

			insts = append(insts, l.lowerExpr(s.TargetVal, alloc)...)
			insts = append(insts, InstLocalSet{Name: targetLocal})

			for i, b := range s.Bindings {
				bLocal := alloc.AllocVar(b.Name, toWasmType(b.Type))
				offset := 0
				if isTuple && i < len(tupType.Offsets) {
					offset = tupType.Offsets[i]
				}
				insts = append(insts, InstLocalGet{Name: targetLocal})
				insts = append(insts, InstLoad{Op: loadOpForType(b.Type), Offset: offset})
				insts = append(insts, InstLocalSet{Name: bLocal})
			}
			return insts
		}

		// Grouped bindings: var [top, bottom, left, right]: int = 0
		if len(s.Bindings) > 0 && s.Variant == nil && s.TargetVal == nil {
			var insts []WasmInst
			for _, b := range s.Bindings {
				bLocal := alloc.AllocVar(b.Name, toWasmType(b.Type))
				insts = append(insts, l.lowerExpr(s.Value, alloc)...)
				insts = append(insts, InstLocalSet{Name: bLocal})
			}
			return insts
		}

		if slot, isSlot := l.varSlots[s.Name]; isSlot {
			var insts []WasmInst
			insts = append(insts, InstGlobalGet{Name: "$__stack_pointer"})
			insts = append(insts, l.lowerExpr(s.Value, alloc)...)
			insts = append(insts, InstStore{Op: storeOpForType(s.Type), Offset: slot.offset})
			return insts
		}

		var insts []WasmInst
		valInsts := l.lowerExpr(s.Value, alloc)
		insts = append(insts, valInsts...)

		// Track string variable lengths for #print
		if lit, ok := s.Value.(*loneanalyzer.TLiteralExpr); ok {
			if strVal, ok := lit.Value.(string); ok {
				l.stringVarLen[s.Name] = len(strVal)
			}
		}

		wType := toWasmType(s.Type)
		localName := alloc.AllocVar(s.Name, wType)
		insts = append(insts, InstLocalSet{Name: localName})
		return insts

	case *loneanalyzer.TAssignStmt:
		if ref, ok := s.Target.(*loneanalyzer.TVarRefExpr); ok {
			if slot, isSlot := l.varSlots[ref.Name]; isSlot {
				var insts []WasmInst
				insts = append(insts, InstGlobalGet{Name: "$__stack_pointer"})
				insts = append(insts, l.lowerExpr(s.Value, alloc)...)
				insts = append(insts, InstStore{Op: storeOpForType(ref.Typ), Offset: slot.offset})
				return insts
			}
			var insts []WasmInst
			insts = append(insts, l.lowerExpr(s.Value, alloc)...)
			if localName, found := alloc.Resolve(ref.Name); found {
				insts = append(insts, InstLocalSet{Name: localName})
			} else {
				// Global variable
				insts = append(insts, InstGlobalSet{Name: "$g_" + ref.Name})
			}
			return insts
		}
		if mem, ok := s.Target.(*loneanalyzer.TMemberAccessExpr); ok {
			if st, ok := mem.Typ.(*loneanalyzer.StructType); ok {
				sz := st.TotalSize
				if sz <= 0 {
					sz = 4
				}
				var insts []WasmInst
				// dst: mem.Base + mem.Offset
				insts = append(insts, l.lowerExpr(mem.Base, alloc)...)
				if mem.Offset > 0 {
					insts = append(insts, InstConstI32{Value: int32(mem.Offset)})
					insts = append(insts, InstBinaryOp{Op: "i32.add"})
				}
				// src: s.Value
				insts = append(insts, l.lowerExpr(s.Value, alloc)...)
				insts = append(insts, InstConstI32{Value: int32(sz)})
				insts = append(insts, InstCall{Target: "$__lone_rt_memcpy"})
				return insts
			}
			var insts []WasmInst
			insts = append(insts, l.lowerExpr(mem.Base, alloc)...)
			insts = append(insts, l.lowerExpr(s.Value, alloc)...)
			insts = append(insts, InstStore{Op: storeOpForType(mem.Typ), Offset: mem.Offset})
			return insts
		}
		if un, ok := s.Target.(*loneanalyzer.TUnaryExpr); ok && un.Op == loneast.UNARY_OP_DEREF {
			if st, ok := s.Value.ExprType().(*loneanalyzer.StructType); ok {
				sz := st.TotalSize
				if sz <= 0 {
					sz = 4
				}
				var insts []WasmInst
				insts = append(insts, l.lowerExpr(un.Expr, alloc)...) // dst
				insts = append(insts, l.lowerExpr(s.Value, alloc)...) // src
				insts = append(insts, InstConstI32{Value: int32(sz)})
				insts = append(insts, InstCall{Target: "$__lone_rt_memcpy"})
				return insts
			}
			var insts []WasmInst
			insts = append(insts, l.lowerExpr(un.Expr, alloc)...)
			insts = append(insts, l.lowerExpr(s.Value, alloc)...)
			insts = append(insts, InstStore{Op: storeOpForType(s.Value.ExprType()), Offset: 0})
			return insts
		}
		if idx, ok := s.Target.(*loneanalyzer.TIndexExpr); ok {
			elemSz := int32(loneanalyzer.TypeSize(s.Value.ExprType()))
			if elemSz <= 0 {
				elemSz = 4
			}
			var insts []WasmInst
			insts = append(insts, l.lowerExpr(idx.Base, alloc)...)
			insts = append(insts, l.lowerExpr(idx.Index, alloc)...)
			if elemSz > 1 {
				insts = append(insts, InstConstI32{Value: elemSz})
				insts = append(insts, InstBinaryOp{Op: "i32.mul"})
			}
			insts = append(insts, InstBinaryOp{Op: "i32.add"})
			insts = append(insts, l.lowerExpr(s.Value, alloc)...)
			insts = append(insts, InstStore{Op: storeOpForType(s.Value.ExprType()), Offset: 0})
			return insts
		}
		return nil

	case *loneanalyzer.TReturnStmt:
		var insts []WasmInst
		if s.Value != nil {
			insts = append(insts, l.lowerExpr(s.Value, alloc)...)
			if l.frameSize > 0 {
				wRetType := toWasmType(s.Value.ExprType())
				temp := alloc.AllocTemp(wRetType)
				insts = append(insts, InstLocalSet{Name: temp})
				insts = append(insts,
					InstGlobalGet{Name: "$__stack_pointer"},
					InstConstI32{Value: int32(l.frameSize)},
					InstBinaryOp{Op: "i32.add"},
					InstGlobalSet{Name: "$__stack_pointer"},
				)
				insts = append(insts, InstLocalGet{Name: temp})
				alloc.FreeTemp(temp, wRetType)
			}
		} else {
			if l.frameSize > 0 {
				insts = append(insts,
					InstGlobalGet{Name: "$__stack_pointer"},
					InstConstI32{Value: int32(l.frameSize)},
					InstBinaryOp{Op: "i32.add"},
					InstGlobalSet{Name: "$__stack_pointer"},
				)
			}
		}
		insts = append(insts, InstReturn{})
		return insts

	case *loneanalyzer.TDropStmt:
		var insts []WasmInst
		exprInsts := l.lowerExpr(s.Expr, alloc)
		insts = append(insts, exprInsts...)
		if s.Expr.ExprType() != loneanalyzer.TypeVoid && s.Expr.ExprType() != nil {
			insts = append(insts, InstDrop{})
		}
		return insts

	case *loneanalyzer.TIfStmt:
		return l.lowerIfStmt(s, alloc)

	case *loneanalyzer.TPackedIfStmt:
		return l.lowerPackedIfStmt(s, alloc)

	case *loneanalyzer.TWhileStmt:
		return l.lowerWhileStmt(s, alloc)

	case *loneanalyzer.TLoopStmt:
		return l.lowerLoopStmt(s, alloc)

	case *loneanalyzer.TBreakStmt:
		var insts []WasmInst
		target := ""
		if s.Label != "" {
			if mapped, ok := l.labelStack[s.Label]; ok {
				target = mapped
			} else {
				target = "$" + s.Label
			}
		} else if len(l.loopStack) > 0 {
			target = l.loopStack[len(l.loopStack)-1].breakLbl
		}
		if s.Value != nil {
			insts = append(insts, l.lowerExpr(s.Value, alloc)...)
		}
		if target != "" {
			insts = append(insts, InstBr{Target: target})
		}
		return insts

	case *loneanalyzer.TLabelDec:
		bodyInsts := l.lowerBlock(s.Body, alloc)
		wType := toWasmType(s.Typ)
		return []WasmInst{
			InstBlock{
				Label:  "$" + s.Name,
				Result: wType,
				Body:   bodyInsts,
			},
		}

	case loneanalyzer.TExpr:
		var insts []WasmInst
		insts = append(insts, l.lowerExpr(s, alloc)...)
		if s.ExprType() != loneanalyzer.TypeVoid && s.ExprType() != nil {
			insts = append(insts, InstDrop{})
		}
		return insts

	default:
		return nil
	}
}

func (l *lowerer) lowerIfStmt(s *loneanalyzer.TIfStmt, alloc *LocalAllocator) []WasmInst {
	alloc.EnterScope()
	defer alloc.ExitScope()

	var insts []WasmInst
	var patternChecks []WasmInst
	var patternBinds []WasmInst

	// 1. Lower scoped header bindings if any (e.g. if: [let x = 0] ... or if: [let Shape::Circle(r) = s])
	for _, h := range s.Header {
		if h.Variant != nil {
			targetLocal := alloc.AllocTemp(TypeI32)
			defer alloc.FreeTemp(targetLocal, TypeI32)

			insts = append(insts, l.lowerExpr(h.TargetVal, alloc)...)
			insts = append(insts, InstLocalSet{Name: targetLocal})

			// Tag check: targetLocal.tag == h.Variant.Tag
			patternChecks = append(patternChecks, InstLocalGet{Name: targetLocal})
			patternChecks = append(patternChecks, InstLoad{Op: "i32.load", Offset: 0})
			patternChecks = append(patternChecks, InstConstI32{Value: int32(h.Variant.Tag)})
			patternChecks = append(patternChecks, InstBinaryOp{Op: "i32.eq"})

			// In Then block: bind fields
			for i, b := range h.Bindings {
				if i < len(h.Variant.Fields) {
					fInfo := h.Variant.Fields[i]
					bLocal := alloc.AllocVar(b.Name, toWasmType(fInfo.Type))
					patternBinds = append(patternBinds, InstLocalGet{Name: targetLocal})
					patternBinds = append(patternBinds, InstLoad{Op: loadOpForType(fInfo.Type), Offset: fInfo.Offset})
					patternBinds = append(patternBinds, InstLocalSet{Name: bLocal})
				}
			}
		} else {
			hInsts := l.lowerStmt(h, alloc)
			insts = append(insts, hInsts...)
		}
	}

	// 2. Evaluate condition
	if len(patternChecks) > 0 {
		insts = append(insts, patternChecks...)
		if s.Cond != nil {
			insts = append(insts, l.lowerExpr(s.Cond, alloc)...)
			insts = append(insts, InstBinaryOp{Op: "i32.and"})
		}
	} else if s.Cond != nil {
		condInsts := l.lowerExpr(s.Cond, alloc)
		insts = append(insts, condInsts...)
	}

	// 3. Lower Then and Else blocks
	thenInsts := append(patternBinds, l.lowerBlock(s.Then, alloc)...)

	var elseInsts []WasmInst
	if s.Else != nil {
		switch el := s.Else.(type) {
		case *loneanalyzer.TBlock:
			elseInsts = l.lowerBlock(*el, alloc)
		case *loneanalyzer.TIfStmt:
			elseInsts = l.lowerIfStmt(el, alloc)
		}
	}

	insts = append(insts, InstIf{
		Then: thenInsts,
		Else: elseInsts,
	})
	return insts
}

func (l *lowerer) lowerPackedIfStmt(p *loneanalyzer.TPackedIfStmt, alloc *LocalAllocator) []WasmInst {
	alloc.EnterScope()
	defer alloc.ExitScope()

	var insts []WasmInst
	for _, h := range p.Header {
		hInsts := l.lowerStmt(h, alloc)
		insts = append(insts, hInsts...)
	}

	// Chain arms as structured nested if/else
	insts = append(insts, l.lowerPackedArms(p.Arms, 0, alloc)...)
	return insts
}

func (l *lowerer) lowerPackedArms(arms []loneanalyzer.TPackedIfArm, index int, alloc *LocalAllocator) []WasmInst {
	if index >= len(arms) {
		return nil
	}
	arm := arms[index]
	if arm.Cond == nil {
		// Else arm
		return l.lowerStmtOrBlock(arm.Body, alloc)
	}

	var insts []WasmInst
	condInsts := l.lowerExpr(arm.Cond, alloc)
	insts = append(insts, condInsts...)

	thenInsts := l.lowerStmtOrBlock(arm.Body, alloc)
	elseInsts := l.lowerPackedArms(arms, index+1, alloc)

	insts = append(insts, InstIf{
		Then: thenInsts,
		Else: elseInsts,
	})
	return insts
}

func (l *lowerer) lowerStmtOrBlock(stmt loneanalyzer.TStmt, alloc *LocalAllocator) []WasmInst {
	if b, ok := stmt.(*loneanalyzer.TBlock); ok {
		return l.lowerBlock(*b, alloc)
	}
	return l.lowerStmt(stmt, alloc)
}

func (l *lowerer) lowerWhileStmt(s *loneanalyzer.TWhileStmt, alloc *LocalAllocator) []WasmInst {
	l.labelId++
	breakLbl := fmt.Sprintf("$while_break_%d", l.labelId)
	loopLbl := fmt.Sprintf("$while_loop_%d", l.labelId)

	l.loopStack = append(l.loopStack, loopContext{
		breakLbl: breakLbl,
		contLbl:  loopLbl,
	})
	defer func() {
		l.loopStack = l.loopStack[:len(l.loopStack)-1]
	}()

	var loopBody []WasmInst

	// Evaluate condition
	condInsts := l.lowerExpr(s.Cond, alloc)
	loopBody = append(loopBody, condInsts...)
	loopBody = append(loopBody, InstUnaryOp{Op: "i32.eqz"})
	loopBody = append(loopBody, InstBrIf{Target: breakLbl})

	// Body statements
	bodyInsts := l.lowerBlock(s.Body, alloc)
	loopBody = append(loopBody, bodyInsts...)

	// Back branch
	loopBody = append(loopBody, InstBr{Target: loopLbl})

	return []WasmInst{
		InstBlock{
			Label: breakLbl,
			Body: []WasmInst{
				InstLoop{
					Label: loopLbl,
					Body:  loopBody,
				},
			},
		},
	}
}

func (l *lowerer) lowerLoopStmt(s *loneanalyzer.TLoopStmt, alloc *LocalAllocator) []WasmInst {
	l.labelId++
	breakLbl := fmt.Sprintf("$loop_break_%d", l.labelId)
	contLbl := fmt.Sprintf("$loop_cont_%d", l.labelId)

	l.loopStack = append(l.loopStack, loopContext{
		breakLbl: breakLbl,
		contLbl:  contLbl,
	})
	defer func() {
		l.loopStack = l.loopStack[:len(l.loopStack)-1]
	}()

	var loopBody []WasmInst
	bodyInsts := l.lowerBlock(s.Body, alloc)
	loopBody = append(loopBody, bodyInsts...)

	if s.WhileCond != nil {
		// loop { ... } while: [cond]
		// Evaluates post-condition at loop tail: if true, branches back to contLbl
		condInsts := l.lowerExpr(s.WhileCond, alloc)
		loopBody = append(loopBody, condInsts...)
		loopBody = append(loopBody, InstBrIf{Target: contLbl})
	} else {
		// Infinite loop: unconditionally branch back
		loopBody = append(loopBody, InstBr{Target: contLbl})
	}

	return []WasmInst{
		InstBlock{
			Label: breakLbl,
			Body: []WasmInst{
				InstLoop{
					Label: contLbl,
					Body:  loopBody,
				},
			},
		},
	}
}

func (l *lowerer) lowerExpr(expr loneanalyzer.TExpr, alloc *LocalAllocator) []WasmInst {
	switch e := expr.(type) {
	case *loneanalyzer.TLiteralExpr:
		switch e.Kind {
		case loneast.LiteralInt:
			if v, ok := e.Value.(int64); ok {
				if e.Typ == loneanalyzer.TypeI64 || e.Typ == loneanalyzer.TypeU64 {
					return []WasmInst{InstConstI64{Value: v}}
				}
				return []WasmInst{InstConstI32{Value: int32(v)}}
			}
			return []WasmInst{InstConstI32{Value: 0}}

		case loneast.LiteralFloat:
			if v, ok := e.Value.(float64); ok {
				if e.Typ == loneanalyzer.TypeF32 {
					return []WasmInst{InstConstF32{Value: float32(v)}}
				}
				return []WasmInst{InstConstF64{Value: v}}
			}
			return []WasmInst{InstConstF64{Value: 0}}

		case loneast.LiteralBool:
			if b, ok := e.Value.(bool); ok && b {
				return []WasmInst{InstConstI32{Value: 1}}
			}
			return []WasmInst{InstConstI32{Value: 0}}

		case loneast.LiteralString:
			if s, ok := e.Value.(string); ok {
				offset, _ := l.allocString(s)
				return []WasmInst{InstConstI32{Value: int32(offset)}}
			}
			return []WasmInst{InstConstI32{Value: 0}}

		default:
			return []WasmInst{InstConstI32{Value: 0}}
		}

	case *loneanalyzer.TVarRefExpr:
		if slot, isSlot := l.varSlots[e.Name]; isSlot {
			return []WasmInst{
				InstGlobalGet{Name: "$__stack_pointer"},
				InstLoad{Op: loadOpForType(e.Typ), Offset: slot.offset},
			}
		}
		if localName, found := alloc.Resolve(e.Name); found {
			return []WasmInst{InstLocalGet{Name: localName}}
		}
		// Module global
		return []WasmInst{InstGlobalGet{Name: "$g_" + e.Name}}

	case *loneanalyzer.TBinaryExpr:
		var insts []WasmInst
		insts = append(insts, l.lowerExpr(e.Left, alloc)...)
		insts = append(insts, l.lowerExpr(e.Right, alloc)...)

		opStr := l.mapBinaryOp(e.Op, e.Left.ExprType())
		insts = append(insts, InstBinaryOp{Op: opStr})
		return insts

	case *loneanalyzer.TUnaryExpr:
		var insts []WasmInst
		switch e.Op {
		case loneast.UNARY_OP_NEGATIVE:
			if isFloatType(e.Expr.ExprType()) {
				insts = append(insts, l.lowerExpr(e.Expr, alloc)...)
				insts = append(insts, InstUnaryOp{Op: "f64.neg"})
			} else {
				insts = append(insts, InstConstI32{Value: 0})
				insts = append(insts, l.lowerExpr(e.Expr, alloc)...)
				insts = append(insts, InstBinaryOp{Op: "i32.sub"})
			}
		case loneast.UNARY_OP_NOT:
			insts = append(insts, l.lowerExpr(e.Expr, alloc)...)
			insts = append(insts, InstUnaryOp{Op: "i32.eqz"})
		case loneast.UNARY_OP_DEREF:
			insts = append(insts, l.lowerExpr(e.Expr, alloc)...)
			insts = append(insts, InstLoad{Op: loadOpForType(e.Typ), Offset: 0})
		case loneast.UNARY_OP_REF, loneast.UNARY_OP_RAW_REF:
			if vref, ok := e.Expr.(*loneanalyzer.TVarRefExpr); ok {
				if _, isStruct := vref.Typ.(*loneanalyzer.StructType); isStruct {
					return l.lowerExpr(vref, alloc)
				}
				if slot, isSlot := l.varSlots[vref.Name]; isSlot {
					return []WasmInst{
						InstGlobalGet{Name: "$__stack_pointer"},
						InstConstI32{Value: int32(slot.offset)},
						InstBinaryOp{Op: "i32.add"},
					}
				}
			}
			if mem, ok := e.Expr.(*loneanalyzer.TMemberAccessExpr); ok {
				var minsts []WasmInst
				minsts = append(minsts, l.lowerExpr(mem.Base, alloc)...)
				if mem.Offset > 0 {
					minsts = append(minsts, InstConstI32{Value: int32(mem.Offset)})
					minsts = append(minsts, InstBinaryOp{Op: "i32.add"})
				}
				return minsts
			}
			insts = append(insts, l.lowerExpr(e.Expr, alloc)...)
		default:
			insts = append(insts, l.lowerExpr(e.Expr, alloc)...)
		}
		return insts

	case *loneanalyzer.TCastExpr:
		var insts []WasmInst
		insts = append(insts, l.lowerExpr(e.Expr, alloc)...)
		castInst := l.mapCastOp(e.Expr.ExprType(), e.TargetType)
		if castInst != nil {
			insts = append(insts, castInst)
		}
		return insts

	case *loneanalyzer.TCallExpr:
		return l.lowerCallExpr(e, alloc)

	case *loneanalyzer.TBlock:
		// Expression block: calc_blk: { ... }
		return l.lowerBlock(*e, alloc)

	case *loneanalyzer.TLabelDec:
		bodyInsts := l.lowerBlock(e.Body, alloc)
		wType := toWasmType(e.Typ)
		return []WasmInst{
			InstBlock{
				Label:  "$" + e.Name,
				Result: wType,
				Body:   bodyInsts,
			},
		}

	case *loneanalyzer.TEnumConstructExpr:
		return l.lowerEnumConstructExpr(e, alloc)

	case *loneanalyzer.TMatchExpr:
		return l.lowerMatchExpr(e, alloc)

	case *loneanalyzer.TIsExpr:
		var insts []WasmInst
		if e.TargetVariant != nil {
			insts = append(insts, l.lowerExpr(e.Expr, alloc)...)
			insts = append(insts, InstLoad{Op: "i32.load", Offset: 0})
			insts = append(insts, InstConstI32{Value: int32(e.TargetVariant.Tag)})
			if e.Negated {
				insts = append(insts, InstBinaryOp{Op: "i32.ne"})
			} else {
				insts = append(insts, InstBinaryOp{Op: "i32.eq"})
			}
			return insts
		}
		insts = append(insts, InstConstI32{Value: 1})
		return insts

	case *loneanalyzer.TMustExpr:
		return l.lowerMustExpr(e, alloc)

	case *loneanalyzer.TTryExpr:
		return l.lowerTryExpr(e, alloc)

	case *loneanalyzer.TTraitMatExpr:
		return []WasmInst{InstConstI32{Value: 0}}

	case *loneanalyzer.TMemberAccessExpr:
		if e.Offset >= 0 && e.Typ != loneanalyzer.TypeVoid && e.Typ != nil {
			var insts []WasmInst
			insts = append(insts, l.lowerExpr(e.Base, alloc)...)
			insts = append(insts, InstLoad{Op: loadOpForType(e.Typ), Offset: e.Offset})
			return insts
		}
		return []WasmInst{InstConstI32{Value: 0}}

	case *loneanalyzer.TIndexExpr:
		elemSz := int32(loneanalyzer.TypeSize(e.Typ))
		if elemSz <= 0 {
			elemSz = 4
		}
		var insts []WasmInst
		insts = append(insts, l.lowerExpr(e.Base, alloc)...)
		insts = append(insts, l.lowerExpr(e.Index, alloc)...)
		if elemSz > 1 {
			insts = append(insts, InstConstI32{Value: elemSz})
			insts = append(insts, InstBinaryOp{Op: "i32.mul"})
		}
		insts = append(insts, InstBinaryOp{Op: "i32.add"})
		insts = append(insts, InstLoad{Op: loadOpForType(e.Typ), Offset: 0})
		return insts

	case *loneanalyzer.TTupleExpr:
		return l.lowerTupleExpr(e, alloc)

	case *loneanalyzer.TStructConstructExpr:
		return l.lowerStructConstructExpr(e, alloc)

	case *loneanalyzer.TAnonFnExpr:
		return []WasmInst{InstConstI32{Value: 0}}

	case *loneanalyzer.TInterpolatedStringExpr:
		return l.lowerInterpolatedString(e, alloc, false, false)

	default:
		return nil
	}
}

func (l *lowerer) lowerStructConstructExpr(e *loneanalyzer.TStructConstructExpr, alloc *LocalAllocator) []WasmInst {
	var insts []WasmInst
	ptrLocal := alloc.AllocTemp(TypeI32)
	defer alloc.FreeTemp(ptrLocal, TypeI32)

	totalSize := e.StructType.TotalSize
	if totalSize <= 0 {
		totalSize = 4
	}

	insts = append(insts, InstConstI32{Value: int32(totalSize)})
	insts = append(insts, InstCall{Target: "$__lone_rt_alloc"})
	insts = append(insts, InstLocalSet{Name: ptrLocal})

	for fName, fVal := range e.Fields {
		if fInfo, ok := e.StructType.Field(fName); ok {
			insts = append(insts, InstLocalGet{Name: ptrLocal})
			insts = append(insts, l.lowerExpr(fVal, alloc)...)
			insts = append(insts, InstStore{Op: storeOpForType(fInfo.Type), Offset: fInfo.Offset})
		}
	}

	insts = append(insts, InstLocalGet{Name: ptrLocal})
	return insts
}

func (l *lowerer) lowerTupleExpr(e *loneanalyzer.TTupleExpr, alloc *LocalAllocator) []WasmInst {
	var insts []WasmInst
	ptrLocal := alloc.AllocTemp(TypeI32)
	defer alloc.FreeTemp(ptrLocal, TypeI32)

	totalSize := loneanalyzer.TypeSize(e.Typ)
	if totalSize <= 0 {
		totalSize = 4
	}

	insts = append(insts, InstConstI32{Value: int32(totalSize)})
	insts = append(insts, InstCall{Target: "$__lone_rt_alloc"})
	insts = append(insts, InstLocalSet{Name: ptrLocal})

	tupType, _ := e.Typ.(*loneanalyzer.TupleType)
	for i, elem := range e.Elements {
		offset := 0
		if tupType != nil && i < len(tupType.Offsets) {
			offset = tupType.Offsets[i]
		}
		insts = append(insts, InstLocalGet{Name: ptrLocal})
		insts = append(insts, l.lowerExpr(elem, alloc)...)
		insts = append(insts, InstStore{Op: storeOpForType(elem.ExprType()), Offset: offset})
	}

	insts = append(insts, InstLocalGet{Name: ptrLocal})
	return insts
}

func (l *lowerer) lowerCallExpr(e *loneanalyzer.TCallExpr, alloc *LocalAllocator) []WasmInst {
	calleeName := ""
	if ref, ok := e.Callee.(*loneanalyzer.TVarRefExpr); ok {
		calleeName = ref.Name
		if target, ok := l.anonFnMap[calleeName]; ok {
			calleeName = strings.TrimPrefix(target, "$")
		}
	} else if anon, ok := e.Callee.(*loneanalyzer.TAnonFnExpr); ok {
		calleeName = strings.TrimPrefix(anon.Name, "$")
	} else if mem, ok := e.Callee.(*loneanalyzer.TMemberAccessExpr); ok {
		calleeName = mem.Field
	}

	switch calleeName {
	case "#print", "print", "println":
		l.usesPrint = true
		var insts []WasmInst
		if len(e.Args) == 1 {
			arg := e.Args[0]
			if interp, ok := arg.(*loneanalyzer.TInterpolatedStringExpr); ok {
				return l.lowerInterpolatedString(interp, alloc, true, calleeName == "println")
			}
			if lit, ok := arg.(*loneanalyzer.TLiteralExpr); ok {
				if strVal, ok := lit.Value.(string); ok {
					if calleeName == "println" {
						strVal += "\n"
					}
					offset, length := l.allocString(strVal)
					insts = append(insts, InstConstI32{Value: int32(offset)})
					insts = append(insts, InstConstI32{Value: int32(length)})
					insts = append(insts, InstCall{Target: "$__lone_rt_print"})
					return insts
				}
			}
			// Variable reference
			insts = append(insts, l.lowerExpr(arg, alloc)...)
			if ref, ok := arg.(*loneanalyzer.TVarRefExpr); ok {
				if strLen, hasLen := l.stringVarLen[ref.Name]; hasLen {
					insts = append(insts, InstConstI32{Value: int32(strLen)})
					insts = append(insts, InstCall{Target: "$__lone_rt_print"})
					if calleeName == "println" {
						nlOffset, nlLen := l.allocString("\n")
						insts = append(insts, InstConstI32{Value: int32(nlOffset)})
						insts = append(insts, InstConstI32{Value: int32(nlLen)})
						insts = append(insts, InstCall{Target: "$__lone_rt_print"})
					}
					return insts
				}
			}
			// Dynamic string length via runtime strlen
			l.usesStrlen = true
			ptrLocal := alloc.AllocTemp(TypeI32)
			defer alloc.FreeTemp(ptrLocal, TypeI32)
			var dynInsts []WasmInst
			dynInsts = append(dynInsts, l.lowerExpr(arg, alloc)...)
			dynInsts = append(dynInsts, InstLocalTee{Name: ptrLocal})
			dynInsts = append(dynInsts, InstLocalGet{Name: ptrLocal})
			dynInsts = append(dynInsts, InstCall{Target: "$__lone_rt_strlen"})
			dynInsts = append(dynInsts, InstCall{Target: "$__lone_rt_print"})
			if calleeName == "println" {
				nlOffset, nlLen := l.allocString("\n")
				dynInsts = append(dynInsts, InstConstI32{Value: int32(nlOffset)})
				dynInsts = append(dynInsts, InstConstI32{Value: int32(nlLen)})
				dynInsts = append(dynInsts, InstCall{Target: "$__lone_rt_print"})
			}
			return dynInsts
		} else if len(e.Args) >= 2 {
			// #print(ptr, len)
			insts = append(insts, l.lowerExpr(e.Args[0], alloc)...)
			insts = append(insts, l.lowerExpr(e.Args[1], alloc)...)
			insts = append(insts, InstCall{Target: "$__lone_rt_print"})
			return insts
		}
		return nil

	case "#alloc":
		var insts []WasmInst
		if len(e.Args) == 1 {
			insts = append(insts, l.lowerExpr(e.Args[0], alloc)...)
			insts = append(insts, InstCall{Target: "$__lone_rt_alloc"})
		}
		return insts

	case "#free":
		var insts []WasmInst
		if len(e.Args) == 1 {
			insts = append(insts, l.lowerExpr(e.Args[0], alloc)...)
			insts = append(insts, InstCall{Target: "$__lone_rt_free"})
		}
		return insts

	case "#realloc":
		var insts []WasmInst
		if len(e.Args) == 2 {
			insts = append(insts, l.lowerExpr(e.Args[0], alloc)...)
			insts = append(insts, l.lowerExpr(e.Args[1], alloc)...)
			insts = append(insts, InstCall{Target: "$__lone_rt_realloc"})
		}
		return insts

	case "#memory_grow":
		var insts []WasmInst
		if len(e.Args) == 1 {
			insts = append(insts, l.lowerExpr(e.Args[0], alloc)...)
			insts = append(insts, InstMemoryGrow{})
		}
		return insts

	case "#memory_size":
		return []WasmInst{InstMemorySize{}}

	case "#stack_pointer":
		return []WasmInst{InstGlobalGet{Name: "$__stack_pointer"}}

	case "#stack_alloc":
		var insts []WasmInst
		if len(e.Args) == 1 {
			spLocal := alloc.AllocTemp(TypeI32)
			defer alloc.FreeTemp(spLocal, TypeI32)

			insts = append(insts, InstGlobalGet{Name: "$__stack_pointer"})
			insts = append(insts, l.lowerExpr(e.Args[0], alloc)...)
			insts = append(insts, InstConstI32{Value: 15})
			insts = append(insts, InstBinaryOp{Op: "i32.add"})
			insts = append(insts, InstConstI32{Value: -16})
			insts = append(insts, InstBinaryOp{Op: "i32.and"})
			insts = append(insts, InstBinaryOp{Op: "i32.sub"})
			insts = append(insts, InstLocalTee{Name: spLocal})

			insts = append(insts, InstGlobalGet{Name: "$__stack_limit"})
			insts = append(insts, InstBinaryOp{Op: "i32.lt_u"})
			insts = append(insts, InstIf{
				Then: []WasmInst{InstUnreachable{}},
			})

			insts = append(insts, InstLocalGet{Name: spLocal})
			insts = append(insts, InstGlobalSet{Name: "$__stack_pointer"})
			insts = append(insts, InstLocalGet{Name: spLocal})
		}
		return insts

	case "#stack_free":
		var insts []WasmInst
		if len(e.Args) == 1 {
			insts = append(insts, InstGlobalGet{Name: "$__stack_pointer"})
			insts = append(insts, l.lowerExpr(e.Args[0], alloc)...)
			insts = append(insts, InstConstI32{Value: 15})
			insts = append(insts, InstBinaryOp{Op: "i32.add"})
			insts = append(insts, InstConstI32{Value: -16})
			insts = append(insts, InstBinaryOp{Op: "i32.and"})
			insts = append(insts, InstBinaryOp{Op: "i32.add"})
			insts = append(insts, InstGlobalSet{Name: "$__stack_pointer"})
		}
		return insts
	}

	// User function call
	var insts []WasmInst
	for _, arg := range e.Args {
		if st, ok := arg.ExprType().(*loneanalyzer.StructType); ok {
			isLValue := false
			switch a := arg.(type) {
			case *loneanalyzer.TVarRefExpr, *loneanalyzer.TMemberAccessExpr:
				isLValue = true
			case *loneanalyzer.TUnaryExpr:
				if a.Op == loneast.UNARY_OP_DEREF {
					isLValue = true
				}
			}

			if isLValue {
				sz := st.TotalSize
				if sz <= 0 {
					sz = 4
				}
				copyLocal := alloc.AllocTemp(TypeI32)
				defer alloc.FreeTemp(copyLocal, TypeI32)
				insts = append(insts,
					InstConstI32{Value: int32(sz)},
					InstCall{Target: "$__lone_rt_alloc"},
					InstLocalTee{Name: copyLocal},
				)
				insts = append(insts, l.lowerExpr(arg, alloc)...) // src
				insts = append(insts,
					InstConstI32{Value: int32(sz)},
					InstCall{Target: "$__lone_rt_memcpy"},
					InstLocalGet{Name: copyLocal},
				)
				continue
			}
		}
		insts = append(insts, l.lowerExpr(arg, alloc)...)
	}
	insts = append(insts, InstCall{Target: "$" + calleeName})
	return insts
}

func (l *lowerer) lowerInterpolatedString(
	expr *loneanalyzer.TInterpolatedStringExpr,
	alloc *LocalAllocator,
	toStack bool,
	isPrintln bool,
) []WasmInst {
	l.usesPrint = true
	var insts []WasmInst

	bufLocal := alloc.AllocTemp(TypeI32)
	curLocal := alloc.AllocTemp(TypeI32)
	defer alloc.FreeTemp(bufLocal, TypeI32)
	defer alloc.FreeTemp(curLocal, TypeI32)

	bufSize := 256
	staticBytes := 0
	for _, p := range expr.Parts {
		if lit, ok := p.(*loneanalyzer.TLiteralExpr); ok && lit.Kind == loneast.LiteralString {
			if s, ok := lit.Value.(string); ok {
				staticBytes += len(s)
			}
		} else {
			staticBytes += 32
		}
	}
	if staticBytes+32 > bufSize {
		bufSize = (staticBytes + 63) & ^15
	}

	if toStack {
		insts = append(insts,
			InstGlobalGet{Name: "$__stack_pointer"},
			InstConstI32{Value: int32(bufSize)},
			InstBinaryOp{Op: "i32.sub"},
			InstLocalTee{Name: bufLocal},
			InstLocalSet{Name: curLocal},
			InstLocalGet{Name: bufLocal},
			InstGlobalSet{Name: "$__stack_pointer"},
		)
	} else {
		insts = append(insts,
			InstConstI32{Value: int32(bufSize)},
			InstCall{Target: "$__lone_rt_alloc"},
			InstLocalTee{Name: bufLocal},
			InstLocalSet{Name: curLocal},
		)
	}

	for _, part := range expr.Parts {
		// 1. Literal string
		if lit, ok := part.(*loneanalyzer.TLiteralExpr); ok && lit.Kind == loneast.LiteralString {
			if strVal, ok := lit.Value.(string); ok && len(strVal) > 0 {
				offset, length := l.allocString(strVal)
				insts = append(insts,
					InstLocalGet{Name: curLocal},
					InstConstI32{Value: int32(offset)},
					InstConstI32{Value: int32(length)},
					InstCall{Target: "$__lone_rt_memcpy"},
					InstLocalGet{Name: curLocal},
					InstConstI32{Value: int32(length)},
					InstBinaryOp{Op: "i32.add"},
					InstLocalSet{Name: curLocal},
				)
				continue
			}
		}

		// 2. Integers
		pt := part.ExprType()
		if pt == loneanalyzer.TypeI32 || pt == loneanalyzer.TypeU32 {
			l.usesFormatI32 = true
			insts = append(insts, l.lowerExpr(part, alloc)...)
			insts = append(insts,
				InstLocalGet{Name: curLocal},
				InstCall{Target: "$__lone_rt_format_i32"},
				InstLocalGet{Name: curLocal},
				InstBinaryOp{Op: "i32.add"},
				InstLocalSet{Name: curLocal},
			)
			continue
		}

		// 3. Boolean
		if pt == loneanalyzer.TypeBool {
			trueOffset, _ := l.allocString("true")
			falseOffset, _ := l.allocString("false")
			boolLocal := alloc.AllocTemp(TypeI32)
			insts = append(insts, l.lowerExpr(part, alloc)...)
			insts = append(insts,
				InstLocalTee{Name: boolLocal},
				InstIf{
					Then: []WasmInst{
						InstLocalGet{Name: curLocal},
						InstConstI32{Value: int32(trueOffset)},
						InstConstI32{Value: 4},
						InstCall{Target: "$__lone_rt_memcpy"},
						InstLocalGet{Name: curLocal},
						InstConstI32{Value: 4},
						InstBinaryOp{Op: "i32.add"},
						InstLocalSet{Name: curLocal},
					},
					Else: []WasmInst{
						InstLocalGet{Name: curLocal},
						InstConstI32{Value: int32(falseOffset)},
						InstConstI32{Value: 5},
						InstCall{Target: "$__lone_rt_memcpy"},
						InstLocalGet{Name: curLocal},
						InstConstI32{Value: 5},
						InstBinaryOp{Op: "i32.add"},
						InstLocalSet{Name: curLocal},
					},
				},
			)
			alloc.FreeTemp(boolLocal, TypeI32)
			continue
		}

		// 4. Dynamic String
		if pt == loneanalyzer.TypeStr {
			l.usesStrlen = true
			strLocal := alloc.AllocTemp(TypeI32)
			lenLocal := alloc.AllocTemp(TypeI32)
			insts = append(insts, l.lowerExpr(part, alloc)...)
			insts = append(insts,
				InstLocalTee{Name: strLocal},
				InstCall{Target: "$__lone_rt_strlen"},
				InstLocalTee{Name: lenLocal},
				InstLocalGet{Name: curLocal},
				InstLocalGet{Name: strLocal},
				InstLocalGet{Name: lenLocal},
				InstCall{Target: "$__lone_rt_memcpy"},
				InstLocalGet{Name: curLocal},
				InstLocalGet{Name: lenLocal},
				InstBinaryOp{Op: "i32.add"},
				InstLocalSet{Name: curLocal},
			)
			alloc.FreeTemp(strLocal, TypeI32)
			alloc.FreeTemp(lenLocal, TypeI32)
			continue
		}

		// 5. Pointers: format as integer address
		if _, ok := pt.(*loneanalyzer.PtrType); ok {
			l.usesFormatI32 = true
			insts = append(insts, l.lowerExpr(part, alloc)...)
			insts = append(insts,
				InstLocalGet{Name: curLocal},
				InstCall{Target: "$__lone_rt_format_i32"},
				InstLocalGet{Name: curLocal},
				InstBinaryOp{Op: "i32.add"},
				InstLocalSet{Name: curLocal},
			)
			continue
		}

		// Fallback: evaluate expression and drop
		insts = append(insts, l.lowerExpr(part, alloc)...)
		insts = append(insts, InstDrop{})
	}

	if toStack {
		if isPrintln {
			insts = append(insts,
				InstLocalGet{Name: curLocal},
				InstConstI32{Value: 10},
				InstStore{Op: "i32.store8", Offset: 0},
				InstLocalGet{Name: curLocal},
				InstConstI32{Value: 1},
				InstBinaryOp{Op: "i32.add"},
				InstLocalSet{Name: curLocal},
			)
		}
		// Exact 1 atomic syscall: $__lone_rt_print(bufLocal, curLocal - bufLocal)
		insts = append(insts,
			InstLocalGet{Name: bufLocal},
			InstLocalGet{Name: curLocal},
			InstLocalGet{Name: bufLocal},
			InstBinaryOp{Op: "i32.sub"},
			InstCall{Target: "$__lone_rt_print"},
		)
		// Rewind Page 0 datastack
		insts = append(insts,
			InstGlobalGet{Name: "$__stack_pointer"},
			InstConstI32{Value: int32(bufSize)},
			InstBinaryOp{Op: "i32.add"},
			InstGlobalSet{Name: "$__stack_pointer"},
		)
	} else {
		// Null-terminate string and return bufLocal pointer
		insts = append(insts,
			InstLocalGet{Name: curLocal},
			InstConstI32{Value: 0},
			InstStore{Op: "i32.store8", Offset: 0},
			InstLocalGet{Name: bufLocal},
		)
	}

	return insts
}

func (l *lowerer) mapBinaryOp(op loneast.BinaryOp, operandType loneanalyzer.Type) string {
	isFloat := isFloatType(operandType)
	prefix := "i32."
	if operandType == loneanalyzer.TypeF32 {
		prefix = "f32."
	} else if operandType == loneanalyzer.TypeF64 {
		prefix = "f64."
	} else if operandType == loneanalyzer.TypeI64 || operandType == loneanalyzer.TypeU64 {
		prefix = "i64."
	}

	switch op {
	case loneast.BINARY_OP_PLUS:
		return prefix + "add"
	case loneast.BINARY_OP_MINUS:
		return prefix + "sub"
	case loneast.BINARY_OP_TIME:
		return prefix + "mul"
	case loneast.BINARY_OP_DIV:
		if isFloat {
			return prefix + "div"
		}
		return prefix + "div_s"
	case loneast.BINARY_OP_MOD:
		return prefix + "rem_s"
	case loneast.BINARY_OP_EQ:
		return prefix + "eq"
	case loneast.BINARY_OP_NOT_EQ:
		return prefix + "ne"
	case loneast.BINARY_OP_LESS:
		if isFloat {
			return prefix + "lt"
		}
		return prefix + "lt_s"
	case loneast.BINARY_OP_LESS_EQ:
		if isFloat {
			return prefix + "le"
		}
		return prefix + "le_s"
	case loneast.BINARY_OP_GREATER:
		if isFloat {
			return prefix + "gt"
		}
		return prefix + "gt_s"
	case loneast.BINARY_OP_GREATER_EQ:
		if isFloat {
			return prefix + "ge"
		}
		return prefix + "ge_s"
	case loneast.BINARY_OP_AND:
		return "i32.and"
	case loneast.BINARY_OP_OR:
		return "i32.or"
	case loneast.BINARY_OP_XOR:
		return "i32.xor"
	default:
		return prefix + "add"
	}
}

func (l *lowerer) mapCastOp(from, to loneanalyzer.Type) WasmInst {
	fromW := toWasmType(from)
	toW := toWasmType(to)

	if fromW == toW {
		return nil
	}

	if fromW == TypeI32 && toW == TypeF64 {
		return InstUnaryOp{Op: "f64.convert_i32_s"}
	}
	if fromW == TypeI32 && toW == TypeF32 {
		return InstUnaryOp{Op: "f32.convert_i32_s"}
	}
	if fromW == TypeF64 && toW == TypeI32 {
		return InstUnaryOp{Op: "i32.trunc_f64_s"}
	}
	if fromW == TypeF32 && toW == TypeI32 {
		return InstUnaryOp{Op: "i32.trunc_f32_s"}
	}
	if fromW == TypeI32 && toW == TypeI64 {
		return InstUnaryOp{Op: "i64.extend_i32_s"}
	}
	if fromW == TypeI64 && toW == TypeI32 {
		return InstUnaryOp{Op: "i32.wrap_i64"}
	}
	if fromW == TypeF32 && toW == TypeF64 {
		return InstUnaryOp{Op: "f64.promote_f32"}
	}
	if fromW == TypeF64 && toW == TypeF32 {
		return InstUnaryOp{Op: "f32.demote_f64"}
	}
	return nil
}

func (l *lowerer) formatGlobalInit(val loneanalyzer.TExpr, typ loneanalyzer.Type) string {
	if lit, ok := val.(*loneanalyzer.TLiteralExpr); ok {
		switch lit.Kind {
		case loneast.LiteralInt:
			if v, ok := lit.Value.(int64); ok {
				if typ == loneanalyzer.TypeI64 {
					return fmt.Sprintf("i64.const %d", v)
				}
				return fmt.Sprintf("i32.const %d", v)
			}
		case loneast.LiteralFloat:
			if v, ok := lit.Value.(float64); ok {
				return fmt.Sprintf("f64.const %g", v)
			}
		case loneast.LiteralBool:
			if b, ok := lit.Value.(bool); ok && b {
				return "i32.const 1"
			}
			return "i32.const 0"
		case loneast.LiteralString:
			if s, ok := lit.Value.(string); ok {
				offset, _ := l.allocString(s)
				return fmt.Sprintf("i32.const %d", offset)
			}
		}
	}
	return "i32.const 0"
}

func toWasmType(t loneanalyzer.Type) WasmType {
	if t == nil {
		return TypeVoid
	}
	switch t {
	case loneanalyzer.TypeI64, loneanalyzer.TypeU64:
		return TypeI64
	case loneanalyzer.TypeF32:
		return TypeF32
	case loneanalyzer.TypeF64:
		return TypeF64
	case loneanalyzer.TypeVoid:
		return TypeVoid
	default:
		return TypeI32
	}
}

func isFloatType(t loneanalyzer.Type) bool {
	return t == loneanalyzer.TypeF32 || t == loneanalyzer.TypeF64
}

func (l *lowerer) lowerEnumConstructExpr(e *loneanalyzer.TEnumConstructExpr, alloc *LocalAllocator) []WasmInst {
	var insts []WasmInst
	ptrLocal := alloc.AllocTemp(TypeI32)
	defer alloc.FreeTemp(ptrLocal, TypeI32)

	totalSize := e.EnumType.TotalSize
	if totalSize < 4 {
		totalSize = 4
	}

	// 1. Allocate memory for enum via Free-List Allocator
	insts = append(insts, InstConstI32{Value: int32(totalSize)})
	insts = append(insts, InstCall{Target: "$__lone_rt_alloc"})
	insts = append(insts, InstLocalSet{Name: ptrLocal})

	// 2. Store tag at offset 0
	insts = append(insts, InstLocalGet{Name: ptrLocal})
	insts = append(insts, InstConstI32{Value: int32(e.Variant.Tag)})
	insts = append(insts, InstStore{Op: "i32.store", Offset: 0})

	// 3. Store payload fields
	for i, arg := range e.Args {
		if i < len(e.Variant.Fields) {
			fieldInfo := e.Variant.Fields[i]
			insts = append(insts, InstLocalGet{Name: ptrLocal})
			insts = append(insts, l.lowerExpr(arg, alloc)...)
			insts = append(insts, InstStore{Op: storeOpForType(fieldInfo.Type), Offset: fieldInfo.Offset})
		}
	}

	// 4. Return pointer to enum on operand stack
	insts = append(insts, InstLocalGet{Name: ptrLocal})
	return insts
}

func (l *lowerer) lowerMatchExpr(m *loneanalyzer.TMatchExpr, alloc *LocalAllocator) []WasmInst {
	var insts []WasmInst

	targetType := m.Target.ExprType()
	targetWType := toWasmType(targetType)
	targetLocal := alloc.AllocTemp(targetWType)
	defer alloc.FreeTemp(targetLocal, targetWType)

	// Evaluate target
	insts = append(insts, l.lowerExpr(m.Target, alloc)...)
	insts = append(insts, InstLocalSet{Name: targetLocal})

	_, isEnum := targetType.(*loneanalyzer.EnumType)
	tagLocal := ""
	if isEnum {
		tagLocal = alloc.AllocTemp(TypeI32)
		defer alloc.FreeTemp(tagLocal, TypeI32)
		// Load tag from offset 0
		insts = append(insts, InstLocalGet{Name: targetLocal})
		insts = append(insts, InstLoad{Op: "i32.load", Offset: 0})
		insts = append(insts, InstLocalSet{Name: tagLocal})
	}

	var resultLocal string
	if m.Typ != loneanalyzer.TypeVoid && m.Typ != nil {
		resWType := toWasmType(m.Typ)
		resultLocal = alloc.AllocTemp(resWType)
		defer alloc.FreeTemp(resultLocal, resWType)
	}

	armInsts := l.lowerMatchArms(m.Arms, 0, targetLocal, tagLocal, resultLocal, isEnum, targetType, alloc)
	insts = append(insts, armInsts...)

	if resultLocal != "" {
		insts = append(insts, InstLocalGet{Name: resultLocal})
	}

	return insts
}

func (l *lowerer) lowerMatchArms(arms []loneanalyzer.TMatchArm, index int, targetLocal, tagLocal, resultLocal string, isEnum bool, targetType loneanalyzer.Type, alloc *LocalAllocator) []WasmInst {
	if index >= len(arms) {
		return nil
	}

	arm := arms[index]
	alloc.EnterScope()
	defer alloc.ExitScope()

	switch pat := arm.Pattern.(type) {
	case loneast.WildcardPattern:
		var thenInsts []WasmInst
		if arm.BodyExpr != nil {
			thenInsts = append(thenInsts, l.lowerExpr(arm.BodyExpr, alloc)...)
			if resultLocal != "" {
				thenInsts = append(thenInsts, InstLocalSet{Name: resultLocal})
			}
		} else if arm.BodyBlock != nil {
			thenInsts = append(thenInsts, l.lowerBlock(*arm.BodyBlock, alloc)...)
		}
		return thenInsts

	case loneast.VariantPattern:
		var condInsts []WasmInst
		condInsts = append(condInsts, InstLocalGet{Name: tagLocal})
		tagVal := 0
		if arm.Variant != nil {
			tagVal = arm.Variant.Tag
		}
		condInsts = append(condInsts, InstConstI32{Value: int32(tagVal)})
		condInsts = append(condInsts, InstBinaryOp{Op: "i32.eq"})

		var thenInsts []WasmInst
		// Bind payload variables
		if arm.Variant != nil {
			for i, b := range arm.Bindings {
				if i < len(arm.Variant.Fields) {
					fInfo := arm.Variant.Fields[i]
					bLocal := alloc.AllocVar(b.Name, toWasmType(fInfo.Type))
					thenInsts = append(thenInsts, InstLocalGet{Name: targetLocal})
					thenInsts = append(thenInsts, InstLoad{Op: loadOpForType(fInfo.Type), Offset: fInfo.Offset})
					thenInsts = append(thenInsts, InstLocalSet{Name: bLocal})
				}
			}
		}

		if arm.BodyExpr != nil {
			thenInsts = append(thenInsts, l.lowerExpr(arm.BodyExpr, alloc)...)
			if resultLocal != "" {
				thenInsts = append(thenInsts, InstLocalSet{Name: resultLocal})
			}
		} else if arm.BodyBlock != nil {
			thenInsts = append(thenInsts, l.lowerBlock(*arm.BodyBlock, alloc)...)
		}

		elseInsts := l.lowerMatchArms(arms, index+1, targetLocal, tagLocal, resultLocal, isEnum, targetType, alloc)
		var res []WasmInst
		res = append(res, condInsts...)
		res = append(res, InstIf{
			Then: thenInsts,
			Else: elseInsts,
		})
		return res

	case loneast.LiteralPattern:
		var condInsts []WasmInst
		condInsts = append(condInsts, InstLocalGet{Name: targetLocal})
		litWasm := l.lowerExpr(&loneanalyzer.TLiteralExpr{
			Kind:  pat.Literal.Kind,
			Value: pat.Literal.Value,
			Typ:   targetType,
			Span_: pat.Span_,
		}, alloc)
		condInsts = append(condInsts, litWasm...)
		eqOp := "i32.eq"
		if isFloatType(targetType) {
			if targetType == loneanalyzer.TypeF32 {
				eqOp = "f32.eq"
			} else {
				eqOp = "f64.eq"
			}
		} else if targetType == loneanalyzer.TypeI64 || targetType == loneanalyzer.TypeU64 {
			eqOp = "i64.eq"
		}
		condInsts = append(condInsts, InstBinaryOp{Op: eqOp})

		var thenInsts []WasmInst
		if arm.BodyExpr != nil {
			thenInsts = append(thenInsts, l.lowerExpr(arm.BodyExpr, alloc)...)
			if resultLocal != "" {
				thenInsts = append(thenInsts, InstLocalSet{Name: resultLocal})
			}
		} else if arm.BodyBlock != nil {
			thenInsts = append(thenInsts, l.lowerBlock(*arm.BodyBlock, alloc)...)
		}

		elseInsts := l.lowerMatchArms(arms, index+1, targetLocal, tagLocal, resultLocal, isEnum, targetType, alloc)
		var res []WasmInst
		res = append(res, condInsts...)
		res = append(res, InstIf{
			Then: thenInsts,
			Else: elseInsts,
		})
		return res

	default:
		return nil
	}
}

func loadOpForType(t loneanalyzer.Type) string {
	switch t {
	case loneanalyzer.TypeI64, loneanalyzer.TypeU64:
		return "i64.load"
	case loneanalyzer.TypeF32:
		return "f32.load"
	case loneanalyzer.TypeF64:
		return "f64.load"
	default:
		return "i32.load"
	}
}

func storeOpForType(t loneanalyzer.Type) string {
	switch t {
	case loneanalyzer.TypeI64, loneanalyzer.TypeU64:
		return "i64.store"
	case loneanalyzer.TypeF32:
		return "f32.store"
	case loneanalyzer.TypeF64:
		return "f64.store"
	default:
		return "i32.store"
	}
}

func (l *lowerer) lowerMustExpr(m *loneanalyzer.TMustExpr, alloc *LocalAllocator) []WasmInst {
	var insts []WasmInst
	targetLocal := alloc.AllocTemp(TypeI32)
	defer alloc.FreeTemp(targetLocal, TypeI32)

	insts = append(insts, l.lowerExpr(m.Expr, alloc)...)
	insts = append(insts, InstLocalSet{Name: targetLocal})

	resLocal := ""
	if m.OkType != loneanalyzer.TypeVoid && m.OkType != nil {
		resWType := toWasmType(m.OkType)
		resLocal = alloc.AllocTemp(resWType)
		defer alloc.FreeTemp(resLocal, resWType)
	}

	// Condition: target.tag == OkVariant.Tag
	var condInsts []WasmInst
	condInsts = append(condInsts, InstLocalGet{Name: targetLocal})
	condInsts = append(condInsts, InstLoad{Op: "i32.load", Offset: 0})
	condInsts = append(condInsts, InstConstI32{Value: int32(m.OkVariant.Tag)})
	condInsts = append(condInsts, InstBinaryOp{Op: "i32.eq"})

	var thenInsts []WasmInst
	if resLocal != "" && len(m.OkVariant.Fields) > 0 {
		fInfo := m.OkVariant.Fields[0]
		thenInsts = append(thenInsts, InstLocalGet{Name: targetLocal})
		thenInsts = append(thenInsts, InstLoad{Op: loadOpForType(fInfo.Type), Offset: fInfo.Offset})
		thenInsts = append(thenInsts, InstLocalSet{Name: resLocal})
	}

	// On Err: unreachable (panics in WebAssembly)
	elseInsts := []WasmInst{
		InstUnreachable{},
	}

	insts = append(insts, condInsts...)
	insts = append(insts, InstIf{
		Then: thenInsts,
		Else: elseInsts,
	})

	if resLocal != "" {
		insts = append(insts, InstLocalGet{Name: resLocal})
	}
	return insts
}

func (l *lowerer) lowerTryExpr(t *loneanalyzer.TTryExpr, alloc *LocalAllocator) []WasmInst {
	var insts []WasmInst
	targetLocal := alloc.AllocTemp(TypeI32)
	defer alloc.FreeTemp(targetLocal, TypeI32)

	insts = append(insts, l.lowerExpr(t.Expr, alloc)...)
	insts = append(insts, InstLocalSet{Name: targetLocal})

	resLocal := ""
	if t.OkType != loneanalyzer.TypeVoid && t.OkType != nil {
		resWType := toWasmType(t.OkType)
		resLocal = alloc.AllocTemp(resWType)
		defer alloc.FreeTemp(resLocal, resWType)
	}

	// Condition: target.tag == OkVariant.Tag
	var condInsts []WasmInst
	condInsts = append(condInsts, InstLocalGet{Name: targetLocal})
	condInsts = append(condInsts, InstLoad{Op: "i32.load", Offset: 0})
	condInsts = append(condInsts, InstConstI32{Value: int32(t.OkVariant.Tag)})
	condInsts = append(condInsts, InstBinaryOp{Op: "i32.eq"})

	var thenInsts []WasmInst
	if resLocal != "" && len(t.OkVariant.Fields) > 0 {
		fInfo := t.OkVariant.Fields[0]
		thenInsts = append(thenInsts, InstLocalGet{Name: targetLocal})
		thenInsts = append(thenInsts, InstLoad{Op: loadOpForType(fInfo.Type), Offset: fInfo.Offset})
		thenInsts = append(thenInsts, InstLocalSet{Name: resLocal})
	}

	// On Err: early-return the target (which is Err)
	elseInsts := []WasmInst{
		InstLocalGet{Name: targetLocal},
		InstReturn{},
	}

	insts = append(insts, condInsts...)
	insts = append(insts, InstIf{
		Then: thenInsts,
		Else: elseInsts,
	})

	if resLocal != "" {
		insts = append(insts, InstLocalGet{Name: resLocal})
	}
	return insts
}
