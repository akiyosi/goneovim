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

type miniMapSignal struct {
	core.QObject
	_ func() `signal:"stopSignal"`
	_ func() `signal:"redrawSignal"`
	_ func() `signal:"guiSignal"`
}

// MiniMap is
type MiniMap struct {
	ws        *Workspace
	widget    *widgets.QWidget
	curRegion *widgets.QWidget
	pos       int
	width     int
	height    int

	font             *Font
	content          [][]*Char
	scrollRegion     []int
	scrollDust       [2]int
	scrollDustDeltaY int
	queueRedrawArea  [4]int
	paintMutex       sync.Mutex
	redrawMutex      sync.Mutex
	bg               *RGBA
	curtab           nvim.Tabpage
	cursor           [2]int
	cmdheight        int
	curWins          map[nvim.Window]*Window
	highlight        Highlight

	sync          sync.Mutex
	signal        *miniMapSignal
	redrawUpdates chan [][]interface{}
	guiUpdates    chan []interface{}
	stopOnce      sync.Once
	stop          chan struct{}

	nvim       *nvim.Nvim
	uiAttached bool
	rows       int
	cols       int

	foreground *RGBA
	background *RGBA
}

func newMiniMap() *MiniMap {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	widget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")
	widget.SetFixedWidth(80)

	//curRegion := widgets.NewQWidget(widget, 0)

	m := &MiniMap{
		widget: widget,
		// curRegion: curRegion,
		scrollRegion:  []int{0, 0, 0, 0},
		stop:          make(chan struct{}),
		signal:        NewMiniMapSignal(nil),
		redrawUpdates: make(chan [][]interface{}, 1000),
		guiUpdates:    make(chan []interface{}, 1000),
	}
	m.signal.ConnectRedrawSignal(func() {
		updates := <-m.redrawUpdates
		m.handleRedraw(updates)
	})
	m.signal.ConnectGuiSignal(func() {
		updates := <-m.guiUpdates
		m.handleRPCGui(updates)
	})
	m.signal.ConnectStopSignal(func() {
	})
	widget.ConnectPaintEvent(m.paint)
	widget.ConnectResizeEvent(func(event *gui.QResizeEvent) {
		m.updateSize()
	})
	fontFamily := ""
	switch runtime.GOOS {
	case "windows":
		fontFamily = "Consolas"
	case "darwin":
		fontFamily = "Courier New"
	default:
		fontFamily = "Monospace"
	}
	m.font = initFontNew(fontFamily, 4, 6)

	neovim, err := nvim.NewChildProcess(nvim.ChildProcessArgs("-u", "NONE", "-n", "--embed", "--noplugin"))
	if err != nil {
		fmt.Println(err)
	}
	m.nvim = neovim
	m.nvim.RegisterHandler("Gui", func(updates ...interface{}) {
		m.guiUpdates <- updates
		m.signal.GuiSignal()
	})
	m.nvim.RegisterHandler("redraw", func(updates ...[]interface{}) {
		m.redrawUpdates <- updates
		m.signal.RedrawSignal()
	})

	go func() {
		err := m.nvim.Serve()
		if err != nil {
			fmt.Println(err)
		}
		m.stopOnce.Do(func() {
			close(m.stop)
		})
		m.signal.StopSignal()
	}()

	err = m.nvim.AttachUI(m.cols, m.rows, m.attachUIOption())
	if err != nil {
		fmt.Println(err)
	}
	m.uiAttached = true

	return m
}

func (m *MiniMap) attachUIOption() map[string]interface{} {
	o := make(map[string]interface{})
	o["rgb"] = true

	apiInfo, err := m.nvim.APIInfo()
	if err == nil {
		for _, item := range apiInfo {
			i, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			for k, v := range i {
				if k != "ui_events" {
					continue
				}
				events, ok := v.([]interface{})
				if !ok {
					continue
				}
				for _, event := range events {
					function, ok := event.(map[string]interface{})
					if !ok {
						continue
					}
					_, ok = function["name"]
					if !ok {
						continue
					}
					//if name == "wildmenu_show" {
					//	o["ext_wildmenu"] = true
					//} else if name == "cmdline_show" {
					//	o["ext_cmdline"] = true
					//} else if name == "msg_chunk" {
					//	o["ext_messages"] = true
					//} else if name == "popupmenu_show" {
					//	o["ext_popupmenu"] = true
					//} else if name == "tabline_update" {
					//	o["ext_tabline"] = m.drawTabline
					//}
				}
			}
		}
	}
	return o
}

