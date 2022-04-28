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
	"time"

	"github.com/akiyosi/goneovim/filer"
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

	_ func() `signal:"statuslineSignal"`
	_ func() `signal:"lintSignal"`
	_ func() `signal:"gitSignal"`

	_ func() `signal:"messageSignal"`

	_ func() `signal:"lazyDrawSignal"`
}

// Workspace is an editor workspace
type Workspace struct {
	foreground         *RGBA
	layout2            *widgets.QHBoxLayout
	stop               chan struct{}
	font               *Font
	fontwide           *Font
	cursor             *Cursor
	tabline            *Tabline
	statusline         *Statusline
	screen             *Screen
	scrollBar          *ScrollBar
	palette            *Palette
	popup              *PopupMenu
	cmdline            *Cmdline
	message            *Message
	minimap            *MiniMap
	guiUpdates         chan []interface{}
	redrawUpdates      chan [][]interface{}
	signal             *workspaceSignal
	nvim               *nvim.Nvim
	widget             *widgets.QWidget
	special            *RGBA
	viewportQue        chan [5]int
	background         *RGBA
	windowsFt          map[nvim.Window]string
	windowsTs          map[nvim.Window]int
	colorscheme        string
	cwdlabel           string
	escKeyInNormal     string
	mode               string
	cwdBase            string
	cwd                string
	escKeyInInsert     string
	filepath           string
	screenbg           string
	normalMappings     []*nvim.Mapping
	modeInfo           []map[string]interface{}
	insertMappings     []*nvim.Mapping
	viewport           [4]int
	oldViewport        [4]int
	height             int
	maxLineDelta       int
	maxLine            int
	rows               int
	cols               int
	showtabline        int
	width              int
	modeIdx            int
	pb                 int
	ts                 int
	ph                 int
	optionsetMutex     sync.RWMutex
	viewportMutex      sync.RWMutex
	stopOnce           sync.Once
	fontMutex          sync.Mutex
	hidden             bool
	uiAttached         bool
	uiRemoteAttached   bool
	isMappingScrollKey bool
	hasLazyUI          bool
	cursorStyleEnabled bool
	isDrawStatusline   bool
	isDrawTabline      bool
	terminalMode       bool
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
		windowsFt:     make(map[nvim.Window]string),
		windowsTs:     make(map[nvim.Window]int),
	}
	w.registerSignal()

	if len(editor.workspaces) > 0 {
		w.font = initFontNew(
			editor.extFontFamily,
			float64(editor.extFontSize),
			0,
			editor.config.Editor.Letterspace,
		)
	} else {
		w.font = editor.font
	}
	w.font.ws = w

	w.widget = widgets.NewQWidget(nil, 0)
	w.widget.SetParent(editor.widget)
	w.widget.SetAcceptDrops(true)
	w.widget.ConnectDragEnterEvent(w.dragEnterEvent)
	w.widget.ConnectDragMoveEvent(w.dragMoveEvent)
	w.widget.ConnectDropEvent(w.dropEvent)

	// Basic Workspace UI component
	// screen
	w.screen = newScreen()
	w.screen.ws = w
	w.screen.font = w.font
	w.screen.initInputMethodWidget()

	// cursor
	w.cursor = initCursorNew()
	w.cursor.ws = w
	w.cursor.SetParent(w.widget)
	w.cursor.setBypassScreenEvent()

	// If ExtFooBar is true, then we create a UI component
	// tabline
	if editor.config.Editor.ExtTabline {
		w.tabline = initTabline()
		w.tabline.ws = w
		w.tabline.font = w.font.fontNew
	}

	// cmdline
	if editor.config.Editor.ExtCmdline {
		w.cmdline = initCmdline()
		w.cmdline.ws = w
	}

	// popupmenu
	if editor.config.Editor.ExtPopupmenu {
		w.popup = initPopupmenuNew()
		w.popup.widget.SetParent(w.widget)
		w.popup.ws = w
		w.popup.widget.Hide()
	}

	// messages
	if editor.config.Editor.ExtMessages {
		w.message = initMessage()
		w.message.ws = w
		w.message.widget.SetParent(w.widget)
	}

	// If Statusline.Visible is true, then we create statusline UI component
	if editor.config.Statusline.Visible {
		w.statusline = initStatusline()
		w.statusline.ws = w
	}

	editor.putLog("initialazed UI components")

	// workspace widget, layouts
	layout := widgets.NewQVBoxLayout()
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(0)
	w.widget.SetContentsMargins(0, 0, 0, 0)
	w.widget.SetLayout(layout)
	w.widget.SetFocusPolicy(core.Qt__StrongFocus)
	w.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	w.widget.ConnectInputMethodEvent(w.InputMethodEvent)
	w.widget.ConnectInputMethodQuery(w.InputMethodQuery)

	// screen widget and scrollBar widget
	widget2 := widgets.NewQWidget(nil, 0)
	widget2.SetContentsMargins(0, 0, 0, 0)
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
		w.layout2.AddWidget(w.minimap.widget, 0, 0)
	}

	if editor.config.ScrollBar.Visible {
		w.layout2.AddWidget(w.scrollBar.widget, 0, 0)
		w.scrollBar.setColor()
	}

	// palette
	w.palette = initPalette()
	w.palette.ws = w
	w.palette.widget.SetParent(w.widget)
	w.palette.setColor()
	w.palette.hide()

	// Add editor feature
	go filer.RegisterPlugin(w.nvim, editor.config.Editor.FileOpenCmd)

	// Asynchronously execute the process for minimap
	go func() {
		if !editor.config.MiniMap.Disable {
			w.minimap.startMinimapProc()
			time.Sleep(time.Millisecond * 50)
			w.minimap.mu.Lock()
			isMinimapVisible := w.minimap.visible
			w.minimap.mu.Unlock()
			if isMinimapVisible {
				w.minimap.bufUpdate()
				w.minimap.bufSync()
				w.updateSize()
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
	if editor.workspaces == nil || len(editor.workspaces) == 1 {
		w.getNvimOptions()
	}

	// connect window resize event
	editor.window.ConnectResizeEvent(func(event *gui.QResizeEvent) {
		editor.resizeMainWindow()
	})
	if len(editor.workspaces) == 1 && runtime.GOOS == "linux" {
		editor.resizeMainWindow()
	}

	w.widget.ConnectFocusInEvent(func(event *gui.QFocusEvent) {
		go w.nvim.Command("if exists('#FocusGained') | doautocmd <nomodeline> FocusGained | endif")
	})
	w.widget.ConnectFocusOutEvent(func(event *gui.QFocusEvent) {
		go w.nvim.Command("if exists('#FocusLost') | doautocmd <nomodeline> FocusLost | endif")
	})

	go func() {

		if !editor.sessionExists {
			time.Sleep(time.Millisecond * 500)
		}
		w.signal.LazyDrawSignal()

		if !editor.sessionExists {
			time.Sleep(time.Millisecond * 400)
		}
		editor.signal.SidebarSignal()

		// put font debug log
		w.font.putDebugLog()

	}()
}

func (w *Workspace) registerSignal() {
	w.signal.ConnectRedrawSignal(func() {
		updates := <-w.redrawUpdates
		editor.putLog("Received redraw event from neovim")
		w.handleRedraw(updates)
	})
	w.signal.ConnectGuiSignal(func() {
		updates := <-w.guiUpdates
		editor.putLog("Received GUI event from neovim")
		w.handleRPCGui(updates)
	})
	w.signal.ConnectLazyDrawSignal(func() {
		if w.hasLazyUI {
			return
		}
		if editor.config.Editor.ExtTabline {
			w.tabline.initTab()
		}
		editor.workspaceUpdate()
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
		maxworkspaceIndex := len(editor.workspaces) - 1
		for i, ws := range editor.workspaces {
			if ws != w {
				workspaces = append(workspaces, ws)
			} else {
				index = i
			}
		}

		if len(workspaces) == 0 {
			// TODO
			// If nvim is an instance on a remote server, the connection `cmd` can be
			// `ssh` or `wsl` command. What kind of exit status should be set?
			if w.uiRemoteAttached {
				editor.close(0)
			} else {
				editor.close(w.nvim.ExitCode())
			}

			return
		}
		editor.workspaces = workspaces

		for i := 0; i < len(editor.side.items); i++ {
			if i >= index && i+1 < len(editor.side.items) {
				editor.side.items[i].copy(editor.side.items[i+1])
			}
			if i+1 == len(editor.side.items) {
				editor.side.items[i].label.SetText("")
				editor.side.items[i].hidden = false
				editor.side.items[i].active = false
				editor.side.items[i].text = ""
				editor.side.items[i].cwdpath = ""
				editor.side.items[i].isContentHide = false

				content := widgets.NewQListWidget(nil)
				content.SetFocusPolicy(core.Qt__NoFocus)
				content.SetFrameShape(widgets.QFrame__NoFrame)
				content.SetHorizontalScrollBarPolicy(core.Qt__ScrollBarAlwaysOff)
				content.SetFont(gui.NewQFont2(editor.extFontFamily, editor.extFontSize, 1, false))
				content.SetIconSize(core.NewQSize2(editor.iconSize*3/4, editor.iconSize*3/4))
				editor.side.items[i].content = content
				editor.side.items[i].widget.Layout().AddWidget(content)
			}
			if i == maxworkspaceIndex {
				editor.side.items[i].hidden = true
				editor.side.items[i].hidden = false
			}
			editor.side.items[i].setSideItemLabel(i)
		}

		w.hide()
		if editor.active == index {
			if index > 0 {
				editor.active--
			}
			editor.workspaceUpdate()
		}

	})
}

func (i *WorkspaceSideItem) copy(ii *WorkspaceSideItem) {
	i.label.SetText(ii.label.Text())
	i.hidden = ii.hidden
	i.active = ii.active
	i.text = ii.text
	i.cwdpath = ii.cwdpath
	i.content = ii.content
	i.isContentHide = ii.isContentHide

	i.widget.Layout().AddWidget(i.content)

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
	w.cursor.update()
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
	var err error
	var neovim *nvim.Nvim

	option := []string{
		"--cmd",
		"let g:gonvim_running=1",
		"--cmd",
		"let g:goneovim=1",
		"--cmd",
		"set termguicolors",
	}

	// Add runtimepath
	runtimepath := getResourcePath() + "/runtime/"
	option = append(option, "--cmd")
	option = append(option, fmt.Sprintf("let &rtp.=',%s'", runtimepath))

	// Generate goneovim helpdoc tag
	helpdocpath := getResourcePath() + "/runtime/doc"
	option = append(option, "--cmd")
	option = append(option, fmt.Sprintf(`try | helptags %s | catch /^Vim\%%((\a\+)\)\=:E/ | endtry`, helpdocpath))

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
	} else if editor.opts.Wsl != nil {
		// Attaching remote nvim via wsl
		w.uiRemoteAttached = true
		neovim, err = newWslProcess()
	} else if editor.opts.Ssh != "" {
		// Attaching remote nvim via ssh
		w.uiRemoteAttached = true
		neovim, err = newRemoteChildProcess()
	} else {
		// Attaching to nvim normally
		neovim, err = nvim.NewChildProcess(childProcessArgs)
	}
	if err != nil {
		editor.putLog(err)
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
			editor.putLog(err)
		}
		// w.stopOnce.Do(func() {
		// 	close(w.stop)
		// })
		w.nvim.Close()
		w.signal.StopSignal()
	}()

	go w.init(path)

	return nil
}

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

	nvimargs := `"nvim --cmd 'let g:gonvim_running=1' --cmd 'let g:goneovim=1' --cmd 'set termguicolors' --embed `
	for _, s := range editor.args {
		nvimargs += s + " "
	}
	nvimargs += `"`
	sshargs := []string{
		userhost,
		"-p", port,
		"$SHELL",
		"--login",
		"-c",
		nvimargs,
	}
	cmd := exec.CommandContext(
		ctx,
		command,
		sshargs...,
	)

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

	v, err := nvim.New(outr, inw, inw, logf)
	if err != nil {
		editor.putLog("error:", err)
		return nil, err
	}

	return v, nil
}

