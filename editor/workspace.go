package editor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

	_ func() `signal:"stopSignal"`
	_ func() `signal:"redrawSignal"`
	_ func() `signal:"guiSignal"`

	// _ func() `signal:"locpopupSignal"`

	_ func() `signal:"statuslineSignal"`
	_ func() `signal:"lintSignal"`
	_ func() `signal:"gitSignal"`

	_ func() `signal:"messageSignal"`

	_ func() `signal:"markdownSignal"`

	_ func() `signal:"lazyDrawSignal"`
}

// Workspace is an editor workspace
type Workspace struct {
	widget    *widgets.QWidget
	layout2   *widgets.QHBoxLayout
	hasLazyUI bool

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
	// loc        *Locpopup
	cmdline   *Cmdline
	signature *Signature
	message   *Message
	minimap   *MiniMap

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
	viewport           [4]int    // topline, botline, curline, curcol
	oldViewport        [4]int    // topline, botline, curline, curcol
	scrollViewport     [2][5]int // 1. topline, botline, curline, curcol, grid, 2. oldtopline, oldbotline, oldcurline, oldcurcol, oldgrid
	viewportQue        chan [5]int
	curPosMutex        sync.RWMutex
	optionsetMutex     sync.RWMutex
	cursorStyleEnabled bool
	modeInfo           []map[string]interface{}
	normalMappings     []*nvim.Mapping
	insertMappings     []*nvim.Mapping
	ts                 int
	ph                 int
	pb                 int
	showtabline        int
	api5               bool

	escKeyInNormal     string
	escKeyInInsert     string
	isMappingScrollKey bool

	signal        *workspaceSignal
	redrawUpdates chan [][]interface{}
	guiUpdates    chan []interface{}
	stopOnce      sync.Once
	stop          chan struct{}
	fontMutex     sync.Mutex

	drawStatusline bool
	drawTabline    bool
	drawLint       bool
}

func newWorkspace(path string) (*Workspace, error) {
	editor.putLog("initialize workspace")
	w := &Workspace{
		stop:          make(chan struct{}),
		signal:        NewWorkspaceSignal(nil),
		redrawUpdates: make(chan [][]interface{}, 1000),
		guiUpdates:    make(chan []interface{}, 1000),
		viewportQue:   make(chan [5]int, 99),
		foreground:    newRGBA(255, 255, 255, 1),
		background:    newRGBA(0, 0, 0, 1),
		special:       newRGBA(255, 255, 255, 1),
	}
	w.registerSignal()

	w.font = editor.font
	w.font.ws = w

	// Basic Workspace UI component
	// screen
	w.screen = newScreen()
	w.screen.ws = w
	w.screen.font = w.font
	w.screen.initInputMethodWidget()

	// cursor
	w.cursor = initCursorNew()
	w.cursor.ws = w

	// If ExtFooBar is true, then we create a UI component
	// tabline
	if editor.config.Editor.ExtTabline {
		w.tabline = initTabline()
		w.tabline.ws = w
	}

	// cmdline
	if editor.config.Editor.ExtCmdline {
		w.cmdline = initCmdline()
		w.cmdline.ws = w
	}

	// popupmenu
	if editor.config.Editor.ExtPopupmenu {
		w.popup = initPopupmenuNew()
		w.popup.widget.SetParent(editor.widget)
		w.popup.ws = w
		w.popup.widget.Hide()
		// w.signature.widget.Hide()
	}

	// messages
	if editor.config.Editor.ExtMessages {
		w.message = initMessage()
		w.message.ws = w
		w.message.widget.SetParent(editor.window)
	}

	// If Statusline.Visible is true, then we create statusline UI component
	if editor.config.Statusline.Visible {
		w.statusline = initStatusline()
		w.statusline.ws = w
	}

	// // Lint
	// if editor.config.Lint.Visible {
	// 	w.loc = initLocpopup()
	// 	w.loc.ws = w
	// 	w.loc.widget.SetParent(editor.widget)
	// 	w.loc.widget.Hide()
	// }

	// w.signature = initSignature()
	// w.signature.widget.SetParent(editor.widget)
	// w.signature.ws = w

	editor.putLog("initialazed UI components")

	// workspace widget, layouts
	layout := widgets.NewQVBoxLayout()
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(0)
	w.widget = widgets.NewQWidget(nil, 0)
	w.widget.SetContentsMargins(0, 0, 0, 0)
	w.widget.SetLayout(layout)
	w.widget.SetFocusPolicy(core.Qt__WheelFocus)
	w.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	w.widget.ConnectInputMethodEvent(w.InputMethodEvent)
	w.widget.ConnectInputMethodQuery(w.InputMethodQuery)

	// screen widget and scrollBar widget
	widget2 := widgets.NewQWidget(nil, 0)
	widget2.SetContentsMargins(0, 0, 0, 0)
	widget2.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	w.layout2 = widgets.NewQHBoxLayout()
	w.layout2.SetContentsMargins(0, 0, 0, 0)
	w.layout2.SetSpacing(0)
	w.layout2.AddWidget(w.screen.widget, 0, 0)
	widget2.SetLayout(w.layout2)

	// assemble all neovim ui components
	if editor.config.Editor.ExtTabline {
		layout.AddWidget(w.tabline.widget, 0, 0)
	}
	layout.AddWidget(widget2, 1, 0)
	if editor.config.Statusline.Visible {
		layout.AddWidget(w.statusline.widget, 0, 0)
	}

	w.widget.SetParent(editor.widget)
	w.widget.Move2(0, 0)
	editor.putLog("assembled UI components")

	go w.startNvim(path)

	return w, nil
}

