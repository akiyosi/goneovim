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
}

// Screen is the main editor area
type Screen struct {
	bg              *RGBA
	width           int
	height          int
	widget          *widgets.QWidget
	wins            map[nvim.Window]*Window
	cursor          [2]int
	lastCursor      [2]int
	content         [][]*Char
	scrollRegion    []int
	curtab          nvim.Tabpage
	cmdheight       int
	highlight       Highlight
	curWins         map[nvim.Window]*Window
	queueRedrawArea [4]int
	specialKeys     map[core.Qt__Key]string
	paintMutex      sync.Mutex
	redrawMutex     sync.Mutex
	drawSplit       bool

	controlModifier core.Qt__KeyboardModifier
	cmdModifier     core.Qt__KeyboardModifier
	shiftModifier   core.Qt__KeyboardModifier
	altModifier     core.Qt__KeyboardModifier
	metaModifier    core.Qt__KeyboardModifier
	keyControl      core.Qt__Key
	keyCmd          core.Qt__Key
	keyAlt          core.Qt__Key
	keyShift        core.Qt__Key
}

func initScreenNew(devicePixelRatio float64) *Screen {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)

	screen := &Screen{
		widget:       widget,
		cursor:       [2]int{0, 0},
		lastCursor:   [2]int{0, 0},
		scrollRegion: []int{0, 0, 0, 0},
	}
	widget.ConnectPaintEvent(screen.paint)
	screen.initSpecialKeys()
	widget.ConnectResizeEvent(func(event *gui.QResizeEvent) {
		if editor == nil {
			return
		}
		screen.updateSize()
	})
	widget.ConnectMousePressEvent(screen.mouseEvent)
	widget.ConnectMouseReleaseEvent(screen.mouseEvent)
	widget.ConnectMouseMoveEvent(screen.mouseEvent)
	return screen
}

func (s *Screen) paint(vqp *gui.QPaintEvent) {
	if editor == nil {
		return
	}
	s.paintMutex.Lock()
	defer s.paintMutex.Unlock()

	rect := vqp.M_rect()
	font := editor.font
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
	if editor.Background != nil {
		p.FillRect5(
			left,
			top,
			width,
			height,
			editor.Background.QColor(),
		)
	}

	p.SetFont(editor.font.fontNew)

	for y := row; y < row+rows; y++ {
		if y >= editor.rows {
			continue
		}
		fillHightlight(p, y, col, cols, [2]int{0, 0})
		drawText(p, y, col, cols, [2]int{0, 0})
	}

	s.drawBorder(p, row, col, rows, cols)
	p.DestroyQPainter()
}

func (s *Screen) initSpecialKeys() {
	s.specialKeys = map[core.Qt__Key]string{}
	s.specialKeys[core.Qt__Key_Up] = "Up"
	s.specialKeys[core.Qt__Key_Down] = "Down"
	s.specialKeys[core.Qt__Key_Left] = "Left"
	s.specialKeys[core.Qt__Key_Right] = "Right"

	s.specialKeys[core.Qt__Key_F1] = "F1"
	s.specialKeys[core.Qt__Key_F2] = "F2"
	s.specialKeys[core.Qt__Key_F3] = "F3"
	s.specialKeys[core.Qt__Key_F4] = "F4"
	s.specialKeys[core.Qt__Key_F5] = "F5"
	s.specialKeys[core.Qt__Key_F6] = "F6"
	s.specialKeys[core.Qt__Key_F7] = "F7"
	s.specialKeys[core.Qt__Key_F8] = "F8"
	s.specialKeys[core.Qt__Key_F9] = "F9"
	s.specialKeys[core.Qt__Key_F10] = "F10"
	s.specialKeys[core.Qt__Key_F11] = "F11"
	s.specialKeys[core.Qt__Key_F12] = "F12"

	s.specialKeys[core.Qt__Key_Backspace] = "BS"
	s.specialKeys[core.Qt__Key_Delete] = "Del"
	s.specialKeys[core.Qt__Key_Insert] = "Insert"
	s.specialKeys[core.Qt__Key_Home] = "Home"
	s.specialKeys[core.Qt__Key_End] = "End"
	s.specialKeys[core.Qt__Key_PageUp] = "PageUp"
	s.specialKeys[core.Qt__Key_PageDown] = "PageDown"

	s.specialKeys[core.Qt__Key_Return] = "Enter"
	s.specialKeys[core.Qt__Key_Enter] = "Enter"
	s.specialKeys[core.Qt__Key_Tab] = "Tab"
	s.specialKeys[core.Qt__Key_Backtab] = "Tab"
	s.specialKeys[core.Qt__Key_Escape] = "Esc"

	s.specialKeys[core.Qt__Key_Backslash] = "Bslash"
	s.specialKeys[core.Qt__Key_Space] = "Space"

	goos := runtime.GOOS
	s.shiftModifier = core.Qt__ShiftModifier
	s.altModifier = core.Qt__AltModifier
	s.keyAlt = core.Qt__Key_Alt
	s.keyShift = core.Qt__Key_Shift
	if goos == "darwin" {
		s.controlModifier = core.Qt__MetaModifier
		s.cmdModifier = core.Qt__ControlModifier
		s.metaModifier = core.Qt__AltModifier
		s.keyControl = core.Qt__Key_Meta
		s.keyCmd = core.Qt__Key_Control
	} else {
		s.controlModifier = core.Qt__ControlModifier
		s.metaModifier = core.Qt__MetaModifier
		s.keyControl = core.Qt__Key_Control
		if goos == "linux" {
			s.cmdModifier = core.Qt__MetaModifier
			s.keyCmd = core.Qt__Key_Meta
		}
	}
}

