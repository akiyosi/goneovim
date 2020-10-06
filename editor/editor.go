package editor

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/akiyosi/goneovim/util"
	frameless "github.com/akiyosi/goqtframelesswindow"
	clipb "github.com/atotto/clipboard"
	"github.com/jessevdk/go-flags"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

var editor *Editor

const (
	GONEOVIMVERSION = "v0.4.8"
	WORKSPACELEN    = 10
)

// ColorPalette is
type ColorPalette struct {
	e *Editor

	fg                    *RGBA
	bg                    *RGBA
	inactiveFg            *RGBA
	comment               *RGBA
	abyss                 *RGBA
	matchFg               *RGBA
	selectedBg            *RGBA
	sideBarFg             *RGBA
	sideBarBg             *RGBA
	sideBarSelectedItemBg *RGBA
	scrollBarFg           *RGBA
	scrollBarBg           *RGBA
	widgetFg              *RGBA
	widgetBg              *RGBA
	widgetInputArea       *RGBA
	minimapCurrentRegion  *RGBA
	windowSeparator       *RGBA
	indentGuide           *RGBA
}

// NotifyButton is
type NotifyButton struct {
	action func()
	text   string
}

// Notify is
type Notify struct {
	level   NotifyLevel
	period  int
	message string
	buttons []*NotifyButton
}

type Option struct {
	Fullscreen bool   `long:"fullscreen" description:"Open the window in fullscreen on startup"`
	Maximized  bool   `long:"maximized" description:"Maximize the window on startup"`
	Geometry   string `long:"geometry" description:"Initial window geomtry [e.g. 800x600]"`

	Server string `long:"server" description:"Remote session address"`
	Nvim   string `long:"nvim" description:"Excutable nvim path to attach"`
}

// Editor is the editor
type Editor struct {
	signal  *editorSignal
	version string
	app     *widgets.QApplication
	ppid    int

	homeDir   string
	configDir string
	args      []string
	macAppArg string
	opts      Option

	notifyStartPos    *core.QPoint
	notificationWidth int
	notify            chan *Notify
	guiInit           chan bool

	workspaces  []*Workspace
	active      int
	window      *frameless.QFramelessWindow
	splitter    *widgets.QSplitter
	wsWidget    *widgets.QWidget
	wsSide      *WorkspaceSide
	sysTray     *widgets.QSystemTrayIcon
	chanVisible chan bool

	width    int
	height   int
	iconSize int

	stop     chan struct{}
	stopOnce sync.Once

	specialKeys        map[core.Qt__Key]string
	controlModifier    core.Qt__KeyboardModifier
	cmdModifier        core.Qt__KeyboardModifier
	keyControl         core.Qt__Key
	keyCmd             core.Qt__Key
	prefixToMapMetaKey string
	muMetaKey          sync.Mutex

	config                 gonvimConfig
	notifications          []*Notification
	isDisplayNotifications bool

	isSetGuiColor bool
	colors        *ColorPalette
	svgs          map[string]*SvgXML

	extFontFamily string
	extFontSize   int
}

type editorSignal struct {
	core.QObject
	_ func() `signal:"notifySignal"`
}

func (hl *Highlight) copy() Highlight {
	highlight := Highlight{}
	if hl.foreground != nil {
		highlight.foreground = hl.foreground.copy()
	}
	if hl.background != nil {
		highlight.background = hl.background.copy()
	}
	highlight.bold = hl.bold
	highlight.italic = hl.italic

	return highlight
}

