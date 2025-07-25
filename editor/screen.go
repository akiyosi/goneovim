package editor

import (
	"bytes"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/akiyosi/goneovim/util"
	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
	"github.com/akiyosi/qt/widgets"
	"github.com/bluele/gcache"
	"github.com/neovim/go-client/nvim"
)

var globalOrder int

// Screen is the main editor area
type Screen struct {
	cache             Cache
	bgcache           Cache
	tooltip           *IMETooltip
	font              *Font
	fallbackfonts     []*Font
	fontwide          *Font
	fallbackfontwides []*Font
	hlAttrDef         map[int]*Highlight
	widget            *widgets.QWidget
	ws                *Workspace
	highlightGroup    map[string]int
	windows           sync.Map
	name              string
	cursor            [2]int
	height            int
	width             int
	resizeCount       uint
	topLevelGrid      int
	lastGridLineGrid  int
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
		cache:          newCache(),
		bgcache:        newBrushCache(),
	}

	widget.ConnectMousePressEvent(screen.mousePressEvent)
	widget.ConnectMouseReleaseEvent(screen.mouseEvent)
	widget.ConnectMouseMoveEvent(screen.mouseEvent)

	return screen
}

func (s *Screen) initInputMethodWidget() {
	tooltip := NewIMETooltip(s.widget, 0)
	tooltip.SetVisible(false)
	tooltip.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	tooltip.ConnectPaintEvent(tooltip.paint)
	tooltip.s = s
	s.tooltip = tooltip
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

	newCols := int(float64(s.width) / s.font.cellwidth)
	newRows := s.height / s.font.lineHeight

	// Adjust the position of the message grid.
	s.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)
		if win == nil {
			return true
		}
		if win.isMsgGrid {
			win.move(win.pos[0], win.pos[1])
		}

		return true
	})

	isNeedTryResize := (newCols != ws.cols || newRows != ws.rows)
	if !isNeedTryResize {
		return
	}

	ws.cols = newCols
	ws.rows = newRows

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

	s.uiTryResize(newCols, newRows)

}

func (s *Screen) uiTryResize(cols, rows int) {
	if cols <= 0 || rows <= 0 {
		return
	}

	ws := s.ws
	done := make(chan error, 5)
	var result error
	go func() {
		result = ws.nvim.TryResizeUI(cols, rows)
		done <- result
	}()
	select {
	case <-done:
	case <-time.After(s.waitTime() * time.Millisecond):
	}
}

func (s *Screen) getGrid(wid nvim.Window) (grid *Window, ok bool) {
	s.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)

		if win == nil {
			return true
		}
		if win.grid == 1 {
			return true
		}

		win.updateMutex.RLock()
		id := win.id
		win.updateMutex.RUnlock()

		if id == wid {
			grid = win
			ok = true
		}

		return true
	})

	return grid, ok
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

func (s *Screen) deleteWindow(grid int) {
	winITF, loaded := s.windows.LoadAndDelete(grid)
	if loaded {
		win := winITF.(*Window)
		if win != nil {
			win.DestroyWindow()
		}
	}
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

	// fontfamily := parts[0]
	// height := 14

	// for _, p := range parts[1:] {
	// 	if strings.HasPrefix(p, "h") {
	// 		var err error
	// 		height, err = strconv.Atoi(p[1:])
	// 		if err != nil {
	// 			return
	// 		}
	// 	} else if strings.HasPrefix(p, "w") {
	// 		var err error
	// 		width, err := strconv.Atoi(p[1:])
	// 		if err != nil {
	// 			return
	// 		}
	// 		height = 2 * width
	// 	}
	// }

	oldWidth := float64(win.cols) * win.getFont().cellwidth
	oldHeight := win.rows * win.getFont().lineHeight
	win.width = oldWidth
	win.height = oldHeight
	win.localWindows = &[4]localWindow{}

	win.fallbackfonts = nil
	s.ws.parseAndApplyFont(updateStr, &win.font, &win.fallbackfonts)

	if win.font == nil {
		return
	}

	// Calculate new cols, rows of current grid
	newCols := int(oldWidth / win.font.cellwidth)
	newRows := oldHeight / win.font.lineHeight

	// Cache
	cache := win.cache
	if cache == (Cache{}) {
		cache := newCache()
		win.cache = cache
	} else {
		win.paintMutex.Lock()
		win.cache.purge()
		win.paintMutex.Unlock()
	}

	_ = s.ws.nvim.TryResizeUIGrid(win.grid, newCols, newRows)

	s.ws.cursor.font = win.getFont()
	s.ws.cursor.fallbackfonts = win.fallbackfonts

	if win.isExternal {
		width := int(float64(newCols)*win.font.cellwidth) + EXTWINBORDERSIZE*2
		height := newRows*win.font.lineHeight + EXTWINBORDERSIZE*2
		win.extwin.Resize2(width, height)
	}
}

