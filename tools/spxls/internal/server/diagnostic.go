package server

import (
	"errors"
	"fmt"
	"go/types"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/goplus/builder/tools/spxls/internal"
	"github.com/goplus/builder/tools/spxls/internal/vfs"
	"github.com/goplus/gogen"
	gopast "github.com/goplus/gop/ast"
	gopparser "github.com/goplus/gop/parser"
	gopscanner "github.com/goplus/gop/scanner"
	goptoken "github.com/goplus/gop/token"
	goptypesutil "github.com/goplus/gop/x/typesutil"
	"github.com/goplus/mod/gopmod"
	gopmodload "github.com/goplus/mod/modload"
)

const (
	SpxGameTypeName       = "github.com/goplus/spx.Game"
	SpxSpriteTypeName     = "github.com/goplus/spx.Sprite"
	SpxSpriteImplTypeName = "github.com/goplus/spx.SpriteImpl"
	SpxSoundTypeName      = "github.com/goplus/spx.Sound"
)

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification#textDocument_diagnostic
func (s *Server) textDocumentDiagnostic(params *DocumentDiagnosticParams) (*DocumentDiagnosticReport, error) {
	diags, err := s.diagnose()
	if err != nil {
		return nil, err
	}

	return &DocumentDiagnosticReport{Value: &RelatedFullDocumentDiagnosticReport{
		FullDocumentDiagnosticReport: FullDocumentDiagnosticReport{
			Kind:  string(DiagnosticFull),
			Items: diags[params.TextDocument.URI],
		},
	}}, nil
}

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.18/specification#workspace_diagnostic
func (s *Server) workspaceDiagnostic(params *WorkspaceDiagnosticParams) (*WorkspaceDiagnosticReport, error) {
	diags, err := s.diagnose()
	if err != nil {
		return nil, err
	}

	var items []WorkspaceDocumentDiagnosticReport
	for file, fileDiags := range diags {
		items = append(items, Or_WorkspaceDocumentDiagnosticReport{
			Value: &WorkspaceFullDocumentDiagnosticReport{
				URI: DocumentURI(file),
				FullDocumentDiagnosticReport: FullDocumentDiagnosticReport{
					Kind:  string(DiagnosticFull),
					Items: fileDiags,
				},
			},
		})
	}
	return &WorkspaceDiagnosticReport{Items: items}, nil
}

