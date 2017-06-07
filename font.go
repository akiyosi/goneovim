package gonvim

import (
	"math"

	"github.com/dzhou121/ui"
	"github.com/therecipe/qt/gui"
)

// Font is
type Font struct {
	fontNew            *gui.QFont
	defaultFont        *gui.QFont
	defaultFontMetrics *gui.QFontMetricsF
	font               *ui.Font
	width              int
	truewidth          float64
	ascent             float64
	height             int
	lineHeight         int
	lineSpace          int
	shift              int
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

func fontSizeNew(font *gui.QFont) (int, int, float64, float64) {
	fontMetrics := gui.NewQFontMetricsF(font)
	h := fontMetrics.Height()
	w := fontMetrics.Width("W")
	ascent := fontMetrics.Ascent()
	width := int(math.Ceil(w))
	height := int(math.Ceil(h))
	fontMetrics.DestroyQFontMetricsF()
	return width, height, w, ascent
}

func initFontNew(family string, size int, lineSpace int) *Font {
	font := gui.NewQFont2(family, size, int(gui.QFont__Normal), false)
	width, height, truewidth, ascent := fontSizeNew(font)
	defaultFont := gui.NewQFont()
	return &Font{
		fontNew:            font,
		defaultFont:        defaultFont,
		defaultFontMetrics: gui.NewQFontMetricsF(defaultFont),
		width:              width,
		truewidth:          truewidth,
		height:             height,
		lineHeight:         height + lineSpace,
		lineSpace:          lineSpace,
		shift:              int(float64(lineSpace)/2 + ascent),
		ascent:             ascent,
	}
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
	f.fontNew.SetFamily(family)
	f.fontNew.SetPixelSize(size)
	width, height, truewidth, ascent := fontSizeNew(f.fontNew)
	f.width = width
	f.height = height
	f.truewidth = truewidth
	f.lineHeight = height + f.lineSpace
	f.ascent = ascent
	f.shift = int(float64(f.lineSpace)/2 + ascent)
}

func (f *Font) changeLineSpace(lineSpace int) {
	f.lineSpace = lineSpace
	f.lineHeight = f.height + lineSpace
	f.shift = int(float64(lineSpace)/2 + f.ascent)
}
