package server

import (
	"errors"
	"go/types"

	gopast "github.com/goplus/gop/ast"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_declaration
func (s *Server) textDocumentDeclaration(params *DeclarationParams) (any, error) {
	return s.textDocumentDefinition(&DefinitionParams{
		TextDocumentPositionParams: params.TextDocumentPositionParams,
		WorkDoneProgressParams:     params.WorkDoneProgressParams,
		PartialResultParams:        params.PartialResultParams,
	})
}

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_definition
func (s *Server) textDocumentDefinition(params *DefinitionParams) (any, error) {
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

	_, obj := result.identAndObjectAtASTFilePosition(astFile, params.Position)
	if !isMainPkgObject(obj) {
		return nil, nil
	}

	locations := s.findDefinitionLocations(result, obj)
	if len(locations) > 0 {
		locations = deduplicateLocations(locations)
		if len(locations) == 1 {
			return locations[0], nil
		}
		return locations, nil
	}
	return nil, nil
}

// findDefinitionLocations returns all locations where the given object is defined.
func (s *Server) findDefinitionLocations(result *compileResult, obj types.Object) []Location {
	var locations []Location
	for ident, objDef := range result.typeInfo.Defs {
		if objDef == obj {
			locations = append(locations, s.createLocationFromIdent(result.fset, ident))
		}
	}
	return locations
}

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_typeDefinition
func (s *Server) textDocumentTypeDefinition(params *TypeDefinitionParams) (any, error) {
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

	_, obj := result.identAndObjectAtASTFilePosition(astFile, params.Position)
	if !isMainPkgObject(obj) {
		return nil, nil
	}

	objType := obj.Type()
	if ptr, ok := objType.(*types.Pointer); ok {
		objType = ptr.Elem()
	}
	named, ok := objType.(*types.Named)
	if !ok {
		return nil, nil
	}

	objPos := named.Obj().Pos()
	if !objPos.IsValid() {
		return nil, nil
	}
	return s.createLocationFromPos(result.fset, objPos), nil
}

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_implementation
func (s *Server) textDocumentImplementation(params *ImplementationParams) (any, error) {
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

	_, obj := result.identAndObjectAtASTFilePosition(astFile, params.Position)
	if !isMainPkgObject(obj) {
		return nil, nil
	}

	if method, ok := obj.(*types.Func); ok && method.Type().(*types.Signature).Recv() != nil {
		if recv := method.Type().(*types.Signature).Recv().Type(); types.IsInterface(recv) {
			locations := s.findImplementingMethodDefinitions(result, recv.(*types.Interface), method.Name())
			return deduplicateLocations(locations), nil
		}
	}

	return s.createLocationFromPos(result.fset, obj.Pos()), nil
}

// findImplementingMethodDefinitions finds the definition locations of all
// methods that implement the given interface method.
func (s *Server) findImplementingMethodDefinitions(result *compileResult, iface *types.Interface, methodName string) []Location {
	var implementations []Location
	for _, obj := range result.typeInfo.Defs {
		if obj == nil {
			continue
		}
		named, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}
		if !types.Implements(named, iface) {
			continue
		}

		for i := range named.NumMethods() {
			method := named.Method(i)
			if method.Name() != methodName {
				continue
			}

			implementations = append(implementations, s.createLocationFromPos(result.fset, method.Pos()))
		}
	}
	return implementations
}

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification/#textDocument_references
func (s *Server) textDocumentReferences(params *ReferenceParams) ([]Location, error) {
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

	_, obj := result.identAndObjectAtASTFilePosition(astFile, params.Position)
	if obj == nil {
		return nil, nil
	}

	var locations []Location

	locations = append(locations, s.findReferenceLocations(result, obj)...)

	if fn, ok := obj.(*types.Func); ok && fn.Type().(*types.Signature).Recv() != nil {
		locations = append(locations, s.handleMethodReferences(result, fn)...)
		locations = append(locations, s.handleEmbeddedFieldReferences(result, obj)...)
	}

	if params.Context.IncludeDeclaration {
		locations = append(locations, s.findDefinitionLocations(result, obj)...)
	}

	return deduplicateLocations(locations), nil
}

// findReferenceLocations returns all locations where the given object is referenced.
func (s *Server) findReferenceLocations(result *compileResult, obj types.Object) []Location {
	var locations []Location
	for ident, objUse := range result.typeInfo.Uses {
		if objUse == obj {
			locations = append(locations, s.createLocationFromIdent(result.fset, ident))
		}
	}
	return locations
}

// handleMethodReferences finds all references to a method, including interface
// implementations and interface method references.
func (s *Server) handleMethodReferences(result *compileResult, fn *types.Func) []Location {
	var locations []Location
	recvType := fn.Type().(*types.Signature).Recv().Type()
	if types.IsInterface(recvType) {
		iface, ok := recvType.(*types.Interface)
		if !ok {
			return nil
		}
		methodName := fn.Name()
		locations = append(locations, s.findEmbeddedInterfaceReferences(result, iface, methodName)...)
		locations = append(locations, s.findImplementingMethodReferences(result, iface, methodName)...)
	} else {
		locations = append(locations, s.findInterfaceMethodReferences(result, fn)...)
	}
	return locations
}