// diagnose performs diagnostic checks for spx source files and returns diagnostics for each file.
func (s *Server) diagnose() (map[DocumentURI][]Diagnostic, error) {
	spxFiles, err := s.spxFiles()
	if err != nil {
		return nil, fmt.Errorf("get spx files failed: %w", err)
	}

	diags := make(map[DocumentURI][]Diagnostic, len(spxFiles))

	fset := goptoken.NewFileSet()
	gpfs := vfs.NewGopParserFS(s.workspaceRootFS)
	mainPkgFiles := make(map[string]*gopast.File)
	for _, spxFile := range spxFiles {
		documentURI := s.toDocumentURI(spxFile)
		diags[documentURI] = nil

		f, err := gopparser.ParseFSEntry(fset, gpfs, spxFile, nil, gopparser.Config{
			Mode: gopparser.AllErrors | gopparser.ParseComments,
		})
		if err != nil {
			// Handle parse errors.
			var parseErr gopscanner.ErrorList
			if errors.As(err, &parseErr) {
				for _, e := range parseErr {
					diags[documentURI] = append(diags[documentURI], Diagnostic{
						Severity: SeverityError,
						Range: Range{
							Start: FromGopTokenPosition(e.Pos),
							End:   FromGopTokenPosition(e.Pos),
						},
						Message: e.Msg,
					})
				}
				continue
			}

			// Handle code generation errors.
			var codeErr *gogen.CodeError
			if errors.As(err, &codeErr) {
				position := codeErr.Fset.Position(codeErr.Pos)
				diags[documentURI] = append(diags[documentURI], Diagnostic{
					Severity: SeverityError,
					Range: Range{
						Start: FromGopTokenPosition(position),
						End:   FromGopTokenPosition(position),
					},
					Message: codeErr.Error(),
				})
				continue
			}

			// Handle unknown errors.
			diags[documentURI] = append(diags[documentURI], Diagnostic{
				Severity: SeverityError,
				Range: Range{
					Start: Position{Line: 0, Character: 0},
					End:   Position{Line: 0, Character: 0},
				},
				Message: fmt.Sprintf("failed to parse spx file: %v", err),
			})
			continue
		}
		if f.Name.Name == "main" {
			mainPkgFiles[spxFile] = f
		}
	}
	if len(mainPkgFiles) == 0 {
		return nil, errors.New("no valid spx files found in main package")
	}

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
			Error: func(err error) {
				if typeErr, ok := err.(types.Error); ok {
					position := typeErr.Fset.Position(typeErr.Pos)
					documentURI := s.toDocumentURI(position.Filename)
					diags[documentURI] = append(diags[documentURI], Diagnostic{
						Severity: SeverityError,
						Range: Range{
							Start: FromGopTokenPosition(position),
							End:   FromGopTokenPosition(position),
						},
						Message: typeErr.Msg,
					})
				}
			},
			Importer: internal.NewImporter(fset),
		},
		&goptypesutil.Config{
			Types: types.NewPackage("main", "main"),
			Fset:  fset,
			Mod:   mod,
		},
		nil,
		typeInfo,
	).Files(nil, gopASTFileMapToSlice(mainPkgFiles)); err != nil {
		// Errors should be handled by the type checker.
	}

	for spxFile, gopastFile := range mainPkgFiles {
		documentURI := s.toDocumentURI(spxFile)
		gopast.Inspect(gopastFile, func(node gopast.Node) bool {
			switch node := node.(type) {
			case *gopast.ValueSpec:
				for _, name := range node.Names {
					obj := typeInfo.Defs[name]
					if obj == nil {
						continue
					}
					objType, ok := obj.Type().(*types.Named)
					if !ok {
						continue
					}

					switch objType.String() {
					case SpxSoundTypeName:
						// Check auto-binding variables of sound resources.
						soundName := name.Name
						subDiags := s.validateSoundResource(soundName, fset.Position(name.Pos()), fset.Position(name.End()))
						if subDiags != nil {
							diags[documentURI] = append(diags[documentURI], subDiags...)
						}
					case SpxSpriteTypeName:
						// Check auto-binding variables of sprite resources.
						spriteName := name.Name
						subDiags := s.validateSpriteResource(spriteName, fset.Position(name.Pos()), fset.Position(name.End()))
						if subDiags != nil {
							diags[documentURI] = append(diags[documentURI], subDiags...)
						}
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

				switch strings.ToLower(fName) {
				case "play":
					subDiags := s.validateSpxGamePlayCall(typeInfo, node, fset)
					if subDiags != nil {
						diags[documentURI] = append(diags[documentURI], subDiags...)
					}
				case "onbackdrop":
					subDiags := s.validateSpxGameOrSpriteOnBackdropCall(typeInfo, node, fset)
					if subDiags != nil {
						diags[documentURI] = append(diags[documentURI], subDiags...)
					}
				case "setcostume":
					subDiags := s.validateSpxSpriteSetCostumeCall(typeInfo, node, fset)
					if subDiags != nil {
						diags[documentURI] = append(diags[documentURI], subDiags...)
					}
				case "animate":
					subDiags := s.validateSpxSpriteAnimateCall(typeInfo, node, fset)
					if subDiags != nil {
						diags[documentURI] = append(diags[documentURI], subDiags...)
					}
				case "getwidget":
					subDiags := s.validateSpxGameGetWidgetCall(typeInfo, node, fset)
					if subDiags != nil {
						diags[documentURI] = append(diags[documentURI], subDiags...)
					}
				}
			}
			return true
		})
	}
	return diags, nil
}

// validateSpxGamePlayCall validates a spx.Game.play call.
//
// See https://pkg.go.dev/github.com/goplus/spx#Game.Play__0
func (s *Server) validateSpxGamePlayCall(typeInfo *goptypesutil.Info, callExpr *gopast.CallExpr, fset *goptoken.FileSet) []Diagnostic {
	tv, ok := typeInfo.Types[callExpr.Fun]
	if !ok {
		return nil
	}
	sig, ok := tv.Type.(*types.Signature)
	if !ok {
		return nil
	}
	recv := sig.Recv()
	if recv == nil {
		return nil
	}
	recvType := recv.Type()
	if ptr, ok := recvType.(*types.Pointer); ok {
		recvType = ptr.Elem()
	}
	if recvType.String() != SpxGameTypeName {
		return nil
	}

	if len(callExpr.Args) == 0 {
		return nil
	}
	argTV, ok := typeInfo.Types[callExpr.Args[0]]
	if !ok {
		return nil
	}
	var soundName string
	if types.AssignableTo(argTV.Type, types.Typ[types.String]) {
		switch arg := callExpr.Args[0].(type) {
		case *gopast.BasicLit:
			if arg.Kind != goptoken.STRING {
				return nil
			}
			soundName = arg.Value
		case *gopast.Ident:
			if argTV.Value != nil {
				// If it's a constant, we can get its value.
				soundName = argTV.Value.String()
			} else {
				// There is nothing we can do for string variables.
				return nil
			}
		default:
			return nil
		}
		soundName = soundName[1 : len(soundName)-1] // Unquote the string.
	} else {
		// Check auto-binding variables of sound resources.
		argType := argTV.Type
		if ptr, ok := argType.(*types.Pointer); ok {
			argType = ptr.Elem()
		}
		if argType.String() != SpxSoundTypeName {
			return nil
		}
		if ident, ok := callExpr.Args[0].(*gopast.Ident); ok {
			// The variable name is the sound name per auto-binding rules.
			soundName = ident.Name
		} else {
			return nil
		}
	}

	return s.validateSoundResource(soundName, fset.Position(callExpr.Args[0].Pos()), fset.Position(callExpr.Args[0].End()))
}

// validateSoundResource checks if the sound resource exists and returns appropriate diagnostics if it doesn't.
func (s *Server) validateSoundResource(soundName string, start, end goptoken.Position) []Diagnostic {
	if _, err := s.getSpxSoundResource(soundName); err != nil {
		return s.collectDiagnosticsFromGetSoundResourceError(err, soundName, start, end)
	}
	return nil
}

// collectDiagnosticsFromGetSoundResourceError collects diagnostics from an error when calling [Server.getSpxSoundResource].
func (s *Server) collectDiagnosticsFromGetSoundResourceError(err error, soundName string, start, end goptoken.Position) []Diagnostic {
	if errors.Is(err, fs.ErrNotExist) {
		return []Diagnostic{{
			Severity: SeverityError,
			Range: Range{
				Start: FromGopTokenPosition(start),
				End:   FromGopTokenPosition(end),
			},
			Message: fmt.Sprintf("sound resource %q not found", soundName),
		}}
	}
	return []Diagnostic{{
		Severity: SeverityError,
		Range: Range{
			Start: FromGopTokenPosition(start),
			End:   FromGopTokenPosition(end),
		},
		Message: fmt.Sprintf("failed to get sound resource %q: %v", soundName, err),
	}}
}

// validateSpxGameOrSpriteOnBackdropCall validates a spx.Game.OnBackdrop or
// spx.Sprite.OnBackdrop call.
//
// See https://pkg.go.dev/github.com/goplus/spx#Game.OnBackdrop__1 and
// https://pkg.go.dev/github.com/goplus/spx#Game.OnBackdrop__1
func (s *Server) validateSpxGameOrSpriteOnBackdropCall(typeInfo *goptypesutil.Info, callExpr *gopast.CallExpr, fset *goptoken.FileSet) []Diagnostic {
	tv, ok := typeInfo.Types[callExpr.Fun]
	if !ok {
		return nil
	}
	sig, ok := tv.Type.(*types.Signature)
	if !ok {
		return nil
	}
	recv := sig.Recv()
	if recv == nil {
		return nil
	}
	recvType := recv.Type()
	if ptr, ok := recvType.(*types.Pointer); ok {
		recvType = ptr.Elem()
	}
	switch recvType.String() {
	case SpxGameTypeName, SpxSpriteTypeName, SpxSpriteImplTypeName:
	default:
		return nil
	}

	if len(callExpr.Args) == 0 {
		return nil
	}
	argTV, ok := typeInfo.Types[callExpr.Args[0]]
	if !ok {
		return nil
	}
	if !types.AssignableTo(argTV.Type, types.Typ[types.String]) {
		return nil
	}

	var backdropName string
	switch arg := callExpr.Args[0].(type) {
	case *gopast.BasicLit:
		if arg.Kind != goptoken.STRING {
			return nil
		}
		backdropName = arg.Value
	case *gopast.Ident:
		if argTV.Value != nil {
			// If it's a constant, we can get its value.
			backdropName = argTV.Value.String()
		} else {
			// There is nothing we can do for string variables.
			return nil
		}
	default:
		return nil
	}
	backdropName = backdropName[1 : len(backdropName)-1] // Unquote the string.
	if _, err := s.getSpxBackdropResource(backdropName); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []Diagnostic{{
				Severity: SeverityError,
				Range: Range{
					Start: FromGopTokenPosition(fset.Position(callExpr.Args[0].Pos())),
					End:   FromGopTokenPosition(fset.Position(callExpr.Args[0].End())),
				},
				Message: fmt.Sprintf("backdrop resource %q not found", backdropName),
			}}
		}
		return []Diagnostic{{
			Severity: SeverityError,
			Range: Range{
				Start: FromGopTokenPosition(fset.Position(callExpr.Args[0].Pos())),
				End:   FromGopTokenPosition(fset.Position(callExpr.Args[0].End())),
			},
			Message: fmt.Sprintf("failed to get backdrop resource %q: %v", backdropName, err),
		}}
	}
	return nil
}

