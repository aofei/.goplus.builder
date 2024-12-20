package server

import (
	"errors"
	"go/types"
	"strings"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification#textDocument_hover
func (s *Server) textDocumentHover(params *HoverParams) (*Hover, error) {
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

	if refKey, ref := result.spxResourceRefAtASTFilePosition(astFile, params.Position); refKey != nil {
		return &Hover{
			Contents: MarkupContent{
				Kind:  Markdown,
				Value: spxResourceToMarkdown(refKey),
			},
			Range: Range{
				Start: FromGopTokenPosition(result.fset.Position(ref.Node.Pos())),
				End:   FromGopTokenPosition(result.fset.Position(ref.Node.End())),
			},
		}, nil
	}

	ident, obj := result.identAndObjectAtASTFilePosition(astFile, params.Position)
	if obj == nil {
		return nil, nil
	}

	var hoverContent strings.Builder
	if obj.Pkg() == nil {
		hoverContent.WriteString(GetSpxBuiltinDefinition(obj.Name()).Markdown())
	} else {
		switch obj := obj.(type) {
		case *types.Var:
			hoverContent.WriteString(NewSpxDefinitionForVar(result, obj, result.inferSelectorTypeNameForIdent(ident)).Markdown())
		case *types.Const:
			hoverContent.WriteString(NewSpxDefinitionForConst(result, obj).Markdown())
		case *types.TypeName:
			hoverContent.WriteString(NewSpxDefinitionForType(result, obj).Markdown())
		case *types.Func:
			for _, def := range NewSpxDefinitionsForFunc(result, obj, result.inferSelectorTypeNameForIdent(ident)) {
				hoverContent.WriteString(def.Markdown())
			}
		case *types.PkgName:
			hoverContent.WriteString(NewSpxDefinitionForPkgName(result, obj).Markdown())
		default:
			return nil, nil
		}
	}
	return &Hover{
		Contents: MarkupContent{
			Kind:  Markdown,
			Value: hoverContent.String(),
		},
		Range: Range{
			Start: FromGopTokenPosition(result.fset.Position(ident.Pos())),
			End:   FromGopTokenPosition(result.fset.Position(ident.End())),
		},
	}, nil
}
