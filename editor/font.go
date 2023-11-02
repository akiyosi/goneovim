package editor

import (
	"bytes"
	"math"
	"os"
	"strings"
	"sync"

	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
	"github.com/go-text/typesetting/font"
)

// Font is
type Font struct {
	family    string
	size      float64
	pixelSize int
	weight    gui.QFont__Weight
	stretch   int

	ws    *Workspace
	qfont *gui.QFont
	// fontMetrics *gui.QFontMetricsF
	rawfont     *RawFont
	width       float64
	cellwidth   float64
	italicWidth float64
	ascent      float64
	height      int
	lineHeight  int
	lineSpace   int
	letterSpace int
	shift       int
}

type RawFont struct {
	regular *gui.QRawFont
	bold    *gui.QRawFont
	italic  *gui.QRawFont

	regularface *font.Face
	boldface    *font.Face
	italicface  *font.Face

	wg sync.WaitGroup
}

func fontSizeNew(rawfont *gui.QRawFont) (float64, int, float64, float64) {
	gi := rawfont.AdvancesForGlyphIndexes2(rawfont.GlyphIndexesForString("w"))
	width := float64(int(gi[0].X()))
	ascent := rawfont.Ascent()
	h := ascent + rawfont.Descent()
	height := int(math.Ceil(h))
	italicWidth := width * 1.5

	return width, height, ascent, italicWidth
}

func initFont(genFontFaceAsync bool, family string, size float64, weight gui.QFont__Weight, stretch, lineSpace, letterSpace int) *Font {
	editor.putLog("start initFont()")

	dpi := editor.app.PrimaryScreen().LogicalDotsPerInch()
	pixelSize := int(math.Ceil(size * dpi / 72.0))

	font := &Font{
		family:    family,
		size:      size,
		pixelSize: pixelSize,
		weight:    weight,
		stretch:   stretch,
	}

	if genFontFaceAsync {
		font.rawfont = newRawFont(true, family, pixelSize, weight)
	} else {
		font.rawfont = newRawFont(false, family, pixelSize, weight)
	}
	if font.rawfont == nil {
		return nil
	}

	qfont := gui.NewQFont()

	if editor.config.Editor.ManualFontFallback {
		qfont.SetStyleHint(gui.QFont__TypeWriter, gui.QFont__NoFontMerging)
	} else {
		qfont.SetStyleHint(gui.QFont__TypeWriter, gui.QFont__PreferDefault|gui.QFont__ForceIntegerMetrics)
	}

	qfont.SetFamily(family)
	qfont.SetPointSizeF(size)

	var width, ascent, italicWidth float64
	var height int
	if font.rawfont != nil && font.rawfont.regular != nil {
		width, height, ascent, italicWidth = fontSizeNew(font.rawfont.regular)
	}

	font.qfont = qfont
	font.width = width
	font.cellwidth = width + float64(letterSpace)
	font.letterSpace = letterSpace
	font.height = height
	font.lineHeight = height + lineSpace
	font.lineSpace = lineSpace
	font.shift = int(float64(lineSpace)/2 + ascent)
	font.ascent = ascent
	font.italicWidth = italicWidth

	editor.putLog("finished initFont()")

	return font
}

func (f *Font) hasGlyph(s string) bool {
	if f == nil || f.rawfont == nil {
		return false
	}

	glyphIdx := f.rawfont.regular.GlyphIndexesForString(s)

	if len(glyphIdx) > 0 {
		glyphIdx0 := glyphIdx[0]

		return glyphIdx0 != 0
	}

	return false
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
	width, _, _, italicWidth := fontSizeNew(f.rawfont.regular)

	f.letterSpace = letterspace
	f.cellwidth = width + float64(letterspace)
	f.italicWidth = italicWidth + float64(letterspace)

	f.ws.screen.purgeTextCacheForWins()
}

func (f *Font) horizontalAdvance(str string) (width float64) {
	if str == "" {
		return
	}

	a := f.rawfont.regular.GlyphIndexesForString(str)
	gi := f.rawfont.regular.AdvancesForGlyphIndexes2(a)
	for _, gie := range gi {
		width += gie.X()
	}

	return
}

