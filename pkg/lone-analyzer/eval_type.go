package loneanalyzer

import (
	"fmt"
	ir "lone/pkg/lone-parser-ast"
	loneruntime "lone/pkg/lone-runtime"
)

// TypeEvaluator analyzes an AST, infers types, and produces a concrete Typed IR (TProgram).
type TypeEvaluator struct {
	stack                *ScopeStack
	diagnostics          []Diagnostic
	enforceConsumeOrDrop bool
	inlineWasmAuthorized bool
	currentSelf          Type
	modules              map[string]map[string]*Symbol            // moduleID -> symbolName -> Symbol
	methodImpls          map[string]map[string]*Symbol            // typeName -> methodName -> Symbol
	traitImpls           map[string]map[string]map[string]*Symbol // traitName -> targetTypeName -> methodName -> Symbol
}

// NewTypeEvaluator creates a new TypeEvaluator for a given file.
func NewTypeEvaluator(fileName string, enforceConsumeOrDrop bool) *TypeEvaluator {
	return &TypeEvaluator{
		stack:                NewScopeStack(fileName),
		diagnostics:          make([]Diagnostic, 0),
		enforceConsumeOrDrop: enforceConsumeOrDrop,
		modules:              make(map[string]map[string]*Symbol),
		methodImpls:          make(map[string]map[string]*Symbol),
		traitImpls:           make(map[string]map[string]map[string]*Symbol),
	}
}

// Diagnostics returns all diagnostics gathered during type evaluation.
func (te *TypeEvaluator) Diagnostics() []Diagnostic {
	return te.diagnostics
}

// Stack returns the ScopeStack used during type evaluation.
func (te *TypeEvaluator) Stack() *ScopeStack {
	return te.stack
}

// TraitImpls returns the map of trait implementations: traitName -> targetTypeName -> methodName -> Symbol.
func (te *TypeEvaluator) TraitImpls() map[string]map[string]map[string]*Symbol {
	return te.traitImpls
}

// MethodImpls returns the map of method implementations: typeName -> methodName -> Symbol.
func (te *TypeEvaluator) MethodImpls() map[string]map[string]*Symbol {
	return te.methodImpls
}

// Modules returns the map of loaded module symbols: moduleID -> symbolName -> Symbol.
func (te *TypeEvaluator) Modules() map[string]map[string]*Symbol {
	return te.modules
}

func (te *TypeEvaluator) addDiag(sev Severity, pos ir.Position, rule, msg string) {
	te.diagnostics = append(te.diagnostics, Diagnostic{
		Message:  msg,
		Severity: sev,
		Pos:      pos,
		Rule:     rule,
	})
}

func (te *TypeEvaluator) resolveTypeSymbol(name string) Type {
	if name == "Self" && te.currentSelf != nil {
		return te.currentSelf
	}
	sym := te.stack.Resolve(name)
	if sym != nil && sym.Kind == SymbolType {
		return sym.Type
	}
	return nil
}

func (te *TypeEvaluator) parseType(tr *ir.TYPE_REF) Type {
	return ParseTypeRefWithResolver(tr, te.resolveTypeSymbol, te.currentSelf)
}

func (te *TypeEvaluator) parseTypeWithSelf(tr *ir.TYPE_REF, selfType Type) Type {
	return ParseTypeRefWithResolver(tr, te.resolveTypeSymbol, selfType)
}

func (te *TypeEvaluator) parseHostTypeString(s string) Type {
	t := ParseTypeString(s)
	return te.bindCustomTypes(t)
}

func (te *TypeEvaluator) bindCustomTypes(t Type) Type {
	switch v := t.(type) {
	case *CustomType:
		if resolved := te.resolveTypeSymbol(v.Name); resolved != nil {
			return resolved
		}
	case *PtrType:
		v.Elem = te.bindCustomTypes(v.Elem)
	case *RefType:
		v.Elem = te.bindCustomTypes(v.Elem)
	case *ListType:
		v.ItemType = te.bindCustomTypes(v.ItemType)
	}
	return t
}

// Evaluate performs type analysis over top-level AST declarations.
func (te *TypeEvaluator) Evaluate(rawDecls []any) *TProgram {
	decls := toDeclSlice(rawDecls)
	program := &TProgram{
		Decls: make([]TNode, 0),
	}

	var entryDecls []any
	for _, raw := range decls {
		unwrapped := unwrapTuple(raw)
		switch m := unwrapped.(type) {
		case ir.ModuleScopeDecl:
			te.evaluateModuleScope(&m, program)
		case *ir.ModuleScopeDecl:
			te.evaluateModuleScope(m, program)
		default:
			entryDecls = append(entryDecls, raw)
		}
	}

	te.evaluateDeclsInCurrentScope(entryDecls, program)
	return program
}

func unwrapTuple(node any) any {
	if tuple, ok := node.([]any); ok && len(tuple) == 2 {
		return tuple[1]
	}
	return node
}

func (te *TypeEvaluator) evaluateModuleScope(mod *ir.ModuleScopeDecl, program *TProgram) {
	prevStack := te.stack
	prevWasm := te.inlineWasmAuthorized

	te.stack = NewScopeStack(mod.FilePath)
	te.inlineWasmAuthorized = false

	te.evaluateDeclsInCurrentScope(mod.Decls, program)

	exports := make(map[string]*Symbol)
	for symName := range mod.Exports {
		if sym := te.stack.Root().LookupLocal(symName); sym != nil {
			exports[symName] = sym
		}
	}
	te.modules[mod.ModuleID] = exports

	te.stack = prevStack
	te.inlineWasmAuthorized = prevWasm
}

func (te *TypeEvaluator) evaluateDeclsInCurrentScope(decls []any, program *TProgram) {
	// Pass 2a-1: Pre-declare type names & imported symbols so cross-referencing types resolve
	for _, raw := range decls {
		te.predeclareTypesAndImports(unwrapTuple(raw))
	}

	// Pass 2a-2: Populate struct fields/layouts, trait signatures, impls, functions, and globals
	for _, raw := range decls {
		te.hoistTopLevel(unwrapTuple(raw))
	}

	// Pass 2b: Analyze bodies & expressions
	te.inlineWasmAuthorized = false
	for _, raw := range decls {
		nodes := te.analyzeTopLevel(unwrapTuple(raw))
		program.Decls = append(program.Decls, nodes...)
	}
}

func (te *TypeEvaluator) predeclareTypesAndImports(node any) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case ir.ResolvedImportDecl:
		te.bindResolvedImport(&n)
	case *ir.ResolvedImportDecl:
		te.bindResolvedImport(n)
	case ir.TYPE_DEC:
		te.predeclareTypeDec(&n)
	case *ir.TYPE_DEC:
		te.predeclareTypeDec(n)
	}
}

func (te *TypeEvaluator) bindResolvedImport(imp *ir.ResolvedImportDecl) {
	modExports := te.modules[imp.ModuleID]
	if imp.Alias != "" {
		exportsCopy := make(map[string]*Symbol)
		for k, v := range modExports {
			exportsCopy[k] = v
		}
		_ = te.stack.Define(&Symbol{
			Name: imp.Alias,
			Kind: SymbolVar,
			Type: &ModuleAliasType{
				Alias:   imp.Alias,
				Source:  imp.Source,
				Exports: exportsCopy,
			},
			Node: imp,
		})
		return
	}

	for _, symName := range imp.Symbols {
		if modExports == nil {
			continue
		}
		orig, ok := modExports[symName]
		if !ok || orig == nil {
			continue
		}
		copied := *orig
		copied.Name = symName
		if orig.MangledName == "" {
			copied.MangledName = orig.Name
		}
		_ = te.stack.Define(&copied)
	}
}

func (te *TypeEvaluator) predeclareTypeDec(td *ir.TYPE_DEC) {
	switch td.Kind {
	case ir.TYPE_DEC_STRUCT:
		st := &StructType{
			Name:    td.Name,
			Methods: make(map[string]*FnType),
		}
		_ = te.stack.Define(&Symbol{
			Name: td.Name,
			Kind: SymbolType,
			Type: st,
			Node: td,
		})
	case ir.TYPE_DEC_TRAIT:
		tr := &TraitType{
			Name:       td.Name,
			TypeParams: td.TypeParams,
			Methods:    make(map[string]*FnType),
		}
		_ = te.stack.Define(&Symbol{
			Name: td.Name,
			Kind: SymbolType,
			Type: tr,
			Node: td,
		})
	case ir.TYPE_DEC_ALIAS:
		_ = te.stack.Define(&Symbol{
			Name: td.Name,
			Kind: SymbolType,
			Type: &CustomType{Name: td.Name},
			Node: td,
		})
	}
}

