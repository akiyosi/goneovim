package editor

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strconv"
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

	snapshots [2]*gui.QPixmap

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

	scrollPixels       [2]int
	scrollPixelsDeltaY int
	isWheelScrolling   bool
	scrollPixels2      int
	scrollCols         int

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

// Screen is the main editor area
type Screen struct {
	ws   *Workspace
	font *Font

	name    string
	widget  *widgets.QWidget
	windows sync.Map
	width   int
	height  int

	cursor [2]int

	hlAttrDef      map[int]*Highlight
	highlightGroup map[string]int

	tooltip *widgets.QLabel

	fgCache Cache

	resizeCount uint
}

type Cache struct {
	gcache.Cache
}

func newScreen() *Screen {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")

	screen := &Screen{
		widget:         widget,
		windows:        sync.Map{},
		cursor:         [2]int{0, 0},
		highlightGroup: make(map[string]int),
		fgCache:        newCache(),
	}

	widget.SetAcceptDrops(true)
	widget.ConnectDragEnterEvent(screen.dragEnterEvent)
	widget.ConnectDragMoveEvent(screen.dragMoveEvent)
	widget.ConnectDropEvent(screen.dropEvent)
	widget.ConnectMousePressEvent(screen.mousePressEvent)
	widget.ConnectMouseReleaseEvent(screen.mouseEvent)
	widget.ConnectMouseMoveEvent(screen.mouseEvent)

	return screen
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

func (s *Screen) initInputMethodWidget() {
	tooltip := widgets.NewQLabel(s.widget, 0)
	tooltip.SetVisible(false)
	s.tooltip = tooltip
}

func (s *Screen) dragEnterEvent(e *gui.QDragEnterEvent) {
	e.AcceptProposedAction()
}

func (s *Screen) dragMoveEvent(e *gui.QDragMoveEvent) {
	e.AcceptProposedAction()
}

func (s *Screen) dropEvent(e *gui.QDropEvent) {
	e.SetDropAction(core.Qt__CopyAction)
	e.AcceptProposedAction()
	e.SetAccepted(true)

	for _, i := range strings.Split(e.MimeData().Text(), "\n") {
		data := strings.Split(i, "://")
		if i != "" {
			switch data[0] {
			case "file":
				buf, _ := s.ws.nvim.CurrentBuffer()
				bufName, _ := s.ws.nvim.BufferName(buf)
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
					s.howToOpen(filepath)
				} else {
					fileOpenInBuf(filepath)
				}
			default:
			}
		}
	}
}

func fileOpenInBuf(file string) {
	isModified, _ := editor.workspaces[editor.active].nvim.CommandOutput("echo &modified")
	if isModified == "1" {
		editor.workspaces[editor.active].nvim.Command(fmt.Sprintf(":tabnew %s", file))
	} else {
		editor.workspaces[editor.active].nvim.Command(fmt.Sprintf(":e %s", file))
	}
}

func (s *Screen) howToOpen(file string) {
	message := "[Goneovim] Do you want to diff between the file being dropped and the current buffer?"
	opts := []*NotifyButton{}
	opt1 := &NotifyButton{
		action: func() {
			editor.workspaces[editor.active].nvim.Command(fmt.Sprintf(":vertical diffsplit %s", file))
		},
		text: "Yes",
	}
	opts = append(opts, opt1)

	opt2 := &NotifyButton{
		action: func() {
			fileOpenInBuf(file)
		},
		text: "No, I want to open with a new buffer",
	}
	opts = append(opts, opt2)

	editor.pushNotification(NotifyInfo, 0, message, notifyOptionArg(opts))
}

func (s *Screen) waitTime() time.Duration {
	var ret time.Duration
	switch s.resizeCount {
	case 0:
		ret = 10
	case 1:
		ret = 100
	default:
		ret = 1000
	}

	s.resizeCount++
	return ret
}

func (s *Screen) updateSize() {
	s.ws.fontMutex.Lock()
	defer s.ws.fontMutex.Unlock()

	ws := s.ws
	scrollvarWidth := 0
	if editor.config.ScrollBar.Visible {
		scrollvarWidth = 10
	}

	minimapWidth := 0
	if s.ws.minimap != nil {
		if s.ws.minimap.visible {
			minimapWidth = editor.config.MiniMap.Width
		}
	}
	s.width = ws.width - scrollvarWidth - minimapWidth
	currentCols := int(float64(s.width) / s.font.truewidth)
	currentRows := s.height / s.font.lineHeight

	isNeedTryResize := (currentCols != ws.cols || currentRows != ws.rows)
	if !isNeedTryResize {
		return
	}

	ws.cols = currentCols
	ws.rows = currentRows

	if !ws.uiAttached {
		return
	}

	s.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)
		if win == nil {
			return true
		}
		if win.font != nil {
			win.width = 0
			win.height = 0
		}

		return true
	})

	s.uiTryResize(currentCols, currentRows)

}

func (s *Screen) uiTryResize(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	ws := s.ws
	done := make(chan error, 5)
	var result error
	go func() {
		result = ws.nvim.TryResizeUI(width, height)
		done <- result
	}()
	select {
	case <-done:
	case <-time.After(s.waitTime() * time.Millisecond):
	}
}

func (s *Screen) getWindow(grid int) (*Window, bool) {
	winITF, ok := s.windows.Load(grid)
	if !ok {
		return nil, false
	}
	win := winITF.(*Window)
	if win == nil {
		return nil, false
	}

	return win, true
}

func (s *Screen) storeWindow(grid int, win *Window) {
	s.windows.Store(grid, win)
}

func (s *Screen) lenWindows() int {
	length := 0
	s.windows.Range(func(_, _ interface{}) bool {
		length++
		return true
	})

	return length
}

func (s *Screen) gridFont(update interface{}) {

	// Get current window
	grid := s.ws.cursor.gridid
	if !editor.config.Editor.ExtCmdline {
		grid = s.ws.cursor.bufferGridid
	}

	win, ok := s.getWindow(grid)
	if !ok {
		return
	}

	updateStr, ok := update.(string)
	if !ok {
		return
	}
	if updateStr == "" {
		return
	}
	parts := strings.Split(updateStr, ":")
	if len(parts) < 1 {
		return
	}

	fontfamily := parts[0]
	height := 14

	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "h") {
			var err error
			height, err = strconv.Atoi(p[1:])
			if err != nil {
				return
			}
		} else if strings.HasPrefix(p, "w") {
			var err error
			width, err := strconv.Atoi(p[1:])
			if err != nil {
				return
			}
			height = 2 * width
		}
	}

	oldWidth := float64(win.cols) * win.getFont().truewidth
	oldHeight := win.rows * win.getFont().lineHeight
	win.width = oldWidth
	win.height = oldHeight
	win.localWindows = &[4]localWindow{}

	win.font = initFontNew(fontfamily, float64(height), 1)

	// Calculate new cols, rows of current grid
	newCols := int(oldWidth / win.font.truewidth)
	newRows := oldHeight / win.font.lineHeight

	// Cache
	cache := win.fgCache
	if cache == (Cache{}) {
		cache := newCache()
		win.fgCache = cache
	} else {
		win.fgCache.purge()
	}

	_ = s.ws.nvim.TryResizeUIGrid(s.ws.cursor.gridid, newCols, newRows)
	font := win.getFont()
	s.ws.cursor.updateFont(font)

	if win.isExternal {
		width := int(float64(newCols)*win.font.truewidth) + EXTWINBORDERSIZE*2
		height := newRows*win.font.lineHeight + EXTWINBORDERSIZE*2
		win.extwin.Resize2(width, height)
	}
}

