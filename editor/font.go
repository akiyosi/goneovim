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
	italicWidth        float64
	ascent             float64
	height             int
	lineHeight         int
	lineSpace          int
	shift              int
}

func fontSizeNew(font *gui.QFont) (int, int, float64, float64, float64) {
	fontMetrics := gui.NewQFontMetricsF(font)
	h := fontMetrics.Height()
	w := fontMetrics.HorizontalAdvance("W", -1)
	ascent := fontMetrics.Ascent()
	width := int(math.Ceil(w))
	height := int(math.Ceil(h))
	font.SetStyle(gui.QFont__StyleItalic)
	italicFontMetrics := gui.NewQFontMetricsF(font)
	italicWidth := italicFontMetrics.BoundingRect("f").Width()
	font.SetStyle(gui.QFont__StyleNormal)

	return width, height, w, ascent, italicWidth
}

func initFontNew(family string, size int, lineSpace int, fast bool) *Font {
	font := gui.NewQFont2(family, size, int(gui.QFont__Normal), false)

	var width, height int
	var truewidth, ascent, italicWidth float64
	// fontMetrics calculate is too slow, so we calculate approximate font metrics
	if !fast {
		width, height, truewidth, ascent, italicWidth = fontSizeNew(font)
	} else {
		h := math.Trunc(float64(size)*1.28) + 2.0
		truewidth = math.Ceil(float64(size) * 0.7)
		height = int(math.Ceil(h))
		width = int(math.Ceil(truewidth))
		ascent = math.Ceil(float64(size) * 1.1)
		italicWidth = math.Ceil(float64(size) * 0.8)
	}
	defaultFont := gui.NewQFont()
	return &Font{
		fontNew:            font,
		fontMetrics:        gui.NewQFontMetricsF(font),
		defaultFont:        defaultFont,
		defaultFontMetrics: gui.NewQFontMetricsF(defaultFont),
		width:              width,
		truewidth:          truewidth,
		height:             height,
		lineHeight:         height + lineSpace,
		lineSpace:          lineSpace,
		shift:              int(float64(lineSpace)/2 + ascent),
		ascent:             ascent,
		italicWidth:        italicWidth,
	}
}

func (f *Font) change(family string, size int) {
	f.fontNew.SetFamily(family)
	f.fontNew.SetPointSize(size)
	f.fontMetrics = gui.NewQFontMetricsF(f.fontNew)
	width, height, truewidth, ascent, italicWidth := fontSizeNew(f.fontNew)
	f.width = width
	f.height = height
	f.truewidth = truewidth
	f.lineHeight = height + f.lineSpace
	f.ascent = ascent
	f.shift = int(float64(f.lineSpace)/2 + ascent)
	f.italicWidth = italicWidth

	// Purge cache
	if editor.config.Editor.CachedDrawing {
		f.ws.screen.textCache.Purge()
		// for _, win := range f.ws.screen.windows {
		// 	if win == nil {
		// 		continue
		// 	}
		// 	if win.font == nil {
		// 		continue
		// 	}
		// 	win.textCache.Purge()
		// }
		f.ws.screen.windows.Range(func(_, winITF interface{}) bool {
			win := winITF.(*Window)
			if win == nil {
				return true
			}
			if win.font == nil {
				return true
			}
			win.textCache.Purge()
			return true
		})
	}
}

func (f *Font) changeLineSpace(lineSpace int) {
	f.lineSpace = lineSpace
	f.lineHeight = f.height + lineSpace
	f.shift = int(float64(lineSpace)/2 + f.ascent)
}
