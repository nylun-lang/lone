(module
  (memory (export "memory") 1)
  (global $APP_VERSION (mut i32) (i32.const 0))
  (global $max_retries (mut i32) (i32.const 0))

  (func $compute_total (param $base i32) (param $multiplier i32) (result i32)
    (local $v_result2 i32)
    (local $t_bin1 i32)
    ;; label entry
    local.get $base
    local.get $multiplier
    i32.mul
    local.set $t_bin1
    local.get $t_bin1
    local.set $v_result2
    local.get $v_result2
    return
  )

  (func $execute_pipeline
    (local $v_initial_val1 i32)
    (local $t_call2 i32)
    (local $v_final_score3 i32)
    (local $t_call4 i32)
    (local $v_loop_count5 i32)
    (local $t_bin6 i32)
    (local $t_bin7 i32)
    (local $v_computed_label8 i32)
    ;; label entry
    i32.const 50
    local.set $v_initial_val1
    local.get $v_initial_val1
    i32.const 2
    call $compute_total
    local.set $t_call2
    local.get $t_call2
    local.set $v_final_score3
    i32.const 10
    i32.const 5
    call $compute_total
    local.set $t_call4
    i32.const 0
    local.set $v_loop_count5
    (block $while_exit_3
      (loop $while_head_1
        ;; Evaluate condition
        local.get $v_loop_count5
        global.get $max_retries
        i32.lt_s
        local.set $t_bin6
        ;; Exit if condition is false
        local.get $t_bin6
        i32.eqz
        br_if $while_exit_3

        ;; Body
        local.get $v_loop_count5
        i32.const 1
        i32.add
        local.set $t_bin7
        br $while_exit_3
        ;; Repeat
        br $while_head_1
      )
    )

    ;; label while_exit_3
    i32.const 0
    local.set $v_computed_label8
    return
  )
)
