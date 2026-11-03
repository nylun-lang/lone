package loneproject

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	loneanalyzer "lone/pkg/lone-analyzer"
	loneparser "lone/pkg/lone-parser"
	loneast "lone/pkg/lone-parser-ast"
)

// DebugSymbolKind classifies inspected symbols.
type DebugSymbolKind string

const (
	DebugKindStruct DebugSymbolKind = "struct"
	DebugKindTrait  DebugSymbolKind = "trait"
	DebugKindFn     DebugSymbolKind = "fn"
	DebugKindVar    DebugSymbolKind = "var"
	DebugKindField  DebugSymbolKind = "field"
	DebugKindMethod DebugSymbolKind = "method"
)

// DebugSymbolInfo contains detailed inspection data for a queried symbol.
type DebugSymbolInfo struct {
	Name       string          `json:"name"`
	ScopedName string          `json:"scoped_name,omitempty"`
	Kind       DebugSymbolKind `json:"kind"`
	Scope      string          `json:"scope"`
	FilePath   string          `json:"file_path"`
	DefinedPos string          `json:"defined_pos"`
	IsPublic   bool            `json:"is_public"`

	// Variable details
	DeclaredType  string `json:"declared_type,omitempty"`
	ResolvedType  string `json:"resolved_type,omitempty"`
	IsInitialized bool   `json:"is_initialized,omitempty"`
	Storage       string `json:"storage,omitempty"` // "datastack frame" or "static linear memory"
	AddressTaken  bool   `json:"address_taken,omitempty"`
	EnclosingFn   string `json:"enclosing_fn,omitempty"`

	// Struct details
	Size              int                  `json:"size,omitempty"`
	Align             int                  `json:"align,omitempty"`
	Fields            []DebugFieldInfo     `json:"fields,omitempty"`
	Constructors      []string             `json:"constructors,omitempty"`
	Methods           []DebugMethodInfo    `json:"methods,omitempty"`
	ImplementedTraits []DebugTraitImplInfo `json:"implemented_traits,omitempty"`

	// Trait details
	TraitMethods []DebugMethodInfo `json:"trait_methods,omitempty"`
	Implementors []string          `json:"implementors,omitempty"`

	// Function / Method details
	Signature   string         `json:"signature,omitempty"`
	Params      []DebugVarInfo `json:"params,omitempty"`
	ReturnType  string         `json:"return_type,omitempty"`
	LocalVars   []DebugVarInfo `json:"local_vars,omitempty"`
	TraitSource string         `json:"trait_source,omitempty"`

	// Struct Field details
	FieldOffset  int    `json:"field_offset,omitempty"`
	FieldSize    int    `json:"field_size,omitempty"`
	ParentStruct string `json:"parent_struct,omitempty"`
}

// DebugFieldInfo represents a field of a struct.
type DebugFieldInfo struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Size   int    `json:"size"`
}

// DebugMethodInfo represents a method signature and trait origin.
type DebugMethodInfo struct {
	Name        string `json:"name"`
	Signature   string `json:"signature"`
	TraitSource string `json:"trait_source,omitempty"`
	IsStatic    bool   `json:"is_static"`
}

// DebugTraitImplInfo captures traits implemented on a struct.
type DebugTraitImplInfo struct {
	TraitName string            `json:"trait_name"`
	Methods   []DebugMethodInfo `json:"methods"`
}

// DebugVarInfo represents variable summary information.
type DebugVarInfo struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Storage      string `json:"storage"`
	AddressTaken bool   `json:"address_taken"`
	Pos          string `json:"pos,omitempty"`
}

// DebugScopeDump represents an overview of all symbols declared in a scope.
type DebugScopeDump struct {
	Scope     string                `json:"scope"`
	FilePath  string                `json:"file_path"`
	Structs   []DebugStructSummary  `json:"structs"`
	Traits    []DebugTraitSummary   `json:"traits"`
	Functions []DebugFnSummary      `json:"functions"`
	Globals   []DebugVarInfo        `json:"globals"`
	Imports   []DebugImportSummary  `json:"imports"`
}