func (m *MiniMap) paint(vqp *gui.QPaintEvent) {
	m.paintMutex.Lock()
	defer m.paintMutex.Unlock()

	rect := vqp.M_rect()
	font := m.font
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

	p := gui.NewQPainter2(m.widget)
	if m.background != nil {
		p.FillRect5(
			left,
			top,
			width,
			height,
			m.background.QColor(),
		)
	}

	p.SetFont(font.fontNew)

	for y := row; y < row+rows; y++ {
		if y >= m.rows {
			continue
		}
		m.fillHightlight(p, y, col, cols, [2]int{0, 0})
		m.drawText(p, y, col, cols, [2]int{0, 0})
	}

	m.drawWindows(p, row, col, rows, cols)
	p.DestroyQPainter()
}

func (m *MiniMap) drawWindows(p *gui.QPainter, row, col, rows, cols int) {
	done := make(chan struct{}, 1000)
	go func() {
		m.getWindows()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(1 * time.Millisecond):
	}
}

func (m *MiniMap) getWindows() {
	wins := map[nvim.Window]*Window{}
	neovim := m.nvim
	curtab, _ := neovim.CurrentTabpage()
	m.curtab = curtab
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
	b.Option("cmdheight", &m.cmdheight)
	err := b.Execute()
	if err != nil {
		return
	}
	m.curWins = wins
	for _, win := range m.curWins {
		buf, _ := neovim.WindowBuffer(win.win)
		win.bufName, _ = neovim.BufferName(buf)

		if win.height+win.pos[0] < m.rows-m.cmdheight {
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

func (m *MiniMap) updateRows() bool {
	var ret bool
	rows := m.height / m.font.lineHeight

	if rows != m.rows {
		ret = true
	}
	m.rows = rows
	return ret
}

func (m *MiniMap) updateCols() bool {
	var ret bool
	m.width = m.widget.Width()
	cols := int(float64(m.width) / m.font.truewidth)

	if cols != m.cols {
		ret = true
	}
	m.cols = cols
	return ret
}

func (m *MiniMap) updateSize() {
	isColDiff := m.updateCols()
	isRowDiff := m.updateRows()
	isTryResize := isColDiff || isRowDiff
	if m.uiAttached && isTryResize {
		m.nvim.TryResizeUI(m.cols, m.rows)
	}
}

func (m *MiniMap) handleRedraw(updates [][]interface{}) {
	for _, update := range updates {
		event := update[0].(string)
		args := update[1:]
		switch event {
		case "update_fg":
		case "update_bg":
		case "update_sp":
		case "cursor_goto":
		case "put":
			m.put(args)
		case "eol_clear":
		case "clear":
		case "resize":
			//m.resize(args)
		case "highlight_set":
			m.highlightSet(args)
		case "set_scroll_region":
			m.setScrollRegion(args)
		case "scroll":
			m.scroll(args)
		case "mode_change":
		case "popupmenu_show":
		case "popupmenu_hide":
		case "popupmenu_select":
		case "tabline_update":
		case "cmdline_show":
		case "cmdline_pos":
		case "cmdline_char":
		case "cmdline_hide":
		case "cmdline_function_show":
		case "cmdline_function_hide":
		case "wildmenu_show":
		case "wildmenu_select":
		case "wildmenu_hide":
		case "msg_start_kind":
		case "msg_chunk":
		case "msg_end":
		case "msg_showcmd":
		case "messages":
		case "busy_start":
		case "busy_stop":
		default:
			fmt.Println("Unhandle event", event)
		}
	}
	m.update()
}

func (m *MiniMap) handleRPCGui(updates []interface{}) {
	//event := updates[0].(string)
	//switch event {
	//case "Font":
	//	w.guiFont(updates[1:])
	//case "Linespace":
	//	w.guiLinespace(updates[1:])
	//case "finder_pattern":
	//	w.finder.showPattern(updates[1:])
	//case "finder_pattern_pos":
	//	w.finder.cursorPos(updates[1:])
	//case "finder_show_result":
	//	w.finder.showResult(updates[1:])
	//case "finder_hide":
	//	w.finder.hide()
	//case "finder_select":
	//	w.finder.selectResult(updates[1:])
	//case "signature_show":
	//	w.signature.showItem(updates[1:])
	//case "signature_pos":
	//	w.signature.pos(updates[1:])
	//case "signature_hide":
	//	w.signature.hide()
	//case "gonvim_copy_clipboard":
	//	go editor.copyClipBoard()
	//case "gonvim_get_maxline":
	//	go w.nvim.Eval("line('$')", &w.maxLine)
	//case "gonvim_workspace_new":
	//	editor.workspaceNew()
	//case "gonvim_workspace_next":
	//	editor.workspaceNext()
	//case "gonvim_workspace_previous":
	//	editor.workspacePrevious()
	//case "gonvim_workspace_switch":
	//	editor.workspaceSwitch(reflectToInt(updates[1]))
	//case "gonvim_workspace_cwd":
	//	w.setCwd()
	//case "gonvim_workspace_redrawSideItem":
	//	go func() {
	//		fl := editor.wsSide.items[editor.active].Filelist
	//		if fl.active != -1 {
	//			if len(fl.Fileitems) != 0 {
	//				fl.Fileitems[fl.active].updateModifiedbadge()
	//			}
	//		}
	//	}()
	//case "gonvim_workspace_redrawSideItems":
	//	go editor.wsSide.items[editor.active].setCurrentFileLabel()
	//	go editor.workspaces[editor.active].setFilepath()
	//case GonvimMarkdownNewBufferEvent:
	//	go w.markdown.newBuffer()
	//case GonvimMarkdownUpdateEvent:
	//	go w.markdown.update()
	//case GonvimMarkdownToggleEvent:
	//	go w.markdown.toggle()
	//case GonvimMarkdownScrollDownEvent:
	//	w.markdown.scrollDown()
	//case GonvimMarkdownScrollUpEvent:
	//	w.markdown.scrollUp()
	//default:
	//	fmt.Println("unhandled Gui event", event)
	//}
}

func (m *MiniMap) put(args []interface{}) {
	numChars := 0
	x := m.cursor[1]
	y := m.cursor[0]
	row := m.cursor[0]
	col := m.cursor[1]
	if row >= m.rows {
		return
	}
	line := m.content[row]
	oldFirstNormal := true
	if x >= len(line) {
		return
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
			char.normalWidth = m.isNormalWidth(char.char)
			lastChar = char
			char.highlight = m.highlight
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
	m.cursor[1] = col
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
	m.queueRedraw(x, y, numChars, 1)
}

func (m *MiniMap) highlightSet(args []interface{}) {
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
			highlight.foreground = m.highlight.background
			highlight.background = m.highlight.foreground
			m.highlight = highlight
			continue
		}

		fg, ok := hl["foreground"]
		if ok {
			rgba := calcColor(reflectToInt(fg))
			highlight.foreground = rgba
		} else {
			highlight.foreground = m.foreground
		}

		bg, ok := hl["background"]
		if ok {
			rgba := calcColor(reflectToInt(bg))
			highlight.background = rgba
		} else {
			highlight.background = m.background
		}
		m.highlight = highlight
	}
}

func (m *MiniMap) setScrollRegion(args []interface{}) {
	arg := args[0].([]interface{})
	top := reflectToInt(arg[0])
	bot := reflectToInt(arg[1])
	left := reflectToInt(arg[2])
	right := reflectToInt(arg[3])
	m.scrollRegion[0] = top
	m.scrollRegion[1] = bot
	m.scrollRegion[2] = left
	m.scrollRegion[3] = right
}

func (m *MiniMap) scroll(args []interface{}) {
	count := int(args[0].([]interface{})[0].(int64))

	isRowDiff := m.updateRows()
	if m.uiAttached && isRowDiff {
		m.nvim.TryResizeUI(m.cols, m.rows)
	}

	top := m.scrollRegion[0]
	bot := m.scrollRegion[1]
	left := m.scrollRegion[2]
	right := m.scrollRegion[3]

	if top == 0 && bot == 0 && left == 0 && right == 0 {
		top = 0
		bot = m.rows - 1
		left = 0
		right = m.cols - 1
	}

	m.queueRedraw(left, top, (right - left + 1), (bot - top + 1))

	if count > 0 {
		for row := top; row <= bot-count; row++ {
			for col := left; col <= right; col++ {
				m.content[row][col] = m.content[row+count][col]
			}
		}
		for row := bot - count + 1; row <= bot; row++ {
			for col := left; col <= right; col++ {
				m.content[row][col] = nil
			}
		}
		m.queueRedraw(left, (bot - count + 1), (right - left), count)
		if top > 0 {
			m.queueRedraw(left, (top - count), (right - left), count)
		}
	} else {
		for row := bot; row >= top-count; row-- {
			for col := left; col <= right; col++ {
				m.content[row][col] = m.content[row+count][col]
			}
		}
		for row := top; row < top-count; row++ {
			for col := left; col <= right; col++ {
				m.content[row][col] = nil
			}
		}
		m.queueRedraw(left, top, (right - left), -count)
		if bot < m.rows-1 {
			m.queueRedraw(left, bot+1, (right - left), -count)
		}
	}
}

func (m *MiniMap) update() {
	x := m.queueRedrawArea[0]
	y := m.queueRedrawArea[1]
	width := m.queueRedrawArea[2] - x
	height := m.queueRedrawArea[3] - y
	if width > 0 && height > 0 {
		// m.item.SetPixmap(s.pixmap)
		m.widget.Update2(
			int(float64(x)*m.font.truewidth),
			y*m.font.lineHeight,
			int(float64(width)*m.font.truewidth),
			height*m.font.lineHeight,
		)
	}
	m.queueRedrawArea[0] = m.cols
	m.queueRedrawArea[1] = m.rows
	m.queueRedrawArea[2] = 0
	m.queueRedrawArea[3] = 0
}

func (m *MiniMap) queueRedrawAll() {
	m.queueRedrawArea = [4]int{0, 0, m.cols, m.rows}
}

func (m *MiniMap) redraw() {
	m.queueRedrawArea = [4]int{m.cols, m.rows, 0, 0}
}

func (m *MiniMap) queueRedraw(x, y, width, height int) {
	if x < m.queueRedrawArea[0] {
		m.queueRedrawArea[0] = x
	}
	if y < m.queueRedrawArea[1] {
		m.queueRedrawArea[1] = y
	}
	if (x + width) > m.queueRedrawArea[2] {
		m.queueRedrawArea[2] = x + width
	}
	if (y + height) > m.queueRedrawArea[3] {
		m.queueRedrawArea[3] = y + height
	}
}

func (m *MiniMap) drawText(p *gui.QPainter, y int, col int, cols int, pos [2]int) {
	if y >= len(m.content) {
		return
	}
	font := p.Font()
	font.SetBold(false)
	font.SetItalic(false)
	pointF := core.NewQPointF()
	line := m.content[y]
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
	if col+cols < m.cols {
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
			fg = m.foreground
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
			p.SetPen2(gui.NewQColor3(fg.R, fg.G, fg.B, int(fg.A*255)))
			pointF.SetX(float64(col-pos[1]) * m.font.truewidth)
			pointF.SetY(float64((y-pos[0])*m.font.lineHeight + m.font.shift))
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
			fg = m.foreground
		}
		p.SetPen2(gui.NewQColor3(fg.R, fg.G, fg.B, int(fg.A*255)))
		pointF.SetX(float64(x-pos[1]) * m.font.truewidth)
		pointF.SetY(float64((y-pos[0])*m.font.lineHeight + m.font.shift))
		font.SetBold(char.highlight.bold)
		font.SetItalic(char.highlight.italic)
		p.DrawText(pointF, char.char)
	}
}

func (m *MiniMap) fillHightlight(p *gui.QPainter, y int, col int, cols int, pos [2]int) {
	rectF := core.NewQRectF()
	if y >= len(m.content) {
		return
	}
	line := m.content[y]
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
						float64(start-pos[1])*m.font.truewidth,
						float64((y-pos[0])*m.font.lineHeight),
						float64(end-start+1)*m.font.truewidth,
						float64(m.font.lineHeight),
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
					float64(start-pos[1])*m.font.truewidth,
					float64((y-pos[0])*m.font.lineHeight),
					float64(end-start+1)*m.font.truewidth,
					float64(m.font.lineHeight),
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
			float64(start-pos[1])*m.font.truewidth,
			float64((y-pos[0])*m.font.lineHeight),
			float64(end-start+1)*m.font.truewidth,
			float64(m.font.lineHeight),
		)
		p.FillRect4(
			rectF,
			gui.NewQColor3(lastBg.R, lastBg.G, lastBg.B, int(lastBg.A*255)),
		)
	}
}

func (m *MiniMap) isNormalWidth(char string) bool {
	if len(char) == 0 {
		return true
	}
	if char[0] <= 127 {
		return true
	}
	//return s.ws.font.fontMetrics.Width(char) == s.ws.font.truewidth
	return m.font.fontMetrics.HorizontalAdvance(char, -1) == m.font.truewidth
}
