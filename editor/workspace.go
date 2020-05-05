package editor

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akiyosi/goneovim/filer"
	"github.com/akiyosi/goneovim/fuzzy"
	"github.com/akiyosi/goneovim/util"
	shortpath "github.com/akiyosi/short_path"
	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

type workspaceSignal struct {
	core.QObject
	_ func() `signal:"markdownSignal"`
	_ func() `signal:"stopSignal"`
	_ func() `signal:"redrawSignal"`
	_ func() `signal:"guiSignal"`
	_ func() `signal:"statuslineSignal"`
	_ func() `signal:"locpopupSignal"`
	_ func() `signal:"lintSignal"`
	_ func() `signal:"gitSignal"`
	_ func() `signal:"messageSignal"`
}

// Workspace is an editor workspace
type Workspace struct {
	widget     *widgets.QWidget
	font       *Font
	fontwide   *Font
	cursor     *Cursor
	tabline    *Tabline
	statusline *Statusline
	screen     *Screen
	scrollBar  *ScrollBar
	markdown   *Markdown
	finder     *Finder
	palette    *Palette
	fpalette   *Palette
	popup      *PopupMenu
	loc        *Locpopup
	cmdline    *Cmdline
	signature  *Signature
	message    *Message
	minimap    *MiniMap

	width  int
	height int
	hidden bool

	nvim               *nvim.Nvim
	rows               int
	cols               int
	uiAttached         bool
	uiRemoteAttached   bool
	screenbg           string
	colorscheme        string
	foreground         *RGBA
	background         *RGBA
	special            *RGBA
	mode               string
	modeIdx            int
	filepath           string
	cwd                string
	cwdBase            string
	cwdlabel           string
	maxLine            int
	curLine            int
	curColm            int
	curPosMutex        sync.RWMutex
	cursorStyleEnabled bool
	modeInfo           []map[string]interface{}
	normalMappings     []*nvim.Mapping
	insertMappings     []*nvim.Mapping
	ts                 int

	escKeyInNormal     string
	escKeyInInsert     string
	isMappingScrollKey bool

	signal        *workspaceSignal
	redrawUpdates chan [][]interface{}
	guiUpdates    chan []interface{}
	doneNvimStart chan bool
	stopOnce      sync.Once
	stop          chan struct{}
	fontMutex     sync.Mutex

	drawStatusline bool
	drawTabline    bool
	drawLint       bool
}

func newWorkspace(path string) (*Workspace, error) {
	w := &Workspace{
		stop:          make(chan struct{}),
		signal:        NewWorkspaceSignal(nil),
		redrawUpdates: make(chan [][]interface{}, 1000),
		guiUpdates:    make(chan []interface{}, 1000),
		doneNvimStart: make(chan bool, 1000),
		foreground:    newRGBA(180, 185, 190, 1),
		background:    newRGBA(9, 13, 17, 1),
		special:       newRGBA(255, 255, 255, 1),
	}
	w.font = initFontNew(editor.extFontFamily, float64(editor.extFontSize), editor.config.Editor.Linespace, true)
	go func() {
		w.fontMutex.Lock()
		defer w.fontMutex.Unlock()
		width, height, truewidth, ascent, italicWidth := fontSizeNew(w.font.fontNew)
		w.font.width = width
		w.font.height = height
		w.font.truewidth = truewidth
		w.font.lineHeight = height + w.font.lineSpace
		w.font.ascent = ascent
		w.font.italicWidth = italicWidth
	}()
	w.font.ws = w

	w.cols = int(float64(editor.config.Editor.Width) / w.font.truewidth)
	w.rows = editor.config.Editor.Height / w.font.lineHeight

	// Basic Workspace UI component
	w.tabline = initTabline()
	w.tabline.ws = w
	w.statusline = initStatusline()
	w.statusline.ws = w
	w.loc = initLocpopup()
	w.loc.ws = w
	w.message = initMessage()
	w.message.ws = w
	w.palette = initPalette()
	w.palette.ws = w
	w.fpalette = initPalette()
	w.fpalette.ws = w

	go w.startNvim(path)
	w.registerSignal()

	w.screen = newScreen()
	w.screen.ws = w
	w.screen.font = w.font
	w.screen.initInputMethodWidget()

	w.loc.widget.SetParent(editor.wsWidget)
	w.message.widget.SetParent(editor.window)
	w.palette.widget.SetParent(editor.window)
	w.fpalette.widget.SetParent(editor.window)

	w.scrollBar = newScrollBar()
	w.scrollBar.ws = w
	w.markdown = newMarkdown(w)
	w.markdown.webview.SetParent(w.screen.widget)
	w.cursor = initCursorNew()
	w.cursor.ws = w
	w.popup = initPopupmenuNew()
	w.popup.widget.SetParent(editor.wsWidget)
	w.popup.ws = w
	w.finder = initFinder()
	w.finder.ws = w
	w.signature = initSignature()
	w.signature.widget.SetParent(editor.wsWidget)
	w.signature.ws = w
	w.cmdline = initCmdline()
	w.cmdline.ws = w
	w.minimap = newMiniMap()
	w.minimap.ws = w

	layout := widgets.NewQVBoxLayout()
	w.widget = widgets.NewQWidget(nil, 0)
	w.widget.SetContentsMargins(0, 0, 0, 0)
	w.widget.SetLayout(layout)
	w.widget.SetFocusPolicy(core.Qt__WheelFocus)
	w.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	w.widget.ConnectInputMethodEvent(w.InputMethodEvent)
	w.widget.ConnectInputMethodQuery(w.InputMethodQuery)

	// screen widget and scrollBar widget
	scrWidget := widgets.NewQWidget(nil, 0)
	scrWidget.SetContentsMargins(0, 0, 0, 0)
	scrWidget.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	scrLayout := widgets.NewQHBoxLayout()
	scrLayout.SetContentsMargins(0, 0, 0, 0)
	scrLayout.SetSpacing(0)
	scrLayout.AddWidget(w.screen.widget, 0, 0)
	scrLayout.AddWidget(w.minimap.widget, 0, 0)
	scrLayout.AddWidget(w.scrollBar.widget, 0, 0)
	scrWidget.SetLayout(scrLayout)

	layout.AddWidget(w.tabline.widget, 0, 0)
	layout.AddWidget(scrWidget, 1, 0)
	layout.AddWidget(w.statusline.widget, 0, 0)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(0)

	w.popup.widget.Hide()
	w.palette.hide()
	w.fpalette.hide()
	w.loc.widget.Hide()
	w.signature.widget.Hide()

	w.widget.SetParent(editor.wsWidget)
	w.widget.Move2(0, 0)
	w.updateSize()

	if !w.uiRemoteAttached && !editor.config.MiniMap.Disable {
		go func() {
			if !editor.config.MiniMap.Visible {
				time.Sleep(1500 * time.Millisecond)
			}
			w.minimap.startMinimapProc()
		}()
	}

	if runtime.GOOS == "windows" {
		<-w.doneNvimStart
	}

	return w, nil
}