// InitEditor is
func InitEditor() {
	// parse option
	var opts Option
	parser := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash)
	args, err := parser.ParseArgs(os.Args[1:])
	if flagsErr, ok := err.(*flags.Error); ok {
		switch flagsErr.Type {
		case flags.ErrDuplicatedFlag:
		case flags.ErrHelp:
			fmt.Println(err)
			os.Exit(1)
		}
	}

	// put shell environment
	putEnv()

	// detect home dir
	home, err := homedir.Dir()
	if err != nil {
		home = "~"
	}

	// detect config dir
	var configDir string
	if runtime.GOOS != "windows" {
		xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfigHome != "" {
			configDir = xdgConfigHome
		} else {
			configDir = filepath.Join(home, ".config", "goneovim")
		}
		if !isFileExist(configDir) {
			configDir = filepath.Join(home, ".goneovim")
		}
	} else {
		configDir = filepath.Join(home, ".goneovim")
	}

	editor = &Editor{
		version:     GONEOVIMVERSION,
		signal:      NewEditorSignal(nil),
		notify:      make(chan *Notify, 10),
		stop:        make(chan struct{}),
		guiInit:     make(chan bool, 1),
		config:      newGonvimConfig(configDir),
		homeDir:     home,
		configDir:   configDir,
		args:        args,
		opts:        opts,
		chanVisible: make(chan bool, 2),
	}
	e := editor

	// application
	core.QCoreApplication_SetAttribute(core.Qt__AA_EnableHighDpiScaling, true)
	e.app = widgets.NewQApplication(len(os.Args), os.Args)
	e.ppid = os.Getppid()

	// set application working directory path
	e.setAppDirPath(home)

	e.initFont()
	e.initSVGS()
	e.initColorPalette()
	e.initNotifications()
	e.initSysTray()

	// application main window
	isframeless := e.config.Editor.BorderlessWindow
	e.window = frameless.CreateQFramelessWindow(e.config.Editor.Transparent, isframeless)
	e.showWindow()
	e.setWindowSizeFromOpts()
	e.setWindowOptions()

	// window layout
	l := widgets.NewQBoxLayout(widgets.QBoxLayout__RightToLeft, nil)
	l.SetContentsMargins(0, 0, 0, 0)
	l.SetSpacing(0)
	e.window.SetupContent(l)

	// window content
	e.wsWidget = widgets.NewQWidget(nil, 0)
	e.wsSide = newWorkspaceSide()
	e.wsSide.newScrollArea()
	e.wsSide.scrollarea.Hide()
	e.newSplitter()
	l.AddWidget(e.splitter, 1, 0)

	// neovim workspaces
	e.initWorkspaces()

	e.connectAppSignals()

	e.window.ConnectCloseEvent(func(event *gui.QCloseEvent) {
		e.app.DisconnectEvent()
		event.Accept()
	})

	// runs goroutine to detect stop events and quit the application
	go func() {
		<-e.stop
		if runtime.GOOS == "darwin" {
			e.app.DisconnectEvent()
		}
		e.app.Quit()
	}()

	e.wsWidget.SetFocus2()

	widgets.QApplication_Exec()
}

// setAppDirPath
// Set the current working directory of the application to the HOME directory in darwin, linux.
// If this process is not executed, CWD is set to the root directory, and
// nvim plugins called as descendants of the application will not work due to lack of permission.
// e.g. #122
func (e *Editor) setAppDirPath(home string) {
	if runtime.GOOS == "windows" {
		return
	}
	if runtime.GOOS == "darwin" {
		if !(e.ppid == 1 && e.macAppArg == "") {
			return
		}
	}
	if runtime.GOOS == "linux" {
		if e.ppid != 1 {
			return
		}
	}

	path := core.QCoreApplication_ApplicationDirPath()
	absHome, err := util.ExpandTildeToHomeDirectory(home)
	if err == nil {
		if path != absHome {
			qdir := core.NewQDir2(path)
			qdir.SetCurrent(absHome)
		}
	}
}

func (e *Editor) newSplitter() {
	splitter := widgets.NewQSplitter2(core.Qt__Horizontal, nil)
	splitter.SetStyleSheet("* {background-color: rgba(0, 0, 0, 0);}")
	splitter.AddWidget(e.wsSide.scrollarea)
	splitter.AddWidget(e.wsWidget)
	splitter.SetStretchFactor(1, 100)
	splitter.SetObjectName("splitter")
	e.splitter = splitter

	if editor.config.SideBar.Visible {
		e.wsSide.show()
	}
}

func (e *Editor) initWorkspaces() {
	e.workspaces = []*Workspace{}
	sessionExists := false
	if e.config.Workspace.RestoreSession {
		for i := 0; i <= WORKSPACELEN; i++ {
			path := filepath.Join(e.configDir, "sessions", strconv.Itoa(i)+".vim")
			_, err := os.Stat(path)
			if err != nil {
				break
			}
			sessionExists = true
			ws, err := newWorkspace(path)
			if err != nil {
				break
			}
			e.workspaces = append(e.workspaces, ws)
		}
	}
	if !sessionExists {
		ws, err := newWorkspace("")
		if err != nil {
			return
		}
		e.workspaces = append(e.workspaces, ws)
	}

	e.workspaceUpdate()

	e.wsWidget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	e.wsWidget.ConnectInputMethodEvent(e.workspaces[e.active].InputMethodEvent)
	e.wsWidget.ConnectInputMethodQuery(e.workspaces[e.active].InputMethodQuery)
}

