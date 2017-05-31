package gonvim

import (
	"math"

	"github.com/dzhou121/ui"
)

// Font is
type Font struct {
	font       *ui.Font
	width      int
	truewidth  float64
	height     int
	lineHeight int
	lineSpace  int
	shift      int
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

func fontSize(font *ui.Font) (int, int, float64) {
	textLayout := ui.NewTextLayout("W", font, -1)
	w, h := textLayout.Extents()
	width := int(math.Ceil(w))
	height := int(math.Ceil(h))
	textLayout.Free()
	return width, height, w
}

func initFont(family string, size int, lineSpace int) *Font {
	font := newFont(family, size)
	width, height, truewidth := fontSize(font)
	shift := lineSpace / 2
	return &Font{
		font:       font,
		width:      width,
		truewidth:  truewidth,
		height:     height,
		lineHeight: height + lineSpace,
		lineSpace:  lineSpace,
		shift:      shift,
	}
}

func (f *Font) change(family string, size int) {
	oldFont := f.font
	font := newFont(family, size)
	width, height, truewidth := fontSize(font)
	lineHeight := height + f.lineSpace
	shift := (lineHeight - height) / 2
	f.font = font
	f.width = width
	f.height = height
	f.truewidth = truewidth
	f.lineHeight = lineHeight
	f.shift = shift
	oldFont.Free()
}

func (f *Font) changeLineSpace(lineSpace int) {
	f.lineSpace = lineSpace
	f.lineHeight = f.height + lineSpace
	shift := lineSpace / 2
	f.shift = shift
}
