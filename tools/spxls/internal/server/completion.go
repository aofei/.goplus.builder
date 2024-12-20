package server

import (
	"errors"
	"go/types"
	"slices"
	"strings"

	"github.com/goplus/builder/tools/spxls/internal/util"
	gopast "github.com/goplus/gop/ast"
	goptoken "github.com/goplus/gop/token"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_completion
func (s *Server) textDocumentCompletion(params *CompletionParams) ([]CompletionItem, error) {
	result, _, astFile, err := s.compileAndGetASTFileForDocumentURI(params.TextDocument.URI)
	if err != nil {
		if errors.Is(err, errNoValidSpxFiles) || errors.Is(err, errNoMainSpxFile) {
			return nil, nil
		}
		return nil, err
	}
	if astFile == nil {
		return nil, nil
	}

	tokenFile := result.fset.File(astFile.Pos())
	line := min(int(params.Position.Line)+1, tokenFile.LineCount())
	lineStart := tokenFile.LineStart(line)
	pos := tokenFile.Pos(tokenFile.Offset(lineStart) + int(params.Position.Character))
	if !pos.IsValid() {
		return nil, nil
	}
	innermostScope := result.innermostScopeAt(pos)
	if innermostScope == nil {
		return nil, nil
	}

	ctx := &completionContext{
		result:         result,
		astFile:        astFile,
		pos:            pos,
		innermostScope: innermostScope,
		fileScope:      result.typeInfo.Scopes[astFile],
	}
	ctx.analyzeCompletionContext()
	return ctx.collectCompletionItems()
}

// completionKind represents different kinds of completion contexts.
type completionKind int

const (
	completionKindUnknown completionKind = iota
	completionKindKeyword
	completionKindImport
	completionKindDot
	completionKindCall
	completionKindStructLiteral
	completionKindAssignment
	completionKindSwitchCase
	completionKindDefer
	completionKindSelect
	completionKindRange
)

// completionContext represents the context for completion operations.
type completionContext struct {
	result         *compileResult
	astFile        *gopast.File
	pos            goptoken.Pos
	innermostScope *types.Scope
	fileScope      *types.Scope

	kind         completionKind
	path         *gopast.SelectorExpr
	enclosing    gopast.Node
	expectedType types.Type

	inStruct     *types.Struct
	inInterface  *types.Interface
	assignTarget types.Type

	inSwitch    *gopast.SwitchStmt
	inGo        bool
	returnIndex int
}

// analyzeCompletionContext analyzes the completion context to determine the
// kind of completion needed.
func (ctx *completionContext) analyzeCompletionContext() {
	path, _ := util.PathEnclosingInterval(ctx.astFile, ctx.pos, ctx.pos)
	if len(path) == 0 {
		return
	}
	for i, node := range path {
		switch n := node.(type) {
		case *gopast.ImportSpec:
			ctx.kind = completionKindImport
		case *gopast.SelectorExpr:
			if n.Sel == nil || n.Sel.Pos() >= ctx.pos {
				ctx.kind = completionKindDot
				ctx.path = n
			}
		case *gopast.CallExpr:
			if n.Lparen.IsValid() && n.Lparen < ctx.pos {
				ctx.kind = completionKindCall
			}
		case *gopast.GoStmt:
			ctx.inGo = true
			ctx.enclosing = n.Call
			ctx.kind = completionKindCall
		case *gopast.DeferStmt:
			ctx.kind = completionKindDefer
			ctx.enclosing = n.Call
			ctx.kind = completionKindCall
		case *gopast.ReturnStmt:
			sig := ctx.enclosingFunction(path[i+1:])
			if sig == nil {
				break
			}
			results := sig.Results()
			if results.Len() == 0 {
				break
			}
			ctx.returnIndex = ctx.findReturnValueIndex(n)
			if ctx.returnIndex >= 0 && ctx.returnIndex < results.Len() {
				ctx.expectedType = results.At(ctx.returnIndex).Type()
			}
		case *gopast.AssignStmt:
			if n.Tok != goptoken.ASSIGN && n.Tok != goptoken.DEFINE {
				break
			}
			for j, rhs := range n.Rhs {
				if rhs.Pos() > ctx.pos || ctx.pos > rhs.End() {
					continue
				}
				if j < len(n.Lhs) {
					if tv, ok := ctx.result.typeInfo.Types[n.Lhs[j]]; ok {
						ctx.kind = completionKindAssignment
						ctx.assignTarget = tv.Type
					}
					break
				}
			}
		case *gopast.CompositeLit:
			tv, ok := ctx.result.typeInfo.Types[n]
			if !ok {
				break
			}
			typ, ok := tv.Type.Underlying().(*types.Struct)
			if !ok {
				break
			}
			ctx.kind = completionKindStructLiteral
			ctx.inStruct = typ
		case *gopast.InterfaceType:
			tv, ok := ctx.result.typeInfo.Types[n]
			if !ok {
				break
			}
			iface, ok := tv.Type.(*types.Interface)
			if !ok {
				break
			}
			ctx.inInterface = iface
		case *gopast.SwitchStmt:
			ctx.inSwitch = n
		case *gopast.SelectStmt:
			ctx.kind = completionKindSelect
			ctx.expectedType = types.NewChan(types.SendRecv, nil)
		case *gopast.RangeStmt:
			ctx.kind = completionKindRange
			if n.X == nil {
				continue
			}
			tv, ok := ctx.result.typeInfo.Types[n.X]
			if !ok {
				break
			}
			ctx.expectedType = tv.Type
		}
	}
}

// findReturnValueIndex finds the index of the return value at the current position.
func (ctx *completionContext) findReturnValueIndex(ret *gopast.ReturnStmt) int {
	if len(ret.Results) == 0 {
		return 0
	}
	for i, expr := range ret.Results {
		if ctx.pos >= expr.Pos() && ctx.pos <= expr.End() {
			return i
		}
	}
	if ctx.pos > ret.Results[len(ret.Results)-1].End() {
		return len(ret.Results)
	}
	return -1
}

// enclosingFunction gets the function signature containing the current position.
func (ctx *completionContext) enclosingFunction(path []gopast.Node) *types.Signature {
	for _, node := range path {
		funcDecl, ok := node.(*gopast.FuncDecl)
		if !ok {
			continue
		}
		obj := ctx.result.typeInfo.ObjectOf(funcDecl.Name)
		if obj == nil {
			break
		}
		fun, ok := obj.(*types.Func)
		if !ok {
			break
		}
		return fun.Type().(*types.Signature)
	}
	return nil
}

// collectCompletionItems collects completion items based on the completion context.
func (ctx *completionContext) collectCompletionItems() ([]CompletionItem, error) {
	var (
		items []CompletionItem
		err   error
	)
	switch ctx.kind {
	case completionKindImport:
		items, err = ctx.collectImportCompletions()
	case completionKindDot:
		items, err = ctx.collectDotCompletions()
	case completionKindStructLiteral:
		items, err = ctx.collectStructLiteralCompletions()
	case completionKindAssignment:
		items, err = ctx.collectTypeSpecificCompletions()
	case completionKindCall:
		items, err = ctx.collectCallCompletions()
	case completionKindDefer:
		items, err = ctx.collectDeferCompletions()
	case completionKindRange:
		items, err = ctx.collectRangeCompletions()
	case completionKindSelect:
		items, err = ctx.collectSelectCompletions()
	case completionKindSwitchCase:
		items, err = ctx.collectSwitchCaseCompletions()
	default:
		items, err = ctx.collectGeneralCompletions()
	}
	if err != nil {
		return nil, err
	}
	sortCompletionItems(items)
	return items, nil
}

// collectDotCompletions collects dot completions for member access.
func (ctx *completionContext) collectDotCompletions() ([]CompletionItem, error) {
	if ctx.path == nil {
		return nil, nil
	}

	tv, ok := ctx.result.typeInfo.Types[ctx.path.X]
	if !ok {
		return nil, nil
	}

	var items []CompletionItem
	seenLabels := make(map[string]bool)

	typ := tv.Type
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	if named, ok := typ.(*types.Named); ok {
		obj := named.Obj()
		if obj.Pkg() != nil {
			for i := 0; i < named.NumMethods(); i++ {
				method := named.Method(i)
				if !method.Exported() && method.Pkg() != ctx.result.mainPkg {
					continue
				}

				defs := NewSpxDefinitionsForFunc(ctx.result, method, obj.Name())
				for _, def := range defs {
					if !seenLabels[def.CompletionItemLabel] {
						items = append(items, def.CompletionItem())
						seenLabels[def.CompletionItemLabel] = true
					}
				}
			}
		}
	}

	if strct, ok := typ.Underlying().(*types.Struct); ok {
		for i := 0; i < strct.NumFields(); i++ {
			field := strct.Field(i)
			if !field.Exported() && field.Pkg() != ctx.result.mainPkg {
				continue
			}

			def := NewSpxDefinitionForVar(ctx.result, field, "")
			if !seenLabels[def.CompletionItemLabel] {
				items = append(items, def.CompletionItem())
				seenLabels[def.CompletionItemLabel] = true
			}
		}
	}

	return items, nil
}

// collectImportCompletions collects import completions.
func (ctx *completionContext) collectImportCompletions() ([]CompletionItem, error) {
	var items []CompletionItem
	seenPkgs := make(map[string]bool)

	for _, pkg := range ctx.result.mainPkg.Imports() {
		if seenPkgs[pkg.Path()] {
			continue
		}
		def := NewSpxDefinitionForPkgName(ctx.result, types.NewPkgName(goptoken.NoPos, ctx.result.mainPkg, pkg.Name(), pkg))
		items = append(items, def.CompletionItem())
		seenPkgs[pkg.Path()] = true
	}

	return items, nil
}

// collectCallCompletions collects function call completions.
func (ctx *completionContext) collectCallCompletions() ([]CompletionItem, error) {
	callExpr, ok := ctx.enclosing.(*gopast.CallExpr)
	if !ok {
		return nil, nil
	}

	tv, ok := ctx.result.typeInfo.Types[callExpr.Fun]
	if !ok {
		return nil, nil
	}

	sig, ok := tv.Type.(*types.Signature)
	if !ok {
		return nil, nil
	}

	argIndex := ctx.getCurrentArgIndex(callExpr)
	if argIndex < 0 {
		return nil, nil
	}

	var expectedType types.Type
	if argIndex < sig.Params().Len() {
		expectedType = sig.Params().At(argIndex).Type()
	} else if sig.Variadic() && argIndex >= sig.Params().Len()-1 {
		expectedType = sig.Params().At(sig.Params().Len() - 1).Type().(*types.Slice).Elem()
	}

	ctx.expectedType = expectedType
	return ctx.collectTypeSpecificCompletions()
}

// getCurrentArgIndex gets the current argument index in a function call.
func (ctx *completionContext) getCurrentArgIndex(callExpr *gopast.CallExpr) int {
	if len(callExpr.Args) == 0 {
		return 0
	}

	for i, arg := range callExpr.Args {
		if ctx.pos >= arg.Pos() && ctx.pos <= arg.End() {
			return i
		}
	}

	if ctx.pos > callExpr.Args[len(callExpr.Args)-1].End() {
		return len(callExpr.Args)
	}

	return -1
}

// collectTypeSpecificCompletions collects type-specific completions.
func (ctx *completionContext) collectTypeSpecificCompletions() ([]CompletionItem, error) {
	var items []CompletionItem
	seenLabels := make(map[string]bool)

	if ctx.expectedType == nil {
		return ctx.collectGeneralCompletions()
	}

	scope := ctx.innermostScope
	for scope != nil {
		for _, name := range scope.Names() {
			if seenLabels[name] {
				continue
			}

			obj := scope.Lookup(name)
			if obj == nil || !obj.Exported() && obj.Pkg() != ctx.result.mainPkg {
				continue
			}

			if isTypeCompatible(obj.Type(), ctx.expectedType) {
				var def SpxDefinition
				switch obj := obj.(type) {
				case *types.Var:
					def = NewSpxDefinitionForVar(ctx.result, obj, "")
				case *types.Const:
					def = NewSpxDefinitionForConst(ctx.result, obj)
				case *types.Func:
					defs := NewSpxDefinitionsForFunc(ctx.result, obj, "")
					if len(defs) > 0 {
						def = defs[0]
					}
				}

				if def.CompletionItemLabel != "" {
					items = append(items, def.CompletionItem())
					seenLabels[def.CompletionItemLabel] = true
				}
			}
		}
		scope = scope.Parent()
	}

	return items, nil
}

// isTypeCompatible checks if two types are compatible.
func isTypeCompatible(got, want types.Type) bool {
	if got == nil || want == nil {
		return false
	}

	if iface, ok := want.(*types.Interface); ok {
		return types.Implements(got, iface)
	}

	if ptr, ok := want.(*types.Pointer); ok {
		if gotPtr, ok := got.(*types.Pointer); ok {
			return types.Identical(ptr.Elem(), gotPtr.Elem())
		}
		return types.Identical(got, ptr.Elem())
	}

	if slice, ok := want.(*types.Slice); ok {
		if gotSlice, ok := got.(*types.Slice); ok {
			return types.Identical(slice.Elem(), gotSlice.Elem())
		}
		return false
	}

	if ch, ok := want.(*types.Chan); ok {
		if gotCh, ok := got.(*types.Chan); ok {
			return types.Identical(ch.Elem(), gotCh.Elem()) &&
				(ch.Dir() == types.SendRecv || ch.Dir() == gotCh.Dir())
		}
		return false
	}

	if types.AssignableTo(got, want) {
		return true
	}

	if named, ok := got.(*types.Named); ok {
		return types.AssignableTo(named.Underlying(), want)
	}

	return false
}

// collectGeneralCompletions collects general completions.
func (ctx *completionContext) collectGeneralCompletions() ([]CompletionItem, error) {
	var items []CompletionItem
	seenLabels := make(map[string]bool)

	// Add built-in definitions.
	for _, def := range GetSpxBuiltinDefinitions() {
		if !seenLabels[def.CompletionItemLabel] {
			items = append(items, def.CompletionItem())
			seenLabels[def.CompletionItemLabel] = true
		}
	}

	// Add general definitions.
	for _, def := range SpxGeneralDefinitions {
		if !seenLabels[def.CompletionItemLabel] {
			items = append(items, def.CompletionItem())
			seenLabels[def.CompletionItemLabel] = true
		}
	}

	// Add file scope definitions if in file scope.
	if ctx.innermostScope == ctx.fileScope {
		for _, def := range SpxFileScopeDefinitions {
			if !seenLabels[def.CompletionItemLabel] {
				items = append(items, def.CompletionItem())
				seenLabels[def.CompletionItemLabel] = true
			}
		}
	}

	// Add all visible objects in the scope.
	scope := ctx.innermostScope
	for scope != nil {
		for _, name := range scope.Names() {
			if seenLabels[name] {
				continue
			}

			obj := scope.Lookup(name)
			if obj == nil || !obj.Exported() && obj.Pkg() != ctx.result.mainPkg {
				continue
			}

			var def SpxDefinition
			switch obj := obj.(type) {
			case *types.Var:
				def = NewSpxDefinitionForVar(ctx.result, obj, "")
			case *types.Const:
				def = NewSpxDefinitionForConst(ctx.result, obj)
			case *types.TypeName:
				def = NewSpxDefinitionForType(ctx.result, obj)
			case *types.Func:
				defs := NewSpxDefinitionsForFunc(ctx.result, obj, "")
				if len(defs) > 0 {
					def = defs[0]
				}
			case *types.PkgName:
				def = NewSpxDefinitionForPkgName(ctx.result, obj)
			}

			if def.CompletionItemLabel != "" {
				items = append(items, def.CompletionItem())
				seenLabels[def.CompletionItemLabel] = true
			}
		}
		scope = scope.Parent()
	}

	return items, nil
}

// collectStructLiteralCompletions collects struct literal completions.
func (ctx *completionContext) collectStructLiteralCompletions() ([]CompletionItem, error) {
	if ctx.inStruct == nil {
		return nil, nil
	}

	var items []CompletionItem
	seenFields := make(map[string]bool)

	// Collect already used fields.
	if composite, ok := ctx.enclosing.(*gopast.CompositeLit); ok {
		for _, elem := range composite.Elts {
			if kv, ok := elem.(*gopast.KeyValueExpr); ok {
				if ident, ok := kv.Key.(*gopast.Ident); ok {
					seenFields[ident.Name] = true
				}
			}
		}
	}

	// Add unused fields.
	for i := 0; i < ctx.inStruct.NumFields(); i++ {
		field := ctx.inStruct.Field(i)
		if !field.Exported() && field.Pkg() != ctx.result.mainPkg {
			continue
		}
		if seenFields[field.Name()] {
			continue
		}

		def := NewSpxDefinitionForVar(ctx.result, field, "")
		def.CompletionItemInsertText = field.Name() + ": ${1:}"
		def.CompletionItemInsertTextFormat = SnippetTextFormat
		items = append(items, def.CompletionItem())
	}

	return items, nil
}

// collectSwitchCaseCompletions collects switch/case completions.
func (ctx *completionContext) collectSwitchCaseCompletions() ([]CompletionItem, error) {
	if ctx.inSwitch == nil {
		return nil, nil
	}

	var items []CompletionItem
	seenLabels := make(map[string]bool)

	var switchType types.Type
	if ctx.inSwitch.Tag != nil {
		if tv, ok := ctx.result.typeInfo.Types[ctx.inSwitch.Tag]; ok {
			switchType = tv.Type
		}
	}

	if ctx.inSwitch.Tag == nil {
		for _, typ := range []string{"int", "string", "bool", "error"} {
			if !seenLabels[typ] {
				def := GetSpxBuiltinDefinition(typ)
				items = append(items, def.CompletionItem())
				seenLabels[typ] = true
			}
		}
		return items, nil
	}

	if named, ok := switchType.(*types.Named); ok {
		if named.Obj().Pkg() != nil {
			scope := named.Obj().Pkg().Scope()
			for _, name := range scope.Names() {
				obj := scope.Lookup(name)
				if c, ok := obj.(*types.Const); ok {
					if types.Identical(c.Type(), switchType) {
						def := NewSpxDefinitionForConst(ctx.result, c)
						items = append(items, def.CompletionItem())
						seenLabels[name] = true
					}
				}
			}
		}
	}

	return items, nil
}

// collectDeferCompletions collects defer statement completions.
func (ctx *completionContext) collectDeferCompletions() ([]CompletionItem, error) {
	return ctx.collectCallCompletions()
}

// collectSelectCompletions collects select statement completions.
func (ctx *completionContext) collectSelectCompletions() ([]CompletionItem, error) {
	var items []CompletionItem
	items = append(items, CompletionItem{
		Label:            "case",
		Kind:             KeywordCompletion,
		InsertText:       "case ${1:ch} <- ${2:value}:$0",
		InsertTextFormat: util.ToPtr(SnippetTextFormat),
	})
	items = append(items, CompletionItem{
		Label:            "default",
		Kind:             KeywordCompletion,
		InsertText:       "default:$0",
		InsertTextFormat: util.ToPtr(SnippetTextFormat),
	})
	return items, nil
}

// collectRangeCompletions collects range statement completions.
func (ctx *completionContext) collectRangeCompletions() ([]CompletionItem, error) {
	if ctx.expectedType == nil {
		return nil, nil
	}

	var items []CompletionItem
	switch ctx.expectedType.Underlying().(type) {
	case *types.Slice:
		items = append(items, CompletionItem{
			Label:            "range slice",
			Kind:             SnippetCompletion,
			InsertText:       "for ${1:i}, ${2:v} := range ${3:slice} {\n\t$0\n}",
			InsertTextFormat: util.ToPtr(SnippetTextFormat),
		})
	case *types.Map:
		items = append(items, CompletionItem{
			Label:            "range map",
			Kind:             SnippetCompletion,
			InsertText:       "for ${1:k}, ${2:v} := range ${3:map} {\n\t$0\n}",
			InsertTextFormat: util.ToPtr(SnippetTextFormat),
		})
	case *types.Chan:
		items = append(items, CompletionItem{
			Label:            "range channel",
			Kind:             SnippetCompletion,
			InsertText:       "for ${1:v} := range ${2:ch} {\n\t$0\n}",
			InsertTextFormat: util.ToPtr(SnippetTextFormat),
		})
	}
	return items, nil
}

// completionItemKindPriority is the priority order for different completion
// item kinds.
var completionItemKindPriority = map[CompletionItemKind]int{
	VariableCompletion:  1,
	FieldCompletion:     2,
	MethodCompletion:    3,
	FunctionCompletion:  4,
	ConstantCompletion:  5,
	ClassCompletion:     6,
	InterfaceCompletion: 7,
	ModuleCompletion:    8,
	KeywordCompletion:   9,
}

// sortCompletionItems sorts completion items.
func sortCompletionItems(items []CompletionItem) {
	slices.SortStableFunc(items, func(a, b CompletionItem) int {
		if p1, p2 := completionItemKindPriority[a.Kind], completionItemKindPriority[b.Kind]; p1 != p2 {
			return p1 - p2
		}
		return strings.Compare(a.Label, b.Label)
	})
}
