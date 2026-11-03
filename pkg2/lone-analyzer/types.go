package loneanalyzer

import (
	"fmt"
	"strings"
)

type TypeKind int

const (
	TypeKindUnknown TypeKind = iota
	TypeKindPrimitive
	TypeKindList
	TypeKindFn
	TypeKindInfer
	TypeKindRef
	TypeKindPtr
	TypeKindStruct
	TypeKindTrait
	TypeKindOptional
)

func (k TypeKind) String() string {
	switch k {
	case TypeKindPrimitive:
		return "Primitive"
	case TypeKindList:
		return "List"
	case TypeKindFn:
		return "Fn"
	case TypeKindInfer:
		return "Infer"
	case TypeKindRef:
		return "Ref"
	case TypeKindPtr:
		return "Ptr"
	case TypeKindStruct:
		return "Struct"
	case TypeKindTrait:
		return "Trait"
	case TypeKindOptional:
		return "Optional"
	default:
		return "Unknown"
	}
}

// Type represents a static type in the Lone type system.
type Type interface {
	Kind() TypeKind
	String() string
	Equals(other Type) bool
	AssignableTo(target Type) bool
}

// =====================
// PRIMITIVES
// =====================

type PrimitiveType struct {
	Name string
}

func (p *PrimitiveType) Kind() TypeKind { return TypeKindPrimitive }
func (p *PrimitiveType) String() string { return p.Name }

func (p *PrimitiveType) Equals(other Type) bool {
	if other == nil {
		return false
	}
	if op, ok := other.(*PrimitiveType); ok {
		return p.Name == op.Name
	}
	return false
}

func (p *PrimitiveType) AssignableTo(target Type) bool {
	if target == nil {
		return false
	}
	if target.Kind() == TypeKindUnknown || target == TypeAny {
		return true
	}
	if op, ok := target.(*PrimitiveType); ok {
		if op.Name == "any" || p.Name == "any" {
			return true
		}
		// Exact match
		if p.Name == op.Name {
			return true
		}
		// NOTE: In accordance with IL1 (Explicit over implicit), Lone rejects silent
		// cross-domain coercions (int to float, f64 to f32, i64 to i32).
		// Such conversions require explicit 'as' cast expressions.
		return false
	}
	if opt, ok := target.(*OptionalType); ok {
		if p == TypeNone {
			return true
		}
		return p.AssignableTo(opt.Inner)
	}
	return false
}

var (
	TypeUnknown = &PrimitiveType{Name: "unknown"}
	TypeInfer   = &PrimitiveType{Name: "infer"}
	TypeVoid    = &PrimitiveType{Name: "void"}
	TypeNone    = &PrimitiveType{Name: "none"}
	TypeAny     = &PrimitiveType{Name: "any"}
	TypeBool    = &PrimitiveType{Name: "bool"}
	TypeStr     = &PrimitiveType{Name: "str"}
	TypeChar    = &PrimitiveType{Name: "char"}
	TypeI8      = &PrimitiveType{Name: "i8"}
	TypeI16     = &PrimitiveType{Name: "i16"}
	TypeI32     = &PrimitiveType{Name: "i32"}
	TypeI64     = &PrimitiveType{Name: "i64"}
	TypeU8      = &PrimitiveType{Name: "u8"}
	TypeU16     = &PrimitiveType{Name: "u16"}
	TypeU32     = &PrimitiveType{Name: "u32"}
	TypeU64     = &PrimitiveType{Name: "u64"}
	TypeF32     = &PrimitiveType{Name: "f32"}
	TypeF64     = &PrimitiveType{Name: "f64"}
)

func PrimitiveByName(name string) *PrimitiveType {
	switch name {
	case "infer":
		return TypeInfer
	case "unknown":
		return TypeUnknown
	case "void":
		return TypeVoid
	case "none":
		return TypeNone
	case "any":
		return TypeAny
	case "bool":
		return TypeBool
	case "str", "string":
		return TypeStr
	case "char":
		return TypeChar
	case "i8", "int8":
		return TypeI8
	case "i16", "int16":
		return TypeI16
	case "i32", "int", "int32":
		return TypeI32
	case "i64", "int64":
		return TypeI64
	case "u8", "uint8":
		return TypeU8
	case "u16", "uint16":
		return TypeU16
	case "u32", "uint", "uint32":
		return TypeU32
	case "u64", "uint64":
		return TypeU64
	case "f32", "float", "float32":
		return TypeF32
	case "f64", "float64":
		return TypeF64
	default:
		return nil
	}
}

