package editor

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akiyosi/goneovim/filer"
	"github.com/akiyosi/goneovim/util"
	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
	"github.com/akiyosi/qt/svg"
	"github.com/akiyosi/qt/widgets"
	shortpath "github.com/akiyosi/short_path"
	"github.com/neovim/go-client/nvim"
)

type neovimSignal struct {
	core.QObject

	_ func() `signal:"stopSignal"`
	_ func() `signal:"redrawSignal"`
	_ func() `signal:"guiSignal"`

	_ func() `signal:"messageSignal"`

	_ func() `signal:"lazyLoadSignal"`
}

type ShouldUpdate struct {
	minimap    bool
	cursor     bool
	globalgrid bool
}

// Workspace is an editor workspace
type Workspace struct {
	shouldUpdate       *ShouldUpdate
	foreground         *RGBA
	layout2            *widgets.QHBoxLayout
	stop               chan struct{}
	font               *Font
	cursor             *Cursor
	tabline            *Tabline
	screen             *Screen
	scrollBar          *ScrollBar
	palette            *Palette
	popup              *PopupMenu
	cmdline            *Cmdline
	message            *Message
	minimap            *MiniMap
	fontdialog         *widgets.QFontDialog
	guiUpdates         chan []interface{}
	redrawUpdates      chan [][]interface{}
	flushCh            chan []interface{}
	signal             *neovimSignal
	nvim               *nvim.Nvim
	widget             *widgets.QWidget
	special            *RGBA
	background         *RGBA
	colorscheme        string
	cwdlabel           string
	escKeyInNormal     string
	mode               string
	cwdBase            string
	cwd                string
	escKeyInInsert     string
	filepath           string
	screenbg           string
	mouseScroll        string
	mouseScrollTemp    string
	normalMappings     []*nvim.Mapping
	modeInfo           []map[string]interface{}
	insertMappings     []*nvim.Mapping
	viewport           [5]int
	oldViewport        [5]int
	height             int
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
	isDrawTabline      bool
	isMouseEnabled     bool
	isTerminalMode     bool
	doGetSnapshot      bool
	doneGetSnapshot    bool
}

func newWorkspace() *Workspace {
	editor.putLog("initialize workspace")
	ws := &Workspace{
		stop:         make(chan struct{}),
		flushCh:      newFlushCh(),
		foreground:   newRGBA(255, 255, 255, 1),
		background:   newRGBA(0, 0, 0, 1),
		special:      newRGBA(255, 255, 255, 1),
		shouldUpdate: &ShouldUpdate{},
	}

	return ws
}

func (ws *Workspace) initUI() {
	ws.widget = widgets.NewQWidget(nil, 0)
	ws.widget.SetParent(editor.widget)
	ws.widget.SetAcceptDrops(true)
	ws.widget.ConnectDragEnterEvent(ws.dragEnterEvent)
	ws.widget.ConnectDragMoveEvent(ws.dragMoveEvent)
	ws.widget.ConnectDropEvent(ws.dropEvent)

	// Basic Workspace UI component
	// screen
	ws.screen = newScreen()
	ws.screen.ws = ws
	ws.screen.initInputMethodWidget()

	// cursor
	ws.cursor = initCursorNew()
	ws.cursor.SetParent(ws.widget)
	ws.cursor.ws = ws
	// ws.cursor.setBypassScreenEvent()

	// If ExtFooBar is true, then we create a UI component
	// tabline
	if editor.config.Editor.ExtTabline {
		ws.tabline = initTabline()
		ws.tabline.ws = ws
		ws.tabline.widget.ConnectShowEvent(ws.tabline.showEvent)
	}

	// cmdline
	if editor.config.Editor.ExtCmdline {
		ws.cmdline = initCmdline()
		ws.cmdline.ws = ws
	}

	// palette for cmdline
	if editor.config.Editor.ExtCmdline {
		ws.palette = initPalette()
		ws.palette.ws = ws
		ws.palette.widget.SetParent(ws.widget)
		ws.palette.setColor()
		ws.palette.hide()
	}

	// popupmenu
	if editor.config.Editor.ExtPopupmenu {
		ws.popup = initPopupmenuNew()
		ws.popup.widget.SetParent(ws.widget)
		ws.popup.ws = ws
		ws.popup.widget.Hide()
	}

	// messages
	if editor.config.Editor.ExtMessages {
		ws.message = initMessage()
		ws.message.ws = ws
		ws.message.widget.SetParent(ws.widget)
		ws.signal.ConnectMessageSignal(func() {
			ws.message.update()
		})
	}

	if ws.tabline != nil {
		ws.isDrawTabline = editor.config.Tabline.Visible && editor.config.Editor.ExtTabline
		ws.tabline.connectUI()
	}
	if ws.message != nil {
		ws.message.connectUI()
	}

	// workspace widget, layouts
	layout := widgets.NewQVBoxLayout()
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(0)
	ws.widget.SetContentsMargins(0, 0, 0, 0)
	ws.widget.SetLayout(layout)
	ws.widget.SetFocusPolicy(core.Qt__StrongFocus)
	ws.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	ws.widget.ConnectInputMethodEvent(ws.InputMethodEvent)
	ws.widget.ConnectInputMethodQuery(ws.InputMethodQuery)

	// screen widget and scrollBar widget
	widget2 := widgets.NewQWidget(nil, 0)
	widget2.SetContentsMargins(0, 0, 0, 0)
	ws.layout2 = widgets.NewQHBoxLayout()
	ws.layout2.SetContentsMargins(0, 0, 0, 0)
	ws.layout2.SetSpacing(0)
	ws.layout2.AddWidget(ws.screen.widget, 0, 0)
	widget2.SetLayout(ws.layout2)

	// assemble all neovim ui components
	if editor.config.Editor.ExtTabline {
		layout.AddWidget(ws.tabline.widget, 0, 0)
	}
	layout.AddWidget(widget2, 1, 0)

	ws.widget.Move2(0, 0)

	editor.putLog("assembled workspace UI components")
}

func (ws *Workspace) initFont() {
	ws.screen.font = editor.font
	ws.screen.fallbackfonts = editor.fallbackfonts
	ws.font = ws.screen.font
	ws.screen.tooltip.setFont(editor.font)
	ws.screen.tooltip.fallbackfonts = editor.fallbackfonts
	ws.font.ws = ws
	if ws.tabline != nil {
		ws.tabline.font = ws.font.qfont
	}
}

func (ws *Workspace) lazyLoadUI() {
	editor.putLog("Start    preparing for deferred drawing UI")

	editor.putLog("preparing scrollbar")
	// scrollbar
	if editor.config.ScrollBar.Visible {
		ws.scrollBar = newScrollBar()
		ws.scrollBar.ws = ws
	}

	editor.putLog("preparing minimap")
	// minimap
	if !editor.config.MiniMap.Disable {
		ws.minimap = newMiniMap()
		ws.minimap.ws = ws
		ws.layout2.AddWidget(ws.minimap.widget, 0, 0)
	}

	editor.putLog("preparing deferred drawing UI")
	if editor.config.ScrollBar.Visible {
		ws.layout2.AddWidget(ws.scrollBar.widget, 0, 0)
		ws.scrollBar.setColor()
	}

	editor.putLog("preparing filer")
	// Add editor feature
	go filer.RegisterPlugin(ws.nvim, editor.config.Editor.FileOpenCmd)

	editor.putLog("preparing minimap buffer")
	// Asynchronously execute the process for minimap
	if !editor.config.MiniMap.Disable {
		ws.minimap.startMinimapProc(editor.ctx)
		time.Sleep(time.Millisecond * 50)
		ws.minimap.mu.Lock()
		isMinimapVisible := ws.minimap.visible
		ws.minimap.mu.Unlock()
		if isMinimapVisible {
			ws.minimap.bufUpdate()
			ws.minimap.bufSync()
			ws.updateSize()
		}
	}

	editor.putLog("Finished preparing the deferred drawing UI.")
}

func (ws *Workspace) initLazyLoadUI() {
	editor.isWindowNowActivated = false

	ws.widget.ConnectFocusInEvent(func(event *gui.QFocusEvent) {
		go ws.nvim.SetFocusUI(true)
	})
	ws.widget.ConnectFocusOutEvent(func(event *gui.QFocusEvent) {
		go ws.nvim.SetFocusUI(false)
	})

	go func() {
		if !editor.doRestoreSessions {
			time.Sleep(time.Millisecond * 500)
		}
		ws.signal.LazyLoadSignal()

		if !editor.doRestoreSessions {
			time.Sleep(time.Millisecond * 500)
		}
		editor.signal.SidebarSignal()

		// put font debug log
		ws.font.putDebugLog()
	}()
}