func (e *Editor) connectAppSignals() {
	if e.app == nil {
		return
	}
	e.app.ConnectAboutToQuit(func() {
		e.cleanup()
	})

	if runtime.GOOS != "darwin" {
		return
	}
	e.app.ConnectEvent(func(event *core.QEvent) bool {
		switch event.Type() {
		case core.QEvent__FileOpen:
			// If goneovim not launched on finder (it is started in terminal)
			if e.ppid != 1 {
				return false
			}
			fileOpenEvent := gui.NewQFileOpenEventFromPointer(event.Pointer())
			e.macAppArg = fileOpenEvent.File()
			e.loadFileInDarwin()
		}
		return true
	})
}

func (e *Editor) loadFileInDarwin() {
	if runtime.GOOS != "darwin" {
		return
	}

	goneovim := e.workspaces[e.active].nvim
	isModified := ""
	isModified, _ = goneovim.CommandOutput("echo &modified")
	if isModified == "1" {
		goneovim.Command(fmt.Sprintf(":tabe %s", e.macAppArg))
	} else {
		goneovim.Command(fmt.Sprintf(":e %s", e.macAppArg))
	}
}

func (e *Editor) initNotifications() {
	e.notifications = []*Notification{}
	e.notificationWidth = e.config.Editor.Width * 2 / 3
	e.notifyStartPos = core.NewQPoint2(e.width-e.notificationWidth-10, e.height-30)
	e.signal.ConnectNotifySignal(func() {
		notify := <-e.notify
		if notify.message == "" {
			return
		}
		if notify.buttons == nil {
			e.popupNotification(notify.level, notify.period, notify.message)
		} else {
			e.popupNotification(notify.level, notify.period, notify.message, notifyOptionArg(notify.buttons))
		}
	})
}

func (e *Editor) initSysTray() {
	if !e.config.Editor.DesktopNotifications {
		return
	}
	pixmap := gui.NewQPixmap()
	color := ""
	size := 0.95
	if runtime.GOOS == "darwin" {
		color = "#434343"
		size = 0.9
	} else {
		color = "#179A33"
	}
	svg := fmt.Sprintf(`<svg viewBox="0 0 128 128"><g transform="translate(2,3) scale(%f)"><path fill="%s" d="M72.6 80.5c.2.2.6.5.9.5h5.3c.3 0 .7-.3.9-.5l1.4-1.5c.2-.2.3-.4.3-.6l1.5-5.1c.1-.5 0-1-.3-1.3l-1.1-.9c-.2-.2-.6-.1-.9-.1h-4.8l-.2-.2-.1-.1c-.2 0-.4-.1-.6.1l-1.9 1.2c-.2 0-.3.5-.4.7l-1.6 4.9c-.2.5-.1 1.1.3 1.5l1.3 1.4zM73.4 106.9l-.4.1h-1.2l7.2-21.1c.2-.7-.1-1.5-.8-1.7l-.4-.1h-12.1c-.5.1-.9.5-1 1l-.7 2.5c-.2.7.3 1.3 1 1.5l.3-.1h1.8l-7.3 20.9c-.2.7.1 1.6.8 1.9l.4.3h11.2c.6 0 1.1-.5 1.3-1.1l.7-2.4c.3-.7-.1-1.5-.8-1.7zM126.5 87.2l-1.9-2.5v-.1c-.3-.3-.6-.6-1-.6h-7.2c-.4 0-.7.4-1 .6l-2 2.4h-3.1l-2.1-2.4v-.1c-.2-.3-.6-.5-1-.5h-4l20.2-20.2-22.6-22.4 20.2-20.8v-9l-2.8-3.6h-40.9l-3.3 3.5v2.9l-11.3-11.4-7.7 7.5-2.4-2.5h-40.4l-3.2 3.7v9.4l3 2.9h3v26.1l-14 14 14 14v32l5.2 2.9h11.6l9.1-9.5 21.6 21.6 14.5-14.5c.1.4.4.5.9.7l.4-.2h9.4c.6 0 1.1-.1 1.2-.6l.7-2c.2-.7-.1-1.3-.8-1.5l-.4.1h-.4l3.4-10.7 2.3-2.3h5l-5 15.9c-.2.7.2 1.1.9 1.4l.4-.2h9.1c.5 0 1-.1 1.2-.6l.8-1.8c.3-.7-.1-1.3-.7-1.6-.1-.1-.3 0-.5 0h-.4l4.2-13h6.1l-5.1 15.9c-.2.7.2 1.1.9 1.3l.4-.3h10c.5 0 1-.1 1.2-.6l.8-2c.3-.7-.1-1.3-.8-1.5-.1-.1-.3.1-.5.1h-.7l5.6-18.5c.2-.5.1-1.1-.1-1.4zm-63.8-82.3l11.3 11.3v4.7l3.4 4.1h1.6l-29 28v-28h3.3l2.7-4.2v-8.9l-.2-.3 6.9-6.7zm-59.8 59.2l12.1-12.1v24.2l-12.1-12.1zm38.9 38.3l58.4-60 21.4 21.5-20.2 20.2h-.1c-.3.1-.5.3-.7.5l-2.1 2.4h-2.9l-2.2-2.4c-.2-.3-.6-.6-1-.6h-8.8c-.6 0-1.1.4-1.3 1l-.8 2.5c-.2.7.1 1.3.8 1.6h1.5l-6.4 18.9-15.1 15.2-20.5-20.8z"></path></g></svg>`, size, color)
	pixmap.LoadFromData2(core.NewQByteArray2(svg, len(svg)), "SVG", core.Qt__ColorOnly)
	trayIcon := gui.NewQIcon2(pixmap)
	image := filepath.Join(e.configDir, "trayicon.png")
	if isFileExist(image) {
		trayIcon = gui.NewQIcon5(image)
	}
	e.sysTray = widgets.NewQSystemTrayIcon2(trayIcon, e.app)
	e.sysTray.Show()
}