func (te *TypeEvaluator) hoistTopLevel(node any) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case ir.WasmDirective:
		if n.Inline {
			te.inlineWasmAuthorized = true
		} else {
			for _, imp := range n.Imports {
				te.hoistRawImport(&imp, true)
			}
		}
	case *ir.WasmDirective:
		if n.Inline {
			te.inlineWasmAuthorized = true
		} else {
			for _, imp := range n.Imports {
				te.hoistRawImport(&imp, true)
			}
		}
	case ir.ImportDecl:
		te.hoistRawImport(&n, te.inlineWasmAuthorized)
	case *ir.ImportDecl:
		te.hoistRawImport(n, te.inlineWasmAuthorized)
	case ir.HostImportDecl:
		te.hoistHostImport(&n)
	case *ir.HostImportDecl:
		te.hoistHostImport(n)
	case ir.TYPE_DEC:
		te.hoistTypeDec(&n)
	case *ir.TYPE_DEC:
		te.hoistTypeDec(n)
	case ir.IMPL_DEC:
		te.hoistImplDec(&n)
	case *ir.IMPL_DEC:
		te.hoistImplDec(n)
	case ir.FN_DEC:
		te.hoistFn(&n)
	case *ir.FN_DEC:
		te.hoistFn(n)
	case ir.BIND_DEC:
		te.hoistVar(&n)
	case *ir.BIND_DEC:
		te.hoistVar(n)
	}
}

func (te *TypeEvaluator) hoistRawImport(imp *ir.ImportDecl, wasmAuthorized bool) {
	if !loneruntime.IsHostNamespace(imp.Source) {
		return
	}
	if !wasmAuthorized {
		return
	}
	ns, ok := loneruntime.LookupHostNamespace(imp.Source)
	if !ok {
		return
	}
	if imp.Alias != "" {
		exports := make(map[string]*Symbol)
		for symName, spec := range ns {
			fnType := te.hostSpecToFnType(spec)
			exports[symName] = &Symbol{
				Name:        symName,
				MangledName: spec.Name,
				Kind:        SymbolFn,
				Type:        fnType,
			}
		}
		_ = te.stack.Define(&Symbol{
			Name: imp.Alias,
			Kind: SymbolVar,
			Type: &ModuleAliasType{
				Alias:   imp.Alias,
				Source:  imp.Source,
				Exports: exports,
			},
			Node: imp,
		})
		return
	}
	for _, sym := range imp.Symbols {
		if spec, ok := ns[sym]; ok {
			_ = te.stack.Define(&Symbol{
				Name:        sym,
				MangledName: spec.Name,
				Kind:        SymbolFn,
				Type:        te.hostSpecToFnType(spec),
				Node:        imp,
			})
		}
	}
}

func (te *TypeEvaluator) hostSpecToFnType(spec loneruntime.HostFuncSpec) *FnType {
	var params []FnParamType
	for _, p := range spec.Params {
		params = append(params, FnParamType{
			Name: p.Name,
			Type: te.parseHostTypeString(p.LoneType),
		})
	}
	return &FnType{
		Params:     params,
		ReturnType: te.parseHostTypeString(spec.ReturnLoneType),
	}
}

func (te *TypeEvaluator) hoistHostImport(h *ir.HostImportDecl) {
	var params []FnParamType
	for _, p := range h.Params {
		params = append(params, FnParamType{
			Name: p.Name,
			Type: te.parseHostTypeString(p.LoneType),
		})
	}
	fnType := &FnType{
		Params:     params,
		ReturnType: te.parseHostTypeString(h.ReturnLoneType),
	}
	_ = te.stack.Define(&Symbol{
		Name:        h.LocalName,
		MangledName: h.LocalName,
		Kind:        SymbolFn,
		Type:        fnType,
		Node:        h,
	})
}

func (te *TypeEvaluator) hoistTypeDec(td *ir.TYPE_DEC) {
	sym := te.stack.Resolve(td.Name)
	if sym == nil {
		return
	}

	switch td.Kind {
	case ir.TYPE_DEC_ALIAS:
		if td.AliasType != nil {
			sym.Type = te.parseType(td.AliasType)
		}

	case ir.TYPE_DEC_STRUCT:
		st, ok := sym.Type.(*StructType)
		if !ok {
			return
		}
		st.Fields = make([]StructFieldInfo, 0, len(td.Fields))
		for _, f := range td.Fields {
			fType := te.parseTypeWithSelf(&f.Type, st)
			st.Fields = append(st.Fields, StructFieldInfo{
				Name: f.Name,
				Type: fType,
			})
		}
		st.ComputeLayout()

		if te.methodImpls[st.Name] == nil {
			te.methodImpls[st.Name] = make(map[string]*Symbol)
		}
		for i := range td.Methods {
			m := &td.Methods[i]
			fnType := te.buildFnType(m.Params, m.ReturnType, st)
			st.Methods[m.Name] = fnType
			mangled := fmt.Sprintf("%s__%s", st.Name, m.Name)
			mSym := &Symbol{
				Name:        m.Name,
				MangledName: mangled,
				Kind:        SymbolFn,
				Type:        fnType,
				Node:        m,
			}
			te.methodImpls[st.Name][m.Name] = mSym
		}

	case ir.TYPE_DEC_TRAIT:
		tr, ok := sym.Type.(*TraitType)
		if !ok {
			return
		}
		selfPlaceholder := &CustomType{Name: "Self"}
		for _, sig := range td.TraitSigs {
			fnType := te.buildFnType(sig.Params, sig.ReturnType, selfPlaceholder)
			tr.Methods[sig.Name] = fnType
		}
	}
}

func (te *TypeEvaluator) hoistImplDec(im *ir.IMPL_DEC) {
	targetType := te.parseType(&im.TargetType)
	targetName := targetType.String()

	if te.methodImpls[targetName] == nil {
		te.methodImpls[targetName] = make(map[string]*Symbol)
	}

	if im.TraitName == "" {
		// Inherent impl block: impl Target { ... }
		for i := range im.Methods {
			m := &im.Methods[i]
			fnType := te.buildFnType(m.Params, m.ReturnType, targetType)
			mangled := fmt.Sprintf("%s__%s", targetName, m.Name)
			mSym := &Symbol{
				Name:        m.Name,
				MangledName: mangled,
				Kind:        SymbolFn,
				Type:        fnType,
				Node:        m,
			}
			te.methodImpls[targetName][m.Name] = mSym
			if st, ok := targetType.(*StructType); ok {
				st.Methods[m.Name] = fnType
			}
		}
		return
	}

	// Trait impl block: impl Trait on Target { ... }
	traitSym := te.stack.Resolve(im.TraitName)
	if traitSym == nil || traitSym.Kind != SymbolType {
		te.addDiag(SeverityError, im.Pos, "type/undefined-trait",
			fmt.Sprintf("undefined trait %q in impl block", im.TraitName))
		return
	}
	traitType, ok := traitSym.Type.(*TraitType)
	if !ok {
		te.addDiag(SeverityError, im.Pos, "type/not-a-trait",
			fmt.Sprintf("type %q is not a trait", im.TraitName))
		return
	}

	if te.traitImpls[traitType.Name] == nil {
		te.traitImpls[traitType.Name] = make(map[string]map[string]*Symbol)
	}
	if te.traitImpls[traitType.Name][targetName] == nil {
		te.traitImpls[traitType.Name][targetName] = make(map[string]*Symbol)
	}

	implemented := make(map[string]bool)
	for i := range im.Methods {
		m := &im.Methods[i]
		if _, exists := traitType.Methods[m.Name]; !exists {
			te.addDiag(SeverityError, m.Pos, "type/unknown-trait-method",
				fmt.Sprintf("method %q is not part of trait %q", m.Name, traitType.Name))
			continue
		}
		implemented[m.Name] = true
		fnType := te.buildFnType(m.Params, m.ReturnType, targetType)
		mangled := fmt.Sprintf("%s__%s__%s", traitType.Name, targetName, m.Name)
		mSym := &Symbol{
			Name:        m.Name,
			MangledName: mangled,
			Kind:        SymbolFn,
			Type:        fnType,
			Node:        m,
		}
		te.traitImpls[traitType.Name][targetName][m.Name] = mSym
		if _, hasInherent := te.methodImpls[targetName][m.Name]; !hasInherent {
			te.methodImpls[targetName][m.Name] = mSym
		}
	}

	for reqMethod := range traitType.Methods {
		if !implemented[reqMethod] {
			te.addDiag(SeverityError, im.Pos, "type/incomplete-trait-impl",
				fmt.Sprintf("impl of trait %q on %q is missing method %q", traitType.Name, targetName, reqMethod))
		}
	}
}

