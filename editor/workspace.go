package editor

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akiyosi/gonvim/fuzzy"
	"github.com/akiyosi/gonvim/util"
	shortpath "github.com/akiyosi/short_path"
	"github.com/jessevdk/go-flags"
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
	fpalette   *Palette
	popup      *PopupMenu
	loc        *Locpopup
	cmdline    *Cmdline
	signature  *Signature
	message    *Message
	minimap    *MiniMap
	width      int
	height     int
	hidden     bool

	nvim               *nvim.Nvim
	rows               int
	cols               int
	uiAttached         bool
	uiRemoteAttached   bool
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
	isInTerm           bool
	cursorStyleEnabled bool
	modeInfo           []map[string]interface{}
	ts                 int

	signal        *workspaceSignal
	redrawUpdates chan [][]interface{}
	guiUpdates    chan []interface{}
	doneNvimStart chan bool
	stopOnce      sync.Once
	stop          chan struct{}

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
	w.font = initFontNew(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, editor.config.Editor.Linespace)
	go func() {
		height := w.font.fontMetrics.Height()
		width := w.font.fontMetrics.HorizontalAdvance("W", -1)
		ascent := w.font.fontMetrics.Ascent()
		w.font.height = int(math.Ceil(height))
		w.font.width = int(math.Ceil(width))
		w.font.truewidth = width
		w.font.ascent = ascent
		w.font.lineHeight = w.font.height + w.font.lineSpace
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

	go w.startNvim(path)
	w.registerSignal()

	w.screen = newScreen()
	w.screen.ws = w
	w.palette = initPalette()
	w.palette.ws = w
	w.fpalette = initPalette()
	w.fpalette.ws = w

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
	w.popup = initPopupmenuNew(w.font)
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

	go func() {
		if !editor.config.MiniMap.Visible {
			time.Sleep(1500 * time.Millisecond)
		}
		w.minimap.startMinimapProc()
	}()

	if runtime.GOOS == "windows" {
		select {
		case <-w.doneNvimStart:
		}
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
	var opts struct {
		ServerPtr string `long:"server" description:"Remote session address"`
	}
	args, _ := flags.ParseArgs(&opts, os.Args[1:])
	var neovim *nvim.Nvim
	var err error
	if opts.ServerPtr != "" {
		neovim, err = nvim.Dial(opts.ServerPtr)
		w.uiRemoteAttached = true
	} else {
		neovim, err = nvim.NewChildProcess(
			nvim.ChildProcessArgs(
				append([]string{
					"--cmd",
					"let g:gonvim_running=1",
					"--embed",
				}, args...)...,
			))
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
	// w.initCwd()
	w.loadGinitVim()
}

func (w *Workspace) configure() {
	if editor.config.Statusline.Visible {
		w.drawStatusline = true
	} else {
		w.drawStatusline = false
	}

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

	if editor.config.Editor.StartFullscreen {
		editor.window.ShowFullScreen()
	}
}

func (w *Workspace) attachUI(path string) error {
	w.nvim.Subscribe("Gui")
	go w.initGonvim()
	if path != "" {
		go w.nvim.Command("so " + path)
	}
	w.tabline.subscribe()
	w.statusline.subscribe()
	w.loc.subscribe()
	w.message.subscribe()
	fuzzy.RegisterPlugin(w.nvim, w.uiRemoteAttached)

	w.uiAttached = true
	err := w.nvim.AttachUI(w.cols, w.rows, w.attachUIOption())
	if err != nil {
		fmt.Println(err)
		editor.close()
		return err
	}

	return nil
}

func (w *Workspace) initGonvim() {
	gonvimAutoCmds := `
	aug GonvimAu | au! | aug END
	au GonvimAu VimEnter * call rpcnotify(1, "Gui", "gonvim_enter", getcwd())
	au GonvimAu VimLeavePre * call rpcnotify(1, "Gui", "gonvim_exit")
	au GonvimAu CursorMoved,CursorMovedI * call rpcnotify(0, "Gui", "gonvim_cursormoved", getpos("."))
	au GonvimAu TermOpen * call rpcnotify(0, "Gui", "gonvim_termopen")
	au GonvimAu TermClose * call rpcnotify(0, "Gui", "gonvim_termclose")
	aug GonvimAuWorkspace | au! | aug END
	au GonvimAuWorkspace DirChanged * call rpcnotify(0, "Gui", "gonvim_workspace_cwd", getcwd())
	aug GonvimAuMd | au! | aug END
	au GonvimAuMd TextChanged,TextChangedI *.md call rpcnotify(0, "Gui", "gonvim_markdown_update")
	au GonvimAuMd BufEnter *.md call rpcnotify(0, "Gui", "gonvim_markdown_new_buffer")
	aug GonvimAuFileExplorer | au! | aug END
	au GonvimAuFileExplorer BufEnter,TabEnter,DirChanged,TermOpen,TermClose * call rpcnotify(0, "Gui", "gonvim_workspace_setCurrentFileLabel", expand("%:p"))
	`
	if !w.uiRemoteAttached {
		gonvimAutoCmds = gonvimAutoCmds + `
		au GonvimAuFileExplorer TextChanged,TextChangedI,BufEnter,BufWrite,DirChanged * call rpcnotify(0, "Gui", "gonvim_workspace_updateMoifiedbadge")
		aug GonvimAuMinimap | au! | aug END
		au GonvimAuMinimap BufEnter,BufWrite * call rpcnotify(0, "Gui", "gonvim_minimap_update")
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
	au GonvimAuStatusline BufEnter,OptionSet,TermOpen,TermClose * call rpcnotify(0, "statusline", "bufenter", &filetype, &fileencoding, &fileformat, &ro)
	`
	}

	isLintEnable := false
	for _, rightItem := range editor.config.Statusline.Right {
		if rightItem == "lint" {
			isLintEnable = true
			break
		}
	}
	for _, leftItem := range editor.config.Statusline.Left {
		if leftItem == "lint" {
			isLintEnable = true
			break
		}
	}
	isLintEnable = isLintEnable || editor.config.Lint.Visible

	if isLintEnable {
		gonvimAutoCmds = gonvimAutoCmds + `
	aug GonvimAuLint | au! | aug END
	au GonvimAuLint CursorMoved,CursorHold,InsertEnter,InsertLeave * call rpcnotify(0, "LocPopup", "update")
	`
	}
	registerAutocmds := fmt.Sprintf(`call execute(%s)`, splitVimscript(gonvimAutoCmds))
	w.nvim.Command(registerAutocmds)

	gonvimCommands := fmt.Sprintf(`
	command! GonvimMarkdown call rpcnotify(0, "Gui", "gonvim_markdown_toggle")
	command! GonvimVersion echo "%s"`, editor.version)
	if !w.uiRemoteAttached {
		gonvimCommands = gonvimCommands + `
	command! GonvimWorkspaceNew call rpcnotify(0, "Gui", "gonvim_workspace_new")
	command! GonvimWorkspaceNext call rpcnotify(0, "Gui", "gonvim_workspace_next")
	command! GonvimWorkspacePrevious call rpcnotify(0, "Gui", "gonvim_workspace_previous")
	command! -nargs=1 GonvimWorkspaceSwitch call rpcnotify(0, "Gui", "gonvim_workspace_switch", <args>)
	command! GonvimMiniMap call rpcnotify(0, "Gui", "gonvim_minimap_toggle")
	`
	}

	registerCommands := fmt.Sprintf(`call execute(%s)`, splitVimscript(gonvimCommands))
	w.nvim.Command(registerCommands)

	gonvimInitNotify := `
	call rpcnotify(0, "statusline", "bufenter", expand("%:p"), &filetype, &fileencoding, &fileformat, &ro)
	call rpcnotify(0, "Gui", "gonvim_cursormoved",  getpos("."))
	`
	if !w.uiRemoteAttached {
		gonvimInitNotify = gonvimInitNotify + `
		call rpcnotify(0, "Gui", "gonvim_workspace_updateMoifiedbadge")
		call rpcnotify(0, "Gui", "gonvim_minimap_update")
		`
	}
	initialNotify := fmt.Sprintf(`call execute(%s)`, splitVimscript(gonvimInitNotify))
	w.nvim.Command(initialNotify)
}

func splitVimscript(s string) string {
	listLines := "["
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		listLines = listLines + `'` + line + `'`
		if i == len(lines)-1 {
			listLines = listLines + "]"
		} else {
			listLines = listLines + ","
		}
	}

	return listLines
}

func (w *Workspace) loadGinitVim() {
	if editor.config.Editor.GinitVim != "" {
		scripts := strings.NewReplacer("\r\n", "\n", "\r", "\n", "\n", "\n").Replace(editor.config.Editor.GinitVim)
		execGinitVim := fmt.Sprintf(`call execute(split('%s', '\n'))`, scripts)
		w.nvim.Command(execGinitVim)
	}

	ts := 8
	w.nvim.Option("ts", &ts)
	w.ts = ts
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
	// cwdITF, err := w.nvimEval("getcwd()")
	// if err != nil {
	// 	return
	// }
	// cwd := cwdITF.(string)
	// if cwd == "" {
	// 	return
	// }
	if w.cwd == "" {
		return
	}
	w.nvim.Command("cd " + w.cwd)
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
			if sideItem.cwdpath == path && sideItem.isload {
				continue
			}

			sideItem.label.SetText(w.cwdlabel)
			sideItem.label.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))
			sideItem.cwdpath = path

			// if editor.activity.editItem.active == false {
			// 	continue
			// }
			if editor.wsSide.scrollarea == nil {
				continue
			}

			filelist, err := newFilelist(path)
			if err != nil {
				continue
			}
			sideItem.setFilelistwidget(filelist)
		}
	}
}