// DebugStructSummary summarizes a struct in a scope dump.
type DebugStructSummary struct {
	Name         string `json:"name"`
	Size         int    `json:"size"`
	Align        int    `json:"align"`
	FieldCount   int    `json:"field_count"`
	MethodCount  int    `json:"method_count"`
	TraitCount   int    `json:"trait_count"`
	IsPublic     bool   `json:"is_public"`
}

// DebugTraitSummary summarizes a trait in a scope dump.
type DebugTraitSummary struct {
	Name         string   `json:"name"`
	MethodCount  int      `json:"method_count"`
	Implementors []string `json:"implementors"`
	IsPublic     bool     `json:"is_public"`
}

// DebugFnSummary summarizes a function in a scope dump.
type DebugFnSummary struct {
	Name       string `json:"name"`
	Signature  string `json:"signature"`
	IsPublic   bool   `json:"is_public"`
	LocalCount int    `json:"local_count"`
}

// DebugImportSummary summarizes an import declaration.
type DebugImportSummary struct {
	Source  string   `json:"source"`
	Alias   string   `json:"alias,omitempty"`
	Symbols []string `json:"symbols,omitempty"`
}

// DebugContext encapsulates analyzed scope state.
type DebugContext struct {
	ScopeName    string
	FilePath     string
	RawAst       any
	ResolvedAst  []any
	TypedProgram *loneanalyzer.TProgram
	Analyzer     *loneanalyzer.Analyzer
	Evaluator    *loneanalyzer.TypeEvaluator
	AstNodeMap   map[string]any // name -> raw AST node (for public flag and declared type)
}

// ResolveScopePath resolves a scope identifier (e.g. ":std:io", "examples/guess.lone") to an absolute file path.
func ResolveScopePath(scope string) (string, error) {
	if strings.HasPrefix(scope, ":") {
		r := NewResolver("")
		path, _, err := r.resolveModulePath("", scope)
		if err != nil {
			return "", err
		}
		return path, nil
	}

	clean := filepath.Clean(scope)
	if fi, err := os.Stat(clean); err == nil && !fi.IsDir() {
		abs, err := filepath.Abs(clean)
		if err == nil {
			return abs, nil
		}
		return clean, nil
	}

	if !strings.HasSuffix(clean, ".lone") {
		lonePath := clean + ".lone"
		if fi, err := os.Stat(lonePath); err == nil && !fi.IsDir() {
			abs, err := filepath.Abs(lonePath)
			if err == nil {
				return abs, nil
			}
			return lonePath, nil
		}
	}

	return "", fmt.Errorf("scope %q not found (must be a valid .lone file or standard namespace like :std:io)", scope)
}

// LoadDebugContext loads and semantically analyzes the target scope.
func LoadDebugContext(scope string) (*DebugContext, error) {
	filePath, err := ResolveScopePath(scope)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read scope file %s: %w", filePath, err)
	}

	rawAst, err := loneparser.Parse(filePath, content)
	if err != nil {
		return nil, fmt.Errorf("syntax error in %s: %w", filePath, err)
	}

	resolvedAst, _ := ResolveAST(filePath, rawAst)
	resolvedSlice := extractDeclarations(resolvedAst)

	analyzer := loneanalyzer.NewAnalyzer(filePath, loneanalyzer.DefaultConfig())
	typedProg, _ := analyzer.AnalyzeTyped(resolvedSlice)
	evaluator := analyzer.Evaluator()

	astNodeMap := make(map[string]any)
	for _, d := range resolvedSlice {
		switch node := d.(type) {
		case *loneast.FN_DEC:
			astNodeMap[node.Name] = node
		case loneast.FN_DEC:
			astNodeMap[node.Name] = &node
		case *loneast.TYPE_DEC:
			astNodeMap[node.Name] = node
		case loneast.TYPE_DEC:
			astNodeMap[node.Name] = &node
		case *loneast.BIND_DEC:
			astNodeMap[node.Name] = node
		case loneast.BIND_DEC:
			astNodeMap[node.Name] = &node
		}
	}

	return &DebugContext{
		ScopeName:    scope,
		FilePath:     filePath,
		RawAst:       rawAst,
		ResolvedAst:  resolvedSlice,
		TypedProgram: typedProg,
		Analyzer:     analyzer,
		Evaluator:    evaluator,
		AstNodeMap:   astNodeMap,
	}, nil
}

