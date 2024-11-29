package server

import (
	"errors"
	"fmt"
	"go/types"
	"path"
	"slices"
	"strings"

	"github.com/goplus/builder/tools/spxls/internal/vfs"
	gopast "github.com/goplus/gop/ast"
	gopparser "github.com/goplus/gop/parser"
	goptoken "github.com/goplus/gop/token"
	goptypesutil "github.com/goplus/gop/x/typesutil"
	"github.com/goplus/mod/gopmod"
	gopmodload "github.com/goplus/mod/modload"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification#textDocument_documentLink
func (s *Server) textDocumentDocumentLink(params *DocumentLinkParams) ([]DocumentLink, error) {
	spxFiles, err := s.spxFiles()
	if err != nil {
		return nil, fmt.Errorf("get spx files failed: %w", err)
	}

	spxFile, err := s.fromDocumentURI(params.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to get path from document uri: %w", err)
	}
	if !slices.Contains(spxFiles, spxFile) {
		return nil, nil // Cannot find the file in workspace.
	}

	fset := goptoken.NewFileSet()
	gpfs := vfs.NewGopParserFS(s.workspaceRootFS)
	mainPkgFiles := make(map[string]*gopast.File)
	for _, spxFile := range spxFiles {
		f, err := gopparser.ParseFSEntry(fset, gpfs, spxFile, nil, gopparser.Config{
			Mode: gopparser.AllErrors | gopparser.ParseComments,
		})
		if err != nil {
			return nil, nil // Return no links if parsing fails since diagnostics will be shown instead.
		}
		if f.Name.Name == "main" {
			mainPkgFiles[spxFile] = f
		}
	}
	if len(mainPkgFiles) == 0 {
		return nil, errors.New("no valid spx files found in main package")
	}

	// TODO: Find another way to extract type information.
	mod := gopmod.New(gopmodload.Default)
	if err := mod.ImportClasses(); err != nil {
		return nil, fmt.Errorf("import classes failed: %w", err)
	}
	typeInfo := &goptypesutil.Info{
		Types:      make(map[gopast.Expr]types.TypeAndValue),
		Defs:       make(map[*gopast.Ident]types.Object),
		Uses:       make(map[*gopast.Ident]types.Object),
		Implicits:  make(map[gopast.Node]types.Object),
		Selections: make(map[*gopast.SelectorExpr]*types.Selection),
		Scopes:     make(map[gopast.Node]*types.Scope),
	}
	if err := goptypesutil.NewChecker(
		&types.Config{
			Importer: s.importer,
		},
		&goptypesutil.Config{
			Types: types.NewPackage("main", "main"),
			Fset:  fset,
			Mod:   mod,
		},
		nil,
		typeInfo,
	).Files(nil, gopASTFileMapToSlice(mainPkgFiles)); err != nil {
		return nil, nil // Return no links if type checking fails since diagnostics will be shown instead.
	}

	var spxSpriteTypeNames []string
	for spxFile := range mainPkgFiles {
		spxFileBase := path.Base(spxFile)
		if spxFileBase != "main.spx" {
			spxSpriteTypeName := "main." + strings.TrimSuffix(spxFileBase, ".spx")
			spxSpriteTypeNames = append(spxSpriteTypeNames, spxSpriteTypeName)
		}
	}

	var links []DocumentLink
	gopast.Inspect(mainPkgFiles[spxFile], func(node gopast.Node) bool {
		isInMainSpx := path.Base(spxFile) == "main.spx"
		switch node := node.(type) {
		case *gopast.ValueSpec:
			if !isInMainSpx {
				return true
			}
			for _, name := range node.Names {
				obj := typeInfo.Defs[name]
				if obj == nil {
					continue
				}
				objType, ok := obj.Type().(*types.Named)
				if !ok {
					continue
				}

				var spxResourceSlug string
				switch typeName := objType.String(); typeName {
				case SpxSoundTypeName:
					spxResourceSlug = "sounds"
				case SpxSpriteTypeName:
					spxResourceSlug = "sprites"
				default:
					for _, spxSpriteTypeName := range spxSpriteTypeNames {
						if typeName == spxSpriteTypeName {
							spxResourceSlug = "sprites"
							break
						}
					}
				}
				if spxResourceSlug != "" {
					links = append(links, DocumentLink{
						Range: Range{
							Start: FromGopTokenPosition(fset.Position(name.Pos())),
							End:   FromGopTokenPosition(fset.Position(name.End())),
						},
						Target: toURI(fmt.Sprintf("spx://resources/%s/%s", spxResourceSlug, name.Name)),
					})
				}
			}
		case *gopast.CallExpr:
			var fName string
			switch fun := node.Fun.(type) {
			case *gopast.Ident:
				fName = fun.Name
			case *gopast.SelectorExpr:
				fName = fun.Sel.Name
			default:
				return true
			}

			if len(node.Args) == 0 {
				return true
			}

			switch strings.ToLower(fName) {
			case "play":
				soundName, diags := s.validateSpxGamePlayCall(fset, typeInfo, node)
				if diags == nil && soundName != "" {
					links = append(links, DocumentLink{
						Range: Range{
							Start: FromGopTokenPosition(fset.Position(node.Args[0].Pos())),
							End:   FromGopTokenPosition(fset.Position(node.Args[0].End())),
						},
						Target: toURI(fmt.Sprintf("spx://resources/sounds/%s", soundName)),
					})
				}
			case "onbackdrop":
				backdropName, diags := s.validateSpxGameOrSpriteOnBackdropCall(fset, typeInfo, node)
				if diags == nil && backdropName != "" {
					links = append(links, DocumentLink{
						Range: Range{
							Start: FromGopTokenPosition(fset.Position(node.Args[0].Pos())),
							End:   FromGopTokenPosition(fset.Position(node.Args[0].End())),
						},
						Target: toURI(fmt.Sprintf("spx://resources/backdrops/%s", backdropName)),
					})
				}
			case "setcostume":
				spriteName, costumeName, diags := s.validateSpxSpriteSetCostumeCall(fset, typeInfo, node)
				if diags == nil && spriteName != "" && costumeName != "" {
					links = append(links, DocumentLink{
						Range: Range{
							Start: FromGopTokenPosition(fset.Position(node.Args[0].Pos())),
							End:   FromGopTokenPosition(fset.Position(node.Args[0].End())),
						},
						Target: toURI(fmt.Sprintf("spx://resources/sprites/%s/costumes/%s", spriteName, costumeName)),
					})
				}
			case "animate":
				spriteName, animationName, diags := s.validateSpxSpriteAnimateCall(fset, typeInfo, node)
				if diags == nil && spriteName != "" && animationName != "" {
					links = append(links, DocumentLink{
						Range: Range{
							Start: FromGopTokenPosition(fset.Position(node.Args[0].Pos())),
							End:   FromGopTokenPosition(fset.Position(node.Args[0].End())),
						},
						Target: toURI(fmt.Sprintf("spx://resources/sprites/%s/animations/%s", spriteName, animationName)),
					})
				}
			case "getwidget":
				if len(node.Args) < 2 {
					return true
				}
				widgetName, diags := s.validateSpxGameGetWidgetCall(fset, typeInfo, node)
				if diags == nil && widgetName != "" {
					links = append(links, DocumentLink{
						Range: Range{
							Start: FromGopTokenPosition(fset.Position(node.Args[1].Pos())),
							End:   FromGopTokenPosition(fset.Position(node.Args[1].End())),
						},
						Target: toURI(fmt.Sprintf("spx://resources/widgets/%s", widgetName)),
					})
				}
			}
		}
		return true
	})
	return links, nil
}

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification#documentLink_resolve
func (s *Server) documentLinkResolve(params *DocumentLink) (*DocumentLink, error) {
	return params, nil // No additional resolution is needed at this time.
}
