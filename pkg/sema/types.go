package sema

import (
	"fmt"
	ir "lone/pkg/ast"
	"strings"
)

// TypeKind identifies the structural category of a type.
type TypeKind int

const (
	TypeKindUnknown TypeKind = iota
	TypeKindPrimitive
	TypeKindCustom
	TypeKindList
	TypeKindFn
	TypeKindObject
	TypeKindInfer
)

func (k TypeKind) String() string {
	switch k {
	case TypeKindPrimitive:
		return "Primitive"
	case TypeKindCustom:
		return "Custom"
	case TypeKindList:
		return "List"
	case TypeKindFn:
		return "Fn"
	case TypeKindObject:
		return "Object"
	case TypeKindInfer:
		return "Infer"
	default:
		return "Unknown"
	}
}

// Type represents a static type in lone-alyzer.
type Type interface {
	Kind() TypeKind
	String() string
	Equals(other Type) bool
	AssignableTo(target Type) bool
}

// =====================
// PRIMITIVE TYPE
// =====================

type PrimitiveType struct {
	Name string
}

func (p *PrimitiveType) Kind() TypeKind { return TypeKindPrimitive }
func (p *PrimitiveType) String() string  { return p.Name }
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
		if op.Name == "any" {
			return true
		}
		if p.Name == op.Name {
			return true
		}
		// Allow numeric flexibility between float types and integer to float
		if (p.Name == "f32" && op.Name == "f64") || (p.Name == "f64" && op.Name == "f32") {
			return true
		}
		if (p.Name == "i32" || p.Name == "i64" || p.Name == "i16" || p.Name == "i8") && (op.Name == "f32" || op.Name == "f64") {
			return true
		}
	}
	return false
}

// Standard primitive singletons
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
	TypeI128    = &PrimitiveType{Name: "i128"}
	TypeU8      = &PrimitiveType{Name: "u8"}
	TypeU16     = &PrimitiveType{Name: "u16"}
	TypeU32     = &PrimitiveType{Name: "u32"}
	TypeU64     = &PrimitiveType{Name: "u64"}
	TypeU128    = &PrimitiveType{Name: "u128"}
	TypeF32     = &PrimitiveType{Name: "f32"}
	TypeF64     = &PrimitiveType{Name: "f64"}
)

// PrimitiveByName returns the singleton for a given primitive name, or nil if not primitive.
func PrimitiveByName(name string) *PrimitiveType {
	switch name {
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
	case "i8":
		return TypeI8
	case "i16":
		return TypeI16
	case "i32":
		return TypeI32
	case "i64":
		return TypeI64
	case "i128":
		return TypeI128
	case "u8":
		return TypeU8
	case "u16":
		return TypeU16
	case "u32":
		return TypeU32
	case "u64":
		return TypeU64
	case "u128":
		return TypeU128
	case "f32":
		return TypeF32
	case "f64":
		return TypeF64
	case "infer":
		return TypeInfer
	default:
		return nil
	}
}

// =====================
// CUSTOM TYPE
// =====================

type CustomType struct {
	Name string
}

func (c *CustomType) Kind() TypeKind { return TypeKindCustom }
func (c *CustomType) String() string  { return c.Name }
func (c *CustomType) Equals(other Type) bool {
	if oc, ok := other.(*CustomType); ok {
		return c.Name == oc.Name
	}
	return false
}
func (c *CustomType) AssignableTo(target Type) bool {
	if target == nil {
		return false
	}
	if target == TypeAny {
		return true
	}
	if oc, ok := target.(*CustomType); ok {
		return c.Name == oc.Name
	}
	return false
}

// =====================
// LIST TYPE
// =====================

type ListType struct {
	ItemType Type
	Size     int // -1 for dynamic
}

func (l *ListType) Kind() TypeKind { return TypeKindList }
func (l *ListType) String() string {
	if l.Size < 0 {
		return fmt.Sprintf("[]%s", l.ItemType.String())
	}
	return fmt.Sprintf("[%d]%s", l.Size, l.ItemType.String())
}
func (l *ListType) Equals(other Type) bool {
	if ol, ok := other.(*ListType); ok {
		return l.Size == ol.Size && l.ItemType.Equals(ol.ItemType)
	}
	return false
}
func (l *ListType) AssignableTo(target Type) bool {
	if target == nil {
		return false
	}
	if target == TypeAny {
		return true
	}
	if ol, ok := target.(*ListType); ok {
		if l.Size >= 0 && ol.Size >= 0 && l.Size != ol.Size {
			return false
		}
		return l.ItemType.AssignableTo(ol.ItemType)
	}
	return false
}

