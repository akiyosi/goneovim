package editor

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/akiyosi/goneovim/util"
	frameless "github.com/akiyosi/goqtframelesswindow"
	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
	"github.com/akiyosi/qt/widgets"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/neovim/go-client/nvim"
)

var editor *Editor

const (
	WORKSPACELEN    = 10
	NVIMCALLTIMEOUT = 320
)

type editorSignal struct {
	core.QObject
	_ func() `signal:"notifySignal"`
	_ func() `signal:"sidebarSignal"`
	_ func() `signal:"geometrySignal"`
}

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
	message string
	buttons []*NotifyButton
	level   NotifyLevel
	period  int
}

type Options struct {
	Geometry     string  `long:"geometry" description:"Initial window geometry [e.g. --geometry=800x600]"`
	Server       string  `long:"server" description:"Remote session address [e.g. --server=host:3456]"`
	Ssh          string  `long:"ssh" description:"Attaching to a remote nvim via ssh. Default port is 22. [e.g. --ssh=user@host:port]"`
	Nvim         string  `long:"nvim" description:"Executable nvim path to attach [e.g. --nvim=/path/to/nvim]"`
	Debug        string  `long:"debug" description:"Run debug mode with debug.log(default) file [e.g. --debug=/path/to/my-debug.log]" optional:"yes" optional-value:"debug.log"`
	Fullscreen   bool    `long:"fullscreen" description:"Open the window in fullscreen on startup"`
	Maximized    bool    `long:"maximized" description:"Maximize the window on startup"`
	Exttabline   bool    `long:"exttabline" description:"Externalize the tabline"`
	Extcmdline   bool    `long:"extcmdline" description:"Externalize the cmdline"`
	Extmessages  bool    `long:"extmessages" description:"Externalize the messages. Sets --extcmdline implicitly"`
	Extpopupmenu bool    `long:"extpopupmenu" description:"Externalize the popupmenu"`
	Version      bool    `long:"version" description:"Print Goneovim version"`
	Wsl          *string `long:"wsl" description:"Attach to nvim process in wsl environment with distribution(default) [e.g. --wsl=Ubuntu]" optional:"yes" optional-value:""`
	Nofork       bool    `long:"nofork" description:"Run in foreground"`
	NoConfig     bool    `long:"noconfig" description:"Run Goneovim with no config. (Equivalent to loading an empty settings.toml)"`
}