func (te *TypeEvaluator) buildFnType(rawParams []ir.FnParam, rawRet *ir.TYPE_REF, selfType Type) *FnType {
	var params []FnParamType
	for _, p := range rawParams {
		params = append(params, FnParamType{
			Name: p.Name,
			Type: te.parseTypeWithSelf(&p.Type, selfType),
		})
	}
	var retType Type = TypeVoid
	if rawRet != nil {
		retType = te.parseTypeWithSelf(rawRet, selfType)
	}
	return &FnType{
		Params:     params,
		ReturnType: retType,
	}
}

func (te *TypeEvaluator) hoistFn(fn *ir.FN_DEC) {
	fnType := te.buildFnType(fn.Params, fn.ReturnType, nil)
	sym := &Symbol{
		Name:        fn.Name,
		MangledName: fn.Name,
		Kind:        SymbolFn,
		Type:        fnType,
		Node:        fn,
	}
	_ = te.stack.Define(sym)
}

func (te *TypeEvaluator) hoistVar(b *ir.BIND_DEC) {
	declaredType := te.parseType(&b.Type)
	sym := &Symbol{
		Name: b.Name,
		Kind: SymbolVar,
		Type: declaredType,
		Node: b,
	}
	_ = te.stack.Define(sym)
}

func (te *TypeEvaluator) analyzeTopLevel(node any) []TNode {
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case ir.WasmDirective:
		return te.analyzeWasmDirective(&n)
	case *ir.WasmDirective:
		return te.analyzeWasmDirective(n)

	case ir.ImportDecl:
		return te.analyzeRawImport(&n)
	case *ir.ImportDecl:
		return te.analyzeRawImport(n)

	case ir.ResolvedImportDecl, *ir.ResolvedImportDecl:
		return nil

	case ir.HostImportDecl:
		return []TNode{te.analyzeHostImport(&n)}
	case *ir.HostImportDecl:
		return []TNode{te.analyzeHostImport(n)}

	case ir.TYPE_DEC:
		return te.analyzeTypeDec(&n)
	case *ir.TYPE_DEC:
		return te.analyzeTypeDec(n)

	case ir.IMPL_DEC:
		return te.analyzeImplDec(&n)
	case *ir.IMPL_DEC:
		return te.analyzeImplDec(n)

	case ir.FN_DEC:
		return []TNode{te.analyzeFnDecWithMangled(&n, n.Name, nil)}
	case *ir.FN_DEC:
		return []TNode{te.analyzeFnDecWithMangled(n, n.Name, nil)}

	case ir.BIND_DEC:
		return []TNode{te.analyzeVarDec(&n)}
	case *ir.BIND_DEC:
		return []TNode{te.analyzeVarDec(n)}

	case ir.DropStmt:
		tExpr := te.EvalExpr(n.Expr)
		return []TNode{&TDropStmt{Expr: tExpr, Pos_: n.Pos}}
	case *ir.DropStmt:
		tExpr := te.EvalExpr(n.Expr)
		return []TNode{&TDropStmt{Expr: tExpr, Pos_: n.Pos}}

	default:
		if expr, ok := node.(ir.Expr); ok {
			tExpr := te.EvalExpr(expr)
			if te.enforceConsumeOrDrop {
				typ := tExpr.ExprType()
				if typ != TypeVoid && typ != TypeNone && typ != TypeUnknown {
					te.addDiag(SeverityError, tExpr.Pos(), "type/unused-result",
						fmt.Sprintf("result of expression of type %q is not consumed; use 'drop' to discard it", typ.String()))
				}
			}
			return []TNode{&TExprStmt{Expr: tExpr, Pos_: tExpr.Pos()}}
		}
	}
	return nil
}

func (te *TypeEvaluator) analyzeWasmDirective(wd *ir.WasmDirective) []TNode {
	if wd.Inline {
		te.inlineWasmAuthorized = true
		return nil
	}
	var nodes []TNode
	for _, imp := range wd.Imports {
		nodes = append(nodes, te.analyzeAuthorizedHostImport(&imp)...)
	}
	return nodes
}

func (te *TypeEvaluator) analyzeRawImport(imp *ir.ImportDecl) []TNode {
	if loneruntime.IsHostNamespace(imp.Source) {
		if !te.inlineWasmAuthorized {
			te.addDiag(SeverityError, imp.Pos, "capability/unauthorized-host-import",
				fmt.Sprintf("unauthorized host capability import from %q: wrap host imports in an explicit @fa:wasm directive", imp.Source))
			return nil
		}
		return te.analyzeAuthorizedHostImport(imp)
	}
	return []TNode{&TImportDecl{
		Symbols:     imp.Symbols,
		Source:      imp.Source,
		IsNamespace: imp.IsNamespace,
		Alias:       imp.Alias,
		Pos_:        imp.Pos,
	}}
}

func (te *TypeEvaluator) analyzeAuthorizedHostImport(imp *ir.ImportDecl) []TNode {
	if !loneruntime.IsHostNamespace(imp.Source) {
		return []TNode{&TImportDecl{
			Symbols:     imp.Symbols,
			Source:      imp.Source,
			IsNamespace: imp.IsNamespace,
			Alias:       imp.Alias,
			Pos_:        imp.Pos,
		}}
	}
	ns, ok := loneruntime.LookupHostNamespace(imp.Source)
	if !ok {
		te.addDiag(SeverityError, imp.Pos, "import/unknown-host-namespace",
			fmt.Sprintf("unknown host capability namespace %q", imp.Source))
		return nil
	}
	var out []TNode
	if imp.Alias != "" {
		for symName, spec := range ns {
			out = append(out, te.hostSpecToTHostImport(spec, symName, imp.Pos))
		}
		return out
	}
	for _, sym := range imp.Symbols {
		spec, ok := ns[sym]
		if !ok {
			te.addDiag(SeverityError, imp.Pos, "import/undefined-symbol",
				fmt.Sprintf("host capability %q does not exist in namespace %q", sym, imp.Source))
			continue
		}
		out = append(out, te.hostSpecToTHostImport(spec, sym, imp.Pos))
	}
	return out
}

func (te *TypeEvaluator) hostSpecToTHostImport(spec loneruntime.HostFuncSpec, localName string, pos ir.Position) *THostImport {
	var tParams []TFnParam
	var wasmParams []string
	for _, p := range spec.Params {
		tParams = append(tParams, TFnParam{
			Name: p.Name,
			Type: te.parseHostTypeString(p.LoneType),
			Pos_: pos,
		})
		wasmParams = append(wasmParams, p.WasmType)
	}
	return &THostImport{
		Namespace:      spec.Namespace,
		WasmModule:     spec.WasmModule,
		WasmName:       spec.Name,
		LocalName:      localName,
		Params:         tParams,
		ReturnType:     te.parseHostTypeString(spec.ReturnLoneType),
		WasmParams:     wasmParams,
		ReturnWasmType: spec.ReturnWasmType,
		Pos_:           pos,
	}
}

func (te *TypeEvaluator) analyzeHostImport(h *ir.HostImportDecl) *THostImport {
	var tParams []TFnParam
	var wasmParams []string
	for _, p := range h.Params {
		tParams = append(tParams, TFnParam{
			Name: p.Name,
			Type: te.parseHostTypeString(p.LoneType),
			Pos_: h.Pos,
		})
		wasmParams = append(wasmParams, p.WasmType)
	}
	return &THostImport{
		Namespace:      h.Namespace,
		WasmModule:     h.WasmModule,
		WasmName:       h.WasmName,
		LocalName:      h.LocalName,
		Params:         tParams,
		ReturnType:     te.parseHostTypeString(h.ReturnLoneType),
		WasmParams:     wasmParams,
		ReturnWasmType: h.ReturnWasmType,
		Pos_:           h.Pos,
	}
}

func (te *TypeEvaluator) analyzeTypeDec(td *ir.TYPE_DEC) []TNode {
	sym := te.stack.Resolve(td.Name)
	var typ Type = TypeUnknown
	if sym != nil {
		typ = sym.Type
	}
	nodes := []TNode{&TTypeDec{
		Name: td.Name,
		Typ:  typ,
		Pos_: td.Pos,
	}}

	if td.Kind == ir.TYPE_DEC_STRUCT {
		for i := range td.Methods {
			m := &td.Methods[i]
			mangled := fmt.Sprintf("%s__%s", td.Name, m.Name)
			nodes = append(nodes, te.analyzeFnDecWithMangled(m, mangled, typ))
		}
	}
	return nodes
}

func (te *TypeEvaluator) analyzeImplDec(im *ir.IMPL_DEC) []TNode {
	targetType := te.parseType(&im.TargetType)
	targetName := targetType.String()

	var nodes []TNode
	for i := range im.Methods {
		m := &im.Methods[i]
		mangled := fmt.Sprintf("%s__%s", targetName, m.Name)
		if im.TraitName != "" {
			mangled = fmt.Sprintf("%s__%s__%s", im.TraitName, targetName, m.Name)
		}
		nodes = append(nodes, te.analyzeFnDecWithMangled(m, mangled, targetType))
	}
	return nodes
}

