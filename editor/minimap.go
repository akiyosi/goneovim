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

	"github.com/akiyosi/goneovim/util"
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
	signal        *miniMapSignal
	nvim          *nvim.Nvim
	stop          chan struct{}
	redrawUpdates chan [][]interface{}
	Screen
	currBuf          string
	colorscheme      string
	viewport         [4]int
	rows             int
	curHeight        int
	curPos           int
	cols             int
	stopOnce         sync.Once
	mu               sync.Mutex
	visible          bool
	uiAttached       bool
	isProcessSync    bool
	isSetColorscheme bool
}

func newMiniMap() *MiniMap {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	widget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")
	widget.SetFixedWidth(editor.config.MiniMap.Width)

	m := &MiniMap{
		Screen: Screen{
			name:   "minimap",
			widget: widget,
			// windows:        make(map[gridId]*Window),
			windows:        sync.Map{},
			cursor:         [2]int{0, 0},
			highlightGroup: make(map[string]int),
		},
		visible:       editor.config.MiniMap.Visible,
		stop:          make(chan struct{}),
		signal:        NewMiniMapSignal(nil),
		redrawUpdates: make(chan [][]interface{}, 1000),
	}
	m.signal.ConnectRedrawSignal(func() {
		updates := <-m.redrawUpdates
		m.handleRedraw(updates)
	})
	// m.signal.ConnectStopSignal(func() {
	// })
	// m.widget.ConnectPaintEvent(m.paint)
	m.widget.ConnectResizeEvent(func(event *gui.QResizeEvent) {
		m.updateSize()
	})
	m.widget.ConnectMousePressEvent(m.mouseEvent)
	m.widget.ConnectWheelEvent(m.wheelEvent)
	m.widget.Hide()

	switch runtime.GOOS {
	case "windows":
		m.font = initFontNew("Consolas", 1.0, 0, 0.0)
	case "darwin":
		m.font = initFontNew("Courier New", 2.0, 0, 0.0)
	default:
		m.font = initFontNew("Monospace", 1.0, 0, 0.0)
	}

	return m
}

func (m *MiniMap) startMinimapProc() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var neovim *nvim.Nvim
	var err error
	minimapProcessArgs := nvim.ChildProcessArgs("-u", "NONE", "-n", "--embed", "--headless")
	if editor.opts.Nvim != "" {
		// Attaching to /path/to/nvim
		childProcessCmd := nvim.ChildProcessCommand(editor.opts.Nvim)
		neovim, err = nvim.NewChildProcess(minimapProcessArgs, childProcessCmd)
	} else if editor.opts.Ssh != "" {
		// Attaching remote nvim via ssh
		neovim, err = newRemoteChildProcess()
	} else {
		// Attaching to nvim normally
		neovim, err = nvim.NewChildProcess(minimapProcessArgs)
	}
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
		_ = m.nvim.Serve()
		// if err != nil {
		// 	fmt.Println(err)
		// }
		// m.stopOnce.Do(func() {
		// 	close(m.stop)
		// })
		// m.signal.StopSignal()
	}()

	err = m.nvim.AttachUI(m.cols, m.rows, m.attachUIOption())
	if err != nil {
		fmt.Println(err)
	}
	m.uiAttached = true

	// neovim.Subscribe("Gui")
	go func() {
		neovim.Command(":syntax on")
		neovim.Command(":set ro nobackup noswapfile mouse=nv laststatus=0 noruler nowrap noshowmode virtualedit+=all ts=4")
	}()
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

func (m *MiniMap) toggle() {
	m.mu.Lock()
	if m.visible {
		m.visible = false
	} else {
		m.visible = true
	}
	m.mu.Unlock()
	m.bufUpdate()
	m.bufSync()
	m.ws.updateSize()
}

func (m *MiniMap) updateRows() bool {
	var ret bool
	fontHeight := m.font.lineHeight
	m.height = m.widget.Height()
	if fontHeight == 0 {
		return false
	}
	rows := m.height / fontHeight

	if rows != m.rows {
		ret = true
	}
	m.rows = rows
	return ret
}