// Editor is the editor
type Editor struct {
	stop                   chan int
	signal                 *editorSignal
	ctx                    context.Context
	app                    *widgets.QApplication
	widget                 *widgets.QWidget
	splitter               *widgets.QSplitter
	window                 *frameless.QFramelessWindow
	specialKeys            map[core.Qt__Key]string
	svgs                   map[string]*SvgXML
	notifyStartPos         *core.QPoint
	colors                 *ColorPalette
	notify                 chan *Notify
	fontCh                 chan *Font
	cbChan                 chan *string
	chUiPrepared           chan bool
	geometryUpdateTimer    *time.Timer
	sysTray                *widgets.QSystemTrayIcon
	side                   *WorkspaceSide
	savedGeometry          *core.QByteArray
	prefixToMapMetaKey     string
	macAppArg              string
	extFontFamily          string
	configDir              string
	homeDir                string
	version                string
	config                 gonvimConfig
	opts                   Options
	font                   *Font
	notifications          []*Notification
	workspaces             []*Workspace
	args                   []string
	ppid                   int
	keyControl             core.Qt__Key
	keyCmd                 core.Qt__Key
	windowSize             [2]int
	width                  int
	active                 int
	startuptime            int64
	iconSize               int
	height                 int
	extFontSize            int
	notificationWidth      int
	stopOnce               sync.Once
	muMetaKey              sync.Mutex
	doRestoreSessions      bool
	isSetGuiColor          bool
	isDisplayNotifications bool
	isKeyAutoRepeating     bool
	isWindowResized        bool
	isWindowNowActivated   bool
	isWindowNowInactivated bool
	isExtWinNowActivated   bool
	isExtWinNowInactivated bool
	isHideMouse            bool
	isBindNvimSizeToAppwin bool
	isUiPrepared           bool
	isWindowMaximizing     bool
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
func InitEditor(options Options, args []string) {

	// --------------------
	// For profiling goneovim
	// --------------------
	//
	// https://blog.golang.org/pprof
	// After running the app, exec the following:
	//  $ go tool pprof -http=localhost:9090 cpuprofile
	//
	// Comment out the following::

	// //  * built-in net/http/pprof
	// f, ferr := os.Create("cpuprofile")
	// if ferr != nil {
	// 	os.Exit(1)
	// }
	// pprof.StartCPUProfile(f)
	//
	// g, ferr := os.Create("memprofile")
	// if ferr != nil {
	// 	os.Exit(1)
	// }

	// // * https://github.com/felixge/fgprof
	// f, ferr := os.Create("cpuprofile")
	// if ferr != nil {
	// 	os.Exit(1)
	// }
	// fgprofStop := fgprof.Start(f, fgprof.FormatPprof)

	editor = &Editor{
		version:      Version,
		args:         args,
		opts:         options,
		startuptime:  time.Now().UnixNano() / 1000,
		signal:       NewEditorSignal(nil),
		stop:         make(chan int),
		notify:       make(chan *Notify, 10),
		cbChan:       make(chan *string, 240),
		chUiPrepared: make(chan bool, 1),
	}
	e := editor

	// Prepare debug log
	e.setDebuglog()
	e.putLog("--- GONEOVIM STARTING ---")

	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx

	var err error
	// detect home dir
	e.homeDir, err = homedir.Dir()
	if err != nil {
		e.homeDir = "~"
	}
	e.putLog("detecting home directory path:", e.homeDir)

	// load config
	e.configDir, e.config = newConfig(e.homeDir, e.opts.NoConfig)
	e.putLog("Detecting the goneovim configuration directory:", e.configDir)
	e.overwriteConfigByCLIOption()

	// get parent process id
	e.ppid = os.Getppid()
	e.putLog("finished getting ppid")

	// put shell environment
	e.setEnvironmentVariables()

	// create qapplication
	e.putLog("start    generating the application")
	core.QCoreApplication_SetAttribute(core.Qt__AA_EnableHighDpiScaling, true)
	e.app = widgets.NewQApplication(len(os.Args), os.Args)
	e.app.SetDoubleClickInterval(0)
	e.putLog("finished generating the application")

	// new nvim instance
	signal, redrawUpdates, guiUpdates, nvimCh, uiRCh, errCh := newNvim(100, 10, e.ctx)

	// e.setAppDirPath(home)

	e.extFontFamily = e.config.Editor.FontFamily
	e.extFontSize = e.config.Editor.FontSize
	e.fontCh = make(chan *Font, 2)
	go func() {
		font := initFontNew(
			editor.extFontFamily,
			float64(editor.extFontSize),
			0,
			e.config.Editor.Letterspace,
		)

		e.fontCh <- font
	}()

	e.initSVGS()

	e.initColorPalette()

	e.initNotifications()

	e.initSysTray()

	e.initSpecialKeys()

	// application main window
	isSetWindowState := e.initAppWindow()
	e.window.Show()

	// window layout
	e.setWindowLayout()

	// neovim workspaces

	nvimErr := <-errCh
	if nvimErr != nil {
		fmt.Println(nvimErr)
		os.Exit(1)
	}

	e.initWorkspaces(e.ctx, signal, redrawUpdates, guiUpdates, nvimCh, uiRCh, isSetWindowState)

	e.connectAppSignals()

	// go e.exitEditor(cancel, f, g)
	go e.exitEditor(cancel)

	e.addDockMenu()

	widgets.QApplication_Exec()
}

func (e *Editor) initAppWindow() bool {
	e.putLog("start    preparing the application window.")
	defer e.putLog("finished preparing the application window.")

	isframeless := e.config.Editor.BorderlessWindow
	e.window = frameless.CreateQFramelessWindow(e.config.Editor.Transparent, isframeless)
	e.connectWindowEvents()
	e.setWindowOptions()
	e.setWindowSizeFromOpts()

	return e.setInitialWindowState()
}

// exitEditor is to detect stop events and quit the application
// func (e *Editor) exitEditor(cancel context.CancelFunc, f, g *os.File) {
func (e *Editor) exitEditor(cancel context.CancelFunc) {
	ret := <-e.stop
	close(e.stop)
	e.putLog("The application was quitted with the exit of Neovim.")
	if runtime.GOOS == "darwin" {
		e.app.DisconnectEvent()
	}
	e.saveAppWindowState()
	cancel()

	// --------------------
	// profile the goneovim
	// --------------------
	//
	// Comment out the following::

	// // * built-in net/http/pprof
	// pprof.StopCPUProfile()
	// f.Close()
	//
	// runtime.GC()
	// pprof.WriteHeapProfile(g)
	// g.Close()

	// // * https://github.com/felixge/fgprof
	// fgprofStop()
	// f.Close()

	os.Exit(ret)
}

func (e *Editor) putLog(v ...interface{}) {
	if e.opts.Debug == "" {
		return
	}

	log.Println(
		fmt.Sprintf("%07.3f", float64(time.Now().UnixNano()/1000-e.startuptime)/1000),
		strings.TrimRight(strings.TrimLeft(fmt.Sprintf("%v", v), "["), "]"),
	)
}

func (e *Editor) bindResizeEvent() {
	e.window.ConnectResizeEvent(func(event *gui.QResizeEvent) {
		if !editor.isBindNvimSizeToAppwin {
			return
		}
		if len(e.workspaces) == 0 {
			return
		}
		e.resizeMainWindow()
	})
}

// addDockMenu add the action menu for app in the Dock.
func (e *Editor) addDockMenu() {
	if runtime.GOOS != "darwin" {
		return
	}
	appExecutable := core.QCoreApplication_ApplicationFilePath()

	menu := widgets.NewQMenu(nil)
	action1 := menu.AddAction("New Instance")
	action1.ConnectTriggered(func(checked bool) {
		go func() {
			cmd := exec.Command(appExecutable)
			cmd.Start()
		}()
	})

	action2 := menu.AddAction("New Instance with -u NONE")
	action2.ConnectTriggered(func(checked bool) {
		go func() {
			cmd := exec.Command(appExecutable, "-u", "NONE")
			cmd.Start()
		}()
	})

	for key, string := range e.config.Editor.DockmenuActions {
		action := menu.AddAction(key)
		strSlice := strings.Split(string, " ")
		action.ConnectTriggered(func(checked bool) {
			go func() {
				cmd := exec.Command(appExecutable, strSlice...)
				cmd.Start()
			}()
		})
	}

	menu.SetAsDockMenu()
}

// setAppDirPath
// set application working directory path
// TODO: This process is problematic and needs a better way to set up CWD
//        * https://github.com/akiyosi/goneovim/issues/43
//        * https://github.com/akiyosi/goneovim/issues/337
//        * https://github.com/akiyosi/goneovim/issues/325
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

	e.putLog("set working directory")
}

