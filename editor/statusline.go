package editor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/akiyosi/gonvim/osdepend"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// Statusline is
type Statusline struct {
	ws     *Workspace
	widget *widgets.QWidget
	bg     *RGBA

	borderTopWidth int
	paddingLeft    int
	paddingRight   int
	margin         int
	height         int

	pos        *StatuslinePos
	mode       *StatusMode
	main       *StatuslineMain
	notify     *StatuslineNotify
	filetype   *StatuslineFiletype
	git        *StatuslineGit
	encoding   *StatuslineEncoding
	fileFormat *StatuslineFileFormat
	lint       *StatuslineLint
	updates    chan []interface{}
}

// StatuslineNotify
type StatuslineNotify struct {
	s      *Statusline
	widget *widgets.QWidget
	icon   *svg.QSvgWidget
	label  *widgets.QLabel
	num    int
}

// StatuslineLint is
type StatuslineLint struct {
	s          *Statusline
	errors     int
	warnings   int
	widget     *widgets.QWidget
	okIcon     *svg.QSvgWidget
	errorIcon  *svg.QSvgWidget
	warnIcon   *svg.QSvgWidget
	okLabel    *widgets.QLabel
	errorLabel *widgets.QLabel
	warnLabel  *widgets.QLabel
	svgLoaded  bool
}

// StatuslineMain is
type StatuslineMain struct {
	s      *Statusline
	widget *widgets.QWidget

	modeIcon  *svg.QSvgWidget
	modeLabel *widgets.QLabel

	folderLabel *widgets.QLabel
	base        string
	dir         string

	file      string
	fileType  string
	fileLabel *widgets.QLabel
}

// StatuslineFiletype is
type StatuslineFiletype struct {
	filetype string
	label    *widgets.QLabel
}

// StatuslinePos is
type StatuslinePos struct {
	ln    int
	col   int
	label *widgets.QLabel
	text  string
}

// StatusMode is
type StatusMode struct {
	s    *Statusline
	mode string
	text string
	bg   *RGBA
}

// StatuslineGit is
type StatuslineGit struct {
	s         *Statusline
	branch    string
	file      string
	widget    *widgets.QWidget
	label     *widgets.QLabel
	icon      *svg.QSvgWidget
	svgLoaded bool
	hidden    bool
}

// StatuslineEncoding is
type StatuslineEncoding struct {
	encoding string
	label    *widgets.QLabel
}

// StatuslineFileFormat
type StatuslineFileFormat struct {
	fileFormat string
	label      *widgets.QLabel
}

