package editor

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/akiyosi/goneovim/util"
	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
	"github.com/akiyosi/qt/widgets"
	"github.com/bluele/gcache"
	"github.com/neovim/go-client/nvim"
)

const (
	EXTWINBORDERSIZE = 5
	EXTWINMARGINSIZE = 10
)

type gridId = int

// Highlight is
type Highlight struct {
	special    *RGBA
	foreground *RGBA
	background *RGBA
	kind       string
	uiName     string
	hlName     string
	// altfont       string
	blend         int
	id            int
	reverse       bool
	bold          bool
	underline     bool
	undercurl     bool
	italic        bool
	strikethrough bool
	underdouble   bool
	underdotted   bool
	underdashed   bool
}

// HlText is used in screen cache
type HlKey struct {
	fg     RGBA
	italic bool
	bold   bool
}

// HlText is used in screen cache
type HlTextKey struct {
	fg     RGBA
	text   string
	italic bool
	bold   bool
}

// HlDecorationKey is used in screen cache
type HlDecorationKey struct {
	fg            *RGBA
	bg            *RGBA
	sp            *RGBA
	underline     bool
	undercurl     bool
	strikethrough bool
	underdouble   bool
	underdotted   bool
	underdashed   bool
}

type HlBgKey struct {
	bg     *RGBA
	length int
}

type BrushKey struct {
	pattern     core.Qt__BrushStyle
	color       *RGBA
	transparent int
}

// PFKey is key for proportional fonts
type PFKey struct {
	char string
	italic bool
	bold bool
}

// Cell is
type Cell struct {
	highlight   *Highlight
	char        string
	normalWidth bool
	covered     bool
	scaled      bool
}

type IntInt [2]int

// ExternalWin is
type ExternalWin struct {
	widgets.QDialog
}

type inputMouseEvent struct {
	button string
	action string
	mod    string
	grid   gridId
	row    int
	col    int
	event  *gui.QMouseEvent
}

type zindex struct {
	value int
	order int

	nearestLowerZOrderWindow *Window
}

// Window is
type Window struct {
	cache Cache
	widgets.QWidget
	smoothScrollAnimation  *core.QPropertyAnimation
	snapshot               *gui.QPixmap
	imagePainter           *gui.QPainter
	font                   *Font
	fallbackfonts          []*Font
	localWindows           *[4]localWindow
	extwin                 *ExternalWin
	background             *RGBA
	s                      *Screen
	anchorwin              *Window
	anchor                 string
	anchorGrid             int
	anchorCol              int
	anchorRow              int
	ft                     string
	lenOldContent          []int
	lenContent             []int
	lenLine                []int
	scrollRegion           []int
	contentMaskOld         [][]bool
	contentMask            [][]bool
	content                [][]*Cell
	extwinAutoLayoutPosY   []int
	extwinAutoLayoutPosX   []int
	charsScaledLineHeight  []string
	scrollViewport         [6]int
	queueRedrawArea        [4]int
	extwinRelativePos      [2]int
	pos                    [2]int
	scrollPixels           [2]int
	viewportMargins        [4]int
	wb                     int
	height                 int
	grid                   gridId
	width                  float64
	_                      float64 `property:"scrollDiff"`
	lastMouseEvent         *inputMouseEvent
	cols                   int
	maxLenContent          int
	ts                     int
	devicePixelRatio       float64
	scrollPixels2          int
	scrollPixels3          int
	id                     nvim.Window
	scrollDelta            float64
	rows                   int
	zindex                 *zindex
	lastScrollphase        core.Qt__ScrollPhase
	updateMutex            sync.RWMutex
	paintMutex             sync.RWMutex
	propMutex              sync.RWMutex
	redrawMutex            sync.Mutex
	extwinConnectResizable bool
	extwinResized          bool
	extwinManualResized    bool
	doErase                bool
	isPopupmenu            bool
	isExternal             bool
	isFloatWin             bool
	isMsgGrid              bool
	isGridDirty            bool
	xPixelsIndexes         [][]float64 // Proportional fonts
	endGutterIdx           int         // Proportional fonts
}

type localWindow struct {
	grid        gridId
	isResized   bool
	localWidth  float64
	localHeight int
}

func purgeQimage(key, value interface{}) {
	image := value.(*gui.QImage)
	image.DestroyQImage()
}

func purgeQBrush(key, value interface{}) {
	brush := value.(*gui.QBrush)
	brush.DestroyQBrush()
}

func newCache() Cache {
	g := gcache.New(editor.config.Editor.CacheSize).LRU().
		EvictedFunc(purgeQimage).
		PurgeVisitorFunc(purgeQimage).
		Build()
	return *(*Cache)(unsafe.Pointer(&g))
}

func newBrushCache() Cache {
	g := gcache.New(64).LRU().
		EvictedFunc(purgeQBrush).
		PurgeVisitorFunc(purgeQBrush).
		Build()
	return *(*Cache)(unsafe.Pointer(&g))
}

func (c *Cache) set(key, value interface{}) error {
	return c.Set(key, value)
}

func (c *Cache) get(key interface{}) (interface{}, error) {
	return c.Get(key)
}

func (c *Cache) purge() {
	c.Purge()
}

func (w *Window) dropScreenSnapshot() {
	if w.snapshot == nil {
		return
	}
	w.paintMutex.Lock()
	w.snapshot.DestroyQPixmap()
	w.snapshot = nil
	w.paintMutex.Unlock()
}

func (w *Window) grabScreenSnapshot() {
	if editor.isKeyAutoRepeating {
		return
	}
	if w.grid == 1 || w.isMsgGrid {
		return
	}

	snapshot := w.grabScreen()
	w.paintMutex.Lock()
	w.snapshot.DestroyQPixmap()
	w.snapshot = snapshot
	w.paintMutex.Unlock()
}

func (w *Window) grabScreen() *gui.QPixmap {
	var rect *core.QRect
	fullRect := w.Rect()
	font := w.getFont()

	editor.putLog("grab screen snapshot:: grid:", w.grid, "rect:", fullRect.Width(), fullRect.Height())

	rect = core.NewQRect4(
		fullRect.X()+w.viewportMargins[2]*int(font.cellwidth),
		fullRect.Y()+(w.viewportMargins[0]*font.lineHeight),
		fullRect.Width()-w.viewportMargins[2]*int(font.cellwidth)-w.viewportMargins[3]*int(font.cellwidth),
		fullRect.Height()-(w.viewportMargins[0]*font.lineHeight)-(w.viewportMargins[1]*font.lineHeight),
	)
	return w.Grab(rect)
}

func (w *Window) paint(event *gui.QPaintEvent) {
	editor.putLog("paint start")

	w.paintMutex.Lock()

	p := gui.NewQPainter2(w)

	// clip rect
	rect := event.Rect()
	// p.SetClipRect2(rect, core.Qt__ReplaceClip)

	// Erase the snapshot used in the animation scroll
	if w.doErase {
		p.EraseRect3(w.Rect())
		p.DestroyQPainter()
		w.paintMutex.Unlock()
		return
	}

	// Set RenderHint
	p.SetRenderHint(gui.QPainter__SmoothPixmapTransform, true)

	// Set font
	font := w.getFont()

	// Set devicePixelRatio if it is not set
	devicePixelRatio := float64(p.PaintEngine().PaintDevice().DevicePixelRatio())
	if w.devicePixelRatio != devicePixelRatio {
		if w.devicePixelRatio != 0 {
			w.s.purgeTextCacheForWins()
		}
		w.devicePixelRatio = devicePixelRatio
	}

	col := int(math.Trunc(float64(rect.Left()) / font.cellwidth))
	row := int(math.Trunc(float64(rect.Top()) / float64(font.lineHeight)))

	cols := int(math.Ceil(float64(rect.Width()) / font.cellwidth))
	if rect.Width()%int(math.Trunc(font.cellwidth)) > 0 || rect.Left()%int(math.Trunc(font.cellwidth)) > 0 {
		cols++
	}

	rows := int(math.Ceil(float64(rect.Height()) / float64(font.lineHeight)))
	if rect.Height()%font.lineHeight > 0 || rect.Top()%font.lineHeight > 0 {
		rows++
	}

	var verScrollPixels int
	if w.lastScrollphase != core.Qt__NoScrollPhase {
		verScrollPixels = w.scrollPixels2
	}
	if editor.config.Editor.LineToScroll == 1 {
		verScrollPixels += w.scrollPixels[1]
	}

	// draw default background color if window is float window or msg grid
	isDrawDefaultBg := false

	if editor.config.Editor.EnableBackgroundBlur ||
		editor.config.Editor.Transparent < 1.0 {
		if !w.isExternal {
			if w.isFloatWin {
				isDrawDefaultBg = true
			}
		}
	} else {
		if w.isMsgGrid && editor.config.Message.Transparent < 1.0 {
			isDrawDefaultBg = true
		} else if w.isPopupmenu && w.s.ws.pb > 0 {
			isDrawDefaultBg = true
		} else if w.isFloatWin && w.wb > 0 {
			isDrawDefaultBg = true
		}
	}

	// In transparent mode and float windows, there is no need to automatically draw the
	//  background color of the entire grid, so the background color is not automatically drawn.
	if isDrawDefaultBg {
		w.SetAutoFillBackground(false)
	}

	if font.proportional {
		// Update the pixel x-position for each cell
		w.refreshLinesPixels(row, row+rows)
		var i int
		// Finding `col` and `cols` for proportional requires to find the lowest
		// possible value for `col` and the highest possible value for `cols`.
		left := float64(rect.Left())
		i = w.cols
		for y := row; y < rows; y++ {
			for ; i > 0; i-- {
				if w.xPixelsIndexes[y][i] < left {
					if i < col {
						col = i
					}
					break
				}
			}
		}
		width := float64(rect.Width())
		i = 0
		for y := row; y < row+rows; y++ {
			for ; i < w.cols; i++ {
				if w.xPixelsIndexes[y][i] > width {
					if i > cols {
						cols = i
					}
					break
				}
			}
		}
	}

	// -------------
	// Draw contents
	// -------------

	if verScrollPixels <= 0 {
		for y := row + rows; y >= row; y-- {
			if y < w.viewportMargins[0] {
				continue
			}
			if y > w.rows-w.viewportMargins[1]-1 {
				continue
			}
			w.drawBackground(p, y, col, cols, isDrawDefaultBg)
			w.drawForeground(p, y, col, cols)
		}
	} else {
		for y := row; y <= row+rows; y++ {
			if y < w.viewportMargins[0] {
				continue
			}
			if y > w.rows-w.viewportMargins[1]-1 {
				continue
			}
			w.drawBackground(p, y, col, cols, isDrawDefaultBg)
			w.drawForeground(p, y, col, cols)
		}
	}

	// Draw scroll snapshot
	w.drawScrollSnapshot(p)

	// // Draw content outside the viewportMargin in the y-axis direction

	for y := row; y <= row+rows; y++ {
		if y >= w.viewportMargins[0] {
			continue
		}
		w.drawBackground(p, y, col, cols, isDrawDefaultBg)
		w.drawForeground(p, y, col, cols)
	}
	for y := row + rows; y >= row; y-- {
		if y <= w.rows-w.viewportMargins[1]-1 {
			continue
		}
		w.drawBackground(p, y, col, cols, isDrawDefaultBg)
		w.drawForeground(p, y, col, cols)
	}

	// TODO: We should use msgSepChar to separate message window area
	// // If Window is Message Area, draw separator
	// if w.isMsgGrid {
	// 	w.drawMsgSeparator(p)
	// }

	// Draw indent guide
	if editor.config.Editor.IndentGuide {
		w.drawIndentguide(p, row, rows)
	}

	// Draw float window border
	if editor.config.Editor.DrawBorderForFloatWindow {
		w.drawFloatWindowBorder(p)
	}

	// Draw vim window separator
	if editor.config.Editor.DrawWindowSeparator {
		w.drawWindowSeparators(p, row, col, rows, cols)
	}

	// Minimap drawing process. It is not involved in the normal drawing of the window at all.
	if w.s.name == "minimap" {
		if w.s.ws.minimap != nil {
			if w.s.ws.minimap.visible && w.s.ws.minimap.widget.IsVisible() {
				w.s.ws.minimap.updateCurrentRegion(p)
			}
		}
	}

	w.adjustSmoothScrollAmount()
	p.DestroyQPainter()
	w.paintMutex.Unlock()
}

func (w *Window) adjustSmoothScrollAmount() {
	// Reset to 0 after drawing is complete.
	// This is to suppress flickering in smooth scroll
	font := w.getFont()
	horizontalScrollAmount := font.cellwidth
	verticalScrollAmount := float64(font.lineHeight)

	dx := math.Abs(float64(w.scrollPixels[0]))
	dy := math.Abs(float64(w.scrollPixels[1]))
	if dx >= horizontalScrollAmount {
		w.scrollPixels[0] = 0
	}
	if dy >= verticalScrollAmount {
		w.scrollPixels[1] = 0
	}

	if w.lastScrollphase == core.Qt__NoScrollPhase {
		w.lastScrollphase = core.Qt__ScrollEnd
	}
}

func (w *Window) drawScrollSnapshot(p *gui.QPainter) {
	if !editor.config.Editor.SmoothScroll {
		return
	}
	if w.s.name == "minimap" {
		return
	}
	if w.snapshot == nil {
		return
	}
	if editor.isKeyAutoRepeating {
		return
	}
	if w.scrollPixels2 == 0 {
		return
	}

	font := w.getFont()
	height := math.Abs(w.scrollDelta) * float64(font.lineHeight)

	var snapshotPosX, snapshotPosY float64
	snapshotPosX = float64(w.viewportMargins[2]) * font.cellwidth
	if w.scrollPixels2 > 0 {
		snapshotPosY = float64(w.scrollPixels2) - height
	} else if w.scrollPixels2 < 0 {
		snapshotPosY = (float64(w.snapshot.Height()) / w.devicePixelRatio) + float64(w.scrollPixels2)
	}
	snapshotPosY += float64(w.viewportMargins[0] * font.lineHeight)

	var drawPos *core.QPointF
	var sourceRect *core.QRectF
	if w.scrollPixels2 > 0 {
		drawPos = core.NewQPointF3(
			snapshotPosX,
			snapshotPosY,
		)
		sourceRect = core.NewQRectF4(
			0,
			0,
			float64(w.snapshot.Width()),
			math.Abs(height)*w.devicePixelRatio,
		)
	} else if w.scrollPixels2 < 0 {
		drawPos = core.NewQPointF3(
			snapshotPosX,
			snapshotPosY,
		)
		sourceRect = core.NewQRectF4(
			0,
			(float64(w.snapshot.Height())/w.devicePixelRatio-math.Abs(height))*w.devicePixelRatio,
			float64(w.snapshot.Width()),
			math.Abs(height)*w.devicePixelRatio,
		)
	}

	if w.scrollPixels2 != 0 {
		p.DrawPixmap5(
			drawPos,
			w.snapshot,
			sourceRect,
		)
	}
}

func (w *Window) getFont() *Font {
	if w.font == nil {
		return w.s.font
	}

	return w.font
}

func (w *Window) getFallbackFonts() []*Font {
	if w.font == nil {
		return w.s.fallbackfonts
	}

	return w.fallbackfonts
}

