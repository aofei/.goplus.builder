package server

import (
	"testing"

	"github.com/goplus/builder/tools/spxls/internal/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestFileMap() map[string][]byte {
	return map[string][]byte{
		"main.spx": []byte(`
var (
	MyAircraft MyAircraft
	Bullet     Bullet
)
run "assets", {Title: "Bullet (by Go+)"}
`),
		"MyAircraft.spx": []byte(`
onStart => {
	for {
		wait 0.1
		Bullet.clone
		play "biu"
		setXYpos mouseX, mouseY
	}
}
`),
		"Bullet.spx": []byte(`
onCloned => {
	setXYpos MyAircraft.xpos, MyAircraft.ypos+5
	show
	for {
		wait 0.04
		step 10
		if touching(Edge) {
			destroy
		}
	}
}
`),
		"assets/index.json":                    []byte(`{"backdrops":[{"x":0,"y":0,"faceRight":0,"bitmapResolution":2,"name":"backdrop1","path":"backdrop1.png"}],"backdropIndex":0,"map":{"width":480,"height":360,"mode":"fillRatio"},"run":{"width":480,"height":360},"zorder":["MyAircraft","Bullet"]}`),
		"assets/backdrop1.png":                 nil,
		"assets/sprites/MyAircraft/index.json": []byte(`{"heading":90,"x":-14.502367071209733,"y":-151.76923076923077,"size":0.45,"rotationStyle":"normal","costumeIndex":0,"visible":true,"isDraggable":false,"pivot":{"x":0,"y":0},"costumes":[{"x":98,"y":122,"faceRight":0,"bitmapResolution":2,"name":"hero","path":"hero.png"}],"fAnimations":{},"animBindings":{}}`),
		"assets/sprites/MyAircraft/hero.png":   nil,
		"assets/sprites/Bullet/index.json":     []byte(`{"heading":0,"x":230,"y":185,"size":0.65,"rotationStyle":"normal","costumeIndex":0,"visible":false,"isDraggable":false,"pivot":{"x":0,"y":0},"costumes":[{"x":8,"y":20,"faceRight":90,"bitmapResolution":2,"name":"bullet","path":"bullet.png"}],"fAnimations":{},"animBindings":{}}`),
		"assets/sprites/Bullet/bullet.png":     nil,
		"assets/sounds/biu/index.json":         []byte(`{"rate":0,"sampleCount":0,"path":"biu.wav"}`),
		"assets/sounds/biu/biu.wav":            nil,
	}
}

func TestServerTextDocumentDiagnostic(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		s := New(vfs.NewMapFS(func() map[string][]byte {
			return newTestFileMap()
		}), nil)
		params := &DocumentDiagnosticParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		report, err := s.textDocumentDiagnostic(params)
		require.NoError(t, err)
		require.NotNil(t, report)

		fullReport, ok := report.Value.(*RelatedFullDocumentDiagnosticReport)
		assert.True(t, ok, "expected *RelatedFullDocumentDiagnosticReport")
		assert.Equal(t, string(DiagnosticFull), fullReport.Kind)
		assert.Empty(t, fullReport.Items)
	})

	t.Run("ParseError", func(t *testing.T) {
		s := New(vfs.NewMapFS(func() map[string][]byte {
			fileMap := newTestFileMap()
			fileMap["main.spx"] = []byte(`
// Invalid syntax, missing closing parenthesis
var (
	MyAircraft MyAircraft
`)
			return fileMap
		}), nil)
		params := &DocumentDiagnosticParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
		}

		report, err := s.textDocumentDiagnostic(params)
		require.NoError(t, err)
		require.NotNil(t, report)

		fullReport, ok := report.Value.(*RelatedFullDocumentDiagnosticReport)
		assert.True(t, ok, "expected *RelatedFullDocumentDiagnosticReport")
		assert.Equal(t, string(DiagnosticFull), fullReport.Kind)
		assert.NotEmpty(t, fullReport.Items)
		assert.Contains(t, fullReport.Items[0].Message, "expected ')'")
		assert.NotNil(t, fullReport.Items[0].Range)
		assert.Equal(t, Position{Line: 4, Character: 24}, fullReport.Items[0].Range.Start)
		assert.Equal(t, Position{Line: 4, Character: 24}, fullReport.Items[0].Range.End)
	})

	t.Run("NonSpxFile", func(t *testing.T) {
		s := New(vfs.NewMapFS(func() map[string][]byte {
			fileMap := newTestFileMap()
			fileMap["main.gop"] = []byte(`echo "Hello, Go+!"`)
			return fileMap
		}), nil)
		params := &DocumentDiagnosticParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///main.gop"},
		}

		report, err := s.textDocumentDiagnostic(params)
		require.NoError(t, err)
		require.NotNil(t, report)

		fullReport, ok := report.Value.(*RelatedFullDocumentDiagnosticReport)
		assert.True(t, ok, "expected *RelatedFullDocumentDiagnosticReport")
		assert.Equal(t, string(DiagnosticFull), fullReport.Kind)
		assert.Empty(t, fullReport.Items)
	})

	t.Run("FileNotFound", func(t *testing.T) {
		s := New(vfs.NewMapFS(func() map[string][]byte {
			return newTestFileMap()
		}), nil)
		params := &DocumentDiagnosticParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///notexist.spx"},
		}

		report, err := s.textDocumentDiagnostic(params)
		require.NoError(t, err)
		require.NotNil(t, report)

		fullReport, ok := report.Value.(*RelatedFullDocumentDiagnosticReport)
		assert.True(t, ok, "expected *RelatedFullDocumentDiagnosticReport")
		assert.Equal(t, string(DiagnosticFull), fullReport.Kind)
		assert.Empty(t, fullReport.Items)
	})
}