// validateSpxSpriteSetCostumeCall validates a spx.Sprite.SetCostume call.
//
// See https://pkg.go.dev/github.com/goplus/spx#Sprite.SetCostume
func (s *Server) validateSpxSpriteSetCostumeCall(typeInfo *goptypesutil.Info, callExpr *gopast.CallExpr, fset *goptoken.FileSet) []Diagnostic {
	spriteName := s.getSpxSpriteNameFromCall(typeInfo, callExpr, fset)
	if spriteName == "" {
		return []Diagnostic{{
			Severity: SeverityWarning,
			Range: Range{
				Start: FromGopTokenPosition(fset.Position(callExpr.Fun.Pos())),
				End:   FromGopTokenPosition(fset.Position(callExpr.Fun.End())),
			},
			Message: "cannot determine sprite name",
		}}
	}

	spriteResource, err := s.getSpxSpriteResource(spriteName)
	if err != nil {
		return s.collectDiagnosticsFromGetSpriteResourceError(
			err,
			spriteName,
			fset.Position(callExpr.Fun.Pos()),
			fset.Position(callExpr.Fun.End()),
		)
	}

	if len(callExpr.Args) == 0 {
		return nil
	}
	argTV, ok := typeInfo.Types[callExpr.Args[0]]
	if !ok {
		return nil
	}
	if !types.AssignableTo(argTV.Type, types.Typ[types.String]) {
		return nil
	}

	var costumeName string
	switch arg := callExpr.Args[0].(type) {
	case *gopast.BasicLit:
		if arg.Kind != goptoken.STRING {
			return nil
		}
		costumeName = arg.Value
	case *gopast.Ident:
		if argTV.Value != nil {
			// If it's a constant, we can get its value.
			costumeName = argTV.Value.String()
		} else {
			// There is nothing we can do for string variables.
			return nil
		}
	default:
		return nil
	}
	costumeName = costumeName[1 : len(costumeName)-1] // Unquote the string.
	for _, costume := range spriteResource.Costumes {
		if costume.Name == costumeName {
			return nil
		}
	}
	return []Diagnostic{
		{
			Severity: SeverityError,
			Message:  fmt.Sprintf("costume resource %q not found in sprite %q", costumeName, spriteName),
		},
	}
}