func (w *Window) drawIndentguide(p *gui.QPainter, row, rows int) {
	if w == nil {
		return
	}
	if w.grid == 1 || w.isMsgGrid {
		return
	}
	if w.s.name == "minimap" {
		return
	}
	if w.isMsgGrid {
		return
	}
	if w.ft == "" {
		return
	}
	for _, v := range editor.config.Editor.IndentGuideIgnoreFtList {
		if v == w.ft {
			return
		}
	}
	if !w.isShown() {
		return
	}
	if w.ts == 0 {
		return
	}

	ts := w.ts

	headspaceOfRows := make(map[int]int)
	for y := row; y < rows; y++ {
		if y+1 >= len(w.content) {
			break
		}
		l, _ := w.countHeadSpaceOfLine(y)
		headspaceOfRows[y] = l
	}

	drawIndents := make(map[IntInt]bool)
	for y := row; y < rows; y++ {
		if y+1 >= len(w.content) {
			break
		}
		// nextline := w.content[y+1]
		line := w.content[y]
		res := 0
		for x := 0; x < w.maxLenContent; x++ {
			if x+1 >= len(line) {
				break
			}
			if line[x+1] == nil {
				continue
			}
			c := line[x]
			if c == nil {
				continue
			}
			if c.highlight.isSignColumn() {
				res++
			}
			if c.char != " " && !c.highlight.isSignColumn() {
				break
			}
			// yylen, _ := w.countHeadSpaceOfLine(y)
			yylen := headspaceOfRows[y]
			if x > res && (x+1-res)%ts == 0 {
				ylen := x + 1

				if ylen > yylen {
					break
				}

				doPaintIndent := false
				for mm := y; mm < len(w.content); mm++ {
					if drawIndents[[2]int{x + 1, mm}] {
						continue
					}

					// mmlen, _ := w.countHeadSpaceOfLine(mm)
					mmlen := headspaceOfRows[mm]

					if mmlen == ylen {
						break
					}

					if mmlen > ylen && w.lenLine[mm] > res {
						doPaintIndent = true
					}

					if mmlen == w.cols && !doPaintIndent {
						for nn := mm + 1; nn < len(w.content); nn++ {
							// nnlen, _ := w.countHeadSpaceOfLine(nn)
							nnlen := headspaceOfRows[nn]
							if nnlen == ylen {
								break
							}

							if nnlen < ylen {
								break
							}

							if nnlen > ylen && w.lenLine[nn] > res {
								doPaintIndent = true
							}
						}
					}

					if mmlen < ylen {
						doBreak := true
						// If the line to draw an indent-guide has a wrapped line
						// in the next line, do not skip drawing
						// TODO: We do not detect the wrapped line when `:set nonu` setting.
						if mm+1 < len(w.content) {
							// lllen, _ := w.countHeadSpaceOfLine(mm+1)
							lllen := headspaceOfRows[mm+1]
							if mm >= 0 {
								if lllen > ylen {
									for xx := 0; xx < w.lenLine[mm]; xx++ {
										if xx >= len(w.content[mm]) {
											continue
										}
										if w.content[mm][xx] == nil {
											continue
										}
										if w.content[mm][xx].highlight.hlName == "LineNr" {
											if w.content[mm][xx].char == " " {
												doBreak = false
											} else if w.content[mm][xx].char != " " {
												doBreak = true
												break
											}
										}
									}
								}
							}
						}
						if doBreak {
							break
						}
					}

					if w.content[mm][x+1] == nil {
						break
					}
					if w.content[mm][x+1].char != " " {
						break
					}
					if !doPaintIndent {
						break
					}
					if !drawIndents[[2]int{x + 1, mm}] {
						drawIndents[[2]int{x + 1, mm}] = true
					}
				}
			}
		}
	}

	// detect current block
	currentBlock := make(map[IntInt]bool)
	for x := w.s.cursor[1]; x >= 0; x-- {
		if drawIndents[[2]int{x + 1, w.s.cursor[0]}] {
			for y := w.s.cursor[0]; y >= 0; y-- {
				if drawIndents[[2]int{x + 1, y}] {
					currentBlock[[2]int{x + 1, y}] = true
				}
				if !drawIndents[[2]int{x + 1, y}] {
					break
				}
			}
			for y := w.s.cursor[0]; y < len(w.content); y++ {
				if drawIndents[[2]int{x + 1, y}] {
					currentBlock[[2]int{x + 1, y}] = true
				}
				if !drawIndents[[2]int{x + 1, y}] {
					break
				}
			}

			break
		}
	}

	// draw indent guide
	for y := row; y < len(w.content); y++ {
		for x := 0; x < w.maxLenContent; x++ {
			if !drawIndents[[2]int{x + 1, y}] {
				continue
			}
			if currentBlock[[2]int{x + 1, y}] {
				w.drawIndentline(p, x+1, y, true)
			} else {
				w.drawIndentline(p, x+1, y, false)
			}
		}
	}
}

func (w *Window) drawIndentline(p *gui.QPainter, x int, y int, b bool) {
	font := w.getFont()

	// Set smooth scroll offset
	var horScrollPixels, verScrollPixels int
	if w.lastScrollphase != core.Qt__NoScrollPhase {
		verScrollPixels = w.scrollPixels2
	}
	if editor.config.Editor.LineToScroll == 1 {
		verScrollPixels += w.scrollPixels[1]
	}

	if w.s.ws.mouseScroll != "" {
		horScrollPixels += w.scrollPixels[0]
	}

	X := float64(x)*font.cellwidth + float64(horScrollPixels)
	Y := float64(y*font.lineHeight) + float64(verScrollPixels)
	var color *RGBA = editor.colors.indentGuide
	var lineWeight float64 = 1
	if b {
		color = warpColor(editor.colors.indentGuide, -40)
		lineWeight = 1.5
	}
	p.FillRect4(
		core.NewQRectF4(
			X,
			Y,
			lineWeight,
			float64(font.lineHeight),
		),
		color.QColor(),
	)

	if w.lenContent[y] < x {
		w.lenContent[y] = x
	}
}

func (w *Window) drawMsgSeparator(p *gui.QPainter) {
	highNo, ok := w.s.highlightGroup["MsgSeparator"]
	if !ok {
		return
	}
	hl, ok := w.s.hlAttrDef[highNo]
	if !ok {
		return
	}
	if hl == nil {
		return
	}
	color := hl.fg()
	p.FillRect4(
		core.NewQRectF4(
			0,
			0,
			float64(w.Width()),
			1,
		),
		gui.NewQColor3(
			color.R,
			color.G,
			color.B,
			200),
	)
}

func (w *Window) drawFloatWindowBorder(p *gui.QPainter) {
	if !w.isFloatWin {
		return
	}
	if !w.isExternal {
		return
	}
	var color *RGBA
	highNo, ok := w.s.highlightGroup["GoneovimFloatWindowBorder"]
	if !ok {
		color = editor.colors.fg
	} else {
		hl, ok := w.s.hlAttrDef[highNo]
		if !ok || hl == nil {
			color = editor.colors.fg
		} else {
			color = hl.fg()
		}
	}

	width := float64(w.Width())
	height := float64(w.Height())

	left := core.NewQRectF4(0, 0, 1, height)
	top := core.NewQRectF4(0, 0, width, 1)
	right := core.NewQRectF4(width-1, 0, 1, height)
	bottom := core.NewQRectF4(0, height-1, width, 1)

	p.FillRect4(
		left,
		gui.NewQColor3(
			color.R,
			color.G,
			color.B,
			128),
	)
	p.FillRect4(
		top,
		gui.NewQColor3(
			color.R,
			color.G,
			color.B,
			128),
	)
	p.FillRect4(
		right,
		gui.NewQColor3(
			color.R,
			color.G,
			color.B,
			128),
	)
	p.FillRect4(
		bottom,
		gui.NewQColor3(
			color.R,
			color.G,
			color.B,
			128),
	)
}

func (w *Window) drawWindowSeparators(p *gui.QPainter, row, col, rows, cols int) {
	if w == nil {
		return
	}
	if w.grid != 1 {
		return
	}

	gwin, ok := w.s.getWindow(1)
	if !ok {
		return
	}
	gwinrows := gwin.rows

	w.s.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)

		if win == nil {
			return true
		}
		if !win.isShown() {
			return true
		}
		if win.isFloatWin {
			return true
		}
		if win.isMsgGrid {
			return true
		}
		if win.pos[0]+win.cols < row && (win.pos[1]+win.rows+1) < col {
			return true
		}
		if win.pos[0] > (row+rows) && (win.pos[1]+win.rows) > (col+cols) {
			return true
		}
		win.drawWindowSeparator(p, gwinrows)

		return true
	})

}

func (w *Window) drawWindowSeparator(p *gui.QPainter, gwinrows int) {
	font := w.getFont()

	// window position is based on cols, rows of global font setting
	x := int(float64(w.pos[0]) * w.s.font.cellwidth)
	y := w.pos[1] * w.s.font.lineHeight
	color := editor.colors.windowSeparator
	width := int(float64(w.cols) * font.cellwidth)
	winHeight := int((float64(w.rows) + 0.92) * float64(font.lineHeight))

	// Vim uses the showtabline option to change the display state of the tabline
	// based on the number of tabs. We need to look at these states to adjust
	// the length and display position of the window separator
	tablineNum := 0
	numOfTabs := w.s.ws.getNumOfTabs()
	if numOfTabs > 1 {
		tablineNum = 1
	}
	isDrawTabline := editor.config.Tabline.Visible && editor.config.Editor.ExtTabline
	if w.s.ws.showtabline == 2 && isDrawTabline && numOfTabs == 1 {
		tablineNum = -1
	}
	shift := font.lineHeight / 2
	if w.rows+w.s.ws.showtabline+tablineNum+1 == gwinrows {
		winHeight = w.rows * font.lineHeight
		shift = 0
	} else {
		if w.pos[1] == tablineNum {
			winHeight = w.rows*font.lineHeight + int(float64(font.lineHeight)/2.0)
			shift = 0
		}
		if w.pos[1]+w.rows == gwinrows-2 {
			winHeight = w.rows*font.lineHeight + int(float64(font.lineHeight)/2.0)
		}
	}

	// Vertical
	if y+font.lineHeight+1 < w.s.widget.Height() {
		p.FillRect5(
			int(float64(x+width)+font.cellwidth/2),
			y-shift,
			2,
			winHeight,
			color.QColor(),
		)
	}
	// vertical gradient
	if editor.config.Editor.WindowSeparatorGradient {
		gradient := gui.NewQLinearGradient3(
			float64(x+width)+font.cellwidth/2,
			0,
			float64(x+width)+font.cellwidth/2-6,
			0,
		)
		gradient.SetColorAt(0, gui.NewQColor3(color.R, color.G, color.B, 125))
		gradient.SetColorAt(1, gui.NewQColor3(color.R, color.G, color.B, 0))
		brush := gui.NewQBrush10(gradient)
		p.FillRect2(
			int(float64(x+width)+font.cellwidth/2)-6,
			y-shift,
			6,
			winHeight,
			brush,
		)
	}

	bottomBorderPos := w.pos[1]*w.s.font.lineHeight + w.Rect().Bottom()
	isSkipDrawBottomBorder := bottomBorderPos > w.s.bottomWindowPos()-w.s.font.lineHeight && bottomBorderPos < w.s.bottomWindowPos()+w.s.font.lineHeight
	if isSkipDrawBottomBorder {
		return
	}

	// Horizontal
	height := w.rows * font.lineHeight
	y2 := y + height - 1 + font.lineHeight/2

	p.FillRect5(
		int(float64(x)-font.cellwidth/2),
		y2,
		int((float64(w.cols)+0.92)*font.cellwidth),
		2,
		color.QColor(),
	)
	// horizontal gradient
	if editor.config.Editor.WindowSeparatorGradient {
		hgradient := gui.NewQLinearGradient3(
			0,
			float64(y2),
			0,
			float64(y2)-6,
		)
		hgradient.SetColorAt(0, gui.NewQColor3(color.R, color.G, color.B, 125))
		hgradient.SetColorAt(1, gui.NewQColor3(color.R, color.G, color.B, 0))
		hbrush := gui.NewQBrush10(hgradient)
		p.FillRect2(
			int(float64(x)-font.cellwidth/2),
			y2-6,
			int((float64(w.cols)+0.92)*font.cellwidth),
			6,
			hbrush,
		)
	}
}

func (w *Window) wheelEvent(event *gui.QWheelEvent) {
	if !w.s.ws.isMouseEnabled {
		return
	}

	w.dropScreenSnapshot()

	var v, h, vert, horiz int
	var action string

	editor.putLog("start wheel event")

	font := w.getFont()

	mouseScroll := w.s.ws.mouseScroll
	if mouseScroll == "" {
		mouseScroll = "ver:2,hor:1"
	}

	pixels := event.PixelDelta()
	if pixels != nil {
		v = pixels.Y()
		h = pixels.X()
	}

	// faster move in darwin
	if runtime.GOOS == "darwin" {
		v = v * 2
		h = h * 2
	}

	phase := event.Phase()
	if phase == core.Qt__ScrollEnd {
		w.scrollPixels3 = 0
	}
	w.lastScrollphase = phase

	emitScrollEnd := (w.lastScrollphase == core.Qt__ScrollEnd)

	// handle MouseScrollingUnit configuration item
	// if value is "line":
	doAngleScroll := false

	if editor.config.Editor.MouseScrollingUnit == "line" {
		doAngleScroll = true
	}
	// if value is "smart":
	if editor.config.Editor.MouseScrollingUnit == "smart" {
		if w.s.ws.mouseScrollTemp != "ver:1,hor:1" {
			w.applyTemporaryMousescroll("ver:1,hor:1")
		}

		if math.Abs(float64(v)) > float64(font.lineHeight*2) {
			doAngleScroll = true
			w.applyTemporaryMousescroll(w.s.ws.mouseScroll)

		} else {
			doAngleScroll = false
			if w.s.ws.mouseScrollTemp != "ver:1,hor:1" {
				w.applyTemporaryMousescroll("ver:1,hor:1")
			}
		}
		if emitScrollEnd {
			w.applyTemporaryMousescroll(w.s.ws.mouseScroll)
		}
	}
	// if value is "pixel":
	if editor.config.Editor.MouseScrollingUnit == "pixel" {
		w.applyTemporaryMousescroll("ver:1,hor:1")
		if emitScrollEnd {
			w.applyTemporaryMousescroll(w.s.ws.mouseScroll)
		}
	}

	if editor.config.Editor.DisableHorizontalScroll {
		h = 0
	}

	if (v == 0 || h == 0) && emitScrollEnd && !doAngleScroll && !w.s.ws.isTerminalMode {
		vert, horiz = w.smoothUpdate(v, h, emitScrollEnd)
	} else if (v != 0 || h != 0) && phase != core.Qt__NoScrollPhase && !doAngleScroll && !w.s.ws.isTerminalMode {
		// If Scrolling has ended, reset the displacement of the line
		vert, horiz = w.smoothUpdate(v, h, emitScrollEnd)
	} else {
		angles := event.AngleDelta()
		vert = angles.Y()
		horiz = angles.X()
		// Scroll per 1 line
		if vert < 0 {
			vert = -1
		} else if vert > 0 {
			vert = 1
		}
		if horiz < 0 {
			horiz = -1
		} else if horiz > 0 {
			horiz = 1
		}
	}

	if vert == 0 && horiz == 0 && w.s.ws.mouseScroll == "" {
		return
	}
	if vert == 0 && horiz == 0 {
		return
	}

	if editor.config.Editor.ReversingScrollDirection {
		if vert < 0 {
			action = "up"
		} else if vert > 0 {
			action = "down"
		}
	} else {
		if vert > 0 {
			action = "up"
		} else if vert < 0 {
			action = "down"
		}
	}

	mod := editor.modPrefix(event.Modifiers())
	col := int(float64(event.X()) / font.cellwidth)
	row := int(float64(event.Y()) / float64(font.lineHeight))

	if w.s.ws.isMappingScrollKey || w.s.ws.mouseScroll != "" {
		if vert != 0 {
			w.s.ws.nvim.InputMouse("wheel", action, mod, w.grid, row, col)
		}
	} else {
		verAmount := editor.config.Editor.LineToScroll * int(math.Abs(float64(vert)))
		scrollUpKey := "<C-y>"
		scrollDownKey := "<C-e>"
		var scrollKey string
		if editor.config.Editor.ReversingScrollDirection {
			if vert > 0 {
				scrollKey = fmt.Sprintf("%v%s", verAmount, scrollDownKey)
			} else if vert < 0 {
				scrollKey = fmt.Sprintf("%v%s", verAmount, scrollUpKey)
			}
		} else {
			if vert > 0 {
				scrollKey = fmt.Sprintf("%v%s", verAmount, scrollUpKey)
			} else if vert < 0 {
				scrollKey = fmt.Sprintf("%v%s", verAmount, scrollDownKey)
			}
		}

		go w.s.ws.nvim.Input(scrollKey)
	}

	if editor.config.Editor.DisableHorizontalScroll {
		return
	}

	if horiz > 0 {
		action = "left"
	} else if horiz < 0 {
		action = "right"
	} else {
		return
	}

	if horiz != 0 {
		go w.s.ws.nvim.InputMouse("wheel", action, mod, w.grid, row, col)
	}

	event.Accept()
}