func (s *Screen) purgeTextCacheForWins() {
	if !editor.config.Editor.CachedDrawing {
		return
	}
	s.fgCache.purge()
	s.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)
		if win == nil {
			return true
		}
		if win.font == nil {
			return true
		}
		cache := win.fgCache
		if cache == (Cache{}) {
			return true
		}
		win.fgCache.purge()
		return true
	})
}

func (s *Screen) toolTipPos() (int, int, int, int) {
	var x, y, candX, candY int
	ws := s.ws
	if s.lenWindows() == 0 {
		return 0, 0, 0, 0
	}
	if ws.palette == nil {
		return 0, 0, 0, 0
	}
	if ws.palette.widget.IsVisible() {
		s.tooltip.SetParent(s.ws.palette.widget)
		font := gui.NewQFont2(editor.extFontFamily, editor.extFontSize, 1, false)
		s.tooltip.SetFont(font)
		x = ws.palette.cursorX + ws.palette.patternPadding
		candX = x + ws.palette.widget.Pos().X()
		y = ws.palette.patternPadding + ws.palette.padding
		candY = y + ws.palette.widget.Pos().Y()
	} else {
		win, ok := s.getWindow(s.ws.cursor.gridid)
		if !ok {
			return 0, 0, 0, 0
		}
		font := win.getFont()
		s.toolTipFont(font)
		row := s.cursor[0]
		col := s.cursor[1]
		x = int(float64(col) * font.truewidth)
		y = row * font.lineHeight

		candX = int(float64(col+win.pos[0]) * font.truewidth)
		tablineMarginTop := 0
		if ws.tabline != nil {
			tablineMarginTop = ws.tabline.marginTop
		}
		tablineHeight := 0
		if ws.tabline != nil {
			tablineHeight = ws.tabline.height
		}
		tablineMarginBottom := 0
		if ws.tabline != nil {
			tablineMarginBottom = ws.tabline.marginBottom
		}
		candY = (row+win.pos[1])*font.lineHeight + tablineMarginTop + tablineHeight + tablineMarginBottom
	}
	return x, y, candX, candY
}

func (s *Screen) toolTipMove(x int, y int) {
	if s.ws.palette == nil {
		return
	}
	padding := 0
	if s.ws.palette.widget.IsVisible() {
		padding = s.ws.palette.padding
	}
	s.tooltip.Move(core.NewQPoint2(x+padding, y))
}

func (s *Screen) toolTipFont(font *Font) {
	s.tooltip.SetFont(font.fontNew)
	s.tooltip.SetContentsMargins(0, font.lineSpace/2, 0, font.lineSpace/2)
}

func (s *Screen) toolTipShow() {
	if s.ws.palette == nil {
		return
	}
	if !s.ws.palette.widget.IsVisible() {
		win, ok := s.getWindow(s.ws.cursor.gridid)
		if ok {
			s.tooltip.SetParent(win)
		}
	}
	s.tooltip.AdjustSize()
	s.tooltip.Show()
}

