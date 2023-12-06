package editor

import (
	"fmt"
	"math"
	"strconv"

	"github.com/akiyosi/qt/gui"
	"github.com/akiyosi/qt/widgets"
	"github.com/go-text/typesetting/fontscan"
)

// Font is
type Font struct {
	ws          *Workspace
	qfont       *gui.QFont
	rawfont     *gui.QRawFont
	fontMetrics *gui.QFontMetricsF
	width       float64
	cellwidth   float64
	italicWidth float64
	ascent      float64
	height      int
	lineHeight  int
	lineSpace   int
	letterSpace float64
	shift       int
	ui          *widgets.QFontDialog
}

func fontSizeNew(font *gui.QFont) (float64, int, float64, float64, *gui.QRawFont) {
	editor.putLog("fontSizeNew debug 1")
	// fontMetrics := gui.NewQFontMetricsF(font)
	editor.putLog("fontSizeNew debug 2")

	// FontMapのインスタンスを作成
	fm := fontscan.NewFontMap(nil)

	// システムフォントをロード
	err := fm.UseSystemFonts("")
	if err != nil {
		panic(err)
	}

	// フォントファミリに基づいてフォントを検索
	var location fontscan.Location
	var found bool
	location, found = fm.FindSystemFont(font.Family())
	if !found {
		fmt.Println("the font not found")
	}

	editor.putLog("font file location:", location.File)

	rawfont := gui.NewQRawFont2(
		location.File,
		float64(font.PixelSize()),
		gui.QFont__PreferDefaultHinting,
	)

	// fmt.Println("pixel size:", font.PixelSize(), "point size:", font.PointSizeF())

	editor.putLog("fontSizeNew debug 2-1")
	uintChar, _ := strconv.ParseUint("w", 10, 64)
	var a []uint
	a = append(a, uint(uintChar))
	gi := rawfont.AdvancesForGlyphIndexes2(a)
	gi0 := gi[0]

	editor.putLog("fontSizeNew debug 2-2")

	// width := fontMetrics.HorizontalAdvance("w", -1)
	width := gi0.X()

	editor.putLog("fontSizeNew debug 3")

	// ascent := fontMetrics.Ascent()
	ascent := rawfont.Ascent()

	editor.putLog("fontSizeNew debug 4")

	// fmt.Println(
	// 	gi0.X(),
	// 	rawfont.Ascent()+rawfont.Descent(),
	// 	"|",
	// 	width,
	// 	h,
	// )

	// h := fontMetrics.Height()
	h := ascent + rawfont.Descent()

	// fmt.Println("rawfont:", width, h, "metrics:", fontMetrics.HorizontalAdvance("w", -1), fontMetrics.Height())

	editor.putLog("fontSizeNew debug 5")
	height := int(math.Ceil(h))
	editor.putLog("fontSizeNew debug 6")
	font.SetStyle(gui.QFont__StyleItalic)
	editor.putLog("fontSizeNew debug 7")

	// italicFontMetrics := gui.NewQFontMetricsF(font)
	// editor.putLog("fontSizeNew debug 8")

	// italicWidth := italicFontMetrics.BoundingRect("w").Width()
	editor.putLog("fontSizeNew debug 9")
	// if italicWidth <= width {
	italicWidth := width * 1.5
	// }
	font.SetStyle(gui.QFont__StyleNormal)
	editor.putLog("fontSizeNew debug 10")

	return width, height, ascent, italicWidth, rawfont
}

func initFontNew(family string, size float64, lineSpace int, letterSpace float64) *Font {
	// font := gui.NewQFont2(family, size, int(gui.QFont__Normal), false)
	font := gui.NewQFont()
	if editor.config.Editor.ManualFontFallback {
		font.SetStyleHint(gui.QFont__TypeWriter, gui.QFont__NoFontMerging)
	} else {
		font.SetStyleHint(gui.QFont__TypeWriter, gui.QFont__PreferDefault|gui.QFont__ForceIntegerMetrics)
	}

	font.SetFamily(family)
	font.SetPixelSize(int(math.Ceil(size)))
	font.SetWeight(int(gui.QFont__Normal))

	font.SetFixedPitch(true)
	font.SetKerning(false)

	editor.putLog("initFontNew:", family)
	width, height, ascent, italicWidth, rawfont := fontSizeNew(font)

	return &Font{
		qfont:       font,
		rawfont:     rawfont,
		fontMetrics: gui.NewQFontMetricsF(font),
		width:       width,
		cellwidth:   width + letterSpace,
		letterSpace: letterSpace,
		height:      height,
		lineHeight:  height + lineSpace,
		lineSpace:   lineSpace,
		shift:       int(float64(lineSpace)/2 + ascent),
		ascent:      ascent,
		italicWidth: italicWidth,
	}
}

func (f *Font) change(family string, size float64, weight gui.QFont__Weight, stretch int) {
	editor.putLog("change debug 0")
	f.qfont.SetFamily(family)
	editor.putLog("change debug 1")

	if editor.config.Editor.ManualFontFallback {
		f.qfont.SetStyleHint(gui.QFont__TypeWriter, gui.QFont__NoFontMerging)
	} else {
		f.qfont.SetStyleHint(gui.QFont__TypeWriter, gui.QFont__PreferDefault|gui.QFont__ForceIntegerMetrics)
	}
	editor.putLog("change debug 2")

	f.qfont.SetPixelSize(int(math.Ceil(size)))
	f.qfont.SetWeight(int(weight))
	f.qfont.SetStretch(stretch)
	editor.putLog("change debug 3")

	f.qfont.SetFixedPitch(true)
	f.qfont.SetKerning(false)

	editor.putLog("change debug 4")
	editor.putLog("change:", family)
	width, height, ascent, italicWidth, rawfont := fontSizeNew(f.qfont)
	editor.putLog("change debug 5")

	f.rawfont = rawfont
	f.fontMetrics = gui.NewQFontMetricsF(f.qfont)
	editor.putLog("change debug 6")
	f.cellwidth = width + f.letterSpace
	// f.letterSpace is no change
	f.height = height
	f.lineHeight = height + f.lineSpace
	f.ascent = ascent
	f.shift = int(float64(f.lineSpace)/2 + ascent)
	f.italicWidth = italicWidth
	editor.putLog("change debug 7")

	// f.putDebugLog()
}

func (f *Font) hasGlyph(s string) bool {
	glyphIdx := f.rawfont.GlyphIndexesForString(s)

	if len(glyphIdx) > 0 {
		glyphIdx0 := glyphIdx[0]

		// fmt.Println(s, "::", glyphIdx0 != 0)
		editor.putLog("hasGlyph:() debug::", s, ",", glyphIdx0 != 0)
		return glyphIdx0 != 0
	}

	return false
}

// func (f *Font) hasGlyph(s string) bool {
// 	if s == "" {
// 		return true
// 	}
//
// 	hasGlyph := f.fontMetrics.InFontUcs4(uint([]rune(s)[0]))
// 	editor.putLog("hasGlyph:() debug::", s, ",", hasGlyph)
//
// 	return hasGlyph
// }

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
	f.shift = int(float64(lineSpace)/2 + f.ascent)

	if f.ws == nil {
		return
	}
	f.ws.screen.purgeTextCacheForWins()
}

func (f *Font) changeLetterSpace(letterspace float64) {
	width, _, _, italicWidth, _ := fontSizeNew(f.qfont)

	f.letterSpace = letterspace
	f.cellwidth = width + letterspace
	f.italicWidth = italicWidth + letterspace

	f.ws.screen.purgeTextCacheForWins()
}