func (te *TypeEvaluator) analyzeFnDecWithMangled(fn *ir.FN_DEC, loweredName string, selfType Type) *TFnDec {
	prevSelf := te.currentSelf
	te.currentSelf = selfType
	defer func() { te.currentSelf = prevSelf }()

	var retType Type = TypeVoid
	if fn.ReturnType != nil {
		retType = te.parseType(fn.ReturnType)
	}

	te.stack.Push(ScopeFunction, loweredName)
	defer te.stack.Pop()

	tParams := make([]TFnParam, len(fn.Params))
	for i, p := range fn.Params {
		pType := te.parseType(&p.Type)
		tParams[i] = TFnParam{
			Name: p.Name,
			Type: pType,
			Pos_: p.Type.Pos,
		}
		_ = te.stack.Define(&Symbol{
			Name:     p.Name,
			Kind:     SymbolVar,
			Type:     pType,
			Node:     p,
			DeclNode: &tParams[i],
		})
	}

	hasReturn := false
	tBody := te.analyzeBlock(&fn.Body, retType, &hasReturn)

	if retType != TypeVoid && retType != TypeNone && retType != TypeUnknown && !hasReturn {
		te.addDiag(SeverityError, fn.Pos, "type/missing-return",
			fmt.Sprintf("function %q expects return type %q but contains no return statement", fn.Name, retType.String()))
	}

	return &TFnDec{
		Name:       loweredName,
		Params:     tParams,
		ReturnType: retType,
		Body:       tBody,
		Pos_:       fn.Pos,
	}
}

func (te *TypeEvaluator) analyzeVarDec(b *ir.BIND_DEC) *TVarDec {
	declaredType := te.parseType(&b.Type)
	tVal := te.EvalExpr(b.Value)

	isUninit := false
	if lit, ok := tVal.(*TLiteralExpr); ok && lit.Kind == ir.LITERAL_UNINITIALIZED {
		isUninit = true
	}

	resolvedType := declaredType
	if declaredType == TypeInfer || declaredType == TypeUnknown {
		resolvedType = tVal.ExprType()
	} else if isUninit {
		if lit, ok := tVal.(*TLiteralExpr); ok {
			lit.Typ = declaredType
		}
	} else if !tVal.ExprType().AssignableTo(declaredType) {
		te.addDiag(SeverityError, b.Pos, "type/assignment-mismatch",
			fmt.Sprintf("cannot assign value of type %q to variable %q of type %q",
				tVal.ExprType().String(), b.Name, declaredType.String()))
	}

	// If an anonymous object literal `{ .a: 1 }` is assigned to a declared StructType, upgrade it to TStructLitExpr
	if st, ok := resolvedType.(*StructType); ok {
		if objExpr, ok := tVal.(*TObjectExpr); ok {
			tVal = &TStructLitExpr{
				StructType: st,
				Fields:     objExpr.Fields,
				Pos_:       objExpr.Pos_,
			}
		}
	}

	tv := &TVarDec{
		Name:  b.Name,
		Type:  resolvedType,
		Value: tVal,
		Pos_:  b.Pos,
	}

	// Update symbol in current scope
	if sym := te.stack.Current().LookupLocal(b.Name); sym != nil {
		sym.Type = resolvedType
		sym.DeclNode = tv
	} else {
		_ = te.stack.Define(&Symbol{
			Name:     b.Name,
			Kind:     SymbolVar,
			Type:     resolvedType,
			Node:     b,
			DeclNode: tv,
		})
	}

	return tv
}

func (te *TypeEvaluator) analyzeBlock(b *ir.Block, expectedReturn Type, hasReturn *bool) TBlock {
	te.stack.Push(ScopeBlock, "")
	defer te.stack.Pop()

	tBlock := TBlock{
		Stmts: make([]TStmt, 0),
		Pos_:  b.Pos,
		Typ:   TypeVoid,
	}

	for _, rawStmt := range b.Stmts {
		stmt := te.analyzeStmt(rawStmt, expectedReturn, hasReturn)
		if stmt != nil {
			tBlock.Stmts = append(tBlock.Stmts, stmt)
			if brk, ok := stmt.(*TBreakStmt); ok && brk.Value != nil {
				tBlock.Typ = brk.Value.ExprType()
			}
		}
	}

	return tBlock
}

func (te *TypeEvaluator) analyzeStmt(raw any, expectedReturn Type, hasReturn *bool) TStmt {
	if raw == nil {
		return nil
	}

	switch s := raw.(type) {
	case ir.BIND_DEC:
		return te.analyzeVarDec(&s)
	case *ir.BIND_DEC:
		return te.analyzeVarDec(s)

	case ir.AssignStmt:
		return te.analyzeAssignStmt(&s)
	case *ir.AssignStmt:
		return te.analyzeAssignStmt(s)

	case ir.ReturnStmt:
		return te.analyzeReturnStmt(&s, expectedReturn, hasReturn)
	case *ir.ReturnStmt:
		return te.analyzeReturnStmt(s, expectedReturn, hasReturn)

	case ir.DropStmt:
		tExpr := te.EvalExpr(s.Expr)
		return &TDropStmt{Expr: tExpr, Pos_: s.Pos}
	case *ir.DropStmt:
		tExpr := te.EvalExpr(s.Expr)
		return &TDropStmt{Expr: tExpr, Pos_: s.Pos}

	case ir.BreakStmt:
		var tVal TExpr
		if s.Value != nil {
			tVal = te.EvalExpr(s.Value)
		}
		return &TBreakStmt{Label: s.Label, Value: tVal, Pos_: s.Pos}
	case *ir.BreakStmt:
		var tVal TExpr
		if s.Value != nil {
			tVal = te.EvalExpr(s.Value)
		}
		return &TBreakStmt{Label: s.Label, Value: tVal, Pos_: s.Pos}

	case ir.IfStmt:
		tCond := te.EvalExpr(s.Cond)
		tThen := te.analyzeBlock(&s.Then, expectedReturn, hasReturn)
		var tElse *TBlock
		if s.Else != nil {
			if elseBlock, ok := s.Else.(ir.Block); ok {
				eb := te.analyzeBlock(&elseBlock, expectedReturn, hasReturn)
				tElse = &eb
			} else if ebPtr, ok := s.Else.(*ir.Block); ok && ebPtr != nil {
				eb := te.analyzeBlock(ebPtr, expectedReturn, hasReturn)
				tElse = &eb
			} else if elseIf, ok := s.Else.(ir.IfStmt); ok {
				stmt := te.analyzeStmt(elseIf, expectedReturn, hasReturn)
				if ts, ok := stmt.(TStmt); ok {
					var typ Type = TypeVoid
					if tIf, ok := ts.(*TIfStmt); ok {
						typ = tIf.Typ
					}
					eb := TBlock{Stmts: []TStmt{ts}, Pos_: elseIf.Pos, Typ: typ}
					tElse = &eb
				}
			} else if elseIfPtr, ok := s.Else.(*ir.IfStmt); ok && elseIfPtr != nil {
				stmt := te.analyzeStmt(*elseIfPtr, expectedReturn, hasReturn)
				if ts, ok := stmt.(TStmt); ok {
					var typ Type = TypeVoid
					if tIf, ok := ts.(*TIfStmt); ok {
						typ = tIf.Typ
					}
					eb := TBlock{Stmts: []TStmt{ts}, Pos_: elseIfPtr.Pos, Typ: typ}
					tElse = &eb
				}
			}
		}
		resTyp := tThen.Typ
		if tElse != nil && tElse.Typ != TypeVoid && resTyp == TypeVoid {
			resTyp = tElse.Typ
		}
		return &TIfStmt{Cond: tCond, Then: tThen, Else: tElse, Pos_: s.Pos, Typ: resTyp}

	case ir.WhileStmt:
		tCond := te.EvalExpr(s.Cond)
		tBody := te.analyzeBlock(&s.Body, expectedReturn, hasReturn)
		return &TWhileStmt{Cond: tCond, Body: tBody, Pos_: s.Pos}
	case *ir.WhileStmt:
		tCond := te.EvalExpr(s.Cond)
		tBody := te.analyzeBlock(&s.Body, expectedReturn, hasReturn)
		return &TWhileStmt{Cond: tCond, Body: tBody, Pos_: s.Pos}

	case ir.ForStmt:
		return te.analyzeForStmt(&s, expectedReturn, hasReturn)
	case *ir.ForStmt:
		return te.analyzeForStmt(s, expectedReturn, hasReturn)

	case ir.LabelDec:
		tBody := te.analyzeBlock(&s.Body, expectedReturn, hasReturn)
		return &TLabelDec{Name: s.Name, Body: tBody, Pos_: s.Pos, Typ: TypeVoid}

	default:
		if expr, ok := raw.(ir.Expr); ok {
			tExpr := te.EvalExpr(expr)
			if te.enforceConsumeOrDrop {
				typ := tExpr.ExprType()
				if typ != TypeVoid && typ != TypeNone && typ != TypeUnknown {
					te.addDiag(SeverityError, tExpr.Pos(), "type/unused-result",
						fmt.Sprintf("result of expression of type %q is not consumed; use 'drop' to discard it", typ.String()))
				}
			}
			return &TExprStmt{Expr: tExpr, Pos_: tExpr.Pos()}
		}
	}

	return nil
}

