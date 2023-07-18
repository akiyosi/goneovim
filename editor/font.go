package editor

import (
	"fmt"
	"math"

	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

// Font is
type Font struct {
	ws          *Workspace
	fontNew     *gui.QFont
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

func fontSizeNew(font *gui.QFont) (float64, int, float64, float64) {
	fontMetrics := gui.NewQFontMetricsF(font)
	h := fontMetrics.Height()
	width := fontMetrics.HorizontalAdvance("w", -1)

	// // On Windows, it may take a long time (~500ms) to drawing CJK characters for qpainter.
	// // Therefore, we will run this process in concurrently in the background of attaching to neovim.
	// // This issue may also be related to the following.
	// // https://github.com/equalsraf/neovim-qt/issues/614
	// if runtime.GOOS == "windows" && !editor.config.Editor.NoFontMerge {
	// 	go fontMetrics.HorizontalAdvance("無未제", -1)
	// }

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

func initFontNew(family string, size float64, lineSpace int, letterSpace float64) *Font {
	// font := gui.NewQFont2(family, size, int(gui.QFont__Normal), false)
	font := gui.NewQFont()
	font.SetFamily(family)
	font.SetPointSizeF(size)
	font.SetWeight(int(gui.QFont__Normal))

	if editor.config.Editor.NoFontMerge {
		font.SetStyleHint(gui.QFont__TypeWriter, gui.QFont__NoFontMerging)
	} else {
		font.SetStyleHint(gui.QFont__TypeWriter, gui.QFont__PreferDefault|gui.QFont__ForceIntegerMetrics)
	}

	font.SetFixedPitch(true)
	font.SetKerning(false)

	width, height, ascent, italicWidth := fontSizeNew(font)

	return &Font{
		fontNew:     font,
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
	f.fontNew.SetFamily(family)
	f.fontNew.SetPointSizeF(size)
	f.fontNew.SetWeight(int(weight))
	f.fontNew.SetStretch(stretch)
	f.fontMetrics = gui.NewQFontMetricsF(f.fontNew)
	width, height, ascent, italicWidth := fontSizeNew(f.fontNew)
	f.height = height
	f.cellwidth = width + f.letterSpace
	f.lineHeight = height + f.lineSpace
	f.ascent = ascent
	f.shift = int(float64(f.lineSpace)/2 + ascent)
	f.italicWidth = italicWidth

	f.putDebugLog()
	f.ws.screen.purgeTextCacheForWins()
}

func (f *Font) putDebugLog() {
	if editor.opts.Debug == "" {
		return
	}

	// rf := gui.NewQRawFont()
	// db := gui.NewQFontDatabase()
	// rf = rf.FromFont(f.fontNew, gui.QFontDatabase__Any)
	fi := gui.NewQFontInfo(f.fontNew)
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

	f.ws.screen.purgeTextCacheForWins()
}

func (f *Font) changeLetterSpace(letterspace float64) {
	width, _, _, italicWidth := fontSizeNew(f.fontNew)

	f.letterSpace = letterspace
	f.cellwidth = width + letterspace
	f.italicWidth = italicWidth + letterspace

	f.ws.screen.purgeTextCacheForWins()
}