func putEnv() {
	if runtime.GOOS == "linux" {
		exe, _ := os.Executable()
		dir, _ := filepath.Split(exe)
		_ = os.Setenv("LD_LIBRARY_PATH", dir+"lib")
		_ = os.Setenv("QT_PLUGIN_PATH", dir+"plugins")
		_ = os.Setenv("RESOURCE_NAME", "goneovim")
	}
	if runtime.GOOS == "darwin" {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = os.Getenv("/bin/bash")
		}
		cmd := exec.Command(shell, "-l", "-c", "env", "-i")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return
		}
		if err := cmd.Start(); err != nil {
			return
		}
		output, err := ioutil.ReadAll(stdout)
		if err != nil {
			stdout.Close()
			return
		}
		for _, b := range strings.Split(string(output), "\n") {
			splits := strings.Split(b, "=")
			if len(splits) > 1 {
				_ = os.Setenv(splits[0], splits[1])
			}
		}
	}
	_ = os.Setenv("QT_AUTO_SCREEN_SCALE_FACTOR", "1")
}

func (e *Editor) initFont() {
	e.extFontFamily = e.config.Editor.FontFamily
	e.extFontSize = e.config.Editor.FontSize
	e.app.SetFont(gui.NewQFont2(e.extFontFamily, e.extFontSize, 1, false), "QWidget")
	e.app.SetFont(gui.NewQFont2(e.extFontFamily, e.extFontSize, 1, false), "QLabel")
}

func (e *Editor) pushNotification(level NotifyLevel, p int, message string, opt ...NotifyOptionArg) {
	opts := NotifyOptions{}
	for _, o := range opt {
		o(&opts)
	}
	n := &Notify{
		level:   level,
		period:  p,
		message: message,
		buttons: opts.buttons,
	}
	e.notify <- n
	e.signal.NotifySignal()
}

func (e *Editor) popupNotification(level NotifyLevel, p int, message string, opt ...NotifyOptionArg) {
	e.updateNotificationPos()
	notification := newNotification(level, p, message, opt...)
	notification.widget.SetParent(e.window)
	notification.widget.AdjustSize()
	x := e.notifyStartPos.X()
	y := e.notifyStartPos.Y() - notification.widget.Height() - 4
	notification.widget.Move2(x, y)
	e.notifyStartPos = core.NewQPoint2(x, y)
	e.notifications = append(e.notifications, notification)
	notification.show()
}

func (e *Editor) initColorPalette() {
	rgbAccent := hexToRGBA(e.config.SideBar.AccentColor)
	fg := newRGBA(180, 185, 190, 1)
	bg := newRGBA(9, 13, 17, 1)
	c := &ColorPalette{
		e:          e,
		bg:         bg,
		fg:         fg,
		selectedBg: bg.brend(rgbAccent, 0.3),
		matchFg:    rgbAccent,
	}

	e.colors = c
	e.colors.update()
}