// InspectSymbol inspects a symbol in scope, supporting scoped paths like "main:guess" or "Stdout:write_str".
func InspectSymbol(scope, symbol string) (*DebugSymbolInfo, error) {
	ctx, err := LoadDebugContext(scope)
	if err != nil {
		return nil, err
	}

	symbol = strings.TrimSpace(symbol)
	var parent, child string
	if idx := strings.IndexAny(symbol, ":."); idx != -1 {
		parent = symbol[:idx]
		child = symbol[idx+1:]
		if strings.HasPrefix(child, ":") || strings.HasPrefix(child, ".") {
			child = child[1:]
		}
	} else {
		child = symbol
	}

	if parent != "" {
		return inspectScopedSymbol(ctx, parent, child, symbol)
	}

	return inspectUnscopedSymbol(ctx, child)
}

func inspectScopedSymbol(ctx *DebugContext, parent, child, rawQuery string) (*DebugSymbolInfo, error) {
	// 1. Is parent a Struct?
	if st := findStructType(ctx, parent); st != nil {
		// Field check
		if f, ok := st.Field(child); ok {
			return &DebugSymbolInfo{
				Name:         child,
				ScopedName:   rawQuery,
				Kind:         DebugKindField,
				Scope:        ctx.ScopeName,
				FilePath:     ctx.FilePath,
				ParentStruct: parent,
				ResolvedType: f.Type.String(),
				FieldOffset:  f.Offset,
				FieldSize:    f.Size,
			}, nil
		}

		// Method check
		if m := findStructMethod(ctx, parent, child); m != nil {
			return &DebugSymbolInfo{
				Name:        child,
				ScopedName:  rawQuery,
				Kind:        DebugKindMethod,
				Scope:       ctx.ScopeName,
				FilePath:    ctx.FilePath,
				Signature:   m.Signature,
				TraitSource: m.TraitSource,
			}, nil
		}

		return nil, fmt.Errorf("field or method %q not found on struct %q", child, parent)
	}

	// 2. Is parent a Function?
	if fn := findFunction(ctx, parent); fn != nil {
		// Check parameter
		for _, p := range fn.Params {
			if p.Name == child {
				return &DebugSymbolInfo{
					Name:          child,
					ScopedName:    rawQuery,
					Kind:          DebugKindVar,
					Scope:         ctx.ScopeName,
					FilePath:      ctx.FilePath,
					DefinedPos:    formatPos(ctx.FilePath, p.Pos()),
					DeclaredType:  p.Type.String(),
					ResolvedType:  p.Type.String(),
					IsInitialized: true,
					Storage:       "datastack frame (parameter)",
					AddressTaken:  p.AddressTaken,
					EnclosingFn:   parent,
				}, nil
			}
		}

		// Check local variables in function body
		localVars := collectLocalVars(&fn.Body)
		for _, v := range localVars {
			if v.Name == child {
				return &DebugSymbolInfo{
					Name:          child,
					ScopedName:    rawQuery,
					Kind:          DebugKindVar,
					Scope:         ctx.ScopeName,
					FilePath:      ctx.FilePath,
					DefinedPos:    formatPos(ctx.FilePath, v.Pos()),
					DeclaredType:  v.Type.String(),
					ResolvedType:  v.Type.String(),
					IsInitialized: v.Value != nil,
					Storage:       "datastack frame",
					AddressTaken:  v.AddressTaken,
					EnclosingFn:   parent,
				}, nil
			}
		}

		return nil, fmt.Errorf("variable %q not found inside function %q", child, parent)
	}

	// 3. Is parent a Trait?
	if tr := findTraitType(ctx, parent); tr != nil {
		if mType, ok := tr.Methods[child]; ok {
			return &DebugSymbolInfo{
				Name:        child,
				ScopedName:  rawQuery,
				Kind:        DebugKindMethod,
				Scope:       ctx.ScopeName,
				FilePath:    ctx.FilePath,
				TraitSource: parent,
				Signature:   formatFnSignature(child, mType),
				ReturnType:  mType.ReturnType.String(),
			}, nil
		}
		return nil, fmt.Errorf("method %q not part of trait %q", child, parent)
	}

	return nil, fmt.Errorf("parent scope %q (struct, function, or trait) not found in %s", parent, ctx.ScopeName)
}