func newWslProcess() (*nvim.Nvim, error) {
	logf := log.Printf
	command := "wsl"
	ctx := context.Background()

	nvimargs := ""
	if editor.config.Editor.NvimInWsl == "" {
		nvimargs = `nvim --cmd 'let g:gonvim_running=1' --cmd 'let g:goneovim=1' --cmd 'set termguicolors' --embed `
	} else {
		nvimargs += fmt.Sprintf("%s --cmd 'let g:gonvim_running=1' --cmd 'let g:goneovim=1' --cmd 'set termguicolors' --embed ", editor.config.Editor.NvimInWsl)
	}
	for _, s := range editor.args {
		nvimargs += s + " "
	}

	wslArgs := []string{
		"$SHELL",
		"-lc",
		nvimargs,
	}
	if editor.opts.Wsl != nil && *editor.opts.Wsl != "" {
		wslArgs = append([]string{"-d", *editor.opts.Wsl}, wslArgs...)
	}

	cmd := exec.CommandContext(
		ctx,
		command,
		wslArgs...,
	)
	util.PrepareRunProc(cmd)
	editor.putLog("exec command:", cmd.String())

	inw, err := cmd.StdinPipe()
	if err != nil {
		editor.putLog("stdin pipe error:", err)
		return nil, err
	}

	outr, err := cmd.StdoutPipe()
	if err != nil {
		inw.Close()
		editor.putLog("stdin pipe error:", err)
		return nil, err
	}
	cmd.Start()

	v, err := nvim.New(outr, inw, inw, logf)
	if err != nil {
		editor.putLog("error:", err)
		return nil, err
	}

	return v, nil
}