func (w *Workspace) lazyDrawUI() {
	editor.putLog("Start    preparing for deferred drawing UI")

	// scrollbar
	if editor.config.ScrollBar.Visible {
		w.scrollBar = newScrollBar()
		w.scrollBar.ws = w
	}

	// minimap
	if !editor.config.MiniMap.Disable {
		w.minimap = newMiniMap()
		w.minimap.ws = w
	}

	w.layout2.AddWidget(w.minimap.widget, 0, 0)
	if editor.config.ScrollBar.Visible {
		w.layout2.AddWidget(w.scrollBar.widget, 0, 0)
		w.scrollBar.setColor()
	}

	// palette
	w.palette = initPalette()
	w.palette.ws = w
	w.palette.widget.SetParent(editor.window)
	w.palette.setColor()
	w.palette.hide()

	// palette 2
	w.fpalette = initPalette()
	w.fpalette.ws = w
	w.fpalette.widget.SetParent(editor.window)
	w.fpalette.setColor()
	w.fpalette.hide()

	// finder
	w.finder = initFinder()
	w.finder.ws = w

	// set current working directory
	w.setCwd(w.getCwd())

	// Add editor feature
	go fuzzy.RegisterPlugin(w.nvim, w.uiRemoteAttached)
	go filer.RegisterPlugin(w.nvim)

	// markdown
	if !editor.config.Markdown.Disable {
		w.markdown = newMarkdown(w)
		w.markdown.webview.SetParent(w.screen.widget)
	}

	// Asynchronously execute the process for minimap
	go func() {
		if !w.uiRemoteAttached && !editor.config.MiniMap.Disable {
			w.minimap.startMinimapProc()
			time.Sleep(time.Millisecond * 50)
			w.minimap.mu.Lock()
			isMinimapVisible := w.minimap.visible
			w.minimap.mu.Unlock()
			if isMinimapVisible {
				w.minimap.setCurrentRegion()
				w.minimap.bufUpdate()
				w.minimap.bufSync()
			}
		}
	}()

	editor.putLog("Finished preparing the deferred drawing UI.")
}

