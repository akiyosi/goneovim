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

// Window is
type Window struct {
	paintMutex  sync.Mutex
	redrawMutex sync.Mutex

	s       *Screen
	content [][]*Char

	id     nvim.Window
	pos    [2]int
	anchor int
	cols   int
	rows   int

	widget          *widgets.QWidget
	queueRedrawArea [4]int
	scrollRegion    []int

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
	highlight        Highlight
	curtab           nvim.Tabpage
	curWins          map[nvim.Window]*Window
	content          [][]*Char
	queueRedrawArea  [4]int
	paintMutex       sync.Mutex
	redrawMutex      sync.Mutex
	drawSplit        bool
	resizeCount      uint
	tooltip          *widgets.QLabel
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
		font := gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false)
		s.tooltip.SetFont(font)
		x = w.palette.cursorX + w.palette.patternPadding + 8
		candX = x + w.palette.widget.Pos().X()
		y = w.palette.patternPadding + 8
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
	s.tooltip.Move(core.NewQPoint2(x, y))
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
	defer w.paintMutex.Unlock()

	rect := event.M_rect()
	top := rect.Y()
	left := rect.X()

	font := w.s.ws.font
	row := int(float64(top) / float64(font.lineHeight))
	col := int(float64(left) / font.truewidth)
	rows := w.rows
	cols := w.cols

	p := gui.NewQPainter2(w.widget)
	p.SetFont(font.fontNew)

	for y := row; y < row+rows; y++ {
		if w == w.s.windows[1] && w.queueRedrawArea[2] == 0 && w.queueRedrawArea[3] == 0 {
			break
		}
		if y >= w.rows {
			continue
		}
		w.fillHightlight(p, y, col, cols, [2]int{0, 0})
		w.drawText(p, y, col, cols)
	}

	if editor.config.Editor.DrawBorder && w == w.s.windows[1] {
		for _, win := range w.s.windows {
			win.drawBorder(p)
		}
	}

	if editor.config.Editor.IndentGuide && w == w.s.windows[1] {
		for _, win := range w.s.windows {
			win.drawIndentguide(p)
		}
	}

	if w != w.s.windows[1] {
		w.s.ws.markdown.updatePos()
	}

	p.DestroyQPainter()
}