func initStatuslineNew() *Statusline {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 6, 0)

	// spacing, padding, paddingtop, rightitemnum, width
	layout := newVFlowLayout(16, 10, 1, 1, 0)
	widget.SetLayout(layout)
	widget.SetObjectName("statusline")

	s := &Statusline{
		widget:  widget,
		updates: make(chan []interface{}, 1000),
	}

	mode := &StatusMode{
		s: s,
	}
	s.mode = mode

	gitIcon := svg.NewQSvgWidget(nil)
	gitIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
	gitLabel := widgets.NewQLabel(nil, 0)
	gitLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
	gitLabel.SetContentsMargins(0, 0, 0, 0)
	gitLayout := widgets.NewQHBoxLayout()
	gitLayout.SetContentsMargins(0, 0, 0, 0)
	gitLayout.SetSpacing(0)
	gitLayout.AddWidget(gitIcon, 0, 0)
	gitLayout.AddWidget(gitLabel, 0, 0)
	gitWidget := widgets.NewQWidget(nil, 0)
	gitWidget.SetLayout(gitLayout)
	gitWidget.Hide()
	git := &StatuslineGit{
		s:      s,
		widget: gitWidget,
		icon:   gitIcon,
		label:  gitLabel,
	}
	s.git = git

	modeLabel := widgets.NewQLabel(nil, 0)
	modeLabel.SetContentsMargins(4, 1, 4, 1)
	modeLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))
	modeIcon := svg.NewQSvgWidget(nil)
	modeIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
	switch editor.config.Statusline.ModeIndicatorType {
	case "none":
		modeLabel.Hide()
		modeIcon.Hide()
	case "textLabel":
		modeIcon.Hide()
	case "icon":
		modeLabel.Hide()
	case "background":
		modeLabel.Hide()
		modeIcon.Hide()
	default:
		modeLabel.Hide()
		modeIcon.Hide()
	}
	fileLabel := widgets.NewQLabel(nil, 0)
	fileLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
	fileLabel.SetContentsMargins(0, 0, 0, 1)
	folderLabel := widgets.NewQLabel(nil, 0)
	folderLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
	folderLabel.SetContentsMargins(0, 0, 0, 1)
	folderLabel.Hide()
	modeAndFileLayout := widgets.NewQHBoxLayout()
	modeAndFileLayout.SetContentsMargins(0, 0, 0, 1)
	modeAndFileLayout.SetSpacing(6)
	modeAndFileLayout.AddWidget(modeLabel, 0, 0)
	modeAndFileLayout.AddWidget(modeIcon, 0, 0)
	modeAndFileLayout.AddWidget(folderLabel, 0, 0)
	modeAndFileLayout.AddWidget(fileLabel, 0, 0)
	modeAndFileWidget := widgets.NewQWidget(nil, 0)
	modeAndFileWidget.SetLayout(modeAndFileLayout)
	main := &StatuslineMain{
		s:           s,
		modeLabel:   modeLabel,
		modeIcon:    modeIcon,
		widget:      modeAndFileWidget,
		fileLabel:   fileLabel,
		folderLabel: folderLabel,
	}
	s.main = main

	fileFormatLabel := widgets.NewQLabel(nil, 0)
	fileFormatLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
	fileFormat := &StatuslineFileFormat{
		label: fileFormatLabel,
	}
	s.fileFormat = fileFormat
	s.fileFormat.label.Hide()

	encodingLabel := widgets.NewQLabel(nil, 0)
	encodingLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
	encoding := &StatuslineEncoding{
		label: encodingLabel,
	}
	s.encoding = encoding
	s.encoding.label.Hide()

	posLabel := widgets.NewQLabel(nil, 0)
	posLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
	pos := &StatuslinePos{
		label: posLabel,
	}
	s.pos = pos

	filetypeLabel := widgets.NewQLabel(nil, 0)
	filetypeLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
	filetype := &StatuslineFiletype{
		label: filetypeLabel,
	}
	s.filetype = filetype
	s.filetype.label.Hide()

	notifyLayout := widgets.NewQHBoxLayout()
	notifyWidget := widgets.NewQWidget(nil, 0)
	notifyWidget.SetLayout(notifyLayout)
	notifyLabel := widgets.NewQLabel(nil, 0)
	notifyLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
	notifyLabel.Hide()
	notifyicon := svg.NewQSvgWidget(nil)
	notifyicon.SetFixedSize2(editor.iconSize, editor.iconSize)
	notifyLayout.AddWidget(notifyicon, 0, 0)
	notifyLayout.AddWidget(notifyLabel, 0, 0)
	notifyLayout.SetContentsMargins(2, 0, 2, 0)
	notifyLayout.SetSpacing(2)
	notify := &StatuslineNotify{
		widget: notifyWidget,
		label:  notifyLabel,
		icon:   notifyicon,
	}
	s.notify = notify
	notifyWidget.ConnectEnterEvent(func(event *core.QEvent) {
		bg := editor.bgcolor
		if editor.config.Statusline.ModeIndicatorType == "background" {
			switch editor.workspaces[editor.active].mode {
			case "normal":
				notifyWidget.SetStyleSheet(fmt.Sprintf(" * { background: %s; }", darkenHex(editor.config.Statusline.NormalModeColor)))
			case "cmdline_normal":
				notifyWidget.SetStyleSheet(fmt.Sprintf(" * { background: %s; }", darkenHex(editor.config.Statusline.CommandModeColor)))
			case "insert":
				notifyWidget.SetStyleSheet(fmt.Sprintf(" * { background: %s; }", darkenHex(editor.config.Statusline.InsertModeColor)))
			case "visual":
				notifyWidget.SetStyleSheet(fmt.Sprintf(" * { background: %s; }", darkenHex(editor.config.Statusline.VisualModeColor)))
			case "replace":
				notifyWidget.SetStyleSheet(fmt.Sprintf(" * { background: %s; }", darkenHex(editor.config.Statusline.ReplaceModeColor)))
			case "terminal-input":
				notifyWidget.SetStyleSheet(fmt.Sprintf(" * { background: %s; }", darkenHex(editor.config.Statusline.TerminalModeColor)))
			}
		} else {
			notifyWidget.SetStyleSheet(fmt.Sprintf(" * { background: %s; }", warpColor(bg, -10).print()))
		}
	})
	notifyWidget.ConnectLeaveEvent(func(event *core.QEvent) {
		notifyWidget.SetStyleSheet("")
	})
	notifyWidget.ConnectMousePressEvent(func(event *gui.QMouseEvent) {
		switch editor.displayNotifications {
		case true:
			editor.hideNotifications()
		case false:
			editor.showNotifications()
		}
	})
	// notifyWidget.ConnectMouseReleaseEvent(func(*gui.QMouseEvent) {
	// })

	okIcon := svg.NewQSvgWidget(nil)
	okIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
	okLabel := widgets.NewQLabel(nil, 0)
	okLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
	okLabel.SetContentsMargins(0, 0, 0, 0)
	errorIcon := svg.NewQSvgWidget(nil)
	errorIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
	//errorIcon.Show()
	errorLabel := widgets.NewQLabel(nil, 0)
	errorLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
	errorLabel.SetContentsMargins(0, 0, 0, 0)
	//errorLabel.Show()
	warnIcon := svg.NewQSvgWidget(nil)
	warnIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
	//warnIcon.Show()
	warnLabel := widgets.NewQLabel(nil, 0)
	warnLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))
	warnLabel.SetContentsMargins(0, 0, 0, 0)
	//warnLabel.Show()
	lintLayout := widgets.NewQHBoxLayout()
	lintLayout.SetContentsMargins(0, 0, 0, 0)
	lintLayout.SetSpacing(0)
	lintLayout.AddWidget(okIcon, 0, 0)
	//lintLayout.AddWidget(okLabel, 0, 0)
	lintLayout.AddWidget(errorIcon, 0, 0)
	lintLayout.AddWidget(errorLabel, 0, 0)
	lintLayout.AddWidget(warnIcon, 0, 0)
	lintLayout.AddWidget(warnLabel, 0, 0)
	lintWidget := widgets.NewQWidget(nil, 0)
	lintWidget.SetLayout(lintLayout)
	lint := &StatuslineLint{
		s:          s,
		widget:     lintWidget,
		okIcon:     okIcon,
		errorIcon:  errorIcon,
		warnIcon:   warnIcon,
		okLabel:    okLabel,
		errorLabel: errorLabel,
		warnLabel:  warnLabel,
		errors:     -1,
		warnings:   -1,
	}
	s.lint = lint

	s.setContentsMarginsForWidgets(0, 7, 0, 9)

	layout.AddWidget(modeAndFileWidget)
	layout.AddWidget(notifyWidget)
	layout.AddWidget(gitWidget)
	layout.AddWidget(filetypeLabel)
	layout.AddWidget(fileFormatLabel)
	layout.AddWidget(encodingLabel)
	layout.AddWidget(posLabel)
	layout.AddWidget(lintWidget)

	return s
}