func (w *Workspace) vimEnterProcess() {
	// Show window if connect remote nvim via ssh
	if editor.opts.Ssh != "" {
		editor.window.Show()
	}

	// get nvim option
	w.getNvimOptions()

	// connect window resize event
	editor.widget.ConnectResizeEvent(func(event *gui.QResizeEvent) {
		for _, ws := range editor.workspaces {
			ws.updateSize()
		}
	})

	w.widget.ConnectFocusInEvent(func(event *gui.QFocusEvent) {
		go w.nvim.Command("if exists('#FocusGained') | doautocmd <nomodeline> FocusGained | endif")
	})
	w.widget.ConnectFocusOutEvent(func(event *gui.QFocusEvent) {
		go w.nvim.Command("if exists('#FocusLost') | doautocmd <nomodeline> FocusLost | endif")
	})

	go func() {

		time.Sleep(time.Millisecond * 500)
		w.signal.LazyDrawSignal()

		time.Sleep(time.Millisecond * 400)
		editor.signal.SidebarSignal()

	}()
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
	w.signal.ConnectLazyDrawSignal(func() {
		if w.hasLazyUI {
			return
		}
		w.hasLazyUI = true
		w.lazyDrawUI()
	})

	// // for debug signal
	// z := 1
	// go func() {
	// 	for {
	// 		w.redrawUpdates <- [][]interface{}{[]interface{}{"test event " + fmt.Sprintf("%d", z)}}
	// 		w.signal.RedrawSignal()
	// 		z++
	// 		time.Sleep(time.Millisecond * 50)
	// 	}
	// }()

	w.signal.ConnectStopSignal(func() {
		// if !w.uiRemoteAttached {
		// 	if !editor.config.MiniMap.Disable {
		// 		editor.workspaces[editor.active].minimap.exit()
		// 	}
		// }
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
		for i := 0; i <= len(editor.side.items) && i <= len(editor.workspaces); i++ {
			if i >= index {
				editor.side.items[i].cwdpath = editor.side.items[i+1].cwdpath
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

func getResourcePath() string {
	p := ""
	if runtime.GOOS == "darwin" {
		p = core.QCoreApplication_ApplicationDirPath() + "/../Resources"
	} else if runtime.GOOS == "linux" {
		p = core.QCoreApplication_ApplicationDirPath()
	} else if runtime.GOOS == "windows" {
		p = core.QCoreApplication_ApplicationDirPath()
	}

	return p
}

func (w *Workspace) startNvim(path string) error {
	editor.putLog("starting nvim")
	var neovim *nvim.Nvim
	var err error

	option := []string{
		"--cmd",
		"let g:gonvim_running=1",
		"--cmd",
		"let g:goneovim=1",
		"--cmd",
		"set termguicolors",
	}

	runtimepath := getResourcePath() + "/runtime/"
	s := fmt.Sprintf("let &rtp.=',%s'", runtimepath)
	if editor.config.Popupmenu.ShowDigit {
		option = append(option, "--cmd")
		option = append(option, s)
	}
	option = append(option, "--embed")
	childProcessArgs := nvim.ChildProcessArgs(
		append(option, editor.args...)...,
	)
	if editor.opts.Server != "" {
		// Attaching to remote nvim session
		neovim, err = nvim.Dial(editor.opts.Server)
		w.uiRemoteAttached = true
	} else if editor.opts.Nvim != "" {
		// Attaching to /path/to/nvim
		childProcessCmd := nvim.ChildProcessCommand(editor.opts.Nvim)
		neovim, err = nvim.NewChildProcess(childProcessArgs, childProcessCmd)
	} else if editor.opts.Ssh != "" {
		// Attaching remote nvim via ssh
		w.uiRemoteAttached = true
		neovim, err = newRemoteChildProcess()
	} else {
		// Attaching to nvim normally
		neovim, err = nvim.NewChildProcess(childProcessArgs)
	}
	if err != nil {
		fmt.Println(err)
		return err
	}

	neovim.RegisterHandler("Gui", func(updates ...interface{}) {
		w.guiUpdates <- updates
		w.signal.GuiSignal()
	})
	neovim.RegisterHandler("redraw", func(updates ...[]interface{}) {
		w.redrawUpdates <- updates
		w.signal.RedrawSignal()
	})

	editor.putLog("done starting nvim")

	w.updateSize()
	editor.putLog("updating size of UI components")

	w.nvim = neovim

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

	return nil
}

var embedProcAttr *syscall.SysProcAttr

func newRemoteChildProcess() (*nvim.Nvim, error) {
	logf := log.Printf
	command := "ssh"
	if runtime.GOOS == "windows" {
		command = `C:\windows\system32\OpenSSH\ssh.exe`
	}
	ctx := context.Background()

	userhost := ""
	port := "22"
	parts := strings.Split(editor.opts.Ssh, ":")
	if len(parts) >= 3 {
		return nil, errors.New("Invalid hostname")
	}
	if len(parts) == 2 {
		userhost = parts[0]
		port = parts[1]
		_, err := strconv.Atoi(port)
		if port == "" || err != nil {
			port = "22"
		}
	}
	if len(parts) == 1 {
		userhost = parts[0]
	}

	cmd := exec.CommandContext(
		ctx,
		command,
		userhost,
		"-p", port,
		"/bin/bash",
		"--login",
		"-c",
		`"nvim --cmd 'let g:gonvim_running=1' --cmd 'let g:goneovim=1' --cmd 'set termguicolors' --embed"`,
	)
	util.PrepareRunProc(cmd)
	cmd.SysProcAttr = embedProcAttr

	inw, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	outr, err := cmd.StdoutPipe()
	if err != nil {
		inw.Close()
		return nil, err
	}
	cmd.Start()

	v, _ := nvim.New(outr, inw, inw, logf)

	return v, nil
}

func (w *Workspace) init(path string) {
	w.configure()
	w.attachUI(path)
	w.loadGoneovimRuntime()
	w.loadGinitVim()
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
	go w.nvim.Subscribe("Gui")
	go w.initGonvim()
	if w.tabline != nil {
		w.tabline.subscribe()
	}
	if w.statusline != nil {
		w.statusline.subscribe()
	}
	// if w.loc != nil {
	// 	w.loc.subscribe()
	// }
	if w.message != nil {
		w.message.connectUI()
	}

	w.fontMutex.Lock()
	defer w.fontMutex.Unlock()
	w.uiAttached = true

	editor.putLog("attaching UI")
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
	au GonvimAu VimEnter * call rpcnotify(1, "Gui", "gonvim_enter")
	au GonvimAu UIEnter * call rpcnotify(1, "Gui", "gonvim_uienter")
	au GonvimAu BufEnter * call rpcnotify(0, "Gui", "gonvim_bufenter", line("$"))
	au GonvimAu WinEnter,FileType * call rpcnotify(0, "Gui", "gonvim_winenter_filetype", &ft, win_getid())
	au GonvimAu OptionSet * if &ro != 1 | silent! call rpcnotify(1, "Gui", "gonvim_optionset") | endif
	au GonvimAu TermEnter * call rpcnotify(0, "Gui", "gonvim_termenter")
	au GonvimAu TermLeave * call rpcnotify(0, "Gui", "gonvim_termleave")
	aug GonvimAuWorkspace | au! | aug END
	au GonvimAuWorkspace DirChanged * call rpcnotify(0, "Gui", "gonvim_workspace_cwd", v:event)
	aug GonvimAuFilepath | au! | aug END
	au GonvimAuFilepath BufEnter,TabEnter,DirChanged,TermOpen,TermClose * silent call rpcnotify(0, "Gui", "gonvim_workspace_filepath", expand("%:p"))
	`
	if !editor.config.Markdown.Disable {
		gonvimAutoCmds += `
		aug GonvimAuMd | au! | aug END
		au GonvimAuMd TextChanged,TextChangedI *.md call rpcnotify(0, "Gui", "gonvim_markdown_update")
		au GonvimAuMd BufEnter *.md call rpcnotify(0, "Gui", "gonvim_markdown_new_buffer")
		`
	}
	if !w.uiRemoteAttached {
		gonvimAutoCmds = gonvimAutoCmds + `
		aug GonvimAuMinimap | au! | aug END
		au GonvimAuMinimap BufEnter,BufWrite * call rpcnotify(0, "Gui", "gonvim_minimap_update")
		au GonvimAuMinimap TextChanged,TextChangedI * call rpcnotify(0, "Gui", "gonvim_minimap_sync")
		au GonvimAuMinimap ColorScheme * call rpcnotify(0, "Gui", "gonvim_colorscheme")
		`
	}

	if editor.config.ScrollBar.Visible {
		gonvimAutoCmds = gonvimAutoCmds + `
	aug GonvimAuScrollbar | au! | aug END
	au GonvimAuScrollbar TextChanged,TextChangedI * call rpcnotify(0, "Gui", "gonvim_textchanged", line("$"))
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
	command! GonvimVersion echo "%s"`, editor.version)
	if !editor.config.Markdown.Disable {
		gonvimCommands += `
		command! GonvimMarkdown call rpcnotify(0, "Gui", "gonvim_markdown_toggle")
		`
	}
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
	initialNotify := fmt.Sprintf(`call execute(%s)`, util.SplitVimscript(gonvimInitNotify))
	w.nvim.Command(initialNotify)
}

func (w *Workspace) loadGoneovimRuntime() {
	if editor.config.Popupmenu.ShowDigit {
		w.nvim.Command("runtime! plugin/showdigit.vim")
	}
}

func (w *Workspace) loadGinitVim() {
	if editor.config.Editor.GinitVim != "" {
		scripts := strings.NewReplacer(`'`, `''`, "\r\n", "\n", "\r", "\n", "\n", "\n").Replace(editor.config.Editor.GinitVim)
		execGinitVim := fmt.Sprintf(`call execute(split('%s', '\n'))`, scripts)
		w.nvim.Command(execGinitVim)
	}
}

func (w *Workspace) getNvimOptions() {
	w.getColorscheme()
	w.getTS()
	w.getBG()
	w.getKeymaps()
}

func (w *Workspace) getColorscheme() {
	done := make(chan bool, 5)
	colorscheme := ""
	go func() {
		w.nvim.Var("colors_name", &colorscheme)
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(40 * time.Millisecond):
	}
	w.colorscheme = colorscheme
}

func (w *Workspace) getTS() {
	done := make(chan bool, 5)
	ts := 8
	go func() {
		w.nvim.Option("ts", &ts)
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(40 * time.Millisecond):
	}
	w.ts = ts
}

func (w *Workspace) getBuffTS(buf nvim.Buffer, bufname string) int {
	done := make(chan bool, 5)
	ts := 8
	go func() {
		w.nvim.BufferOption(buf, "ts", &ts)
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(90 * time.Millisecond):
	}

	return ts
}

func (w *Workspace) getBG() {
	done := make(chan bool, 5)
	screenbg := "dark"
	go func() {
		w.nvim.Option("background", &screenbg)
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(40 * time.Millisecond):
	}
	w.screenbg = screenbg

	if w.screenbg == "light" {
		editor.colors.bg = newRGBA(
			editor.colors.fg.R,
			editor.colors.fg.G,
			editor.colors.fg.B,
			1,
		)
		editor.colors.fg = newRGBA(
			editor.colors.bg.R,
			editor.colors.bg.G,
			editor.colors.bg.B,
			1,
		)
	}
}

func (w *Workspace) getKeymaps() {
	done := make(chan bool, 5)
	w.escKeyInInsert = "<Esc>"
	w.escKeyInNormal = "<Esc>"

	var nmappings, imappings []*nvim.Mapping
	var err error

	go func() {
		nmappings, err = w.nvim.KeyMap("normal")
		if err != nil {
			return
		}
		w.normalMappings = nmappings
		imappings, err = w.nvim.KeyMap("insert")
		if err != nil {
			return
		}
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(80 * time.Millisecond):
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
		if strings.EqualFold(mapping.LHS, "<C-y>") || strings.EqualFold(mapping.LHS, "<C-e>") {
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

	editor.muMetaKey.Lock()
	if altkeyCount >= metakeyCount {
		editor.prefixToMapMetaKey = "A-"
	} else {
		editor.prefixToMapMetaKey = "M-"
	}
	editor.muMetaKey.Unlock()
}

func (w *Workspace) getNumOfTabs() int {
	done := make(chan bool, 5)
	num := 0
	go func() {
		w.nvim.Eval("tabpagenr('$')", &num)
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(40 * time.Millisecond):
	}

	return num
}

func (w *Workspace) getCwd() string {
	done := make(chan bool, 5)
	cwd := ""
	go func() {
		w.nvim.Eval("getcwd()", &cwd)
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
	}

	return cwd
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

func (w *Workspace) handleChangeCwd(cwdinfo map[string]interface{}) {
	scope, ok := cwdinfo["scope"]
	if !ok {
		scope = "global"
	}
	cwdITF, ok := cwdinfo["cwd"]
	if !ok {
		return
	}
	cwd := cwdITF.(string)
	switch scope {
	case "global":
		w.setCwd(cwd)
	case "tab":
		w.setCwdInTab(cwd)
	case "window":
		w.setCwdInWin(cwd)
	}
}

func (w *Workspace) setCwd(cwd string) {
	w.cwd = cwd
	if editor.side == nil {
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
		if i >= len(editor.side.items) {
			return
		}

		if ws == w {
			path, _ := filepath.Abs(cwd)
			sideItem := editor.side.items[i]
			if sideItem.cwdpath == path {
				continue
			}

			sideItem.label.SetText(w.cwdlabel)
			sideItem.label.SetFont(gui.NewQFont2(editor.extFontFamily, editor.extFontSize-1, 1, false))
			sideItem.cwdpath = path
		}
	}
}

func (w *Workspace) setCwdInTab(cwd string) {
	w.screen.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)

		if win == nil {
			return true
		}
		if win.grid == 1 {
			return true
		}
		if win.isMsgGrid {
			return true
		}
		if win.isShown() {
			win.cwd = cwd
		}

		return true
	})
}

func (w *Workspace) setCwdInWin(cwd string) {
	w.screen.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)

		if win == nil {
			return true
		}
		if win.grid == 1 {
			return true
		}
		if win.isMsgGrid {
			return true
		}
		if win.grid == w.cursor.gridid {
			win.cwd = cwd
		}

		return true
	})
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
					case "win_viewport":
						w.api5 = true
					}
				}
			}
		}
	}

	return o
}

