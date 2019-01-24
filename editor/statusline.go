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

	left       *LeftStatusItem

	pos        *StatuslinePos
	mode       *StatusMode
	file       *StatuslineFile
	notify     *StatuslineNotify
	filetype   *StatuslineFiletype
	git        *StatuslineGit
	encoding   *StatuslineEncoding
	fileFormat *StatuslineFileFormat
	lint       *StatuslineLint
	updates    chan []interface{}
}

type LeftStatusItem struct {
	s      *Statusline
	widget *widgets.QWidget
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

// StatuslineFile is
type StatuslineFile struct {
	s      *Statusline

	folderLabel *widgets.QLabel
	base        string
	dir         string

	file      string
	fileType  string
	fileLabel *widgets.QLabel

	ro     bool
	roIcon *svg.QSvgWidget
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

	icon  *svg.QSvgWidget
	label *widgets.QLabel
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
	gitIcon.SetFixedSize2(editor.iconSize+1, editor.iconSize+1)
	gitLabel := widgets.NewQLabel(nil, 0)
	gitLabel.SetContentsMargins(0, 0, 0, 0)
	gitLayout := widgets.NewQHBoxLayout()
	gitLayout.SetContentsMargins(0, 0, 0, 0)
	gitLayout.SetSpacing(editor.iconSize / 3)
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
	s.mode.label = modeLabel
	s.mode.icon = modeIcon

	fileLabel := widgets.NewQLabel(nil, 0)
	fileLabel.SetContentsMargins(0, 0, 0, 1)
	folderLabel := widgets.NewQLabel(nil, 0)
	folderLabel.SetContentsMargins(0, 0, 0, 1)
	folderLabel.Hide()

	roIcon := svg.NewQSvgWidget(nil)
	roIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
	leftLayout := widgets.NewQHBoxLayout()
	leftLayout.SetContentsMargins(0, 0, 0, 1)
	leftLayout.SetSpacing(8)
	leftWidget := widgets.NewQWidget(nil, 0)
	left := &LeftStatusItem{
		widget:      leftWidget,
	}
	s.left = left
	left.s = s
	file := &StatuslineFile{
		s:           s,
		fileLabel:   fileLabel,
		folderLabel: folderLabel,
		roIcon:      roIcon,
	}
	s.file = file

	fileFormatLabel := widgets.NewQLabel(nil, 0)
	fileFormat := &StatuslineFileFormat{
		label: fileFormatLabel,
	}
	s.fileFormat = fileFormat
	s.fileFormat.label.Hide()

	encodingLabel := widgets.NewQLabel(nil, 0)
	encoding := &StatuslineEncoding{
		label: encodingLabel,
	}
	s.encoding = encoding
	s.encoding.label.Hide()

	posLabel := widgets.NewQLabel(nil, 0)
	pos := &StatuslinePos{
		label: posLabel,
	}
	s.pos = pos

	filetypeLabel := widgets.NewQLabel(nil, 0)
	filetype := &StatuslineFiletype{
		label: filetypeLabel,
	}
	s.filetype = filetype
	s.filetype.label.Hide()

	notifyLayout := widgets.NewQHBoxLayout()
	notifyWidget := widgets.NewQWidget(nil, 0)
	notifyWidget.SetLayout(notifyLayout)
	notifyLabel := widgets.NewQLabel(nil, 0)
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
			notifyWidget.SetStyleSheet(fmt.Sprintf(" * { background: %s; }", editor.colors.widgetBg.String()))
		}
	})
	notifyWidget.ConnectLeaveEvent(func(event *core.QEvent) {
		notifyWidget.SetStyleSheet("")
	})
	notifyWidget.ConnectMousePressEvent(func(event *gui.QMouseEvent) {
		switch editor.isDisplayNotifications {
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
	okLabel.SetContentsMargins(0, 0, 0, 0)
	errorIcon := svg.NewQSvgWidget(nil)
	errorIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
	//errorIcon.Show()
	errorLabel := widgets.NewQLabel(nil, 0)
	errorLabel.SetContentsMargins(0, 0, 0, 0)
	//errorLabel.Show()
	warnIcon := svg.NewQSvgWidget(nil)
	warnIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
	//warnIcon.Show()
	warnLabel := widgets.NewQLabel(nil, 0)
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
	left.widget.SetLayout(leftLayout)
	layout.AddWidget(leftWidget)

	left.setWidget()
	s.setWidget()

	return s
}


func (s *Statusline) setWidget() {
	for _, rightItem := range editor.config.Statusline.Right {
		switch rightItem {
		case "mode" :
			s.widget.Layout().AddWidget(s.mode.label)
			s.widget.Layout().AddWidget(s.mode.icon)
		case "filepath" :
			s.widget.Layout().AddWidget(s.file.folderLabel)
		case "filename" :
			s.widget.Layout().AddWidget(s.file.fileLabel)
			s.widget.Layout().AddWidget(s.file.roIcon)
		case "message" :
			s.widget.Layout().AddWidget(s.notify.widget)
		case "git" :
			s.widget.Layout().AddWidget(s.git.widget)
		case "filetype" :
			s.widget.Layout().AddWidget(s.filetype.label)
		case "fileformat" :
			s.widget.Layout().AddWidget(s.fileFormat.label)
		case "fileencoding" :
			s.widget.Layout().AddWidget(s.encoding.label)
		case "curpos" :
			s.widget.Layout().AddWidget(s.pos.label)
		case "lint" :
			s.widget.Layout().AddWidget(s.lint.widget)
		default:
		}
	}
}

func (left *LeftStatusItem) setWidget() {
	for _, leftItem := range editor.config.Statusline.Left {
		switch leftItem {
		case "mode" :
			left.widget.Layout().AddWidget(left.s.mode.label)
			left.widget.Layout().AddWidget(left.s.mode.icon)
		case "filepath" :
			left.widget.Layout().AddWidget(left.s.file.folderLabel)
		case "filename" :
			left.widget.Layout().AddWidget(left.s.file.fileLabel)
			left.widget.Layout().AddWidget(left.s.file.roIcon)
		case "message" :
			left.widget.Layout().AddWidget(left.s.notify.widget)
		case "git" :
			left.widget.Layout().AddWidget(left.s.git.widget)
		case "filetype" :
			left.widget.Layout().AddWidget(left.s.filetype.label)
		case "fileformat" :
			left.widget.Layout().AddWidget(left.s.fileFormat.label)
		case "fileencoding" :
			left.widget.Layout().AddWidget(left.s.encoding.label)
		case "curpos" :
			left.widget.Layout().AddWidget(left.s.pos.label)
		case "lint" :
			left.widget.Layout().AddWidget(left.s.lint.widget)
		default:
		}
	}
}

func (s *Statusline) setContentsMarginsForWidgets(l int, u int, r int, d int) {
	s.left.widget.SetContentsMargins(l, u, r, d)
	s.pos.label.SetContentsMargins(l, u, r, d)
	s.notify.widget.SetContentsMargins(l, u, r, d)
	s.filetype.label.SetContentsMargins(l, u, r, d)
	s.git.widget.SetContentsMargins(l, u, r, d)
	s.fileFormat.label.SetContentsMargins(l, u, r, d)
	s.encoding.label.SetContentsMargins(l, u, r, d)
	s.lint.widget.SetContentsMargins(l, u, r, d)
}

func (s *Statusline) setColor() {
	if editor.config.Statusline.ModeIndicatorType == "background" {
		return
	}

	comment := editor.colors.comment.String()
	fg := editor.colors.fg.String()
	bg := editor.colors.bg.String()

	s.file.folderLabel.SetStyleSheet(fmt.Sprintf("color: %s;", comment))
	s.widget.SetStyleSheet(fmt.Sprintf("QWidget#statusline { border-top: 0px solid %s; background-color: %s; } * { color: %s; }", bg, bg, fg))

	svgContent := editor.getSvg("git", nil)
	s.git.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	svgContent = editor.getSvg("bell", nil)
	s.notify.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
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
}

func (s *Statusline) handleUpdates(updates []interface{}) {
	event := updates[0].(string)
	switch event {
	case "bufenter":
		// file := updates[1].(string)
		filetype := updates[1].(string)
		encoding := updates[2].(string)
		fileFormat := updates[3].(string)

		ro := 0
		switch updates[4].(type) {
		case int:
			ro = updates[4].(int)
		case uint:
			ro = int(updates[4].(uint))
		case int64:
			ro = int(updates[4].(int64))
		case uint64:
			ro = int(updates[4].(uint64))
		default:
		}
		if ro == 1 {
			s.file.ro = true
		} else {
			s.file.ro = false
		}

		s.file.redraw()
		s.filetype.redraw(filetype)
		s.encoding.redraw(encoding)
		s.fileFormat.redraw(fileFormat)
		go s.git.redraw(s.ws.filepath)
	default:
		fmt.Println("unhandled statusline event", event)
	}
}

func (s *StatusMode) update() {
	s.s.mode.label.SetText(s.text)
	s.s.mode.label.SetStyleSheet(fmt.Sprintf("color: #ffffff; background-color: %s;", s.bg.String()))
}

func (s *StatusMode) updateStatusline() {
	s.s.file.folderLabel.SetStyleSheet(fmt.Sprintf("color: %s;", newRGBA(230, 230, 230, 1)))
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
		// svgContent := editor.getSvg("hjkl", newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
		svgContent := editor.getSvg("thought", newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
		s.s.mode.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		s.s.mode.label.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))
	case "cmdline_normal":
		text = "Normal"
		bg = hexToRGBA(editor.config.Statusline.CommandModeColor)
		svgContent := editor.getSvg("command", newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
		s.s.mode.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		s.s.mode.label.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))
	case "insert":
		text = "Insert"
		bg = hexToRGBA(editor.config.Statusline.InsertModeColor)
		svgContent := editor.getSvg("edit", newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
		s.s.mode.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		s.s.mode.label.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))
	case "visual":
		text = "Visual"
		bg = hexToRGBA(editor.config.Statusline.VisualModeColor)
		svgContent := editor.getSvg("select", newRGBA(warpColor(fg, 30).R, warpColor(fg, 30).G, warpColor(fg, 30).B, 1))
		s.s.mode.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		s.s.mode.label.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-1, 1, false))
	case "replace":
		text = "Replace"
		bg = hexToRGBA(editor.config.Statusline.ReplaceModeColor)
		svgContent := editor.getSvg("replace", newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
		s.s.mode.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		s.s.mode.label.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-3, 1, false))
	case "terminal-input":
		text = "Terminal"
		bg = hexToRGBA(editor.config.Statusline.TerminalModeColor)
		svgContent := editor.getSvg("terminal", newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
		s.s.mode.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		s.s.mode.label.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize-4, 1, false))
	default:
	}

	s.text = text
	s.bg = bg

	switch editor.config.Statusline.ModeIndicatorType {
	case "none":
		s.s.mode.label.Hide()
		s.s.mode.icon.Hide()
	case "textLabel":
		s.s.mode.icon.Hide()
		s.update()
	case "icon":
		s.s.mode.label.Hide()
	case "background":
		s.s.mode.label.Hide()
		s.s.mode.icon.Hide()
		s.updateStatusline()
	default:
		s.s.mode.label.Hide()
		s.s.mode.icon.Hide()
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

func (s *StatuslineFile) redraw() {
	file := s.s.ws.filepath
	if file == "" {
		file = "[No Name]"
	}

	if s.ro {
		svgContent := editor.getSvg("lock", nil)
		s.roIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		s.roIcon.Show()
	} else {
		s.roIcon.Hide()
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
		switch editor.colors.fg {
		case nil:
			return
		default:
			if editor.config.Statusline.ModeIndicatorType == "background" {
				lintNoErrColor = newRGBA(255, 255, 255, 1)
			} else {
				lintNoErrColor = editor.colors.fg
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
	switch editor.colors.fg {
	case nil:
		return
	default:
		if editor.config.Statusline.ModeIndicatorType == "background" {
			lintNoErrColor = newRGBA(255, 255, 255, 1)
		} else {
			lintNoErrColor = editor.colors.fg
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