func (s *Statusline) setContentsMarginsForWidgets(l int, u int, r int, d int) {
	s.main.widget.SetContentsMargins(l, u, r, d)
	s.pos.label.SetContentsMargins(l, u, r, d)
	s.notify.widget.SetContentsMargins(l, u, r, d)
	s.filetype.label.SetContentsMargins(l, u, r, d)
	s.git.widget.SetContentsMargins(l, u, r, d)
	s.fileFormat.label.SetContentsMargins(l, u, r, d)
	s.encoding.label.SetContentsMargins(l, u, r, d)
	s.lint.widget.SetContentsMargins(l, u, r, d)
}

func (s *Statusline) subscribe() {
	if !s.ws.drawStatusline {
		s.widget.Hide()
		return
	}
	s.ws.signal.ConnectStatuslineSignal(func() {
		updates := <-s.updates
		s.handleUpdates(updates)
	})
	s.ws.signal.ConnectLintSignal(func() {
		s.lint.update()
	})
	s.ws.signal.ConnectGitSignal(func() {
		s.git.update()
	})
	s.ws.nvim.RegisterHandler("statusline", func(updates ...interface{}) {
		s.updates <- updates
		s.ws.signal.StatuslineSignal()
	})
	editor.signal.ConnectNotifySignal(func() {
		s.notify.update()
	})
	s.ws.nvim.Subscribe("statusline")
	s.ws.nvim.Command(`autocmd BufEnter,OptionSet,TermOpen,TermClose * call rpcnotify(0, "statusline", "bufenter", expand("%:p"), &filetype, &fileencoding, &fileformat)`)
}

