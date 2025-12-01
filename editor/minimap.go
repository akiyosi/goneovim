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

var minimapScaleValue float64 = 0.7 // scale value

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

func (m *MiniMap) asyncMiniNvim(f func(n *nvim.Nvim)) {
	go func() {
		m.mu.Lock()
		n := m.nvim
		m.mu.Unlock()
		if n == nil {
			return
		}
		f(n)
	}()
}

func (m *MiniMap) asyncWSNvim(f func(n *nvim.Nvim)) {
	go func() {
		ws := m.ws
		if ws == nil || ws.nvim == nil {
			return
		}
		f(ws.nvim)
	}()
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
		select {
		case m.redrawUpdates <- updates:
		default:
		}
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

	if err := m.nvim.AttachUI(m.cols, m.rows, m.attachUIOption()); err != nil {
		editor.putLog("[minimap] AttachUI error:", err)
	}
	m.uiAttached = true
	m.updateSize()

	ws := m.ws
	mini := m.nvim
	go func(ws *Workspace, mini *nvim.Nvim) {
		if ws == nil || ws.nvim == nil || mini == nil {
			return
		}

		wsRtp, _ := ws.nvim.CommandOutput(`echo &runtimepath`)
		wsPp, _ := ws.nvim.CommandOutput(`echo &packpath`)
		wsData, _ := ws.nvim.CommandOutput(`echo stdpath('data')`)
		wsRtp = strings.TrimSpace(wsRtp)
		wsPp = strings.TrimSpace(wsPp)
		wsData = strings.TrimSpace(wsData)

		if wsRtp != "" {
			_ = mini.Command(fmt.Sprintf(`lua do local v=%q; local t=vim.split(v,","); vim.opt.runtimepath = t end`, wsRtp))
		}
		if wsPp != "" {
			_ = mini.Command(fmt.Sprintf(`lua do local v=%q; local t=vim.split(v,","); vim.opt.packpath = t end`, wsPp))
		}

		_ = mini.Command(fmt.Sprintf(`silent! lua pcall(function()
	  local ok, cfg = pcall(require, 'nvim-treesitter.configs')
	  if ok and cfg and cfg.setup then
	    cfg.setup({
	      parser_install_dir = %q .. "/site",
	      highlight = { enable = true, additional_vim_regex_highlighting = false },
	    })
	    local site = %q .. "/site"
	    if vim.fn.isdirectory(site) == 1 then
	      vim.opt.runtimepath:append(site)
	    end
	  end
	end)`, wsData, wsData))

		rtpCount, _ := mini.CommandOutput(`echo len(split(&runtimepath, ","))`)
		ppCount, _ := mini.CommandOutput(`echo len(split(&packpath, ","))`)
		editor.putLog("[minimap] &rtp set from WS, len=", strings.TrimSpace(rtpCount))
		editor.putLog("[minimap] &pp  set from WS, len=", strings.TrimSpace(ppCount))

		_ = mini.Command("silent! set nowrap")
		_ = mini.Command("silent! syntax on")
		_ = mini.Command("silent! set laststatus=0")
		_ = mini.Command("silent! set noshowmode")
		_ = mini.Command("silent! set noruler")
	}(ws, mini)
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
	cols := int(float64(m.width) / minimapScaleValue / fontWidth)
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

	if !m.visible {
		m.widget.Hide()
		m.mu.Unlock()
		return
	}
	if m.ws == nil || m.ws.nvim == nil || m.nvim == nil {
		m.mu.Unlock()
		return
	}

	m.setColorscheme()
	m.widget.Show()

	ws := m.ws
	wsNvim := ws.nvim
	wsPath := ws.filepath

	var (
		newKey   string
		filetype string
	)

	if wsPath == "" {
		if buf, err := wsNvim.CurrentBuffer(); err == nil {
			newKey = fmt.Sprintf("__unnamed__:%d", int(buf))
		} else {
			m.mu.Unlock()
			m.mapScroll()
			return
		}
		if out, err := wsNvim.CommandOutput("echo &filetype"); err == nil {
			filetype = strings.TrimSpace(out)
		}
	} else {
		newKey = wsPath
	}

	if newKey == "" {
		m.mu.Unlock()
		m.mapScroll()
		return
	}

	changed := (newKey != m.currBuf)
	m.currBuf = newKey
	m.mu.Unlock()

	if !changed {
		m.mapScroll()
		return
	}

	if wsPath == "" {
		ft := filetype
		m.asyncMiniNvim(func(n *nvim.Nvim) {
			_ = n.Command("enew!")
			_ = n.Command("silent! filetype plugin indent on")
			if ft != "" {
				_ = n.Command("silent! setlocal filetype=" + ft)
			}

			// tree-sitter attach
			_ = n.Command(`silent! lua pcall(function()
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
		})
	} else {
		currBuf := wsPath
		m.asyncMiniNvim(func(n *nvim.Nvim) {
			_ = n.Command(":e! " + currBuf)
			_ = n.Command("silent! filetype plugin indent on")
			_ = n.Command("silent! filetype detect")

			// tree-sitter attach
			_ = n.Command(`silent! lua pcall(function()
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
		})
	}

	m.bufSync()
	m.mapScroll()
}

func (m *MiniMap) setColorscheme() {
	if m.ws.colorscheme == "" {
		m.ws.getColorscheme()
	}
	colo := m.ws.colorscheme
	m.colorscheme = colo

	m.asyncMiniNvim(func(n *nvim.Nvim) {
		_ = n.Command("silent! colorscheme " + colo)
		editor.putLog("[minimap] coloscheme is ", colo)
	})

	if mmWin, ok := m.getWindow(1); ok {
		mmWin.refreshUpdateArea(1)
		mmWin.update()
	}
}

func (m *MiniMap) attachTreesitterForCurrentBuffer() {
	m.asyncMiniNvim(func(n *nvim.Nvim) {
		_ = n.Command(`silent! lua pcall(function()
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
	})
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

	oldTop := oldPos
	oldBottom := oldPos + oldHeight
	newTop := m.curPos
	newBottom := m.curPos + m.curHeight

	if oldHeight == 0 {
		oldTop = newTop
		oldBottom = newBottom
	}

	top := oldTop
	if newTop < top {
		top = newTop
	}
	bottom := oldBottom
	if newBottom > bottom {
		bottom = newBottom
	}

	padLines := 1
	pad := padLines * m.font.lineHeight

	top -= pad
	if top < 0 {
		top = 0
	}
	widgetHeight := m.widget.Height()
	bottom += pad
	if bottom > widgetHeight {
		bottom = widgetHeight
	}

	height := bottom - top
	if height < 0 {
		height = 0
	}

	mmWin.refreshUpdateArea(1)

	width := m.widget.Width()
	mmWin.Update2(0, top, width, height)
}

func (m *MiniMap) bufSync() {
	m.mu.Lock()
	if m.isProcessSync {
		m.mu.Unlock()
		return
	}
	m.isProcessSync = true

	visible := m.visible
	wsNvim := m.ws.nvim
	miniNvim := m.nvim
	m.mu.Unlock()

	if !visible {
		m.widget.Hide()
		m.mu.Lock()
		m.isProcessSync = false
		m.mu.Unlock()
		return
	}
	if wsNvim == nil || miniNvim == nil {
		m.mu.Lock()
		m.isProcessSync = false
		m.mu.Unlock()
		return
	}

	go func(wsNvim, miniNvim *nvim.Nvim) {
		defer func() {
			m.mu.Lock()
			m.isProcessSync = false
			m.mu.Unlock()
		}()

		win, err := wsNvim.CurrentWindow()
		if err != nil {
			return
		}
		config, err := wsNvim.WindowConfig(win)
		if err != nil {
			return
		}
		if isWindowFloatForConfig(config) {
			return
		}

		buf, err := wsNvim.CurrentBuffer()
		if err != nil {
			return
		}

		start := 0
		end := 0
		var pos [4]int
		wsNvim.Eval("getpos('$')", &pos)
		var minimapPos [4]int
		miniNvim.Eval("getpos('$')", &minimapPos)
		if pos[1] > minimapPos[1] {
			end = pos[1] + 1
		} else {
			end = minimapPos[1] + 1
		}

		replacement, err := wsNvim.BufferLines(buf, start, end, false)
		if err != nil {
			return
		}
		if len(replacement) < 1 {
			return
		}

		minimapBuf, err := miniNvim.CurrentBuffer()
		if err != nil {
			return
		}

		_ = miniNvim.SetBufferLines(minimapBuf, start, end, false, replacement)
	}(wsNvim, miniNvim)
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

	delta := vert
	acc := accel
	n := m.nvim

	go func(delta, accel int, n *nvim.Nvim) {
		if n == nil {
			return
		}
		if delta > 0 {
			_, _ = n.Input(fmt.Sprintf("%v<C-y>", accel))
		} else if delta < 0 {
			_, _ = n.Input(fmt.Sprintf("%v<C-e>", accel))
		}
	}(delta, acc, n)

	event.Accept()
}

func (m *MiniMap) mouseEvent(event *gui.QMouseEvent) {
	font := m.font
	y := int(float64(event.Y()) / float64(font.lineHeight))
	targetPos := m.viewport[0] + y

	m.asyncWSNvim(func(n *nvim.Nvim) {
		_ = n.Command(fmt.Sprintf("%d", targetPos))
	})

	m.asyncWSNvim(func(n *nvim.Nvim) {
		mappings, err := n.KeyMap("normal")
		if err != nil {
			return
		}
		var isThereZzMap bool
		for _, mapping := range mappings {
			if mapping.LHS == "zz" {
				isThereZzMap = true
				break
			}
		}
		if !isThereZzMap {
			_, _ = n.Input("zz")
		}
	})
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

			runStart := -1
			width := 0

			for i, c := range text {
				if c == '@' {
					if width == 0 {
						runStart = i
					}
					width++
				}
				if c == ' ' || i == len(text)-1 {
					if width > 0 {
						startCol := col + runStart
						path := gui.NewQPainterPath()
						path.AddRoundedRect2(
							float64(startCol)*wsfont.cellwidth*minimapScaleValue,
							float64(y*wsfont.lineHeight+wsfont.baselineOffset),
							float64(width)*wsfont.cellwidth*minimapScaleValue,
							float64(wsfont.lineHeight)*minimapScaleValue,
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
	col := m.currentRegionColor()
	if col == nil {
		return
	}

	curRegionRect := core.NewQRectF4(
		0,
		float64(m.curPos),
		float64(m.widget.Width()),
		float64(m.curHeight),
	)

	p.FillRect(
		curRegionRect,
		gui.NewQBrush3(
			col.QColor(),
			core.Qt__SolidPattern,
		),
	)
}

func (m *MiniMap) currentRegionColor() *RGBA {
	alpha := 0.25
	if m.ws != nil && m.ws.background != nil {
		return m.ws.background.OverlayAccent(alpha)
	}

	if editor.colors.bg != nil {
		return editor.colors.bg.OverlayAccent(alpha)
	}

	// どちらも取れない場合の保険
	return &RGBA{R: 128, G: 128, B: 128, A: alpha}
}