func (e *Editor) overwriteConfigByCLIOption() {
	e.config.Editor.ExtTabline = e.opts.Exttabline || e.config.Editor.ExtTabline
	e.config.Editor.ExtCmdline = e.opts.Extcmdline || e.config.Editor.ExtCmdline
	e.config.Editor.ExtPopupmenu = e.opts.Extpopupmenu || e.config.Editor.ExtPopupmenu
	e.config.Editor.ExtMessages = e.opts.Extmessages || e.config.Editor.ExtMessages
	e.config.Editor.ExtCmdline = e.opts.Extmessages || e.config.Editor.ExtCmdline

	e.config.Editor.StartFullscreen = e.opts.Fullscreen || e.config.Editor.StartFullscreen
	e.config.Editor.StartMaximizedWindow = e.opts.Maximized || e.config.Editor.StartMaximizedWindow
}

func (e *Editor) newSplitter() {
	splitter := widgets.NewQSplitter2(core.Qt__Horizontal, nil)
	splitter.SetStyleSheet("* {background-color: rgba(0, 0, 0, 0);}")
	splitter.SetStretchFactor(1, 100)
	splitter.SetObjectName("splitter")
	e.splitter = splitter
}

func (e *Editor) setDebuglog() (file *os.File) {
	if e.opts.Debug == "" {
		return nil
	}

	file, err := os.OpenFile(e.opts.Debug, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println(err)

		os.Exit(1)
	}
	log.SetOutput(file)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	return
}

func (e *Editor) initWorkspaces(ctx context.Context, signal *neovimSignal, redrawUpdates chan [][]interface{}, guiUpdates chan []interface{}, nvimCh chan *nvim.Nvim, uiRemoteAttachedCh chan bool, isSetWindowState bool) {
	e.workspaces = []*Workspace{}

	// ws := newWorkspace()
	// ws.registerSignal(signal, &redrawUpdates, &guiUpdates)
	// ws.initUI()
	// ws.updateSize()
	// go ws.bindNvim(nvimCh, uiRCh, isSetWindowState)
	// e.workspaces = append(e.workspaces, ws)
	// ws.widget.SetParent(e.widget)
	editor.putLog("start initializing workspaces")

	// Detect session file
	sessionExists := false
	restoreFiles := []string{}
	for i := 0; i <= WORKSPACELEN; i++ {
		path := filepath.Join(e.configDir, "sessions", strconv.Itoa(i)+".vim")
		_, err := os.Stat(path)
		if err != nil {
			continue
		}
		sessionExists = true
		restoreFiles = append(restoreFiles, path)

	}
	e.doRestoreSessions = sessionExists && e.config.Workspace.RestoreSession
	if len(restoreFiles) == 0 || !e.doRestoreSessions {
		restoreFiles = []string{""}
	}

	editor.putLog("done checking sessions")

	for i, file := range restoreFiles {
		ws := newWorkspace()
		ws.initUI()

		if i == 0 {
			e.font = <-e.fontCh
		}

		ws.initFont()
		e.initAppFont()
		ws.registerSignal(signal, redrawUpdates, guiUpdates)
		ws.updateSize()

		// Only the first nvim instance is lazy-bound to the workspace,
		// but the second and subsequent instances are not lazy-bound
		isLazyBind := true
		if i > 0 {
			isLazyBind = false
			signal, redrawUpdates, guiUpdates, nvimCh, uiRemoteAttachedCh, _ = newNvim(ws.cols, ws.rows, ctx)
		}

		e.workspaces = append(e.workspaces, ws)
		go ws.bindNvim(nvimCh, uiRemoteAttachedCh, isSetWindowState, isLazyBind, file)
	}

	e.putLog("done initialazing workspaces")
}