func newRawFont(genFontFaceAsync bool, fontFamilyName string, size int, weight gui.QFont__Weight) *RawFont {

	locations := editor.fontmap.FindSystemFonts(fontFamilyName)
	if len(locations) == 0 {
		return nil
	}

	var italic string
	var regular string
	var bold string

	var thinPath string
	var extralightPath string
	var lightPath string
	var regularPath string
	var mediumPath string
	var semiboldPath string
	var boldPath string
	var extraboldPath string
	var blackPath string

	for _, l := range locations {
		location := l.File
		if isFontIsThinStyle(location) {
			thinPath = location
		} else if isFontIsExtraLightStyle(location) {
			extralightPath = location
		} else if isFontIsLightStyle(location) {
			lightPath = location
		} else if isFontIsMediumStyle(location) {
			mediumPath = location
		} else if isFontIsSemiBoldStyle(location) {
			semiboldPath = location
		} else if isFontIsExtraBoldStyle(location) {
			extraboldPath = location
		} else if isFontIsBoldStyle(location) {
			boldPath = location
		} else if isFontIsBlackStyle(location) {
			blackPath = location
		} else if isFontIsNormalStyle(location) {
			regularPath = location
		} else if isItalic(location) {
			italic = location
		} else {
			regularPath = location
		}
	}

	filePathR := []string{thinPath, extralightPath, lightPath, regularPath, mediumPath}
	filePathB := []string{semiboldPath, boldPath, extraboldPath, blackPath}

	switch weight {
	case gui.QFont__Thin:
		regular = assignFontWeight(filePathR, 0)
		bold = assignFontWeight(filePathB, 0)
	case gui.QFont__ExtraLight:
		regular = assignFontWeight(filePathR, 1)
		bold = assignFontWeight(filePathB, 1)
	case gui.QFont__Light:
		regular = assignFontWeight(filePathR, 2)
		bold = assignFontWeight(filePathB, 2)
	case gui.QFont__Normal:
		regular = assignFontWeight(filePathR, 3)
		bold = assignFontWeight(filePathB, 3)
	case gui.QFont__Medium:
		regular = assignFontWeight(filePathR, 4)
		bold = assignFontWeight(filePathB, 3)
	case gui.QFont__DemiBold:
		regular = assignFontWeight(filePathB, 0)
		bold = assignFontWeight(filePathB, 3)
	case gui.QFont__Bold:
		regular = assignFontWeight(filePathB, 1)
		bold = assignFontWeight(filePathB, 3)
	case gui.QFont__ExtraBold:
		regular = assignFontWeight(filePathB, 2)
		bold = assignFontWeight(filePathB, 3)
	case gui.QFont__Black:
		regular = assignFontWeight(filePathB, 3)
		bold = assignFontWeight(filePathB, 3)
	}

	var regularFace *font.Face
	var boldFace *font.Face
	var italicFace *font.Face

	var regularFont *gui.QRawFont
	var boldFont *gui.QRawFont
	var italicFont *gui.QRawFont

	// var regularBytes []byte
	// var boldBytes []byte
	// var italicBytes []byte

	var waitgroup sync.WaitGroup

	rawfont := &RawFont{
		regular: regularFont,
		bold:    boldFont,
		italic:  italicFont,

		regularface: regularFace,
		boldface:    boldFace,
		italicface:  italicFace,

		wg: waitgroup,
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		rawfont.regular = loadQRawFont2(regular, size)
	}()

	if bold != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rawfont.bold = loadQRawFont2(bold, size)
		}()
	}

	if italic != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rawfont.italic = loadQRawFont2(italic, size)
		}()
	}

	if genFontFaceAsync {
		wg.Wait()

		rawfont.wg.Add(1)
		go func() {
			defer rawfont.wg.Done()
			rawfont.regularface, _ = readFontFile(regular)
			// rawfont.boldface, _ = readFontFile(bold)
			// rawfont.italicface, _ = readFontFile(italic)
		}()

		rawfont.wg.Add(1)
		go func() {
			defer rawfont.wg.Done()
			rawfont.boldface, _ = readFontFile(bold)
		}()

		rawfont.wg.Add(1)
		go func() {
			defer rawfont.wg.Done()
			rawfont.italicface, _ = readFontFile(italic)
		}()

	} else {

		wg.Add(1)
		go func() {
			defer wg.Done()
			rawfont.regularface, _ = readFontFile(regular)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			rawfont.boldface, _ = readFontFile(bold)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			rawfont.italicface, _ = readFontFile(italic)
		}()

		wg.Wait()
	}

	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	rawfont.regularface, _ = readFontFile(regular)
	// }()

	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	rawfont.boldface, _ = readFontFile(bold)
	// }()

	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	rawfont.italicface, _ = readFontFile(italic)
	// }()

	wg.Wait()

	return rawfont
}

