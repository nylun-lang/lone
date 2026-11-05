package loneanalyzer

import (
	"fmt"
	loneast "lone/pkg2/lone-parser-ast"
	"strconv"
	"strings"
	"unicode"
)

// Generic templates
type GenericTypeTemplate struct {
	Name        string
	TypeParams  []loneast.TypeParam
	Constraints []loneast.GenericConstraint
	Decl        *loneast.TYPE_DEC
}

type GenericFnTemplate struct {
	Name        string
	TypeParams  []loneast.TypeParam
	Constraints []loneast.GenericConstraint
	Decl        *loneast.FN_DEC
}

// TypeEvaluator performs type checking, symbol resolution, and immutability validation.
type TypeEvaluator struct {
	stack           *ScopeStack
	diags           []Diagnostic
	structs         map[string]*StructType
	traits          map[string]*TraitType
	interfaces      map[string]*InterfaceType
	enums           map[string]*EnumType
	traitImpls      map[string]map[string]*loneast.IMPL_DEC
	genericTypes    map[string]*GenericTypeTemplate
	genericFns      map[string]*GenericFnTemplate
	instantiatedFns   map[string]*TFnDec
	inferredErrors    map[string][]Type
	currentTypeParams map[string]Type
	currentSelfType   Type
	currentFnReturn   Type
	currentFnName     string
	extraDecls        []TNode
}

func NewTypeEvaluator() *TypeEvaluator {
	te := &TypeEvaluator{
		stack:             NewScopeStack(),
		structs:           make(map[string]*StructType),
		traits:            make(map[string]*TraitType),
		interfaces:        make(map[string]*InterfaceType),
		enums:             make(map[string]*EnumType),
		traitImpls:        make(map[string]map[string]*loneast.IMPL_DEC),
		genericTypes:      make(map[string]*GenericTypeTemplate),
		genericFns:        make(map[string]*GenericFnTemplate),
		instantiatedFns:   make(map[string]*TFnDec),
		inferredErrors:    make(map[string][]Type),
		currentTypeParams: make(map[string]Type),
	}
	te.initBuiltins()
	return te
}

func (te *TypeEvaluator) initBuiltins() {
	// Built-in Error enum
	errorEnum := &EnumType{
		Name:       "Error",
		VariantMap: make(map[string]*EnumVariantInfo),
		TotalSize:  8,
		TotalAlign: 4,
	}
	vGeneric := EnumVariantInfo{
		Name:       "Generic",
		Tag:        0,
		Fields:     []StructFieldInfo{{Name: "msg", Type: TypeStr, Offset: 4}},
		FieldMap:   map[string]StructFieldInfo{"msg": {Name: "msg", Type: TypeStr, Offset: 4}},
		TotalSize:  8,
		TotalAlign: 4,
	}
	vCode := EnumVariantInfo{
		Name:       "Code",
		Tag:        1,
		Fields:     []StructFieldInfo{{Name: "code", Type: TypeI32, Offset: 4}},
		FieldMap:   map[string]StructFieldInfo{"code": {Name: "code", Type: TypeI32, Offset: 4}},
		TotalSize:  8,
		TotalAlign: 4,
	}
	vNotFound := EnumVariantInfo{
		Name:       "NotFound",
		Tag:        2,
		Fields:     nil,
		FieldMap:   make(map[string]StructFieldInfo),
		TotalSize:  4,
		TotalAlign: 4,
	}
	errorEnum.Variants = []EnumVariantInfo{vGeneric, vCode, vNotFound}
	errorEnum.VariantMap["Generic"] = &errorEnum.Variants[0]
	errorEnum.VariantMap["Code"] = &errorEnum.Variants[1]
	errorEnum.VariantMap["NotFound"] = &errorEnum.Variants[2]
	te.enums["Error"] = errorEnum
	_ = te.stack.Define(&Symbol{Name: "Error", Kind: SymbolType, Type: errorEnum})

	// Built-in Result<V, U = Error where { U is Error }> generic template
	te.genericTypes["Result"] = &GenericTypeTemplate{
		Name: "Result",
		TypeParams: []loneast.TypeParam{
			{Name: "V"},
			{
				Name: "U",
				Default: &loneast.TYPE_REF{
					Kind:  loneast.TYPE_BASE,
					Value: &loneast.TypeBase{Name: "Error"},
				},
			},
		},
		Constraints: []loneast.GenericConstraint{
			{
				TypeParam: "U",
				Op:        loneast.ConstraintIs,
				Target: loneast.TYPE_REF{
					Kind:  loneast.TYPE_BASE,
					Value: &loneast.TypeBase{Name: "Error"},
				},
			},
		},
		Decl: &loneast.TYPE_DEC{
			Name: "Result",
			Kind: loneast.TYPE_DEC_ENUM,
			Variants: []loneast.EnumVariant{
				{
					Name: "Ok",
					Fields: []loneast.StructField{
						{
							Name: "val",
							Type: loneast.TYPE_REF{
								Kind:  loneast.TYPE_BASE,
								Value: &loneast.TypeBase{Name: "V"},
							},
						},
					},
					Tag: 0,
				},
				{
					Name: "Err",
					Fields: []loneast.StructField{
						{
							Name: "err",
							Type: loneast.TYPE_REF{
								Kind:  loneast.TYPE_BASE,
								Value: &loneast.TypeBase{Name: "U"},
							},
						},
					},
					Tag: 1,
				},
			},
		},
	}

	// Built-in TrDisplay trait for string interpolation and display
	trDisplay := &TraitType{
		Name: "TrDisplay",
		Methods: map[string]*FnType{
			"fmt": {
				Params: []FnParamInfo{
					{Name: "self", Type: TypeAny},
					{Name: "buf", Type: TypeAny},
				},
				Return: TypeVoid,
			},
		},
	}
	te.traits["TrDisplay"] = trDisplay
	_ = te.stack.Define(&Symbol{Name: "TrDisplay", Kind: SymbolType, Type: trDisplay})
}

func (te *TypeEvaluator) Diagnostics() []Diagnostic {
	return te.diags
}

func (te *TypeEvaluator) addDiag(sev Severity, span loneast.Span, rule, msg string) {
	te.diags = append(te.diags, Diagnostic{
		Severity: sev,
		Span:     span,
		Rule:     rule,
		Message:  msg,
	})
}

// EvaluateProgram executes a full semantic evaluation pass on a slice of AST declarations.
func (te *TypeEvaluator) EvaluateProgram(decls []any) *TProgram {
	// Pass 1: Hoist structs, traits, and aliases
	for _, decl := range decls {
		if tdec, ok := decl.(loneast.TYPE_DEC); ok {
			te.hoistTypeDec(&tdec)
		} else if ptdec, ok := decl.(*loneast.TYPE_DEC); ok {
			te.hoistTypeDec(ptdec)
		}
	}

	// Pass 2: Hoist function signatures, impl blocks, global variables, and host capability directives
	for _, decl := range decls {
		switch d := decl.(type) {
		case loneast.FN_DEC:
			te.hoistFn(&d)
		case *loneast.FN_DEC:
			te.hoistFn(d)
		case loneast.BIND_DEC:
			te.hoistVar(&d)
		case *loneast.BIND_DEC:
			te.hoistVar(d)
		case loneast.WasmDirective:
			te.hoistWasmDirective(&d)
		case *loneast.WasmDirective:
			te.hoistWasmDirective(d)
		case loneast.ImportDecl:
			te.hoistImport(&d)
		case *loneast.ImportDecl:
			te.hoistImport(d)
		case loneast.IMPL_DEC:
			te.hoistImpl(&d)
		case *loneast.IMPL_DEC:
			te.hoistImpl(d)
		}
	}

	// Pass 3: Analyze bodies
	var tDecls []TNode
	for _, decl := range decls {
		td := te.analyzeTopLevel(decl)
		if td != nil {
			tDecls = append(tDecls, td)
		}
	}
	// Append monomorphized function declarations
	for _, extra := range te.extraDecls {
		tDecls = append(tDecls, extra)
	}

	return &TProgram{Decls: tDecls}
}

// =====================
// HOISTING
// =====================

func (te *TypeEvaluator) hoistTypeDec(t *loneast.TYPE_DEC) {
	if len(t.TypeParams) > 0 {
		te.genericTypes[t.Name] = &GenericTypeTemplate{
			Name:        t.Name,
			TypeParams:  t.TypeParams,
			Constraints: t.Constraints,
			Decl:        t,
		}
		return
	}

	// Validate Trait Naming Convention: Tr + PascalCase
	if t.Kind == loneast.TYPE_DEC_TRAIT {
		if !isTrNaming(t.Name) {
			te.addDiag(SeverityError, t.Span_, "convention/trait-naming",
				fmt.Sprintf("trait %q must begin with 'Tr' followed by an uppercase letter (e.g. 'Tr%s')", t.Name, t.Name))
		}
	} else {
		// Non-trait types must NOT start with Tr + uppercase
		if isTrNaming(t.Name) {
			te.addDiag(SeverityError, t.Span_, "convention/non-trait-naming",
				fmt.Sprintf("non-trait type %q must not begin with 'Tr' prefix reserved for traits", t.Name))
		}
	}

	// Validate Interface Naming Convention: Ifc + PascalCase
	if t.Kind == loneast.TYPE_DEC_INTERFACE {
		if !isIfcNaming(t.Name) {
			te.addDiag(SeverityError, t.Span_, "naming/interface",
				fmt.Sprintf("interface %q must begin with 'Ifc' followed by an uppercase letter (e.g. 'Ifc%s')", t.Name, t.Name))
		}
	} else {
		// Non-interface types must NOT start with Ifc + uppercase
		if isIfcNaming(t.Name) {
			te.addDiag(SeverityError, t.Span_, "naming/interface",
				fmt.Sprintf("non-interface type %q must not begin with 'Ifc' prefix reserved for interfaces", t.Name))
		}
	}

	if t.Kind == loneast.TYPE_DEC_INTERFACE {
		ifc := &InterfaceType{
			Name:     t.Name,
			FieldMap: make(map[string]StructFieldInfo),
		}

		// Expand embedded interfaces
		for _, embName := range t.Embedded {
			embIfc, ok := te.interfaces[embName]
			if !ok {
				te.addDiag(SeverityError, t.Span_, "type/undefined-interface",
					fmt.Sprintf("undefined interface %q embedded in %q", embName, t.Name))
				continue
			}
			for _, ef := range embIfc.Fields {
				if existing, exists := ifc.FieldMap[ef.Name]; exists {
					if !existing.Type.Equals(ef.Type) || existing.IsReadOnly != ef.IsReadOnly {
						te.addDiag(SeverityError, t.Span_, "type/interface-embedding-collision",
							fmt.Sprintf("embedded interfaces declare conflicting definitions for field %q", ef.Name))
					}
				} else {
					ifc.Fields = append(ifc.Fields, ef)
					ifc.FieldMap[ef.Name] = ef
				}
			}
		}

		for _, f := range t.Fields {
			if strings.HasPrefix(f.Name, "_") {
				te.addDiag(SeverityError, f.Span_, "type/interface-private-field",
					fmt.Sprintf("interface %q cannot declare private field %q", t.Name, f.Name))
				continue
			}
			fType := te.resolveType(f.Type)
			fInfo := StructFieldInfo{
				Name:       f.Name,
				Type:       fType,
				IsReadOnly: f.IsReadOnly,
			}
			if existing, exists := ifc.FieldMap[f.Name]; exists {
				if !existing.Type.Equals(fType) || existing.IsReadOnly != f.IsReadOnly {
					te.addDiag(SeverityError, f.Span_, "type/interface-embedding-collision",
						fmt.Sprintf("field %q conflicts with embedded interface definition", f.Name))
				}
			} else {
				ifc.Fields = append(ifc.Fields, fInfo)
				ifc.FieldMap[f.Name] = fInfo
			}
		}

		te.interfaces[t.Name] = ifc
		_ = te.stack.Define(&Symbol{Name: t.Name, Kind: SymbolType, Type: ifc, Node: t})
		return
	}

	if t.Kind == loneast.TYPE_DEC_STRUCT {
		st := &StructType{
			Name:     t.Name,
			FieldMap: make(map[string]StructFieldInfo),
			Methods:  make(map[string]*FnType),
		}

		offset := 0
		maxAlign := 1
		for _, f := range t.Fields {
			fType := te.resolveType(f.Type)
			sz := TypeSize(fType)
			al := TypeAlign(fType)
			if al > maxAlign {
				maxAlign = al
			}
			// Align offset
			if al > 0 && offset%al != 0 {
				offset += al - (offset % al)
			}
			info := StructFieldInfo{
				Name:       f.Name,
				Type:       fType,
				Offset:     offset,
				IsReadOnly: f.IsReadOnly,
				IsPrivate:  strings.HasPrefix(f.Name, "_"),
			}
			st.Fields = append(st.Fields, info)
			st.FieldMap[f.Name] = info
			offset += sz
		}

		if maxAlign > 0 && offset%maxAlign != 0 {
			offset += maxAlign - (offset % maxAlign)
		}
		st.TotalSize = offset
		st.TotalAlign = maxAlign

		for _, m := range t.Methods {
			var mParams []FnParamInfo
			for _, p := range m.Params {
				mParams = append(mParams, FnParamInfo{Name: p.Name, Type: te.resolveType(p.Type)})
			}
			mRet := Type(TypeVoid)
			if m.ReturnType != nil {
				mRet = te.resolveType(*m.ReturnType)
			}
			st.Methods[m.Name] = &FnType{Params: mParams, Return: mRet}
		}

		hasRoPrivate := false
		for _, f := range st.Fields {
			if f.IsReadOnly && f.IsPrivate {
				hasRoPrivate = true
				break
			}
		}
		if hasRoPrivate {
			if _, hasInit := st.Methods["__init__"]; !hasInit {
				te.addDiag(SeverityError, t.Span_, "type/missing-init-for-ro-private-field",
					fmt.Sprintf("struct %q declares read-only private field(s) but provides no '__init__' constructor", t.Name))
			}
		}

		te.structs[t.Name] = st
		_ = te.stack.Define(&Symbol{Name: t.Name, Kind: SymbolType, Type: st, Node: t})
	}

	if t.Kind == loneast.TYPE_DEC_TRAIT {
		tr := &TraitType{
			Name:    t.Name,
			Methods: make(map[string]*FnType),
		}
		for _, sig := range t.TraitSigs {
			var params []FnParamInfo
			for _, p := range sig.Params {
				pType := te.resolveType(p.Type)
				params = append(params, FnParamInfo{Name: p.Name, Type: pType})
			}
			retType := Type(TypeVoid)
			if sig.ReturnType != nil {
				retType = te.resolveType(*sig.ReturnType)
			}
			tr.Methods[sig.Name] = &FnType{Params: params, Return: retType}
		}
		te.traits[t.Name] = tr
		_ = te.stack.Define(&Symbol{Name: t.Name, Kind: SymbolType, Type: tr, Node: t})
	}

	if t.Kind == loneast.TYPE_DEC_ENUM {
		et := &EnumType{
			Name:       t.Name,
			VariantMap: make(map[string]*EnumVariantInfo),
		}

		maxPayloadSize := 0
		maxAlign := 4 // tag is i32, so align is at least 4

		for tag, v := range t.Variants {
			var fields []StructFieldInfo
			fieldMap := make(map[string]StructFieldInfo)
			vOffset := 4 // Tag is at offset 0 (4 bytes), payload starts at 4
			vMaxAlign := 1

			for _, f := range v.Fields {
				fType := te.resolveType(f.Type)
				sz := TypeSize(fType)
				al := TypeAlign(fType)
				if al > vMaxAlign {
					vMaxAlign = al
				}
				if al > maxAlign {
					maxAlign = al
				}
				if al > 0 && vOffset%al != 0 {
					vOffset += al - (vOffset % al)
				}
				info := StructFieldInfo{Name: f.Name, Type: fType, Offset: vOffset}
				fields = append(fields, info)
				fieldMap[f.Name] = info
				vOffset += sz
			}

			payloadSize := vOffset - 4
			if payloadSize > maxPayloadSize {
				maxPayloadSize = payloadSize
			}

			vInfo := EnumVariantInfo{
				Name:       v.Name,
				Tag:        tag,
				Fields:     fields,
				FieldMap:   fieldMap,
				TotalSize:  vOffset,
				TotalAlign: vMaxAlign,
			}
			et.Variants = append(et.Variants, vInfo)
			et.VariantMap[v.Name] = &et.Variants[len(et.Variants)-1]
		}

		totalSize := 4 + maxPayloadSize
		if maxAlign > 0 && totalSize%maxAlign != 0 {
			totalSize += maxAlign - (totalSize % maxAlign)
		}
		et.TotalSize = totalSize
		et.TotalAlign = maxAlign

		te.enums[t.Name] = et
		_ = te.stack.Define(&Symbol{Name: t.Name, Kind: SymbolType, Type: et, Node: t})
	}
}

