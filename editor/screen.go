package editor

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"

	"github.com/akiyosi/gonvim/util"
)

type gridId = int

// Highlight is
type Highlight struct {
	id int
	// kind       string
	uiName     string
	hlName     string
	foreground *RGBA
	background *RGBA
	special    *RGBA
	reverse    bool
	italic     bool
	bold       bool
	underline  bool
	undercurl  bool
}

// Cell is
type Cell struct {
	normalWidth bool
	char        string
	highlight   Highlight
}

// Window is
type Window struct {
	paintMutex  sync.Mutex
	redrawMutex sync.Mutex

	s       *Screen
	content [][]*Cell
	lenLine []int

	grid       gridId
	id         nvim.Window
	pos        [2]int
	anchor     int
	cols       int
	rows       int
	isMsgGrid  bool
	isFloatWin bool

	widget           *widgets.QWidget
	shown            bool
	queueRedrawArea  [4]int
	scrollRegion     []int
	devicePixelRatio float64

	// NOTE:
	// Only use minimap
	// Plan to remove in the future
	width  int
	height int
}

// Screen is the main editor area
type Screen struct {
	bg               *RGBA
	width            int
	height           int
	widget           *widgets.QWidget
	ws               *Workspace
	windows          map[gridId]*Window
	wins             map[nvim.Window]*Window
	cursor           [2]int
	lastCursor       [2]int
	scrollRegion     []int
	scrollDust       [2]int
	scrollDustDeltaY int
	cmdheight        int
	highAttrDef      map[int]*Highlight
	highlightGroup   map[string]int
	highlight        Highlight
	curtab           nvim.Tabpage
	curWins          map[nvim.Window]*Window
	content          [][]*Cell
	queueRedrawArea  [4]int
	paintMutex       sync.Mutex
	redrawMutex      sync.Mutex
	drawSplit        bool
	resizeCount      uint
	tooltip          *widgets.QLabel
	glyphMap         map[Cell]gui.QImage
	isScrollOver     bool
	scrollOverCount  int
}