func (w *Window) applyTemporaryMousescroll(ms string) {
	cmd := "set mousescroll=" + ms
	o := make(map[string]interface{})
	o["output"] = true
	outCh := make(chan map[string]interface{}, 5)
	go func() {
		out, _ := w.s.ws.nvim.Exec(cmd, o)
		outCh <- out
	}()
	select {
	case <-outCh:
		w.s.ws.mouseScrollTemp = ms
	case <-time.After(NVIMCALLTIMEOUT * time.Millisecond):
	}
}

func (w *Window) isSameAsCursorGrid() bool {
	return w.grid == w.s.ws.cursor.gridid
}

// screen smooth update with touchpad
func (w *Window) smoothUpdate(v, h int, emitScrollEnd bool) (int, int) {
	var vert, horiz int
	font := w.getFont()

	if emitScrollEnd {
		w.scrollPixels[0] = 0
		w.scrollPixels[1] = 0
		w.queueRedrawAll()
		w.refreshUpdateArea(1)
		w.update()
		w.s.ws.cursor.update()
		return 0, 0
	}

	if h < 0 && w.scrollPixels[0] > 0 {
		w.scrollPixels[0] = 0
		if !emitScrollEnd {
			w.scrollPixels3 = 0
		}
	}
	if v < 0 && w.scrollPixels[1] > 0 {
		w.scrollPixels[1] = 0
	}

	dx := math.Abs(float64(w.scrollPixels[0]))
	dy := math.Abs(float64(w.scrollPixels[1]))
	horizontalScrollAmount := font.cellwidth
	verticalScrollAmount := float64(font.lineHeight)

	if math.Abs(float64(w.scrollPixels3)) < 20 {
		if math.Abs(float64(h)) > math.Abs(float64(v)) {
			w.scrollPixels3 += h
		}
		h = 0
	}

	if dx < horizontalScrollAmount {
		w.scrollPixels[0] += h
	}
	if dy < verticalScrollAmount {
		w.scrollPixels[1] += v
	}

	dx = math.Abs(float64(w.scrollPixels[0]))
	dy = math.Abs(float64(w.scrollPixels[1]))

	if dx >= horizontalScrollAmount {
		horiz = int(float64(w.scrollPixels[0]) / horizontalScrollAmount)
	}
	if dy >= verticalScrollAmount {
		vert = int(float64(w.scrollPixels[1]) / verticalScrollAmount)
		// NOTE: Reset to 0 after paint event is complete.
		//       This is to suppress flickering.
	}

	// w.update()
	// w.s.ws.cursor.update()
	if !(dx >= horizontalScrollAmount || dy > verticalScrollAmount) {
		w.update()
		w.s.ws.cursor.update()
	}

	return vert, horiz
}

// smoothscroll makes Neovim's scroll command behavior smooth and animated.
func (win *Window) smoothScroll(delta float64) {
	if !editor.config.Editor.SmoothScroll {
		return
	}

	win.initializeOrReuseSmoothScrollAnimation()

	if win.smoothScrollAnimation.State() == core.QAbstractAnimation__Running {
		win.smoothScrollAnimation.Stop()

		scrollingDelta := float64(win.scrollPixels2) / float64(win.getFont().lineHeight)
		win.scrollDelta = delta + scrollingDelta
		// win.snapshot = win.combinePixmap(win.snapshot, win.grabScreen(), scrollingDelta)
		win.smoothScrollAnimation.SetStartValue(core.NewQVariant10(win.scrollDelta))
		win.smoothScrollAnimation.SetEndValue(core.NewQVariant10(0))

		win.smoothScrollAnimation.Start(core.QAbstractAnimation__DeletionPolicy(core.QAbstractAnimation__KeepWhenStopped))
	} else {

		win.scrollDelta = delta
		win.smoothScrollAnimation.SetStartValue(core.NewQVariant10(win.scrollDelta))
		win.smoothScrollAnimation.SetEndValue(core.NewQVariant10(0))

		win.smoothScrollAnimation.Start(core.QAbstractAnimation__DeletionPolicy(core.QAbstractAnimation__KeepWhenStopped))
	}

}

func (hl *Highlight) fg() *RGBA {
	var color *RGBA
	if hl.reverse {
		color = hl.background
		if color == nil {
			// color = w.s.ws.background
			color = editor.colors.bg
		}
	} else {
		color = hl.foreground
		if color == nil {
			// color = w.s.ws.foreground
			color = editor.colors.fg
		}
	}

	return color
}

func (hl *Highlight) bg() *RGBA {
	var color *RGBA
	if hl.reverse {
		color = hl.foreground
		if color == nil {
			// color = w.s.ws.foreground
			color = editor.colors.fg
		}
	} else {
		color = hl.background
		if color == nil {
			// color = w.s.ws.background
			color = editor.colors.bg
		}
	}

	return color
}

func (win *Window) updateGridContent(row, colStart int, cells []interface{}) {
	if colStart < 0 {
		return
	}

	if row >= win.rows {
		return
	}

	// Suppresses flickering during smooth scrolling
	if win.scrollPixels[0] != 0 {
		win.scrollPixels[0] = 0
	}
	if win.scrollPixels[1] != 0 {
		win.scrollPixels[1] = 0
	}

	lenContent, doNotCountContent, isPartialUpdate := win.updateLine(row, colStart, cells)
	if !editor.config.Editor.IndentGuide {
		if !doNotCountContent && !isPartialUpdate {
			win.countContent(row)
		} else if doNotCountContent {
			win.lenContent[row] = lenContent
		}
	} else {
		win.countContent(row)
	}

	if !win.isShown() {
		win.show()
	}

	// Related to #364, it seems that in a UI consisting of multiple float windows,
	// there are cases where the grid in which the grid_line event is emitted
	// must be considered in the z-order of the UI.
	if win.isFloatWin && !win.isMsgGrid {
		if !editor.isExtWinNowInactivated && !editor.isWindowNowInactivated {
			if win.s.lastGridLineGrid != win.grid {
				win.zindex.order = globalOrder
				globalOrder++
				win.raise()
			}
		}
	}

	if win.grid == 1 && win.s.name == "minimap" {
		return
	}
	if win.maxLenContent < win.lenContent[row] {
		win.maxLenContent = win.lenContent[row]
	}
}

func (w *Window) updateLine(row, col int, cells []interface{}) (int, bool, bool) {
	line := w.content[row]
	maskRow := w.contentMask[row]
	colStart := col
	hlAttrDef := w.s.hlAttrDef
	linelen := len(line)
	lenScaledChars := len(w.charsScaledLineHeight)

	hl := -1
	lastSpaces := 0
	lenCells := len(cells)
	for k, arg := range cells {
		if col >= linelen {
			continue
		}
		// cell
		cell := arg.([]interface{})

		// char of cell
		char := cell[0].(string)

		// is the char is scraled?
		scaled := false
		if lenScaledChars > 0 {
			for _, charScaled := range w.charsScaledLineHeight {
				if char == charScaled {
					scaled = true
				}
			}
		}

		if len(cell) >= 2 {
			hl = util.ReflectToInt(cell[1])
		}

		repeat := 1
		if len(cell) == 3 {
			repeat = util.ReflectToInt(cell[2])

			// Count spaces in line end for culcurate content
			if k == lenCells-1 {
				if char == " " && hl == 0 {
					lastSpaces = util.ReflectToInt(cell[2])
				}
			}
		}

		for ; repeat > 0 && col < linelen; repeat-- {
			if line[col] == nil {
				line[col] = &Cell{}
				maskRow[col] = true
			}

			line[col].char = char
			line[col].normalWidth = w.isNormalWidth(char)
			line[col].scaled = scaled

			// if w.grid == 2 {
			// 	fmt.Printf(
			// 		fmt.Sprintf("'%s',", line[col].char),
			// 	)
			// }

			if hl != -1 || col == 0 {
				line[col].highlight = hlAttrDef[hl]
			} else {
				line[col].highlight = line[col-1].highlight
			}

			maskRow[col] = line[col].char != " " ||
				!line[col].highlight.bg().equals(w.background) ||
				line[col].highlight.underline ||
				line[col].highlight.undercurl ||
				line[col].highlight.strikethrough ||
				line[col].highlight.underdouble ||
				line[col].highlight.underdotted ||
				line[col].highlight.underdashed

			if !w.isPopupmenu &&
				(line[col].highlight.uiName == "Pmenu" ||
					line[col].highlight.uiName == "PmenuSel" ||
					line[col].highlight.uiName == "PmenuSbar") {
				w.isPopupmenu = true
				w.move(w.pos[0], w.pos[1], w.anchorwin)
			}

			if line[col].highlight.blend > 0 {
				if w.wb != line[col].highlight.blend {
					w.s.ws.screen.purgeTextCacheForWins()
				}
				w.wb = line[col].highlight.blend
			}

			col++
		}
	}

	w.queueRedraw(colStart, row, col-colStart+1, 1)

	lenContentRow := w.cols
	if len(w.lenContent) >= row+1 {
		lenContentRow = w.lenContent[row]
	}
	doNotCountContent1 := (col == w.cols && lastSpaces > 0)
	doNotCountContent2 := (col == lenContentRow && lastSpaces > 0)
	doNotCountContent := doNotCountContent1 || doNotCountContent2
	isPartialUpdate := col < lenContentRow && lenContentRow < w.cols
	newlenContent := 0
	if doNotCountContent1 {
		newlenContent = w.cols - lastSpaces
	} else if doNotCountContent2 {
		newlenContent = w.lenContent[row] - lastSpaces
	}

	return newlenContent, doNotCountContent, isPartialUpdate
}

func (w *Window) countContent(row int) {
	line := w.content[row]
	lenLine := w.cols - 1
	width := w.cols - 1

	var breakFlag0, breakFlag1 bool
	if !editor.config.Editor.IndentGuide {
		breakFlag0 = true
	}

	for j := w.cols - 1; j >= 0; j-- {
		cell := line[j]

		if !breakFlag0 {
			if cell == nil || cell.char == " " {
				lenLine--
			} else {
				breakFlag0 = true
			}
		}

		if !breakFlag1 {
			if cell == nil || (cell.char == " " && cell.highlight.bg().equals(w.background) &&
				!cell.highlight.underline && !cell.highlight.undercurl &&
				!cell.highlight.strikethrough && !cell.highlight.underdouble &&
				!cell.highlight.underdotted && !cell.highlight.underdashed) {
				width--
			} else {
				breakFlag1 = true
				break
			}
		}

		if breakFlag0 && breakFlag1 {
			break
		}
	}

	w.lenLine[row] = lenLine + 1
	w.lenContent[row] = width + 1
}

// func (w *Window) makeUpdateMask(row, col int, cells []interface{}) {
// 	for j, cell := range w.content[row] {
// 		if cell == nil {
// 			w.contentMask[row][j] = true
// 			continue
//
// 			// If the target cell is blank and there is no text decoration of any kind
// 		} else if cell.char == " " &&
// 			cell.highlight.bg().equals(w.background) &&
// 			!cell.highlight.underline &&
// 			!cell.highlight.undercurl &&
// 			!cell.highlight.strikethrough {
//
// 			w.contentMask[row][j] = false
//
// 		} else {
// 			w.contentMask[row][j] = true
// 		}
// 	}
// }

func (w *Window) countHeadSpaceOfLine(y int) (int, error) {
	if w == nil {
		return 0, errors.New("window is nil")
	}
	if y >= len(w.content) || w.content == nil {
		return 0, errors.New("content is nil")
	}
	line := w.content[y]
	count := 0
	for _, c := range line {
		if c == nil {
			continue
		}
		if c.char != " " && !c.highlight.isSignColumn() {
			break
		} else {
			count++
		}
	}
	return count, nil
}

func (h *Highlight) isSignColumn() bool {
	switch h.hlName {
	case "SignColumn",
		"FoldColumn",
		"LineNr",
		"CursorLineNr",
		"ALEErrorSign",
		"ALEStyleErrorSign",
		"ALEWarningSign",
		"ALEStyleWarningSign",
		"ALEInfoSign",
		"ALESignColumnWithErrors",
		"GitSignsAdd",
		"GitSignsChange",
		"GitSignsDelete",
		"LspErrorHighlight",
		"LspWarningHighlight",
		"LspInformationHighlight",
		"LspHintHighlight":
		return true
	default:
		return false

	}
}

func (w *Window) scroll(count int) {
	top := w.scrollRegion[0]
	bot := w.scrollRegion[1]
	left := w.scrollRegion[2]
	right := w.scrollRegion[3]

	// If the rectangular area to be scrolled matches
	// the entire area of the grid, we simply shift the content slice.
	if top == 0 && bot == w.rows-1 {
		c := count
		if count < 0 {
			c = c * -1
		}

		content := make([][]*Cell, c)
		contentMask := make([][]bool, c)
		lenLine := make([]int, c)
		lenContent := make([]int, c)

		for i := 0; i < c; i++ {
			content[i] = make([]*Cell, w.cols)
			contentMask[i] = make([]bool, w.cols)
		}

		if count > 0 {
			w.content = append(w.content[count:], content...)
			w.contentMask = append(w.contentMask[count:], contentMask...)
			w.lenLine = append(w.lenLine[count:], lenLine...)
			w.lenContent = append(w.lenContent[count:], lenContent...)
		}
		if count < 0 {
			// w.content = w.content[:w.rows+count]
			w.content = append(content, w.content...)
			// w.contentMask = w.contentMask[:w.rows+count]
			w.contentMask = append(contentMask, w.contentMask...)

			// w.lenLine = w.lenLine[:w.rows+count]
			w.lenLine = append(lenLine, w.lenLine...)
			// w.lenContent = w.lenContent[:w.rows+count]
			w.lenContent = append(lenContent, w.lenContent...)
		}
	} else {
		// If the rectangular area to be scrolled does not match
		// the entire area of the grid

		if count > 0 {
			for row := top; row <= bot-count; row++ {
				w.scrollContentByCount(row, left, right, bot, count)
			}
			for row := bot - count + 1; row <= bot; row++ {
				w.clearLinesWhereContentHasPassed(row, left, right)
			}
		}
		if count < 0 {
			for row := bot; row >= top-count; row-- {
				w.scrollContentByCount(row, left, right, bot, count)
			}
			for row := top - count - 1; row >= top; row-- {
				w.clearLinesWhereContentHasPassed(row, left, right)
			}
		}
	}

	// Suppresses flickering during smooth scrolling
	if w.scrollPixels[0] != 0 {
		w.scrollPixels[0] = 0
	}
	if w.scrollPixels[1] != 0 {
		w.scrollPixels[1] = 0
	}

	// w.queueRedraw(left, top, (right - left + 1), (bot - top + 1))
	w.queueRedraw(0, top, w.cols, bot-top+1)
}

