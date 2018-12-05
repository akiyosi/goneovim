package editor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akiyosi/gonvim/fuzzy"
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
	cursor     *Cursor
	tabline    *Tabline
	statusline *Statusline
	screen     *Screen
	scrollBar  *ScrollBar
	markdown   *Markdown
	finder     *Finder
	palette    *Palette
	popup      *PopupMenu
	loc        *Locpopup
	cmdline    *Cmdline
	signature  *Signature
	// Need https://github.com/neovim/neovim/pull/7466 to be merged
	// message    *Message
	minimap *MiniMap
	width   int
	height  int
	hidden  bool

	nvim             *nvim.Nvim
	rows             int
	cols             int
	uiAttached       bool
	uiInitialResized bool
	foreground       *RGBA
	background       *RGBA
	special          *RGBA
	mode             string
	filepath         string
	cwd              string
	cwdBase          string
	cwdlabel         string
	maxLine          int
	curLine          int
	curColm          int

	signal        *workspaceSignal
	redrawUpdates chan [][]interface{}
	guiUpdates    chan []interface{}
	doneNvimStart chan bool
	stopOnce      sync.Once
	stop          chan struct{}

	drawStatusline bool
	drawTabline    bool
	drawLint       bool

	isSetGuiColor      bool
	noColorschemeCount int
	flushCount         int
}

