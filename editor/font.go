package editor

import (
	"math"

	"github.com/therecipe/qt/gui"
)

// Font is
type Font struct {
	ws                 *Workspace
	fontNew            *gui.QFont
	fontMetrics        *gui.QFontMetricsF
	defaultFont        *gui.QFont
	defaultFontMetrics *gui.QFontMetricsF
	width              int
	truewidth          float64
	ascent             float64
	height             int
	lineHeight         int
	lineSpace          int
	shift              int
}

func fontSizeNew(font *gui.QFont) (int, int, float64, float64) {
	fontMetrics := gui.NewQFontMetricsF(font)
	h := fontMetrics.Height()
	w := fontMetrics.HorizontalAdvance("W", -1)
	ascent := fontMetrics.Ascent()
	width := int(math.Ceil(w))
	height := int(math.Ceil(h))
	return width, height, w, ascent
}

func initFontNew(family string, size int, lineSpace int) *Font {
	font := gui.NewQFont2(family, size, int(gui.QFont__Normal), false)

	// fontMetrics calculate is too slow, so we calculate approximate font metrics
	//// width, height, truewidth, ascent := fontSizeNew(font)
	h := math.Trunc(float64(size)*1.28) + 2.0
	w := math.Ceil(float64(size) * 0.7)
	height := int(math.Ceil(h))
	width := int(math.Ceil(w))
	ascent := math.Ceil(float64(size) * 1.1)
	defaultFont := gui.NewQFont()
	return &Font{
		fontNew:            font,
		fontMetrics:        gui.NewQFontMetricsF(font),
		defaultFont:        defaultFont,
		defaultFontMetrics: gui.NewQFontMetricsF(defaultFont),
		width:              width,
		truewidth:          w,
		height:             height,
		lineHeight:         height + lineSpace,
		lineSpace:          lineSpace,
		shift:              int(float64(lineSpace)/2 + ascent),
		ascent:             ascent,
	}
}

func (f *Font) change(family string, size int) {
	f.fontNew.SetFamily(family)
	f.fontNew.SetPointSize(size)
	f.fontMetrics = gui.NewQFontMetricsF(f.fontNew)
	width, height, truewidth, ascent := fontSizeNew(f.fontNew)
	f.width = width
	f.height = height
	f.truewidth = truewidth
	f.lineHeight = height + f.lineSpace
	f.ascent = ascent
	f.shift = int(float64(f.lineSpace)/2 + ascent)

	// reset character cache
	f.ws.screen.glyphMap = make(map[Cell]gui.QImage)
}

func (f *Font) changeLineSpace(lineSpace int) {
	f.lineSpace = lineSpace
	f.lineHeight = f.height + lineSpace
	f.shift = int(float64(lineSpace)/2 + f.ascent)
}
