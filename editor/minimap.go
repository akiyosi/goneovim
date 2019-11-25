package editor

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

type miniMapSignal struct {
	core.QObject
	_ func() `signal:"stopSignal"`
	_ func() `signal:"redrawSignal"`
}

// MiniMap is
type MiniMap struct {
	Screen

	curRegion *widgets.QWidget
	currBuf   string

	visible bool

	isSetColorscheme bool

	sync          sync.Mutex
	signal        *miniMapSignal
	redrawUpdates chan [][]interface{}
	stopOnce      sync.Once
	stop          chan struct{}

	nvim       *nvim.Nvim
	uiAttached bool
	rows       int
	cols       int
}

func newMiniMap() *MiniMap {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	widget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")
	widget.SetFixedWidth(140)

	curRegion := widgets.NewQWidget(nil, 0)
	curRegion.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	curRegion.SetFixedWidth(140)
	curRegion.SetFixedHeight(1)

	m := &MiniMap{
		Screen: Screen{
			name:           "minimap",
			widget:         widget,
			windows:        make(map[gridId]*Window),
			cursor:         [2]int{0, 0},
			scrollRegion:   []int{0, 0, 0, 0},
			highlightGroup: make(map[string]int),
		},
		curRegion:     curRegion,
		stop:          make(chan struct{}),
		signal:        NewMiniMapSignal(nil),
		redrawUpdates: make(chan [][]interface{}, 1000),
	}
	m.signal.ConnectRedrawSignal(func() {
		updates := <-m.redrawUpdates
		m.handleRedraw(updates)
	})
	m.signal.ConnectStopSignal(func() {
	})
	// m.widget.ConnectPaintEvent(m.paint)
	m.widget.ConnectResizeEvent(func(event *gui.QResizeEvent) {
		m.updateSize()
	})
	m.widget.ConnectMousePressEvent(m.mouseEvent)
	m.widget.ConnectWheelEvent(m.wheelEvent)
	m.widget.Hide()

	switch runtime.GOOS {
	case "windows":
		m.font = initFontNew("Consolas", 1, 0, false)
	case "darwin":
		m.font = initFontNew("Courier New", 2, 0, false)
	default:
		m.font = initFontNew("Monospace", 1, 0, false)
	}

	return m
}

func (m *MiniMap) startMinimapProc() {
	neovim, err := nvim.NewChildProcess(nvim.ChildProcessArgs("-u", "NONE", "-n", "--embed", "--headless"))
	if err != nil {
		fmt.Println(err)
	}
	m.nvim = neovim
	m.nvim.RegisterHandler("redraw", func(updates ...[]interface{}) {
		m.redrawUpdates <- updates
		m.signal.RedrawSignal()
	})
	m.width = m.widget.Width()
	m.height = m.widget.Height()

	m.updateSize()

	go func() {
		err = m.nvim.Serve()
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
	m.visible = editor.config.MiniMap.Visible

	m.nvim.Subscribe("Gui")
	m.nvim.Command(":set laststatus=0 | set noruler")
	m.nvim.Command(":syntax on")
	m.nvim.Command(":set nowrap")
	m.nvim.Command(":set virtualedit+=all")
}

func (m *MiniMap) exit() {
	go m.nvim.Command(":q!")
}

func (m *MiniMap) attachUIOption() map[string]interface{} {
	o := make(map[string]interface{})
	o["rgb"] = true
	o["ext_linegrid"] = true
	return o
}

func (m *MiniMap) setColor() {
	c := editor.colors.fg
	m.curRegion.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(%d, %d, %d, 0.1);}", c.R, c.G, c.B))
}

func (m *MiniMap) toggle() {
	win, ok := m.windows[1]
	if !ok {
		return
	}
	if win == nil {
		return
	}
	if m.visible {
		m.visible = false
	} else {
		m.visible = true
	}
	m.curRegion.SetParent(win.widget)
	m.bufUpdate()
	m.bufSync()
}