func inspectUnscopedSymbol(ctx *DebugContext, name string) (*DebugSymbolInfo, error) {
	// 1. Check Struct
	if st := findStructType(ctx, name); st != nil {
		return inspectStruct(ctx, name, st), nil
	}

	// 2. Check Trait
	if tr := findTraitType(ctx, name); tr != nil {
		return inspectTrait(ctx, name, tr), nil
	}

	// 3. Check Function
	if fn := findFunction(ctx, name); fn != nil {
		return inspectFunction(ctx, fn), nil
	}

	// 4. Check Global Variable
	if v := findGlobalVar(ctx, name); v != nil {
		return inspectGlobalVar(ctx, v), nil
	}

	// 5. Check if symbol exists as a unique local variable in any function
	var candidates []*DebugSymbolInfo
	if ctx.TypedProgram != nil {
		for _, d := range ctx.TypedProgram.Decls {
			if fn, ok := d.(*loneanalyzer.TFnDec); ok {
				for _, p := range fn.Params {
					if p.Name == name {
						candidates = append(candidates, &DebugSymbolInfo{
							Name:          name,
							ScopedName:    fmt.Sprintf("%s:%s", fn.Name, name),
							Kind:          DebugKindVar,
							Scope:         ctx.ScopeName,
							FilePath:      ctx.FilePath,
							DefinedPos:    formatPos(ctx.FilePath, p.Pos()),
							DeclaredType:  p.Type.String(),
							ResolvedType:  p.Type.String(),
							IsInitialized: true,
							Storage:       "datastack frame (parameter)",
							AddressTaken:  p.AddressTaken,
							EnclosingFn:   fn.Name,
						})
					}
				}
				for _, v := range collectLocalVars(&fn.Body) {
					if v.Name == name {
						candidates = append(candidates, &DebugSymbolInfo{
							Name:          name,
							ScopedName:    fmt.Sprintf("%s:%s", fn.Name, name),
							Kind:          DebugKindVar,
							Scope:         ctx.ScopeName,
							FilePath:      ctx.FilePath,
							DefinedPos:    formatPos(ctx.FilePath, v.Pos()),
							DeclaredType:  v.Type.String(),
							ResolvedType:  v.Type.String(),
							IsInitialized: v.Value != nil,
							Storage:       "datastack frame",
							AddressTaken:  v.AddressTaken,
							EnclosingFn:   fn.Name,
						})
					}
				}
			}
		}
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	} else if len(candidates) > 1 {
		var fns []string
		for _, c := range candidates {
			fns = append(fns, c.ScopedName)
		}
		return nil, fmt.Errorf("symbol %q is ambiguous (found in multiple functions: %s; use <fn>:<var>)",
			name, strings.Join(fns, ", "))
	}

	return nil, fmt.Errorf("symbol %q not found in scope %s", name, ctx.ScopeName)
}