func (i *WorkspaceSideItem) setFilelistwidget(f *Filelist) {
	i.mu.Lock()
	defer i.mu.Unlock()

	// if i.Filelistwidget != nil {
	// 	i.Filelistwidget.DestroyQWidget()
	// }
	// i.layout.AddWidget(f.widget, 0, 0)

	i.Filelistwidget.Hide()
	i.layout.RemoveWidget(i.Filelistwidget)
	i.layout.AddWidget(f.widget, 0, 0)

	i.Filelistwidget = f.widget
	i.Filelist = f
	i.Filelist.WSitem = i
	i.active = true
	i.isload = true

	if i.isFilelistHide {
		i.closeFilelist()
	} else {
		i.openFilelist()
	}

	editor.wsSide.scrollarea.ConnectResizeEvent(func(*gui.QResizeEvent) {
		// if editor.activity.editItem.active == false {
		// 	return
		// }
		if !i.isload {
			return
		}
		if len(i.Filelist.Fileitems) == 0 {
			return
		}

		width := editor.wsSide.scrollarea.Width()
		charWidth := int(editor.workspaces[editor.active].font.defaultFontMetrics.HorizontalAdvance("W", -1))
		length := float64(width - (2 * editor.iconSize) - FilewidgetLeftMargin - charWidth - 35)

		for _, item := range editor.wsSide.items {
			item.label.SetMaximumWidth(width)
			item.label.SetMinimumWidth(width)
			for _, fileitem := range item.Filelist.Fileitems {
				fileitem.widget.SetMaximumWidth(width)
				fileitem.widget.SetMinimumWidth(width)
				fileitem.setFilename(length)
			}
		}

	})
}

