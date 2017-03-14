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
	lineSpace  int
}

func newFont(family string, size int) *ui.Font {
	fontDesc := &ui.FontDescriptor{
		Family:  family,
		Size:    float64(size),
		Weight:  ui.TextWeightNormal,
		Italic:  ui.TextItalicNormal,
		Stretch: ui.TextStretchNormal,
	}
	font := ui.LoadClosestFont(fontDesc)
	return font
}

func fontSize(font *ui.Font) (int, int) {
	textLayout := ui.NewTextLayout("a", font, -1)
	w, h := textLayout.Extents()
	width := int(math.Ceil(w))
	height := int(math.Ceil(h))
	return width, height
}

func initFont(family string, size int, lineSpace int) *Font {
	font := newFont(family, size)
	width, height := fontSize(font)
	lineHeight := height + lineSpace
	return &Font{
		font:       font,
		width:      width,
		height:     height,
		lineHeight: lineHeight,
		lineSpace:  lineSpace,
	}
}

func (f *Font) change(family string, size int) {
	f.font.Free()
	font := newFont(family, size)
	width, height := fontSize(font)
	lineHeight := height + f.lineSpace
	f.font = font
	f.width = width
	f.height = height
	f.lineHeight = lineHeight
}

func (f *Font) changeLineSpace(lineSpace int) {
	f.lineSpace = lineSpace
	f.lineHeight = f.height + lineSpace
}