func newScreen() *Screen {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")

	tooltip := widgets.NewQLabel(widget, 0)
	tooltip.SetVisible(false)

	screen := &Screen{
		widget:       widget,
		windows:      make(map[gridId]*Window),
		cursor:       [2]int{0, 0},
		lastCursor:   [2]int{0, 0},
		scrollRegion: []int{0, 0, 0, 0},
		tooltip:      tooltip,
		glyphMap:     make(map[Cell]gui.QImage),
	}

	widget.ConnectMousePressEvent(screen.mouseEvent)
	widget.ConnectMouseReleaseEvent(screen.mouseEvent)
	widget.ConnectMouseMoveEvent(screen.mouseEvent)
	widget.ConnectResizeEvent(func(event *gui.QResizeEvent) {
		screen.updateSize()
	})

	return screen
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
	message := fmt.Sprintf("[Gonvvim] Do you want to diff between the file being dropped and the current buffer?")
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

func (s *Screen) updateRows() bool {
	var ret bool
	w := s.ws
	rows := s.height / w.font.lineHeight

	if rows != w.rows {
		ret = true
	}
	w.rows = rows
	return ret
}

func (s *Screen) updateCols() bool {
	var ret bool
	w := s.ws
	s.width = s.widget.Width()
	cols := int(float64(s.width) / w.font.truewidth)

	if cols != w.cols {
		ret = true
	}
	w.cols = cols
	return ret
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
	w := s.ws
	s.width = s.widget.Width()
	currentCols := int(float64(s.width) / w.font.truewidth)
	currentRows := s.height / w.font.lineHeight

	isNeedTryResize := (currentCols != w.cols || currentRows != w.rows)
	if !isNeedTryResize {
		return
	}

	w.cols = currentCols
	w.rows = currentRows

	if !w.uiAttached {
		return
	}
	s.uiTryResize(currentCols, currentRows)
}

func (s *Screen) uiTryResize(width, height int) {
	w := s.ws
	done := make(chan error, 5)
	var result error
	go func() {
		result = w.nvim.TryResizeUI(width, height)
		// rewrite with nvim_ui_try_resize_grid
		// result = w.nvim.Call("nvim_ui_try_resize_grid", s.activeGrid, currentCols, currentRows)
		done <- result
	}()
	select {
	case <-done:
	case <-time.After(s.waitTime() * time.Millisecond):
		// In this case, assuming that nvim is returning an error
		//  at startup and the TryResizeUI() function hangs up.
		w.nvim.Input("<Enter>")
		s.uiTryResize(width, height)
	}
}

func (s *Screen) toolTipPos() (int, int, int, int) {
	var x, y, candX, candY int
	w := s.ws
	if s.ws.palette.widget.IsVisible() {
		s.tooltip.SetParent(s.ws.palette.widget)
		font := gui.NewQFont2(editor.extFontFamily, editor.extFontSize, 1, false)
		s.tooltip.SetFont(font)
		x = w.palette.cursorX + w.palette.patternPadding
		candX = x + w.palette.widget.Pos().X()
		y = w.palette.patternPadding + s.ws.palette.padding
		candY = y + w.palette.widget.Pos().Y()
	} else {
		s.toolTipFont(w.font)
		row := s.cursor[0]
		col := s.cursor[1]
		x = int(float64(col) * w.font.truewidth)
		y = row * w.font.lineHeight
		candX = int(float64(col) * w.font.truewidth)
		candY = row*w.font.lineHeight + w.tabline.height + w.tabline.marginTop + w.tabline.marginBottom
	}
	return x, y, candX, candY
}

func (s *Screen) toolTipMove(x int, y int) {
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

func (s *Screen) toolTip(text string) {
	s.tooltip.SetText(text)
	s.tooltip.AdjustSize()
	s.tooltip.Show()

	row := s.cursor[0]
	col := s.cursor[1]
	c := s.ws.cursor
	c.x = int(float64(col)*s.ws.font.truewidth) + s.tooltip.Width()
	c.y = row * s.ws.font.lineHeight
	c.move()
}

func (w *Window) paint(event *gui.QPaintEvent) {
	w.paintMutex.Lock()

	rect := event.Rect()
	top := rect.Y()
	left := rect.X()
	width := rect.Width()
	height := rect.Height()
	right := left + width
	bottom := top + height

	font := w.s.ws.font
	row := int(float64(top) / float64(font.lineHeight))
	col := int(float64(left) / font.truewidth)
	rows := int(math.Ceil(float64(bottom)/float64(font.lineHeight))) - row
	cols := int(math.Ceil(float64(right)/font.truewidth)) - col

	p := gui.NewQPainter2(w.widget)

	if !editor.config.Editor.CachedDrawing {
		p.SetFont(font.fontNew)
	}

	// Draw contents
	for y := row; y < row+rows; y++ {
		if y >= w.rows {
			continue
		}
		w.fillBackground(p, y, col, cols)
		w.drawContents(p, y, col, cols)
		w.drawTextDecoration(p, y, col, cols)
	}

	// If Window is Message Area, draw separator
	if w.isMsgGrid {
		w.drawMsgSeparator(p)
	}

	// Draw vim window separator
	if editor.config.Editor.DrawBorder && w.grid == 1 {
		for _, win := range w.s.windows {
			win.drawBorder(p)
		}
	}

	// Draw indent guide
	if editor.config.Editor.IndentGuide && w.grid == 1 {
		for _, win := range w.s.windows {
			win.drawIndentguide(p)
		}
	}

	// Update markdown preview
	if w.grid != 1 {
		w.s.ws.markdown.updatePos()
	}

	p.DestroyQPainter()
	w.paintMutex.Unlock()
}

func (w *Window) drawIndentguide(p *gui.QPainter) {
	if w == nil {
		return
	}
	if w.s.ws.ts == 0 {
		return
	}
	if !w.isShown() {
		return
	}
	for y := 0; y < len(w.content); y++ {
		if y+1 >= len(w.content) {
			return
		}
		nextline := w.content[y+1]
		line := w.content[y]
		res := 0
		skipDraw := false
		for x := 0; x < w.lenLine[y]; x++ {
			skipDraw = false

			// if x+1 == w.lenLine[y+1] {
			//         break
			// }
			if x+1 >= len(nextline) {
				break
			}
			nlnc := nextline[x+1]
			if nlnc == nil {
				continue
			}
			nlc := nextline[x]
			if nlc == nil {
				continue
			}
			nc := line[x+1]
			if nc == nil {
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
			if x > res &&
				(x+1-res)%w.s.ws.ts == 0 &&
				c.char == " " && nc.char != " " &&
				nlc.char == " " && nlnc.char == " " {

				if w.lenLine[y] >= len(line) {
					break
				}
				bra := line[w.lenLine[y]-1].char
				cket := getCket(bra)

				for row := y; row < len(w.content); row++ {
					if row+1 == len(w.content) {
						break
					}

					for z := y + 1; z < len(w.content); z++ {
						if w.content[z][x+1] == nil {
							break
						}
						if w.content[z][x+1].char != " " {
							if w.content[z][x+1].char != cket {
								break
							}

							for v := x; v >= res; v-- {
								if w.content[z][v] == nil {
									break
								}
								if w.content[z][v].char == " " {
									skipDraw = true
								} else {
									skipDraw = false
									break
								}
							}
							if skipDraw {
								break
							}
						}
					}
					if !skipDraw {
						break
					}

					ylen, _ := w.countHeadSpaceOfLine(y)
					ynlen, _ := w.countHeadSpaceOfLine(y + 1)
					if ynlen <= ylen {
						break
					}
					if w.content[row+1][x+1] == nil {
						break
					}
					if w.content[row+1][x+1].char != " " {
						break
					}
					w.drawIndentline(p, x+1, row+1)
				}
				break
			}
		}
	}
}

func getCket(bra string) string {
	cket := " "

	switch bra {
	case "{":
		cket = "}"
	case "[":
		cket = "]"
	case "(":
		cket = ")"
	case "<":
		cket = ">"
	case `"`:
		cket = `"`
	case `'`:
		cket = `'`
	case "`":
		cket = "`"
	}

	return cket
}

func (w *Window) drawIndentline(p *gui.QPainter, x int, y int) {
	X := float64(w.pos[0]+x) * w.s.ws.font.truewidth
	Y := float64((w.pos[1] + y) * w.s.ws.font.lineHeight)
	p.FillRect4(
		core.NewQRectF4(
			X,
			Y,
			1,
			float64(w.s.ws.font.lineHeight),
		),
		gui.NewQColor3(
			editor.colors.indentGuide.R,
			editor.colors.indentGuide.G,
			editor.colors.indentGuide.B,
			255),
	)
}

func (w *Window) drawMsgSeparator(p *gui.QPainter) {
	color := w.s.highAttrDef[w.s.highlightGroup["MsgSeparator"]].foreground
	p.FillRect4(
		core.NewQRectF4(
			0,
			0,
			float64(w.widget.Width()),
			1,
		),
		gui.NewQColor3(
			color.R,
			color.G,
			color.B,
			200),
	)
}

func (w *Window) drawBorder(p *gui.QPainter) {
	if w == nil {
		return
	}
	if !w.isShown() {
		return
	}
	if w.isFloatWin {
		return
	}
	if w.isMsgGrid {
		return
	}
	x := int(float64(w.pos[0]) * w.s.ws.font.truewidth)
	// y := (w.pos[1] - w.s.scrollOverCount) * int(w.s.ws.font.lineHeight)
	y := w.pos[1] * int(w.s.ws.font.lineHeight)
	width := int(float64(w.cols) * w.s.ws.font.truewidth)
	winHeight := int((float64(w.rows) + 0.92) * float64(w.s.ws.font.lineHeight))
	color := gui.NewQColor3(
		editor.colors.windowSeparator.R,
		editor.colors.windowSeparator.G,
		editor.colors.windowSeparator.B,
		255)

	// Vertical
	if y+w.s.ws.font.lineHeight+1 < w.s.widget.Height() {
		p.FillRect5(
			int(float64(x+width)+w.s.ws.font.truewidth/2),
			y-int(w.s.ws.font.lineHeight/2),
			1,
			winHeight,
			color,
		)
	}

	if w.widget.MapToGlobal(w.widget.Pos()).Y() == w.s.bottomWindowPos() {
		return
	}

	// Horizontal
	height := w.rows * int(w.s.ws.font.lineHeight)
	y2 := y + height - 1 + w.s.ws.font.lineHeight/2

	p.FillRect5(
		int(float64(x)-w.s.ws.font.truewidth/2),
		y2,
		int((float64(w.cols)+0.92)*w.s.ws.font.truewidth),
		1,
		color,
	)
}

func (s *Screen) bottomWindowPos() int {
	pos := 0
	for _, win := range s.windows {
		if win == nil {
			continue
		}
		if win.grid == 1 {
			continue
		}
		if win.isMsgGrid {
			continue
		}
		position := win.widget.MapToGlobal(win.widget.Pos()).Y()
		if pos < position {
			pos = position
		}
	}

	return pos
}

func (s *Screen) wheelEvent(event *gui.QWheelEvent) {
	var m sync.Mutex
	m.Lock()
	defer m.Unlock()

	var v, h, vert, horiz int
	var horizKey string
	var accel int
	font := s.ws.font

	switch runtime.GOOS {
	case "darwin":
		pixels := event.PixelDelta()
		if pixels != nil {
			v = pixels.Y()
			h = pixels.X()
		}
		if pixels.X() < 0 && s.scrollDust[0] > 0 {
			s.scrollDust[0] = 0
		}
		if pixels.Y() < 0 && s.scrollDust[1] > 0 {
			s.scrollDust[1] = 0
		}

		dx := math.Abs(float64(s.scrollDust[0]))
		dy := math.Abs(float64(s.scrollDust[1]))

		fontheight := float64(font.lineHeight)
		fontwidth := float64(font.truewidth)

		s.scrollDust[0] += h
		s.scrollDust[1] += v

		if dx >= fontwidth {
			horiz = int(math.Trunc(float64(s.scrollDust[0]) / fontwidth))
			s.scrollDust[0] = 0
		}
		if dy >= fontheight {
			vert = int(math.Trunc(float64(s.scrollDust[1]) / fontheight))
			s.scrollDust[1] = 0
		}

		s.scrollDustDeltaY = int(math.Abs(float64(vert)) - float64(s.scrollDustDeltaY))
		if s.scrollDustDeltaY < 1 {
			s.scrollDustDeltaY = 0
		}
		if s.scrollDustDeltaY <= 2 {
			accel = 1
		} else if s.scrollDustDeltaY > 2 {
			accel = int(float64(s.scrollDustDeltaY) / float64(4))
		}

	default:
		vert = event.AngleDelta().Y()
		horiz = event.AngleDelta().X()
		accel = 2
	}

	mod := event.Modifiers()

	if horiz > 0 {
		horizKey = "Left"
	} else {
		horizKey = "Right"
	}

	x := int(float64(event.X()) / font.truewidth)
	y := int(float64(event.Y()) / float64(font.lineHeight))
	pos := []int{x, y}

	if vert == 0 && horiz == 0 {
		return
	}

	mode := s.ws.mode
	if mode == "insert" {
		s.ws.nvim.Input(fmt.Sprintf("<Esc>"))
	} else if mode == "terminal-input" {
		s.ws.nvim.Input(fmt.Sprintf(`<C-\><C-n>`))
	}

	if vert > 0 {
		s.ws.nvim.Input(fmt.Sprintf("%v<C-y>", accel))
	} else if vert < 0 {
		s.ws.nvim.Input(fmt.Sprintf("%v<C-e>", accel))
	}

	if horiz != 0 {
		s.ws.nvim.Input(fmt.Sprintf("<%sScrollWheel%s><%d,%d>", editor.modPrefix(mod), horizKey, pos[0], pos[1]))
	}

	event.Accept()
}

func (s *Screen) mouseEvent(event *gui.QMouseEvent) {
	inp := s.convertMouse(event)
	if inp == "" {
		return
	}
	s.ws.nvim.Input(inp)
}

func (s *Screen) convertMouse(event *gui.QMouseEvent) string {
	font := s.ws.font
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
		s.assignMdGridid(gridid)
		s.resizeWindow(gridid, cols, rows)
	}
}

func (s *Screen) assignMdGridid(gridid gridId) {
	if !s.ws.markdown.gridIdTrap || gridid == 1 {
		return
	}
	maxid := 0
	for id, _ := range s.windows {
		if maxid < id {
			maxid = id
		}
	}
	if maxid < gridid {
		s.ws.markdown.mdGridId = gridid
		s.ws.markdown.gridIdTrap = false
	}
}

func (s *Screen) resizeWindow(gridid gridId, cols int, rows int) {
	win := s.windows[gridid]
	if win != nil {
		if win.cols == cols && win.rows == rows {
			return
		}
	}

	// make new size content
	content := make([][]*Cell, rows)
	lenLine := make([]int, rows)

	for i := 0; i < rows; i++ {
		content[i] = make([]*Cell, cols)
	}

	if win != nil && gridid != 1 {
		for i := 0; i < rows; i++ {
			if i >= len(win.content) {
				continue
			}
			lenLine[i] = win.lenLine[i]
			for j := 0; j < cols; j++ {
				if j >= len(win.content[i]) {
					continue
				}
				content[i][j] = win.content[i][j]
			}
		}
	}

	if win == nil {
		s.windows[gridid] = newWindow()
		s.windows[gridid].s = s
		s.windows[gridid].setParent(s.widget)
		s.windows[gridid].grid = gridid
		s.windows[gridid].widget.SetAttribute(core.Qt__WA_KeyCompression, true)
		s.windows[gridid].widget.SetAcceptDrops(true)
		s.windows[gridid].widget.ConnectWheelEvent(s.wheelEvent)
		s.windows[gridid].widget.ConnectDragEnterEvent(s.dragEnterEvent)
		s.windows[gridid].widget.ConnectDragMoveEvent(s.dragMoveEvent)
		s.windows[gridid].widget.ConnectDropEvent(s.dropEvent)
		// reassign win
		win = s.windows[gridid]

		// first cursor pos at startup app
		if gridid == 1 {
			s.ws.cursor.widget.SetParent(win.widget)
		}
	}

	win.lenLine = lenLine
	win.content = content
	win.cols = cols
	win.rows = rows

	width := int(float64(cols) * s.ws.font.truewidth)
	height := rows * int(s.ws.font.lineHeight)
	rect := core.NewQRect4(0, 0, width, height)
	win.setGeometry(rect)

	win.move(win.pos[0], win.pos[1])
	win.show()

	win.queueRedrawAll()
}

func (s *Screen) gridCursorGoto(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		s.cursor[0] = util.ReflectToInt(arg.([]interface{})[1])
		s.cursor[1] = util.ReflectToInt(arg.([]interface{})[2])
		if isSkipGlobalId(gridid) {
			continue
		}

		if s.windows[gridid] == nil {
			continue
		}

		if s.ws.cursor.gridid != gridid {
			s.ws.cursor.gridid = gridid
			s.windows[gridid].raise()
		}
	}
}

func (s *Screen) setHighAttrDef(args []interface{}) {
	var h map[int]*Highlight
	if s.highAttrDef == nil {
		h = make(map[int]*Highlight)
	} else {
		h = s.highAttrDef
	}
	h[0] = &Highlight{
		foreground: editor.colors.fg,
		background: editor.colors.bg,
	}

	for _, arg := range args {
		id := util.ReflectToInt(arg.([]interface{})[0])
		h[id] = s.getHighlight(arg)
	}

	s.highAttrDef = h
}

func (s *Screen) setHighlightGroup(args []interface{}) {
	h := make(map[string]int)
	for _, arg := range args {
		a := arg.([]interface{})
		hlName := a[0].(string)
		hlIndex := util.ReflectToInt(a[1])
		h[hlName] = hlIndex
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

	// kind, ok := info["kind"]
	// if ok {
	// 	highlight.kind = kind.(string)
	// }

	uiName, ok := info["ui_name"]
	if ok {
		highlight.uiName = uiName.(string)
	}

	id, ok := info["id"]
	if ok {
		highlight.id = util.ReflectToInt(id)
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
	} else {
		highlight.special = highlight.foreground
	}

	return &highlight
}

func (h *Highlight) fg() *RGBA {
	var color *RGBA
	if h.reverse {
		color = h.background
		if color == nil {
			// color = w.s.ws.background
			color = editor.colors.bg
		}
	} else {
		color = h.foreground
		if color == nil {
			// color = w.s.ws.foreground
			color = editor.colors.fg
		}
	}

	return color
}

func (h *Highlight) bg() *RGBA {
	var color *RGBA
	if h.reverse {
		color = h.foreground
		if color == nil {
			// color = w.s.ws.foreground
			color = editor.colors.fg
		}
	} else {
		color = h.background
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

		s.windows[gridid].content = make([][]*Cell, s.windows[gridid].rows)
		s.windows[gridid].lenLine = make([]int, s.windows[gridid].rows)

		for i := 0; i < s.windows[gridid].rows; i++ {
			s.windows[gridid].content[i] = make([]*Cell, s.windows[gridid].cols)
		}
		s.windows[gridid].queueRedrawAll()
	}
}

func (s *Screen) gridLine(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		if isSkipGlobalId(gridid) {
			continue
		}

		s.updateGridContent(arg.([]interface{}))
		if !s.windows[gridid].isShown() {
			s.windows[gridid].show()
		}
	}

	// // For legacy UI API
	// if s.isScrollOver {
	// 	s.gridScrollOver()
	// }
}

func (s *Screen) gridScrollOver() {
	for _, win := range s.windows {
		if win.grid == 1 {
			s.scrollOverCount++
		}
		if win.grid != 1 {
			if win == nil {
				continue
			}
			if !win.isShown() {
				continue
			}
			win.move(win.pos[0], win.pos[1]-s.scrollOverCount)
		}
	}
}

func (s *Screen) updateGridContent(arg []interface{}) {
	gridid := util.ReflectToInt(arg[0])
	row := util.ReflectToInt(arg[1])
	colStart := util.ReflectToInt(arg[2])

	if isSkipGlobalId(gridid) {
		return
	}
	if colStart < 0 {
		return
	}

	content := s.windows[gridid].content
	if row >= s.windows[gridid].rows {
		return
	}
	col := colStart
	line := content[row]
	cells := arg[3].([]interface{})

	buffLenLine := colStart
	lenLine := 0

	isMulitigridAndGlobalGrid := gridid == 1 && editor.config.Editor.ExtMultigrid

	countLenLine := false
	for _, arg := range cells {
		if col >= len(line) {
			continue
		}
		cell := arg.([]interface{})

		var hi, repeat int
		hi = -1
		text := cell[0]
		if len(cell) >= 2 {
			hi = util.ReflectToInt(cell[1])
		}

		// if drawborder is true, and row is statusline's row
		if s.isSkipDrawStatusline(hi) {
			return
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

			if !isMulitigridAndGlobalGrid {
				line[col].char = text.(string)
				line[col].normalWidth = s.isNormalWidth(line[col].char)
			}

			// If `hl_id` is not present the most recently seen `hl_id` in
			//	the same call should be used (it is always sent for the first
			//	cell in the event).
			switch col {
			case 0:
				line[col].highlight = *s.highAttrDef[hi]
			default:
				if hi == -1 {
					line[col].highlight = line[col-1].highlight
				} else {
					line[col].highlight = *s.highAttrDef[hi]
				}
			}

			// Count Line content length
			buffLenLine++
			if line[col].char != " " {
				countLenLine = true
			} else if line[col].char == " " {
				countLenLine = false
			}
			if countLenLine {
				lenLine += buffLenLine
				buffLenLine = 0
				countLenLine = false
			}

			col++
			r++
		}

		s.windows[gridid].queueRedraw(0, row, s.windows[gridid].cols, 1)
	}

	// If the array of cell changes doesn't reach to the end of the line,
	// the rest should remain unchanged.
	buffLenLine = 0
	if lenLine < s.windows[gridid].lenLine[row] {
		for x := lenLine; x < s.windows[gridid].lenLine[row]; x++ {
			if x >= len(line) {
				break
			}
			if line[x] == nil {
				break
			}
			// Count Line content length
			buffLenLine++
			if line[x].char != " " {
				countLenLine = true
			} else if line[x].char == " " {
				countLenLine = false
			}
			if countLenLine {
				lenLine += buffLenLine
				buffLenLine = 0
				countLenLine = false
			}
		}
	}
	// Set content length of line
	s.windows[gridid].lenLine[row] = lenLine

	return
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
	if count == len(line) {
		count = 0
	}
	return count, nil
}

func (c *Cell) isSignColumn() bool {
	switch c.highlight.hlName {
	case "SignColumn",
		"LineNr",
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

func (s *Screen) isSkipDrawStatusline(hi int) bool {
	// If ext_statusline is implemented in Neovim, the implementation may be revised
	if !editor.config.Editor.DrawBorder {
		return false
	}
	if s.highAttrDef[hi] == nil {
		return false
	}
	if s.highAttrDef[hi].hlName == "StatusLine" ||
		s.highAttrDef[hi].hlName == "StatusLineNC" ||
		s.highAttrDef[hi].hlName == "VertSplit" {
		return true
	}
	return false
}

func (s *Screen) gridScroll(args []interface{}) {
	var gridid gridId
	var rows int
	for _, arg := range args {
		gridid = util.ReflectToInt(arg.([]interface{})[0])
		if isSkipGlobalId(gridid) {
			continue
		}
		s.windows[gridid].scrollRegion[0] = util.ReflectToInt(arg.([]interface{})[1])     // top
		s.windows[gridid].scrollRegion[1] = util.ReflectToInt(arg.([]interface{})[2]) - 1 // bot
		s.windows[gridid].scrollRegion[2] = util.ReflectToInt(arg.([]interface{})[3])     // left
		s.windows[gridid].scrollRegion[3] = util.ReflectToInt(arg.([]interface{})[4]) - 1 // right
		rows = util.ReflectToInt(arg.([]interface{})[5])
		s.windows[gridid].scroll(rows)
	}
}

func (w *Window) scroll(count int) {
	top := w.scrollRegion[0]
	bot := w.scrollRegion[1]
	left := w.scrollRegion[2]
	right := w.scrollRegion[3]
	content := w.content
	lenLine := w.lenLine

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
			copy(content[row], content[row+count])
			lenLine[row] = lenLine[row+count]

		}
		for row := bot - count + 1; row <= bot; row++ {
			for col := left; col <= right; col++ {
				content[row][col] = nil
			}
		}
	} else {
		for row := bot; row >= top-count; row-- {
			if len(content) <= row {
				continue
			}
			copy(content[row], content[row+count])
			lenLine[row] = lenLine[row+count]
		}
		for row := top; row < top-count; row++ {
			for col := left; col <= right; col++ {
				content[row][col] = nil
			}
		}
	}

	w.queueRedraw(left, top, (right - left + 1), (bot - top + 1))
}

func (w *Window) update() {
	if w == nil {
		return
	}
	if w.queueRedrawArea[2] == 0 && w.queueRedrawArea[3] == 0 {
		return
	}

	x := int(float64(w.queueRedrawArea[0]) * w.s.ws.font.truewidth)
	y := w.queueRedrawArea[1] * w.s.ws.font.lineHeight
	width := int(float64(w.queueRedrawArea[2]-w.queueRedrawArea[0]) * w.s.ws.font.truewidth)
	height := (w.queueRedrawArea[3] - w.queueRedrawArea[1]) * w.s.ws.font.lineHeight

	if width > 0 && height > 0 {
		w.widget.Update2(
			x,
			y,
			width,
			height,
		)
	}

	w.queueRedrawArea[0] = w.cols
	w.queueRedrawArea[1] = w.rows
	w.queueRedrawArea[2] = 0
	w.queueRedrawArea[3] = 0
}

func (s *Screen) update() {
	for _, win := range s.windows {
		if win != nil {
			win.update()
		}
	}
}

func (w *Window) queueRedrawAll() {
	w.queueRedrawArea = [4]int{0, 0, w.cols, w.rows}
}

func (w *Window) queueRedraw(x, y, width, height int) {
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
}

func (w *Window) transparent(bg *RGBA) int {
	t := 255
	transparent := int(math.Trunc(editor.config.Editor.Transparent * float64(255)))

	if w.s.ws.background.equals(bg) {
		t = 0
	} else {
		t = transparent
	}
	return t
}

func (w *Window) fillHightlight(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	font := w.s.ws.font
	line := w.content[y]
	start := -1
	end := -1
	var lastBg *RGBA
	var bg *RGBA
	var lastCell *Cell
	for x := col; x < col+cols; x++ {
		if x >= len(line) {
			continue
		}
		if line[x] == nil {
			continue
		}
		if line[x].char != " " {
			continue
		}
		if line[x] != nil {
			bg = line[x].highlight.background
		} else {
			bg = nil
		}
		if lastCell != nil && !lastCell.normalWidth {
			bg = lastCell.highlight.background
		}
		if bg != nil {
			if lastBg == nil {
				start = x
				end = x
				lastBg = bg
			} else {
				if lastBg.equals(bg) {
					end = x
				} else {
					// Draw a background only when it's color differs from the entire background
					if !lastBg.equals(editor.colors.bg) {
						// last bg is different; draw the previous and start a new one
						rectF := core.NewQRectF4(
							float64(start)*font.truewidth,
							float64((y)*font.lineHeight),
							float64(end-start+1)*font.truewidth,
							float64(font.lineHeight),
						)
						p.FillRect(
							rectF,
							gui.NewQBrush3(
								gui.NewQColor3(
									lastBg.R,
									lastBg.G,
									lastBg.B,
									w.transparent(lastBg),
								),
								core.Qt__SolidPattern,
							),
						)
					}

					// start a new one
					start = x
					end = x
					lastBg = bg
				}
			}
		} else {
			if lastBg != nil {
				if !lastBg.equals(editor.colors.bg) {
					rectF := core.NewQRectF4(
						float64(start)*font.truewidth,
						float64((y)*font.lineHeight),
						float64(end-start+1)*font.truewidth,
						float64(font.lineHeight),
					)
					p.FillRect(
						rectF,
						gui.NewQBrush3(
							gui.NewQColor3(
								lastBg.R,
								lastBg.G,
								lastBg.B,
								w.transparent(lastBg),
							),
							core.Qt__SolidPattern,
						),
					)
				}

				// start a new one
				start = x
				end = x
				lastBg = nil
			}
		}
		lastCell = line[x]
	}
	if lastBg != nil {
		if !lastBg.equals(editor.colors.bg) {
			rectF := core.NewQRectF4(
				float64(start)*font.truewidth,
				float64((y)*font.lineHeight),
				float64(end-start+1)*font.truewidth,
				float64(font.lineHeight),
			)
			p.FillRect(
				rectF,
				gui.NewQBrush3(
					gui.NewQColor3(
						lastBg.R,
						lastBg.G,
						lastBg.B,
						w.transparent(lastBg),
					),
					core.Qt__SolidPattern,
				),
			)
		}
	}
}

func (w *Window) fillBackground(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	font := w.s.ws.font
	line := w.content[y]
	var bg *RGBA

	start := -1
	end := -1
	var lastBg *RGBA
	var lastCell *Cell

	// draw default background color if window is float window or msg grid
	var drawDefaultBackground bool
	if w.isFloatWin || w.isMsgGrid {
	        drawDefaultBackground = true
	}

	for x := col; x < col+cols; x++ {
		if x >= len(line) {
			continue
		}
		if line[x] == nil {
			continue
		}
		if line[x].highlight.uiName == "TermCursor" {
			continue
		}
		bg = line[x].highlight.bg()

		// if !bg.equals(w.s.ws.background) {
		// 	// Set diff pattern
		// 	pattern, color, transparent := getFillpatternAndTransparent(w.content[y][x], bg)

		// 	// Fill background with pattern
		// 	rectF := core.NewQRectF4(
		// 		float64(x)*font.truewidth,
		// 		float64((y)*font.lineHeight),
		// 		font.truewidth,
		// 		float64(font.lineHeight),
		// 	)
		// 	p.FillRect(
		// 		rectF,
		// 		gui.NewQBrush3(
		// 			gui.NewQColor3(
		// 				color.R,
		// 				color.G,
		// 				color.B,
		// 				transparent,
		// 			),
		// 			pattern,
		// 		),
		// 	)
		// }

		if lastBg == nil {
			if !drawDefaultBackground && bg.equals(w.s.ws.background) {
			        continue
			}
			start = x
			lastBg = bg
		}
		if lastBg != nil {
			if lastBg.equals(bg) {
				lastCell = line[x]
				end = x
			}
			if !lastBg.equals(bg) || x+1 == col+cols {
				width := end - start + 1
				if !drawDefaultBackground && lastBg.equals(w.s.ws.background) {
					width = 0
				}
				if width > 0 {
					// Set diff pattern
					pattern, color, transparent := getFillpatternAndTransparent(lastCell, lastBg)

					// Fill background with pattern
					rectF := core.NewQRectF4(
						float64(start)*font.truewidth,
						float64((y)*font.lineHeight),
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
				}
				start = x
				end = x
				lastBg = bg
			}
		}
	}
}

func (w *Window) drawChars(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	wsfont := w.s.ws.font
	specialChars := []int{}

	for x := col; x < col+cols; x++ {
		if x >= len(w.content[y]) {
			continue
		}

		// If not drawing background in drawchar()
		// if x >= w.lenLine[y] {
		// 	break
		// }

		cell := w.content[y][x]
		if cell == nil {
			continue
		}
		if cell.char == "" {
			continue
		}
		if !cell.normalWidth {
			specialChars = append(specialChars, x)
			continue
		}

		// If not drawing background in drawchar()
		if cell.char == " " {
			continue
		}

		// // If drawing background in drawchar()
		// if cell.highlight.background == nil {
		// 	cell.highlight.background = w.s.ws.background
		// }
		// if cell.char == " " && cell.highlight.background.equals(w.s.ws.background) {
		// 	continue
		// }
		// if !cell.highlight.background.equals(w.s.ws.background) {
		// 	// Set diff pattern
		// 	pattern, color, transparent := getFillpatternAndTransparent(cell, nil)
		//
		// 	// Fill background with pattern
		// 	rectF := core.NewQRectF4(
		// 		float64(x)*wsfont.truewidth,
		// 		float64((y)*wsfont.lineHeight),
		// 		wsfont.truewidth,
		// 		float64(wsfont.lineHeight),
		// 	)
		// 	p.FillRect(
		// 		rectF,
		// 		gui.NewQBrush3(
		// 			gui.NewQColor3(
		// 				color.R,
		// 				color.G,
		// 				color.B,
		// 				transparent,
		// 			),
		// 			pattern,
		// 		),
		// 	)
		// }

		glyph, ok := w.s.glyphMap[*cell]
		if !ok {
			glyph = w.newGlyph(p, cell)
		}
		p.DrawImage7(
			core.NewQPointF3(
				float64(x)*wsfont.truewidth,
				float64(y*wsfont.lineHeight),
			),
			&glyph,
		)
	}

	for _, x := range specialChars {
		cell := w.content[y][x]
		if cell == nil || cell.char == " " {
			continue
		}
		glyph, ok := w.s.glyphMap[*cell]
		if !ok {
			glyph = w.newGlyph(p, cell)
		}
		p.DrawImage7(
			core.NewQPointF3(
				float64(x)*wsfont.truewidth,
				float64(y*wsfont.lineHeight),
			),
			&glyph,
		)
	}

}

func getFillpatternAndTransparent(cell *Cell, color *RGBA) (core.Qt__BrushStyle, *RGBA, int) {
	if color == nil {
		color = cell.highlight.bg()
	}
	pattern := core.Qt__BrushStyle(1)
	transparent := int(transparent() * 255.0)

	if editor.config.Editor.DiffChangePattern != 1 && cell.highlight.hlName == "DiffChange" {
		pattern = core.Qt__BrushStyle(editor.config.Editor.DiffChangePattern)
		if editor.config.Editor.DiffChangePattern >= 7 &&
			editor.config.Editor.DiffChangePattern <= 14 {
			transparent = int(editor.config.Editor.Transparent * 255)
		}
		color = color.HSV().Colorfulness().RGB()
	} else if editor.config.Editor.DiffDeletePattern != 1 && cell.highlight.hlName == "DiffDelete" {
		pattern = core.Qt__BrushStyle(editor.config.Editor.DiffDeletePattern)
		if editor.config.Editor.DiffDeletePattern >= 7 &&
			editor.config.Editor.DiffDeletePattern <= 14 {
			transparent = int(editor.config.Editor.Transparent * 255)
		}
		color = color.HSV().Colorfulness().RGB()
	} else if editor.config.Editor.DiffAddPattern != 1 && cell.highlight.hlName == "DiffAdd" {
		pattern = core.Qt__BrushStyle(editor.config.Editor.DiffAddPattern)
		if editor.config.Editor.DiffAddPattern >= 7 &&
			editor.config.Editor.DiffAddPattern <= 14 {
			transparent = int(editor.config.Editor.Transparent * 255)
		}
		color = color.HSV().Colorfulness().RGB()
	}

	return pattern, color, transparent
}

func (w *Window) drawText(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	wsfont := w.s.ws.font
	font := p.Font()
	line := w.content[y]
	chars := map[Highlight][]int{}
	specialChars := []int{}

	for x := col; x < col+cols; x++ {
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

		highlight := line[x].highlight
		colorSlice, ok := chars[highlight]
		if !ok {
			colorSlice = []int{}
		}
		colorSlice = append(colorSlice, x)
		chars[highlight] = colorSlice
	}

	pointF := core.NewQPointF3(
		float64(col)*wsfont.truewidth,
		float64((y)*wsfont.lineHeight+wsfont.shift),
	)

	for highlight, colorSlice := range chars {
		var buffer bytes.Buffer
		slice := colorSlice[:]
		for x := col; x < col+cols; x++ {
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
		if text != "" {
			fg := highlight.fg()
			if fg != nil {
				p.SetPen2(gui.NewQColor3(fg.R, fg.G, fg.B, int(fg.A*255)))
			}
			font.SetBold(highlight.bold)
			font.SetItalic(highlight.italic)
			p.DrawText(pointF, text)
		}

	}

	for _, x := range specialChars {
		if line[x] == nil || line[x].char == " " {
			continue
		}
		fg := line[x].highlight.fg()
		p.SetPen2(gui.NewQColor3(fg.R, fg.G, fg.B, int(fg.A*255)))
		pointF.SetX(float64(x) * wsfont.truewidth)
		pointF.SetY(float64((y)*wsfont.lineHeight + wsfont.shift))
		font.SetBold(line[x].highlight.bold)
		font.SetItalic(line[x].highlight.italic)
		p.DrawText(pointF, line[x].char)
	}
}

func (w *Window) newGlyph(p *gui.QPainter, cell *Cell) gui.QImage {
	// * TODO: Further optimization, whether it is possible
	// * Ref: https://stackoverflow.com/questions/40458515/a-best-way-to-draw-a-lot-of-independent-characters-in-qt5/40476430#40476430

	width := w.s.ws.font.truewidth
	if !cell.normalWidth {
		width = math.Ceil(w.s.ws.font.fontMetrics.HorizontalAdvance(cell.char, -1))
	}

	char := cell.char

	// Skip draw char if
	if editor.config.Editor.DiffAddPattern != 1 && cell.highlight.hlName == "DiffAdd" {
		char = " "
	}
	if editor.config.Editor.DiffDeletePattern != 1 && cell.highlight.hlName == "DiffDelete" {
		char = " "
	}

	// // If drawing background
	// if cell.highlight.background == nil {
	// 	cell.highlight.background = w.s.ws.background
	// }
	fg := cell.highlight.fg()

	// QImage default device pixel ratio is 1.0,
	// So we set the correct device pixel ratio
	glyph := gui.NewQImage2(
		// core.NewQRectF4(
		// 	0,
		// 	0,
		// 	w.devicePixelRatio*width,
		// 	w.devicePixelRatio*float64(w.s.ws.font.lineHeight),
		// ).Size().ToSize(),
		core.NewQSize2(
			int(w.devicePixelRatio*width),
			int(w.devicePixelRatio*float64(w.s.ws.font.lineHeight)),
		),
		gui.QImage__Format_ARGB32_Premultiplied,
	)
	glyph.SetDevicePixelRatio(w.devicePixelRatio)

	// // If drawing background
	// glyph.Fill2(gui.NewQColor3(
	// 	cell.highlight.background.R,
	// 	cell.highlight.background.G,
	// 	cell.highlight.background.B,
	// 	int(editor.config.Editor.Transparent * 255),
	// ))
	glyph.Fill3(core.Qt__transparent)

	p = gui.NewQPainter2(glyph)
	p.SetPen2(gui.NewQColor3(
		fg.R,
		fg.G,
		fg.B,
		255))

	p.SetFont(w.s.ws.font.fontNew)
	if cell.highlight.bold {
		p.Font().SetBold(true)
	}
	if cell.highlight.italic {
		p.Font().SetItalic(true)
	}

	p.DrawText6(
		core.NewQRectF4(
			0,
			0,
			width,
			float64(w.s.ws.font.lineHeight),
		),
		char,
		gui.NewQTextOption2(core.Qt__AlignVCenter),
	)

	w.s.glyphMap[*cell] = *glyph

	return *glyph
}

func (w *Window) drawContents(p *gui.QPainter, y int, col int, cols int) {
	if editor.config.Editor.CachedDrawing {
		w.drawChars(p, y, col, cols)
	} else {
		w.drawText(p, y, col, cols)
	}
}

func (w *Window) drawTextDecoration(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	line := w.content[y]
	lenLine := w.lenLine[y]
	font := w.s.ws.font
	for x := col; x < col+cols; x++ {
		if x > lenLine {
			break
		}
		if x >= len(line) {
			continue
		}
		if line[x] == nil {
			continue
		}
		if !line[x].highlight.underline && !line[x].highlight.undercurl {
			continue
		}
		pen := gui.NewQPen()
		sp := line[x].highlight.special
		if sp != nil {
			color := gui.NewQColor3(sp.R, sp.G, sp.B, 255)
			pen.SetColor(color)
		} else {
			fg := editor.colors.fg
			color := gui.NewQColor3(fg.R, fg.G, fg.B, 255)
			pen.SetColor(color)
		}
		p.SetPen(pen)
		start := float64(x) * font.truewidth
		end := float64(x+1) * font.truewidth
		Y := float64((y)*font.lineHeight) + font.ascent + float64(font.lineSpace)
		if line[x].highlight.underline {
			linef := core.NewQLineF3(start, Y, end, Y)
			p.DrawLine(linef)
		} else if line[x].highlight.undercurl {
			height := font.ascent / 3.0
			amplitude := font.ascent / 8.0
			freq := 1.0
			phase := 0.0
			y := Y + height/2 + amplitude*math.Sin(0)
			point := core.NewQPointF3(start, y)
			path := gui.NewQPainterPath2(point)
			for i := int(point.X()); i <= int(end); i++ {
				y = Y + height/2 + amplitude*math.Sin(2*math.Pi*freq*float64(i)/font.truewidth+phase)
				path.LineTo(core.NewQPointF3(float64(i), y))
			}
			p.DrawPath(path)
		}
	}
}

func (s *Screen) isNormalWidth(char string) bool {
	if len(char) == 0 {
		return true
	}
	if char[0] <= 127 {
		return true
	}
	return s.ws.font.fontMetrics.HorizontalAdvance(char, -1) == s.ws.font.truewidth
}

func (s *Screen) windowPosition(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		id := util.ReflectToInt(arg.([]interface{})[1])
		row := util.ReflectToInt(arg.([]interface{})[2])
		col := util.ReflectToInt(arg.([]interface{})[3])

		if isSkipGlobalId(gridid) {
			continue
		}

		win := s.windows[gridid]
		if win == nil {
			continue
		}

		win.id = *(*nvim.Window)(unsafe.Pointer(&id))
		win.pos[0] = col
		win.pos[1] = row
		win.move(col, row)
		win.hideOverlappingWindows()
		win.show()
	}

}

func (s *Screen) gridDestroy(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		if isSkipGlobalId(gridid) {
			continue
		}
		if s.windows[gridid] == nil {
			continue
		}
		s.windows[gridid].hide()
		s.windows[gridid] = nil
	}
}

func (s *Screen) windowFloatPosition(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		if isSkipGlobalId(gridid) {
			continue
		}
		id := util.ReflectToInt(arg.([]interface{})[1])
		s.windows[gridid].id = *(*nvim.Window)((unsafe.Pointer)(&id))
		s.windows[gridid].anchor = util.ReflectToInt(arg.([]interface{})[2])
		anchorGrid := util.ReflectToInt(arg.([]interface{})[3])
		anchorRow := int(util.ReflectToFloat(arg.([]interface{})[4]))
		anchorCol := int(util.ReflectToFloat(arg.([]interface{})[5]))
		// focusable := arg.([]interface{})[6]

		s.windows[gridid].pos[0] = anchorCol
		s.windows[gridid].pos[1] = anchorRow
		s.windows[gridid].isFloatWin = true

		x := s.windows[anchorGrid].pos[0] + s.windows[gridid].pos[0]
		y := s.windows[anchorGrid].pos[1] + s.windows[gridid].pos[1]
		s.windows[gridid].move(x, y)
		s.windows[gridid].setShadow()

		s.windows[gridid].show()
	}
}