func (w *Window) drawIndentguide(p *gui.QPainter) {
	if w == nil {
		return
	}
	for y := 0; y < len(w.content); y++ {
		if y+1 == len(w.content) {
			return
		}
		nline := w.content[y+1]
		line := w.content[y]
		res := 0
		for x := 0; x < len(line); x++ {
			if x+1 == len(line) {
				break
			}
			nd := nline[x+1]
			if nd == nil {
				continue
			}
			d := nline[x]
			if d == nil {
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
				d.char == " " && nd.char == " " {
				for row := y; row < len(w.content); row++ {
					if row+1 == len(w.content) {
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

func (w *Window) drawIndentline(p *gui.QPainter, x int, y int) {
	X := float64(x) * w.s.ws.font.truewidth
	Y := float64(y * w.s.ws.font.lineHeight)
	p.FillRect4(
		core.NewQRectF4(
			X,
			Y,
			1,
			float64(w.s.ws.font.lineHeight),
		),
		gui.NewQColor3(
			editor.colors.windowSeparator.R,
			editor.colors.windowSeparator.G,
			editor.colors.windowSeparator.B,
			255),
	)
}

func (w *Window) drawBorder(p *gui.QPainter) {
	if w == nil {
		return
	}
	x := int(float64(w.pos[0]) * w.s.ws.font.truewidth)
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

	// Horizontal
	height := w.rows * int(w.s.ws.font.lineHeight)
	y2 := y + height - 1 + w.s.ws.font.lineHeight/2
	if y2+w.s.ws.font.lineHeight+w.s.ws.font.lineHeight/2 > w.s.widget.Height() {
		return
	}
	p.FillRect5(
		int(float64(x)-w.s.ws.font.truewidth/2),
		y2,
		int((float64(w.cols)+0.92)*w.s.ws.font.truewidth),
		1,
		color,
	)
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
			horiz = int(math.Trunc(float64(s.scrollDust[0]) / fontheight))
			s.scrollDust[0] = 0
		}
		if dy >= fontwidth {
			vert = int(math.Trunc(float64(s.scrollDust[1]) / fontwidth))
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
	content := make([][]*Char, rows)

	for i := 0; i < rows; i++ {
		content[i] = make([]*Char, cols)
	}

	if win != nil && gridid != 1 {
		for i := 0; i < rows; i++ {
			if i >= len(win.content) {
				continue
			}
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
		s.windows[gridid].widget.SetParent(s.widget)
		s.windows[gridid].widget.SetAttribute(core.Qt__WA_KeyCompression, true)
		s.windows[gridid].widget.SetAcceptDrops(true)
		s.windows[gridid].widget.ConnectWheelEvent(s.wheelEvent)
		s.windows[gridid].widget.ConnectDragEnterEvent(s.dragEnterEvent)
		s.windows[gridid].widget.ConnectDragMoveEvent(s.dragMoveEvent)
		s.windows[gridid].widget.ConnectDropEvent(s.dropEvent)
		// reassign win
		win = s.windows[gridid]
	}

	win.content = content
	win.cols = cols
	win.rows = rows

	width := int(float64(cols) * s.ws.font.truewidth)
	height := rows * int(s.ws.font.lineHeight)
	rect := core.NewQRect4(0, 0, width, height)
	win.widget.SetGeometry(rect)
	win.move(win.pos[0], win.pos[1])
	win.widget.Show()
	win.raise()

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

	uiName, ok := info["ui_name"]
	if ok {
		highlight.uiName = uiName.(string)
	}

	hiName, ok := info["hi_name"]
	if ok {
		highlight.hiName = hiName.(string)
	}

	bold := hl["bold"]
	if bold != nil {
		highlight.bold = true
	} else {
		highlight.bold = false
	}

	italic := hl["italic"]
	if italic != nil {
		highlight.italic = true
	} else {
		highlight.italic = false
	}

	_, ok = hl["reverse"]
	if ok {
		highlight.foreground = s.highlight.background
		highlight.background = s.highlight.foreground
		s.highlight = highlight
		return &highlight
	}

	fg, ok := hl["foreground"]
	if ok {
		rgba := calcColor(util.ReflectToInt(fg))
		highlight.foreground = rgba
	} else {
		highlight.foreground = s.ws.foreground
	}

	bg, ok := hl["background"]
	if ok {
		rgba := calcColor(util.ReflectToInt(bg))
		highlight.background = rgba
	} else {
		highlight.background = s.ws.background
	}

	return &highlight
}

func (s *Screen) gridClear(args []interface{}) {
	var gridid gridId
	for _, arg := range args {
		gridid = util.ReflectToInt(arg.([]interface{})[0])
		if isSkipGlobalId(gridid) {
			continue
		}

		s.windows[gridid].content = make([][]*Char, s.windows[gridid].rows)

		for i := 0; i < s.windows[gridid].rows; i++ {
			s.windows[gridid].content[i] = make([]*Char, s.windows[gridid].cols)
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
	}
}

func (s *Screen) updateGridContent(arg []interface{}) {
	gridid := util.ReflectToInt(arg[0])
	row := util.ReflectToInt(arg[1])
	col := util.ReflectToInt(arg[2])

	if isSkipGlobalId(gridid) {
		return
	}

	content := s.windows[gridid].content
	if row >= s.windows[gridid].rows {
		return
	}
	line := content[row]
	cells := arg[3].([]interface{})

	oldNormalWidth := true
	lastChar := &Char{}

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

		makeCells := func() {
			if line[col] != nil && !line[col].normalWidth {
				oldNormalWidth = false
			} else {
				oldNormalWidth = true
			}

			if line[col] == nil {
				line[col] = &Char{}
			}

			line[col].char = text.(string)
			line[col].normalWidth = s.isNormalWidth(line[col].char)
			lastChar = line[col]

			switch col {
			case 0:
				line[col].highlight = *(s.highAttrDef[hi])
			default:
				if hi == -1 {
					line[col].highlight = line[col-1].highlight
				} else {
					line[col].highlight = *s.highAttrDef[hi]
				}
			}
			col++
		} // end of makeCells()

		r := 1
		if repeat == 0 {
			repeat = 1
		}
		for r <= repeat {
			if col >= len(line) {
				break
			}
			makeCells()
			r++
		}
		s.windows[gridid].queueRedraw(0, row, s.windows[gridid].cols, 1)
	}
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

func (c *Char) isSignColumn() bool {
	switch c.highlight.hiName {
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
	if s.highAttrDef[hi].hiName == "StatusLine" ||
		s.highAttrDef[hi].hiName == "StatusLineNC" ||
		s.highAttrDef[hi].hiName == "VertSplit" {
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

func (w *Window) fillHightlight(p *gui.QPainter, y int, col int, cols int, pos [2]int) {
	if y >= len(w.content) {
		return
	}
	rectF := core.NewQRectF()
	font := w.s.ws.font
	line := w.content[y]
	start := -1
	end := -1
	var lastBg *RGBA
	var bg *RGBA
	var lastChar *Char
	for x := col; x < col+cols; x++ {
		if x >= len(line) {
			continue
		}
		char := line[x]
		if char != nil {
			bg = char.highlight.background
		} else {
			bg = nil
		}
		if lastChar != nil && !lastChar.normalWidth {
			bg = lastChar.highlight.background
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
					// last bg is different; draw the previous and start a new one
					rectF.SetRect(
						float64(start)*font.truewidth,
						float64((y)*font.lineHeight),
						float64(end-start+1)*font.truewidth,
						float64(font.lineHeight),
					)
					p.FillRect(
						rectF,
						gui.NewQBrush3(gui.NewQColor3(lastBg.R, lastBg.G, lastBg.B, w.transparent(lastBg)), core.Qt__SolidPattern),
					)

					// start a new one
					start = x
					end = x
					lastBg = bg
				}
			}
		} else {
			if lastBg != nil {
				rectF.SetRect(
					float64(start)*font.truewidth,
					float64((y)*font.lineHeight),
					float64(end-start+1)*font.truewidth,
					float64(font.lineHeight),
				)
				p.FillRect(
					rectF,
					gui.NewQBrush3(gui.NewQColor3(lastBg.R, lastBg.G, lastBg.B, w.transparent(lastBg)), core.Qt__SolidPattern),
				)

				// start a new one
				start = x
				end = x
				lastBg = nil
			}
		}
		lastChar = char
	}
	if lastBg != nil {
		rectF.SetRect(
			float64(start)*font.truewidth,
			float64((y)*font.lineHeight),
			float64(end-start+1)*font.truewidth,
			float64(font.lineHeight),
		)
		p.FillRect(
			rectF,
			gui.NewQBrush3(gui.NewQColor3(lastBg.R, lastBg.G, lastBg.B, w.transparent(lastBg)), core.Qt__SolidPattern),
		)
	}
}

func (w *Window) drawText(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	wsfont := w.s.ws.font
	font := p.Font()
	font.SetBold(false)
	font.SetItalic(false)
	pointF := core.NewQPointF()
	line := w.content[y]
	chars := map[Highlight][]int{}
	specialChars := []int{}

	for x := col; x < col+cols; x++ {
		if x >= len(line) {
			continue
		}
		char := line[x]
		if char == nil {
			continue
		}
		if char.char == " " {
			continue
		}
		if char.char == "" {
			continue
		}
		if !char.normalWidth {
			specialChars = append(specialChars, x)
			continue
		}
		highlight := Highlight{}
		fg := char.highlight.foreground
		if fg == nil {
			fg = w.s.ws.foreground
		}
		highlight.foreground = fg
		highlight.italic = char.highlight.italic
		highlight.bold = char.highlight.bold

		colorSlice, ok := chars[highlight]
		if !ok {
			colorSlice = []int{}
		}
		colorSlice = append(colorSlice, x)
		chars[highlight] = colorSlice
	}

	X := float64(col) * wsfont.truewidth
	Y := float64((y)*wsfont.lineHeight + wsfont.shift)

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
			fg := highlight.foreground
			if fg != nil {
				p.SetPen2(gui.NewQColor3(fg.R, fg.G, fg.B, int(fg.A*255)))
			}
			pointF.SetX(X)
			pointF.SetY(Y)
			font.SetBold(highlight.bold)
			font.SetItalic(highlight.italic)
			p.DrawText(pointF, text)
		}

	}

	for _, x := range specialChars {
		char := line[x]
		if char == nil || char.char == " " {
			continue
		}
		fg := char.highlight.foreground
		if fg == nil {
			fg = w.s.ws.foreground
		}
		p.SetPen2(gui.NewQColor3(fg.R, fg.G, fg.B, int(fg.A*255)))
		pointF.SetX(float64(x) * wsfont.truewidth)
		pointF.SetY(float64((y)*wsfont.lineHeight + wsfont.shift))
		font.SetBold(char.highlight.bold)
		font.SetItalic(char.highlight.italic)
		p.DrawText(pointF, char.char)
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

func newWindow() *Window {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	widget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")

	w := &Window{
		widget:       widget,
		scrollRegion: []int{0, 0, 0, 0},
	}

	widget.ConnectPaintEvent(w.paint)

	return w
}

func (s *Screen) windowPosition(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		id := util.ReflectToInt(arg.([]interface{})[1])
		startRow := util.ReflectToInt(arg.([]interface{})[2])
		startCol := util.ReflectToInt(arg.([]interface{})[3])

		if isSkipGlobalId(gridid) {
			continue
		}

		win := s.windows[gridid]
		if win == nil {
			continue
		}

		win.id = *(*nvim.Window)(unsafe.Pointer(&id))

		win.pos[0] = startCol
		win.pos[1] = startRow
		win.move(startCol, startRow)
		win.widget.Show()
	}

}

func (s *Screen) gridDestroy(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		if isSkipGlobalId(gridid) {
			continue
		}
		s.windows[gridid].widget.Hide()
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
		// why float types??
		anchorRow := int(util.ReflectToFloat(arg.([]interface{})[4]))
		anchorCol := int(util.ReflectToFloat(arg.([]interface{})[5]))
		// focusable := arg.([]interface{})[6]

		s.windows[gridid].pos[0] = anchorCol
		s.windows[gridid].pos[1] = anchorRow
		s.windows[gridid].move(s.windows[gridid].pos[0], s.windows[gridid].pos[1])
		s.windows[gridid].widget.SetParent(s.windows[anchorGrid].widget)

		shadow := widgets.NewQGraphicsDropShadowEffect(nil)
		shadow.SetBlurRadius(38)
		shadow.SetColor(gui.NewQColor3(0, 0, 0, 200))
		shadow.SetOffset3(-2, 6)
		s.windows[gridid].widget.SetGraphicsEffect(shadow)

		s.windows[gridid].widget.Show()
	}
}

func (s *Screen) windowHide(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		if isSkipGlobalId(gridid) {
			continue
		}
		s.windows[gridid].widget.Hide()
	}
}

func (s *Screen) windowClose(args []interface{}) {
	for _, arg := range args {
		gridid := util.ReflectToInt(arg.([]interface{})[0])
		if isSkipGlobalId(gridid) {
			continue
		}
		win := s.windows[gridid]
		s.ws.nvim.SetCurrentWindow(win.id)
	}
}

func (w *Window) raise() {
	if w == w.s.windows[1] {
		return
	}
	w.widget.Raise()
	w.s.tooltip.SetParent(w.widget)
	w.s.ws.cursor.widget.SetParent(w.widget)
	w.s.ws.cursor.widget.Hide()
	w.s.ws.cursor.widget.Show()
}

func (w *Window) move(col int, row int) {
	x := int(float64(col) * w.s.ws.font.truewidth)
	y := row * int(w.s.ws.font.lineHeight)
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