// validateSpxSpriteAnimateCall validates a spx.Sprite.animate call.
//
// See https://pkg.go.dev/github.com/goplus/spx#Sprite.Animate
func (s *Server) validateSpxSpriteAnimateCall(typeInfo *goptypesutil.Info, callExpr *gopast.CallExpr, fset *goptoken.FileSet) []Diagnostic {
	spriteName := s.getSpxSpriteNameFromCall(typeInfo, callExpr, fset)
	if spriteName == "" {
		return []Diagnostic{{
			Severity: SeverityWarning,
			Range: Range{
				Start: FromGopTokenPosition(fset.Position(callExpr.Fun.Pos())),
				End:   FromGopTokenPosition(fset.Position(callExpr.Fun.End())),
			},
			Message: "cannot determine sprite name",
		}}
	}

	spriteResource, err := s.getSpxSpriteResource(spriteName)
	if err != nil {
		return s.collectDiagnosticsFromGetSpriteResourceError(
			err,
			spriteName,
			fset.Position(callExpr.Fun.Pos()),
			fset.Position(callExpr.Fun.End()),
		)
	}

	if len(callExpr.Args) == 0 {
		return nil
	}
	argTV, ok := typeInfo.Types[callExpr.Args[0]]
	if !ok {
		return nil
	}
	if !types.AssignableTo(argTV.Type, types.Typ[types.String]) {
		return nil
	}

	var animationName string
	switch arg := callExpr.Args[0].(type) {
	case *gopast.BasicLit:
		if arg.Kind != goptoken.STRING {
			return nil
		}
		animationName = arg.Value
	case *gopast.Ident:
		if argTV.Value != nil {
			// If it's a constant, we can get its value.
			animationName = argTV.Value.String()
		} else {
			// There is nothing we can do for string variables.
			return nil
		}
	default:
		return nil
	}
	animationName = animationName[1 : len(animationName)-1] // Unquote the string.
	for _, animation := range spriteResource.Animations {
		if animation.Name == animationName {
			return nil
		}
	}
	return []Diagnostic{
		{
			Severity: SeverityError,
			Range: Range{
				Start: FromGopTokenPosition(fset.Position(callExpr.Fun.Pos())),
				End:   FromGopTokenPosition(fset.Position(callExpr.Fun.End())),
			},
			Message: fmt.Sprintf("animation resource %q not found in sprite %q", animationName, spriteName),
		},
	}
}