func (w *Workspace) registerSignal() {
	w.signal.ConnectRedrawSignal(func() {
		updates := <-w.redrawUpdates
		w.handleRedraw(updates)
	})
	w.signal.ConnectGuiSignal(func() {
		updates := <-w.guiUpdates
		w.handleRPCGui(updates)
	})
	w.signal.ConnectStopSignal(func() {
		if !w.uiRemoteAttached {
			if !editor.config.MiniMap.Disable {
				editor.workspaces[editor.active].minimap.exit()
			}
		}
		workspaces := []*Workspace{}
		index := 0
		for i, ws := range editor.workspaces {
			if ws != w {
				workspaces = append(workspaces, ws)
			} else {
				index = i
			}
		}
		if len(workspaces) == 0 {
			editor.close()
			return
		}
		for i := 0; i <= len(editor.wsSide.items) && i <= len(editor.workspaces); i++ {
			if i >= index {
				editor.wsSide.items[i].cwdpath = editor.wsSide.items[i+1].cwdpath
			}
		}
		editor.workspaces = workspaces
		w.hide()
		if editor.active == index {
			if index > 0 {
				editor.active--
			}
			editor.workspaceUpdate()
		}
	})
}

func (w *Workspace) hide() {
	if w.hidden {
		return
	}
	w.hidden = true
	w.widget.Hide()
}

func (w *Workspace) show() {
	if !w.hidden {
		return
	}
	w.hidden = false
	w.widget.Show()
	w.widget.SetFocus2Default()
}

func (w *Workspace) startNvim(path string) error {
	var neovim *nvim.Nvim
	var err error

	childProcessArgs := nvim.ChildProcessArgs(
		append([]string{
			"--cmd",
			"let g:gonvim_running=1",
			"--cmd",
			"set termguicolors",
			"--embed",
		}, editor.args...)...,
	)
	if editor.opts.Server != "" {
		// Attaching to remote nvim session
		neovim, err = nvim.Dial(editor.opts.Server)
		w.uiRemoteAttached = true
	} else if editor.opts.Nvim != "" {
		// Attaching to /path/to/nvim
		childProcessCmd := nvim.ChildProcessCommand(editor.opts.Nvim)
		neovim, err = nvim.NewChildProcess(childProcessArgs, childProcessCmd)
	} else {
		// Attaching to nvim normaly
		neovim, err = nvim.NewChildProcess(childProcessArgs)
	}
	if err != nil {
		return err
	}
	w.nvim = neovim
	w.nvim.RegisterHandler("Gui", func(updates ...interface{}) {
		w.guiUpdates <- updates
		w.signal.GuiSignal()
	})
	w.nvim.RegisterHandler("redraw", func(updates ...[]interface{}) {
		w.redrawUpdates <- updates
		w.signal.RedrawSignal()
	})

	go func() {
		err := w.nvim.Serve()
		if err != nil {
			fmt.Println(err)
		}
		w.stopOnce.Do(func() {
			close(w.stop)
		})
		w.signal.StopSignal()
	}()

	go w.init(path)

	if runtime.GOOS == "windows" {
		w.doneNvimStart <- true
	}

	return nil
}

func (w *Workspace) init(path string) {
	w.configure()
	w.attachUI(path)
	w.loadGinitVim()
	w.getNvimOptions()
}

func (w *Workspace) configure() {
	w.drawStatusline = editor.config.Statusline.Visible

	if editor.config.Tabline.Visible && editor.config.Editor.ExtTabline {
		w.drawTabline = true
	} else {
		w.drawTabline = false
	}

	if editor.config.Lint.Visible {
		w.drawLint = true
	} else {
		w.drawLint = false
	}
}

func (w *Workspace) attachUI(path string) error {
	w.nvim.Subscribe("Gui")
	go w.initGonvim()
	w.tabline.subscribe()
	w.statusline.subscribe()
	// w.loc.subscribe()
	w.message.subscribe()

	// Add editor feature
	fuzzy.RegisterPlugin(w.nvim, w.uiRemoteAttached)
	filer.RegisterPlugin(w.nvim)

	w.uiAttached = true
	err := w.nvim.AttachUI(w.cols, w.rows, w.attachUIOption())
	if err != nil {
		fmt.Println(err)
		editor.close()
		return err
	}
	if path != "" {
		go w.nvim.Command("so " + path)
	}

	return nil
}

func (w *Workspace) initGonvim() {
	gonvimAutoCmds := `
	aug GonvimAu | au! | aug END
	au GonvimAu VimEnter * call rpcnotify(1, "Gui", "gonvim_enter", getcwd())
	au GonvimAu TermEnter * call rpcnotify(0, "Gui", "gonvim_termenter")
	au GonvimAu TermLeave * call rpcnotify(0, "Gui", "gonvim_termleave")
	aug GonvimAuWorkspace | au! | aug END
	au GonvimAuWorkspace DirChanged * call rpcnotify(0, "Gui", "gonvim_workspace_cwd", getcwd())
	aug GonvimAuFilepath | au! | aug END
	au GonvimAuFilepath BufEnter,TabEnter,DirChanged,TermOpen,TermClose * silent call rpcnotify(0, "Gui", "gonvim_workspace_filepath", expand("%:p"))
	aug GonvimAuMd | au! | aug END
	au GonvimAuMd TextChanged,TextChangedI *.md call rpcnotify(0, "Gui", "gonvim_markdown_update")
	au GonvimAuMd BufEnter *.md call rpcnotify(0, "Gui", "gonvim_markdown_new_buffer")
	`
	if !w.uiRemoteAttached {
		gonvimAutoCmds = gonvimAutoCmds + `
		aug GonvimAuMinimap | au! | aug END
		au GonvimAuMinimap BufEnter,BufWrite * call rpcnotify(0, "Gui", "gonvim_minimap_update")
		aug GonvimAuMinimapSync | au! | aug END
		au GonvimAuMinimapSync TextChanged,TextChangedI * call rpcnotify(0, "Gui", "gonvim_minimap_sync")
		`
	}

	if editor.config.ScrollBar.Visible {
		gonvimAutoCmds = gonvimAutoCmds + `
	aug GonvimAuScrollbar | au! | aug END
	au GonvimAuScrollbar TextChanged,TextChangedI,BufReadPost * call rpcnotify(0, "Gui", "gonvim_get_maxline", line("$"))
	`
	}
	if editor.config.Editor.Clipboard {
		gonvimAutoCmds = gonvimAutoCmds + `
	aug GonvimAuClipboard | au! | aug END
	au GonvimAuClipboard TextYankPost * call rpcnotify(0, "Gui", "gonvim_copy_clipboard")
	`
	}
	if editor.config.Statusline.Visible {
		gonvimAutoCmds = gonvimAutoCmds + `
	aug GonvimAuStatusline | au! | aug END
	au GonvimAuStatusline BufEnter,TermOpen,TermClose * call rpcnotify(0, "statusline", "bufenter", &filetype, &fileencoding, &fileformat, &ro)
	`
	}

	registerScripts := fmt.Sprintf(`call execute(%s)`, util.SplitVimscript(gonvimAutoCmds))
	w.nvim.Command(registerScripts)

	gonvimCommands := fmt.Sprintf(`
	command! -nargs=1 GonvimResize call rpcnotify(0, "Gui", "gonvim_resize", <args>)
	command! GonvimSidebarShow call rpcnotify(0, "Gui", "side_open")
	command! GonvimMarkdown call rpcnotify(0, "Gui", "gonvim_markdown_toggle")
	command! GonvimVersion echo "%s"`, editor.version)
	if !w.uiRemoteAttached {
		if !editor.config.MiniMap.Disable {
		gonvimCommands = gonvimCommands + `
		command! GonvimMiniMap call rpcnotify(0, "Gui", "gonvim_minimap_toggle")
		`
		}
		gonvimCommands = gonvimCommands + `
	command! GonvimWorkspaceNew call rpcnotify(0, "Gui", "gonvim_workspace_new")
	command! GonvimWorkspaceNext call rpcnotify(0, "Gui", "gonvim_workspace_next")
	command! GonvimWorkspacePrevious call rpcnotify(0, "Gui", "gonvim_workspace_previous")
	command! -nargs=1 GonvimWorkspaceSwitch call rpcnotify(0, "Gui", "gonvim_workspace_switch", <args>)
	command! -nargs=1 GonvimGridFont call rpcnotify(0, "Gui", "gonvim_grid_font", <args>)
	`
	}
	if runtime.GOOS == "darwin" {
		gonvimCommands = gonvimCommands + `
		command! GonvimMaximize call rpcnotify(0, "Gui", "gonvim_maximize")
	`
	}
	registerScripts = fmt.Sprintf(`call execute(%s)`, util.SplitVimscript(gonvimCommands))
	w.nvim.Command(registerScripts)

	gonvimInitNotify := `
	call rpcnotify(0, "statusline", "bufenter", expand("%:p"), &filetype, &fileencoding, &fileformat, &ro)
	`
	if !w.uiRemoteAttached {
		gonvimInitNotify = gonvimInitNotify + `
		call rpcnotify(0, "Gui", "gonvim_minimap_update")
		`
	}
	initialNotify := fmt.Sprintf(`call execute(%s)`, util.SplitVimscript(gonvimInitNotify))
	w.nvim.Command(initialNotify)
}