func (s *Screen) mouseEvent(event *gui.QMouseEvent) {
	inp := s.convertMouse(event)
	if inp == "" {
		return
	}
	editor.nvim.Input(inp)
}

func (s *Screen) convertMouse(event *gui.QMouseEvent) string {
	font := editor.font
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

	return fmt.Sprintf("<%s%s%s><%d,%d>", s.modPrefix(mod), buttonName, evType, pos[0], pos[1])
}

func (s *Screen) keyPress(event *gui.QKeyEvent) {
	if editor == nil {
		return
	}
	input := s.convertKey(event.Text(), event.Key(), event.Modifiers())
	if input != "" {
		editor.nvim.Input(input)
	}
}

func (s *Screen) convertKey(text string, key int, mod core.Qt__KeyboardModifier) string {
	if mod&core.Qt__KeypadModifier > 0 {
		switch core.Qt__Key(key) {
		case core.Qt__Key_Home:
			return fmt.Sprintf("<%sHome>", s.modPrefix(mod))
		case core.Qt__Key_End:
			return fmt.Sprintf("<%sEnd>", s.modPrefix(mod))
		case core.Qt__Key_PageUp:
			return fmt.Sprintf("<%sPageUp>", s.modPrefix(mod))
		case core.Qt__Key_PageDown:
			return fmt.Sprintf("<%sPageDown>", s.modPrefix(mod))
		case core.Qt__Key_Plus:
			return fmt.Sprintf("<%sPlus>", s.modPrefix(mod))
		case core.Qt__Key_Minus:
			return fmt.Sprintf("<%sMinus>", s.modPrefix(mod))
		case core.Qt__Key_multiply:
			return fmt.Sprintf("<%sMultiply>", s.modPrefix(mod))
		case core.Qt__Key_division:
			return fmt.Sprintf("<%sDivide>", s.modPrefix(mod))
		case core.Qt__Key_Enter:
			return fmt.Sprintf("<%sEnter>", s.modPrefix(mod))
		case core.Qt__Key_Period:
			return fmt.Sprintf("<%sPoint>", s.modPrefix(mod))
		case core.Qt__Key_0:
			return fmt.Sprintf("<%s0>", s.modPrefix(mod))
		case core.Qt__Key_1:
			return fmt.Sprintf("<%s1>", s.modPrefix(mod))
		case core.Qt__Key_2:
			return fmt.Sprintf("<%s2>", s.modPrefix(mod))
		case core.Qt__Key_3:
			return fmt.Sprintf("<%s3>", s.modPrefix(mod))
		case core.Qt__Key_4:
			return fmt.Sprintf("<%s4>", s.modPrefix(mod))
		case core.Qt__Key_5:
			return fmt.Sprintf("<%s5>", s.modPrefix(mod))
		case core.Qt__Key_6:
			return fmt.Sprintf("<%s6>", s.modPrefix(mod))
		case core.Qt__Key_7:
			return fmt.Sprintf("<%s7>", s.modPrefix(mod))
		case core.Qt__Key_8:
			return fmt.Sprintf("<%s8>", s.modPrefix(mod))
		case core.Qt__Key_9:
			return fmt.Sprintf("<%s9>", s.modPrefix(mod))
		}
	}

	if text == "<" {
		return "<lt>"
	}

	specialKey, ok := s.specialKeys[core.Qt__Key(key)]
	if ok {
		return fmt.Sprintf("<%s%s>", s.modPrefix(mod), specialKey)
	}

	if text == "\\" {
		return fmt.Sprintf("<%s%s>", s.modPrefix(mod), "Bslash")
	}

	c := ""
	if mod&s.controlModifier > 0 || mod&s.cmdModifier > 0 {
		if int(s.keyControl) == key || int(s.keyCmd) == key || int(s.keyAlt) == key || int(s.keyShift) == key {
			return ""
		}
		c = string(key)
		if !(mod&s.shiftModifier > 0) {
			c = strings.ToLower(c)
		}
	} else {
		c = text
	}

	if c == "" {
		return ""
	}

	char := core.NewQChar10(c)
	if char.Unicode() < 0x100 && !char.IsNumber() && char.IsPrint() {
		mod &= ^s.shiftModifier
	}

	prefix := s.modPrefix(mod)
	if prefix != "" {
		return fmt.Sprintf("<%s%s>", prefix, c)
	}

	return c
}