func (m *MiniMap) updateRows() bool {
	var ret bool
	m.height = m.widget.Height()
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

func (m *MiniMap) bufUpdate() {
	if strings.Contains(m.ws.filepath, "[denite]") {
		return
	}
	if !m.visible {
		m.widget.Hide()
		return
	}
	if m.ws.nvim == nil || m.nvim == nil {
		return
	}
	if !m.isSetColorscheme {
		m.setColorscheme()
	}
	m.widget.Show()

	if m.currBuf == m.ws.filepath {
		return
	}
	m.currBuf = m.ws.filepath

	if m.currBuf == "" {
		go m.nvim.Command(":e! [No Name]")
	} else {
		go m.nvim.Command(":e! " + m.currBuf)
	}
	m.mapScroll()
}

func (m *MiniMap) setColorscheme() {
	colo, _ := m.ws.nvim.CommandOutput("colo")

	sep := "/"
	switch runtime.GOOS {
	case "windows":
		sep = `\`
	default:
	}

	runtimePaths, _ := m.ws.nvim.RuntimePaths()
	runtimeDir := ""
	colorschemePath := ""
	for _, path := range runtimePaths {
		lsDirs, _ := ioutil.ReadDir(path)
		for _, d := range lsDirs {
			dirname := d.Name()
			finfo, err := os.Stat(path + sep + dirname)
			if err != nil {
				continue
			}
			if finfo.IsDir() {
				packDirs, _ := ioutil.ReadDir(path + sep + dirname)
				for _, p := range packDirs {
					plugname := p.Name()
					if strings.Contains(plugname, colo) {
						runtimeDir = path
						colorschemePath = path + sep + dirname + sep + plugname
						break
					}
				}
				if colorschemePath != "" {
					break
				}
			}
		}
	}
	m.nvim.Command("set runtimepath^=" + runtimeDir)
	m.nvim.Command(":colorscheme " + colo)

	m.isSetColorscheme = true
}

func (m *MiniMap) mapScroll() {
	absScreenTop := m.ws.curLine - m.ws.screen.cursor[0]
	var absMapTop int
	m.nvim.Eval("line('w0')", &absMapTop)

	win, ok := m.ws.screen.windows[m.ws.cursor.gridid]
	if !ok {
		return
	}
	if win == nil {
		return
	}
	regionHeight := win.rows

	m.curRegion.SetFixedHeight(int(float64(regionHeight) * float64(m.font.lineHeight)))
	pos := int(float64(m.font.lineHeight) * float64(absScreenTop-absMapTop))
	m.curRegion.Move2(0, pos)
}

func (m *MiniMap) bufSync() {
	if strings.Contains(m.ws.filepath, "[denite]") {
		return
	}
	if !m.visible {
		m.widget.Hide()
		return
	}
	if m.ws.nvim == nil || m.nvim == nil {
		return
	}

	// Get current buffer
	buf, err := m.ws.nvim.CurrentBuffer()
	if err != nil {
		return
	}

	// Get minimap window
	mmWin, ok := m.windows[1]
	if !ok {
		return
	}
	if mmWin == nil {
		return
	}

	start := m.ws.curLine - m.ws.screen.cursor[0]
	end := m.ws.curLine - m.ws.screen.cursor[0] + mmWin.rows

	// Get buffer contents
	replacement, err := m.ws.nvim.BufferLines(
		buf,
		start,
		end,
		false,
	)
	if err != nil {
		return
	}

	// Get current buffer of minimap
	minimapBuf, err := m.nvim.CurrentBuffer()
	if err != nil {
		return
	}

	// Set buffer contents
	m.nvim.SetBufferLines(
		minimapBuf,
		start,
		end,
		false,
		replacement,
	)
}

func (m *MiniMap) handleRedraw(updates [][]interface{}) {
	for _, update := range updates {
		event := update[0].(string)
		args := update[1:]
		switch event {

		case "grid_resize":
			m.gridResize(args)
		// case "default_colors_set":
		// 	args := update[1].([]interface{})
		// 	w.setColorsSet(args)
		case "hl_attr_define":
			m.setHighAttrDef(args)
		case "hl_group_set":
			m.setHighlightGroup(args)
			m.setColor()
		case "grid_line":
			m.gridLine(args)
		case "grid_clear":
			m.gridClear(args)
		case "grid_destroy":
			m.gridDestroy(args)
		case "grid_cursor_goto":
			// m.gridCursorGoto(args)
		case "grid_scroll":
			m.gridScroll(args)
			m.mapScroll()

		default:
		}
	}
	m.update()
}

func (m *MiniMap) transparent(bg *RGBA) int {
	transparent := int(math.Trunc(editor.config.Editor.Transparent * float64(255)))

	var t int
	if m.ws.background.equals(bg) {
		t = 0
	} else {
		t = transparent
	}
	return t
}

func (m *MiniMap) wheelEvent(event *gui.QWheelEvent) {
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()

	var v, h, vert, horiz int
	var accel int
	font := m.font

	switch runtime.GOOS {
	case "darwin":
		pixels := event.PixelDelta()
		if pixels != nil {
			v = pixels.Y()
			h = pixels.X()
		}
		if pixels.X() < 0 && m.scrollDust[0] > 0 {
			m.scrollDust[0] = 0
		}
		if pixels.Y() < 0 && m.scrollDust[1] > 0 {
			m.scrollDust[1] = 0
		}

		dx := math.Abs(float64(m.scrollDust[0]))
		dy := math.Abs(float64(m.scrollDust[1]))

		fontheight := float64(font.lineHeight)
		fontwidth := font.truewidth

		m.scrollDust[0] += h
		m.scrollDust[1] += v

		if dx >= fontwidth {
			horiz = int(math.Trunc(float64(m.scrollDust[0]) / fontheight))
			m.scrollDust[0] = 0
		}
		if dy >= fontwidth {
			vert = int(math.Trunc(float64(m.scrollDust[1]) / fontwidth))
			m.scrollDust[1] = 0
		}

		m.scrollDustDeltaY = int(math.Abs(float64(vert)) - float64(m.scrollDustDeltaY))
		if m.scrollDustDeltaY < 1 {
			m.scrollDustDeltaY = 0
		}
		if m.scrollDustDeltaY <= 2 {
			accel = 1
		} else if m.scrollDustDeltaY > 2 {
			accel = int(float64(m.scrollDustDeltaY) / float64(4))
		}

	default:
		vert = event.AngleDelta().Y()
		accel = 16
	}

	if vert == 0 && horiz == 0 {
		return
	}

	if vert > 0 {
		m.nvim.Input(fmt.Sprintf("%v<C-y>", accel))
	} else if vert < 0 {
		m.nvim.Input(fmt.Sprintf("%v<C-e>", accel))
	}

	event.Accept()
}

func (m *MiniMap) mouseEvent(event *gui.QMouseEvent) {
	font := m.font
	y := int(float64(event.Y()) / float64(font.lineHeight))
	var absMapTop int
	m.nvim.Eval("line('w0')", &absMapTop)
	targetPos := absMapTop + y
	m.ws.nvim.Command(fmt.Sprintf("%d", targetPos))
}

func (w *Window) drawMinimap(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	wsfont := w.getFont()
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
				buffer.WriteString(`@`)
				slice = slice[1:]
			}
		}

		text := buffer.String()
		if text != "" {
			fg := highlight.fg()
			if fg != nil {
				p.SetPen2(fg.QColor())
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
		p.SetPen2(fg.QColor())
		pointF.SetX(float64(x) * wsfont.truewidth)
		pointF.SetY(float64((y)*wsfont.lineHeight + wsfont.shift))
		font.SetBold(line[x].highlight.bold)
		font.SetItalic(line[x].highlight.italic)
		p.DrawText(pointF, line[x].char)
	}
}