func newWorkspace(path string) (*Workspace, error) {
	w := &Workspace{
		stop:          make(chan struct{}),
		signal:        NewWorkspaceSignal(nil),
		redrawUpdates: make(chan [][]interface{}, 1000),
		guiUpdates:    make(chan []interface{}, 1000),
		doneNvimStart: make(chan bool, 1000),
	}
	w.signal.ConnectRedrawSignal(func() {
		updates := <-w.redrawUpdates
		w.handleRedraw(updates)
	})
	w.signal.ConnectGuiSignal(func() {
		updates := <-w.guiUpdates
		w.handleRPCGui(updates)
	})
	w.signal.ConnectStopSignal(func() {
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
	w.font = initFontNew(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, editor.config.Editor.Linespace)
	w.tabline = newTabline()
	w.tabline.ws = w
	w.statusline = initStatuslineNew()
	w.statusline.ws = w
	w.screen = newScreen()
	w.screen.toolTipFont(w.font)
	w.screen.ws = w
	w.scrollBar = newScrollBar()
	w.scrollBar.ws = w
	w.markdown = newMarkdown(w)
	w.markdown.webview.SetParent(w.screen.widget)
	w.cursor = initCursorNew()
	w.cursor.widget.SetParent(w.screen.widget)
	w.cursor.ws = w
	w.popup = initPopupmenuNew(w.font)
	w.popup.widget.SetParent(w.screen.widget)
	w.popup.ws = w
	w.finder = initFinder()
	w.finder.ws = w
	w.palette = initPalette()
	w.palette.widget.SetParent(editor.window)
	w.palette.ws = w
	w.loc = initLocpopup()
	w.loc.widget.SetParent(w.screen.widget)
	w.loc.ws = w
	w.signature = initSignature()
	w.signature.widget.SetParent(w.screen.widget)
	w.signature.ws = w
	// Need https://github.com/neovim/neovim/pull/7466 to be merged
	// w.message = initMessage()
	// w.message.widget.SetParent(w.screen.widget)
	// w.message.ws = w
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
	editor.wsWidget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	editor.wsWidget.ConnectInputMethodEvent(w.InputMethodEvent)
	editor.wsWidget.ConnectInputMethodQuery(w.InputMethodQuery)

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
	w.loc.widget.Hide()
	w.signature.widget.Hide()

	w.widget.SetParent(editor.wsWidget)
	w.widget.Move2(0, 0)
	w.updateSize()

	go w.minimap.startMinimapProc()
	go w.startNvim(path)

	select {
	case <-w.doneNvimStart:
	}

	return w, nil
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
	neovim, err := nvim.NewChildProcess(nvim.ChildProcessArgs(append([]string{"--cmd", "let g:gonvim_running=1", "--embed"}, os.Args[1:]...)...))
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

	w.doneNvimStart <- true

	return nil
}

func (w *Workspace) init(path string) {
	w.configure()
	w.attachUI(path)
	w.initCwd()
	w.updateSize()
}

func (w *Workspace) configure() {
	if editor.config.Statusline.Visible {
		w.drawStatusline = true
	} else {
		w.drawStatusline = false
	}

	if editor.config.Tabline.Visible {
		w.drawTabline = true
	} else {
		w.drawTabline = false
	}

	if editor.config.Lint.Visible {
		w.drawLint = true
	} else {
		w.drawLint = false
	}

	if editor.config.Editor.StartFullscreen {
		editor.window.ShowFullScreen()
	}
}

func (w *Workspace) attachUI(path string) error {
	w.initGonvim()
	w.initGonvimCommands(path)
	w.markdown.commands()
	fuzzy.RegisterPlugin(w.nvim)
	w.tabline.subscribe()
	w.statusline.subscribe()
	w.loc.subscribe()
	// NOTE: Need https://github.com/neovim/neovim/pull/7466 to be merged
	// w.message.subscribe()

	w.uiAttached = true
	option := w.attachUIOption()
	err := w.nvim.AttachUI(w.cols, w.rows, option)
	if err != nil {
		return err
	}
	return nil
}

func (w *Workspace) initGonvim() {
	_, _ = w.nvimEval("0")
	w.nvim.Subscribe("Gui")
	w.nvim.Command("runtime plugin/nvim_gui_shim.vim")
	w.nvim.Command(fmt.Sprintf("command! GonvimVersion echo \"%s\"", editor.version))
	w.nvim.Command(`call rpcnotify(0, "statusline", "bufenter", expand("%:p"), &filetype, &fileencoding, &fileformat)`)
	w.nvim.Command(`call rpcnotify(0, "Gui", "gonvim_cursormoved",  getpos("."))`)
	w.nvim.Command(`call rpcnotify(0, "Gui", "gonvim_workspace_redrawSideItem")`)
	w.nvim.Command(`call rpcnotify(0, "Gui", "gonvim_minimap_update")`)
	w.loadGinitVim()
}

func (w *Workspace) loadGinitVim() {
	w.nvim.Command("runtime! ginit.vim") // ginit.vim is deprecated and will be removed in a future
	if editor.config.Editor.GinitVim != "" {
		execGinitVim := fmt.Sprintf(`call execute(split('%s', '\n'))`, editor.config.Editor.GinitVim)
		w.nvim.Command(execGinitVim)
	}
}

func (w *Workspace) initGonvimCommands(path string) {
	if path != "" {
		w.nvim.Command("so " + path)
	}
	w.nvim.Command(`autocmd VimEnter * call rpcnotify(1, "Gui", "gonvim_enter")`)
	w.nvim.Command(`autocmd VimLeavePre * call rpcnotify(1, "Gui", "gonvim_exit")`)
	// w.nvim.Command(`autocmd ColorScheme * call rpcnotify(1, "Gui", "gonvim_set_colorscheme")`)
	w.nvim.Command(`autocmd VimEnter,DirChanged * call rpcnotify(0, "Gui", "gonvim_workspace_cwd")`)
	w.nvim.Command(`autocmd BufEnter,TabEnter,DirChanged,TermOpen,TermClose * call rpcnotify(0, "Gui", "gonvim_workspace_redrawSideItems", expand("%:p"))`)
	w.nvim.Command(`autocmd TextChanged,TextChangedI,BufEnter,BufWrite,DirChanged * call rpcnotify(0, "Gui", "gonvim_workspace_redrawSideItem")`)
	if editor.config.ScrollBar.Visible == true {
		w.nvim.Command(`autocmd TextChanged,TextChangedI,BufEnter * call rpcnotify(0, "Gui", "gonvim_get_maxline")`)
	}
	w.nvim.Command(`autocmd BufEnter,BufWrite * call rpcnotify(0, "Gui", "gonvim_minimap_update")`)
	if editor.config.Editor.Clipboard == true {
		w.nvim.Command(`autocmd TextYankPost * call rpcnotify(0, "Gui", "gonvim_copy_clipboard")`)
	}
	w.nvim.Command(`autocmd CursorMoved,CursorMovedI * call rpcnotify(0, "Gui", "gonvim_cursormoved", getpos("."))`)

	w.nvim.Command(`command! GonvimMiniMap call rpcnotify(0, 'Gui', 'gonvim_minimap_toggle')`)
	w.nvim.Command(`command! GonvimWorkspaceNew call rpcnotify(0, 'Gui', 'gonvim_workspace_new')`)
	w.nvim.Command(`command! GonvimWorkspaceNext call rpcnotify(0, 'Gui', 'gonvim_workspace_next')`)
	w.nvim.Command(`command! GonvimWorkspacePrevious call rpcnotify(0, 'Gui', 'gonvim_workspace_previous')`)
	w.nvim.Command(`command! -nargs=1 GonvimWorkspaceSwitch call rpcnotify(0, 'Gui', 'gonvim_workspace_switch', <args>)`)
}

func (w *Workspace) nvimCommandOutput(s string) (string, error) {
	doneChannel := make(chan string, 5)
	var result string
	go func() {
		result, _ = w.nvim.CommandOutput(s)
		doneChannel <- result
	}()
	select {
	case done := <-doneChannel:
		return done, nil
	case <-time.After(40 * time.Millisecond):
		err := errors.New("neovim busy")
		return "", err
	}
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

func (w *Workspace) initCwd() {
	cwdITF, err := w.nvimEval("getcwd()")
	if err != nil {
		return
	}
	cwd := cwdITF.(string)
	if cwd == "" {
		return
	}
	w.nvim.Command("cd " + cwd)
}

func (w *Workspace) setCwd() {
	cwdITF, err := w.nvimEval("getcwd()")
	if err != nil {
		return
	}
	cwd := cwdITF.(string)
	if cwd == "" {
		return
	}
	w.cwd = cwd

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
			if sideItem.cwdpath == path && sideItem.isload {
				continue
			}

			sideItem.label.SetText(w.cwdlabel)
			sideItem.label.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))
			sideItem.cwdpath = path

			if editor.activity.editItem.active == false {
				continue
			}

			filelist := newFilelistwidget(path)
			sideItem.isload = true
			sideItem.setFilelistwidget(filelist)
			continue
		}
	}
}