func (w *Workspace) init(path string) {
	w.configure()
	w.attachUI(path)
	w.loadGinitVim()
}

func (w *Workspace) configure() {
	w.isDrawStatusline = editor.config.Statusline.Visible

	if editor.config.Tabline.Visible && editor.config.Editor.ExtTabline {
		w.isDrawTabline = true
	} else {
		w.isDrawTabline = false
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
		editor.putLog(err)
		editor.close(0)
		return err
	}

	if path != "" {
		go w.nvim.Command("so " + path)
	}

	return nil
}

func (w *Workspace) initGonvim() {
	// autocmds that goneovim uses
	gonvimAutoCmds := `
	aug GoneovimCore | au! | aug END
	au GoneovimCore VimEnter * call rpcnotify(0, "Gui", "gonvim_enter")
	au GoneovimCore UIEnter * call rpcnotify(0, "Gui", "gonvim_uienter")
	au GoneovimCore BufEnter * call rpcnotify(0, "Gui", "gonvim_bufenter", line("$"), win_getid())
	au GoneovimCore WinEnter,FileType * call rpcnotify(0, "Gui", "gonvim_winenter_filetype", &ft, win_getid())
	au GoneovimCore OptionSet * if &ro != 1 | silent! call rpcnotify(0, "Gui", "gonvim_optionset", expand("<amatch>"), v:option_new, v:option_old, win_getid()) | endif
	au GoneovimCore TermEnter * call rpcnotify(0, "Gui", "gonvim_termenter")
	au GoneovimCore TermLeave * call rpcnotify(0, "Gui", "gonvim_termleave")
	aug Goneovim | au! | aug END
	au Goneovim DirChanged * call rpcnotify(0, "Gui", "gonvim_workspace_cwd", v:event)
	au Goneovim BufEnter,TabEnter,DirChanged,TermOpen,TermClose * silent call rpcnotify(0, "Gui", "gonvim_workspace_filepath", expand("%:p"))
	`
	if editor.opts.Server == "" && !editor.config.MiniMap.Disable {
		gonvimAutoCmds = gonvimAutoCmds + `
		au Goneovim BufEnter,BufWrite * call rpcnotify(0, "Gui", "gonvim_minimap_update")
		au Goneovim TextChanged,TextChangedI * call rpcnotify(0, "Gui", "gonvim_minimap_sync")
		au Goneovim ColorScheme * call rpcnotify(0, "Gui", "gonvim_colorscheme")
		`
	}

	if editor.config.ScrollBar.Visible || editor.config.Editor.SmoothScroll {
		gonvimAutoCmds = gonvimAutoCmds + `
	au GoneovimCore TextChanged,TextChangedI * call rpcnotify(0, "Gui", "gonvim_textchanged", line("$"))
	`
	}
	if editor.config.Editor.Clipboard {
		gonvimAutoCmds = gonvimAutoCmds + `
	au GoneovimCore TextYankPost * call rpcnotify(0, "Gui", "gonvim_copy_clipboard")
	`
	}
	if editor.config.Statusline.Visible {
		gonvimAutoCmds = gonvimAutoCmds + `
	au Goneovim BufEnter,TermOpen,TermClose * call rpcnotify(0, "statusline", "bufenter", &filetype, &fileencoding, &fileformat, &ro)
	`
	}
	registerScripts := fmt.Sprintf(`call execute(%s)`, util.SplitVimscript(gonvimAutoCmds))
	w.nvim.Command(registerScripts)

	// Definition of the commands that goneovim provides
	gonvimCommands := fmt.Sprintf(`
	command! -nargs=1 GonvimResize call rpcnotify(0, "Gui", "gonvim_resize", <args>)
	command! GonvimSidebarShow call rpcnotify(0, "Gui", "side_open")
	command! GonvimVersion echo "%s"`, editor.version)
	if editor.opts.Server == "" {
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
		`
	}
	gonvimCommands = gonvimCommands + `
		command! -nargs=1 GonvimGridFont call rpcnotify(0, "Gui", "gonvim_grid_font", <args>)
		command! -nargs=1 GonvimLetterSpacing call rpcnotify(0, "Gui", "gonvim_letter_spacing", <args>)
		command! -nargs=1 GuiMacmeta call rpcnotify(0, "Gui", "gonvim_macmeta", <args>)
		command! -nargs=? GonvimMaximize call rpcnotify(0, "Gui", "gonvim_maximize", <args>)
		command! -nargs=? GonvimFullscreen call rpcnotify(0, "Gui", "gonvim_fullscreen", <args>)
		command! -nargs=+ GonvimWinpos call rpcnotify(0, "Gui", "gonvim_winpos", <f-args>)
		command! GonvimLigatures call rpcnotify(0, "Gui", "gonvim_ligatures")
		command! GonvimSmoothScroll call rpcnotify(0, "Gui", "gonvim_smoothscroll")
		command! GonvimSmoothCursor call rpcnotify(0, "Gui", "gonvim_smoothcursor")
		command! GonvimIndentguide call rpcnotify(0, "Gui", "gonvim_indentguide")
	`
	registerScripts = fmt.Sprintf(`call execute(%s)`, util.SplitVimscript(gonvimCommands))
	w.nvim.Command(registerScripts)

	if editor.config.Statusline.Visible {
		gonvimInitNotify := `
		call rpcnotify(0, "statusline", "bufenter", expand("%:p"), &filetype, &fileencoding, &fileformat, &ro)
		`
		initialNotify := fmt.Sprintf(`call execute(%s)`, util.SplitVimscript(gonvimInitNotify))
		w.nvim.Command(initialNotify)
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
	strChan := make(chan string, 5)
	go func() {
		colorscheme := ""
		w.nvim.Var("colors_name", &colorscheme)
		strChan <- colorscheme
	}()
	select {
	case colo := <-strChan:
		w.colorscheme = colo
	case <-time.After(80 * time.Millisecond):
	}
}

func (w *Workspace) getTS() {
	intChan := make(chan int, 5)
	go func() {
		ts := 8
		w.nvim.Option(editor.config.Editor.OptionsToUseGuideWidth, &ts)
		intChan <- ts
	}()
	select {
	case ts := <-intChan:
		w.ts = ts
	case <-time.After(80 * time.Millisecond):
	}
}

func (w *Workspace) getBuffTS(buf nvim.Buffer) int {
	intChan := make(chan int, 5)
	go func() {
		ts := 8
		w.nvim.BufferOption(buf, editor.config.Editor.OptionsToUseGuideWidth, &ts)
		intChan <- ts
	}()

	ts := 8
	select {
	case ts = <-intChan:
	case <-time.After(80 * time.Millisecond):
	}

	return ts
}

func (w *Workspace) getBG() {
	strChan := make(chan string, 5)
	go func() {
		screenbg := "dark"
		w.nvim.Option("background", &screenbg)
		strChan <- screenbg
	}()

	select {
	case screenbg := <-strChan:
		w.screenbg = screenbg
	case <-time.After(80 * time.Millisecond):
	}
}

func (w *Workspace) getKeymaps() {
	w.escKeyInInsert = "<Esc>"
	w.escKeyInNormal = "<Esc>"

	nmapChan := make(chan []*nvim.Mapping, 5)
	imapChan := make(chan []*nvim.Mapping, 5)

	// Get user mappings
	go func() {
		var nmappings, imappings []*nvim.Mapping
		var err1, err2 error
		nmappings, err1 = w.nvim.KeyMap("normal")
		if err1 != nil {
			return
		}
		nmapChan <- nmappings
		imappings, err2 = w.nvim.KeyMap("insert")
		if err2 != nil {
			return
		}
		imapChan <- imappings
	}()

	// wait to getting user mappings
	var ok [2]bool
	for {
		select {
		case nmappings := <-nmapChan:
			w.normalMappings = nmappings
			ok[0] = true
		case imappings := <-imapChan:
			w.insertMappings = imappings
			ok[1] = true
		case <-time.After(160 * time.Millisecond):
			ok[0] = true
			ok[1] = true
		}

		if ok[0] && ok[1] {
			break
		}
	}

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
	done := make(chan int, 5)
	num := 0
	go func() {
		tn := 0
		w.nvim.Eval("tabpagenr('$')", &tn)
		done <- tn
	}()
	select {
	case tn := <-done:
		num = tn
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
	case <-time.After(400 * time.Millisecond):
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
	if cwd == "" {
		cwd = w.getCwd()
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
	if editor.side == nil {
		return
	}
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
					}
				}
			}
		}
	}

	return o
}

func (w *Workspace) updateSize() (windowWidth, windowHeight int) {
	e := editor

	geometry := e.window.Geometry()
	width := geometry.Width()
	marginWidth := e.window.BorderSize()*4 + e.window.WindowGap()*2
	sideWidth := 0
	if e.side != nil {
		if e.side.widget.IsVisible() {
			sideWidth = e.splitter.Sizes()[0] + e.splitter.HandleWidth()
		}
	}
	width -= marginWidth + sideWidth

	height := geometry.Height()
	marginHeight := e.window.BorderSize() * 4
	titlebarHeight := 0
	if e.config.Editor.BorderlessWindow && runtime.GOOS != "linux" {
		titlebarHeight = e.window.TitleBar.Height()
	}
	height -= marginHeight + titlebarHeight

	tablineHeight := 0
	if w.isDrawTabline && w.tabline != nil {
		if w.tabline.showtabline != -1 {
			w.tabline.height = w.tabline.Tabs[0].widget.Height() + (TABLINEMARGIN * 2)
			tablineHeight = w.tabline.height
		}
	}

	statuslineHeight := 0
	if w.isDrawStatusline && w.statusline != nil {
		w.statusline.height = w.statusline.widget.Height()
		statuslineHeight = w.statusline.height
	}

	scrollbarWidth := 0
	if e.config.ScrollBar.Visible {
		scrollbarWidth = e.config.ScrollBar.Width
	}

	minimapWidth := 0
	if w.minimap != nil {
		if w.minimap.visible {
			minimapWidth = e.config.MiniMap.Width
		}
	}

	screenWidth := width - scrollbarWidth - minimapWidth
	screenHeight := height - tablineHeight - statuslineHeight

	rw := int(screenWidth) % int(w.screen.font.cellwidth)
	rh := screenHeight % w.screen.font.lineHeight
	screenWidth -= rw
	screenHeight -= rh
	width -= rw
	height -= rh

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

	if w.screen != nil {
		w.screen.width = screenWidth
		w.screen.height = screenHeight
		w.screen.updateSize()
	}
	if w.cursor != nil {
		w.cursor.Resize2(w.screen.width, w.screen.height)
		w.cursor.update()
	}
	if w.palette != nil {
		w.palette.resize()
	}
	if w.message != nil {
		w.message.resize()
	}

	windowWidth = marginWidth + sideWidth + scrollbarWidth + minimapWidth + w.screen.width
	windowHeight = marginHeight + titlebarHeight + tablineHeight + statuslineHeight + w.screen.height

	return
}

func (w *Workspace) updateApplicationWindowSize(cols, rows int) {
	e := editor
	font := w.font

	appWinWidth := int(font.cellwidth * float64(cols))
	appWinHeight := int(float64(font.lineHeight) * float64(rows))

	marginWidth := e.window.BorderSize()*4 + e.window.WindowGap()*2
	sideWidth := 0
	if e.side != nil {
		if e.side.widget.IsVisible() {
			sideWidth = e.splitter.Sizes()[0] + e.splitter.HandleWidth()
		}
	}
	appWinWidth += marginWidth + sideWidth

	marginHeight := e.window.BorderSize() * 4
	titlebarHeight := 0
	if e.config.Editor.BorderlessWindow && runtime.GOOS != "linux" {
		titlebarHeight = e.window.TitleBar.Height()
	}
	appWinHeight += marginHeight + titlebarHeight

	tablineHeight := 0
	if w.isDrawTabline && w.tabline != nil {
		if w.tabline.showtabline != -1 {
			w.tabline.height = w.tabline.Tabs[0].widget.Height() + (TABLINEMARGIN * 2)
			tablineHeight = w.tabline.height
		}
	}

	statuslineHeight := 0
	if w.isDrawStatusline && w.statusline != nil {
		w.statusline.height = w.statusline.widget.Height()
		statuslineHeight = w.statusline.height
	}

	scrollbarWidth := 0
	if e.config.ScrollBar.Visible {
		scrollbarWidth = e.config.ScrollBar.Width
	}

	minimapWidth := 0
	if w.minimap != nil {
		if w.minimap.visible {
			minimapWidth = e.config.MiniMap.Width
		}
	}

	appWinWidth += scrollbarWidth + minimapWidth
	appWinHeight += tablineHeight + statuslineHeight

	// Disable size specifications larger than the desktop screen size
	desktopRect := e.app.Desktop().AvailableGeometry2(e.window)
	desktopWidth := desktopRect.Width()
	desktopHeight := desktopRect.Height()
	if appWinWidth > desktopWidth {
		appWinWidth = desktopWidth
	}
	if appWinHeight > desktopHeight {
		appWinHeight = desktopHeight
	}

	e.putLog("update app win size::", appWinWidth, appWinHeight)

	e.window.Resize2(
		appWinWidth,
		appWinHeight,
	)

	return
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
	shouldUpdateMinimap := false
	shouldUpdateCursor := false
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
		case "option_set":
			w.setOption(update)
		case "mode_change":
			arg := update[len(update)-1].([]interface{})
			w.mode = arg[0].(string)
			w.modeIdx = util.ReflectToInt(arg[1])
			if w.cursor.modeIdx != w.modeIdx {
				w.cursor.modeIdx = w.modeIdx
			}
			w.disableImeInNormal()
			shouldUpdateCursor = true

		// Not used in the current specification.
		case "mouse_on":
		case "mouse_off":

		// Indicates to the UI that it must stop rendering the cursor. This event
		// is misnamed and does not actually have anything to do with busyness.
		// NOTE: In goneovim, the wdiget layer of the cursor has special processing,
		//       so it cannot be hidden straightforwardly.
		case "busy_start":
			w.cursor.isBusy = true
			shouldUpdateCursor = true
		case "busy_stop":
			w.cursor.isBusy = false
			shouldUpdateCursor = true

		case "suspend":
		case "update_menu":
		case "bell":
		case "visual_bell":

		case "flush":
			w.flush(shouldUpdateCursor, shouldUpdateMinimap)

		// Grid Events
		case "grid_resize":
			s.gridResize(args)
		case "default_colors_set":
			for _, u := range update[1:] {
				w.setColorsSet(u.([]interface{}))
			}
			// Show a window when connecting to the remote nvim.
			// The reason for handling the process here is that
			// in some cases, VimEnter will not occur if an error occurs in the remote nvim.
			if !editor.window.IsVisible() {
				if editor.opts.Ssh != "" {
					editor.window.Show()
				}
			}

			// Purge all text cache for window's
			w.screen.purgeTextCacheForWins()

		case "hl_attr_define":
			s.setHlAttrDef(args)
			// if goneovim own statusline is visible
			if w.isDrawStatusline {
				w.statusline.getColor()
			}
		case "hl_group_set":
			s.setHighlightGroup(args)
		case "grid_line":
			s.gridLine(args)
			shouldUpdateCursor = true
			shouldUpdateMinimap = true
		case "grid_clear":
			s.gridClear(args)
		case "grid_destroy":
			s.gridDestroy(args)
		case "grid_cursor_goto":
			s.gridCursorGoto(args)
			shouldUpdateMinimap = true
			shouldUpdateCursor = true
		case "grid_scroll":
			s.gridScroll(args)
			shouldUpdateMinimap = true

		// Multigrid Events
		case "win_pos":
			s.windowPosition(args)
		case "win_float_pos":
			s.windowFloatPosition(args)
		case "win_external_pos":
			s.windowExternalPosition(args)
		case "win_hide":
			s.windowHide(args)
		case "win_close":
			s.windowClose()
		case "msg_set_pos":
			s.msgSetPos(args)
		case "win_viewport":
			w.windowViewport(args)

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
				w.tabline.handle(args)
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

func (w *Workspace) flush(shouldUpdateCursor, shouldUpdateMinimap bool) {
	// handle viewport event for smooth scroll
	for {
		if len(w.viewportQue) == 0 {
			break
		}
		select {
		case viewport := <-w.viewportQue:
			win, diff, ok := w.handleViewport(viewport)
			if diff == 0 {
				continue
			}
			if ok {
				win.smoothScroll(diff)
			}
		default:
		}
	}

	// update cursor
	if shouldUpdateCursor {
		w.cursor.update()
	}

	// update screen
	w.screen.update()

	// update external statusline
	w.updateStatusline()

	// update external scrollbar
	w.updateScrollbar()

	// update IME tooltip
	w.updateIMETooltip()

	// update minimap
	if shouldUpdateMinimap {
		w.updateMinimap()
	}

	w.maxLineDelta = 0
}

func (w *Workspace) updateStatusline() {
	if w.isDrawStatusline {
		if w.statusline != nil {
			w.statusline.pos.redraw(w.viewport[2], w.viewport[3])
			w.statusline.mode.redraw()
		}
	}
}

func (w *Workspace) updateScrollbar() {
	if w.scrollBar != nil {
		if editor.config.ScrollBar.Visible {
			w.scrollBar.update()
		}
	}
}

func (w *Workspace) updateIMETooltip() {
	if w.screen.tooltip.IsVisible() {
		x, y, _, _ := w.screen.tooltip.pos()
		w.screen.tooltip.move(x, y)
	}
}

func (w *Workspace) updateMinimap() {
	if w.minimap != nil {
		if w.minimap.visible && w.minimap.widget.IsVisible() {
			w.scrollMinimap()
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
	if w.popup != nil {
		w.popup.setColor()
	}

	if w.message != nil {
		w.message.setColor()
	}
	// TODO w.screen.setColor()

	if w.isDrawStatusline {
		if w.statusline != nil {
			w.statusline.setColor()
		}
	}

	if w.scrollBar != nil {
		if editor.config.ScrollBar.Visible {
			w.scrollBar.setColor()
		}
	}

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

func (w *Workspace) windowViewport(args []interface{}) {
	for _, e := range args {
		arg := e.([]interface{})
		viewport := [4]int{
			util.ReflectToInt(arg[2]) + 1, // top
			util.ReflectToInt(arg[3]) + 1, // bottom
			util.ReflectToInt(arg[4]) + 1, // curline
			util.ReflectToInt(arg[5]) + 1, // curcol
		}

		scrollvp := [5]int{
			util.ReflectToInt(arg[2]) + 1,
			util.ReflectToInt(arg[3]) + 1,
			util.ReflectToInt(arg[4]) + 1,
			util.ReflectToInt(arg[5]) + 1,
			util.ReflectToInt(arg[0]),
		}
		if scrollvp[0] < scrollvp[1] {
			w.viewportQue <- scrollvp
		}

		// Only the viewport of the buffer where the cursor is located is used internally.
		grid := util.ReflectToInt(arg[0])
		if grid == w.cursor.gridid {
			if viewport != w.viewport {
				w.viewportMutex.Lock()
				w.oldViewport = w.viewport
				w.viewport = viewport
				w.viewportMutex.Unlock()
			}
		}
	}
}

func (w *Workspace) handleViewport(vp [5]int) (*Window, int, bool) {
	win, ok := w.screen.getWindow(vp[4])
	if !ok {
		return nil, 0, false
	}
	if win.isMsgGrid || vp[4] == 1 { // if grid is message grid or global grid
		return nil, 0, false
	}

	win.scrollViewport[1] = win.scrollViewport[0]
	win.scrollViewport[0] = vp
	viewport := win.scrollViewport[0]
	oldViewport := win.scrollViewport[1]

	if viewport[0] == oldViewport[0] && viewport[1] != oldViewport[1] {
		return nil, 0, false
	}

	diff := viewport[0] - oldViewport[0]
	if diff == 0 {
		diff = viewport[1] - oldViewport[1]
	}

	// // TODO: Control processing of wrapped lines.
	// //  This process is very incomplete and does not take into consideration the possibility
	// //  of a wrapped line at any position in the buffer.
	// if int(math.Abs(float64(diff))) >= win.rows/2 && viewport[1] < w.maxLine+2 {
	// 	wrappedLines1 := win.rows - (viewport[1] - viewport[0] - 1)
	// 	wrappedLines2 := win.rows - (oldViewport[1] - oldViewport[0] - 1)
	// 	if diff < 0 {
	// 		diff -= wrappedLines1
	// 	} else if diff > 0 {
	// 		diff += wrappedLines2
	// 	}
	// }

	// smooth scroll feature disabled
	if !editor.config.Editor.SmoothScroll {
		return nil, 0, false
	}

	// if If the maximum line is increased and there is content to be pasted into the maximum line
	if (w.maxLine == viewport[1]-1) && w.maxLineDelta != 0 {
		if diff != 0 {
			win.doGetSnapshot = false
		}
	}

	if win.doGetSnapshot {
		if !editor.isKeyAutoRepeating {
			win.grabScreenSnapshot(win.Rect())
		}
		win.doGetSnapshot = false
	}

	// // do not scroll smoothly when the maximum line is less than buffer rows,
	// // and topline has not been changed
	if w.maxLine-w.maxLineDelta < w.rows && viewport[0] == oldViewport[0] {
		return nil, 0, false
	}

	// suppress snapshot
	if editor.isKeyAutoRepeating {
		return nil, 0, false
	}

	if diff != 0 {
		win.scrollCols = int(math.Abs(float64(diff)))
	}

	// Compatibility of smooth scrolling with touchpad and smooth scrolling with scroll commands
	if win.lastScrollphase != core.Qt__ScrollEnd {
		return nil, 0, false
	}

	// isGridGoto := viewport[4] != oldViewport[4]
	// if isGridGoto {
	// 	return win, diff, false
	// }

	if diff == 0 {
		return win, diff, false
	}

	return win, diff, true
}

func (w *Workspace) scrollMinimap() {
	absMapTop := w.minimap.viewport[0]
	absMapBottom := w.minimap.viewport[1]

	w.viewportMutex.RLock()
	topLine := w.viewport[0]
	botLine := w.viewport[1]
	currLine := w.viewport[2]
	w.viewportMutex.RUnlock()

	switch {
	case botLine > absMapBottom:
		go func() {
			w.minimap.nvim.Input(`<ScrollWheelDown>`)
			w.minimap.nvim.Command(fmt.Sprintf("call cursor(%d, %d)", currLine, 0))
			w.minimap.nvim.Input(`zz`)
		}()
	case absMapTop > topLine:
		go func() {
			w.minimap.nvim.Input(`<ScrollWheelUp>`)
			w.minimap.nvim.Command(fmt.Sprintf("call cursor(%d, %d)", currLine, 0))
			w.minimap.nvim.Input(`zz`)
		}()
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
	case "gonvim_fullscreen":
		arg := 1
		if len(updates) == 2 {
			arg = util.ReflectToInt(updates[1])
		}
		if arg == 0 {
			// On MacOS, exiting from fullscreen does not work properly
			// unless the window is fullscreened again beforehand.
			if runtime.GOOS == "darwin" {
				editor.window.WindowFullScreen()
			}
			editor.window.WindowExitFullScreen()
			if runtime.GOOS == "darwin" && editor.savedGeometry != nil && editor.config.Editor.BorderlessWindow {
				editor.window.RestoreGeometry(editor.savedGeometry)
			}
		} else {
			if runtime.GOOS == "darwin" && editor.config.Editor.BorderlessWindow {
				editor.savedGeometry = editor.window.SaveGeometry()
			}
			editor.window.WindowFullScreen()
		}
	case "gonvim_maximize":
		arg := 1
		if len(updates) == 2 {
			arg = util.ReflectToInt(updates[1])
		}
		if arg == 0 {
			editor.window.WindowExitMaximize()
		} else {
			editor.window.WindowMaximize()
		}
	case "gonvim_winpos":
		if len(updates) == 3 {
			x, ok_x := strconv.Atoi(updates[1].(string))
			y, ok_y := strconv.Atoi(updates[2].(string))
			if (ok_x == nil && ok_y == nil) {
				newPos := core.NewQPoint2(x, y)
				editor.window.Move(newPos)
			}
		}
	case "gonvim_smoothscroll":
		w.toggleSmoothScroll()
	case "gonvim_smoothcursor":
		w.toggleSmoothCursor()
	case "gonvim_indentguide":
		w.toggleIndentguide()
	case "gonvim_ligatures":
		w.toggleLigatures()
	case "Font":
		w.guiFont(updates[1].(string))
	case "Linespace":
		w.guiLinespace(updates[1])
	// case "finder_pattern":
	// 	w.finder.showPattern(updates[1:])
	// case "finder_pattern_pos":
	// 	w.finder.cursorPos(updates[1:])
	// case "finder_show_result":
	// 	w.finder.showResult(updates[1:])
	// case "finder_show":
	// 	w.finder.show()
	// case "finder_hide":
	// 	w.finder.hide()
	// case "finder_select":
	// 	w.finder.selectResult(updates[1:])
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
	case "gonvim_letter_spacing":
		w.letterSpacing(updates[1])
	case "gonvim_grid_font":
		w.screen.gridFont(updates[1])
	case "gonvim_macmeta":
		w.handleMacmeta(updates[1])
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
		w.minimap.toggle()
	case "gonvim_colorscheme":
		if w.minimap != nil {
			w.minimap.isSetColorscheme = false
			w.minimap.setColorscheme()
		}

		win, ok := w.screen.getWindow(w.cursor.gridid)
		if !ok {
			return
		}
		win.dropScreenSnapshot()

	case "gonvim_copy_clipboard":
		go editor.copyClipBoard()
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
		w.optionSet(updates)
	case "gonvim_termenter":
		w.terminalMode = true
		w.cursor.update()
	case "gonvim_termleave":
		w.terminalMode = false
		w.cursor.update()
	case "gonvim_bufenter":
		w.maxLine = util.ReflectToInt(updates[1])
		// w.setBuffname(updates[2], updates[3])
		w.setBuffTS(util.ReflectToInt(updates[2]))
	case "gonvim_winenter_filetype":
		// w.setBuffname(updates[2], updates[3])
		w.setBuffTS(util.ReflectToInt(updates[2]))
		w.setFileType(updates)
	case "gonvim_textchanged":
		if editor.config.Editor.SmoothScroll {
			ws := editor.workspaces[editor.active]
			win, ok := ws.screen.getWindow(ws.cursor.gridid)
			if !ok {
				return
			}
			win.doGetSnapshot = true
		}
		w.maxLineDelta = util.ReflectToInt(updates[1]) - w.maxLine
		w.maxLine = util.ReflectToInt(updates[1])

	default:
		fmt.Println("unhandled Gui event", event)
	}

}

func (w *Workspace) getSnapshot() {
	if !editor.config.Editor.SmoothScroll {
		return
	}
	win, ok := w.screen.getWindow(w.cursor.gridid)
	if !ok {
		return
	}
	win.grabScreenSnapshot(win.Rect())
}

func (w *Workspace) letterSpacing(arg interface{}) {
	if arg == "" {
		return
	}

	letterSpace := util.ReflectToFloat(arg)
	editor.config.Editor.Letterspace = letterSpace

	w.font.changeLetterSpace(letterSpace)
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
	w.screen.tooltip.setFont(font)
	w.cursor.updateFont(nil, font)
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
			fontFamily = strings.Replace(font.Family(), " ", "_", -1)
			fontHeight = font.PointSizeF()
			editor.putLog(fmt.Sprintf("Request to change to the following font:: %s:h%f", fontFamily, fontHeight))

			// Fix the problem that the value of echo &guifont is set to * after setting.
			// w.guiFont(fmt.Sprintf("%s:h%f", fontFamily, fontHeight))
			w.nvim.Command(fmt.Sprintf("set guifont=%s:h%f", fontFamily, fontHeight))

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
	w.screen.tooltip.setFont(font)
	w.cursor.updateFont(nil, font)

	// Change external font if font setting of setting.yml is nothing
	if editor.config.Editor.FontFamily == "" {
		editor.extFontFamily = fontFamily
	}
	if editor.config.Editor.FontSize == 0 {
		editor.extFontSize = int(fontHeight)
	}

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
			editor.config.Editor.Letterspace,
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
	// A '_' can be used in the place of a space, so you don't need to use
	// backslashes to escape the spaces.
	family := strings.Replace(parts[0], "_", " ", -1)

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

// func (w *Workspace) setBuffname(idITF, nameITF interface{}) {
// 	id := (nvim.Window)(util.ReflectToInt(idITF))
//
// 	w.screen.windows.Range(func(_, winITF interface{}) bool {
// 		win := winITF.(*Window)
//
// 		if win == nil {
// 			return true
// 		}
// 		if win.grid == 1 {
// 			return true
// 		}
// 		if win.isMsgGrid {
// 			return true
// 		}
// 		if win.id != id && win.bufName != "" {
// 			return true
// 		}
//
// 		name := nameITF.(string)
// 		bufChan := make(chan nvim.Buffer, 10)
// 		var buf nvim.Buffer
// 		strChan := make(chan string, 10)
//
// 		win.updateMutex.RLock()
// 		id := win.id
// 		win.updateMutex.RUnlock()
// 		// set buffer name
// 		go func() {
// 			resultBuffer, _ := w.nvim.WindowBuffer(id)
// 			bufChan <- resultBuffer
// 		}()
//
// 		select {
// 		case buf = <-bufChan:
// 		case <-time.After(40 * time.Millisecond):
// 		}
//
// 		if win.bufName == "" {
// 			go func() {
// 				resultStr, _ := w.nvim.BufferName(buf)
// 				strChan <- resultStr
// 			}()
//
// 			select {
// 			case name = <-strChan:
// 			case <-time.After(40 * time.Millisecond):
// 			}
//
// 			win.bufName = name
// 		}
//
// 		// // NOTE: Getting buftype
// 		// // Process to get buftype. Comment it out when the time comes to need it.
// 		// errChan := make(chan error, 2)
// 		// var btITF interface{}
// 		// go func() {
// 		// 	err := w.nvim.BufferOption(buf, "buftype", &btITF)
// 		// 	errChan <- err
// 		// }()
// 		// var bt string
// 		// select {
// 		// case <-errChan:
// 		// 	bt = btITF.(string)
// 		// case <-time.After(40 * time.Millisecond):
// 		// }
//
// 		// win.bufType = bt
//
// 		return true
// 	})
// }

func (w *Workspace) setBuffTS(arg int) {
	if !editor.config.Editor.IndentGuide {
		return
	}
	bufChan := make(chan nvim.Buffer, 10)
	var buf nvim.Buffer
	wid := (nvim.Window)(arg)
	go func() {
		resultBuffer, _ := w.nvim.WindowBuffer(wid)
		bufChan <- resultBuffer
	}()
	select {
	case buf = <-bufChan:
	case <-time.After(40 * time.Millisecond):
	}
	w.windowsTs[wid] = w.getBuffTS(buf)

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
		if !win.IsVisible() {
			return true
		}

		win.updateMutex.RLock()
		id := win.id
		win.updateMutex.RUnlock()
		if id == wid {
			win.ts = w.windowsTs[wid]
			return true
		}
		// set buffer name
		go func() {
			resultBuffer, _ := w.nvim.WindowBuffer(id)
			bufChan <- resultBuffer
		}()
		select {
		case buf = <-bufChan:
		case <-time.After(40 * time.Millisecond):
		}

		win.ts = w.getBuffTS(buf)

		return true
	})
}

// optionSet is
// This function gets the value of an option that cannot be caught by the set_option event.
func (w *Workspace) optionSet(updates []interface{}) {
	optionName := updates[1]
	wid := util.ReflectToInt(updates[4])
	// new, err := strconv.Atoi(updates[2].(string))
	// if err != nil {
	// 	return
	// }

	w.optionsetMutex.Lock()
	switch optionName {
	case editor.config.Editor.OptionsToUseGuideWidth:
		w.setBuffTS(wid)

	}
	w.optionsetMutex.Unlock()
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

func (w *Workspace) setFileType(args []interface{}) {
	if !editor.config.Editor.IndentGuide {
		return
	}
	ft := args[1].(string)
	wid := (nvim.Window)(util.ReflectToInt(args[2]))

	for _, v := range editor.config.Editor.IndentGuideIgnoreFtList {
		if v == ft {
			return
		}
	}

	// NOTE: There are cases where the target grid is not yet created on the front-end side
	//       when the FileType autocmd is fired. There may be a better way to implement this,
	//       and it is an item for improvement in the future.
	w.optionsetMutex.Lock()
	w.windowsFt[wid] = ft
	w.optionsetMutex.Unlock()

	// Update ft on the window
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
		if win.id != wid {
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
			w.screen.tooltip.updateText(preeditString)
			w.screen.tooltip.update()
			w.screen.tooltip.show()
		}
	}
}

// InputMethodQuery is
func (w *Workspace) InputMethodQuery(query core.Qt__InputMethodQuery) *core.QVariant {
	if query == core.Qt__ImMicroFocus || query == core.Qt__ImCursorRectangle {
		x, y, candX, candY := w.screen.tooltip.pos()
		w.screen.tooltip.move(x, y)
		imrect := core.NewQRect()

		res := 0
		win, ok := w.screen.getWindow(w.cursor.gridid)
		if ok {
			if win.isMsgGrid {
				res = win.s.widget.Height() - win.rows*w.font.lineHeight
			}
			if res < 0 {
				res = 0
			}
		}
		imrect.SetRect(candX, candY+res+5, 1, w.font.lineHeight)

		if w.palette != nil {
			if w.palette.widget.IsVisible() {
				// w.cursor.x = float64(x + w.screen.tooltip.Width())
				// w.cursor.y = float64(w.palette.patternPadding + w.cursor.shift)
				// w.cursor.Update()
				w.cursor.Hide()
			}
		}

		return core.NewQVariant31(imrect)
	}
	return core.NewQVariant()
}

func (w *Workspace) dragEnterEvent(e *gui.QDragEnterEvent) {
	e.AcceptProposedAction()
}

func (w *Workspace) dragMoveEvent(e *gui.QDragMoveEvent) {
	e.AcceptProposedAction()
}

func (w *Workspace) dropEvent(e *gui.QDropEvent) {
	e.SetDropAction(core.Qt__CopyAction)
	e.AcceptProposedAction()
	e.SetAccepted(true)

	w.screen.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)
		if win == nil {
			return true
		}
		if win.isMsgGrid {
			win.move(win.pos[0], win.pos[1])
		}
		if win.isExternal {
			return true
		}
		if win.grid == 1 {
			return true
		}

		if win.Geometry().Contains(e.Pos(), true) {
			win.DropEvent(e)
			return false
		}

		return true
	})
}

func (w *Workspace) getPointInWidget(col, row, grid int) (int, int, int, bool) {
	win, ok := w.screen.getWindow(grid)
	if !ok {
		return 0, 0, w.font.lineHeight, false
	}
	font := win.getFont()

	isCursorBelowTheCenter := false
	if (win.pos[1]+row)*font.lineHeight > w.screen.height/2 {
		isCursorBelowTheCenter = true
	}

	x := int(float64(col) * font.cellwidth)
	y := row * font.lineHeight
	if w.isDrawTabline {
		if w.tabline != nil {
			y += w.tabline.widget.Height()
		}
	}
	x += int(float64(win.pos[0]) * font.cellwidth)
	y += win.pos[1] * font.lineHeight

	return x, y, font.lineHeight, isCursorBelowTheCenter
}

func (w *Workspace) toggleSmoothScroll() {
	editor.config.mu.Lock()
	if editor.config.Editor.SmoothScroll {
		editor.config.Editor.SmoothScroll = false
	} else {
		editor.config.Editor.SmoothScroll = true
	}
	editor.config.mu.Unlock()
}

func (w *Workspace) toggleSmoothCursor() {
	editor.config.mu.Lock()
	if editor.config.Cursor.SmoothMove {
		editor.config.Cursor.SmoothMove = false
	} else {
		editor.config.Cursor.SmoothMove = true
	}
	w.cursor.hasSmoothMove = editor.config.Cursor.SmoothMove
	editor.config.mu.Unlock()
}

func (w *Workspace) handleMacmeta(v interface{}) {
	value := util.ReflectToInt(v)
	editor.config.mu.Lock()
	if value == 0 {
		editor.config.Editor.Macmeta = false
	} else {
		editor.config.Editor.Macmeta = true
	}
	editor.config.mu.Unlock()
}

func (w *Workspace) toggleLigatures() {
	editor.config.mu.Lock()
	if editor.config.Editor.DisableLigatures {
		editor.config.Editor.DisableLigatures = false
		editor.config.Editor.Letterspace = 0
	} else {
		editor.config.Editor.DisableLigatures = true
	}
	editor.config.mu.Unlock()

	w.screen.purgeTextCacheForWins()
}

func (w *Workspace) toggleIndentguide() {
	editor.config.mu.Lock()
	if editor.config.Editor.IndentGuide {
		editor.config.Editor.IndentGuide = false
	} else {
		editor.config.Editor.IndentGuide = true
	}
	editor.config.mu.Unlock()
	go w.nvim.Command("doautocmd <nomodeline> WinEnter")
}

// WorkspaceSide is
type WorkspaceSide struct {
	widget       *widgets.QWidget
	scrollarea   *widgets.QScrollArea
	header       *widgets.QLabel
	scrollBg     *RGBA
	selectBg     *RGBA
	accent       *RGBA
	fg           *RGBA
	sfg          *RGBA
	scrollFg     *RGBA
	items        []*WorkspaceSideItem
	isShown      bool
	isInitResize bool
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
		if i >= len(editor.workspaces) {
			break
		}
		if side.items[i] == nil {
			continue
		}
		// if !side.items[i].active {
		// 	continue
		// }
		if editor.workspaces[i] != nil {
			if side.items[i].label.Text() == "" {
				editor.workspaces[i].setCwd(editor.workspaces[i].cwdlabel)
			}
		}
		side.items[i].setSideItemLabel(i)
		side.items[i].show()
		editor.workspaces[i].hide()
		if i == editor.active {
			editor.workspaces[i].show()
		}
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
	label         *widgets.QLabel
	content       *widgets.QListWidget
	side          *WorkspaceSide
	openIcon      *svg.QSvgWidget
	closeIcon     *svg.QSvgWidget
	widget        *widgets.QWidget
	layout        *widgets.QBoxLayout
	labelWidget   *widgets.QWidget
	text          string
	cwdpath       string
	hidden        bool
	active        bool
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

	// exec := ""
	// switch runtime.GOOS {
	// case "darwin":
	// 	exec = ":silent !open "
	// case "windows":
	// 	exec = ":silent !explorer "
	// case "linux":
	// 	exec = ":silent !xdg-open "
	// }
	exec := editor.config.Editor.FileOpenCmd + " "

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
