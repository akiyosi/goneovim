package editor

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"

	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
	"github.com/go-text/typesetting/font"
	apifont "github.com/go-text/typesetting/opentype/api/font"
	"github.com/go-text/typesetting/opentype/api/metadata"
	"github.com/go-text/typesetting/opentype/loader"
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
	regular    *gui.QRawFont
	bold       *gui.QRawFont
	italic     *gui.QRawFont
	boldItalic *gui.QRawFont

	regularface    *font.Face
	boldface       *font.Face
	italicface     *font.Face
	boldItalicFace *font.Face

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

	fontpathes := []string{}
	for _, l := range locations {
		skip := false
		for _, fontpath := range fontpathes {
			if fontpath == l.File {
				skip = true
			}
		}
		if skip {
			continue
		}
		fontpathes = append(fontpathes, l.File)
	}

	isTTC := isTtcFile(fontpathes[0])

	var rawfont *RawFont
	if isTTC {
		rawfont = newRawFontFromTTC(fontpathes, genFontFaceAsync, fontFamilyName, size, weight)
	} else {
		rawfont = newRawFontFromTTF(fontpathes, genFontFaceAsync, fontFamilyName, size, weight)
	}

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

func assignFontWeight2(loaderList []*loader.Loader, i int) (fontld *loader.Loader) {
	for j, ld := range loaderList {
		if j < i {
			continue
		}
		if ld == nil {
			continue
		}
		if fontld == nil {
			fontld = ld
			break
		}
	}
	if fontld == nil {
		for j := i - 1; j >= 0; j-- {
			if j < 0 {
				break
			}
			ld := loaderList[j]
			if ld == nil {
				continue
			}
			if fontld == nil {
				fontld = ld
				break
			}
		}
	}

	return
}

