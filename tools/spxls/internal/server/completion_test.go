package server

import (
	"testing"

	"github.com/goplus/builder/tools/spxls/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerTextDocumentCompletion(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		s := New(newMapFSWithoutModTime(map[string][]byte{
			"main.spx": []byte(`
var (
	MySprite Sprite
)

MySprite.
run "assets", {Title: "My Game"}
`),
			"MySprite.spx": []byte(`
onStart => {
	MySprite.turn Right
}
`),
			"assets/index.json":                  []byte(`{}`),
			"assets/sprites/MySprite/index.json": []byte(`{}`),
		}), nil)

		emptyLineItems, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 4, Character: 0},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, emptyLineItems)
		assert.Contains(t, emptyLineItems, GetSpxBuiltinDefinition("println").CompletionItem())
		assert.Contains(t, emptyLineItems, SpxDefinition{
			ID: SpxDefinitionIdentifier{
				Package: util.ToPtr("main"),
				Name:    util.ToPtr("MySprite"),
			},
			Overview: "type MySprite struct{SpriteImpl; *main.Game}",

			CompletionItemLabel:            "MySprite",
			CompletionItemKind:             StructCompletion,
			CompletionItemInsertText:       "MySprite",
			CompletionItemInsertTextFormat: PlainTextTextFormat,
		}.CompletionItem())

		mySpriteDotItems, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 5, Character: 9},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, mySpriteDotItems)
		assert.NotContains(t, mySpriteDotItems, GetSpxBuiltinDefinition("println").CompletionItem())
		assert.Contains(t, mySpriteDotItems, SpxDefinition{
			ID: SpxDefinitionIdentifier{
				Package:    util.ToPtr("github.com/goplus/spx"),
				Name:       util.ToPtr("Sprite.turn"),
				OverloadID: util.ToPtr("0"),
			},
			Overview: "func turn(degree float64)",

			CompletionItemLabel:            "turn",
			CompletionItemKind:             FunctionCompletion,
			CompletionItemInsertText:       "turn",
			CompletionItemInsertTextFormat: PlainTextTextFormat,
		}.CompletionItem())
		assert.Contains(t, mySpriteDotItems, SpxDefinition{
			ID: SpxDefinitionIdentifier{
				Package:    util.ToPtr("github.com/goplus/spx"),
				Name:       util.ToPtr("Sprite.turn"),
				OverloadID: util.ToPtr("1"),
			},
			Overview: "func turn(dir specialDir)",

			CompletionItemLabel:            "turn",
			CompletionItemKind:             FunctionCompletion,
			CompletionItemInsertText:       "turn",
			CompletionItemInsertTextFormat: PlainTextTextFormat,
		}.CompletionItem())
		assert.Contains(t, mySpriteDotItems, SpxDefinition{
			ID: SpxDefinitionIdentifier{
				Package:    util.ToPtr("github.com/goplus/spx"),
				Name:       util.ToPtr("Sprite.turn"),
				OverloadID: util.ToPtr("2"),
			},
			Overview: "func turn(ti *TurningInfo)",

			CompletionItemLabel:            "turn",
			CompletionItemKind:             FunctionCompletion,
			CompletionItemInsertText:       "turn",
			CompletionItemInsertTextFormat: PlainTextTextFormat,
		}.CompletionItem())
		assert.Contains(t, mySpriteDotItems, SpxDefinition{
			ID: SpxDefinitionIdentifier{
				Package:    util.ToPtr("github.com/goplus/spx"),
				Name:       util.ToPtr("Sprite.clone"),
				OverloadID: util.ToPtr("0"),
			},
			Overview: "func clone()",

			CompletionItemLabel:            "clone",
			CompletionItemKind:             FunctionCompletion,
			CompletionItemInsertText:       "clone",
			CompletionItemInsertTextFormat: PlainTextTextFormat,
		}.CompletionItem())
		assert.Contains(t, mySpriteDotItems, SpxDefinition{
			ID: SpxDefinitionIdentifier{
				Package:    util.ToPtr("github.com/goplus/spx"),
				Name:       util.ToPtr("Sprite.clone"),
				OverloadID: util.ToPtr("1"),
			},
			Overview: "func clone(data interface{})",

			CompletionItemLabel:            "clone",
			CompletionItemKind:             FunctionCompletion,
			CompletionItemInsertText:       "clone",
			CompletionItemInsertTextFormat: PlainTextTextFormat,
		}.CompletionItem())
	})

	t.Run("InComment", func(t *testing.T) {
		s := New(newMapFSWithoutModTime(map[string][]byte{
			"main.spx": []byte(`
// Run My G
run "assets", {Title: "My Game"}
`),
		}), nil)

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 1, Character: 11},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.Empty(t, items)
	})

	t.Run("InStringLit", func(t *testing.T) {
		s := New(newMapFSWithoutModTime(map[string][]byte{
			"main.spx": []byte(`
run "a
`),
		}), nil)

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 1, Character: 6},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.Empty(t, items)
	})

	t.Run("InImportStringLit", func(t *testing.T) {
		s := New(newMapFSWithoutModTime(map[string][]byte{
			"main.spx": []byte(`
import "g
`),
		}), nil)

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 1, Character: 9},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
	})

	t.Run("InImportGroupStringLit", func(t *testing.T) {
		s := New(newMapFSWithoutModTime(map[string][]byte{
			"main.spx": []byte(`
import (
	"g
`),
		}), nil)

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 2, Character: 3},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, items)
		assert.NotEmpty(t, items)
	})
}