func (w *Workspace) updateSize() {
	e := editor
	width := e.window.Geometry().Width() - e.window.BorderSize()*4 - e.window.WindowGap()*2
	if e.side != nil {
		if e.side.widget.IsVisible() {
			width = width - e.splitter.Sizes()[0] - e.splitter.HandleWidth()
		}
	}
	height := e.window.Geometry().Height() - e.window.BorderSize()*4
	if e.config.Editor.BorderlessWindow && runtime.GOOS != "linux" {
		height = height - e.window.TitleBar.Height()
	}
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

	if w.drawTabline && w.tabline != nil {
		w.tabline.height = w.tabline.widget.Height()
	}
	if w.drawStatusline && w.statusline != nil {
		w.statusline.height = w.statusline.widget.Height()
	}

	if w.screen != nil {
		t := 0
		s := 0
		if w.tabline != nil {
			t = w.tabline.height
		}
		if w.statusline != nil {
			s = w.statusline.height
		}
		w.screen.height = w.height - t - s
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
		editor.putLog("start   ", event)
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
			w.flush()

		// Grid Events
		case "grid_resize":
			s.gridResize(args)
		case "default_colors_set":
			for _, u := range update[1:] {
				w.setColorsSet(u.([]interface{}))
			}
		case "hl_attr_define":
			s.setHlAttrDef(args)
			// if goneovim own statusline is visible
			if w.drawStatusline {
				w.statusline.getColor()
			}
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
		case "win_float_pos":
			s.windowFloatPosition(args)
		case "win_external_pos":
			s.windowExternalPosition(args)
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
		case "win_viewport":
			w.windowViewport(args[0].([]interface{}))

		// Popupmenu Events
		case "popupmenu_show":
			if w.cmdline != nil {
				if w.cmdline.shown {
					w.cmdline.cmdWildmenuShow(args)
				}
			}
			if w.popup != nil {
				if w.cmdline != nil {
					if !w.cmdline.shown {
						w.popup.showItems(args)
					}
				} else {
					w.popup.showItems(args)
				}
			}
		case "popupmenu_select":
			if w.cmdline != nil {
				if w.cmdline.shown {
					w.cmdline.cmdWildmenuSelect(args)
				}
			}
			if w.popup != nil {
				if w.cmdline != nil {
					if !w.cmdline.shown {
						w.popup.selectItem(args)
					}
				} else {
					w.popup.selectItem(args)
				}
			}
		case "popupmenu_hide":
			if w.cmdline != nil {
				if w.cmdline.shown {
					w.cmdline.cmdWildmenuHide()
				}
			}
			if w.popup != nil {
				if w.cmdline != nil {
					if !w.cmdline.shown {
						w.popup.hide()
					}
				} else {
					w.popup.hide()
				}
			}
		// Tabline Events
		case "tabline_update":
			if w.tabline != nil {
				w.tabline.update(args)
			}

		// Cmdline Events
		case "cmdline_show":
			if w.cmdline != nil {
				w.cmdline.show(args)
			}

		case "cmdline_pos":
			if w.cmdline != nil {
				w.cmdline.changePos(args)
			}

		case "cmdline_special_char":

		case "cmdline_char":
			if w.cmdline != nil {
				w.cmdline.putChar(args)
			}
		case "cmdline_hide":
			if w.cmdline != nil {
				w.cmdline.hide()
			}
		case "cmdline_function_show":
			if w.cmdline != nil {
				w.cmdline.functionShow()
			}
		case "cmdline_function_hide":
			if w.cmdline != nil {
				w.cmdline.functionHide()
			}
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
		editor.putLog("finished", event)
	}
}