func (s *Screen) toolTip(text string) {
	s.tooltip.SetText(text)
	s.tooltip.AdjustSize()
	s.toolTipShow()

	row := s.cursor[0]
	col := s.cursor[1]
	c := s.ws.cursor
	c.x = int(float64(col)*s.font.truewidth) + s.tooltip.Width()
	c.y = row * s.font.lineHeight
	c.move()
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

	snapshot := w.snapshots[1]
	if snapshot == nil {
		snapshot = w.snapshots[0]
	}
	if snapshot == nil || editor.isKeyAutoRepeating {
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
			snapshot,
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
	if w.ts <= 0 {
		return w.s.ws.ts
	}

	return w.ts
}

func (w *Window) drawIndentguide(p *gui.QPainter, row, rows int) {
	if w == nil {
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
	X := float64(x) * font.truewidth
	Y := float64(y*font.lineHeight) + float64(w.scrollPixels[1]) + float64(w.scrollPixels2)
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

func (s *Screen) bottomWindowPos() int {
	pos := 0
	s.windows.Range(func(_, winITF interface{}) bool {
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
		position := win.pos[1]*win.s.font.lineHeight + win.Rect().Bottom()
		if pos < position {
			pos = position
		}

		return true
	})

	return pos
}

func (w *Window) wheelEvent(event *gui.QWheelEvent) {
	var v, h, vert, horiz int
	var vertKey string
	var horizKey string

	font := w.getFont()

	// Detect current mode
	mode := w.s.ws.mode
	if mode == "terminal-input" {
		w.s.ws.nvim.Input(`<C-\><C-n>`)
	} else if mode != "normal" {
		w.s.ws.nvim.Input(w.s.ws.escKeyInInsert)
	}

	pixels := event.PixelDelta()
	if pixels != nil {
		v = pixels.Y()
		h = pixels.X()
	}
	isStopScroll := (event.Phase() == core.Qt__ScrollEnd)
	w.isWheelScrolling = !isStopScroll

	if (v == 0 || h == 0) && isStopScroll {
		vert, horiz = w.smoothUpdate(v, h, isStopScroll)
	} else if (v != 0 || h != 0) && event.Phase() != core.Qt__NoScrollPhase {
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
	if vert > 0 {
		vertKey = "Up"
	} else {
		vertKey = "Down"
	}
	if horiz > 0 {
		horizKey = "Left"
	} else {
		horizKey = "Right"
	}

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
	mod := event.Modifiers()

	if w.s.ws.isMappingScrollKey {
		if vert != 0 {
			go w.s.ws.nvim.Input(fmt.Sprintf("<%sScrollWheel%s>", editor.modPrefix(mod), vertKey))
		}
	} else {
		if vert > 0 {
			go w.s.ws.nvim.Input(fmt.Sprintf("%v<C-y>", int(math.Abs(float64(vert)))))
		} else if vert < 0 {
			go w.s.ws.nvim.Input(fmt.Sprintf("%v<C-e>", int(math.Abs(float64(vert)))))
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

// screen smooth update with touchpad
func (w *Window) smoothUpdate(v, h int, isStopScroll bool) (int, int) {
	var vert, horiz int
	font := w.getFont()

	if isStopScroll {
		w.scrollPixels[0] = 0
		w.scrollPixels[1] = 0
		for i := 0; i < w.rows; i++ {
			w.lenContent[i] = w.maxLenContent
		}

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

func (s *Screen) mousePressEvent(event *gui.QMouseEvent) {
	s.mouseEvent(event)
	if !editor.config.Editor.ClickEffect {
		return
	}

	win, ok := s.getWindow(s.ws.cursor.gridid)
	if !ok {
		return
	}
	font := win.getFont()

	widget := widgets.NewQWidget(nil, 0)
	widget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")
	widget.SetParent(win)
	widget.SetFixedSize2(font.lineHeight*4/3, font.lineHeight*4/3)
	widget.Show()
	widget.ConnectPaintEvent(func(e *gui.QPaintEvent) {
		p := gui.NewQPainter2(widget)
		p.SetRenderHint(gui.QPainter__Antialiasing, true)
		p.SetRenderHint(gui.QPainter__HighQualityAntialiasing, true)
		rgbAccent := hexToRGBA(editor.config.SideBar.AccentColor)

		x := float64(font.lineHeight * 2 / 3)
		y := float64(font.lineHeight * 2 / 3)
		r := float64(font.lineHeight * 2 / 3)
		point := core.NewQPointF3(x, y)

		rgbAccent = newRGBA(rgbAccent.R, rgbAccent.G, rgbAccent.B, 0.1)
		p.SetBrush(gui.NewQBrush3(rgbAccent.QColor(), core.Qt__SolidPattern))
		p.DrawEllipse4(
			point,
			r,
			r,
		)

		rgbAccent = newRGBA(rgbAccent.R, rgbAccent.G, rgbAccent.B, 0.2)
		p.SetBrush(gui.NewQBrush3(rgbAccent.QColor(), core.Qt__SolidPattern))
		p.DrawEllipse4(
			point,
			r*0.9,
			r*0.9,
		)
		rgbAccent = newRGBA(rgbAccent.R, rgbAccent.G, rgbAccent.B, 0.3)
		p.SetBrush(gui.NewQBrush3(rgbAccent.QColor(), core.Qt__SolidPattern))
		p.DrawEllipse4(
			point,
			r*0.85,
			r*0.85,
		)
		rgbAccent = newRGBA(rgbAccent.R, rgbAccent.G, rgbAccent.B, 0.4)
		p.SetBrush(gui.NewQBrush3(rgbAccent.QColor(), core.Qt__SolidPattern))
		p.DrawEllipse4(
			point,
			r*0.8,
			r*0.8,
		)
		rgbAccent = newRGBA(rgbAccent.R, rgbAccent.G, rgbAccent.B, 0.7)
		p.SetBrush(gui.NewQBrush3(rgbAccent.QColor(), core.Qt__SolidPattern))
		p.DrawEllipse4(
			point,
			r*0.7,
			r*0.7,
		)
		rgbAccent = newRGBA(rgbAccent.R, rgbAccent.G, rgbAccent.B, 0.9)
		p.SetBrush(gui.NewQBrush3(rgbAccent.QColor(), core.Qt__SolidPattern))
		p.DrawEllipse4(
			point,
			r*0.65,
			r*0.65,
		)

		p.DestroyQPainter()
	})
	widget.Move2(
		event.Pos().X()-font.lineHeight*2/3-1,
		event.Pos().Y()-font.lineHeight*2/3-1,
	)

	eff := widgets.NewQGraphicsOpacityEffect(widget)
	widget.SetGraphicsEffect(eff)
	a := core.NewQPropertyAnimation2(eff, core.NewQByteArray2("opacity", len("opacity")), widget)
	a.SetDuration(500)
	a.SetStartValue(core.NewQVariant5(1))
	a.SetEndValue(core.NewQVariant5(0))
	a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__InOutQuart))
	a.Start(core.QAbstractAnimation__DeletionPolicy(core.QAbstractAnimation__DeleteWhenStopped))
	go func() {
		time.Sleep(500 * time.Millisecond)
		widget.Hide()
		s.update()
	}()
}

func (s *Screen) mouseEvent(event *gui.QMouseEvent) {
	inp := s.convertMouse(event)
	if inp == "" {
		return
	}
	s.ws.nvim.Input(inp)
}

func (s *Screen) convertMouse(event *gui.QMouseEvent) string {
	font := s.font
	x := int(float64(event.X()) / font.truewidth)
	y := int(float64(event.Y()) / float64(font.lineHeight))
	pos := []int{x, y}

	bt := event.Button()
	if event.Type() == core.QEvent__MouseMove {
		if event.Buttons()&core.Qt__LeftButton > 0 {
			bt = core.Qt__LeftButton
		} else if event.Buttons()&core.Qt__RightButton > 0 {
			bt = core.Qt__RightButton
		} else if event.Buttons()&core.Qt__MidButton > 0 {
			bt = core.Qt__MidButton
		} else {
			return ""
		}
	}

	mod := event.Modifiers()
	buttonName := ""
	switch bt {
	case core.Qt__LeftButton:
		buttonName += "Left"
	case core.Qt__RightButton:
		buttonName += "Right"
	case core.Qt__MidButton:
		buttonName += "Middle"
	case core.Qt__NoButton:
	default:
		return ""
	}

	evType := ""
	switch event.Type() {
	case core.QEvent__MouseButtonDblClick:
		evType += "Mouse"
	case core.QEvent__MouseButtonPress:
		evType += "Mouse"
	case core.QEvent__MouseButtonRelease:
		evType += "Release"
	case core.QEvent__MouseMove:
		evType += "Drag"
	default:
		return ""
	}

	return fmt.Sprintf("<%s%s%s><%d,%d>", editor.modPrefix(mod), buttonName, evType, pos[0], pos[1])
}

func (s *Screen) gridResize(args []interface{}) {
	var gridid gridId
	var rows, cols int
	for _, arg := range args {
		gridid = util.ReflectToInt(arg.([]interface{})[0])
		cols = util.ReflectToInt(arg.([]interface{})[1])
		rows = util.ReflectToInt(arg.([]interface{})[2])

		if isSkipGlobalId(gridid) {
			continue
		}

		s.resizeWindow(gridid, cols, rows)
	}
}

func (s *Screen) resizeWindow(gridid gridId, cols int, rows int) {
	win, _ := s.getWindow(gridid)

	// make new size content
	content := make([][]*Cell, rows)
	contentMask := make([][]bool, rows)
	contentMaskOld := make([][]bool, rows)
	lenLine := make([]int, rows)
	lenContent := make([]int, rows)
	lenOldContent := make([]int, rows)

	for i := 0; i < rows; i++ {
		content[i] = make([]*Cell, cols)
		contentMask[i] = make([]bool, cols)
		contentMaskOld[i] = make([]bool, cols)
		lenContent[i] = cols - 1
	}

	if win != nil && gridid != 1 {
		for i := 0; i < rows; i++ {
			if i >= len(win.content) {
				continue
			}
			lenLine[i] = win.lenLine[i]
			lenContent[i] = win.lenContent[i]
			lenOldContent[i] = win.lenOldContent[i]
			for j := 0; j < cols; j++ {
				if j >= len(win.content[i]) {
					continue
				}
				content[i][j] = win.content[i][j]
				contentMask[i][j] = win.contentMask[i][j]
				contentMaskOld[i][j] = win.contentMaskOld[i][j]
			}
		}
	}

	if win == nil {
		win = newWindow()

		win.s = s
		s.storeWindow(gridid, win)

		win.SetParent(s.widget)

		win.grid = gridid
		win.s.ws.optionsetMutex.RLock()
		ts := s.ws.ts
		win.s.ws.optionsetMutex.RUnlock()
		win.paintMutex.RLock()
		win.ts = ts
		win.paintMutex.RUnlock()

		// set scroll
		if s.name != "minimap" {
			win.ConnectWheelEvent(win.wheelEvent)
		}

		// // first cursor pos at startup app
		// if gridid == 1 && s.name != "minimap" {
		// 	s.ws.cursor.widget.SetParent(win)
		// }
		s.ws.cursor.widget.Raise()

	}
	winOldCols := win.cols
	winOldRows := win.rows

	win.lenLine = lenLine
	win.lenContent = lenContent
	win.lenOldContent = lenOldContent
	win.content = content
	win.contentMask = contentMask
	win.contentMaskOld = contentMaskOld
	win.cols = cols
	win.rows = rows

	s.resizeIndependentFontGrid(win, winOldCols, winOldRows)

	font := win.getFont()

	width := int(math.Ceil(float64(cols) * font.truewidth))
	height := rows * font.lineHeight

	win.setGridGeometry(width, height)

	win.move(win.pos[0], win.pos[1])

	win.show()

	win.queueRedrawAll()
}

func (s *Screen) resizeIndependentFontGrid(win *Window, oldCols, oldRows int) {
	var isExistMsgGrid bool
	s.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)

		if win == nil {
			return true
		}
		if win.isMsgGrid {
			isExistMsgGrid = true
		}
		return true
	})

	if !isExistMsgGrid && s.lenWindows() < 3 {
		return
	}
	if isExistMsgGrid && s.lenWindows() < 4 {
		return
	}
	if oldCols == 0 && oldRows == 0 {
		return
	}
	// var width, height int
	deltaCols := win.cols - oldCols
	deltaRows := win.rows - oldRows
	absDeltaCols := math.Abs(float64(deltaCols))
	absDeltaRows := math.Abs(float64(deltaRows))
	if absDeltaCols > 0 && absDeltaRows > 0 {
		return
	}
	var isResizeWidth, isResizeHeight bool
	if absDeltaCols > 0 {
		isResizeWidth = true
	} else if absDeltaRows > 0 {
		isResizeHeight = true
	}

	leftWindowPos := win.pos[0] + oldCols + 1 + deltaCols
	topWindowPos := win.pos[1] + oldRows + 1 + deltaRows

	s.windows.Range(func(_, winITF interface{}) bool {
		w := winITF.(*Window)
		if w == nil {
			return true
		}
		if w.grid == 1 {
			return true
		}
		if w.isMsgGrid {
			return true
		}
		if w.grid == win.grid {
			return true
		}
		if w.font == nil {
			return true
		}

		if isResizeWidth && w.width > 0 {
			// right window is gridfont window
			if w.localWindows[2].grid == win.grid || (w.pos[1] == win.pos[1] && w.pos[0] == leftWindowPos) {
				if w.localWindows[2].grid == 0 {
					w.localWindows[2].grid = win.grid
				}
				if !w.localWindows[2].isResized {
					w.localWindows[2].isResized = true
					w.localWindows[2].localWidth = w.width + float64(oldCols)*win.getFont().truewidth
				}
				newWidth := w.localWindows[2].localWidth - (float64(win.cols) * win.getFont().truewidth)
				newCols := int(newWidth / w.font.truewidth)
				if newCols != w.cols {
					_ = s.ws.nvim.TryResizeUIGrid(w.grid, newCols, w.rows)
					w.width = float64(newCols) * w.getFont().truewidth
					w.localWindows[0].isResized = false
				}
			}

			// left window is gridfont window
			// calcurate win window posision aa w window coordinate
			var resizeflag bool
			winPosX := float64(win.pos[0]) * win.s.font.truewidth
			rightWindowPos1 := float64(w.cols)*w.getFont().truewidth + float64(w.pos[0]+1-deltaCols+1)*win.s.font.truewidth
			rightWindowPos2 := float64(w.cols-1)*w.getFont().truewidth + float64(w.pos[0]+1-deltaCols+1)*win.s.font.truewidth
			rightWindowPos := int(float64(w.cols)*w.getFont().truewidth/win.s.font.truewidth) + w.pos[0] + 1 - deltaCols + 1
			if win.s.font.truewidth < w.getFont().truewidth {
				resizeflag = winPosX <= rightWindowPos1 && winPosX >= rightWindowPos2
			} else {
				resizeflag = win.pos[0] == rightWindowPos
			}
			if w.localWindows[0].grid == win.grid || (w.pos[1] == win.pos[1] && resizeflag) {
				if w.localWindows[0].grid == 0 {
					w.localWindows[0].grid = win.grid
				}
				if !w.localWindows[0].isResized {
					w.localWindows[0].isResized = true
					w.localWindows[0].localWidth = w.width + float64(oldCols)*win.getFont().truewidth
				}
				newWidth := w.localWindows[0].localWidth - (float64(win.cols) * win.getFont().truewidth)
				newCols := int(newWidth / w.font.truewidth)
				if newCols != w.cols {
					_ = s.ws.nvim.TryResizeUIGrid(w.grid, newCols, w.rows)
					w.width = float64(newCols) * w.getFont().truewidth
					w.localWindows[2].isResized = false
				}
			}

		}
		if isResizeHeight && w.height > 0 {
			// bottom window is gridfont window
			if w.localWindows[1].grid == win.grid || (w.pos[0] == win.pos[0] && w.pos[1] == topWindowPos) {
				if w.localWindows[1].grid == 0 {
					w.localWindows[1].grid = win.grid
				}
				if !w.localWindows[1].isResized {
					w.localWindows[1].isResized = true
					w.localWindows[1].localHeight = w.height + oldRows*win.getFont().lineHeight
				}
				newHeight := w.localWindows[1].localHeight - (win.rows * win.getFont().lineHeight)
				newRows := newHeight / w.font.lineHeight
				if newRows != w.rows {
					_ = s.ws.nvim.TryResizeUIGrid(w.grid, w.cols, newRows)
					w.height = newRows * w.getFont().lineHeight
					w.localWindows[3].isResized = false
				}
			}

			// top window is gridfont window
			// calcurate win window posision aa w window coordinate
			var resizeflag bool
			winPosY := win.pos[1] * win.s.font.lineHeight
			bottomWindowPos1 := w.rows*w.getFont().lineHeight + (w.pos[1]+1-deltaRows+1)*win.s.font.lineHeight
			bottomWindowPos2 := (w.rows-1)*w.getFont().lineHeight + (w.pos[1]+1-deltaRows+1)*win.s.font.lineHeight
			bottomWindowPos := (w.rows * w.getFont().lineHeight / win.s.font.lineHeight) + w.pos[1] + 1 - deltaRows + 1
			if win.s.font.lineHeight < w.getFont().lineHeight {
				resizeflag = winPosY <= bottomWindowPos1 && winPosY >= bottomWindowPos2
			} else {
				resizeflag = win.pos[1] == bottomWindowPos
			}
			if w.localWindows[3].grid == win.grid || (w.pos[0] == win.pos[0] && resizeflag) {
				if w.localWindows[3].grid == 0 {
					w.localWindows[3].grid = win.grid
				}
				if !w.localWindows[3].isResized {
					w.localWindows[3].isResized = true
					w.localWindows[3].localHeight = w.height + oldRows*win.getFont().lineHeight
				}
				newHeight := w.localWindows[3].localHeight - (win.rows * win.getFont().lineHeight)
				newRows := newHeight / w.font.lineHeight
				if newRows != w.rows {
					_ = s.ws.nvim.TryResizeUIGrid(w.grid, w.cols, newRows)
					w.height = newRows * w.getFont().lineHeight
					w.localWindows[1].isResized = false
				}
			}

		}

		return true
	})
}

func (s *Screen) gridCursorGoto(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])

		s.cursor[0] = util.ReflectToInt(arg.([]interface{})[1])
		s.cursor[1] = util.ReflectToInt(arg.([]interface{})[2])
		if isSkipGlobalId(gridid) {
			continue
		}

		win, ok := s.getWindow(gridid)
		if !ok {
			continue
		}

		if s.ws.cursor.gridid != gridid {
			if !win.isMsgGrid {
				s.ws.cursor.bufferGridid = gridid

			}
			s.ws.cursor.gridid = gridid
			s.ws.cursor.font = win.getFont()
			win.raise()

			// reset smooth scroll scrolling offset
			win.scrollPixels2 = 0

		}
	}
}

func (s *Screen) setHlAttrDef(args []interface{}) {
	var h map[int]*Highlight
	if s.hlAttrDef == nil {
		h = make(map[int]*Highlight)
	} else {
		h = s.hlAttrDef
	}

	isUpdateBg := true
	curwin, ok := s.getWindow(s.ws.cursor.gridid)
	if ok {
		isUpdateBg = !curwin.background.equals(s.ws.background)
	}

	h[0] = &Highlight{
		foreground: s.ws.foreground,
		background: s.ws.background,
	}

	for _, arg := range args {
		id := util.ReflectToInt(arg.([]interface{})[0])
		h[id] = s.getHighlight(arg)
	}

	if s.hlAttrDef == nil {
		s.hlAttrDef = h
	}

	// Update all cell's highlight
	if isUpdateBg {
		s.windows.Range(func(_, winITF interface{}) bool {
			win := winITF.(*Window)
			if win == nil {
				return true
			}
			if !win.isShown() {
				return true
			}
			if win.content == nil {
				return true
			}
			for _, line := range win.content {
				for _, cell := range line {
					if cell != nil {
						cell.highlight = s.hlAttrDef[cell.highlight.id]
					}
				}
			}

			return true
		})
	}
}

func (s *Screen) setHighlightGroup(args []interface{}) {
	for _, arg := range args {
		a := arg.([]interface{})
		hlName := a[0].(string)
		hlIndex := util.ReflectToInt(a[1])
		s.highlightGroup[hlName] = hlIndex
	}
}

func (s *Screen) getHighlight(args interface{}) *Highlight {
	arg := args.([]interface{})
	highlight := Highlight{}

	hl := arg[1].(map[string]interface{})
	info := make(map[string]interface{})
	for _, arg := range arg[3].([]interface{}) {
		info = arg.(map[string]interface{})
		break
	}

	kind, ok := info["kind"]
	if ok {
		highlight.kind = kind.(string)
	}

	id, ok := info["id"]
	if ok {
		highlight.id = util.ReflectToInt(id)
	}

	uiName, ok := info["ui_name"]
	if ok {
		highlight.uiName = uiName.(string)
	}

	hlName, ok := info["hi_name"]
	if ok {
		highlight.hlName = hlName.(string)
	}

	italic := hl["italic"]
	if italic != nil {
		highlight.italic = true
	} else {
		highlight.italic = false
	}

	bold := hl["bold"]
	if bold != nil {
		highlight.bold = true
	} else {
		highlight.bold = false
	}

	underline := hl["underline"]
	if underline != nil {
		highlight.underline = true
	} else {
		highlight.underline = false
	}

	undercurl := hl["undercurl"]
	if undercurl != nil {
		highlight.undercurl = true
	} else {
		highlight.undercurl = false
	}

	strikethrough := hl["strikethrough"]
	if strikethrough != nil {
		highlight.strikethrough = true
	} else {
		highlight.strikethrough = false
	}

	reverse := hl["reverse"]
	if reverse != nil {
		highlight.reverse = true
	} else {
		highlight.reverse = false
	}

	fg, ok := hl["foreground"]
	if ok {
		rgba := calcColor(util.ReflectToInt(fg))
		highlight.foreground = rgba
	}
	if highlight.foreground == nil {
		highlight.foreground = s.ws.foreground
	}

	bg, ok := hl["background"]
	if ok {
		rgba := calcColor(util.ReflectToInt(bg))
		highlight.background = rgba
	}
	if highlight.background == nil {
		highlight.background = s.ws.background
	}

	sp, ok := hl["special"]
	if ok {
		rgba := calcColor(util.ReflectToInt(sp))
		highlight.special = rgba
	}

	bl, ok := hl["blend"]
	if ok {
		highlight.blend = util.ReflectToInt(bl)
	}

	// TODO: brend, ok := hl["blend"]

	return &highlight
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

func (s *Screen) gridClear(args []interface{}) {
	var gridid gridId
	for _, arg := range args {
		gridid = util.ReflectToInt(arg.([]interface{})[0])
		if isSkipGlobalId(gridid) {
			continue
		}

		win, ok := s.getWindow(gridid)
		if !ok {
			continue
		}
		win.content = make([][]*Cell, win.rows)
		win.contentMask = make([][]bool, win.rows)
		win.contentMaskOld = make([][]bool, win.rows)
		win.lenLine = make([]int, win.rows)
		win.lenContent = make([]int, win.rows)

		for i := 0; i < win.rows; i++ {
			win.content[i] = make([]*Cell, win.cols)
			win.contentMask[i] = make([]bool, win.cols)
			win.contentMaskOld[i] = make([]bool, win.cols)
			for j := 0; j < win.cols; j++ {
				win.contentMask[i][j] = true
			}
			win.lenContent[i] = win.cols - 1
		}
		win.queueRedrawAll()
	}
}

func (s *Screen) gridLine(args []interface{}) {
	var win *Window
	var ok bool
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		row := util.ReflectToInt(arg.([]interface{})[1])
		colStart := util.ReflectToInt(arg.([]interface{})[2])

		if isSkipGlobalId(gridid) {
			continue
		}

		if win == nil || win.grid != gridid {
			win, ok = s.getWindow(gridid)
			if !ok {
				continue
			}
		}

		win.updateGridContent(row, colStart, arg.([]interface{})[3].([]interface{}))
	}
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
	for j := 0; j <= w.cols-1; j++ {
		cell := line[j]
		if cell == nil {
			w.contentMask[row][j] = false
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

func (s *Screen) gridScroll(args []interface{}) {
	var gridid gridId
	var rows int
	for _, arg := range args {
		gridid = util.ReflectToInt(arg.([]interface{})[0])
		if isSkipGlobalId(gridid) {
			continue
		}

		win, ok := s.getWindow(gridid)
		if !ok {
			continue
		}
		win.scrollRegion[0] = util.ReflectToInt(arg.([]interface{})[1])     // top
		win.scrollRegion[1] = util.ReflectToInt(arg.([]interface{})[2]) - 1 // bot
		win.scrollRegion[2] = util.ReflectToInt(arg.([]interface{})[3])     // left
		win.scrollRegion[3] = util.ReflectToInt(arg.([]interface{})[4]) - 1 // right
		rows = util.ReflectToInt(arg.([]interface{})[5])
		win.scroll(rows)
	}
}

func (w *Window) scroll(count int) {
	top := w.scrollRegion[0]
	bot := w.scrollRegion[1]
	left := w.scrollRegion[2]
	right := w.scrollRegion[3]
	content := w.content
	lenLine := w.lenLine
	lenContent := w.lenContent

	if top == 0 && bot == 0 && left == 0 && right == 0 {
		top = 0
		bot = w.rows - 1
		left = 0
		right = w.cols - 1
	}

	if count > 0 {
		for row := top; row <= bot-count; row++ {
			if len(content) <= row+count {
				continue
			}

			// copy(content[row], content[row+count])
			for col := left; col <= right; col++ {
				content[row][col] = content[row+count][col]
				w.contentMask[row][col] = w.contentMask[row+count][col]
			}
			lenLine[row] = lenLine[row+count]
			lenContent[row] = lenContent[row+count]
		}
		for row := bot - count + 1; row <= bot; row++ {
			for col := left; col <= right; col++ {
				content[row][col] = nil
				w.contentMask[row][col] = false
			}
		}
	} else {
		for row := bot; row >= top-count; row-- {
			if len(content) <= row {
				continue
			}

			// copy(content[row], content[row+count])
			for col := left; col <= right; col++ {
				content[row][col] = content[row+count][col]
				w.contentMask[row][col] = w.contentMask[row+count][col]
			}
			lenLine[row] = lenLine[row+count]
			lenContent[row] = lenContent[row+count]
		}
		for row := top; row < top-count; row++ {
			for col := left; col <= right; col++ {
				content[row][col] = nil
				w.contentMask[row][col] = false
			}
		}
	}

	// Suppresses flickering during smooth scrolling
	if w.scrollPixels[1] != 0 {
		w.scrollPixels[1] = 0
	}

	w.queueRedraw(left, top, (right - left + 1), (bot - top + 1))
}

func (w *Window) update() {
	w.redrawMutex.Lock()

	if w == nil {
		w.redrawMutex.Unlock()
		return
	}
	font := w.getFont()

	start := w.queueRedrawArea[1]
	end := w.queueRedrawArea[3]
	if w.scrollPixels[1] != 0 {
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
		} else {
			for j, cm := range w.contentMask[i] {
				mask := cm || w.contentMaskOld[i][j]
				if mask && !isCreateRect {
					start = j
					isCreateRect = true
				}
				if (!mask && isCreateRect) || (j >= len(w.contentMask[i])-1 && isCreateRect) {
					jj := j
					if j >= len(w.contentMask[i])-1 && isCreateRect {
						jj++
					}
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

func (s *Screen) update() {
	s.windows.Range(func(grid, winITF interface{}) bool {
		win := winITF.(*Window)
		// if grid is dirty, we remove this grid
		if win.isGridDirty {
			// if win.queueRedrawArea[2] > 0 || win.queueRedrawArea[3] > 0 {
			// 	// If grid has an update area even though it has a dirty flag,
			// 	// it will still not be removed as a valid grid
			// 	win.isGridDirty = false
			// } else {
			// 	// Remove dirty grid
			// 	win.hide()
			// 	s.windows.Delete(grid)
			// }
			win.hide()
			win.deleteExternalWin()
			s.windows.Delete(grid)
		}
		if win != nil {
			// Fill entire background if background color changed
			if !win.background.equals(s.ws.background) {
				win.background = s.ws.background.copy()
				win.fill()
			}
			win.update()

			// Update markdown preview
			if win.grid != 1 && !win.isMsgGrid && s.ws.markdown != nil {
				s.ws.markdown.updatePos()
			}
		}

		return true
	})
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

	for x := col; x <= col+cols; x++ {
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
					float64((y)*font.lineHeight+w.scrollPixels[1]+w.scrollPixels2),
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

func (w *Window) drawTextWithCache(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	wsfont := w.getFont()
	// font := p.Font()
	line := w.content[y]
	chars := map[*Highlight][]int{}
	specialChars := []int{}

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

		if editor.config.Editor.DisableLigatures {
			w.drawTextInPosWithCache(
				p,
				core.NewQPointF3(
					float64(x)*wsfont.truewidth,
					float64(y*wsfont.lineHeight+w.scrollPixels[1]+w.scrollPixels2),
				),
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

	// If config.DisableLigatures is false
	if !editor.config.Editor.DisableLigatures {

		for highlight, colorSlice := range chars {
			var buffer bytes.Buffer
			slice := colorSlice[:]

			pos := col
			isIndentationWhitespace := true
			for x := col; x <= col+cols; x++ {
				if len(slice) == 0 {
					break
				}
				index := slice[0]

				if x < index {
					if isIndentationWhitespace {
						pos++
					} else {
						buffer.WriteString(" ")
					}
					continue
				}

				if x == index {
					buffer.WriteString(line[x].char)
					slice = slice[1:]
					isIndentationWhitespace = false
				}
			}

			text := buffer.String()
			w.drawTextInPosWithCache(
				p,
				core.NewQPointF3(
					float64(pos)*wsfont.truewidth,
					float64(y*wsfont.lineHeight+w.scrollPixels[1]+w.scrollPixels2),
				),
				text,
				highlight,
				true,
			)
		}
	}

	for _, x := range specialChars {
		if line[x] == nil || line[x].char == " " {
			continue
		}
		w.drawTextInPosWithCache(
			p,
			core.NewQPointF3(
				float64(x)*wsfont.truewidth,
				float64(y*wsfont.lineHeight+w.scrollPixels[1]+w.scrollPixels2),
			),
			line[x].char,
			line[x].highlight,
			false,
		)
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
	// p.DrawImage9(
	// 	x, y,
	// 	image,
	// 	0, 0,
	// 	0, 0,
	// 	core.Qt__AutoColor,
	// )
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
		Y := float64(0*font.lineHeight+w.scrollPixels[1]) + float64(font.ascent)*0.65 + float64(font.lineSpace/2)
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
			int(float64((0+1)*font.lineHeight+w.scrollPixels[1]+w.scrollPixels2))-weight,
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
		Y := float64(0*font.lineHeight+w.scrollPixels[1]+w.scrollPixels2) + float64(font.ascent+descent*0.3) + float64(font.lineSpace/2) + space
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

	width := float64(len(text)) * font.italicWidth
	fg := highlight.fg()
	if !isNormalWidth {
		width = math.Ceil(w.s.runeTextWidth(font, text))
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

func (w *Window) drawText(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	wsfont := w.getFont()

	p.SetFont(wsfont.fontNew)

	line := w.content[y]
	chars := map[*Highlight][]int{}
	specialChars := []int{}

	for x := col; x <= col+cols; x++ {
		if x >= len(line) {
			continue
		}
		if line[x] == nil {
			continue
		}
		if line[x].char == " " {
			continue
		}
		if line[x].char == "" {
			continue
		}
		if !line[x].normalWidth {
			specialChars = append(specialChars, x)
			continue
		}

		if editor.config.Editor.DisableLigatures {
			w.drawTextInPos(
				p,
				core.NewQPointF3(
					float64(x)*wsfont.truewidth,
					float64(y*wsfont.lineHeight+wsfont.shift+w.scrollPixels[1]+w.scrollPixels2),
				),
				line[x].char,
				line[x].highlight,
				true)
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

	pointF := core.NewQPointF3(
		float64(col)*wsfont.truewidth,
		float64((y)*wsfont.lineHeight+wsfont.shift+w.scrollPixels[1]+w.scrollPixels2),
	)

	// If config.DisableLigatures is false
	if !editor.config.Editor.DisableLigatures {

		for highlight, colorSlice := range chars {
			var buffer bytes.Buffer
			slice := colorSlice[:]
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
			// if text != "" {
			// 	fg := highlight.fg()
			// 	if fg != nil {
			// 		p.SetPen2(fg.QColor())
			// 	}
			// 	if highlight.bold {
			// 		// font.SetBold(true)
			// 		font.SetWeight(wsfont.fontNew.Weight() + 25)
			// 	} else {
			// 		// font.SetBold(false)
			// 		font.SetWeight(wsfont.fontNew.Weight())
			// 	}
			// 	if highlight.italic {
			// 		font.SetItalic(true)
			// 	} else {
			// 		font.SetItalic(false)
			// 	}
			// 	p.DrawText(pointF, text)
			// }
			w.drawTextInPos(p, pointF, text, highlight, true)
		}
	}

	if len(specialChars) >= 1 {
		if w.s.ws.fontwide != nil && w.font == nil && w.s.ws.fontwide.fontNew != nil {
			p.SetFont(w.s.ws.fontwide.fontNew)
			// font = p.Font()
		}
		for _, x := range specialChars {
			if line[x] == nil || line[x].char == " " {
				continue
			}
			pointF.SetX(float64(x) * wsfont.truewidth)
			pointF.SetY(float64((y)*wsfont.lineHeight + wsfont.shift + w.scrollPixels[1] + w.scrollPixels2))

			// p.SetPen2(fg.QColor())

			// if line[x].highlight.bold {
			// 	// font.SetBold(true)
			// 	font.SetWeight(wsfont.fontNew.Weight() + 25)
			// } else {
			// 	// font.SetBold(false)
			// 	font.SetWeight(wsfont.fontNew.Weight())
			// }
			// if line[x].highlight.italic {
			// 	font.SetItalic(true)
			// } else {
			// 	font.SetItalic(false)
			// }
			// p.DrawText(pointF, line[x].char)
			w.drawTextInPos(p, pointF, line[x].char, line[x].highlight, false)
		}
		if w.s.ws.fontwide != nil && w.font == nil {
			p.SetFont(w.getFont().fontNew)
		}
	}
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
		// font.SetBold(true)
		font.SetWeight(wsfont.fontNew.Weight() + 25)
	} else {
		// font.SetBold(false)
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
	} else if !editor.config.Editor.CachedDrawing {
		w.drawText(p, y, col, cols)
		w.drawTextDecoration(p, y, col, cols)
	} else {
		w.drawTextWithCache(p, y, col, cols)
		w.drawTextDecorationWithCache(p, y, col, cols)
	}
}

func (w *Window) drawTextDecoration(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	line := w.content[y]
	font := w.getFont()
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
			Y := float64(y*font.lineHeight+w.scrollPixels[1]) + float64(font.ascent)*0.65 + float64(font.lineSpace/2)
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
				int(float64((y+1)*font.lineHeight+w.scrollPixels[1]+w.scrollPixels2))-weight,
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
			Y := float64(y*font.lineHeight+w.scrollPixels[1]+w.scrollPixels2) + float64(font.ascent+descent*0.3) + float64(font.lineSpace/2) + space
			Y2 := Y + amplitude*math.Sin(0)
			point := core.NewQPointF3(start, Y2)
			path := gui.NewQPainterPath2(point)
			for i := int(point.X()); i <= int(end); i++ {
				Y2 = Y + amplitude*math.Sin(2*math.Pi*freq*float64(i)/font.truewidth+phase)
				path.LineTo(core.NewQPointF3(float64(i), Y2))
			}
			p.DrawPath(path)
		}
	}
}

func (w *Window) drawTextDecorationWithCache(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	line := w.content[y]
	font := w.getFont()
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
				float64(y*font.lineHeight)+float64(w.scrollPixels[1])+float64(w.scrollPixels2),
			),
			image,
		)

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

func (s *Screen) runeTextWidth(font *Font, text string) float64 {
	cjk := 0
	ascii := 0
	var buffer bytes.Buffer
	for _, c := range []rune(text) {
		if isCJK(c) {
			cjk++
		} else if c <= 127 {
			ascii++
		} else {
			buffer.WriteString(string(c))
		}
	}

	r := buffer.String()
	width := (font.truewidth)*float64(ascii) + (font.truewidth)*float64(cjk)*2
	if r == "" {
		width += font.fontMetrics.HorizontalAdvance(r, -1)
	}
	if width == 0 {
		width = font.truewidth * 2
	}

	return width
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

func (s *Screen) windowPosition(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		id := arg.([]interface{})[1].(nvim.Window)
		row := util.ReflectToInt(arg.([]interface{})[2])
		col := util.ReflectToInt(arg.([]interface{})[3])

		if isSkipGlobalId(gridid) {
			continue
		}

		win, ok := s.getWindow(gridid)
		if !ok {
			continue
		}

		win.updateMutex.Lock()
		win.id = id
		win.pos[0] = col
		win.pos[1] = row
		win.updateMutex.Unlock()
		win.move(col, row)
		win.show()

		// // for goneovim internal use
		// win.setBufferName()
		// win.setFiletype()
	}
}

func (s *Screen) gridDestroy(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		if isSkipGlobalId(gridid) {
			continue
		}

		// NOTE: what should we actually do in the event ??
		win, ok := s.getWindow(gridid)
		if !ok {
			continue
		}
		win.isGridDirty = true
	}

	// Redraw each displayed window.Because shadows leave dust before and after float window drawing.
	s.windows.Range(func(_, winITF interface{}) bool {
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
		if win.isGridDirty {
			return true
		}
		if win.isShown() {
			win.queueRedrawAll()
		}

		return true
	})
}

func (w *Window) deleteExternalWin() {
	if w.extwin != nil {
		w.extwin.Hide()
		w.extwin = nil
	}
}

func (s *Screen) windowFloatPosition(args []interface{}) {
	// A workaround for the problem that the position of the float window,
	// which is created as a tooltip suggested by LSP, is not the correct
	// position in multigrid ui api.
	isExistPopupmenu := false
	if s.ws.mode == "insert" && !editor.config.Editor.ExtPopupmenu {
		s.windows.Range(func(_, winITF interface{}) bool {
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
			if win.isPopupmenu {
				isExistPopupmenu = true
			}

			return true
		})
	}

	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		if isSkipGlobalId(gridid) {
			continue
		}

		win, ok := s.getWindow(gridid)
		if !ok {
			continue
		}

		win.updateMutex.Lock()
		win.id = arg.([]interface{})[1].(nvim.Window)
		win.updateMutex.Unlock()
		win.anchor = arg.([]interface{})[2].(string)
		anchorGrid := util.ReflectToInt(arg.([]interface{})[3])
		anchorRow := int(util.ReflectToFloat(arg.([]interface{})[4]))
		anchorCol := int(util.ReflectToFloat(arg.([]interface{})[5]))
		// focusable := arg.([]interface{})[6]

		if isExistPopupmenu && win.id != -1 {
			anchorGrid = s.ws.cursor.gridid
		}

		win.SetParent(editor.widget)

		win.propMutex.Lock()
		win.isFloatWin = true

		if win.isExternal {
			win.deleteExternalWin()
			win.isExternal = false
		}
		win.propMutex.Unlock()

		anchorwin, ok := s.getWindow(anchorGrid)
		if !ok {
			continue
		}

		anchorposx := anchorwin.pos[0]
		anchorposy := anchorwin.pos[1]

		anchorwin.propMutex.Lock()
		anchorwinIsExternal := anchorwin.isExternal
		anchorwin.propMutex.Unlock()

		if anchorwinIsExternal {
			win.SetParent(anchorwin)
			anchorposx = 0
			anchorposy = 0
		}

		// In multigrid ui, the completion float window on the message window appears to be misaligned.
		// Therefore, a hack to workaround this problem is implemented on the GUI front-end side.
		// This workaround assumes that the anchor window for the completion window on the message window is always a global grid.
		pumInMsgWin := false
		if anchorwin.grid == 1 && !(s.cursor[0] == 0 && s.cursor[1] == 0) && win.id == -1 {
			cursorgridwin, ok := s.getWindow(s.ws.cursor.gridid)
			if !ok {
				continue
			}
			if cursorgridwin.isMsgGrid {
				anchorwin = cursorgridwin
				anchorRow = cursorgridwin.pos[0]
				anchorposx = cursorgridwin.pos[0]
				anchorposy = cursorgridwin.pos[1]
			}
			pumInMsgWin = true
		}

		var x, y int
		switch win.anchor {
		case "NW":
			x = anchorposx + anchorCol
			y = anchorposy + anchorRow
		case "NE":
			x = anchorposx + anchorCol - win.cols
			y = anchorposy + anchorRow
		case "SW":
			x = anchorposx + anchorCol
			// In multigrid ui, the completion float window position information is not correct.
			// Therefore, we implement a hack to compensate for this.
			// ref: src/nvim/popupmenu.c:L205-, L435-
			if win.id == -1 && !pumInMsgWin {

				row := 0
				contextLine := 0
				if anchorwin.rows-s.cursor[0] >= 2 {
					contextLine = 2
				} else {
					contextLine = anchorwin.rows - s.cursor[0]
				}
				if anchorposy+s.cursor[0] >= win.rows+contextLine {
					row = anchorRow + win.rows
				} else {
					row = -anchorposy
				}
				y = anchorposy + row
			} else {
				y = anchorposy + anchorRow - win.rows
			}
		case "SE":
			x = anchorposx + anchorCol - win.cols
			y = anchorposy + anchorRow - win.rows
		}

		if x < 0 {
			x = 0
		}

		win.pos[0] = x
		win.pos[1] = y

		win.move(x, y)
		win.setShadow()
		win.show()

		// Redraw anchor window.Because shadows leave dust before and after float window drawing.
		anchorwin.queueRedrawAll()
	}
}

func (s *Screen) windowExternalPosition(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		// winid := arg.([]interface{})[1].(nvim.Window)

		s.windows.Range(func(_, winITF interface{}) bool {
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

			if win.grid == gridid && !win.isExternal {
				win.isExternal = true

				extwin := createExternalWin()
				win.SetParent(extwin)
				extwin.ConnectKeyPressEvent(editor.keyPress)
				win.background = s.ws.background.copy()
				extwin.SetAutoFillBackground(true)
				p := gui.NewQPalette()
				p.SetColor2(gui.QPalette__Background, s.ws.background.QColor())
				extwin.SetPalette(p)

				extwin.Show()
				win.extwin = extwin
				font := win.getFont()
				win.extwin.ConnectMoveEvent(func(ev *gui.QMoveEvent) {
					if win.extwin == nil {
						return
					}
					if ev == nil {
						return
					}
					pos := ev.Pos()
					if pos == nil {
						return
					}
					gPos := editor.window.Pos()
					win.pos[0] = int(float64(pos.X()-gPos.X()) / font.truewidth)
					win.pos[1] = int(float64(pos.Y()-gPos.Y()) / float64(font.lineHeight))
				})
				width := int(math.Ceil(float64(win.cols) * font.truewidth))
				height := win.rows * font.lineHeight
				win.setGridGeometry(width, height)
				win.setResizableForExtWin()
				win.move(win.pos[0], win.pos[1])
			}

			return true
		})

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

func (s *Screen) windowHide(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		if isSkipGlobalId(gridid) {
			continue
		}
		win, ok := s.getWindow(gridid)
		if !ok {
			continue
		}
		win.hide()
	}
}

func (s *Screen) msgSetPos(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		row := util.ReflectToInt(arg.([]interface{})[1])
		scrolled := arg.([]interface{})[2].(bool)
		// TODO We should imprement to drawing msgSepChar
		// sepChar := arg.([]interface{})[3].(string)

		win, ok := s.getWindow(gridid)
		if !ok {
			continue
		}
		win.isMsgGrid = true
		win.pos[1] = row
		win.move(win.pos[0], win.pos[1])
		win.show()
		if scrolled {
			win.Raise() // Fix #111
		}
	}
}

func (s *Screen) windowClose() {
}

func (s *Screen) setColor() {
	s.tooltip.SetStyleSheet(
		fmt.Sprintf(
			" * {background-color: %s; text-decoration: underline; color: %s; }",
			editor.colors.selectedBg.String(),
			editor.colors.fg.String(),
		),
	)
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

	return win
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
	// w.s.ws.cursor.widget.SetParent(w)
	w.s.ws.cursor.widget.Hide()
	w.s.ws.cursor.widget.Show()
	if !w.isExternal {
		editor.window.Raise()
	} else if w.isExternal {
		w.extwin.Raise()
	}
	w.s.ws.cursor.widget.Raise()
}

func (w *Window) show() {
	w.fill()
	w.Show()
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

func (w *Window) fill() {
	for i := 0; i < len(w.lenContent); i++ {
		w.lenContent[i] = w.cols
		for j, _ := range w.contentMask[i] {
			w.contentMask[i][j] = true
		}
	}
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
	res := 0
	// window position is based on cols, rows of global font setting
	// font := w.getFont()
	font := w.s.font
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

func isSkipGlobalId(id gridId) bool {
	if editor.config.Editor.SkipGlobalId {
		if id == 1 {
			return true
		}
	}

	return false
}