func (s *Screen) purgeTextCacheForWins() {
	if !editor.config.Editor.CachedDrawing {
		return
	}
	s.cache.purge()
	s.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)
		if win == nil {
			return true
		}
		if win.font == nil {
			return true
		}
		cache := win.cache
		if cache == (Cache{}) {
			return true
		}
		win.paintMutex.Lock()
		win.cache.purge()
		win.paintMutex.Unlock()
		return true
	})
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
		if win.isFloatWin {
			return true
		}
		if !win.IsVisible() {
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

func (s *Screen) mousePressEvent(event *gui.QMouseEvent) {
	if !s.ws.isMouseEnabled {
		return
	}

	s.mouseEvent(event)
	if !editor.config.Editor.ClickEffect {
		return
	}

	// TODO: If a float window exists, the process of selecting it is necessary.

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
	if !s.ws.isMouseEnabled {
		return
	}

	var targetwin *Window

	// If a mouse event has already occurred and the mouse is being
	// used to drag content, the window being operated on will be the target window.
	s.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)
		if win == nil {
			return true
		}
		if !win.IsVisible() {
			return true
		}
		if win.lastMouseEvent != nil && win.lastMouseEvent.event != nil {
			if win.lastMouseEvent.event.Type() == core.QEvent__MouseMove {
				targetwin = win
				return false
			}
		}

		return true
	})

	if targetwin == nil {

		// If there is an externalized float window and
		// the mouse pointer is in that window,
		// make that window the target window.
		s.windows.Range(func(_, winITF interface{}) bool {
			win := winITF.(*Window)
			if win == nil {
				return true
			}
			if !win.IsVisible() {
				return true
			}
			if win.grid == 1 {
				return true
			}
			if win.isMsgGrid {
				return true
			}
			if win.isExternal {
				if win.grid == s.ws.cursor.gridid {
					targetwin = win
					return false
				}
			}

			return true
		})

		// If a float window exists, the float window with the smallest size
		// that covers the position of the mouse event that occurred
		// is used as the target window.
		if targetwin == nil {
			s.windows.Range(func(_, winITF interface{}) bool {
				win := winITF.(*Window)
				if win == nil {
					return true
				}
				if !win.IsVisible() {
					return true
				}
				if win.grid == 1 {
					return true
				}
				if win.isMsgGrid {
					return true
				}
				if win.isFloatWin && !win.isExternal {
					if win.Geometry().Contains(event.Pos(), true) {
						if targetwin != nil {
							if targetwin.Geometry().Contains2(win.Geometry(), true) {
								targetwin = win
							}
						} else {
							if win.Geometry().Contains(event.Pos(), true) {
								targetwin = win
							}
						}
					}
				}

				return true
			})
		}

		// If none of the above processes apply,
		// the target window is a normal window that covers the position
		// where the mouse event occurs.
		if targetwin == nil {
			s.windows.Range(func(_, winITF interface{}) bool {
				win := winITF.(*Window)
				if win == nil {
					return true
				}
				if !win.IsVisible() {
					return true
				}
				if win.isFloatWin || win.isExternal {
					return true
				}

				// Judgment of the global grid is performed in
				// a separate process when there is no corresponding result
				// in the judgment of the coverage of mouse click coordinates
				// in all grids.
				if win.grid == 1 {
					return true
				}

				if win.Geometry().Contains(event.Pos(), true) {
					targetwin = win
					return false
				}

				return true
			})
		}

		if targetwin == nil {
			s.windows.Range(func(_, winITF interface{}) bool {
				win := winITF.(*Window)
				if win == nil {
					return true
				}
				if win.grid == 1 {
					if win.Geometry().Contains(event.Pos(), true) {
						targetwin = win
						return false
					}
				}

				return true
			})
		}
	}

	if targetwin == nil {
		return
	}

	// Reset the mouse event state of each window.
	s.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)
		if win == nil {
			return true
		}
		if win.lastMouseEvent != nil {
			win.lastMouseEvent.event = nil
		}

		return true
	})

	var localpos *core.QPointF
	if targetwin.isExternal {
		localpos = core.NewQPointF3(
			event.ScreenPos().X()-float64(targetwin.extwin.Pos().X()+targetwin.Pos().X()),
			event.ScreenPos().Y()-float64(targetwin.extwin.Pos().Y()+targetwin.Pos().Y()),
		)
	} else {
		// Fixed mouse events handling in relation to #316#issuecomment-1039978355 fixes.
		offsetX := float64(targetwin.Pos().X())
		offsetY := float64(targetwin.Pos().Y())

		localpos = core.NewQPointF3(
			event.LocalPos().X()-offsetX,
			event.LocalPos().Y()-offsetY,
		)
	}

	targetwin.mouseEvent(
		gui.NewQMouseEvent3(
			event.Type(),
			localpos,
			event.WindowPos(),
			event.ScreenPos(),
			event.Button(),
			event.Buttons(),
			event.Modifiers(),
		),
	)
}