// slow...
func (i *WorkspaceSideItem) addFilewidget(f *Fileitem) {
	// f.makeWidget()
	// i.Filelist.Fileitems = append(i.Filelist.Fileitems, f)
	// i.Filelist.widget.Layout().AddWidget(f.widget)

	i.Filelist.Fileitems = append(i.Filelist.Fileitems, f)
	i.Filelist.widget.Layout().AddWidget(f.widget)
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
						case "cmdline_show" :
							o["ext_cmdline"] = editor.config.Editor.ExtCmdline
						case "msg_show":
							o["ext_messages"] = editor.config.Editor.ExtMessage
						case "popupmenu_show" :
							o["ext_popupmenu"] = editor.config.Editor.ExtPopupmenu
						case "tabline_update" :
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
			s.update()
			w.cursor.update()

		// Grid Events
		case "grid_resize":
			s.gridResize(args)
		case "default_colors_set":
			args := update[1].([]interface{})
			w.setColorsSet(args)
		case "hl_attr_define":
			s.setHighAttrDef(args)
		case "hl_group_set":
		case "grid_line":
			s.gridLine(args)
		case "grid_clear":
			s.gridClear(args)
		case "grid_destroy":
			s.gridDestroy(args)
		case "grid_cursor_goto":
			s.gridCursorGoto(args)
			if w.minimap.visible {
				go w.updateMinimap()
				w.minimap.mapScroll()
			}
		case "grid_scroll":
			s.gridScroll(args)

		// Multigrid Events
		case "win_pos":
			s.windowPosition(args)
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

	w.drawOtherUI()
}