func (te *TypeEvaluator) analyzeAssignStmt(a *ir.AssignStmt) *TAssignStmt {
	tTarget := te.EvalExpr(a.Target)
	tVal := te.EvalExpr(a.Value)

	if !te.isLValue(tTarget) {
		te.addDiag(SeverityError, a.Pos, "type/invalid-lvalue",
			"left-hand side of assignment must be a variable, dereference, field, or index expression")
	} else if te.isReadOnlyTarget(tTarget) {
		te.addDiag(SeverityError, a.Pos, "type/readonly-mutation",
			"cannot mutate value through a read-only (ro) reference or pointer")
	}

	if tTarget.ExprType() != TypeUnknown && tVal.ExprType() != TypeUnknown {
		if !tVal.ExprType().AssignableTo(tTarget.ExprType()) {
			te.addDiag(SeverityError, a.Pos, "type/assignment-mismatch",
				fmt.Sprintf("cannot assign value of type %q to target of type %q",
					tVal.ExprType().String(), tTarget.ExprType().String()))
		}
	}

	return &TAssignStmt{
		Target: tTarget,
		Value:  tVal,
		Pos_:   a.Pos,
	}
}

func (te *TypeEvaluator) analyzeForStmt(f *ir.ForStmt, expectedReturn Type, hasReturn *bool) TStmt {
	tIter := te.EvalExpr(f.Iter)
	iterType := tIter.ExprType()

	st := unwrapStructType(iterType)
	var iterExpr ir.Expr = f.Iter

	if st != nil {
		hasIterMethod := te.lookupMethod(st.Name, "iterator") != nil
		hasDirectNext := te.lookupMethod(st.Name, "has_next") != nil && te.lookupMethod(st.Name, "next") != nil

		if hasIterMethod {
			iterExpr = ir.PostFixExpr{
				Base: f.Iter,
				Suffix: []ir.Suffix{
					ir.SuffixMemberAccess{Name: "iterator", Pos: f.Pos},
					ir.SuffixCall{Args: nil, Pos: f.Pos},
				},
			}
		} else if !hasDirectNext && iterType != TypeUnknown && iterType != TypeAny {
			te.addDiag(SeverityError, tIter.Pos(), "type/not-iterable",
				fmt.Sprintf("type %q is not iterable (missing 'iterator()' or 'has_next()'/'next()' methods)", st.Name))
		}
	} else if iterType != TypeUnknown && iterType != TypeAny {
		te.addDiag(SeverityError, tIter.Pos(), "type/not-iterable",
			fmt.Sprintf("type %q is not iterable (must implement IteratorTrait or IterableTrait)", iterType.String()))
	}

	itVarName := fmt.Sprintf("__it_%d_%d", f.Pos.Line, f.Pos.Col)

	bindIt := ir.BIND_DEC{
		Name:  itVarName,
		Value: iterExpr,
		Pos:   f.Pos,
	}

	condExpr := ir.PostFixExpr{
		Base: ir.NameRef{Name: itVarName, Pos: f.Pos},
		Suffix: []ir.Suffix{
			ir.SuffixMemberAccess{Name: "has_next", Pos: f.Pos},
			ir.SuffixCall{Args: nil, Pos: f.Pos},
		},
	}

	nextExpr := ir.PostFixExpr{
		Base: ir.NameRef{Name: itVarName, Pos: f.Pos},
		Suffix: []ir.Suffix{
			ir.SuffixMemberAccess{Name: "next", Pos: f.Pos},
			ir.SuffixCall{Args: nil, Pos: f.Pos},
		},
	}

	bindLoopVar := ir.BIND_DEC{
		Name:  f.VarName,
		Value: nextExpr,
		Pos:   f.Pos,
	}

	whileBodyStmts := make([]any, 0, len(f.Body.Stmts)+1)
	whileBodyStmts = append(whileBodyStmts, bindLoopVar)
	whileBodyStmts = append(whileBodyStmts, f.Body.Stmts...)

	whileStmt := ir.WhileStmt{
		Cond: condExpr,
		Body: ir.Block{
			Stmts: whileBodyStmts,
			Pos:   f.Body.Pos,
		},
		Pos: f.Pos,
	}

	forBlock := ir.Block{
		Stmts: []any{
			bindIt,
			whileStmt,
		},
		Pos: f.Pos,
	}

	tBlock := te.analyzeBlock(&forBlock, expectedReturn, hasReturn)
	return &tBlock
}

func (te *TypeEvaluator) isLValue(e TExpr) bool {
	switch u := e.(type) {
	case *TVarRefExpr, *TIndexExpr, *TMemberExpr:
		return true
	case *TUnaryExpr:
		return u.Op == ir.UNARY_OP_DEREF
	default:
		return false
	}
}

func (te *TypeEvaluator) isReadOnlyTarget(e TExpr) bool {
	switch u := e.(type) {
	case *TUnaryExpr:
		if u.Op == ir.UNARY_OP_DEREF {
			if pt, ok := u.Expr.ExprType().(*PtrType); ok && pt.ReadOnly {
				return true
			}
			if rt, ok := u.Expr.ExprType().(*RefType); ok && rt.ReadOnly {
				return true
			}
		}
	case *TMemberExpr:
		if pt, ok := u.Base.ExprType().(*PtrType); ok && pt.ReadOnly {
			return true
		}
		if rt, ok := u.Base.ExprType().(*RefType); ok && rt.ReadOnly {
			return true
		}
		return te.isReadOnlyTarget(u.Base)
	case *TIndexExpr:
		if pt, ok := u.Base.ExprType().(*PtrType); ok && pt.ReadOnly {
			return true
		}
		if u.Base.ExprType().Equals(TypeStr) {
			return true // string literals/values are immutable through indexing
		}
		return te.isReadOnlyTarget(u.Base)
	}
	return false
}

func (te *TypeEvaluator) markAddressTaken(e TExpr) {
	switch v := e.(type) {
	case *TVarRefExpr:
		sym := te.stack.Resolve(v.Name)
		if sym != nil {
			sym.AddressTaken = true
			switch decl := sym.DeclNode.(type) {
			case *TVarDec:
				decl.AddressTaken = true
			case *TFnParam:
				decl.AddressTaken = true
			}
		}
	case *TMemberExpr:
		te.markAddressTaken(v.Base)
	case *TIndexExpr:
		if _, isList := v.Base.ExprType().(*ListType); isList {
			te.markAddressTaken(v.Base)
		}
	}
}

func (te *TypeEvaluator) analyzeReturnStmt(ret *ir.ReturnStmt, expectedReturn Type, hasReturn *bool) *TReturnStmt {
	if hasReturn != nil {
		*hasReturn = true
	}

	var tVal TExpr
	var actualType Type = TypeVoid
	if ret.Value != nil {
		tVal = te.EvalExpr(ret.Value)
		actualType = tVal.ExprType()
	}

	if expectedReturn != nil && expectedReturn != TypeInfer && expectedReturn != TypeUnknown {
		if !actualType.AssignableTo(expectedReturn) {
			te.addDiag(SeverityError, ret.Pos, "type/return-mismatch",
				fmt.Sprintf("expected return type %q, got %q", expectedReturn.String(), actualType.String()))
		} else if st, ok := expectedReturn.(*StructType); ok {
			if objExpr, ok := tVal.(*TObjectExpr); ok {
				tVal = &TStructLitExpr{
					StructType: st,
					Fields:     objExpr.Fields,
					Pos_:       objExpr.Pos_,
				}
			}
		}
	}

	return &TReturnStmt{
		Value: tVal,
		Pos_:  ret.Pos,
	}
}