func (i *WorkspaceSideItem) setFilelistwidget(f *Filelist) {
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()

	if i.Filelistwidget != nil {
		i.Filelistwidget.DestroyQWidget()
	}
	i.layout.AddWidget(f.widget, 0, 0)
	i.Filelistwidget = f.widget
	i.Filelist = f
	i.Filelist.WSitem = i
	i.active = true
	if i.isFilelistHide {
		i.closeFilelist()
	} else {
		i.openFilelist()
	}
}

func (i *WorkspaceSideItem) toggleFilelist(event *gui.QMouseEvent) {
	if i.hidden {
		return
	}
	if i.isFilelistHide {
		i.openFilelist()
	} else {
		i.closeFilelist()
	}
}

func (i *WorkspaceSideItem) openFilelist() {
	i.Filelistwidget.Show()
	i.openIcon.Show()
	i.closeIcon.Hide()
	i.isFilelistHide = false
}

func (i *WorkspaceSideItem) closeFilelist() {
	i.Filelistwidget.Hide()
	i.openIcon.Hide()
	i.closeIcon.Show()
	i.isFilelistHide = true
}

func (w *Workspace) attachUIOption() map[string]interface{} {
	o := make(map[string]interface{})
	o["rgb"] = true

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
					if name == "wildmenu_show" {
						o["ext_wildmenu"] = true
					} else if name == "cmdline_show" {
						o["ext_cmdline"] = true
					} else if name == "msg_chunk" {
						o["ext_messages"] = true
					} else if name == "popupmenu_show" {
						o["ext_popupmenu"] = true
					} else if name == "tabline_update" {
						o["ext_tabline"] = w.drawTabline
					}
				}
			}
		}
	}
	return o
}