func (e *Editor) connectAppSignals() {
	if e.app == nil {
		return
	}
	if runtime.GOOS == "darwin" {
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
	e.signal.ConnectSidebarSignal(func() {
		if e.side != nil {
			return
		}
		e.putLog("create workspace sidebar")
		e.side = newWorkspaceSide()
		e.side.newScrollArea()
		e.side.scrollarea.Hide()
		e.side.scrollarea.SetWidget(e.side.widget)
		e.splitter.InsertWidget(0, e.side.scrollarea)
		side := e.side
		if e.config.SideBar.Visible {
			side.show()
		}
	})

	go e.toEmmitGeometrySignal()
	go e.signal.ConnectGeometrySignal(func() {
		e.AdjustSizeBasedOnFontmetrics(e.windowSize[0], e.windowSize[1])
	})

	// When an application is closed with the Close button
	e.window.ConnectCloseEvent(func(event *gui.QCloseEvent) {
		e.putLog("The application was closed outside of Neovim's commands, such as the Close button.")

		// A request to exit the application via the close button has been issued,
		// intercept this request and send quit command to the nvim process.
		event.Ignore()

		if e.config.Workspace.RestoreSession {
			e.cleanup()
			e.saveSessions()
		}

		var cmd string
		if e.config.Editor.IgnoreSaveConfirmationWithCloseButton {
			cmd = "qa!"
		} else {
			cmd = "confirm qa"
		}

		for _, ws := range e.workspaces {
			go ws.nvim.Command(cmd)
		}

		return
	})
	e.putLog("done connecting UI siganal")
}

func (e *Editor) toEmmitGeometrySignal() {
	for {
		time.Sleep(100 * time.Millisecond)
		if e.geometryUpdateTimer == nil {
			continue
		}
		select {
		case <-e.geometryUpdateTimer.C:
			e.signal.GeometrySignal()
		default:
			continue
		}
		// wait
		// e.geometryUpdates <- true
		// editor.signal.GeometrySignal()

	}
}

func (e *Editor) resizeMainWindow() {
	cws := e.workspaces[e.active]
	windowWidth, windowHeight, _, _ := cws.updateSize()
	e.windowSize = [2]int{windowWidth, windowHeight}

	if !editor.config.Editor.WindowGeometryBasedOnFontmetrics {
		return
	}

	if e.geometryUpdateTimer == nil {
		e.geometryUpdateTimer = time.NewTimer(200 * time.Millisecond)
	} else {
		if !e.geometryUpdateTimer.Stop() {
			select {
			case <-e.geometryUpdateTimer.C:
			default:
			}
		}
		e.geometryUpdateTimer.Reset(200 * time.Millisecond)
	}

}

func (e *Editor) AdjustSizeBasedOnFontmetrics(windowWidth, windowHeight int) {
	if e.window.WindowState() == core.Qt__WindowFullScreen || e.isWindowMaximizing {
		return
	}

	// quantization of window geometry with font metrics as the smallest unit of change.
	geometry := editor.window.Geometry()
	width := geometry.Width()
	height := geometry.Height()

	if !(width == windowWidth && height == windowHeight) {
		e.window.Resize2(
			windowWidth,
			windowHeight,
		)
	}
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
		goneovim.Command(fmt.Sprintf("%s %s", e.config.Editor.FileOpenCmd, e.macAppArg))
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

	e.putLog("initializing notification UI")
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
	e.putLog("initialize system tray")
}

func (e *Editor) setEnvironmentVariables() {
	// For Linux
	if runtime.GOOS == "linux" {
		// // It was not a necessary process to export the following environment variables.
		// exe, _ := os.Executable()
		// dir, _ := filepath.Split(exe)
		// _ = os.Setenv("LD_LIBRARY_PATH", dir+"lib")
		// _ = os.Setenv("QT_PLUGIN_PATH", dir+"plugins")
		// _ = os.Setenv("RESOURCE_NAME", "goneovim")
	}

	// If the OS is MacOS and the application is launched from an .app
	if runtime.GOOS == "darwin" && e.ppid == 1 {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = os.Getenv("/bin/bash")
		}
		cmd := exec.Command(shell, "-l", "-c", "env", "-i")
		// cmd := exec.Command("printenv")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			e.putLog(err)
			return
		}
		if err := cmd.Start(); err != nil {
			e.putLog(err)
			return
		}
		output, err := ioutil.ReadAll(stdout)
		if err != nil {
			e.putLog(err)
			stdout.Close()
			return
		}
		for _, b := range strings.Split(string(output), "\n") {
			splits := strings.SplitN(b, "=", 2)
			if len(splits) > 1 {
				_ = os.Setenv(splits[0], splits[1])
			}
		}
	}

	// For Windows
	// https://github.com/equalsraf/neovim-qt/issues/391
	if runtime.GOOS == "windows" {
		_ = os.Setenv("QT_AUTO_SCREEN_SCALE_FACTOR", "1")
	}

	e.putLog("setting environment variable")
}