// scrollContentByCount a function to shift the contents of w.content array by count.
func (w *Window) scrollContentByCount(row, left, right, bot, count int) {
	if len(w.content) <= bot {
		return
	}
	if len(w.content[row]) <= right {
		return
	}

	for col := left; col <= right; col++ {
		w.content[row][col] = w.content[row+count][col]
		w.contentMask[row][col] = w.contentMask[row+count][col]
	}

	w.lenLine[row] = w.lenLine[row+count]
	w.lenContent[row] = w.lenContent[row+count]
}

// clearLinesWhereContentHasPassed is a function to clear the source area
// after shifting the contents of w.content array by count.
func (w *Window) clearLinesWhereContentHasPassed(row, left, right int) {
	if len(w.content) <= row {
		return
	}
	if len(w.content[row]) <= right {
		return
	}
	for col := left; col <= right; col++ {
		w.content[row][col] = nil
		w.contentMask[row][col] = true
	}
}

func (w *Window) update() {
	if w == nil {
		return
	}

	w.redrawMutex.Lock()

	font := w.getFont()
	begin := w.queueRedrawArea[1]
	end := w.queueRedrawArea[3]
	extendedDrawingArea := int(font.cellwidth)

	// `|| font.proportional` is a workaround, and may be avoided when
	// the loop that compute `rects` sizes will support Proportional fonts.
	drawWithSingleRect := (w.lastScrollphase != core.Qt__ScrollEnd && (w.scrollPixels[0] != 0 || w.scrollPixels[1] != 0)) || editor.config.Editor.IndentGuide || w.s.name == "minimap" || (editor.config.Editor.SmoothScroll && w.scrollPixels2 != 0) || font.proportional
	if drawWithSingleRect {
		begin = 0
		end = w.rows
	}

	// Mitigate #389
	if runtime.GOOS == "windows" {
		begin = 0
		end = w.rows
	}

	for i := begin; i < end; i++ {

		if len(w.content) <= i {
			continue
		}

		width := w.lenContent[i]
		contentMaskI := w.contentMask[i]
		lenContentMaskI := len(contentMaskI)
		lineHeightI := i * font.lineHeight

		if width < w.lenOldContent[i] {
			width = w.lenOldContent[i]
		}
		w.lenOldContent[i] = w.lenContent[i]

		// If DrawIndentGuide is enabled
		if editor.config.Editor.IndentGuide {
			if i < w.rows-1 {
				if width < w.lenContent[i+1] {
					width = w.lenContent[i+1]
				}
			}
		}

		// If screen is minimap
		if drawWithSingleRect && w.s.name == "minimap" {
			width = w.cols
		} else if drawWithSingleRect {
			width = w.maxLenContent
		}
		width++

		// Create rectangles that require updating.
		var rects [][4]int
		isCreateRect := false

		// TODO: Cellwidth cannot be used for proportional font.
		if drawWithSingleRect {
			rect := [4]int{
				0,
				lineHeightI,
				int(math.Ceil(float64(width)*font.cellwidth)) + extendedDrawingArea,
				font.lineHeight,
			}
			rects = append(rects, rect)
		} else {
			start := 0
			for j, cm := range contentMaskI {
				mask := cm || w.contentMaskOld[i][j]
				// Starting point for creating a rectangular area
				if mask && !isCreateRect {
					start = j
					isCreateRect = true
				}
				// Judgment point for end of rectangular area creation
				if (!mask && isCreateRect) || (j >= lenContentMaskI-1 && isCreateRect) {
					// If the next rectangular area will be created with only one cell separating it, merge it.
					if j+1 <= lenContentMaskI-1 {
						if contentMaskI[j+1] {
							continue
						}
					}

					jj := j

					// If it reaches the edge of the grid
					if j >= lenContentMaskI-1 && isCreateRect {
						jj++
					}

					// create rectangular area
					// To avoid leaving drawing debris, update a slightly larger area.
					x := int(float64(start)*font.cellwidth) - 1
					if x < 0 {
						x = 0
					}
					rect := [4]int{
						x, // update a slightly larger area.
						lineHeightI,
						int(math.Ceil(float64(jj-start)*font.cellwidth)) + extendedDrawingArea, // update a slightly larger area.
						font.lineHeight,
					}
					rects = append(rects, rect)
					isCreateRect = false
				}
			}
		}

		// Request screen refresh for each rectangle region.
		for _, rect := range rects {
			w.Update2(
				rect[0],
				rect[1],
				rect[2],
				rect[3],
			)
		}

		// Update contentMaskOld
		copy(w.contentMaskOld[i], contentMaskI)
	}

	// reset redraw area
	w.queueRedrawArea[0] = w.cols
	w.queueRedrawArea[1] = w.rows
	w.queueRedrawArea[2] = 0
	w.queueRedrawArea[3] = 0

	w.redrawMutex.Unlock()
}

func (w *Window) queueRedrawAll() {
	w.redrawMutex.Lock()
	w.queueRedrawArea = [4]int{0, 0, w.cols, w.rows}
	w.redrawMutex.Unlock()
}

func (w *Window) queueRedraw(x, y, width, height int) {
	w.redrawMutex.Lock()
	if x < w.queueRedrawArea[0] {
		w.queueRedrawArea[0] = x
	}
	if y < w.queueRedrawArea[1] {
		w.queueRedrawArea[1] = y
	}
	if (x + width) > w.queueRedrawArea[2] {
		w.queueRedrawArea[2] = x + width
	}
	if (y + height) > w.queueRedrawArea[3] {
		w.queueRedrawArea[3] = y + height
	}
	w.redrawMutex.Unlock()
}

func (w *Window) drawBackground(p *gui.QPainter, y int, col int, cols int, isDrawDefaultBg bool) {
	if y >= len(w.content) {
		return
	}

	line := w.content[y]
	var bg *RGBA

	// Set smooth scroll offset
	var horScrollPixels, verScrollPixels int

	// isDrawDefaultBg := true
	// // Simply paint the color into a rectangle
	// for x := col; x <= col+cols; x++ {
	// 	if x >= len(line) {
	// 		continue
	// 	}
	// 	var highlight *Highlight
	// 	if line[x] == nil {
	// 		highlight = w.s.hlAttrDef[0]
	// 	} else {
	// 		highlight = line[x].highlight
	// 	}
	// 	if !bg.equals(w.s.ws.background) || isDrawDefaultBg {
	// 		 // Set diff pattern
	// 		 pattern, color, transparent := w.getFillpatternAndTransparent(highlight)
	// 		 // Fill background with pattern
	// 		 rectF := core.NewQRectF4(
	// 				 float64(x)*font.cellwidth,
	// 				 float64((y)*font.lineHeight),
	// 				 font.cellwidth,
	// 				 float64(font.lineHeight),
	// 		 )
	// 		 p.FillRect(
	// 				 rectF,
	// 				 gui.NewQBrush3(
	// 						 gui.NewQColor3(
	// 								 color.R,
	// 								 color.G,
	// 								 color.B,
	// 								 transparent,
	// 						 ),
	// 						 pattern,
	// 				 ),
	// 		 )
	// 	}
	// }

	if y < w.viewportMargins[0] || y > w.rows-w.viewportMargins[1]-1 {
		verScrollPixels = 0
		horScrollPixels = 0
		isDrawDefaultBg = true
	}

	// The same color combines the rectangular areas and paints at once
	var start, end int
	var lastBg *RGBA
	var lastHighlight, highlight *Highlight

	for x := col; x <= col+cols; x++ {

		if x >= len(line)+1 {
			continue
		}

		if !(y < w.viewportMargins[0] || y > w.rows-w.viewportMargins[1]-1) {
			if w.s.ws.mouseScroll != "" {
				horScrollPixels = w.scrollPixels[0]
			}
			if w.lastScrollphase != core.Qt__NoScrollPhase {
				verScrollPixels = w.scrollPixels2
			}
			if editor.config.Editor.LineToScroll == 1 {
				verScrollPixels += w.scrollPixels[1]
			}
		}

		if x < len(line) {
			if line[x] == nil {
				highlight = w.s.hlAttrDef[0]
			} else {
				highlight = line[x].highlight
			}
			if line[x] != nil {
				if line[x].covered {
					highlight = w.s.hlAttrDef[0]
				}
			}
		} else {
			highlight = w.s.hlAttrDef[0]
		}

		if highlight.isSignColumn() {
			horScrollPixels = 0
		}

		bg = highlight.bg()

		bounds := col + cols
		if col+cols > len(line) {
			bounds = len(line)
		}

		if lastBg == nil {
			start = x
			end = x
			lastBg = bg
			lastHighlight = highlight
		}

		if lastBg != nil {
			if lastBg.equals(bg) {
				end = x
			}
			if x < w.viewportMargins[2] {
				horScrollPixels = 0
				w.fillCellRect(p, lastHighlight, lastBg, y, start, end, horScrollPixels, verScrollPixels, isDrawDefaultBg)

				continue
			}
			if x > w.cols-w.viewportMargins[3]-1 {
				horScrollPixels = 0
				w.fillCellRect(p, lastHighlight, lastBg, y, start, end, horScrollPixels, verScrollPixels, isDrawDefaultBg)

				start = x
				end = x

				if x == bounds {
					w.fillCellRect(p, lastHighlight, lastBg, y, start, end, horScrollPixels, verScrollPixels, isDrawDefaultBg)
				}

				continue
			}
			if !lastBg.equals(bg) || x == bounds {
				w.fillCellRect(p, lastHighlight, lastBg, y, start, end, horScrollPixels, verScrollPixels, isDrawDefaultBg)

				start = x
				end = x
				lastBg = bg
				lastHighlight = highlight

				if x == bounds {
					w.fillCellRect(p, lastHighlight, lastBg, y, start, end, horScrollPixels, verScrollPixels, isDrawDefaultBg)
				}
			}
		}
	}

	w.drawMsgSep(p)
}

// Get the x-position of a cell, in pixel, for fixed/proportional fonts.
// When using proportional fonts, this functions dose not check that the `row+col`
// index exists in the `Window.xPixelsIndexes` field. Thus, it must only be
// used when it's sure that `Window.refreshLinesPixels` has already been called.
// For other cases, use `getSinglePixelX` below.
func (w *Window) getPixelX(font *Font, row, col int) float64 {
	if !font.proportional || row >= len(w.xPixelsIndexes) {
		return float64(col) * font.cellwidth
	}
	if col >= len(w.xPixelsIndexes[row]) {
		col = len(w.xPixelsIndexes[row])-1
	}
	return w.xPixelsIndexes[row][col]
}

func (w *Window) getSinglePixelX(row, col int) float64 {
	var x float64 = 0
	if row < 0 || row >= w.rows || col < 0 || col >= w.cols {
		return 0
	}
	font := w.getFont()
	endGutterIdx, _ := w.getTextOff(1)
	var i int = 0
	for ; i < endGutterIdx; i++ {
		x += font.width
	}
	for ; i < col; i++ {
		cell := w.content[row][i]
		x += getFontMetrics(font, cell.highlight).HorizontalAdvance(cell.char, -1)
	}
	return x
}

func (w *Window) fillCellRect(p *gui.QPainter, lastHighlight *Highlight, lastBg *RGBA, y, start, end, horScrollPixels, verScrollPixels int, isDrawDefaultBg bool) {

	if lastHighlight == nil {
		return
	}

	// If the background color to be painted is a Normal highlight group and another float window
	// that covers the float window and is closest in z-order has the same background color,
	// the background color should not be painted.
	if w.isFloatWin && !w.isMsgGrid {
		if w.zindex.nearestLowerZOrderWindow != nil && w.zindex.nearestLowerZOrderWindow.isFloatWin {
			if lastHighlight.uiName == "NormalFloat" || lastHighlight.uiName == "NormalNC" {
				if w.s.getHighlightByUiname("Normal").bg().Hex() == lastHighlight.bg().Hex() {
					return
				}
			}
		}
	}

	width := end - start + 1
	if width < 0 {
		width = 0
	}
	if !isDrawDefaultBg && lastBg.equals(w.background) {
		width = 0
	}
	if lastHighlight.isSignColumn() {
		horScrollPixels = 0
	}

	if width == 0 {
		return
	}

	font := w.getFont()
	pattern, color, transparent := w.getFillpatternAndTransparent(lastHighlight)

	var brush *gui.QBrush
	if editor.config.Editor.CachedDrawing {
		brushv, err := w.s.bgcache.get(BrushKey{
			pattern:     pattern,
			color:       color,
			transparent: transparent,
		})
		if err != nil {
			brush = newBgBrushCache(pattern, color, transparent)
			w.setBgBrushCache(pattern, color, transparent, brush)
		} else {
			brush = brushv.(*gui.QBrush)
		}
	} else {
		brush = newBgBrushCache(pattern, color, transparent)
	}

	// Get the position and width of the Rect in pixels.
	var pixelStart, pixelWidth float64
	if !font.proportional {
		pixelStart = float64(start) * font.cellwidth
		pixelWidth = float64(width) * font.cellwidth
	} else {
		pixelStart = w.getPixelX(font, y, start)
		pixelWidth = w.getPixelX(font, y, start+width) - pixelStart
	}

	if verScrollPixels == 0 ||
		verScrollPixels > 0 && (y < w.rows-w.viewportMargins[1]-1) ||
		verScrollPixels < 0 && (y > w.viewportMargins[0]) {

		// Fill background with pattern
		rectF := core.NewQRectF4(
			pixelStart+float64(horScrollPixels),
			float64((y)*font.lineHeight+verScrollPixels),
			pixelWidth,
			float64(font.lineHeight),
		)

		p.FillRect(
			rectF,
			brush,
		)

	}

	// Addresses an issue where smooth scrolling with a touchpad causes incomplete
	// background rendering at the top or bottom of floating windows.
	// Adds compensation drawing for areas partially scrolled into view by checking
	// `verScrollPixels` and filling the necessary background to prevent visual gaps.

	if verScrollPixels > 0 {
		if y == w.viewportMargins[0] {

			ypos := float64((y) * font.lineHeight)
			if verScrollPixels < 0 {
				ypos = ypos + float64(font.lineHeight+verScrollPixels)
			}

			// Fill background with pattern
			rectF := core.NewQRectF4(
				float64(start)*font.cellwidth+float64(horScrollPixels),
				ypos,
				float64(width)*font.cellwidth,
				math.Abs(float64(verScrollPixels)),
			)
			p.FillRect(
				rectF,
				brush,
			)
		}

		if y == w.rows-w.viewportMargins[1]-1 {
			// Fill background with pattern
			rectF := core.NewQRectF4(
				float64(start)*font.cellwidth+float64(horScrollPixels),
				float64((y)*font.lineHeight+verScrollPixels),
				float64(width)*font.cellwidth,
				float64(font.lineHeight-verScrollPixels),
			)
			p.FillRect(
				rectF,
				brush,
			)
		}

	}
	if verScrollPixels < 0 {
		if y == w.viewportMargins[0] {
			// Fill background with pattern
			rectF := core.NewQRectF4(
				float64(start)*font.cellwidth+float64(horScrollPixels),
				float64((y)*font.lineHeight),
				float64(width)*font.cellwidth,
				float64(font.lineHeight+verScrollPixels),
			)
			p.FillRect(
				rectF,
				brush,
			)
		}
		if y == w.rows-w.viewportMargins[1]-1 {

			ypos := float64((y) * font.lineHeight)
			ypos = ypos + float64(font.lineHeight+verScrollPixels)

			// Fill background with pattern
			rectF := core.NewQRectF4(
				float64(start)*font.cellwidth+float64(horScrollPixels),
				ypos,
				float64(width)*font.cellwidth,
				math.Abs(float64(verScrollPixels)),
			)
			p.FillRect(
				rectF,
				brush,
			)
		}

	}

}