func (te *TypeEvaluator) hoistFn(f *loneast.FN_DEC) {
	if len(f.TypeParams) > 0 {
		te.genericFns[f.Name] = &GenericFnTemplate{
			Name:        f.Name,
			TypeParams:  f.TypeParams,
			Constraints: f.Constraints,
			Decl:        f,
		}
		return
	}

	var params []FnParamInfo
	for _, p := range f.Params {
		pType := te.resolveType(p.Type)
		params = append(params, FnParamInfo{Name: p.Name, Type: pType})
	}
	retType := Type(TypeVoid)
	if f.ReturnType != nil {
		retType = te.resolveType(*f.ReturnType)
	}
	fnType := &FnType{Params: params, Return: retType}
	_ = te.stack.Define(&Symbol{Name: f.Name, Kind: SymbolFn, Type: fnType, Node: f})
}

func (te *TypeEvaluator) hoistVar(b *loneast.BIND_DEC) {
	varType := te.resolveType(b.Type)
	if len(b.Names) > 0 {
		for _, name := range b.Names {
			_ = te.stack.Define(&Symbol{
				Name:      name,
				Kind:      SymbolVar,
				Type:      varType,
				IsMutable: b.IsMutable,
				Node:      b,
			})
		}
		return
	}
	_ = te.stack.Define(&Symbol{
		Name:      b.Name,
		Kind:      SymbolVar,
		Type:      varType,
		IsMutable: b.IsMutable,
		Node:      b,
	})
}

func (te *TypeEvaluator) hoistWasmDirective(wd *loneast.WasmDirective) {
	if wd == nil {
		return
	}
	for _, imp := range wd.Imports {
		if imp.Source == ":wasi:snapshot_preview1" {
			for _, sym := range imp.Symbols {
				symName := strings.TrimPrefix(sym, ".")
				switch symName {
				case "fd_write":
					_ = te.stack.Define(&Symbol{
						Name: symName,
						Kind: SymbolFn,
						Type: &FnType{
							Params: []FnParamInfo{
								{Name: "fd", Type: TypeI32},
								{Name: "iovs", Type: &PtrType{Elem: TypeAny}},
								{Name: "iovs_len", Type: TypeI32},
								{Name: "nwritten", Type: &PtrType{Elem: TypeI32}},
							},
							Return: TypeI32,
						},
					})
				case "fd_read":
					_ = te.stack.Define(&Symbol{
						Name: symName,
						Kind: SymbolFn,
						Type: &FnType{
							Params: []FnParamInfo{
								{Name: "fd", Type: TypeI32},
								{Name: "iovs", Type: &PtrType{Elem: TypeAny}},
								{Name: "iovs_len", Type: TypeI32},
								{Name: "nread", Type: &PtrType{Elem: TypeI32}},
							},
							Return: TypeI32,
						},
					})
				case "fd_close":
					_ = te.stack.Define(&Symbol{
						Name: symName,
						Kind: SymbolFn,
						Type: &FnType{
							Params: []FnParamInfo{
								{Name: "fd", Type: TypeI32},
							},
							Return: TypeI32,
						},
					})
				case "proc_exit":
					_ = te.stack.Define(&Symbol{
						Name: symName,
						Kind: SymbolFn,
						Type: &FnType{
							Params: []FnParamInfo{
								{Name: "code", Type: TypeI32},
							},
							Return: TypeVoid,
						},
					})
				case "random_get":
					_ = te.stack.Define(&Symbol{
						Name: symName,
						Kind: SymbolFn,
						Type: &FnType{
							Params: []FnParamInfo{
								{Name: "buf", Type: &PtrType{Elem: TypeAny}},
								{Name: "buf_len", Type: TypeI32},
							},
							Return: TypeI32,
						},
					})
				}
			}
		}
	}
}

func (te *TypeEvaluator) hoistImport(imp *loneast.ImportDecl) {
	if imp == nil {
		return
	}
	if imp.Source == ":std:sys" {
		for _, sym := range imp.Symbols {
			symName := strings.TrimPrefix(sym, ".")
			switch symName {
			case "STDIN_FD", "STDOUT_FD", "STDERR_FD":
				_ = te.stack.Define(&Symbol{Name: symName, Kind: SymbolVar, Type: TypeI32, IsMutable: false})
			case "sys_write":
				_ = te.stack.Define(&Symbol{
					Name: symName,
					Kind: SymbolFn,
					Type: &FnType{
						Params: []FnParamInfo{
							{Name: "fd", Type: TypeI32},
							{Name: "buf", Type: &PtrType{Elem: TypeU8}},
							{Name: "len", Type: TypeI32},
						},
						Return: TypeI32,
					},
				})
			case "sys_read":
				_ = te.stack.Define(&Symbol{
					Name: symName,
					Kind: SymbolFn,
					Type: &FnType{
						Params: []FnParamInfo{
							{Name: "fd", Type: TypeI32},
							{Name: "buf", Type: &PtrType{Elem: TypeU8}},
							{Name: "max_len", Type: TypeI32},
						},
						Return: TypeI32,
					},
				})
			case "sys_random":
				_ = te.stack.Define(&Symbol{
					Name: symName,
					Kind: SymbolFn,
					Type: &FnType{
						Params: []FnParamInfo{
							{Name: "buf", Type: &PtrType{Elem: TypeAny}},
							{Name: "len", Type: TypeI32},
						},
						Return: TypeI32,
					},
				})
			case "sys_close":
				_ = te.stack.Define(&Symbol{
					Name: symName,
					Kind: SymbolFn,
					Type: &FnType{
						Params: []FnParamInfo{{Name: "fd", Type: TypeI32}},
						Return: TypeI32,
					},
				})
			case "sys_exit":
				_ = te.stack.Define(&Symbol{
					Name: symName,
					Kind: SymbolFn,
					Type: &FnType{
						Params: []FnParamInfo{{Name: "code", Type: TypeI32}},
						Return: TypeVoid,
					},
				})
			}
		}
	}
}

func (te *TypeEvaluator) hoistImpl(imp *loneast.IMPL_DEC) {
	if imp == nil {
		return
	}
	targetType := te.resolveType(imp.TargetType)
	if te.traitImpls[imp.TraitName] == nil {
		te.traitImpls[imp.TraitName] = make(map[string]*loneast.IMPL_DEC)
	}
	te.traitImpls[imp.TraitName][targetType.String()] = imp
	if st, ok := targetType.(*StructType); ok {
		for _, m := range imp.Methods {
			var mParams []FnParamInfo
			for _, p := range m.Params {
				pType := te.resolveType(p.Type)
				if p.Name == "self" && (pType == TypeInfer || pType == TypeUnknown) {
					pType = targetType
				}
				mParams = append(mParams, FnParamInfo{Name: p.Name, Type: pType})
			}
			mRet := Type(TypeVoid)
			if m.ReturnType != nil {
				mRet = te.resolveType(*m.ReturnType)
			}
			st.Methods[m.Name] = &FnType{Params: mParams, Return: mRet}
		}
	}
}

func (te *TypeEvaluator) analyzeTypeDecNode(d *loneast.TYPE_DEC) TNode {
	if st, ok := te.structs[d.Name]; ok {
		oldSelf := te.currentSelfType
		te.currentSelfType = st
		var tMethods []*TFnDec
		for _, m := range d.Methods {
			mCopy := m
			if !strings.Contains(mCopy.Name, ".") {
				mCopy.Name = d.Name + "." + mCopy.Name
			}
			tMethod := te.analyzeFnDec(&mCopy)
			tMethods = append(tMethods, tMethod)
		}
		te.currentSelfType = oldSelf
		return &TStructDec{Name: d.Name, Type: st, Methods: tMethods, Span_: d.Span_}
	}
	if tr, ok := te.traits[d.Name]; ok {
		return &TTraitDec{Name: d.Name, Type: tr, Span_: d.Span_}
	}
	if et, ok := te.enums[d.Name]; ok {
		return &TEnumDec{Name: d.Name, Type: et, Span_: d.Span_}
	}
	return nil
}

// =====================
// ANALYSIS
// =====================

func (te *TypeEvaluator) analyzeTopLevel(decl any) TNode {
	switch d := decl.(type) {
	case loneast.BIND_DEC:
		return te.analyzeVarDec(&d)
	case *loneast.BIND_DEC:
		return te.analyzeVarDec(d)

	case loneast.FN_DEC:
		if len(d.TypeParams) > 0 {
			return nil
		}
		return te.analyzeFnDec(&d)
	case *loneast.FN_DEC:
		if len(d.TypeParams) > 0 {
			return nil
		}
		return te.analyzeFnDec(d)

	case loneast.TYPE_DEC:
		return te.analyzeTypeDecNode(&d)
	case *loneast.TYPE_DEC:
		return te.analyzeTypeDecNode(d)

	case loneast.IMPL_DEC:
		return te.analyzeImplDec(&d)
	case *loneast.IMPL_DEC:
		return te.analyzeImplDec(d)

	case loneast.IfStmt:
		return te.analyzeIfStmt(&d, TypeVoid)
	case *loneast.IfStmt:
		return te.analyzeIfStmt(d, TypeVoid)

	case loneast.PackedIfStmt:
		return te.analyzePackedIfStmt(&d, TypeVoid)
	case *loneast.PackedIfStmt:
		return te.analyzePackedIfStmt(d, TypeVoid)

	case loneast.WhileStmt:
		return te.analyzeWhileStmt(&d, TypeVoid)
	case *loneast.WhileStmt:
		return te.analyzeWhileStmt(d, TypeVoid)

	case loneast.LoopStmt:
		return te.analyzeLoopStmt(&d, TypeVoid)
	case *loneast.LoopStmt:
		return te.analyzeLoopStmt(d, TypeVoid)

	case loneast.ForStmt:
		return te.analyzeForStmt(&d, TypeVoid)
	case *loneast.ForStmt:
		return te.analyzeForStmt(d, TypeVoid)

	case loneast.AssignStmt:
		return te.analyzeAssignStmt(&d)
	case *loneast.AssignStmt:
		return te.analyzeAssignStmt(d)

	case loneast.DropStmt:
		expr := te.EvalExpr(d.Expr)
		return &TDropStmt{Expr: expr, Span_: d.Span_}
	case *loneast.DropStmt:
		expr := te.EvalExpr(d.Expr)
		return &TDropStmt{Expr: expr, Span_: d.Span_}

	case loneast.LabelDec:
		return te.analyzeLabelDec(&d, TypeVoid)
	case *loneast.LabelDec:
		return te.analyzeLabelDec(d, TypeVoid)

	default:
		return nil
	}
}