func inspectStruct(ctx *DebugContext, name string, st *loneanalyzer.StructType) *DebugSymbolInfo {
	info := &DebugSymbolInfo{
		Name:     name,
		Kind:     DebugKindStruct,
		Scope:    ctx.ScopeName,
		FilePath: ctx.FilePath,
		Size:     st.Size,
		Align:    st.Align,
	}

	if raw, ok := ctx.AstNodeMap[name].(*loneast.TYPE_DEC); ok {
		info.IsPublic = raw.IsPublic
		info.DefinedPos = formatPos(ctx.FilePath, raw.Pos)
	}

	// Fields
	for _, f := range st.Fields {
		info.Fields = append(info.Fields, DebugFieldInfo{
			Name:   f.Name,
			Type:   f.Type.String(),
			Offset: f.Offset,
			Size:   f.Size,
		})
	}

	// Methods on struct (inherent)
	if ctx.Evaluator != nil {
		inherentMap := ctx.Evaluator.MethodImpls()[name]
		var methodNames []string
		for mName := range inherentMap {
			methodNames = append(methodNames, mName)
		}
		sort.Strings(methodNames)
		for _, mName := range methodNames {
			sym := inherentMap[mName]
			if fnType, ok := sym.Type.(*loneanalyzer.FnType); ok {
				traitSource := ""
				for tName, targets := range ctx.Evaluator.TraitImpls() {
					if _, ok := targets[name][mName]; ok {
						traitSource = tName
						break
					}
				}
				info.Methods = append(info.Methods, DebugMethodInfo{
					Name:        mName,
					Signature:   formatFnSignature(mName, fnType),
					TraitSource: traitSource,
					IsStatic:    !hasSelfParam(fnType),
				})
			}
		}

		// Implemented Traits
		traitImpls := ctx.Evaluator.TraitImpls()
		var traitNames []string
		for tName, targets := range traitImpls {
			if _, ok := targets[name]; ok {
				traitNames = append(traitNames, tName)
			}
		}
		sort.Strings(traitNames)
		for _, tName := range traitNames {
			targetMethods := traitImpls[tName][name]
			var tMethodInfos []DebugMethodInfo
			var mNames []string
			for m := range targetMethods {
				mNames = append(mNames, m)
			}
			sort.Strings(mNames)
			for _, m := range mNames {
				sym := targetMethods[m]
				if fnType, ok := sym.Type.(*loneanalyzer.FnType); ok {
					tMethodInfos = append(tMethodInfos, DebugMethodInfo{
						Name:        m,
						Signature:   formatFnSignature(m, fnType),
						TraitSource: tName,
						IsStatic:    !hasSelfParam(fnType),
					})
				}
			}
			info.ImplementedTraits = append(info.ImplementedTraits, DebugTraitImplInfo{
				TraitName: tName,
				Methods:   tMethodInfos,
			})
		}
	}

	// Constructors (functions returning this struct)
	if ctx.TypedProgram != nil {
		for _, d := range ctx.TypedProgram.Decls {
			if fn, ok := d.(*loneanalyzer.TFnDec); ok {
				if fn.ReturnType != nil && fn.ReturnType.Equals(st) {
					info.Constructors = append(info.Constructors, formatCleanTFnSignature(fn, st.Name))
				}
			}
		}
	}

	return info
}

func inspectTrait(ctx *DebugContext, name string, tr *loneanalyzer.TraitType) *DebugSymbolInfo {
	info := &DebugSymbolInfo{
		Name:     name,
		Kind:     DebugKindTrait,
		Scope:    ctx.ScopeName,
		FilePath: ctx.FilePath,
	}

	if raw, ok := ctx.AstNodeMap[name].(*loneast.TYPE_DEC); ok {
		info.IsPublic = raw.IsPublic
		info.DefinedPos = formatPos(ctx.FilePath, raw.Pos)
	}

	var methodNames []string
	for mName := range tr.Methods {
		methodNames = append(methodNames, mName)
	}
	sort.Strings(methodNames)
	for _, mName := range methodNames {
		mType := tr.Methods[mName]
		info.TraitMethods = append(info.TraitMethods, DebugMethodInfo{
			Name:        mName,
			Signature:   formatFnSignature(mName, mType),
			TraitSource: name,
			IsStatic:    !hasSelfParam(mType),
		})
	}

	// Concrete Implementors
	if ctx.Evaluator != nil {
		if targets, ok := ctx.Evaluator.TraitImpls()[name]; ok {
			for targetName := range targets {
				info.Implementors = append(info.Implementors, targetName)
			}
			sort.Strings(info.Implementors)
		}
	}

	return info
}

func inspectFunction(ctx *DebugContext, fn *loneanalyzer.TFnDec) *DebugSymbolInfo {
	info := &DebugSymbolInfo{
		Name:       fn.Name,
		Kind:       DebugKindFn,
		Scope:      ctx.ScopeName,
		FilePath:   ctx.FilePath,
		DefinedPos: formatPos(ctx.FilePath, fn.Pos()),
		Signature:  formatTFnSignature(fn),
		ReturnType: fn.ReturnType.String(),
	}

	if raw, ok := ctx.AstNodeMap[fn.Name].(*loneast.FN_DEC); ok {
		info.IsPublic = raw.IsPublic
	}

	for _, p := range fn.Params {
		info.Params = append(info.Params, DebugVarInfo{
			Name:         p.Name,
			Type:         p.Type.String(),
			Storage:      "datastack frame (parameter)",
			AddressTaken: p.AddressTaken,
			Pos:          formatPos(ctx.FilePath, p.Pos()),
		})
	}

	for _, v := range collectLocalVars(&fn.Body) {
		info.LocalVars = append(info.LocalVars, DebugVarInfo{
			Name:         v.Name,
			Type:         v.Type.String(),
			Storage:      "datastack frame",
			AddressTaken: v.AddressTaken,
			Pos:          formatPos(ctx.FilePath, v.Pos()),
		})
	}

	return info
}