// getSpxSpriteNameFromCall extracts spx sprite name from a method call on a
// sprite type. It returns empty string if the name cannot be determined.
func (s *Server) getSpxSpriteNameFromCall(typeInfo *goptypesutil.Info, callExpr *gopast.CallExpr, fset *goptoken.FileSet) string {
	tv, ok := typeInfo.Types[callExpr.Fun]
	if !ok {
		return ""
	}
	sig, ok := tv.Type.(*types.Signature)
	if !ok {
		return ""
	}
	recv := sig.Recv()
	if recv == nil {
		return ""
	}
	recvType := recv.Type()
	if ptr, ok := recvType.(*types.Pointer); ok {
		recvType = ptr.Elem()
	}

	var isAutoBinding bool
	switch recvType.String() {
	case SpxSpriteTypeName:
		isAutoBinding = true
	case SpxSpriteImplTypeName:
	default:
		return ""
	}

	var spriteName string
	switch fun := callExpr.Fun.(type) {
	case *gopast.Ident:
		spriteName = strings.TrimSuffix(filepath.Base(fset.Position(callExpr.Pos()).Filename), ".spx")
	case *gopast.SelectorExpr:
		if ident, ok := fun.X.(*gopast.Ident); ok {
			// Try to get the original definition.
			if obj := typeInfo.Defs[ident]; obj != nil {
				// Check if it's a variable declaration.
				if varObj, ok := obj.(*types.Var); ok {
					if isAutoBinding {
						// The variable name is the sprite name per auto-binding rules.
						spriteName = varObj.Name()
					} else if named, ok := varObj.Type().(*types.Named); ok {
						spriteName = strings.TrimPrefix(named.String(), "main.") // Remove the package prefix.
					}
				}
			} else if obj := typeInfo.Uses[ident]; obj != nil {
				// If not in Defs, check Uses for variables defined elsewhere.
				if varObj, ok := obj.(*types.Var); ok {
					if isAutoBinding {
						// The variable name is the sprite name per auto-binding rules.
						spriteName = varObj.Name()
					} else if named, ok := varObj.Type().(*types.Named); ok {
						spriteName = strings.TrimPrefix(named.String(), "main.") // Remove the package prefix.
					}
				}
			}
			// Fallback to direct identifier name if we couldn't find the definition.
			if spriteName == "" {
				spriteName = ident.Name
			}
		}
	}
	return spriteName
}