func (s *Screen) gridResize(args []interface{}) {
	var gridid gridId
	var rows, cols int
	for _, arg := range args {
		gridid = util.ReflectToInt(arg.([]interface{})[0])
		cols = util.ReflectToInt(arg.([]interface{})[1])
		rows = util.ReflectToInt(arg.([]interface{})[2])

		// for debug
		if isSkipGlobalId(gridid) {
			continue
		}

		// resize neovim's window
		s.resizeWindow(gridid, cols, rows)

		win, ok := s.getWindow(gridid)
		if !ok {
			continue
		}
		if win.isFloatWin && !win.isMsgGrid {
			win.setFloatWindowPosition()
		}
		win.move(win.pos[0], win.pos[1], win.anchorwin)
		win.show()

		// If events related to the global grid are included
		// Determine to resize the application window
		if s.name != "minimap" && s.ws.uiAttached {
			if win.grid == 1 {
				if !editor.isBindNvimSizeToAppwin {
					continue
				}
				if !(s.ws.cols == cols && s.ws.rows == rows) {
					s.ws.cols = cols
					s.ws.rows = rows
					s.ws.updateApplicationWindowSize(cols, rows)
				}
			}
			if win.grid != 1 && !win.isMsgGrid {
				if !editor.isBindNvimSizeToAppwin {
					editor.isBindNvimSizeToAppwin = true
					newCols := s.ws.cols
					if editor.isSetColumns {
						newCols = editor.initialColumns
					}
					newRows := s.ws.rows
					if editor.isSetLines {
						newRows = editor.initialLines
					}
					s.uiTryResize(newCols, newRows)

				}
			}
		}
	}
}