func (s *Statusline) handleUpdates(updates []interface{}) {
	event := updates[0].(string)
	switch event {
	case "bufenter":
		file := updates[1].(string)
		filetype := updates[2].(string)
		encoding := updates[3].(string)
		fileFormat := updates[4].(string)
		s.main.redraw(file)
		s.filetype.redraw(filetype)
		s.encoding.redraw(encoding)
		s.fileFormat.redraw(fileFormat)
		go s.git.redraw(file)
	default:
		fmt.Println("unhandled statusline event", event)
	}
}

func (s *StatusMode) update() {
	s.s.main.modeLabel.SetText(s.text)
	s.s.main.modeLabel.SetStyleSheet(fmt.Sprintf("color: #ffffff; background-color: %s;", s.bg.String()))
}

func (s *StatusMode) updateStatusline() {
	s.s.main.folderLabel.SetStyleSheet(fmt.Sprintf("color: %s;", newRGBA(230, 230, 230, 1)))
	s.s.widget.SetStyleSheet(fmt.Sprintf("background-color: %s;", s.bg.String()))
	s.s.widget.SetStyleSheet(fmt.Sprintf("QWidget#statusline { border-top: 0px solid %s; background-color: %s; } * { color: %s; }", s.bg.String(), s.bg.String(), "#ffffff"))
	svgContent := editor.getSvg("git", newRGBA(255, 255, 255, 1))
	s.s.git.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	svgContent = editor.getSvg("bell", newRGBA(255, 255, 255, 1))
	s.s.notify.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))

	var svgErrContent, svgWrnContent string
	if s.s.lint.errors != 0 {
		svgErrContent = editor.getSvg("bad", newRGBA(204, 62, 68, 1))
	} else {
		svgErrContent = editor.getSvg("bad", newRGBA(255, 255, 255, 1))
	}
	if s.s.lint.warnings != 0 {
		svgWrnContent = editor.getSvg("exclamation", newRGBA(203, 203, 65, 1))
	} else {
		svgWrnContent = editor.getSvg("exclamation", newRGBA(255, 255, 255, 1))
	}
	s.s.lint.errorIcon.Load2(core.NewQByteArray2(svgErrContent, len(svgErrContent)))
	s.s.lint.warnIcon.Load2(core.NewQByteArray2(svgWrnContent, len(svgWrnContent)))
}