func (te *TypeEvaluator) analyzeVarDec(b *loneast.BIND_DEC) *TVarDec {
	if b.Pattern != nil {
		tVal := te.EvalExpr(b.Value)
		var vInfo *EnumVariantInfo
		var bindings []*TVarDec

		if vp, ok := b.Pattern.(loneast.VariantPattern); ok {
			et, isEnum := tVal.ExprType().(*EnumType)
			if !isEnum {
				te.addDiag(SeverityError, b.Pattern.NodeSpan(), "type/invalid-pattern",
					fmt.Sprintf("cannot destructure non-enum type %q", tVal.ExprType().String()))
			} else {
				if vp.TypeName != "" && vp.TypeName != et.Name && !strings.HasPrefix(et.Name, vp.TypeName+"<") {
					te.addDiag(SeverityError, b.Pattern.NodeSpan(), "type/enum-type-mismatch",
						fmt.Sprintf("variant %q belongs to enum %q, but value is %q", vp.Variant, vp.TypeName, et.Name))
				}
				v, found := et.Variant(vp.Variant)
				if !found {
					te.addDiag(SeverityError, b.Pattern.NodeSpan(), "type/unknown-variant",
						fmt.Sprintf("enum %q has no variant %q", et.Name, vp.Variant))
				} else {
					vInfo = v
					if len(vp.Bindings) != len(v.Fields) {
						te.addDiag(SeverityError, b.Pattern.NodeSpan(), "type/pattern-arity-mismatch",
							fmt.Sprintf("variant %q has %d fields, but pattern binds %d variables",
								vp.Variant, len(v.Fields), len(vp.Bindings)))
					} else {
						for i, bName := range vp.Bindings {
							fInfo := v.Fields[i]
							if bName != "_" {
								sym := &Symbol{
									Name:      bName,
									Kind:      SymbolVar,
									Type:      fInfo.Type,
									IsMutable: b.IsMutable,
								}
								_ = te.stack.Define(sym)
								bindings = append(bindings, &TVarDec{
									Name:      bName,
									IsMutable: b.IsMutable,
									Type:      fInfo.Type,
									Value:     nil,
									Span_:     b.Pattern.NodeSpan(),
								})
							}
						}
					}
				}
			}
		} else if tp, ok := b.Pattern.(loneast.TuplePattern); ok {
			tupType, isTuple := tVal.ExprType().(*TupleType)
			if !isTuple {
				te.addDiag(SeverityError, b.Pattern.NodeSpan(), "type/invalid-pattern",
					fmt.Sprintf("cannot destructure non-tuple type %q", tVal.ExprType().String()))
			} else {
				if len(tp.Elements) != len(tupType.Elements) {
					te.addDiag(SeverityError, b.Pattern.NodeSpan(), "type/tuple-arity-mismatch",
						fmt.Sprintf("tuple has %d elements, but pattern binds %d variables",
							len(tupType.Elements), len(tp.Elements)))
				} else {
					for i, elemPat := range tp.Elements {
						elemType := tupType.Elements[i]
						bName := ""
						if vp, ok := elemPat.(loneast.VariantPattern); ok {
							bName = vp.Variant
						} else if _, ok := elemPat.(loneast.WildcardPattern); ok {
							bName = "_"
						}
						if bName != "" && bName != "_" {
							sym := &Symbol{
								Name:      bName,
								Kind:      SymbolVar,
								Type:      elemType,
								IsMutable: b.IsMutable,
							}
							_ = te.stack.Define(sym)
							bindings = append(bindings, &TVarDec{
								Name:      bName,
								IsMutable: b.IsMutable,
								Type:      elemType,
								Value:     nil,
								Span_:     elemPat.NodeSpan(),
							})
						}
					}
				}
			}
		}

		return &TVarDec{
			Name:      b.Name,
			Variant:   vInfo,
			TargetVal: tVal,
			Bindings:  bindings,
			IsMutable: b.IsMutable,
			Type:      tVal.ExprType(),
			Value:     tVal,
			Span_:     b.Span_,
		}
	}

	declaredType := te.resolveType(b.Type)
	tVal := te.EvalExpr(b.Value)

	resolvedType := declaredType
	if declaredType.Equals(TypeInfer) || declaredType.Equals(TypeUnknown) {
		if tVal != nil && tVal.ExprType() != TypeUnknown {
			resolvedType = tVal.ExprType()
		} else {
			resolvedType = TypeUnknown
		}
	} else if tVal != nil && tVal.ExprType() != TypeUnknown && !tVal.ExprType().Equals(TypeInfer) {
		// Type mismatch check
		if !tVal.ExprType().AssignableTo(declaredType) {
			te.addDiag(SeverityError, b.Span_, "type/mismatch",
				fmt.Sprintf("cannot assign value of type %q to binding %q of type %q",
					tVal.ExprType().String(), b.Name, declaredType.String()))
		}
	}

	if len(b.Names) > 0 {
		var bindings []*TVarDec
		for _, name := range b.Names {
			if existing := te.stack.LookupLocal(name); existing != nil {
				existing.Type = resolvedType
				existing.IsMutable = b.IsMutable
			} else {
				sym := &Symbol{
					Name:      name,
					Kind:      SymbolVar,
					Type:      resolvedType,
					IsMutable: b.IsMutable,
					Node:      b,
				}
				_ = te.stack.Define(sym)
			}
			tVar := &TVarDec{
				Name:         name,
				IsMutable:    b.IsMutable,
				AddressTaken: false,
				Type:         resolvedType,
				Value:        tVal,
				Span_:        b.Span_,
			}
			if sym := te.stack.LookupLocal(name); sym != nil {
				sym.Node = tVar
				tVar.AddressTaken = sym.AddressTaken
			}
			bindings = append(bindings, tVar)
		}
		return &TVarDec{
			Name:      b.Names[0],
			Bindings:  bindings,
			IsMutable: b.IsMutable,
			Type:      resolvedType,
			Value:     tVal,
			Span_:     b.Span_,
		}
	}

	if existing := te.stack.LookupLocal(b.Name); existing != nil {
		existing.Type = resolvedType
		existing.IsMutable = b.IsMutable
	} else {
		sym := &Symbol{
			Name:      b.Name,
			Kind:      SymbolVar,
			Type:      resolvedType,
			IsMutable: b.IsMutable,
			Node:      b,
		}
		_ = te.stack.Define(sym)
	}

	tVar := &TVarDec{
		Name:         b.Name,
		IsMutable:    b.IsMutable,
		AddressTaken: false,
		Type:         resolvedType,
		Value:        tVal,
		Span_:        b.Span_,
	}
	if sym := te.stack.LookupLocal(b.Name); sym != nil {
		sym.Node = tVar
		tVar.AddressTaken = sym.AddressTaken
	}
	return tVar
}

func (te *TypeEvaluator) analyzeFnDec(f *loneast.FN_DEC) *TFnDec {
	te.stack.Push(ScopeFunction)
	defer te.stack.Pop()

	for name, typ := range te.currentTypeParams {
		_ = te.stack.Define(&Symbol{Name: name, Kind: SymbolType, Type: typ})
	}

	oldFnName := te.currentFnName
	te.currentFnName = f.Name
	defer func() { te.currentFnName = oldFnName }()

	var tParams []TFnParam
	for _, p := range f.Params {
		pType := te.resolveType(p.Type)
		if p.Name == "self" && (pType == TypeUnknown || pType == TypeInfer || p.Type.Value == nil) {
			if te.currentSelfType != nil {
				pType = te.currentSelfType
			}
		}
		_ = te.stack.Define(&Symbol{Name: p.Name, Kind: SymbolVar, Type: pType, IsMutable: false})
		tParams = append(tParams, TFnParam{Name: p.Name, Type: pType, Span_: p.Span_})
	}

	retType := Type(TypeVoid)
	if f.ReturnType != nil {
		retType = te.resolveType(*f.ReturnType)
	}

	oldFnRet := te.currentFnReturn
	te.currentFnReturn = retType
	defer func() { te.currentFnReturn = oldFnRet }()

	body := te.analyzeBlock(&f.Body, retType)

	// Resolve 'infer' in return type if present (e.g. Result<V, infer>)
	if strings.Contains(retType.String(), "infer") {
		errs := te.inferredErrors[f.Name]
		var inferredErr Type = te.enums["Error"]
		if len(errs) == 1 {
			inferredErr = errs[0]
		} else if len(errs) > 1 {
			same := true
			for i := 1; i < len(errs); i++ {
				if !errs[i].Equals(errs[0]) {
					same = false
					break
				}
			}
			if same {
				inferredErr = errs[0]
			} else {
				inferredErr = te.enums["Error"]
			}
		}
		if et, ok := retType.(*EnumType); ok && strings.HasPrefix(et.Name, "Result<") {
			okV, _ := et.Variant("Ok")
			var okType Type = TypeVoid
			if len(okV.Fields) > 0 {
				okType = okV.Fields[0].Type
			}
			if tmpl, ok := te.genericTypes["Result"]; ok {
				concreteRes := te.instantiateGenericType(tmpl, []loneast.TYPE_REF{
					{Kind: loneast.TYPE_BASE, Value: &loneast.TypeBase{Name: okType.String()}},
					{Kind: loneast.TYPE_BASE, Value: &loneast.TypeBase{Name: inferredErr.String()}},
				})
				retType = concreteRes
			}
		}
	}

	return &TFnDec{
		Name:       f.Name,
		Params:     tParams,
		ReturnType: retType,
		Body:       body,
		Span_:      f.Span_,
	}
}

func (te *TypeEvaluator) analyzeImplDec(i *loneast.IMPL_DEC) *TImplDec {
	targetType := te.resolveType(i.TargetType)
	oldSelf := te.currentSelfType
	te.currentSelfType = targetType
	defer func() { te.currentSelfType = oldSelf }()

	var tMethods []*TFnDec

	for _, m := range i.Methods {
		tMethod := te.analyzeFnDec(&m)
		tMethods = append(tMethods, tMethod)
	}

	return &TImplDec{
		TraitName:  i.TraitName,
		TargetType: targetType,
		Methods:    tMethods,
		Span_:      i.Span_,
	}
}

func (te *TypeEvaluator) analyzeBlock(b *loneast.Block, expectedRet Type) TBlock {
	te.stack.Push(ScopeBlock)
	defer te.stack.Pop()

	var stmts []TStmt
	blockType := Type(TypeVoid)
	for _, s := range b.Stmts {
		tStmt := te.analyzeStmt(s, expectedRet)
		if tStmt != nil {
			stmts = append(stmts, tStmt)
			if brk, ok := tStmt.(*TBreakStmt); ok && brk.Value != nil {
				blockType = brk.Value.ExprType()
			} else if expr, ok := tStmt.(TExpr); ok {
				blockType = expr.ExprType()
			}
		}
	}

	return TBlock{
		Stmts: stmts,
		Typ:   blockType,
		Span_: b.Span_,
	}
}

func (te *TypeEvaluator) analyzeStmt(raw any, expectedRet Type) TStmt {
	switch s := raw.(type) {
	case loneast.BIND_DEC:
		return te.analyzeVarDec(&s)
	case *loneast.BIND_DEC:
		return te.analyzeVarDec(s)

	case loneast.AssignStmt:
		return te.analyzeAssignStmt(&s)
	case *loneast.AssignStmt:
		return te.analyzeAssignStmt(s)

	case loneast.ReturnStmt:
		var tVal TExpr
		if s.Value != nil {
			tVal = te.EvalExpr(s.Value)
			if expectedRet != TypeVoid && !tVal.ExprType().AssignableTo(expectedRet) {
				te.addDiag(SeverityError, s.Span_, "type/return-mismatch",
					fmt.Sprintf("cannot return value of type %q from function expecting %q",
						tVal.ExprType().String(), expectedRet.String()))
			}
		}
		return &TReturnStmt{Value: tVal, Span_: s.Span_}

	case loneast.BreakStmt:
		var tVal TExpr
		if s.Value != nil {
			tVal = te.EvalExpr(s.Value)
		}
		return &TBreakStmt{Label: s.Label, Value: tVal, Span_: s.Span_}

	case loneast.DropStmt:
		tExpr := te.EvalExpr(s.Expr)
		return &TDropStmt{Expr: tExpr, Span_: s.Span_}

	case loneast.IfStmt:
		return te.analyzeIfStmt(&s, expectedRet)
	case *loneast.IfStmt:
		return te.analyzeIfStmt(s, expectedRet)

	case loneast.PackedIfStmt:
		return te.analyzePackedIfStmt(&s, expectedRet)
	case *loneast.PackedIfStmt:
		return te.analyzePackedIfStmt(s, expectedRet)

	case loneast.WhileStmt:
		return te.analyzeWhileStmt(&s, expectedRet)
	case *loneast.WhileStmt:
		return te.analyzeWhileStmt(s, expectedRet)

	case loneast.LoopStmt:
		return te.analyzeLoopStmt(&s, expectedRet)
	case *loneast.LoopStmt:
		return te.analyzeLoopStmt(s, expectedRet)

	case loneast.ForStmt:
		return te.analyzeForStmt(&s, expectedRet)
	case *loneast.ForStmt:
		return te.analyzeForStmt(s, expectedRet)

	case loneast.LabelDec:
		return te.analyzeLabelDec(&s, expectedRet)
	case *loneast.LabelDec:
		return te.analyzeLabelDec(s, expectedRet)

	case loneast.Expr:
		return te.EvalExpr(s)

	default:
		return nil
	}
}

// analyzeAssignStmt enforces IL1 mutability invariant: cannot reassign immutable 'let' bindings.
func (te *TypeEvaluator) analyzeAssignStmt(a *loneast.AssignStmt) *TAssignStmt {
	tTarget := te.EvalExpr(a.Target)
	tVal := te.EvalExpr(a.Value)

	// Check if target is a variable reference and whether it is immutable
	if vref, ok := tTarget.(*TVarRefExpr); ok {
		sym := te.stack.Lookup(vref.Name)
		if sym != nil && !sym.IsMutable {
			te.addDiag(SeverityError, a.Span_, "type/immutable-reassignment",
				fmt.Sprintf("cannot reassign immutable binding %q declared with 'let'; use 'var' if mutation is intended", sym.Name))
		}
	}

	// Check if target is a member access (struct field) and whether it is read-only or on an immutable binding
	if mem, ok := tTarget.(*TMemberAccessExpr); ok {
		if st, ok := mem.Base.ExprType().(*StructType); ok {
			if fInfo, exists := st.Field(mem.Field); exists {
				if fInfo.IsReadOnly {
					te.addDiag(SeverityError, a.Span_, "type/immutable-field-write",
						fmt.Sprintf("cannot assign to read-only field %q on struct %q; 'ro' fields are immutable after construction", mem.Field, st.Name))
				}
			}
		}
		if vref, ok := mem.Base.(*TVarRefExpr); ok {
			sym := te.stack.Lookup(vref.Name)
			if sym != nil && !sym.IsMutable {
				te.addDiag(SeverityError, a.Span_, "type/immutable-reassignment",
					fmt.Sprintf("cannot mutate field %q on immutable binding %q declared with 'let'", mem.Field, sym.Name))
			}
		}
	}

	// Check if target is a dereference: *r = val or *p = val
	if un, ok := tTarget.(*TUnaryExpr); ok && un.Op == loneast.UNARY_OP_DEREF {
		if refT, ok := un.Expr.ExprType().(*RefType); ok && refT.ReadOnly {
			te.addDiag(SeverityError, a.Span_, "type/immutable-reference-write",
				fmt.Sprintf("cannot write through read-only reference %q", refT.String()))
		}
		if ptrT, ok := un.Expr.ExprType().(*PtrType); ok && ptrT.ReadOnly {
			te.addDiag(SeverityError, a.Span_, "type/immutable-pointer-write",
				fmt.Sprintf("cannot write through read-only pointer %q", ptrT.String()))
		}
	}

	// Check if target is an index into a buffer: p[i] = val
	if idx, ok := tTarget.(*TIndexExpr); ok {
		if ptrT, ok := idx.Base.ExprType().(*PtrType); ok && ptrT.ReadOnly {
			te.addDiag(SeverityError, a.Span_, "type/immutable-pointer-write",
				fmt.Sprintf("cannot write to read-only buffer through %q", ptrT.String()))
		}
	}

	if tTarget.ExprType() != TypeUnknown && tVal.ExprType() != TypeUnknown {
		if !tVal.ExprType().AssignableTo(tTarget.ExprType()) {
			te.addDiag(SeverityError, a.Span_, "type/assignment-mismatch",
				fmt.Sprintf("cannot assign value of type %q to target of type %q",
					tVal.ExprType().String(), tTarget.ExprType().String()))
		}
	}

	return &TAssignStmt{
		Target: tTarget,
		Value:  tVal,
		Span_:  a.Span_,
	}
}

