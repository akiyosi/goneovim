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
	"unicode"
	"unsafe"

	"github.com/akiyosi/goneovim/util"
	"github.com/bluele/gcache"
	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

const (
	EXTWINBORDERSIZE = 5
	EXTWINMARGINSIZE = 10
)

type gridId = int

// Highlight is
type Highlight struct {
	special       *RGBA
	foreground    *RGBA
	background    *RGBA
	kind          string
	uiName        string
	hlName        string
	blend         int
	id            int
	reverse       bool
	bold          bool
	underline     bool
	undercurl     bool
	italic        bool
	strikethrough bool
}

// HlChars is used in screen cache
type HlChars struct {
	fg     *RGBA
	text   string
	italic bool
	bold   bool
}

// HlDecoration is used in screen cache
type HlDecoration struct {
	fg            *RGBA
	underline     bool
	undercurl     bool
	strikethrough bool
}

// Cell is
type Cell struct {
	highlight   *Highlight
	char        string
	normalWidth bool
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
}

// Window is
type Window struct {
	fgCache Cache
	widgets.QWidget
	snapshot               *gui.QPixmap
	font                   *Font
	localWindows           *[4]localWindow
	extwin                 *ExternalWin
	background             *RGBA
	s                      *Screen
	anchorwin              *Window
	cwd                    string
	ft                     string
	anchor                 string
	lenOldContent          []int
	lenContent             []int
	scrollRegion           []int
	contentMaskOld         [][]bool
	contentMask            [][]bool
	content                [][]*Cell
	extwinAutoLayoutPosY   []int
	lenLine                []int
	extwinAutoLayoutPosX   []int
	scrollViewport         [2][5]int
	queueRedrawArea        [4]int
	extwinRelativePos      [2]int
	pos                    [2]int
	scrollPixels           [2]int
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
	scrollPixelsDeltaY     int
	id                     nvim.Window
	scrollCols             int
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
	doGetSnapshot          bool
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

func newCache() Cache {
	g := gcache.New(editor.config.Editor.CacheSize).LRU().
		EvictedFunc(purgeQimage).
		PurgeVisitorFunc(purgeQimage).
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
	w.paintMutex.Lock()
	w.snapshot.DestroyQPixmap()
	w.snapshot = nil
	w.paintMutex.Unlock()
}

func (w *Window) grabScreenSnapshot(rectangle core.QRect_ITF) {
	snapshot := w.Grab(rectangle)
	w.paintMutex.Lock()
	w.snapshot.DestroyQPixmap()
	w.snapshot = snapshot
	w.paintMutex.Unlock()
}

func (w *Window) paint(event *gui.QPaintEvent) {
	w.paintMutex.Lock()
	defer w.paintMutex.Unlock()

	p := gui.NewQPainter2(w)

	// Erase the snapshot used in the animation scroll
	if w.doErase {
		p.EraseRect3(w.Rect())
		p.DestroyQPainter()
		return
	}

	// Set RenderHint
	p.SetRenderHint(gui.QPainter__SmoothPixmapTransform, true)

	// Set font
	font := w.getFont()

	// Set devicePixelRatio if it is not set
	if w.devicePixelRatio == 0 {
		w.devicePixelRatio = float64(p.PaintEngine().PaintDevice().DevicePixelRatio())
	}

	rect := event.Rect()
	col := int(float64(rect.Left()) / font.cellwidth)
	row := int(float64(rect.Top()) / float64(font.lineHeight))
	cols := int(math.Ceil(float64(rect.Width()) / font.cellwidth))
	rows := int(math.Ceil(float64(rect.Height()) / float64(font.lineHeight)))

	// Draw contents
	for y := row; y < row+rows; y++ {
		if y >= w.rows {
			continue
		}
		w.drawBackground(p, y, col, cols)
		w.drawForeground(p, y, col, cols)
	}

	// Draw scroll snapshot
	// TODO: If there are wrapped lines in the viewport, the snapshot will be misaligned.
	w.drawScrollSnapshot(p)

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

	// Reset to 0 after drawing is complete.
	// This is to suppress flickering in smooth scroll
	dx := math.Abs(float64(w.scrollPixels[0]))
	dy := math.Abs(float64(w.scrollPixels[1]))
	if dx >= font.cellwidth {
		w.scrollPixels[0] = 0
	}
	if dy >= float64(font.lineHeight) {
		w.scrollPixels[1] = 0
	}

	if w.lastScrollphase == core.Qt__NoScrollPhase {
		w.lastScrollphase = core.Qt__ScrollEnd
	}

	p.DestroyQPainter()
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
	height := w.scrollCols * font.lineHeight
	snapshotPos := 0
	if w.scrollPixels2 > 0 {
		snapshotPos = w.scrollPixels2 - height
	} else if w.scrollPixels2 < 0 {
		snapshotPos = height + w.scrollPixels2
	}
	if w.scrollPixels2 != 0 {
		p.DrawPixmap9(
			0,
			snapshotPos,
			w.snapshot,
		)
	}
}

func (w *Window) getFont() *Font {
	if w.font == nil {
		return w.s.font
	}

	return w.font
}

func (w *Window) getTS() int {
	var ts int
	var ok bool
	if w.id != 0 {
		ts, ok = w.s.ws.windowsTs[w.id]
		if ok {
			return ts
		}
	}

	return w.ts
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
		w.s.ws.optionsetMutex.Lock()
		w.ft = w.s.ws.windowsFt[w.id]
		w.s.ws.optionsetMutex.Unlock()
	}
	if w.ft == "" {
		return
	}
	if !w.isShown() {
		return
	}

	ts := w.getTS()
	if ts <= 0 {
		return
	}

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
			if c.isSignColumn() {
				res++
			}
			if c.char != " " && !c.isSignColumn() {
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
						// TODO: We do not detect the wraped line when `:set nonu` setting.
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
	scrollPixels := 0
	if w.lastScrollphase != core.Qt__NoScrollPhase {
		scrollPixels = w.scrollPixels2
	}
	if editor.config.Editor.LineToScroll == 1 {
		scrollPixels += w.scrollPixels[1]
	}

	X := float64(x) * font.cellwidth
	Y := float64(y*font.lineHeight) + float64(scrollPixels)
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

	var v, h, vert, horiz int
	var action string

	editor.putLog("start wheel event")

	font := w.getFont()

	// Detect current mode
	mode := w.s.ws.mode
	isTmode := w.s.ws.terminalMode
	editor.putLog("detect neovim mode:", mode)
	editor.putLog("detect neovim terminal mode:", fmt.Sprintf("%v", isTmode))
	if isTmode {
		w.s.ws.nvim.Input(`<C-\><C-n>`)
	} else if mode != "normal" {
		w.s.ws.nvim.Input(w.s.ws.escKeyInInsert)
	}

	pixels := event.PixelDelta()
	if pixels != nil {
		v = pixels.Y()
		h = pixels.X()
	}
	phase := event.Phase()
	if w.lastScrollphase != phase && w.lastScrollphase != core.Qt__ScrollEnd {
		w.doGetSnapshot = true
	}
	w.lastScrollphase = phase
	isStopScroll := (w.lastScrollphase == core.Qt__ScrollEnd)

	// Smooth scroll with touchpad
	if (v == 0 || h == 0) && isStopScroll {
		vert, horiz = w.smoothUpdate(v, h, isStopScroll)
	} else if (v != 0 || h != 0) && phase != core.Qt__NoScrollPhase {
		// If Scrolling has ended, reset the displacement of the line
		vert, horiz = w.smoothUpdate(v, h, isStopScroll)
	} else {
		angles := event.AngleDelta()
		vert = angles.Y()
		horiz = angles.X()
		if event.Inverted() {
			vert = -1 * vert
		}
		// Scroll per 1 line
		if math.Abs(float64(vert)) > 1 {
			vert = vert / int(math.Abs(float64(vert)))
		}
		if math.Abs(float64(horiz)) > 1 {
			horiz = horiz / int(math.Abs(float64(horiz)))
		}
	}

	// Scroll acceleration
	accel := 1
	if math.Abs(float64(v)) > float64(font.lineHeight) {
		accel = int(math.Abs(float64(v)) * 2.5 / float64(font.lineHeight))
		if accel > 6 {
			accel = 6
		}
	}
	if accel == 0 {
		accel = 1
	}
	vert = vert * int(math.Sqrt(float64(accel)))

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
	if action == "" {
		if horiz > 0 {
			action = "left"
		} else if horiz < 0 {
			action = "right"
		}
	}

	// If the window at the mouse pointer is not the current window
	w.focusGrid()

	mod := editor.modPrefix(event.Modifiers())
	row := int(float64(event.X()) / font.cellwidth)
	col := int(float64(event.Y()) / float64(font.lineHeight))

	if w.s.ws.isMappingScrollKey {
		editor.putLog("detect a mapping to <C-e>, <C-y> keys.")
		if vert != 0 {
			w.s.ws.nvim.InputMouse("wheel", action, mod, w.grid, row, col)
		}
	} else {
		if editor.config.Editor.ReversingScrollDirection {
			if vert > 0 {
				go w.s.ws.nvim.Input(fmt.Sprintf("%v<C-e>", editor.config.Editor.LineToScroll*int(math.Abs(float64(vert)))))
			} else if vert < 0 {
				go w.s.ws.nvim.Input(fmt.Sprintf("%v<C-y>", editor.config.Editor.LineToScroll*int(math.Abs(float64(vert)))))
			}
		} else {
			if vert > 0 {
				go w.s.ws.nvim.Input(fmt.Sprintf("%v<C-y>", editor.config.Editor.LineToScroll*int(math.Abs(float64(vert)))))
			} else if vert < 0 {
				go w.s.ws.nvim.Input(fmt.Sprintf("%v<C-e>", editor.config.Editor.LineToScroll*int(math.Abs(float64(vert)))))
			}
		}
	}

	if editor.config.Editor.DisableHorizontalScroll {
		return
	}
	if vert != 0 {
		return
	}

	if horiz != 0 {
		w.s.ws.nvim.InputMouse("wheel", action, mod, w.grid, row, col)
	}

	event.Accept()
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
		case <-time.After(40 * time.Millisecond):
		}
	}
}

