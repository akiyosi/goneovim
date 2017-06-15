package gonvim

import (
	"fmt"
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
	win    nvim.Window
	width  int
	height int
	pos    [2]int
	tab    nvim.Tabpage
	hl     string
	bg     *RGBA
}

// Screen is the main editor area
type Screen struct {
	width            int
	height           int
	widget           *widgets.QWidget
	wins             map[nvim.Window]*Window
	cursor           [2]int
	lastCursor       [2]int
	devicePixelRatio float64
	content          [][]*Char
	scrollRegion     []int
	curtab           nvim.Tabpage
	highlight        Highlight
	curWins          map[nvim.Window]*Window
	queueRedrawArea  [4]int
	specialKeys      map[core.Qt__Key]string
	paintMutex       sync.Mutex
	redrawMutex      sync.Mutex
	pixmap           *gui.QPixmap
	pixmapPainter    *gui.QPainter
	item             *widgets.QGraphicsPixmapItem

	controlModifier core.Qt__KeyboardModifier
	cmdModifier     core.Qt__KeyboardModifier
	shiftModifier   core.Qt__KeyboardModifier
	altModifier     core.Qt__KeyboardModifier
	metaModifier    core.Qt__KeyboardModifier
	keyControl      core.Qt__Key
	keyCmd          core.Qt__Key
	keyAlt          core.Qt__Key
}

func initScreenNew(devicePixelRatio float64) *Screen {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	// widget.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)

	// view := widgets.NewQGraphicsView(nil)
	// scene := widgets.NewQGraphicsScene(nil)
	// view.SetScene(scene)
	// view.SetFixedSize2(200, 200)
	// item := scene.AddPixmap(gui.NewQPixmap())
	// view.SetParent(widget)

	screen := &Screen{
		widget:           widget,
		cursor:           [2]int{0, 0},
		lastCursor:       [2]int{0, 0},
		scrollRegion:     []int{0, 0, 0, 0},
		devicePixelRatio: devicePixelRatio,
		// pixmap:           gui.NewQPixmap3(100, 100),
		// item:             item,
	}
	widget.ConnectPaintEvent(screen.paint)
	widget.ConnectCustomEvent(func(event *core.QEvent) {
		screen.redraw()
	})
	screen.initSpecialKeys()
	widget.ConnectResizeEvent(func(event *gui.QResizeEvent) {
		if editor == nil {
			return
		}
		screen.updateSize()
	})
	// go func() {
	// 	for range time.Tick(20 * time.Millisecond) {
	// 		widget.Update()
	// 	}
	// }()
	return screen
}

func (s *Screen) redrawRequest() {
	s.widget.CustomEvent(core.NewQEvent(core.QEvent__UpdateRequest))
}

func (s *Screen) paint(vqp *gui.QPaintEvent) {
	if editor == nil {
		return
	}
	s.paintMutex.Lock()
	defer s.paintMutex.Unlock()

	rect := vqp.M_rect()
	// font := editor.font
	// top := rect.Y()
	// left := rect.X()
	// width := rect.Width()
	// height := rect.Height()
	// right := left + width
	// bottom := top + height
	// row := int(float64(top) / float64(font.lineHeight))
	// col := int(float64(left) / font.truewidth)
	// rows := int(math.Ceil(float64(bottom)/float64(font.lineHeight))) - row
	// cols := int(math.Ceil(float64(right)/font.truewidth)) - col

	p := gui.NewQPainter2(s.widget)
	source := core.NewQRect4(
		int(float64(rect.X())*s.devicePixelRatio),
		int(float64(rect.Y())*s.devicePixelRatio),
		int(float64(rect.Width())*s.devicePixelRatio),
		int(float64(rect.Height())*s.devicePixelRatio),
	)
	p.DrawPixmap2(rect, s.pixmap, source)
	p.DestroyQPainter()

	// p.SetFont(editor.font.fontNew)

	// for y := row; y < row+rows; y++ {
	// 	if y >= editor.rows {
	// 		continue
	// 	}
	// 	fillHightlight(p, y, col, cols, [2]int{0, 0})
	// 	drawText(p, y, col, cols, [2]int{0, 0})
	// }

	// s.drawBorder(p, row, col, rows, cols)
	// p.DestroyQPainter()
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
	if text == "" {
		if mod&s.controlModifier > 0 || mod&s.cmdModifier > 0 {
			if int(s.keyControl) == key || int(s.keyCmd) == key || int(s.keyAlt) == key {
				return ""
			}
			c = string(key)
		} else {
			return ""
		}
	} else {
		c = string(text[0])
	}

	prefix := s.modPrefix(mod)
	if prefix != "" {
		return fmt.Sprintf("<%s%s>", prefix, c)
	}

	return c
}