// EvalExpr evaluates an ir.Expr and returns its typed counterpart TExpr with concrete Type.
func (te *TypeEvaluator) EvalExpr(raw ir.Expr) TExpr {
	if raw == nil {
		return &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: TypeUnknown}
	}

	switch e := raw.(type) {
	case ir.Literal:
		return te.evalLiteral(&e)
	case *ir.Literal:
		return te.evalLiteral(e)

	case ir.NameRef:
		return te.evalNameRef(&e)
	case *ir.NameRef:
		return te.evalNameRef(e)

	case ir.BinaryExpr:
		return te.evalBinaryExpr(&e)
	case *ir.BinaryExpr:
		return te.evalBinaryExpr(e)

	case ir.UnaryExpr:
		return te.evalUnaryExpr(&e)
	case *ir.UnaryExpr:
		return te.evalUnaryExpr(e)

	case ir.PostFixExpr:
		return te.evalPostFixExpr(&e)
	case *ir.PostFixExpr:
		return te.evalPostFixExpr(e)

	case ir.ObjectLiteral:
		return te.evalObjectLiteral(&e)
	case *ir.ObjectLiteral:
		return te.evalObjectLiteral(e)

	case ir.ListLiteral:
		return te.evalListLiteral(&e)
	case *ir.ListLiteral:
		return te.evalListLiteral(e)

	case ir.DropStmt:
		inner := te.EvalExpr(e.Expr)
		return &TDropStmt{Expr: inner, Pos_: e.Pos}
	case *ir.DropStmt:
		inner := te.EvalExpr(e.Expr)
		return &TDropStmt{Expr: inner, Pos_: e.Pos}

	case ir.LabelDec:
		tBody := te.analyzeBlock(&e.Body, nil, nil)
		return &TLabelDec{Name: e.Name, Body: tBody, Pos_: e.Pos, Typ: tBody.Typ}
	case *ir.LabelDec:
		tBody := te.analyzeBlock(&e.Body, nil, nil)
		return &TLabelDec{Name: e.Name, Body: tBody, Pos_: e.Pos, Typ: tBody.Typ}

	case ir.BreakStmt:
		var tVal TExpr
		if e.Value != nil {
			tVal = te.EvalExpr(e.Value)
		}
		return &TBreakStmt{Label: e.Label, Value: tVal, Pos_: e.Pos}
	case *ir.BreakStmt:
		var tVal TExpr
		if e.Value != nil {
			tVal = te.EvalExpr(e.Value)
		}
		return &TBreakStmt{Label: e.Label, Value: tVal, Pos_: e.Pos}

	case ir.IfStmt:
		stmt := te.analyzeStmt(e, nil, nil)
		if ifStmt, ok := stmt.(*TIfStmt); ok {
			return ifStmt
		}
		return &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: TypeUnknown, Pos_: e.Pos}
	case *ir.IfStmt:
		stmt := te.analyzeStmt(*e, nil, nil)
		if ifStmt, ok := stmt.(*TIfStmt); ok {
			return ifStmt
		}
		return &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: TypeUnknown, Pos_: e.Pos}

	case ir.WhileStmt:
		stmt := te.analyzeStmt(e, nil, nil)
		if ws, ok := stmt.(*TWhileStmt); ok {
			return ws
		}
		return &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: TypeVoid, Pos_: e.Pos}
	case *ir.WhileStmt:
		stmt := te.analyzeStmt(*e, nil, nil)
		if ws, ok := stmt.(*TWhileStmt); ok {
			return ws
		}
		return &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: TypeVoid, Pos_: e.Pos}

	case ir.ForStmt:
		stmt := te.analyzeStmt(e, nil, nil)
		if tb, ok := stmt.(*TBlock); ok {
			return tb
		}
		return &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: TypeVoid, Pos_: e.Pos}
	case *ir.ForStmt:
		stmt := te.analyzeStmt(*e, nil, nil)
		if tb, ok := stmt.(*TBlock); ok {
			return tb
		}
		return &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: TypeVoid, Pos_: e.Pos}

	default:
		return &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: TypeUnknown}
	}
}

func (te *TypeEvaluator) evalLiteral(lit *ir.Literal) *TLiteralExpr {
	var typ Type = TypeUnknown
	switch lit.Kind {
	case ir.LITERAL_INT:
		typ = TypeI32
	case ir.LITERAL_FLOAT:
		typ = TypeF64
	case ir.LITERAL_STRING:
		typ = TypeStr
	case ir.LITERAL_BOOL:
		typ = TypeBool
	case ir.LITERAL_NONE:
		typ = TypeNone
	case ir.LITERAL_UNINITIALIZED, ir.LITERAL_DYNAMIC_SIZE:
		typ = TypeUnknown
	}

	return &TLiteralExpr{
		Kind:  lit.Kind,
		Value: lit.Value,
		Typ:   typ,
		Pos_:  lit.Pos,
	}
}

func (te *TypeEvaluator) evalNameRef(nr *ir.NameRef) *TVarRefExpr {
	if nr.Name == "Self" && te.currentSelf != nil {
		return &TVarRefExpr{Name: nr.Name, Typ: te.currentSelf, Pos_: nr.Pos}
	}
	sym := te.stack.Resolve(nr.Name)
	if sym == nil {
		if IsIntrinsic(nr.Name) || nr.Name == "#refGrab" {
			return &TVarRefExpr{Name: nr.Name, Typ: TypeAny, Pos_: nr.Pos}
		}
		te.addDiag(SeverityError, nr.Pos, "type/undefined-identifier",
			fmt.Sprintf("undefined identifier %q", nr.Name))
		return &TVarRefExpr{Name: nr.Name, Typ: TypeUnknown, Pos_: nr.Pos}
	}
	refName := nr.Name
	if sym.MangledName != "" && sym.Kind == SymbolFn {
		refName = sym.MangledName
	}
	return &TVarRefExpr{Name: refName, Typ: sym.Type, Pos_: nr.Pos}
}

func (te *TypeEvaluator) evalBinaryExpr(b *ir.BinaryExpr) *TBinaryExpr {
	tLeft := te.EvalExpr(b.Left)
	tRight := te.EvalExpr(b.Right)

	var resType Type = TypeUnknown
	switch b.Op {
	case ir.BINARY_OP_PLUS, ir.BINARY_OP_MINUS:
		if pt, ok := tLeft.ExprType().(*PtrType); ok {
			resType = pt
		} else {
			resType = tLeft.ExprType()
		}
	case ir.BINARY_OP_TIME, ir.BINARY_OP_DIV, ir.BINARY_OP_MOD:
		resType = tLeft.ExprType()
	case ir.BINARY_OP_EQ, ir.BINARY_OP_NOT_EQ, ir.BINARY_OP_LESS, ir.BINARY_OP_LESS_EQ, ir.BINARY_OP_GREATER, ir.BINARY_OP_GREATER_EQ:
		resType = TypeBool
	case ir.BINARY_OP_AND, ir.BINARY_OP_OR, "&&", "||", "&", "|", "^":
		resType = TypeBool
	default:
		resType = tLeft.ExprType()
	}

	return &TBinaryExpr{
		Op:    b.Op,
		Left:  tLeft,
		Right: tRight,
		Typ:   resType,
		Pos_:  b.Pos,
	}
}

func (te *TypeEvaluator) evalUnaryExpr(u *ir.UnaryExpr) *TUnaryExpr {
	tExpr := te.EvalExpr(u.Expr)
	var resType Type = tExpr.ExprType()

	switch u.Op {
	case ir.UNARY_OP_NOT:
		resType = TypeBool
	case ir.UNARY_OP_POSITIVE, ir.UNARY_OP_NEGATIVE:
		resType = tExpr.ExprType()
	case ir.UNARY_OP_RAW_REF:
		if !te.isLValue(tExpr) {
			te.addDiag(SeverityError, u.Pos, "type/invalid-address-of",
				"cannot take raw pointer (&raw) of a temporary rvalue; bind it to a variable first")
		}
		te.markAddressTaken(tExpr)
		resType = &PtrType{Elem: tExpr.ExprType()}
	case ir.UNARY_OP_REF:
		te.markAddressTaken(tExpr)
		resType = &RefType{Elem: tExpr.ExprType()}
	case ir.UNARY_OP_DEREF:
		switch pt := tExpr.ExprType().(type) {
		case *PtrType:
			resType = pt.Elem
		case *RefType:
			resType = pt.Elem
		default:
			if tExpr.ExprType() != TypeUnknown && tExpr.ExprType() != TypeAny {
				te.addDiag(SeverityError, u.Pos, "type/invalid-deref",
					fmt.Sprintf("cannot dereference non-pointer/reference type %q", tExpr.ExprType().String()))
			}
			resType = TypeUnknown
		}
	}

	return &TUnaryExpr{
		Op:   u.Op,
		Expr: tExpr,
		Typ:  resType,
		Pos_: u.Pos,
	}
}