func (w *Workspace) updateSize() {
	e := editor
	e.width = e.window.Width()
	e.height = e.window.Height()
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

	// height = w.height - w.tabline.height - w.statusline.height
	// rows := height / w.font.lineHeight

	// remainingHeight := height - rows*w.font.lineHeight
	// remainingHeightBottom := remainingHeight / 2
	// remainingHeightTop := remainingHeight - remainingHeightBottom
	// w.tabline.marginTop = w.tabline.marginDefault + remainingHeightTop
	// w.tabline.marginBottom = w.tabline.marginDefault + remainingHeightBottom
	// w.tabline.updateMargin()
	// w.screen.height = height - remainingHeight

	w.screen.height = w.height - w.tabline.height - w.statusline.height

	w.screen.updateSize()
	w.palette.resize()
	// w.message.resize() // Need https://github.com/neovim/neovim/pull/7466 to be merged

	// notification
	e.notifyStartPos = core.NewQPoint2(e.width-e.notificationWidth-10, e.height-30)
	x := e.notifyStartPos.X()
	y := e.notifyStartPos.Y()
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
	doMinimapScroll := false
	isSetColorscheme := false
	var fgcolor, bgcolor int
	for _, update := range updates {
		event := update[0].(string)
		args := update[1:]
		switch event {
		case "set_title":
			titleString := (update[1].([]interface{}))[0].(string)
			editor.window.SetWindowTitle(titleString)
		case "option_set":
			optKey := (update[1].([]interface{}))[0].(string)
			optValue := (update[1].([]interface{}))[1]
			switch optKey {
			case "arabicshape":
			case "ambiwidth":
			case "emoji":
			case "guifont":
				w.guiFont(optValue.(string))
			case "guifontset":
			case "guifontwide":
			case "linespace":
				w.guiLinespace(optValue)
			case "showtabline":
			case "termguicolors":
			default:
			}
		case "update_fg":
			args := update[1].([]interface{})
			fgcolor = reflectToInt(args[0])
			if w.isSetGuiColor == false {
				if fgcolor == -1 && w.foreground == nil {
					w.foreground = newRGBA(180, 185, 190, 1)
				} else if fgcolor != -1 {
					isSetColorscheme = true
					w.foreground = calcColor(fgcolor)
					w.minimap.foreground = w.foreground
					editor.fgcolor = w.foreground
				}

			} else if fgcolor != -1 {
				w.foreground = calcColor(fgcolor)
				w.minimap.foreground = w.foreground
			}
		case "update_bg":
			args := update[1].([]interface{})
			bgcolor = reflectToInt(args[0])
			if w.isSetGuiColor == false {
				if bgcolor == -1 && w.background == nil {
					// TODO: Detecting &background option in buffer
					//       and Set colors according to 'light', 'dark' value respectively
					w.background = newRGBA(9, 13, 17, 1)
				} else if bgcolor != -1 {
					isSetColorscheme = true
					w.background = calcColor(bgcolor)
					w.minimap.background = w.background
					editor.bgcolor = w.background
				}
			} else if bgcolor != -1 {
				w.background = calcColor(bgcolor)
				w.minimap.background = w.background
			}
		case "update_sp":
			args := update[1].([]interface{})
			color := reflectToInt(args[0])
			if color == -1 {
				w.special = newRGBA(255, 255, 255, 1)
			} else {
				w.special = calcColor(reflectToInt(args[0]))
				w.minimap.special = calcColor(reflectToInt(args[0]))
			}
		case "cursor_goto":
			s.cursorGoto(args)
			doMinimapScroll = true
		case "put":
			s.put(args)
		case "eol_clear":
			s.eolClear(args)
		case "clear":
			s.clear(args)
		case "resize":
			s.resize(args)
		case "highlight_set":
			s.highlightSet(args)
		case "set_scroll_region":
			s.setScrollRegion(args)
		case "scroll":
			s.scroll(args)
			doMinimapScroll = true
		case "mode_change":
			arg := update[len(update)-1].([]interface{})
			w.mode = arg[0].(string)
			if editor.config.Editor.DisableImeInNormal {
				switch w.mode {
				case "normal":
					w.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, false)
					editor.wsWidget.SetAttribute(core.Qt__WA_InputMethodEnabled, false)
				case "insert":
					w.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
					editor.wsWidget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
				}
			}
		case "popupmenu_show":
			w.popup.showItems(args)
		case "popupmenu_hide":
			w.popup.hide()
		case "popupmenu_select":
			w.popup.selectItem(args)
		case "tabline_update":
			w.tabline.update(args)
		case "cmdline_show":
			w.cmdline.show(args)
		case "cmdline_pos":
			w.cmdline.changePos(args)
		case "cmdline_char":
			w.cmdline.putChar(args)
		case "cmdline_hide":
			w.cmdline.hide(args)
		case "cmdline_function_show":
			w.cmdline.functionShow()
		case "cmdline_function_hide":
			w.cmdline.functionHide()
		case "wildmenu_show":
			w.cmdline.wildmenuShow(args)
		case "wildmenu_select":
			w.cmdline.wildmenuSelect(args)
		case "wildmenu_hide":
			w.cmdline.wildmenuHide()
		case "msg_start_kind":
			// Need https://github.com/neovim/neovim/pull/7466 to be merged
			// if len(args) > 0 {
			// 	kinds, ok := args[len(args)-1].([]interface{})
			// 	if ok {
			// 		if len(kinds) > 0 {
			// 			kind, ok := kinds[len(kinds)-1].(string)
			// 			if ok {
			// 				w.message.kind = kind
			// 			}
			// 		}
			// 	}
			// }
		case "msg_chunk":
			// Need https://github.com/neovim/neovim/pull/7466 to be merged
			// w.message.chunk(args)
		case "msg_end":
		case "msg_showcmd":
		case "messages":
		case "busy_start":
		case "busy_stop":
		case "flush":
			w.flushCount++
		default:
			fmt.Println("Unhandle event", event)
		}
	}

	w.drawGuiColor(isSetColorscheme)
	if !w.isSetGuiColor {
		if (fgcolor == -1 && bgcolor == -1) || w.flushCount >= 9 {
			w.noColorschemeCount++
		}
	}

	s.update()
	w.cursor.update()
	if editor.config.ScrollBar.Visible == true {
		w.scrollBar.update()
	}
	if doMinimapScroll && w.minimap.visible {
		go func() {
			var absMapTop int
			w.minimap.nvim.Eval("line('w0')", &absMapTop)
			var absMapBottom int
			w.minimap.nvim.Eval("line('w$')", &absMapBottom)
			w.minimap.nvim.Command(fmt.Sprintf("call cursor(%d, %d)", w.curLine, 0))
			switch {
			case w.curLine >= absMapBottom:
				w.minimap.nvim.Input("<C-d>")
			case absMapTop >= w.curLine:
				w.minimap.nvim.Input("<C-u>")
			default:
			}
		}()
		w.minimap.mapScroll()
	}
	w.statusline.mode.redraw()
}