func (s *StatusMode) redraw() {
	if s.s.ws.mode == s.mode {
		return
	}

	fg := s.s.ws.foreground

	s.mode = s.s.ws.mode
	text := s.mode
	bg := newRGBA(102, 153, 204, 1)
	switch s.mode {
	case "normal":
		text = "Normal"
		bg = hexToRGBA(editor.config.Statusline.NormalModeColor)
		svgContent := editor.getSvg("hjkl", newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
		s.s.main.modeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		s.s.main.modeLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))
	case "cmdline_normal":
		text = "Normal"
		bg = hexToRGBA(editor.config.Statusline.CommandModeColor)
		svgContent := editor.getSvg("command", newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
		s.s.main.modeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		s.s.main.modeLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))
	case "insert":
		text = "Insert"
		bg = hexToRGBA(editor.config.Statusline.InsertModeColor)
		svgContent := editor.getSvg("edit", newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
		s.s.main.modeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		s.s.main.modeLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))
	case "visual":
		text = "Visual"
		bg = hexToRGBA(editor.config.Statusline.VisualModeColor)
		svgContent := editor.getSvg("select", newRGBA(warpColor(fg, 30).R, warpColor(fg, 30).G, warpColor(fg, 30).B, 1))
		s.s.main.modeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		s.s.main.modeLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))
	case "replace":
		text = "Replace"
		bg = hexToRGBA(editor.config.Statusline.ReplaceModeColor)
		svgContent := editor.getSvg("replace", newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
		s.s.main.modeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		s.s.main.modeLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-3, 1, false))
	case "terminal-input":
		text = "Terminal"
		bg = hexToRGBA(editor.config.Statusline.TerminalModeColor)
		svgContent := editor.getSvg("terminal", newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
		s.s.main.modeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		s.s.main.modeLabel.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-4, 1, false))
	default:
	}

	s.text = text
	s.bg = bg

	switch editor.config.Statusline.ModeIndicatorType {
	case "none":
		s.s.main.modeLabel.Hide()
		s.s.main.modeIcon.Hide()
	case "textLabel":
		s.s.main.modeIcon.Hide()
		s.update()
	case "icon":
		s.s.main.modeLabel.Hide()
	case "background":
		s.s.main.modeLabel.Hide()
		s.s.main.modeIcon.Hide()
		s.updateStatusline()
	default:
		s.s.main.modeLabel.Hide()
		s.s.main.modeIcon.Hide()
	}
}

func (s *StatuslineGit) hide() {
	if s.hidden {
		return
	}
	s.hidden = true
	s.s.ws.signal.GitSignal()
}

func (s *StatuslineGit) update() {
	if s.hidden {
		s.widget.Hide()
		return
	}
	//fg := s.s.ws.screen.highlight.foreground
	s.label.SetText(s.branch)
	//if !s.svgLoaded {
	//	s.svgLoaded = true
	//	svgContent := editor.getSvg("git", newRGBA(shiftColor(fg, -12).R, shiftColor(fg, -12).G, shiftColor(fg, -12).B, 1))
	//	s.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	//}
	s.widget.Show()
}

func (s *StatuslineGit) redraw(file string) {
	if file == "" || strings.HasPrefix(file, "term://") {
		s.file = file
		s.hide()
		s.branch = ""
		return
	}

	if s.file == file {
		return
	}

	s.file = file
	dir := filepath.Dir(file)
	cmd := exec.Command("git", "-C", dir, "branch")
	osdepend.PrepareRunProc(cmd)
	out, err := cmd.Output()
	if err != nil {
		s.hide()
		s.branch = ""
		return
	}

	branch := ""
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "* ") {
			if strings.HasPrefix(line, "* (HEAD detached at ") {
				branch = line[20 : len(line)-1]
			} else {
				branch = line[2:]
			}
		}
	}
	cmd = exec.Command("git", "-C", dir, "diff", "--quiet")
	osdepend.PrepareRunProc(cmd)
	_, err = cmd.Output()
	if err != nil {
		branch += "*"
	}

	if s.branch != branch {
		s.branch = branch
		s.hidden = false
		s.s.ws.signal.GitSignal()
	}
}

func (s *StatuslineMain) redraw(file string) {
	if file == "" {
		file = "[No Name]"
	}

	if file == s.file {
		return
	}

	s.file = file

	base := filepath.Base(file)
	dir := filepath.Dir(file)
	if dir == "." {
		dir = ""
	}
	if strings.HasPrefix(file, "term://") {
		base = file
		dir = ""
	}
	if dir != "" {
		s.folderLabel.Show()
	} else {
		s.folderLabel.Hide()
	}
	if s.base != base {
		s.base = base
		s.fileLabel.SetText(s.base)
	}
	if s.dir != dir {
		s.dir = dir
		s.folderLabel.SetText(s.dir)
	}
}

func (s *StatuslinePos) redraw(ln, col int) {
	if ln == s.ln && col == s.col {
		return
	}
	text := fmt.Sprintf("%d,%d", ln, col)
	s.ln = ln
	s.col = col
	if text != s.text {
		s.text = text
		s.label.SetText(text)
	}
}

func (s *StatuslineEncoding) redraw(encoding string) {
	if s.encoding == encoding {
		return
	}
	s.encoding = encoding
	s.label.SetText(s.encoding)
	s.label.Show()
}