func (w *Workspace) loadGinitVim() {
	if editor.config.Editor.GinitVim != "" {
		scripts := strings.NewReplacer("\r\n", "\n", "\r", "\n", "\n", "\n").Replace(editor.config.Editor.GinitVim)
		execGinitVim := fmt.Sprintf(`call execute(split('%s', '\n'))`, scripts)
		w.nvim.Command(execGinitVim)
	}
}

func (w *Workspace) getNvimOptions() {
	ts := 8
	w.nvim.Option("ts", &ts)
	w.ts = ts
	w.getColorscheme()
	screenbg := ""
	w.nvim.Eval(":echo &background", &screenbg)
	w.screenbg = screenbg
	if w.screenbg == "light" {
		fg := newRGBA(editor.colors.fg.R, editor.colors.fg.G, editor.colors.fg.B, 1)
		bg := newRGBA(editor.colors.bg.R, editor.colors.bg.G, editor.colors.bg.B, 1)
		editor.colors.fg = bg
		editor.colors.bg = fg
	}

	w.escKeyInInsert = "<Esc>"
	w.escKeyInNormal = "<Esc>"
	nmappings, err := w.nvim.KeyMap("normal")
	if err != nil {
		return
	}
	w.normalMappings = nmappings
	imappings, err := w.nvim.KeyMap("insert")
	if err != nil {
		return
	}
	w.insertMappings = imappings
	altkeyCount := 0
	metakeyCount := 0
	for _, mapping := range w.insertMappings {
		// Check Esc mapping
		if strings.EqualFold(mapping.RHS, "<Esc>") || strings.EqualFold(mapping.RHS, "<C-[>") {
			if mapping.NoRemap == 1 {
				w.escKeyInInsert = mapping.LHS
			}
		}
		// Count user def alt/meta key mappings
		if strings.HasPrefix(mapping.LHS, "<A-") {
			altkeyCount++
		}
		if strings.HasPrefix(mapping.LHS, "<M-") {
			metakeyCount++
		}
	}
	for _, mapping := range w.normalMappings {
		if strings.EqualFold(mapping.RHS, "<Esc>") || strings.EqualFold(mapping.RHS, "<C-[>") {
			if mapping.NoRemap == 1 {
				w.escKeyInNormal = mapping.LHS
			}
		}
		if strings.EqualFold(mapping.LHS, "<C-y>") || strings.EqualFold(mapping.LHS, "<C-e>"){
			w.isMappingScrollKey = true
		}
		// Count user def alt/meta key mappings
		if strings.HasPrefix(mapping.LHS, "<A-") {
			altkeyCount++
		}
		if strings.HasPrefix(mapping.LHS, "<M-") {
			metakeyCount++
		}
	}
	if altkeyCount >= metakeyCount {
		editor.prefixToMapMetaKey = "A-"
	} else {
		editor.prefixToMapMetaKey = "M-"
	}
}

func (w *Workspace) getColorscheme() {
	colorscheme := ""
	w.nvim.Var("colors_name", &colorscheme)
	w.colorscheme = colorscheme
}

func (w *Workspace) nvimEval(s string) (interface{}, error) {
	doneChannel := make(chan interface{}, 5)
	var result interface{}
	go func() {
		w.nvim.Eval(s, &result)
		doneChannel <- result
	}()
	select {
	case done := <-doneChannel:
		return done, nil
	case <-time.After(40 * time.Millisecond):
		err := errors.New("neovim busy")
		return nil, err
	}
}

func (w *Workspace) setCwd(cwd string) {
	w.cwd = cwd
	if editor.wsSide == nil {
		return
	}

	var labelpath string
	switch editor.config.Workspace.PathStyle {
	case "name":
		labelpath = filepath.Base(cwd)
	case "minimum":
		labelpath, _ = shortpath.Minimum(cwd)
	case "full":
		labelpath, _ = filepath.Abs(cwd)
	default:
		labelpath, _ = filepath.Abs(cwd)
	}
	w.cwdlabel = labelpath
	w.cwdBase = filepath.Base(cwd)
	for i, ws := range editor.workspaces {
		if i >= len(editor.wsSide.items) {
			return
		}

		if ws == w {
			path, _ := filepath.Abs(cwd)
			sideItem := editor.wsSide.items[i]
			if sideItem.cwdpath == path {
				continue
			}

			sideItem.label.SetText(w.cwdlabel)
			sideItem.label.SetFont(gui.NewQFont2(editor.extFontFamily, editor.extFontSize-1, 1, false))
			sideItem.cwdpath = path
		}
	}
}

func (w *Workspace) attachUIOption() map[string]interface{} {
	o := make(map[string]interface{})
	o["rgb"] = true
	// o["ext_multigrid"] = editor.config.Editor.ExtMultigrid
	o["ext_multigrid"] = true
	o["ext_hlstate"] = true

	apiInfo, err := w.nvim.APIInfo()
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
					name, ok := function["name"]
					if !ok {
						continue
					}

					switch name {
					// case "wildmenu_show" :
					// 	o["ext_wildmenu"] = editor.config.Editor.ExtCmdline
					case "cmdline_show":
						o["ext_cmdline"] = editor.config.Editor.ExtCmdline
					case "msg_show":
						o["ext_messages"] = editor.config.Editor.ExtMessages
					case "popupmenu_show":
						o["ext_popupmenu"] = editor.config.Editor.ExtPopupmenu
					case "tabline_update":
						o["ext_tabline"] = editor.config.Editor.ExtTabline
					}
				}
			}
		}
	}
	return o
}