func (s *Screen) resizeWindow(gridid gridId, cols int, rows int) {
	win, _ := s.getWindow(gridid)

	if win != nil {
		if win.cols == cols && win.rows == rows {
			return
		}
	}

	if win != nil && win.snapshot != nil {
		win.dropScreenSnapshot()
	}

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

	for i := 0; i < rows; i++ {
		// if win != nil {
		// 	if i < len(win.content) {
		// 		// lenLine[i] = win.lenLine[i]
		// 		lenContent[i] = win.lenContent[i]
		// 		if i < len(win.lenOldContent) {
		// 			lenOldContent[i] = win.lenOldContent[i]
		// 		}
		// 	}
		// }

		for j := 0; j < cols; j++ {

			if win != nil {
				if i < len(win.content) && j < len(win.content[i]) {
					if i < len(win.contentMaskOld) && j < len(win.contentMaskOld[i]) {
						contentMaskOld[i][j] = win.contentMaskOld[i][j]
					}
					contentMask[i][j] = win.contentMask[i][j]
					content[i][j] = win.content[i][j]
				}
			}
			if content[i][j] == nil {
				content[i][j] = &Cell{
					highlight:   s.hlAttrDef[0],
					char:        " ",
					normalWidth: false,
					covered:     false,
				}
			}
		}
	}

	if win == nil {
		win = s.newWindowGird(gridid)
	}

	win.redrawMutex.Lock()

	// winOldCols := win.cols
	// winOldRows := win.rows

	win.lenLine = lenLine
	win.lenContent = lenContent
	win.lenOldContent = lenOldContent
	win.content = content
	win.contentMask = contentMask
	win.contentMaskOld = contentMaskOld
	win.cols = cols
	win.rows = rows
	win.redrawMutex.Unlock()

	// s.resizeIndependentFontGrid(win, winOldCols, winOldRows)

	font := win.getFont()

	width := int(math.Ceil(float64(cols) * font.cellwidth))
	height := rows * font.lineHeight

	win.setGridGeometry(width, height)

	// win.move(win.pos[0], win.pos[1])
	// win.show()

	win.queueRedrawAll()
}

func (s *Screen) newWindowGird(gridid int) (win *Window) {
	win = newWindow()

	win.s = s
	s.storeWindow(gridid, win)

	win.SetParent(s.widget)

	win.grid = gridid

	// set scroll
	if s.name != "minimap" {
		win.ConnectWheelEvent(win.wheelEvent)
	}

	s.ws.cursor.raise()

	return
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
					w.localWindows[2].localWidth = w.width + float64(oldCols)*win.getFont().cellwidth
				}
				newWidth := w.localWindows[2].localWidth - (float64(win.cols) * win.getFont().cellwidth)
				newCols := int(newWidth / w.font.cellwidth)
				if newCols != w.cols {
					_ = s.ws.nvim.TryResizeUIGrid(w.grid, newCols, w.rows)
					w.width = float64(newCols) * w.getFont().cellwidth
					w.localWindows[0].isResized = false
				}
			}

			// left window is gridfont window
			// calculate win window posision aa w window coordinate
			var resizeflag bool
			winPosX := float64(win.pos[0]) * win.s.font.cellwidth
			rightWindowPos1 := float64(w.cols)*w.getFont().cellwidth + float64(w.pos[0]+1-deltaCols+1)*win.s.font.cellwidth
			rightWindowPos2 := float64(w.cols-1)*w.getFont().cellwidth + float64(w.pos[0]+1-deltaCols+1)*win.s.font.cellwidth
			rightWindowPos := int(float64(w.cols)*w.getFont().cellwidth/win.s.font.cellwidth) + w.pos[0] + 1 - deltaCols + 1
			if win.s.font.cellwidth < w.getFont().cellwidth {
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
					w.localWindows[0].localWidth = w.width + float64(oldCols)*win.getFont().cellwidth
				}
				newWidth := w.localWindows[0].localWidth - (float64(win.cols) * win.getFont().cellwidth)
				newCols := int(newWidth / w.font.cellwidth)
				if newCols != w.cols {
					_ = s.ws.nvim.TryResizeUIGrid(w.grid, newCols, w.rows)
					w.width = float64(newCols) * w.getFont().cellwidth
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
			// calculate win window posision aa w window coordinate
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

		x := util.ReflectToInt(arg.([]interface{})[1])
		y := util.ReflectToInt(arg.([]interface{})[2])

		if isSkipGlobalId(gridid) {
			continue
		}

		win, ok := s.getWindow(gridid)
		if !ok {
			continue
		}

		// Suppress unnecessary detours of the smooth cursor.
		if win.isMsgGrid && x == 0 && y == 0 {
			continue
		}
		if win.isMsgGrid && editor.config.Editor.ExtCmdline {
			continue
		}

		s.cursor[0] = x
		s.cursor[1] = y

		if !win.isMsgGrid {
			s.ws.cursor.prevGridid = s.ws.cursor.bufferGridid
		}
		if s.ws.cursor.gridid != win.grid {
			if !win.isMsgGrid {
				s.ws.cursor.bufferGridid = gridid
			}

			// Set new cursor grid id
			s.ws.cursor.gridid = gridid
			if s.ws.cursor.prevGridid == 0 {
				s.ws.cursor.prevGridid = gridid
			}

			win.raise()
			win.s.tooltip.setGrid()

			// reset smooth scroll scrolling offset
			win.scrollPixels2 = 0
		}
	}
}

