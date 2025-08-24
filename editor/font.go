package editor

import (
	"fmt"
	"math"

	"github.com/akiyosi/qt/gui"
)

// Font is
type Font struct {
	family  string
	size    float64
	weight  gui.QFont__Weight
	stretch int

	ws          *Workspace
	qfont       *gui.QFont
	fontMetrics *gui.QFontMetricsF
	width       float64
	cellwidth   float64
	italicWidth float64
	ascent      float64
	height      int
	lineHeight  int
	lineSpace   int
	letterSpace int
	shift       int

	// Variant font metrics (only set for Proportional Fonts)
	proportional          bool
	italicFontMetrics     *gui.QFontMetricsF
	boldFontMetrics       *gui.QFontMetricsF
	italicBoldFontMetrics *gui.QFontMetricsF
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

	proportional := !gui.NewQFontInfo(font).FixedPitch()
	font.SetFixedPitch(proportional)
	font.SetKerning(false)

	width, height, ascent, italicWidth := fontSizeNew(font)

	// Proportional Fonts
	var italicFM, boldFM, italicBoldFM *gui.QFontMetricsF
	if proportional {
		italicFM, boldFM, italicBoldFM = getFontMetricsVariants(font)
	}

	return &Font{
		family:      family,
		size:        size,
		weight:      weight,
		stretch:     stretch,
		qfont:       font,
		fontMetrics: gui.NewQFontMetricsF(font),
		width:       width,
		cellwidth:   width + float64(letterSpace),
		letterSpace: letterSpace,
		height:      height,
		lineHeight:  height + lineSpace,
		lineSpace:   lineSpace,
		shift:       int(float64(lineSpace)/2 + ascent),
		ascent:      ascent,
		italicWidth: italicWidth,

		// Proportional fonts
		proportional:          proportional,
		italicFontMetrics:     italicFM,
		boldFontMetrics:       boldFM,
		italicBoldFontMetrics: italicBoldFM,
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

	f.proportional = !gui.NewQFontInfo(f.qfont).FixedPitch()
	f.qfont.SetFixedPitch(f.proportional)
	f.qfont.SetKerning(false)

	width, height, ascent, italicWidth := fontSizeNew(f.qfont)

	f.fontMetrics = gui.NewQFontMetricsF(f.qfont)
	f.cellwidth = width + float64(f.letterSpace)
	// f.letterSpace is no change
	f.height = height
	f.lineHeight = height + f.lineSpace
	f.ascent = ascent
	f.shift = int(float64(f.lineSpace)/2 + ascent)
	f.italicWidth = italicWidth

	// Font Metrics variants for proportional fonts
	if f.proportional {
		f.italicFontMetrics, f.boldFontMetrics, f.italicBoldFontMetrics = getFontMetricsVariants(f.qfont)
	}

	f.family = family
	f.size = size
	f.weight = weight
	f.stretch = stretch

	// f.putDebugLog()
}

// Get Font Metrics variants (italic, bold, bold+italic)
func getFontMetricsVariants(font *gui.QFont) (italicFM, boldFM, italicBoldFM *gui.QFontMetricsF) {
	font.SetStyle(gui.QFont__StyleItalic)
	italicFM = gui.NewQFontMetricsF(font)
	font.SetBold(true)
	italicBoldFM = gui.NewQFontMetricsF(font)
	font.SetStyle(gui.QFont__StyleNormal)
	boldFM = gui.NewQFontMetricsF(font)
	font.SetBold(false)
	return italicFM, boldFM, italicBoldFM
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
		fmt.Sprintf("(fixed pitch: %v) ", fi.FixedPitch()),
	)
}

func (f *Font) changeLineSpace(lineSpace int) {
	f.lineSpace = lineSpace
	f.lineHeight = f.height + lineSpace
	f.shift = int(float64(lineSpace)/2 + f.ascent)

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