func (w *Workspace) updateSize() {
	e := editor
	width := e.wsWidget.Width()
	height := e.wsWidget.Height()
	if width != w.width || height != w.height {
		w.width = width
		w.height = height
		w.widget.Resize2(width, height)
		if !w.hidden {
			w.hide()
			w.show()
		} else {
			w.show()
			w.hide()
		}
	}

	if w.drawTabline {
		w.tabline.height = w.tabline.widget.Height()
	}
	if w.drawStatusline {
		w.statusline.height = w.statusline.widget.Height()
	}

	if w.screen != nil {
		w.screen.height = w.height - w.tabline.height - w.statusline.height
		w.screen.updateSize()
	}
	if w.palette != nil {
		w.palette.resize()
	}
	if w.fpalette != nil {
		w.fpalette.resize()
	}
	if w.message != nil {
		w.message.resize()
	}

	// notification
	e.updateNotificationPos()
}

func (e *Editor) updateNotificationPos() {
	e.width = e.window.Width()
	e.height = e.window.Height()
	e.notifyStartPos = core.NewQPoint2(e.width-e.notificationWidth-10, e.height-30)
	var x, y int
	var newNotifications []*Notification
	for _, item := range e.notifications {
		x = e.notifyStartPos.X()
		y = e.notifyStartPos.Y() - item.widget.Height() - 4
		if !item.isHide && !item.isMoved {
			item.widget.Move2(x, y)
			e.notifyStartPos = core.NewQPoint2(x, y)
		}
		newNotifications = append(newNotifications, item)
	}
	e.notifications = newNotifications
}

func (w *Workspace) handleRedraw(updates [][]interface{}) {
	s := w.screen
	for _, update := range updates {
		event := update[0].(string)
		args := update[1:]
		switch event {

		// Global Events
		case "set_title":
			titleStr := (update[1].([]interface{}))[0].(string)
			editor.window.SetupTitle(titleStr)
			if runtime.GOOS == "linux" {
				editor.window.SetWindowTitle(titleStr)
			}
		case "set_icon":
		case "mode_info_set":
			w.modeInfoSet(args)
			w.cursor.modeIdx = 0
			w.cursor.update()
		case "option_set":
			w.setOption(update)
		case "mode_change":
			arg := update[len(update)-1].([]interface{})
			w.mode = arg[0].(string)
			w.modeIdx = util.ReflectToInt(arg[1])
			if w.cursor.modeIdx != w.modeIdx {
				w.cursor.modeIdx = w.modeIdx
				w.cursor.update()
			}
			w.disableImeInNormal()
		case "mouse_on":
		case "mouse_off":
		case "busy_start":
		case "busy_stop":
		case "suspend":
		case "update_menu":
		case "bell":
		case "visual_bell":
		case "flush":
			w.cursor.update()

		// Grid Events
		case "grid_resize":
			s.gridResize(args)
		case "default_colors_set":
			for _, u := range update[1:] {
				w.setColorsSet(u.([]interface{}))
			}
		case "hl_attr_define":
			s.setHlAttrDef(args)
		case "hl_group_set":
			s.setHighlightGroup(args)
		case "grid_line":
			s.gridLine(args)
		case "grid_clear":
			s.gridClear(args)
		case "grid_destroy":
			s.gridDestroy(args)
		case "grid_cursor_goto":
			s.gridCursorGoto(args)
		case "grid_scroll":
			s.gridScroll(args)

		// Multigrid Events
		case "win_pos":
			s.windowPosition(args)
			s.setBufferNames()
		case "win_float_pos":
			s.windowFloatPosition(args)
		case "win_external_pos":
		case "win_hide":
			s.windowHide(args)
		case "win_scroll_over_start":
			// old impl
			// s.windowScrollOverStart()
		case "win_scroll_over_reset":
			// old impl
			// s.windowScrollOverReset()
		case "win_close":
			s.windowClose()
		case "msg_set_pos":
			s.msgSetPos(args)
		// case "win_viewport":

		// Popupmenu Events
		case "popupmenu_show":
			if w.cmdline.shown {
				w.cmdline.cmdWildmenuShow(args)
			} else {
				w.popup.showItems(args)
			}
		case "popupmenu_select":
			if w.cmdline.shown {
				w.cmdline.cmdWildmenuSelect(args)
			} else {
				w.popup.selectItem(args)
			}
		case "popupmenu_hide":
			if w.cmdline.shown {
				w.cmdline.cmdWildmenuHide()
			} else {
				w.popup.hide()
			}

		// Tabline Events
		case "tabline_update":
			w.tabline.update(args)

		// Cmdline Events
		case "cmdline_show":
			w.cmdline.show(args)
		case "cmdline_pos":
			w.cmdline.changePos(args)
		case "cmdline_special_char":
		case "cmdline_char":
			w.cmdline.putChar(args)
		case "cmdline_hide":
			w.cmdline.hide(args)
		case "cmdline_function_show":
			w.cmdline.functionShow()
		case "cmdline_function_hide":
			w.cmdline.functionHide()
		case "cmdline_block_show":
		case "cmdline_block_append":
		case "cmdline_block_hide":

		// // -- deprecated events
		// case "wildmenu_show":
		// 	w.cmdline.wildmenuShow(args)
		// case "wildmenu_select":
		// 	w.cmdline.wildmenuSelect(args)
		// case "wildmenu_hide":
		// 	w.cmdline.wildmenuHide()

		// Message/Dialog Events
		case "msg_show":
			w.message.msgShow(args)
		case "msg_clear":
			w.message.msgClear()
		case "msg_showmode":
		case "msg_showcmd":
		case "msg_ruler":
		case "msg_history_show":
			w.message.msgHistoryShow(args)

		default:
		}
	}

	s.update()
	w.drawOtherUI()
}

func (w *Workspace) drawOtherUI() {
	s := w.screen

	if w.minimap.visible || w.drawStatusline || editor.config.ScrollBar.Visible {
		w.getPos()
	}

	if w.drawStatusline {
		w.statusline.pos.redraw(w.curLine, w.curColm)
		w.statusline.mode.redraw()
	}

	if editor.config.ScrollBar.Visible {
		w.scrollBar.update()
	}

	if s.tooltip.IsVisible() {
		x, y, _, _ := w.screen.toolTipPos()
		w.screen.toolTipMove(x, y)
	}

	if w.minimap.visible {
		go w.updateMinimap()
		w.minimap.mapScroll()
	}
}

func (w *Workspace) disableImeInNormal() {
	if !editor.config.Editor.DisableImeInNormal {
		return
	}
	switch w.mode {
	case "insert":
		w.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
		editor.wsWidget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	case "cmdline_normal":
		w.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
		editor.wsWidget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	default:
		w.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, false)
		editor.wsWidget.SetAttribute(core.Qt__WA_InputMethodEnabled, false)
	}
}