// analyzeIfStmt handles standard if with optional scoped header bindings: if: [let x = 0] x != 9 { ... }
func (te *TypeEvaluator) analyzeIfStmt(i *loneast.IfStmt, expectedRet Type) *TIfStmt {
	hasHeader := len(i.Header) > 0
	if hasHeader {
		te.stack.Push(ScopeHeader)
	}

	var tHeaders []*TVarDec
	for _, h := range i.Header {
		tVar := te.analyzeVarDec(&h)
		tHeaders = append(tHeaders, tVar)
	}

	var tCond TExpr
	if i.Cond != nil {
		tCond = te.EvalExpr(i.Cond)
		if !tCond.ExprType().Equals(TypeBool) && tCond.ExprType() != TypeUnknown {
			te.addDiag(SeverityError, i.Cond.NodeSpan(), "type/condition-not-bool",
				fmt.Sprintf("if condition must be boolean, got %q", tCond.ExprType().String()))
		}
	} else {
		tCond = &TLiteralExpr{Kind: loneast.LiteralBool, Value: true, Typ: TypeBool, Span_: i.Span_}
	}

	tThen := te.analyzeBlock(&i.Then, expectedRet)

	var tElse TStmt
	if i.Else != nil {
		switch el := i.Else.(type) {
		case loneast.Block:
			b := te.analyzeBlock(&el, expectedRet)
			tElse = &b
		case *loneast.Block:
			b := te.analyzeBlock(el, expectedRet)
			tElse = &b
		case loneast.IfStmt:
			tElse = te.analyzeIfStmt(&el, expectedRet)
		case *loneast.IfStmt:
			tElse = te.analyzeIfStmt(el, expectedRet)
		}
	}

	if hasHeader {
		te.stack.Pop()
	}

	return &TIfStmt{
		Header: tHeaders,
		Cond:   tCond,
		Then:   tThen,
		Else:   tElse,
		Span_:  i.Span_,
	}
}

// analyzePackedIfStmt handles packed if with optional scoped header bindings: if: [let x = 0, let y = 7] { ... }
func (te *TypeEvaluator) analyzePackedIfStmt(p *loneast.PackedIfStmt, expectedRet Type) *TPackedIfStmt {
	hasHeader := len(p.Header) > 0
	if hasHeader {
		te.stack.Push(ScopeHeader)
	}

	var tHeaders []*TVarDec
	for _, h := range p.Header {
		tVar := te.analyzeVarDec(&h)
		tHeaders = append(tHeaders, tVar)
	}

	var tArms []TPackedIfArm
	for _, arm := range p.Arms {
		var tCond TExpr
		if arm.Cond != nil {
			tCond = te.EvalExpr(arm.Cond)
			if !tCond.ExprType().Equals(TypeBool) && tCond.ExprType() != TypeUnknown {
				te.addDiag(SeverityError, arm.Cond.NodeSpan(), "type/condition-not-bool",
					fmt.Sprintf("packed if arm condition must be boolean, got %q", tCond.ExprType().String()))
			}
		}
		var tBody TStmt
		if blk, ok := arm.Body.(loneast.Block); ok {
			b := te.analyzeBlock(&blk, expectedRet)
			tBody = &b
		} else if pblk, ok := arm.Body.(*loneast.Block); ok {
			b := te.analyzeBlock(pblk, expectedRet)
			tBody = &b
		} else {
			tBody = te.analyzeStmt(arm.Body, expectedRet)
		}
		tArms = append(tArms, TPackedIfArm{
			Cond:  tCond,
			Body:  tBody,
			Span_: arm.Span_,
		})
	}

	if hasHeader {
		te.stack.Pop()
	}

	return &TPackedIfStmt{
		Header: tHeaders,
		Arms:   tArms,
		Span_:  p.Span_,
	}
}

func (te *TypeEvaluator) analyzeWhileStmt(w *loneast.WhileStmt, expectedRet Type) *TWhileStmt {
	tCond := te.EvalExpr(w.Cond)
	tBody := te.analyzeBlock(&w.Body, expectedRet)
	return &TWhileStmt{Cond: tCond, Body: tBody, Span_: w.Span_}
}

// analyzeLoopStmt handles infinite loops and post-condition loops: loop { ... } while: [cond]
func (te *TypeEvaluator) analyzeLoopStmt(l *loneast.LoopStmt, expectedRet Type) *TLoopStmt {
	te.stack.Push(ScopeLoop)
	defer te.stack.Pop()

	var stmts []TStmt
	for _, s := range l.Body.Stmts {
		tStmt := te.analyzeStmt(s, expectedRet)
		if tStmt != nil {
			stmts = append(stmts, tStmt)
		}
	}
	tBody := TBlock{Stmts: stmts, Typ: TypeVoid, Span_: l.Body.Span_}

	var tWhileCond TExpr
	if l.WhileCond != nil {
		// Post-condition has access to loop-scoped variables!
		tWhileCond = te.EvalExpr(l.WhileCond)
		if !tWhileCond.ExprType().Equals(TypeBool) && tWhileCond.ExprType() != TypeUnknown {
			te.addDiag(SeverityError, l.WhileCond.NodeSpan(), "type/condition-not-bool",
				fmt.Sprintf("post-condition loop condition must be boolean, got %q", tWhileCond.ExprType().String()))
		}
	}

	return &TLoopStmt{
		Body:      tBody,
		WhileCond: tWhileCond,
		Span_:     l.Span_,
	}
}

func (te *TypeEvaluator) analyzeForStmt(f *loneast.ForStmt, expectedRet Type) *TForStmt {
	tIter := te.EvalExpr(f.Iter)

	te.stack.Push(ScopeBlock)
	defer te.stack.Pop()

	elemType := Type(TypeI32) // default fallback
	if lt, ok := tIter.ExprType().(*ListType); ok {
		elemType = lt.Elem
	}
	_ = te.stack.Define(&Symbol{Name: f.VarName, Kind: SymbolVar, Type: elemType, IsMutable: false})

	var stmts []TStmt
	for _, s := range f.Body.Stmts {
		tStmt := te.analyzeStmt(s, expectedRet)
		if tStmt != nil {
			stmts = append(stmts, tStmt)
		}
	}
	tBody := TBlock{Stmts: stmts, Typ: TypeVoid, Span_: f.Body.Span_}

	return &TForStmt{
		VarName: f.VarName,
		Iter:    tIter,
		Body:    tBody,
		Span_:   f.Span_,
	}
}

func (te *TypeEvaluator) analyzeLabelDec(l *loneast.LabelDec, expectedRet Type) *TLabelDec {
	body := te.analyzeBlock(&l.Body, expectedRet)
	return &TLabelDec{
		Name:  l.Name,
		Body:  body,
		Typ:   body.Typ,
		Span_: l.Span_,
	}
}

// =====================
// EXPRESSION EVALUATION
// =====================

func (te *TypeEvaluator) EvalExpr(e loneast.Expr) TExpr {
	if e == nil {
		return nil
	}

	switch expr := e.(type) {
	case loneast.Literal:
		return te.evalLiteral(expr)

	case loneast.ListLiteral:
		return te.evalListLiteral(expr)

	case loneast.ObjectLiteral:
		return te.evalObjectLiteral(expr)

	case loneast.NameRef:
		sym := te.stack.Lookup(expr.Name)
		if sym != nil {
			return &TVarRefExpr{Name: expr.Name, Typ: sym.Type, Span_: expr.Span_}
		}
		if et, ok := te.enums[expr.Name]; ok {
			return &TVarRefExpr{Name: expr.Name, Typ: et, Span_: expr.Span_}
		}
		if st, ok := te.structs[expr.Name]; ok {
			return &TVarRefExpr{Name: expr.Name, Typ: st, Span_: expr.Span_}
		}
		if tr, ok := te.traits[expr.Name]; ok {
			return &TVarRefExpr{Name: expr.Name, Typ: tr, Span_: expr.Span_}
		}
		if _, ok := te.genericTypes[expr.Name]; ok {
			return &TVarRefExpr{Name: expr.Name, Typ: TypeUnknown, Span_: expr.Span_}
		}
		if _, ok := te.genericFns[expr.Name]; ok {
			return &TVarRefExpr{Name: expr.Name, Typ: TypeUnknown, Span_: expr.Span_}
		}
		te.addDiag(SeverityError, expr.Span_, "type/undefined-identifier",
			fmt.Sprintf("undefined identifier %q", expr.Name))
		return &TVarRefExpr{Name: expr.Name, Typ: TypeUnknown, Span_: expr.Span_}

	case loneast.BinaryExpr:
		return te.evalBinaryExpr(expr)

	case loneast.UnaryExpr:
		return te.evalUnaryExpr(expr)

	case loneast.CastExpr:
		return te.evalCastExpr(expr)

	case loneast.IsExpr:
		return te.evalIsExpr(expr)

	case loneast.PostFixExpr:
		return te.evalPostfixExpr(expr)

	case loneast.LabelDec:
		return te.analyzeLabelDec(&expr, TypeUnknown)
	case *loneast.LabelDec:
		return te.analyzeLabelDec(expr, TypeUnknown)

	case loneast.MatchExpr:
		return te.evalMatchExpr(expr)
	case *loneast.MatchExpr:
		return te.evalMatchExpr(*expr)

	case loneast.MustExpr:
		return te.evalMustExpr(expr)
	case *loneast.MustExpr:
		return te.evalMustExpr(*expr)

	case loneast.TryExpr:
		return te.evalTryExpr(expr)
	case *loneast.TryExpr:
		return te.evalTryExpr(*expr)

	case loneast.TupleExpr:
		return te.evalTupleExpr(expr)
	case *loneast.TupleExpr:
		return te.evalTupleExpr(*expr)

	case loneast.AnonFnExpr:
		return te.evalAnonFnExpr(expr)
	case *loneast.AnonFnExpr:
		return te.evalAnonFnExpr(*expr)

	case loneast.InterpolatedStringExpr:
		return te.evalInterpolatedStringExpr(expr)
	case *loneast.InterpolatedStringExpr:
		return te.evalInterpolatedStringExpr(*expr)

	default:
		return nil
	}
}

func (te *TypeEvaluator) evalTupleExpr(t loneast.TupleExpr) TExpr {
	var elements []TExpr
	var elementTypes []Type
	for _, elem := range t.Elements {
		telem := te.EvalExpr(elem)
		if telem != nil {
			elements = append(elements, telem)
			elementTypes = append(elementTypes, telem.ExprType())
		} else {
			elementTypes = append(elementTypes, TypeUnknown)
		}
	}
	tupType := NewTupleType(elementTypes)
	return &TTupleExpr{
		Elements: elements,
		Typ:      tupType,
		Span_:    t.Span_,
	}
}

func (te *TypeEvaluator) evalAnonFnExpr(a loneast.AnonFnExpr) TExpr {
	te.stack.Push(ScopeFunction)
	defer te.stack.Pop()

	var fnParams []FnParamInfo
	var tParams []*TFnParam
	for _, p := range a.Params {
		pType := te.resolveType(p.Type)
		fnParams = append(fnParams, FnParamInfo{Name: p.Name, Type: pType})
		sym := &Symbol{Name: p.Name, Kind: SymbolVar, Type: pType, IsMutable: false}
		_ = te.stack.Define(sym)
		tParams = append(tParams, &TFnParam{Name: p.Name, Type: pType, Span_: p.Span_})
	}

	retType := Type(TypeVoid)
	if a.ReturnType != nil {
		retType = te.resolveType(*a.ReturnType)
	}

	body := te.analyzeBlock(&a.Body, retType)
	te.checkClosureCaptures(&body, a.Span_)

	fnType := &FnType{Params: fnParams, Return: retType}
	anonName := fmt.Sprintf("anon_fn_%d", len(te.extraDecls)+1)
	anonExpr := &TAnonFnExpr{
		Name:       anonName,
		Params:     tParams,
		ReturnType: retType,
		Body:       &body,
		Typ:        fnType,
		Span_:      a.Span_,
	}

	// Register generated function in extraDecls
	fnDec := &TFnDec{
		Name:       anonName,
		Params:     make([]TFnParam, len(tParams)),
		ReturnType: retType,
		Body:       body,
		Span_:      a.Span_,
	}
	for i, tp := range tParams {
		fnDec.Params[i] = *tp
	}
	te.extraDecls = append(te.extraDecls, fnDec)

	return anonExpr
}

func (te *TypeEvaluator) checkClosureCaptures(block *TBlock, closureSpan loneast.Span) {
	for _, stmt := range block.Stmts {
		te.walkStmtForClosureCaptures(stmt, closureSpan)
	}
}