// =====================
// FN TYPE
// =====================

type FnParamType struct {
	Name string
	Type Type
}

type FnType struct {
	Params     []FnParamType
	ReturnType Type
}

func (f *FnType) Kind() TypeKind { return TypeKindFn }
func (f *FnType) String() string {
	var params []string
	for _, p := range f.Params {
		if p.Name != "" {
			params = append(params, fmt.Sprintf("%s: %s", p.Name, p.Type.String()))
		} else {
			params = append(params, p.Type.String())
		}
	}
	ret := "void"
	if f.ReturnType != nil {
		ret = f.ReturnType.String()
	}
	return fmt.Sprintf("fn(%s) %s", strings.Join(params, ", "), ret)
}
func (f *FnType) Equals(other Type) bool {
	of, ok := other.(*FnType)
	if !ok {
		return false
	}
	if len(f.Params) != len(of.Params) {
		return false
	}
	for i := range f.Params {
		if !f.Params[i].Type.Equals(of.Params[i].Type) {
			return false
		}
	}
	if (f.ReturnType == nil) != (of.ReturnType == nil) {
		return false
	}
	if f.ReturnType != nil && !f.ReturnType.Equals(of.ReturnType) {
		return false
	}
	return true
}
func (f *FnType) AssignableTo(target Type) bool {
	return f.Equals(target)
}

// =====================
// OBJECT TYPE
// =====================

type ObjectType struct {
	Fields map[string]Type
}

func (o *ObjectType) Kind() TypeKind { return TypeKindObject }
func (o *ObjectType) String() string {
	var parts []string
	for k, v := range o.Fields {
		parts = append(parts, fmt.Sprintf(".%s: %s", k, v.String()))
	}
	return fmt.Sprintf("{ %s }", strings.Join(parts, ", "))
}
func (o *ObjectType) Equals(other Type) bool {
	oo, ok := other.(*ObjectType)
	if !ok {
		return false
	}
	if len(o.Fields) != len(oo.Fields) {
		return false
	}
	for k, v := range o.Fields {
		ov, exists := oo.Fields[k]
		if !exists || !v.Equals(ov) {
			return false
		}
	}
	return true
}
func (o *ObjectType) AssignableTo(target Type) bool {
	if target == nil {
		return false
	}
	if target == TypeAny {
		return true
	}
	oo, ok := target.(*ObjectType)
	if !ok {
		return false
	}
	for k, targetFieldType := range oo.Fields {
		srcFieldType, exists := o.Fields[k]
		if !exists || !srcFieldType.AssignableTo(targetFieldType) {
			return false
		}
	}
	return true
}

// ParseTypeRef converts an AST ir.TYPE_REF into a lone-alyzer Type.
func ParseTypeRef(tr *ir.TYPE_REF) Type {
	if tr == nil {
		return TypeInfer
	}

	switch tr.Kind {
	case ir.TYPE_BASE:
		base := tr.AsBase()
		if base == nil || base.Name == "" {
			return TypeInfer
		}
		if prim := PrimitiveByName(base.Name); prim != nil {
			return prim
		}
		return &CustomType{Name: base.Name}

	case ir.TYPE_LIST:
		list := tr.AsList()
		if list == nil {
			return &ListType{ItemType: TypeUnknown, Size: -1}
		}
		itemType := ParseTypeRef(&list.ItemType)
		size := -1
		if list.IsFixed() {
			if lit, ok := list.Size.(ir.Literal); ok {
				if s, ok := lit.Value.(int); ok {
					size = s
				} else if s64, ok := lit.Value.(int64); ok {
					size = int(s64)
				}
			}
		}
		return &ListType{ItemType: itemType, Size: size}

	case ir.TYPE_FN:
		fn := tr.AsFn()
		if fn == nil {
			return &FnType{ReturnType: TypeVoid}
		}
		var params []FnParamType
		for _, p := range fn.Params {
			params = append(params, FnParamType{
				Name: p.Name,
				Type: ParseTypeRef(&p.Type),
			})
		}
		var retType Type = TypeVoid
		if fn.ReturnType != nil {
			retType = ParseTypeRef(fn.ReturnType)
		}
		return &FnType{
			Params:     params,
			ReturnType: retType,
		}
	}

	return TypeUnknown
}