func (w *Workspace) flush() {
	for {
		if len(w.viewportQue) == 0 {
			break
		}
		select {
		case viewport := <-w.viewportQue:
			win, diff, ok := w.handleViewport(viewport)
			if ok {
				w.smoothScroll(win, diff)
			}
		default:
		}
	}
	w.screen.update()
	w.cursor.update()
	w.drawOtherUI()
}

func (w *Workspace) drawOtherUI() {
	s := w.screen

	if (w.minimap != nil && w.minimap.visible) || w.drawStatusline || editor.config.ScrollBar.Visible {
		w.getPos()
	}

	if w.drawStatusline {
		if w.statusline != nil {
			w.statusline.pos.redraw(w.viewport[2], w.viewport[3])
			w.statusline.mode.redraw()
		}
	}

	if w.scrollBar != nil {
		if editor.config.ScrollBar.Visible {
			w.scrollBar.update()
		}
	}

	if s.tooltip.IsVisible() {
		x, y, _, _ := w.screen.toolTipPos()
		w.screen.toolTipMove(x, y)
	}

	if w.minimap != nil {
		if w.minimap.visible && w.minimap.widget.IsVisible() {
			go w.updateMinimap()
			w.minimap.mapScroll()
		}
	}
}

func (w *Workspace) disableImeInNormal() {
	if !editor.config.Editor.DisableImeInNormal {
		return
	}
	switch w.mode {
	case "insert":
		w.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
		editor.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	case "cmdline_normal":
		w.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
		editor.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	default:
		w.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, false)
		editor.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, false)
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
		isChangeFg = !editor.colors.fg.equals(w.foreground)
	}
	if editor.colors.bg != nil {
		isChangeBg = !editor.colors.bg.equals(w.background)
	}

	if isChangeFg || isChangeBg {
		editor.isSetGuiColor = false
	}
	if len(editor.workspaces) > 1 {
		w.updateWorkspaceColor()
		// Ignore setting GUI color when create second workspace and fg, bg equals -1
		if fg == -1 && bg == -1 {
			editor.isSetGuiColor = true
		}
	}

	// Exit if there is no change in foreground / background
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
	// w.signature.setColor()
	// if w.palette != nil {
	// 	w.palette.setColor()
	// }
	// if w.fpalette != nil {
	// 	w.fpalette.setColor()
	// }
	if w.popup != nil {
		w.popup.setColor()
	}

	if w.message != nil {
		w.message.setColor()
	}
	w.screen.setColor()

	// if w.drawTabline {
	// 	if w.tabline != nil {
	// 		w.tabline.setColor()
	// 	}
	// }

	if w.drawStatusline {
		if w.statusline != nil {
			w.statusline.setColor()
		}
	}

	if w.scrollBar != nil {
		if editor.config.ScrollBar.Visible {
			w.scrollBar.setColor()
		}
	}

	// if editor.config.Lint.Visible {
	// 	w.loc.setColor()
	// }

	if editor.side != nil {
		editor.side.setColor()
		editor.side.setColorForItems()
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
			w.setPumblend(val)
			if w.popup != nil {
				w.popup.setPumblend(w.pb)
			}
		case "showtabline":
			w.showtabline = util.ReflectToInt(val)
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
	if w.api5 {
		return
	}
	done := make(chan error, 20)
	var curPos [4]int
	go func() {
		err := w.nvim.Eval("getpos('.')", &curPos)
		done <- err
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Millisecond):
		return
	}

	w.curPosMutex.Lock()
	w.viewport[2] = curPos[1]
	w.viewport[3] = curPos[2]
	w.curPosMutex.Unlock()
}

func (w *Workspace) windowViewport(arg []interface{}) {
	viewport := [4]int{
		util.ReflectToInt(arg[2]) + 1,
		util.ReflectToInt(arg[3]) + 1,
		util.ReflectToInt(arg[4]) + 1,
		util.ReflectToInt(arg[5]) + 1,
	}

	if w.viewport == viewport && (w.oldViewport[0] != w.viewport[0] || w.oldViewport[1] != w.viewport[1]) {
		scrollvp := [5]int{
			util.ReflectToInt(arg[2]) + 1,
			util.ReflectToInt(arg[3]) + 1,
			util.ReflectToInt(arg[4]) + 1,
			util.ReflectToInt(arg[5]) + 1,
			util.ReflectToInt(arg[0]),
		}
		w.viewportQue <- scrollvp
	}

	if w.viewport != w.oldViewport {
		w.oldViewport = w.viewport
	}
	w.viewport = viewport

}

func (w *Workspace) handleViewport(vp [5]int) (*Window, int, bool) {
	win, ok := w.screen.getWindow(vp[4])
	if !ok {
		return nil, 0, false
	}
	if win.isMsgGrid {
		return nil, 0, false
	}
	if vp[4] == 1 { // if global grid
		return nil, 0, false
	}

	w.curPosMutex.Lock()
	w.scrollViewport[1] = w.scrollViewport[0]
	w.scrollViewport[0] = vp
	w.curPosMutex.Unlock()
	viewport := w.scrollViewport[0]
	oldViewport := w.scrollViewport[1]

	diff := viewport[0] - oldViewport[0]
	if diff == 0 {
		diff = viewport[1] - oldViewport[1]
	}
	isGridGoto := viewport[4] != oldViewport[4]
	if int(math.Abs(float64(diff))) >= win.rows/2 {
		wrappedLines1 := win.rows - (viewport[1] - viewport[0] - 1)
		wrappedLines2 := win.rows - (oldViewport[1] - oldViewport[0] - 1)
		if diff < 0 {
			diff -= wrappedLines1
		} else if diff > 0 {
			diff += wrappedLines2
		}
	}

	// smooth scroll
	if !editor.config.Editor.SmoothScroll {
		return nil, 0, false
	}

	if w.maxLine < w.rows {
		return nil, 0, false
	}

	// Compatibility of smooth scrolling with touchpad and smooth scrolling with scroll commands
	if win.isWheelScrolling || editor.isKeyAutoRepeating {
		return nil, 0, false
	}

	// get snapshot
	if diff != 0 {
		win.snapshots[1] = win.snapshots[0]
		win.snapshots[0] = win.Grab(win.Rect())
		win.scrollCols = int(math.Abs(float64(diff)))
	}

	if isGridGoto {
		return win, diff, false
	}

	return win, diff, true
}