func (w *Workspace) setColorsSet(args []interface{}) {
	fg := util.ReflectToInt(args[0])
	bg := util.ReflectToInt(args[1])
	sp := util.ReflectToInt(args[2])

	if fg != -1 {
		w.foreground.R = calcColor(fg).R
		w.foreground.G = calcColor(fg).G
		w.foreground.B = calcColor(fg).B
	}
	if bg != -1 {
		w.background.R = calcColor(bg).R
		w.background.G = calcColor(bg).G
		w.background.B = calcColor(bg).B
	}
	if sp != -1 {
		w.special.R = calcColor(sp).R
		w.special.G = calcColor(sp).G
		w.special.B = calcColor(sp).B
	}

	var isChangeFg, isChangeBg bool
	if editor.colors.fg != nil {
		isChangeFg = editor.colors.fg.equals(w.foreground)
	}
	if editor.colors.bg != nil {
		isChangeBg = editor.colors.bg.equals(w.background)
	}

	if !w.uiRemoteAttached {
		if !isChangeFg || !isChangeBg {
			editor.isSetGuiColor = false
			aw := editor.workspaces[editor.active]
			// change minimap colorscheme
			aw.minimap.isSetColorscheme = false
			if aw.minimap.visible && aw.minimap.nvim != nil && aw.nvim != nil {
				editor.workspaces[editor.active].minimap.setColorscheme()
			}
		}
	}
	if len(editor.workspaces) > 1 {
		w.updateWorkspaceColor()
		// Ignore setting GUI color when create second workspace and fg, bg equals -1
		if fg == -1 && bg == -1 {
			editor.isSetGuiColor = true
		}
	}

	// Exit if there si no change in foreground / background
	if editor.isSetGuiColor {
		return
	}

	editor.colors.fg = w.foreground.copy()
	editor.colors.bg = w.background.copy()
	// Reset hlAttrDef map 0 index:
	if w.screen.hlAttrDef != nil {
		w.screen.hlAttrDef[0] = &Highlight{
			foreground: editor.colors.fg,
			background: editor.colors.bg,
		}
	}

	editor.colors.update()
	if !(w.colorscheme == "" && fg == -1 && bg == -1 && w.screenbg == "dark") {
		editor.updateGUIColor()
	}
	editor.isSetGuiColor = true
}

func (w *Workspace) updateWorkspaceColor() {
	w.palette.setColor()
	w.fpalette.setColor()
	w.popup.setColor()
	w.signature.setColor()
	w.message.setColor()
	w.screen.setColor()
	if w.drawTabline {
		w.tabline.setColor()
	}
	if w.drawStatusline {
		w.statusline.setColor()
	}
	if editor.config.ScrollBar.Visible {
		w.scrollBar.setColor()
	}
	if editor.config.Lint.Visible {
		w.loc.setColor()
	}
	if editor.wsSide != nil {
		editor.wsSide.setColor()
	}
}

func (w *Workspace) modeInfoSet(args []interface{}) {
	for _, arg := range args {
		w.cursorStyleEnabled = arg.([]interface{})[0].(bool)
		modePropList := arg.([]interface{})[1].([]interface{})
		w.modeInfo = make([]map[string]interface{}, len(modePropList))
		w.cursor.isNeedUpdateModeInfo = true
		for i, modeProp := range modePropList {
			// Note: i is the index which given by the `mode_idx` of the `mode_change` event
			w.modeInfo[i] = modeProp.(map[string]interface{})
		}
	}
}

func (w *Workspace) setOption(update []interface{}) {
	for n, option := range update {
		if n == 0 {
			continue
		}
		key := (option.([]interface{}))[0].(string)
		val := (option.([]interface{}))[1]
		switch key {
		case "arabicshape":
		case "ambiwidth":
		case "emoji":
		case "guifont":
			w.guiFont(val.(string))
		case "guifontset":
		case "guifontwide":
			w.guiFontWide(val.(string))
		case "linespace":
			w.guiLinespace(val)
		case "pumblend":
			w.popup.setPumblend(val)
		case "showtabline":
		case "termguicolors":
		// case "ext_cmdline":
		// case "ext_hlstate":
		// case "ext_linegrid":
		// case "ext_messages":
		// case "ext_multigrid":
		// case "ext_popupmenu":
		// case "ext_tabline":
		// case "ext_termcolors":
		default:
		}
	}
}

func (w *Workspace) getPos() {
	done := make(chan error, 2000)
	var curPos [4]int
	go func() {
		err := w.nvim.ExecuteLua(`
			-- if vim.fn.has('nvim-0.5') == 1 then
			if vim.fn == nil then
			  return vim.api.nvim_eval('getpos(".")')
			else
			  return vim.fn.getpos('.')
			end
			`, &curPos)
		done <- err
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Millisecond):
		return
	}

	w.curLine = curPos[1]
	w.curColm = curPos[2]
}

func (w *Workspace) updateMinimap() {
	var absMapTop int
	var absMapBottom int
	w.minimap.nvim.Eval("line('w0')", &absMapTop)
	w.minimap.nvim.Eval("line('w$')", &absMapBottom)
	w.minimap.nvim.Command(fmt.Sprintf("call cursor(%d, %d)", w.curLine, 0))
	w.curPosMutex.RLock()
	defer w.curPosMutex.RUnlock()
	switch {
	case w.curLine >= absMapBottom:
		w.minimap.nvim.Input("<C-d>")
	case absMapTop >= w.curLine:
		w.minimap.nvim.Input("<C-u>")
	default:
	}
}

func (w *Workspace) handleRPCGui(updates []interface{}) {
	event := updates[0].(string)
	switch event {
	case "gonvim_enter":
		editor.window.SetWindowOpacity(1.0)
		w.setCwd(updates[1].(string))
	case "gonvim_resize":
		width, height := editor.setWindowSize(updates[1].(string))
		editor.window.Resize2(width, height)
	case "gonvim_maximize":
		editor.window.WindowMaximize()
	case "Font":
		w.guiFont(updates[1].(string))
	case "Linespace":
		w.guiLinespace(updates[1])
	case "finder_pattern":
		w.finder.showPattern(updates[1:])
	case "finder_pattern_pos":
		w.finder.cursorPos(updates[1:])
	case "finder_show_result":
		w.finder.showResult(updates[1:])
	case "finder_show":
		w.finder.show()
	case "finder_hide":
		w.finder.hide()
	case "finder_select":
		w.finder.selectResult(updates[1:])
	case "signature_show":
		w.signature.showItem(updates[1:])
	case "signature_pos":
		w.signature.pos(updates[1:])
	case "signature_hide":
		w.signature.hide()
	case "side_open":
		editor.wsSide.show()
	case "side_close":
		editor.wsSide.hide()
	case "side_toggle":
		editor.wsSide.toggle()
	case "filer_update":
		if !editor.wsSide.scrollarea.IsVisible() {
			return
		}
		if !editor.wsSide.items[editor.active].isContentHide {
			go w.nvim.Call("rpcnotify", nil, 0, "GonvimFiler", "redraw")
		}
	case "filer_open":
		editor.wsSide.items[w.getNum()].isContentHide = false
		editor.wsSide.items[w.getNum()].openContent()
	case "filer_clear":
		editor.wsSide.items[w.getNum()].clear()
	case "filer_resize":
		editor.wsSide.items[w.getNum()].resizeContent()
	case "filer_item_add":
		editor.wsSide.items[w.getNum()].addItem(updates[1:])
	case "filer_item_select":
		editor.wsSide.items[w.getNum()].selectItem(updates[1:])
	case "gonvim_grid_font":
		w.screen.gridFont(updates[1])
	case "gonvim_minimap_update":
		if w.minimap.visible {
			w.minimap.bufUpdate()
		}
	case "gonvim_minimap_sync":
		if w.minimap.visible {
			go w.minimap.bufSync()
		}
	case "gonvim_minimap_toggle":
		go w.minimap.toggle()
	case "gonvim_copy_clipboard":
		go editor.copyClipBoard()
	case "gonvim_get_maxline":
		w.maxLine = util.ReflectToInt(updates[1])
	case "gonvim_workspace_new":
		editor.workspaceNew()
	case "gonvim_workspace_next":
		editor.workspaceNext()
	case "gonvim_workspace_previous":
		editor.workspacePrevious()
	case "gonvim_workspace_switch":
		editor.workspaceSwitch(util.ReflectToInt(updates[1]))
	case "gonvim_workspace_cwd":
		w.setCwd(updates[1].(string))
	case "gonvim_workspace_filepath":
		w.filepath = updates[1].(string)
	case "gonvim_termenter":
		w.mode = "terminal-input"
	case "gonvim_termleave":
		w.mode = "normal"
	case GonvimMarkdownNewBufferEvent:
		go w.markdown.newBuffer()
	case GonvimMarkdownUpdateEvent:
		go w.markdown.update()
	case GonvimMarkdownToggleEvent:
		w.markdown.toggle()
	case GonvimMarkdownScrollDownEvent:
		w.markdown.scrollDown()
	case GonvimMarkdownScrollUpEvent:
		w.markdown.scrollUp()
	case GonvimMarkdownScrollTopEvent:
		w.markdown.scrollTop()
	case GonvimMarkdownScrollBottomEvent:
		w.markdown.scrollBottom()
	case GonvimMarkdownScrollPageDownEvent:
		w.markdown.scrollPageDown()
	case GonvimMarkdownScrollPageUpEvent:
		w.markdown.scrollPageUp()
	case GonvimMarkdownScrollHalfPageDownEvent:
		w.markdown.scrollHalfPageDown()
	case GonvimMarkdownScrollHalfPageUpEvent:
		w.markdown.scrollHalfPageUp()
	default:
		fmt.Println("unhandled Gui event", event)
	}
}

