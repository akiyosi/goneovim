package editor

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

// Window is
type Window struct {
	win        nvim.Window
	width      int
	height     int
	pos        [2]int
	tab        nvim.Tabpage
	hl         string
	bg         *RGBA
	statusline bool
	bufName    string
}

// Screen is the main editor area
type Screen struct {
	bg               *RGBA
	width            int
	height           int
	widget           *widgets.QWidget
	ws               *Workspace
	wins             map[nvim.Window]*Window
	cursor           [2]int
	lastCursor       [2]int
	content          [][]*Char
	scrollRegion     []int
	scrollDust       [2]int
	scrollDustDeltaY int
	curtab           nvim.Tabpage
	cmdheight        int
	highlight        Highlight
	curWins          map[nvim.Window]*Window
	queueRedrawArea  [4]int
	paintMutex       sync.Mutex
	redrawMutex      sync.Mutex
	drawSplit        bool
	tooltip          *widgets.QLabel
}

func newScreen() *Screen {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	widget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")

	tooltip := widgets.NewQLabel(widget, 0)
	tooltip.SetVisible(false)

	screen := &Screen{
		widget:       widget,
		cursor:       [2]int{0, 0},
		lastCursor:   [2]int{0, 0},
		scrollRegion: []int{0, 0, 0, 0},
		tooltip:      tooltip,
	}

	widget.ConnectPaintEvent(screen.paint)
	widget.ConnectMousePressEvent(screen.mouseEvent)
	widget.ConnectMouseReleaseEvent(screen.mouseEvent)
	widget.ConnectMouseMoveEvent(screen.mouseEvent)
	widget.ConnectWheelEvent(screen.wheelEvent)
	widget.ConnectResizeEvent(func(event *gui.QResizeEvent) {
		screen.updateSize()
	})
	widget.SetAttribute(core.Qt__WA_KeyCompression, false)
	widget.SetAcceptDrops(true)

	widget.ConnectDragEnterEvent(screen.dragEnterEvent)
	widget.ConnectDragMoveEvent(screen.dragMoveEvent)
	widget.ConnectDropEvent(screen.dropEvent)

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

func (s *Screen) updateSize() {
	w := s.ws
	isColDiff := s.updateCols()
	isRowDiff := s.updateRows()
	isTryResize := isColDiff || isRowDiff
	if w.uiAttached && isTryResize {
		w.nvim.TryResizeUI(w.cols, w.rows)
	}
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

func (s *Screen) paint(vqp *gui.QPaintEvent) {
	s.paintMutex.Lock()
	defer s.paintMutex.Unlock()

	rect := vqp.M_rect()
	font := s.ws.font
	top := rect.Y()
	left := rect.X()
	width := rect.Width()
	height := rect.Height()
	right := left + width
	bottom := top + height
	row := int(float64(top) / float64(font.lineHeight))
	col := int(float64(left) / font.truewidth)
	rows := int(math.Ceil(float64(bottom)/float64(font.lineHeight))) - row
	cols := int(math.Ceil(float64(right)/font.truewidth)) - col

	p := gui.NewQPainter2(s.widget)
	if s.ws.background != nil {
		p.FillRect5(
			left,
			top,
			width,
			height,
			s.ws.background.QColor(),
		)
	}

	p.SetFont(font.fontNew)

	for y := row; y < row+rows; y++ {
		if y >= s.ws.rows {
			continue
		}
		s.fillHightlight(p, y, col, cols, [2]int{0, 0})
		s.drawText(p, y, col, cols, [2]int{0, 0})
	}

	s.drawWindows(p, row, col, rows, cols)
	p.DestroyQPainter()
	s.ws.markdown.updatePos()
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

		fontheight := float64(float64(font.lineHeight))
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

func (s *Screen) drawWindows(p *gui.QPainter, row, col, rows, cols int) {
	done := make(chan struct{}, 1000)
	go func() {
		s.getWindows()
		close(done)
	}()
	select {
	case <-done:
	//case <-time.After(1 * time.Millisecond):
	case <-time.After(500 * time.Microsecond):
	}
	// for _, win := range s.curWins {
	// 	if win.pos[0]+win.height < row && (win.pos[1]+win.width+1) < col {
	// 		continue
	// 	}
	// 	if win.pos[0] > (row+rows) && (win.pos[1]+win.width) > (col+cols) {
	// 		continue
	// 	}
	// 	win.drawBorder(p, s)
	// }
}

func (s *Screen) getWindows() {
	wins := map[nvim.Window]*Window{}
	neovim := s.ws.nvim
	curtab, _ := neovim.CurrentTabpage()
	s.curtab = curtab
	nwins, _ := neovim.TabpageWindows(curtab)
	b := neovim.NewBatch()
	for _, nwin := range nwins {
		win := &Window{
			win: nwin,
		}
		b.WindowWidth(nwin, &win.width)
		b.WindowHeight(nwin, &win.height)
		b.WindowPosition(nwin, &win.pos)
		b.WindowTabpage(nwin, &win.tab)
		wins[nwin] = win
	}
	b.Option("cmdheight", &s.cmdheight)
	err := b.Execute()
	if err != nil {
		return
	}
	s.curWins = wins
	for _, win := range s.curWins {
		buf, _ := neovim.WindowBuffer(win.win)
		win.bufName, _ = neovim.BufferName(buf)

		if win.height+win.pos[0] < s.ws.rows-s.cmdheight {
			win.statusline = true
		} else {
			win.statusline = false
		}
		neovim.WindowOption(win.win, "winhl", &win.hl)
		if win.hl != "" {
			parts := strings.Split(win.hl, ",")
			for _, part := range parts {
				if strings.HasPrefix(part, "Normal:") {
					hl := part[7:]
					result := ""
					neovim.Eval(fmt.Sprintf("synIDattr(hlID('%s'), 'bg')", hl), &result)
					if result != "" {
						var r, g, b int
						format := "#%02x%02x%02x"
						n, err := fmt.Sscanf(result, format, &r, &g, &b)
						if err != nil {
							continue
						}
						if n != 3 {
							continue
						}
						win.bg = newRGBA(r, g, b, 1)
					}
				}
			}
		}
	}
}

func (s *Screen) size() (int, int) {
	geo := s.widget.Geometry()
	return geo.Width(), geo.Height()
}

func (s *Screen) resize(args []interface{}) {
	s.cursor[0] = 0
	s.cursor[1] = 0
	s.content = make([][]*Char, s.ws.rows)
	for i := 0; i < s.ws.rows; i++ {
		s.content[i] = make([]*Char, s.ws.cols)
	}
	s.queueRedrawAll()
}

func (s *Screen) clear(args []interface{}) {
	s.cursor[0] = 0
	s.cursor[1] = 0
	s.content = make([][]*Char, s.ws.rows)
	for i := 0; i < s.ws.rows; i++ {
		s.content[i] = make([]*Char, s.ws.cols)
	}
	s.queueRedrawAll()
}

func (s *Screen) eolClear(args []interface{}) {
	row := s.cursor[0]
	col := s.cursor[1]
	if row >= s.ws.rows {
		return
	}
	line := s.content[row]
	numChars := 0
	for x := col; x < len(line); x++ {
		line[x] = nil
		numChars++
	}
	s.queueRedraw(col, row, numChars+1, 1)
}

func (s *Screen) cursorGoto(args []interface{}) {
	pos, _ := args[0].([]interface{})
	s.cursor[0] = reflectToInt(pos[0])
	s.cursor[1] = reflectToInt(pos[1])
}

func (s *Screen) put(args []interface{}) {
	numChars := 0
	x := s.cursor[1]
	y := s.cursor[0]
	row := s.cursor[0]
	col := s.cursor[1]
	if row >= s.ws.rows {
		return
	}
	line := s.content[row]
	oldFirstNormal := true
	if x >= len(line) {
		x = len(line) - 1
	}
	char := line[x] // sometimes crash at this line
	if char != nil && !char.normalWidth {
		oldFirstNormal = false
	}
	var lastChar *Char
	oldNormalWidth := true
	for _, arg := range args {
		chars := arg.([]interface{})
		for _, c := range chars {
			if col >= len(line) {
				continue
			}
			char := line[col]
			if char != nil && !char.normalWidth {
				oldNormalWidth = false
			} else {
				oldNormalWidth = true
			}
			if char == nil {
				char = &Char{}
				line[col] = char
			}
			char.char = c.(string)
			char.normalWidth = s.isNormalWidth(char.char)
			lastChar = char
			char.highlight = s.highlight
			col++
			numChars++
		}
	}
	if lastChar != nil && !lastChar.normalWidth {
		numChars++
	}
	if !oldNormalWidth {
		numChars++
	}
	s.cursor[1] = col
	if x > 0 {
		char := line[x-1]
		if char != nil && char.char != "" && !char.normalWidth {
			x--
			numChars++
		} else {
			if !oldFirstNormal {
				x--
				numChars++
			}
		}
	}
	s.queueRedraw(x, y, numChars, 1)
}

func (s *Screen) highlightSet(args []interface{}) {
	for _, arg := range args {
		hl := arg.([]interface{})[0].(map[string]interface{})
		highlight := Highlight{}

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

		_, ok := hl["reverse"]
		if ok {
			highlight.foreground = s.highlight.background
			highlight.background = s.highlight.foreground
			s.highlight = highlight
			continue
		}

		fg, ok := hl["foreground"]
		if ok {
			rgba := calcColor(reflectToInt(fg))
			highlight.foreground = rgba
		} else {
			highlight.foreground = s.ws.foreground
		}

		bg, ok := hl["background"]
		if ok {
			rgba := calcColor(reflectToInt(bg))
			highlight.background = rgba
		} else {
			highlight.background = s.ws.background
		}
		s.highlight = highlight
		//s.ws.minimap.highlight = highlight
	}
}

func (s *Screen) setScrollRegion(args []interface{}) {
	arg := args[0].([]interface{})
	top := reflectToInt(arg[0])
	bot := reflectToInt(arg[1])
	left := reflectToInt(arg[2])
	right := reflectToInt(arg[3])
	s.scrollRegion[0] = top
	s.scrollRegion[1] = bot
	s.scrollRegion[2] = left
	s.scrollRegion[3] = right
}

func (s *Screen) scroll(args []interface{}) {
	count := int(args[0].([]interface{})[0].(int64))

	top := s.scrollRegion[0]
	bot := s.scrollRegion[1]
	left := s.scrollRegion[2]
	right := s.scrollRegion[3]

	if top == 0 && bot == 0 && left == 0 && right == 0 {
		top = 0
		bot = s.ws.rows - 1
		left = 0
		right = s.ws.cols - 1
	}

	s.queueRedraw(left, top, (right - left + 1), (bot - top + 1))

	if count > 0 {
		for row := top; row <= bot-count; row++ {
			for col := left; col <= right; col++ {
				if len(s.content) <= row+count {
					continue
				}
				for _, line := range s.content {
					if len(line) <= col {
						return
					}
				}
				s.content[row][col] = s.content[row+count][col]
			}
		}
		for row := bot - count + 1; row <= bot; row++ {
			for col := left; col <= right; col++ {
				s.content[row][col] = nil
			}
		}
		s.queueRedraw(left, (bot - count + 1), (right - left), count)
		if top > 0 {
			s.queueRedraw(left, (top - count), (right - left), count)
		}
	} else {
		for row := bot; row >= top-count; row-- {
			for col := left; col <= right; col++ {
				if len(s.content) <= row+count {
					continue
				}
				for _, line := range s.content {
					if len(line) <= col {
						return
					}
				}
				s.content[row][col] = s.content[row+count][col]
			}
		}
		for row := top; row < top-count; row++ {
			for col := left; col <= right; col++ {
				s.content[row][col] = nil
			}
		}
		s.queueRedraw(left, top, (right - left), -count)
		if bot < s.ws.rows-1 {
			s.queueRedraw(left, bot+1, (right - left), -count)
		}
	}
}

func (s *Screen) update() {
	x := s.queueRedrawArea[0]
	y := s.queueRedrawArea[1]
	width := s.queueRedrawArea[2] - x
	height := s.queueRedrawArea[3] - y
	if width > 0 && height > 0 {
		// s.item.SetPixmap(s.pixmap)
		s.widget.Update2(
			int(float64(x)*s.ws.font.truewidth),
			y*s.ws.font.lineHeight,
			int(float64(width)*s.ws.font.truewidth),
			height*s.ws.font.lineHeight,
		)
	}
	s.queueRedrawArea[0] = s.ws.cols
	s.queueRedrawArea[1] = s.ws.rows
	s.queueRedrawArea[2] = 0
	s.queueRedrawArea[3] = 0
}

func (s *Screen) queueRedrawAll() {
	s.queueRedrawArea = [4]int{0, 0, s.ws.cols, s.ws.rows}
}

func (s *Screen) redraw() {
	s.queueRedrawArea = [4]int{s.ws.cols, s.ws.rows, 0, 0}
}

func (s *Screen) queueRedraw(x, y, width, height int) {
	if x < s.queueRedrawArea[0] {
		s.queueRedrawArea[0] = x
	}
	if y < s.queueRedrawArea[1] {
		s.queueRedrawArea[1] = y
	}
	if (x + width) > s.queueRedrawArea[2] {
		s.queueRedrawArea[2] = x + width
	}
	if (y + height) > s.queueRedrawArea[3] {
		s.queueRedrawArea[3] = y + height
	}
}

func (s *Screen) posWin(x, y int) *Window {
	for _, win := range s.curWins {
		if win.pos[0] <= y && win.pos[1] <= x && (win.pos[0]+win.height+1) >= y && (win.pos[1]+win.width >= x) {
			return win
		}
	}
	return nil
}

func (s *Screen) cursorWin() *Window {
	return s.posWin(s.cursor[1], s.cursor[0])
}

func (s *Screen) fillHightlight(p *gui.QPainter, y int, col int, cols int, pos [2]int) {
	rectF := core.NewQRectF()
	screen := s.ws.screen
	if y >= len(screen.content) {
		return
	}
	line := screen.content[y]
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
						float64(start-pos[1])*s.ws.font.truewidth,
						float64((y-pos[0])*s.ws.font.lineHeight),
						float64(end-start+1)*s.ws.font.truewidth,
						float64(s.ws.font.lineHeight),
					)
					p.FillRect4(
						rectF,
						gui.NewQColor3(lastBg.R, lastBg.G, lastBg.B, int(lastBg.A*255)),
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
					float64(start-pos[1])*s.ws.font.truewidth,
					float64((y-pos[0])*s.ws.font.lineHeight),
					float64(end-start+1)*s.ws.font.truewidth,
					float64(s.ws.font.lineHeight),
				)
				p.FillRect4(
					rectF,
					gui.NewQColor3(lastBg.R, lastBg.G, lastBg.B, int(lastBg.A*255)),
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
			float64(start-pos[1])*s.ws.font.truewidth,
			float64((y-pos[0])*s.ws.font.lineHeight),
			float64(end-start+1)*s.ws.font.truewidth,
			float64(s.ws.font.lineHeight),
		)
		p.FillRect4(
			rectF,
			gui.NewQColor3(lastBg.R, lastBg.G, lastBg.B, int(lastBg.A*255)),
		)
	}
}

func (s *Screen) drawText(p *gui.QPainter, y int, col int, cols int, pos [2]int) {
	screen := s.ws.screen
	if y >= len(screen.content) {
		return
	}
	font := p.Font()
	font.SetBold(false)
	font.SetItalic(false)
	pointF := core.NewQPointF()
	line := screen.content[y]
	chars := map[Highlight][]int{}
	specialChars := []int{}
	if col > 0 {
		char := line[col-1]
		if char != nil && char.char != "" {
			if !char.normalWidth {
				col--
				cols++
			}
		}
	}
	if col+cols < s.ws.cols {
	}
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
			fg = s.ws.foreground
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
	for highlight, colorSlice := range chars {
		text := ""
		slice := colorSlice[:]
		for x := col; x < col+cols; x++ {
			if len(slice) == 0 {
				break
			}
			index := slice[0]
			if x < index {
				text += " "
				continue
			}
			if x == index {
				text += line[x].char
				slice = slice[1:]
			}
		}
		if text != "" {
			fg := highlight.foreground
			if fg != nil {
				p.SetPen2(gui.NewQColor3(fg.R, fg.G, fg.B, int(fg.A*255)))
			}
			pointF.SetX(float64(col-pos[1]) * s.ws.font.truewidth)
			pointF.SetY(float64((y-pos[0])*s.ws.font.lineHeight + s.ws.font.shift))
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
			fg = s.ws.foreground
		}
		p.SetPen2(gui.NewQColor3(fg.R, fg.G, fg.B, int(fg.A*255)))
		pointF.SetX(float64(x-pos[1]) * s.ws.font.truewidth)
		pointF.SetY(float64((y-pos[0])*s.ws.font.lineHeight + s.ws.font.shift))
		font.SetBold(char.highlight.bold)
		font.SetItalic(char.highlight.italic)
		p.DrawText(pointF, char.char)
	}
}

func (w *Window) drawBorder(p *gui.QPainter, s *Screen) {
	bg := s.ws.background
	if w.bg != nil {
		bg = w.bg
	}
	if bg == nil {
		return
	}
	height := w.height
	if w.statusline {
		height++
	}
	p.FillRect5(
		int(float64(w.pos[1]+w.width)*s.ws.font.truewidth),
		w.pos[0]*s.ws.font.lineHeight,
		int(s.ws.font.truewidth),
		height*s.ws.font.lineHeight,
		gui.NewQColor3(bg.R, bg.G, bg.B, 255),
	)
	p.FillRect5(
		int(float64(w.pos[1]+1+w.width)*s.ws.font.truewidth-1),
		w.pos[0]*s.ws.font.lineHeight,
		1,
		height*s.ws.font.lineHeight,
		gui.NewQColor3(0, 0, 0, 255),
	)

	gradient := gui.NewQLinearGradient3(
		(float64(w.width+w.pos[1])+1)*float64(s.ws.font.truewidth),
		0,
		(float64(w.width+w.pos[1])+1)*float64(s.ws.font.truewidth)-6,
		0,
	)
	gradient.SetColorAt(0, gui.NewQColor3(10, 10, 10, 125))
	gradient.SetColorAt(1, gui.NewQColor3(10, 10, 10, 0))
	brush := gui.NewQBrush10(gradient)
	p.FillRect2(
		int((float64(w.width+w.pos[1])+1)*s.ws.font.truewidth)-6,
		w.pos[0]*s.ws.font.lineHeight,
		6,
		height*s.ws.font.lineHeight,
		brush,
	)

	// p.FillRect5(
	// 	int(float64(w.pos[1])*editor.font.truewidth),
	// 	(w.pos[0]+w.height)*editor.font.lineHeight-1,
	// 	int(float64(w.width+1)*editor.font.truewidth),
	// 	1,
	// 	gui.NewQColor3(0, 0, 0, 255),
	// )

	if w.pos[0] > 0 {
		p.FillRect5(
			int(float64(w.pos[1])*s.ws.font.truewidth),
			w.pos[0]*s.ws.font.lineHeight-1,
			int(float64(w.width+1)*s.ws.font.truewidth),
			1,
			gui.NewQColor3(0, 0, 0, 255),
		)
	}
	gradient = gui.NewQLinearGradient3(
		float64(w.pos[1])*s.ws.font.truewidth,
		float64(w.pos[0]*s.ws.font.lineHeight),
		float64(w.pos[1])*s.ws.font.truewidth,
		float64(w.pos[0]*s.ws.font.lineHeight+5),
	)
	gradient.SetColorAt(0, gui.NewQColor3(10, 10, 10, 125))
	gradient.SetColorAt(1, gui.NewQColor3(10, 10, 10, 0))
	brush = gui.NewQBrush10(gradient)
	p.FillRect2(
		int(float64(w.pos[1])*s.ws.font.truewidth),
		w.pos[0]*s.ws.font.lineHeight,
		int(float64(w.width+1)*s.ws.font.truewidth),
		5,
		brush,
	)
}

func (s *Screen) isNormalWidth(char string) bool {
	if len(char) == 0 {
		return true
	}
	if char[0] <= 127 {
		return true
	}
	//return s.ws.font.fontMetrics.Width(char) == s.ws.font.truewidth
	return s.ws.font.fontMetrics.HorizontalAdvance(char, -1) == s.ws.font.truewidth
}