func (s *Screen) windowHide(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		if isSkipGlobalId(gridid) {
			continue
		}
		if s.windows[gridid] == nil {
			continue
		}
		s.windows[gridid].hide()
	}
}

func (s *Screen) windowScrollOverStart() {
	// main purposs is to scroll over the windows in the `grid_line` event
	// when messages is shown
	s.isScrollOver = true
}

func (s *Screen) windowScrollOverReset() {
	s.isScrollOver = false
	s.scrollOverCount = 0
	for _, win := range s.windows {
		if win == nil {
			continue
		}
		if win.grid != 1 {
			win.move(win.pos[0], win.pos[1])
		}
	}

	// reset message contents in global grid
	gwin := s.windows[1]
	content := make([][]*Cell, gwin.rows)
	lenLine := make([]int, gwin.rows)

	for i := 0; i < gwin.rows; i++ {
		content[i] = make([]*Cell, gwin.cols)
	}

	for i := 0; i < gwin.rows; i++ {
		if i >= len(gwin.content) {
			continue
		}
		lenLine[i] = gwin.cols
		for j := 0; j < gwin.cols; j++ {
			if j >= len(gwin.content[i]) {
				continue
			}
			if gwin.content[i][j] == nil {
				continue
			}
			if gwin.content[i][j].highlight.hlName == "StatusLine" ||
				gwin.content[i][j].highlight.hlName == "StatusLineNC" ||
				gwin.content[i][j].highlight.hlName == "VertSplit" {
				content[i][j] = gwin.content[i][j]
			}
		}
	}
	s.windows[1].content = content
	s.windows[1].lenLine = lenLine

	gwin.queueRedrawAll()
}