func (e *Editor) initAppFont() {
	// e.extFontFamily = e.config.Editor.FontFamily
	// e.extFontSize = e.config.Editor.FontSize
	e.app.SetFont(gui.NewQFont2(e.extFontFamily, e.extFontSize, 1, false), "QWidget")
	e.app.SetFont(gui.NewQFont2(e.extFontFamily, e.extFontSize, 1, false), "QLabel")

	e.putLog("initializing font")
}

// pushNotification is
//   level: notify level
//   period: display period
func (e *Editor) pushNotification(level NotifyLevel, p int, message string, opt ...NotifyOptionArg) {
	a := NotifyOptions{}
	for _, o := range opt {
		o(&a)
	}
	n := &Notify{
		level:   level,
		period:  p,
		message: message,
		buttons: a.buttons,
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

	e.putLog("initializing color palette")
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
	e.putLog("start    updating GUI color")
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
	e.putLog("finished updating GUI color")
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
	e.window.SetupBorderSize(e.config.Editor.Margin)
	e.window.SetupWindowGap(e.config.Editor.Gap)

	if e.opts.Geometry != "" {
		width, height := e.setWindowSize(e.opts.Geometry)
		e.config.Editor.Width = width
		e.config.Editor.Height = height
	}
}

func (e *Editor) setWindowSize(s string) (int, int) {
	var width, height int
	var err error
	width, err = strconv.Atoi(strings.SplitN(s, "x", 2)[0])
	if err != nil || width < 40 {
		width = 40
	}
	height, err = strconv.Atoi(strings.SplitN(s, "x", 2)[1])
	if err != nil || height < 30 {
		height = 30
	}

	return width, height
}

func (e *Editor) restoreWindow() {
	if !e.config.Editor.RestoreWindowGeometry {
		return
	}

	settings := core.NewQSettings("neovim", "goneovim", nil)
	geometry := settings.Value("geometry", core.NewQVariant13(core.NewQByteArray()))
	state := settings.Value("windowState", core.NewQVariant13(core.NewQByteArray()))

	geometryBA := geometry.ToByteArray()
	stateBA := state.ToByteArray()

	if geometryBA.Length() != 0 {
		e.window.RestoreGeometry(geometryBA)
		// isRestoreGeometry = true
	}
	if stateBA.Length() != 0 {
		e.window.RestoreState(stateBA, 0)
		// isRestoreState = true
	}

	return
}

func (e *Editor) connectWindowEvents() {
	e.window.ConnectKeyPressEvent(e.keyPress)
	e.window.ConnectKeyReleaseEvent(e.keyRelease)
	e.bindResizeEvent()
	e.window.ConnectShowEvent(func(event *gui.QShowEvent) {
		editor.putLog("show application window")
	})

	e.window.InstallEventFilter(e.window)
	e.window.ConnectEventFilter(func(watched *core.QObject, event *core.QEvent) bool {
		switch event.Type() {
		case core.QEvent__ActivationChange:
			if e.window.IsActiveWindow() {
				e.isWindowNowActivated = true
				e.isWindowNowInactivated = false
			} else if !e.window.IsActiveWindow() {
				e.isWindowNowActivated = false
				e.isWindowNowInactivated = true
			}
		case core.QEvent__WindowStateChange:
			if e.window.WindowState() == core.Qt__WindowMaximized {
				if !e.isWindowMaximizing {
					e.isWindowMaximizing = true

					if editor.config.Editor.WindowGeometryBasedOnFontmetrics {
						go e.window.WindowMaximize()
					}
				}
			} else {
				e.isWindowMaximizing = false
			}
		default:
		}

		return e.window.QFramelessDefaultEventFilter(watched, event)
	})
}

func (e *Editor) setWindowOptions() {
	// e.window.SetupTitle("Neovim")
	e.window.SetMinimumSize2(40, 30)
	e.window.SetAttribute(core.Qt__WA_KeyCompression, false)
	e.window.SetAcceptDrops(true)
}

func (e *Editor) setInitialWindowState() (isSetWindowState bool) {
	e.width = e.config.Editor.Width
	e.height = e.config.Editor.Height
	if e.config.Editor.HideTitlebar {
		e.window.IsTitlebarHidden = true
		e.window.TitleBar.Hide()
	}

	// isRestoreGeometry, isRestoreState := e.restoreWindow()

	// If command line options are given, they take priority.
	if e.config.Editor.StartFullscreen ||
		e.config.Editor.StartMaximizedWindow {
		if e.config.Editor.StartFullscreen {
			e.window.WindowFullScreen()
		} else if e.config.Editor.StartMaximizedWindow {
			e.window.WindowMaximize()
		}
		isSetWindowState = true
	} else {
		if e.config.Editor.RestoreWindowGeometry && e.opts.Geometry == "" {
			e.restoreWindow()
		} else {
			e.window.Resize2(e.width, e.height)
		}
	}

	return
}

func (e *Editor) setWindowLayout() {
	// window layout
	l := widgets.NewQBoxLayout(widgets.QBoxLayout__RightToLeft, nil)
	l.SetContentsMargins(0, 0, 0, 0)
	l.SetSpacing(0)
	e.window.SetupContent(l)

	// window content
	e.widget = widgets.NewQWidget(nil, 0)
	e.newSplitter()
	e.splitter.InsertWidget(1, e.widget)
	l.AddWidget(e.splitter, 1, 0)

	e.putLog("window content layout done")
}

func isFileExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func (e *Editor) workspaceAdd() {
	if len(e.workspaces) == WORKSPACELEN {
		return
	}
	editor.isSetGuiColor = false

	ws := newWorkspace()
	ws.initUI()
	ws.updateSize()
	signal, redrawUpdates, guiUpdates, nvimCh, uiRemoteAttachedCh, _ := newNvim(ws.cols, ws.rows, e.ctx)
	ws.registerSignal(signal, redrawUpdates, guiUpdates)

	e.workspaces = append(e.workspaces, ws)
	e.active = len(e.workspaces) - 1

	e.workspaces[e.active] = ws
	ws.bindNvim(nvimCh, uiRemoteAttachedCh, false, false, "")
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
	if e.side == nil {
		return
	}
	for i, ws := range e.workspaces {
		if i == e.active {
			ws.hide()
			ws.show()
		} else {
			ws.hide()
		}
	}
	for i := 0; i < len(e.side.items) && i < len(e.workspaces); i++ {
		if e.side.items[i] == nil {
			continue
		}
		if e.side.items[i].label.Text() == "" {
			e.workspaces[i].setCwd(e.workspaces[i].cwdlabel)
		}
		e.side.items[i].setSideItemLabel(i)
		// e.side.items[i].setText(e.workspaces[i].cwdlabel)
		e.side.items[i].show()
	}
	for i := len(e.workspaces); i < len(e.side.items); i++ {
		if e.side.items[i] == nil {
			continue
		}
		e.side.items[i].hide()
	}
}

func (e *Editor) keyRelease(event *gui.QKeyEvent) {
	if !e.isKeyAutoRepeating {
		return
	}

	if editor.config.Editor.SmoothScroll {
		ws := e.workspaces[e.active]
		win, ok := ws.screen.getWindow(ws.cursor.gridid)
		if !ok {
			return
		}
		if win.scrollPixels2 != 0 {
			return
		}
		win.grabScreenSnapshot()
	}

	e.isKeyAutoRepeating = false
}

func (e *Editor) keyPress(event *gui.QKeyEvent) {
	if len(e.workspaces) <= e.active {
		return
	}
	ws := e.workspaces[e.active]
	if ws.nvim == nil {
		return
	}
	if !e.isKeyAutoRepeating {
		ws.getSnapshot()
	}
	if !e.isHideMouse && e.config.Editor.HideMouseWhenTyping {
		bc := gui.NewQCursor2(core.Qt__BlankCursor)
		gui.QGuiApplication_SetOverrideCursor(bc)
		gui.QGuiApplication_ChangeOverrideCursor(bc)
		e.isHideMouse = true
	}

	input := e.convertKey(event)

	e.putLog("key input for nvim::", "input:", input)
	if event.IsAutoRepeat() {
		e.isKeyAutoRepeating = true
	}
	if input != "" {
		ws.nvim.Input(input)
	}
}

func keyToText(key int, mod core.Qt__KeyboardModifier) string {
	text := string(rune(key))
	if !(mod&core.Qt__ShiftModifier > 0) {
		text = strings.ToLower(text)
	}

	return text
}

// controlModifier is
func controlModifier() (controlModifier core.Qt__KeyboardModifier) {
	if runtime.GOOS == "windows" {
		controlModifier = core.Qt__ControlModifier
	}
	if runtime.GOOS == "linux" {
		controlModifier = core.Qt__ControlModifier
	}
	if runtime.GOOS == "darwin" {
		controlModifier = core.Qt__MetaModifier
	}

	return
}

// cmdModifier is
func cmdModifier() (cmdModifier core.Qt__KeyboardModifier) {
	if runtime.GOOS == "windows" {
		cmdModifier = core.Qt__NoModifier
	}
	if runtime.GOOS == "linux" {
		cmdModifier = core.Qt__MetaModifier
	}
	if runtime.GOOS == "darwin" {
		cmdModifier = core.Qt__ControlModifier
	}

	return
}

func isAsciiCharRequiringAlt(key int, mod core.Qt__KeyboardModifier, c rune) bool {
	// Ignore all key events where Alt is not pressed
	if !(mod&core.Qt__AltModifier > 0) {
		return false
	}

	// These low-ascii characters may require AltModifier on MacOS
	if (c == '[' && key != int(core.Qt__Key_BracketLeft)) ||
		(c == ']' && key != int(core.Qt__Key_BracketRight)) ||
		(c == '{' && key != int(core.Qt__Key_BraceLeft)) ||
		(c == '}' && key != int(core.Qt__Key_BraceRight)) ||
		(c == '|' && key != int(core.Qt__Key_Bar)) ||
		(c == '~' && key != int(core.Qt__Key_AsciiTilde)) ||
		(c == '@' && key != int(core.Qt__Key_At)) {
		return true
	}

	return false
}

func (e *Editor) convertKey(event *gui.QKeyEvent) string {
	text := event.Text()
	key := event.Key()
	mod := event.Modifiers()

	if e.opts.Debug != "" {
		e.putLog("key input(raw)::", fmt.Sprintf("text: %s, key: %d, mod: %v", text, key, mod))
	}

	// this is macmeta alternatively
	if e.config.Editor.Macmeta {
		if mod&core.Qt__AltModifier > 0 && mod&core.Qt__ShiftModifier > 0 {
			text = string(rune(key))
		} else if mod&core.Qt__AltModifier > 0 && !(mod&core.Qt__ShiftModifier > 0) {
			text = strings.ToLower(string(rune(key)))
		}
	}

	if mod&core.Qt__KeypadModifier > 0 {
		keypadKeys := map[core.Qt__Key]string{
			core.Qt__Key_Home:     "kHome",
			core.Qt__Key_End:      "kEnd",
			core.Qt__Key_PageUp:   "kPageUp",
			core.Qt__Key_PageDown: "kPageDown",
			core.Qt__Key_Plus:     "kPlus",
			core.Qt__Key_Minus:    "kMinus",
			core.Qt__Key_multiply: "kMultiply",
			core.Qt__Key_division: "kDivide",
			core.Qt__Key_Enter:    "kEnter",
			core.Qt__Key_Period:   "kPoint",
			core.Qt__Key_0:        "k0",
			core.Qt__Key_1:        "k1",
			core.Qt__Key_2:        "k2",
			core.Qt__Key_3:        "k3",
			core.Qt__Key_4:        "k4",
			core.Qt__Key_5:        "k5",
			core.Qt__Key_6:        "k6",
			core.Qt__Key_7:        "k7",
			core.Qt__Key_8:        "k8",
			core.Qt__Key_9:        "k9",
		}

		if value, ok := keypadKeys[core.Qt__Key(key)]; ok {
			return fmt.Sprintf("<%s%s>", e.modPrefix(mod), value)
		}
	}

	specialKey, ok := e.specialKeys[core.Qt__Key(key)]
	if ok {
		if key == int(core.Qt__Key_Space) || key == int(core.Qt__Key_Backspace) {
			mod &= ^core.Qt__ShiftModifier
		}
		return fmt.Sprintf("<%s%s>", e.modPrefix(mod), specialKey)
	}

	if text == "<" {
		modNoShift := mod & ^core.Qt__ShiftModifier
		return fmt.Sprintf("<%s%s>", e.modPrefix(modNoShift), "lt")
	}

	isCaretKey := key == int(core.Qt__Key_6) || key == int(core.Qt__Key_AsciiCircum)

	// Normalize modifiers, CTRL+^ always sends as <C-^>
	if isCaretKey && (mod&controlModifier() > 0) {
		modNoShiftMeta := mod & ^core.Qt__ShiftModifier & ^cmdModifier()
		return fmt.Sprintf("<%s%s>", e.modPrefix(modNoShiftMeta), "^")
	}

	if text == "\\" {
		return fmt.Sprintf("<%s%s>", e.modPrefix(mod), "Bslash")
	}

	rtext := []rune(text)
	isGraphic := false
	if len(rtext) > 0 {
		isGraphic = unicode.IsGraphic(rtext[0])
	}
	if text == "" || !isGraphic {
		if key == int(core.Qt__Key_Alt) ||
			key == int(core.Qt__Key_AltGr) ||
			key == int(core.Qt__Key_CapsLock) ||
			key == int(core.Qt__Key_Control) ||
			key == int(core.Qt__Key_Meta) ||
			key == int(core.Qt__Key_Shift) ||
			key == int(core.Qt__Key_Super_L) ||
			key == int(core.Qt__Key_Super_R) {
			return ""
		}

		// Ignore special keys
		if key == int(core.Qt__Key_VolumeDown) ||
			key == int(core.Qt__Key_VolumeMute) ||
			key == int(core.Qt__Key_VolumeUp) {
			return ""
		}

		text = keyToText(key, mod)
	}
	c := text
	if c == "" {
		return ""
	}

	char := []rune(c)[0]
	// char := core.NewQChar11(c)

	// Remove SHIFT
	if (int(char) >= 0x80 || unicode.IsPrint(char)) && !(mod&controlModifier() > 0) && !(mod&cmdModifier() > 0) {
		mod &= ^core.Qt__ShiftModifier
	}
	// if (char.Unicode() >= 0x80 || char.IsPrint()) && !(mod&controlModifier() > 0) && !(mod&cmdModifier() > 0) {
	// 	mod &= ^core.Qt__ShiftModifier
	// }

	// Remove CTRL
	if int(char) < 0x20 {
		text = keyToText(key, mod)
	}
	// if char.Unicode() < 0x20 {
	// 	text = keyToText(key, mod)
	// }

	if runtime.GOOS == "darwin" {
		// Remove ALT/OPTION
		if int(char) >= 0x80 && unicode.IsPrint(char) {
			mod &= ^core.Qt__AltModifier
		}
		// if char.Unicode() >= 0x80 && char.IsPrint() {
		// 	mod &= ^core.Qt__AltModifier
		// }

		// Some locales require Alt for basic low-ascii characters,
		// remove AltModifer. Ex) German layouts use Alt for "{".
		if isAsciiCharRequiringAlt(key, mod, []rune(c)[0]) {
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
		if mod&controlModifier() > 0 && !(mod&core.Qt__AltModifier > 0) {
			prefix += "C-"
		}
		if mod&core.Qt__ShiftModifier > 0 {
			prefix += "S-"
		}
		if mod&core.Qt__AltModifier > 0 && !(mod&controlModifier() > 0) {
			prefix += e.prefixToMapMetaKey
		}
	} else {
		if mod&cmdModifier() > 0 {
			prefix += "D-"
		}
		if mod&controlModifier() > 0 {
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
	e.specialKeys[core.Qt__Key_Delete] = "Del"
	e.specialKeys[core.Qt__Key_Insert] = "Insert"
	e.specialKeys[core.Qt__Key_Home] = "Home"
	e.specialKeys[core.Qt__Key_End] = "End"
	e.specialKeys[core.Qt__Key_PageUp] = "PageUp"
	e.specialKeys[core.Qt__Key_PageDown] = "PageDown"
	e.specialKeys[core.Qt__Key_Return] = "Enter"
	e.specialKeys[core.Qt__Key_Enter] = "Enter"
	e.specialKeys[core.Qt__Key_Tab] = "Tab"
	e.specialKeys[core.Qt__Key_Backtab] = "Tab"
	e.specialKeys[core.Qt__Key_Escape] = "Esc"
	e.specialKeys[core.Qt__Key_Backslash] = "Bslash"
	e.specialKeys[core.Qt__Key_Space] = "Space"

	if runtime.GOOS == "darwin" {
		e.keyControl = core.Qt__Key_Meta
		e.keyCmd = core.Qt__Key_Control
	} else if runtime.GOOS == "windows" {
		e.keyControl = core.Qt__Key_Control
		e.keyCmd = (core.Qt__Key)(0)
	} else {
		e.keyControl = core.Qt__Key_Control
		e.keyCmd = core.Qt__Key_Meta
	}

	e.prefixToMapMetaKey = "A-"
}

func (e *Editor) close(exitcode int) {
	e.stopOnce.Do(func() {
		e.stop <- exitcode
	})
}

func (e *Editor) saveAppWindowState() {
	e.putLog("save application window state")
	settings := core.NewQSettings("neovim", "goneovim", nil)
	settings.SetValue("geometry", core.NewQVariant13(e.window.SaveGeometry()))
	settings.SetValue("windowState", core.NewQVariant13(e.window.SaveState(0)))
}

func (e *Editor) cleanup() {
	// TODO We need to kill the minimap nvim process explicitly?

	if !e.config.Workspace.RestoreSession {
		return
	}
	sessions := filepath.Join(e.configDir, "sessions")
	os.RemoveAll(sessions)
	os.MkdirAll(sessions, 0755)
}

func (e *Editor) saveSessions() {
	if !e.config.Workspace.RestoreSession {
		return
	}

	sessions := filepath.Join(e.configDir, "sessions")

	for i, ws := range e.workspaces {
		if ws.uiRemoteAttached {
			continue
		}
		sessionPath := filepath.Join(sessions, strconv.Itoa(i)+".vim")
		ws.nvim.Command("mksession " + sessionPath)
	}
}