func newBgBrushCache(pattern core.Qt__BrushStyle, color *RGBA, transparent int) *gui.QBrush {

	return gui.NewQBrush3(
		gui.NewQColor3(
			color.R,
			color.G,
			color.B,
			transparent,
		),
		pattern,
	)

}

func (w *Window) setBgBrushCache(pattern core.Qt__BrushStyle, color *RGBA, transparent int, brush *gui.QBrush) {
	w.s.bgcache.set(
		BrushKey{
			pattern:     pattern,
			color:       color,
			transparent: transparent,
		},
		brush,
	)
}

func (w *Window) newBgCache(lastHighlight *Highlight, length int) *gui.QImage {
	font := w.getFont()
	width := float64(length) * font.cellwidth
	height := float64(font.lineHeight)

	image := gui.NewQImage3(
		int(w.devicePixelRatio*width),
		int(w.devicePixelRatio*height),
		gui.QImage__Format_ARGB32_Premultiplied,
	)

	image.SetDevicePixelRatio(w.devicePixelRatio)

	// Set diff pattern
	_, color, transparent := w.getFillpatternAndTransparent(lastHighlight)

	image.Fill2(
		gui.NewQColor3(
			color.R,
			color.G,
			color.B,
			transparent,
		),
	)

	return image
}

func (w *Window) setBgCache(highlight *Highlight, length int, image *gui.QImage) {
	if w.font != nil {
		w.cache.set(
			HlBgKey{
				bg:     highlight.bg(),
				length: length,
			},
			image,
		)
	} else {
		w.s.cache.set(
			HlBgKey{
				bg:     highlight.bg(),
				length: length,
			},
			image,
		)
	}
}

func (w *Window) drawMsgSep(p *gui.QPainter) {
	if !w.isMsgGrid {
		return
	}
	if !editor.config.Message.ShowMessageSeparators {
		return
	}

	hl := w.s.getHighlightByUiname("MsgSeparator")
	color := hl.bg().QColor()

	p.FillRect5(
		0,
		0,
		w.Width(),
		1,
		color,
	)
}

func resolveFontFallback(font *Font, fallbackfonts []*Font, char string) *Font {
	if len(fallbackfonts) == 0 {
		return font
	}

	hasGlyph := font.hasGlyph(char)
	if hasGlyph {
		return font
	} else {
		for _, ff := range fallbackfonts {
			hasGlyph = ff.hasGlyph(char)
			if hasGlyph {
				return ff
			}
		}
	}

	return font
}

func getFontMetrics(font *Font,  highlight *Highlight) *gui.QFontMetricsF {
	if !highlight.italic {
		if !highlight.bold {
			return font.fontMetrics
		}
		return font.boldFontMetrics
	}
	if !highlight.bold {
		return font.italicFontMetrics
	}
	return font.italicBoldFontMetrics
}

/* Compute the pixel index of each character for each row in range.
 * This function is only usefull for proportional fonts,
 * because we can't do `col * font.cellwidth`. */
func (w *Window) refreshLinesPixels(row_start, row_end int) {
	if row_end >= w.rows {
		row_end = w.rows - 1
	}
	// Font Metrics is used to get the length of each character
	font := w.getFont()
	// For gutter alignment
	endGutterIdx, _ := w.getTextOff(1)
	// Only Reallocate slices if necessary
	// - Reallocation for the whole matrix
	if w.xPixelsIndexes == nil || cap(w.xPixelsIndexes) <= row_end {
		w.xPixelsIndexes = make([][]float64, w.rows+1)
	}
	// - Reallocation for the subslices
	if len(w.xPixelsIndexes) == 0 ||
		w.xPixelsIndexes[0] == nil ||
		cap(w.xPixelsIndexes[0]) < w.cols+1 {
		for i, _ := range w.xPixelsIndexes {
			w.xPixelsIndexes[i] = make([]float64, w.cols+1)
		}
	}
	// Temporary Pseudo Cache for character lengths
	cache := make(map[PFKey]float64)
	// Iterate over the lines to be drawn
	for y := row_start; y <= row_end; y++ {
		line := w.content[y]
		var x float64
		var i int
		// It will behave strangely if `vim.opt.showcmd` is set to true.
		for i = 0; i < endGutterIdx; i++ {
			w.xPixelsIndexes[y][i] = x
			x += font.width
		}
		for ; i < len(line); i++ {
			w.xPixelsIndexes[y][i] = x
			cell := line[i]
			key := PFKey{char: cell.char, italic: cell.highlight.italic, bold: cell.highlight.bold}
			charLen, ok := cache[key]
			if !ok {
				charLen = getFontMetrics(font, cell.highlight).HorizontalAdvance(cell.char, -1)
				cache[key] = charLen
			}
			// Update the index
			x += charLen
		}
		w.xPixelsIndexes[y][w.cols] = x
	}
}

/* Get the buffer offset, i.e. SignColumn and Numbers. */
func (w *Window) getTextOff(delayMs time.Duration) (int, bool) {
	if !editor.config.Editor.ProportionalFontAlignGutter {
		return 0, true
	}
	if w.isFloatWin || w.isMsgGrid || w.isPopupmenu {
		return 0, false
	}
	var isValid bool
	validCh := make(chan bool)
	errCh := make(chan error)
	go func() {
		result, err := w.s.ws.nvim.IsWindowValid(w.id)
		if err != nil {
			errCh <- err
			validCh <- false
			return
		} else {
			validCh <- result
			errCh <- nil
			return
		}
	}()
	select {
	case <- errCh:
		return 0, false
	case isValid = <-validCh:
	case <-time.After(delayMs * time.Millisecond):
		// If the delay is over, returns the last stored version.
		// This avoids flickering.
		return w.endGutterIdx, false
	}
	if !isValid {
		return 0, false
	}
	var output any
	err := w.s.ws.nvim.Call("getwininfo", &output, w.id)
	if err != nil {
		return 0, false
	}
	outputArr, ok := output.([]any)
	if !ok || len(outputArr) < 1 {
		return 0, false
	}
	outputMap, ok := outputArr[0].(map[string]any)
	if !ok {
		return 0, false
	}
	textoff, ok := outputMap["textoff"]
	if !ok {
		return 0, false
	}
	textoffInt, ok := textoff.(int64)
	if !ok {
		return 0, false
	}
	w.endGutterIdx = int(textoffInt)
	return w.endGutterIdx, true
}

func (w *Window) drawText(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	wsfont := w.getFont()

	if !editor.config.Editor.CachedDrawing {
		p.SetFont(wsfont.qfont)
	}

	line := w.content[y]
	chars := map[HlKey][]int{}
	specialChars := []int{}
	cellBasedDrawing := editor.config.Editor.DisableLigatures || (editor.config.Editor.Letterspace > 0)
	wsfontLineHeight := y * wsfont.lineHeight

	// Set smooth scroll offset
	var horScrollPixels, verScrollPixels int

	for x := col; x <= col+cols; x++ {
		if x >= len(line) {
			continue
		}
		if line[x] == nil {
			continue
		}
		if line[x].char == "" {
			continue
		}
		if line[x].char == " " {
			continue
		}
		if !line[x].normalWidth && !wsfont.proportional {
			specialChars = append(specialChars, x)
			continue
		}
		if line[x].scaled {
			specialChars = append(specialChars, x)
			continue
		}

		// If the ligature setting is disabled,
		// we will draw the characters on the screen one by one.
		if cellBasedDrawing {

			if line[x].covered && w.grid == 1 {
				continue
			}

			if w.s.ws.mouseScroll != "" {
				horScrollPixels = w.scrollPixels[0]
			}
			if w.lastScrollphase != core.Qt__NoScrollPhase {
				verScrollPixels = w.scrollPixels2
			}
			if editor.config.Editor.LineToScroll == 1 {
				verScrollPixels += w.scrollPixels[1]
			}
			if line[x].highlight.isSignColumn() {
				horScrollPixels = 0
			}
			if x < w.viewportMargins[2] || x > w.cols-w.viewportMargins[3]-1 {
				horScrollPixels = 0
				verScrollPixels = 0
			}
			if y < w.viewportMargins[0] || y > w.rows-w.viewportMargins[1]-1 {
				horScrollPixels = 0
				verScrollPixels = 0
			}

			w.drawTextInPos(
				p,
				int(float64(x)*wsfont.cellwidth)+horScrollPixels,
				wsfontLineHeight+verScrollPixels,
				line[x].char,
				HlKey{
					fg:     *(line[x].highlight.fg()),
					bold:   line[x].highlight.bold,
					italic: line[x].highlight.italic,
				},
				true,
				line[x].scaled,
			)

		} else {
			// Prepare to draw a group of identical highlight units.
			highlight := line[x].highlight
			if x < w.viewportMargins[2] || x > w.cols-w.viewportMargins[3]-1 {
				highlight.special = highlight.special.copy()
				highlight.foreground = highlight.foreground.copy()
				highlight.background = highlight.background.copy()
			}

			hlkey := HlKey{
				fg:     *(highlight.fg()),
				italic: highlight.italic,
				bold:   highlight.bold,
			}
			colorSlice, ok := chars[hlkey]
			if !ok {
				colorSlice = []int{}
			}
			colorSlice = append(colorSlice, x)
			chars[hlkey] = colorSlice
		}
	}

	// This is the normal rendering process for goneovim,
	// we draw a word snippet of the same highlight on the screen for each of the highlights.
	if !cellBasedDrawing {
		for hlkey, colorSlice := range chars {
			var buffer bytes.Buffer
			slice := colorSlice

			isIndentationWhiteSpace := true
			pos := col
			for x := col; x <= col+cols; x++ {
				if w.s.ws.mouseScroll != "" {
					horScrollPixels = w.scrollPixels[0]
				}
				if w.lastScrollphase != core.Qt__NoScrollPhase {
					verScrollPixels = w.scrollPixels2
				}
				if editor.config.Editor.LineToScroll == 1 {
					verScrollPixels += w.scrollPixels[1]
				}
				if line[x].highlight.isSignColumn() {
					horScrollPixels = 0
				}
				if x < w.viewportMargins[2] || x > w.cols-w.viewportMargins[3]-1 {
					horScrollPixels = 0
					verScrollPixels = 0
				}
				if y < w.viewportMargins[0] || y > w.rows-w.viewportMargins[1]-1 {
					horScrollPixels = 0
					verScrollPixels = 0
				}

				isDrawWord := false
				index := slice[0]

				if len(slice) != 0 {

					// e.g. when the contents of the line is;
					//    [ 'a', 'b', ' ', 'c', ' ', ' ', 'd', 'e', 'f' ]
					//
					// then, the slice is [ 1,2,4,7,8,9 ]
					// the following process is
					//  * If a word is separated by a single space, it is treated as a single word.
					//  * If there are more than two continuous spaces, each word separated by a space
					//    is treated as an independent word.
					//
					//  therefore, the above example will treet that;
					//  "ab c" and "def"

					if x != index {
						if isIndentationWhiteSpace {
							continue
						} else {
							if len(slice) > 1 {
								if x+1 == index {
									if buffer.Len() > 0 {
										pos++
										buffer.WriteString(" ")
									}
								} else {
									isDrawWord = true
								}
							} else {
								isDrawWord = true
							}
						}
					}

					if x == index {
						pos++

						char := line[x].char
						if line[x].covered && w.grid == 1 {
							char = " "
						}
						buffer.WriteString(char)
						slice = slice[1:]
						isIndentationWhiteSpace = false

					}
				}

				if isDrawWord || len(slice) == 0 {
					if len(slice) == 0 {
						x++
					}

					if buffer.Len() != 0 {
						w.drawTextInPos(
							p,
							int(w.getPixelX(wsfont, y, x-pos))+horScrollPixels,
							wsfontLineHeight+verScrollPixels,
							buffer.String(),
							hlkey,
							true,
							false,
						)

						buffer.Reset()
						isDrawWord = false
						pos = 0
					}

					if len(slice) == 0 {
						break
					}
				}
			}
		}
	}

	if len(specialChars) >= 1 {
		for _, x := range specialChars {
			if line[x] == nil {
				continue
			}
			char := line[x].char
			if line[x].covered && w.grid == 1 {
				char = " "
			}
			if char == " " {
				continue
			}

			if w.s.ws.mouseScroll != "" {
				horScrollPixels = w.scrollPixels[0]
			}
			if w.lastScrollphase != core.Qt__NoScrollPhase {
				verScrollPixels = w.scrollPixels2
			}
			if editor.config.Editor.LineToScroll == 1 {
				verScrollPixels += w.scrollPixels[1]
			}
			if line[x].highlight.isSignColumn() {
				horScrollPixels = 0
			}
			if x < w.viewportMargins[2] || x > w.cols-w.viewportMargins[3]-1 {
				horScrollPixels = 0
				verScrollPixels = 0
			}
			if y < w.viewportMargins[0] || y > w.rows-w.viewportMargins[1]-1 {
				horScrollPixels = 0
				verScrollPixels = 0
			}

			w.drawTextInPos(
				p,
				int(w.getPixelX(wsfont, y, x))+horScrollPixels,
				wsfontLineHeight+verScrollPixels,
				line[x].char,
				HlKey{
					fg:     *(line[x].highlight.fg()),
					bold:   line[x].highlight.bold,
					italic: line[x].highlight.italic,
				},
				false,
				line[x].scaled,
			)

		}
	}
}

func (w *Window) drawTextInPos(p *gui.QPainter, x, y int, text string, hlkey HlKey, isNormalWidth bool, scaled bool) {
	wsfont := w.getFont()

	// var horScrollPixels int
	// horScrollPixels = w.scrollPixels[0]
	// if highlight.isSignColumn() {
	// 	horScrollPixels = 0
	// }

	// if CachedDrawing is disabled
	if !editor.config.Editor.CachedDrawing {
		w.drawTextInPosWithNoCache(
			p,
			x, //+horScrollPixels,
			y+wsfont.shift,
			text,
			hlkey,
			isNormalWidth,
			scaled,
		)
	} else { // if CachedDrawing is enabled
		w.drawTextInPosWithCache(
			p,
			x, //+horScrollPixels,
			y,
			text,
			hlkey,
			isNormalWidth,
			scaled,
		)
	}
}

func (w *Window) drawTextInPosWithNoCache(p *gui.QPainter, x, y int, text string, hlkey HlKey, isNormalWidth bool, scaled bool) {
	if text == "" {
		return
	}

	var fontfallbacked *Font
	if !isASCII(text) && w.font == nil && w.s.fontwide != nil {
		fontfallbacked = resolveFontFallback(w.s.fontwide, w.s.fallbackfontwides, text)
	} else {
		if w.font == nil {
			fontfallbacked = resolveFontFallback(w.s.font, w.s.fallbackfonts, text)
		} else {
			fontfallbacked = resolveFontFallback(w.font, w.fallbackfonts, text)
		}
	}

	p.SetFont(fontfallbacked.qfont)

	font := p.Font()
	fg := &(hlkey.fg)
	p.SetPen2(fg.QColor())

	font.SetBold(hlkey.bold)
	font.SetItalic(hlkey.italic)
	p.DrawText3(x, y, text)
}