func (s *Screen) modPrefix(mod core.Qt__KeyboardModifier) string {
	prefix := ""
	if mod&s.cmdModifier > 0 {
		prefix += "D-"
	}

	if mod&s.controlModifier > 0 {
		prefix += "C-"
	}

	if mod&s.altModifier > 0 {
		prefix += "A-"
	}

	return prefix
}

func (s *Screen) getWindows() map[nvim.Window]*Window {
	wins := map[nvim.Window]*Window{}
	neovim := editor.nvim
	curtab, _ := neovim.CurrentTabpage()
	s.curtab = curtab
	nwins, _ := neovim.Windows()
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
	err := b.Execute()
	if err != nil {
		return nil
	}
	return wins
}

// Draw the screen
// func (s *Screen) Draw(a *ui.Area, dp *ui.AreaDrawParams) {
// 	return
// if editor == nil {
// 	return
// }
// font := editor.font
// row := int(math.Ceil(dp.ClipY / float64(font.lineHeight)))
// col := int(math.Ceil(dp.ClipX / font.truewidth))
// rows := int(math.Ceil(dp.ClipHeight / float64(font.lineHeight)))
// cols := int(math.Ceil(dp.ClipWidth / font.truewidth))

// p := ui.NewPath(ui.Winding)
// p.AddRectangle(dp.ClipX, dp.ClipY, dp.ClipWidth, dp.ClipHeight)
// p.End()

// bg := editor.Background

// dp.Context.Fill(p, &ui.Brush{
// 	Type: ui.Solid,
// 	R:    bg.R,
// 	G:    bg.G,
// 	B:    bg.B,
// 	A:    1,
// })
// p.Free()

// for y := row; y < row+rows; y++ {
// 	if y >= editor.rows {
// 		continue
// 	}
// 	fillHightlight(dp, y, col, cols, [2]int{0, 0})
// 	drawText(dp, y, col, cols, [2]int{0, 0})
// }

// s.drawBorder(dp, row, col, rows, cols)
// }