func (m *MiniMap) updateCols() bool {
	var ret bool
	m.width = m.widget.Width()
	fontWidth := m.font.cellwidth
	if fontWidth == 0 {
		return false
	}
	cols := int(float64(m.width) / fontWidth)

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
	m.mu.Lock()
	defer m.mu.Unlock()

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
	m.setColorscheme()
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
	if m.isSetColorscheme {
		return
	}
	colo, _ := m.ws.nvim.CommandOutput("colo")

	sep := "/"
	switch runtime.GOOS {
	case "windows":
		sep = `\`
	default:
	}

	runtimePaths, _ := m.ws.nvim.RuntimePaths()
	colorschemePath := ""
	treesitterPath := ""
	isColorschmepathDetected := false
	for _, path := range runtimePaths {
		lsDirs, err := ioutil.ReadDir(path)
		if err != nil {
			continue
		}
		if strings.Contains(path, "nvim-treesitter") {
			if treesitterPath == "" || len(treesitterPath) > len(path) {
				treesitterPath = path
			}
		}
		editor.putLog("Serrch for colorscheme of minimap:: rtp:", path)
		if !isColorschmepathDetected {
			for _, d := range lsDirs {
				dirname := d.Name()
				finfo, err := os.Stat(path + sep + dirname)
				if err != nil {
					continue
				}
				editor.putLog("Serrch for colorscheme of minimap:: directory:", dirname)
				if finfo.IsDir() {
					packDirs, _ := ioutil.ReadDir(path + sep + dirname)
					for _, p := range packDirs {
						plugname := p.Name()
						if plugname == colo+".vim" {
							plugdirInfo, err2 := os.Stat(path + sep + dirname + sep + plugname)
							if err2 != nil {
								continue
							}
							if plugdirInfo.IsDir() {
								colorschemePath = path + sep + dirname + sep + plugname
							} else {
								colorschemePath = path + sep + dirname
								if dirname == "colors" {
									colorschemePath = path
								}
							}
							isColorschmepathDetected = true
							editor.putLog("Serrch for colorscheme of minimap:: colorscheme path:", colorschemePath)
							break
						}
						if strings.Contains(plugname, colo) {
							colorschemePath = path + sep + dirname + sep + plugname
							editor.putLog("Serrch for colorscheme of minimap:: colorscheme path:", colorschemePath)
							continue
						}
					}
					if colorschemePath != "" {
						continue
					}
				}
			}
		}
	}

	// set runtimepath
	m.nvim.Command("set runtimepath^=" + colorschemePath)
	editor.putLog("set rtp for minimap colorscheme settings:", colorschemePath)
	output, _ := m.nvim.CommandOutput("echo getcompletion('', 'color')")
	editor.putLog("available colorscheme for minimap:", output)

	if m.colorscheme == colo {
		return
	}
	// set colorscheme
	m.nvim.Command(":colorscheme " + colo)
	m.colorscheme = colo
	m.isSetColorscheme = true

	editor.putLog("detected treesitter runtime path:", treesitterPath)

	// if nvim-treesitter is installed
	m.nvim.Command("luafile " + treesitterPath + "/lua/nvim-treesitter.lua")
	m.nvim.Command("luafile " + treesitterPath + "/lua/nvim-treesitter/highlight.lua")
	m.nvim.Command("luafile " + treesitterPath + "/lua/nvim-treesitter/configs.lua")
	m.nvim.Command("source  " + treesitterPath + "/plugin//nvim-treesitter.vim")
	m.nvim.Command("TSEnableAll highlight")

}

func (m *MiniMap) mapScroll() {
	relativeRegionPos := 0
	regionHeight := 0

	regionHeight = m.ws.viewport[1] - m.ws.viewport[0]
	relativeRegionPos = m.ws.viewport[0] - m.viewport[0]

	win, ok := m.ws.screen.getWindow(m.ws.cursor.gridid)
	if !ok {
		return
	}

	if regionHeight <= 0 {
		regionHeight = win.rows
	}

	if relativeRegionPos < 0 {
		regionHeight = regionHeight + relativeRegionPos
		relativeRegionPos = 0
	}
	if regionHeight < 0 {
		regionHeight = 0
	}

	oldPos := m.curPos
	oldHeight := m.curHeight
	if oldPos > 0 {
		oldPos = oldPos - 1
	}

	m.curPos = int(float64(m.font.lineHeight) * float64(relativeRegionPos))
	m.curHeight = int(
		float64(regionHeight) * float64(m.font.lineHeight),
	)

	mmWin, ok := m.getWindow(1)
	if !ok {
		return
	}

	mmWin.refreshUpdateArea(1)
	width := m.widget.Width()
	mmWin.Update2(
		0, oldPos,
		width,
		oldHeight,
	)
	mmWin.Update2(
		0, m.curPos,
		width,
		m.curHeight,
	)
}

func (m *MiniMap) bufSync() {
	m.mu.Lock()
	defer func() {
		m.isProcessSync = false
		m.mu.Unlock()
	}()
	if m.isProcessSync {
		return
	}
	m.isProcessSync = true

	if !m.visible {
		m.widget.Hide()
		return
	}
	if m.ws.nvim == nil || m.nvim == nil {
		return
	}

	// Get current window
	win, err := m.ws.nvim.CurrentWindow()
	if err != nil {
		return
	}
	// Get window config
	config, err := m.ws.nvim.WindowConfig(win)
	if err != nil {
		return
	}
	// if float window
	isFloat := isWindowFloatForConfig(config)
	if isFloat {
		return
	}

	// Get current buffer
	buf, err := m.ws.nvim.CurrentBuffer()
	if err != nil {
		return
	}

	// mmWin, ok := m.getWindow(1)
	// if !ok {
	// 	return
	// }

	// TODO: Rewire code based win_viewport event
	start := 0
	end := 0
	var pos [4]int
	m.ws.nvim.Eval("getpos('$')", &pos)
	var minimapPos [4]int
	m.nvim.Eval("getpos('$')", &minimapPos)
	if pos[1] > minimapPos[1] {
		end = pos[1] + 1
	} else {
		end = minimapPos[1] + 1
	}

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
	if len(replacement) < 1 {
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

func isWindowFloatForConfig(wc *nvim.WindowConfig) bool {
	if wc == nil {
		return false
	}

	if wc.Relative == "" {
		return false
	}

	return true
}

func (m *MiniMap) handleRedraw(updates [][]interface{}) {
	for _, update := range updates {
		event := update[0].(string)
		args := update[1:]
		switch event {
		case "grid_resize":
			m.gridResize(args)
		case "hl_attr_define":
			m.setHlAttrDef(args)
		case "hl_group_set":
			m.setHighlightGroup(args)
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

		case "win_viewport":
			vp := args[0].([]interface{})
			m.viewport = [4]int{
				util.ReflectToInt(vp[2]) + 1,
				util.ReflectToInt(vp[3]) + 1,
				util.ReflectToInt(vp[4]) + 1,
				util.ReflectToInt(vp[5]) + 1,
			}
		case "flush":
			m.mapScroll()
			m.update()

		default:
		}
	}
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
	m.mu.Lock()
	defer m.mu.Unlock()

	var v, h, vert, horiz, accel int
	font := m.font
	win, ok := m.getWindow(1)
	if !ok {
		return
	}

	switch runtime.GOOS {
	case "darwin":
		pixels := event.PixelDelta()
		if pixels != nil {
			v = pixels.Y()
			h = pixels.X()
		}
		if pixels.X() < 0 && win.scrollPixels[0] > 0 {
			win.scrollPixels[0] = 0
		}
		if pixels.Y() < 0 && win.scrollPixels[1] > 0 {
			win.scrollPixels[1] = 0
		}

		dx := math.Abs(float64(win.scrollPixels[0]))
		dy := math.Abs(float64(win.scrollPixels[1]))

		fontheight := float64(font.lineHeight)
		fontwidth := font.cellwidth

		win.scrollPixels[0] += h
		win.scrollPixels[1] += v

		if dx >= fontwidth {
			horiz = int(math.Trunc(float64(win.scrollPixels[0]) / fontheight))
			win.scrollPixels[0] = 0
		}
		if dy >= fontwidth {
			vert = int(math.Trunc(float64(win.scrollPixels[1]) / fontwidth))
			win.scrollPixels[1] = 0
		}

		win.scrollPixelsDeltaY = int(math.Abs(float64(vert)) - float64(win.scrollPixelsDeltaY))
		if win.scrollPixelsDeltaY < 1 {
			win.scrollPixelsDeltaY = 0
		}

		if win.scrollPixelsDeltaY <= 2 {
			accel = 1
		} else if win.scrollPixelsDeltaY > 2 {
			accel = int(float64(win.scrollPixelsDeltaY) / float64(2))
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
	targetPos := 0
	targetPos = m.viewport[0] + y
	go m.ws.nvim.Command(fmt.Sprintf("%d", targetPos))

	mappings, err := m.ws.nvim.KeyMap("normal")
	if err != nil {
		return
	}
	var isThereZzMap bool
	for _, mapping := range mappings {
		if mapping.LHS == "zz" {
			isThereZzMap = true
		}
	}
	if !isThereZzMap {
		m.ws.nvim.Input("zz")
	}
}

func (w *Window) drawMinimap(p *gui.QPainter, y int, col int, cols int) {
	if y >= len(w.content) {
		return
	}
	wsfont := w.getFont()
	p.SetFont(wsfont.fontNew)
	p.SetRenderHint(gui.QPainter__Antialiasing, true)
	line := w.content[y]
	chars := map[*Highlight][]int{}

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

		highlight := line[x].highlight
		colorSlice, ok := chars[highlight]
		if !ok {
			colorSlice = []int{}
		}
		colorSlice = append(colorSlice, x)
		chars[highlight] = colorSlice
	}

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
			fg := highlight.fg().HSV().Colorless().RGB()
			if fg != nil {
				p.SetPen2(fg.QColor())
			}

			width := 0
			for i, c := range text {
				if string(c) == "@" {
					width++
				}
				if string(c) == " " || i == len(text)-1 {
					if width > 0 {
						k := 0.8
						path := gui.NewQPainterPath()
						path.AddRoundedRect2(
							float64(col+i-width)*wsfont.cellwidth*k,
							float64(y*wsfont.lineHeight+wsfont.shift),
							float64(width)*wsfont.cellwidth*k,
							float64(wsfont.lineHeight)*k,
							2,
							2,
							core.Qt__AbsoluteSize,
						)
						p.FillPath(path, gui.NewQBrush3(fg.QColor(), core.Qt__SolidPattern))
						width = 0
					} else {
						continue
					}
				}
			}
		}

	}
}

func (m *MiniMap) updateCurrentRegion(p *gui.QPainter) {
	curRegionRect := core.NewQRectF4(
		float64(0),
		float64(m.curPos),
		float64(m.widget.Width()),
		float64(m.curHeight),
	)
	color := editor.colors.fg
	p.FillRect(
		curRegionRect,
		gui.NewQBrush3(
			gui.NewQColor3(
				color.R,
				color.G,
				color.B,
				25,
			),
			1,
		),
	)
}