func (s *Screen) setTopLevelGrid(n int) {
	s.topLevelGrid = n
}

func (s *Screen) setHlAttrDef(args []interface{}) {
	var h map[int]*Highlight
	if s.hlAttrDef == nil {
		h = make(map[int]*Highlight)
	} else {
		h = s.hlAttrDef
	}

	h[0] = &Highlight{
		foreground: s.ws.foreground,
		background: s.ws.background,
	}

	for _, arg := range args {
		id := util.ReflectToInt(arg.([]interface{})[0])
		h[id] = s.makeHighlight(arg)
	}

	if s.hlAttrDef == nil {
		s.hlAttrDef = h
	}

	// // Update all cell's highlight
	// // It looks like we don't need it anymore.
	// isUpdateBg := true
	// curwin, ok := s.getWindow(s.ws.cursor.gridid)
	// if ok {
	// 	isUpdateBg = !curwin.background.equals(s.ws.background)
	// }
	// if isUpdateBg {
	// 	s.windows.Range(func(_, winITF interface{}) bool {
	// 		win := winITF.(*Window)
	// 		if win == nil {
	// 			return true
	// 		}
	// 		if !win.isShown() {
	// 			return true
	// 		}
	// 		if win.content == nil {
	// 			return true
	// 		}
	// 		for _, line := range win.content {
	// 			for _, cell := range line {
	// 				if cell != nil {
	// 					cell.highlight = s.hlAttrDef[cell.highlight.id]
	// 				}
	// 			}
	// 		}
	// 		return true
	// 	})
	// }
}

func (s *Screen) setHighlightGroup(args []interface{}) {
	for _, arg := range args {
		a := arg.([]interface{})
		hlName := a[0].(string)
		hlIndex := util.ReflectToInt(a[1])
		s.highlightGroup[hlName] = hlIndex
	}
}