func (w *Workspace) smoothScroll(win *Window, diff int) {
	// process smooth scroll
	a := core.NewQPropertyAnimation2(win, core.NewQByteArray2("scrollDiff", len("scrollDiff")), win)
	a.ConnectValueChanged(func(value *core.QVariant) {
		ok := false
		v := value.ToDouble(&ok)
		if !ok {
			return
		}
		font := win.getFont()
		win.scrollPixels2 = int(float64(diff) * v * float64(font.lineHeight))
		// win.update()
		win.Update2(
			0,
			0,
			int(float64(win.cols)*font.truewidth),
			win.cols*font.lineHeight,
		)
		if win.scrollPixels2 == 0 {
			win.doErase = true
			win.Update2(
				0,
				0,
				int(float64(win.cols)*font.truewidth),
				win.cols*font.lineHeight,
			)
			win.doErase = false
		}
	})
	a.SetDuration(180)
	a.SetStartValue(core.NewQVariant10(1))
	a.SetEndValue(core.NewQVariant10(0))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutQuart))
	a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutExpo))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutCirc))
	a.Start(core.QAbstractAnimation__DeletionPolicy(core.QAbstractAnimation__DeleteWhenStopped))
}

func (w *Workspace) updateMinimap() {
	var absMapTop int
	var absMapBottom int
	w.minimap.nvim.Eval("line('w0')", &absMapTop)
	w.minimap.nvim.Eval("line('w$')", &absMapBottom)
	w.curPosMutex.RLock()
	w.minimap.nvim.Command(fmt.Sprintf("call cursor(%d, %d)", w.viewport[2], 0))
	defer w.curPosMutex.RUnlock()
	switch {
	case w.viewport[2] >= absMapBottom:
		w.minimap.nvim.Input("<C-d>")
	case absMapTop >= w.viewport[2]:
		w.minimap.nvim.Input("<C-u>")
	default:
	}
}

func (w *Workspace) handleRPCGui(updates []interface{}) {
	event := updates[0].(string)
	switch event {
	case "gonvim_enter":
		editor.putLog("vim enter")
		w.vimEnterProcess()
	case "gonvim_uienter":
		editor.putLog("ui enter")
	case "gonvim_resize":
		width, height := editor.setWindowSize(updates[1].(string))
		editor.window.Resize2(width, height)
	case "gonvim_maximize":
		// editor.window.WindowMaximize()
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
	// case "signature_show":
	// 	w.signature.showItem(updates[1:])
	// case "signature_pos":
	// 	w.signature.pos(updates[1:])
	// case "signature_hide":
	// 	w.signature.hide()
	case "side_open":
		editor.side.show()
	case "side_close":
		editor.side.hide()
	case "side_toggle":
		editor.side.toggle()
		w.updateSize()
	case "filer_update":
		if !editor.side.scrollarea.IsVisible() {
			return
		}
		if !editor.side.items[editor.active].isContentHide {
			go w.nvim.Call("rpcnotify", nil, 0, "GonvimFiler", "redraw")
		}
	case "filer_open":
		editor.side.items[w.getNum()].isContentHide = false
		editor.side.items[w.getNum()].openContent()
	case "filer_clear":
		editor.side.items[w.getNum()].clear()
	case "filer_resize":
		editor.side.items[w.getNum()].resizeContent()
	case "filer_item_add":
		editor.side.items[w.getNum()].addItem(updates[1:])
	case "filer_item_select":
		editor.side.items[w.getNum()].selectItem(updates[1:])
	case "gonvim_grid_font":
		w.screen.gridFont(updates[1])
	case "gonvim_minimap_update":
		if w.minimap != nil {
			if w.minimap.visible {
				w.minimap.bufUpdate()
			}
		}
	case "gonvim_minimap_sync":
		if w.minimap != nil {
			if w.minimap.visible {
				go w.minimap.bufSync()
			}
		}
	case "gonvim_minimap_toggle":
		go w.minimap.toggle()
	case "gonvim_colorscheme":
		if w.minimap != nil {
			w.minimap.isSetColorscheme = false
			w.minimap.setColorscheme()
		}

		win, ok := w.screen.getWindow(w.cursor.gridid)
		if !ok {
			return
		}
		win.snapshots[0] = nil
		win.snapshots[1] = nil

	case "gonvim_copy_clipboard":
		go editor.copyClipBoard()
	case "gonvim_textchanged":
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
		cwdinfo := updates[1].(map[string]interface{})
		w.handleChangeCwd(cwdinfo)
	case "gonvim_workspace_filepath":
		if w.minimap != nil {
			w.minimap.mu.Lock()
			w.filepath = updates[1].(string)
			w.minimap.mu.Unlock()
		}
	case "gonvim_optionset":
		go w.optionSet()
	case "gonvim_termenter":
		w.mode = "terminal-input"
	case "gonvim_termleave":
		w.mode = "normal"
	case "gonvim_bufenter":
		w.getBufnameAndTS()
		w.maxLine = util.ReflectToInt(updates[1])
	case "gonvim_winenter_filetype":
		w.getFileType(updates)
		w.getBufnameAndTS()
	case "gonvim_markdown_new_buffer":
		if editor.config.Markdown.Disable {
			return
		}
		if w.markdown == nil {
			w.signal.LazyDrawSignal()
		}
		go w.markdown.newBuffer()
	case "gonvim_markdown_update":
		if editor.config.Markdown.Disable {
			return
		}
		if w.markdown == nil {
			w.signal.LazyDrawSignal()
		}
		go w.markdown.update()
	case "gonvim_markdown_toggle":
		if editor.config.Markdown.Disable {
			return
		}
		if w.markdown == nil {
			w.signal.LazyDrawSignal()
		}
		w.markdown.toggle()
	case "gonvim_markdown_scroll_down":
		w.markdown.scrollDown()
	case "gonvim_markdown_scroll_up":
		w.markdown.scrollUp()
	case "gonvim_markdown_scroll_top":
		w.markdown.scrollTop()
	case "gonvim_markdown_scroll_bottom":
		w.markdown.scrollBottom()
	case "gonvim_markdown_scroll_pagedown":
		w.markdown.scrollPageDown()
	case "gonvim_markdown_scroll_pageup":
		w.markdown.scrollPageUp()
	case "gonvim_markdown_scroll_halfpagedown":
		w.markdown.scrollHalfPageDown()
	case "gonvim_markdown_scroll_halfpageup":
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
	var fontWeight gui.QFont__Weight
	var fontStretch int

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
		fontFamily, fontHeight, fontWeight, fontStretch = getFontFamilyAndHeightAndWeightAndStretch(gfn)
		ok := checkValidFont(fontFamily)
		if ok {
			break
		}
	}

	if fontHeight == 0 {
		fontHeight = 10.0
	}

	w.font.change(fontFamily, fontHeight, fontWeight, fontStretch)
	w.screen.font = w.font

	font := w.font
	win, ok := w.screen.getWindow(w.cursor.gridid)
	if ok {
		font = win.getFont()
	}

	w.updateSize()

	if w.popup != nil {
		w.popup.updateFont(font)
	}
	if w.message != nil {
		w.message.updateFont()
	}
	w.screen.toolTipFont(font)
	w.cursor.updateFont(font)

	// Change external font if font setting of setting.yml is nothing
	if editor.config.Editor.FontFamily == "" {
		editor.extFontFamily = fontFamily
	}
	if editor.config.Editor.FontSize == 0 {
		editor.extFontSize = int(fontHeight)
	}

	// w.palette.updateFont()
	// w.fpalette.updateFont()

	if w.tabline != nil {
		w.tabline.updateFont()
	}
	if w.statusline != nil {
		w.statusline.updateFont()
	}
}