// validateSpriteResource checks if the sprite resource exists and returns
// appropriate diagnostics if it doesn't.
func (s *Server) validateSpriteResource(spriteName string, start, end goptoken.Position) []Diagnostic {
	if _, err := s.getSpxSpriteResource(spriteName); err != nil {
		return s.collectDiagnosticsFromGetSpriteResourceError(err, spriteName, start, end)
	}
	return nil
}

// collectDiagnosticsFromGetSpriteResourceError collects diagnostics from an
// error when calling [Server.getSpxSpriteResource].
func (s *Server) collectDiagnosticsFromGetSpriteResourceError(err error, spriteName string, start, end goptoken.Position) []Diagnostic {
	if errors.Is(err, fs.ErrNotExist) {
		return []Diagnostic{{
			Severity: SeverityError,
			Range: Range{
				Start: FromGopTokenPosition(start),
				End:   FromGopTokenPosition(end),
			},
			Message: fmt.Sprintf("sprite resource %q not found", spriteName),
		}}
	}
	return []Diagnostic{{
		Severity: SeverityError,
		Range: Range{
			Start: FromGopTokenPosition(start),
			End:   FromGopTokenPosition(end),
		},
		Message: fmt.Sprintf("failed to get sprite resource %q: %v", spriteName, err),
	}}
}

// validateSpxGameGetWidgetCall validates a spx.Game.getWidget call.
//
// See https://pkg.go.dev/github.com/goplus/spx#Gopt_Game_Gopx_GetWidget
func (s *Server) validateSpxGameGetWidgetCall(typeInfo *goptypesutil.Info, callExpr *gopast.CallExpr, fset *goptoken.FileSet) []Diagnostic {
	tv, ok := typeInfo.Types[callExpr.Fun]
	if !ok {
		return nil
	}
	sig, ok := tv.Type.(*types.Signature)
	if !ok {
		return nil
	}
	recv := sig.Recv()
	if recv == nil {
		return nil
	}
	recvType := recv.Type()
	if ptr, ok := recvType.(*types.Pointer); ok {
		recvType = ptr.Elem()
	}
	if recvType.String() != SpxGameTypeName {
		return nil
	}

	if len(callExpr.Args) == 0 {
		return nil
	}
	argTV, ok := typeInfo.Types[callExpr.Args[0]]
	if !ok {
		return nil
	}
	if !types.AssignableTo(argTV.Type, types.Typ[types.String]) {
		return nil
	}

	var widgetName string
	switch arg := callExpr.Args[0].(type) {
	case *gopast.BasicLit:
		if arg.Kind != goptoken.STRING {
			return nil
		}
		widgetName = arg.Value
	case *gopast.Ident:
		if argTV.Value != nil {
			// If it's a constant, we can get its value.
			widgetName = argTV.Value.String()
		} else {
			// There is nothing we can do for string variables.
			return nil
		}
	default:
		return nil
	}
	widgetName = widgetName[1 : len(widgetName)-1] // Unquote the string.
	if _, err := s.getSpxWidgetResource(widgetName); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []Diagnostic{{
				Severity: SeverityError,
				Range: Range{
					Start: FromGopTokenPosition(fset.Position(callExpr.Args[0].Pos())),
					End:   FromGopTokenPosition(fset.Position(callExpr.Args[0].End())),
				},
				Message: fmt.Sprintf("widget resource %q not found", widgetName),
			}}
		}
		return []Diagnostic{{
			Severity: SeverityError,
			Range: Range{
				Start: FromGopTokenPosition(fset.Position(callExpr.Args[0].Pos())),
				End:   FromGopTokenPosition(fset.Position(callExpr.Args[0].End())),
			},
			Message: fmt.Sprintf("failed to get widget resource %q: %v", widgetName, err),
		}}
	}
	return nil
}
