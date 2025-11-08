package editor

import (
	"fmt"
	"math"

	"github.com/akiyosi/qt/gui"
)

type CellMetrics struct {
	cellwidth  float64
	lineHeight int
}

// Font is
type Font struct {
	family  string
	size    float64
	weight  gui.QFont__Weight
	stretch int

	ws             *Workspace
	qfont          *gui.QFont
	fontMetrics    *gui.QFontMetricsF
	width          float64
	cellwidth      float64
	italicWidth    float64
	ascent         float64
	descent        float64
	height         int
	lineHeight     int
	lineSpace      int
	letterSpace    int
	baselineOffset int
}

func fontSizeNew(font *gui.QFont) (float64, int, float64, float64) {
	fontMetrics := gui.NewQFontMetricsF(font)
	h := fontMetrics.Height()
	width := fontMetrics.HorizontalAdvance("w", -1)

	ascent := fontMetrics.Ascent()
	height := int(math.Ceil(h))
	font.SetStyle(gui.QFont__StyleItalic)
	italicFontMetrics := gui.NewQFontMetricsF(font)
	italicWidth := italicFontMetrics.BoundingRect("w").Width()
	if italicWidth <= width {
		italicWidth = width * 1.5
	}
	font.SetStyle(gui.QFont__StyleNormal)

	return width, height, ascent, italicWidth
}

func initFontNew(family string, size float64, weight gui.QFont__Weight, stretch, lineSpace, letterSpace int) *Font {
	// font := gui.NewQFont2(family, size, int(gui.QFont__Normal), false)
	font := gui.NewQFont()
	if editor.config.Editor.ManualFontFallback {
		font.SetStyleHint(gui.QFont__TypeWriter, gui.QFont__NoFontMerging|gui.QFont__ForceIntegerMetrics)
	} else {
		font.SetStyleHint(gui.QFont__TypeWriter, gui.QFont__PreferDefault|gui.QFont__ForceIntegerMetrics)
	}

	font.SetFamily(family)
	font.SetPointSizeF(size)
	font.SetWeight(int(weight))
	font.SetStretch(stretch)

	font.SetFixedPitch(true)
	font.SetKerning(false)

	width, height, ascent, italicWidth := fontSizeNew(font)

	return &Font{
		family:         family,
		size:           size,
		weight:         weight,
		stretch:        stretch,
		qfont:          font,
		fontMetrics:    gui.NewQFontMetricsF(font),
		width:          width,
		cellwidth:      width + float64(letterSpace),
		letterSpace:    letterSpace,
		height:         height,
		lineHeight:     height + lineSpace,
		lineSpace:      lineSpace,
		baselineOffset: int(math.Ceil(float64(height+lineSpace) - (float64(height) - ascent) - float64(lineSpace)/2.0)),
		ascent:         ascent,
		descent:        float64(height) - ascent,
		italicWidth:    italicWidth,
	}
}

func (f *Font) change(family string, size float64, weight gui.QFont__Weight, stretch int) {
	if f.family == family && f.size == size && f.weight == weight && f.stretch == stretch {
		return
	}

	f.qfont.SetFamily(family)

	if editor.config.Editor.ManualFontFallback {
		f.qfont.SetStyleHint(gui.QFont__TypeWriter, gui.QFont__NoFontMerging|gui.QFont__ForceIntegerMetrics)
	} else {
		f.qfont.SetStyleHint(gui.QFont__TypeWriter, gui.QFont__PreferDefault|gui.QFont__ForceIntegerMetrics)
	}

	f.qfont.SetPointSizeF(size)
	f.qfont.SetWeight(int(weight))
	f.qfont.SetStretch(stretch)

	f.qfont.SetFixedPitch(true)
	f.qfont.SetKerning(false)

	width, height, ascent, italicWidth := fontSizeNew(f.qfont)

	f.fontMetrics = gui.NewQFontMetricsF(f.qfont)
	f.width = width
	f.cellwidth = width + float64(f.letterSpace)
	// f.letterSpace is no change
	f.height = height
	f.lineHeight = height + f.lineSpace
	// f.lineSpace is no change
	f.baselineOffset = int(math.Ceil(float64(height+f.lineSpace) - (float64(height) - ascent) - float64(f.lineSpace)/2.0))
	f.ascent = ascent
	f.descent = float64(height) - ascent
	f.italicWidth = italicWidth

	f.family = family
	f.size = size
	f.weight = weight
	f.stretch = stretch

	// f.putDebugLog()
}

// func (f *Font) hasGlyph(s string) bool {
// 	rawfont := gui.NewQRawFont()
// 	rawfont = rawfont.FromFont(f.qfont, gui.QFontDatabase__Any)
//
// 	glyphIdx := rawfont.GlyphIndexesForString(s)
//
// 	if len(glyphIdx) > 0 {
// 		glyphIdx0 := glyphIdx[0]
//
// 		// fmt.Println(s, "::", glyphIdx0 != 0)
//		editor.putLog("hasGlyph:() debug::", s, ",", glyphIdx0 != 0)
// 		return glyphIdx0 != 0
// 	}
//
// 	return false
// }

func (f *Font) hasGlyph(s string) bool {
	if s == "" {
		return true
	}

	hasGlyph := f.fontMetrics.InFontUcs4(uint([]rune(s)[0]))

	return hasGlyph
}

func (f *Font) putDebugLog() {
	if editor.opts.Debug == "" {
		return
	}

	// rf := gui.NewQRawFont()
	// db := gui.NewQFontDatabase()
	// rf = rf.FromFont(f.qfont, gui.QFontDatabase__Any)
	fi := gui.NewQFontInfo(f.qfont)
	editor.putLog(
		"font family:",
		fi.Family(),
		fi.PointSizeF(),
		fi.StyleName(),
		fmt.Sprintf("%v", fi.PointSizeF()),
	)
}

func (f *Font) changeLineSpace(lineSpace int) {
	f.lineSpace = lineSpace
	f.lineHeight = f.height + lineSpace
	f.baselineOffset = int(math.Ceil(float64(f.height+lineSpace) - (float64(f.height) - f.ascent) - float64(lineSpace)/2.0))

	if f.ws == nil {
		return
	}
	f.ws.screen.purgeTextCacheForWins()
}

func (f *Font) changeLetterSpace(letterspace int) {
	width, _, _, italicWidth := fontSizeNew(f.qfont)

	f.letterSpace = letterspace
	f.cellwidth = width + float64(letterspace)
	f.italicWidth = italicWidth + float64(letterspace)

	f.ws.screen.purgeTextCacheForWins()
}