func (w *Workspace) guiFontWide(args string) {
	if args == "" {
		return
	}

	if w.fontwide == nil {
		w.fontwide = initFontNew(
			editor.extFontFamily,
			float64(editor.extFontSize),
			editor.config.Editor.Linespace,
		)
		w.fontwide.ws = w
		w.cursor.fontwide = w.fontwide
	}

	var fontHeight float64
	var fontFamily string
	var fontWeight gui.QFont__Weight
	var fontStretch int

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
		fontFamily, fontHeight, fontWeight, fontStretch = getFontFamilyAndHeightAndWeightAndStretch(gfn)
		ok := checkValidFont(fontFamily)
		if ok {
			break
		}
	}

	if fontHeight == 0 {
		fontHeight = 10.0
	}

	w.fontwide.change(fontFamily, fontHeight, fontWeight, fontStretch)

	w.updateSize()
	// w.cursor.updateFont(w.font)
	// w.screen.toolTipFont(w.font)
}

func getFontFamilyAndHeightAndWeightAndStretch(s string) (string, float64, gui.QFont__Weight, int) {
	parts := strings.Split(s, ":")
	height := -1.0
	width := -1.0
	weight := gui.QFont__Normal
	if len(parts) > 1 {
		for _, p := range parts[1:] {
			if strings.HasPrefix(p, "h") {
				// height, err = strconv.Atoi(p[1:])
				h, err := strconv.ParseFloat(p[1:], 64)
				if err == nil {
					height = h
				}
			} else if strings.HasPrefix(p, "w") {
				// width, err := strconv.Atoi(p[1:])
				w, err := strconv.ParseFloat(p[1:], 64)
				if err == nil {
					width = w
				}
			} else if p == "t" {
				weight = gui.QFont__Thin
			} else if p == "el" {
				weight = gui.QFont__ExtraLight
			} else if p == "l" {
				weight = gui.QFont__Light
			} else if p == "n" {
				// default weight, we do nothing
			} else if p == "db" || p == "sb" {
				weight = gui.QFont__DemiBold
			} else if p == "b" {
				weight = gui.QFont__Bold
			} else if p == "eb" {
				weight = gui.QFont__ExtraBold
			} else {
				weight = gui.QFont__Normal
			}
		}
	}
	family := parts[0]

	if height <= 1.0 && width <= 0 {
		height = 12
		width = 6
	} else if height > 1.0 && width == -1.0 {
		width = height / 2.0
	} else if height <= 1.0 && width >= 1.0 {
		height = width * 2.0
	}

	stretch := int(float64(width) / float64(height) * 2.0 * 100.0)

	return family, height, weight, stretch
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
	if lineSpace < 0 {
		return
	}
	w.font.changeLineSpace(lineSpace)
	w.updateSize()
}

func (w *Workspace) setPumblend(arg interface{}) {
	var pumblend int
	var err error
	switch val := arg.(type) {
	case string:
		pumblend, err = strconv.Atoi(val)
		if err != nil {
			return
		}
	case int32: // can't combine these in a type switch without compile error
		pumblend = int(val)
	case int64:
		pumblend = int(val)
	default:
		return
	}

	w.pb = pumblend
}

func (w *Workspace) getBufnameAndTS() {
	w.screen.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)

		if win == nil {
			return true
		}
		if win.grid == 1 {
			return true
		}

		// set buffer name
		bufChan := make(chan nvim.Buffer, 2)
		id := win.id
		var buf nvim.Buffer
		go func() {
			resultBuffer, _ := w.nvim.WindowBuffer(id)
			bufChan <- resultBuffer
		}()
		select {
		case buf = <-bufChan:
		case <-time.After(40 * time.Millisecond):
		}

		strChan := make(chan string, 2)
		var bufName string
		go func() {
			resultStr, _ := w.nvim.BufferName(buf)
			strChan <- resultStr
		}()
		select {
		case bufName = <-strChan:
		case <-time.After(40 * time.Millisecond):
		}

		win.bufName = bufName
		win.ts = w.getBuffTS(buf, bufName)
		return true
	})
}

func (w *Workspace) optionSet() {
	w.optionsetMutex.Lock()
	w.getTS()
	// w.getPumHeight()
	w.getWinblendAll()
	w.optionsetMutex.Unlock()
	// w.getFileType()
}

func (w *Workspace) getPumHeight() {
	ph := w.ph
	errCh := make(chan error, 60)
	go func() {
		err := w.nvim.Option("pumheight", &ph)
		errCh <- err
	}()
	select {
	case <-errCh:
	case <-time.After(40 * time.Millisecond):
	}

	w.ph = ph
}

