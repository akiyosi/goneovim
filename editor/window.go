package editor

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"strings"
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
	id            int
	kind          string
	uiName        string
	hlName        string
	foreground    *RGBA
	background    *RGBA
	special       *RGBA
	reverse       bool
	italic        bool
	bold          bool
	underline     bool
	undercurl     bool
	blend         int
	strikethrough bool
}

// HlChars is used in screen cache
type HlChars struct {
	text string
	fg   *RGBA
	// bg     *RGBA
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
	normalWidth bool
	char        string
	highlight   *Highlight
}

type IntInt [2]int

// ExternalWin is
type ExternalWin struct {
	widgets.QDialog
}

// Window is
type Window struct {
	widgets.QWidget
	_ float64 `property:"scrollDiff"`

	snapshot *gui.QPixmap

	paintMutex  sync.RWMutex
	redrawMutex sync.Mutex
	updateMutex sync.RWMutex

	doErase bool

	s              *Screen
	content        [][]*Cell
	lenLine        []int
	lenContent     []int
	lenOldContent  []int
	maxLenContent  int
	contentMask    [][]bool
	contentMaskOld [][]bool

	grid        gridId
	isGridDirty bool
	id          nvim.Window
	bufName     string
	pos         [2]int
	anchor      string
	cols        int
	rows        int
	cwd         string
	ts          int
	wb          int
	ft          string

	propMutex   sync.RWMutex
	isMsgGrid   bool
	isFloatWin  bool
	isExternal  bool
	isPopupmenu bool

	scrollRegion    []int
	queueRedrawArea [4]int

	scrollPixels       [2]int // for touch pad scroll
	scrollPixels2      int    // for viewport
	scrollPixelsDeltaY int
	lastScrollphase    core.Qt__ScrollPhase
	scrollCols         int
	scrollViewport     [2][5]int // 1. topline, botline, curline, curcol, grid, 2. oldtopline, oldbotline, oldcurline, oldcurcol, oldgrid
	doGetSnapshot      bool

	devicePixelRatio float64
	fgCache          Cache

	extwin                 *ExternalWin
	extwinConnectResizable bool
	extwinResized          bool
	extwinManualResized    bool
	extwinAutoLayoutPosX   []int
	extwinAutoLayoutPosY   []int
	extwinRelativePos      [2]int

	font         *Font
	background   *RGBA
	width        float64
	height       int
	localWindows *[4]localWindow
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

func (w *Window) paint(event *gui.QPaintEvent) {
	w.paintMutex.Lock()

	p := gui.NewQPainter2(w)
	if w.doErase {
		p.EraseRect3(w.Rect())
		p.DestroyQPainter()
		return
	}
	font := w.getFont()

	// Set devicePixelRatio if it is not set
	if w.devicePixelRatio == 0 {
		w.devicePixelRatio = float64(p.PaintEngine().PaintDevice().DevicePixelRatio())
	}

	rect := event.Rect()
	col := int(float64(rect.Left()) / font.truewidth)
	row := int(float64(rect.Top()) / float64(font.lineHeight))
	cols := int(math.Ceil(float64(rect.Width()) / font.truewidth))
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
	if w.isFloatWin {
		w.drawFloatWindowBorder(p)
	}

	// Draw vim window separator
	w.drawWindowSeparators(p, row, col, rows, cols)

	// Reset to 0 after drawing is complete.
	// This is to suppress flickering in smooth scroll
	dx := math.Abs(float64(w.scrollPixels[0]))
	dy := math.Abs(float64(w.scrollPixels[1]))
	if dx >= font.truewidth {
		w.scrollPixels[0] = 0
	}
	if dy >= float64(font.lineHeight) {
		w.scrollPixels[1] = 0
	}

	if w.lastScrollphase == core.Qt__NoScrollPhase {
		w.lastScrollphase = core.Qt__ScrollEnd
	}

	p.DestroyQPainter()

	w.paintMutex.Unlock()
}

func (w *Window) drawScrollSnapshot(p *gui.QPainter) {
	if !editor.config.Editor.SmoothScroll {
		return
	}
	if w.s.name == "minimap" {
		return
	}
	if w.snapshot == nil || editor.isKeyAutoRepeating {
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

	X := float64(x) * font.truewidth
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
	if !editor.config.Editor.DrawBorderForFloatWindow {
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
	if !editor.config.Editor.DrawWindowSeparator {
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
	x := int(float64(w.pos[0]) * w.s.font.truewidth)
	y := w.pos[1] * w.s.font.lineHeight
	color := editor.colors.windowSeparator
	width := int(float64(w.cols) * font.truewidth)
	winHeight := int((float64(w.rows) + 0.92) * float64(font.lineHeight))

	// Vim uses the showtabline option to change the display state of the tabline
	// based on the number of tabs. We need to look at these states to adjust
	// the length and display position of the window separator
	tablineNum := 0
	numOfTabs := w.s.ws.getNumOfTabs()
	if numOfTabs > 1 {
		tablineNum = 1
	}
	drawTabline := editor.config.Tabline.Visible && editor.config.Editor.ExtTabline
	if w.s.ws.showtabline == 2 && drawTabline && numOfTabs == 1 {
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
			int(float64(x+width)+font.truewidth/2),
			y-shift,
			2,
			winHeight,
			color.QColor(),
		)
	}
	// vertical gradient
	if editor.config.Editor.WindowSeparatorGradient {
		gradient := gui.NewQLinearGradient3(
			float64(x+width)+font.truewidth/2,
			0,
			float64(x+width)+font.truewidth/2-6,
			0,
		)
		gradient.SetColorAt(0, gui.NewQColor3(color.R, color.G, color.B, 125))
		gradient.SetColorAt(1, gui.NewQColor3(color.R, color.G, color.B, 0))
		brush := gui.NewQBrush10(gradient)
		p.FillRect2(
			int(float64(x+width)+font.truewidth/2)-6,
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
		int(float64(x)-font.truewidth/2),
		y2,
		int((float64(w.cols)+0.92)*font.truewidth),
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
			int(float64(x)-font.truewidth/2),
			y2-6,
			int((float64(w.cols)+0.92)*font.truewidth),
			6,
			hbrush,
		)
	}
}

func (w *Window) wheelEvent(event *gui.QWheelEvent) {
	var v, h, vert, horiz int
	var vertKey string
	var horizKey string

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
			vertKey = "Up"
		} else {
			vertKey = "Down"
		}
	} else {
		if vert > 0 {
			vertKey = "Up"
		} else {
			vertKey = "Down"
		}
	}
	if horiz > 0 {
		horizKey = "Left"
	} else {
		horizKey = "Right"
	}

	// If the window at the mouse pointer is not the current window
	w.focusGrid()
	mod := event.Modifiers()

	if w.s.ws.isMappingScrollKey {
		editor.putLog("detect a mapping to <C-e>, <C-y> keys.")
		if vert != 0 {
			go w.s.ws.nvim.Input(fmt.Sprintf("<%sScrollWheel%s>", editor.modPrefix(mod), vertKey))
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
	if vert == 0 {
		return
	}

	x := int(float64(event.X()) / font.truewidth)
	y := int(float64(event.Y()) / float64(font.lineHeight))
	pos := []int{x + w.pos[0], y + w.pos[1]}

	if horiz != 0 {
		w.s.ws.nvim.Input(fmt.Sprintf("<%sScrollWheel%s><%d,%d>", editor.modPrefix(mod), horizKey, pos[0], pos[1]))
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

	if dx < font.truewidth {
		w.scrollPixels[0] += h
	}
	if dy < float64(font.lineHeight) {
		w.scrollPixels[1] += v
	}

	dx = math.Abs(float64(w.scrollPixels[0]))
	dy = math.Abs(float64(w.scrollPixels[1]))

	if dx >= font.truewidth {
		horiz = int(float64(w.scrollPixels[0]) / font.truewidth)
	}
	if dy >= float64(font.lineHeight) {
		vert = int(float64(w.scrollPixels[1]) / float64(font.lineHeight))
		// NOTE: Reset to 0 after paint event is complete.
		//       This is to suppress flickering.
	}

	// w.update()
	// w.s.ws.cursor.update()
	if !(dx >= font.truewidth || dy > float64(font.lineHeight)) {
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
			int(float64(win.cols)*font.truewidth),
			win.rows*font.lineHeight,
		)
		if win.scrollPixels2 == 0 {
			win.doErase = true
			win.Update2(
				0,
				0,
				int(float64(win.cols)*font.truewidth),
				win.cols*font.lineHeight,
			)
			win.doErase = false
			win.fill()

			// get snapshot
			if !editor.isKeyAutoRepeating && editor.config.Editor.SmoothScroll {
				win.snapshot = win.Grab(win.Rect())
			}
		}
	})
	a.SetDuration(220)
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

	// We should control to draw statusline, vsplitter
	if editor.config.Editor.DrawWindowSeparator && win.grid == 1 {

		isSkipDraw := true
		if win.s.name != "minimap" {

			// Draw  bottom statusline
			if row == win.rows-2 {
				isSkipDraw = false
			}
			// Draw tabline
			if row == 0 {
				isSkipDraw = false
			}

			// // Do not Draw statusline of splitted window
			// win.s.windows.Range(func(_, winITF interface{}) bool {
			// 	w := winITF.(*Window)
			// 	if w == nil {
			// 		return true
			// 	}
			// 	if !w.isShown() {
			// 		return true
			// 	}
			// 	if row == w.pos[1]-1 {
			// 		isDraw = true
			// 		return false
			// 	}
			// 	return true
			// })
		} else {
			isSkipDraw = false
		}

		if isSkipDraw {
			return
		}
	}

	win.updateLine(colStart, row, cells)
	win.countContent(row)
	win.makeUpdateMask(row)
	if !win.isShown() {
		win.show()
	}

	if win.isMsgGrid {
		return
	}
	if win.grid == 1 {
		return
	}
	if win.maxLenContent < win.lenContent[row] {
		win.maxLenContent = win.lenContent[row]
	}
}

func (w *Window) updateLine(col, row int, cells []interface{}) {
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
		text := cell[0]
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
			}

			line[col].char = text.(string)
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

			// Detect popupmenu
			if line[col].highlight.uiName == "Pmenu" ||
				line[col].highlight.uiName == "PmenuSel" ||
				line[col].highlight.uiName == "PmenuSbar" {
				w.isPopupmenu = true
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
			} else if cell.char == " " && cell.highlight.bg().equals(w.background) {
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

func (w *Window) makeUpdateMask(row int) {
	line := w.content[row]
	for j, cell := range line {
		if cell == nil {
			w.contentMask[row][j] = true
		} else if cell.char == " " && cell.highlight.bg().equals(w.background) {
			w.contentMask[row][j] = false
		} else {
			w.contentMask[row][j] = true
		}
	}
}

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
	if w.scrollPixels[1] != 0 || editor.config.Editor.IndentGuide {
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
			width = w.cols
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
		start := 0
		if drawWithSingleRect {
			rect := [4]int{
				0,
				i * font.lineHeight,
				int(math.Ceil(float64(width) * font.truewidth)),
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
					// // If the next rectangular area will be created with only one cell separating it, merge it.
					// if j+1 <= len(w.contentMask[i])-1 {
					// 	if w.contentMask[i][j+1] {
					// 		continue
					// 	}
					// }

					jj := j

					// If it reaches the edge of the grid
					if j >= len(w.contentMask[i])-1 && isCreateRect {
						jj++
					}

					// create rectangular area
					rect := [4]int{
						int(float64(start) * font.truewidth),
						i * font.lineHeight,
						int(math.Ceil(float64(jj-start+1) * font.truewidth)),
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
				int(float64(width)*font.truewidth),
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
	font := w.getFont()
	line := w.content[y]
	var bg *RGBA

	// draw default background color if window is float window or msg grid
	isDrawDefaultBg := false
	if w.isFloatWin || (w.isMsgGrid && editor.config.Message.Transparent < 1.0) {
		isDrawDefaultBg = true
		// If popupmenu pumblend is set
		if w.isPopupmenu && w.s.ws.pb > 0 {
			w.SetAutoFillBackground(false)
		}

		// If float window winblend is set
		if !w.isPopupmenu && w.isFloatWin && w.wb > 0 {
			w.SetAutoFillBackground(false)
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
	// 				 float64(x)*font.truewidth,
	// 				 float64((y)*font.lineHeight),
	// 				 font.truewidth,
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
	var start, end, width int
	var lastBg *RGBA
	var lastHighlight, highlight *Highlight

	fillCellRect := func() {
		if lastHighlight == nil {
			return
		}
		width = end - start + 1
		if width < 0 {
			width = 0
		}
		if !isDrawDefaultBg && lastBg.equals(w.background) {
			width = 0
		}
		if width > 0 {
			// Set diff pattern
			pattern, color, transparent := w.getFillpatternAndTransparent(lastHighlight)

			// Fill background with pattern
			rectF := core.NewQRectF4(
				float64(start)*font.truewidth,
				float64((y)*font.lineHeight+scrollPixels),
				float64(width)*font.truewidth,
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
			width = 0
		}
	}
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
				fillCellRect()

				start = x
				end = x
				lastBg = bg
				lastHighlight = highlight

				if x == bounds {
					fillCellRect()
				}
			}
		}
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

	pointX := float64(col) * wsfont.truewidth
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
		if editor.config.Editor.DisableLigatures {

			// if CachedDrawing is disabled
			if !editor.config.Editor.CachedDrawing {
				w.drawTextInPos(
					p,
					core.NewQPointF3(
						float64(x)*wsfont.truewidth,
						float64(y*wsfont.lineHeight+wsfont.shift+scrollPixels),
					),
					line[x].char,
					line[x].highlight,
					true,
				)

				// if CachedDrawing is enabled
			} else {
				w.drawTextInPosWithCache(
					p,
					core.NewQPointF3(
						float64(x)*wsfont.truewidth,
						float64(y*wsfont.lineHeight+scrollPixels),
					),
					line[x].char,
					line[x].highlight,
					false,
				)
			}

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
	if !editor.config.Editor.DisableLigatures {

		var pointf *core.QPointF
		// if CachedDrawing is disabled
		if !editor.config.Editor.CachedDrawing {
			pointf = core.NewQPointF3(
				pointX,
				float64((y)*wsfont.lineHeight+wsfont.shift+scrollPixels),
			)
		} else { // if CachedDrawing is enabled
			pointf = core.NewQPointF3(
				pointX,
				float64(y*wsfont.lineHeight+scrollPixels),
			)
		}

		for highlight, colorSlice := range chars {
			var buffer bytes.Buffer
			slice := colorSlice

			for x := col; x <= col+cols; x++ {
				if len(slice) == 0 {
					break
				}
				index := slice[0]

				if x < index {
					buffer.WriteString(" ")
					continue
				}

				if x == index {
					buffer.WriteString(line[x].char)
					slice = slice[1:]
				}
			}

			text := buffer.String()

			// if CachedDrawing is disabled
			if !editor.config.Editor.CachedDrawing {
				w.drawTextInPos(
					p,
					pointf,
					text,
					highlight,
					true,
				)
			} else { // if CachedDrawing is enabled
				w.drawTextInPosWithCache(
					p,
					pointf,
					text,
					highlight,
					true,
				)
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

			// if CachedDrawing is disabled
			if !editor.config.Editor.CachedDrawing {
				w.drawTextInPos(
					p,
					core.NewQPointF3(
						float64(x)*wsfont.truewidth,
						float64(y*wsfont.lineHeight+wsfont.shift+scrollPixels),
					),
					line[x].char,
					line[x].highlight,
					false,
				)
			} else { // if CachedDrawing is enabled
				w.drawTextInPosWithCache(
					p,
					core.NewQPointF3(
						float64(x)*wsfont.truewidth,
						float64(y*wsfont.lineHeight+scrollPixels),
					),
					line[x].char,
					line[x].highlight,
					false,
				)
			}
		}
	}
}

// func (w *Window) drawTextInPosWithCache(p *gui.QPainter, x, y int, text string, highlight *Highlight, isNormalWidth bool) {
func (w *Window) drawTextInPosWithCache(p *gui.QPainter, point *core.QPointF, text string, highlight *Highlight, isNormalWidth bool) {
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

	p.DrawImage7(
		point,
		image,
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

	width := font.truewidth
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
	start := float64(0) * font.truewidth
	end := float64(width) * font.truewidth

	space := float64(font.lineSpace) / 3.0
	if space > font.ascent/3.0 {
		space = font.ascent / 3.0
	}
	descent := float64(font.height) - font.ascent
	weight := int(math.Ceil(float64(font.height) / 16.0))
	if weight < 1 {
		weight = 1
	}
	if highlight.strikethrough {
		Y := float64(0*font.lineHeight+scrollPixels) + float64(font.ascent)*0.65 + float64(font.lineSpace/2)
		pi.FillRect5(
			int(start),
			int(Y),
			int(math.Ceil(font.truewidth)),
			weight,
			color,
		)
	}
	if highlight.underline {
		pi.FillRect5(
			int(start),
			int(float64((0+1)*font.lineHeight+scrollPixels))-weight,
			int(math.Ceil(font.truewidth)),
			weight,
			color,
		)
	}
	if highlight.undercurl {
		amplitude := descent*0.65 + float64(font.lineSpace)
		maxAmplitude := font.ascent / 8.0
		if amplitude >= maxAmplitude {
			amplitude = maxAmplitude
		}
		freq := 1.0
		phase := 0.0
		Y := float64(0*font.lineHeight+scrollPixels) + float64(font.ascent+descent*0.3) + float64(font.lineSpace/2) + space
		Y2 := Y + amplitude*math.Sin(0)
		point := core.NewQPointF3(start, Y2)
		path := gui.NewQPainterPath2(point)
		for i := int(point.X()); i <= int(end); i++ {
			Y2 = Y + amplitude*math.Sin(2*math.Pi*freq*float64(i)/font.truewidth+phase)
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
		pi.Font().SetWeight(font.fontNew.Weight() + 25)
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

func (w *Window) drawTextInPos(p *gui.QPainter, point *core.QPointF, text string, highlight *Highlight, isNormalWidth bool) {
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
	p.DrawText(point, text)
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
			start := float64(x) * font.truewidth
			end := float64(x+1) * font.truewidth

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
					int(math.Ceil(font.truewidth)),
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
					int(math.Ceil(font.truewidth)),
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
					Y2 = Y + amplitude*math.Sin(2*math.Pi*freq*float64(i)/font.truewidth+phase)
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
					float64(x)*font.truewidth,
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
		t = int((transparent() * 255.0) * ((100.0 - float64(w.s.ws.pb)) / 100.0))
	}
	// if winblend > 0
	if !w.isPopupmenu && w.isFloatWin && w.wb > 0 {
		t = int((transparent() * 255.0) * ((100.0 - float64(w.wb)) / 100.0))
	}
	if w.isMsgGrid && editor.config.Message.Transparent < 1.0 {
		t = int(editor.config.Message.Transparent * 255.0)
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

	return w.getFont().fontMetrics.HorizontalAdvance(char, -1) == w.getFont().truewidth
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
			cols := int((float64(width) / w.getFont().truewidth))
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
	win.SetContentsMargins(0, 0, 0, 0)
	win.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	win.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")
	win.scrollRegion = []int{0, 0, 0, 0}
	win.background = editor.colors.bg

	win.ConnectPaintEvent(win.paint)
	win.SetAcceptDrops(true)
	win.ConnectDragEnterEvent(win.dragEnterEvent)
	win.ConnectDragMoveEvent(win.dragMoveEvent)
	win.ConnectDropEvent(win.dropEvent)

	return win
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
	w.Raise()

	font := w.getFont()
	w.s.ws.cursor.updateFont(font)
	w.s.ws.cursor.isInPalette = false
	if !w.isExternal {
		editor.window.Raise()
		w.s.ws.cursor.SetParent(w.s.ws.widget)
	} else if w.isExternal {
		w.extwin.Raise()
		w.s.ws.cursor.SetParent(w.extwin)
	}
	w.s.ws.cursor.Raise()
	w.s.ws.cursor.Hide()
	w.s.ws.cursor.Show()
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

// refreshUpdateArea:: arg:1 => full, arg:1 => full only text
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

func (w *Window) move(col int, row int) {
	font := w.s.font
	res := 0
	if w.isMsgGrid {
		res = w.s.widget.Height() - w.rows*font.lineHeight
	}
	if res < 0 {
		res = 0
	}
	x := int(float64(col) * font.truewidth)
	y := (row * font.lineHeight) + res

	if w.isFloatWin {
		if w.s.ws.drawTabline {
			y += w.s.ws.tabline.widget.Height()
		}
		// A workarround for ext_popupmenu and displaying a LSP tooltip
		if editor.config.Editor.ExtPopupmenu {
			if w.s.ws.mode == "insert" && w.s.ws.popup.widget.IsVisible() {
				if w.s.ws.popup.widget.IsVisible() {
					w.SetGraphicsEffect(util.DropShadow(0, 25, 125, 110))
					w.Move2(
						w.s.ws.popup.widget.X()+w.s.ws.popup.widget.Width()+5,
						w.s.ws.popup.widget.Y(),
					)
					w.Raise()
				}

				return
			}
		}
	}
	if w.isExternal {
		w.Move2(EXTWINBORDERSIZE, EXTWINBORDERSIZE)
		w.layoutExternalWindow(x, y)

		return
	}

	w.Move2(x, y)

}

func (w *Window) layoutExternalWindow(x, y int) {
	font := w.s.font

	// float windows width, height
	width := int(float64(w.cols) * font.truewidth)
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

					widthRatio := float64(w.cols+win.cols) * font.truewidth / float64(editor.window.Width())
					heightRatio := float64((w.rows+win.rows)*font.lineHeight) / float64(editor.window.Height())
					if w.cols == win.cols {
						dy = append(dy, win.rows)
						height += win.rows*font.lineHeight + EXTWINBORDERSIZE*2 + EXTWINMARGINSIZE
					} else if w.rows == win.rows {
						dx = append(dx, win.cols)
						width += int(float64(win.cols)*font.truewidth) + EXTWINBORDERSIZE*2 + EXTWINMARGINSIZE
					} else {
						if widthRatio > heightRatio {
							dy = append(dy, win.rows)
							height += win.rows*font.lineHeight + EXTWINBORDERSIZE*2 + EXTWINMARGINSIZE
						} else {
							dx = append(dx, win.cols)
							width += int(float64(win.cols)*font.truewidth) + EXTWINBORDERSIZE*2 + EXTWINMARGINSIZE
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
			x += int(float64(e)*font.truewidth) + EXTWINBORDERSIZE*2 + EXTWINMARGINSIZE
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
