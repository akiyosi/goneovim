package gonvim

import (
	"math"

	"github.com/dzhou121/ui"
)

// Font is
type Font struct {
	font       *ui.Font
	width      int
	height     int
	lineHeight int
}

func initFont() *Font {
	fontDesc := &ui.FontDescriptor{
		Family:  "InconsolataforPowerline Nerd Font",
		Size:    14,
		Weight:  ui.TextWeightNormal,
		Italic:  ui.TextItalicNormal,
		Stretch: ui.TextStretchNormal,
	}
	font := ui.LoadClosestFont(fontDesc)
	textLayout := ui.NewTextLayout("a", font, -1)
	w, h := textLayout.Extents()
	width := int(math.Ceil(w))
	height := int(math.Ceil(h))
	lineHeight := int(math.Ceil(h * 1.5))
	return &Font{
		font:       font,
		width:      width,
		height:     height,
		lineHeight: lineHeight,
	}
}