func (c *ColorPalette) update() {
	fg := c.fg
	bg := c.bg
	c.selectedBg = bg.brend(hexToRGBA(c.e.config.SideBar.AccentColor), 0.3)
	c.inactiveFg = warpColor(bg, -80)
	c.comment = warpColor(fg, -80)
	c.abyss = warpColor(bg, 5)
	c.sideBarFg = warpColor(fg, -5)
	c.sideBarBg = warpColor(bg, -5)
	c.sideBarSelectedItemBg = warpColor(bg, -15)
	c.scrollBarFg = warpColor(bg, -20)
	c.scrollBarBg = bg
	c.widgetFg = warpColor(fg, 5)
	c.widgetBg = warpColor(bg, -10)
	c.widgetInputArea = warpColor(bg, -30)
	c.minimapCurrentRegion = warpColor(bg, 20)
	if c.e.config.Editor.WindowSeparatorColor != "" {
		c.windowSeparator = hexToRGBA(c.e.config.Editor.WindowSeparatorColor)
	} else if c.e.config.Editor.WindowSeparatorTheme == "light" && c.e.config.Editor.WindowSeparatorColor == "" {
		if fg.R < 250 && fg.G < 250 && fg.B < 250 {
			c.windowSeparator = warpColor(fg, 10)
		} else {
			c.windowSeparator = warpColor(fg, -10)
		}
	} else if c.e.config.Editor.WindowSeparatorTheme == "dark" && c.e.config.Editor.WindowSeparatorColor == "" {
		if bg.R > 10 && bg.G > 10 && bg.B > 10 {
			c.windowSeparator = warpColor(bg, 10)
		} else {
			c.windowSeparator = warpColor(bg, -10)
		}
	}
	c.indentGuide = warpColor(bg, -30)
}

func (e *Editor) updateGUIColor() {
	e.workspaces[e.active].updateWorkspaceColor()

	// Do not use frameless drawing on linux
	if runtime.GOOS == "linux" {
		e.window.TitleBar.Hide()
		e.window.WindowWidget.SetStyleSheet(fmt.Sprintf(" #QFramelessWidget { background-color: rgba(%d, %d, %d, %f); border-radius: 0px;}", e.colors.bg.R, e.colors.bg.G, e.colors.bg.B, e.config.Editor.Transparent))
		e.window.SetWindowFlag(core.Qt__FramelessWindowHint, false)
		e.window.SetWindowFlag(core.Qt__NoDropShadowWindowHint, false)
		e.window.Show()
	} else {
		e.window.SetupWidgetColor((uint16)(e.colors.bg.R), (uint16)(e.colors.bg.G), (uint16)(e.colors.bg.B))
		e.window.SetupTitleColor((uint16)(e.colors.fg.R), (uint16)(e.colors.fg.G), (uint16)(e.colors.fg.B))
	}

	// e.window.SetWindowOpacity(1.0)
}

func hexToRGBA(hex string) *RGBA {
	format := "#%02x%02x%02x"
	if len(hex) == 4 {
		format = "#%1x%1x%1x"
	}
	var r, g, b uint8
	n, err := fmt.Sscanf(hex, format, &r, &g, &b)
	if err != nil {
		return nil
	}
	if n != 3 {
		return nil
	}
	rgba := &RGBA{
		R: (int)(r),
		G: (int)(g),
		B: (int)(b),
		A: 1,
	}

	return rgba
}

func (e *Editor) setWindowSizeFromOpts() {
	if e.opts.Geometry == "" {
		return
	}
	width, height := e.setWindowSize(e.opts.Geometry)
	e.config.Editor.Width = width
	e.config.Editor.Height = height
}

func (e *Editor) setWindowSize(s string) (int, int) {
	var width, height int
	var err error
	width, err = strconv.Atoi(strings.SplitN(s, "x", 2)[0])
	if err != nil || width < 400 {
		width = 400
	}
	height, err = strconv.Atoi(strings.SplitN(s, "x", 2)[1])
	if err != nil || height < 300 {
		height = 300
	}

	return width, height
}