func (w *Workspace) getFileType(args []interface{}) {
	ft := args[1].(string)
	wid := util.ReflectToInt(args[2])

	for _, v := range editor.config.Editor.IndentGuideIgnoreFtList {
		if v == ft {
			return
		}
	}

	w.screen.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)

		if win == nil {
			return true
		}
		if win.grid == 1 {
			return true
		}
		if win.isMsgGrid {
			return true
		}
		if int(win.id) != wid {
			return true
		}

		// ftChan := make(chan error, 60)
		// var err error
		// var ft string
		// go func() {
		// 	ft, err = w.nvim.CommandOutput(`echo &ft`)
		// 	ftChan <-err
		// }()
		// select {
		// case <-ftChan:
		// case <-time.After(40 * time.Millisecond):
		// }

		win.paintMutex.Lock()
		win.ft = ft
		win.paintMutex.Unlock()

		return true
	})

}

func (w *Workspace) getWinblendAll() {
	w.screen.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)

		if win == nil {
			return true
		}

		win.propMutex.RLock()
		isFloatWin := win.isFloatWin
		win.propMutex.RUnlock()

		if !isFloatWin {
			return true
		}

		win.getWinblend()
		return true
	})
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
	if query == core.Qt__ImMicroFocus || query == core.Qt__ImCursorRectangle {
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
		if w.tabline != nil {
			y += w.tabline.widget.Height()
		}
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

	isShown      bool
	isInitResize bool

	fg       *RGBA
	sfg      *RGBA
	scrollFg *RGBA
	scrollBg *RGBA
	selectBg *RGBA
	accent   *RGBA
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
	for i := 0; i < WORKSPACELEN; i++ {
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
	side.setColor()
	if side.isShown {
		return
	}
	if !side.isInitResize {
		editor.splitter.SetSizes(
			[]int{editor.config.SideBar.Width,
				editor.width - editor.config.SideBar.Width},
		)
		side.isInitResize = true
	}
	side.scrollarea.Show()
	side.isShown = true

	for i := 0; i < WORKSPACELEN; i++ {
		if side.items[i] == nil {
			continue
		}
		if !side.items[i].active {
			continue
		}
		if editor.workspaces[i] != nil {
			editor.workspaces[i].setCwd(editor.workspaces[i].getCwd())
		}
		side.items[i].show()
	}
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
	labelWidget.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)
	labelLayout.SetContentsMargins(15, 1, 1, 1)
	labelLayout.SetSpacing(editor.iconSize / 2)

	label := widgets.NewQLabel(nil, 0)
	label.SetContentsMargins(0, 0, 0, 0)
	label.SetAlignment(core.Qt__AlignLeft)
	width := editor.config.SideBar.Width
	label.SetMaximumWidth(width)
	label.SetMinimumWidth(width)

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
		if editor.side.items[j] == nil {
			continue
		}
		sideItem := editor.side.items[j]
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
			if editor.side.items[j] == nil {
				continue
			}
			sideItem := editor.side.items[j]
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
	if side.fg.equals(editor.colors.fg) &&
		side.sfg.equals(editor.colors.sideBarFg) &&
		side.scrollFg.equals(editor.colors.scrollBarFg) &&
		side.scrollBg.equals(editor.colors.scrollBarBg) &&
		side.selectBg.equals(editor.colors.sideBarSelectedItemBg) &&
		side.accent.equals(editor.colors.matchFg) {

		return
	}

	side.fg = editor.colors.fg
	side.sfg = editor.colors.sideBarFg
	side.scrollFg = editor.colors.scrollBarFg
	side.scrollBg = editor.colors.scrollBarBg
	side.selectBg = editor.colors.sideBarSelectedItemBg
	side.accent = editor.colors.matchFg

	scrfg := side.scrollFg.String()
	scrbg := side.scrollBg.StringTransparent()
	hover := side.accent.String()

	side.header.SetStyleSheet(fmt.Sprintf(" .QLabel{ color: %s;} ", side.sfg.String()))
	side.widget.SetStyleSheet(
		fmt.Sprintf(`
		.QWidget { border: 0px solid #000; padding-top: 5px; background-color: rgba(0, 0, 0, 0); }
		QWidget { color: %s; border-right: 0px solid; }
		`, side.sfg.String()),
	)
	if side.scrollarea == nil {
		return
	}
	side.scrollarea.SetStyleSheet(
		fmt.Sprintf(`
		.QScrollBar { border-width: 0px; background-color: %s; width: 5px; margin: 0 0 0 0; }
		.QScrollBar::handle:vertical {background-color: %s; min-height: 25px;}
		.QScrollBar::handle:vertical:hover {background-color: %s; min-height: 25px;}
		.QScrollBar::add-line:vertical, .QScrollBar::sub-line:vertical { border: none; background: none; }
		.QScrollBar::add-page:vertical, QScrollBar::sub-page:vertical { background: none; }`,
			scrbg, scrfg, hover),
	)

	if len(editor.workspaces) == 1 {
		side.items[0].active = true
		// side.items[0].labelWidget.SetStyleSheet(
		// 	fmt.Sprintf(
		// 		" * { background-color: %s; color: %s; }",
		// 		side.selectBg.String(), side.sfg.String(),
		// 	),
		// )
		transparent := transparent() * transparent()
		side.items[0].labelWidget.SetStyleSheet(
			fmt.Sprintf(
				" * { background-color: rgba(%d, %d, %d, %f); color: %s; }",
				side.selectBg.R, side.selectBg.G, side.selectBg.B,
				transparent,
				side.fg.String(),
			),
		)
	}
}

func (side *WorkspaceSide) setColorForItems() {
	for _, item := range side.items {
		if item == nil {
			continue
		}
		if item.hidden {
			continue
		}
		item.content.SetStyleSheet(
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
		item.hide()
		item.show()
		// update icon color
		for i := 0; i < item.content.Count(); i++ {
			l := item.content.Item(i)
			if l == nil {
				break
			}
			filename := l.Text()
			parts := strings.SplitN(filename, ".", -1)
			filetype := ""
			if len(parts) > 1 {
				filetype = parts[len(parts)-1]
			}
			// If it is directory
			if filename[len(filename)-1] == '/' {
				filetype = string("/")
			}
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
		}
	}
}

func (i *WorkspaceSideItem) setActive() {
	if editor.colors.fg == nil {
		return
	}
	if editor.side.scrollarea == nil {
		return
	}
	i.active = true
	bg := editor.colors.sideBarSelectedItemBg
	fg := editor.colors.fg
	transparent := transparent() * transparent()
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
	if editor.side.scrollarea == nil {
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