func (te *TypeEvaluator) walkStmtForClosureCaptures(stmt TStmt, closureSpan loneast.Span) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *TVarDec:
		if s.Value != nil {
			te.walkExprForClosureCaptures(s.Value, closureSpan)
		}
	case *TAssignStmt:
		te.walkExprForClosureCaptures(s.Target, closureSpan)
		te.walkExprForClosureCaptures(s.Value, closureSpan)
	case *TReturnStmt:
		if s.Value != nil {
			te.walkExprForClosureCaptures(s.Value, closureSpan)
		}
	case *TDropStmt:
		te.walkExprForClosureCaptures(s.Expr, closureSpan)
	case TExpr:
		te.walkExprForClosureCaptures(s, closureSpan)
	}
}

func (te *TypeEvaluator) walkExprForClosureCaptures(expr TExpr, closureSpan loneast.Span) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *TUnaryExpr:
		if e.Op == loneast.UNARY_OP_REF || e.Op == loneast.UNARY_OP_RAW_REF {
			if vref, ok := e.Expr.(*TVarRefExpr); ok {
				sym := te.stack.Lookup(vref.Name)
				if sym != nil && te.stack.LookupLocal(vref.Name) == nil {
					te.addDiag(SeverityError, e.Span_, "closure/escaping-reference-capture",
						fmt.Sprintf("closure captures local stack variable %q by reference; escaping closures must capture by value or use #alloc", vref.Name))
				}
			}
		}
		te.walkExprForClosureCaptures(e.Expr, closureSpan)
	case *TBinaryExpr:
		te.walkExprForClosureCaptures(e.Left, closureSpan)
		te.walkExprForClosureCaptures(e.Right, closureSpan)
	case *TCallExpr:
		te.walkExprForClosureCaptures(e.Callee, closureSpan)
		for _, arg := range e.Args {
			te.walkExprForClosureCaptures(arg, closureSpan)
		}
	case *TMemberAccessExpr:
		te.walkExprForClosureCaptures(e.Base, closureSpan)
	case *TTupleExpr:
		for _, elem := range e.Elements {
			te.walkExprForClosureCaptures(elem, closureSpan)
		}
	case *TInterpolatedStringExpr:
		for _, part := range e.Parts {
			te.walkExprForClosureCaptures(part, closureSpan)
		}
	}
}

func (te *TypeEvaluator) isDisplayable(t Type) bool {
	if t == nil || t == TypeUnknown {
		return true
	}
	if refT, ok := t.(*RefType); ok {
		t = refT.Elem
	}
	// Primitives
	if t == TypeStr || t == TypeI32 || t == TypeI64 || t == TypeU32 || t == TypeU64 ||
		t == TypeF32 || t == TypeF64 || t == TypeBool || t == TypeChar {
		return true
	}
	if _, ok := t.(*PtrType); ok {
		return true
	}
	// Check TrDisplay trait implementation
	if te.traitImpls["TrDisplay"] != nil {
		if te.traitImpls["TrDisplay"][t.String()] != nil {
			return true
		}
	}
	return false
}

func (te *TypeEvaluator) evalInterpolatedStringExpr(s loneast.InterpolatedStringExpr) TExpr {
	var parts []TExpr
	for _, part := range s.Parts {
		tpart := te.EvalExpr(part)
		if tpart == nil {
			continue
		}
		parts = append(parts, tpart)

		pt := tpart.ExprType()
		if !te.isDisplayable(pt) {
			te.addDiag(SeverityError, part.NodeSpan(), "type/missing-trait-impl",
				fmt.Sprintf("type %q does not implement trait \"TrDisplay\"", pt.String()))
		}
	}

	return &TInterpolatedStringExpr{
		Parts: parts,
		Typ:   TypeStr,
		Span_: s.Span_,
	}
}

func (te *TypeEvaluator) evalLiteral(l loneast.Literal) TExpr {
	switch l.Kind {
	case loneast.LiteralInt:
		return &TLiteralExpr{Kind: l.Kind, Value: l.Value, Typ: TypeI32, Span_: l.Span_}
	case loneast.LiteralFloat:
		return &TLiteralExpr{Kind: l.Kind, Value: l.Value, Typ: TypeF32, Span_: l.Span_}
	case loneast.LiteralString:
		return &TLiteralExpr{Kind: l.Kind, Value: l.Value, Typ: TypeStr, Span_: l.Span_}
	case loneast.LiteralChar:
		return &TLiteralExpr{Kind: l.Kind, Value: l.Value, Typ: TypeChar, Span_: l.Span_}
	case loneast.LiteralBool:
		return &TLiteralExpr{Kind: l.Kind, Value: l.Value, Typ: TypeBool, Span_: l.Span_}
	case loneast.LiteralNone:
		return &TLiteralExpr{Kind: l.Kind, Value: nil, Typ: TypeNone, Span_: l.Span_}
	case loneast.LiteralUninitialized:
		return &TLiteralExpr{Kind: l.Kind, Value: nil, Typ: TypeInfer, Span_: l.Span_}
	default:
		return &TLiteralExpr{Kind: l.Kind, Value: l.Value, Typ: TypeUnknown, Span_: l.Span_}
	}
}

func (te *TypeEvaluator) evalListLiteral(l loneast.ListLiteral) TExpr {
	var elemType Type = TypeAny
	if len(l.Elements) > 0 {
		first := te.EvalExpr(l.Elements[0])
		elemType = first.ExprType()
	}
	return &TLiteralExpr{
		Kind:  loneast.LiteralInt, // stored as list value
		Value: l.Elements,
		Typ:   &ListType{Elem: elemType, Fixed: true, Size: len(l.Elements)},
		Span_: l.Span_,
	}
}

func (te *TypeEvaluator) evalObjectLiteral(o loneast.ObjectLiteral) TExpr {
	return &TLiteralExpr{
		Kind:  loneast.LiteralInt,
		Value: o.Fields,
		Typ:   TypeAny,
		Span_: o.Span_,
	}
}

func (te *TypeEvaluator) evalBinaryExpr(b loneast.BinaryExpr) TExpr {
	left := te.EvalExpr(b.Left)
	right := te.EvalExpr(b.Right)

	lType := left.ExprType()
	rType := right.ExprType()

	switch b.Op {
	case "+", "-", "*", "/", "%", "^", "&", "|":
		if lType.Equals(rType) {
			return &TBinaryExpr{Op: b.Op, Left: left, Right: right, Typ: lType, Span_: b.Span_}
		}
		// In Lone, binary ops require identical types; no silent int-to-float promotion!
		te.addDiag(SeverityError, b.Span_, "type/mismatch",
			fmt.Sprintf("operator %s cannot be applied to mismatched types %q and %q (use explicit 'as' cast)",
				b.Op, lType.String(), rType.String()))
		return &TBinaryExpr{Op: b.Op, Left: left, Right: right, Typ: lType, Span_: b.Span_}

	case "==", "!=", "<", "<=", ">", ">=":
		if !lType.Equals(rType) && lType != TypeUnknown && rType != TypeUnknown {
			te.addDiag(SeverityError, b.Span_, "type/comparison-mismatch",
				fmt.Sprintf("cannot compare mismatched types %q and %q", lType.String(), rType.String()))
		}
		return &TBinaryExpr{Op: b.Op, Left: left, Right: right, Typ: TypeBool, Span_: b.Span_}

	case "and", "or", "xor":
		if !lType.Equals(TypeBool) || !rType.Equals(TypeBool) {
			te.addDiag(SeverityError, b.Span_, "type/logical-not-bool",
				fmt.Sprintf("logical %s operator requires boolean operands, got %q and %q",
					b.Op, lType.String(), rType.String()))
		}
		return &TBinaryExpr{Op: b.Op, Left: left, Right: right, Typ: TypeBool, Span_: b.Span_}

	default:
		return &TBinaryExpr{Op: b.Op, Left: left, Right: right, Typ: lType, Span_: b.Span_}
	}
}

func (te *TypeEvaluator) evalUnaryExpr(u loneast.UnaryExpr) TExpr {
	sub := te.EvalExpr(u.Expr)
	sType := sub.ExprType()

	switch u.Op {
	case loneast.UNARY_OP_POSITIVE, loneast.UNARY_OP_NEGATIVE:
		return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: sType, Span_: u.Span_}

	case loneast.UNARY_OP_NOT:
		return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: TypeBool, Span_: u.Span_}

	case loneast.UNARY_OP_DEREF: // *expr
		if pt, ok := sType.(*PtrType); ok {
			return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: pt.Elem, Span_: u.Span_}
		}
		if rt, ok := sType.(*RefType); ok {
			return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: rt.Elem, Span_: u.Span_}
		}
		te.addDiag(SeverityError, u.Span_, "type/invalid-deref",
			fmt.Sprintf("cannot dereference non-pointer, non-reference type %q", sType.String()))
		return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: TypeUnknown, Span_: u.Span_}

	case loneast.UNARY_OP_REF: // &expr
		isRo := false
		if vref, ok := sub.(*TVarRefExpr); ok {
			sym := te.stack.Lookup(vref.Name)
			if sym != nil {
				sym.AddressTaken = true
				if tDec, ok := sym.Node.(*TVarDec); ok {
					tDec.AddressTaken = true
				}
				if !sym.IsMutable {
					isRo = true
				}
			}
		} else if mem, ok := sub.(*TMemberAccessExpr); ok {
			if st, ok := mem.Base.ExprType().(*StructType); ok {
				if fInfo, exists := st.Field(mem.Field); exists && fInfo.IsReadOnly {
					isRo = true
				}
			}
			if vref, ok := mem.Base.(*TVarRefExpr); ok {
				sym := te.stack.Lookup(vref.Name)
				if sym != nil && !sym.IsMutable {
					isRo = true
				}
			}
		}
		return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: &RefType{Elem: sType, ReadOnly: isRo}, Span_: u.Span_}

	case loneast.UNARY_OP_RAW_REF: // &raw expr
		isRo := false
		if vref, ok := sub.(*TVarRefExpr); ok {
			sym := te.stack.Lookup(vref.Name)
			if sym != nil {
				sym.AddressTaken = true
				if tDec, ok := sym.Node.(*TVarDec); ok {
					tDec.AddressTaken = true
				}
				if !sym.IsMutable {
					isRo = true
				}
			}
		} else if mem, ok := sub.(*TMemberAccessExpr); ok {
			if st, ok := mem.Base.ExprType().(*StructType); ok {
				if fInfo, exists := st.Field(mem.Field); exists && fInfo.IsReadOnly {
					isRo = true
				}
			}
			if vref, ok := mem.Base.(*TVarRefExpr); ok {
				sym := te.stack.Lookup(vref.Name)
				if sym != nil && !sym.IsMutable {
					isRo = true
				}
			}
		}
		return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: &PtrType{Elem: sType, ReadOnly: isRo}, Span_: u.Span_}

	default:
		return &TUnaryExpr{Op: u.Op, Expr: sub, Typ: sType, Span_: u.Span_}
	}
}

// evalCastExpr validates explicit type casts: expr as TargetType
func (te *TypeEvaluator) evalCastExpr(c loneast.CastExpr) TExpr {
	sub := te.EvalExpr(c.Expr)
	targetType := te.resolveType(c.TargetType)
	srcType := sub.ExprType()

	// Validation rules for explicit casts
	isSrcNum := IsIntegerType(srcType) || IsFloatType(srcType)
	isDstNum := IsIntegerType(targetType) || IsFloatType(targetType)

	if isSrcNum && isDstNum {
		// Valid numeric cast: int to float, float to int, widening, narrowing
		return &TCastExpr{Expr: sub, TargetType: targetType, Span_: c.Span_}
	}

	// Pointer casts: ptr<T> as ptr<U>
	_, isSrcPtr := srcType.(*PtrType)
	_, isDstPtr := targetType.(*PtrType)
	if isSrcPtr && isDstPtr {
		return &TCastExpr{Expr: sub, TargetType: targetType, Span_: c.Span_}
	}

	// Ref to Ptr cast: ref<T> as ptr<T>
	_, isSrcRef := srcType.(*RefType)
	if isSrcRef && isDstPtr {
		return &TCastExpr{Expr: sub, TargetType: targetType, Span_: c.Span_}
	}

	te.addDiag(SeverityError, c.Span_, "type/invalid-cast",
		fmt.Sprintf("invalid explicit cast from type %q to %q", srcType.String(), targetType.String()))
	return &TCastExpr{Expr: sub, TargetType: targetType, Span_: c.Span_}
}

func (te *TypeEvaluator) evalIsExpr(i loneast.IsExpr) TExpr {
	sub := te.EvalExpr(i.Expr)

	if tb, ok := i.TargetType.Value.(*loneast.TypeBase); ok {
		// 1. Explicit variant path: s is Shape::Circle or s is Option::Some
		if tb.Variant != "" {
			var et *EnumType
			if e, ok := te.enums[tb.Name]; ok {
				et = e
			} else if subEt, ok := sub.ExprType().(*EnumType); ok && (subEt.Name == tb.Name || strings.HasPrefix(subEt.Name, tb.Name+"<")) {
				et = subEt
			} else if tmpl, ok := te.genericTypes[tb.Name]; ok {
				if subEt, ok := sub.ExprType().(*EnumType); ok && strings.HasPrefix(subEt.Name, tmpl.Name+"<") {
					et = subEt
				}
			}
			if et != nil {
				if vInfo, found := et.Variant(tb.Variant); found {
					return &TIsExpr{
						Expr:          sub,
						TargetType:    et,
						TargetVariant: vInfo,
						Negated:       i.Negated,
						Span_:         i.Span_,
					}
				}
				te.addDiag(SeverityError, i.TargetType.NodeSpan(), "type/undefined-variant",
					fmt.Sprintf("enum %q has no variant %q", tb.Name, tb.Variant))
			}
		} else {
			// 2. Short variant name: s is Circle
			if et, ok := sub.ExprType().(*EnumType); ok {
				if vInfo, found := et.Variant(tb.Name); found {
					return &TIsExpr{
						Expr:          sub,
						TargetType:    et,
						TargetVariant: vInfo,
						Negated:       i.Negated,
						Span_:         i.Span_,
					}
				}
			}
		}
	}

	targetType := te.resolveType(i.TargetType)
	return &TIsExpr{Expr: sub, TargetType: targetType, Negated: i.Negated, Span_: i.Span_}
}