func (w *Window) drawTextInPosWithCache(p *gui.QPainter, x, y int, text string, hlkey HlKey, isNormalWidth bool, scaled bool) {
	if text == "" {
		return
	}

	cache := w.getCache()
	var image *gui.QImage
	imagev, err := cache.get(HlTextKey{
		text:   text,
		fg:     hlkey.fg,
		italic: hlkey.italic,
		bold:   hlkey.bold,
	})

	if err != nil {
		image = w.newTextCache(text, hlkey, isNormalWidth)
		if image != nil {
			w.setTextCache(text, hlkey, image)
		}
	} else {
		image = imagev.(*gui.QImage)
	}

	// return if image is invalid
	if image == nil {
		return
	}

	// Scale specific characters to full line height
	yOffset := 0
	if scaled {
		font := w.getFont()
		ratio := (float64(font.lineHeight) / float64(font.height))
		if ratio != 1.0 {
			newHeight := int(math.Floor(w.devicePixelRatio * float64(font.lineHeight) * ratio))
			newSpace := int(math.Ceil(float64(font.lineSpace) * ratio / 2.0))
			yOffset = newSpace
			image = image.Scaled2(
				image.Width(),
				newHeight,
				core.Qt__IgnoreAspectRatio,
				core.Qt__SmoothTransformation,
			)
		}
	}

	p.DrawImage9(
		x, y-yOffset,
		image,
		0, 0,
		-1, -1,
		core.Qt__AutoColor,
	)
}

func (w *Window) setDecorationCache(highlight *Highlight, image *gui.QImage) {
	if w.font != nil {
		// If window has own font setting
		w.cache.set(
			HlDecorationKey{
				fg:            highlight.foreground,
				bg:            highlight.background,
				sp:            highlight.special,
				underline:     highlight.underline,
				undercurl:     highlight.undercurl,
				strikethrough: highlight.strikethrough,
				underdouble:   highlight.underdouble,
				underdotted:   highlight.underdotted,
				underdashed:   highlight.underdashed,
			},
			image,
		)
	} else {
		// screen text cache
		w.s.cache.set(
			HlDecorationKey{
				fg:            highlight.foreground,
				bg:            highlight.background,
				sp:            highlight.special,
				underline:     highlight.underline,
				undercurl:     highlight.undercurl,
				strikethrough: highlight.strikethrough,
				underdouble:   highlight.underdouble,
				underdotted:   highlight.underdotted,
				underdashed:   highlight.underdashed,
			},
			image,
		)
	}
}

func (w *Window) newDecorationCache(char string, highlight *Highlight, isNormalWidth bool) *gui.QImage {
	font := w.getFont()

	width := font.cellwidth

	// // Set smooth scroll offset
	// var horScrollPixels, verScrollPixels int
	// if w.s.ws.mouseScroll != "" {
	// 	horScrollPixels += w.scrollPixels[0]
	// }
	// if w.lastScrollphase != core.Qt__NoScrollPhase {
	// 	verScrollPixels = w.scrollPixels2
	// }
	// if editor.config.Editor.LineToScroll == 1 {
	// 	verScrollPixels += w.scrollPixels[1]
	// }

	// create QImage
	image := gui.NewQImage3(
		int(math.Ceil(w.devicePixelRatio*width)),
		int(w.devicePixelRatio*float64(font.lineHeight)),
		gui.QImage__Format_ARGB32_Premultiplied,
	)
	image.SetDevicePixelRatio(w.devicePixelRatio)
	image.Fill3(core.Qt__transparent)

	pi := gui.NewQPainter2(image)

	w.drawDecoration(pi, highlight, font, 0, 0, int(width), 0, 0)

	pi.DestroyQPainter()

	return image
}

func (w *Window) setTextCache(text string, hlkey HlKey, image *gui.QImage) {
	if w.font != nil {
		// If window has own font setting
		w.cache.set(
			HlTextKey{
				text:   text,
				fg:     hlkey.fg,
				italic: hlkey.italic,
				bold:   hlkey.bold,
			},
			image,
		)
	} else {
		// screen text cache
		w.s.cache.set(
			HlTextKey{
				text:   text,
				fg:     hlkey.fg,
				italic: hlkey.italic,
				bold:   hlkey.bold,
			},
			image,
		)
	}
}

func (w *Window) initImagePainter() {
	if w.imagePainter == nil {
		w.imagePainter = gui.NewQPainter()
	}
}

func (w *Window) destroyImagePainter() {
	if w.imagePainter != nil {
		w.imagePainter.DestroyQPainter()
		w.imagePainter = nil
	}
}

func (w *Window) newTextCache(text string, hlkey HlKey, isNormalWidth bool) *gui.QImage {
	// * Ref: https://stackoverflow.com/questions/40458515/a-best-way-to-draw-a-lot-of-independent-characters-in-qt5/40476430#40476430
	editor.putLog("start creating word cache:", text)

	font := w.getFont()
	var fontfallbacked *Font
	if !isASCII(text) && w.font == nil && w.s.fontwide != nil {
		fontfallbacked = resolveFontFallback(w.s.fontwide, w.s.fallbackfontwides, text)
	} else {
		if w.font == nil {
			fontfallbacked = resolveFontFallback(w.s.font, w.s.fallbackfonts, text)
		} else {
			fontfallbacked = resolveFontFallback(w.font, w.fallbackfonts, text)
		}
	}

	// Put debug log
	if editor.opts.Debug != "" {
		fi := gui.NewQFontInfo(font.qfont)
		editor.putLog(
			"Outputs font information creating word cache:",
			fi.Family(),
			fi.PointSizeF(),
			fi.StyleName(),
			fmt.Sprintf("%v", fi.PointSizeF()),
		)
	}

	width := float64(len(text))*font.cellwidth + 1
	if fontfallbacked.proportional {
		fmWidth := fontfallbacked.fontMetrics.HorizontalAdvance(text, -1)
		if fmWidth > width {
			width = fmWidth
		}
	} else if hlkey.italic {
		width = float64(len(text))*font.italicWidth + 1
	}

	fg := hlkey.fg
	if !isNormalWidth {
		advance := fontfallbacked.fontMetrics.HorizontalAdvance(text, -1)
		if advance > 0 {
			width = advance
		}
	}

	imageWidth := int(math.Ceil(w.devicePixelRatio * width))
	imageHeight := int(w.devicePixelRatio * float64(font.lineHeight))
	if imageWidth <= 0 || imageHeight <= 0 {
		return nil
	}

	// QImage default device pixel ratio is 1.0,
	// So we set the correct device pixel ratio

	// image := gui.NewQImage2(
	// 	core.NewQRectF4(
	// 		0,
	// 		0,
	// 		w.devicePixelRatio*width,
	// 		w.devicePixelRatio*float64(font.lineHeight),
	// 	).Size().ToSize(),
	// 	gui.QImage__Format_ARGB32_Premultiplied,
	// )
	image := gui.NewQImage3(
		imageWidth,
		imageHeight,
		gui.QImage__Format_ARGB32_Premultiplied,
	)

	image.SetDevicePixelRatio(w.devicePixelRatio)
	image.Fill3(core.Qt__transparent)

	w.initImagePainter()
	w.imagePainter.Begin(image)
	// pi := gui.NewQPainter2(image)

	w.imagePainter.SetPen2(fg.QColor())
	w.imagePainter.SetFont(fontfallbacked.qfont)

	if hlkey.bold {
		w.imagePainter.Font().SetBold(hlkey.bold)
	}
	if hlkey.italic {
		w.imagePainter.Font().SetItalic(hlkey.italic)
	}

	w.imagePainter.DrawText6(
		core.NewQRectF4(
			0,
			0,
			width,
			float64(font.lineHeight),
		), text, gui.NewQTextOption2(core.Qt__AlignVCenter),
	)

	w.imagePainter.End()
	// pi.DestroyQPainter()

	editor.putLog("finished creating word cache:", text)

	if !isNormalWidth {
		image = scaleToGridCell(
			image,
			float64(font.cellwidth)*2.0/width,
		)
	}

	return image
}

func scaleToGridCell(image *gui.QImage, ratio float64) *gui.QImage {
	if ratio >= 1.0 {
		return image
	}

	return image.Scaled2(
		int(float64(image.Width())*ratio),
		int(float64(image.Height())*ratio),
		core.Qt__IgnoreAspectRatio,
		core.Qt__SmoothTransformation,
	)
}

func (w *Window) drawForeground(p *gui.QPainter, y int, col int, cols int) {
	if w.s.name == "minimap" {
		w.drawMinimap(p, y, col, cols)
	} else {
		// w.drawText(p, y, col, cols)
		w.drawText(p, y, 0, w.cols)
		w.drawTextDecoration(p, y, col, cols)
	}
}

func (w *Window) drawTextDecoration(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	line := w.content[y]
	font := w.getFont()
	cache := w.getCache()

	// Set smooth scroll offset
	var horScrollPixels, verScrollPixels int
	if w.s.ws.mouseScroll != "" {
		horScrollPixels += w.scrollPixels[0]
	}
	if w.lastScrollphase != core.Qt__NoScrollPhase {
		verScrollPixels = w.scrollPixels2
	}
	if editor.config.Editor.LineToScroll == 1 {
		verScrollPixels += w.scrollPixels[1]
	}

	for x := col; x <= col+cols; x++ {
		if x >= len(line) {
			continue
		}
		if line[x] == nil {
			continue
		}

		highlight := line[x].highlight

		if !highlight.underline &&
			!highlight.undercurl &&
			!highlight.strikethrough &&
			!highlight.underdouble &&
			!highlight.underdotted &&
			!highlight.underdashed {
			continue
		}
		if line[x].covered && w.grid == 1 {
			continue
		}

		// if CachedDrawing is disabled
		if !editor.config.Editor.CachedDrawing {
			w.drawDecoration(p, highlight, font, y, x, x+1, verScrollPixels, horScrollPixels)
		} else { // if CachedDrawing is enabled
			var image *gui.QImage
			imagev, err := cache.get(HlDecorationKey{
				fg:            highlight.foreground,
				bg:            highlight.background,
				sp:            highlight.special,
				underline:     highlight.underline,
				undercurl:     highlight.undercurl,
				strikethrough: highlight.strikethrough,
				underdouble:   highlight.underdouble,
				underdotted:   highlight.underdotted,
				underdashed:   highlight.underdashed,
			})

			if err != nil {
				image = w.newDecorationCache(line[x].char, highlight, line[x].normalWidth)
				w.setDecorationCache(highlight, image)
			} else {
				image = imagev.(*gui.QImage)
			}

			p.DrawImage7(
				core.NewQPointF3(
					w.getPixelX(font, y, x)+float64(horScrollPixels),
					float64(y*font.lineHeight)+float64(verScrollPixels),
				),
				image,
			)
		}
	}

}

func (w *Window) drawDecoration(p *gui.QPainter, highlight *Highlight, font *Font, row, x1, x2, verScrollPixels, horScrollPixels int) {
	pen := gui.NewQPen()
	var color *gui.QColor
	sp := highlight.special
	if sp != nil {
		color = sp.QColor()
		pen.SetColor(color)
	} else {
		color = highlight.fg().QColor()
		pen.SetColor(color)
	}
	p.SetPen(pen)

	var start, end float64
	if !w.getFont().proportional {
		start = float64(x1) * font.cellwidth
		end = float64(x2) * font.cellwidth
	} else {
		start = w.xPixelsIndexes[row][x1]
		end = w.xPixelsIndexes[row][x2]
	}

	if highlight.strikethrough {
		drawStrikethrough(p, font, color, row, start, end, verScrollPixels, horScrollPixels)
	}
	if highlight.underline {
		drawUnderline(p, font, color, row, start, end, verScrollPixels, horScrollPixels)
	}
	if highlight.undercurl {
		drawUndercurl(p, font, color, row, start, end, verScrollPixels, horScrollPixels)
	}
	if highlight.underdouble {
		drawUnderdouble(p, font, color, row, start, end, verScrollPixels, horScrollPixels)
	}
	if highlight.underdotted {
		drawUnderdotted(p, font, color, row, start, end, verScrollPixels, horScrollPixels)
	}
	if highlight.underdashed {
		drawUnderdashed(p, font, color, row, start, end, verScrollPixels, horScrollPixels)
	}
}

func drawStrikethrough(p *gui.QPainter, font *Font, color *gui.QColor, row int, start, end float64, verScrollPixels, horScrollPixels int) {
	space := float64(font.lineSpace) / 3.0
	if math.Abs(space) > font.ascent/3.0 {
		space = font.ascent / 3.0
	}
	space2 := float64(font.lineSpace)
	if space2 < -1 {
		space2 = float64(font.lineSpace) / 2.0
	}
	// descent := float64(font.height) - font.ascent

	weight := int(math.Ceil(float64(font.height) / 16.0))
	if weight < 1 {
		weight = 1
	}

	width := int(end - start)
	if width < 0 {
		width = 0
	}

	Y := float64(row*font.lineHeight+verScrollPixels) + float64(font.ascent)*0.65 + float64(space2/2)

	p.FillRect5(
		int(start)+horScrollPixels,
		int(Y),
		width,
		weight,
		color,
	)
}

func drawUnderline(p *gui.QPainter, font *Font, color *gui.QColor, row int, start, end float64, verScrollPixels, horScrollPixels int) {
	space := float64(font.lineSpace) / 3.0
	if math.Abs(space) > font.ascent/3.0 {
		space = font.ascent / 3.0
	}
	space2 := float64(font.lineSpace)
	if space2 < -1 {
		space2 = float64(font.lineSpace) / 2.0
	}
	descent := float64(font.height) - font.ascent

	weight := int(math.Ceil(float64(font.height) / 18.0))
	if weight < 1 {
		weight = 1
	}

	Y := float64(row*font.lineHeight+verScrollPixels) + float64(font.ascent) + descent*0.5 + float64(font.lineSpace/2) + space

	width := int(end - start)
	if width < 0 {
		width = 0
	}

	p.FillRect5(
		int(start)+horScrollPixels,
		int(Y),
		width,
		weight,
		color,
	)
}

func drawUndercurl(p *gui.QPainter, font *Font, color *gui.QColor, row int, start, end float64, verScrollPixels, horScrollPixels int) {
	space := float64(font.lineSpace) / 3.0
	if math.Abs(space) > font.ascent/3.0 {
		space = font.ascent / 3.0
	}
	space2 := float64(font.lineSpace)
	if space2 < -1 {
		space2 = float64(font.lineSpace) / 2.0
	}
	descent := float64(font.height) - font.ascent

	weight := int(math.Ceil(float64(font.height) / 16.0))
	if weight < 1 {
		weight = 1
	}

	amplitude := descent*0.65 + float64(space2)
	maxAmplitude := font.ascent / 8.0
	if amplitude >= maxAmplitude {
		amplitude = maxAmplitude
	}
	freq := 1.0
	phase := 0.0
	Y := float64(row*font.lineHeight+verScrollPixels) + float64(font.ascent+descent*0.3) + float64(space2/2) + space
	Y2 := Y + amplitude*math.Sin(0)
	point := core.NewQPointF3(start+float64(horScrollPixels), Y2)
	path := gui.NewQPainterPath2(point)
	for i := int(point.X()); i <= int(end); i++ {
		Y2 = Y + amplitude*math.Sin(2*math.Pi*freq*float64(i)/font.cellwidth+phase)
		path.LineTo(core.NewQPointF3(float64(i), Y2))
	}
	p.DrawPath(path)
}

func drawUnderdouble(p *gui.QPainter, font *Font, color *gui.QColor, row int, start, end float64, verScrollPixels, horScrollPixels int) {
	space := float64(font.lineSpace) / 3.0
	if math.Abs(space) > font.ascent/3.0 {
		space = font.ascent / 3.0
	}
	space2 := float64(font.lineSpace)
	if space2 < -1 {
		space2 = float64(font.lineSpace) / 2.0
	}
	descent := float64(font.height) - font.ascent

	weight := int(math.Ceil(float64(font.height) / 16.0))
	if weight < 1 {
		weight = 1
	}

	Y1 := float64(row*font.lineHeight+verScrollPixels) + float64(font.ascent) + descent*0.1 + float64(font.lineSpace/2) + space
	Y2 := float64(row*font.lineHeight+verScrollPixels) + float64(font.ascent) + descent*0.8 + float64(font.lineSpace/2) + space

	width := int(end - start)
	if width < 0 {
		width = 0
	}

	doubleLineWeight := int(math.Ceil(float64(font.height) / 20.0))

	p.FillRect5(
		int(start)+horScrollPixels,
		int(Y1),
		width,
		doubleLineWeight,
		color,
	)
	p.FillRect5(
		int(start)+horScrollPixels,
		int(Y2),
		width,
		doubleLineWeight,
		color,
	)
}