func (s *Screen) modPrefix(mod core.Qt__KeyboardModifier) string {
	prefix := ""
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		if mod&s.cmdModifier > 0 {
			prefix += "D-"
		}
	}

	if mod&s.controlModifier > 0 {
		prefix += "C-"
	}

	if mod&s.shiftModifier > 0 {
		prefix += "S-"
	}

	if mod&s.altModifier > 0 {
		prefix += "A-"
	}

	return prefix
}

func (s *Screen) drawBorder(p *gui.QPainter, row, col, rows, cols int) {
	done := make(chan struct{})
	go func() {
		s.getWindows()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
	}
	for _, win := range s.curWins {
		if win.pos[0]+win.height < row && (win.pos[1]+win.width+1) < col {
			continue
		}
		if win.pos[0] > (row+rows) && (win.pos[1]+win.width) > (col+cols) {
			continue
		}

		win.drawBorder(p)
	}
}

func (s *Screen) getWindows() {
	wins := map[nvim.Window]*Window{}
	neovim := editor.nvim
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
		if win.height+win.pos[0] < editor.rows-s.cmdheight {
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

func (s *Screen) updateBg(args []interface{}) {
	color := reflectToInt(args[0])
	if color == -1 {
		editor.Background = newRGBA(0, 0, 0, 1)
	} else {
		bg := calcColor(reflectToInt(args[0]))
		editor.Background = bg
	}
}

func (s *Screen) size() (int, int) {
	geo := s.widget.Geometry()
	return geo.Width(), geo.Height()
}

func (s *Screen) updateSize() {
	s.width, s.height = s.size()
	editor.nvimResize()
	editor.palette.resize()
	editor.message.resize()
}

func (s *Screen) resize(args []interface{}) {
	s.cursor[0] = 0
	s.cursor[1] = 0
	s.content = make([][]*Char, editor.rows)
	for i := 0; i < editor.rows; i++ {
		s.content[i] = make([]*Char, editor.cols)
	}
	s.queueRedrawAll()
}

func (s *Screen) clear(args []interface{}) {
	s.cursor[0] = 0
	s.cursor[1] = 0
	s.content = make([][]*Char, editor.rows)
	for i := 0; i < editor.rows; i++ {
		s.content[i] = make([]*Char, editor.cols)
	}
	s.queueRedrawAll()
}

func (s *Screen) eolClear(args []interface{}) {
	row := s.cursor[0]
	col := s.cursor[1]
	if row >= editor.rows {
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
	if row >= editor.rows {
		return
	}
	line := s.content[row]
	oldFirstNormal := true
	char := line[x]
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
			char.normalWidth = isNormalWidth(char.char)
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
		_, ok := hl["reverse"]
		if ok {
			highlight := Highlight{}
			highlight.foreground = s.highlight.background
			highlight.background = s.highlight.foreground
			s.highlight = highlight
			continue
		}

		highlight := Highlight{}
		fg, ok := hl["foreground"]
		if ok {
			rgba := calcColor(reflectToInt(fg))
			highlight.foreground = rgba
		} else {
			highlight.foreground = editor.Foreground
		}

		bg, ok := hl["background"]
		if ok {
			rgba := calcColor(reflectToInt(bg))
			highlight.background = rgba
		} else {
			highlight.background = editor.Background
		}
		s.highlight = highlight
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
		bot = editor.rows - 1
		left = 0
		right = editor.cols - 1
	}

	s.queueRedraw(left, top, (right - left + 1), (bot - top + 1))

	if count > 0 {
		for row := top; row <= bot-count; row++ {
			for col := left; col <= right; col++ {
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
				s.content[row][col] = s.content[row+count][col]
			}
		}
		for row := top; row < top-count; row++ {
			for col := left; col <= right; col++ {
				s.content[row][col] = nil
			}
		}
		s.queueRedraw(left, top, (right - left), -count)
		if bot < editor.rows-1 {
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
			int(float64(x)*editor.font.truewidth),
			y*editor.font.lineHeight,
			int(float64(width)*editor.font.truewidth),
			height*editor.font.lineHeight,
		)
	}
	s.queueRedrawArea[0] = editor.cols
	s.queueRedrawArea[1] = editor.rows
	s.queueRedrawArea[2] = 0
	s.queueRedrawArea[3] = 0
}

func (s *Screen) queueRedrawAll() {
	s.queueRedrawArea = [4]int{0, 0, editor.cols, editor.rows}
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

func fillHightlight(p *gui.QPainter, y int, col int, cols int, pos [2]int) {
	rectF := core.NewQRectF()
	screen := editor.screen
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
						float64(start-pos[1])*editor.font.truewidth,
						float64((y-pos[0])*editor.font.lineHeight),
						float64(end-start+1)*editor.font.truewidth,
						float64(editor.font.lineHeight),
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
					float64(start-pos[1])*editor.font.truewidth,
					float64((y-pos[0])*editor.font.lineHeight),
					float64(end-start+1)*editor.font.truewidth,
					float64(editor.font.lineHeight),
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
			float64(start-pos[1])*editor.font.truewidth,
			float64((y-pos[0])*editor.font.lineHeight),
			float64(end-start+1)*editor.font.truewidth,
			float64(editor.font.lineHeight),
		)
		p.FillRect4(
			rectF,
			gui.NewQColor3(lastBg.R, lastBg.G, lastBg.B, int(lastBg.A*255)),
		)
	}
}

func drawText(p *gui.QPainter, y int, col int, cols int, pos [2]int) {
	screen := editor.screen
	if y >= len(screen.content) {
		return
	}
	pointF := core.NewQPointF()
	line := screen.content[y]
	chars := map[*RGBA][]int{}
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
	if col+cols < editor.cols {
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
		fg := char.highlight.foreground
		if fg == nil {
			fg = editor.Foreground
		}
		colorSlice, ok := chars[fg]
		if !ok {
			colorSlice = []int{}
		}
		colorSlice = append(colorSlice, x)
		chars[fg] = colorSlice
	}

	for fg, colorSlice := range chars {
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
			p.SetPen2(gui.NewQColor3(fg.R, fg.G, fg.B, int(fg.A*255)))
			pointF.SetX(float64(col-pos[1]) * editor.font.truewidth)
			pointF.SetY(float64((y-pos[0])*editor.font.lineHeight + editor.font.shift))
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
			fg = editor.Foreground
		}
		p.SetPen2(gui.NewQColor3(fg.R, fg.G, fg.B, int(fg.A*255)))
		pointF.SetX(float64(x-pos[1]) * editor.font.truewidth)
		pointF.SetY(float64((y-pos[0])*editor.font.lineHeight + editor.font.shift))
		p.DrawText(pointF, char.char)
	}
}