func (s *Screen) makeHighlight(args interface{}) *Highlight {
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

	underdouble := hl["underdouble"]
	if underdouble != nil {
		highlight.underdouble = true
	} else {
		highlight.underdouble = false
	}

	underdashed := hl["underdashed"]
	if underdashed != nil {
		highlight.underdashed = true
	} else {
		highlight.underdashed = false
	}

	underdotted := hl["underdotted"]
	if underdotted != nil {
		highlight.underdotted = true
	} else {
		highlight.underdotted = false
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

	// af, ok := hl["altfont"]
	// if ok {
	// 	highlight.altfont = af.(string)
	// }

	bl, ok := hl["blend"]
	if ok {
		highlight.blend = util.ReflectToInt(bl)
	}

	return &highlight
}

func (s *Screen) getHighlightByHlname(name string) (hl *Highlight) {
	keys := make([]int, 0, len(s.hlAttrDef))
	for k := range s.hlAttrDef {
		if s.hlAttrDef[k].hlName == "" {
			continue
		}
		keys = append(keys, k)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(keys)))

	for _, k := range keys {
		h := s.hlAttrDef[k]
		if h.hlName == name {
			hl = h
			break
		}
	}

	return
}

func (s *Screen) getHighlightByUiname(name string) (hl *Highlight) {
	keys := make([]int, 0, len(s.hlAttrDef))
	for k := range s.hlAttrDef {
		if s.hlAttrDef[k].uiName == "" {
			continue
		}
		keys = append(keys, k)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(keys)))

	for _, k := range keys {
		h := s.hlAttrDef[k]
		if h.uiName == name {
			hl = h
			break
		}
	}

	return
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

		for i, line := range win.content {
			for j, _ := range line {
				win.content[i][j] = &Cell{
					highlight:   win.s.hlAttrDef[0],
					char:        " ",
					normalWidth: false,
					covered:     false,
				}
			}
		}

		win.queueRedrawAll()
	}
}

func (s *Screen) gridLine(args []interface{}) {
	var win *Window
	var ok bool

	for _, arg := range args {

		gridid := util.ReflectToInt(arg.([]interface{})[0])
		gridArgs := arg.([]interface{})

		if isSkipGlobalId(gridid) {
			continue
		}

		if win == nil || win.grid != gridid {
			win, ok = s.getWindow(gridid)
			if !ok {
				continue
			}
		}

		win.updateGridContent(
			util.ReflectToInt(gridArgs[1]),
			util.ReflectToInt(gridArgs[2]),
			gridArgs[3].([]interface{}),
		)

		s.lastGridLineGrid = win.grid
	}
}

func (s *Screen) gridScroll(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
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

		win.scroll(
			util.ReflectToInt(arg.([]interface{})[5]), // rows
		)
	}
}

// In Neovim's multigrid UI architecture, a window is represented by multiple independent grids,
// and grid1 is a special grid called the global grid, which displays window splits, tabs, etc.
// This is problematic when applying transparency effects to the entire application.
// If each grid is displayed transparently, the information in global grid behind it will show through,
// but the current UI specification expects the global grid to be covered by other grids,
// so there will be unwanted content left over before and after updates.
// detectCoveredCellInGlobalgrid() logically controls this and detects information
// about the covered state of each grid in the global grid. These coverage states are used to control
// whether or not certain cell contents should be drawn when processing the painting of the global grid.
func (s *Screen) detectCoveredCellInGlobalgrid() {
	if editor.config.Editor.Transparent == 1.0 {
		return
	}
	gwin, ok := s.getWindow(1)
	if !ok {
		return
	}

	for _, line := range gwin.content {
		for _, cell := range line {
			if cell == nil {
				continue
			}
			cell.covered = false
		}
	}

	font := s.font

	s.windows.Range(func(grid, winITF interface{}) bool {

		win := winITF.(*Window)
		if win == nil {
			return true
		}
		if win.isFloatWin {
			return true
		}
		if win.isExternal {
			return true
		}
		if win.isMsgGrid {
			return true
		}
		if win.isGridDirty {
			return true
		}
		if win.grid == 1 {
			return true
		}
		if win.IsVisible() {
			x := int(math.Floor(float64(win.pos[0]) * font.cellwidth))
			y := win.pos[1] * font.lineHeight
			width := int(math.Ceil(float64(win.cols) * font.cellwidth))
			height := win.rows * font.lineHeight

			winrect := core.NewQRect4(
				x,
				y,
				width,
				height,
			)

			for i, line := range gwin.content {
				for j, cell := range line {

					if cell == nil {
						continue
					}

					p := core.NewQPoint2(
						int(font.cellwidth*float64(j)+font.cellwidth/2.0),
						font.lineHeight*i+int(float64(font.lineHeight)/2.0),
					)
					if winrect.Contains(p, true) {
						if !cell.covered {
							cell.covered = true
							gwin.contentMask[i][j] = true
						}
					}

					// v := 0
					// if cell.covered {
					// 	v = 1
					// }
					// fmt.Printf("%d", v)
				}

				// fmt.Printf("\n")
			}
		}
		gwin.queueRedraw(0, 0, gwin.cols, gwin.rows)

		return true
	})

}