func drawUnderdotted(p *gui.QPainter, font *Font, color *gui.QColor, row int, start, end float64, verScrollPixels, horScrollPixels int) {
	space := float64(font.lineSpace) / 3.0
	if math.Abs(space) > font.ascent/3.0 {
		space = font.ascent / 3.0
	}
	space2 := float64(font.lineSpace)
	if space2 < -1 {
		space2 = float64(font.lineSpace) / 2.0
	}
	descent := float64(font.height) - font.ascent

	weight := int(math.Ceil(float64(font.height) / 16.0))
	if weight < 1 {
		weight = 1
	}
	dottedWeight := int(float64(weight) * 0.8)
	if dottedWeight < 1 {
		dottedWeight = 1
	}
	pen := gui.NewQPen()
	pen.SetWidth(dottedWeight)
	pen.SetColor(color)
	pen.SetStyle(core.Qt__DotLine)
	p.SetPen(pen)
	Y := float64(row*font.lineHeight+verScrollPixels) + float64(font.ascent) + descent*0.5 + float64(font.lineSpace/2) + space

	p.DrawLine3(
		int(start)+horScrollPixels,
		int(Y),
		int(end)+horScrollPixels,
		int(Y),
	)
}

func drawUnderdashed(p *gui.QPainter, font *Font, color *gui.QColor, row int, start, end float64, verScrollPixels, horScrollPixels int) {
	space := float64(font.lineSpace) / 3.0
	if math.Abs(space) > font.ascent/3.0 {
		space = font.ascent / 3.0
	}
	space2 := float64(font.lineSpace)
	if space2 < -1 {
		space2 = float64(font.lineSpace) / 2.0
	}
	descent := float64(font.height) - font.ascent

	weight := int(math.Ceil(float64(font.height) / 16.0))
	if weight < 1 {
		weight = 1
	}

	width := int((end - start) / 2)
	if width < 0 {
		width = 0
	}

	Y := float64(row*font.lineHeight+verScrollPixels) + float64(font.ascent) + descent*0.5 + float64(font.lineSpace/2) + space

	p.FillRect5(
		int(start)+int(math.Ceil(font.cellwidth*0.25))+horScrollPixels,
		int(Y),
		// int(math.Ceil(font.cellwidth*0.5)),
		width,
		weight,
		color,
	)
}

func (w *Window) getFillpatternAndTransparent(hl *Highlight) (core.Qt__BrushStyle, *RGBA, int) {
	color := hl.bg()
	pattern := core.Qt__BrushStyle(1)
	var t int

	// We do not use the editor's transparency for popupmenu, float window, message window.
	// It is recommended to use pumblend or winblend or editor.config.Message.Transparent to get those transparencies.
	if w.isPopupmenu {
		t = int(255 * ((100.0 - float64(w.s.ws.pb)) / 100.0))
	} else if !w.isPopupmenu && w.isFloatWin && !w.isMsgGrid {
		t = int(255 * ((100.0 - float64(w.wb)) / 100.0))
	} else if w.isMsgGrid {
		t = int(editor.config.Message.Transparent * 255.0)
	} else {
		t = int(editor.config.Editor.Transparent * 255.0)
	}

	if editor.config.Editor.DiffChangePattern != 1 && hl.hlName == "DiffChange" {
		pattern = core.Qt__BrushStyle(editor.config.Editor.DiffChangePattern)
		if editor.config.Editor.DiffChangePattern >= 7 &&
			editor.config.Editor.DiffChangePattern <= 14 {
			t = int(editor.config.Editor.Transparent * 255)
		}
		color = color.HSV().Colorfulness().RGB()
	} else if editor.config.Editor.DiffDeletePattern != 1 && hl.hlName == "DiffDelete" {
		pattern = core.Qt__BrushStyle(editor.config.Editor.DiffDeletePattern)
		if editor.config.Editor.DiffDeletePattern >= 7 &&
			editor.config.Editor.DiffDeletePattern <= 14 {
			t = int(editor.config.Editor.Transparent * 255)
		}
		color = color.HSV().Colorfulness().RGB()
	} else if editor.config.Editor.DiffAddPattern != 1 && hl.hlName == "DiffAdd" {
		pattern = core.Qt__BrushStyle(editor.config.Editor.DiffAddPattern)
		if editor.config.Editor.DiffAddPattern >= 7 &&
			editor.config.Editor.DiffAddPattern <= 14 {
			t = int(editor.config.Editor.Transparent * 255)
		}
		color = color.HSV().Colorfulness().RGB()
	}

	return pattern, color, t
}

// func isCJK(char rune) bool {
// 	if unicode.Is(unicode.Han, char) {
// 		return true
// 	}
// 	if unicode.Is(unicode.Hiragana, char) {
// 		return true
// 	}
// 	if unicode.Is(unicode.Katakana, char) {
// 		return true
// 	}
// 	if unicode.Is(unicode.Hangul, char) {
// 		return true
// 	}
//
// 	return false
// }

// isNormalWidth is:
// On Windows, HorizontalAdvance() may take a long time to get the width of CJK characters.
// For this reason, for CJK characters, the character width should be the double width of ASCII characters.
// This issue may also be related to the following.
// https://github.com/equalsraf/neovim-qt/issues/614
func (w *Window) isNormalWidth(char string) bool {
	if len(char) == 0 {
		return true
	}

	if isASCII(char) {
		return true
	}

	var fontfallbacked *Font
	if w.font == nil {
		fontfallbacked = resolveFontFallback(w.s.font, w.s.fallbackfonts, char)
	} else {
		fontfallbacked = resolveFontFallback(w.font, w.fallbackfonts, char)
	}

	return fontfallbacked.fontMetrics.HorizontalAdvance(char, -1) <= w.getFont().cellwidth
}

func isASCII(c string) bool {
	r := []rune(c)[0]
	return r >= 0 && r <= 127
}

func (w *Window) deleteExternalWin() {
	if w.extwin != nil {
		w.extwin.Hide()
		w.extwin.DestroyQDialog()
		w.extwin = nil
	}
}

func (w *Window) setResizableForExtWin() {
	if !w.extwinConnectResizable {
		w.extwin.ConnectResizeEvent(func(event *gui.QResizeEvent) {
			height := w.extwin.Height() - EXTWINBORDERSIZE*2
			width := w.extwin.Width() - EXTWINBORDERSIZE*2
			cols := int((float64(width) / w.getFont().cellwidth))
			rows := height / w.getFont().lineHeight
			w.extwinResized = true
			w.extwinManualResized = true
			_ = w.s.ws.nvim.TryResizeUIGrid(w.grid, cols, rows)
		})
		w.extwinConnectResizable = true
	}
}

func (w *Window) getCache() Cache {
	if w.font != nil {
		return w.cache
	}

	return w.s.cache
}

func newWindow() *Window {
	// widget := widgets.NewQWidget(nil, 0)
	win := NewWindow(nil, 0)
	// cursor.StackUnder(win)
	// cursor.raise()

	win.SetContentsMargins(0, 0, 0, 0)
	win.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	win.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")
	win.scrollRegion = []int{0, 0, 0, 0}
	win.background = editor.colors.bg
	win.lastMouseEvent = &inputMouseEvent{}
	win.zindex = &zindex{}
	win.charsScaledLineHeight = editor.config.Editor.CharsScaledLineHeight

	win.SetAcceptDrops(true)
	win.ConnectPaintEvent(win.paint)
	win.ConnectDragEnterEvent(win.dragEnterEvent)
	win.ConnectDragMoveEvent(win.dragMoveEvent)
	win.ConnectDropEvent(win.dropEvent)
	win.ConnectDestroyed(func(*core.QObject) {
		win.destroyImagePainter()
	})

	// HideMouseWhenTyping process
	if editor.config.Editor.HideMouseWhenTyping {
		win.InstallEventFilter(win)
		win.SetMouseTracking(true)
	}

	win.ConnectEventFilter(func(watched *core.QObject, event *core.QEvent) bool {
		switch event.Type() {
		case core.QEvent__MouseMove:
			if editor.isHideMouse && editor.config.Editor.HideMouseWhenTyping {
				gui.QGuiApplication_RestoreOverrideCursor()
				editor.isHideMouse = false
			}
		default:
		}

		return win.EventFilterDefault(watched, event)
	})

	return win
}

func (win *Window) initializeOrReuseSmoothScrollAnimation() {
	if win.smoothScrollAnimation == nil {
		win.smoothScrollAnimation = core.NewQPropertyAnimation2(win, core.NewQByteArray2("scrollDiff", -1), win)
		win.smoothScrollAnimation.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutExpo))
		win.smoothScrollAnimation.SetDuration(editor.config.Editor.SmoothScrollDuration)

		win.smoothScrollAnimation.ConnectValueChanged(func(value *core.QVariant) {
			ok := false
			v := value.ToDouble(&ok)
			if !ok {
				return
			}
			font := win.getFont()

			win.scrollPixels2 = int(v * float64(font.lineHeight))

			// var x, y int
			// win.Update2(
			// 	x+win.viewportMargins[2]*int(font.cellwidth),
			// 	y+(win.viewportMargins[0]*font.lineHeight),
			// 	int(float64(win.cols)*font.cellwidth)-win.viewportMargins[2]*int(font.cellwidth)-win.viewportMargins[3]*int(font.cellwidth),
			// 	win.rows*font.lineHeight-(win.viewportMargins[0]*font.lineHeight)-(win.viewportMargins[1]*font.lineHeight),
			// )

			var y int
			win.Update2(
				0,
				y+(win.viewportMargins[0]*font.lineHeight),
				int(float64(win.cols)*font.cellwidth),
				win.rows*font.lineHeight-(win.viewportMargins[0]*font.lineHeight)-(win.viewportMargins[1]*font.lineHeight),
			)

			if v == 0 {
				win.scrollPixels2 = 0
				win.scrollDelta = 0
				win.doErase = true
				win.Update2(
					0,
					y+(win.viewportMargins[0]*font.lineHeight),
					int(float64(win.cols)*font.cellwidth),
					win.rows*font.lineHeight-(win.viewportMargins[0]*font.lineHeight)-(win.viewportMargins[1]*font.lineHeight),
				)
				win.doErase = false
				win.fill()

			}

		})
	}
}

func (w *Window) mouseEvent(event *gui.QMouseEvent) {
	defer func() {
		editor.isWindowNowActivated = false
		editor.isWindowNowInactivated = false
		editor.isExtWinNowActivated = false
		editor.isExtWinNowInactivated = false
	}()

	if editor.config.Editor.IgnoreFirstMouseClickWhenAppInactivated {
		if w.isExternal {
			if editor.isExtWinNowActivated && !editor.isWindowNowInactivated {
				return
			}
		} else {
			if editor.isWindowNowActivated && !editor.isExtWinNowInactivated {
				return
			}
		}
	}

	if w.lastMouseEvent == nil {
		w.lastMouseEvent = &inputMouseEvent{}
	}
	if w.lastMouseEvent.event == event {
		return
	}
	w.lastMouseEvent.event = event

	if !w.s.ws.isMouseEnabled {
		return
	}

	bt := event.Button()
	if event.Type() == core.QEvent__MouseMove {
		if event.Buttons()&core.Qt__LeftButton > 0 {
			bt = core.Qt__LeftButton
		} else if event.Buttons()&core.Qt__RightButton > 0 {
			bt = core.Qt__RightButton
		} else if event.Buttons()&core.Qt__MidButton > 0 {
			bt = core.Qt__MidButton
		}
	}

	button := ""
	switch bt {
	case core.Qt__LeftButton:
		button += "left"
	case core.Qt__RightButton:
		button += "right"
	case core.Qt__MidButton:
		button += "middle"
	case core.Qt__NoButton:
	default:
	}

	action := ""
	switch event.Type() {
	case core.QEvent__MouseButtonDblClick:
		action = "press"
	case core.QEvent__MouseButtonPress:
		action = "press"
	case core.QEvent__MouseButtonRelease:
		action = "release"
	case core.QEvent__MouseMove:
		action = "drag"
	default:
	}

	mod := editor.modPrefix(event.Modifiers())

	font := w.getFont()
	col := int(float64(event.X()) / font.cellwidth)
	row := int(float64(event.Y()) / float64(font.lineHeight))

	if w.lastMouseEvent.button == button &&
		w.lastMouseEvent.action == action &&
		w.lastMouseEvent.mod == mod &&
		w.lastMouseEvent.grid == w.grid &&
		w.lastMouseEvent.row == row &&
		w.lastMouseEvent.col == col {
		return
	}

	w.lastMouseEvent.button = button
	w.lastMouseEvent.action = action
	w.lastMouseEvent.mod = mod
	w.lastMouseEvent.grid = w.grid
	w.lastMouseEvent.row = row
	w.lastMouseEvent.col = col

	w.s.ws.nvim.InputMouse(button, action, mod, w.grid, row, col)
}

func (w *Window) dragEnterEvent(e *gui.QDragEnterEvent) {
	e.AcceptProposedAction()
}

func (w *Window) dragMoveEvent(e *gui.QDragMoveEvent) {
	e.AcceptProposedAction()
}

func (w *Window) dropEvent(e *gui.QDropEvent) {
	e.SetDropAction(core.Qt__CopyAction)
	e.AcceptProposedAction()
	e.SetAccepted(true)

	w.focusGrid()

	for _, i := range strings.Split(e.MimeData().Text(), "\n") {
		data := strings.Split(i, "://")
		if i != "" {
			switch data[0] {
			case "file":
				buf, _ := w.s.ws.nvim.CurrentBuffer()
				bufName, _ := w.s.ws.nvim.BufferName(buf)
				var filepath string
				switch data[1][0] {
				case '/':
					if runtime.GOOS == "windows" {
						filepath = strings.Trim(data[1], `/`)
					} else {
						filepath = data[1]
					}
				default:
					if runtime.GOOS == "windows" {
						filepath = fmt.Sprintf(`//%s`, data[1])
					} else {
						filepath = data[1]
					}
				}

				// Check if running under WSL and convert path if necessary
				if editor.opts.Wsl != nil {
					filepath = convertWindowsToUnixPath(filepath)
				}

				// if message grid is active and drop the file in message area,
				// then, we put the filepath string into message area.
				if w.isMsgGrid && w.s.ws.cursor.gridid == w.grid {
					w.s.ws.nvim.FeedKeys(
						filepath,
						"t",
						false,
					)
				} else {
					if editor.config.Editor.ShowDiffDialogOnDrop && bufName != "" {
						w.s.howToOpen(filepath)
					} else {
						fileOpenInBuf(filepath)
					}
				}
			default:
			}
		}
	}
}

func (w *Window) focusGrid() {
	// If the window at the mouse pointer is not the current window
	if w.grid != w.s.ws.cursor.gridid {
		done := make(chan bool, 2)
		go func() {
			_ = w.s.ws.nvim.SetCurrentWindow(w.id)
			done <- true
		}()

		select {
		case <-done:
		case <-time.After(NVIMCALLTIMEOUT * time.Millisecond):
		}
	}
}