func (s *StatuslineFileFormat) redraw(fileFormat string) {
	if fileFormat == "" {
		return
	}
	if s.fileFormat == fileFormat {
		return
	}
	s.fileFormat = fileFormat
	s.label.SetText(s.fileFormat)
	s.label.Show()
}

func (s *StatuslineFiletype) redraw(filetype string) {
	if filetype == s.filetype {
		return
	}
	s.filetype = filetype
	typetext := strings.Title(s.filetype)
	if typetext == "Cpp" {
		typetext = "C++"
	}
	s.label.SetText(typetext)
	s.label.Show()
}

func (s *StatuslineNotify) update() {
	s.num = len(editor.notifications)
	if s.num == 0 {
		s.label.Hide()
		return
	} else {
		s.label.Show()
	}
	s.label.SetText(fmt.Sprintf("%v", s.num))
}

func (s *StatuslineLint) update() {
	s.errorLabel.SetText(strconv.Itoa(s.errors))
	s.warnLabel.SetText(strconv.Itoa(s.warnings))
	if !s.svgLoaded {
		s.svgLoaded = true
		//svgContent := editor.getSvg("check", newRGBA(141, 193, 73, 1))
		//s.okIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		var svgErrContent, svgWrnContent string

		var lintNoErrColor *RGBA
		switch editor.fgcolor {
		case nil:
			return
		default:
			if editor.config.Statusline.ModeIndicatorType == "background" {
				lintNoErrColor = newRGBA(255, 255, 255, 1)
			} else {
				lintNoErrColor = editor.fgcolor
			}
		}

		if s.errors != 0 {
			svgErrContent = editor.getSvg("bad", newRGBA(204, 62, 68, 1))
		} else {
			svgErrContent = editor.getSvg("bad", lintNoErrColor)
		}
		if s.warnings != 0 {
			svgWrnContent = editor.getSvg("exclamation", newRGBA(203, 203, 65, 1))
		} else {
			svgWrnContent = editor.getSvg("exclamation", lintNoErrColor)
		}
		s.errorIcon.Load2(core.NewQByteArray2(svgErrContent, len(svgErrContent)))
		s.warnIcon.Load2(core.NewQByteArray2(svgWrnContent, len(svgWrnContent)))
	}

	//if s.errors == 0 && s.warnings == 0 {
	//	s.okIcon.Show()
	//	//s.okLabel.SetText("ok")
	//	s.okLabel.Show()
	//	s.errorIcon.Hide()
	//	s.errorLabel.Hide()
	//	s.warnIcon.Hide()
	//	s.warnLabel.Hide()
	//} else {
	s.okIcon.Hide()
	s.okLabel.Hide()
	s.errorIcon.Show()
	s.errorLabel.Show()
	s.warnIcon.Show()
	s.warnLabel.Show()
	//}
}

func (s *StatuslineLint) redraw(errors, warnings int) {
	if errors == s.errors && warnings == s.warnings {
		return
	}
	var svgErrContent, svgWrnContent string

	var lintNoErrColor *RGBA
	switch editor.fgcolor {
	case nil:
		return
	default:
		if editor.config.Statusline.ModeIndicatorType == "background" {
			lintNoErrColor = newRGBA(255, 255, 255, 1)
		} else {
			lintNoErrColor = editor.fgcolor
		}
	}

	if errors != 0 {
		svgErrContent = editor.getSvg("bad", newRGBA(204, 62, 68, 1))
	} else {
		svgErrContent = editor.getSvg("bad", lintNoErrColor)
	}
	if warnings != 0 {
		svgWrnContent = editor.getSvg("exclamation", newRGBA(203, 203, 65, 1))
	} else {
		svgWrnContent = editor.getSvg("exclamation", lintNoErrColor)
	}
	s.errorIcon.Load2(core.NewQByteArray2(svgErrContent, len(svgErrContent)))
	s.warnIcon.Load2(core.NewQByteArray2(svgWrnContent, len(svgWrnContent)))
	s.errors = errors
	s.warnings = warnings
	s.s.ws.signal.LintSignal()
}