func (e *Editor) setWindowOptions() {
	e.window.SetupTitle("Neovim")
	e.window.SetMinimumSize2(400, 300)
	e.initSpecialKeys()
	e.window.ConnectKeyPressEvent(e.keyPress)
	e.window.SetAttribute(core.Qt__WA_KeyCompression, false)
	e.window.SetAcceptDrops(true)
	if e.config.Editor.StartFullscreen || e.opts.Fullscreen {
		e.window.ShowFullScreen()
	} else if e.config.Editor.StartMaximizedWindow || e.opts.Maximized {
		e.window.WindowMaximize()
	}
}

func (e *Editor) showWindow() {
	e.width = e.config.Editor.Width
	e.height = e.config.Editor.Height
	e.window.Resize2(e.width, e.height)
	e.window.Show()
	// go func() {
	// 	for {
	// 		time.Sleep(20 * time.Millisecond)
	// 		if e.window.IsVisible() {
	// 			e.chanVisible <-true
	// 			return
	// 		}
	// 	}
	// }()
}

func isFileExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func (e *Editor) copyClipBoard() {
	var yankedText string
	yankedText, _ = e.workspaces[e.active].nvim.CommandOutput("echo getreg()")
	if yankedText != "" {
		if runtime.GOOS != "darwin" {
			clipb.WriteAll(yankedText)
		} else {
			c := e.app.Clipboard()
			c.SetText(yankedText, gui.QClipboard__Clipboard)
		}
	}

}

func (e *Editor) workspaceNew() {
	editor.isSetGuiColor = false
	ws, err := newWorkspace("")
	if err != nil {
		return
	}

	e.workspaces = append(e.workspaces, nil)
	e.active = len(e.workspaces) - 1

	e.workspaces[e.active] = ws
	e.workspaceUpdate()
}

func (e *Editor) workspaceSwitch(index int) {
	index--
	if index < 0 || index >= len(e.workspaces) {
		return
	}
	e.active = index
	e.workspaceUpdate()
}

func (e *Editor) workspaceNext() {
	e.active++
	if e.active >= len(e.workspaces) {
		e.active = 0
	}
	e.workspaceUpdate()
}

func (e *Editor) workspacePrevious() {
	e.active--
	if e.active < 0 {
		e.active = len(e.workspaces) - 1
	}
	e.workspaceUpdate()
}

func (e *Editor) workspaceUpdate() {
	if e.wsSide == nil {
		return
	}
	for i, ws := range e.workspaces {
		if i == e.active {
			ws.show()
		} else {
			ws.hide()
		}
	}
	for i := 0; i < len(e.wsSide.items) && i < len(e.workspaces); i++ {
		e.wsSide.items[i].setSideItemLabel(i)
		e.wsSide.items[i].setText(e.workspaces[i].cwdlabel)
		e.wsSide.items[i].show()
	}
	for i := len(e.workspaces); i < len(e.wsSide.items); i++ {
		e.wsSide.items[i].hide()
	}
}

func (e *Editor) keyPress(event *gui.QKeyEvent) {
	input := e.convertKey(event)
	if input != "" {
		e.workspaces[e.active].nvim.Input(input)
	}
}

