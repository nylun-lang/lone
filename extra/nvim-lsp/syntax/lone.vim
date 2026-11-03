" Vim syntax file for Lone programming language
if exists("b:current_syntax")
  finish
endif

syntax keyword loneKeyword fn var let type struct trait impl return break if else while drop ro comptime pub import from as and or
syntax keyword loneBoolean true false
syntax keyword loneConstant none
syntax keyword loneType i8 i16 i32 i64 i128 u8 u16 u32 u64 u128 f32 f64 int bool str string char void any infer ptr ref

" Compiler intrinsics start with #
syntax match loneIntrinsic "#[a-zA-Z_][a-zA-Z0-9_]*"

" Numbers (integers and floats with underscores)
syntax match loneNumber "\<\d\+\(_\d\+\)*\(\.\d\+\(_\d\+\)*\)\?\>"

" Strings
syntax region loneString start=+"+ skip=+\\"+ end=+"+ contains=loneEscape
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
highlight default link loneNumber Number
highlight default link loneString String
highlight default link loneEscape SpecialChar
highlight default link loneBlockComment Comment
highlight default link loneLineComment Comment

let b:current_syntax = "lone"
