package editor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"

	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/shaping"
)

// Font is
type Font struct {
	family    string
	size      float64
	pixelSize int
	weight    gui.QFont__Weight
	stretch   int
	features  *([]shaping.FontFeature)

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

	font.rawfont = newRawFont(genFontFaceAsync, family, pixelSize, weight)
	if font.rawfont == nil {
		fmt.Println("rawfont is nil")
		return nil
	}

	qfont := gui.NewQFont()

	// Associate related font features
	font.features = setFontFeatures(family)

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

func setFontFeatures(family string) *([]shaping.FontFeature) {
	features := []shaping.FontFeature{}
	for key, fts := range editor.config.Editor.FontFeatures {
		if normalizeFamily(key) == normalizeFamily(family) {
			for _, ftag := range fts {
				if len(ftag) == 0 {
					continue
				}
				var v uint32 = 0
				if string(ftag[0]) == "+" {
					v = 1
				}

				features = append(
					features,
					shaping.FontFeature{
						Tag:   opentype.MustNewTag(string(ftag[1:])),
						Value: v,
					},
				)
			}
		}
	}

	return &features
}

func normalizeFamily(family string) string {
	rp := strings.NewReplacer(" ", "", "_", "", "\t", "", "\x00", "")
	return rp.Replace(strings.ToLower(family))
}

// TableEntry represents a table directory entry in the font file
type TableEntry struct {
	Tag      opentype.Tag
	CheckSum uint32
	Offset   uint32
	Length   uint32
}

// calculateChecksum calculates the checksum of the table data
func calculateChecksum(data []byte) uint32 {
	var sum uint32
	for i := 0; i < len(data); i += 4 {
		if i+3 < len(data) {
			sum += binary.BigEndian.Uint32(data[i : i+4])
		}
	}
	return sum
}

// align4Bytes ensures that the data is aligned to a 4-byte boundary
func align4Bytes(data []byte) []byte {
	rem := len(data) % 4
	if rem != 0 {
		padding := 4 - rem
		data = append(data, make([]byte, padding)...)
	}
	return data
}

// combineTables combines tables into a valid OpenType font binary
func combineTables(loader *opentype.Loader) ([]byte, error) {
	tables := loader.Tables()
	var fontData []byte

	// Create a buffer for the table directory and entries
	var tableEntries []TableEntry
	tableDataBuffer := bytes.NewBuffer(nil)

	// Offset for the first table (after the table directory)
	offset := uint32(12 + len(tables)*16)

	// Iterate through each table, calculate checksum and prepare table directory entries
	for _, tableTag := range tables {
		// Retrieve table data
		tableData, err := loader.RawTable(tableTag)
		if err != nil {
			return nil, fmt.Errorf("failed to get table %s: %v", tableTag, err)
		}

		// Align the table data to a 4-byte boundary
		tableData = align4Bytes(tableData)

		// Calculate the checksum for the table
		checksum := calculateChecksum(tableData)

		// Append the table to the buffer
		tableDataBuffer.Write(tableData)

		// Create the table entry
		tableEntries = append(tableEntries, TableEntry{
			Tag:      tableTag,
			CheckSum: checksum,
			Offset:   offset,
			Length:   uint32(len(tableData)),
		})

		// Update the offset for the next table
		offset += uint32(len(tableData))
	}

	// Build the table directory (font header)
	numTables := len(tables)
	searchRange := 1
	entrySelector := 0
	for (1 << entrySelector) <= numTables {
		entrySelector++
	}
	entrySelector--
	searchRange = (1 << entrySelector) * 16

	// Write the SFNT version (0x00010000 for TrueType, 0x4F54544F for OpenType CFF fonts)
	sfntVersion := uint32(0x00010000)
	fontData = append(fontData, uint32ToBytes(sfntVersion)...)

	// Write the number of tables, searchRange, entrySelector, rangeShift
	fontData = append(fontData, uint16ToBytes(uint16(numTables))...)
	fontData = append(fontData, uint16ToBytes(uint16(searchRange))...)
	fontData = append(fontData, uint16ToBytes(uint16(entrySelector))...)
	fontData = append(fontData, uint16ToBytes(uint16(numTables*16-searchRange))...)

	// Write each table entry
	for _, entry := range tableEntries {
		fontData = append(fontData, uint32ToBytes(uint32(entry.Tag))...)
		fontData = append(fontData, uint32ToBytes(entry.CheckSum)...)
		fontData = append(fontData, uint32ToBytes(entry.Offset)...)
		fontData = append(fontData, uint32ToBytes(entry.Length)...)
	}

	// Append the actual table data after the table directory
	fontData = append(fontData, tableDataBuffer.Bytes()...)

	return fontData, nil
}

// Helper functions to convert integers to byte slices
func uint16ToBytes(val uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, val)
	return buf
}