func (s *Screen) update() {
	s.windows.Range(func(grid, winITF interface{}) bool {
		win := winITF.(*Window)
		// if grid is dirty, we remove this grid
		if win.isGridDirty && !win.isSameAsCursorGrid() {
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
			s.deleteWindow(win.grid)
		}
		if win != nil {
			// Fill entire background if background color changed
			if !win.background.equals(s.ws.background) {
				win.background = s.ws.background.copy()
				win.fill()
			}

			// Suppressed an issue where the cursor remains
			// when moving the cursor in PopOS environment.
			if runtime.GOOS == "linux" {
				if s.ws.viewport == s.ws.oldViewport {
					_, col := win.s.ws.cursor.getRowAndColFromScreen()
					font := win.getFont()
					x := int(math.Ceil(float64(col) * font.cellwidth))
					win.Update2(
						x, 0,
						int(math.Ceil(font.cellwidth)),
						s.height,
					)
				}
			}

			win.update()
		}

		return true
	})
}

func (s *Screen) refresh() {
	s.windows.Range(func(grid, winITF interface{}) bool {
		win := winITF.(*Window)
		if win != nil {
			// Fill entire background if background color changed
			if !win.background.equals(s.ws.background) {
				win.background = s.ws.background.copy()
			}
			win.fill()
			win.Update()
		}

		return true
	})
}

func (s *Screen) runeTextWidth(font *Font, text string) float64 {
	cjk := 0
	ascii := 0
	var buffer bytes.Buffer
	for _, c := range []rune(text) {
		if c >= 0 && c <= 127 {
			ascii++
		} else {
			buffer.WriteString(string(c))
		}
	}

	r := buffer.String()
	width := (font.cellwidth)*float64(ascii) + (font.cellwidth)*float64(cjk)*2
	if r == "" {
		width += font.fontMetrics.HorizontalAdvance(r, -1)
	}
	if width == 0 {
		width = font.cellwidth * 2
	}

	return width
}