func (w *Window) drawBorder(p *gui.QPainter) {
	bg := editor.Background
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
		int(float64(w.pos[1]+w.width)*editor.font.truewidth),
		w.pos[0]*editor.font.lineHeight,
		int(editor.font.truewidth),
		height*editor.font.lineHeight,
		gui.NewQColor3(bg.R, bg.G, bg.B, 255),
	)
	p.FillRect5(
		int(float64(w.pos[1]+1+w.width)*editor.font.truewidth-1),
		w.pos[0]*editor.font.lineHeight,
		1,
		height*editor.font.lineHeight,
		gui.NewQColor3(0, 0, 0, 255),
	)

	gradient := gui.NewQLinearGradient3(
		(float64(w.width+w.pos[1])+1)*float64(editor.font.truewidth),
		0,
		(float64(w.width+w.pos[1])+1)*float64(editor.font.truewidth)-6,
		0,
	)
	gradient.SetColorAt(0, gui.NewQColor3(10, 10, 10, 125))
	gradient.SetColorAt(1, gui.NewQColor3(10, 10, 10, 0))
	brush := gui.NewQBrush10(gradient)
	p.FillRect2(
		int((float64(w.width+w.pos[1])+1)*editor.font.truewidth)-6,
		w.pos[0]*editor.font.lineHeight,
		6,
		height*editor.font.lineHeight,
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
			int(float64(w.pos[1])*editor.font.truewidth),
			w.pos[0]*editor.font.lineHeight-1,
			int(float64(w.width+1)*editor.font.truewidth),
			1,
			gui.NewQColor3(0, 0, 0, 255),
		)
	}
	gradient = gui.NewQLinearGradient3(
		float64(w.pos[1])*editor.font.truewidth,
		float64(w.pos[0]*editor.font.lineHeight),
		float64(w.pos[1])*editor.font.truewidth,
		float64(w.pos[0]*editor.font.lineHeight+5),
	)
	gradient.SetColorAt(0, gui.NewQColor3(10, 10, 10, 125))
	gradient.SetColorAt(1, gui.NewQColor3(10, 10, 10, 0))
	brush = gui.NewQBrush10(gradient)
	p.FillRect2(
		int(float64(w.pos[1])*editor.font.truewidth),
		w.pos[0]*editor.font.lineHeight,
		int(float64(w.width+1)*editor.font.truewidth),
		5,
		brush,
	)
}