func (s *Screen) drawBorder(p *gui.QPainter, row, col, rows, cols int) {
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

func (s *Screen) subscribe() {
	editor.nvim.RegisterHandler("winsupdate", func(updates ...interface{}) {
		s.redrawWindows()
	})
	editor.nvim.Subscribe("winsupdate")
	editor.nvim.Command(`autocmd VimResized,WinEnter,WinLeave * call rpcnotify(0, "winsupdate")`)
}

func (s *Screen) redrawWindows() {
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
	err := b.Execute()
	if err != nil {
		return
	}
	s.curWins = wins
	for _, win := range s.curWins {
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
	s.pixmap.Fill(editor.Background.QColor())
	css := fmt.Sprintf("background-color: %s;", editor.Background.String())
	s.widget.SetStyleSheet(css)
}

func (s *Screen) size() (int, int) {
	geo := s.widget.Geometry()
	return geo.Width(), geo.Height()
}

func (s *Screen) updateSize() {
	width, height := s.size()
	s.width = width
	s.height = height
	// pixmap := gui.NewQPixmap3(width, height)
	// if s.devicePixelRatio != 1 {
	// 	pixmap = pixmap.Scaled2(int(s.devicePixelRatio*float64(width)), int(s.devicePixelRatio*float64(height)), core.Qt__IgnoreAspectRatio, core.Qt__SmoothTransformation)
	// 	pixmap.SetDevicePixelRatio(s.devicePixelRatio)
	// }
	oldpixmap := s.pixmap
	s.pixmap = gui.NewQPixmap3(int(s.devicePixelRatio*float64(width)), int(s.devicePixelRatio*float64(height)))
	s.pixmap.SetDevicePixelRatio(s.devicePixelRatio)
	go func() {
		time.Sleep(time.Second)
		oldpixmap.DestroyQPixmap()
	}()
	// s.pixmap = pixmap
	editor.nvimResize()
	editor.finder.resize()
}

func (s *Screen) resize(args []interface{}) {
	s.cursor[0] = 0
	s.cursor[1] = 0
	s.content = make([][]*Char, editor.rows)
	for i := 0; i < editor.rows; i++ {
		s.content[i] = make([]*Char, editor.cols)
	}
	s.pixmap.Fill(editor.Background.QColor())
	s.queueRedrawAll()
}

func (s *Screen) clear(args []interface{}) {
	s.cursor[0] = 0
	s.cursor[1] = 0
	s.content = make([][]*Char, editor.rows)
	for i := 0; i < editor.rows; i++ {
		s.content[i] = make([]*Char, editor.cols)
	}
	s.pixmap.Fill(editor.Background.QColor())
	s.queueRedrawAll()
}

func (s *Screen) eolClear(args []interface{}) {
	row := s.cursor[0]
	col := s.cursor[1]
	line := s.content[row]
	numChars := 0
	for x := col; x < len(line); x++ {
		line[x] = nil
		numChars++
	}
	rect := core.NewQRectF4(
		float64(col)*editor.font.truewidth,
		float64(row*editor.font.lineHeight),
		float64(numChars)*editor.font.truewidth,
		float64(editor.font.lineHeight),
	)
	s.pixmapPainter.FillRect4(rect, editor.Background.QColor())
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
	text := ""
	if x > 0 {
		char := line[x-1]
		if char != nil && char.char != "" {
			if !isNormalWidth(char.char) {
				x++
				col++
				args = args[1:]
			}
		}
	}
	lastChar := ""
	for _, arg := range args {
		chars := arg.([]interface{})
		for _, c := range chars {
			char := line[col]
			if char == nil {
				char = &Char{}
				line[col] = char
			}
			char.char = c.(string)
			text += char.char
			lastChar = char.char
			char.highlight = s.highlight
			col++
			numChars++
		}
	}
	if lastChar != "" && !isNormalWidth(lastChar) {
		numChars++
	}
	point := core.NewQPointF3(
		float64(x)*editor.font.truewidth,
		float64(y*editor.font.lineHeight+editor.font.shift),
	)
	rect := core.NewQRectF4(
		float64(x)*editor.font.truewidth,
		float64(y*editor.font.lineHeight),
		float64(numChars)*editor.font.truewidth,
		float64(editor.font.lineHeight),
	)
	bg := editor.Background
	fg := editor.Foreground
	if s.highlight.background != nil {
		bg = s.highlight.background
	}
	if s.highlight.foreground != nil {
		fg = s.highlight.foreground
	}
	s.pixmapPainter.FillRect4(rect, bg.QColor())
	s.pixmapPainter.SetPen2(fg.QColor())
	s.pixmapPainter.DrawText(point, text)
	point.DestroyQPointF()
	s.cursor[1] = col
	// we redraw one character more than the chars put for double width characters
	s.queueRedraw(x, y, numChars+1, 1)
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

	s.pixmap.Scroll(
		0,
		-int(float64(count*editor.font.lineHeight)*(s.devicePixelRatio)),
		int(float64(left)*editor.font.truewidth*s.devicePixelRatio),
		int(float64(top*editor.font.lineHeight)*s.devicePixelRatio),
		int(float64(right-left+1)*editor.font.truewidth*s.devicePixelRatio),
		int(float64((bot-top+1)*editor.font.lineHeight)*s.devicePixelRatio),
		nil,
	)

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
		rect := core.NewQRectF4(
			float64(left)*editor.font.truewidth,
			float64((bot-count+1)*editor.font.lineHeight),
			float64(right-left+1)*editor.font.truewidth,
			float64(count*editor.font.lineHeight),
		)
		s.pixmapPainter.FillRect4(
			rect,
			editor.Background.QColor(),
		)
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
		rect := core.NewQRectF4(
			float64(left)*editor.font.truewidth,
			float64(top*editor.font.lineHeight),
			float64(right-left+1)*editor.font.truewidth,
			float64(-count*editor.font.lineHeight),
		)
		s.pixmapPainter.FillRect4(
			rect,
			editor.Background.QColor(),
		)
		if bot < editor.rows-1 {
			s.queueRedraw(left, bot+1, (right - left), -count)
		}
	}
}

func (s *Screen) update() {
	s.widget.CustomEvent(core.NewQEvent(core.QEvent__UpdateRequest))
}

func (s *Screen) redraw() {
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
	s.queueRedrawArea[0] = 0
	s.queueRedrawArea[1] = 0
	s.queueRedrawArea[2] = 0
	s.queueRedrawArea[3] = 0
}

func (s *Screen) queueRedrawAll() {
	s.queueRedrawArea = [4]int{0, 0, editor.cols, editor.rows}
}

func (s *Screen) queueRedraw(x, y, width, height int) {
	// s.widget.Update2(
	// 	int(float64(x)*editor.font.truewidth),
	// 	y*editor.font.lineHeight,
	// 	int(math.Ceil(float64(width)*editor.font.truewidth)),
	// 	height*editor.font.lineHeight,
	// )
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
	defer rectF.DestroyQRectF()
	screen := editor.screen
	if y >= len(screen.content) {
		return
	}
	line := screen.content[y]
	start := -1
	end := -1
	var lastBg *RGBA
	var bg *RGBA
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
	defer pointF.DestroyQPointF()
	line := screen.content[y]
	chars := map[*RGBA][]int{}
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
		if !isNormalWidth(char.char) {
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
		// fg := editor.Foreground
		// if char.highlight.foreground != nil {
		// 	fg = char.highlight.foreground
		// }
		// textLayout := ui.NewTextLayout(char.char, editor.font.font, -1)
		// textLayout.SetColor(0, 1, fg.R, fg.G, fg.B, fg.A)
		// dp.Context.Text(float64(x-pos[1])*editor.font.truewidth, float64((y-pos[0])*editor.font.lineHeight+shift), textLayout)
		// textLayout.Free()
	}

	// text := ""
	// var specialChars []int
	// start := -1
	// end := col
	// for x := col; x < col+cols; x++ {
	// 	if x >= len(line) {
	// 		continue
	// 	}
	// 	char := line[x]
	// 	if char == nil {
	// 		text += " "
	// 		continue
	// 	}
	// 	if char.char == " " {
	// 		text += " "
	// 		continue
	// 	}
	// 	if char.char == "" {
	// 		text += " "
	// 		continue
	// 	}
	// 	if !isNormalWidth(char.char) {
	// 		text += " "
	// 		specialChars = append(specialChars, x)
	// 		continue
	// 	}
	// 	text += char.char
	// 	if start == -1 {
	// 		start = x
	// 	}
	// 	end = x
	// }
	// if start == -1 {
	// 	return
	// }
	// text = strings.TrimSpace(text)
	// shift := editor.font.shift

	// for x := start; x <= end; x++ {
	// 	char := line[x]
	// 	if char == nil || char.char == " " {
	// 		continue
	// 	}
	// 	// fg := editor.Foreground
	// 	// if char.highlight.foreground != nil {
	// 	// 	fg = char.highlight.foreground
	// 	// }
	// 	// textLayout.SetColor(x-start, x-start+1, fg.R, fg.G, fg.B, fg.A)
	// }
	// p.DrawText3(
	// 	int(float64(start-pos[1])*editor.font.truewidth),
	// 	(y-pos[0])*editor.font.lineHeight+shift,
	// 	text,
	// )
	// // dp.Context.Text(float64(start-pos[1])*editor.font.truewidth, float64((y-pos[0])*editor.font.lineHeight+shift), textLayout)
	// // textLayout.Free()

	// for _, x := range specialChars {
	// 	char := line[x]
	// 	if char == nil || char.char == " " {
	// 		continue
	// 	}
	// 	// fg := editor.Foreground
	// 	// if char.highlight.foreground != nil {
	// 	// 	fg = char.highlight.foreground
	// 	// }
	// 	// textLayout := ui.NewTextLayout(char.char, editor.font.font, -1)
	// 	// textLayout.SetColor(0, 1, fg.R, fg.G, fg.B, fg.A)
	// 	// dp.Context.Text(float64(x-pos[1])*editor.font.truewidth, float64((y-pos[0])*editor.font.lineHeight+shift), textLayout)
	// 	// textLayout.Free()
	// }
}

func (w *Window) drawBorder(p *gui.QPainter) {
	bg := editor.Background
	if w.bg != nil {
		bg = w.bg
	}
	p.FillRect5(
		int(float64(w.pos[1]+w.width)*editor.font.truewidth),
		w.pos[0]*editor.font.lineHeight,
		int(editor.font.truewidth),
		w.height*editor.font.lineHeight,
		gui.NewQColor3(bg.R, bg.G, bg.B, 255),
	)
	p.FillRect5(
		int(float64(w.pos[1]+1+w.width)*editor.font.truewidth-1),
		w.pos[0]*editor.font.lineHeight,
		1,
		w.height*editor.font.lineHeight,
		gui.NewQColor3(0, 0, 0, 255),
	)

	gradient := gui.NewQLinearGradient3(
		(float64(w.width+w.pos[1])+1)*float64(editor.font.truewidth),
		0,
		(float64(w.width+w.pos[1]))*float64(editor.font.truewidth),
		0,
	)
	gradient.SetColorAt(0, gui.NewQColor3(10, 10, 10, 125))
	gradient.SetColorAt(1, gui.NewQColor3(10, 10, 10, 0))
	brush := gui.NewQBrush10(gradient)
	p.FillRect2(
		int((float64(w.width+w.pos[1]))*editor.font.truewidth),
		w.pos[0]*editor.font.lineHeight,
		int(editor.font.truewidth),
		w.height*editor.font.lineHeight,
		brush,
	)
	brush.DestroyQBrush()
	gradient.DestroyQGradient()

}