func (s *Screen) windowPosition(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		id := arg.([]interface{})[1].(nvim.Window)
		row := util.ReflectToInt(arg.([]interface{})[2])
		col := util.ReflectToInt(arg.([]interface{})[3])
		cols := util.ReflectToInt(arg.([]interface{})[4])
		rows := util.ReflectToInt(arg.([]interface{})[5])

		// fmt.Println("win_pos::", "grid:", gridid, col, row)

		if isSkipGlobalId(gridid) {
			continue
		}

		s.resizeWindow(gridid, cols, rows)

		win, ok := s.getWindow(gridid)
		if !ok {
			continue
		}

		// winbar := s.ws.getOpWinbar(nvim.LocalScope, id)
		win.updateMutex.Lock()
		win.id = id
		win.pos = [2]int{col, row}
		win.setOptions()

		win.isFloatWin = false
		if win.isExternal {
			win.isExternal = false
			win.deleteExternalWin()
			win.SetParent(s.widget)
			win.raise()
		}

		win.updateMutex.Unlock()

		win.move(col, row)
		win.show()
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

func (s *Screen) windowFloatPosition(args []interface{}) {
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
		win.anchor = arg.([]interface{})[2].(string)
		win.anchorGrid = util.ReflectToInt(arg.([]interface{})[3])
		win.anchorCol = int(util.ReflectToFloat(arg.([]interface{})[5]))
		win.anchorRow = int(util.ReflectToFloat(arg.([]interface{})[4]))
		// focusable := (arg.([]interface{})[6]).(bool)

		if len(arg.([]interface{})) >= 8 {
			win.zindex.value = int(util.ReflectToInt(arg.([]interface{})[7]))
			win.zindex.order = globalOrder
			globalOrder++
		}

		editor.putLog("float window generated:", "anchorgrid", win.anchorGrid, "anchor", win.anchor, "anchorCol", win.anchorCol, "anchorRow", win.anchorRow)

		shouldStackPerZIndex := !win.IsVisible()
		if !win.isFloatWin {
			win.isFloatWin = true
			shouldStackPerZIndex = shouldStackPerZIndex || true
		}

		if win.isExternal {
			win.deleteExternalWin()
			win.isExternal = false
		}

		anchorwin, ok := s.getWindow(win.anchorGrid)
		if !ok {
			continue
		}

		win.anchorwin = anchorwin
		win.setOptions()

		anchorwinIsExternal := win.anchorwin.isExternal

		win.updateMutex.Unlock()

		if anchorwinIsExternal {
			win.SetParent(anchorwin)
		}

		win.setFloatWindowPosition()

		win.move(win.pos[0], win.pos[1], win.anchorwin)

		if shouldStackPerZIndex {
			win.raise()
		}

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
				extwin.ConnectKeyReleaseEvent(editor.keyRelease)
				extwin.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
				extwin.ConnectInputMethodEvent(s.ws.InputMethodEvent)
				extwin.ConnectInputMethodQuery(s.ws.InputMethodQuery)
				extwin.ConnectMousePressEvent(win.mouseEvent)
				extwin.ConnectMouseReleaseEvent(win.mouseEvent)

				extwin.InstallEventFilter(extwin)
				extwin.ConnectEventFilter(func(watched *core.QObject, event *core.QEvent) bool {
					switch event.Type() {
					case core.QEvent__ActivationChange:
						if extwin.IsActiveWindow() {
							editor.isExtWinNowActivated = true
							editor.isExtWinNowInactivated = false
						} else if !extwin.IsActiveWindow() {
							editor.isExtWinNowActivated = false
							editor.isExtWinNowInactivated = true
						}
					default:
					}
					return extwin.EventFilterDefault(watched, event)
				})

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
					win.pos[0] = int(float64(pos.X()-gPos.X()) / font.cellwidth)
					win.pos[1] = int(float64(pos.Y()-gPos.Y()) / float64(font.lineHeight))
				})
				width := int(math.Ceil(float64(win.cols) * font.cellwidth))
				height := win.rows * font.lineHeight
				win.setGridGeometry(width, height)
				win.setResizableForExtWin()
				win.move(win.pos[0], win.pos[1])
				win.setOptions()
				win.raise()
			}

			return true
		})

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

		var win *Window
		var ok bool
		win, ok = s.getWindow(gridid)
		// If message grid does not exist, create it
		if !ok {
			gwin, okok := s.getWindow(1)
			if !okok {
				continue
			}
			s.resizeWindow(gridid, gwin.cols, gwin.rows)
			win, ok = s.getWindow(gridid)
			if !ok {
				continue
			}
		}
		win.isMsgGrid = true
		win.isFloatWin = true
		win.zindex.value = 200
		win.pos[1] = row
		win.move(win.pos[0], win.pos[1])
		win.show()
		if scrolled {
			win.raise() // Fix #111
		}
	}
}

func (s *Screen) windowClose() {
	// Close the window.
}

func isSkipGlobalId(id gridId) bool {
	if editor.config.Editor.SkipGlobalId {
		if id == 1 {
			return true
		}
	}

	return false
}