// screen smooth update with touchpad
func (w *Window) smoothUpdate(v, h int, isStopScroll bool) (int, int) {
	var vert, horiz int
	font := w.getFont()

	if isStopScroll {
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
	}
	// if v < 0 && w.scrollPixels[1] > 0 {
	// 	w.scrollPixels[1] = 0
	// }

	dx := math.Abs(float64(w.scrollPixels[0]))
	dy := math.Abs(float64(w.scrollPixels[1]))

	if dx < font.cellwidth {
		w.scrollPixels[0] += h
	}
	if dy < float64(font.lineHeight) {
		w.scrollPixels[1] += v
	}

	dx = math.Abs(float64(w.scrollPixels[0]))
	dy = math.Abs(float64(w.scrollPixels[1]))

	if dx >= font.cellwidth {
		horiz = int(float64(w.scrollPixels[0]) / font.cellwidth)
	}
	if dy >= float64(font.lineHeight) {
		vert = int(float64(w.scrollPixels[1]) / float64(font.lineHeight))
		// NOTE: Reset to 0 after paint event is complete.
		//       This is to suppress flickering.
	}

	// w.update()
	// w.s.ws.cursor.update()
	if !(dx >= font.cellwidth || dy > float64(font.lineHeight)) {
		w.update()
		w.s.ws.cursor.update()
	}

	return vert, horiz
}

