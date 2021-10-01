package editor

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akiyosi/goneovim/util"
	"github.com/bluele/gcache"
	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

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

	tooltip *IMETooltip

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

	widget.ConnectMousePressEvent(screen.mousePressEvent)
	widget.ConnectMouseReleaseEvent(screen.mouseEvent)
	widget.ConnectMouseMoveEvent(screen.mouseEvent)

	return screen
}

func (s *Screen) initInputMethodWidget() {
	tooltip := NewIMETooltip(s.widget, 0)
	tooltip.SetVisible(false)
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
		position := win.pos[1]*win.s.font.lineHeight + win.Rect().Bottom()
		if pos < position {
			pos = position
		}

		return true
	})

	return pos
}

func (s *Screen) mousePressEvent(event *gui.QMouseEvent) {
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
		s.ws.cursor.Raise()

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

func (s *Screen) windowFloatPosition(args []interface{}) {

	// A workaround for the problem that the position of the float window,
	// which is created as a tooltip suggested by LSP, is not the correct
	// position in multigrid ui api.
	isExistPopupmenu := false
	if s.ws.mode == "insert" {
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
		// focusable := (arg.([]interface{})[6]).(bool)

		editor.putLog("float window generated:", "anchorgrid", anchorGrid, "anchor", win.anchor, "anchorCol", anchorCol, "anchorRow", anchorRow)

		// if editor.config.Editor.WorkAroundNeovimIssue12985 {
		if isExistPopupmenu && win.id != -1 {
			anchorGrid = s.ws.cursor.gridid
		}
		// }

		// win.SetParent(win.s.ws.screen.widget)
		win.SetParent(win.s.ws.widget)

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
		if editor.config.Editor.WorkAroundNeovimIssue12985 {
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

			if editor.config.Editor.WorkAroundNeovimIssue12985 {
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
		win.s.ws.cursor.Raise()

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

// func (s *Screen) setColor() {
// 	s.tooltip.SetStyleSheet(
// 		fmt.Sprintf(
// 			" * {background-color: %s; text-decoration: underline; color: %s; }",
// 			editor.colors.selectedBg.String(),
// 			editor.colors.fg.String(),
// 		),
// 	)
// }

func isSkipGlobalId(id gridId) bool {
	if editor.config.Editor.SkipGlobalId {
		if id == 1 {
			return true
		}
	}

	return false
}