func inspectGlobalVar(ctx *DebugContext, v *loneanalyzer.TVarDec) *DebugSymbolInfo {
	info := &DebugSymbolInfo{
		Name:          v.Name,
		Kind:          DebugKindVar,
		Scope:         ctx.ScopeName,
		FilePath:      ctx.FilePath,
		DefinedPos:    formatPos(ctx.FilePath, v.Pos()),
		DeclaredType:  v.Type.String(),
		ResolvedType:  v.Type.String(),
		IsInitialized: v.Value != nil,
		Storage:       "static linear memory",
		AddressTaken:  v.AddressTaken,
	}

	if raw, ok := ctx.AstNodeMap[v.Name].(*loneast.BIND_DEC); ok {
		info.IsPublic = raw.IsPublic
	}

	return info
}

// DumpScope produces a full inventory of all structs, traits, functions, globals, and imports in a scope.
func DumpScope(scope string) (*DebugScopeDump, error) {
	ctx, err := LoadDebugContext(scope)
	if err != nil {
		return nil, err
	}

	dump := &DebugScopeDump{
		Scope:    ctx.ScopeName,
		FilePath: ctx.FilePath,
	}

	// 1. Structs
	structsSeen := make(map[string]bool)
	if ctx.TypedProgram != nil {
		for _, d := range ctx.TypedProgram.Decls {
			if td, ok := d.(*loneanalyzer.TTypeDec); ok {
				if st, ok := td.Typ.(*loneanalyzer.StructType); ok {
					if structsSeen[st.Name] {
						continue
					}
					structsSeen[st.Name] = true

					methodCount := 0
					if ctx.Evaluator != nil {
						methodCount = len(ctx.Evaluator.MethodImpls()[st.Name])
					}
					traitCount := 0
					if ctx.Evaluator != nil {
						for _, targets := range ctx.Evaluator.TraitImpls() {
							if _, ok := targets[st.Name]; ok {
								traitCount++
							}
						}
					}

					isPub := false
					if raw, ok := ctx.AstNodeMap[st.Name].(*loneast.TYPE_DEC); ok {
						isPub = raw.IsPublic
					}

					dump.Structs = append(dump.Structs, DebugStructSummary{
						Name:        st.Name,
						Size:        st.Size,
						Align:       st.Align,
						FieldCount:  len(st.Fields),
						MethodCount: methodCount,
						TraitCount:  traitCount,
						IsPublic:    isPub,
					})
				}
			}
		}
	}

	// 2. Traits
	traitsSeen := make(map[string]bool)
	if ctx.TypedProgram != nil {
		for _, d := range ctx.TypedProgram.Decls {
			if td, ok := d.(*loneanalyzer.TTypeDec); ok {
				if tr, ok := td.Typ.(*loneanalyzer.TraitType); ok {
					if traitsSeen[tr.Name] {
						continue
					}
					traitsSeen[tr.Name] = true

					var implementors []string
					if ctx.Evaluator != nil {
						if targets, ok := ctx.Evaluator.TraitImpls()[tr.Name]; ok {
							for tgt := range targets {
								implementors = append(implementors, tgt)
							}
							sort.Strings(implementors)
						}
					}

					isPub := false
					if raw, ok := ctx.AstNodeMap[tr.Name].(*loneast.TYPE_DEC); ok {
						isPub = raw.IsPublic
					}

					dump.Traits = append(dump.Traits, DebugTraitSummary{
						Name:         tr.Name,
						MethodCount:  len(tr.Methods),
						Implementors: implementors,
						IsPublic:     isPub,
					})
				}
			}
		}
	}

	// 3. Functions
	if ctx.TypedProgram != nil {
		for _, d := range ctx.TypedProgram.Decls {
			if fn, ok := d.(*loneanalyzer.TFnDec); ok {
				// Filter out lowered methods (which contain __)
				if strings.Contains(fn.Name, "__") {
					continue
				}

				isPub := false
				if raw, ok := ctx.AstNodeMap[fn.Name].(*loneast.FN_DEC); ok {
					isPub = raw.IsPublic
				}

				dump.Functions = append(dump.Functions, DebugFnSummary{
					Name:       fn.Name,
					Signature:  formatTFnSignature(fn),
					IsPublic:   isPub,
					LocalCount: len(collectLocalVars(&fn.Body)),
				})
			}
		}
	}

	// 4. Globals
	if ctx.TypedProgram != nil {
		for _, d := range ctx.TypedProgram.Decls {
			if v, ok := d.(*loneanalyzer.TVarDec); ok {
				dump.Globals = append(dump.Globals, DebugVarInfo{
					Name:         v.Name,
					Type:         v.Type.String(),
					Storage:      "static linear memory",
					AddressTaken: v.AddressTaken,
					Pos:          formatPos(ctx.FilePath, v.Pos()),
				})
			}
		}
	}

	// 5. Imports
	for _, raw := range ctx.ResolvedAst {
		switch imp := raw.(type) {
		case *loneast.ImportDecl:
			dump.Imports = append(dump.Imports, DebugImportSummary{
				Source:  imp.Source,
				Alias:   imp.Alias,
				Symbols: imp.Symbols,
			})
		case loneast.ImportDecl:
			dump.Imports = append(dump.Imports, DebugImportSummary{
				Source:  imp.Source,
				Alias:   imp.Alias,
				Symbols: imp.Symbols,
			})
		case *loneast.ResolvedImportDecl:
			dump.Imports = append(dump.Imports, DebugImportSummary{
				Source: imp.Source,
				Alias:  imp.Alias,
			})
		case loneast.ResolvedImportDecl:
			dump.Imports = append(dump.Imports, DebugImportSummary{
				Source: imp.Source,
				Alias:  imp.Alias,
			})
		}
	}

	return dump, nil
}