// smoothscroll makes Neovim's scroll command behavior smooth and animated.
func (win *Window) smoothScroll(diff int) {
	// process smooth scroll
	a := core.NewQPropertyAnimation2(win, core.NewQByteArray2("scrollDiff", len("scrollDiff")), win)
	a.ConnectValueChanged(func(value *core.QVariant) {
		ok := false
		v := value.ToDouble(&ok)
		if !ok {
			return
		}
		font := win.getFont()
		win.scrollPixels2 = int(float64(diff) * v * float64(font.lineHeight))
		win.Update2(
			0,
			0,
			int(float64(win.cols)*font.cellwidth),
			win.rows*font.lineHeight,
		)
		if win.scrollPixels2 == 0 {
			win.doErase = true
			win.Update2(
				0,
				0,
				int(float64(win.cols)*font.cellwidth),
				win.cols*font.lineHeight,
			)
			win.doErase = false
			win.fill()

			// get snapshot
			if !editor.isKeyAutoRepeating && editor.config.Editor.SmoothScroll {
				win.grabScreenSnapshot(win.Rect())
			}
		}
	})
	a.SetDuration(editor.config.Editor.SmoothScrollDuration)
	a.SetStartValue(core.NewQVariant10(1))
	a.SetEndValue(core.NewQVariant10(0))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutQuart))
	a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutExpo))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutCirc))
	a.Start(core.QAbstractAnimation__DeletionPolicy(core.QAbstractAnimation__DeleteWhenStopped))
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
	if win.scrollPixels[1] != 0 {
		win.scrollPixels[1] = 0
	}

	win.updateLine(row, colStart, cells)
	win.countContent(row)

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

	if win.isMsgGrid {
		return
	}
	if win.grid == 1 && win.s.name != "minimap" {
		return
	}
	if win.maxLenContent < win.lenContent[row] {
		win.maxLenContent = win.lenContent[row]
	}
}

func (w *Window) updateLine(row, col int, cells []interface{}) {
	w.updateMutex.Lock()
	line := w.content[row]
	colStart := col
	for _, arg := range cells {
		if col >= len(line) {
			continue
		}
		cell := arg.([]interface{})

		var hl, repeat int
		hl = -1
		if len(cell) >= 2 {
			hl = util.ReflectToInt(cell[1])
		}

		if len(cell) == 3 {
			repeat = util.ReflectToInt(cell[2])
		}

		// If `repeat` is present, the cell should be
		// repeated `repeat` times (including the first time), otherwise just
		// once.
		r := 1
		if repeat == 0 {
			repeat = 1
		}
		for r <= repeat {
			if col >= len(line) {
				break
			}

			if line[col] == nil {
				line[col] = &Cell{}
				w.contentMask[row][col] = true
			}

			line[col].char = cell[0].(string)
			line[col].normalWidth = w.isNormalWidth(line[col].char)

			// If `hl_id` is not present the most recently seen `hl_id` in
			//	the same call should be used (it is always sent for the first
			//	cell in the event).
			switch col {
			case 0:
				line[col].highlight = w.s.hlAttrDef[hl]
			default:
				if hl == -1 {
					line[col].highlight = line[col-1].highlight
				} else {
					line[col].highlight = w.s.hlAttrDef[hl]
				}
			}

			// Update contentmask for screen update
			if line[col].char == " " &&
				line[col].highlight.bg().equals(w.background) &&
				!line[col].highlight.underline &&
				!line[col].highlight.undercurl &&
				!line[col].highlight.strikethrough {
				w.contentMask[row][col] = false
			} else {
				w.contentMask[row][col] = true
			}

			// Detect popupmenu
			if line[col].highlight.uiName == "Pmenu" ||
				line[col].highlight.uiName == "PmenuSel" ||
				line[col].highlight.uiName == "PmenuSbar" {
				if !w.isPopupmenu {
					w.isPopupmenu = true
					w.move(w.pos[0], w.pos[1], w.anchorwin)
				}
			}

			// Detect winblend
			if line[col].highlight.blend > 0 {
				w.wb = line[col].highlight.blend
			}

			col++
			r++
		}
	}
	w.updateMutex.Unlock()

	w.queueRedraw(colStart, row, col-colStart+1, 1)
}

func (w *Window) countContent(row int) {
	line := w.content[row]
	lenLine := w.cols - 1
	width := w.cols - 1
	var breakFlag [2]bool
	for j := w.cols - 1; j >= 0; j-- {
		cell := line[j]

		if !breakFlag[0] {
			if cell == nil {
				lenLine--
			} else if cell.char == " " {
				lenLine--
			} else {
				breakFlag[0] = true
			}
		}

		if !breakFlag[1] {
			if cell == nil {
				width--
			} else if cell.char == " " &&
				cell.highlight.bg().equals(w.background) &&
				!cell.highlight.underline &&
				!cell.highlight.undercurl &&
				!cell.highlight.strikethrough {
				width--
			} else {
				breakFlag[1] = true
			}
		}

		if breakFlag[0] && breakFlag[1] {
			break
		}
	}
	lenLine++
	width++

	w.lenLine[row] = lenLine
	w.lenContent[row] = width
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

		if c.char != " " && !c.isSignColumn() {
			break
		} else {
			count++
		}
	}
	return count, nil
}

