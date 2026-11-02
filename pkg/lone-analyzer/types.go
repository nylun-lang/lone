package loneanalyzer

import (
	"fmt"
	ir "lone/pkg/lone-parser-ast"
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
	TypeKindRef
	TypeKindPtr
	TypeKindStruct
	TypeKindTrait
	TypeKindTraitMat
	TypeKindModule
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
	case TypeKindRef:
		return "Ref"
	case TypeKindPtr:
		return "Ptr"
	case TypeKindStruct:
		return "Struct"
	case TypeKindTrait:
		return "Trait"
	case TypeKindTraitMat:
		return "TraitMat"
	case TypeKindModule:
		return "Module"
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

func isSmallOrWordInt(name string) bool {
	switch name {
	case "i8", "i16", "i32", "u8", "u16", "u32", "char":
		return true
	default:
		return false
	}
}

func isAnyInt(name string) bool {
	return isSmallOrWordInt(name) || name == "i64" || name == "u64" || name == "i128" || name == "u128"
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
		if p.Name == op.Name {
			return true
		}
		if isSmallOrWordInt(p.Name) && isAnyInt(op.Name) {
			return true
		}
		if isAnyInt(p.Name) && (op.Name == "i64" || op.Name == "u64") {
			return true
		}
		// Allow numeric flexibility between float types and integer to float
		if (p.Name == "f32" && op.Name == "f64") || (p.Name == "f64" && op.Name == "f32") {
			return true
		}
		if isAnyInt(p.Name) && (op.Name == "f32" || op.Name == "f64") {
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
	case "i8", "int8":
		return TypeI8
	case "i16", "int16":
		return TypeI16
	case "i32", "int", "int32":
		return TypeI32
	case "i64", "int64":
		return TypeI64
	case "i128":
		return TypeI128
	case "u8", "uint8":
		return TypeU8
	case "u16", "uint16":
		return TypeU16
	case "u32", "uint", "uint32":
		return TypeU32
	case "u64", "uint64":
		return TypeU64
	case "u128":
		return TypeU128
	case "f32", "float", "float32":
		return TypeF32
	case "f64", "float64":
		return TypeF64
	case "infer":
		return TypeInfer
	default:
		return nil
	}
}

// =====================
// REF & PTR TYPES
// =====================

// RefType represents a managed reference ref<T>.
type RefType struct {
	Elem     Type
	ReadOnly bool
}

func (r *RefType) Kind() TypeKind { return TypeKindRef }
func (r *RefType) String() string {
	if r.ReadOnly {
		return fmt.Sprintf("ref<ro %s>", r.Elem.String())
	}
	return fmt.Sprintf("ref<%s>", r.Elem.String())
}
func (r *RefType) Equals(other Type) bool {
	or, ok := other.(*RefType)
	if !ok {
		return false
	}
	return r.ReadOnly == or.ReadOnly && r.Elem.Equals(or.Elem)
}
func (r *RefType) AssignableTo(target Type) bool {
	if target == nil {
		return false
	}
	if target == TypeAny || target.Kind() == TypeKindUnknown {
		return true
	}
	or, ok := target.(*RefType)
	if !ok {
		return false
	}
	if r.ReadOnly && !or.ReadOnly {
		return false
	}
	if or.Elem == TypeAny || r.Elem == TypeAny {
		return true
	}
	return r.Elem.Equals(or.Elem)
}

// PtrType represents an unsafe raw linear memory address ptr<T> (Wasm i32 byte offset).
type PtrType struct {
	Elem     Type
	ReadOnly bool
}

func (p *PtrType) Kind() TypeKind { return TypeKindPtr }
func (p *PtrType) String() string {
	if p.ReadOnly {
		return fmt.Sprintf("ptr<ro %s>", p.Elem.String())
	}
	return fmt.Sprintf("ptr<%s>", p.Elem.String())
}
func (p *PtrType) Equals(other Type) bool {
	op, ok := other.(*PtrType)
	if !ok {
		return false
	}
	return p.ReadOnly == op.ReadOnly && p.Elem.Equals(op.Elem)
}
func (p *PtrType) AssignableTo(target Type) bool {
	if target == nil {
		return false
	}
	if target == TypeAny || target.Kind() == TypeKindUnknown {
		return true
	}
	op, ok := target.(*PtrType)
	if !ok {
		return false
	}
	if p.ReadOnly && !op.ReadOnly {
		return false
	}
	if op.Elem == TypeAny || p.Elem == TypeAny {
		return true
	}
	return p.Elem.Equals(op.Elem)
}

// =====================
// STRUCT, TRAIT & MODULE TYPES
// =====================

type StructFieldInfo struct {
	Name   string
	Type   Type
	Offset int
	Size   int
}

type StructType struct {
	Name    string
	Fields  []StructFieldInfo
	Methods map[string]*FnType
	Size    int
	Align   int
}

func (s *StructType) Kind() TypeKind { return TypeKindStruct }
func (s *StructType) String() string { return s.Name }
func (s *StructType) Equals(other Type) bool {
	if os, ok := other.(*StructType); ok {
		return s.Name == os.Name
	}
	if oc, ok := other.(*CustomType); ok {
		return s.Name == oc.Name
	}
	return false
}
func (s *StructType) AssignableTo(target Type) bool {
	if target == nil {
		return false
	}
	if target == TypeAny || target.Kind() == TypeKindUnknown {
		return true
	}
	return s.Equals(target)
}

func (s *StructType) Field(name string) (*StructFieldInfo, bool) {
	for i := range s.Fields {
		if s.Fields[i].Name == name {
			return &s.Fields[i], true
		}
	}
	return nil, false
}

func alignUp(offset, align int) int {
	if align <= 1 {
		return offset
	}
	rem := offset % align
	if rem == 0 {
		return offset
	}
	return offset + (align - rem)
}

func (s *StructType) ComputeLayout() {
	offset := 0
	maxAlign := 4
	if len(s.Fields) > 0 {
		maxAlign = 1
	}
	for i := range s.Fields {
		fAlign := TypeAlign(s.Fields[i].Type)
		fSize := TypeSize(s.Fields[i].Type)
		if fAlign > maxAlign {
			maxAlign = fAlign
		}
		offset = alignUp(offset, fAlign)
		s.Fields[i].Offset = offset
		s.Fields[i].Size = fSize
		offset += fSize
	}
	if len(s.Fields) == 0 {
		s.Size = 4
		s.Align = 4
		return
	}
	s.Align = maxAlign
	s.Size = alignUp(offset, maxAlign)
}

type TraitType struct {
	Name       string
	TypeParams []string
	Methods    map[string]*FnType
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
	return t.Equals(target)
}

type TraitMatType struct {
	Trait  *TraitType
	Target Type
}

func (m *TraitMatType) Kind() TypeKind { return TypeKindTraitMat }
func (m *TraitMatType) String() string {
	return fmt.Sprintf("%s::|%s|", m.Trait.Name, m.Target.String())
}
func (m *TraitMatType) Equals(other Type) bool {
	om, ok := other.(*TraitMatType)
	if !ok {
		return false
	}
	return m.Trait.Equals(om.Trait) && m.Target.Equals(om.Target)
}
func (m *TraitMatType) AssignableTo(target Type) bool {
	if target == TypeAny {
		return true
	}
	return m.Equals(target)
}

type ModuleAliasType struct {
	Alias   string
	Source  string
	Exports map[string]*Symbol
}

func (m *ModuleAliasType) Kind() TypeKind { return TypeKindModule }
func (m *ModuleAliasType) String() string { return fmt.Sprintf("module<%s>", m.Source) }
func (m *ModuleAliasType) Equals(other Type) bool {
	om, ok := other.(*ModuleAliasType)
	return ok && m.Source == om.Source
}
func (m *ModuleAliasType) AssignableTo(target Type) bool { return m.Equals(target) }

// TypeSize returns the linear memory byte size of t.
func TypeSize(t Type) int {
	if t == nil {
		return 4
	}
	switch v := t.(type) {
	case *PrimitiveType:
		switch v.Name {
		case "void", "none":
			return 0
		case "i8", "u8", "bool", "char":
			return 1
		case "i16", "u16":
			return 2
		case "i64", "u64", "f64":
			return 8
		case "i128", "u128":
			return 16
		default:
			return 4
		}
	case *PtrType, *RefType, *TraitMatType, *FnType:
		return 4
	case *ListType:
		if v.Size >= 0 {
			sz := v.Size * TypeSize(v.ItemType)
			if sz == 0 {
				return 4
			}
			return sz
		}
		return 8 // slice fat pointer { ptr, len }
	case *StructType:
		if v.Size == 0 {
			v.ComputeLayout()
		}
		return v.Size
	default:
		return 4
	}
}

// TypeAlign returns the natural byte alignment of t in linear memory.
func TypeAlign(t Type) int {
	if t == nil {
		return 4
	}
	switch v := t.(type) {
	case *PrimitiveType:
		switch v.Name {
		case "i8", "u8", "bool", "char":
			return 1
		case "i16", "u16":
			return 2
		case "i64", "u64", "f64", "i128", "u128":
			return 8
		default:
			return 4
		}
	case *PtrType, *RefType, *TraitMatType, *FnType:
		return 4
	case *ListType:
		if v.Size >= 0 {
			return TypeAlign(v.ItemType)
		}
		return 4
	case *StructType:
		if v.Align == 0 {
			v.ComputeLayout()
		}
		return v.Align
	default:
		return 4
	}
}

// IsUnsignedType returns true if t is an unsigned integer or boolean/byte type.
func IsUnsignedType(t Type) bool {
	if p, ok := t.(*PrimitiveType); ok {
		switch p.Name {
		case "u8", "u16", "u32", "u64", "u128", "bool", "char":
			return true
		}
	}
	return false
}

// =====================
// CUSTOM TYPE
// =====================

type CustomType struct {
	Name string
}

func (c *CustomType) Kind() TypeKind { return TypeKindCustom }
func (c *CustomType) String() string { return c.Name }
func (c *CustomType) Equals(other Type) bool {
	if oc, ok := other.(*CustomType); ok {
		return c.Name == oc.Name
	}
	if os, ok := other.(*StructType); ok {
		return c.Name == os.Name
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
	return c.Equals(target)
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
	if st, ok := target.(*StructType); ok {
		if len(o.Fields) != len(st.Fields) {
			return false
		}
		for _, sf := range st.Fields {
			srcFieldType, exists := o.Fields[sf.Name]
			if !exists || !srcFieldType.AssignableTo(sf.Type) {
				return false
			}
		}
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

// ParseTypeRef converts an AST ir.TYPE_REF into a lone-alyzer Type without scope context.
func ParseTypeRef(tr *ir.TYPE_REF) Type {
	return ParseTypeRefWithResolver(tr, nil, nil)
}

// ParseTypeRefWithResolver converts an AST ir.TYPE_REF into a lone-alyzer Type using a scope type resolver and optional Self type.
func ParseTypeRefWithResolver(tr *ir.TYPE_REF, resolve func(string) Type, selfType Type) Type {
	if tr == nil {
		return TypeInfer
	}

	switch tr.Kind {
	case ir.TYPE_BASE:
		base := tr.AsBase()
		if base == nil || base.Name == "" {
			return TypeInfer
		}
		if base.Name == "Self" && selfType != nil {
			return selfType
		}
		if base.Name == "ptr" {
			var elem Type = TypeAny
			readOnly := false
			if len(base.Args) > 0 {
				readOnly = base.Args[0].Constrain.ReadOnly
				elem = ParseTypeRefWithResolver(&base.Args[0], resolve, selfType)
			}
			return &PtrType{Elem: elem, ReadOnly: readOnly}
		}
		if base.Name == "ref" {
			var elem Type = TypeAny
			readOnly := false
			if len(base.Args) > 0 {
				readOnly = base.Args[0].Constrain.ReadOnly
				elem = ParseTypeRefWithResolver(&base.Args[0], resolve, selfType)
			}
			return &RefType{Elem: elem, ReadOnly: readOnly}
		}
		if prim := PrimitiveByName(base.Name); prim != nil {
			return prim
		}
		if resolve != nil {
			if resolved := resolve(base.Name); resolved != nil {
				return resolved
			}
		}
		return &CustomType{Name: base.Name}

	case ir.TYPE_LIST:
		list := tr.AsList()
		if list == nil {
			return &ListType{ItemType: TypeUnknown, Size: -1}
		}
		itemType := ParseTypeRefWithResolver(&list.ItemType, resolve, selfType)
		size := -1
		if list.IsFixed() {
			sizeExpr := list.Size
			if pf, ok := sizeExpr.(ir.PostFixExpr); ok && len(pf.Suffix) == 0 {
				sizeExpr = pf.Base
			}
			if lit, ok := sizeExpr.(ir.Literal); ok {
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
				Type: ParseTypeRefWithResolver(&p.Type, resolve, selfType),
			})
		}
		var retType Type = TypeVoid
		if fn.ReturnType != nil {
			retType = ParseTypeRefWithResolver(fn.ReturnType, resolve, selfType)
		}
		return &FnType{
			Params:     params,
			ReturnType: retType,
		}
	}

	return TypeUnknown
}

// ParseTypeString parses simple host capability type strings (e.g. "i32", "void", "ptr<u8>", "ptr<any>").
func ParseTypeString(s string) Type {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "ptr<") && strings.HasSuffix(s, ">") {
		inner := strings.TrimSpace(s[4 : len(s)-1])
		ro := false
		if strings.HasPrefix(inner, "ro ") {
			ro = true
			inner = strings.TrimSpace(inner[3:])
		}
		return &PtrType{Elem: ParseTypeString(inner), ReadOnly: ro}
	}
	if strings.HasPrefix(s, "ref<") && strings.HasSuffix(s, ">") {
		inner := strings.TrimSpace(s[4 : len(s)-1])
		ro := false
		if strings.HasPrefix(inner, "ro ") {
			ro = true
			inner = strings.TrimSpace(inner[3:])
		}
		return &RefType{Elem: ParseTypeString(inner), ReadOnly: ro}
	}
	if prim := PrimitiveByName(s); prim != nil {
		return prim
	}
	return &CustomType{Name: s}
}