func (e *Editor) convertKey(event *gui.QKeyEvent) string {
	text := event.Text()
	key := event.Key()
	mod := event.Modifiers()

	// this is macmeta alternatively
	if runtime.GOOS == "darwin" {
		if e.config.Editor.Macmeta {
			if mod&core.Qt__AltModifier > 0 && mod&core.Qt__ShiftModifier > 0 {
				text = string(key)
			} else if mod&core.Qt__AltModifier > 0 && !(mod&core.Qt__ShiftModifier > 0) {
				text = strings.ToLower(string(key))
			}
		}
	}

	if mod&core.Qt__KeypadModifier > 0 {
		switch core.Qt__Key(key) {
		case core.Qt__Key_Home:
			return fmt.Sprintf("<%sHome>", e.modPrefix(mod))
		case core.Qt__Key_End:
			return fmt.Sprintf("<%sEnd>", e.modPrefix(mod))
		case core.Qt__Key_PageUp:
			return fmt.Sprintf("<%sPageUp>", e.modPrefix(mod))
		case core.Qt__Key_PageDown:
			return fmt.Sprintf("<%sPageDown>", e.modPrefix(mod))
		case core.Qt__Key_Plus:
			return fmt.Sprintf("<%sPlus>", e.modPrefix(mod))
		case core.Qt__Key_Minus:
			return fmt.Sprintf("<%sMinus>", e.modPrefix(mod))
		case core.Qt__Key_multiply:
			return fmt.Sprintf("<%sMultiply>", e.modPrefix(mod))
		case core.Qt__Key_division:
			return fmt.Sprintf("<%sDivide>", e.modPrefix(mod))
		case core.Qt__Key_Enter:
			return fmt.Sprintf("<%sEnter>", e.modPrefix(mod))
		case core.Qt__Key_Period:
			return fmt.Sprintf("<%sPoint>", e.modPrefix(mod))
		case core.Qt__Key_0:
			return fmt.Sprintf("<%s0>", e.modPrefix(mod))
		case core.Qt__Key_1:
			return fmt.Sprintf("<%s1>", e.modPrefix(mod))
		case core.Qt__Key_2:
			return fmt.Sprintf("<%s2>", e.modPrefix(mod))
		case core.Qt__Key_3:
			return fmt.Sprintf("<%s3>", e.modPrefix(mod))
		case core.Qt__Key_4:
			return fmt.Sprintf("<%s4>", e.modPrefix(mod))
		case core.Qt__Key_5:
			return fmt.Sprintf("<%s5>", e.modPrefix(mod))
		case core.Qt__Key_6:
			return fmt.Sprintf("<%s6>", e.modPrefix(mod))
		case core.Qt__Key_7:
			return fmt.Sprintf("<%s7>", e.modPrefix(mod))
		case core.Qt__Key_8:
			return fmt.Sprintf("<%s8>", e.modPrefix(mod))
		case core.Qt__Key_9:
			return fmt.Sprintf("<%s9>", e.modPrefix(mod))
		}
	}

	if text == "<" {
		// return "<lt>"
		modNoShift := mod & ^core.Qt__ShiftModifier
		return fmt.Sprintf("<%s%s>", e.modPrefix(modNoShift), "lt")
	}

	specialKey, ok := e.specialKeys[core.Qt__Key(key)]
	if ok {
		return fmt.Sprintf("<%s%s>", e.modPrefix(mod), specialKey)
	}

	if text == "\\" {
		return fmt.Sprintf("<%s%s>", e.modPrefix(mod), "Bslash")
	}

	c := ""
	if text == "" {

		if key == int(core.Qt__Key_Alt     ) ||
		   key == int(core.Qt__Key_AltGr   ) ||
		   key == int(core.Qt__Key_CapsLock) ||
		   key == int(core.Qt__Key_Control ) ||
		   key == int(core.Qt__Key_Meta    ) ||
		   key == int(core.Qt__Key_Shift   ) ||
		   key == int(core.Qt__Key_Super_L ) ||
		   key == int(core.Qt__Key_Super_R ) {
			return ""
		}
		text = string(key)
		if !(mod&core.Qt__ShiftModifier > 0) {
			text = strings.ToLower(text)
		}
	}
	c = text
	if c == "" {
		return ""
	}

	char := core.NewQChar11(c)

	// Remove SHIFT
	if char.Unicode() >= 0x80 || char.IsPrint() {
		mod &= ^core.Qt__ShiftModifier
	}

	// Remove CTRL
	if char.Unicode() < 0x20 {
		mod &= ^e.controlModifier
	}

	if runtime.GOOS == "darwin" {
		// Remove ALT/OPTION
		if char.Unicode() >= 0x80 && char.IsPrint() {
			mod &= ^core.Qt__AltModifier
		}
	}

	prefix := e.modPrefix(mod)
	if prefix != "" {
		return fmt.Sprintf("<%s%s>", prefix, c)
	}

	return c
}

func (e *Editor) modPrefix(mod core.Qt__KeyboardModifier) string {
	prefix := ""
	if runtime.GOOS == "windows" {
		if mod&e.controlModifier > 0 && !(mod&core.Qt__AltModifier > 0) {
			prefix += "C-"
		}
		if mod&core.Qt__ShiftModifier > 0 {
			prefix += "S-"
		}
		if mod&core.Qt__AltModifier > 0 && !(mod&e.controlModifier > 0) {
			prefix += e.prefixToMapMetaKey
		}
	} else {
		if mod&e.cmdModifier > 0 {
			prefix += "D-"
		}
		if mod&e.controlModifier > 0 {
			prefix += "C-"
		}
		if mod&core.Qt__ShiftModifier > 0 {
			prefix += "S-"
		}
		if mod&core.Qt__AltModifier > 0 {
			prefix += e.prefixToMapMetaKey
		}
	}

	return prefix
}