func (w *Workspace) handleRPCGui(updates []interface{}) {
	event := updates[0].(string)
	switch event {
	case "gonvim_enter":
		editor.window.SetWindowOpacity(1.0)
		go func() {
			time.Sleep(2000 * time.Millisecond)
			msg, _ := w.nvimCommandOutput("messages")
			if msg != "" {
				editor.pushNotification(NotifyWarn, -1, msg)
			}
		}()
	case "gonvim_exit":
		editor.workspaces[editor.active].minimap.exit()
	// case "gonvim_set_colorscheme":
	// 	fmt.Println("set_colorscheme")
	// 	w.isSetGuiColor = false
	case "Font":
		w.guiFont(updates[1].(string))
	case "Linespace":
		//w.guiLinespace(updates[1:])
		w.guiLinespace(updates[1])
	case "finder_pattern":
		w.finder.showPattern(updates[1:])
	case "finder_pattern_pos":
		w.finder.cursorPos(updates[1:])
	case "finder_show_result":
		w.finder.showResult(updates[1:])
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
	case "gonvim_cursormoved":
		pos := updates[1].([]interface{})
		ln := reflectToInt(pos[1])
		col := reflectToInt(pos[2]) + reflectToInt(pos[3])
		w.statusline.pos.redraw(ln, col)
		w.curLine = ln
		w.curColm = col
	case "gonvim_minimap_update":
		if w.minimap.visible {
			w.minimap.bufUpdate()
		}
	case "gonvim_minimap_toggle":
		go w.minimap.toggle()
	case "gonvim_copy_clipboard":
		go editor.copyClipBoard()
	case "gonvim_get_maxline":
		go func() {
			lnITF, err := w.nvimEval("line('$')")
			if err != nil {
				// w.maxLine = 0    // denite is not working if uncomment this line
			} else {
				switch lnITF.(type) {
				case uint64:
					w.maxLine = int(lnITF.(uint64))
				case uint:
					w.maxLine = int(lnITF.(uint))
				case int64:
					w.maxLine = int(lnITF.(int64))
				case int:
					w.maxLine = lnITF.(int)
				default:
				}
			}
		}()
	case "gonvim_workspace_new":
		editor.workspaceNew()
	case "gonvim_workspace_next":
		editor.workspaceNext()
	case "gonvim_workspace_previous":
		editor.workspacePrevious()
	case "gonvim_workspace_switch":
		editor.workspaceSwitch(reflectToInt(updates[1]))
	case "gonvim_workspace_cwd":
		w.setCwd()
	case "gonvim_workspace_redrawSideItem":
		fl := editor.wsSide.items[editor.active].Filelist
		if fl.active != -1 {
			if len(fl.Fileitems) != 0 {
				fl.Fileitems[fl.active].updateModifiedbadge()
			}
		}
	case "gonvim_workspace_redrawSideItems":
		file := updates[1].(string)
		editor.workspaces[editor.active].filepath = file
		editor.wsSide.items[editor.active].setCurrentFileLabel()
	case GonvimMarkdownNewBufferEvent:
		go w.markdown.newBuffer()
	case GonvimMarkdownUpdateEvent:
		go w.markdown.update()
	case GonvimMarkdownToggleEvent:
		go w.markdown.toggle()
	case GonvimMarkdownScrollDownEvent:
		w.markdown.scrollDown()
	case GonvimMarkdownScrollUpEvent:
		w.markdown.scrollUp()
	default:
		fmt.Println("unhandled Gui event", event)
	}
}

func (w *Workspace) drawGuiColor(isSetColorscheme bool) {
	if w.isSetGuiColor {
		return
	}
	// If colorscheme is not set, enable default color
	if w.noColorschemeCount >= 2 && !isSetColorscheme {
		editor.fgcolor = newRGBA(180, 185, 190, 1)
		editor.bgcolor = newRGBA(9, 13, 17, 1)
		w.foreground = editor.fgcolor
		w.background = editor.bgcolor
		w.minimap.foreground = editor.fgcolor
		w.minimap.background = editor.bgcolor
	}
	w.setGuiColor(editor.fgcolor, editor.bgcolor)
}

func (w *Workspace) guiFont(args string) {
	parts := strings.Split(args, ":")
	if len(parts) < 1 {
		return
	}

	height := 14
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "h") {
			var err error
			height, err = strconv.Atoi(p[1:])
			if err != nil {
				return
			}
		}
	}

	w.font.change(parts[0], height)
	w.updateSize()
	w.popup.updateFont(w.font)
	w.screen.toolTipFont(w.font)
}