func (s *Screen) msgSetPos(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		msgCount := util.ReflectToInt(arg.([]interface{})[1])
		win := s.windows[gridid]
		win.isMsgGrid = true
		win.pos[1] = msgCount
		win.move(win.pos[0], win.pos[1])
		win.show()
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

func newWindow() *Window {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	widget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")

	var devicePixelRatio float64
	if runtime.GOOS == "darwin" {
		devicePixelRatio = 2.0
	} else {
		devicePixelRatio = 1.0
	}

	w := &Window{
		widget:           widget,
		scrollRegion:     []int{0, 0, 0, 0},
		devicePixelRatio: devicePixelRatio,
	}

	widget.ConnectPaintEvent(w.paint)

	return w
}

func (w *Window) isShown() bool {
	if w == nil {
		return false
	}
	if !w.shown {
		return false
	}

	return true
}

func (w *Window) raise() {
	if w.grid == 1 {
		return
	}
	w.widget.Raise()
	w.s.tooltip.SetParent(w.widget)
	w.s.ws.cursor.widget.SetParent(w.widget)
	w.s.ws.cursor.widget.Hide()
	w.s.ws.cursor.widget.Show()
}

func (w *Window) hideOverlappingWindows() {
	if w.isMsgGrid {
		return
	}
	for _, win := range w.s.windows {
		if win == nil {
			continue
		}
		if win.grid == 1 {
			continue
		}
		if w == win {
			continue
		}
		if win.isMsgGrid {
			continue
		}
		if !win.shown {
			continue
		}
		if w.widget.Geometry().Contains2(win.widget.Geometry(), false) {
			win.hide()
		}
	}
}

func (w *Window) show() {
	// w.hideOverlappingWindows()
	w.widget.Show()
	w.shown = true
}

func (w *Window) hide() {
	w.widget.Hide()
	w.shown = false
}

func (w *Window) setParent(a widgets.QWidget_ITF) {
	w.widget.SetParent(a)
}

func (w *Window) setGeometry(rect core.QRect_ITF) {
	w.widget.SetGeometry(rect)
}

func (w *Window) setShadow() {
	w.widget.SetGraphicsEffect(util.DropShadow(-2, 6, 40, 100))
}

func (w *Window) move(col int, row int) {
	res := 0
	if w.isMsgGrid {
		res = w.s.widget.Height() - w.rows*w.s.ws.font.lineHeight
	}
	if res < 0 {
		res = 0
	}
	x := int(float64(col) * w.s.ws.font.truewidth)
	y := row*int(w.s.ws.font.lineHeight) + res
	w.widget.Move2(x, y)

}

func isSkipGlobalId(id gridId) bool {
	if editor.config.Editor.SkipGlobalId {
		if id == 1 {
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}