func (e *Editor) initSpecialKeys() {
	e.specialKeys = map[core.Qt__Key]string{}
	e.specialKeys[core.Qt__Key_Up] = "Up"
	e.specialKeys[core.Qt__Key_Down] = "Down"
	e.specialKeys[core.Qt__Key_Left] = "Left"
	e.specialKeys[core.Qt__Key_Right] = "Right"

	e.specialKeys[core.Qt__Key_F1] = "F1"
	e.specialKeys[core.Qt__Key_F2] = "F2"
	e.specialKeys[core.Qt__Key_F3] = "F3"
	e.specialKeys[core.Qt__Key_F4] = "F4"
	e.specialKeys[core.Qt__Key_F5] = "F5"
	e.specialKeys[core.Qt__Key_F6] = "F6"
	e.specialKeys[core.Qt__Key_F7] = "F7"
	e.specialKeys[core.Qt__Key_F8] = "F8"
	e.specialKeys[core.Qt__Key_F9] = "F9"
	e.specialKeys[core.Qt__Key_F10] = "F10"
	e.specialKeys[core.Qt__Key_F11] = "F11"
	e.specialKeys[core.Qt__Key_F12] = "F12"
	e.specialKeys[core.Qt__Key_F13] = "F13"
	e.specialKeys[core.Qt__Key_F14] = "F14"
	e.specialKeys[core.Qt__Key_F15] = "F15"
	e.specialKeys[core.Qt__Key_F16] = "F16"
	e.specialKeys[core.Qt__Key_F17] = "F17"
	e.specialKeys[core.Qt__Key_F18] = "F18"
	e.specialKeys[core.Qt__Key_F19] = "F19"
	e.specialKeys[core.Qt__Key_F20] = "F20"
	e.specialKeys[core.Qt__Key_F21] = "F21"
	e.specialKeys[core.Qt__Key_F22] = "F22"
	e.specialKeys[core.Qt__Key_F23] = "F23"
	e.specialKeys[core.Qt__Key_F24] = "F24"
	e.specialKeys[core.Qt__Key_Backspace] = "BS"
	e.specialKeys[core.Qt__Key_Delete]    = "Del"
	e.specialKeys[core.Qt__Key_Insert]    = "Insert"
	e.specialKeys[core.Qt__Key_Home]      = "Home"
	e.specialKeys[core.Qt__Key_End]       = "End"
	e.specialKeys[core.Qt__Key_PageUp]    = "PageUp"
	e.specialKeys[core.Qt__Key_PageDown]  = "PageDown"
	e.specialKeys[core.Qt__Key_Return]    = "Enter"
	e.specialKeys[core.Qt__Key_Enter]     = "Enter"
	e.specialKeys[core.Qt__Key_Tab]       = "Tab"
	e.specialKeys[core.Qt__Key_Backtab]   = "Tab"
	e.specialKeys[core.Qt__Key_Escape]    = "Esc"
	e.specialKeys[core.Qt__Key_Backslash] = "Bslash"
	e.specialKeys[core.Qt__Key_Space]     = "Space"

	if runtime.GOOS == "darwin" {
		e.controlModifier = core.Qt__MetaModifier
		e.cmdModifier = core.Qt__ControlModifier
		e.keyControl = core.Qt__Key_Meta
		e.keyCmd = core.Qt__Key_Control
	} else if runtime.GOOS == "windows" {
		e.controlModifier = core.Qt__ControlModifier
		e.cmdModifier = (core.Qt__KeyboardModifier)(0)
		e.keyControl = core.Qt__Key_Control
		e.keyCmd = (core.Qt__Key)(0)
	} else {
		e.controlModifier = core.Qt__ControlModifier
		e.cmdModifier = core.Qt__MetaModifier
		e.keyControl = core.Qt__Key_Control
		e.keyCmd = core.Qt__Key_Meta
	}

	e.prefixToMapMetaKey = "A-"
}

func (e *Editor) close() {
	e.stopOnce.Do(func() {
		close(e.stop)
	})
}

func (e *Editor) cleanup() {
	sessions := filepath.Join(e.configDir, "sessions")
	os.RemoveAll(sessions)
	os.MkdirAll(sessions, 0755)

	select {
	case <-e.stop:
		return
	default:
	}

	for i, ws := range e.workspaces {
		sessionPath := filepath.Join(sessions, strconv.Itoa(i)+".vim")
		fmt.Println(sessionPath)
		fmt.Println(ws.nvim.Command("mksession " + sessionPath))
		fmt.Println("mksession finished")
	}
}
