package editor

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"

	"github.com/akiyosi/goneovim/util"
	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
	"github.com/akiyosi/qt/widgets"
	"github.com/neovim/go-client/nvim"
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
	currBuf            string
	colorscheme        string
	viewport           [4]int
	rows               int
	curHeight          int
	curPos             int
	cols               int
	stopOnce           sync.Once
	mu                 sync.Mutex
	visible            bool
	uiAttached         bool
	isProcessSync      bool
	isSetColorscheme   bool
	scrollPixelsDeltaY int
}

func newMiniMap() *MiniMap {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	widget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")
	widget.SetFixedWidth(editor.config.MiniMap.Width)

	m := &MiniMap{
		Screen: Screen{
			name:           "minimap",
			widget:         widget,
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
	m.widget.ConnectResizeEvent(func(event *gui.QResizeEvent) {
		m.updateSize()
	})
	m.widget.ConnectMousePressEvent(m.mouseEvent)
	m.widget.ConnectWheelEvent(m.wheelEvent)
	m.widget.Hide()

	switch runtime.GOOS {
	case "windows":
		m.font = initFontNew("Consolas", 1.0, gui.QFont__Normal, 100, 0, 0)
	case "darwin":
		m.font = initFontNew("Courier New", 2.0, gui.QFont__Normal, 100, 0, 0)
	default:
		m.font = initFontNew("Monospace", 1.0, gui.QFont__Normal, 100, 0, 0)
	}

	return m
}

func (m *MiniMap) startMinimapProc(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var neovim *nvim.Nvim
	var err error
	minimapProcessArgs := nvim.ChildProcessArgs("-u", "NONE", "-n", "--embed", "--headless")
	minimapProcessServe := nvim.ChildProcessServe(false)
	minimapProcessContext := nvim.ChildProcessContext(ctx)

	useWSL := editor.opts.Wsl != nil || editor.config.Editor.UseWSL
	if runtime.GOOS != "windows" {
		useWSL = false
	}

	if editor.opts.Nvim != "" {
		childProcessCmd := nvim.ChildProcessCommand(editor.opts.Nvim)
		neovim, err = nvim.NewChildProcess(minimapProcessArgs, childProcessCmd, minimapProcessServe, minimapProcessContext)
	} else if useWSL {
		neovim, err = newWslProcess()
	} else if editor.opts.Ssh != "" {
		neovim, err = newRemoteChildProcess()
	} else {
		neovim, err = nvim.NewChildProcess(minimapProcessArgs, minimapProcessServe, minimapProcessContext)
	}
	if err != nil {
		editor.putLog("[minimap] start nvim error:", err)
	}
	m.nvim = neovim
	m.nvim.RegisterHandler("redraw", func(updates ...[]interface{}) {
		m.redrawUpdates <- updates
		m.signal.RedrawSignal()
	})

	m.updateSize()
	if m.cols < 1 {
		m.cols = 1
	}
	if m.rows < 1 {
		m.rows = 1
	}

	go func() { _ = m.nvim.Serve() }()

	// Attach UI
	if err := m.nvim.AttachUI(m.cols, m.rows, m.attachUIOption()); err != nil {
		editor.putLog("[minimap] AttachUI error:", err)
	}
	m.uiAttached = true
	m.updateSize()

	wsRtp, _ := m.ws.nvim.CommandOutput(`echo &runtimepath`)
	wsPp, _ := m.ws.nvim.CommandOutput(`echo &packpath`)
	wsData, _ := m.ws.nvim.CommandOutput(`echo stdpath('data')`)
	wsRtp = strings.TrimSpace(wsRtp)
	wsPp = strings.TrimSpace(wsPp)
	wsData = strings.TrimSpace(wsData)

	if wsRtp != "" {
		_ = m.nvim.Command(fmt.Sprintf(`lua do local v=%q; local t=vim.split(v,","); vim.opt.runtimepath = t end`, wsRtp))
	}
	if wsPp != "" {
		_ = m.nvim.Command(fmt.Sprintf(`lua do local v=%q; local t=vim.split(v,","); vim.opt.packpath = t end`, wsPp))
	}

	_ = m.nvim.Command(fmt.Sprintf(`silent! lua pcall(function()
	  local ok, cfg = pcall(require, 'nvim-treesitter.configs')
	  if ok and cfg and cfg.setup then
	    cfg.setup({
	      parser_install_dir = %q .. "/site",
	      highlight = { enable = true, additional_vim_regex_highlighting = false },
	    })
	    -- 重要：parser_install_dir を rtp にも追加（queries解決の一助）
	    local site = %q .. "/site"
	    if vim.fn.isdirectory(site) == 1 then
	      vim.opt.runtimepath:append(site)
	    end
	  end
	end)`, wsData, wsData))

	rtpCount, _ := m.nvim.CommandOutput(`echo len(split(&runtimepath, ","))`)
	ppCount, _ := m.nvim.CommandOutput(`echo len(split(&packpath, ","))`)
	editor.putLog("[minimap] &rtp set from WS, len=", strings.TrimSpace(rtpCount))
	editor.putLog("[minimap] &pp  set from WS, len=", strings.TrimSpace(ppCount))
}

func (m *MiniMap) exit() { go m.nvim.Command(":q!") }

func (m *MiniMap) attachUIOption() map[string]interface{} {
	return map[string]interface{}{"rgb": true, "ext_linegrid": true}
}

func (m *MiniMap) toggle() {
	m.mu.Lock()
	m.visible = !m.visible
	m.mu.Unlock()
	m.bufUpdate()
	m.bufSync()
	m.ws.updateSize()
}

func (m *MiniMap) updateRows() bool {
	fontHeight := m.font.lineHeight
	m.height = m.widget.Height()
	if fontHeight == 0 {
		return false
	}
	rows := m.height / fontHeight
	changed := rows != m.rows
	m.rows = rows
	return changed
}

func (m *MiniMap) updateCols() bool {
	m.width = m.widget.Width()
	fontWidth := m.font.cellwidth
	if fontWidth == 0 {
		return false
	}
	cols := int(float64(m.width) / fontWidth)
	changed := cols != m.cols
	m.cols = cols
	return changed
}

func (m *MiniMap) updateSize() {
	if m.uiAttached && (m.updateCols() || m.updateRows()) {
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
		return
	}

	m.nvim.Command(":e! " + m.currBuf)

	_ = m.nvim.Command("silent! filetype plugin indent on")
	_ = m.nvim.Command("silent! filetype detect")

	m.attachTreesitterForCurrentBuffer()

	m.mapScroll()
}

func (m *MiniMap) setColorscheme() {
	if m.isSetColorscheme {
		return
	}
	colo := m.ws.colorscheme

	_ = m.nvim.Command("silent! colorscheme " + colo)
	m.colorscheme = colo
	m.isSetColorscheme = true
}

func (m *MiniMap) attachTreesitterForCurrentBuffer() {
	_ = m.nvim.Command(`silent! lua pcall(function()
	  local ft = vim.bo.filetype or ''
	  if ft == '' then return end
	  local okP, parsers = pcall(require, 'nvim-treesitter.parsers')
	  local okTS, ts = pcall(require, 'vim.treesitter')
	  if not okTS or not ts or not ts.start then return end
	  local lang = ft
	  if okP and parsers and parsers.ft_to_lang then
	    lang = parsers.ft_to_lang(ft)
	  end
	  pcall(ts.start, 0, lang)
	end)`)
}

func (m *MiniMap) mapScroll() {
	regionHeight := m.ws.viewport[1] - m.ws.viewport[0]
	relativeRegionPos := m.ws.viewport[0] - m.viewport[0]

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
	m.curHeight = int(float64(regionHeight) * float64(m.font.lineHeight))

	mmWin, ok := m.getWindow(1)
	if !ok {
		return
	}

	mmWin.refreshUpdateArea(1)
	width := m.widget.Width()
	mmWin.Update2(0, oldPos, width, oldHeight)
	mmWin.Update2(0, m.curPos, width, m.curHeight)
}

func (m *MiniMap) bufSync() {
	m.mu.Lock()
	defer func() { m.isProcessSync = false; m.mu.Unlock() }()
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

	win, err := m.ws.nvim.CurrentWindow()
	if err != nil {
		return
	}
	config, err := m.ws.nvim.WindowConfig(win)
	if err != nil {
		return
	}
	if isWindowFloatForConfig(config) {
		return
	}

	buf, err := m.ws.nvim.CurrentBuffer()
	if err != nil {
		return
	}

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

	replacement, err := m.ws.nvim.BufferLines(buf, start, end, false)
	if err != nil {
		return
	}
	if len(replacement) < 1 {
		return
	}

	minimapBuf, err := m.nvim.CurrentBuffer()
	if err != nil {
		return
	}

	m.nvim.SetBufferLines(minimapBuf, start, end, false, replacement)
}

func isWindowFloatForConfig(wc *nvim.WindowConfig) bool {
	if wc == nil {
		return false
	}
	return wc.Relative != ""
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
		}
	}
}

func (m *MiniMap) transparent(bg *RGBA) int {
	transparent := int(math.Trunc(editor.config.Editor.Transparent * float64(255)))
	if m.ws.background.equals(bg) {
		return 0
	}
	return transparent
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
		m.scrollPixelsDeltaY = int(math.Abs(float64(vert)) - float64(m.scrollPixelsDeltaY))
		if m.scrollPixelsDeltaY < 1 {
			m.scrollPixelsDeltaY = 0
		}
		if m.scrollPixelsDeltaY <= 2 {
			accel = 1
		} else {
			accel = int(float64(m.scrollPixelsDeltaY) / 2.0)
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
	targetPos := m.viewport[0] + y
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
	p.SetFont(wsfont.qfont)
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
							float64(y*wsfont.lineHeight+wsfont.baselineOffset),
							float64(width)*wsfont.cellwidth*k,
							float64(wsfont.lineHeight)*k,
							2, 2, core.Qt__AbsoluteSize,
						)
						p.FillPath(path, gui.NewQBrush3(fg.QColor(), core.Qt__SolidPattern))
						width = 0
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
			gui.NewQColor3(color.R, color.G, color.B, 25),
			1,
		),
	)
}