func uint32ToBytes(val uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, val)
	return buf
}

func createQRawFontFromFace(loader *opentype.Loader, size int) (*gui.QRawFont, error) {
	fontData, err := combineTables(loader)
	if err != nil {
		return nil, err
	}

	qrawFont := loadQRawFont(fontData, size)

	if !qrawFont.IsValid() {
		return nil, fmt.Errorf("failed to create QRawFont from face data")
	}

	return qrawFont, nil
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

func (f *Font) italicHorizontalAdvance(str string) (width float64) {
	if str == "" {
		return
	}
	if f.rawfont.italic == nil {
		return
	}

	a := f.rawfont.italic.GlyphIndexesForString(str)
	gi := f.rawfont.italic.AdvancesForGlyphIndexes2(a)
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

func assignFontWeight[T any](list []T, i int, isValid func(T) bool) (selected T) {
	for j, item := range list {
		if j < i {
			continue
		}
		if !isValid(item) {
			continue
		}
		if !isValid(selected) {
			selected = item
			break
		}
	}
	if !isValid(selected) {
		for j := i - 1; j >= 0; j-- {
			if j < 0 {
				break
			}
			item := list[j]
			if !isValid(item) {
				continue
			}
			if !isValid(selected) {
				selected = item
				break
			}
		}
	}
	return
}

func isValidLoader(loader *opentype.Loader) bool {
	return loader != nil
}

func isValidPath(path string) bool {
	return path != ""
}

func getFontWeight[T any](weight gui.QFont__Weight, regularList, boldList []T, isValid func(T) bool) (regular, bold T) {
	var regularIndex, boldIndex int
	switch weight {
	case gui.QFont__Thin:
		regularIndex, boldIndex = 0, 0
	case gui.QFont__ExtraLight:
		regularIndex, boldIndex = 1, 1
	case gui.QFont__Light:
		regularIndex, boldIndex = 2, 2
	case gui.QFont__Normal:
		regularIndex, boldIndex = 3, 3
	case gui.QFont__Medium:
		regularIndex, boldIndex = 4, 3
	case gui.QFont__DemiBold:
		regularIndex, boldIndex = 0, 3
	case gui.QFont__Bold:
		regularIndex, boldIndex = 1, 3
	case gui.QFont__ExtraBold:
		regularIndex, boldIndex = 2, 3
	case gui.QFont__Black:
		regularIndex, boldIndex = 3, 3
	}

	regular = assignFontWeight(regularList, regularIndex, isValid)
	bold = assignFontWeight(boldList, boldIndex, isValid)
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

	return face, fileBytes
}

func readTtcFile(genFontFaceAsync bool, path string, size int, weight gui.QFont__Weight) *RawFont {
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

	lds, err := opentype.NewLoaders(bytes.NewReader(fileBytes))
	if err != nil {
		return nil
	}
	// out := make([]*font.Face, len(lds))

	var regular *opentype.Loader
	var bold *opentype.Loader
	var italic *opentype.Loader

	var thinLd *opentype.Loader
	var extraLightLd *opentype.Loader
	var lightLd *opentype.Loader
	var regularLd *opentype.Loader
	var mediumLd *opentype.Loader
	var semiBoldLd *opentype.Loader
	var boldLd *opentype.Loader
	var extraBoldLd *opentype.Loader
	var blackLd *opentype.Loader

	for _, ld := range lds {
		desc, _ := font.Describe(ld, nil)
		aspect := desc.Aspect
		fmt.Println(aspect)

		if aspect.Style == 1 && aspect.Weight == font.WeightThin {

			thinLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == font.WeightExtraLight {
			extraLightLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == font.WeightLight {
			lightLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == font.WeightNormal {
			regularLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == font.WeightMedium {
			mediumLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == font.WeightSemibold {
			semiBoldLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == font.WeightBold {
			boldLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == font.WeightExtraBold {
			extraBoldLd = ld
		}
		if aspect.Style == 1 && aspect.Weight == font.WeightBlack {
			blackLd = ld
		}

		// TODO
		if aspect.Style == 2 && aspect.Weight == font.WeightNormal {
			italic = ld
		}
	}

	ldR := []*opentype.Loader{thinLd, extraLightLd, lightLd, regularLd, mediumLd}
	ldB := []*opentype.Loader{semiBoldLd, boldLd, extraBoldLd, blackLd}

	regular, bold = getFontWeight(weight, ldR, ldB, isValidLoader)

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

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rawfont.regular, _ = createQRawFontFromFace(regular, size)
	}()

	if bold != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rawfont.bold, _ = createQRawFontFromFace(bold, size)
		}()
	}

	if italic != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rawfont.italic, _ = createQRawFontFromFace(italic, size)
		}()
	}

	if genFontFaceAsync {
		wg.Wait()

		rawfont.wg.Add(1)
		go func() {
			defer rawfont.wg.Done()
			regularFt, _ := font.NewFont(regular)
			rawfont.regularface = &font.Face{Font: regularFt}
		}()

		if bold != nil {
			rawfont.wg.Add(1)
			go func() {
				defer rawfont.wg.Done()
				boldFt, _ := font.NewFont(bold)
				rawfont.boldface = &font.Face{Font: boldFt}
			}()
		}

		if italic != nil {
			rawfont.wg.Add(1)
			go func() {
				defer rawfont.wg.Done()
				italicFt, _ := font.NewFont(italic)
				rawfont.italicface = &font.Face{Font: italicFt}
			}()
		}

		if bold == nil {
			rawfont.bold = rawfont.regular
			rawfont.boldface = rawfont.regularface
		}
	} else {
		wg.Add(1)
		go func() {
			defer wg.Done()
			regularFt, _ := font.NewFont(regular)
			rawfont.regularface = &font.Face{Font: regularFt}
		}()

		if bold != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				boldFt, _ := font.NewFont(bold)
				rawfont.boldface = &font.Face{Font: boldFt}
			}()
		}

		if italic != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				italicFt, _ := font.NewFont(italic)
				rawfont.italicface = &font.Face{Font: italicFt}
			}()
		}

		wg.Wait()
	}

	if bold == nil {
		rawfont.bold = rawfont.regular
		rawfont.boldface = rawfont.regularface
	}

	return rawfont

}