func (c *Cell) isSignColumn() bool {
	switch c.highlight.hlName {
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
			w.content = w.content[count:]
			w.content = append(w.content, content...)
			w.contentMask = w.contentMask[count:]
			w.contentMask = append(w.contentMask, contentMask...)

			w.lenLine = w.lenLine[count:]
			w.lenLine = append(w.lenLine, lenLine...)
			w.lenContent = w.lenContent[count:]
			w.lenContent = append(w.lenContent, lenContent...)
		}
		if count < 0 {
			w.content = w.content[:w.rows+count]
			w.content = append(content, w.content...)
			w.contentMask = w.contentMask[:w.rows+count]
			w.contentMask = append(contentMask, w.contentMask...)

			w.lenLine = w.lenLine[:w.rows+count]
			w.lenLine = append(lenLine, w.lenLine...)
			w.lenContent = w.lenContent[:w.rows+count]
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

	// copy(w.content[row], w.content[row+count])
	// copy(w.contentMask[row], w.contentMask[row+count])
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
	start := w.queueRedrawArea[1]
	end := w.queueRedrawArea[3]
	// Update all lines when using the wheel scroll or indent guide feature.
	if w.scrollPixels[1] != 0 || editor.config.Editor.IndentGuide || w.s.name == "minimap" {
		start = 0
		end = w.rows
	}

	for i := start; i < end; i++ {

		if len(w.content) <= i {
			continue
		}

		width := w.lenContent[i]

		if width < w.lenOldContent[i] {
			width = w.lenOldContent[i]
		}
		w.lenOldContent[i] = w.lenContent[i]

		drawWithSingleRect := false

		// If DrawIndentGuide is enabled
		if editor.config.Editor.IndentGuide {
			if i < w.rows-1 {
				if width < w.lenContent[i+1] {
					width = w.lenContent[i+1]
				}
			}
			drawWithSingleRect = true
		}
		// If screen is minimap
		if w.s.name == "minimap" {
			width = w.maxLenContent
			drawWithSingleRect = true
		}
		// If scroll is smooth with touchpad
		if w.scrollPixels[1] != 0 {
			width = w.maxLenContent
			drawWithSingleRect = true
		}
		// If scroll is smooth
		if editor.config.Editor.SmoothScroll {
			if w.scrollPixels2 != 0 {
				width = w.maxLenContent
				drawWithSingleRect = true
			}
		}
		width++

		// Create rectangles that require updating.
		var rects [][4]int
		isCreateRect := false
		extendedDrawingArea := int(font.italicWidth - font.cellwidth + 1)

		start := 0
		if drawWithSingleRect {
			rect := [4]int{
				0,
				i * font.lineHeight,
				int(math.Ceil(float64(width)*font.cellwidth)) + extendedDrawingArea,
				font.lineHeight,
			}
			rects = append(rects, rect)
			for j, _ := range w.contentMask[i] {
				w.contentMaskOld[i][j] = w.contentMask[i][j]
			}
		} else {
			for j, cm := range w.contentMask[i] {
				mask := cm || w.contentMaskOld[i][j]
				// Starting point for creating a rectangular area
				if mask && !isCreateRect {
					start = j
					isCreateRect = true
				}
				// Judgment point for end of rectangular area creation
				if (!mask && isCreateRect) || (j >= len(w.contentMask[i])-1 && isCreateRect) {
					// If the next rectangular area will be created with only one cell separating it, merge it.
					if j+1 <= len(w.contentMask[i])-1 {
						if w.contentMask[i][j+1] {
							continue
						}
					}

					jj := j

					// If it reaches the edge of the grid
					if j >= len(w.contentMask[i])-1 && isCreateRect {
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
						i * font.lineHeight,
						int(math.Ceil(float64(jj-start)*font.cellwidth)) + extendedDrawingArea, // update a slightly larger area.
						font.lineHeight,
					}
					rects = append(rects, rect)
					isCreateRect = false
				}
				w.contentMaskOld[i][j] = w.contentMask[i][j]
			}
		}

		// Request screen refresh for each rectangle region.
		if len(rects) == 0 {
			w.Update2(
				0,
				i*font.lineHeight,
				int(float64(width)*font.cellwidth),
				font.lineHeight,
			)
		} else {
			for _, rect := range rects {
				w.Update2(
					rect[0],
					rect[1],
					rect[2],
					rect[3],
				)
			}
		}
	}

	// reset redraw area
	w.queueRedrawArea[0] = w.cols
	w.queueRedrawArea[1] = w.rows
	w.queueRedrawArea[2] = 0
	w.queueRedrawArea[3] = 0

	w.redrawMutex.Unlock()
}

func (w *Window) queueRedrawAll() {
	w.queueRedrawArea = [4]int{0, 0, w.cols, w.rows}
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

func (w *Window) drawBackground(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}

	line := w.content[y]
	var bg *RGBA

	// draw default background color if window is float window or msg grid
	isDrawDefaultBg := false
	if w.isFloatWin || w.isMsgGrid {
		// If transparent is true, then we should draw every cell's background color
		if editor.config.Editor.Transparent < 1.0 || editor.config.Message.Transparent < 1.0 {
			w.SetAutoFillBackground(false)
			isDrawDefaultBg = true
		}

		// If the window is popupmenu and  pumblend is set
		if w.isPopupmenu && w.s.ws.pb > 0 {
			w.SetAutoFillBackground(false)
			isDrawDefaultBg = true
		}

		// If the window is float window and winblend is set
		if w.isFloatWin && !w.isPopupmenu && w.wb > 0 {
			w.SetAutoFillBackground(false)
			isDrawDefaultBg = true
		}
	}

	// Set smooth scroll offset
	scrollPixels := 0
	if w.lastScrollphase != core.Qt__NoScrollPhase {
		scrollPixels = w.scrollPixels2
	}
	if editor.config.Editor.LineToScroll == 1 {
		scrollPixels += w.scrollPixels[1]
	}

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

	// The same color combines the rectangular areas and paints at once
	var start, end int
	var lastBg *RGBA
	var lastHighlight, highlight *Highlight

	for x := col; x <= col+cols; x++ {

		if x >= len(line)+1 {
			continue
		}

		if x < len(line) {
			if line[x] == nil {
				highlight = w.s.hlAttrDef[0]
			} else {
				highlight = line[x].highlight
			}
		} else {
			highlight = w.s.hlAttrDef[0]
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
			if !lastBg.equals(bg) || x == bounds {
				w.fillCellRect(p, lastHighlight, lastBg, y, start, end, scrollPixels, isDrawDefaultBg)

				start = x
				end = x
				lastBg = bg
				lastHighlight = highlight

				if x == bounds {
					w.fillCellRect(p, lastHighlight, lastBg, y, start, end, scrollPixels, isDrawDefaultBg)
				}
			}
		}
	}
}

func (w *Window) fillCellRect(p *gui.QPainter, lastHighlight *Highlight, lastBg *RGBA, y, start, end, scrollPixels int, isDrawDefaultBg bool) {

	if lastHighlight == nil {
		return
	}

	width := end - start + 1
	if width < 0 {
		width = 0
	}
	if !isDrawDefaultBg && lastBg.equals(w.background) {
		width = 0
	}

	font := w.getFont()
	if width > 0 {
		// Set diff pattern
		pattern, color, transparent := w.getFillpatternAndTransparent(lastHighlight)

		// Fill background with pattern
		rectF := core.NewQRectF4(
			float64(start)*font.cellwidth,
			float64((y)*font.lineHeight+scrollPixels),
			float64(width)*font.cellwidth,
			float64(font.lineHeight),
		)
		p.FillRect(
			rectF,
			gui.NewQBrush3(
				gui.NewQColor3(
					color.R,
					color.G,
					color.B,
					transparent,
				),
				pattern,
			),
		)
	}
}

func (w *Window) drawText(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	wsfont := w.getFont()

	if !editor.config.Editor.CachedDrawing {
		p.SetFont(wsfont.fontNew)
	}

	line := w.content[y]
	chars := map[*Highlight][]int{}
	specialChars := []int{}

	// Set smooth scroll offset
	scrollPixels := 0
	if w.lastScrollphase != core.Qt__NoScrollPhase {
		scrollPixels = w.scrollPixels2
	}
	if editor.config.Editor.LineToScroll == 1 {
		scrollPixels += w.scrollPixels[1]
	}

	cellBasedDrawing := editor.config.Editor.DisableLigatures || (editor.config.Editor.Letterspace > 0)

	// pointX := float64(col) * wsfont.cellwidth
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
		if !line[x].normalWidth {
			specialChars = append(specialChars, x)
			continue
		}

		// If the ligature setting is disabled,
		// we will draw the characters on the screen one by one.
		if cellBasedDrawing {

			w.drawTextInPos(
				p,
				int(float64(x)*wsfont.cellwidth),
				y*wsfont.lineHeight+scrollPixels,
				line[x].char,
				line[x].highlight,
				false,
			)

		} else {
			// Prepare to draw a group of identical highlight units.
			highlight := line[x].highlight
			colorSlice, ok := chars[highlight]
			if !ok {
				colorSlice = []int{}
			}
			colorSlice = append(colorSlice, x)
			chars[highlight] = colorSlice
		}
	}

	// This is the normal rendering process for goneovim,
	// we draw a word snippet of the same highlight on the screen for each of the highlights.
	if !cellBasedDrawing {
		for highlight, colorSlice := range chars {
			var buffer bytes.Buffer
			slice := colorSlice

			isIndentationWhiteSpace := true
			pos := col
			for x := col; x <= col+cols; x++ {

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
						buffer.WriteString(line[x].char)
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
							int(float64(x-pos)*wsfont.cellwidth),
							y*wsfont.lineHeight+scrollPixels,
							buffer.String(),
							highlight,
							true,
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
		if !editor.config.Editor.CachedDrawing {
			if w.s.ws.fontwide != nil && w.font == nil && w.s.ws.fontwide.fontNew != nil {
				p.SetFont(w.s.ws.fontwide.fontNew)
			}
		}

		for _, x := range specialChars {
			if line[x] == nil || line[x].char == " " {
				continue
			}
			w.drawTextInPos(
				p,
				int(float64(x)*wsfont.cellwidth),
				y*wsfont.lineHeight+scrollPixels,
				line[x].char,
				line[x].highlight,
				false,
			)

		}
	}
}

func (w *Window) drawTextInPos(p *gui.QPainter, x, y int, text string, highlight *Highlight, isNormalWidth bool) {
	wsfont := w.getFont()
	// if CachedDrawing is disabled
	if !editor.config.Editor.CachedDrawing {
		w.drawTextInPosWithNoCache(
			p,
			x,
			y+wsfont.shift,
			text,
			highlight,
			isNormalWidth,
		)
	} else { // if CachedDrawing is enabled
		w.drawTextInPosWithCache(
			p,
			x,
			y,
			text,
			highlight,
			isNormalWidth,
		)
	}
}

func (w *Window) drawTextInPosWithNoCache(p *gui.QPainter, x, y int, text string, highlight *Highlight, isNormalWidth bool) {
	if text == "" {
		return
	}

	font := p.Font()
	fg := highlight.fg()
	p.SetPen2(fg.QColor())
	wsfont := w.getFont()

	if highlight.bold {
		font.SetWeight(wsfont.fontNew.Weight() + 25)
	} else {
		font.SetWeight(wsfont.fontNew.Weight())
	}
	if highlight.italic {
		font.SetItalic(true)
	} else {
		font.SetItalic(false)
	}
	// p.DrawText(point, text)
	p.DrawText3(x, y, text)
}

func (w *Window) drawTextInPosWithCache(p *gui.QPainter, x, y int, text string, highlight *Highlight, isNormalWidth bool) {
	if text == "" {
		return
	}

	fgCache := w.getCache()
	var image *gui.QImage
	imagev, err := fgCache.get(HlChars{
		text:   text,
		fg:     highlight.fg(),
		italic: highlight.italic,
		bold:   highlight.bold,
	})

	if err != nil {
		image = w.newTextCache(text, highlight, isNormalWidth)
		w.setTextCache(text, highlight, image)
	} else {
		image = imagev.(*gui.QImage)
	}

	// p.DrawImage7(
	// 	point,
	// 	image,
	// )
	p.DrawImage9(
		x, y,
		image,
		0, 0,
		-1, -1,
		core.Qt__AutoColor,
	)
}

func (w *Window) setDecorationCache(highlight *Highlight, image *gui.QImage) {
	if w.font != nil {
		// If window has own font setting
		w.fgCache.set(
			HlDecoration{
				fg:            highlight.fg(),
				underline:     highlight.underline,
				undercurl:     highlight.undercurl,
				strikethrough: highlight.strikethrough,
			},
			image,
		)
	} else {
		// screen text cache
		w.s.fgCache.set(
			HlDecoration{
				fg:            highlight.fg(),
				underline:     highlight.underline,
				undercurl:     highlight.undercurl,
				strikethrough: highlight.strikethrough,
			},
			image,
		)
	}
}

func (w *Window) newDecorationCache(char string, highlight *Highlight, isNormalWidth bool) *gui.QImage {
	font := w.getFont()

	width := font.cellwidth
	fg := highlight.fg()
	if !isNormalWidth {
		width = math.Ceil(w.s.runeTextWidth(font, char))
	}

	// Set smooth scroll offset
	scrollPixels := 0
	if w.lastScrollphase != core.Qt__NoScrollPhase {
		scrollPixels = w.scrollPixels2
	}
	if editor.config.Editor.LineToScroll == 1 {
		scrollPixels += w.scrollPixels[1]
	}

	// QImage default device pixel ratio is 1.0,
	// So we set the correct device pixel ratio
	image := gui.NewQImage2(
		core.NewQRectF4(
			0,
			0,
			w.devicePixelRatio*width,
			w.devicePixelRatio*float64(font.lineHeight),
		).Size().ToSize(),
		gui.QImage__Format_ARGB32_Premultiplied,
	)
	image.SetDevicePixelRatio(w.devicePixelRatio)
	image.Fill3(core.Qt__transparent)

	pi := gui.NewQPainter2(image)
	pi.SetPen2(fg.QColor())

	pen := gui.NewQPen()
	var color *gui.QColor
	sp := highlight.special
	if sp != nil {
		color = sp.QColor()
		pen.SetColor(color)
	} else {
		fg := highlight.foreground
		color = fg.QColor()
		pen.SetColor(color)
	}
	pi.SetPen(pen)
	start := float64(0) * font.cellwidth
	end := float64(width) * font.cellwidth

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
	if highlight.strikethrough {
		Y := float64(0*font.lineHeight+scrollPixels) + float64(font.ascent)*0.65 + float64(space2/2)
		pi.FillRect5(
			int(start),
			int(Y),
			int(math.Ceil(font.cellwidth)),
			weight,
			color,
		)
	}
	if highlight.underline {
		pi.FillRect5(
			int(start),
			int(float64((0+1)*font.lineHeight+scrollPixels))-weight,
			int(math.Ceil(font.cellwidth)),
			weight,
			color,
		)
	}
	if highlight.undercurl {
		amplitude := descent*0.65 + float64(space2)
		maxAmplitude := font.ascent / 8.0
		if amplitude >= maxAmplitude {
			amplitude = maxAmplitude
		}
		freq := 1.0
		phase := 0.0
		Y := float64(0*font.lineHeight+scrollPixels) + float64(font.ascent+descent*0.3) + float64(space2/2) + space
		Y2 := Y + amplitude*math.Sin(0)
		point := core.NewQPointF3(start, Y2)
		path := gui.NewQPainterPath2(point)
		for i := int(point.X()); i <= int(end); i++ {
			Y2 = Y + amplitude*math.Sin(2*math.Pi*freq*float64(i)/font.cellwidth+phase)
			path.LineTo(core.NewQPointF3(float64(i), Y2))
		}
		pi.DrawPath(path)
	}

	pi.DestroyQPainter()

	return image
}

func (w *Window) setTextCache(text string, highlight *Highlight, image *gui.QImage) {
	if w.font != nil {
		// If window has own font setting
		w.fgCache.set(
			HlChars{
				text:   text,
				fg:     highlight.fg(),
				italic: highlight.italic,
				bold:   highlight.bold,
			},
			image,
		)
	} else {
		// screen text cache
		w.s.fgCache.set(
			HlChars{
				text:   text,
				fg:     highlight.fg(),
				italic: highlight.italic,
				bold:   highlight.bold,
			},
			image,
		)
	}
}

func (w *Window) newTextCache(text string, highlight *Highlight, isNormalWidth bool) *gui.QImage {
	// * Ref: https://stackoverflow.com/questions/40458515/a-best-way-to-draw-a-lot-of-independent-characters-in-qt5/40476430#40476430
	editor.putLog("start creating word cache:", text)

	font := w.getFont()

	// Put debug log
	if editor.opts.Debug != "" {
		fi := gui.NewQFontInfo(font.fontNew)
		editor.putLog(
			"Outputs font information creating word cache:",
			fi.Family(),
			fi.PointSizeF(),
			fi.StyleName(),
			fmt.Sprintf("%v", fi.PointSizeF()),
		)
	}

	width := float64(len(text)) * font.italicWidth
	fg := highlight.fg()
	if !isNormalWidth {
		width = math.Ceil(w.s.runeTextWidth(font, text))
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
		int(w.devicePixelRatio*width),
		int(w.devicePixelRatio*float64(font.lineHeight)),
		gui.QImage__Format_ARGB32_Premultiplied,
	)
	image.SetDevicePixelRatio(w.devicePixelRatio)
	image.Fill3(core.Qt__transparent)

	pi := gui.NewQPainter2(image)
	pi.SetPen2(fg.QColor())

	if !isNormalWidth && w.font == nil && w.s.ws.fontwide != nil {
		pi.SetFont(w.s.ws.fontwide.fontNew)
	} else {
		pi.SetFont(font.fontNew)
	}

	if highlight.bold {
		// pi.Font().SetBold(true)
		pi.Font().SetWeight(font.fontNew.Weight() + 50)
	}
	if highlight.italic {
		pi.Font().SetItalic(true)
	}

	pi.DrawText6(
		core.NewQRectF4(
			0,
			0,
			width,
			float64(font.lineHeight),
		), text, gui.NewQTextOption2(core.Qt__AlignVCenter),
	)
	pi.DestroyQPainter()

	editor.putLog("finished creating word cache:", text)

	return image
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

	// Set smooth scroll offset
	scrollPixels := 0
	if w.lastScrollphase != core.Qt__NoScrollPhase {
		scrollPixels = w.scrollPixels2
	}
	if editor.config.Editor.LineToScroll == 1 {
		scrollPixels += w.scrollPixels[1]
	}

	for x := col; x <= col+cols; x++ {
		if x >= len(line) {
			continue
		}
		if line[x] == nil {
			continue
		}
		if !line[x].highlight.underline && !line[x].highlight.undercurl && !line[x].highlight.strikethrough {
			continue
		}

		// if CachedDrawing is disabled
		if !editor.config.Editor.CachedDrawing {
			pen := gui.NewQPen()
			var color *gui.QColor
			sp := line[x].highlight.special
			if sp != nil {
				color = sp.QColor()
				pen.SetColor(color)
			} else {
				fg := line[x].highlight.foreground
				color = fg.QColor()
				pen.SetColor(color)
			}
			p.SetPen(pen)
			start := float64(x) * font.cellwidth
			end := float64(x+1) * font.cellwidth

			space := float64(font.lineSpace) / 3.0
			if space > font.ascent/3.0 {
				space = font.ascent / 3.0
			}
			descent := float64(font.height) - font.ascent
			weight := int(math.Ceil(float64(font.height) / 16.0))
			if weight < 1 {
				weight = 1
			}
			if line[x].highlight.strikethrough {
				// strikeLinef := core.NewQLineF3(start, halfY, end, halfY)
				// p.DrawLine(strikeLinef)
				Y := float64(y*font.lineHeight+scrollPixels) + float64(font.ascent)*0.65 + float64(font.lineSpace/2)
				p.FillRect5(
					int(start),
					int(Y),
					int(math.Ceil(font.cellwidth)),
					weight,
					color,
				)
			}
			if line[x].highlight.underline {
				// linef := core.NewQLineF3(start, Y, end, Y)
				// p.DrawLine(linef)
				p.FillRect5(
					int(start),
					int(float64((y+1)*font.lineHeight+scrollPixels))-weight,
					int(math.Ceil(font.cellwidth)),
					weight,
					color,
				)
			}
			if line[x].highlight.undercurl {
				amplitude := descent*0.65 + float64(font.lineSpace)
				maxAmplitude := font.ascent / 8.0
				if amplitude >= maxAmplitude {
					amplitude = maxAmplitude
				}
				freq := 1.0
				phase := 0.0
				Y := float64(y*font.lineHeight+scrollPixels) + float64(font.ascent+descent*0.3) + float64(font.lineSpace/2) + space
				Y2 := Y + amplitude*math.Sin(0)
				point := core.NewQPointF3(start, Y2)
				path := gui.NewQPainterPath2(point)
				for i := int(point.X()); i <= int(end); i++ {
					Y2 = Y + amplitude*math.Sin(2*math.Pi*freq*float64(i)/font.cellwidth+phase)
					path.LineTo(core.NewQPointF3(float64(i), Y2))
				}
				p.DrawPath(path)
			}
		} else { // if CachedDrawing is enabled
			fgCache := w.getCache()
			var image *gui.QImage
			imagev, err := fgCache.get(HlDecoration{
				fg:            line[x].highlight.fg(),
				underline:     line[x].highlight.underline,
				undercurl:     line[x].highlight.undercurl,
				strikethrough: line[x].highlight.strikethrough,
			})

			if err != nil {
				image = w.newDecorationCache(line[x].char, line[x].highlight, line[x].normalWidth)
				w.setDecorationCache(line[x].highlight, image)
			} else {
				image = imagev.(*gui.QImage)
			}

			p.DrawImage7(
				core.NewQPointF3(
					float64(x)*font.cellwidth,
					float64(y*font.lineHeight)+float64(scrollPixels),
				),
				image,
			)
		}
	}
}

func (w *Window) getFillpatternAndTransparent(hl *Highlight) (core.Qt__BrushStyle, *RGBA, int) {
	color := hl.bg()
	pattern := core.Qt__BrushStyle(1)
	t := 255

	// if pumblend > 0
	if w.isPopupmenu {
		// t = int((transparent() * 255.0) * ((100.0 - float64(w.s.ws.pb)) / 100.0))
		// NOTE:
		// We do not use the editor's transparency for completion menus or float windows.
		// It is recommended to use pumblend or winblend to get those transparencies.
		t = int(255 * ((100.0 - float64(w.s.ws.pb)) / 100.0))
	}
	// if winblend > 0
	if !w.isPopupmenu && w.isFloatWin {
		// t = int((transparent() * 255.0) * ((100.0 - float64(w.wb)) / 100.0))
		// NOTE:
		// We do not use the editor's transparency for completion menus or float windows.
		// It is recommended to use pumblend or winblend to get those transparencies.
		t = int(255 * ((100.0 - float64(w.wb)) / 100.0))
	}
	if w.isMsgGrid {
		if editor.config.Message.Transparent < 1.0 {
			t = int(editor.config.Message.Transparent * 255.0)
		} else {
			t = 255
		}
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

func isCJK(char rune) bool {
	if unicode.Is(unicode.Han, char) {
		return true
	}
	if unicode.Is(unicode.Hiragana, char) {
		return true
	}
	if unicode.Is(unicode.Katakana, char) {
		return true
	}
	if unicode.Is(unicode.Hangul, char) {
		return true
	}

	return false
}

// isNormalWidth is:
// On Windows, HorizontalAdvance() may take a long time to get the width of CJK characters.
// For this reason, for CJK characters, the character width should be the double width of ASCII characters.
// This issue may also be related to the following.
// https://github.com/equalsraf/neovim-qt/issues/614
func (w *Window) isNormalWidth(char string) bool {
	if len(char) == 0 {
		return true
	}

	// if ASCII
	if char[0] <= 127 {
		return true
	}

	// if CJK
	if isCJK([]rune(char)[0]) {
		return false
	}

	return w.getFont().fontMetrics.HorizontalAdvance(char, -1) == w.getFont().cellwidth
}

func (w *Window) deleteExternalWin() {
	if w.extwin != nil {
		w.extwin.Hide()
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
		return w.fgCache
	}

	return w.s.fgCache
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

	win.SetAcceptDrops(true)
	win.ConnectPaintEvent(win.paint)
	win.ConnectDragEnterEvent(win.dragEnterEvent)
	win.ConnectDragMoveEvent(win.dragMoveEvent)
	win.ConnectDropEvent(win.dropEvent)

	// win.ConnectMousePressEvent(screen.mousePressEvent)
	win.ConnectMouseReleaseEvent(win.mouseEvent)
	win.ConnectMouseMoveEvent(win.mouseEvent)

	return win
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

				if bufName != "" {
					w.s.howToOpen(filepath)
				} else {
					fileOpenInBuf(filepath)
				}
			default:
			}
		}
	}
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
		if win.isFloatWin {
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
	for _, win := range floatWins {
		win.Raise()
	}

	// handle cursor widget
	w.setUIParent()
}

func (w *Window) setUIParent() {
	// Update cursor font
	w.s.ws.cursor.updateFont(w, w.getFont())
	defer func() {
		w.s.ws.cursor.isInPalette = false
	}()

	// // ws := editor.workspaces[editor.active]
	// prevCursorWin, ok := w.s.ws.screen.getWindow(w.s.ws.cursor.prevGridid)

	// for handling external window
	if !w.isExternal {
		editor.window.Raise()
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
		w.lenContent[i] = boundary
		for j, _ := range w.contentMask[i] {
			w.contentMask[i][j] = true
		}
	}
}

func (w *Window) fill() {
	w.refreshUpdateArea(0)
	if editor.config.Editor.Transparent < 1.0 {
		return
	}
	if w.isMsgGrid && editor.config.Message.Transparent < 1.0 {
		return
	}
	// If popupmenu pumblend is set
	if w.isPopupmenu && w.s.ws.pb > 0 {
		return
	}
	// If window winblend > 0 is set
	if !w.isPopupmenu && w.isFloatWin && w.wb > 0 {
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

func (w *Window) move(col int, row int, anchorwindow ...*Window) {
	font := w.s.font
	var anchorwin *Window
	if len(anchorwindow) > 0 {
		anchorwin = anchorwindow[0]
		font = anchorwin.getFont()
	}

	res := 0
	if w.isMsgGrid {
		res = w.s.widget.Height() - w.rows*font.lineHeight
	}
	if res < 0 || w.isExternal {
		res = 0
	}
	x := int(float64(col) * font.cellwidth)
	y := (row * font.lineHeight) + res

	// Fix https://github.com/akiyosi/goneovim/issues/316#issuecomment-1039978355
	// Adjustment of the float window position when the repositioning process
	// is being applied to the anchor window when it is outside the application window.
	var anchorposx, anchorposy int
	if len(anchorwindow) > 0 {
		if anchorwin.grid != w.grid {
			anchorposx = anchorwin.Pos().X()
			anchorposy = anchorwin.Pos().Y()
		}
	}

	if w.isFloatWin && !w.isMsgGrid {
		// A workarround for ext_popupmenu and displaying a LSP tooltip
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