// convertWindowsToUnixPath converts a Windows path to a Unix path as wslpath does.
func convertWindowsToUnixPath(winPath string) string {
	unixPath := strings.ReplaceAll(winPath, `\`, `/`)
	if len(unixPath) <= 2 {
		return unixPath
	}
	// Convert drive letter to /mnt/<drive-letter>
	if unixPath[1] == ':' {
		driveLetter := strings.ToLower(string(unixPath[0]))
		unixPath = fmt.Sprintf("/mnt/%s%s", driveLetter, unixPath[2:])
	}
	return unixPath
}

func (w *Window) isShown() bool {
	if w == nil {
		return false
	}

	return w.IsVisible()
}

func (w *Window) raise() {
	if w.grid == 1 {
		return
	}

	if !w.isFloatWin && !w.isExternal {
		w.Raise()
	}
	w.s.setTopLevelGrid(w.grid)

	// Float windows are re-stacked according to the "z-index" and generation order.
	var floatWins []*Window
	w.s.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)
		if win == nil {
			return true
		}
		if win.isFloatWin && !win.isExternal {
			floatWins = append(floatWins, win)
		}

		return true
	})
	sort.Slice(
		floatWins,
		func(i, j int) bool {
			if floatWins[i].zindex.value < floatWins[j].zindex.value {
				return true
			} else if floatWins[i].zindex.value == floatWins[j].zindex.value {
				if floatWins[i].zindex.order < floatWins[j].zindex.order {
					return true
				}

			}
			return false
		},
	)

	// For each window object, set the pointer of closest window in the z-order
	// that covers the target window on the region.
	for i := 0; i < len(floatWins); i++ {
		floatWins[i].Raise()
		if i > 0 {
			if floatWins[i].isMsgGrid {
				continue
			}
			for j := i - 1; j >= 0; j-- {
				if floatWins[j].isMsgGrid {
					continue
				}
				if floatWins[j].Geometry().Contains2(floatWins[i].Geometry(), false) {
					floatWins[i].zindex.nearestLowerZOrderWindow = floatWins[j]
				}
			}
		}
	}

	// handle cursor widget
	w.setUIParent()
}

func (w *Window) setUIParent() {
	// Update cursor font
	w.s.ws.cursor.updateFont(w, w.getFont(), w.getFallbackFonts())
	defer func() {
		w.s.ws.cursor.isInPalette = false
	}()

	// // ws := editor.workspaces[editor.active]
	// prevCursorWin, ok := w.s.ws.screen.getWindow(w.s.ws.cursor.prevGridid)

	// for handling external window
	if !w.isExternal {

		// Suppress window activation when font selection dialog is displayed
		if w.s.ws.font != nil {
			if w.s.ws.fontdialog != nil {
				if !w.s.ws.fontdialog.IsVisible() {
					editor.window.Raise()
				}
			}
		}

		w.s.ws.cursor.SetParent(w.s.widget)
		if editor.config.Editor.ExtCmdline {
			if w.s.ws.palette != nil {
				w.s.ws.palette.setParent(w)
				w.s.ws.palette.resize()
			}
		}

		// if ok {
		// 	if prevCursorWin.isExternal {
		// 		w.s.ws.cursor.raise()
		// 	}
		// }
		// if w.s.ws.cursor.isInPalette {
		// 	w.s.ws.cursor.raise()
		// }
	} else if w.isExternal {
		w.extwin.Raise()
		w.s.ws.cursor.SetParent(w.extwin)
		if editor.config.Editor.ExtCmdline {
			if w.s.ws.palette != nil {
				w.s.ws.palette.setParent(w)
				w.s.ws.palette.resize()
			}
		}

		// if ok {
		// 	if !prevCursorWin.isExternal {
		// 		w.s.ws.cursor.raise()
		// 	}
		// }
		// if w.s.ws.cursor.isInPalette {
		// 	w.s.ws.cursor.raise()
		// }
	}

	w.s.ws.cursor.raise()
}

func (w *Window) show() {
	w.fill()
	w.Show()

	// set buffer local ts value
	if w.s.ws.ts != w.ts {
		w.s.ws.optionsetMutex.Lock()
		w.s.ws.ts = w.ts
		w.s.ws.optionsetMutex.Unlock()
	}
}

func (w *Window) hide() {
	w.Hide()
}

func (w *Window) setGridGeometry(width, height int) {
	if w.isExternal && !w.extwinResized {
		w.extwin.Resize2(width+EXTWINBORDERSIZE*2, height+EXTWINBORDERSIZE*2)
	}
	w.extwinResized = false

	rect := core.NewQRect4(0, 0, width, height)
	w.SetGeometry(rect)
	w.fill()
}

// refreshUpdateArea:: arg:0 => full, arg:1 => full only text
func (w *Window) refreshUpdateArea(fullmode int) {
	var boundary int
	if fullmode == 0 {
		boundary = w.cols
	} else {
		boundary = w.maxLenContent
	}
	for i := 0; i < len(w.lenContent); i++ {
		w.countContent(i)
		w.lenContent[i] = boundary
		for j, _ := range w.contentMask[i] {
			w.contentMask[i][j] = true
		}
	}
}

func (w *Window) fill() {
	w.refreshUpdateArea(0)

	setAutoFill := true
	if editor.config.Editor.EnableBackgroundBlur ||
		editor.config.Editor.Transparent < 1.0 {
		if !w.isExternal {
			setAutoFill = false
		}
	} else {
		if w.isMsgGrid && editor.config.Message.Transparent < 1.0 {
			setAutoFill = false
		} else if w.isPopupmenu && w.s.ws.pb > 0 {
			setAutoFill = false
		} else if w.isFloatWin && w.wb > 0 {
			setAutoFill = false
		}
	}

	if !setAutoFill {
		return
	}

	if w.background != nil {
		w.SetAutoFillBackground(true)
		p := gui.NewQPalette()
		p.SetColor2(gui.QPalette__Background, w.background.QColor())
		w.SetPalette(p)
	}
}

func (w *Window) setShadow() {
	if !editor.config.Editor.DrawShadowForFloatWindow {
		return
	}
	w.SetGraphicsEffect(util.DropShadow(0, 25, 125, 110))
}

func (w *Window) position() (int, int) {
	pos := w.Pos()
	posx := int(float64(pos.X()) / w.s.font.cellwidth)
	posy := int(float64(pos.Y()) / float64(w.s.font.lineHeight))

	return posx, posy
}

func (w *Window) move(col int, row int, anchorwindow ...*Window) {
	font := w.s.font
	var anchorwin *Window
	if len(anchorwindow) > 0 {
		anchorwin = anchorwindow[0]
		if anchorwin != nil {
			font = anchorwin.getFont()
		}
	}

	x := int(float64(col) * font.cellwidth)
	y := (row * font.lineHeight)

	// Fix https://github.com/akiyosi/goneovim/issues/316#issuecomment-1039978355
	// Adjustment of the float window position when the repositioning process
	// is being applied to the anchor window when it is outside the application window.
	var anchorposx, anchorposy int
	if anchorwin != nil {
		if anchorwin.grid != w.grid {
			anchorposx = anchorwin.Pos().X()
			anchorposy = anchorwin.Pos().Y()
		}
	}

	if w.isFloatWin && !w.isMsgGrid {
		// A workaround for ext_popupmenu and displaying a LSP tooltip
		if editor.config.Editor.ExtPopupmenu {
			if w.s.ws.mode == "insert" && w.s.ws.popup.widget.IsVisible() {
				if w.s.ws.popup.widget.IsVisible() {
					w.SetGraphicsEffect(util.DropShadow(0, 25, 125, 110))
					w.Move2(
						w.s.ws.popup.widget.X()+w.s.ws.popup.widget.Width()+5,
						w.s.ws.popup.widget.Y(),
					)
					w.raise()
				}

				return
			}
		}

		// There is currently no reliable way to identify cursor-based floating
		// window. All completion plugins set `relative` to `"editor"` in the
		// window config (and not to `"cursor"`).
		// Related issue: https://github.com/neovim/neovim/issues/34595
		// For now, it seems that best workaround is to make the position of
		// every editor/cursor-relative floating window relative to the actual
		// content of the grid. This will place completion window correctly.
		// As a consequence, centered floating window won't be centered anymore,
		// but it's a minor issue since text itself doesn't occupy (as now) the
		// whole window (because of text wrapping).
		winwithcontent, ok := w.s.getWindow(w.s.ws.cursor.gridid)
		if ok && winwithcontent.getFont().proportional {
			config, err := w.s.ws.nvim.WindowConfig(w.id)
			if err == nil && anchorwin != nil && config != nil &&
				config.Relative == "cursor" {
				contentRow := anchorwin.s.ws.cursor.row
				x = int(winwithcontent.getSinglePixelX(contentRow, col))
				y = winwithcontent.getFont().lineHeight * row // TESTING
			}
		}

		// #316
		// Adjust the position of the floating window to the inside of the screen
		// when it is outside of the screen.
		x, y = w.repositioningFloatwindow([2]int{anchorposx + x, anchorposy + y})
	}
	if w.isExternal {
		w.Move2(EXTWINBORDERSIZE, EXTWINBORDERSIZE)
		w.layoutExternalWindow(x, y)

		return
	}

	w.Move2(x, y)
}

func (w *Window) setFloatWindowPosition() {
	wincols := int(float64(w.cols) * w.getFont().cellwidth / w.anchorwin.getFont().cellwidth)
	winrows := int(math.Ceil(float64(w.rows*w.getFont().lineHeight) / float64(w.anchorwin.getFont().lineHeight)))

	var col, row int
	switch w.anchor {
	case "NW":
		col = w.anchorCol
		row = w.anchorRow
	case "NE":
		col = w.anchorCol - wincols
		row = w.anchorRow
	case "SW":
		col = w.anchorCol
		row = w.anchorRow - winrows

	case "SE":
		col = w.anchorCol - w.cols
		row = w.anchorRow - w.rows
	}

	w.updateMutex.Lock()
	w.pos = [2]int{col, row}
	w.updateMutex.Unlock()
}

func (w *Window) setOptions() {
	if w.s == nil || w.s.ws == nil {
		return
	}
	if w.id == 0 {
		return
	}

	if editor.config.Editor.IndentGuide {
		// get tabstop
		w.ts = util.ReflectToInt(w.s.ws.getBufferOption(NVIMCALLTIMEOUT, "ts", w.id))

		if w.ft == "" {
			// get filetype
			ftITF := w.s.ws.getBufferOption(NVIMCALLTIMEOUT, "ft", w.id)
			ft, ok := ftITF.(string)
			if ok {
				w.ft = ft
			}
		}
	}
}

func (w *Window) repositioningFloatwindow(pos ...[2]int) (int, int) {
	baseFont := w.s.ws.screen.font

	var winx, winy int
	if len(pos) > 0 {
		winx = pos[0][0]
		winy = pos[0][1]
	} else {
		winx = w.Pos().X()
		winy = w.Pos().Y()
	}

	if w.isMsgGrid {
		return winx, winy
	}

	width := w.Width()
	height := w.Height()
	screenWidth := w.s.widget.Width()
	screenHeight := w.s.widget.Height()

	if float64((winx+width)-screenWidth) >= baseFont.cellwidth {
		winx -= winx + width - screenWidth
	}
	if (winy+height)-screenHeight >= baseFont.lineHeight && !w.isPopupmenu {
		winy -= winy + height - screenHeight
	}

	// If the position coordinate is a negative value, it is reset to zero.
	if winx < 0 {
		winx = 0
	}
	if winy < 0 && !w.isPopupmenu {
		winy = 0
	}

	return winx, winy
}

func (w *Window) layoutExternalWindow(x, y int) {
	font := w.s.font

	// float windows width, height
	width := int(float64(w.cols) * font.cellwidth)
	height := w.rows * font.lineHeight
	dx := []int{}
	dy := []int{}

	// layout external windows
	// Adjacent to each other through edges of the same length as much as possible.
	if w.pos[0] == 0 && w.pos[1] == 0 && !w.extwinManualResized {
		w.s.windows.Range(func(_, winITF interface{}) bool {
			win := winITF.(*Window)
			if win == nil {
				return true
			}
			if win.grid == 1 {
				return true
			}
			if win.isMsgGrid {
				return true
			}
			if win.grid == w.grid {
				return true
			}
			if win.isExternal {

				dc := 0
				dr := 0
				for _, e := range dx {
					dc += e
				}
				for _, e := range dy {
					dr += e
				}

				winx := 0
				winy := 0
				for _, e := range win.extwinAutoLayoutPosX {
					winx += e
				}
				for _, e := range win.extwinAutoLayoutPosY {
					winy += e
				}

				if winx <= w.pos[0]+dc && winx+win.cols > w.pos[0]+dc &&
					winy <= w.pos[1]+dr && winy+win.rows > w.pos[1]+dr {

					widthRatio := float64(w.cols+win.cols) * font.cellwidth / float64(editor.window.Width())
					heightRatio := float64((w.rows+win.rows)*font.lineHeight) / float64(editor.window.Height())
					if w.cols == win.cols {
						dy = append(dy, win.rows)
						height += win.rows*font.lineHeight + EXTWINBORDERSIZE*2 + EXTWINMARGINSIZE
					} else if w.rows == win.rows {
						dx = append(dx, win.cols)
						width += int(float64(win.cols)*font.cellwidth) + EXTWINBORDERSIZE*2 + EXTWINMARGINSIZE
					} else {
						if widthRatio > heightRatio {
							dy = append(dy, win.rows)
							height += win.rows*font.lineHeight + EXTWINBORDERSIZE*2 + EXTWINMARGINSIZE
						} else {
							dx = append(dx, win.cols)
							width += int(float64(win.cols)*font.cellwidth) + EXTWINBORDERSIZE*2 + EXTWINMARGINSIZE
						}
					}

					return true
				}

			}

			return true
		})
		x := 0
		y := 0
		for _, e := range dx {
			x += int(float64(e)*font.cellwidth) + EXTWINBORDERSIZE*2 + EXTWINMARGINSIZE
		}
		for _, e := range dy {
			y += e*font.lineHeight + EXTWINBORDERSIZE*2 + EXTWINMARGINSIZE
		}
		w.extwinAutoLayoutPosX = dx
		w.extwinAutoLayoutPosY = dy
		w.extwinRelativePos = [2]int{editor.window.Pos().X() + x, editor.window.Pos().Y() + y}
	}

	w.extwin.Move2(editor.window.Pos().X()+x, editor.window.Pos().Y()+y)

	// centering
	if w.pos[0] == 0 && w.pos[1] == 0 && !w.extwinManualResized {
		var newx, newy int
		if editor.window.Width()-width > 0 {
			newx = int(float64(editor.window.Width()-width)/2.0) - (EXTWINMARGINSIZE / 2)
		}
		if editor.window.Height()-height > 0 {
			newy = int(float64(editor.window.Height()-height)/2.0) - (EXTWINMARGINSIZE / 2)
		}

		w.s.windows.Range(func(_, winITF interface{}) bool {
			win := winITF.(*Window)
			if win == nil {
				return true
			}
			if win.grid == 1 {
				return true
			}
			if win.isMsgGrid {
				return true
			}
			if win.isExternal && !win.extwinManualResized {
				win.extwin.Move2(win.extwinRelativePos[0]+newx, win.extwinRelativePos[1]+newy)
			}

			return true
		})

	}

}

func (w *Window) combinePixmap(p1, p2 *gui.QPixmap, overlapping float64) *gui.QPixmap {
	newHeight := float64(p1.Height()+p2.Height()) - overlapping

	newpixmap := gui.NewQPixmap2(
		core.NewQSize2(
			int(w.devicePixelRatio*float64(p1.Width())),
			int(w.devicePixelRatio*math.Ceil(newHeight)),
		),
	)
	newpixmap.SetDevicePixelRatio(w.devicePixelRatio)
	newpixmap.Fill(newRGBA(0, 0, 0, 0).QColor())

	p := gui.NewQPainter2(newpixmap)
	p.DrawPixmap9(
		0,
		0,
		p1,
	)
	p.DrawPixmap9(
		0,
		int(float64(p1.Height())-overlapping),
		p2,
	)
	p.End()

	return newpixmap

}