func (te *TypeEvaluator) evalPostfixExpr(p loneast.PostFixExpr) TExpr {
	curr := te.EvalExpr(p.Base)

	for _, s := range p.Suffix {
		switch suff := s.(type) {
		case loneast.SuffixMemberAccess:
			if tup, ok := curr.ExprType().(*TupleType); ok {
				idx, err := strconv.Atoi(suff.Name)
				if err != nil || idx < 0 || idx >= len(tup.Elements) {
					te.addDiag(SeverityError, suff.Span_, "type/tuple-index-out-of-bounds",
						fmt.Sprintf("tuple index %s is out of bounds for tuple %s", suff.Name, tup.String()))
					curr = &TMemberAccessExpr{Base: curr, Field: suff.Name, Typ: TypeUnknown, Span_: suff.Span_}
				} else {
					curr = &TMemberAccessExpr{
						Base:   curr,
						Field:  suff.Name,
						Offset: tup.Offsets[idx],
						Typ:    tup.Elements[idx],
						Span_:  suff.Span_,
					}
				}
				break
			}

			var targetStruct *StructType
			if st, ok := curr.ExprType().(*StructType); ok {
				targetStruct = st
			} else if vref, ok := curr.(*TVarRefExpr); ok {
				if st, ok := te.structs[vref.Name]; ok {
					targetStruct = st
				}
			}

			if targetStruct != nil {
				if fInfo, exists := targetStruct.Field(suff.Name); exists {
					if fInfo.IsPrivate {
						isAllowed := false
						if te.currentSelfType != nil && te.currentSelfType.Equals(targetStruct) {
							isAllowed = true
						}
						if !isAllowed {
							te.addDiag(SeverityError, suff.Span_, "type/private-field-access",
								fmt.Sprintf("cannot access private field %q on struct %q outside its associated methods", suff.Name, targetStruct.Name))
						}
					}
					curr = &TMemberAccessExpr{Base: curr, Field: suff.Name, Offset: fInfo.Offset, Typ: fInfo.Type, Span_: suff.Span_}
				} else if mInfo, exists := targetStruct.Method(suff.Name); exists {
					curr = &TMemberAccessExpr{Base: curr, Field: suff.Name, Typ: mInfo, Span_: suff.Span_}
				} else {
					te.addDiag(SeverityError, suff.Span_, "type/undefined-field",
						fmt.Sprintf("struct %q has no field or method %q", targetStruct.Name, suff.Name))
					curr = &TMemberAccessExpr{Base: curr, Field: suff.Name, Typ: TypeUnknown, Span_: suff.Span_}
				}
			} else if mat, ok := curr.ExprType().(*MaterializedTraitType); ok {
				if mInfo, exists := mat.Methods[suff.Name]; exists {
					curr = &TMemberAccessExpr{Base: curr, Field: suff.Name, Typ: mInfo, Span_: suff.Span_}
				} else {
					te.addDiag(SeverityError, suff.Span_, "type/undefined-trait-method",
						fmt.Sprintf("trait %q has no method %q", mat.Trait.Name, suff.Name))
					curr = &TMemberAccessExpr{Base: curr, Field: suff.Name, Typ: TypeUnknown, Span_: suff.Span_}
				}
			} else if tr, ok := curr.ExprType().(*TraitType); ok {
				if mInfo, exists := tr.Methods[suff.Name]; exists {
					curr = &TMemberAccessExpr{Base: curr, Field: suff.Name, Typ: mInfo, Span_: suff.Span_}
				} else {
					te.addDiag(SeverityError, suff.Span_, "type/undefined-trait-method",
						fmt.Sprintf("trait %q has no method %q", tr.Name, suff.Name))
					curr = &TMemberAccessExpr{Base: curr, Field: suff.Name, Typ: TypeUnknown, Span_: suff.Span_}
				}
			} else {
				curr = &TMemberAccessExpr{Base: curr, Field: suff.Name, Typ: TypeUnknown, Span_: suff.Span_}
			}

		case loneast.SuffixCall:
			var tArgs []TExpr
			for _, arg := range suff.Args {
				tArgs = append(tArgs, te.EvalExpr(arg))
			}
			if vref, ok := curr.(*TVarRefExpr); ok {
				if tmpl, ok := te.genericFns[vref.Name]; ok {
					specialized := te.monomorphizeFn(tmpl, tArgs)
					if specialized != nil {
						var fnParams []FnParamInfo
						for _, p := range specialized.Params {
							fnParams = append(fnParams, FnParamInfo{Name: p.Name, Type: p.Type})
						}
						fnTyp := &FnType{Params: fnParams, Return: specialized.ReturnType}
						curr = &TVarRefExpr{Name: specialized.Name, Typ: fnTyp, Span_: vref.Span_}
					}
				}
			}
			if enumConstr, ok := curr.(*TEnumConstructExpr); ok {
				if enumConstr.Variant.Name == "Err" && len(tArgs) > 0 {
					if te.currentFnReturn != nil {
						if retEnum, ok := te.currentFnReturn.(*EnumType); ok {
							if errV, ok := retEnum.Variant("Err"); ok && len(errV.Fields) > 0 && errV.Fields[0].Type == TypeInfer {
								te.inferredErrors[te.currentFnName] = append(te.inferredErrors[te.currentFnName], tArgs[0].ExprType())
							}
						}
					}
				}
				// If enum constructor has unknown/unspecified type parameters, infer from tArgs
				baseEnumName := enumConstr.EnumType.Name
				if idx := strings.Index(baseEnumName, "<"); idx != -1 {
					baseEnumName = baseEnumName[:idx]
				}
				if tmpl, ok := te.genericTypes[baseEnumName]; ok && len(tArgs) > 0 {
					if strings.Contains(enumConstr.EnumType.Name, "unknown") || strings.Contains(enumConstr.EnumType.Name, "any") {
						var inferredArgs []loneast.TYPE_REF
						for _, arg := range tArgs {
							inferredArgs = append(inferredArgs, loneast.TYPE_REF{
								Kind:  loneast.TYPE_BASE,
								Value: &loneast.TypeBase{Name: arg.ExprType().String()},
							})
						}
						concreteRes := te.instantiateGenericType(tmpl, inferredArgs)
						if crEnum, ok := concreteRes.(*EnumType); ok {
							enumConstr.EnumType = crEnum
							if v, ok := crEnum.Variant(enumConstr.Variant.Name); ok {
								enumConstr.Variant = v
							}
							enumConstr.Typ = crEnum
						}
					}
				}
				if len(tArgs) != len(enumConstr.Variant.Fields) {
					te.addDiag(SeverityError, suff.Span_, "type/variant-arity-mismatch",
						fmt.Sprintf("variant %q::%q expects %d arguments, got %d",
							enumConstr.EnumType.Name, enumConstr.Variant.Name, len(enumConstr.Variant.Fields), len(tArgs)))
				} else {
					for i, arg := range tArgs {
						expectedF := enumConstr.Variant.Fields[i]
						if expectedF.Type != TypeInfer && !arg.ExprType().AssignableTo(expectedF.Type) {
							te.addDiag(SeverityError, arg.Span(), "type/mismatch",
								fmt.Sprintf("cannot pass argument of type %q to field %q of type %q",
									arg.ExprType().String(), expectedF.Name, expectedF.Type.String()))
						}
					}
				}
				enumConstr.Args = tArgs
				enumConstr.Span_ = loneast.Span{Start: enumConstr.Span_.Start, End: suff.Span_.End}
				curr = enumConstr
			} else {
				retType := Type(TypeVoid)
				if fnT, ok := curr.ExprType().(*FnType); ok {
					retType = fnT.Return
				}
				curr = &TCallExpr{Callee: curr, Args: tArgs, Typ: retType, Span_: suff.Span_}
			}

		case loneast.SuffixAssociatedAccess:
			var targetEnum *EnumType
			if et, ok := curr.ExprType().(*EnumType); ok {
				targetEnum = et
			} else if vref, ok := curr.(*TVarRefExpr); ok {
				if et, ok := te.enums[vref.Name]; ok {
					targetEnum = et
				} else if tmpl, ok := te.genericTypes[vref.Name]; ok {
					if fnRetEnum, ok := te.currentFnReturn.(*EnumType); ok && strings.HasPrefix(fnRetEnum.Name, tmpl.Name+"<") {
						targetEnum = fnRetEnum
					} else {
						if instantiated, ok := te.instantiateGenericType(tmpl, nil).(*EnumType); ok {
							targetEnum = instantiated
						}
					}
				}
			}

			if targetEnum != nil {
				vInfo, ok := targetEnum.Variant(suff.Name)
				if !ok {
					te.addDiag(SeverityError, suff.Span_, "type/undefined-variant",
						fmt.Sprintf("enum %q has no variant %q", targetEnum.Name, suff.Name))
					curr = &TVarRefExpr{Name: suff.Name, Typ: TypeUnknown, Span_: suff.Span_}
				} else {
					curr = &TEnumConstructExpr{
						EnumType: targetEnum,
						Variant:  vInfo,
						Args:     nil,
						Typ:      targetEnum,
						Span_:    loneast.Span{Start: curr.Span().Start, End: suff.Span_.End},
					}
				}
			} else {
				curr = &TVarRefExpr{Name: suff.Name, Typ: TypeUnknown, Span_: suff.Span_}
			}

		case loneast.SuffixCallTypeMake:
			var targetStruct *StructType
			if st, ok := curr.ExprType().(*StructType); ok {
				targetStruct = st
			} else if vref, ok := curr.(*TVarRefExpr); ok {
				if st, ok := te.structs[vref.Name]; ok {
					targetStruct = st
				} else if tmpl, ok := te.genericTypes[vref.Name]; ok {
					if fnRetSt, ok := te.currentFnReturn.(*StructType); ok && strings.HasPrefix(fnRetSt.Name, tmpl.Name+"<") {
						targetStruct = fnRetSt
					} else {
						if instantiated, ok := te.instantiateGenericType(tmpl, nil).(*StructType); ok {
							targetStruct = instantiated
						}
					}
				}
			}

			if targetStruct != nil && targetStruct.HasPrivateFields() {
				isAllowed := false
				if te.currentSelfType != nil && te.currentSelfType.Equals(targetStruct) {
					isAllowed = true
				}
				if !isAllowed {
					te.addDiag(SeverityError, suff.Span_, "type/raw-construct-private-fields",
						fmt.Sprintf("struct %q contains private field(s) and cannot be instantiated with raw aggregate syntax '::{ ... }'; use %s::(...) instead", targetStruct.Name, targetStruct.Name))
				}
			}

			if targetStruct != nil {
				fieldExprs := make(map[string]TExpr)
				if objLit, ok := suff.Data.(loneast.ObjectLiteral); ok {
					for _, f := range objLit.Fields {
						fieldExprs[f.Name] = te.EvalExpr(f.Expr)
					}
				}
				curr = &TStructConstructExpr{
					StructType: targetStruct,
					Fields:     fieldExprs,
					Typ:        targetStruct,
					Span_:      loneast.Span{Start: curr.Span().Start, End: suff.Span_.End},
				}
			} else {
				curr = &TLiteralExpr{
					Kind:  loneast.LiteralInt,
					Value: suff.Data,
					Typ:   TypeAny,
					Span_: suff.Span_,
				}
			}

		case loneast.SuffixConstructorCall:
			var targetStruct *StructType
			if st, ok := curr.ExprType().(*StructType); ok {
				targetStruct = st
			} else if vref, ok := curr.(*TVarRefExpr); ok {
				if st, ok := te.structs[vref.Name]; ok {
					targetStruct = st
				} else if tmpl, ok := te.genericTypes[vref.Name]; ok {
					if fnRetSt, ok := te.currentFnReturn.(*StructType); ok && strings.HasPrefix(fnRetSt.Name, tmpl.Name+"<") {
						targetStruct = fnRetSt
					} else {
						if instantiated, ok := te.instantiateGenericType(tmpl, nil).(*StructType); ok {
							targetStruct = instantiated
						}
					}
				}
			}

			if targetStruct == nil {
				te.addDiag(SeverityError, suff.Span_, "type/unresolved-constructor",
					fmt.Sprintf("cannot invoke constructor on non-struct type %q", curr.ExprType().String()))
				curr = &TCallExpr{Callee: curr, Args: nil, Typ: TypeUnknown, Span_: suff.Span_}
			} else {
				initMethod, hasInit := targetStruct.Method("__init__")
				if !hasInit {
					te.addDiag(SeverityError, suff.Span_, "type/unresolved-constructor",
						fmt.Sprintf("struct %q does not define an '__init__' constructor; use raw aggregate %s::{ ... } instead", targetStruct.Name, targetStruct.Name))
					curr = &TCallExpr{Callee: curr, Args: nil, Typ: targetStruct, Span_: suff.Span_}
				} else {
					var tArgs []TExpr
					for _, arg := range suff.Args {
						tArgs = append(tArgs, te.EvalExpr(arg))
					}
					if len(tArgs) != len(initMethod.Params) {
						te.addDiag(SeverityError, suff.Span_, "type/call-arg-count",
							fmt.Sprintf("constructor %s::(...) expects %d arguments, but got %d", targetStruct.Name, len(initMethod.Params), len(tArgs)))
					} else {
						for i, param := range initMethod.Params {
							argTyp := tArgs[i].ExprType()
							if !argTyp.AssignableTo(param.Type) && param.Type != TypeInfer && argTyp != TypeUnknown {
								te.addDiag(SeverityError, tArgs[i].Span(), "type/call-arg-type",
									fmt.Sprintf("cannot pass argument of type %q to parameter %q of type %q in constructor %s::(...)",
										argTyp.String(), param.Name, param.Type.String(), targetStruct.Name))
							}
						}
					}
					callee := &TVarRefExpr{
						Name:  targetStruct.Name + ".__init__",
						Typ:   initMethod,
						Span_: loneast.Span{Start: curr.Span().Start, End: suff.Span_.End},
					}
					curr = &TCallExpr{
						Callee: callee,
						Args:   tArgs,
						Typ:    targetStruct,
						Span_:  suff.Span_,
					}
				}
			}

		case loneast.SuffixIndex:
			tIdx := te.EvalExpr(suff.Index)
			elemType := Type(TypeUnknown)
			if lt, ok := curr.ExprType().(*ListType); ok {
				elemType = lt.Elem
			} else if pt, ok := curr.ExprType().(*PtrType); ok {
				elemType = pt.Elem
			} else if curr.ExprType() == TypeStr {
				elemType = TypeI32
			} else if rt, ok := curr.ExprType().(*RefType); ok {
				te.addDiag(SeverityError, suff.Span_, "type/invalid-index",
					fmt.Sprintf("cannot index managed reference of type %q; indexing is only supported on raw pointer buffers 'ptr<T>'", rt.String()))
			} else {
				te.addDiag(SeverityError, suff.Span_, "type/invalid-index",
					fmt.Sprintf("cannot index value of non-buffer type %q", curr.ExprType().String()))
			}
			curr = &TIndexExpr{Base: curr, Index: tIdx, Typ: elemType, Span_: suff.Span_}

		case loneast.SuffixTraitMat:
			targetType := te.resolveType(suff.TargetType)
			traitName := ""
			var tr *TraitType
			if traitT, ok := curr.ExprType().(*TraitType); ok {
				tr = traitT
				traitName = traitT.Name
			} else if vref, ok := curr.(*TVarRefExpr); ok {
				if traitT, ok := te.traits[vref.Name]; ok {
					tr = traitT
					traitName = vref.Name
				}
			}

			if tr == nil {
				te.addDiag(SeverityError, suff.Span_, "type/not-a-trait",
					fmt.Sprintf("cannot materialize non-trait %q", curr.ExprType().String()))
				curr = &TTraitMatExpr{TraitName: traitName, TargetType: targetType, Typ: TypeUnknown, Span_: suff.Span_}
			} else {
				if targetType != TypeUnknown && targetType != TypeInfer {
					if te.traitImpls[traitName] == nil || te.traitImpls[traitName][targetType.String()] == nil {
						te.addDiag(SeverityError, suff.Span_, "type/missing-trait-impl",
							fmt.Sprintf("type %q does not implement trait %q", targetType.String(), traitName))
					}
				}
				mMethods := make(map[string]*FnType)
				for mName, sig := range tr.Methods {
					var mParams []FnParamInfo
					for _, p := range sig.Params {
						pTyp := p.Type
						if p.Name == "self" {
							pTyp = targetType
						}
						mParams = append(mParams, FnParamInfo{Name: p.Name, Type: pTyp})
					}
					mMethods[mName] = &FnType{Params: mParams, Return: sig.Return}
				}
				matType := &MaterializedTraitType{
					Trait:      tr,
					TargetType: targetType,
					Methods:    mMethods,
				}
				curr = &TTraitMatExpr{TraitName: traitName, TargetType: targetType, Typ: matType, Span_: suff.Span_}
			}

		default:
			// No-op for now
		}
	}

	return curr
}

