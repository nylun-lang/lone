" Vim syntax file for Lone programming language
if exists("b:current_syntax")
  finish
endif

syntax keyword loneKeyword fn var let type struct interface enum trait impl return break if else while loop for in match must try drop ro comptime pub import from as and or xor is not where satisfies self Self
syntax keyword loneBoolean true false
syntax keyword loneConstant none
syntax keyword loneType i8 i16 i32 i64 i128 u8 u16 u32 u64 u128 f32 f64 int bool str string char void any infer ptr ref

" Compiler intrinsics start with #
syntax match loneIntrinsic "#[a-zA-Z_][a-zA-Z0-9_]*"

" Compiler directives start with @
syntax match loneDirective "@[a-zA-Z_][a-zA-Z0-9_]*:[a-zA-Z_][a-zA-Z0-9_]*"

" Numbers (integers and floats with underscores)
syntax match loneNumber "\<\d\+\(_\d\+\)*\(\.\d\+\(_\d\+\)*\)\?\>"

" Strings & String Interpolation
syntax region loneString start=+"+ skip=+\\"+ end=+"+ contains=loneEscape
syntax region loneFormatString start=+f"+ skip=+\\"+ end=+"+ contains=loneEscape,loneInterpolation
syntax region loneInterpolation contained matchgroup=Special start=+\${+ end=+}+ contains=TOP
syntax match loneEscape contained "\\."

" Comments
syntax region loneBlockComment start="---" end="---"
syntax match loneLineComment "--.*$"

" Highlights
highlight default link loneKeyword Keyword
highlight default link loneBoolean Boolean
highlight default link loneConstant Constant
highlight default link loneType Type
highlight default link loneIntrinsic Special
highlight default link loneDirective PreProc
highlight default link loneNumber Number
highlight default link loneString String
highlight default link loneFormatString String
highlight default link loneEscape SpecialChar
highlight default link loneBlockComment Comment
highlight default link loneLineComment Comment

let b:current_syntax = "lone"