func TestServerWorkspaceDiagnostic(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		s := New(vfs.NewMapFS(func() map[string][]byte {
			return newTestFileMap()
		}), nil)

		report, err := s.workspaceDiagnostic(&WorkspaceDiagnosticParams{})
		require.NoError(t, err)
		require.NotNil(t, report)
		assert.Len(t, report.Items, 3)

		foundFiles := make(map[string]bool)
		for _, item := range report.Items {
			fullReport := item.Value.(*WorkspaceFullDocumentDiagnosticReport)
			relPath, err := s.fromDocumentURI(fullReport.URI)
			require.NoError(t, err)
			foundFiles[relPath] = true
			assert.Equal(t, string(DiagnosticFull), fullReport.Kind)
			assert.Empty(t, fullReport.Items)
		}
		assert.Contains(t, foundFiles, "main.spx")
		assert.Contains(t, foundFiles, "MyAircraft.spx")
		assert.Contains(t, foundFiles, "Bullet.spx")
	})

	t.Run("ParseError", func(t *testing.T) {
		s := New(vfs.NewMapFS(func() map[string][]byte {
			return map[string][]byte{
				"main.spx": []byte(`
// Invalid syntax, missing closing parenthesis
var (
	MyAircraft MyAircraft
`),
				"MyAircraft.spx": []byte(`var x int`),
			}
		}), nil)

		report, err := s.workspaceDiagnostic(&WorkspaceDiagnosticParams{})
		require.NoError(t, err)
		require.NotNil(t, report)
		assert.Len(t, report.Items, 2)

		for _, item := range report.Items {
			fullReport := item.Value.(*WorkspaceFullDocumentDiagnosticReport)
			if fullReport.URI == "file:///main.spx" {
				assert.NotEmpty(t, fullReport.Items)
				assert.Contains(t, fullReport.Items[0].Message, "expected ')'")
			} else {
				assert.Empty(t, fullReport.Items)
			}
		}
	})

	t.Run("EmptyWorkspace", func(t *testing.T) {
		s := New(vfs.NewMapFS(func() map[string][]byte {
			return map[string][]byte{}
		}), nil)

		report, err := s.workspaceDiagnostic(&WorkspaceDiagnosticParams{})
		require.EqualError(t, err, "no valid spx files found in main package")
		require.Nil(t, report)
	})

	t.Run("SoundResourceNotFound", func(t *testing.T) {
		s := New(vfs.NewMapFS(func() map[string][]byte {
			return map[string][]byte{
				"main.spx": []byte(`
run "assets", {Title: "My Game"}
`),
				"MySprite.spx": []byte(`
const ConstSoundName = "ConstSoundName"
var VarSoundName string
VarSoundName = "VarSoundName"
var AutoBindingSoundName Sound
onStart => {
	play ConstSoundName
	play "LiteralSoundName"
	play VarSoundName
	play AutoBindingSoundName
}
`),
			}
		}), nil)

		report, err := s.workspaceDiagnostic(&WorkspaceDiagnosticParams{})
		require.NoError(t, err)
		require.NotNil(t, report)
		assert.Len(t, report.Items, 2)
		for _, item := range report.Items {
			fullReport := item.Value.(*WorkspaceFullDocumentDiagnosticReport)
			assert.Equal(t, string(DiagnosticFull), fullReport.Kind)
			switch fullReport.URI {
			case "file:///MySprite.spx":
				assert.NotEmpty(t, fullReport.Items)
				assert.Len(t, fullReport.Items, 4)
				assert.Equal(t, `sound resource "AutoBindingSoundName" not found`, fullReport.Items[0].Message)
				assert.Equal(t, `sound resource "ConstSoundName" not found`, fullReport.Items[1].Message)
				assert.Equal(t, `sound resource "LiteralSoundName" not found`, fullReport.Items[2].Message)
				assert.Equal(t, `sound resource "AutoBindingSoundName" not found`, fullReport.Items[3].Message)
			default:
				assert.Empty(t, fullReport.Items)
			}
		}
	})

	t.Run("BackdropResourceNotFound", func(t *testing.T) {
		s := New(vfs.NewMapFS(func() map[string][]byte {
			return map[string][]byte{
				"main.spx": []byte(`
onBackdrop "NonExistentBackdrop", func() {}
run "assets", {Title: "My Game"}
`),
				"MySprite.spx": []byte(`
const ConstBackdropName = "ConstBackdropName"
var VarBackdropName string
VarBackdropName = "VarBackdropName"
onStart => {
	onBackdrop ConstBackdropName, func() {}
	onBackdrop "LiteralBackdropName", func() {}
	onBackdrop VarBackdropName, func() {}
}
`),
			}
		}), nil)

		report, err := s.workspaceDiagnostic(&WorkspaceDiagnosticParams{})
		require.NoError(t, err)
		require.NotNil(t, report)
		assert.Len(t, report.Items, 2)

		for _, item := range report.Items {
			fullReport := item.Value.(*WorkspaceFullDocumentDiagnosticReport)
			assert.Equal(t, string(DiagnosticFull), fullReport.Kind)
			switch fullReport.URI {
			case "file:///main.spx":
				assert.NotEmpty(t, fullReport.Items)
				assert.Len(t, fullReport.Items, 1)
				assert.Equal(t, `backdrop resource "NonExistentBackdrop" not found`, fullReport.Items[0].Message)
			case "file:///MySprite.spx":
				assert.NotEmpty(t, fullReport.Items)
				assert.Len(t, fullReport.Items, 2)
				assert.Equal(t, `backdrop resource "ConstBackdropName" not found`, fullReport.Items[0].Message)
				assert.Equal(t, `backdrop resource "LiteralBackdropName" not found`, fullReport.Items[1].Message)
			default:
				assert.Empty(t, fullReport.Items)
			}
		}
	})

	t.Run("SpriteResourceNotFound", func(t *testing.T) {
		s := New(vfs.NewMapFS(func() map[string][]byte {
			return map[string][]byte{
				"main.spx": []byte(`
var (
	MySprite1 Sprite
	MySprite2a MySprite2
)
run "assets", {Title: "My Game"}
`),
				"MySprite1.spx": []byte(`
onStart => {
	animate "roll-in"
	MySprite2a.animate "roll-out"
}
`),
				"MySprite2.spx": []byte(`
onStart => {
	MySprite1.animate "roll-out"
	animate "roll-in"
	MySprite2a.animate "roll-out"
}
`),
			}
		}), nil)

		report, err := s.workspaceDiagnostic(&WorkspaceDiagnosticParams{})
		require.NoError(t, err)
		require.NotNil(t, report)
		assert.Len(t, report.Items, 3)
		for _, item := range report.Items {
			fullReport := item.Value.(*WorkspaceFullDocumentDiagnosticReport)
			assert.Equal(t, string(DiagnosticFull), fullReport.Kind)
			switch fullReport.URI {
			case "file:///main.spx":
				assert.NotEmpty(t, fullReport.Items)
				assert.Len(t, fullReport.Items, 1)
				assert.Equal(t, `sprite resource "MySprite1" not found`, fullReport.Items[0].Message)
			case "file:///MySprite1.spx":
				assert.NotEmpty(t, fullReport.Items)
				assert.Len(t, fullReport.Items, 2)
				assert.Equal(t, `sprite resource "MySprite1" not found`, fullReport.Items[0].Message)
				assert.Equal(t, `sprite resource "MySprite2" not found`, fullReport.Items[1].Message)
			case "file:///MySprite2.spx":
				assert.NotEmpty(t, fullReport.Items)
				assert.Len(t, fullReport.Items, 3)
				assert.Equal(t, `sprite resource "MySprite1" not found`, fullReport.Items[0].Message)
				assert.Equal(t, `sprite resource "MySprite2" not found`, fullReport.Items[1].Message)
				assert.Equal(t, `sprite resource "MySprite2" not found`, fullReport.Items[2].Message)
			default:
				assert.Empty(t, fullReport.Items)
			}
		}
	})

	t.Run("SpriteCostumeResourceNotFound", func(t *testing.T) {
		s := New(vfs.NewMapFS(func() map[string][]byte {
			return map[string][]byte{
				"main.spx": []byte(`
run "assets", {Title: "My Game"}
`),
				"MySprite.spx": []byte(`
onStart => {
	setCostume "NonExistentCostume"
}
`),
				"assets/sprites/MySprite/index.json": []byte(`{}`),
			}
		}), nil)

		report, err := s.workspaceDiagnostic(&WorkspaceDiagnosticParams{})
		require.NoError(t, err)
		require.NotNil(t, report)
		assert.Len(t, report.Items, 2)
		for _, item := range report.Items {
			fullReport := item.Value.(*WorkspaceFullDocumentDiagnosticReport)
			assert.Equal(t, string(DiagnosticFull), fullReport.Kind)
			switch fullReport.URI {
			case "file:///MySprite.spx":
				assert.NotEmpty(t, fullReport.Items)
				assert.Len(t, fullReport.Items, 1)
				assert.Equal(t, `costume resource "NonExistentCostume" not found in sprite "MySprite"`, fullReport.Items[0].Message)
			default:
				assert.Empty(t, fullReport.Items)
			}
		}
	})

	t.Run("SpriteAnimationResourceNotFound", func(t *testing.T) {
		s := New(vfs.NewMapFS(func() map[string][]byte {
			return map[string][]byte{
				"main.spx": []byte(`
run "assets", {Title: "My Game"}
`),
				"MySprite.spx": []byte(`
onStart => {
	animate "roll-in"
}
`),
				"assets/sprites/MySprite/index.json": []byte(`{}`),
			}
		}), nil)

		report, err := s.workspaceDiagnostic(&WorkspaceDiagnosticParams{})
		require.NoError(t, err)
		require.NotNil(t, report)
		assert.Len(t, report.Items, 2)
		for _, item := range report.Items {
			fullReport := item.Value.(*WorkspaceFullDocumentDiagnosticReport)
			assert.Equal(t, string(DiagnosticFull), fullReport.Kind)
			switch fullReport.URI {
			case "file:///MySprite.spx":
				assert.NotEmpty(t, fullReport.Items)
				assert.Len(t, fullReport.Items, 1)
				assert.Equal(t, `animation resource "roll-in" not found in sprite "MySprite"`, fullReport.Items[0].Message)
			default:
				assert.Empty(t, fullReport.Items)
			}
		}
	})

	t.Run("WidgetResourceNotFound", func(t *testing.T) {
		s := New(vfs.NewMapFS(func() map[string][]byte {
			return map[string][]byte{
				"main.spx": []byte(`
run "assets", {Title: "My Game"}
`),
				"MySprite.spx": []byte(`
const ConstWidgetName = "ConstWidgetName"
var VarWidgetName string
VarWidgetName = "VarWidgetName"
onStart => {
	getWidget ConstWidgetName
	getWidget "LiteralWidgetName"
	getWidget VarWidgetName
}
`),
				"assets/index.json": []byte(`{}`),
			}
		}), nil)

		report, err := s.workspaceDiagnostic(&WorkspaceDiagnosticParams{})
		require.NoError(t, err)
		require.NotNil(t, report)
		assert.Len(t, report.Items, 2)

		for _, item := range report.Items {
			fullReport := item.Value.(*WorkspaceFullDocumentDiagnosticReport)
			assert.Equal(t, string(DiagnosticFull), fullReport.Kind)
			switch fullReport.URI {
			case "file:///MySprite.spx":
				assert.NotEmpty(t, fullReport.Items)
				assert.Len(t, fullReport.Items, 5)
				assert.Equal(t, `widget resource "ConstWidgetName" not found`, fullReport.Items[0].Message)
				assert.Equal(t, `widget resource "LiteralWidgetName" not found`, fullReport.Items[1].Message)
			default:
				assert.Empty(t, fullReport.Items)
			}
		}
	})
}