func readTtfFile(path string) (*font.Face, []byte) {
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

func readTtcFile(path string, size int, weight gui.QFont__Weight) *RawFont {
	if path == "" {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil
	}

	fileBytes := make([]byte, fileInfo.Size())
	_, err = file.Read(fileBytes)
	if err != nil {
		return nil
	}

	lds, err := loader.NewLoaders(bytes.NewReader(fileBytes))
	if err != nil {
		return nil
	}
	// out := make([]*font.Face, len(lds))

	var thinLd *loader.Loader
	var extraLightLd *loader.Loader
	var lightLd *loader.Loader
	var regularLd *loader.Loader
	var mediumLd *loader.Loader
	var semiBoldLd *loader.Loader
	var boldLd *loader.Loader
	var extraBoldLd *loader.Loader
	var blackLd *loader.Loader

	var regular *loader.Loader
	var bold *loader.Loader
	var italic *loader.Loader

	for _, ld := range lds {
		aspect := metadata.Metadata(ld).Aspect
		fmt.Println(aspect)

		if aspect.Style == 1 && aspect.Weight == metadata.WeightThin {

			thinLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == metadata.WeightExtraLight {
			extraLightLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == metadata.WeightLight {
			lightLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == metadata.WeightNormal {
			regularLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == metadata.WeightMedium {
			mediumLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == metadata.WeightSemibold {
			semiBoldLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == metadata.WeightBold {
			boldLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == metadata.WeightExtraBold {
			extraBoldLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == metadata.WeightBlack {
			blackLd = ld
		}

		// TODO
		if aspect.Style == 2 && aspect.Weight == metadata.WeightNormal {
			italic = ld
		}

	}

	ldR := []*loader.Loader{thinLd, extraLightLd, lightLd, regularLd, mediumLd}
	ldB := []*loader.Loader{semiBoldLd, boldLd, extraBoldLd, blackLd}

	switch weight {
	case gui.QFont__Thin:
		regular = assignFontWeight2(ldR, 0)
		bold = assignFontWeight2(ldB, 0)
	case gui.QFont__ExtraLight:
		regular = assignFontWeight2(ldR, 1)
		bold = assignFontWeight2(ldB, 1)
	case gui.QFont__Light:
		regular = assignFontWeight2(ldR, 2)
		bold = assignFontWeight2(ldB, 2)
	case gui.QFont__Normal:
		regular = assignFontWeight2(ldR, 3)
		bold = assignFontWeight2(ldB, 3)
	case gui.QFont__Medium:
		regular = assignFontWeight2(ldR, 4)
		bold = assignFontWeight2(ldB, 3)
	case gui.QFont__DemiBold:
		regular = assignFontWeight2(ldB, 0)
		bold = assignFontWeight2(ldB, 3)
	case gui.QFont__Bold:
		regular = assignFontWeight2(ldB, 1)
		bold = assignFontWeight2(ldB, 3)
	case gui.QFont__ExtraBold:
		regular = assignFontWeight2(ldB, 2)
		bold = assignFontWeight2(ldB, 3)
	case gui.QFont__Black:
		regular = assignFontWeight2(ldB, 3)
		bold = assignFontWeight2(ldB, 3)
	}

	var regularFace *font.Face
	var boldFace *font.Face
	var italicFace *font.Face

	var regularFont *gui.QRawFont
	var boldFont *gui.QRawFont
	var italicFont *gui.QRawFont

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

	// regular face
	regularFt, err := apifont.NewFont(regular)
	if err != nil {
		return nil
	}
	regularface := &apifont.Face{Font: regularFt}
	rawfont.regularface = &regularface

	if bold != nil {
		// bold face
		boldFt, err := apifont.NewFont(bold)
		if err != nil {
			return nil
		}
		boldface := &apifont.Face{Font: boldFt}
		rawfont.boldface = &boldface
	} else {
		rawfont.boldface = &regularface
	}

	if italic != nil {
		// italic face
		italicFt, err := apifont.NewFont(italic)
		if err != nil {
			return nil
		}
		italicface := &apifont.Face{Font: italicFt}
		rawfont.italicface = &italicface
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// cmap, err1 := regular.RawTable(loader.MustNewTag("cmap"))
		// glyf, err2 := regular.RawTable(loader.MustNewTag("glyf"))
		// head, err3 := regular.RawTable(loader.MustNewTag("head"))
		// hhea, err4 := regular.RawTable(loader.MustNewTag("hhea"))
		// hmtx, err5 := regular.RawTable(loader.MustNewTag("hmtx"))
		// loca, err6 := regular.RawTable(loader.MustNewTag("loca"))
		// maxp, err7 := regular.RawTable(loader.MustNewTag("maxp"))
		// name, err8 := regular.RawTable(loader.MustNewTag("name"))
		// post, err9 := regular.RawTable(loader.MustNewTag("post"))
		// data := []byte{}
		// data = append(data, cmap...)
		// data = append(data, glyf...)
		// data = append(data, head...)
		// data = append(data, hhea...)
		// data = append(data, hmtx...)
		// data = append(data, loca...)
		// data = append(data, maxp...)
		// data = append(data, name...)
		// data = append(data, post...)
		rawfont.regular = loadQRawFont2(path, size)
	}()

	// fmt.Println(err1, err2, err3, err4, err5, err6, err7, err8, err9)
	// data := byteConcat(cmap, glyf, head, hhea, hmtx, loca, maxp, name, post)

	// if bold != nil {
	// 	wg.Add(1)
	// 	go func() {
	// 		defer wg.Done()
	// 		data, _ := bold.RawTable(loader.MustNewTag("glyf"))
	// 		rawfont.bold = loadQRawFont(data, size)
	// 		// rawfont.bold = loadQRawFont(bold, size)
	// 	}()
	// }

	// if italic != nil {
	// 	wg.Add(1)
	// 	go func() {
	// 		defer wg.Done()
	// 		data, _ := italic.RawTable(loader.MustNewTag("glyf"))
	// 		rawfont.italic = loadQRawFont(data, size)
	// 		// rawfont.italic = loadQRawFont(italic, size)
	// 	}()
	// }
	wg.Wait()

	// return &RawFont{
	// 	regular: regularFont,
	// 	bold:    boldFont,
	// 	italic:  italicFont,

	// 	regularface: regularFace,
	// 	boldface:    boldFace,
	// 	italicface:  italicFace,

	// 	wg: waitgroup,
	// }

	return rawfont

}

func newRawFontFromTTC(pathes []string, genFontFaceAsync bool, fontFamilyName string, size int, weight gui.QFont__Weight) *RawFont {
	var rawfont *RawFont
	for i, path := range pathes {
		fmt.Println(i)
		rawfont = readTtcFile(path, size, weight)
	}

	return rawfont
}

func newRawFontFromTTF(pathes []string, genFontFaceAsync bool, fontFamilyName string, size int, weight gui.QFont__Weight) *RawFont {
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

	for _, path := range pathes {

		if isFontIsThinStyle(path) && !isOblique(path) && !isItalic(path) {
			thinPath = path
		} else if isFontIsExtraLightStyle(path) && !isOblique(path) && !isItalic(path) {
			extralightPath = path
		} else if isFontIsLightStyle(path) && !isOblique(path) && !isItalic(path) {
			lightPath = path
		} else if isFontIsMediumStyle(path) && !isOblique(path) && !isItalic(path) {
			mediumPath = path
		} else if isFontIsSemiBoldStyle(path) && !isOblique(path) && !isItalic(path) {
			semiboldPath = path
		} else if isFontIsExtraBoldStyle(path) && !isOblique(path) && !isItalic(path) {
			extraboldPath = path
		} else if isFontIsBoldStyle(path) && !isOblique(path) && !isItalic(path) {
			boldPath = path
		} else if isFontIsBlackStyle(path) && !isOblique(path) && !isItalic(path) {
			blackPath = path
		} else if isFontIsNormalStyle(path) && !isOblique(path) && !isItalic(path) {
			regularPath = path
		} else if isItalic(path) {
			italic = path
		} else {
			regularPath = path
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
			rawfont.regularface, _ = readTtfFile(regular)
		}()

		rawfont.wg.Add(1)
		go func() {
			defer rawfont.wg.Done()
			rawfont.boldface, _ = readTtfFile(bold)
		}()

		rawfont.wg.Add(1)
		go func() {
			defer rawfont.wg.Done()
			rawfont.italicface, _ = readTtfFile(italic)
		}()

	} else {

		wg.Add(1)
		go func() {
			defer wg.Done()
			rawfont.regularface, _ = readTtfFile(regular)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			rawfont.boldface, _ = readTtfFile(bold)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			rawfont.italicface, _ = readTtfFile(italic)
		}()

		wg.Wait()
	}

	wg.Wait()

	return rawfont
}

func isTtcFile(path string) bool {
	return strings.HasSuffix(path, ".ttc")
}

// // ParseTTC parse an Opentype font file, with support for collections.
// // Single font files are supported, returning a slice with length 1.
// func parseTTC(file Resource) ([]Face, error) {
// 	lds, err := loader.NewLoaders(file)
// 	if err != nil {
// 		return nil, err
// 	}
// 	out := make([]Face, len(lds))
// 	for i, ld := range lds {
// 		ft, err := font.NewFont(ld)
// 		if err != nil {
// 			return nil, fmt.Errorf("reading font %d of collection: %s", i, err)
// 		}
// 		out[i] = &font.Face{Font: ft}
// 	}
//
// 	return out, nil
// }

func byteConcat(s ...[]byte) []byte {
	n := 0
	for _, v := range s {
		n += len(v)
	}

	b, i := make([]byte, n), 0
	for _, v := range s {
		i += copy(b[i:], v)
	}
	return b
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

func isOblique(location string) bool {
	return strings.Contains(strings.ToLower(location), "oblique")
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