func (ws *Workspace) registerSignal(signal *neovimSignal, redrawUpdates chan [][]interface{}, guiUpdates chan []interface{}) {
	ws.signal = signal
	ws.redrawUpdates = redrawUpdates
	ws.guiUpdates = guiUpdates

	ws.signal.ConnectRedrawSignal(func() {
		updates := <-ws.redrawUpdates
		editor.putLog("Received redraw event from neovim")
		ws.handleRedraw(updates)
	})
	ws.signal.ConnectGuiSignal(func() {
		updates := <-ws.guiUpdates
		editor.putLog("Received GUI event from neovim")
		ws.handleGui(updates)
	})
	ws.signal.ConnectLazyLoadSignal(func() {
		if ws.hasLazyUI {
			return
		}
		if editor.config.Editor.ExtTabline {
			ws.tabline.initTab()
		}
		editor.workspaceUpdate()
		ws.hasLazyUI = true
		ws.lazyLoadUI()
	})

	ws.signal.ConnectStopSignal(func() {
		// Need cleanup?
		workspaces := []*Workspace{}
		index := 0
		maxworkspaceIndex := len(editor.workspaces) - 1
		for i, wse := range editor.workspaces {
			if ws != wse {
				workspaces = append(workspaces, wse)
			} else {
				index = i
			}
		}

		if len(workspaces) == 0 {
			// TODO
			// If nvim is an instance on a remote server, the connection `cmd` can be
			// `ssh` or `wsl` command. What kind of exit status should be set?
			if ws.uiRemoteAttached || ws.nvim == nil {
				editor.close(0)
			} else {
				editor.close(ws.nvim.ExitCode())
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
				content.SetFont(editor.font.qfont)
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

		ws.hide()
		if editor.active == index {
			if index > 0 {
				editor.active--
			}
			editor.workspaceUpdate()
		}

	})
}

func (ws *Workspace) bindNvim(nvimCh chan *nvim.Nvim, uiRemoteAttachedCh chan bool, isSetWindowState, isLazyBind bool, file string) {
	ws.nvim = <-nvimCh
	ws.uiRemoteAttached = <-uiRemoteAttachedCh

	// Get nvim options
	ws.getGlobalOptions()

	// Adjust nvim geometry to fit application window size
	ws.uiAttached = true
	if len(editor.workspaces) == 1 {
		editor.chUiPrepared <- true
	}

	// Load goneovim's neovim settings
	loadHelpDoc(ws.nvim)
	loadGinitVim(ws.nvim)
	source(ws.nvim, file)

	// Initialize lazy load UI
	ws.initLazyLoadUI()
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

func (ws *Workspace) hide() {
	if ws.hidden {
		return
	}
	ws.hidden = true
	ws.widget.Hide()
}

func (ws *Workspace) show() {
	if !ws.hidden {
		return
	}
	ws.hidden = false
	ws.widget.Show()
	ws.widget.SetFocus2Default()
	ws.cursor.update()
}

func (ws *Workspace) getGlobalOptions() {
	ws.getColorscheme()
	ws.getBG()
	ws.getKeymaps()
	ws.getMousescroll()
}

func (ws *Workspace) getMousescroll() {
	msChan := make(chan string, 5)
	go func() {
		mousescroll := ""
		ws.nvim.Option("mousescroll", &mousescroll)
		msChan <- mousescroll
	}()

	select {
	case ws.mouseScroll = <-msChan:
	case <-time.After(NVIMCALLTIMEOUT * time.Millisecond):
	}
}

func (ws *Workspace) getColorscheme() {
	strChan := make(chan string, 5)
	go func() {
		colorscheme := ""
		ws.nvim.Var("colors_name", &colorscheme)
		strChan <- colorscheme
	}()
	select {
	case colo := <-strChan:
		ws.colorscheme = colo
	case <-time.After(NVIMCALLTIMEOUT * time.Millisecond):
	}
}

func (ws *Workspace) getBG() {
	strChan := make(chan string, 5)
	go func() {
		screenbg := "dark"
		ws.nvim.Option("background", &screenbg)
		strChan <- screenbg
	}()

	select {
	case screenbg := <-strChan:
		ws.screenbg = screenbg
	case <-time.After(NVIMCALLTIMEOUT * time.Millisecond):
	}
}

func (ws *Workspace) getKeymaps() {
	ws.escKeyInInsert = "<Esc>"
	ws.escKeyInNormal = "<Esc>"

	nmapChan := make(chan []*nvim.Mapping, 5)
	imapChan := make(chan []*nvim.Mapping, 5)

	// Get user mappings
	go func() {
		var nmappings, imappings []*nvim.Mapping
		var err1, err2 error
		nmappings, err1 = ws.nvim.KeyMap("normal")
		if err1 != nil {
			return
		}
		nmapChan <- nmappings
		imappings, err2 = ws.nvim.KeyMap("insert")
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
			ws.normalMappings = nmappings
			ok[0] = true
		case imappings := <-imapChan:
			ws.insertMappings = imappings
			ok[1] = true
		case <-time.After(NVIMCALLTIMEOUT * time.Millisecond):
			ok[0] = true
			ok[1] = true
		}

		if ok[0] && ok[1] {
			break
		}
	}

	altkeyCount := 0
	metakeyCount := 0
	for _, mapping := range ws.insertMappings {
		// Check Esc mapping
		if strings.EqualFold(mapping.RHS, "<Esc>") || strings.EqualFold(mapping.RHS, "<C-[>") {
			if mapping.NoRemap == 1 {
				ws.escKeyInInsert = mapping.LHS
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
	for _, mapping := range ws.normalMappings {
		if strings.EqualFold(mapping.RHS, "<Esc>") || strings.EqualFold(mapping.RHS, "<C-[>") {
			if mapping.NoRemap == 1 {
				ws.escKeyInNormal = mapping.LHS
			}
		}
		if strings.EqualFold(mapping.LHS, "<C-y>") || strings.EqualFold(mapping.LHS, "<C-e>") {
			ws.isMappingScrollKey = true
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

func (ws *Workspace) getNumOfTabs() int {
	done := make(chan int, 5)
	num := 0
	go func() {
		tn := 0
		ws.nvim.Eval("tabpagenr('$')", &tn)
		done <- tn
	}()
	select {
	case tn := <-done:
		num = tn
	case <-time.After(NVIMCALLTIMEOUT * time.Millisecond):
	}

	return num
}

func (ws *Workspace) getCwd() string {
	done := make(chan bool, 5)
	cwd := ""
	go func() {
		ws.nvim.Eval("getcwd()", &cwd)
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(NVIMCALLTIMEOUT * time.Millisecond):
	}

	return cwd
}

func (ws *Workspace) nvimEval(s string) (interface{}, error) {
	doneChannel := make(chan interface{}, 5)
	var result interface{}
	go func() {
		ws.nvim.Eval(s, &result)
		doneChannel <- result
	}()
	select {
	case done := <-doneChannel:
		return done, nil
	case <-time.After(NVIMCALLTIMEOUT * time.Millisecond):
		err := errors.New("neovim busy")
		return nil, err
	}
}

func (ws *Workspace) changeCwd(args []interface{}) {
	for _, arg := range args {
		cwd := (arg.([]interface{}))[0].(string)
		ws.setCwd(cwd)
	}
}

// func (ws *Workspace) handleChangeCwd(cwdinfo map[string]interface{}) {
// 	fmt.Println("handle change cwd:", cwdinfo)
// 	scope, ok := cwdinfo["scope"]
// 	if !ok {
// 		scope = "global"
// 	}
// 	cwdITF, ok := cwdinfo["cwd"]
// 	if !ok {
// 		return
// 	}
// 	cwd := cwdITF.(string)
// 	switch scope {
// 	case "global":
// 		ws.setCwd(cwd)
// 	case "tab":
// 		ws.setCwdInTab(cwd)
// 	case "window":
// 		ws.setCwdInWin(cwd)
// 	}
// }

func (ws *Workspace) setCwd(cwd string) {
	if cwd == "" {
		cwd = ws.getCwd()
	}
	ws.cwd = cwd

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
	ws.cwdlabel = labelpath
	ws.cwdBase = filepath.Base(cwd)
	if editor.side == nil {
		return
	}
	for i, wse := range editor.workspaces {
		if i >= len(editor.side.items) {
			return
		}

		if ws == wse {
			path, _ := filepath.Abs(cwd)
			sideItem := editor.side.items[i]
			if sideItem.cwdpath == path {
				continue
			}

			sideItem.label.SetText(wse.cwdlabel)
			sideItem.label.SetFont(editor.font.qfont)
			sideItem.cwdpath = path
		}
	}
}

// func (ws *Workspace) setCwdInTab(cwd string) {
// 	ws.screen.windows.Range(func(_, winITF interface{}) bool {
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
// 		if win.isShown() {
// 			win.cwd = cwd
// 		}
//
// 		return true
// 	})
// }
//
// func (ws *Workspace) setCwdInWin(cwd string) {
// 	ws.screen.windows.Range(func(_, winITF interface{}) bool {
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
// 		if win.grid == ws.cursor.gridid {
// 			win.cwd = cwd
// 		}
//
// 		return true
// 	})
// }

func (ws *Workspace) updateSize() (windowWidth, windowHeight, cols, rows int) {
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
	height -= marginHeight

	titlebarHeight := 0
	if e.config.Editor.BorderlessWindow && runtime.GOOS != "linux" {
		if !e.config.Editor.HideTitlebar {
			titlebarHeight = e.window.TitleBar.Height()
		}
	}
	height -= titlebarHeight

	tablineHeight := 0
	if ws.isDrawTabline && ws.tabline != nil {
		if ws.tabline.showtabline != -1 {
			ws.tabline.height = ws.tabline.Tabs[0].widget.Height() + (TABLINEMARGIN * 2)
			tablineHeight = ws.tabline.height
		}
	}

	scrollbarWidth := 0
	if e.config.ScrollBar.Visible {
		scrollbarWidth = e.config.ScrollBar.Width
	}

	minimapWidth := 0
	if ws.minimap != nil {
		if ws.minimap.visible {
			minimapWidth = e.config.MiniMap.Width
		}
	}

	screenWidth := width - scrollbarWidth - minimapWidth
	screenHeight := height - tablineHeight

	if ws.screen.font == nil {
		return
	}
	rw := screenWidth - int(math.Ceil(float64(int(float64(screenWidth)/ws.screen.font.cellwidth))*ws.screen.font.cellwidth))
	rh := screenHeight % ws.screen.font.lineHeight
	screenWidth -= rw
	screenHeight -= rh
	width -= rw
	height -= rh

	if width != ws.width || height != ws.height {
		ws.width = width
		ws.height = height
		ws.widget.Resize2(width, height)
		if !ws.hidden {
			ws.hide()
			ws.show()
		} else {
			ws.show()
			ws.hide()
		}
	}

	if ws.screen != nil {
		ws.screen.width = screenWidth
		ws.screen.height = screenHeight
		ws.screen.updateSize()
	}
	if ws.cursor != nil {
		ws.cursor.resize(ws.cursor.width, ws.cursor.height)
		ws.cursor.update()
	}
	if ws.palette != nil {
		ws.palette.resize()
	}
	if ws.message != nil {
		ws.message.resize()
	}

	windowWidth = marginWidth + sideWidth + scrollbarWidth + minimapWidth + ws.screen.width
	windowHeight = marginHeight + titlebarHeight + tablineHeight + ws.screen.height
	cols = ws.cols
	rows = ws.rows

	return
}

func (ws *Workspace) updateApplicationWindowSize(cols, rows int) {
	e := editor
	font := ws.font

	if e.window.WindowState() == core.Qt__WindowFullScreen ||
		e.window.WindowState() == core.Qt__WindowMaximized {
		return
	}

	appWinWidth := int(math.Ceil(font.cellwidth * float64(cols)))
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
		if !e.config.Editor.HideTitlebar {
			titlebarHeight = e.window.TitleBar.Height()
		}
	}
	appWinHeight += marginHeight + titlebarHeight

	tablineHeight := 0
	if ws.isDrawTabline && ws.tabline != nil {
		if ws.tabline.showtabline != -1 {
			ws.tabline.height = ws.tabline.Tabs[0].widget.Height() + (TABLINEMARGIN * 2)
			tablineHeight = ws.tabline.height
		}
	}

	scrollbarWidth := 0
	if e.config.ScrollBar.Visible {
		scrollbarWidth = e.config.ScrollBar.Width
	}

	minimapWidth := 0
	if ws.minimap != nil {
		if ws.minimap.visible {
			minimapWidth = e.config.MiniMap.Width
		}
	}

	appWinWidth += scrollbarWidth + minimapWidth
	appWinHeight += tablineHeight

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

func handleEvent(update interface{}) (event string, ok bool) {
	switch update.(type) {
	case string:
		event = update.(string)
		ok = true
	default:
		event = ""
		ok = false
	}

	return event, ok
}

func (ws *Workspace) handleRedraw(updates [][]interface{}) {

	ws.doGetSnapshot = ws.shouldGetSnapshot(updates)
	if ws.doGetSnapshot {
		ws.getSnapshot()
		ws.doneGetSnapshot = true
	}

	for _, update := range updates {
		event, ok := handleEvent(update[0])
		if !ok {
			continue
		}

		args := update[1:]
		editor.putLog("start   ", event)

		switch event {
		case "set_title":
			ws.setTitle(args)
		case "set_icon":
		case "mode_info_set":
			ws.modeInfoSet(args)
		case "option_set":
			ws.optionSet(args)
		case "chdir":
			ws.changeCwd(args)
		case "mode_change":
			ws.modeChange(args)

		case "mouse_on":
			ws.mouseOn()
		case "mouse_off":
			ws.mouseOff()

		case "busy_start":
			ws.busyStart()
		case "busy_stop":
			ws.busyStop()

		case "suspend":
		case "update_menu":
		case "bell":
		case "visual_bell":

		case "flush":
			ws.flush()

		// Grid Events
		case "grid_resize":
			ws.gridResize(args)
		case "default_colors_set":
			ws.defaultColorsSet(args)
		case "hl_attr_define":
			ws.hlAttrDefine(args)
		case "hl_group_set":
			ws.hlGroupSet(args)

		case "grid_line", "grid_clear", "grid_destroy", "grid_cursor_goto", "grid_scroll":
			select {
			case ws.flushCh <- update:

			case <-time.After(500 * time.Millisecond):
				// https://github.com/akiyosi/goneovim/issues/569
				// Due to the above issue, in certain situations, Nvim may continue notifying redraw events
				// without sending a flush event for a period of time. If the buffer size is too small,
				// this can cause the application to freeze.
				// Therefore, data on the existing channel queue is discarded and a new channel is re-
				// created if it cannot be sent to the channel.
				editor.putLog("flushCh send timeout, recreating channel")
				ws.flushCh = newFlushCh()
				ws.flushCh <- update
			}

		// Multigrid Events
		case "win_pos":
			ws.winPos(args)
		case "win_float_pos":
			ws.winFloatPos(args)
		case "win_external_pos":
			ws.winExternalPos(args)
		case "win_hide":
			ws.winHide(args)
		case "win_close":
			ws.winClose()
		case "msg_set_pos":
			ws.msgSetPos(args)
		case "win_viewport":
			ws.flushCh <- update
		case "win_viewport_margins":
			ws.flushCh <- update

		// Popupmenu Events
		case "popupmenu_show":
			ws.popupmenuShow(args)
		case "popupmenu_select":
			ws.popupmenuSelect(args)
		case "popupmenu_hide":
			ws.popupmenuHide(args)

		// Tabline Events
		case "tabline_update":
			ws.tablineUpdate(args)

		// Cmdline Events
		case "cmdline_show":
			ws.cmdlineShow(args)
		case "cmdline_pos":
			ws.cmdlinePos(args)
		case "cmdline_special_char":
		case "cmdline_char":
			ws.cmdlineChar(args)
		case "cmdline_hide":
			ws.cmdlineHide(args)
		case "cmdline_function_show":
			ws.cmdlineFunctionShow(args)
		case "cmdline_function_hide":
			ws.cmdlineFunctionHide(args)
		case "cmdline_block_show":
		case "cmdline_block_append":
		case "cmdline_block_hide":

		// Message/Dialog Events
		case "msg_show":
			ws.msgShow(args)
		case "msg_clear":
			ws.msgClear()
		case "msg_showmode":
		case "msg_showcmd":
		case "msg_ruler":
		case "msg_history_show":
			ws.msgHistoryShow(args)
		default:
		}

		editor.putLog("finished", event)
	}
}

func (ws *Workspace) flush() {
	close(ws.flushCh)
	for update := range ws.flushCh {
		event, ok := handleEvent(update[0])
		if !ok {
			continue
		}

		args := update[1:]
		switch event {
		case "grid_line":
			ws.gridLine(args)
		case "grid_clear":
			ws.gridClear(args)
		case "grid_destroy":
			ws.gridDestroy(args)
		case "grid_cursor_goto":
			ws.gridCursorGoto(args)
		case "grid_scroll":
			ws.gridScroll(args)
		case "win_viewport":
			ws.winViewport(args)
		case "win_viewport_margins":
			ws.winViewportMargins(args)
		}
	}
	ws.flushCh = newFlushCh()
	ws.doneGetSnapshot = false

	if ws.shouldUpdate.globalgrid {
		ws.screen.detectCoveredCellInGlobalgrid()
		ws.shouldUpdate.globalgrid = false
	}

	// update cursor
	if ws.shouldUpdate.cursor {
		ws.cursor.update()
		ws.shouldUpdate.cursor = false
	}

	// update screen
	ws.screen.update()

	// update external scrollbar
	ws.updateScrollbar()

	// update IME tooltip
	ws.updateIMETooltip()

	// update minimap
	if ws.shouldUpdate.minimap {
		ws.updateMinimap()
		ws.shouldUpdate.minimap = false
	}
}

// This function returns `flushCh` with an appropriate buffer size.
//
// `flushCh` is a channel that buffers specific `redraw` events and processes them collectively when a `flush` event is received.
// Neovim sends multiple `redraw` batches before completing a full screen redraw, with `flush` following only the last batch.
// This behavior can cause issues in smooth scrolling, as intermediate states before `flush` may lead to inconsistencies.
// To ensure a stable rendering state, only relevant events are processed when `flush` is received.
//
// In smooth scrolling, capturing the screen triggers a `paint` event internally.
// At this point, if `flush` has not yet been received, the UI state may be incomplete.
// Additionally, smooth scrolling relies on `win_viewport` events, but if UI events are dispatched in
// separate batches before `flush`, maintaining a consistent view becomes difficult.
// By buffering these events appropriately, the system ensures that rendering remains stable and coherent.
func newFlushCh() chan []interface{} {
	return make(chan []interface{}, 300)
}

func (ws *Workspace) updateScrollbar() {
	if ws.scrollBar != nil {
		if editor.config.ScrollBar.Visible {
			ws.scrollBar.update()
		}
	}
}

func (ws *Workspace) updateIMETooltip() {
	if ws.screen.tooltip.IsVisible() {
		x, y, _, _ := ws.screen.tooltip.pos()
		ws.screen.tooltip.move(x, y)
	}
}

func (ws *Workspace) updateMinimap() {
	if ws.minimap != nil {
		if ws.minimap.visible && ws.minimap.widget.IsVisible() {
			ws.scrollMinimap()
			ws.minimap.mapScroll()
		}
	}
}

func (ws *Workspace) disableImeInNormal() {
	if !editor.config.Editor.DisableImeInNormal {
		return
	}
	switch ws.mode {
	case "insert":
		ws.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
		editor.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	case "cmdline_normal":
		ws.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
		editor.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	default:
	}
}

func (ws *Workspace) shouldGetSnapshot(updates [][]interface{}) bool {
	doGetSnapshot := false
	for _, update := range updates {
		event, ok := handleEvent(update[0])
		if !ok {
			continue
		}

		args := update[1:]

		switch event {
		case "grid_scroll":
			for _, arg := range args {
				gridid := util.ReflectToInt(arg.([]interface{})[0])
				if gridid == 1 {
					continue
				}

				win, ok := ws.screen.getWindow(gridid)
				if !ok {
					continue
				}

				if win.isMsgGrid {
					continue
				}

				cols := arg.([]interface{})[5]
				if cols != 0 {
					doGetSnapshot = true
				}

			}
		case "win_viewport":
			delta := -1
			for _, a := range args {
				arg := a.([]interface{})
				if len(arg) >= 8 {
					delta = util.ReflectToInt(arg[7])
				}
				grid := util.ReflectToInt(arg[0])
				win, ok := ws.screen.getWindow(grid)
				if !ok {
					continue
				}
				if win.grid == 1 || win.isMsgGrid {
					continue
				}

				if delta != 0 {
					doGetSnapshot = true
				}
			}
		}
	}

	if ws.doneGetSnapshot {
		doGetSnapshot = false
	}

	return doGetSnapshot
}

func (ws *Workspace) modeEnablingIME(mode string) {
	if len(editor.config.Editor.ModeEnablingIME) == 0 {
		return
	}
	// if ws.mode == mode {
	// 	return
	// }
	if ws.isTerminalMode {
		mode = "terminal"
	}
	doEnable := false

	for _, m := range editor.config.Editor.ModeEnablingIME {
		if mode == m {
			doEnable = true
		}
	}
	if doEnable {
		ws.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
		editor.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	} else {
		ws.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, false)
		editor.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, false)
	}
}

func (ws *Workspace) setDefaultColorsSet(args []interface{}) {
	fg := util.ReflectToInt(args[0])
	bg := util.ReflectToInt(args[1])
	sp := util.ReflectToInt(args[2])

	editor.putLog("default colors set", fg, bg, sp)

	if fg != -1 {
		ws.foreground.R = calcColor(fg).R
		ws.foreground.G = calcColor(fg).G
		ws.foreground.B = calcColor(fg).B
	}
	if bg != -1 {
		ws.background.R = calcColor(bg).R
		ws.background.G = calcColor(bg).G
		ws.background.B = calcColor(bg).B
	}
	if sp != -1 {
		ws.special.R = calcColor(sp).R
		ws.special.G = calcColor(sp).G
		ws.special.B = calcColor(sp).B
	}

	editor.putLog(bg, ws.background.R, ws.background.G, ws.background.B)

	var isChangeFg, isChangeBg bool
	if editor.colors.fg != nil {
		isChangeFg = !editor.colors.fg.equals(ws.foreground)
	}
	if editor.colors.bg != nil {
		isChangeBg = !editor.colors.bg.equals(ws.background)
	}

	if isChangeFg || isChangeBg {
		editor.isSetGuiColor = false
		editor.putLog("isSetGuiColor:", editor.isSetGuiColor)
	}

	// If it is the second or subsequent nvim instance
	if len(editor.workspaces) > 1 {
		ws.updateWorkspaceColor()
		// Ignore setting GUI color when create second workspace and fg, bg equals -1
		if fg == -1 && bg == -1 {
			editor.isSetGuiColor = true
		}
	}

	// Exit if there is no change in foreground / background
	if editor.isSetGuiColor {
		return
	}

	editor.colors.fg = ws.foreground.copy()
	editor.colors.bg = ws.background.copy()
	// Reset hlAttrDef map 0 index:
	if ws.screen.hlAttrDef != nil {
		ws.screen.hlAttrDef[0] = &Highlight{
			foreground: editor.colors.fg,
			background: editor.colors.bg,
		}
	}

	editor.colors.update()
	if !(ws.colorscheme == "" && fg == -1 && bg == -1 && ws.screenbg == "dark") {
		editor.putLog(ws.colorscheme, fg, bg, ws.screenbg)
		editor.updateGUIColor()
	}
	editor.isSetGuiColor = true
}

func (ws *Workspace) updateWorkspaceColor() {
	editor.putLog("update Workspace Color")

	if ws.popup != nil {
		ws.popup.setColor()
	}

	if ws.message != nil {
		ws.message.setColor()
	}

	// ws.screen.setColor()

	if ws.cursor != nil {
		ws.cursor.setColor()
	}

	if ws.scrollBar != nil {
		if editor.config.ScrollBar.Visible {
			ws.scrollBar.setColor()
		}
	}

	if editor.side != nil {
		editor.side.setColor()
		editor.side.setColorForItems()
	}
}

func (ws *Workspace) setTitle(args []interface{}) {
	titleStr := (args[0].([]interface{}))[0].(string)
	editor.window.SetupTitle(titleStr)
	if runtime.GOOS == "linux" {
		editor.window.SetWindowTitle(titleStr)
	}
}

func (ws *Workspace) modeInfoSet(args []interface{}) {
	for _, arg := range args {
		ws.cursorStyleEnabled = arg.([]interface{})[0].(bool)
		modePropList := arg.([]interface{})[1].([]interface{})
		ws.modeInfo = make([]map[string]interface{}, len(modePropList))
		ws.cursor.isNeedUpdateModeInfo = true
		for i, modeProp := range modePropList {
			// Note: i is the index which given by the `mode_idx` of the `mode_change` event
			ws.modeInfo[i] = modeProp.(map[string]interface{})
		}
	}

	ws.cursor.modeIdx = 0
}

func (ws *Workspace) optionSet(args []interface{}) {
	for _, option := range args {
		key := (option.([]interface{}))[0].(string)
		val := (option.([]interface{}))[1]
		switch key {
		case "arabicshape":
		case "ambiwidth":
		case "emoji":
		case "guifont":
			ws.guiFont(val.(string))
		case "guifontset":
		case "guifontwide":
			ws.guiFontWide(val.(string))
		case "linespace":
			ws.guiLinespace(val)
		case "pumblend":
			ws.setPumblend(val)
			if ws.popup != nil {
				ws.popup.setPumblend(ws.pb)
			}
		case "showtabline":
			ws.showtabline = util.ReflectToInt(val)
		case "termguicolors":
		// case "ext_cmdline":
		// case "ext_hlstate":
		// case "ext_linegrid":
		// case "ext_multigrid":
		// case "ext_messages":
		// case "ext_popupmenu":
		// case "ext_tabline":
		// case "ext_termcolors":
		default:
		}
	}

	// Set Transparent blue effect
	if runtime.GOOS == "darwin" && editor.config.Editor.EnableBackgroundBlur {
		isLight := ws.screenbg == "light"
		editor.window.SetBlurEffectForMacOS(isLight)
	}
}

func (ws *Workspace) msgHistoryShow(args []interface{}) {
	if ws.message != nil {
		ws.message.msgHistoryShow(args)
	}
}

func (ws *Workspace) msgClear() {
	if ws.message != nil {
		ws.message.msgClear()
	}
}

func (ws *Workspace) msgShow(args []interface{}) {
	if ws.message != nil {
		ws.message.msgShow(args)
	}
}

func (ws *Workspace) cmdlineFunctionHide(args []interface{}) {
	if ws.cmdline != nil {
		ws.cmdline.functionHide()
	}
}

func (ws *Workspace) cmdlineFunctionShow(args []interface{}) {
	if ws.cmdline != nil {
		ws.cmdline.functionShow()
	}
}

func (ws *Workspace) cmdlineHide(args []interface{}) {
	if ws.cmdline != nil {
		ws.cmdline.hide()
	}
}

func (ws *Workspace) cmdlineChar(args []interface{}) {
	if ws.cmdline != nil {
		ws.cmdline.putChar(args)
	}
}

func (ws *Workspace) cmdlineSpecialChar(args []interface{}) {
}

func (ws *Workspace) cmdlinePos(args []interface{}) {
	if ws.cmdline != nil {
		ws.cmdline.changePos(args)
	}
}

func (ws *Workspace) cmdlineShow(args []interface{}) {
	if ws.cmdline != nil {
		ws.cmdline.show(args)
	}
}

func (ws *Workspace) tablineUpdate(args []interface{}) {
	if ws.tabline != nil {
		ws.tabline.handle(args)
	}
}

func (ws *Workspace) popupmenuHide(args []interface{}) {
	if ws.cmdline != nil {
		if ws.cmdline.shown {
			ws.cmdline.cmdWildmenuHide()
		}
	}
	if ws.popup != nil {
		if ws.cmdline != nil {
			if !ws.cmdline.shown {
				ws.popup.hide()
			}
		} else {
			ws.popup.hide()
		}
	}
}

func (ws *Workspace) popupmenuSelect(args []interface{}) {
	if ws.cmdline != nil {
		if ws.cmdline.shown {
			ws.cmdline.cmdWildmenuSelect(args)
		}
	}
	if ws.popup != nil {
		if ws.cmdline != nil {
			if !ws.cmdline.shown {
				ws.popup.selectItem(args)
			}
		} else {
			ws.popup.selectItem(args)
		}
	}
}

func (ws *Workspace) popupmenuShow(args []interface{}) {
	if ws.cmdline != nil {
		if ws.cmdline.shown {
			ws.cmdline.cmdWildmenuShow(args)
		}
	}
	if ws.popup != nil {
		if ws.cmdline != nil {
			if !ws.cmdline.shown {
				ws.popup.showItems(args)
			}
		} else {
			ws.popup.showItems(args)
		}
	}
}

func (ws *Workspace) msgSetPos(args []interface{}) {
	ws.screen.msgSetPos(args)
}

func (ws *Workspace) winClose() {
	ws.screen.windowClose()
}

func (ws *Workspace) winHide(args []interface{}) {
	ws.screen.windowHide(args)
}

func (ws *Workspace) winExternalPos(args []interface{}) {
	ws.screen.windowExternalPosition(args)
}

func (ws *Workspace) winFloatPos(args []interface{}) {
	ws.screen.windowFloatPosition(args)
}

func (ws *Workspace) winPos(args []interface{}) {
	ws.screen.windowPosition(args)
	ws.shouldUpdate.globalgrid = true
}

func (ws *Workspace) gridScroll(args []interface{}) {
	ws.screen.gridScroll(args)
	ws.shouldUpdate.minimap = true
}

func (ws *Workspace) gridCursorGoto(args []interface{}) {
	ws.screen.gridCursorGoto(args)
	ws.shouldUpdate.cursor = true
	ws.shouldUpdate.minimap = true
}

func (ws *Workspace) gridDestroy(args []interface{}) {
	ws.screen.gridDestroy(args)
}

func (ws *Workspace) gridClear(args []interface{}) {
	ws.screen.gridClear(args)
}

func (ws *Workspace) gridLine(args []interface{}) {
	ws.screen.gridLine(args)
	ws.shouldUpdate.cursor = true
	ws.shouldUpdate.minimap = true
}

func (ws *Workspace) hlGroupSet(args []interface{}) {
	ws.screen.setHighlightGroup(args)
}

func (ws *Workspace) hlAttrDefine(args []interface{}) {
	ws.screen.setHlAttrDef(args)
}

func (ws *Workspace) defaultColorsSet(args []interface{}) {
	for _, u := range args {
		ws.setDefaultColorsSet(u.([]interface{}))
	}

	// Purge all text cache for window's
	ws.screen.purgeTextCacheForWins()

}

func (ws *Workspace) gridResize(args []interface{}) {
	ws.screen.gridResize(args)
	ws.shouldUpdate.globalgrid = true
}

func (ws *Workspace) busyStart() {
	ws.cursor.isBusy = true
	ws.shouldUpdate.cursor = true
}

func (ws *Workspace) busyStop() {
	ws.cursor.isBusy = false
	ws.shouldUpdate.cursor = true
}

func (ws *Workspace) mouseOn() {
	ws.isMouseEnabled = true
}

func (ws *Workspace) mouseOff() {
	ws.isMouseEnabled = false
}

func (ws *Workspace) modeChange(args []interface{}) {
	arg := args[0].([]interface{})
	ws.modeEnablingIME(arg[0].(string))
	ws.mode = arg[0].(string)
	ws.modeIdx = util.ReflectToInt(arg[1])
	if ws.cursor.modeIdx != ws.modeIdx {
		ws.cursor.modeIdx = ws.modeIdx
	}
	ws.disableImeInNormal()
	ws.shouldUpdate.cursor = true
}

func (ws *Workspace) winViewport(args []interface{}) {
	// smooth scroll feature disabled
	if !editor.config.Editor.SmoothScroll {
		return
	}

	// Suppress smooth scroll rendering when key auto-repeat is enabled
	if editor.isKeyAutoRepeating {
		return
	}

	for _, e := range args {
		arg := e.([]interface{})

		grid := util.ReflectToInt(arg[0])
		top := util.ReflectToInt(arg[2]) + 1
		bottom := util.ReflectToInt(arg[3]) + 1
		curLine := util.ReflectToInt(arg[4]) + 1
		curCol := util.ReflectToInt(arg[5]) + 1
		viewport := [5]int{
			top,
			bottom,
			curLine,
			curCol,
			grid,
		}

		// fmt.Println(
		// 	fmt.Sprintf("top:%d, bottom:%d", top, bottom),
		// )

		maxLine := 0
		if len(arg) >= 7 {
			maxLine = util.ReflectToInt(arg[6])
		}

		delta := -1
		if len(arg) >= 8 {
			delta = util.ReflectToInt(arg[7])
		}

		// Only the viewport of the buffer where the cursor is located is used internally.
		if grid == ws.cursor.gridid {
			ws.viewportMutex.Lock()
			ws.oldViewport = ws.viewport
			ws.viewport = viewport
			ws.viewportMutex.Unlock()
			ws.maxLine = maxLine
		}

		if delta == 0 {
			continue
		}

		// // do not scroll smoothly when the maximum line is less than buffer rows
		// if ws.maxLine < ws.rows {
		// 	continue
		// }

		// Does not scroll smoothly if the size of the grid is increased without
		// changing the position of the top
		if top == ws.oldViewport[0] && bottom != ws.oldViewport[1] {
			continue
		}

		win, ok := ws.screen.getWindow(grid)
		if !ok {
			win = ws.screen.newWindowGird(grid)
		}

		// if grid is message grid or global grid
		if win.isMsgGrid || win.grid == 1 {
			continue
		}

		// Compatibility of smooth scrolling with touchpad and smooth scrolling with scroll commands
		if win.lastScrollphase != core.Qt__ScrollEnd {
			continue
		}

		// Suppresses smooth scrolling by command while touchpad scrolling is in progress
		if win.scrollPixels[1] != 0 || win.lastScrollphase != core.Qt__ScrollEnd {
			continue
		}

		// If the mouse is off in terminal mode and the cursor column is 0,
		// it is assumed that tig, lazygit, or other proprietary UI has been executed.
		if ws.isTerminalMode && ws.cursor.isBusy && curCol == 1 {
			win.dropScreenSnapshot()
			continue
		}

		if delta > 0 && delta > win.rows {
			delta = 1
			win.dropScreenSnapshot()
		}
		if delta < 0 && delta*-1 > win.rows {
			delta = -1
			win.dropScreenSnapshot()
		}

		win.smoothScroll(float64(delta))
	}
}

func (ws *Workspace) winViewportMargins(args []interface{}) {
	for _, e := range args {
		arg := e.([]interface{})

		grid := util.ReflectToInt(arg[0])
		top := util.ReflectToInt(arg[2])
		bottom := util.ReflectToInt(arg[3])
		left := util.ReflectToInt(arg[4])
		right := util.ReflectToInt(arg[5])

		win, ok := ws.screen.getWindow(grid)
		if !ok {
			win = ws.screen.newWindowGird(grid)
		}

		win.viewportMargins = [4]int{top, bottom, left, right}
	}
}

func (ws *Workspace) scrollMinimap() {
	absMapTop := ws.minimap.viewport[0]
	absMapBottom := ws.minimap.viewport[1]

	ws.viewportMutex.RLock()
	topLine := ws.viewport[0]
	botLine := ws.viewport[1]
	currLine := ws.viewport[2]
	ws.viewportMutex.RUnlock()

	switch {
	case botLine > absMapBottom:
		ws.minimap.nvim.Input(`<ScrollWheelDown>`)
		ws.minimap.nvim.Command(fmt.Sprintf("call cursor(%d, %d)", currLine, 0))
		ws.minimap.nvim.Input(`zz`)
	case absMapTop > topLine:
		ws.minimap.nvim.Input(`<ScrollWheelUp>`)
		ws.minimap.nvim.Command(fmt.Sprintf("call cursor(%d, %d)", currLine, 0))
		ws.minimap.nvim.Input(`zz`)
	default:
	}
}

func (ws *Workspace) handleGui(updates []interface{}) {
	event := updates[0].(string)
	switch event {
	case "gonvim_vimenter":
		go setupGoneovimClipBoard(ws.nvim)
	case "gonvim_uienter":
	case "gonvim_resize":
		arg, ok := updates[1].(string)
		if ok {
			width, height := editor.setWindowSize(arg)
			editor.window.Resize2(width, height)
		}
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
			if ok_x == nil && ok_y == nil {
				newPos := core.NewQPoint2(x, y)
				editor.window.Move(newPos)
			}
		}
	case "gonvim_toggle_horizontal_scroll":
		if editor.config.Editor.DisableHorizontalScroll {
			editor.config.Editor.DisableHorizontalScroll = false
		} else {
			editor.config.Editor.DisableHorizontalScroll = true
		}

	case "gonvim_smoothscroll":
		ws.toggleSmoothScroll()
	case "gonvim_smoothcursor":
		ws.toggleSmoothCursor()
	case "gonvim_indentguide":
		ws.toggleIndentguide()
	case "gonvim_ligatures":
		ws.toggleLigatures()
	case "gonvim_mousescroll_unit":
		ws.setMousescrollUnit(updates[1].(string))
	case "Font":
		ws.guiFont(updates[1].(string))
	case "Linespace":
		ws.guiLinespace(updates[1])
	// case "finder_pattern":
	// 	ws.finder.showPattern(updates[1:])
	// case "finder_pattern_pos":
	// 	ws.finder.cursorPos(updates[1:])
	// case "finder_show_result":
	// 	ws.finder.showResult(updates[1:])
	// case "finder_show":
	// 	ws.finder.show()
	// case "finder_hide":
	// 	ws.finder.hide()
	// case "finder_select":
	// 	ws.finder.selectResult(updates[1:])
	// case "signature_show":
	// 	ws.signature.showItem(updates[1:])
	// case "signature_pos":
	// 	ws.signature.pos(updates[1:])
	// case "signature_hide":
	// 	ws.signature.hide()
	case "side_open":
		editor.side.show()
	case "side_close":
		editor.side.hide()
	case "side_toggle":
		editor.side.toggle()
		ws.updateSize()
	case "filer_update":
		if !editor.side.scrollarea.IsVisible() {
			return
		}
		if !editor.side.items[editor.active].isContentHide {
			go ws.nvim.Call("rpcnotify", nil, 0, "GonvimFiler", "redraw")
		}
	case "filer_open":
		editor.side.items[ws.getNum()].isContentHide = false
		editor.side.items[ws.getNum()].openContent()
	case "filer_clear":
		editor.side.items[ws.getNum()].clear()
	case "filer_resize":
		editor.side.items[ws.getNum()].resizeContent()
	case "filer_item_add":
		editor.side.items[ws.getNum()].addItem(updates[1:])
	case "filer_item_select":
		editor.side.items[ws.getNum()].selectItem(updates[1:])
	case "gonvim_letter_spacing":
		ws.letterSpacing(updates[1])
	case "gonvim_grid_font":
		ws.screen.gridFont(updates[1])
	case "gonvim_macmeta":
		ws.handleMacmeta(updates[1])
	case "gonvim_minimap_update":
		if ws.minimap != nil {
			if ws.minimap.visible {
				ws.minimap.bufUpdate()
			}
		}
	case "gonvim_minimap_sync":
		if ws.minimap != nil {
			if ws.minimap.visible {
				ws.minimap.bufSync()
			}
		}
	case "gonvim_minimap_toggle":
		ws.minimap.toggle()
	case "gonvim_colorscheme":
		if ws.minimap != nil {
			ws.minimap.isSetColorscheme = false
			ws.minimap.setColorscheme()
		}

		win, ok := ws.screen.getWindow(ws.cursor.gridid)
		if !ok {
			return
		}
		win.dropScreenSnapshot()

	case "gonvim_workspace_new":
		editor.workspaceAdd()
	case "gonvim_workspace_next":
		editor.workspaceNext()
	case "gonvim_workspace_previous":
		editor.workspacePrevious()
	case "gonvim_workspace_switch":
		editor.workspaceSwitch(util.ReflectToInt(updates[1]))
	case "gonvim_workspace_filepath":
		if ws.minimap != nil {
			ws.minimap.mu.Lock()
			ws.filepath = updates[1].(string)
			ws.minimap.mu.Unlock()
		}
	case "gonvim_termenter":
		ws.isTerminalMode = true
		ws.modeEnablingIME(ws.mode)
	case "gonvim_termleave":
		ws.isTerminalMode = false
		ws.modeEnablingIME(ws.mode)
	case "gonvim_bufenter":
		wid := (nvim.Window)(util.ReflectToInt(updates[1]))

		win, ok := ws.screen.getGrid(wid)
		if !ok {
			return
		}

		if editor.config.Editor.IndentGuide {
			// get tabstop
			win.ts = util.ReflectToInt(
				ws.getBufferOption(NVIMCALLTIMEOUT, editor.config.Editor.OptionsToUseGuideWidth, wid),
			)

			// get filetype
			ftITF := ws.getBufferOption(NVIMCALLTIMEOUT, "filetype", wid)
			ft, ok := ftITF.(string)
			if !ok {
				return
			}
			win.ft = ft
		}

	case "gonvim_optionset":
		wid := (nvim.Window)(util.ReflectToInt(updates[4]))
		win, ok := ws.screen.getGrid(wid)
		if !ok {
			return
		}
		if win.lastScrollphase != core.Qt__ScrollEnd {
			return
		}

		optionName, ok := updates[1].(string)
		if !ok {
			return
		}
		ws.setOption(optionName, wid)

	default:
		fmt.Println("unhandled Gui event", event)
	}

}

func (ws *Workspace) getSnapshot() {
	if !editor.config.Editor.SmoothScroll {
		return
	}

	ws.screen.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)

		if win == nil {
			return true
		}
		if !win.IsVisible() {
			return true
		}

		// Suppress snapshot capture during touchpad scrolling
		if win.scrollPixels[1] != 0 || win.lastScrollphase != core.Qt__ScrollEnd {
			return true
		}
		win.grabScreenSnapshot()

		return true
	})
}

func (ws *Workspace) setMousescrollUnit(ms string) {
	if !(ms == "line" || ms == "smart" || ms == "pixel") {
		editor.config.Editor.MouseScrollingUnit = "line"
		return
	}
	editor.config.Editor.MouseScrollingUnit = ms
}

func (ws *Workspace) letterSpacing(arg interface{}) {
	if arg == "" {
		return
	}

	letterSpace := util.ReflectToInt(arg)
	editor.config.Editor.Letterspace = letterSpace

	ws.screen.font.changeLetterSpace(letterSpace)
	for _, font := range ws.screen.fallbackfonts {
		font.changeLetterSpace(letterSpace)
	}

	font := ws.screen.font
	fallbackfonts := ws.screen.fallbackfonts

	win, ok := ws.screen.getWindow(ws.cursor.gridid)
	if ok {
		font = win.getFont()
	}

	ws.updateSize()

	if ws.popup != nil {
		ws.popup.updateFont(font)
	}
	if ws.message != nil {
		ws.message.updateFont()
	}
	ws.screen.tooltip.setFont(font)
	ws.cursor.updateFont(nil, font, fallbackfonts)
}

func (ws *Workspace) handleFontDialog(guifontStr string, args string) {
	if ws.fontdialog == nil {
		fDialog := widgets.NewQFontDialog(nil)
		fDialog.SetOption(widgets.QFontDialog__MonospacedFonts, true)
		fDialog.SetOption(widgets.QFontDialog__ProportionalFonts, false)
		fDialog.ConnectFontSelected(func(font *gui.QFont) {
			ff := strings.Replace(font.Family(), " ", "_", -1)
			fh := font.PointSizeF()
			editor.putLog(fmt.Sprintf("Request to change to the following font:: %s:h%f", ff, fh))

			// Fix the problem that the value of echo &guifont is set to * after setting.
			// ws.guiFont(fmt.Sprintf("%s:h%f", fontFamily, fontHeight))
			ws.nvim.Command(fmt.Sprintf("set %s=%s:h%f", guifontStr, ff, fh))
		})
		ws.fontdialog = fDialog
	}
	ws.fontdialog.Show()
}

func (ws *Workspace) guiFont(args string) {
	if args == "" {
		return
	}

	editor.bindResizeEvent()

	if args == "*" {
		ws.handleFontDialog("guifont", args)
		return
	}

	ws.screen.fallbackfonts = nil

	ws.parseAndApplyFont(args, &ws.screen.font, &ws.screen.fallbackfonts)
	editor.showFontErrors()
	ws.screen.purgeTextCacheForWins()

	// When setting up a different font for a workspace other than the neovim drawing screen,
	// it is necessary to consider handling the fonts on the workspace side independently, etc.
	ws.font = ws.screen.font

	font := ws.screen.font
	fallbackfonts := ws.screen.fallbackfonts

	win, ok := ws.screen.getWindow(ws.cursor.gridid)
	if ok {
		font = win.getFont()
	}

	ws.updateSize()

	editor.iconSize = int(float64(ws.screen.font.height) * 11 / 9)

	if ws.popup != nil {
		ws.popup.updateFont(font)
	}
	if ws.message != nil {
		ws.message.updateFont()
	}

	ws.screen.tooltip.setFont(font)
	ws.screen.tooltip.fallbackfonts = fallbackfonts

	ws.cursor.updateFont(nil, font, fallbackfonts)
	ws.cursor.fallbackfonts = fallbackfonts

	// TODO:
	// Consideration of application UI policies related to external and Neovim internal fonts,
	// and provide a way to change external fonts.

	if ws.tabline != nil {
		ws.tabline.updateFont()
	}
}

func (ws *Workspace) guiFontWide(args string) {
	if args == "" {
		return
	}

	if args == "*" {
		ws.handleFontDialog("guifontwide", args)
		return
	}

	ws.screen.fallbackfontwides = nil

	ws.parseAndApplyFont(args, &ws.screen.fontwide, &ws.screen.fallbackfontwides)
	ws.screen.purgeTextCacheForWins()

	ws.updateSize()
}

func (ws *Workspace) parseAndApplyFont(str string, font *(*Font), fonts *([]*Font)) {
	for i, gfn := range strings.Split(str, ",") {
		fontFamily, fontHeight, fontWeight, fontStretch := getFontFamilyAndHeightAndWeightAndStretch(gfn)

		ok := checkValidFont(fontFamily)
		if !ok {
			editor.fontErrors = append(editor.fontErrors, fontFamily)
			continue
		}

		if i == 0 {
			if *font == nil {
				*font = initFontNew(
					fontFamily,
					fontHeight,
					fontWeight,
					fontStretch,
					ws.screen.font.lineSpace,
					ws.screen.font.letterSpace,
				)
			} else {
				(*font).change(fontFamily, fontHeight, fontWeight, fontStretch)
			}
		} else {
			ff := initFontNew(
				fontFamily,
				fontHeight,
				fontWeight,
				fontStretch,
				(*font).lineSpace,
				(*font).letterSpace,
			)
			*fonts = append(*fonts, ff)
		}
	}
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
	// f := gui.NewQFont2(family, 10.0, 1, false)
	f := gui.NewQFont()
	f.SetStyleHint(gui.QFont__TypeWriter, gui.QFont__NoFontMerging)
	f.SetFamily(family)
	fi := gui.NewQFontInfo(f)

	fname1 := fi.Family()
	fname2 := f.Family()

	ret := strings.EqualFold(fname1, fname2)
	if !ret {
		editor.putLog(
			fmt.Sprintf("The specified font family '%s' was not found on this system.", family),
		)
	}

	return ret
}

func (ws *Workspace) guiLinespace(args interface{}) {
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

	// #330: From a rendering architecture perspective, specifying negative values
	// may not render screen content correctly, but there is a need to set negative values,
	// so there is no restriction on setting negative values.
	// if lineSpace < 0 {
	// 	return
	// }
	if lineSpace <= -1*ws.font.height {
		return
	}

	ws.screen.font.changeLineSpace(lineSpace)
	for _, font := range ws.screen.fallbackfonts {
		font.changeLineSpace(lineSpace)
	}

	ws.font = ws.screen.font
	ws.updateSize()
}

func (ws *Workspace) setPumblend(arg interface{}) {
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

	ws.pb = pumblend
	ws.screen.purgeTextCacheForWins()
}

func (ws *Workspace) getWindowOption(timeout int, option, scope string, wid ...nvim.Window) string {
	opts := fmt.Sprintf(
		`{"scope":"%s"`,
		scope,
	)
	if len(wid) > 0 {
		opts += fmt.Sprintf(
			`, "win":%d}`,
			int(wid[0]),
		)
	} else {
		opts += "}"
	}
	nvimGetOptionValue := fmt.Sprintf(
		`echo nvim_get_option_value("%s", %s)`,
		option,
		opts,
	)
	c := make(chan string, 10)
	go func() {
		result, _ := ws.nvim.CommandOutput(nvimGetOptionValue)
		c <- result
	}()

	var result string
	select {
	case result = <-c:
	case <-time.After(time.Duration(timeout) * time.Millisecond):
	}

	return result
}

func (ws *Workspace) getBuffer(wid nvim.Window) (buf nvim.Buffer) {
	// get neovim buffer
	bufChan := make(chan nvim.Buffer, 10)
	go func() {
		resultBuffer, _ := ws.nvim.WindowBuffer(wid)
		bufChan <- resultBuffer
	}()
	select {
	case buf = <-bufChan:
	case <-time.After(NVIMCALLTIMEOUT * time.Millisecond):
	}

	return
}

func (ws *Workspace) getBufferOption(timeout int, option string, wid nvim.Window) interface{} {
	buf := ws.getBuffer(wid)

	// get buffer tabstop
	c := make(chan interface{}, 5)
	go func() {
		var result interface{}
		ws.nvim.BufferOption(buf, option, &result)
		c <- result
	}()

	var result interface{}
	select {
	case result = <-c:
	case <-time.After(time.Duration(timeout) * time.Millisecond):
	}

	return result
}

// setOption is
// This function gets the value of an option that cannot be caught by the set_option event.
func (ws *Workspace) setOption(optionName string, wid nvim.Window) {
	win, ok := ws.screen.getGrid(wid)
	if !ok {
		return
	}

	ws.optionsetMutex.Lock()
	switch optionName {
	case editor.config.Editor.OptionsToUseGuideWidth:
		win.ts = util.ReflectToInt(
			ws.getBufferOption(NVIMCALLTIMEOUT, optionName, wid),
		)
	case "filetype":
		ftITF := ws.getBufferOption(NVIMCALLTIMEOUT, optionName, wid)
		ft, ok := ftITF.(string)
		if !ok {
			return
		}
		win.ft = ft
	}
	ws.optionsetMutex.Unlock()
}

// InputMethodEvent is
func (ws *Workspace) InputMethodEvent(event *gui.QInputMethodEvent) {
	ws.screen.tooltip.cursorPos, ws.screen.tooltip.selectionLength = selectionPosInPreeditStr(event)

	if event.CommitString() != "" {
		ws.screen.tooltip.cursorVisualPos = 0
		ws.nvim.Input(event.CommitString())
		ws.screen.tooltip.hide()
		ws.screen.tooltip.clearText()
	} else {
		preeditString := event.PreeditString()

		if preeditString == "" {
			ws.screen.tooltip.hide()
			ws.screen.refresh()
		} else {
			ws.screen.tooltip.setGrid()
			ws.screen.tooltip.show()
			ws.screen.tooltip.parsePreeditString(preeditString)
			ws.screen.tooltip.update()

		}

		ws.screen.tooltip.updateVirtualCursorPos()
	}

	ws.cursor.update()

	editor.putLog(
		fmt.Sprintf(
			"QInputMethodEvent:: IME preeditstr: cursorpos: %d, selectionLength: %d, cursorVisualPos: %d",
			ws.screen.tooltip.cursorPos,
			ws.screen.tooltip.selectionLength,
			ws.screen.tooltip.cursorVisualPos,
		),
	)
}

// InputMethodQuery is
func (ws *Workspace) InputMethodQuery(query core.Qt__InputMethodQuery) *core.QVariant {
	if ws.screen == nil {
		return core.NewQVariant()
	}
	if ws.screen.tooltip == nil {
		return core.NewQVariant()
	}
	if !ws.screen.tooltip.isShown {
		return core.NewQVariant()
	}

	editor.putLog(
		fmt.Sprintf(
			"InputMethodQuery:: query: %d", query,
		),
	)

	if query == core.Qt__ImMicroFocus || query == core.Qt__ImCursorRectangle {
		x, y, candX, candY := ws.screen.tooltip.pos()
		ws.screen.tooltip.move(x, y)
		imrect := core.NewQRect()

		res := 0
		win, ok := ws.screen.getWindow(ws.cursor.gridid)
		if ok {
			if win.isMsgGrid {
				res = win.s.widget.Height() - win.rows*ws.font.lineHeight
			}
			if res < 0 {
				res = 0
			}
		}
		imrect.SetRect(candX, candY+res+5, 1, ws.font.lineHeight)

		return core.NewQVariant31(imrect)
	}

	return core.NewQVariant()
}

func (ws *Workspace) dragEnterEvent(e *gui.QDragEnterEvent) {
	e.AcceptProposedAction()
}

func (ws *Workspace) dragMoveEvent(e *gui.QDragMoveEvent) {
	e.AcceptProposedAction()
}

func (ws *Workspace) dropEvent(e *gui.QDropEvent) {
	e.SetDropAction(core.Qt__CopyAction)
	e.AcceptProposedAction()
	e.SetAccepted(true)

	ws.screen.windows.Range(func(_, winITF interface{}) bool {
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

func (ws *Workspace) getPointInWidget(col, row, grid int) (int, int, *Font, bool) {
	win, ok := ws.screen.getWindow(grid)
	if !ok {
		return 0, 0, ws.font, false
	}
	font := win.getFont()

	isCursorBelowTheCenter := false
	if (win.pos[1]+row)*font.lineHeight > ws.screen.height/2 {
		isCursorBelowTheCenter = true
	}

	x := int(float64(col) * font.cellwidth)
	y := row * font.lineHeight
	if ws.isDrawTabline {
		if ws.tabline != nil {
			y += ws.tabline.widget.Height()
		}
	}
	x += int(float64(win.pos[0]) * font.cellwidth)
	y += win.pos[1] * font.lineHeight

	return x, y, font, isCursorBelowTheCenter
}

func (ws *Workspace) toggleSmoothScroll() {
	editor.config.mu.Lock()
	if editor.config.Editor.SmoothScroll {
		editor.config.Editor.SmoothScroll = false
	} else {
		editor.config.Editor.SmoothScroll = true
	}
	editor.config.mu.Unlock()
}

func (ws *Workspace) toggleSmoothCursor() {
	editor.config.mu.Lock()
	if editor.config.Cursor.SmoothMove {
		editor.config.Cursor.SmoothMove = false
	} else {
		editor.config.Cursor.SmoothMove = true
	}
	ws.cursor.hasSmoothMove = editor.config.Cursor.SmoothMove
	editor.config.mu.Unlock()
}

func (ws *Workspace) handleMacmeta(v interface{}) {
	value := util.ReflectToInt(v)
	editor.config.mu.Lock()
	if value == 0 {
		editor.config.Editor.Macmeta = false
	} else {
		editor.config.Editor.Macmeta = true
	}
	editor.config.mu.Unlock()
}

func (ws *Workspace) toggleLigatures() {
	editor.config.mu.Lock()
	if editor.config.Editor.DisableLigatures {
		editor.config.Editor.DisableLigatures = false
		editor.config.Editor.Letterspace = 0
	} else {
		editor.config.Editor.DisableLigatures = true
	}
	editor.config.mu.Unlock()

	ws.screen.purgeTextCacheForWins()
}

func (ws *Workspace) toggleIndentguide() {
	editor.config.mu.Lock()
	if editor.config.Editor.IndentGuide {
		editor.config.Editor.IndentGuide = false
	} else {
		editor.config.Editor.IndentGuide = true
	}
	editor.config.mu.Unlock()
	ws.screen.refresh()
	go ws.nvim.Command("doautocmd <nomodeline> WinEnter")
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

	// HideMouseWhenTyping process
	if editor.config.Editor.HideMouseWhenTyping {
		widget.InstallEventFilter(widget)
		widget.SetMouseTracking(true)
	}
	widget.ConnectEventFilter(func(watched *core.QObject, event *core.QEvent) bool {
		switch event.Type() {
		case core.QEvent__MouseMove:
			if editor.isHideMouse && editor.config.Editor.HideMouseWhenTyping {
				gui.QGuiApplication_RestoreOverrideCursor()
				editor.isHideMouse = false
			}
		default:
		}

		return widget.EventFilterDefault(watched, event)
	})

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
		side.hide()
	} else {
		side.show()
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

func (ws *Workspace) getNum() int {
	for i, wse := range editor.workspaces {
		if ws == wse {
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
	content.SetFont(editor.font.qfont)
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