// Helper inspection finders

func findStructType(ctx *DebugContext, name string) *loneanalyzer.StructType {
	if ctx.Evaluator != nil && ctx.Evaluator.Stack() != nil {
		sym := ctx.Evaluator.Stack().Resolve(name)
		if sym != nil && sym.Kind == loneanalyzer.SymbolType {
			if st, ok := sym.Type.(*loneanalyzer.StructType); ok {
				return st
			}
		}
	}
	if ctx.TypedProgram != nil {
		for _, d := range ctx.TypedProgram.Decls {
			if td, ok := d.(*loneanalyzer.TTypeDec); ok && td.Name == name {
				if st, ok := td.Typ.(*loneanalyzer.StructType); ok {
					return st
				}
			}
		}
	}
	return nil
}

func findTraitType(ctx *DebugContext, name string) *loneanalyzer.TraitType {
	if ctx.Evaluator != nil && ctx.Evaluator.Stack() != nil {
		sym := ctx.Evaluator.Stack().Resolve(name)
		if sym != nil && sym.Kind == loneanalyzer.SymbolType {
			if tr, ok := sym.Type.(*loneanalyzer.TraitType); ok {
				return tr
			}
		}
	}
	if ctx.TypedProgram != nil {
		for _, d := range ctx.TypedProgram.Decls {
			if td, ok := d.(*loneanalyzer.TTypeDec); ok && td.Name == name {
				if tr, ok := td.Typ.(*loneanalyzer.TraitType); ok {
					return tr
				}
			}
		}
	}
	return nil
}

func findFunction(ctx *DebugContext, name string) *loneanalyzer.TFnDec {
	if ctx.TypedProgram != nil {
		for _, d := range ctx.TypedProgram.Decls {
			if fn, ok := d.(*loneanalyzer.TFnDec); ok && fn.Name == name {
				return fn
			}
		}
	}
	return nil
}