func (te *TypeEvaluator) evalPostFixExpr(pf *ir.PostFixExpr) TExpr {
	tBase := te.EvalExpr(pf.Base)

	if len(pf.Suffix) == 0 {
		return tBase
	}

	var curExpr TExpr = tBase
	var boundReceiver TExpr

	for _, s := range pf.Suffix {
		switch suf := s.(type) {
		case ir.SuffixCall:
			var args []TExpr
			for _, a := range suf.Args {
				args = append(args, te.EvalExpr(a))
			}

			// Special intrinsic #refGrab(r) -> dereferences ref<T>
			if baseRef, ok := curExpr.(*TVarRefExpr); ok && baseRef.Name == "#refGrab" && len(args) == 1 {
				var elemType Type = TypeUnknown
				if rt, ok := args[0].ExprType().(*RefType); ok {
					elemType = rt.Elem
				} else if pt, ok := args[0].ExprType().(*PtrType); ok {
					elemType = pt.Elem
				} else {
					te.addDiag(SeverityError, suf.Pos, "type/call-arg-type",
						fmt.Sprintf("#refGrab expects ref<T>, got %q", args[0].ExprType().String()))
				}
				curExpr = &TUnaryExpr{
					Op:   ir.UNARY_OP_DEREF,
					Expr: args[0],
					Typ:  elemType,
					Pos_: suf.Pos,
				}
				continue
			}

			var retType Type = TypeUnknown
			baseType := curExpr.ExprType()
			if fnType, ok := baseType.(*FnType); ok {
				retType = fnType.ReturnType
				if len(args) != len(fnType.Params) {
					te.addDiag(SeverityError, suf.Pos, "type/call-arg-count",
						fmt.Sprintf("function expects %d arguments, but got %d", len(fnType.Params), len(args)))
				} else {
					for i, arg := range args {
						paramType := fnType.Params[i].Type
						if !arg.ExprType().AssignableTo(paramType) {
							te.addDiag(SeverityError, arg.Pos(), "type/call-arg-type",
								fmt.Sprintf("argument %d expects type %q, got %q", i+1, paramType.String(), arg.ExprType().String()))
						} else if st, ok := paramType.(*StructType); ok {
							if objExpr, ok := arg.(*TObjectExpr); ok {
								args[i] = &TStructLitExpr{
									StructType: st,
									Fields:     objExpr.Fields,
									Pos_:       objExpr.Pos_,
								}
							}
						}
					}
				}
			} else if baseRef, ok := curExpr.(*TVarRefExpr); ok && IsIntrinsic(baseRef.Name) {
				retType = TypeVoid
			} else if baseType == TypeUnknown || baseType == TypeAny {
				retType = TypeAny
			} else {
				te.addDiag(SeverityError, suf.Pos, "type/not-callable",
					fmt.Sprintf("expression of type %q is not callable", baseType.String()))
			}

			if boundReceiver != nil {
				args = append([]TExpr{boundReceiver}, args...)
				boundReceiver = nil
			}

			curExpr = &TCallExpr{
				Callee: curExpr,
				Args:   args,
				Typ:    retType,
				Pos_:   suf.Pos,
			}

		case ir.SuffixCallTypeMake:
			st, ok := curExpr.ExprType().(*StructType)
			if !ok {
				te.addDiag(SeverityError, suf.Pos, "type/not-a-struct",
					fmt.Sprintf("cannot construct non-struct type %q with ::{ ... }", curExpr.ExprType().String()))
				curExpr = &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: TypeUnknown, Pos_: suf.Pos}
				continue
			}
			objLit, ok := suf.Arg.(ir.ObjectLiteral)
			if !ok {
				te.addDiag(SeverityError, suf.Pos, "type/invalid-struct-init",
					fmt.Sprintf("struct %q expects named field initialization ::{ .field: val }", st.Name))
				curExpr = &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: st, Pos_: suf.Pos}
				continue
			}
			provided := make(map[string]TExpr)
			var tFields []TFieldAssign
			for _, f := range objLit.Fields {
				val := te.EvalExpr(f.Expr)
				sf, exists := st.Field(f.Name)
				if !exists {
					te.addDiag(SeverityError, suf.Pos, "type/field-not-found",
						fmt.Sprintf("struct %q has no field %q", st.Name, f.Name))
				} else if !val.ExprType().AssignableTo(sf.Type) {
					te.addDiag(SeverityError, val.Pos(), "type/field-type-mismatch",
						fmt.Sprintf("field %q of struct %q expects %q, got %q",
							f.Name, st.Name, sf.Type.String(), val.ExprType().String()))
				}
				provided[f.Name] = val
				tFields = append(tFields, TFieldAssign{Name: f.Name, Expr: val})
			}
			for _, sf := range st.Fields {
				if _, ok := provided[sf.Name]; !ok {
					te.addDiag(SeverityError, suf.Pos, "type/missing-struct-field",
						fmt.Sprintf("missing initializer for field %q in struct %q", sf.Name, st.Name))
				}
			}
			curExpr = &TStructLitExpr{
				StructType: st,
				Fields:     tFields,
				Pos_:       suf.Pos,
			}

		case ir.SuffixCallTypeInit:
			st, ok := curExpr.ExprType().(*StructType)
			if !ok {
				te.addDiag(SeverityError, suf.Pos, "type/not-a-struct",
					fmt.Sprintf("cannot invoke ::(...) init constructor on %q", curExpr.ExprType().String()))
				curExpr = &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: TypeUnknown, Pos_: suf.Pos}
				continue
			}
			var args []TExpr
			for _, a := range suf.Args {
				args = append(args, te.EvalExpr(a))
			}
			mSym := te.lookupMethod(st.Name, "#init")
			if mSym == nil {
				mSym = te.lookupMethod(st.Name, "new")
			}
			if mSym == nil {
				te.addDiag(SeverityError, suf.Pos, "type/missing-init",
					fmt.Sprintf("struct %q does not define a #init or new constructor", st.Name))
				curExpr = &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: st, Pos_: suf.Pos}
				continue
			}
			fnType, _ := mSym.Type.(*FnType)
			if fnType != nil && len(args) != len(fnType.Params) {
				te.addDiag(SeverityError, suf.Pos, "type/call-arg-count",
					fmt.Sprintf("constructor for %q expects %d arguments, got %d", st.Name, len(fnType.Params), len(args)))
			} else if fnType != nil {
				for i, arg := range args {
					if !arg.ExprType().AssignableTo(fnType.Params[i].Type) {
						te.addDiag(SeverityError, arg.Pos(), "type/call-arg-type",
							fmt.Sprintf("argument %d expects %q, got %q", i+1, fnType.Params[i].Type.String(), arg.ExprType().String()))
					}
				}
			}
			retType := Type(st)
			if fnType != nil && fnType.ReturnType != nil {
				retType = fnType.ReturnType
			}
			curExpr = &TCallExpr{
				Callee: &TVarRefExpr{Name: mSym.MangledName, Typ: mSym.Type, Pos_: suf.Pos},
				Args:   args,
				Typ:    retType,
				Pos_:   suf.Pos,
			}

		case ir.SuffixAssociatedAccess:
			if st, ok := curExpr.ExprType().(*StructType); ok {
				mSym := te.lookupMethod(st.Name, suf.Name)
				if mSym == nil {
					te.addDiag(SeverityError, suf.Pos, "type/method-not-found",
						fmt.Sprintf("associated function %q not found on struct %q", suf.Name, st.Name))
					curExpr = &TVarRefExpr{Name: suf.Name, Typ: TypeUnknown, Pos_: suf.Pos}
					continue
				}
				curExpr = &TVarRefExpr{Name: mSym.MangledName, Typ: mSym.Type, Pos_: suf.Pos}
				continue
			}
			if modAlias, ok := curExpr.ExprType().(*ModuleAliasType); ok {
				curExpr = te.resolveModuleMember(modAlias, suf.Name, suf.Pos)
				continue
			}
			te.addDiag(SeverityError, suf.Pos, "type/invalid-associated-access",
				fmt.Sprintf("cannot access ::%s on %q", suf.Name, curExpr.ExprType().String()))
			curExpr = &TVarRefExpr{Name: suf.Name, Typ: TypeUnknown, Pos_: suf.Pos}

		case ir.SuffixNamespaceAccess:
			if modAlias, ok := curExpr.ExprType().(*ModuleAliasType); ok {
				curExpr = te.resolveModuleMember(modAlias, suf.Name, suf.Pos)
				continue
			}
			te.addDiag(SeverityError, suf.Pos, "type/invalid-namespace-access",
				fmt.Sprintf("cannot access :%s on %q", suf.Name, curExpr.ExprType().String()))
			curExpr = &TVarRefExpr{Name: suf.Name, Typ: TypeUnknown, Pos_: suf.Pos}

		case ir.SuffixTraitMat:
			traitType, ok := curExpr.ExprType().(*TraitType)
			if !ok {
				te.addDiag(SeverityError, suf.Pos, "type/not-a-trait",
					fmt.Sprintf("cannot materialize non-trait %q", curExpr.ExprType().String()))
				curExpr = &TLiteralExpr{Kind: ir.LITERAL_UNINITIALIZED, Typ: TypeUnknown, Pos_: suf.Pos}
				continue
			}
			targetType := te.parseType(&suf.TargetType)
			targetName := targetType.String()
			if te.traitImpls[traitType.Name] == nil || te.traitImpls[traitType.Name][targetName] == nil {
				te.addDiag(SeverityError, suf.Pos, "type/missing-trait-impl",
					fmt.Sprintf("type %q does not implement trait %q", targetName, traitType.Name))
			}
			curExpr = &TTraitMatExpr{
				Trait:  traitType,
				Target: targetType,
				Typ:    &TraitMatType{Trait: traitType, Target: targetType},
				Pos_:   suf.Pos,
			}

		case ir.SuffixMemberAccess:
			boundReceiver = nil
			baseType := curExpr.ExprType()

			// 1. Module alias access: io.println
			if modAlias, ok := baseType.(*ModuleAliasType); ok {
				curExpr = te.resolveModuleMember(modAlias, suf.Name, suf.Pos)
				continue
			}

			// 2. Materialized trait access: w.write
			if mat, ok := baseType.(*TraitMatType); ok {
				targetName := mat.Target.String()
				if implMap := te.traitImpls[mat.Trait.Name]; implMap != nil {
					if methods := implMap[targetName]; methods != nil {
						if mSym, exists := methods[suf.Name]; exists {
							curExpr = &TVarRefExpr{Name: mSym.MangledName, Typ: mSym.Type, Pos_: suf.Pos}
							continue
						}
					}
				}
				te.addDiag(SeverityError, suf.Pos, "type/method-not-found",
					fmt.Sprintf("method %q not found on materialized trait %q", suf.Name, mat.String()))
				curExpr = &TVarRefExpr{Name: suf.Name, Typ: TypeUnknown, Pos_: suf.Pos}
				continue
			}

			// 3. Struct, ptr<Struct>, or ref<Struct> field / instance method access
			if st := unwrapStructType(baseType); st != nil {
				if sf, exists := st.Field(suf.Name); exists {
					curExpr = &TMemberExpr{
						Base:   curExpr,
						Member: suf.Name,
						Offset: sf.Offset,
						Typ:    sf.Type,
						Pos_:   suf.Pos,
					}
					continue
				}
				if mSym := te.lookupMethod(st.Name, suf.Name); mSym != nil {
					if fnType, ok := mSym.Type.(*FnType); ok {
						if len(fnType.Params) > 0 && fnType.Params[0].Name == "self" {
							selfParamType := fnType.Params[0].Type
							if _, isPtr := selfParamType.(*PtrType); isPtr {
								te.markAddressTaken(curExpr)
							} else if _, isRef := selfParamType.(*RefType); isRef {
								te.markAddressTaken(curExpr)
							}
							if isMutSelfReceiver(selfParamType) && isReadOnlyReceiverType(baseType) {
								te.addDiag(SeverityError, suf.Pos, "type/readonly-mutation",
									fmt.Sprintf("cannot call mutable method %q on read-only %q", suf.Name, baseType.String()))
							}
							boundReceiver = curExpr
							stripped := &FnType{
								Params:     fnType.Params[1:],
								ReturnType: fnType.ReturnType,
							}
							curExpr = &TVarRefExpr{Name: mSym.MangledName, Typ: stripped, Pos_: suf.Pos}
							continue
						}
						curExpr = &TVarRefExpr{Name: mSym.MangledName, Typ: fnType, Pos_: suf.Pos}
						continue
					}
				}
				te.addDiag(SeverityError, suf.Pos, "type/field-not-found",
					fmt.Sprintf("field or method %q not found on struct %q", suf.Name, st.Name))
				curExpr = &TMemberExpr{Base: curExpr, Member: suf.Name, Typ: TypeUnknown, Pos_: suf.Pos}
				continue
			}

			// 4. Anonymous ObjectType field access
			var memberType Type = TypeUnknown
			if objType, ok := baseType.(*ObjectType); ok {
				if ft, ok := objType.Fields[suf.Name]; ok {
					memberType = ft
				} else {
					te.addDiag(SeverityError, suf.Pos, "type/field-not-found",
						fmt.Sprintf("field %q not found on type %q", suf.Name, objType.String()))
				}
			} else if baseType != TypeUnknown && baseType != TypeAny {
				te.addDiag(SeverityError, suf.Pos, "type/field-not-found",
					fmt.Sprintf("cannot access member %q on type %q", suf.Name, baseType.String()))
			}
			curExpr = &TMemberExpr{Base: curExpr, Member: suf.Name, Typ: memberType, Pos_: suf.Pos}

		case ir.SuffixIndex:
			tIdx := te.EvalExpr(suf.Index)
			if prim, ok := tIdx.ExprType().(*PrimitiveType); !ok || !isAnyInt(prim.Name) {
				if tIdx.ExprType() != TypeUnknown && tIdx.ExprType() != TypeAny {
					te.addDiag(SeverityError, tIdx.Pos(), "type/invalid-index-type",
						fmt.Sprintf("index expression must be an integer, got %q", tIdx.ExprType().String()))
				}
			}

			var elemType Type = TypeUnknown
			switch bt := curExpr.ExprType().(type) {
			case *ListType:
				elemType = bt.ItemType
			case *PtrType:
				elemType = bt.Elem
			case *PrimitiveType:
				if bt.Name == "str" {
					elemType = TypeU8
				} else if bt.Name != "unknown" && bt.Name != "any" {
					te.addDiag(SeverityError, suf.Pos, "type/not-indexable",
						fmt.Sprintf("type %q does not support indexing", bt.String()))
				}
			default:
				te.addDiag(SeverityError, suf.Pos, "type/not-indexable",
					fmt.Sprintf("type %q does not support indexing", curExpr.ExprType().String()))
			}

			curExpr = &TIndexExpr{
				Base:  curExpr,
				Index: tIdx,
				Typ:   elemType,
				Pos_:  suf.Pos,
			}
		}
	}

	return curExpr
}