func (w *Workspace) detectTerminalMode() {
	if !strings.HasPrefix(w.filepath, `term://`) {
		return
	}
	m := new(sync.Mutex)

	go func() {
		m.Lock()
		mode, _ := w.nvim.Mode()
		switch mode.Mode {
		case "t":
			w.mode = "terminal-input"
		case "n":
			w.mode = "normal"
		case "c":
			w.mode = "cmdline_normal"
		case "i":
			w.mode = "insert"
		case "v":
			w.mode = "visual"
		case "R":
			w.mode = "replace"
		}
		m.Unlock()
	}()
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
	qv := core.NewQVariant()
	if query == core.Qt__ImCursorRectangle {
		imrect := core.NewQRect()
		row := w.screen.cursor[0]
		col := w.screen.cursor[1]
		x := int(float64(col)*w.font.truewidth) - 1
		y := row*w.font.lineHeight + w.tabline.height + w.tabline.marginTop + w.tabline.marginBottom
		imrect.SetRect(x, y, 1, w.font.lineHeight)
		return core.NewQVariant33(imrect)
	}
	return qv
}

// WorkspaceSide is
type WorkspaceSide struct {
	widget     *widgets.QWidget
	scrollarea *widgets.QScrollArea
	header     *widgets.QLabel
	items      []*WorkspaceSideItem
}

func newWorkspaceSide() *WorkspaceSide {
	layout := newHFlowLayout(0, 0, 0, 0, 20)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(0)
	header := widgets.NewQLabel(nil, 0)
	header.SetContentsMargins(22, 15, 20, 10)
	header.SetText("WORKSPACE")
	header.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))
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
	for i := 0; i < 20; i++ {
		item := newWorkspaceSideItem()
		side.items = append(side.items, item)
		side.items[len(side.items)-1].side = side
		layout.AddWidget(side.items[len(side.items)-1].widget)
		side.items[len(side.items)-1].hide()
	}

	return side
}

// WorkspaceSideItem is
type WorkspaceSideItem struct {
	hidden    bool
	active    bool
	side      *WorkspaceSide
	openIcon  *svg.QSvgWidget
	closeIcon *svg.QSvgWidget

	widget *widgets.QWidget
	layout *widgets.QBoxLayout
	//layout    *widgets.QLayout

	text    string
	cwdpath string

	Filelist       *Filelist
	isload         bool
	labelWidget    *widgets.QWidget
	label          *widgets.QLabel
	Filelistwidget *widgets.QWidget
	isFilelistHide bool
}

func newWorkspaceSideItem() *WorkspaceSideItem {
	widget := widgets.NewQWidget(nil, 0)

	layout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, widget)
	layout.SetContentsMargins(0, 5, 0, 5)

	labelWidget := widgets.NewQWidget(nil, 0)
	labelLayout := widgets.NewQHBoxLayout()
	labelWidget.SetLayout(labelLayout)
	labelLayout.SetContentsMargins(15, 1, 1, 1)

	label := widgets.NewQLabel(nil, 0)
	label.SetContentsMargins(0, 0, 0, 0)
	label.SetMaximumWidth(editor.config.SideBar.Width)
	label.SetMinimumWidth(editor.config.SideBar.Width)

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

	// flwidget := widgets.NewQWidget(nil, 0)

	filelist := &Filelist{}
	// filelist := &Filelist{
	// 	widget: flwidget,
	// }

	labelLayout.AddWidget(openIcon, 0, 0)
	labelLayout.AddWidget(closeIcon, 0, 0)
	labelLayout.AddWidget(label, 0, 0)
	// layout.AddWidget(flwidget, 0, 0)

	layout.AddWidget(labelWidget, 0, 0)
	openIcon.Hide()
	closeIcon.Hide()

	sideitem := &WorkspaceSideItem{
		widget:      widget,
		layout:      layout,
		labelWidget: labelWidget,
		label:       label,
		openIcon:    openIcon,
		closeIcon:   closeIcon,
		Filelist:    filelist,
		// Filelistwidget: flwidget,
	}
	sideitem.Filelist.WSitem = sideitem

	sideitem.widget.ConnectMousePressEvent(sideitem.toggleFilelist)

	return sideitem
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

func (i *WorkspaceSideItem) setActive() {
	// if i.active {
	// 	return
	// }
	if editor.fgcolor == nil || editor.bgcolor == nil {
		return
	}
	i.active = true
	bg := editor.bgcolor
	fg := editor.fgcolor
	wsSideitemActiveBgColor := warpColor(bg, -6)
	i.labelWidget.SetStyleSheet(fmt.Sprintf(" * { background-color: %s; color: %s; }", wsSideitemActiveBgColor.print(), fg.print()))
	svgOpenContent := editor.getSvg("chevron-down", fg)
	i.openIcon.Load2(core.NewQByteArray2(svgOpenContent, len(svgOpenContent)))
	svgCloseContent := editor.getSvg("chevron-right", fg)
	i.closeIcon.Load2(core.NewQByteArray2(svgCloseContent, len(svgCloseContent)))

	reloadFilelist := i.cwdpath != i.Filelist.cwdpath

	if reloadFilelist && editor.activity.editItem.active {
		filelist := newFilelistwidget(i.cwdpath)
		i.setFilelistwidget(filelist)
	}
	if !i.isFilelistHide {
		i.Filelistwidget.Show()
	} else {
		i.Filelistwidget.Hide()
	}
}