func (te *TypeEvaluator) evalMatchExpr(m loneast.MatchExpr) TExpr {
	target := te.EvalExpr(m.Target)
	targetType := target.ExprType()

	var tArms []TMatchArm
	var armTypes []Type
	hasWildcard := false
	coveredVariants := make(map[string]bool)

	enumType, isEnum := targetType.(*EnumType)

	for _, arm := range m.Arms {
		te.stack.Push(ScopeBlock)

		var vInfo *EnumVariantInfo
		var bindings []*TVarDec

		switch pat := arm.Pattern.(type) {
		case loneast.WildcardPattern:
			hasWildcard = true

		case loneast.VariantPattern:
			if !isEnum {
				te.addDiag(SeverityError, pat.NodeSpan(), "type/invalid-pattern",
					fmt.Sprintf("cannot match variant on non-enum type %q", targetType.String()))
			} else {
				if pat.TypeName != "" && pat.TypeName != enumType.Name && !strings.HasPrefix(enumType.Name, pat.TypeName+"<") {
					te.addDiag(SeverityError, pat.NodeSpan(), "type/enum-type-mismatch",
						fmt.Sprintf("variant pattern belongs to enum %q, but target is enum %q", pat.TypeName, enumType.Name))
				}
				varName := pat.Variant
				v, found := enumType.Variant(varName)
				if !found {
					te.addDiag(SeverityError, pat.NodeSpan(), "type/unknown-variant",
						fmt.Sprintf("enum %q has no variant %q", enumType.Name, varName))
				} else {
					vInfo = v
					coveredVariants[varName] = true
					if len(pat.Bindings) != len(v.Fields) {
						te.addDiag(SeverityError, pat.NodeSpan(), "type/pattern-arity-mismatch",
							fmt.Sprintf("variant %q has %d fields, but pattern binds %d variables",
								varName, len(v.Fields), len(pat.Bindings)))
					} else {
						for i, bName := range pat.Bindings {
							fInfo := v.Fields[i]
							if bName != "_" {
								sym := &Symbol{
									Name:      bName,
									Kind:      SymbolVar,
									Type:      fInfo.Type,
									IsMutable: false,
								}
								_ = te.stack.Define(sym)
								bindings = append(bindings, &TVarDec{
									Name:      bName,
									IsMutable: false,
									Type:      fInfo.Type,
									Value:     nil,
									Span_:     pat.NodeSpan(),
								})
							}
						}
					}
				}
			}

		case loneast.LiteralPattern:
			litExpr := te.evalLiteral(pat.Literal)
			if !litExpr.ExprType().AssignableTo(targetType) && !targetType.AssignableTo(litExpr.ExprType()) {
				te.addDiag(SeverityError, pat.NodeSpan(), "type/pattern-mismatch",
					fmt.Sprintf("pattern of type %q cannot match target of type %q",
						litExpr.ExprType().String(), targetType.String()))
			}
		}

		var bodyExpr TExpr
		var bodyBlock *TBlock
		var armType Type = TypeVoid

		if blk, ok := arm.Body.(loneast.Block); ok {
			tb := te.analyzeBlock(&blk, TypeUnknown)
			bodyBlock = &tb
			armType = tb.Typ
		} else if pblk, ok := arm.Body.(*loneast.Block); ok {
			tb := te.analyzeBlock(pblk, TypeUnknown)
			bodyBlock = &tb
			armType = tb.Typ
		} else if expr, ok := arm.Body.(loneast.Expr); ok {
			bodyExpr = te.EvalExpr(expr)
			armType = bodyExpr.ExprType()
		}

		te.stack.Pop()

		tArms = append(tArms, TMatchArm{
			Pattern:   arm.Pattern,
			Variant:   vInfo,
			Bindings:  bindings,
			BodyExpr:  bodyExpr,
			BodyBlock: bodyBlock,
			Span_:     arm.Span_,
		})
		armTypes = append(armTypes, armType)
	}

	// Check exhaustiveness for enum targets
	if isEnum && !hasWildcard {
		if len(coveredVariants) < len(enumType.Variants) {
			var missing []string
			for _, v := range enumType.Variants {
				if !coveredVariants[v.Name] {
					missing = append(missing, v.Name)
				}
			}
			te.addDiag(SeverityError, m.Span_, "type/non-exhaustive-match",
				fmt.Sprintf("non-exhaustive match on enum %q: missing variants %s",
					enumType.Name, strings.Join(missing, ", ")))
		}
	} else if !isEnum && !hasWildcard && len(m.Arms) > 0 {
		te.addDiag(SeverityError, m.Span_, "type/non-exhaustive-match",
			fmt.Sprintf("non-exhaustive match on type %q: missing wildcard '_' arm", targetType.String()))
	}

	// Unify arm result types
	var resultType Type = TypeVoid
	if len(armTypes) > 0 {
		resultType = armTypes[0]
		for i := 1; i < len(armTypes); i++ {
			if !armTypes[i].AssignableTo(resultType) && !resultType.AssignableTo(armTypes[i]) {
				te.addDiag(SeverityError, m.Arms[i].Span_, "type/match-arm-mismatch",
					fmt.Sprintf("match arm has incompatible return type %q (expected %q)",
						armTypes[i].String(), resultType.String()))
			}
		}
	}

	return &TMatchExpr{
		Target: target,
		Arms:   tArms,
		Typ:    resultType,
		Span_:  m.Span_,
	}
}

func (te *TypeEvaluator) evalMustExpr(m loneast.MustExpr) TExpr {
	sub := te.EvalExpr(m.Expr)
	et, ok := sub.ExprType().(*EnumType)
	if !ok {
		te.addDiag(SeverityError, m.Span_, "type/must-requires-result",
			fmt.Sprintf("'must' requires a Result/enum type, got %q", sub.ExprType().String()))
		return &TMustExpr{Expr: sub, OkType: TypeUnknown, Span_: m.Span_}
	}
	okV, hasOk := et.Variant("Ok")
	if !hasOk {
		te.addDiag(SeverityError, m.Span_, "type/must-requires-ok-variant",
			fmt.Sprintf("'must' requires enum %q to have an 'Ok' variant", et.Name))
		return &TMustExpr{Expr: sub, OkType: TypeUnknown, Span_: m.Span_}
	}
	var okType Type = TypeVoid
	if len(okV.Fields) > 0 {
		okType = okV.Fields[0].Type
	}
	return &TMustExpr{
		Expr:       sub,
		ResultType: et,
		OkVariant:  okV,
		OkType:     okType,
		Span_:      m.Span_,
	}
}

func (te *TypeEvaluator) instantiateGenericType(tmpl *GenericTypeTemplate, args []loneast.TYPE_REF) Type {
	paramMap := make(map[string]Type)
	var typeArgNames []string

	for i, p := range tmpl.TypeParams {
		var resolved Type
		if i < len(args) {
			resolved = te.resolveType(args[i])
		} else if p.Default != nil {
			resolved = te.resolveType(*p.Default)
		} else {
			resolved = TypeUnknown
		}
		paramMap[p.Name] = resolved
		typeArgNames = append(typeArgNames, resolved.String())
	}

	monoName := fmt.Sprintf("%s<%s>", tmpl.Name, strings.Join(typeArgNames, ", "))

	// Check caches
	if et, ok := te.enums[monoName]; ok {
		return et
	}
	if st, ok := te.structs[monoName]; ok {
		return st
	}

	// Validate constraints
	for _, c := range tmpl.Constraints {
		actualType, exists := paramMap[c.TypeParam]
		if !exists || actualType == TypeUnknown {
			continue
		}
		switch c.Op {
		case loneast.ConstraintIs:
			targetType := te.resolveType(c.Target)
			targetName := ""
			if tb, ok := c.Target.Value.(*loneast.TypeBase); ok {
				targetName = tb.Name
			} else {
				targetName = targetType.String()
			}

			if targetName == "Error" {
				isErr := false
				if actualType.String() == "Error" || actualType == TypeInfer {
					isErr = true
				} else if _, isEnum := actualType.(*EnumType); isEnum {
					isErr = true
				}
				if !isErr {
					te.addDiag(SeverityError, c.Span_, "type/constraint-violation",
						fmt.Sprintf("type parameter %q bound to %q does not satisfy 'is Error'", c.TypeParam, actualType.String()))
					return TypeUnknown
				}
			} else {
				if !actualType.AssignableTo(targetType) && !actualType.Equals(targetType) {
					te.addDiag(SeverityError, c.Span_, "type/constraint-violation",
						fmt.Sprintf("type parameter %q bound to %q does not satisfy 'is %s'", c.TypeParam, actualType.String(), targetType.String()))
					return TypeUnknown
				}
			}

		case loneast.ConstraintSatisfies:
			targetName := ""
			if tb, ok := c.Target.Value.(*loneast.TypeBase); ok {
				targetName = tb.Name
			} else {
				targetName = te.resolveType(c.Target).String()
			}
			if ifc, isIfc := te.interfaces[targetName]; isIfc {
				if !te.TypeSatisfiesInterface(actualType, ifc, c.Span_) {
					return TypeUnknown
				}
			} else {
				if te.traitImpls[targetName] == nil || te.traitImpls[targetName][actualType.String()] == nil {
					te.addDiag(SeverityError, c.Span_, "type/constraint-violation",
						fmt.Sprintf("type parameter %q bound to %q does not satisfy trait %q", c.TypeParam, actualType.String(), targetName))
					return TypeUnknown
				}
			}
		}
	}

	substType := func(tr loneast.TYPE_REF) Type {
		if tr.Kind == loneast.TYPE_BASE {
			if tb, ok := tr.Value.(*loneast.TypeBase); ok {
				if tMapped, found := paramMap[tb.Name]; found {
					return tMapped
				}
			}
		}
		return te.resolveType(tr)
	}

	if tmpl.Decl.Kind == loneast.TYPE_DEC_ENUM {
		et := &EnumType{
			Name:       monoName,
			VariantMap: make(map[string]*EnumVariantInfo),
		}

		maxPayloadSize := 0
		maxAlign := 4 // tag is i32

		for tag, v := range tmpl.Decl.Variants {
			var fields []StructFieldInfo
			fieldMap := make(map[string]StructFieldInfo)
			vOffset := 4
			vMaxAlign := 1

			for _, f := range v.Fields {
				fType := substType(f.Type)
				sz := TypeSize(fType)
				al := TypeAlign(fType)
				if al > vMaxAlign {
					vMaxAlign = al
				}
				if al > maxAlign {
					maxAlign = al
				}
				if al > 0 && vOffset%al != 0 {
					vOffset += al - (vOffset % al)
				}
				info := StructFieldInfo{Name: f.Name, Type: fType, Offset: vOffset}
				fields = append(fields, info)
				fieldMap[f.Name] = info
				vOffset += sz
			}

			payloadSize := vOffset - 4
			if payloadSize > maxPayloadSize {
				maxPayloadSize = payloadSize
			}

			vInfo := EnumVariantInfo{
				Name:       v.Name,
				Tag:        tag,
				Fields:     fields,
				FieldMap:   fieldMap,
				TotalSize:  vOffset,
				TotalAlign: vMaxAlign,
			}
			et.Variants = append(et.Variants, vInfo)
		}

		for i := range et.Variants {
			et.VariantMap[et.Variants[i].Name] = &et.Variants[i]
		}

		if maxAlign > 0 && maxPayloadSize%maxAlign != 0 {
			maxPayloadSize += maxAlign - (maxPayloadSize % maxAlign)
		}
		et.TotalSize = 4 + maxPayloadSize
		et.TotalAlign = maxAlign

		te.enums[monoName] = et
		return et
	}

	if tmpl.Decl.Kind == loneast.TYPE_DEC_STRUCT {
		st := &StructType{
			Name:     monoName,
			FieldMap: make(map[string]StructFieldInfo),
			Methods:  make(map[string]*FnType),
		}

		offset := 0
		maxAlign := 1
		for _, f := range tmpl.Decl.Fields {
			fType := substType(f.Type)
			sz := TypeSize(fType)
			al := TypeAlign(fType)
			if al > maxAlign {
				maxAlign = al
			}
			if al > 0 && offset%al != 0 {
				offset += al - (offset % al)
			}
			info := StructFieldInfo{
				Name:       f.Name,
				Type:       fType,
				Offset:     offset,
				IsReadOnly: f.IsReadOnly,
				IsPrivate:  strings.HasPrefix(f.Name, "_"),
			}
			st.Fields = append(st.Fields, info)
			st.FieldMap[f.Name] = info
			offset += sz
		}

		if maxAlign > 0 && offset%maxAlign != 0 {
			offset += maxAlign - (offset % maxAlign)
		}
		st.TotalSize = offset
		st.TotalAlign = maxAlign

		te.structs[monoName] = st
		return st
	}

	return TypeUnknown
}

