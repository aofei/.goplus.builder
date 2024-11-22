package internal

//go:generate qexp -outdir pkg github.com/goplus/spx
//go:generate qexp -outdir pkg github.com/hajimehoshi/ebiten/v2

import (
	"fmt"

	_ "github.com/goplus/builder/tools/spxls/internal/pkg/github.com/goplus/spx"
	_ "github.com/goplus/builder/tools/spxls/internal/pkg/github.com/hajimehoshi/ebiten/v2"
	"github.com/goplus/igop"
	"github.com/goplus/igop/gopbuild"
)

var IGopContext = igop.NewContext(0)

func init() {
	gopbuild.RegisterClassFileType(".spx", "Game", []*gopbuild.Class{{Ext: ".spx", Class: "SpriteImpl"}}, "github.com/goplus/spx")
	if err := gopbuild.RegisterPackagePatch(IGopContext, "github.com/goplus/spx", `
package spx

import (
	. "github.com/goplus/spx"
)

func Gopt_Game_Gopx_GetWidget[T any](sg ShapeGetter, name string) *T {
	widget := GetWidget_(sg, name)
	if result, ok := widget.(interface{}).(*T); ok {
		return result
	} else {
		panic("GetWidget: type mismatch")
	}
}
`); err != nil {
		panic(fmt.Errorf("register package patch failed: %w", err))
	}
}