func assignFontWeight(filePathList []string, i int) (fontWeightPath string) {
	for j, path := range filePathList {
		if j < i {
			continue
		}
		if path == "" {
			continue
		}
		if fontWeightPath == "" {
			fontWeightPath = path
			break
		}
	}
	if fontWeightPath == "" {
		for j := i - 1; j >= 0; j-- {
			if j < 0 {
				break
			}
			path := filePathList[j]
			if path == "" {
				continue
			}
			if fontWeightPath == "" {
				fontWeightPath = path
				break
			}
		}
	}

	return
}

func readFontFile(path string) (*font.Face, []byte) {
	if path == "" {
		return nil, []byte{}
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, []byte{}
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, []byte{}
	}

	fileBytes := make([]byte, fileInfo.Size())
	_, err = file.Read(fileBytes)
	if err != nil {
		return nil, []byte{}
	}

	face, err := font.ParseTTF(bytes.NewReader(fileBytes))
	if err != nil {
		return nil, []byte{}
	}

	return &face, fileBytes
}

func loadQRawFont(data []byte, size int) *gui.QRawFont {
	return gui.NewQRawFont3(
		core.NewQByteArray2(string(data), len(data)),
		float64(size),
		gui.QFont__PreferDefaultHinting,
	)
}

func loadQRawFont2(filepath string, size int) *gui.QRawFont {
	return gui.NewQRawFont2(
		filepath,
		float64(size),
		gui.QFont__PreferDefaultHinting,
	)
}

// TODO: Support Oblique

func isItalic(location string) bool {
	return strings.Contains(strings.ToLower(location), "italic") || strings.Contains(strings.ToLower(location), "it")
}

func isFontIsThinStyle(location string) bool {
	isThin := strings.Contains(strings.ToLower(location), "thin") || strings.Contains(strings.ToLower(location), "Hairline")

	return isThin && !isItalic(location)
}

func isFontIsExtraLightStyle(location string) bool {
	isEL := strings.Contains(strings.ToLower(location), "extralight") || strings.Contains(strings.ToLower(location), "ultralight")
	return isEL && !isItalic(location)
}

func isFontIsLightStyle(location string) bool {
	isLight := strings.Contains(strings.ToLower(location), "light")
	return isLight && !isItalic(location)
}

func isFontIsNormalStyle(location string) bool {
	isRegular := strings.Contains(strings.ToLower(location), "regular") || strings.Contains(strings.ToLower(location), "normal")
	return isRegular && !isItalic(location)
}

func isFontIsMediumStyle(location string) bool {
	isMedium := strings.Contains(strings.ToLower(location), "medium")
	return isMedium && !isItalic(location)
}

func isFontIsSemiBoldStyle(location string) bool {
	isSB := strings.Contains(strings.ToLower(location), "semibold") || strings.Contains(strings.ToLower(location), "demibold")
	return isSB && !isItalic(location)
}

func isFontIsBoldStyle(location string) bool {
	isBold := strings.Contains(strings.ToLower(location), "bold")
	return isBold && !isItalic(location)
}

func isFontIsExtraBoldStyle(location string) bool {
	isEB := strings.Contains(strings.ToLower(location), "extrabold") || strings.Contains(strings.ToLower(location), "ultrabold")
	return isEB && !isItalic(location)
}

func isFontIsBlackStyle(location string) bool {
	isBlack := strings.Contains(strings.ToLower(location), "black") || strings.Contains(strings.ToLower(location), "heavy")
	return isBlack && !isItalic(location)
}