func (i *WorkspaceSideItem) setInactive() {
	if !i.active {
		return
	}
	if editor.fgcolor == nil || editor.bgcolor == nil {
		return
	}
	i.active = false
	bg := editor.bgcolor
	fg := editor.fgcolor
	i.labelWidget.SetStyleSheet(fmt.Sprintf(" * { background-color: %s; color: %s; }", shiftColor(bg, -5).print(), shiftColor(fg, 0).print()))
	svgOpenContent := editor.getSvg("chevron-down", fg)
	i.openIcon.Load2(core.NewQByteArray2(svgOpenContent, len(svgOpenContent)))
	svgCloseContent := editor.getSvg("chevron-right", fg)
	i.closeIcon.Load2(core.NewQByteArray2(svgCloseContent, len(svgCloseContent)))

	i.Filelistwidget.Hide()
}

func (i *WorkspaceSideItem) show() {
	if !i.hidden {
		return
	}
	i.hidden = false
	i.label.Show()
	if !i.isFilelistHide {
		i.Filelistwidget.Show()
		i.openIcon.Show()
		i.closeIcon.Hide()
	} else {
		i.Filelistwidget.Hide()
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
	i.Filelistwidget.Hide()
	i.openIcon.Hide()
	i.closeIcon.Hide()
}

func (w *Workspace) setGuiColor(fg *RGBA, bg *RGBA) {
	if fg == nil || bg == nil {
		return
	}
	if w.isSetGuiColor == true {
		return
	}
	w.isSetGuiColor = true

	activityBarColor := shiftColor(bg, -8)
	sideBarColor := shiftColor(bg, -5)
	STRONGFg := warpColor(fg, 15)
	strongFg := warpColor(fg, 10)
	weakBg := gradColor(bg)
	//weakFg := gradColor(fg)
	//darkerBg := shiftColor(bg, 10)

	tablineFgColor := gradColor(fg)
	tablineBgColor := shiftColor(bg, 10)

	statuslineFolderLabelColor := gradColor(fg)
	statuslineBorderColor := bg
	statuslineBgColor := bg
	statuslineFgColor := strongFg

	scrollBarThumbColor := weakBg
	scrollBarColor := bg

	paletteFgColor := shiftColor(fg, -5)
	paletteBorderColor := shiftColor(bg, -8)
	paletteBgColor := shiftColor(bg, -8)
	paletteLightBgColor := shiftColor(bg, -25)

	popFgColor := shiftColor(fg, 5)
	popFgDetailColor := gradColor(fg)
	popBgColor := shiftColor(bg, 15)
	popScrollBarColor := gradColor(bg)

	locFgColor := shiftColor(fg, 5)
	locBorderColor := shiftColor(bg, 20)
	locBgColor := shiftColor(bg, 10)

	signatureFgColor := gradColor(fg)
	signatureBorderColor := shiftColor(bg, -5)
	signatureBgColor := shiftColor(bg, -7)

	tooltipFgColor := shiftColor(fg, -40)
	tooltipBgColor := weakBg

	wsHeaderColor := fg
	wsSideColor := gradColor(fg)
	wsSideBorderColor := shiftColor(bg, 10)
	wsSideBgColor := shiftColor(bg, -5)

	wsSideScrollBarHandleColor := gradColor(bg)

	wsSideitemActiveBgColor := warpColor(bg, -6)

	// for splitter
	editor.splitter.SetStyleSheet(fmt.Sprintf(" QSplitter::handle:horizontal { background-color: %s; }", sideBarColor.print()))

	// for Activity Bar
	editor.activity.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: %s; } ", activityBarColor.print()))

	var svgEditContent string
	if editor.activity.editItem.active == true {
		svgEditContent = editor.getSvg("activityedit", STRONGFg)
	} else {
		svgEditContent = editor.getSvg("activityedit", weakBg)
	}
	editor.activity.editItem.icon.Load2(core.NewQByteArray2(svgEditContent, len(svgEditContent)))

	var svgDeinContent string
	if editor.activity.deinItem.active == true {
		svgDeinContent = editor.getSvg("activitydein", STRONGFg)
	} else {
		svgDeinContent = editor.getSvg("activitydein", weakBg)
	}
	editor.activity.deinItem.icon.Load2(core.NewQByteArray2(svgDeinContent, len(svgDeinContent)))

	// tab
	w.tabline.widget.SetStyleSheet(fmt.Sprintf(".QWidget { border-left: 8px solid %s; border-bottom: 0px solid; border-right: 0px solid; background-color: %s; } QWidget { color: %s; } ", tablineBgColor.print(), tablineBgColor.print(), tablineFgColor.print()))

	// statusline
	if editor.config.Statusline.ModeIndicatorType != "background" {
		w.statusline.main.folderLabel.SetStyleSheet(fmt.Sprintf("color: %s;", statuslineFolderLabelColor.print()))
		w.statusline.widget.SetStyleSheet(fmt.Sprintf("QWidget#statusline {	border-top: 0px solid %s; background-color: %s; } * { color: %s; }", statuslineBorderColor.print(), statuslineBgColor.print(), statuslineFgColor.print()))

		svgContent := editor.getSvg("git", statuslineFgColor)
		w.statusline.git.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		svgContent = editor.getSvg("bell", statuslineFgColor)
		w.statusline.notify.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}

	// scrollBar
	w.scrollBar.thumb.SetStyleSheet(fmt.Sprintf(" * { background: %s;}", scrollBarThumbColor.print()))
	w.scrollBar.widget.SetStyleSheet(fmt.Sprintf(" * { background: %s;}", scrollBarColor.print()))

	w.minimap.curRegion.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(%d, %d, %d, 35);}", gradColor(fg).R, gradColor(fg).G, gradColor(fg).B))

	// for Gonvim UI Color form colorscheme
	w.palette.cursor.SetStyleSheet(fmt.Sprintf("background-color: %s;", paletteFgColor.print()))
	w.palette.widget.SetStyleSheet(fmt.Sprintf(" QWidget#palette { border: 1px solid %s; } .QWidget { background-color: %s; } * { color: %s; } ", paletteBorderColor.print(), paletteBgColor.print(), paletteFgColor.print()))
	w.palette.scrollBar.SetStyleSheet(fmt.Sprintf("background-color: %s;", paletteLightBgColor.print()))
	w.palette.pattern.SetStyleSheet(fmt.Sprintf("background-color: %s;", paletteLightBgColor.print()))

	// popup
	w.popup.scrollBar.SetStyleSheet(fmt.Sprintf("background-color: %s;", popScrollBarColor.print()))
	w.popup.widget.SetStyleSheet(fmt.Sprintf("* {background-color: %s; color: %s;} #detailpopup { color: %s; }", popBgColor.print(), popFgColor.print(), popFgDetailColor.print()))

	// loc
	w.loc.widget.SetStyleSheet(fmt.Sprintf(".QWidget { border: 1px solid %s; } * { background-color: %s;  color: %s; }", locBorderColor.print(), locBgColor.print(), locFgColor.print()))

	// signature
	w.signature.widget.SetStyleSheet(fmt.Sprintf(".QWidget { border: 1px solid %s; } QWidget { background-color: %s; } * { color: %s; }", signatureBorderColor.print(), signatureBgColor.print(), signatureFgColor.print()))

	// screan tooltip
	w.screen.tooltip.SetStyleSheet(fmt.Sprintf(" * {background-color: %s; text-decoration: underline; color: %s; }", tooltipBgColor.print(), tooltipFgColor.print()))

	// for Workspaceside
	editor.wsSide.header.SetStyleSheet(fmt.Sprintf(" .QLabel{ color: %s;} ", wsHeaderColor.print()))
	editor.wsSide.widget.SetStyleSheet(fmt.Sprintf(".QWidget { border-color: %s; padding-top: 5px; background-color: %s; } QWidget { color: %s; border-right: 0px solid; }", wsSideBorderColor.print(), wsSideBgColor.print(), wsSideColor.print()))
	editor.wsSide.scrollarea.SetStyleSheet(fmt.Sprintf(".QScrollBar { border-width: 0px; background-color: %s; width: 5px; margin: 0 0 0 0; } .QScrollBar::handle:vertical {background-color: %s; min-height: 25px;} .QScrollBar::handle:vertical:hover {background-color: %s; min-height: 25px;} .QScrollBar::add-line:vertical, .QScrollBar::sub-line:vertical { border: none; background: none; } .QScrollBar::add-page:vertical, QScrollBar::sub-page:vertical { background: none; }", wsSideBgColor.print(), wsSideScrollBarHandleColor.print(), editor.config.SideBar.AccentColor))

	if len(editor.workspaces) == 1 {
		editor.wsSide.items[0].active = true
		editor.wsSide.items[0].labelWidget.SetStyleSheet(fmt.Sprintf(" * { background-color: %s; color: %s; }", wsSideitemActiveBgColor.print(), fg.print()))
	}

	editor.window.SetWindowOpacity(1.0)
}