// =====================
// POINTER & REFERENCE
// =====================

type PtrType struct {
	Elem     Type
	ReadOnly bool
}

func (p *PtrType) Kind() TypeKind { return TypeKindPtr }
func (p *PtrType) String() string {
	ro := ""
	if p.ReadOnly {
		ro = "ro "
	}
	return fmt.Sprintf("%sptr<%s>", ro, p.Elem.String())
}
func (p *PtrType) Equals(other Type) bool {
	if op, ok := other.(*PtrType); ok {
		return p.ReadOnly == op.ReadOnly && p.Elem.Equals(op.Elem)
	}
	return false
}
func (p *PtrType) AssignableTo(target Type) bool {
	if op, ok := target.(*PtrType); ok {
		if op.ReadOnly || !p.ReadOnly {
			return p.Elem.Equals(op.Elem)
		}
	}
	return false
}

type RefType struct {
	Elem     Type
	ReadOnly bool
}

func (r *RefType) Kind() TypeKind { return TypeKindRef }
func (r *RefType) String() string {
	ro := ""
	if r.ReadOnly {
		ro = "ro "
	}
	return fmt.Sprintf("%sref<%s>", ro, r.Elem.String())
}
func (r *RefType) Equals(other Type) bool {
	if or, ok := other.(*RefType); ok {
		return r.ReadOnly == or.ReadOnly && r.Elem.Equals(or.Elem)
	}
	return false
}
func (r *RefType) AssignableTo(target Type) bool {
	if or, ok := target.(*RefType); ok {
		if or.ReadOnly || !r.ReadOnly {
			return r.Elem.Equals(or.Elem)
		}
	}
	return false
}

// =====================
// LIST & ARRAY
// =====================

type ListType struct {
	Elem  Type
	Fixed bool
	Size  int
}

func (l *ListType) Kind() TypeKind { return TypeKindList }
func (l *ListType) String() string {
	if l.Fixed {
		return fmt.Sprintf("[%d]%s", l.Size, l.Elem.String())
	}
	return fmt.Sprintf("[]%s", l.Elem.String())
}
func (l *ListType) Equals(other Type) bool {
	if ol, ok := other.(*ListType); ok {
		return l.Fixed == ol.Fixed && l.Size == ol.Size && l.Elem.Equals(ol.Elem)
	}
	return false
}
func (l *ListType) AssignableTo(target Type) bool {
	if ol, ok := target.(*ListType); ok {
		if !ol.Fixed {
			return l.Elem.Equals(ol.Elem)
		}
		return l.Fixed && l.Size == ol.Size && l.Elem.Equals(ol.Elem)
	}
	return false
}

// =====================
// STRUCT
// =====================

type StructFieldInfo struct {
	Name   string
	Type   Type
	Offset int
}

type StructType struct {
	Name        string
	Fields      []StructFieldInfo
	FieldMap    map[string]StructFieldInfo
	Methods     map[string]*FnType
	TotalSize   int
	TotalAlign  int
}

func (s *StructType) Kind() TypeKind { return TypeKindStruct }
func (s *StructType) String() string { return s.Name }
func (s *StructType) Equals(other Type) bool {
	if os, ok := other.(*StructType); ok {
		return s.Name == os.Name
	}
	return false
}
func (s *StructType) AssignableTo(target Type) bool {
	if os, ok := target.(*StructType); ok {
		return s.Name == os.Name
	}
	return false
}
func (s *StructType) Field(name string) (StructFieldInfo, bool) {
	f, ok := s.FieldMap[name]
	return f, ok
}

// =====================
// TRAIT
// =====================

type TraitMethodSig struct {
	Name   string
	FnType *FnType
}

