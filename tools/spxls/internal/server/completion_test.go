package server

import (
	"testing"

	"github.com/goplus/builder/tools/spxls/internal/util"
	"github.com/goplus/builder/tools/spxls/internal/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerTextDocumentCompletion(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		s := New(vfs.NewMapFS(func() map[string][]byte {
			return map[string][]byte{
				"main.spx": []byte(`
var (
	MySprite Sprite
)

func test() {
	x := 1

}
`),
				"MySprite.spx": []byte(`
onStart => {
	MySprite.turn Right
}
`),
				"assets/sprites/MySprite/index.json": []byte(`{}`),
			}
		}), nil)

		items, err := s.textDocumentCompletion(&CompletionParams{
			TextDocumentPositionParams: TextDocumentPositionParams{
				TextDocument: TextDocumentIdentifier{URI: "file:///main.spx"},
				Position:     Position{Line: 7, Character: 0},
			},
		})

		require.NoError(t, err)
		require.NotNil(t, items)

		assert.Contains(t, items, SpxDefinition{
			ID: SpxDefinitionIdentifier{
				Package: util.ToPtr("main"),
				Name:    util.ToPtr("x")},
			Overview: "var x int",

			CompletionItemLabel:            "x",
			CompletionItemKind:             VariableCompletion,
			CompletionItemInsertText:       "x",
			CompletionItemInsertTextFormat: PlainTextTextFormat,
		}.CompletionItem())
		assert.Contains(t, items, GetSpxBuiltinDefinition("len").CompletionItem())
	})
}