func unwrapStructType(t Type) *StructType {
	switch v := t.(type) {
	case *StructType:
		return v
	case *PtrType:
		if st, ok := v.Elem.(*StructType); ok {
			return st
		}
	case *RefType:
		if st, ok := v.Elem.(*StructType); ok {
			return st
		}
	}
	return nil
}

func isMutSelfReceiver(t Type) bool {
	switch v := t.(type) {
	case *PtrType:
		return !v.ReadOnly
	case *RefType:
		return !v.ReadOnly
	}
	return false
}

func isReadOnlyReceiverType(t Type) bool {
	switch v := t.(type) {
	case *PtrType:
		return v.ReadOnly
	case *RefType:
		return v.ReadOnly
	}
	return false
}

func (te *TypeEvaluator) lookupMethod(typeName, methodName string) *Symbol {
	if methods, ok := te.methodImpls[typeName]; ok {
		return methods[methodName]
	}
	return nil
}

func (te *TypeEvaluator) resolveModuleMember(modAlias *ModuleAliasType, name string, pos ir.Position) *TVarRefExpr {
	if modAlias.Exports != nil {
		if sym, ok := modAlias.Exports[name]; ok && sym != nil {
			refName := sym.Name
			if sym.MangledName != "" {
				refName = sym.MangledName
			}
			return &TVarRefExpr{Name: refName, Typ: sym.Type, Pos_: pos}
		}
	}
	te.addDiag(SeverityError, pos, "import/undefined-symbol",
		fmt.Sprintf("symbol %q is not exported by module %q", name, modAlias.Source))
	return &TVarRefExpr{Name: name, Typ: TypeUnknown, Pos_: pos}
}

func (te *TypeEvaluator) evalObjectLiteral(obj *ir.ObjectLiteral) *TObjectExpr {
	fields := make([]TFieldAssign, 0, len(obj.Fields))
	typeMap := make(map[string]Type)

	for _, f := range obj.Fields {
		tExpr := te.EvalExpr(f.Expr)
		fields = append(fields, TFieldAssign{
			Name: f.Name,
			Expr: tExpr,
		})
		typeMap[f.Name] = tExpr.ExprType()
	}

	return &TObjectExpr{
		Fields: fields,
		Typ:    &ObjectType{Fields: typeMap},
		Pos_:   obj.Pos,
	}
}

func (te *TypeEvaluator) evalListLiteral(list *ir.ListLiteral) *TListExpr {
	items := make([]TExpr, 0, len(list.Content))
	var elemType Type = TypeAny

	for _, c := range list.Content {
		tExpr := te.EvalExpr(c)
		items = append(items, tExpr)
		if elemType == TypeAny {
			elemType = tExpr.ExprType()
		}
	}

	return &TListExpr{
		Items: items,
		Typ:   &ListType{ItemType: elemType, Size: len(items)},
		Pos_:  list.Pos,
	}
}