func findGlobalVar(ctx *DebugContext, name string) *loneanalyzer.TVarDec {
	if ctx.TypedProgram != nil {
		for _, d := range ctx.TypedProgram.Decls {
			if v, ok := d.(*loneanalyzer.TVarDec); ok && v.Name == name {
				return v
			}
		}
	}
	return nil
}

func findStructMethod(ctx *DebugContext, structName, methodName string) *DebugMethodInfo {
	if ctx.Evaluator == nil {
		return nil
	}

	var traitSource string
	for traitName, targetMap := range ctx.Evaluator.TraitImpls() {
		if _, ok := targetMap[structName][methodName]; ok {
			traitSource = traitName
			break
		}
	}

	// 1. Inherent methods
	if mSym, ok := ctx.Evaluator.MethodImpls()[structName][methodName]; ok {
		if fnType, ok := mSym.Type.(*loneanalyzer.FnType); ok {
			return &DebugMethodInfo{
				Name:        methodName,
				Signature:   formatFnSignature(methodName, fnType),
				TraitSource: traitSource,
				IsStatic:    !hasSelfParam(fnType),
			}
		}
	}

	// 2. Trait methods
	for traitName, targetMap := range ctx.Evaluator.TraitImpls() {
		if mSym, ok := targetMap[structName][methodName]; ok {
			if fnType, ok := mSym.Type.(*loneanalyzer.FnType); ok {
				return &DebugMethodInfo{
					Name:        methodName,
					Signature:   formatFnSignature(methodName, fnType),
					TraitSource: traitName,
					IsStatic:    !hasSelfParam(fnType),
				}
			}
		}
	}

	return nil
}

func collectLocalVars(block *loneanalyzer.TBlock) []*loneanalyzer.TVarDec {
	var vars []*loneanalyzer.TVarDec
	var walkBlock func(b *loneanalyzer.TBlock)
	walkBlock = func(b *loneanalyzer.TBlock) {
		if b == nil {
			return
		}
		for _, stmt := range b.Stmts {
			switch s := stmt.(type) {
			case *loneanalyzer.TVarDec:
				vars = append(vars, s)
			case *loneanalyzer.TIfStmt:
				walkBlock(&s.Then)
				if s.Else != nil {
					walkBlock(s.Else)
				}
			case *loneanalyzer.TWhileStmt:
				walkBlock(&s.Body)
			case *loneanalyzer.TLabelDec:
				walkBlock(&s.Body)
			}
		}
	}
	walkBlock(block)
	return vars
}

func hasSelfParam(fn *loneanalyzer.FnType) bool {
	if len(fn.Params) == 0 {
		return false
	}
	return fn.Params[0].Name == "self"
}

func formatFnSignature(name string, fn *loneanalyzer.FnType) string {
	var paramStrs []string
	for _, p := range fn.Params {
		paramStrs = append(paramStrs, fmt.Sprintf("%s: %s", p.Name, p.Type.String()))
	}
	ret := "void"
	if fn.ReturnType != nil {
		ret = fn.ReturnType.String()
	}
	return fmt.Sprintf("fn %s(%s) %s", name, strings.Join(paramStrs, ", "), ret)
}

func formatTFnSignature(fn *loneanalyzer.TFnDec) string {
	return formatCleanTFnSignature(fn, "")
}

func formatCleanTFnSignature(fn *loneanalyzer.TFnDec, structContext string) string {
	var paramStrs []string
	for _, p := range fn.Params {
		paramStrs = append(paramStrs, fmt.Sprintf("%s: %s", p.Name, p.Type.String()))
	}
	ret := "void"
	if fn.ReturnType != nil {
		ret = fn.ReturnType.String()
	}
	name := fn.Name
	if structContext != "" && strings.HasPrefix(name, structContext+"__") {
		name = structContext + "." + strings.TrimPrefix(name, structContext+"__")
	}
	return fmt.Sprintf("fn %s(%s) %s", name, strings.Join(paramStrs, ", "), ret)
}

func formatPos(filePath string, pos loneast.Position) string {
	if pos.Line == 0 && pos.Col == 0 {
		return filePath
	}
	return fmt.Sprintf("%s:%d:%d", filepath.Base(filePath), pos.Line, pos.Col)
}