func (w *Workspace) drawOtherUI() {
	s := w.screen

	if w.statusline.widget.IsVisible() {
		w.statusline.mode.redraw()
	}
	if s.tooltip.IsVisible() {
		x, y, _, _ := w.screen.toolTipPos()
		w.screen.toolTipMove(x, y)
	}
	if editor.config.ScrollBar.Visible {
		w.scrollBar.update()
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

	w.minimap.foreground = w.foreground
	w.minimap.background = w.background
	w.minimap.special = w.special

	var isChangeFg, isChangeBg bool
	if editor.colors.fg != nil {
		isChangeFg = editor.colors.fg.equals(w.foreground)
	}
	if editor.colors.bg != nil {
		isChangeBg = editor.colors.bg.equals(w.background)
	}
	if !isChangeFg || !isChangeBg {
		editor.isSetGuiColor = false
	}
	// Ignore setting GUI color when create second workspace and fg, bg equals -1
	if len(editor.workspaces) > 1 && fg == -1 && bg == -1 {
		w.updateWorkspaceColor()
		editor.isSetGuiColor = true
	}
	if editor.isSetGuiColor == true {
		return
	}
	editor.colors.fg = newRGBA(w.foreground.R, w.foreground.G, w.foreground.B, 1)
	editor.colors.bg = newRGBA(w.background.R, w.background.G, w.background.B, 1)
	//w.setGuiColor(editor.colors.fg, editor.colors.bg)
	editor.colors.update()
	editor.updateGUIColor()
	editor.isSetGuiColor = true
}

func (w *Workspace) updateWorkspaceColor() {
	w.palette.setColor()
	w.fpalette.setColor()
	w.popup.setColor()
	w.signature.setColor()
	w.tabline.setColor()
	w.statusline.setColor()
	w.scrollBar.setColor()
	w.minimap.setColor()
	w.loc.setColor()
	w.message.setColor()
	w.screen.setColor()
}

func (w *Workspace) modeInfoSet(args []interface{}) {
	for _, arg := range args {
		w.cursorStyleEnabled = arg.([]interface{})[0].(bool)
		modePropList := arg.([]interface{})[1].([]interface{})
		w.modeInfo = make([]map[string]interface{}, len(modePropList))
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

func (w *Workspace) updateMinimap() {
	var absMapTop int
	var absMapBottom int
	w.minimap.nvim.Eval("line('w0')", &absMapTop)
	w.minimap.nvim.Eval("line('w$')", &absMapBottom)
	w.minimap.nvim.Command(fmt.Sprintf("call cursor(%d, %d)", w.curLine, 0))
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
	case "gonvim_exit":
		editor.workspaces[editor.active].minimap.exit()
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
	case "gonvim_cursormoved":
		pos := updates[1].([]interface{})
		ln := util.ReflectToInt(pos[1])
		col := util.ReflectToInt(pos[2]) + util.ReflectToInt(pos[3])
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
	case "gonvim_workspace_setCurrentFileLabel":
		file := updates[1].(string)
		w.filepath = file
		if w.uiRemoteAttached {
			return
		}
		if editor.wsSide == nil {
			return
		}
		if strings.Contains(w.filepath, "[denite]") || w.filepath == "" {
			return
		}
		editor.wsSide.items[editor.active].setCurrentFileLabel()
	case "gonvim_workspace_updateMoifiedbadge":
		if editor.wsSide == nil {
			return
		}
		if strings.Contains(w.filepath, "[denite]") || w.filepath == "" {
			return
		}
		fl := editor.wsSide.items[editor.active].Filelist
		if fl.active != -1 {
			if len(fl.Fileitems) != 0 {
				fl.Fileitems[fl.active].updateModifiedbadge()
			}
		}
	case "gonvim_termopen":
		w.isInTerm = true
		w.detectTerminalMode()
	case "gonvim_termclose":
		w.isInTerm = false
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

func (w *Workspace) guiFont(args string) {
	if args == "" {
		return
	}
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
	w.message.updateFont(w.font)
	w.screen.toolTipFont(w.font)
	w.cursor.widget.SetFont(w.font.fontNew)
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

func (w *Workspace) detectTerminalMode() {
	// Note: I'm waiting for the merge of neovim/neovim#8550
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
	for i := 0; i < 20; i++ {
		item := newWorkspaceSideItem()
		side.items = append(side.items, item)
		side.items[len(side.items)-1].side = side
		layout.AddWidget(side.items[len(side.items)-1].widget)
		side.items[len(side.items)-1].hide()
	}

	return side
}

func (s *WorkspaceSide) newScrollArea() {
	sideArea := widgets.NewQScrollArea(nil)
	sideArea.SetWidgetResizable(true)
	sideArea.SetVerticalScrollBarPolicy(core.Qt__ScrollBarAlwaysOff)
	sideArea.ConnectEnterEvent(func(event *core.QEvent) {
		sideArea.SetVerticalScrollBarPolicy(core.Qt__ScrollBarAsNeeded)
	})
	sideArea.ConnectLeaveEvent(func(event *core.QEvent) {
		sideArea.SetVerticalScrollBarPolicy(core.Qt__ScrollBarAlwaysOff)
	})
	sideArea.SetFocusPolicy(core.Qt__ClickFocus)
	sideArea.SetFrameShape(widgets.QFrame__NoFrame)

	s.scrollarea = sideArea
	s.scrollarea.SetWidget(s.widget)
}

type filelistSignal struct {
	core.QObject
	_ func() `signal:"filelistUpdateSignal"`
}

// WorkspaceSideItem is
type WorkspaceSideItem struct {
	signal         *filelistSignal
	filelistUpdate chan *Fileitem
	mu             sync.Mutex

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
	widget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0); }")

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
		filelistUpdate: make(chan *Fileitem, 20000),
		signal:         NewFilelistSignal(nil),

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
	sideitem.signal.ConnectFilelistUpdateSignal(func() {
		update := <-sideitem.filelistUpdate
		sideitem.addFilewidget(update)
	})

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

func (s *WorkspaceSide) setColor() {
	fg := editor.colors.sideBarFg.String()
	sfg := editor.colors.scrollBarFg.String()
	sbg := editor.colors.scrollBarBg.StringTransparent()
	s.header.SetStyleSheet(fmt.Sprintf(" .QLabel{ color: %s;} ", fg))
	s.widget.SetStyleSheet(fmt.Sprintf(".QWidget { border: 0px solid #000; padding-top: 5px; background-color: rgba(0, 0, 0, 0); } QWidget { color: %s; border-right: 0px solid; }", fg))
	if s.scrollarea == nil {
		return
	}
	s.scrollarea.SetStyleSheet(fmt.Sprintf(".QScrollBar { border-width: 0px; background-color: %s; width: 5px; margin: 0 0 0 0; } .QScrollBar::handle:vertical {background-color: %s; min-height: 25px;} .QScrollBar::handle:vertical:hover {background-color: %s; min-height: 25px;} .QScrollBar::add-line:vertical, .QScrollBar::sub-line:vertical { border: none; background: none; } .QScrollBar::add-page:vertical, QScrollBar::sub-page:vertical { background: none; }", sbg, sfg, editor.config.SideBar.AccentColor))

	if len(editor.workspaces) == 1 {
		s.items[0].active = true
		s.items[0].labelWidget.SetStyleSheet(fmt.Sprintf(" * { background-color: %s; color: %s; }", editor.colors.sideBarSelectedItemBg, fg))
	}
}

func (i *WorkspaceSideItem) setActive() {
	if editor.colors.fg == nil || editor.colors.bg == nil {
		return
	}
	if editor.wsSide.scrollarea == nil {
		return
	}
	i.active = true
	bg := editor.colors.sideBarSelectedItemBg
	fg := editor.colors.fg
	transparent := transparent()
	i.labelWidget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(%d, %d, %d, %f); color: %s; }", bg.R, bg.G, bg.B, transparent, fg.String()))
	svgOpenContent := editor.getSvg("chevron-down", fg)
	i.openIcon.Load2(core.NewQByteArray2(svgOpenContent, len(svgOpenContent)))
	svgCloseContent := editor.getSvg("chevron-right", fg)
	i.closeIcon.Load2(core.NewQByteArray2(svgCloseContent, len(svgCloseContent)))

	reloadFilelist := i.cwdpath != i.Filelist.cwdpath

	if reloadFilelist {
		filelist, err := newFilelist(i.cwdpath)
		if err != nil {
			return
		}
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
	if editor.colors.fg == nil || editor.colors.bg == nil {
		return
	}
	i.active = false
	fg := editor.colors.inactiveFg
	i.labelWidget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(0, 0, 0, 0); color: %s; }", fg.String()))
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
