package server

import (
	"errors"
	"go/types"

	gopast "github.com/goplus/gop/ast"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification#textDocument_documentLink
func (s *Server) textDocumentDocumentLink(params *DocumentLinkParams) ([]DocumentLink, error) {
	result, spxFile, astFile, err := s.compileAndGetASTFileForDocumentURI(params.TextDocument.URI)
	if err != nil {
		if errors.Is(err, errNoValidSpxFiles) || errors.Is(err, errNoMainSpxFile) {
			return nil, nil
		}
		return nil, err
	}
	if astFile == nil {
		return nil, nil
	}

	var links []DocumentLink

	for refKey, refs := range result.spxResourceRefs {
		for _, ref := range refs {
			startPos := result.fset.Position(ref.Node.Pos())
			if startPos.Filename != spxFile {
				continue
			}
			target := URI(refKey.URI())
			links = append(links, DocumentLink{
				Range: Range{
					Start: FromGopTokenPosition(startPos),
					End:   FromGopTokenPosition(result.fset.Position(ref.Node.End())),
				},
				Target: &target,
				Data: SpxResourceRefDocumentLinkData{
					Kind: ref.Kind,
				},
			})
		}
	}

	gopast.Inspect(astFile, func(node gopast.Node) bool {
		if node == nil || !node.Pos().IsValid() {
			return true
		}
		ident, ok := node.(*gopast.Ident)
		if !ok {
			return true
		}
		obj := result.typeInfo.ObjectOf(ident)
		if obj == nil {
			return true
		}

		var defID SpxDefinitionIdentifier
		if obj.Pkg() == nil {
			def := GetSpxBuiltinDefinition(obj.Name())
			defID = def.ID
		} else {
			switch obj := obj.(type) {
			case *types.Func:
				defs := NewSpxDefinitionsForFunc(result, obj, result.inferSelectorTypeNameForIdent(ident))
				if len(defs) > 0 {
					defID = defs[0].ID
				}
			case *types.Var:
				def := NewSpxDefinitionForVar(result, obj, result.inferSelectorTypeNameForIdent(ident))
				defID = def.ID
			case *types.Const:
				def := NewSpxDefinitionForConst(result, obj)
				defID = def.ID
			case *types.TypeName:
				def := NewSpxDefinitionForType(result, obj)
				defID = def.ID
			case *types.PkgName:
				def := NewSpxDefinitionForPkg(result, obj)
				defID = def.ID
			default:
				return true
			}
		}

		target := URI(defID.String())
		links = append(links, DocumentLink{
			Range: Range{
				Start: FromGopTokenPosition(result.fset.Position(ident.Pos())),
				End:   FromGopTokenPosition(result.fset.Position(ident.End())),
			},
			Target: &target,
		})
		return true
	})

	return links, nil
}
