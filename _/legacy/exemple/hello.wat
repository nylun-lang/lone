(module
  (import "env" "print" (func $__lone_rt_print (param i32 i32)))
  (memory (export "memory") 1)
  (data (i32.const 16) "Hello from Lone !\00")

  (func $main (export "_start") (export "main")
    ;; label entry
    i32.const 16
    i32.const 17
    call $__lone_rt_print
    return
  )
)