func (w *Workspace) guiFont(args string) {
	if args == "" {
		return
	}
	var fontHeight float64
	var fontFamily string

	if args == "*" {
		fDialog := widgets.NewQFontDialog(nil)
		fDialog.SetOption(widgets.QFontDialog__MonospacedFonts, true)
		fDialog.SetOption(widgets.QFontDialog__ProportionalFonts, false)
		fDialog.ConnectFontSelected(func(font *gui.QFont) {
			fontFamily = font.Family()
			fontHeight = font.PointSizeF()
			w.guiFont(fmt.Sprintf("%s:h%f", fontFamily, fontHeight))
		})
		fDialog.Show()
		return
	}

	for _, gfn := range strings.Split(args, ",") {
		fontFamily, fontHeight = getFontFamilyAndHeight(gfn)
		ok := checkValidFont(fontFamily)
		if ok {
			break
		}
	}

	if fontHeight == 0 {
		fontHeight = 10.0
	}

	w.font.change(fontFamily, fontHeight)
	w.screen.font = w.font

	w.updateSize()
	w.popup.updateFont(w.font)
	w.message.updateFont(w.font)
	w.cursor.updateFont(w.font)
	w.screen.toolTipFont(w.font)

	// Change external font if font setting of setting.yml is nothing
	if editor.config.Editor.FontFamily == "" {
		editor.extFontFamily = fontFamily
	}
	if editor.config.Editor.FontSize == 0 {
		editor.extFontSize = int(fontHeight)
	}

	w.palette.updateFont()
	w.fpalette.updateFont()
	w.tabline.updateFont()
	w.statusline.updateFont()
}

func (w *Workspace) guiFontWide(args string) {
	if args == "" {
		return
	}

	if w.fontwide == nil {
		w.fontwide = initFontNew(editor.extFontFamily, float64(editor.extFontSize), editor.config.Editor.Linespace, false)
		w.fontwide.ws = w
		w.cursor.fontwide = w.fontwide
	}

	var fontHeight float64
	var fontFamily string

	if args == "*" {
		fDialog := widgets.NewQFontDialog(nil)
		fDialog.SetOption(widgets.QFontDialog__MonospacedFonts, true)
		fDialog.SetOption(widgets.QFontDialog__ProportionalFonts, false)
		fDialog.ConnectFontSelected(func(font *gui.QFont) {
			fontFamily = font.Family()
			fontHeight = font.PointSizeF()
			w.guiFontWide(fmt.Sprintf("%s:h%f", fontFamily, fontHeight))
		})
		fDialog.Show()
		return
	}

	for _, gfn := range strings.Split(args, ",") {
		fontFamily, fontHeight = getFontFamilyAndHeight(gfn)
		ok := checkValidFont(fontFamily)
		if ok {
			break
		}
	}

	if fontHeight == 0 {
		fontHeight = 10.0
	}

	w.fontwide.change(fontFamily, fontHeight)

	w.updateSize()
	// w.cursor.updateFont(w.font)
	// w.screen.toolTipFont(w.font)
}

func getFontFamilyAndHeight(s string) (string, float64) {
	parts := strings.Split(s, ":")
	height := 10.0
	if len(parts) > 1 {
		for _, p := range parts[1:] {
			if strings.HasPrefix(p, "h") {
				var err error
				// height, err = strconv.Atoi(p[1:])
				height, err = strconv.ParseFloat(p[1:], 64)
				if err != nil {
					height = 10.0
				}
			} else if strings.HasPrefix(p, "w") {
				var err error
				// width, err := strconv.Atoi(p[1:])
				width, err := strconv.ParseFloat(p[1:], 64)
				if err != nil {
					height = 10.0
				}
				height = 2.0 * width
			}
		}
	}
	family := parts[0]

	return family, height
}

func checkValidFont(family string) bool {
	f := gui.NewQFont2(family, 10.0, 1, false)
	fi := gui.NewQFontInfo(f)

	return strings.EqualFold(fi.Family(), f.Family())
}

func (w *Workspace) guiLinespace(args interface{}) {
	// fontArg := args[0].([]interface{})
	var lineSpace int
	var err error
	switch arg := args.(type) {
	case string:
		lineSpace, err = strconv.Atoi(arg)
		if err != nil {
			return
		}
	case int32: // can't combine these in a type switch without compile error
		lineSpace = int(arg)
	case int64:
		lineSpace = int(arg)
	default:
		return
	}
	w.font.changeLineSpace(lineSpace)
	w.updateSize()
	// w.cursor.updateShape()
}

// InputMethodEvent is
func (w *Workspace) InputMethodEvent(event *gui.QInputMethodEvent) {
	if event.CommitString() != "" {
		w.nvim.Input(event.CommitString())
		w.screen.tooltip.Hide()
	} else {
		preeditString := event.PreeditString()
		if preeditString == "" {
			w.screen.tooltip.Hide()
			w.cursor.update()
		} else {
			w.screen.toolTip(preeditString)
		}
	}
}

// InputMethodQuery is
func (w *Workspace) InputMethodQuery(query core.Qt__InputMethodQuery) *core.QVariant {
	if query == core.Qt__ImCursorRectangle {
		x, y, candX, candY := w.screen.toolTipPos()
		w.screen.toolTipMove(x, y)
		imrect := core.NewQRect()
		imrect.SetRect(candX, candY, 1, w.font.lineHeight)

		if w.palette.widget.IsVisible() {
			w.cursor.x = x
			w.cursor.y = w.palette.patternPadding + w.cursor.shift
			w.cursor.widget.Move2(w.cursor.x, w.cursor.y)
		}

		return core.NewQVariant31(imrect)
	}
	return core.NewQVariant()
}

