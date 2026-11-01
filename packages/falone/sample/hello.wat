(module
  (import "wasi_snapshot_preview1" "fd_write" (func $__wasi_fd_write (param i32 i32 i32 i32) (result i32)))
  (memory (export "memory") 1)
  (data (i32.const 16) "Hello from Lone!\0a\00")

  ;; Inlined WASI #print runtime implementation
  (func $__lone_rt_print (param $ptr i32) (param $len i32)
    (i32.store (i32.const 0) (local.get $ptr))
    (i32.store (i32.const 4) (local.get $len))
    (drop (call $__wasi_fd_write (i32.const 1) (i32.const 0) (i32.const 1) (i32.const 8)))
  )

  (func $main (export "_start") (export "main")
    ;; label entry
    i32.const 16
    i32.const 17
    call $__lone_rt_print
    return
  )
)