func newRawFontFromTTC(pathes []string, genFontFaceAsync bool, fontFamilyName string, size int, weight gui.QFont__Weight) *RawFont {
	var rawfont *RawFont
	for i, path := range pathes {
		fmt.Println(i)
		rawfont = readTtcFile(genFontFaceAsync, path, size, weight)
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

	regular, bold = getFontWeight(weight, filePathR, filePathB, isValidPath)

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

		if bold != "" {
			rawfont.wg.Add(1)
			go func() {
				defer rawfont.wg.Done()
				rawfont.boldface, _ = readTtfFile(bold)
			}()
		}

		if italic != "" {
			rawfont.wg.Add(1)
			go func() {
				defer rawfont.wg.Done()
				rawfont.italicface, _ = readTtfFile(italic)
			}()
		}

	} else {

		wg.Add(1)
		go func() {
			defer wg.Done()
			rawfont.regularface, _ = readTtfFile(regular)
		}()

		if bold != "" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rawfont.boldface, _ = readTtfFile(bold)
			}()
		}

		if italic != "" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rawfont.italicface, _ = readTtfFile(italic)
			}()
		}

		wg.Wait()
	}

	if bold == "" {
		rawfont.bold = rawfont.regular
		rawfont.boldface = rawfont.regularface
	}

	return rawfont
}

func isTtcFile(path string) bool {
	return strings.HasSuffix(path, ".ttc")
}

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