func (w *Workspace) getPointInWidget(col, row, grid int) (int, int, int, bool) {
	win, ok := w.screen.getWindow(grid)
	if !ok {
		return 0, 0, w.font.lineHeight, false
	}
	font := win.getFont()

	isCursorBelowTheCenter := false
	if row*font.lineHeight > w.screen.height/2 {
		isCursorBelowTheCenter = true
	}

	x := int(float64(col) * font.truewidth)
	y := row * font.lineHeight
	if w.drawTabline {
		y += w.tabline.widget.Height()
	}
	x += int(float64(win.pos[0]) * font.truewidth)
	y += win.pos[1] * font.lineHeight

	return x, y, font.lineHeight, isCursorBelowTheCenter
}

// WorkspaceSide is
type WorkspaceSide struct {
	widget     *widgets.QWidget
	scrollarea *widgets.QScrollArea
	header     *widgets.QLabel
	items      []*WorkspaceSideItem

	isShown bool
}

func newWorkspaceSide() *WorkspaceSide {
	layout := util.NewHFlowLayout(0, 0, 0, 0, 20)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(0)
	header := widgets.NewQLabel(nil, 0)
	header.SetContentsMargins(22, 15, 20, 10)
	header.SetText("WORKSPACE")
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 100)
	widget.SetLayout(layout)
	widget.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)

	side := &WorkspaceSide{
		widget: widget,
		header: header,
	}

	layout.AddWidget(header)
	side.header.Show()

	items := []*WorkspaceSideItem{}
	side.items = items
	for i := 0; i < WorkspaceLen; i++ {
		item := newWorkspaceSideItem()
		side.items = append(side.items, item)
		side.items[len(side.items)-1].side = side
		layout.AddWidget(side.items[len(side.items)-1].widget)
		side.items[len(side.items)-1].hide()
	}

	return side
}

func (side *WorkspaceSide) newScrollArea() {
	sideArea := widgets.NewQScrollArea(nil)
	sideArea.SetWidgetResizable(true)
	sideArea.SetVerticalScrollBarPolicy(core.Qt__ScrollBarAlwaysOff)
	sideArea.ConnectEnterEvent(func(event *core.QEvent) {
		sideArea.SetVerticalScrollBarPolicy(core.Qt__ScrollBarAsNeeded)
	})
	sideArea.ConnectLeaveEvent(func(event *core.QEvent) {
		sideArea.SetVerticalScrollBarPolicy(core.Qt__ScrollBarAlwaysOff)
	})
	sideArea.SetFocusPolicy(core.Qt__NoFocus | core.Qt__ClickFocus)
	sideArea.SetFrameShape(widgets.QFrame__NoFrame)
	// sideArea.SetFixedWidth(editor.config.SideBar.Width)

	side.scrollarea = sideArea
	side.scrollarea.SetWidget(side.widget)

	side.scrollarea.ConnectResizeEvent(func(*gui.QResizeEvent) {
		width := side.scrollarea.Width()
		for _, item := range side.items {
			item.label.SetMaximumWidth(width)
			item.label.SetMinimumWidth(width)
			item.content.SetMinimumWidth(width)
			item.content.SetMinimumWidth(width)
		}

	})
}

func (side *WorkspaceSide) toggle() {
	if side == nil {
		return
	}
	if side.isShown {
		side.scrollarea.Hide()
		side.isShown = false
	} else {
		side.scrollarea.Show()
		side.isShown = true
	}
}

func (side *WorkspaceSide) show() {
	if side == nil {
		return
	}
	if side.isShown {
		return
	}
	side.scrollarea.Show()
	side.isShown = true
}

func (side *WorkspaceSide) hide() {
	if side == nil {
		return
	}
	if editor.config.SideBar.Visible {
		return
	}
	if !side.isShown {
		return
	}
	side.scrollarea.Hide()
	side.isShown = false
}

func (w *Workspace) getNum() int {
	for i, ws := range editor.workspaces {
		if ws == w {
			return i
		}
	}
	return 0
}

// WorkspaceSideItem is
type WorkspaceSideItem struct {
	mu sync.Mutex

	hidden    bool
	active    bool
	side      *WorkspaceSide
	openIcon  *svg.QSvgWidget
	closeIcon *svg.QSvgWidget

	widget *widgets.QWidget
	layout *widgets.QBoxLayout

	text    string
	cwdpath string

	labelWidget *widgets.QWidget
	label       *widgets.QLabel

	content       *widgets.QListWidget
	isContentHide bool
}

func newWorkspaceSideItem() *WorkspaceSideItem {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0); }")

	layout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, widget)
	layout.SetContentsMargins(0, 5, 0, 5)

	labelWidget := widgets.NewQWidget(nil, 0)
	labelLayout := widgets.NewQHBoxLayout()
	labelWidget.SetLayout(labelLayout)
	labelLayout.SetContentsMargins(15, 1, 1, 1)
	labelLayout.SetSpacing(editor.iconSize / 2)

	label := widgets.NewQLabel(nil, 0)
	label.SetContentsMargins(0, 0, 0, 0)
	label.SetAlignment(core.Qt__AlignLeft)

	openIcon := svg.NewQSvgWidget(nil)
	openIcon.SetFixedWidth(editor.iconSize - 1)
	openIcon.SetFixedHeight(editor.iconSize - 1)
	svgContent := editor.getSvg("chevron-down", nil)
	openIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))

	closeIcon := svg.NewQSvgWidget(nil)
	closeIcon.SetFixedWidth(editor.iconSize - 1)
	closeIcon.SetFixedHeight(editor.iconSize - 1)
	svgContent = editor.getSvg("chevron-right", nil)
	closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))

	content := widgets.NewQListWidget(nil)
	content.SetFocusPolicy(core.Qt__NoFocus)
	content.SetFrameShape(widgets.QFrame__NoFrame)
	content.SetHorizontalScrollBarPolicy(core.Qt__ScrollBarAlwaysOff)
	content.SetFont(gui.NewQFont2(editor.extFontFamily, editor.extFontSize, 1, false))
	content.SetIconSize(core.NewQSize2(editor.iconSize*3/4, editor.iconSize*3/4))

	labelLayout.AddWidget(openIcon, 0, 0)
	labelLayout.AddWidget(closeIcon, 0, 0)
	labelLayout.AddWidget(label, 0, 0)

	labelLayout.SetAlignment(openIcon, core.Qt__AlignLeft)
	labelLayout.SetAlignment(closeIcon, core.Qt__AlignLeft)
	labelLayout.SetAlignment(label, core.Qt__AlignLeft)
	// layout.AddWidget(flwidget, 0, 0)

	layout.AddWidget(labelWidget, 1, 0)
	layout.AddWidget(content, 0, 0)
	layout.SetAlignment(labelWidget, core.Qt__AlignLeft)
	layout.SetAlignment(content, core.Qt__AlignLeft)

	openIcon.Hide()
	closeIcon.Show()

	sideitem := &WorkspaceSideItem{
		widget:        widget,
		layout:        layout,
		labelWidget:   labelWidget,
		label:         label,
		openIcon:      openIcon,
		closeIcon:     closeIcon,
		content:       content,
		isContentHide: true,
	}

	sideitem.widget.ConnectMousePressEvent(sideitem.toggleContent)
	content.ConnectItemDoubleClicked(sideitem.fileDoubleClicked)

	return sideitem
}