// findEmbeddedInterfaceReferences finds references to methods in interfaces
// that embed the given interface.
func (s *Server) findEmbeddedInterfaceReferences(result *compileResult, iface *types.Interface, methodName string) []Location {
	var locations []Location
	seenIfaces := make(map[*types.Interface]bool)

	var find func(*types.Interface)
	find = func(current *types.Interface) {
		if seenIfaces[current] {
			return
		}
		seenIfaces[current] = true

		if err := result.rangeTypeDecls(func(typeSpec *gopast.TypeSpec, typeName types.Object) error {
			embedIface, ok := typeName.Type().Underlying().(*types.Interface)
			if !ok {
				return nil
			}

			for i := range embedIface.NumEmbeddeds() {
				if types.Identical(embedIface.EmbeddedType(i), current) {
					method, index, _ := types.LookupFieldOrMethod(embedIface, false, typeName.Pkg(), methodName)
					if method != nil && index != nil {
						locations = append(locations, s.findReferenceLocations(result, method)...)
					}
					find(embedIface)
				}
			}
			return nil
		}); err != nil {
			return
		}
	}
	find(iface)
	return locations
}

// findImplementingMethodReferences finds references to all methods that
// implement the given interface method.
func (s *Server) findImplementingMethodReferences(result *compileResult, iface *types.Interface, methodName string) []Location {
	var locations []Location
	if err := result.rangeTypeDecls(func(typeSpec *gopast.TypeSpec, typeName types.Object) error {
		named, ok := typeName.Type().(*types.Named)
		if !ok {
			return nil
		}
		if !types.Implements(named, iface) {
			return nil
		}

		method, index, _ := types.LookupFieldOrMethod(named, false, named.Obj().Pkg(), methodName)
		if method == nil || index == nil {
			return nil
		}
		locations = append(locations, s.findReferenceLocations(result, method)...)
		return nil
	}); err != nil {
		return nil
	}
	return locations
}

// findInterfaceMethodReferences finds references to interface methods that this
// method implements, including methods from embedded interfaces.
func (s *Server) findInterfaceMethodReferences(result *compileResult, fn *types.Func) []Location {
	var locations []Location
	recvType := fn.Type().(*types.Signature).Recv().Type()
	seenIfaces := make(map[*types.Interface]bool)

	if err := result.rangeTypeDecls(func(typeSpec *gopast.TypeSpec, typeName types.Object) error {
		ifaceType, ok := typeName.Type().Underlying().(*types.Interface)
		if !ok {
			return nil
		}
		if !types.Implements(recvType, ifaceType) {
			return nil
		}
		if seenIfaces[ifaceType] {
			return nil
		}
		seenIfaces[ifaceType] = true

		method, index, _ := types.LookupFieldOrMethod(ifaceType, false, typeName.Pkg(), fn.Name())
		if method == nil || index == nil {
			return nil
		}
		locations = append(locations, s.findReferenceLocations(result, method)...)
		locations = append(locations, s.findEmbeddedInterfaceReferences(result, ifaceType, fn.Name())...)
		return nil
	}); err != nil {
		return nil
	}
	return locations
}

// handleEmbeddedFieldReferences finds all references through embedded fields.
func (s *Server) handleEmbeddedFieldReferences(result *compileResult, obj types.Object) []Location {
	var locations []Location
	if fn, ok := obj.(*types.Func); ok {
		recv := fn.Type().(*types.Signature).Recv()
		if recv == nil {
			return nil
		}

		seenTypes := make(map[types.Type]bool)
		if err := result.rangeTypeDecls(func(typeSpec *gopast.TypeSpec, typeName types.Object) error {
			if named, ok := typeName.Type().(*types.Named); ok {
				locations = append(locations, s.findEmbeddedMethodReferences(result, fn, named, recv.Type(), seenTypes)...)
			}
			return nil
		}); err != nil {
			return nil
		}
	}
	return locations
}

// findEmbeddedMethodReferences recursively finds all references to a method
// through embedded fields.
func (s *Server) findEmbeddedMethodReferences(result *compileResult, fn *types.Func, named *types.Named, targetType types.Type, seenTypes map[types.Type]bool) []Location {
	if seenTypes[named] {
		return nil
	}
	seenTypes[named] = true

	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil
	}

	var locations []Location
	hasEmbed := false
	for i := range st.NumFields() {
		field := st.Field(i)
		if !field.Embedded() {
			continue
		}

		if types.Identical(field.Type(), targetType) {
			hasEmbed = true

			method, _, _ := types.LookupFieldOrMethod(named, false, fn.Pkg(), fn.Name())
			if method != nil {
				locations = append(locations, s.findReferenceLocations(result, method)...)
			}
		}

		if fieldNamed, ok := field.Type().(*types.Named); ok {
			locations = append(locations, s.findEmbeddedMethodReferences(result, fn, fieldNamed, targetType, seenTypes)...)
		}
	}
	if hasEmbed {
		if err := result.rangeTypeDecls(func(typeSpec *gopast.TypeSpec, typeName types.Object) error {
			if embedNamed, ok := typeName.Type().(*types.Named); ok {
				locations = append(locations, s.findEmbeddedMethodReferences(result, fn, embedNamed, named, seenTypes)...)
			}
			return nil
		}); err != nil {
			return nil
		}
	}
	return locations
}