type TraitType struct {
	Name    string
	Methods map[string]*FnType
}

func (t *TraitType) Kind() TypeKind { return TypeKindTrait }
func (t *TraitType) String() string { return t.Name }
func (t *TraitType) Equals(other Type) bool {
	if ot, ok := other.(*TraitType); ok {
		return t.Name == ot.Name
	}
	return false
}
func (t *TraitType) AssignableTo(target Type) bool {
	if ot, ok := target.(*TraitType); ok {
		return t.Name == ot.Name
	}
	return false
}

// =====================
// OPTIONAL
// =====================

type OptionalType struct {
	Inner Type
}

func (o *OptionalType) Kind() TypeKind { return TypeKindOptional }
func (o *OptionalType) String() string { return fmt.Sprintf("?%s", o.Inner.String()) }
func (o *OptionalType) Equals(other Type) bool {
	if oo, ok := other.(*OptionalType); ok {
		return o.Inner.Equals(oo.Inner)
	}
	return false
}
func (o *OptionalType) AssignableTo(target Type) bool {
	if oo, ok := target.(*OptionalType); ok {
		return o.Inner.AssignableTo(oo.Inner)
	}
	return false
}

// =====================
// FUNCTION
// =====================

type FnParamInfo struct {
	Name string
	Type Type
}

type FnType struct {
	Params []FnParamInfo
	Return Type
}

func (f *FnType) Kind() TypeKind { return TypeKindFn }
func (f *FnType) String() string {
	var parts []string
	for _, p := range f.Params {
		parts = append(parts, fmt.Sprintf("%s: %s", p.Name, p.Type.String()))
	}
	return fmt.Sprintf("fn(%s) %s", strings.Join(parts, ", "), f.Return.String())
}
func (f *FnType) Equals(other Type) bool {
	if of, ok := other.(*FnType); ok {
		if len(f.Params) != len(of.Params) || !f.Return.Equals(of.Return) {
			return false
		}
		for i := range f.Params {
			if !f.Params[i].Type.Equals(of.Params[i].Type) {
				return false
			}
		}
		return true
	}
	return false
}
func (f *FnType) AssignableTo(target Type) bool {
	return f.Equals(target)
}

// =====================
// SIZES & ALIGNMENTS
// =====================

func TypeSize(t Type) int {
	if t == nil {
		return 0
	}
	switch v := t.(type) {
	case *PrimitiveType:
		switch v.Name {
		case "i8", "u8", "bool", "char":
			return 1
		case "i16", "u16":
			return 2
		case "i32", "u32", "f32":
			return 4
		case "i64", "u64", "f64":
			return 8
		case "str":
			return 8 // ptr (4 bytes) + len (4 bytes)
		case "void", "none":
			return 0
		default:
			return 4
		}
	case *PtrType, *RefType:
		return 4 // 32-bit linear memory offset
	case *OptionalType:
		return TypeSize(v.Inner) + 4 // discriminant tag + value
	case *StructType:
		return v.TotalSize
	case *ListType:
		if v.Fixed {
			return v.Size * TypeSize(v.Elem)
		}
		return 8 // slice view: ptr (4) + len (4)
	default:
		return 4
	}
}

func TypeAlign(t Type) int {
	if t == nil {
		return 1
	}
	switch v := t.(type) {
	case *PrimitiveType:
		switch v.Name {
		case "i8", "u8", "bool", "char":
			return 1
		case "i16", "u16":
			return 2
		case "i32", "u32", "f32", "str":
			return 4
		case "i64", "u64", "f64":
			return 8
		default:
			return 4
		}
	case *PtrType, *RefType:
		return 4
	case *StructType:
		return v.TotalAlign
	default:
		return 4
	}
}

func IsIntegerType(t Type) bool {
	if p, ok := t.(*PrimitiveType); ok {
		switch p.Name {
		case "i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64":
			return true
		}
	}
	return false
}

func IsFloatType(t Type) bool {
	if p, ok := t.(*PrimitiveType); ok {
		return p.Name == "f32" || p.Name == "f64"
	}
	return false
}