func (i *WorkspaceSideItem) fileDoubleClicked(item *widgets.QListWidgetItem) {
	filename := item.Text()
	path := i.cwdpath
	sep := ""
	if runtime.GOOS == "windows" {
		sep = `\`
	} else {
		sep = `/`
	}
	filepath := path + sep + filename

	exec := ""
	switch runtime.GOOS {
	case "darwin":
		exec = ":silent !open "
	case "windows":
		exec = ":silent !explorer "
	case "linux":
		exec = ":silent !xdg-open "
	}

	execCommand := exec + filepath
	for j, ws := range editor.workspaces {
		if editor.wsSide.items[j] == nil {
			continue
		}
		sideItem := editor.wsSide.items[j]
		if i == sideItem {
			go ws.nvim.Command(execCommand)
		}
	}
}

func (i *WorkspaceSideItem) toggleContent(event *gui.QMouseEvent) {
	if i.hidden {
		return
	}
	if i.isContentHide {
		for j, ws := range editor.workspaces {
			if editor.wsSide.items[j] == nil {
				continue
			}
			sideItem := editor.wsSide.items[j]
			if i == sideItem {
				i.isContentHide = false
				i.openContent()
				go ws.nvim.Call("rpcnotify", nil, 0, "GonvimFiler", "redraw")
			}
		}
	} else {
		i.closeContent()
	}
}

func (i *WorkspaceSideItem) openContent() {
	if i.content.StyleSheet() == "" {
		i.content.SetStyleSheet(
			fmt.Sprintf(`
				QListWidget::item {
				   color: %s;
				   padding-left: 20px;
				   background-color: rgba(0, 0, 0, 0.0);
				}
				QListWidget::item:selected {
				   background-color: %s;
				}`,
				editor.colors.sideBarFg.String(),
				editor.colors.selectedBg.String(),
			),
		)
	}
	i.openIcon.Show()
	i.closeIcon.Hide()
	i.isContentHide = false
	i.content.Show()
}

func (i *WorkspaceSideItem) closeContent() {
	i.openIcon.Hide()
	i.closeIcon.Show()
	i.isContentHide = true
	i.content.Hide()
}

func (i *WorkspaceSideItem) setText(text string) {
	if i.text == text {
		return
	}
	i.text = text
	i.label.SetText(text)
	i.widget.Show()
}

func (i *WorkspaceSideItem) setSideItemLabel(n int) {
	if n == editor.active {
		i.setActive()
	} else {
		i.setInactive()
	}
	i.label.SetContentsMargins(1, 3, 0, 3)
}

func (i *WorkspaceSideItem) clear() {
	i.content.Clear()
}

func (i *WorkspaceSideItem) addItem(args []interface{}) {
	filename := args[0].(string)
	filetype := args[1].(string)
	l := widgets.NewQListWidgetItem(i.content, 1)
	var svg string
	if filetype == `/` {
		svg = editor.getSvg("directory", nil)
	} else {
		svg = editor.getSvg(filetype, nil)
	}
	pixmap := gui.NewQPixmap()
	pixmap.LoadFromData2(core.NewQByteArray2(svg, len(svg)), "SVG", core.Qt__ColorOnly)
	icon := gui.NewQIcon2(pixmap)

	l.SetIcon(icon)
	l.SetText(filename)
	i.content.AddItem2(l)
}

func (i *WorkspaceSideItem) resizeContent() {
	rowNum := i.content.Count()
	if rowNum > editor.config.FileExplore.MaxDisplayItems {
		rowNum = editor.config.FileExplore.MaxDisplayItems
	}
	itemHeight := i.content.RectForIndex(i.content.IndexFromItem(i.content.Item(0))).Height()
	i.content.SetFixedHeight(itemHeight * rowNum)
}

func (i *WorkspaceSideItem) selectItem(args []interface{}) {
	i.content.SetCurrentRow(util.ReflectToInt(args[0]))
}

func (side *WorkspaceSide) setColor() {
	fg := editor.colors.sideBarFg.String()
	sfg := editor.colors.scrollBarFg.String()
	sbg := editor.colors.scrollBarBg.StringTransparent()
	side.header.SetStyleSheet(fmt.Sprintf(" .QLabel{ color: %s;} ", fg))
	side.widget.SetStyleSheet(fmt.Sprintf(".QWidget { border: 0px solid #000; padding-top: 5px; background-color: rgba(0, 0, 0, 0); } QWidget { color: %s; border-right: 0px solid; }", fg))
	if side.scrollarea == nil {
		return
	}
	side.scrollarea.SetStyleSheet(fmt.Sprintf(".QScrollBar { border-width: 0px; background-color: %s; width: 5px; margin: 0 0 0 0; } .QScrollBar::handle:vertical {background-color: %s; min-height: 25px;} .QScrollBar::handle:vertical:hover {background-color: %s; min-height: 25px;} .QScrollBar::add-line:vertical, .QScrollBar::sub-line:vertical { border: none; background: none; } .QScrollBar::add-page:vertical, QScrollBar::sub-page:vertical { background: none; }", sbg, sfg, editor.config.SideBar.AccentColor))

	if len(editor.workspaces) == 1 {
		side.items[0].active = true
		side.items[0].labelWidget.SetStyleSheet(
			fmt.Sprintf(
				" * { background-color: %s; color: %s; }",
				editor.colors.sideBarSelectedItemBg, fg,
			),
		)
	}

}

func (i *WorkspaceSideItem) setActive() {
	if editor.colors.fg == nil {
		return
	}
	if editor.wsSide.scrollarea == nil {
		return
	}
	i.active = true
	bg := editor.colors.sideBarSelectedItemBg
	fg := editor.colors.fg
	transparent := transparent()
	i.labelWidget.SetStyleSheet(
		fmt.Sprintf(
			" * { background-color: rgba(%d, %d, %d, %f); color: %s; }",
			bg.R, bg.G, bg.B,
			transparent,
			fg.String(),
		),
	)
	svgOpenContent := editor.getSvg("chevron-down", fg)
	i.openIcon.Load2(core.NewQByteArray2(svgOpenContent, len(svgOpenContent)))
	svgCloseContent := editor.getSvg("chevron-right", fg)
	i.closeIcon.Load2(core.NewQByteArray2(svgCloseContent, len(svgCloseContent)))
}

func (i *WorkspaceSideItem) setInactive() {
	if editor.colors.fg == nil {
		return
	}
	if editor.wsSide.scrollarea == nil {
		return
	}
	i.active = false
	fg := editor.colors.inactiveFg
	i.labelWidget.SetStyleSheet(
		fmt.Sprintf(
			" * { background-color: rgba(0, 0, 0, 0); color: %s; }",
			fg.String(),
		),
	)
	svgOpenContent := editor.getSvg("chevron-down", fg)
	i.openIcon.Load2(core.NewQByteArray2(svgOpenContent, len(svgOpenContent)))
	svgCloseContent := editor.getSvg("chevron-right", fg)
	i.closeIcon.Load2(core.NewQByteArray2(svgCloseContent, len(svgCloseContent)))
}

func (i *WorkspaceSideItem) show() {
	if !i.hidden {
		return
	}
	i.hidden = false
	i.label.Show()

	if !i.isContentHide {
		i.content.Show()
		i.openIcon.Show()
		i.closeIcon.Hide()
	} else {
		i.content.Hide()
		i.openIcon.Hide()
		i.closeIcon.Show()
	}
}

func (i *WorkspaceSideItem) hide() {
	if i.hidden {
		return
	}
	i.hidden = true
	i.label.Hide()
	i.openIcon.Hide()
	i.closeIcon.Hide()

	i.content.Hide()
}