func (te *TypeEvaluator) monomorphizeFn(tmpl *GenericFnTemplate, args []TExpr) *TFnDec {
	paramMap := make(map[string]Type)
	var typeArgNames []string

	for _, p := range tmpl.TypeParams {
		var resolved Type = TypeUnknown
		for argIdx, param := range tmpl.Decl.Params {
			if param.Type.Kind == loneast.TYPE_BASE {
				if tb, ok := param.Type.Value.(*loneast.TypeBase); ok && tb.Name == p.Name {
					if argIdx < len(args) {
						resolved = args[argIdx].ExprType()
						break
					}
				}
			}
		}
		if resolved == TypeUnknown && p.Default != nil {
			resolved = te.resolveType(*p.Default)
		}
		paramMap[p.Name] = resolved
		typeArgNames = append(typeArgNames, resolved.String())
	}

	monoIdent := fmt.Sprintf("%s__%s", tmpl.Name, strings.Join(typeArgNames, "_"))

	if existing, ok := te.instantiatedFns[monoIdent]; ok {
		return existing
	}

	cloned := *tmpl.Decl
	cloned.Name = monoIdent
	cloned.TypeParams = nil

	var newParams []loneast.FnParam
	for _, p := range tmpl.Decl.Params {
		newP := p
		if p.Type.Kind == loneast.TYPE_BASE {
			if tb, ok := p.Type.Value.(*loneast.TypeBase); ok {
				if mappedT, ok := paramMap[tb.Name]; ok {
					newP.Type = loneast.TYPE_REF{
						Kind:  loneast.TYPE_BASE,
						Value: &loneast.TypeBase{Name: mappedT.String()},
						Span_: p.Span_,
					}
				}
			}
		}
		newParams = append(newParams, newP)
	}
	cloned.Params = newParams

	if tmpl.Decl.ReturnType != nil {
		rt := *tmpl.Decl.ReturnType
		if rt.Kind == loneast.TYPE_BASE {
			if tb, ok := rt.Value.(*loneast.TypeBase); ok {
				var newArgs []loneast.TYPE_REF
				for _, arg := range tb.Args {
					if arg.Kind == loneast.TYPE_BASE {
						if atb, ok := arg.Value.(*loneast.TypeBase); ok {
							if mappedT, ok := paramMap[atb.Name]; ok {
								newArgs = append(newArgs, loneast.TYPE_REF{
									Kind:  loneast.TYPE_BASE,
									Value: &loneast.TypeBase{Name: mappedT.String()},
									Span_: arg.Span_,
								})
								continue
							}
						}
					}
					newArgs = append(newArgs, arg)
				}
				newTb := *tb
				newTb.Args = newArgs
				if mappedT, ok := paramMap[tb.Name]; ok {
					newTb.Name = mappedT.String()
				}
				rt.Value = &newTb
			}
		}
		cloned.ReturnType = &rt
	}

	// Validate constraints
	for _, c := range tmpl.Constraints {
		actualType, exists := paramMap[c.TypeParam]
		if !exists || actualType == TypeUnknown {
			continue
		}
		switch c.Op {
		case loneast.ConstraintIs:
			targetType := te.resolveType(c.Target)
			targetName := ""
			if tb, ok := c.Target.Value.(*loneast.TypeBase); ok {
				targetName = tb.Name
			} else {
				targetName = targetType.String()
			}
			if targetName == "Error" {
				isErr := false
				if actualType.String() == "Error" || actualType == TypeInfer {
					isErr = true
				} else if _, isEnum := actualType.(*EnumType); isEnum {
					isErr = true
				}
				if !isErr {
					te.addDiag(SeverityError, c.Span_, "type/constraint-violation",
						fmt.Sprintf("type parameter %q bound to %q does not satisfy 'is Error'", c.TypeParam, actualType.String()))
					return nil
				}
			} else {
				if !actualType.AssignableTo(targetType) && !actualType.Equals(targetType) {
					te.addDiag(SeverityError, c.Span_, "type/constraint-violation",
						fmt.Sprintf("type parameter %q bound to %q does not satisfy 'is %s'", c.TypeParam, actualType.String(), targetType.String()))
					return nil
				}
			}

		case loneast.ConstraintSatisfies:
			targetName := ""
			if tb, ok := c.Target.Value.(*loneast.TypeBase); ok {
				targetName = tb.Name
			} else {
				targetName = te.resolveType(c.Target).String()
			}
			if ifc, isIfc := te.interfaces[targetName]; isIfc {
				if !te.TypeSatisfiesInterface(actualType, ifc, c.Span_) {
					return nil
				}
			} else {
				if te.traitImpls[targetName] == nil || te.traitImpls[targetName][actualType.String()] == nil {
					te.addDiag(SeverityError, c.Span_, "type/constraint-violation",
						fmt.Sprintf("type parameter %q bound to %q does not satisfy trait %q", c.TypeParam, actualType.String(), targetName))
					return nil
				}
			}
		}
	}

	oldParams := te.currentTypeParams
	te.currentTypeParams = paramMap
	defer func() { te.currentTypeParams = oldParams }()

	te.hoistFn(&cloned)
	tfn := te.analyzeFnDec(&cloned)
	te.instantiatedFns[monoIdent] = tfn
	te.extraDecls = append(te.extraDecls, tfn)
	return tfn
}

func (te *TypeEvaluator) evalTryExpr(t loneast.TryExpr) TExpr {
	sub := te.EvalExpr(t.Expr)
	et, ok := sub.ExprType().(*EnumType)
	if !ok {
		te.addDiag(SeverityError, t.Span_, "type/try-requires-result",
			fmt.Sprintf("'try' requires a Result/enum type, got %q", sub.ExprType().String()))
		return &TTryExpr{Expr: sub, OkType: TypeUnknown, Span_: t.Span_}
	}
	okV, hasOk := et.Variant("Ok")
	errV, hasErr := et.Variant("Err")
	if !hasOk || !hasErr {
		te.addDiag(SeverityError, t.Span_, "type/try-requires-ok-err",
			fmt.Sprintf("'try' requires enum %q to have 'Ok' and 'Err' variants", et.Name))
		return &TTryExpr{Expr: sub, OkType: TypeUnknown, Span_: t.Span_}
	}

	var subErrType Type = TypeVoid
	if len(errV.Fields) > 0 {
		subErrType = errV.Fields[0].Type
	}

	// Check current function return type
	if te.currentFnReturn != nil {
		currRetEnum, isRetEnum := te.currentFnReturn.(*EnumType)
		if !isRetEnum || currRetEnum.VariantMap["Ok"] == nil || currRetEnum.VariantMap["Err"] == nil {
			te.addDiag(SeverityError, t.Span_, "type/try-incompatible-return",
				fmt.Sprintf("cannot use 'try' in function returning %q; enclosing function must return a Result enum",
					te.currentFnReturn.String()))
		} else {
			// Check error compatibility
			currErrV, _ := currRetEnum.Variant("Err")
			var currErrType Type = TypeVoid
			if len(currErrV.Fields) > 0 {
				currErrType = currErrV.Fields[0].Type
			}

			if currErrType == TypeInfer {
				te.inferredErrors[te.currentFnName] = append(te.inferredErrors[te.currentFnName], subErrType)
			} else if currErrType.Equals(te.enums["Error"]) {
				// Error is the general unified error
			} else if !subErrType.AssignableTo(currErrType) {
				te.addDiag(SeverityError, t.Span_, "type/try-incompatible-error",
					fmt.Sprintf("cannot propagate error of type %q in function returning %q",
						subErrType.String(), currRetEnum.String()))
			}
		}
	}

	var okType Type = TypeVoid
	if len(okV.Fields) > 0 {
		okType = okV.Fields[0].Type
	}
	return &TTryExpr{
		Expr:       sub,
		ResultType: et,
		OkVariant:  okV,
		OkType:     okType,
		Span_:      t.Span_,
	}
}

func (te *TypeEvaluator) resolveType(tr loneast.TYPE_REF) Type {
	base := te.resolveBaseType(tr)
	if tr.Constrain.Optional {
		return &OptionalType{Inner: base}
	}
	return base
}

func (te *TypeEvaluator) resolveBaseType(tr loneast.TYPE_REF) Type {
	switch tr.Kind {
	case loneast.TYPE_LIST:
		if tl, ok := tr.Value.(*loneast.TypeList); ok {
			elemType := te.resolveType(tl.ItemType)
			size := 0
			if tl.Size != nil {
				if lit, ok := tl.Size.(loneast.Literal); ok && lit.Kind == loneast.LiteralInt {
					if v, ok := lit.Value.(int64); ok {
						size = int(v)
					}
				}
			}
			return &ListType{Elem: elemType, Fixed: tl.IsFixed(), Size: size}
		}
	case loneast.TYPE_FN:
		if tf, ok := tr.Value.(*loneast.TypeFn); ok {
			var params []FnParamInfo
			for _, p := range tf.Params {
				params = append(params, FnParamInfo{
					Name: p.Name,
					Type: te.resolveType(p.Type),
				})
			}
			retType := Type(TypeVoid)
			if tf.ReturnType != nil {
				retType = te.resolveType(*tf.ReturnType)
			}
			return &FnType{Params: params, Return: retType}
		}
	case loneast.TYPE_TUPLE:
		if tt, ok := tr.Value.(*loneast.TypeTuple); ok {
			var elemTypes []Type
			for _, elemTr := range tt.Types {
				elemTypes = append(elemTypes, te.resolveType(elemTr))
			}
			return NewTupleType(elemTypes)
		}
	case loneast.TYPE_BASE:
		if tb, ok := tr.Value.(*loneast.TypeBase); ok {
			// Handle pointer ptr<T>
			if tb.Name == "ptr" && len(tb.Args) > 0 {
				inner := te.resolveType(tb.Args[0])
				return &PtrType{Elem: inner, ReadOnly: tr.Constrain.ReadOnly}
			}
			// Handle reference ref<T>
			if tb.Name == "ref" && len(tb.Args) > 0 {
				inner := te.resolveType(tb.Args[0])
				return &RefType{Elem: inner, ReadOnly: tr.Constrain.ReadOnly}
			}
			// Handle infer
			if tb.Name == "infer" {
				return TypeInfer
			}
			// If type arguments are provided (e.g. Result<i32>), check generic templates first
			if len(tb.Args) > 0 {
				if tmpl, ok := te.genericTypes[tb.Name]; ok {
					return te.instantiateGenericType(tmpl, tb.Args)
				}
			}
			// Handle Self / self
			if (tb.Name == "Self" || tb.Name == "self") && te.currentSelfType != nil {
				return te.currentSelfType
			}
			// Primitive lookup
			if prim := PrimitiveByName(tb.Name); prim != nil {
				return prim
			}
			// Scope symbol lookup (struct / trait / enum)
			sym := te.stack.Lookup(tb.Name)
			if sym != nil && sym.Kind == SymbolType {
				return sym.Type
			}
			if et, ok := te.enums[tb.Name]; ok {
				return et
			}
			if st, ok := te.structs[tb.Name]; ok {
				return st
			}
			if tr, ok := te.traits[tb.Name]; ok {
				return tr
			}
			if ifc, ok := te.interfaces[tb.Name]; ok {
				return ifc
			}
			// Generic template with default arguments (e.g. Result with 0 args)
			if tmpl, ok := te.genericTypes[tb.Name]; ok {
				return te.instantiateGenericType(tmpl, tb.Args)
			}
			return &PrimitiveType{Name: tb.Name}
		}
	}
	return TypeUnknown
}

func isTrNaming(name string) bool {
	if len(name) >= 3 && name[:2] == "Tr" && unicode.IsUpper(rune(name[2])) {
		return true
	}
	return false
}

func isIfcNaming(name string) bool {
	if len(name) >= 4 && name[:3] == "Ifc" && unicode.IsUpper(rune(name[3])) {
		return true
	}
	return false
}

func (te *TypeEvaluator) TypeSatisfiesInterface(target Type, ifc *InterfaceType, span loneast.Span) bool {
	if _, isEnum := target.(*EnumType); isEnum {
		te.addDiag(SeverityError, span, "type/interface-enum-rejected",
			fmt.Sprintf("enum %q cannot satisfy interface %q", target.String(), ifc.Name))
		return false
	}

	st, isStruct := target.(*StructType)
	if !isStruct {
		te.addDiag(SeverityError, span, "type/interface-not-satisfied",
			fmt.Sprintf("type %q cannot satisfy interface %q", target.String(), ifc.Name))
		return false
	}

	for _, req := range ifc.Fields {
		actual, exists := st.Field(req.Name)
		if !exists {
			te.addDiag(SeverityError, span, "type/interface-missing-field",
				fmt.Sprintf("type %q does not satisfy interface %q: missing required field %q", st.Name, ifc.Name, req.Name))
			return false
		}
		if !actual.Type.AssignableTo(req.Type) {
			te.addDiag(SeverityError, span, "type/interface-field-type-mismatch",
				fmt.Sprintf("field %q on %q has type %q, but interface %q requires %q", req.Name, st.Name, actual.Type.String(), ifc.Name, req.Type.String()))
			return false
		}
		if !req.IsReadOnly && actual.IsReadOnly {
			te.addDiag(SeverityError, span, "type/field-mutability-mismatch",
				fmt.Sprintf("field %q on %q is read-only, but interface %q requires a mutable field", req.Name, st.Name, ifc.Name))
			return false
		}
	}
	return true
}

