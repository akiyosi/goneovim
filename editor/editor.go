package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	clipb "github.com/atotto/clipboard"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

var editor *Editor

// Highlight is
type Highlight struct {
	foreground *RGBA
	background *RGBA
	bold       bool
	italic     bool
}

// Char is
type Char struct {
	normalWidth bool
	char        string
	highlight   Highlight
}

// NotifyButton is
type NotifyButton struct {
	action func()
	text   string
}

// ColorPalette is
type ColorPalette struct {
	fg                    *RGBA
	bg                    *RGBA
	inactiveFg            *RGBA
	comment               *RGBA
	abyss                 *RGBA
	matchFg               *RGBA
	selectedBg            *RGBA
	activityBarFg         *RGBA
	activityBarBg         *RGBA
	sideBarFg             *RGBA
	sideBarBg             *RGBA
	sideBarSelectedItemBg *RGBA
	scrollBarFg           *RGBA
	scrollBarBg           *RGBA
	widgetFg              *RGBA
	widgetBg              *RGBA
	widgetInputArea       *RGBA
	minimapCurrentRegion  *RGBA
}

// Notify is
type Notify struct {
	level   NotifyLevel
	period  int
	message string
	buttons []*NotifyButton
}

// Editor is the editor
type Editor struct {
	signal  *editorSignal
	version string
	app     *widgets.QApplication

	activity          *Activity
	splitter          *widgets.QSplitter
	notifyStartPos    *core.QPoint
	notificationWidth int
	notify            chan *Notify
	guiInit           chan bool
	doneGuiInit       bool

	workspaces []*Workspace
	active     int
	nvim       *nvim.Nvim
	window     *widgets.QMainWindow
	wsWidget   *widgets.QWidget
	wsSide     *WorkspaceSide
	deinSide   *DeinSide

	statuslineHeight int
	width            int
	height           int
	iconSize         int
	tablineHeight    int

	stop     chan struct{}
	stopOnce sync.Once

	specialKeys     map[core.Qt__Key]string
	controlModifier core.Qt__KeyboardModifier
	cmdModifier     core.Qt__KeyboardModifier
	shiftModifier   core.Qt__KeyboardModifier
	altModifier     core.Qt__KeyboardModifier
	metaModifier    core.Qt__KeyboardModifier
	keyControl      core.Qt__Key
	keyCmd          core.Qt__Key
	keyAlt          core.Qt__Key
	keyShift        core.Qt__Key

	config                 gonvimConfig
	notifications          []*Notification
	isDisplayNotifications bool

	isSetGuiColor bool
	colors        *ColorPalette
	svgs          map[string]*SvgXML
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
	runtime.GOMAXPROCS(16)

	home, err := homedir.Dir()
	if err != nil {
		home = "~"
	}
	editor = &Editor{
		version: "v0.3.3",
		signal:  NewEditorSignal(nil),
		notify:  make(chan *Notify, 10),
		stop:    make(chan struct{}),
		guiInit: make(chan bool, 1),
		config:  newGonvimConfig(home),
	}
	e := editor
	e.app = widgets.NewQApplication(0, nil)
	e.app.ConnectAboutToQuit(func() {
		editor.cleanup()
	})
	e.app.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false), "QWidget")
	e.app.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false), "QLabel")

	e.initSVGS()
	font := gui.NewQFontMetricsF(gui.NewQFont2(editor.config.Editor.FontFamily, int(editor.config.Editor.FontSize*23/25), 1, false))
	e.iconSize = int(font.Height())
	e.colors = initColorPalette()
	e.colors.update()
	e.initNotifications()

	//create a window
	e.window = widgets.NewQMainWindow(nil, 0)
	e.setWindowOptions()

	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)

	layout := widgets.NewQBoxLayout(widgets.QBoxLayout__RightToLeft, widget)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(0)

	e.wsWidget = widgets.NewQWidget(nil, 0)

	e.wsSide = newWorkspaceSide()
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
	sideArea.SetWidget(e.wsSide.widget)
	sideArea.SetFrameShape(widgets.QFrame__NoFrame)
	e.wsSide.scrollarea = sideArea

	activityWidget := widgets.NewQWidget(nil, 0)
	activityWidget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")
	activity := newActivity()
	activity.widget = activityWidget
	activityWidget.SetLayout(activity.layout)
	e.activity = activity
	e.activity.sideArea.AddWidget(e.wsSide.scrollarea)
	e.activity.sideArea.SetCurrentWidget(e.wsSide.scrollarea)

	go e.dropShadow()

	if e.config.ActivityBar.Visible == false {
		e.activity.widget.Hide()
	}
	if e.config.SideBar.Visible == false {
		e.activity.sideArea.Hide()
	}

	splitter := widgets.NewQSplitter2(core.Qt__Horizontal, nil)
	splitter.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")
	splitter.AddWidget(e.activity.sideArea)
	splitter.AddWidget(e.wsWidget)
	splitter.SetSizes([]int{editor.config.SideBar.Width, editor.width - editor.config.SideBar.Width})
	splitter.SetStretchFactor(1, 100)
	splitter.SetObjectName("splitter")
	e.splitter = splitter

	go func() {
		layout.AddWidget(splitter, 1, 0)
		layout.AddWidget(e.activity.widget, 0, 0)
	}()

	e.workspaces = []*Workspace{}
	sessionExists := false
	if err == nil {
		if e.config.Workspace.RestoreSession == true {
			for i := 0; i < 20; i++ {
				path := filepath.Join(home, ".gonvim", "sessions", strconv.Itoa(i)+".vim")
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

	e.wsWidget.ConnectResizeEvent(func(event *gui.QResizeEvent) {
		for _, ws := range e.workspaces {
			ws.updateSize()
		}
	})
	// for macos, open file via Finder
	macosArg := ""
	if runtime.GOOS == "darwin" {
		e.app.ConnectEvent(func(event *core.QEvent) bool {
			switch event.Type() {
			case core.QEvent__FileOpen:
				fileOpenEvent := gui.NewQFileOpenEventFromPointer(event.Pointer())
				macosArg = fileOpenEvent.File()
				gonvim := e.workspaces[e.active].nvim
				isModified := ""
				isModified, _ = gonvim.CommandOutput("echo &modified")
				if isModified == "1" {
					gonvim.Command(fmt.Sprintf(":tabe %s", macosArg))
				} else {
					gonvim.Command(fmt.Sprintf(":e %s", macosArg))
				}
			}
			return true
		})
		e.window.ConnectCloseEvent(func(event *gui.QCloseEvent) {
			e.app.DisconnectEvent()
			event.Accept()
		})
	}

	e.window.SetCentralWidget(widget)

	go func() {
		<-editor.stop
		if runtime.GOOS == "darwin" {
			e.app.DisconnectEvent()
		}
		e.app.Quit()
	}()

	e.window.Show()
	e.wsWidget.SetFocus2()
	widgets.QApplication_Exec()
}

func (e *Editor) initNotifications() {
	e.notifications = []*Notification{}
	e.notificationWidth = editor.config.Editor.Width * 2 / 3
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

func (e *Editor) dropShadow() {
	// Drop shadow to Side Bar
	if e.config.SideBar.DropShadow == true {
		shadow := widgets.NewQGraphicsDropShadowEffect(nil)
		shadow.SetBlurRadius(60)
		shadow.SetColor(gui.NewQColor3(0, 0, 0, 35))
		shadow.SetOffset3(6, 2)
		e.activity.sideArea.SetGraphicsEffect(shadow)
	}

	// Drop shadow for Activity Bar
	if e.config.ActivityBar.DropShadow == true {
		shadow := widgets.NewQGraphicsDropShadowEffect(nil)
		shadow.SetBlurRadius(60)
		shadow.SetColor(gui.NewQColor3(0, 0, 0, 35))
		shadow.SetOffset3(6, 2)
		e.activity.widget.SetGraphicsEffect(shadow)
	}
}

func initColorPalette() *ColorPalette {
	rgbAccent := hexToRGBA(editor.config.SideBar.AccentColor)
	fg := newRGBA(180, 185, 190, 1)
	bg := newRGBA(9, 13, 17, 1)
	return &ColorPalette{
		bg:         bg,
		fg:         fg,
		selectedBg: bg.brend(rgbAccent, 0.3),
		matchFg:    rgbAccent,
	}
}

func (c *ColorPalette) update() {
	fg := c.fg
	bg := c.bg
	rgbAccent := hexToRGBA(editor.config.SideBar.AccentColor)
	c.selectedBg = bg.brend(rgbAccent, 0.3)
	c.inactiveFg = warpColor(bg, -40)
	c.comment = warpColor(fg, -40)
	c.abyss = warpColor(bg, 5)
	c.activityBarFg = fg
	c.activityBarBg = warpColor(bg, -5)
	c.sideBarFg = warpColor(fg, -3)
	c.sideBarBg = warpColor(bg, -3)
	c.sideBarSelectedItemBg = warpColor(bg, -7)
	c.scrollBarFg = warpColor(bg, -10)
	c.scrollBarBg = bg
	c.widgetFg = warpColor(fg, 3)
	c.widgetBg = warpColor(bg, -4)
	c.widgetInputArea = warpColor(bg, -14)
	c.minimapCurrentRegion = warpColor(bg, 10)
}

func (e *Editor) updateGUIColor() {
	// if activity & sidebar is enabled
	if e.activity != nil && e.wsSide != nil {
		// for splitter
		e.splitter.SetStyleSheet(fmt.Sprintf(" QSplitter::handle:horizontal { background-color: %s; }", e.colors.sideBarBg.StringTransparent()))

		// for Activity Bar
		e.activity.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: %s; } ", e.colors.activityBarBg.StringTransparent()))

		var svgEditContent string
		if e.activity.editItem.active == true {
			svgEditContent = e.getSvg("activityedit", e.colors.fg)
		} else {
			svgEditContent = e.getSvg("activityedit", e.colors.inactiveFg)
		}
		e.activity.editItem.icon.Load2(core.NewQByteArray2(svgEditContent, len(svgEditContent)))

		var svgDeinContent string
		if e.activity.deinItem.active == true {
			svgDeinContent = e.getSvg("activitydein", e.colors.fg)

		} else {
			svgDeinContent = e.getSvg("activitydein", e.colors.inactiveFg)

		}
		e.activity.deinItem.icon.Load2(core.NewQByteArray2(svgDeinContent, len(svgDeinContent)))
		e.wsSide.setColor()
	}

	e.workspaces[e.active].updateWorkspaceColor()

	e.window.SetWindowOpacity(1.0)
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

func darkenHex(hex string) string {
	c := hexToRGBA(hex)
	d := shiftColor(c, 20)
	return fmt.Sprintf("#%02x%02x%02x", (int)(d.R*255.0), (int)(d.G*255.0), (int)(d.B*255.0))
}

func shiftHex(hex string, v int) string {
	c := hexToRGBA(hex)
	d := shiftColor(c, v)
	return fmt.Sprintf("#%02x%02x%02x", (int)(d.R*255.0), (int)(d.G*255.0), (int)(d.B*255.0))
}

func (e *Editor) setWindowOptions() {
	e.window.SetWindowTitle("Gonvim")
	e.width = e.config.Editor.Width
	e.height = e.config.Editor.Height
	e.window.SetMinimumSize2(e.width, e.height)
	e.window.SetContentsMargins(0, 0, 0, 0)
	e.window.SetAttribute(core.Qt__WA_TranslucentBackground, true)
	e.window.SetStyleSheet(" * {background-color: rgba(0, 0, 0, 0);}")
	e.window.SetWindowFlag(core.Qt__FramelessWindowHint, true)
	e.window.SetWindowOpacity(0.0)
	e.initSpecialKeys()
	e.window.ConnectKeyPressEvent(e.keyPress)
	e.window.SetAcceptDrops(true)
}

func isFileExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func (e *Editor) copyClipBoard() {
	go func() {
		var yankedText string
		yankedText, _ = e.workspaces[e.active].nvim.CommandOutput("echo getreg()")
		if yankedText != "" {
			clipb.WriteAll(yankedText)
		}
	}()

}

func (e *Editor) workspaceNew() {
	editor.isSetGuiColor = false
	ws, err := newWorkspace("")
	if err != nil {
		return
	}

	//e.active++
	//e.workspaces = append(e.workspaces, nil)
	//copy(e.workspaces[e.active+1:], e.workspaces[e.active:])

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
			ws.hide()
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
	input := e.convertKey(event.Text(), event.Key(), event.Modifiers())
	if input == "<C-¥>" {
		input = `<C-\>`
	}
	if input != "" {
		if input == "<Esc>" {
			e.unfocusGonvimUI()
		}
		e.workspaces[e.active].nvim.Input(input)
		e.workspaces[e.active].detectTerminalMode()
	}
}

func (e *Editor) unfocusGonvimUI() {
	if e.activity == nil || e.wsSide == nil {
		return
	}
	if e.activity.deinItem.active {
		e.deinSide.searchbox.editBox.ClearFocus()
		e.deinSide.widget.ClearFocus()
		e.deinSide.scrollarea.ClearFocus()
	}
	if e.activity.editItem.active {
		e.wsSide.widget.ClearFocus()
		e.wsSide.widget.ClearFocus()
		e.wsSide.scrollarea.ClearFocus()
	}
}

func (e *Editor) convertKey(text string, key int, mod core.Qt__KeyboardModifier) string {
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
		return "<lt>"
	}

	specialKey, ok := e.specialKeys[core.Qt__Key(key)]
	if ok {
		return fmt.Sprintf("<%s%s>", e.modPrefix(mod), specialKey)
	}

	if text == "\\" || text == "¥" {
		return fmt.Sprintf("<%s%s>", e.modPrefix(mod), "Bslash")
	}

	c := ""
	if mod&e.controlModifier > 0 || mod&e.cmdModifier > 0 {
		if int(e.keyControl) == key || int(e.keyCmd) == key || int(e.keyAlt) == key || int(e.keyShift) == key {
			return ""
		}
		c = string(key)
		if !(mod&e.shiftModifier > 0) {
			c = strings.ToLower(c)
		}
	} else {
		c = text
	}

	if c == "" {
		return ""
	}

	char := core.NewQChar11(c)
	if char.Unicode() < 0x100 && !char.IsNumber() && char.IsPrint() {
		mod &= ^e.shiftModifier
	}

	prefix := e.modPrefix(mod)
	if prefix != "" {
		return fmt.Sprintf("<%s%s>", prefix, c)
	}

	return c
}

func (e *Editor) modPrefix(mod core.Qt__KeyboardModifier) string {
	prefix := ""
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		if mod&e.cmdModifier > 0 {
			prefix += "D-"
		}
	}

	if mod&e.controlModifier > 0 {
		prefix += "C-"
	}

	if mod&e.shiftModifier > 0 {
		prefix += "S-"
	}

	if mod&e.altModifier > 0 {
		prefix += "A-"
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

	goos := runtime.GOOS
	e.shiftModifier = core.Qt__ShiftModifier
	e.altModifier = core.Qt__AltModifier
	e.keyAlt = core.Qt__Key_Alt
	e.keyShift = core.Qt__Key_Shift
	if goos == "darwin" {
		e.controlModifier = core.Qt__MetaModifier
		e.cmdModifier = core.Qt__ControlModifier
		e.metaModifier = core.Qt__AltModifier
		e.keyControl = core.Qt__Key_Meta
		e.keyCmd = core.Qt__Key_Control
	} else {
		e.controlModifier = core.Qt__ControlModifier
		e.metaModifier = core.Qt__MetaModifier
		e.keyControl = core.Qt__Key_Control
		if goos == "linux" {
			e.cmdModifier = core.Qt__MetaModifier
			e.keyCmd = core.Qt__Key_Meta
		}
	}
}

func (e *Editor) close() {
	e.stopOnce.Do(func() {
		close(e.stop)
	})
}

func (e *Editor) cleanup() {
	home, err := homedir.Dir()
	if err != nil {
		return
	}
	sessions := filepath.Join(home, ".gonvim", "sessions")
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
